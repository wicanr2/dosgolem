// Command oracle 是 FD2 原版側的受版控執行器：在有界指令預算下以真實 LE
// entry 啟動固定版本 FD2.EXE，於 BIOS 等待邊界接受宣告式輸入，並在每個控制
// 邊界輸出 320×200 索引畫面與同一時點的原始單位／視圖狀態。
//
// 它是 bootprobe 的超集：不指定 -run-dir 時行為與 bootprobe 相同；指定之後
// 進入互動模式，由該目錄下的 control.json 逐步推進，供對拍驅動器使用。
// 互動模式存在的理由是「對拍執行器必須受版控且可重跑」——先前那一版是在
// 容器內動態改寫 bootprobe 產生的，下一個工作階段讀不到它。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

func main() {
	exe := flag.String("exe", "/orig/FD2.EXE", "固定版本原版執行檔")
	root := flag.String("root", "/orig", "唯讀原版資料根目錄")
	frame := flag.String("frame", "", "原生320×200停點PNG輸出（不覆寫）")
	budget := flag.Int("steps", 500000, "有界指令數（1至2000000000）")
	timedKeys := flag.String("timed-keys", "", "指令時點輸入：80000000:enter，逗號分隔且嚴格遞增")
	keys := flag.String("keys", "", "等待邊界的BIOS按鍵序列，以逗號分隔：up,down,left,right,enter,esc")
	state := flag.String("state", "", "獨立可寫檔案覆蓋目錄；空值維持唯讀")
	traceTail := flag.Bool("trace-tail", false, "唯讀記錄最後2000個指令位置")
	dialogueEnters := flag.Int("dialogue-enters", 0, "等待邊界送 Enter 的有界次數")
	dialogueFrames := flag.String("dialogue-frames", "", "逐頁原生收據目錄，必須存在且不得覆寫")
	heapMiB := flag.Int("heap-mib", 32, "明示近堆預算MiB（1至64），非原版配置器位址契約")
	runDir := flag.String("run-dir", "", "互動對拍目錄；空值維持與 bootprobe 相同的一次性行為")
	firstChunk := flag.Int("first-chunk", 700000000, "互動模式第一個控制邊界前的指令數（1至2000000000）")
	waitTimeout := flag.Duration("wait-timeout", 15*time.Minute, "互動模式等待下一個 control.json 的上限")
	cpuProfile := flag.String("cpuprofile", "", "CPU 剖析輸出路徑；只為量測用，預設關閉")
	frameDir := flag.String("frame-dir", "", "逐幀 PNG 輸出目錄；空值不啟用")
	frameStride := flag.Int("frame-stride", 20000, "逐幀取樣間隔（指令數）；0 表示只在 -frame-eip 取樣")
	frameSettle := flag.Int("frame-settle", 0, "內容連續相同幾次取樣才寫出；0 表示一有變化就寫")
	frameMax := flag.Int("frame-max", 4000, "逐幀輸出張數上限")
	frameFrom := flag.Int("frame-from", 0, "逐幀擷取的起始指令數；之前不取樣")
	frameTo := flag.Int("frame-to", 0, "逐幀擷取的結束指令數；0 表示不設上界")
	frameEIP := flag.String("frame-eip", "", "在此 EIP 取一幀（十六進位，如 0x11CAC）；可與 -frame-stride 並用")
	eipWatch := flag.String("eip-watch", "", "逗號分隔的十六進位位址（最多16個）；每一幀記錄各自的累計進入次數")
	flag.Parse()
	if *cpuProfile != "" {
		f, e := os.Create(*cpuProfile)
		if e != nil {
			panic(e)
		}
		if e = pprof.StartCPUProfile(f); e != nil {
			panic(e)
		}
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}
	if *runDir != "" {
		info, e := os.Stat(*runDir)
		if e != nil || !info.IsDir() {
			panic("互動對拍目錄不存在")
		}
		if *firstChunk < 1 || *firstChunk > 2000000000 {
			panic("第一段指令數越界")
		}
		if *waitTimeout <= 0 {
			panic("等待上限必須為正")
		}
	}
	if *heapMiB < 1 || *heapMiB > 64 {
		panic("近堆容量越界")
	}
	if *dialogueEnters < 0 || *dialogueEnters > 512 {
		panic("對話輸入次數越界")
	}
	if *dialogueFrames != "" {
		if info, e := os.Stat(*dialogueFrames); e != nil || !info.IsDir() {
			panic("對話收據目錄不存在")
		}
	}

	// 一次性模式沿用既有上限。互動模式的每一段另外受 control.steps
	// （1..1e8）與 -wait-timeout 兩重限制，所以總預算可以放寬，仍然有界。
	maxBudget := 2000000000
	if *runDir != "" {
		maxBudget = 200000000000
	}
	if *budget < 1 || *budget > maxBudget {
		panic("指令預算越界")
	}
	b, err := os.ReadFile(*exe)
	if err != nil {
		panic(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	if hash != "222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f" {
		panic("FD2 hash mismatch")
	}
	m, err := machine.LoadLE(b)
	if err != nil {
		panic(err)
	}
	if err = machine.InstallDOS4GWBIOSData(m); err != nil {
		panic(err)
	}
	var files interface {
		machine.ReadOnlyFileProvider
		Close() error
	}
	if *state == "" {
		files, err = machine.OpenDirectoryReadOnlyFiles(*root)
	} else {
		files, err = machine.OpenDirectoryOverlayFiles(*root, *state)
	}
	if err != nil {
		panic(err)
	}
	defer files.Close()
	services := machine.NewFD2StartupDOS(files)
	defer services.Close()
	fileCalls := []map[string]any{}
	m.CPU.IntHook = func(c *cpu386.CPU, n byte) bool {
		ax := uint16(c.R[cpu386.EAX])
		path := ""
		observe := n == 0x21 && (ax>>8 == 0x3d || ax>>8 == 0x3c)
		if observe {
			for i := uint32(0); i < 260; i++ {
				v, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+i)
				if !ok || v == 0 {
					break
				}
				path += string([]byte{v})
			}
		}
		handled := services.Handle(c, n)
		if observe {
			fileCalls = append(fileCalls, map[string]any{"AX_in": fmt.Sprintf("%04X", ax), "path": path, "handled": handled, "AX_out": fmt.Sprintf("%04X", uint16(c.R[cpu386.EAX])), "carry": c.EFlags&cpu386.CF != 0})
			if len(fileCalls) > 128 {
				fileCalls = fileCalls[len(fileCalls)-128:]
			}
		}
		return handled
	}
	opl := machine.NewLEOPLPorts()
	services.DPMI.RealModeIO = opl
	m.CPU.PortIn = opl.In8
	m.CPU.PortOut = opl.Out8
	if !machine.InstallLEVideo(m, opl) {
		panic("LEVideo安裝失敗")
	}
	if _, err = machine.InstallFD2WatcomRuntimeWithHeapCapacity(m, uint32(*heapMiB)*1024*1024, services.DPMI); err != nil {
		panic(err)
	}
	if !machine.InstallLEBIOSClock(m, opl) {
		panic("BIOS clock安裝失敗")
	}
	if !machine.InstallLEBIOSKeyboard(m) {
		panic("BIOS鍵盤安裝失敗")
	}
	var input []uint16
	keyNames := map[string]uint16{"up": 0x48e0, "down": 0x50e0, "left": 0x4be0, "right": 0x4de0, "enter": 0x1c0d, "esc": 0x011b}
	if *keys != "" {
		for _, name := range strings.Split(*keys, ",") {
			key, ok := keyNames[strings.TrimSpace(name)]
			if !ok {
				panic("未知按鍵名稱")
			}
			input = append(input, key)
		}
	}
	if len(input) > 100 {
		panic("按鍵序列超過100")
	}
	type scheduledKey struct {
		Step int
		Key  uint16
		Name string
	}
	var scheduled []scheduledKey
	if *timedKeys != "" {
		for _, item := range strings.Split(*timedKeys, ",") {
			pair := strings.Split(strings.TrimSpace(item), ":")
			if len(pair) != 2 {
				panic("排程輸入格式錯誤")
			}
			n, e := strconv.Atoi(pair[0])
			key, ok := keyNames[pair[1]]
			if e != nil || !ok || n < 0 || n >= *budget || (len(scheduled) > 0 && n <= scheduled[len(scheduled)-1].Step) {
				panic("排程輸入時點／名稱無效")
			}
			scheduled = append(scheduled, scheduledKey{n, key, pair[1]})
		}
	}
	if len(scheduled)+len(input)+*dialogueEnters > 512 {
		panic("總輸入超過100")
	}
	timedIndex := 0
	delivered := []scheduledKey{}
	inputIndex := 0
	waiting := false
	steps := 0
	mainStep := -1
	var stop error
	var instructionEIP uint32
	tail := make([]uint32, 0, 2000)
	// 互動對拍狀態。controlSeq 與 chunkEnd 只在 -run-dir 有值時生效。
	controlSeq := 0
	chunkEnd := *firstChunk
	if *runDir == "" {
		chunkEnd = *budget + 1
	}
	// capture 把同一時點的畫面與原始狀態一起落地。位址取自固定版本
	// FD2.EXE：0x53A45 單位陣列基底、0x53BEB 單位數、0x53AA9..0x53ABD 視圖、
	// 0x53BEF 回合。每筆單位保留完整 80 byte raw_hex，欄位只是導覽。
	// withFrame=false 時只寫狀態 JSON。current 一定與剛寫好的
	// checkpoint-NNNN 同一時點，再編一次 PNG 沒有新資訊，卻是細粒度追蹤
	// 每一步最大的一筆固定成本。
	// readViewGlobals 只讀固定版本 FD2.EXE 的視圖全域：0x53AA9..0x53ABD 與
	// 0x53BEF，外加 0x51A83 的 overlay selector（0x122DC 用它決定畫哪一組範圍
	// ／游標圖示，0 表示不畫）。逐幀擷取與控制邊界收據共用同一份讀法，避免
	// 兩邊漂移。
	readViewGlobals := func() map[string]uint32 {
		view := map[string]uint32{}
		for key, addr := range map[string]uint32{
			"camera_x": 0x53aa9, "camera_y": 0x53aad,
			"cursor_x": 0x53ab1, "cursor_y": 0x53ab5,
			"visible_x": 0x53ab9, "visible_y": 0x53abd,
			"round": 0x53bef, "overlay_selector": 0x51a83,
		} {
			v, _ := m.Read32(addr)
			view[key] = v
		}
		return view
	}
	capture := func(label string, withFrame bool) {
		pixels := m.Video.Indexed()
		palette := m.Video.Palette()
		pal := make(color.Palette, 256)
		for i, v := range palette {
			pal[i] = color.RGBA{v[0], v[1], v[2], 255}
		}
		if withFrame && len(pixels) == 64000 {
			pic := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
			copy(pic.Pix, pixels)
			f, e := os.Create(filepath.Join(*runDir, label+".png"))
			if e != nil {
				panic(e)
			}
			if e = png.Encode(f, pic); e != nil {
				panic(e)
			}
			f.Close()
		}
		base, _ := m.Read32(0x53a45)
		count, _ := m.Read32(0x53beb)
		units := []map[string]any{}
		if count <= 128 && uint64(base)+uint64(count)*80 <= uint64(len(m.Mem)) {
			for i := uint32(0); i < count; i++ {
				r := m.Mem[base+80*i : base+80*(i+1)]
				units = append(units, map[string]any{
					"index": i, "x": r[0], "y": r[1], "pose": r[3], "motion": r[4],
					"byte5": r[5], "camp": r[6], "fig": r[7], "identity": r[8],
					"level": r[0x21], "exp": r[0x3c],
					"hp":      uint16(r[0x40]) | uint16(r[0x41])<<8,
					"raw_hex": fmt.Sprintf("%x", r),
				})
			}
		}
		view := readViewGlobals()
		// BIOS 環形緩衝的頭尾在 0x41a／0x41c，兩者相差即尚未被遊戲取走的按鍵
		// 數。驅動端要靠它決定「這一格該不該再送鍵」——固定速率送鍵會在遊戲
		// 消化不及時撐爆緩衝。kbd_reads 是遊戲實際取走的鍵數，也就是有效推進
		// 次數，和送出的鍵數不是同一件事。
		head, _ := m.Read16(0x41a)
		tail, _ := m.Read16(0x41c)
		pending := (int(tail) - int(head) + 32) % 32 / 2
		reads := uint64(0)
		if m.Keyboard != nil {
			reads = m.Keyboard.Reads
		}
		// 誰在等鍵盤。快照都取在 BIOS 等待點，那時堆疊上留著這一條輸入路徑的
		// 返回位址；把落在程式碼範圍的值收起來，就是「現在是哪一個介面在收鍵」
		// 的可觀測特徵。驅動端需要它才分得出地圖游標、指令環、系統選單與對白——
		// 這四種在 eip、view 與 units 上都看不出差別（overlay selector 恆為 1），
		// 沒有這條鏈就只能猜，猜錯了會把方向鍵送進選單裡。
		// 掃堆疊取「真正的返回位址」：只收前面五個 byte 是 `E8`（call rel32）的
		// 值。直接收所有落在程式碼範圍的 dword 會把殘值一起帶進來，同一個介面
		// 每次取樣的前幾項都不一樣；走 EBP 連結串則只拿得到一層——Watcom 這份
		// 執行檔省略了 frame pointer。
		//
		// 這條鏈是驅動端唯一分得出 UI 模式的東西：地圖游標、指令環、目標選擇、
		// 系統選單與對白在 eip、view 與 units 上都一樣（overlay selector 恆為 1）。
		isCallSite := func(addr uint32) bool {
			if addr < 5 || uint64(addr) >= uint64(len(m.Mem)) {
				return false
			}
			return m.Mem[addr-5] == 0xE8
		}
		chain := []string{}
		for offset := uint32(0); offset < 512 && len(chain) < 10; offset += 4 {
			v, e := m.Read32(m.CPU.R[cpu386.ESP] + offset)
			if e != nil {
				break
			}
			if v >= 0x10000 && v < 0x60000 && isCallSite(v) {
				chain = append(chain, fmt.Sprintf("0x%X", v))
			}
		}
		d, _ := json.Marshal(map[string]any{
			"schema": 1, "runner": "dosgolem", "input_kind": "normal BIOS keys",
			"state_injections": []string{}, "steps": steps,
			"eip":         fmt.Sprintf("0x%X", m.CPU.EIP),
			"control_seq": controlSeq, "unit_base": base,
			"kbd_pending": pending, "kbd_reads": reads,
			"input_chain": chain,
			"view":        view, "registers": m.CPU.R, "units": units,
		})
		if e := os.WriteFile(filepath.Join(*runDir, label+".json"), d, 0o600); e != nil {
			panic(e)
		}
	}
	dialogueArmed, dialogueIndex := false, 0
	dialogueReceipts := []map[string]any{}

	// ── 逐幀擷取 ────────────────────────────────────────────────────────
	// mode13 沒有「換頁」這個動作：程式直接畫進 0xA0000，所以一幀的邊界要嘛
	// 由取樣認定（內容變了就是新的一幀），要嘛由遊戲自己的繪圖進入點認定
	// （-frame-eip）。兩者都只讀 VGA 記憶體與 EIP，不改執行語意；旗標沒給就
	// 完全不進入這條路徑。
	//
	// 直接畫進畫面的程式在取樣點可能剛好畫到一半。-frame-settle 要求內容連續
	// 相同幾次才寫出，用來過濾這種半成品；預設 0 表示不過濾，把每一個中間狀態
	// 都留下來。
	frameEIPValue := uint64(0)
	if *frameEIP != "" {
		v, e := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(*frameEIP, "0x"), "0X"), 16, 32)
		if e != nil {
			panic("frame-eip 不是十六進位位址")
		}
		frameEIPValue = v
	}
	if *frameDir != "" && *frameStride == 0 && frameEIPValue == 0 {
		panic("frame-dir 需要 -frame-stride 或 -frame-eip 至少一項")
	}
	if *frameStride < 0 || *frameSettle < 0 || *frameMax < 1 ||
		*frameFrom < 0 || *frameTo < 0 || (*frameTo > 0 && *frameTo <= *frameFrom) {
		panic("逐幀參數越界")
	}
	// eip-watch 是找繪圖路徑用的最小工具：對少數幾個位址計次，每一幀把當下的
	// 累計值寫進收據。要回答「這一段畫面到底是誰畫的」時，逐幀畫面告訴你
	// 「有沒有畫」，這個計數器告訴你「誰被呼叫了」。位址個數上限 16，用線性
	// 掃描比對；沒給旗標就完全不比。
	var eipWatchAddrs []uint32
	var eipWatchHits []uint64
	for _, part := range strings.Split(*eipWatch, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, e := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(part, "0x"), "0X"), 16, 32)
		if e != nil {
			panic("eip-watch 不是十六進位位址")
		}
		eipWatchAddrs = append(eipWatchAddrs, uint32(v))
		eipWatchHits = append(eipWatchHits, 0)
	}
	if len(eipWatchAddrs) > 16 {
		panic("eip-watch 位址超過 16 個")
	}
	frameIndex, frameNext, framePendingCount := 0, 0, 0
	var frameLast, framePending [32]byte
	var frameLog *os.File
	if *frameDir != "" {
		if e := os.MkdirAll(*frameDir, 0o700); e != nil {
			panic(e)
		}
		f, e := os.OpenFile(filepath.Join(*frameDir, "frames.jsonl"),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if e != nil {
			panic(e)
		}
		frameLog = f
		defer frameLog.Close()
	}
	captureFrame := func(reason string) {
		if frameLog == nil || frameIndex >= *frameMax {
			return
		}
		if steps < *frameFrom || (*frameTo > 0 && steps > *frameTo) {
			return
		}
		pixels := m.Video.Indexed()
		if len(pixels) != 64000 {
			return
		}
		sum := sha256.Sum256(pixels)
		if sum == frameLast {
			framePendingCount = 0
			return
		}
		if *frameSettle > 0 {
			if sum != framePending {
				framePending, framePendingCount = sum, 1
				return
			}
			framePendingCount++
			if framePendingCount <= *frameSettle {
				return
			}
		}
		palette := m.Video.Palette()
		pal := make(color.Palette, 256)
		for i, v := range palette {
			pal[i] = color.RGBA{v[0], v[1], v[2], 255}
		}
		pic := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
		copy(pic.Pix, pixels)
		name := fmt.Sprintf("frame-%06d.png", frameIndex)
		f, e := os.OpenFile(filepath.Join(*frameDir, name),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if e != nil {
			panic(e)
		}
		if e = png.Encode(f, pic); e != nil {
			panic(e)
		}
		if e = f.Close(); e != nil {
			panic(e)
		}
		base, _ := m.Read32(0x53a45)
		count, _ := m.Read32(0x53beb)
		record := map[string]any{
			"index": frameIndex, "file": name, "step": steps, "reason": reason,
			"eip":            fmt.Sprintf("0x%X", instructionEIP),
			"indexed_sha256": fmt.Sprintf("%x", sum),
			"view":           readViewGlobals(),
			"unit_base":      base,
			"unit_count":     count,
			"port_3da_reads": opl.Reads[0x3da],
			"palette_writes": opl.Writes[0x3c9],
		}
		if len(eipWatchAddrs) > 0 {
			watch := map[string]uint64{}
			for i, addr := range eipWatchAddrs {
				watch[fmt.Sprintf("0x%X", addr)] = eipWatchHits[i]
			}
			record["eip_watch"] = watch
		}
		d, _ := json.Marshal(record)
		if _, e = fmt.Fprintf(frameLog, "%s\n", d); e != nil {
			panic(e)
		}
		frameLast = sum
		framePendingCount = 0
		frameIndex++
	}

	for ; steps < *budget; steps++ {
		if *runDir != "" && steps >= chunkEnd {
			capture(fmt.Sprintf("checkpoint-%04d", controlSeq), true)
			capture("current", false)
			deadline := time.Now().Add(*waitTimeout)
			// 輪詢間隔由短往長退避。固定 100ms 會讓細粒度追蹤整段被輪詢延遲
			// 支配：每格 10 萬指令只要約 8ms 執行，卻要等一次 100ms。
			poll := 200 * time.Microsecond
			for {
				if time.Now().After(deadline) {
					panic("有界普通輸入等待逾時")
				}
				d, e := os.ReadFile(filepath.Join(*runDir, "control.json"))
				var cmd struct {
					Seq   int
					Key   string
					Steps int
					Stop  bool
				}
				if e == nil && json.Unmarshal(d, &cmd) == nil && cmd.Seq > controlSeq {
					if cmd.Stop {
						capture("final", true)
						os.Exit(0)
					}
					if cmd.Steps < 1 || cmd.Steps > 100000000 {
						panic("互動步數越界")
					}
					if cmd.Key != "" {
						k, ok := keyNames[cmd.Key]
						if !ok {
							panic("未知普通鍵")
						}
						if e = m.Keyboard.Enqueue(k); e != nil {
							panic(e)
						}
					}
					controlSeq, chunkEnd = cmd.Seq, steps+cmd.Steps
					f, e := os.OpenFile(filepath.Join(*runDir, "control-history.jsonl"),
						os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
					if e != nil {
						panic(e)
					}
					fmt.Fprintf(f, "%s\n", d)
					f.Close()
					break
				}
				time.Sleep(poll)
				if poll < 20*time.Millisecond {
					poll *= 2
				}
			}
		}
		if m.CPU.EIP == 0x25bf4 && mainStep < 0 {
			mainStep = steps
		}
		if timedIndex < len(scheduled) && scheduled[timedIndex].Step == steps {
			if e := m.Keyboard.Enqueue(scheduled[timedIndex].Key); e != nil {
				panic(e)
			}
			delivered = append(delivered, scheduled[timedIndex])
			timedIndex++
		}
		instructionEIP = m.CPU.EIP
		for i, addr := range eipWatchAddrs {
			if instructionEIP == addr {
				eipWatchHits[i]++
				break
			}
		}
		if frameLog != nil {
			if frameEIPValue != 0 && uint64(instructionEIP) == frameEIPValue {
				captureFrame("eip")
			}
			if *frameStride > 0 && steps >= frameNext {
				frameNext = steps + *frameStride
				captureFrame("stride")
			}
		}
		if *dialogueEnters > 0 || *dialogueFrames != "" {
			if instructionEIP == 0x16c57 {
				dialogueArmed = true
			}
			if instructionEIP == 0x16cf3 && dialogueArmed {
				dialogueArmed = false
				pixels := m.Video.Indexed()
				if len(pixels) != 64000 {
					panic("對話等待沒有完整mode13畫面")
				}
				palette := m.Video.Palette()
				pal := make(color.Palette, 256)
				for i, v := range palette {
					pal[i] = color.RGBA{v[0], v[1], v[2], 255}
				}
				snapshot := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
				copy(snapshot.Pix, pixels)
				var encoded bytes.Buffer
				if err := png.Encode(&encoded, snapshot); err != nil {
					panic(err)
				}
				path := ""
				if *dialogueFrames != "" {
					path = filepath.Join(*dialogueFrames, fmt.Sprintf("dialogue-%03d.png", dialogueIndex))
					f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if e != nil {
						panic(e)
					}
					_, e = f.Write(encoded.Bytes())
					ce := f.Close()
					if e != nil {
						panic(e)
					}
					if ce != nil {
						panic(ce)
					}
				}
				anchor, e := m.Read32(0x53c67)
				if e != nil {
					panic(e)
				}
				dialogueReceipts = append(dialogueReceipts, map[string]any{"index": dialogueIndex, "step": steps, "eip": "0x16CF3", "path": path, "portrait_anchor": anchor, "sha256": fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())), "indexed_sha256": fmt.Sprintf("%x", sha256.Sum256(pixels))})
				if dialogueIndex == *dialogueEnters {
					waiting = true
					break
				}
				if e := m.Keyboard.Enqueue(0x1c0d); e != nil {
					panic(e)
				}
				dialogueIndex++
			}
		}

		if *traceTail && steps >= *budget-2000 {
			tail = append(tail, instructionEIP)
		}
		if stop = m.CPU.Step(); stop != nil {
			break
		}
		if m.Keyboard.Waiting {
			if inputIndex == len(input) {
				if timedIndex < len(scheduled) {
					continue
				}
				if *runDir != "" {
					// 互動模式把等待邊界交還控制迴圈，由 control.json 決定
					// 下一個按鍵；一次性模式維持原本的「停下並回報等待」。
					chunkEnd = steps + 1
					continue
				}
				waiting = true
				break
			}
			if err := m.Keyboard.Enqueue(input[inputIndex]); err != nil {
				panic(err)
			}
			inputIndex++
		}
	}
	end := int(instructionEIP) + 16
	if end > len(m.Mem) {
		end = len(m.Mem)
	}
	raw := ""
	if int(instructionEIP) < end {
		raw = fmt.Sprintf("% X", m.Mem[instructionEIP:uint32(end)])
	}
	msg := "step budget exhausted"
	if waiting {
		msg = "waiting for BIOS keyboard input"
	}
	if stop != nil {
		msg = stop.Error()
	}
	r := map[string]any{"schema": 1, "runner": "dosgolem", "exe_sha256": hash, "steps_completed": steps, "step_budget": *budget, "main_entry_step": mainStep, "stop_eip": fmt.Sprintf("0x%X", instructionEIP), "cpu_eip_after_error": fmt.Sprintf("0x%X", m.CPU.EIP), "address_space": "dosgolem relocated LE linear", "next_bytes": raw, "error": msg, "registers": m.CPU.R, "esp": fmt.Sprintf("0x%X", m.CPU.R[cpu386.ESP]), "state_injections": []string{}, "runtime_adapter": "existing InstallFD2WatcomRuntime and FD2StartupDOS", "original_root_read_only": true, "game_screen_produced": false, "normal_player_path_verified": false}

	if instructionEIP == 0x36d98 {
		stack := make([]uint32, 4)
		valid := true
		for i := range stack {
			v, e := m.Read32(m.CPU.R[cpu386.ESP] + uint32(i*4))
			if e != nil {
				valid = false
				break
			}
			stack[i] = v
		}
		if valid {
			regs := make([]uint32, 7)
			for i := range regs {
				v, e := m.Read32(stack[2] + uint32(i*4))
				if e != nil {
					valid = false
					break
				}
				regs[i] = v
			}
			r["watcom_int386_input"] = map[string]any{"stack": stack, "regs": regs, "valid": valid, "abi": "existing cdecl REGS; read-only observation"}
		}
	}
	r["heap_capacity_bytes"] = uint32(*heapMiB) * 1024 * 1024
	r["instruction_tail"] = tail
	r["state_directory"] = *state
	r["dos_file_calls"] = fileCalls
	r["keyboard"] = m.Keyboard
	r["dialogue_enter_count"] = dialogueIndex
	r["dialogue_receipts"] = dialogueReceipts
	r["dialogue_input_method"] = "明示啟用的16C57呼叫首次16CF3等待邊界；只送BIOS Enter，不改遊戲狀態"
	r["input_schedule"] = "BIOS等待邊界及明示指令時點；未注入遊戲狀態"
	r["timed_keys_requested"] = scheduled
	r["timed_keys_delivered"] = delivered
	r["waiting_for_input"] = waiting
	pixels := m.Video.Indexed()
	nonzero := 0
	for _, v := range pixels {
		if v != 0 {
			nonzero++
		}
	}
	r["video_mode"] = m.Video.Mode
	r["video_mode_sets"] = m.Video.ModeSets
	r["video_nonzero_pixels"] = nonzero
	r["video_indexed_sha256"] = fmt.Sprintf("%x", sha256.Sum256(pixels))
	r["video_palette_writes"] = opl.Writes[0x3c9]
	if *frame != "" {
		if len(pixels) != 64000 {
			panic("尚無mode13畫面可擷取")
		}
		palette := m.Video.Palette()
		pal := make(color.Palette, 256)
		for i, v := range palette {
			pal[i] = color.RGBA{v[0], v[1], v[2], 255}
		}
		snapshot := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
		copy(snapshot.Pix, pixels)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, snapshot); err != nil {
			panic(err)
		}
		f, err := os.OpenFile(*frame, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			panic(err)
		}
		_, err = f.Write(encoded.Bytes())
		closeErr := f.Close()
		if err != nil {
			panic(err)
		}
		if closeErr != nil {
			panic(closeErr)
		}
		r["frame"] = map[string]any{"path": *frame, "sha256": fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())), "classification": "dosgolem自身停點畫面，完整度與玩家路徑尚待驗證"}
	}

	if uint64(instructionEIP)+64 <= uint64(len(m.Mem)) {
		r["next_bytes_64"] = fmt.Sprintf("% X", m.Mem[instructionEIP:instructionEIP+64])
	}
	r["pit0"] = opl.PIT0
	r["pit_timing"] = "hardware-spec approximation: 1 us per protected/real instruction; 1193182Hz PIT; default BIOS IRQ0 only"
	r["bios_clock"] = opl.BIOSClock
	if v, e := m.Read32(0x46c); e == nil {
		r["bios_ticks"] = v
	}
	r["protected_port_routing"] = "strict platform dispatch; unknown output rejected"
	r["bios_profile"] = "existing Machine.initBDA, DOS4GW selector 0040 base 0400"
	r["dpmi_real_mode_last"] = services.DPMI.RealModeLast
	r["dpmi_real_mode_history"] = services.DPMI.RealModeHistory
	r["opl_ports"] = opl.Log
	r["opl_reads"] = opl.Reads
	r["opl_writes"] = opl.Writes
	r["vga_timing"] = "hardware-spec approximation: existing deterministic status-read ticks"
	r["device_state"] = opl.State()
	r["dma_completions"] = opl.DMACompletions
	r["irq7_deliveries"] = opl.IRQ7Deliveries
	r["pcm_observed_bytes"] = len(opl.PCM)
	ivt := map[string]string{}
	for _, n := range []uint32{8, 15, 0x66} {
		v, e := m.Read32(n * 4)
		if e == nil {
			ivt[fmt.Sprintf("%02X", n)] = fmt.Sprintf("%04X:%04X", v>>16, v&0xffff)
		}
	}
	r["memory_ivt"] = ivt
	r["pic_profile"] = "DOSBox-X observed startup IMR 21=F8 A1=2C; mask registers and IRQ7/non-specific EOI subset"
	r["sb_profile"] = "SB16 DSP 4.05; base 0220; IRQ 7; DMA 1; HDMA 5; unsupported commands rejected"
	r["dsp_timing"] = "hardware-spec approximation: immediate reset; 1 us per real-mode instruction; rational sample rate 1000000/(256-TC), reset default 22050 Hz; no audio output"
	r["opl_timing"] = "hardware-spec approximation: existing immediate timer detection model"
	r["dpmi_calls"] = services.DPMI.Calls
	r["dpmi_unimplemented"] = services.DPMI.Unimplemented
	r["dos_memory"] = services.DPMI.DOSMemory()
	seg, off := services.DPMI.RealModeVector(0x66)
	r["real_mode_vector_66"] = fmt.Sprintf("%04X:%04X", seg, off)
	if uint16(m.CPU.R[cpu386.EAX]) == 0x0300 {
		packet := make([]byte, 50)
		valid := true
		for i := range packet {
			v, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegES], m.CPU.R[cpu386.EDI]+uint32(i))
			if !ok {
				valid = false
				break
			}
			packet[i] = v
		}
		if valid {
			r["real_mode_packet_hex"] = fmt.Sprintf("% X", packet)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		panic(err)
	}
	if stop != nil {
		os.Exit(2)
	}
}
