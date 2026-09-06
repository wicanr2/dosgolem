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
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	exe := flag.String("exe", "", "要跑的執行檔（必填；MZ 或 .COM，看檔頭 magic 自動判斷）")
	root := flag.String("root", ".", "原版素材目錄")
	cmdline := flag.String("cmdline", "", "命令列尾巴，寫進最外層程式的 `PSP+80h`。"+
		"靠參數決定要做什麼的程式（例如 ENDING.EXE 要演哪一個結局）沒有它就直接結束")
	steps := flag.Uint64("steps", 20_000_000, "最多執行幾道指令")
	trace := flag.Uint64("trace", 0, "最後幾道指令的軌跡（0 ＝ 不記）")
	dumpVRAM := flag.String("dump-vram", "", "把 A0000 的 320×200 色號陣列寫到這個檔")
	dumpPal := flag.String("dump-palette", "", "把 256×3 的 RGB 調色盤寫到這個檔")
	peek := flag.String("peek", "", "跑完之後印出這些位址的內容，逗號分隔。格式："+
		"<段>:<偏移>:<長度>（軌跡印的形式）、lin:<執行期線性>:<長度>、"+
		"ds:<偏移>:<長度>，或 <IDA 線性位址>:<長度>（rich2 專用，會減 IDAOffset）")
	find := flag.String("find", "", "跑完之後在 1 MB 記憶體裡找這些 hex bytes，"+
		"逗號分隔可以一次找多組，印出所有命中的線性位址"+
		"（除錯壞指標、或驗證位元組簽章還定位得到）")
	mouseX := flag.Int("mouse-x", -1, "滑鼠要移到的像素 X（−1 ＝ 不動）")
	mouseY := flag.Int("mouse-y", -1, "滑鼠要移到的像素 Y")
	mouseAt := flag.Uint64("mouse-at", 0, "第幾道指令時移動滑鼠（0 ＝ steps 的一半）")
	clickX := flag.Int("click-x", -1, "點擊的像素 X（−1 ＝ 不點）")
	clickY := flag.Int("click-y", -1, "點擊的像素 Y")
	clickAt := flag.Uint64("click-at", 0, "第幾道指令時按下")
	clickHold := flag.Uint64("click-hold", 2_000_000, "按住幾道指令")
	rclicks := flag.String("rclicks", "", "多次**右鍵**點擊，格式與 -clicks 相同"+
		"（`docs/spec/016`）。DOS 遊戲拿右鍵當取消／關閉視窗，"+
		"少了它被蓋住的視窗一個都點不到，而症狀是畫面完全不動")
	clicks := flag.String("clicks", "", "多次點擊，格式 `x,y,at[,hold]`，用分號分隔。"+
		"與 -click-x 併用時兩邊都會生效")
	watchVideo := flag.Bool("watch-video", false,
		"統計寫進 A0000–BFFFF 的位址範圍（回答「它到底畫在哪裡」）")
	watch := flag.String("watch", "", "監看記憶體寫入，格式 <線性hex>-<線性hex>；"+
		"每次寫入印出步數與 CS:IP（除錯「誰把向量改掉了」）")
	logCalls := flag.Bool("log-calls", false, "統計每一種 (中斷, AH) 呼叫幾次")
	tick := flag.Uint64("tick", 0, "每幾道指令送一次計時器中斷（0 ＝ 用預設）")
	keys := flag.String("keys", "", "先排進 BIOS 鍵盤緩衝區的按鍵（`\\n` 是 Enter）；"+
		"程式自己裝 int 09h 的話這條沒用，要用 -press")
	press := flag.String("press", "", "用 IRQ1 送的按鍵，逗號分隔。"+
		"可用名稱：up down left right enter esc space，或單一字元／16 進位掃描碼")
	pressAt := flag.Uint64("press-at", 0, "第幾道指令開始送鍵（0 ＝ steps 的八成）")
	pressEvery := flag.Uint64("press-every", 500_000, "每幾道指令送一個掃描碼")
	dumpCGA := flag.String("dump-cga", "", "把 B8000 當 CGA mode 06h（640×200 雙 bank）畫成 PNG")
	dumpLinear := flag.String("dump-linear", "", "把 A0000 的 64 KB raw bytes 寫到這個檔"+
		"（planar 模式的原始內容，**不是**解碼後的畫面——spec 008 §5）")
	watchScreen := flag.Uint64("watch-screen", 0,
		"每 N 道指令算一次畫面雜湊，畫面變了就印一行。"+
			"回答「這個輸入到底有沒有效」——比對著幾張傾印猜快得多")
	screenDelta := flag.Int("screen-delta", 5000,
		"畫面變化超過幾個像素才算一次「換畫面」（配 -watch-screen）")
	regsFrom := flag.Uint64("regs-from", 0,
		"-regs-at 只從這個步數之後開始記。開場與主選單會把前 20 筆佔滿，"+
			"要看後面某一次呼叫就得跳過前面")
	keysAt := flag.String("keys-at", "",
		"在指定步數送一個鍵：`<步數>:<鍵>`，分號分隔。與 -press 的差別是"+
			"**每個鍵各自指定時機**——探索「哪一個鍵讓畫面動了」要用這個，"+
			"固定間隔會讓你分不出是哪一次生效")
	dumpAt := flag.String("dump-at", "",
		"跑到指定步數就傾印一張畫面：`<步數>:<檔名>`，分號分隔可以給很多張。"+
			"探索「點下去之後跑到哪個畫面」用——一次跑就看得到中間的每一格，"+
			"不必為了每張畫面重跑一次")
	dumpMem := flag.String("dump-mem", "",
		"把一段記憶體原封不動寫成檔案：`<位址>:<長度>:<檔名>`（位址寫法同 -peek）。"+
			"資料被程式改過之後長什麼樣，只有這樣看得到")
	dumpEGA := flag.String("dump-ega", "", "把 planar VRAM 解成 PNG 寫到這個檔"+
		"（依 BDA 目前模式選尺寸：12h ＝ 640×480、10h ＝ 640×350；spec 009）")
	adlib := flag.Bool("adlib", false, "讓 AdLib（OPL2，埠 388h）偵測存在"+
		"（預設不存在，開機快；音樂路徑要它才會跑）")
	poke := flag.String("poke", "",
		"在指定步數直接改記憶體：`<位址>@<步數>=<hex bytes>`，分號分隔。"+
			"位址寫法同 -peek。**對拍要固定的是狀態，不是運氣**——"+
			"要某個局面就直接把它寫進去，不要靠亂數重跑")
	regsAt := flag.String("regs-at", "",
		"執行到這些 `<seg>:<off>` 時記下暫存器（逗號分隔，各印前 8 次）。"+
			"用來回答「這支繪圖常式的來源指標指到哪」——"+
			"監看寫入只看得到目的地，看不到它從哪裡搬")
	regsMax := flag.Int("regs-max", 20,
		"每個 -regs-at 位址最多記幾次。逐格處理的迴圈跑幾百次，"+
			"預設的 20 次只看得到第一個物件")
	vramSites := flag.Bool("vram-sites", false,
		"統計「誰在寫視訊記憶體」，印出前 20 名 CS:IP（找繪圖常式）")
	vramAt := flag.String("vram-at", "",
		"把 -vram-sites 限定在這個 VRAM 位移（16 進位）。"+
			"盯單一像素用——「這一點是誰畫的」比「誰畫得最多」更能定位")
	flag.Parse()

	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	img, err := os.ReadFile(*exe)
	if err != nil {
		die(err)
	}

	m := machine.New()
	if *adlib {
		m.SetAdLib(true)
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
	if *tick > 0 {
		m.IRQ0Every = *tick
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
	if *watch != "" {
		f := strings.SplitN(*watch, "-", 2)
		lo, err1 := strconv.ParseUint(f[0], 16, 32)
		var hi uint64
		var err2 error
		if len(f) == 2 {
			hi, err2 = strconv.ParseUint(f[1], 16, 32)
		} else {
			hi = lo
		}
		if err1 != nil || err2 != nil {
			die(fmt.Errorf("-watch 格式看不懂：%s", *watch))
		}
		m.WatchWrites(uint32(lo), uint32(hi), func(a uint32, old, nv uint8) {
			fmt.Printf("[watch] #%d %05X: %02X → %02X  ← %04X:%04X\n",
				m.Steps, a, old, nv, m.CPU.Seg[cpu.CS], m.CPU.IP)
		})
	}
	if *cmdline != "" {
		tail := *cmdline
		if len(tail) > 126 {
			tail = tail[:126]
		}
		base := uint32(machine.PSPSeg)*16 + 0x80
		m.Write8(base, uint8(len(tail)))
		for i := 0; i < len(tail); i++ {
			m.Write8(base+1+uint32(i), tail[i])
		}
		m.Write8(base+1+uint32(len(tail)), 0x0D)
		fmt.Printf("命令列尾巴 %q 寫進 PSP+80h\n", tail)
	}
	d := dos.New(m, *root)
	if *logCalls {
		d.Calls = map[dos.Call]int{}
	}
	d.Install()
	if *press != "" {
		m.KeyEvery = *pressEvery
		at := *pressAt
		if at == 0 {
			at = *steps * 8 / 10
		}
		m.SetNextKey(at)
		for _, k := range strings.Split(*press, ",") {
			sc, ok := scanOf(strings.TrimSpace(k))
			if !ok {
				die(fmt.Errorf("看不懂的按鍵 %q", k))
			}
			m.QueueKey(sc)
		}
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

	// moveLead 是「移動事件比按下早多少道指令」。要留給程式把新位置處理完，
// 否則按下時它手上還是舊座標。
const moveLead = 200_000

type clickEv struct {
		x, y     int
		at, hold uint64
		right    bool
	}
	var evs []clickEv
	parseClicks := func(spec, flagName string, right bool) {
		for _, one := range strings.Split(spec, ";") {
			if one = strings.TrimSpace(one); one == "" {
				continue
			}
			f := strings.Split(one, ",")
			if len(f) < 3 {
				die(fmt.Errorf("%s 的 %q 少了欄位，要 x,y,at[,hold]", flagName, one))
			}
			var e clickEv
			e.right = right
			e.x, _ = strconv.Atoi(strings.TrimSpace(f[0]))
			e.y, _ = strconv.Atoi(strings.TrimSpace(f[1]))
			e.at, _ = strconv.ParseUint(strings.TrimSpace(f[2]), 10, 64)
			e.hold = *clickHold
			if len(f) > 3 {
				e.hold, _ = strconv.ParseUint(strings.TrimSpace(f[3]), 10, 64)
			}
			evs = append(evs, e)
		}
	}
	parseClicks(*clicks, "-clicks", false)
	parseClicks(*rclicks, "-rclicks", true)

	if *vramSites || *vramAt != "" {
		m.VRAMSites = map[uint32]uint64{}
		m.VRAMAt = -1
		if *vramAt != "" {
			v, err := strconv.ParseUint(strings.TrimPrefix(*vramAt, "0x"), 16, 32)
			if err != nil {
				die(fmt.Errorf("-vram-at 不是 16 進位：%w", err))
			}
			m.VRAMAt = int32(v)
		}
	}
	type regSite struct {
		seg, off uint16
	}
	regHits := map[regSite][]string{}
	regLast := map[regSite]uint32{}
	var regWatch []regSite
	for _, item := range strings.Split(*regsAt, ",") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		f := strings.Split(item, ":")
		if len(f) != 2 {
			die(fmt.Errorf("-regs-at 的 %q 要寫成 <seg>:<off>", item))
		}
		sg, err1 := strconv.ParseUint(f[0], 16, 16)
		of, err2 := strconv.ParseUint(f[1], 16, 16)
		if err1 != nil || err2 != nil {
			die(fmt.Errorf("-regs-at 的 %q 不是 16 進位的 <seg>:<off>", item))
		}
		regWatch = append(regWatch, regSite{uint16(sg), uint16(of)})
	}

	type keyEv struct {
		at   uint64
		scan uint8
	}
	var keyEvs []keyEv
	for _, item := range strings.Split(*keysAt, ";") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		i := strings.Index(item, ":")
		if i < 0 {
			die(fmt.Errorf("-keys-at 要寫成 <步數>:<鍵>：%q", item))
		}
		at, err := strconv.ParseUint(strings.TrimSpace(item[:i]), 10, 64)
		if err != nil {
			die(fmt.Errorf("-keys-at 的步數看不懂：%q", item))
		}
		sc, ok := scanOf(strings.TrimSpace(item[i+1:]))
		if !ok {
			die(fmt.Errorf("看不懂的按鍵 %q", item[i+1:]))
		}
		keyEvs = append(keyEvs, keyEv{at: at, scan: sc})
	}

	type shot struct {
		at   uint64
		path string
	}
	var shots []shot
	for _, item := range strings.Split(*dumpAt, ";") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		i := strings.Index(item, ":")
		if i < 0 {
			die(fmt.Errorf("-dump-at 要寫成 <步數>:<檔名>：%q", item))
		}
		at, err := strconv.ParseUint(strings.TrimSpace(item[:i]), 10, 64)
		if err != nil {
			die(fmt.Errorf("-dump-at 的步數看不懂：%q", item))
		}
		shots = append(shots, shot{at: at, path: item[i+1:]})
	}

	pokes, err := parsePokes(*poke)
	if err != nil {
		die(err)
	}
	var lastScreen []uint8
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
			// **改座標之後要送移動事件**，跟 -clicks 一樣。
			// 遊戲的游標是靠事件回呼畫的（`docs/spec/013`）：只改座標的話，
			// 舊位置那隻游標不會被擦掉，新位置也不會畫出來——
			// 畫面上留著一隻停在原地的游標，看起來像「滑鼠沒動」。
			d.Mouse.X, d.Mouse.Y = uint16(*mouseX), uint16(*mouseY)
			d.MouseEvent(dos.EvMove)
		}
		// 點擊：按下 → 按住 clickHold 道指令 → 放開。
		// **按住時間不能短**。遊戲輪詢 int 33h 的頻率很低，
		// 按下與放開之間隔太近會整個被跳過（`rich2/docs/playtest/001` §5.6：
		// DOSBox 那邊同一題要點三次才生效一次，改成按住 0.35 秒才穩）。
		if *clickX >= 0 {
			switch m.Steps {
			case *clickAt:
				d.Mouse.X, d.Mouse.Y = uint16(*clickX), uint16(*clickY)
				d.Mouse.Buttons = 1
				d.Mouse.Press[0]++
			case *clickAt + *clickHold:
				d.Mouse.Buttons = 0
				d.Mouse.Release[0]++
			}
		}
		for _, e := range evs {
			switch m.Steps {
			case e.at - moveLead:
				// **先送移動再送按下。** 只改座標不送移動事件的話，
				// 靠回呼追游標的程式手上還是舊位置，按下就落在別的地方——
				// 而畫面上什麼都不會發生，看起來像「點擊沒送到」。
				d.Mouse.X, d.Mouse.Y = uint16(e.x), uint16(e.y)
				d.MouseEvent(dos.EvMove)
			case e.at:
				btn, down := 0, uint16(dos.EvLeftDown)
				if e.right {
					btn, down = 1, dos.EvRightDown
				}
				d.Mouse.X, d.Mouse.Y = uint16(e.x), uint16(e.y)
				d.Mouse.Buttons |= uint16(1) << uint(btn)
				d.Mouse.Press[btn]++
				d.MouseEvent(down)
			case e.at + e.hold:
				btn, up := 0, uint16(dos.EvLeftUp)
				if e.right {
					btn, up = 1, dos.EvRightUp
				}
				d.Mouse.Buttons &^= uint16(1) << uint(btn)
				d.Mouse.Release[btn]++
				d.MouseEvent(up)
			}
		}
		for _, p := range pokes {
			if m.Steps == p.at {
				for i, b := range p.data {
					m.Write8(p.addr+uint32(i), b)
				}
				fmt.Printf("#%d 改記憶體 %s（%05X）%d bytes\n", m.Steps, p.label, p.addr, len(p.data))
			}
		}
		for _, w := range regWatch {
			if m.CPU.Seg[cpu.CS] == w.seg && m.CPU.IP == w.off && m.Steps >= *regsFrom {
				c := m.CPU
				// **只記來源基底換掉的那一次。** 同一塊圖的 16 或 20 列
				// 是連續的位移，全部印出來只會看到同一個東西 20 遍，
				// 而「換了一份來源」正是要找的訊號。
				cur := uint32(c.Seg[cpu.DS])<<16 | uint32(c.R[cpu.SI])
				prev, seen := regLast[w]
				regLast[w] = cur
				// 只跳過「同一份來源往下走一格」的那種重複（blit 迴圈）；
				// 位置完全相同的重複呼叫要記——那是不同的一次事件。
				if seen && cur > prev && cur-prev <= 16 {
					continue
				}
				if h := regHits[w]; len(h) < *regsMax {
					regHits[w] = append(h, fmt.Sprintf(
						"#%d AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X DS=%04X ES=%04X SS:SP=%04X:%04X",
						m.Steps, c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX],
						c.R[cpu.SI], c.R[cpu.DI], c.R[cpu.BP], c.Seg[cpu.DS], c.Seg[cpu.ES],
						c.Seg[cpu.SS], c.R[cpu.SP])+stackDump(m, c))
				}
			}
		}

		if *watchScreen > 0 && m.Steps%*watchScreen == 0 {
			// **報變了多少，不要只報變沒變。** 遊戲會一直重畫閃爍的游標
			// 與提示，每一次取樣都「變了」——那個訊號全是雜訊，
			// 分不出畫面有沒有真的換掉。
			cur := m.PlanarPixels(m.PixelWidth(), 480)
			if lastScreen != nil {
				n := 0
				for i := range cur {
					if cur[i] != lastScreen[i] {
						n++
					}
				}
				if n >= *screenDelta {
					fmt.Printf("[畫面] #%d 變了 %d 個像素\n", m.Steps, n)
				}
			}
			lastScreen = cur
		}
		for _, k := range keyEvs {
			if m.Steps == k.at {
				m.QueueKey(k.scan)
				m.SetNextKey(m.Steps + 1)
			}
		}
		for _, sh := range shots {
			if m.Steps == sh.at {
				if err := writeEGA(sh.path, m); err != nil {
					die(err)
				}
				fmt.Printf("#%d 傾印畫面 → %s\n", m.Steps, sh.path)
			}
		}
		ring.push(m.CPU)
		if runErr = m.Step(); runErr != nil {
			break
		}
	}

	report(m, d, ring, runErr, *steps)
	for _, w := range regWatch {
		h := regHits[regSite{w.seg, w.off}]
		fmt.Printf("\n%04X:%04X 執行時的暫存器（前 %d 次）：\n", w.seg, w.off, len(h))
		for _, line := range h {
			fmt.Println("  " + line)
		}
	}
	if len(m.VRAMSites) > 0 {
		type kv struct {
			a uint32
			n uint64
		}
		var list []kv
		for k, v := range m.VRAMSites {
			list = append(list, kv{k, v})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
		fmt.Printf("\n寫視訊記憶體的指令（%d 處，前 20 名）：\n", len(list))
		for i, e := range list {
			if i >= 20 {
				break
			}
			fmt.Printf("  %04X:%04X ×%d\n", e.a>>16, e.a&0xFFFF, e.n)
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
	if *dumpLinear != "" {
		raw := append([]byte(nil), m.Mem[0xA0000:0xB0000]...)
		if err := os.WriteFile(*dumpLinear, raw, 0o644); err != nil {
			die(err)
		}
		fmt.Printf("寫出 %s（A0000 raw 64 KB，planar 未解碼）\n", *dumpLinear)
	}
	if *dumpEGA != "" {
		if err := writeEGA(*dumpEGA, m); err != nil {
			die(err)
		}
	}
	if *peek != "" {
		dumpPeek(m, *peek)
	}
	if *dumpMem != "" {
		i := strings.LastIndex(*dumpMem, ":")
		if i < 0 {
			die(fmt.Errorf("-dump-mem 要寫成 <位址>:<長度>:<檔名>"))
		}
		spec, path := (*dumpMem)[:i], (*dumpMem)[i+1:]
		addr, n, label, ok := parseAddr(spec)
		if !ok || n <= 0 {
			die(fmt.Errorf("-dump-mem 的位址或長度看不懂：%q", spec))
		}
		buf := make([]byte, n)
		for k := range buf {
			buf[k] = m.Read8(addr + uint32(k))
		}
		if err := os.WriteFile(path, buf, 0o644); err != nil {
			die(err)
		}
		fmt.Printf("\n倒出 %s（%05X）%d bytes → %s\n", label, addr, n, path)
	}
	if *find != "" {
		// 逗號分隔多組樣式：一次跑完可以驗一整批位元組簽章。
		for _, one := range strings.Split(*find, ",") {
			if one = strings.TrimSpace(one); one != "" {
				dumpFind(m, d, one)
			}
		}
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
	if m.KeyIRQs > 0 || m.KeyEvery > 0 {
		fmt.Printf("鍵盤中斷送出 %d 次（int 09h 向量 %04X:%04X）\n",
			m.KeyIRQs, m.Read16(0x09*4+2), m.Read16(0x09*4))
	}
	fmt.Printf("planar write mode 使用次數：0=%d 1=%d 2=%d 3=%d\n",
		m.WriteModeUse[0], m.WriteModeUse[1], m.WriteModeUse[2], m.WriteModeUse[3])
	if len(m.ModeChanges) > 0 {
		fmt.Printf("模式切換記錄（%d 次）：", len(m.ModeChanges))
		for _, mc := range m.ModeChanges {
			fmt.Printf(" #%d→%02Xh", mc.Step, mc.Mode)
		}
		fmt.Println()
	}

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

	fmt.Printf("\n開過的檔（%d）：%s\n", len(d.Opened), join(d.Opened))
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
		fmt.Printf("找不到的檔（%d）：%s\n", len(d.Missing), join(d.Missing))
	}
	if len(d.VecSets) > 0 {
		fmt.Printf("\n設過的中斷向量（%d 次）：\n", len(d.VecSets))
		for _, v := range d.VecSets {
			fmt.Printf("  #%d int %02Xh ← %04X:%04X\n", v.Step, v.Int, v.Seg, v.Off)
		}
	}
	{
		var n int
		var first, last uint64
		for _, p := range d.Mouse.Polls {
			if p.Buttons != 0 {
				if n == 0 {
					first = p.Step
				}
				last = p.Step
				n++
			}
		}
		fmt.Printf("\nAX=3 看到按著的次數：%d（#%d–#%d）\n", n, first, last)
	}
	if d.Mouse.EventSeg != 0 || d.Mouse.EventOff != 0 {
		fmt.Printf("\n事件回呼：遮罩 %04X handler %04X:%04X，送出 %d 次\n",
			d.Mouse.EventMask, d.Mouse.EventSeg, d.Mouse.EventOff, len(d.Mouse.Events))
		for _, e := range d.Mouse.Events {
			fmt.Printf("  #%d 旗標 %02X 於 (%d,%d)\n", e.Step, e.Buttons, e.X, e.Y)
		}
	}
	if d.Mouse.PressQ[0]+d.Mouse.PressQ[1] > 0 {
		fmt.Printf("\n按鍵統計查詢：鍵0 ×%d 鍵1 ×%d；結束時待領 按下[%d %d] 放開[%d %d]\n",
			d.Mouse.PressQ[0], d.Mouse.PressQ[1],
			d.Mouse.Press[0], d.Mouse.Press[1], d.Mouse.Release[0], d.Mouse.Release[1])
	}
	if len(d.Mouse.PressReads) > 0 {
		fmt.Printf("\n回報出去的按鍵統計（%d 次）：\n", len(d.Mouse.PressReads))
		for i, p := range d.Mouse.PressReads {
			if i >= 20 {
				fmt.Printf("  …（還有 %d 次）\n", len(d.Mouse.PressReads)-20)
				break
			}
			fmt.Printf("  #%d AX=%d 鍵%d ×%d 於 (%d,%d)  呼叫端 %04X:%04X\n",
				p.Step, p.Fn, p.Button, p.Count, p.X, p.Y, p.CS, p.IP)
		}
	}
	if d.Mouse.RangeSet[0] || d.Mouse.RangeSet[1] {
		fmt.Printf("滑鼠座標範圍：X %v（設過 %v）  Y %v（設過 %v）\n",
			d.Mouse.RangeX, d.Mouse.RangeSet[0], d.Mouse.RangeY, d.Mouse.RangeSet[1])
	}
	if len(d.Mouse.Calls) > 0 {
		fmt.Printf("\nint 33h 各功能（%d 種）：", len(d.Mouse.Calls))
		fns := make([]int, 0, len(d.Mouse.Calls))
		for f := range d.Mouse.Calls {
			fns = append(fns, int(f))
		}
		sort.Ints(fns)
		for _, f := range fns {
			fmt.Printf(" AX=%04X×%d", f, d.Mouse.Calls[uint16(f)])
		}
		fmt.Println()
	}
	if len(d.FileOps) > 0 {
		fmt.Printf("\n檔案存取（%d 次）：\n", len(d.FileOps))
		for _, o := range d.FileOps {
			if o.Fn == 0x42 {
				fmt.Printf("  #%d seek %s → %d (0x%X)\n", o.Step, o.Name, o.Pos, o.Pos)
			} else {
				fmt.Printf("  #%d read %s @%d (0x%X) %d bytes\n",
					o.Step, o.Name, o.Pos, o.Pos, o.Len)
			}
		}
	}
	if len(d.PalOps) > 0 {
		fmt.Printf("\nint 10h AH=10h（%d 次）：\n", len(d.PalOps))
		for _, o := range d.PalOps {
			fmt.Printf("  #%d AL=%02X BX=%04X CX=%04X ES:DX=%04X:%04X\n",
				o.Step, o.AL, o.BX, o.CX, o.ES, o.DX)
		}
	}
	if len(d.MemOps) > 0 {
		fmt.Printf("\n記憶體配置（%d 次）：\n", len(d.MemOps))
		for _, o := range d.MemOps {
			if o.OK {
				fmt.Printf("  #%d AH=%02X BX=%04X ES=%04X → AX=%04X\n",
					o.Step, o.Fn, o.BX, o.ES, o.AX)
			} else {
				fmt.Printf("  #%d AH=%02X BX=%04X ES=%04X → 失敗（可用 %04X）\n",
					o.Step, o.Fn, o.BX, o.ES, o.AX)
			}
		}
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

	ring.dump()
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
	cs, ip, ax, sp uint16
	ds, es, bx, si uint16
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
	r.buf[r.n%r.size] = trace{c.Seg[cpu.CS], c.IP, c.R[cpu.AX], c.R[cpu.SP],
		c.Seg[cpu.DS], c.Seg[cpu.ES], c.R[cpu.BX], c.R[cpu.SI]}
	r.n++
}

func (r *ring) dump() {
	if r.size == 0 || r.n == 0 {
		return
	}
	fmt.Printf("\n最後 %d 道指令：\n", min(r.n, r.size))
	start := uint64(0)
	if r.n > r.size {
		start = r.n - r.size
	}
	for i := start; i < r.n; i++ {
		t := r.buf[i%r.size]
		fmt.Printf("  #%d %04X:%04X AX=%04X SP=%04X DS=%04X ES=%04X BX=%04X SI=%04X\n",
			i, t.cs, t.ip, t.ax, t.sp, t.ds, t.es, t.bx, t.si)
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
// stackDump 印 [bp+0] 起的 12 個 word。**參數多半在堆疊上，不在暫存器裡**
// ——只印暫存器的話，看得到「呼叫了誰」卻看不到「拿什麼去呼叫」。
func stackDump(m *machine.Machine, c *cpu.CPU) string {
	var b strings.Builder
	b.WriteString("\n      [bp+]")
	base := uint32(c.Seg[cpu.SS])*16 + uint32(c.R[cpu.BP])
	for i := 0; i < 12; i++ {
		b.WriteString(fmt.Sprintf(" %02X:%04X", i*2, m.Read16(base+uint32(i*2))))
	}
	return b.String()
}

// parseAddr 認得四種位址寫法：`lin:<hex>:<len>`、`ds:<hex>:<len>`、
// `<IDA hex>:<len>`、`<seg>:<off>:<len>`（軌跡印出來的就是最後這種）。
func parseAddr(item string) (addr uint32, n int, label string, ok bool) {
	f := strings.Split(strings.TrimSpace(item), ":")
	// **解析失敗一律回 false。** 忽略 ParseUint 的錯誤會讓打錯的位址
	// 變成 0（或 0−IDAOffset），然後安靜地讀寫到別的地方去。
	hex := func(s string, bits int) (uint64, bool) {
		v, err := strconv.ParseUint(strings.TrimSpace(s), 16, bits)
		return v, err == nil
	}
	dec := func(s string) (int, bool) {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		return v, err == nil
	}
	switch {
	case len(f) == 3 && f[0] == "lin":
		// 執行期線性位址，不做任何換算。IDA 那條路是 per-binary 的
		// （IDAOffset 是 rich2 的），別的程式要看記憶體走這條。
		lin, ok1 := hex(f[1], 32)
		nn, ok2 := dec(f[2])
		if !ok1 || !ok2 {
			return 0, 0, "", false
		}
		return uint32(lin), nn, fmt.Sprintf("lin:%s", strings.ToUpper(strings.TrimSpace(f[1]))), true
	case len(f) == 3 && f[0] == "ds":
		off, ok1 := hex(f[1], 16)
		nn, ok2 := dec(f[2])
		if !ok1 || !ok2 {
			return 0, 0, "", false
		}
		return uint32(DGROUPSeg)*16 + uint32(off), nn,
			fmt.Sprintf("ds:%s", strings.ToUpper(strings.TrimSpace(f[1]))), true
	case len(f) == 2:
		ida, ok1 := hex(f[0], 32)
		nn, ok2 := dec(f[1])
		if !ok1 || !ok2 {
			return 0, 0, "", false
		}
		return uint32(ida) - IDAOffset, nn,
			fmt.Sprintf("IDA %s", strings.ToUpper(strings.TrimSpace(f[0]))), true
	case len(f) == 3:
		// 段:偏移:長度——軌跡印出來的就是這個形式，直接貼進來。
		seg, ok1 := hex(f[0], 16)
		off, ok2 := hex(f[1], 16)
		nn, ok3 := dec(f[2])
		if !ok1 || !ok2 || !ok3 {
			return 0, 0, "", false
		}
		return uint32(seg)*16 + uint32(off), nn,
			fmt.Sprintf("%s:%s", strings.ToUpper(strings.TrimSpace(f[0])), strings.ToUpper(strings.TrimSpace(f[1]))), true
	}
	return 0, 0, "", false
}

func dumpPeek(m *machine.Machine, spec string) {
	fmt.Println("\n記憶體：")
	for _, item := range strings.Split(spec, ",") {
		addr, n, label, ok := parseAddr(item)
		if !ok {
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

// pokeSpec 是一次「在第幾道指令把某段記憶體改成什麼」。
type pokeSpec struct {
	at    uint64
	addr  uint32
	data  []byte
	label string
}

// parsePokes 解 `-poke` 的內容：`<位址>@<步數>=<hex bytes>`，分號分隔。
//
// 步數用 `@` 分隔而不是 `:`——位址本身就有冒號，`0040:0049=07` 會被
// 當成「IDA 位址 0040、步數 49」而不報錯，然後安靜地寫到別的地方去。
//
// **對拍要固定的是狀態，不是運氣。** 靠亂數種子或重跑到某個局面，
// 換一版執行器就全部作廢；直接把記憶體改成要的值，收據才可重現。
func parsePokes(spec string) ([]pokeSpec, error) {
	var out []pokeSpec
	for _, item := range strings.Split(spec, ";") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		lhs, hex, found := strings.Cut(item, "=")
		if !found {
			return nil, fmt.Errorf("-poke 的 %q 少了 `=<hex bytes>`", item)
		}
		where, when, found := strings.Cut(lhs, "@")
		if !found {
			return nil, fmt.Errorf("-poke 的 %q 少了步數（位址@步數=bytes）", item)
		}
		at, err := strconv.ParseUint(strings.TrimSpace(when), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("-poke 的 %q 步數看不懂：%w", item, err)
		}
		// 位址部分借用 peek 的寫法，長度欄補 0（poke 的長度由 bytes 決定）。
		addr, _, label, ok := parseAddr(strings.TrimSpace(where) + ":0")
		if !ok {
			return nil, fmt.Errorf("-poke 的 %q 位址看不懂", item)
		}
		var data []byte
		for _, b := range strings.Fields(strings.ReplaceAll(strings.TrimSpace(hex), ",", " ")) {
			v, err := strconv.ParseUint(b, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("-poke 的 %q 有不是 hex 的位元組 %q", item, b)
			}
			data = append(data, byte(v))
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("-poke 的 %q 沒有要寫的位元組", item)
		}
		out = append(out, pokeSpec{at: at, addr: addr, data: data, label: label})
	}
	return out, nil
}

// dumpFind 在 1 MB 記憶體裡找一串 bytes，印出所有命中的線性位址。
// 除錯壞指標用：retf 跳到垃圾的時候，先找垃圾值是誰放進去的。
func dumpFind(m *machine.Machine, d *dos.DOS, hexpat string) {
	hexpat = strings.ReplaceAll(hexpat, " ", "")
	if len(hexpat)%2 != 0 {
		fmt.Println("  -find：hex 長度要是偶數")
		return
	}
	pat := make([]byte, len(hexpat)/2)
	for i := range pat {
		v, err := strconv.ParseUint(hexpat[i*2:i*2+2], 16, 8)
		if err != nil {
			fmt.Printf("  -find：%q 不是 hex\n", hexpat)
			return
		}
		pat[i] = byte(v)
	}
	fmt.Printf("\n搜尋 % X：\n", pat)
	hits := 0
	for a := 0; a+len(pat) <= len(m.Mem); a++ {
		match := true
		for j, b := range pat {
			if m.Mem[a+j] != b {
				match = false
				break
			}
		}
		if match {
			fmt.Printf("  線性 %05X\n", a)
			hits++
			if hits == 40 {
				fmt.Println("  …（只印前 40 個）")
				break
			}
		}
	}
	// **EMS 也要掃。** 遊戲把字型、圖庫這類大東西放在 EMS，只有當下映射
	// 進頁框的幾頁會出現在 1 MB 位址空間裡。漏掉它的話「找不到」會被誤讀成
	// 「不存在」——實際踩過：GRAPH.IMG 的字型在記憶體搜尋一無所獲，
	// 因為它整份在 EMS（`~/cht/logh3/docs/re/07`）。
	for _, pg := range d.EMSPages() {
		for a := 0; a+len(pat) <= len(pg.Data); a++ {
			match := true
			for j, b := range pat {
				if pg.Data[a+j] != b {
					match = false
					break
				}
			}
			if match {
				fmt.Printf("  EMS handle %d 第 %d 頁 +%04X\n", pg.Handle, pg.Page, a)
				hits++
				if hits >= 40 {
					fmt.Println("  …（只印前 40 個）")
					return
				}
			}
		}
	}
	if hits == 0 {
		fmt.Println("  （沒有命中）")
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

// scanOf 把按鍵名稱轉成 set-1 掃描碼。
func scanOf(s string) (uint8, bool) {
	named := map[string]uint8{
		"up": 0x48, "down": 0x50, "left": 0x4B, "right": 0x4D,
		"enter": 0x1C, "esc": 0x01, "space": 0x39, "tab": 0x0F,
	}
	if v, ok := named[strings.ToLower(s)]; ok {
		return v, true
	}
	if v, err := strconv.ParseUint(s, 16, 8); err == nil && len(s) == 2 {
		return uint8(v), true
	}
	if len(s) == 1 {
		chars := "1234567890"
		if i := strings.IndexByte(chars, s[0]); i >= 0 {
			return uint8(0x02 + i), true
		}
	}
	return 0, false
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "probe:", err)
	os.Exit(1)
}

// writeEGA 把 planar VRAM 解成 PNG（spec 009 §1）。
// 色號陣列（.bin）是驗收依據，PNG 只是給人看的——比照 mode 13h。
func writeEGA(path string, m *machine.Machine) error {
	var w, h int
	switch m.VideoMode() {
	case 0x12:
		w, h = 640, 480
	case 0x10:
		w, h = 640, 350
	case 0x0D:
		w, h = 320, 200
	case 0x0E, 0x0F, 0x11:
		w, h = 640, 200
	default:
		return fmt.Errorf("模式 %02Xh 不是 planar（或還沒支援尺寸）", m.VideoMode())
	}
	idx := m.PlanarPixels(w, h)
	bin := strings.TrimSuffix(path, ".png") + ".bin"
	if err := os.WriteFile(bin, idx, 0o644); err != nil {
		return err
	}
	// 16 色模式：色號 → 屬性調色盤 → DAC（spec 009 §1）。
	dac := m.Palette()
	p := make(color.Palette, 256)
	for i := range p {
		c := dac[m.AttrPal[i&15]]
		p[i] = color.RGBA{c[0], c[1], c[2], 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), p)
	copy(img.Pix, idx)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	fmt.Printf("寫出 %s（%d×%d，planar 解碼）＋ %s（色號陣列）\n", path, w, h, bin)
	return nil
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
