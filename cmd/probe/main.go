// Command probe 把一個 DOS 執行檔餵進 dosgolem，跑到停下來為止，
// 然後印出**它到底做了什麼**。
//
// 這不是 oracle API（那是 `docs/spec/005`，還是 DRAFT），是 MVP-B 的
// 診斷工具：判斷「跑得動」與「跑得動但行為不對」的差別
// （`docs/spec/004` §1.3）。
//
// ⚠ **本專案不含任何原版檔案**，`-exe` 與 `-root` 都由玩家自備。
//
//	go run ./cmd/probe -exe path/to/RUN_full.EXE -root path/to/RICH2 -steps 5000000
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// traceFilePath 是 -trace-file 的值；report 在另一個函式裡，所以放套件層。
var traceFilePath string

// portLogFrom／portLogTo 是 -port-log-from／-to 的值，report 在別的函式裡。
var portLogFrom, portLogTo uint64

func main() {
	exe := flag.String("exe", "", "要跑的執行檔（必填；MZ 或 .COM，看檔頭 magic 自動判斷）")
	root := flag.String("root", ".", "原版素材目錄")
	steps := flag.Uint64("steps", 20_000_000, "最多執行幾道指令")
	trace := flag.Uint64("trace", 0, "最後幾道指令的軌跡（0 ＝ 不記）")
	dumpVRAM := flag.String("dump-vram", "", "把 A0000 的 320×200 色號陣列寫到這個檔")
	dumpPal := flag.String("dump-palette", "", "把 256×3 的 RGB 調色盤寫到這個檔")
	peek := flag.String("peek", "", "跑完之後印出這些位址的內容，逗號分隔，"+
		"格式 <IDA 線性位址>:<長度> 或 ds:<偏移>:<長度>")
	mouseX := flag.Int("mouse-x", -1, "滑鼠要移到的像素 X（−1 ＝ 不動）")
	mouseY := flag.Int("mouse-y", -1, "滑鼠要移到的像素 Y")
	mouseAt := flag.Uint64("mouse-at", 0, "第幾道指令時移動滑鼠（0 ＝ steps 的一半）")
	clickX := flag.Int("click-x", -1, "點擊的像素 X（−1 ＝ 不點）")
	clickY := flag.Int("click-y", -1, "點擊的像素 Y")
	clickAt := flag.Uint64("click-at", 0, "第幾道指令時按下")
	clickHold := flag.Uint64("click-hold", 2_000_000, "按住幾道指令")
	clickPolls := flag.Int("click-polls", 0,
		"改用「按住到遊戲讀了 N 次滑鼠才放開」（0 ＝ 用 -click-hold 的指令數）。"+
			"各畫面的輪詢頻率差三個數量級——磁片提示每千萬道只問 12 次，讀檔選單問一千次——"+
			"固定指令數的按住不是漏掉就是重複觸發幾千次")
	vgaRows := flag.String("vga-trace-rows", "", "記錄寫進這幾列的 planar 寫入與顯示卡狀態：`起-迄`")
	vgaCols := flag.String("vga-trace-cols", "", "配 -vga-trace-rows：只記這幾個位元組欄（一欄八像素）`起-迄`")
	portFrom := flag.Uint64("port-log-from", 0, "印出這一段之後的 I/O 埠寫入序列（0 ＝ 不印）")
	portTo := flag.Uint64("port-log-to", 0, "配 -port-log-from 用")
	rowWrites := flag.Uint64("row-writes-from", 0,
		"從第幾道指令開始統計 planar 的每列寫入量（0 ＝ 不統計）")
	sweepSpec := flag.String("sweep", "",
		"掃描點擊：`起始步數:每點步數:x0:y0:x1:y1:格距`。逐格點一次，"+
			"每點前後印畫面雜湊——找互動熱點時不要用眼睛猜座標一次跑一個")
	pokeScript := flag.String("poke", "",
		"跑到某一步就改記憶體：`步數:線性位址=值[:值…]`（十六進位位址與值，"+
			"逗號分隔多組）。用來直接把局面設成要對拍的樣子，"+
			"不必靠遊戲內的隨機或一路點進去")
	shotScript := flag.String("shots", "",
		"在指定步數各存一張畫面：`步數:路徑` 用逗號分隔。"+
			"色盤存成同名 .pal。一次跑要看好幾個畫面時用這個，"+
			"不要為了看中途的畫面重跑")
	clickScript := flag.String("clicks", "",
		"點擊腳本：`步數:X:Y[:鍵]` 用逗號分隔（鍵 1 ＝ 左、2 ＝ 右，預設 1）。"+
			"按住時間用 -click-hold")
	watchVideo := flag.Bool("watch-video", false,
		"統計寫進 A0000–BFFFF 的位址範圍（回答「它到底畫在哪裡」）")
	logCalls := flag.Bool("log-calls", false, "統計每一種 (中斷, AH) 呼叫幾次")
	tick := flag.Uint64("tick", 0, "每幾道指令送一次計時器中斷（0 ＝ 用預設）")
	keys := flag.String("keys", "", "先排進鍵盤佇列的按鍵（`\\n` 是 Enter）")
	keysAt := flag.String("keys-at", "",
		"在指定步數送按鍵：`步數:字串` 用逗號分隔（`\\n` 是 Enter）。"+
			"一開始就排進佇列的按鍵，程式還沒開始輪詢就被吃掉了")
	args := flag.String("args", "", "命令列尾（寫進 PSP+80h，.COM 的參數走這裡）")
	queue := flag.String("queue", "", "主程式結束／常駐後接著跑的程式（監督佇列，`docs/spec/009` §4），逗號分隔")
	dumpCGA := flag.String("dump-cga", "", "把 B8000 當 CGA mode 06h（640×200 雙 bank）畫成 PNG")
	segLog := flag.Bool("seg-log", false, "記錄 CS 的每一次改變，報告裡印出每個段第一次執行的時間與來源")
	dumpMem := flag.String("dump-mem", "",
		"跑完把幾段線性記憶體各寫成一個檔：`<lo>-<hi>:<路徑>`（位址十六進位），"+
			"逗號分隔多段。一次跑要挖好幾塊緩衝區時用這個，不要為了第二塊重跑")
	dumpScreen := flag.String("dump-screen", "", "跑完把畫面的色號寫成檔案（planar 模式是 VideoSize() 那個尺寸）")
	watch := flag.String("watch", "", "監看一段線性位址的寫入：<lo>-<hi>（十六進位）")
	watchDS := flag.String("watch-ds", "", "記下 DS 每一次被設成這個段值的時刻（十六進位）")
	flag.StringVar(&traceFilePath, "trace-file", "", "把 -trace 的軌跡寫到這個檔，不印在畫面上")
	callArgs := flag.String("call-args", "",
		"每次執行到某個 CS:IP 就把堆疊上的參數印出來："+
			"`CS:IP:字數:起:迄`（位址十六進位，步數十進位）。"+
			"位置取進入點（尚未 push bp），所以參數從 SS:SP+4 起算——遠呼叫的返回位址佔 4 bytes")
	argRegs := flag.Bool("arg-regs", false,
		"配 -call-args／-frame-args：連 AX BX CX DX SI DI ES BP 一起印。"+
			"繪圖驅動有些參數走暫存器不走堆疊")
	frameArgs := flag.String("frame-args", "",
		"同 -call-args，但位址在 prologue 之後：參數從 `SS:BP+6` 取。"+
			"反組譯給的通常是函式中間那幾行，用這個不必猜進入點")
	ipLog := flag.String("ip-log", "",
		"把 [起,迄) 這段每一道指令的 CS:IP 以二進位寫出來（每筆 4 bytes，小端 CS 後 IP）：`起:迄:路徑`。"+
			"用來對兩次只差一個輸入的執行，找出控制流第一次分岔的位置")
	saveState := flag.String("save-state", "",
		"跑到某一步就把整台機器存成檔案：`步數:路徑`，逗號分隔多個檢查點。"+
			"配 -load-state 用——要觀測的畫面在幾億道指令之後時，"+
			"存一次，之後每個實驗從那裡展開，一輪從幾分鐘變成幾秒")
	loadState := flag.String("load-state", "",
		"從狀態檔接著跑（-save-state 存的）。這時 -exe 不必給；"+
			"步數從存檔當時算起，-steps／-clicks／-shots 的數字都照舊")
	flag.Parse()

	if *exe == "" && *loadState == "" {
		flag.Usage()
		os.Exit(2)
	}

	m := machine.New()
	var err error
	if *loadState == "" {
		img, rerr := os.ReadFile(*exe)
		if rerr != nil {
			die(rerr)
		}
		// 副檔名不是判準：看 MZ magic。不是 MZ 就當 .COM（無檔頭、載到 PSP+100h）。
		if len(img) >= 2 && img[0] == 'M' && img[1] == 'Z' {
			err = m.LoadEXE(img)
		} else {
			err = m.LoadCOM(img)
		}
		if err != nil {
			die(err)
		}
	}
	if *args != "" && *loadState == "" {
		// 命令列尾：PSP+80h ＝ 長度 ＋ 內容 ＋ CR。
		b := []byte(*args)
		if len(b) > 126 {
			b = b[:126]
		}
		psp := uint32(machine.PSPSeg) * 16
		m.Write8(psp+0x80, uint8(len(b)))
		m.WriteBytes(psp+0x81, b)
		m.Write8(psp+0x81+uint32(len(b)), 0x0D)
	}
	if *tick > 0 {
		m.IRQ0Every = *tick
	}
	m.TraceSegs = *segLog
	m.RowWritesFrom = *rowWrites
	if *vgaRows != "" {
		if *vgaCols != "" {
			if _, err := fmt.Sscanf(*vgaCols, "%d-%d", &m.VGATraceCol0, &m.VGATraceCol1); err != nil {
				die(err)
			}
		}
		if _, err := fmt.Sscanf(*vgaRows, "%d-%d", &m.VGATraceRow0, &m.VGATraceRow1); err != nil {
			fmt.Fprintln(os.Stderr, "vga-trace-rows 格式要 起-迄：", err)
			os.Exit(2)
		}
	}
	portLogFrom, portLogTo = *portFrom, *portTo
	if *watchDS != "" {
		var v uint16
		if _, err := fmt.Sscanf(*watchDS, "%x", &v); err != nil {
			die(err)
		}
		m.WatchDS, m.WatchDSOn = v, true
	}
	type memWrite struct {
		addr    uint32
		old, nw uint8
		step    uint64
		cs, ip  uint16
	}
	var writes []memWrite
	var dropped int
	if *watch != "" {
		var lo, hi uint32
		if _, err := fmt.Sscanf(*watch, "%x-%x", &lo, &hi); err != nil {
			die(err)
		}
		// **保留最後 20000 筆，不是前 20000 筆。** 要找的通常是「誰最後
		// 寫壞了它」；砍前面那版會在開機階段就填滿，之後真正的兇手一筆都不留。
		m.WatchWrites(lo, hi, func(a uint32, old, nw uint8) {
			w := memWrite{a, old, nw, m.Steps, m.CPU.Seg[cpu.CS], m.CPU.IP}
			if len(writes) < 20000 {
				writes = append(writes, w)
				return
			}
			copy(writes, writes[1:])
			writes[len(writes)-1] = w
			dropped++
		})
	}
	var vidLo, vidHi uint32 = 0xFFFFFFFF, 0
	var vidN int
	if *watchVideo {
		m.WatchWrites(0xA0000, 0xBFFFF, func(a uint32, old, nv uint8) {
			vidN++
			if a < vidLo {
				vidLo = a
			}
			if a > vidHi {
				vidHi = a
			}
		})
	}
	d := dos.New(m, *root)
	if *queue != "" {
		for _, q := range strings.Split(*queue, ",") {
			d.Enqueue(strings.TrimSpace(q), "")
		}
	}
	if *logCalls {
		d.Calls = map[dos.Call]int{}
	}
	d.Install()
	if *loadState != "" {
		if err := state.Load(*loadState, m, d); err != nil {
			die(err)
		}
		fmt.Printf("從 %s 接著跑（第 %d 道指令）\n", *loadState, m.Steps)
	}
	saves, err := parseSaveState(*saveState)
	if err != nil {
		die(err)
	}
	if *keys != "" {
		d.Stdin = append(d.Stdin, []byte(strings.ReplaceAll(*keys, "\\n", "\n"))...)
	}

	// **游標是畫面內容的一部分**——遊戲自己畫那隻小手（16×27）。
	// 兩邊位置不同的話逐點比對會在兩個位置各差一整塊，而畫面看起來完全正常。
	//
	// ⚠ **設初始值沒有用**：遊戲只在座標**變化**時才重畫游標，
	// 一開始就等於目標值的話它一次都不會動（實測改了初始值，
	// 差異一個像素都沒少）。所以要在跑的中途真的移動一次。
	moveAt := *mouseAt
	if moveAt == 0 {
		moveAt = *steps / 2
	}

	clicks, err := parseClicks(*clickScript)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	pokes, err := parsePokes(*pokeScript)
	if err != nil {
		die(err)
	}
	shots, err := parseShots(*shotScript)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	keyPlan, err := parseShots(*keysAt) // 同樣是 步數:字串
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sweep, err := parseSweep(*sweepSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var lastSum string
	pollsAtPress := 0
	held := false
	downIdx := -1

	ca, err := parseCallArgs(*callArgs, false)
	if err == nil && ca == nil {
		ca, err = parseCallArgs(*frameArgs, true)
	}
	if ca != nil {
		ca.regs = *argRegs
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ipw, ipFrom, ipTo, err := openIPLog(*ipLog)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if ipw != nil {
		defer ipw.close()
	}

	ring := newRing(*trace)
	var runErr error
	for m.Steps < *steps && !m.CPU.Halted && !d.Exited {
		// **護欄：程式碼不該跑進 A0000 以上。** 那裡是視訊記憶體與 BIOS，
		// 在我們這台上全是 0，而 `00 00` ＝ `add [bx+si],al` 一路解得下去，
		// 所以飛掉之後不會有任何錯誤——只會安靜地走完幾百萬道指令
		// （第一次實驗就是這樣，停在 A303:60BD）。
		if a := cpu.Addr(m.CPU.Seg[cpu.CS], m.CPU.IP); a >= machine.VideoSeg*16 {
			runErr = fmt.Errorf("跑出可用記憶體：CS:IP ＝ %04X:%04X（線性 %05X）",
				m.CPU.Seg[cpu.CS], m.CPU.IP, a)
			break
		}
		if *mouseX >= 0 && m.Steps == moveAt {
			d.Mouse.X, d.Mouse.Y = uint16(*mouseX), uint16(*mouseY)
		}
		// 點擊：按下 → 按住 clickHold 道指令 → 放開。
		// **按住時間不能短**。遊戲輪詢 int 33h 的頻率很低，
		// 按下與放開之間隔太近會整個被跳過（`rich2/docs/playtest/001` §5.6：
		// DOSBox 那邊同一題要點三次才生效一次，改成按住 0.35 秒才穩）。
		if *clickX >= 0 {
			switch {
			case m.Steps == *clickAt:
				d.Mouse.X, d.Mouse.Y = uint16(*clickX), uint16(*clickY)
				d.Mouse.Buttons = 1
				d.Mouse.Press++
				pollsAtPress = len(d.Mouse.Polls)
				held = true
			case held && releaseNow(d, *clickPolls, *clickHold, pollsAtPress, m.Steps, *clickAt):
				d.Mouse.Buttons = 0
				d.Mouse.Release++
				held = false
			}
		}
		// 點擊腳本：**一次跑帶著整串輸入**才走得到深處的畫面。
		// 一次一個點擊代表每多走一步就要重跑一次，而遊戲跑到主選單
		// 就要六千萬道指令。
		for i, c := range clicks {
			switch {
			case m.Steps == c.step:
				d.Mouse.X, d.Mouse.Y = c.x, c.y
				d.Mouse.Buttons = c.btn
				d.Mouse.Press++
				pollsAtPress = len(d.Mouse.Polls)
				downIdx = i
			case downIdx == i && releaseNow(d, *clickPolls, *clickHold, pollsAtPress, m.Steps, c.step):
				d.Mouse.Buttons = 0
				d.Mouse.Release++
				downIdx = -1
			}
		}
		if len(keyPlan) > 0 {
			if txt, ok := keyPlan[m.Steps]; ok {
				d.Stdin = append(d.Stdin, []byte(strings.ReplaceAll(txt, "\\n", "\n"))...)
			}
		}
		if len(saves) > 0 {
			if path, ok := saves[m.Steps]; ok {
				if err := state.Save(path, m, d); err != nil {
					die(err)
				}
				fmt.Printf("第 %d 道指令的狀態存到 %s\n", m.Steps, path)
			}
		}
		if len(pokes) > 0 {
			if list, ok := pokes[m.Steps]; ok {
				for _, pk := range list {
					for i, v := range pk.vals {
						m.Write8(pk.addr+uint32(i), v)
					}
				}
			}
		}
		if len(shots) > 0 {
			if path, ok := shots[m.Steps]; ok {
				writeShot(m, path)
			}
		}
		if sweep != nil && m.Steps >= sweep.from &&
			(m.Steps-sweep.from)%sweep.every == 0 {
			k := int((m.Steps - sweep.from) / sweep.every)
			sum := fmt.Sprintf("%x", sha256.Sum256(m.Indexed()))[:12]
			if k > 0 {
				mark := ""
				if sum != lastSum {
					mark = "  ← 畫面變了"
				}
				fmt.Printf("掃描 #%d (%d,%d) → %s%s\n",
					k-1, sweep.pt(k-1).x, sweep.pt(k-1).y, sum, mark)
			}
			lastSum = sum
			if k < sweep.n() {
				p := sweep.pt(k)
				d.Mouse.X, d.Mouse.Y = p.x, p.y
				d.Mouse.Buttons = 1
				d.Mouse.Press++
			}
		}
		if sweep != nil && m.Steps >= sweep.from &&
			(m.Steps-sweep.from)%sweep.every == sweep.every/2 {
			d.Mouse.Buttons = 0
			d.Mouse.Release++
		}
		if ca != nil && m.Steps >= ca.from && m.Steps < ca.to &&
			m.CPU.Seg[cpu.CS] == ca.seg && m.CPU.IP == ca.off {
			ca.record(m)
		}
		if ipw != nil && m.Steps >= ipFrom && m.Steps < ipTo {
			ipw.push(m.CPU.Seg[cpu.CS], m.CPU.IP)
		}
		ring.push(m.CPU)
		if runErr = m.Step(); runErr != nil {
			break
		}
	}

	if ca != nil {
		ca.dump()
	}
	report(m, d, ring, runErr, *steps)
	if *watch != "" {
		const showN = 200
		fmt.Printf("\n監看 %s 的寫入（留下 %d 筆，前面丟掉 %d 筆，列最後 %d）：\n",
			*watch, len(writes), dropped, showN)
		if n := len(writes); n > showN {
			writes = writes[n-showN:]
		}
		for _, w := range writes {
			fmt.Printf("  #%-9d %05X: %02X→%02X  ip=%04X:%04X\n",
				w.step, w.addr, w.old, w.nw, w.cs, w.ip)
		}
	}
	if len(m.DSLoads) > 0 {
		fmt.Printf("\nDS 被設成 %04X 的時刻（%d 次，最多列 20）：\n",
			m.WatchDS, len(m.DSLoads))
		for i, c := range m.DSLoads {
			if i >= 20 {
				break
			}
			fmt.Printf("  #%-9d 在 %04X:%04X（BX=%04X）\n", c.Step, c.FromSeg, c.FromOff, c.ToOff)
		}
	}
	writeMemDump(m, *dumpMem)
	if *dumpScreen != "" {
		w, h := m.VideoSize()
		if err := os.WriteFile(*dumpScreen, m.Indexed(), 0o644); err != nil {
			fmt.Println("dump-screen 寫檔失敗:", err)
		} else {
			fmt.Printf("畫面 %d×%d 的色號寫到 %s\n", w, h, *dumpScreen)
		}
	}
	if *watchVideo {
		if vidN == 0 {
			fmt.Println("視訊記憶體：一次都沒寫過")
		} else {
			fmt.Printf("視訊記憶體：寫了 %d 次，範圍 0x%05X–0x%05X\n", vidN, vidLo, vidHi)
		}
	}
	if len(d.Calls) > 0 {
		type kv struct {
			c dos.Call
			n int
		}
		var list []kv
		for k, v := range d.Calls {
			list = append(list, kv{k, v})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
		fmt.Printf("\n服務呼叫（%d 種）\n", len(list))
		for i, e := range list {
			if i >= 25 {
				break
			}
			fmt.Printf("  int %02Xh AH=%02X  ×%d\n", e.c.Int, e.c.AH, e.n)
		}
	}
	if len(d.Missing) > 0 {
		fmt.Printf("找不到的檔（%d）：%v\n", len(d.Missing), d.Missing)
	}
	// CGA／EGA 的畫面在 B8000／A0000，`-dump-vram` 只看 mode 13h 的 A0000。
	// 這個遊戲跑在 mode 06h（CGA 640×200），所以另外報一行「那一塊有沒有東西」。
	{
		nz := 0
		for a := uint32(0xB8000); a < 0xB8000+0x8000; a++ {
			if m.Read8(a) != 0 {
				nz++
			}
		}
		fmt.Printf("B8000 非零 bytes %d / 32768\n", nz)
	}
	if *dumpCGA != "" {
		if err := writeCGA(*dumpCGA, m); err != nil {
			die(err)
		}
		fmt.Printf("寫出 %s（B8000 當 640×200 mode 06h）\n", *dumpCGA)
	}
	if *peek != "" {
		dumpPeek(m, *peek)
	}
	if *dumpVRAM != "" {
		if err := os.WriteFile(*dumpVRAM, m.Indexed(), 0o644); err != nil {
			die(err)
		}
		fmt.Printf("\n寫出 %s（320×200 色號）\n", *dumpVRAM)
		// 順手出一張 PNG。**色號陣列是驗收的依據，PNG 只是給人看的**
		// （MVP-B 比的是逐點色號，不是圖片檔）。
		png := strings.TrimSuffix(*dumpVRAM, ".bin") + ".png"
		if err := writePNG(png, m.Indexed(), m.Palette()); err != nil {
			die(err)
		}
		fmt.Printf("寫出 %s\n", png)
	}
	if *dumpPal != "" {
		pal := m.Palette()
		buf := make([]byte, 0, 768)
		for _, c := range pal {
			buf = append(buf, c[0], c[1], c[2])
		}
		if err := os.WriteFile(*dumpPal, buf, 0o644); err != nil {
			die(err)
		}
		fmt.Printf("寫出 %s（256×3 RGB）\n", *dumpPal)
	}
}

func report(m *machine.Machine, d *dos.DOS, ring *ring, runErr error, limit uint64) {
	fmt.Printf("執行 %d 道指令\n", m.Steps)
	switch {
	case runErr != nil:
		fmt.Printf("停止原因：%v\n", runErr)
	case d.Exited:
		fmt.Printf("停止原因：程式呼叫 int 21h AH=4Ch，離開碼 %d\n", d.ExitCode)
	case m.CPU.Halted:
		fmt.Println("停止原因：HLT")
	default:
		fmt.Printf("停止原因：跑滿 %d 道指令上限（程式還活著）\n", limit)
	}
	c := m.CPU
	fmt.Printf("CS:IP ＝ %04X:%04X  AX=%04X BX=%04X CX=%04X DX=%04X\n",
		c.Seg[cpu.CS], c.IP, c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX])
	fmt.Printf("DS=%04X ES=%04X SS:SP=%04X:%04X  視訊模式 %02Xh\n",
		c.Seg[cpu.DS], c.Seg[cpu.ES], c.Seg[cpu.SS], c.R[cpu.SP], m.VideoMode())

	// 主控台。**錯誤訊息走這條**，空的不代表沒事，代表沒說話。
	fmt.Printf("\n主控台（%d bytes）：\n", len(d.Console))
	if len(d.Console) == 0 {
		fmt.Println("  （空）")
	} else {
		for _, line := range strings.Split(strings.ReplaceAll(
			string(d.Console), "\r", "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	if m.TraceSegs {
		reportSegs(m)
	}
	fmt.Printf("\n開過的檔（%d）：%s\n", len(d.Opened), join(d.Opened))
	if len(d.Allocs) > 0 {
		fmt.Printf("\n記憶體配置（%d 次，最多列 20）：\n", len(d.Allocs))
		for i, a := range d.Allocs {
			if i >= 20 {
				break
			}
			st := "失敗"
			if a.OK {
				st = "成功"
			}
			fmt.Printf("  #%-9d AH=%02X 要 %5d 段 → %04X %s\n", a.Step, a.Fn, a.Want, a.Seg, st)
		}
	}
	if len(d.XMSMoves) > 0 {
		fmt.Printf("\nXMS move（%d 次，最多列 30）：\n", len(d.XMSMoves))
		for i, w := range d.XMSMoves {
			if i >= 30 {
				break
			}
			fmt.Printf("  #%-9d %6d bytes  handle %d:%08X → handle %d:%08X  bits=%d\n",
				w.Step, w.Len, w.SrcH, w.SrcOff, w.DstH, w.DstOff, w.Bits)
		}
	}
	if len(d.EMSOps) > 0 {
		fmt.Printf("\nEMS（%d 次，最多列 40）：\n", len(d.EMSOps))
		for i, o := range d.EMSOps {
			if i >= 40 {
				break
			}
			switch o.Fn {
			case 0x43:
				fmt.Printf("  #%-9d 配置 handle=%d %d 頁\n", o.Step, o.Handle, o.Pages)
			case 0x44:
				st := ""
				if o.Status != 0 {
					st = fmt.Sprintf(" 失敗 %02X", o.Status)
				}
				fmt.Printf("  #%-9d 映射 handle=%d 邏輯頁 %d → 實體頁 %d%s\n",
					o.Step, o.Handle, o.Logical, o.Phys, st)
			case 0x45:
				fmt.Printf("  #%-9d 釋放 handle=%d\n", o.Step, o.Handle)
			}
		}
	}
	if len(d.Reads) > 0 {
		// **前 15 筆 ＋ 後 15 筆**：只列前面的話，開機階段就把配額用光，
		// 而要查的通常是「最後讀了什麼」。
		fmt.Printf("\n讀檔（%d 次，列前 15 與後 15）：\n", len(d.Reads))
		for i, r := range d.Reads {
			if len(d.Reads) > 30 && i == 15 {
				fmt.Printf("  …中間 %d 筆略過…\n", len(d.Reads)-30)
			}
			if len(d.Reads) > 30 && i >= 15 && i < len(d.Reads)-15 {
				continue
			}
			fmt.Printf("  #%-9d %-14s handle=%04X → %04X:%04X 要 %d 得 %d（線性 %05X–%05X）\n",
				r.Step, r.Name, r.Handle, r.Seg, r.Off, r.Want, r.Got,
				uint32(r.Seg)*16+uint32(r.Off), uint32(r.Seg)*16+uint32(r.Off)+uint32(r.Got))
		}
	}
	if len(d.ExecLog) > 0 {
		fmt.Printf("\nEXEC 紀錄（%d）：\n", len(d.ExecLog))
		for _, e := range d.ExecLog {
			fmt.Printf("  %-14s PSP=%04X exit=%d TSR=%v keep=%04X\n",
				e.Base, e.PSP, e.Exit, e.TSR, e.Keep)
		}
	}
	if len(d.Missing) > 0 {
		fmt.Printf("找不到的檔（%d）：%s\n", len(d.Missing), join(d.Missing))
	}
	if len(d.Wrote) > 0 {
		fmt.Printf("被擋下來的寫檔（%d）：", len(d.Wrote))
		for i, w := range d.Wrote {
			if i == 10 {
				fmt.Printf(" …另外 %d 次", len(d.Wrote)-10)
				break
			}
			fmt.Printf(" %s(%d)", w.Name, w.N)
		}
		fmt.Println()
	}

	// 沒實作的服務。**「宣告成功」本身也會說謊**——該填的緩衝區沒填就是垃圾。
	rep := d.UnimplementedReport()
	fmt.Printf("\n沒實作的服務（%d 種）：\n", len(rep))
	if len(rep) == 0 {
		fmt.Println("  （無）")
	}
	for i, r := range rep {
		if i == 25 {
			fmt.Printf("  …另外還有 %d 種\n", len(rep)-25)
			break
		}
		fmt.Printf("  %s\n", r)
	}

	// int 33h 的功能分佈。**「輪詢很多次」不代表遊戲在讀按鍵**——
	// 只叫 AH=3 與同時叫 AH=5／6 是兩種不同的輸入模型，點不到按鈕時
	// 要先分得出來是哪一種。
	if len(d.Mouse.Calls) > 0 {
		fns := make([]int, 0, len(d.Mouse.Calls))
		for f := range d.Mouse.Calls {
			fns = append(fns, int(f))
		}
		sort.Ints(fns)
		fmt.Printf("\nint 33h 功能：")
		for _, f := range fns {
			fmt.Printf(" AX=%04X×%d", f, d.Mouse.Calls[uint16(f)])
		}
		fmt.Println()
	}
	// 輪詢的時間分佈。**點擊要落在遊戲真的在輪詢的視窗裡**——
	// 它畫面重畫時可以兩千萬道指令一次都不問滑鼠，點在那段等於沒點，
	// 而畫面看起來就只是「沒反應」。
	if n := len(d.Mouse.Polls); n > 0 {
		const bucket = 10_000_000
		hist := map[uint64]int{}
		for _, p := range d.Mouse.Polls {
			hist[p.Step/bucket]++
		}
		ks := make([]int, 0, len(hist))
		for k := range hist {
			ks = append(ks, int(k))
		}
		sort.Ints(ks)
		fmt.Printf("\n輪詢分佈（每千萬道）：")
		for _, k := range ks {
			fmt.Printf(" %dM:%d", k*10, hist[uint64(k)])
		}
		fmt.Println()
	}
	if sizes := d.EMBSizes(); len(sizes) > 0 {
		fmt.Printf("\nXMS EMB：")
		for _, kv := range sizes {
			fmt.Printf(" handle %d ＝ %d bytes", kv[0], kv[1])
		}
		fmt.Println()
	}
	fmt.Printf("\n滑鼠輪詢 %d 次", len(d.Mouse.Polls))
	if n := len(d.Mouse.Polls); n > 0 {
		f, l := d.Mouse.Polls[0], d.Mouse.Polls[n-1]
		fmt.Printf("（第一次 #%d 回報 (%d,%d)；最後一次 #%d 回報 (%d,%d)）",
			f.Step, f.X, f.Y, l.Step, l.X, l.Y)
	}
	fmt.Println()

	if n := len(d.Mouse.Sets); n > 0 {
		l := d.Mouse.Sets[n-1]
		fmt.Printf("程式自己設游標位置 %d 次（最後一次 #%d 設成 (%d,%d)）\n",
			n, l.Step, l.X, l.Y)
	}

	// 埠寫入：mode 13h 之後 DAC（3C8h/3C9h）與序列器會有動作。
	ports := make([]int, 0, len(m.Ports))
	for p := range m.Ports {
		ports = append(ports, int(p))
	}
	sort.Ints(ports)
	fmt.Printf("寫過的 I/O 埠（%d）：", len(ports))
	for i, p := range ports {
		if i == 20 {
			fmt.Printf(" …另外 %d 個", len(ports)-20)
			break
		}
		fmt.Printf(" %03X", p)
	}
	fmt.Println()

	// 視訊記憶體：非零的點數。全 0 表示還沒畫任何東西。
	nz := 0
	for _, v := range m.Indexed() {
		if v != 0 {
			nz++
		}
	}
	fmt.Printf("A0000 非零像素 %d / %d\n", nz, machine.VideoWidth*machine.VideoHigh)

	if portLogFrom > 0 {
		fmt.Printf("\n#%d–#%d 的埠寫入：\n", portLogFrom, portLogTo)
		n := 0
		for _, w := range m.PortLog {
			if w.Step < portLogFrom || (portLogTo > 0 && w.Step > portLogTo) {
				continue
			}
			// PIC 的 EOI 與計時器每個 tick 都寫，會把顯示卡的設定淹掉。
			if w.Port < 0x3C0 || w.Port > 0x3DF {
				continue
			}
			if n++; n > 400 {
				fmt.Println("  …（超過 400 筆，只列前 400）")
				break
			}
			fmt.Printf("  #%-10d %03X ← %02X\n", w.Step, w.Port, w.Val)
		}
	}
	if len(m.CPU.DivErrors) > 0 {
		fmt.Printf("\n除以零 %d 次（位址是下一道指令）：", len(m.CPU.DivErrors))
		for _, e := range m.CPU.DivErrors {
			fmt.Printf(" %04X:%04X", e.CS, e.IP)
		}
		fmt.Println()
	}
	if len(m.VGATrace) > 0 {
		fmt.Printf("\n第 %d–%d 列的前 %d 筆 planar 寫入：\n", m.VGATraceRow0, m.VGATraceRow1, len(m.VGATrace))
		for _, t := range m.VGATrace {
			fmt.Printf("  #%-10d %04X:%04X 列%3d off=%04X val=%02X  mode=%d map=%02X bit=%02X sr=%02X esr=%02X rot=%02X latch=%02X%02X%02X%02X\n",
				t.Step, t.CS, t.IP, t.Row, t.Off, t.Val, t.Mode, t.MapMask, t.BitMask,
				t.SetReset, t.EnableSR, t.Rotate, t.Latch[0], t.Latch[1], t.Latch[2], t.Latch[3])
		}
	}
	if m.RowWritesFrom > 0 {
		h, w := m.VideoSize()
		_ = h
		fmt.Printf("\nplanar 每列寫入量（自 #%d，每 8 列一格）：\n", m.RowWritesFrom)
		_, rows := m.VideoSize()
		for r := 0; r < rows; r += 8 {
			sum := uint64(0)
			for i := r; i < r+8 && i < rows; i++ {
				sum += m.VideoRowWrites[i]
			}
			fmt.Printf("  %3d %d\n", r, sum)
		}
		_ = w
	}

	if traceFilePath != "" {
		if err := ring.writeFile(traceFilePath); err != nil {
			fmt.Fprintln(os.Stderr, "寫軌跡失敗：", err)
		} else {
			fmt.Printf("\n軌跡已寫到 %s（%d 道）\n", traceFilePath, min(ring.n, ring.size))
		}
	} else {
		ring.dump()
	}
}

// sweepSpec 是掃描點擊的設定。
type sweepGrid struct {
	from, every    uint64
	x0, y0, x1, y1 uint16
	step           uint16
}

func (g *sweepGrid) cols() int { return int((g.x1-g.x0)/g.step) + 1 }
func (g *sweepGrid) rows() int { return int((g.y1-g.y0)/g.step) + 1 }
func (g *sweepGrid) n() int    { return g.cols() * g.rows() }
func (g *sweepGrid) pt(k int) struct{ x, y uint16 } {
	return struct{ x, y uint16 }{
		x: g.x0 + uint16(k%g.cols())*g.step,
		y: g.y0 + uint16(k/g.cols())*g.step,
	}
}

// parseSweep 讀 `起始步數:每點步數:x0:y0:x1:y1:格距`。
func parseSweep(spec string) (*sweepGrid, error) {
	if spec == "" {
		return nil, nil
	}
	f := strings.Split(spec, ":")
	if len(f) != 7 {
		return nil, fmt.Errorf("掃描設定 %q 要七個欄位：起始:每點:x0:y0:x1:y1:格距", spec)
	}
	v := make([]uint64, 7)
	for i, x := range f {
		n, err := strconv.ParseUint(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("掃描設定 %q 第 %d 欄不是數字：%w", spec, i+1, err)
		}
		v[i] = n
	}
	g := &sweepGrid{from: v[0], every: v[1],
		x0: uint16(v[2]), y0: uint16(v[3]), x1: uint16(v[4]), y1: uint16(v[5]),
		step: uint16(v[6])}
	if g.every == 0 || g.step == 0 || g.x1 < g.x0 || g.y1 < g.y0 {
		return nil, fmt.Errorf("掃描設定 %q 的範圍或間隔不合理", spec)
	}
	return g, nil
}

// parseShots 讀 `步數:路徑,步數:路徑`。
// poke 是「跑到第 step 步就把 addr 起的位元組換成 vals」。
//
// 對拍要的局面直接設進去，不要靠遊戲內的隨機或一路點進去湊——
// 那兩者都不決定性，而且慢。
type poke struct {
	addr uint32
	vals []uint8
}

// parsePokes 讀 `步數:位址=值:值:…` 這種腳本（位址與值都是十六進位）。
func parsePokes(spec string) (map[uint64][]poke, error) {
	if spec == "" {
		return nil, nil
	}
	out := map[uint64][]poke{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		head, tail, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("poke %q 格式不對，要 步數:位址=值", part)
		}
		n, err := strconv.ParseUint(head, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("poke %q 的步數不是數字：%w", part, err)
		}
		as, vs, ok := strings.Cut(tail, "=")
		if !ok {
			return nil, fmt.Errorf("poke %q 少了 =", part)
		}
		addr, err := strconv.ParseUint(strings.TrimSpace(as), 16, 32)
		if err != nil {
			return nil, fmt.Errorf("poke %q 的位址不是十六進位：%w", part, err)
		}
		var vals []uint8
		for _, v := range strings.Split(vs, ":") {
			b, err := strconv.ParseUint(strings.TrimSpace(v), 16, 8)
			if err != nil {
				return nil, fmt.Errorf("poke %q 的值不是十六進位位元組：%w", part, err)
			}
			vals = append(vals, uint8(b))
		}
		out[n] = append(out[n], poke{uint32(addr), vals})
	}
	return out, nil
}

func parseShots(spec string) (map[uint64]string, error) {
	if spec == "" {
		return nil, nil
	}
	out := map[uint64]string{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.Index(part, ":")
		if i < 0 {
			return nil, fmt.Errorf("畫面腳本 %q 格式不對，要 步數:路徑", part)
		}
		n, err := strconv.ParseUint(part[:i], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("畫面腳本 %q 的步數不是數字：%w", part, err)
		}
		out[n] = part[i+1:]
	}
	return out, nil
}

// writeShot 把當下的畫面色號與色盤各存一份。
func writeShot(m *machine.Machine, path string) {
	if err := os.WriteFile(path, m.Indexed(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "存畫面失敗：", err)
		return
	}
	pal := m.Palette()
	flat := make([]byte, 0, len(pal)*3)
	for _, c := range pal {
		flat = append(flat, c[0], c[1], c[2])
	}
	if err := os.WriteFile(strings.TrimSuffix(path, filepath.Ext(path))+".pal", flat, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "存色盤失敗：", err)
	}
	w, h := m.VideoSize()
	fmt.Printf("#%d 畫面 %d×%d → %s\n", m.Steps, w, h, path)
}

// click 是點擊腳本裡的一次點擊。
type click struct {
	step uint64
	x, y uint16
	btn  uint16
}

// parseClicks 讀 `步數:X:Y,步數:X:Y` 這種腳本。
func parseClicks(spec string) ([]click, error) {
	if spec == "" {
		return nil, nil
	}
	var out []click
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := strings.Split(part, ":")
		if len(f) != 3 && len(f) != 4 {
			return nil, fmt.Errorf("點擊腳本 %q 格式不對，要 步數:X:Y[:鍵]", part)
		}
		c := click{btn: 1}
		n, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("點擊腳本 %q 的步數不是數字：%w", part, err)
		}
		c.step = n
		for i, dst := range []*uint16{&c.x, &c.y} {
			v, err := strconv.ParseUint(f[i+1], 10, 16)
			if err != nil {
				return nil, fmt.Errorf("點擊腳本 %q 的座標不是數字：%w", part, err)
			}
			*dst = uint16(v)
		}
		if len(f) == 4 {
			v, err := strconv.ParseUint(f[3], 10, 16)
			if err != nil {
				return nil, fmt.Errorf("點擊腳本 %q 的鍵不是數字：%w", part, err)
			}
			c.btn = uint16(v)
		}
		out = append(out, c)
	}
	return out, nil
}

func join(s []string) string {
	if len(s) == 0 {
		return "（無）"
	}
	if len(s) > 30 {
		return strings.Join(s[:30], " ") + fmt.Sprintf(" …另外 %d 個", len(s)-30)
	}
	return strings.Join(s, " ")
}

// ring 是最後 N 道指令的環狀緩衝。
//
// 停在錯誤指令上時，**只看 CS:IP 分不出「跳到垃圾」與「解錯一道指令」**——
// 前者的軌跡會有一個突兀的遠跳，後者是一路連續走過去的。
type ring struct {
	buf  []trace
	n    uint64
	size uint64
}

type trace struct {
	cs, ip                         uint16
	ax, bx, cx, dx, si, di, bp, sp uint16
	ds, es, ss                     uint16
}

func newRing(size uint64) *ring {
	if size == 0 {
		return &ring{}
	}
	return &ring{buf: make([]trace, size), size: size}
}

func (r *ring) push(c *cpu.CPU) {
	if r.size == 0 {
		return
	}
	r.buf[r.n%r.size] = trace{
		cs: c.Seg[cpu.CS], ip: c.IP,
		ax: c.R[cpu.AX], bx: c.R[cpu.BX], cx: c.R[cpu.CX], dx: c.R[cpu.DX],
		si: c.R[cpu.SI], di: c.R[cpu.DI], bp: c.R[cpu.BP], sp: c.R[cpu.SP],
		ds: c.Seg[cpu.DS], es: c.Seg[cpu.ES], ss: c.Seg[cpu.SS],
	}
	r.n++
}

func (r *ring) dump() { r.write(os.Stdout) }

// writeFile 把軌跡寫到檔案。**長軌跡不要走 stdout**——幾十萬行印在
// 終端機上不能搜也不能比對，寫成檔案才分析得動。
func (r *ring) writeFile(path string) error {
	if r.size == 0 || r.n == 0 || path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	r.write(bw)
	return bw.Flush()
}

func (r *ring) write(out io.Writer) {
	if r.size == 0 || r.n == 0 {
		return
	}
	fmt.Fprintf(out, "\n最後 %d 道指令：\n", min(r.n, r.size))
	start := uint64(0)
	if r.n > r.size {
		start = r.n - r.size
	}
	for i := start; i < r.n; i++ {
		t := r.buf[i%r.size]
		fmt.Fprintf(out, "#%d %04X:%04X AX=%04X BX=%04X CX=%04X DX=%04X "+
			"SI=%04X DI=%04X BP=%04X SP=%04X DS=%04X ES=%04X SS=%04X\n",
			i, t.cs, t.ip, t.ax, t.bx, t.cx, t.dx,
			t.si, t.di, t.bp, t.sp, t.ds, t.es, t.ss)
	}
}

// IDAOffset 是執行期線性位址與 `RUN_full.EXE` 的 IDA 線性位址之差。
//
//	IDA 線性 ＝ 執行期線性 ＋ IDAOffset
//
// 由「執行期 3014:167F ＝ IDA 線性 406BF」定出（那個防拷等待迴圈）。
// 前提是映像載在 machine.LoadSeg。
//
// **rich2 的每一份筆記都用 IDA 線性位址**，沒有這個換算就對不回去。
const IDAOffset = 0xEF00

// DGROUPSeg 是 BASIC 的 DGROUP 在執行期的段。
//
// rich2 的筆記寫 `ds:XXXX`，它的 IDA 線性基底是 41E90（`rich2/CLAUDE.md` §4.1）；
// 減掉 IDAOffset 得 32F90 ＝ 段 32F9 偏移 0。**那正是執行期的 SS**——
// 編譯後 BASIC 的 DGROUP 與堆疊同段，是這一族的慣例。
const DGROUPSeg = 0x32F9

// dumpPeek 印出指定位址的內容。
func dumpPeek(m *machine.Machine, spec string) {
	fmt.Println("\n記憶體：")
	for _, item := range strings.Split(spec, ",") {
		f := strings.Split(strings.TrimSpace(item), ":")
		var addr uint32
		var n int
		var label string
		switch {
		case len(f) == 3 && f[0] == "ds":
			off, _ := strconv.ParseUint(f[1], 16, 16)
			n, _ = strconv.Atoi(f[2])
			addr = uint32(DGROUPSeg)*16 + uint32(off)
			label = fmt.Sprintf("ds:%s", strings.ToUpper(f[1]))
		case len(f) == 2:
			ida, _ := strconv.ParseUint(f[0], 16, 32)
			n, _ = strconv.Atoi(f[1])
			addr = uint32(ida) - IDAOffset
			label = fmt.Sprintf("IDA %s", strings.ToUpper(f[0]))
		default:
			fmt.Printf("  %s：格式看不懂\n", item)
			continue
		}
		buf := make([]string, 0, n)
		for i := 0; i < n; i++ {
			buf = append(buf, fmt.Sprintf("%02X", m.Read8(addr+uint32(i))))
		}
		fmt.Printf("  %-14s（執行期 %05X）%s\n", label, addr, strings.Join(buf, " "))
	}
}

// writePNG 把色號陣列配上調色盤存成 PNG。
func writePNG(path string, idx []uint8, pal [256][3]uint8) error {
	p := make(color.Palette, 256)
	for i := range pal {
		p[i] = color.RGBA{pal[i][0], pal[i][1], pal[i][2], 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, machine.VideoWidth, machine.VideoHigh), p)
	copy(img.Pix, idx)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "probe:", err)
	os.Exit(1)
}

// writeCGA 把 B8000 依 CGA mode 06h 的版面畫出來：
// 偶數列在 B8000、奇數列在 BA000，每列 80 bytes、每 byte 8 個像素（MSB 在左）。
func writeCGA(path string, m *machine.Machine) error {
	img := image.NewGray(image.Rect(0, 0, 640, 200))
	for y := 0; y < 200; y++ {
		base := uint32(0xB8000) + uint32(y&1)*0x2000 + uint32(y/2)*80
		for x := 0; x < 640; x++ {
			b := m.Read8(base + uint32(x/8))
			if b&(0x80>>uint(x%8)) != 0 {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// reportSegs 印出每個 CS 第一次被執行的時間與來源。
//
// 「執行流是什麼時候跑到不該去的地方」——看這張表比翻 IP 的 trace 快，
// 因為飛掉的那一跳通常早就滾出 ring buffer 了。
func reportSegs(m *machine.Machine) {
	order := make([]uint16, 0, len(m.SegFirst))
	for seg := range m.SegFirst {
		order = append(order, seg)
	}
	sort.Slice(order, func(i, j int) bool {
		return m.SegFirst[order[i]].Step < m.SegFirst[order[j]].Step
	})
	fmt.Printf("\n段轉移（保留最近 %d 筆，%d 個相異 CS）：\n", len(m.SegLog), len(order))
	for _, seg := range order {
		c := m.SegFirst[seg]
		fmt.Printf("  CS=%04X 首見 #%d ← %04X:%04X → %04X:%04X\n",
			seg, c.Step, c.FromSeg, c.FromOff, c.ToSeg, c.ToOff)
	}

	// 序列本身：首見表說「第一次是誰跳過來的」，序列才看得出**它自己是
	// 怎麼被叫起來的**（例如 `push seg / push off / retf` 的間接遠跳，
	// 來源會是一個 RETF 的位址）。
	n := len(m.SegLog)
	if n > 400 {
		n = 400
	}
	fmt.Printf("\n最後 %d 筆段轉移：\n", n)
	for _, c := range m.SegLog[len(m.SegLog)-n:] {
		fmt.Printf("  #%-9d %04X:%04X → %04X:%04X\n", c.Step, c.FromSeg, c.FromOff, c.ToSeg, c.ToOff)
	}
}

// writeMemDump 把一段線性記憶體寫成檔案（`-dump-mem <lo>-<hi>:<路徑>`）。
//
// 解壓過的資料只存在記憶體裡——檔案是壓縮的、格式還沒解，而程式跑完就
// 沒了。要對照「壓縮前後」就得把這一份留下來。
func writeMemDump(m *machine.Machine, spec string) {
	if spec == "" {
		return
	}
	for _, one := range strings.Split(spec, ",") {
		writeOneMemDump(m, one)
	}
}

func writeOneMemDump(m *machine.Machine, spec string) {
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		fmt.Println("dump-mem 格式是 <lo>-<hi>:<路徑>")
		return
	}
	var lo, hi uint32
	if _, err := fmt.Sscanf(spec[:i], "%x-%x", &lo, &hi); err != nil {
		fmt.Println("dump-mem 位址解不開:", err)
		return
	}
	if hi > uint32(len(m.Mem)) {
		hi = uint32(len(m.Mem))
	}
	if err := os.WriteFile(spec[i+1:], m.Mem[lo:hi], 0o644); err != nil {
		fmt.Println("dump-mem 寫檔失敗:", err)
		return
	}
	fmt.Printf("\n記憶體 %05X–%05X 寫到 %s\n", lo, hi, spec[i+1:])
}

// ipWriter 把每一道指令的 CS:IP 寫成二進位。
//
// **只記 CS:IP，不記暫存器**：要回答的問題是「兩次執行的控制流在哪裡
// 第一次分岔」，而分岔一定表現在 IP 上。三百萬道指令的完整暫存器軌跡
// 是三百 MB 的文字，同一段 CS:IP 只有 12 MB，`cmp -l` 一秒就給出答案；
// 拿到位置之後再用 -trace 對那一小段抓暫存器。
type ipWriter struct {
	f  *os.File
	bw *bufio.Writer
	b  [4]byte
}

func openIPLog(spec string) (*ipWriter, uint64, uint64, error) {
	if spec == "" {
		return nil, 0, 0, nil
	}
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) != 3 {
		return nil, 0, 0, fmt.Errorf("-ip-log 要寫成 起:迄:路徑，收到 %q", spec)
	}
	from, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("-ip-log 的起點：%w", err)
	}
	to, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("-ip-log 的終點：%w", err)
	}
	if to <= from {
		return nil, 0, 0, fmt.Errorf("-ip-log 的終點要大於起點")
	}
	f, err := os.Create(parts[2])
	if err != nil {
		return nil, 0, 0, err
	}
	return &ipWriter{f: f, bw: bufio.NewWriterSize(f, 1<<20)}, from, to, nil
}

func (w *ipWriter) push(cs, ip uint16) {
	binary.LittleEndian.PutUint16(w.b[0:], cs)
	binary.LittleEndian.PutUint16(w.b[2:], ip)
	w.bw.Write(w.b[:])
}

func (w *ipWriter) close() {
	w.bw.Flush()
	w.f.Close()
}

// callArgLog 記錄某個進入點每一次被呼叫時堆疊上的參數。
//
// 用途是**把「程式在比什麼」看出來**：源平合戰的點擊判定是一支
// `pointInRect(x, y, x0, y0, x1, y1)` 遠呼叫，光看控制流只知道「沒中」，
// 把六個參數印出來才知道畫面上到底註冊了哪些矩形。
type callArgLog struct {
	seg, off uint16
	n        int
	from, to uint64
	// viaBP：位址落在 prologue 之後（`bp` 已經架好），參數要從 `SS:BP+6`
	// 取，不是 `SS:SP+4`。反組譯給的位址多半是函式中間那幾行
	// （`mov bx,[bp+06]`），比猜進入點在哪可靠。
	viaBP bool
	// regs：連暫存器一起印。繪圖驅動有些參數走暫存器不走堆疊
	// （`yuan/docs/re/002`），只印堆疊會漏掉來源位址。
	regs bool
	rows []callArgRow
}

type callArgRow struct {
	step   uint64
	retSeg uint16
	retOff uint16
	w      []uint16
	regs   [8]uint16 // AX BX CX DX SI DI ES BP
}

// parseSaveState 解 `步數:路徑[,步數:路徑…]`。
func parseSaveState(spec string) (map[uint64]string, error) {
	if spec == "" {
		return nil, nil
	}
	out := map[uint64]string{}
	for _, one := range strings.Split(spec, ",") {
		i := strings.Index(one, ":")
		if i < 0 {
			return nil, fmt.Errorf("-save-state 要寫成 步數:路徑，收到 %q", one)
		}
		n, err := strconv.ParseUint(strings.TrimSpace(one[:i]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("-save-state 的步數不是數字：%w", err)
		}
		out[n] = one[i+1:]
	}
	return out, nil
}

func parseCallArgs(spec string, viaBP bool) (*callArgLog, error) {
	if spec == "" {
		return nil, nil
	}
	var seg, off uint16
	var n int
	var from, to uint64
	if _, err := fmt.Sscanf(spec, "%x:%x:%d:%d:%d", &seg, &off, &n, &from, &to); err != nil {
		return nil, fmt.Errorf("-call-args 要寫成 CS:IP:字數:起:迄，收到 %q：%w", spec, err)
	}
	if n <= 0 || n > 32 {
		return nil, fmt.Errorf("-call-args 的字數要在 1–32，收到 %d", n)
	}
	return &callArgLog{seg: seg, off: off, n: n, from: from, to: to, viaBP: viaBP}, nil
}

func snapRegs(m *machine.Machine) [8]uint16 {
	return [8]uint16{
		m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX],
		m.CPU.R[cpu.SI], m.CPU.R[cpu.DI], m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BP],
	}
}

func (c *callArgLog) record(m *machine.Machine) {
	if c.viaBP {
		bp := uint32(m.CPU.Seg[cpu.SS])*16 + uint32(m.CPU.R[cpu.BP])
		w := make([]uint16, c.n)
		for i := range w {
			w[i] = m.Read16(bp + 6 + uint32(i*2))
		}
		c.rows = append(c.rows, callArgRow{
			step: m.Steps, retOff: m.Read16(bp + 2), retSeg: m.Read16(bp + 4), w: w, regs: snapRegs(m)})
		return
	}
	base := uint32(m.CPU.Seg[cpu.SS])*16 + uint32(m.CPU.R[cpu.SP])
	// 遠呼叫的返回位址佔前四個位元組，參數從第五個開始；
	// 返回位址本身就是**呼叫端是誰**，同一支被叫上千次時只有它分得出來。
	off := m.Read16(base)
	seg := m.Read16(base + 2)
	w := make([]uint16, c.n)
	for i := range w {
		w[i] = m.Read16(base + 4 + uint32(i*2))
	}
	c.rows = append(c.rows, callArgRow{
		step: m.Steps, retSeg: seg, retOff: off, w: w, regs: snapRegs(m)})
}

func (c *callArgLog) dump() {
	fmt.Printf("\n%04X:%04X 被呼叫 %d 次（步數 %d–%d）：\n", c.seg, c.off, len(c.rows), c.from, c.to)
	for _, r := range c.rows {
		s := make([]string, len(r.w))
		for i, v := range r.w {
			s[i] = fmt.Sprintf("%d", int16(v))
		}
		reg := ""
		if c.regs {
			reg = fmt.Sprintf("  | AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X ES=%04X BP=%04X",
				r.regs[0], r.regs[1], r.regs[2], r.regs[3], r.regs[4], r.regs[5], r.regs[6], r.regs[7])
		}
		fmt.Printf("  #%d  由 %04X:%04X  %s%s\n", r.step, r.retSeg, r.retOff, strings.Join(s, " "), reg)
	}
}

// releaseNow 決定按住的滑鼠鍵什麼時候放開。
//
// 兩種模式：`-click-polls N` 是「等遊戲真的讀了 N 次滑鼠」，
// `-click-hold` 是固定指令數。前者才是可移植的——同一支遊戲不同畫面的
// 輪詢頻率可以差三個數量級（源平合戰的磁片提示每千萬道問 12 次，
// 讀檔選單每千萬道問一千次），固定指令數在一邊漏掉、在另一邊按成連點。
func releaseNow(d *dos.DOS, clickPolls int, clickHold uint64,
	pollsAtPress int, step, pressStep uint64) bool {
	if clickPolls > 0 {
		return len(d.Mouse.Polls)-pollsAtPress >= clickPolls
	}
	return step == pressStep+clickHold
}
