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
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

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
	root := flag.String("root", ".", "原版素材目錄（配 -load-state 時不必再給，"+
		"狀態檔裡存著；真的給了就以命令列為準）")
	steps := flag.Uint64("steps", 20_000_000,
		"跑到第幾道指令為止（**絕對步數**，配 -load-state 時要大於檢查點的步數）")
	trace := flag.Uint64("trace", 0, "最後幾道指令的軌跡（0 ＝ 不記）")
	typeText := flag.String("type", "", "啟動前排入DOS標準輸入的可重播位元組字串")
	dumpVRAM := flag.String("dump-vram", "", "把 A0000 的 320×200 色號陣列寫到這個檔")
	dumpScreenPNG := flag.String("dump-screen-png", "",
		"把平面模式（mode 0Dh–12h）的畫面存成 PNG（經屬性控制器與 DAC）。\n"+
			"    與 -dump-screen 的差別是**那一支存色號、這一支存看得到的顏色**。")
	cropTop := flag.Int("crop-top", 0, "存畫面前從上面裁掉幾列")
	cropH := flag.Int("crop-height", 0, "存畫面只留幾列（0 ＝ 全部）")
	xscale := flag.Int("xscale", 0,
		"int 33h 的水平虛擬座標倍率（0 ＝ 依視訊模式自動決定：320 寬 → 2、640 寬 → 1）。"+
			"**寫死一個值會讓另一半的模式全錯**，而症狀是點擊落在別的地方、"+
			"畫面完全不動——看起來像輸入沒送到")
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
		"（`步數:X:Y`，鍵欄可省；`docs/spec/016`）。DOS 遊戲拿右鍵當取消／"+
		"關閉視窗，少了它被蓋住的視窗一個都點不到，而症狀是畫面完全不動")
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
	shotScript := flag.String("shots", "",
		"在指定步數各存一張畫面：`步數:路徑` 用逗號分隔。"+
			"色盤存成同名 .pal。一次跑要看好幾個畫面時用這個，"+
			"不要為了看中途的畫面重跑")
	clickPremove := flag.Int("click-premove", 0,
		"每一次腳本點擊先把游標移過去、等遊戲讀了幾次滑鼠才按下去（0 ＝ 移到就按）。"+
			"有些對話框的鈕吃「游標已經在上面」這個狀態（`docs/spec/004` §4.19）")
	clickScript := flag.String("clicks", "",
		"點擊腳本：`步數:X:Y[:鍵]` 用逗號分隔（鍵 1 ＝ 左、2 ＝ 右，預設 1）。"+
			"按住時間用 -click-hold")
	watchVideo := flag.Bool("watch-video", false,
		"統計寫進 A0000–BFFFF 的位址範圍（回答「它到底畫在哪裡」）")
	watch := flag.String("watch", "", "監看記憶體寫入，格式 <線性hex>-<線性hex>；"+
		"每次寫入印出步數與 CS:IP（除錯「誰把向量改掉了」）")
	logCalls := flag.Bool("log-calls", false, "統計每一種 (中斷, AH) 呼叫幾次")
	tick := flag.Uint64("tick", 0, "每幾道指令送一次計時器中斷（0 ＝ 用預設）")
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
	dumpAt := flag.String("dump-at", "",
		"跑到指定步數就傾印一張畫面：`<步數>:<檔名>`，分號分隔可以給很多張。"+
			"探索「點下去之後跑到哪個畫面」用——一次跑就看得到中間的每一格，"+
			"不必為了每張畫面重跑一次")
	dumpMem := flag.String("dump-mem", "",
		"跑完把幾段線性記憶體各寫成一個檔：`<lo>-<hi>:<路徑>`（位址十六進位），"+
			"逗號分隔多段。一次跑要挖好幾塊緩衝區時用這個，不要為了第二塊重跑")
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
	vgaAt := flag.String("vga-regs-at", "",
		"執行到這個 `<seg>:<off>` 時印出 VGA 圖形控制器／序列器／latch。"+
			"畫面上的位元不等於 CPU 寫的位元組時（write mode、bit mask、"+
			"set/reset），只看寫入指令看不出所以然")
	regsMax := flag.Int("regs-max", 20,
		"每個 -regs-at 位址最多記幾次。逐格處理的迴圈跑幾百次，"+
			"預設的 20 次只看得到第一個物件")
	vramSites := flag.Bool("vram-sites", false,
		"統計「誰在寫視訊記憶體」，印出前 20 名 CS:IP（找繪圖常式）")
	vramAt := flag.String("vram-at", "",
		"把 -vram-sites 限定在這個 VRAM 位移（16 進位）。"+
			"盯單一像素用——「這一點是誰畫的」比「誰畫得最多」更能定位")
	args := flag.String("args", "", "命令列尾（寫進 PSP+80h，.COM 的參數走這裡）。"+
		"靠參數決定要做什麼的程式（例如 ENDING.EXE 要演哪一個結局）沒有它就直接結束")
	queue := flag.String("queue", "", "主程式結束／常駐後接著跑的程式（監督佇列，`docs/spec/009` §4），逗號分隔")
	segLog := flag.Bool("seg-log", false, "記錄 CS 的每一次改變，報告裡印出每個段第一次執行的時間與來源")
	dumpScreen := flag.String("dump-screen", "", "跑完把畫面的色號寫成檔案（planar 模式是 VideoSize() 那個尺寸）")
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
	flag.StringVar(&readsOf, "reads-of", "",
		"只列這個檔的讀檔紀錄（不分大小寫的子字串），而且**全部列出來**。"+
			"預設只印前 15 後 15，追某一個檔的讀取版面時中間那段才是重點")
	ipLog := flag.String("ip-log", "",
		"把 [起,迄) 這段每一道指令的 CS:IP 以二進位寫出來（每筆 4 bytes，小端 CS 後 IP）：`起:迄:路徑`。"+
			"用來對兩次只差一個輸入的執行，找出控制流第一次分岔的位置")
	saveState := flag.String("save-state", "",
		"跑到某一步就把整台機器存成檔案：`步數:路徑`，逗號分隔多個檢查點。"+
			"配 -load-state 用——要觀測的畫面在幾億道指令之後時，"+
			"存一次，之後每個實驗從那裡展開，一輪從幾分鐘變成幾秒")
	loadState := flag.String("load-state", "",
		"從狀態檔接著跑（-save-state 存的）。這時 -exe 不必給。"+
			"⚠ **步數一律是絕對值**：讀檔之後步數從存檔當時繼續往上加，"+
			"-steps／-clicks／-shots／-save-state 的數字都要用絕對步數，"+
			"給「還要跑幾道」那種預算值會一道都不跑（而且不會報錯）")
	keys := flag.String("keys", "", "先排進鍵盤佇列的按鍵（`\\n` 是 Enter）")
	biosKeys := flag.String("bios-keys", "",
		"逐個送進 **BIOS 鍵盤緩衝區**（BDA 0040:001E）的字元（`\\n` ＝ Enter）。\n"+
			"    -keys／-keys-at 走的是可重播的 Stdin 佇列與硬體 IRQ1；\n"+
			"    直接比對 0040:001A／001C 判斷有沒有按鍵的程式只認這一條。")
	biosKeyEvery := flag.Uint64("bios-key-every", 2_000_000, "兩次送鍵之間隔幾道指令")
	biosKeyFrom := flag.Uint64("bios-key-from", 2_000_000, "第幾道指令開始送第一個鍵")
	covOut := flag.String("coverage", "",
		"把執行過的線性位址寫成 JSON 區段表。\n"+
			"    打包過的執行檔靜態反組譯是亂碼；這份清單是「哪些 byte 是程式碼」\n"+
			"    唯一直接的答案，拿去當 IDA 的種子。")
	keysAt := flag.String("keys-at", "",
		"在指定的指令數餵鍵：<步數>:<鍵>[,<步數>:<鍵>…]。\n"+
			"    -keys 是開場就塞進佇列，會在早期的提示就被吃光；\n"+
			"    後面才出現的「按任意鍵」要用這個。")
	dumpEGA := flag.String("dump-ega", "",
		"把平面式 VRAM 存成 PNG：<寬>x<高>=<檔名>（例 640x350=t.png）。\n"+
			"    平面資料本身不記解析度，尺寸猜錯會得到錯位但看起來像圖的東西。")
	dumpSeg := flag.String("dump-seg", "",
		"把記憶體寫成檔：<seg>:<off>:<長度>[,...]=<檔名前綴>。\n"+
			"    與 -dump-mem 的差別是**吃段:位移不是線性位址**——\n"+
			"    -peek／-dump-mem 都要靠映像基底換算，而執行期搬到別的段的\n"+
			"    程式碼（overlay 管理員的 thunk、載進來的 overlay）在映像裡\n"+
			"    根本不存在，換算不到。")
	blockAfter := flag.Uint64("block-after", 100_000,
		"連續阻塞在鍵盤輸入這麼多步就停（0 ＝ 不停）。留一段是給計時器 ISR 推背景動畫用的")
	egaEvery := flag.String("ega-every", "",
		"每 N 道指令存一張 EGA 畫面：<N>:<寬>x<高>=<檔名前綴>。\n"+
			"    單張只看得到終點，看不出按鍵是送早了還是送晚了。")
	memTrace := flag.Bool("mem-trace", false,
		"逐筆記 AH=48h／49h 的要求與回應（含呼叫端 CS:IP）。\n"+
			"    「總共佔了多少」答不出「哪一次開始偏離」——要比對配置器\n"+
			"    跟真 DOS 的差別只能一筆一筆看。")
	dumpPorts := flag.String("dump-ports", "",
		"把 I/O 寫入序列存成 TSV：`<檔名>` 全部，或 `<埠>,<埠>=<檔名>` 只存那幾個埠")
	clickBtn := flag.Int("click-button", 0, "按哪一個鍵（0 左／1 右／2 中）")
	dumpWAV := flag.String("dump-wav", "",
		"把 PC 喇叭的波形寫成 8 位元單聲道 WAV（語音對拍用）")
	cpuProfile := flag.String("cpuprofile", "",
		"把 CPU 剖析結果寫到這個檔（找瓶頸用；`go tool pprof` 讀）")
	flag.Parse()

	if *exe == "" && *loadState == "" {
		flag.Usage()
		os.Exit(2)
	}

	if *cpuProfile != "" {
		pf, err := os.Create(*cpuProfile)
		if err != nil {
			die(err)
		}
		if err := pprof.StartCPUProfile(pf); err != nil {
			die(err)
		}
		defer func() {
			pprof.StopCPUProfile()
			pf.Close()
		}()
	}
	m := machine.New()
	if *adlib {
		m.SetAdLib(true)
	}
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
		// ⚠ **`WatchWrites` 只留一個回呼**（後註冊的蓋掉前一個），
		// 所以列印與收集要在同一支裡做。分成兩次註冊的話，先註冊的那個
		// 靜靜失效——症狀是 `-watch` 照印，但 `-watch-file` 永遠是空的。
		m.WatchWrites(lo, hi, func(a uint32, old, nw uint8) {
			fmt.Printf("[watch] #%d %05X: %02X → %02X  ← %04X:%04X\n",
				m.Steps, a, old, nw, m.CPU.Seg[cpu.CS], m.CPU.IP)
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
	obsSetup(m) // 觀測用旗標的掛鉤（observe.go）
	d := dos.New(m, *root)
	d.Mouse.XScale = uint16(*xscale)
	if *queue != "" {
		for _, q := range strings.Split(*queue, ",") {
			d.Enqueue(strings.TrimSpace(q), "")
		}
	}
	d.CallTrace = []dos.CallRec{}
	if *memTrace {
		d.MemTrace = []dos.MemCall{}
	}
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
	if *loadState != "" {
		if err := state.Load(*loadState, m, d); err != nil {
			die(err)
		}
		// 命令列真的給了 -root 才蓋掉狀態檔裡的。
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "root" {
				d.Root = *root
			}
		})
		fmt.Printf("從 %s 接著跑（第 %d 道指令，素材目錄 %s）\n",
			*loadState, m.Steps, d.Root)
	}
	saves, err := parseSaveState(*saveState)
	if err != nil {
		die(err)
	}
	if *keys != "" {
		feedKeys(m, d, []byte(strings.ReplaceAll(*keys, "\\n", "\n")))
	}
	d.Stdin = append(d.Stdin, []byte(*typeText)...)

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

	var vgaSite *regSite
	vgaShots := 0
	if f := strings.Split(strings.TrimSpace(*vgaAt), ":"); len(f) == 2 {
		sg, err1 := strconv.ParseUint(f[0], 16, 16)
		of, err2 := strconv.ParseUint(f[1], 16, 16)
		if err1 != nil || err2 != nil {
			die(fmt.Errorf("-vga-regs-at 要寫成 <seg>:<off>：%q", *vgaAt))
		}
		vgaSite = &regSite{uint16(sg), uint16(of)}
	}

	type shot struct {
		at   uint64
		path string
	}
	var dumpShots []shot
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
		dumpShots = append(dumpShots, shot{at: at, path: item[i+1:]})
	}

	memPokes, err := parsePokes(obsPokeScript(*poke))
	if err != nil {
		die(err)
	}
	var lastScreen []uint8
	clicks, err := parseClicks(*clickScript)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// -rclicks 是同一份清單，只是鍵預設右鍵。DOS 遊戲拿右鍵當取消／
	// 關閉視窗，少了它被蓋住的視窗一個都點不到（`docs/spec/016`）。
	rc, err := parseClicks(*rclicks)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, c := range rc {
		if c.btn == 1 {
			c.btn = 2
		}
		clicks = append(clicks, c)
	}
	shots, err := parseShots(*shotScript)
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
	preIdx := -1
	var pressStep uint64

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
	// -bios-keys 攤成 rune，一次送一個。
	biosRunes := []rune(strings.ReplaceAll(*biosKeys, "\\n", "\n"))
	bki := 0

	pendingKeys, err := parseKeysAt(*keysAt)
	if err != nil {
		die(err)
	}
	var shotEvery uint64
	var shotSpec string
	var shotN int
	if *egaEvery != "" {
		parts := strings.SplitN(*egaEvery, ":", 2)
		if len(parts) != 2 {
			die(fmt.Errorf("-ega-every 格式是 <N>:<寬>x<高>=<檔名前綴>"))
		}
		if _, err := fmt.Sscanf(parts[0], "%d", &shotEvery); err != nil || shotEvery == 0 {
			die(fmt.Errorf("-ega-every 的 N 解不出來：%q", parts[0]))
		}
		shotSpec = parts[1]
	}
	if *covOut != "" {
		m.Coverage = make([]bool, 1<<20)
	}
	ring := newRing(*trace)

	var runErr error
	var blockedFor uint64
	var blockedStop bool
	// 執行速度要量得到才調得動。**沒有這個數字的時候，「慢」是感覺**，
	// 而感覺分不出「這一段本來就有幾十億道指令」與「執行器每道指令太貴」。
	started := time.Now()
	startSteps := m.Steps
	for m.Steps < *steps && !m.CPU.Halted && !d.Exited {
		// -ega-every：每 N 道指令存一張。**單張只看得到終點**，
		// 看不出按鍵是送早了還是送晚了。
		if shotEvery > 0 && m.Steps >= uint64(shotN+1)*shotEvery {
			shotN++
			sp := strings.SplitN(shotSpec, "=", 2)
			if len(sp) == 2 {
				name := fmt.Sprintf("%s=%s-%03d.png", sp[0], strings.TrimSuffix(sp[1], ".png"), shotN)
				if err := doDumpEGA(m, name); err != nil {
					fmt.Fprintln(os.Stderr, "ega-every:", err)
				}
			}
		}
		// **護欄：程式碼不該跑進 A0000 以上。** 那裡是視訊記憶體與 BIOS，
		// 在我們這台上全是 0，而 `00 00` ＝ `add [bx+si],al` 一路解得下去，
		// 所以飛掉之後不會有任何錯誤——只會安靜地走完幾百萬道指令
		// （第一次實驗就是這樣，停在 A303:60BD）。
		if a := cpu.Linear(m.CPU.Seg[cpu.CS], m.CPU.IP); a >= machine.VideoSeg*16 && a < machine.MemSize {
			runErr = fmt.Errorf("跑出可用記憶體：CS:IP ＝ %04X:%04X（線性 %05X）",
				m.CPU.Seg[cpu.CS], m.CPU.IP, a)
			break
		}
		if *mouseX >= 0 && m.Steps == moveAt {
			d.MoveMouse(*mouseX, *mouseY)
		}
		if len(pendingKeys) > 0 && m.Steps >= pendingKeys[0].at {
			feedKeys(m, d, pendingKeys[0].key)
			pendingKeys = pendingKeys[1:]
		}
		// -bios-keys：**照指令數排程，不照時間**，這樣對拍才是決定性的。
		// 而且要等緩衝區空了才送下一個——遊戲的輪詢頻率遠低於送鍵頻率，
		// 不等的話同一格會被連續蓋掉，看起來像「只有最後一個鍵進去了」。
		if bki < len(biosRunes) && m.Steps >= *biosKeyFrom+uint64(bki)*(*biosKeyEvery) {
			if _, pending := m.PeekKey(); !pending {
				d.PushKey(biosKeyOf(biosRunes[bki]))
				bki++
			}
		}
		if *mouseX >= 0 && m.Steps == moveAt {
			// **改座標之後要送移動事件**，跟 -clicks 一樣。
			// 遊戲的游標是靠事件回呼畫的（`docs/spec/013`）：只改座標的話，
			// 舊位置那隻游標不會被擦掉，新位置也不會畫出來——
			// 畫面上留著一隻停在原地的游標，看起來像「滑鼠沒動」。
			d.MoveMouse(*mouseX, *mouseY)
		}
		// 點擊：按下 → 按住 → 放開。
		//
		// **按住時間不能短**。遊戲輪詢 int 33h 的頻率很低，按下與放開之間
		// 隔太近會整個被跳過（`rich2/docs/playtest/001` §5.6：DOSBox 那邊
		// 同一題要點三次才生效一次，改成按住 0.35 秒才穩）。而各畫面的
		// 輪詢頻率差三個數量級——磁片提示每千萬道只問 12 次，讀檔選單問
		// 一千次——所以固定指令數的按住不是漏掉就是重複觸發幾千次，
		// -click-polls 才是穩的那一個。
		if *clickX >= 0 {
			switch {
			case m.Steps == *clickAt:
				d.MoveMouse(*clickX, *clickY)
				d.PressMouse(*clickBtn)
				pollsAtPress = len(d.Mouse.Polls)
				held = true
			case held && releaseNow(d, *clickPolls, *clickHold, pollsAtPress, m.Steps, *clickAt):
				d.ReleaseMouse(*clickBtn)
				held = false
			}
		}
		// 點擊腳本：**一次跑帶著整串輸入**才走得到深處的畫面。
		// 一次一個點擊代表每多走一步就要重跑一次，而遊戲跑到主選單
		// 就要六千萬道指令。
		//
		// ⚠ **改座標之後一定要送移動事件。** 遊戲的游標是靠 int 33h 的
		// 事件回呼畫的：只改座標的話，靠回呼追游標的程式手上還是舊位置，
		// 按下就落在別的地方——而畫面上什麼都不會發生，看起來像
		// 「點擊沒送到」。
		for i, c := range clicks {
			// 腳本的鍵欄是 1 ＝ 左、2 ＝ 右；服務層的編號是 0／1／2。
			btn := 0
			if c.btn&2 != 0 {
				btn = 1
			}
			switch {
			case m.Steps == c.step:
				d.MoveMouse(int(c.x), int(c.y))
				if *clickPremove > 0 {
					// 先移過去，等遊戲看到游標在那裡再按。
					pollsAtPress = len(d.Mouse.Polls)
					preIdx = i
					break
				}
				d.PressMouse(btn)
				pollsAtPress = len(d.Mouse.Polls)
				downIdx, pressStep = i, m.Steps
			case preIdx == i && len(d.Mouse.Polls)-pollsAtPress >= *clickPremove:
				d.PressMouse(btn)
				pollsAtPress = len(d.Mouse.Polls)
				preIdx, downIdx = -1, i
				pressStep = m.Steps
			case downIdx == i && releaseNow(d, *clickPolls, *clickHold, pollsAtPress, m.Steps, pressStep):
				d.ReleaseMouse(btn)
				downIdx = -1
			}
		}
		for _, p := range memPokes {
			if m.Steps == p.at {
				for i, b := range p.data {
					m.Write8(p.addr+uint32(i), b)
				}
				fmt.Printf("#%d 改記憶體 %s（%05X）%d bytes\n", m.Steps, p.label, p.addr, len(p.data))
			}
		}
		if vgaSite != nil && m.CPU.Seg[cpu.CS] == vgaSite.seg && m.CPU.IP == vgaSite.off &&
			m.Steps >= *regsFrom && vgaShots < 4 {
			gc, sq, la := m.VGAState()
			fmt.Printf("#%d VGA @%04X:%04X  GC=%X\n           Seq=%X  latch=%X\n",
				m.Steps, vgaSite.seg, vgaSite.off, gc[:9], sq[:5], la)
			vgaShots++
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
		for _, sh := range dumpShots {
			if m.Steps == sh.at {
				if err := writeEGA(sh.path, m); err != nil {
					die(err)
				}
				fmt.Printf("#%d 傾印畫面 → %s\n", m.Steps, sh.path)
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
				d.MoveMouse(int(p.x), int(p.y))
				d.PressMouse(0)
			}
		}
		if sweep != nil && m.Steps >= sweep.from &&
			(m.Steps-sweep.from)%sweep.every == sweep.every/2 {
			d.ReleaseMouse(0)
		}
		if ca != nil && m.Steps >= ca.from && m.Steps < ca.to &&
			m.CPU.Seg[cpu.CS] == ca.seg && m.CPU.IP == ca.off {
			ca.record(m)
		}
		if ipw != nil && m.Steps >= ipFrom && m.Steps < ipTo {
			ipw.push(m.CPU.Seg[cpu.CS], m.CPU.IP)
		}
		ring.push(m.CPU)
		obsStep(m) // 觀測用的每一步掛鉤（observe.go）；沒開旗標時是一個比較
		if runErr = m.Step(); runErr != nil {
			break
		}
		// 阻塞在鍵盤輸入時程式一道指令都不往前走（spec 008），
		// 再跑下去只是把 INT 重跑幾百萬次。留 blockAfter 步給計時器
		// 推背景動畫，之後就停——繼續燒預算不會有新資訊。
		if d.Blocked {
			blockedFor++
			if *blockAfter > 0 && blockedFor >= *blockAfter {
				blockedStop = true
				break
			}
		} else {
			blockedFor = 0
		}
	}

	if ca != nil {
		ca.dump()
	}
	if ran := m.Steps - startSteps; ran > 0 {
		el := time.Since(started)
		fmt.Printf("\n執行 %d 道指令，耗時 %s，%.1f M 道／秒\n",
			ran, el.Round(time.Millisecond),
			float64(ran)/el.Seconds()/1e6)
	}
	if blockedStop {
		fmt.Printf("\n⏸ 在鍵盤輸入上連續阻塞 %d 步，提早停下（-block-after）。\n"+
			"   阻塞時程式一道指令都不走，繼續跑只是把同一道 INT 重跑。\n", blockedFor)
	}
	if *dumpEGA != "" {
		if err := doDumpEGA(m, *dumpEGA); err != nil {
			fmt.Fprintln(os.Stderr, "dump-ega:", err)
		}
	}
	if *dumpSeg != "" {
		if err := doDumpMem(m, *dumpSeg); err != nil {
			fmt.Fprintln(os.Stderr, "dump-seg:", err)
		}
	}
	if *covOut != "" {
		if err := writeCoverage(m, *covOut); err != nil {
			fmt.Fprintln(os.Stderr, "coverage:", err)
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
	obsReport(m, d, func(w *bufio.Writer) {
		for _, x := range writes {
			fmt.Fprintf(w, "%d %05x %02x %02x %04x:%04x\n",
				x.step, x.addr, x.old, x.nw, x.cs, x.ip)
		}
	})
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
	if n := len(m.Speaker); n > 0 {
		fmt.Printf("\nPC 喇叭：切換 %d 次，8253 通道 0 分頻值 %d（%.0f Hz）\n",
			n, m.PITDivisor(), m.PITHz())
		if m.IRQ0Clamped > 0 {
			fmt.Printf("  ⚠ 中斷間隔被夾到下限 %d 次——波形的時間軸不可信\n",
				m.IRQ0Clamped)
		}
	}
	if *dumpWAV != "" {
		if err := writeSpeakerWAV(m, *dumpWAV); err != nil {
			fmt.Fprintln(os.Stderr, "dump-wav:", err)
		} else {
			fmt.Printf("喇叭波形 → %s\n", *dumpWAV)
		}
	}
	if *dumpPorts != "" {
		if err := writePortLog(m, *dumpPorts); err != nil {
			die(err)
		}
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
	// `-dump-mem` 有兩種寫法：`<lo>-<hi>:<檔名>`（範圍，`writeMemDump`
	// 在上面已經處理完，可以一次給好幾段）與 `<位址>:<長度>:<檔名>`
	// （`parseAddr` 的寫法，支援 `lin:`／`ds:`／段:位移）。
	// **範圍那種到這裡要跳過，不能當成解析失敗**——不然檔案照樣寫出來了，
	// 程式卻以 exit 1 結束，看起來像整趟跑壞掉。
	if *dumpMem != "" && !strings.Contains(*dumpMem, "-") {
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
	if *dumpScreenPNG != "" {
		if err := writeScreen(m, *dumpScreenPNG, *cropTop, *cropH); err != nil {
			die(err)
		}
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

// readsOf 是 -reads-of 的值；report 在另一個函式裡，用套件層變數傳。
var readsOf string

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
	// 計時器的狀態要印。**「等 tick 的迴圈轉不出來」與「程式本來就沒事做」
	// 從 CS:IP 看起來一模一樣**——Pool of Radiance 的開場就是停在
	// `CMP AL, ES:[DI]` / `JZ −5`（`ES:DI` ＝ `0040:006C`，BIOS 的 tick），
	// 而 tick 沒動的原因只有兩種：沒送中斷，或 IF 一直是 0。
	fmt.Printf("計時器：送出 %d 次  IF=%v  int08 向量 %04X:%04X  int1C 向量 %04X:%04X\n",
		m.Ticks, m.CPU.Flag(cpu.IF),
		m.Read16(0x08*4+2), m.Read16(0x08*4),
		m.Read16(0x1C*4+2), m.Read16(0x1C*4))
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
	if len(d.Reads) > 0 && readsOf != "" {
		want := strings.ToUpper(readsOf)
		n := 0
		fmt.Printf("\n讀檔（只列 %s）：\n", readsOf)
		for _, r := range d.Reads {
			if !strings.Contains(strings.ToUpper(r.Name), want) {
				continue
			}
			n++
			fmt.Printf("  #%-9d %-14s handle=%04X → %04X:%04X 要 %d 得 %d（線性 %05X–%05X）\n",
				r.Step, r.Name, r.Handle, r.Seg, r.Off, r.Want, r.Got,
				uint32(r.Seg)*16+uint32(r.Off), uint32(r.Seg)*16+uint32(r.Off)+uint32(r.Got))
		}
		fmt.Printf("  共 %d 筆\n", n)
	}
	if len(d.Reads) > 0 && readsOf == "" {
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
	if d.Mouse.Handler.Set {
		fmt.Printf("\n事件回呼：遮罩 %04X handler %04X:%04X，送出 %d 次\n",
			d.Mouse.Handler.Mask, d.Mouse.Handler.Seg, d.Mouse.Handler.Off, len(d.Mouse.Events))
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
	if d.Mouse.MaxX > 0 || d.Mouse.MaxY > 0 {
		// 範圍是**虛擬座標**（`AX=7`／`AX=8` 設的）。程式用它宣告自己
		// 期待的座標系；兩邊對不上時游標與命中判定會整個偏移，
		// 而畫面看起來完全正常。
		fmt.Printf("滑鼠座標範圍（虛擬）：X %d–%d  Y %d–%d\n",
			d.Mouse.MinX, d.Mouse.MaxX, d.Mouse.MinY, d.Mouse.MaxY)
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
	for _, detail := range d.UnimplementedDetails {
		fmt.Printf("  首次暫存器 %s @ %04X:%04X DS=%04X ES=%04X AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X\n",
			detail.Call, detail.CS, detail.IP, detail.DS, detail.ES, detail.AX, detail.BX,
			detail.CX, detail.DX, detail.SI, detail.DI)
		if detail.Path != "" {
			fmt.Printf("    路徑 %q；參數區 % X\n", detail.Path, detail.Param)
		}
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
	if n := m.IRQ1Delivered(); n > 0 || m.KeyQueueLen() > 0 {
		fmt.Printf("\n硬體鍵盤：送出 IRQ1 %d 次（其中 %d 次因為沒人裝 int 09h 而留著），"+
			"佇列還剩 %d 個事件\n  int 09h 向量 ＝ %04X:%04X（stub 段是 %04X）\n",
			n, m.KeyStalls(), m.KeyQueueLen(),
			m.Read16(0x09*4+2), m.Read16(0x09*4), uint16(machine.StubSeg))
	}
	if len(d.KeyReads) > 0 || len(d.Stdin) > 0 {
		fmt.Printf("\n按鍵去向（取走 %d 個，佇列還剩 %d 個）：\n",
			len(d.KeyReads), len(d.Stdin))
		for _, k := range d.KeyReads {
			ch := "."
			if k.Key >= 0x20 && k.Key < 0x7F {
				ch = string(rune(k.Key))
			}
			fmt.Printf("  #%-11d %-11s %02X %s\n", k.Step, k.Via, k.Key, ch)
		}
		if len(d.Stdin) > 0 {
			fmt.Printf("  ⚠ 沒人取走：% X —— 送進去的鍵不等於程式收到的鍵\n", d.Stdin)
		}
	}

	if d.KeyWaits > 0 {
		fmt.Printf("\n⚠ 佇列空時被要求讀鍵 %d 次——它在**等鍵盤**，不是在做事。\n"+
			"   加大 -steps 沒有用，要用 -keys 餵鍵。\n", d.KeyWaits)
	}

	if len(d.FileOps) > 0 {
		fmt.Printf("\n檔案操作（%d 次）：\n", len(d.FileOps))
		for _, f := range d.FileOps {
			fmt.Printf("  #%-9d %-5s h=%d %-12s arg=%-10d pos=%-10d len=%d\n",
				f.Step, f.Op, f.Handle, f.Name, f.Arg, f.Pos, f.Len)
		}
	}
	if len(d.MemTrace) > 0 {
		var nAlloc, nFree, nFail int
		var held int64 // 目前握在手上的段數
		var peak int64
		for _, m := range d.MemTrace {
			switch {
			case m.Op == 0x48 && m.OK:
				nAlloc++
				held += int64(m.Got)
			case m.Op == 0x48:
				nFail++
			case m.Op == 0x49 && m.OK:
				nFree++
				held -= int64(m.Got)
			default:
				nFail++
			}
			if held > peak {
				peak = held
			}
		}
		fmt.Printf("\n配置器逐筆帳（AH=48h 成功 %d／失敗 %d，AH=49h %d 次；"+
			"淨持有 %d 段 ≈ %d KB，峰值 %d KB）：\n",
			nAlloc, nFail, nFree, held, held*16/1024, peak*16/1024)
		for i, m := range d.MemTrace {
			if len(d.MemTrace) > 60 && i == 30 {
				fmt.Printf("  …中間 %d 筆略過…\n", len(d.MemTrace)-60)
			}
			if len(d.MemTrace) > 60 && i >= 30 && i < len(d.MemTrace)-30 {
				continue
			}
			op, res := "配置", ""
			if m.Op == 0x49 {
				op = "釋放"
			}
			switch {
			case m.Op == 0x48 && m.OK:
				res = fmt.Sprintf("→ %04X（實得 %04X 段 ＝ %d KB）", m.Seg, m.Got, int(m.Got)*16/1024)
			case m.Op == 0x48:
				res = fmt.Sprintf("✗ 不足，最大自由 %04X 段 ＝ %d KB", m.Got, int(m.Got)*16/1024)
			case m.OK:
				res = fmt.Sprintf("ES=%04X（%04X 段）", m.Seg, m.Got)
			default:
				res = fmt.Sprintf("✗ ES=%04X 不是已配置的區塊", m.Seg)
			}
			fmt.Printf("  #%-10d %s want=%04X %-42s  呼叫端 %04X:%04X DS=%04X ES=%04X\n",
				m.Step, op, m.Want, res, m.CS, m.IP, m.DS, m.ES)
		}
	}
	fmt.Printf("\n配置器狀態：\n")
	for _, l := range d.ArenaDump() {
		fmt.Println("  " + l)
	}
	if len(d.CallTrace) > 0 {
		fmt.Printf("\nint 21h 的 ES:BX（最後 20 次；★ 表示服務動了 ES 或 BX）：\n")
		from := len(d.CallTrace) - 20
		if from < 0 {
			from = 0
		}
		for _, r := range d.CallTrace[from:] {
			mark := "  "
			if r.ESOut != r.ESIn || r.BXOut != r.BXIn {
				mark = "★ "
			}
			fmt.Printf("  %s#%-9d AH=%02X AL=%02X  ES:BX %04X:%04X → %04X:%04X\n",
				mark, r.Step, r.AH, r.AL, r.ESIn, r.BXIn, r.ESOut, r.BXOut)
		}
	}
	if len(d.Resizes) > 0 {
		fmt.Printf("\nAH=4Ah 調整區塊（%d 次）：\n", len(d.Resizes))
		for _, r := range d.Resizes {
			where := "arena 外（PSP／映像／記憶體探測）"
			if r.InArena {
				where = fmt.Sprintf("arena 內 %04X→%04X 段（%+d KB）",
					r.Before, r.After, (int(r.After)-int(r.Before))*16/1024)
			}
			st := "✗"
			if r.OK {
				st = "✓"
			}
			fmt.Printf("  ES=%04X want=%04X %s %-44s freeSeg=%04X  呼叫端 %04X:%04X\n",
				r.Seg, r.Want, st, where, r.FreeSeg, r.CS, r.IP)
		}
	}
	if len(d.Overlays) > 0 {
		fmt.Printf("\n載入的 overlay（%d）：\n", len(d.Overlays))
		for _, o := range d.Overlays {
			fmt.Printf("  %-14s → %04X:0  重定位加數 %04X  檔案 %d bytes\n",
				o.Name, o.Seg, o.Reloc, o.Size)
			fmt.Printf("      載入於第 %d 道指令\n", o.Steps)
			fmt.Printf("      參數區塊 %04X:%04X ＝ % X   呼叫端 %04X:%04X\n",
				o.PBSeg, o.PBOff, o.PBRaw, o.CallCS, o.CallIP)
			fmt.Printf("      呼叫端 %04X:%04X 前後 32 byte（INT 在 offset 22）：\n        % X\n",
				o.CallCS, o.CallIP-24, o.CallSite)
		}
	}
	fmt.Printf("\n滑鼠輪詢 %d 次", len(d.Mouse.Polls))
	if n := len(d.Mouse.Polls); n > 0 {
		f, l := d.Mouse.Polls[0], d.Mouse.Polls[n-1]
		fmt.Printf("（第一次 #%d 回報 (%d,%d)；最後一次 #%d 回報 (%d,%d)）",
			f.Step, f.X, f.Y, l.Step, l.X, l.Y)
	}
	fmt.Println()

	// 每個 int 33h 功能號被叫了幾次。**「點了沒反應」的第一步是先確認
	// 遊戲在讀哪一支**——讀 AX=3（即時狀態）與讀 AX=5/6（按下／放開的統計）
	// 要餵的東西不一樣。
	if len(d.Mouse.Calls) > 0 {
		fns := make([]int, 0, len(d.Mouse.Calls))
		for k := range d.Mouse.Calls {
			fns = append(fns, int(k))
		}
		sort.Ints(fns)
		fmt.Printf("int 33h 功能號：")
		for _, k := range fns {
			fmt.Printf(" %04X×%d", k, d.Mouse.Calls[uint16(k)])
		}
		fmt.Println()
	}

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
	fmt.Printf("埠寫入紀錄 %d 筆（約 %.1f MB）\n",
		len(m.PortLog), float64(len(m.PortLog))*16/1e6)
	fmt.Printf("寫過的 I/O 埠（%d）：", len(ports))
	for i, p := range ports {
		if i == 20 {
			fmt.Printf(" …另外 %d 個", len(ports)-20)
			break
		}
		fmt.Printf(" %03X", p)
	}
	fmt.Println()

	// 字型服務：**「沒畫字」與「畫了但看不見」的畫面一樣空**，
	// 所以要先問常式被叫了幾次。
	fmt.Printf("字型常式：全形 %d 次、半形 %d 次，讀不到字模 %d 次\n",
		d.Font.Calls[0], d.Font.Calls[1], d.Font.Missing)
	if len(d.Sound) > 0 {
		cmds := make([]int, 0, len(d.Sound))
		for k := range d.Sound {
			cmds = append(cmds, int(k))
		}
		sort.Ints(cmds)
		fmt.Printf("int 61h 音源 command：")
		for _, k := range cmds {
			fmt.Printf(" %02Xh×%d", k, d.Sound[uint8(k)])
		}
		fmt.Println()
	}
	if h := d.Mouse.Handler; h.Set {
		fmt.Printf("int 33h 事件常式 %04X:%04X（遮罩 %04X），送出 %d 次；"+
			"座標範圍 %d–%d × %d–%d\n",
			h.Seg, h.Off, h.Mask, d.Mouse.Events,
			d.Mouse.MinX, d.Mouse.MaxX, d.Mouse.MinY, d.Mouse.MaxY)
	}

	// 視訊記憶體：非零的點數。全 0 表示還沒畫任何東西。
	if w, h, px := m.Planar(); px != nil {
		nz := 0
		for _, v := range px {
			if v != 0 {
				nz++
			}
		}
		fmt.Printf("平面畫面 %d×%d，非零像素 %d / %d\n", w, h, nz, len(px))
	} else {
		nz := 0
		for _, v := range m.Indexed() {
			if v != 0 {
				nz++
			}
		}
		fmt.Printf("A0000 非零像素 %d / %d\n", nz, machine.VideoWidth*machine.VideoHigh)
	}

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

// writeScreen 把平面模式的畫面存成 PNG。
//
// ⚠ **裁切是呼叫端的事，不是機器的事。** 臥龍傳的內容是 640×400，
// 但它跑在 640×480 的 mode 12h 上、y 原點在第 40 列
// （`docs/spec/007` §2）。把裁切寫進機器層會讓「畫面是 480 高」
// 這個事實消失，之後查「上面那 40 列有沒有東西」就沒得查了。
func writeScreen(m *machine.Machine, path string, top, height int) error {
	w, h, px := m.Planar()
	if px == nil {
		return fmt.Errorf("現在不是平面模式（視訊模式 %02Xh），沒有畫面可存",
			m.VideoMode())
	}
	if height == 0 {
		height = h - top
	}
	if top < 0 || height <= 0 || top+height > h {
		return fmt.Errorf("裁切範圍 %d..%d 超出畫面高 %d", top, top+height, h)
	}
	pal := make(color.Palette, 256)
	dac := m.Palette()
	for i := range dac {
		pal[i] = color.RGBA{dac[i][0], dac[i][1], dac[i][2], 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, w, height), pal)
	for y := 0; y < height; y++ {
		for x := 0; x < w; x++ {
			img.Pix[y*img.Stride+x] = m.VGA.DACIndex(px[(y+top)*w+x])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	fmt.Printf("\n寫出 %s（%d×%d，y 偏移 %d）\n", path, w, height, top)
	return nil
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
		c := dac[m.VGA.DACIndex(uint8(i))]
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

// doDumpMem 把 <seg>:<off>:<長度> 的記憶體寫成檔。
//
// **為什麼不用 -peek**：-peek 吃的是 IDA 線性位址，要靠映像基底換算，
// 而執行期搬到別的段的程式碼（overlay 管理員的 thunk、載進來的 overlay）
// **在映像裡根本不存在**，換算不到。要看它們只能直接給段:位移。
func doDumpMem(m *machine.Machine, spec string) error {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("格式是 <seg>:<off>:<len>[,...]=<檔名前綴>")
	}
	prefix := parts[1]
	for _, one := range strings.Split(parts[0], ",") {
		f := strings.Split(one, ":")
		if len(f) != 3 {
			return fmt.Errorf("%q 不是 <seg>:<off>:<len>", one)
		}
		seg, err := strconv.ParseUint(strings.TrimPrefix(f[0], "0x"), 16, 16)
		if err != nil {
			return fmt.Errorf("段 %q：%w", f[0], err)
		}
		off, err := strconv.ParseUint(strings.TrimPrefix(f[1], "0x"), 16, 16)
		if err != nil {
			return fmt.Errorf("位移 %q：%w", f[1], err)
		}
		n, err := strconv.Atoi(f[2])
		if err != nil || n <= 0 {
			return fmt.Errorf("長度 %q 不是正整數", f[2])
		}
		buf := make([]byte, n)
		base := cpu.Addr(uint16(seg), uint16(off))
		for i := range buf {
			buf[i] = m.Read8(base + uint32(i))
		}
		name := fmt.Sprintf("%s-%04X_%04X.bin", prefix, seg, off)
		if err := os.WriteFile(name, buf, 0o644); err != nil {
			return err
		}
		fmt.Printf("dump %04X:%04X %d bytes → %s\n", seg, off, n, name)
	}
	return nil
}

// egaPalette 是 EGA 的 16 色預設調色盤（6 位元 RGB 展開成 8 位元）。
//
// **這不是遊戲的調色盤**——遊戲會自己設 EGA 的 palette 暫存器。
// 這裡用預設值只為了讓 dump 出來的圖看得懂；要對拍顏色得另外把
// 那些暫存器的寫入攔下來。
var egaPalette = [16][3]uint8{
	{0, 0, 0}, {0, 0, 170}, {0, 170, 0}, {0, 170, 170},
	{170, 0, 0}, {170, 0, 170}, {170, 85, 0}, {170, 170, 170},
	{85, 85, 85}, {85, 85, 255}, {85, 255, 85}, {85, 255, 255},
	{255, 85, 85}, {255, 85, 255}, {255, 255, 85}, {255, 255, 255},
}

// doDumpEGA 把平面式 VRAM 依指定尺寸組成 PNG。
func doDumpEGA(m *machine.Machine, spec string) error {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("格式是 <寬>x<高>=<檔名>")
	}
	var w, h int
	if _, err := fmt.Sscanf(parts[0], "%dx%d", &w, &h); err != nil {
		return fmt.Errorf("尺寸 %q 解不出來：%w", parts[0], err)
	}
	idx := m.IndexedEGASize(w, h)
	if idx == nil {
		return fmt.Errorf("%dx%d 放不進平面（寬要是 8 的倍數）", w, h)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	nonZero := 0
	for i, v := range idx {
		c := egaPalette[v&0x0F]
		img.SetRGBA(i%w, i/w, color.RGBA{c[0], c[1], c[2], 255})
		if v != 0 {
			nonZero++
		}
	}
	f, err := os.Create(parts[1])
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	fmt.Printf("EGA %dx%d → %s（%d/%d 個非零像素，Map Mask 曾動過＝%v）\n",
		w, h, parts[1], nonZero, len(idx), m.EGAPlanarActive())
	return nil
}

// timedKey 是一次定時餵鍵。
type timedKey struct {
	at  uint64
	key []byte
}

// parseKeysAt 解析 `<步數>:<鍵>[,…]`，並依步數排序。
//
// **為什麼需要它**：`-keys` 是開場就把鍵塞進佇列，早期的提示會把它們
// 吃光。後面才出現的「按任意鍵繼續」（智冠《三國演義》的開場插圖用
// `int 16h AH=01` 輪詢了三百三十萬次）拿不到任何鍵，於是永遠停在那裡——
// 從外面看是「程式還活著」。
func parseKeysAt(spec string) ([]timedKey, error) {
	if spec == "" {
		return nil, nil
	}
	var out []timedKey
	for _, one := range strings.Split(spec, ",") {
		i := strings.Index(one, ":")
		if i < 0 {
			return nil, fmt.Errorf("keys-at: %q 不是 <步數>:<鍵>", one)
		}
		at, err := strconv.ParseUint(one[:i], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("keys-at: 步數 %q：%w", one[:i], err)
		}
		k := strings.ReplaceAll(one[i+1:], "\\n", "\n")
		k = strings.ReplaceAll(k, "\\r", "\r")
		if k == "" {
			return nil, fmt.Errorf("keys-at: %q 沒有指定按鍵", one)
		}
		out = append(out, timedKey{at: at, key: []byte(k)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out, nil
}

// writeCoverage 把執行過的位址壓成連續區段寫出去。
//
// 區段而不是逐個位址：一段連續的程式碼會產生幾千個相鄰位址，
// 列成區段之後種子只要每段一個起點。
func writeCoverage(m *machine.Machine, path string) error {
	type span struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Len   int    `json:"len"`
	}
	var spans []span
	i := 0
	total := 0
	for i < len(m.Coverage) {
		if !m.Coverage[i] {
			i++
			continue
		}
		j := i
		for j < len(m.Coverage) && m.Coverage[j] {
			j++
		}
		spans = append(spans, span{Start: fmt.Sprintf("0x%05X", i),
			End: fmt.Sprintf("0x%05X", j), Len: j - i})
		total += j - i
		i = j
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"executed_bytes": total,
		"span_count":     len(spans),
		"spans":          spans,
	}); err != nil {
		return err
	}
	fmt.Printf("覆蓋率：%d 個位址被執行過，壓成 %d 段 → %s\n", total, len(spans), path)
	return nil
}

// feedKeys 同時餵**兩條路**：DOS／BIOS 的字元佇列，與硬體鍵盤的掃描碼。
//
// ⚠ **只餵其中一條會得到「程式沒反應」而不是錯誤。** 自己裝 IRQ1
// 處理常式、直接讀埠 0x60 的程式看不到字元佇列；用 `int 21h AH=08`
// 的程式看不到掃描碼。哪一條才對是程式決定的，而它不會告訴你。
func feedKeys(m *machine.Machine, d *dos.DOS, keys []byte) {
	d.Stdin = append(d.Stdin, keys...)
	for _, b := range keys {
		if sc, ok := dos.ScanCode(b); ok {
			m.PushKey(sc)
		}
	}
}

// writePortLog 把 I/O 寫入序列存成 TSV。
//
// 規格是 `<檔名>`（全部）或 `<埠>,<埠>=<檔名>`（只存那幾個埠）。
// 埠用十六進位，例如 `388,389=opl.tsv`。
//
// **要序列不要最後值**：像 OPL2 這種「先選暫存器再寫值」的介面，
// 只看每個埠最後寫進去的值什麼都看不出來。
func writePortLog(m *machine.Machine, spec string) error {
	want := map[uint16]bool{}
	path := spec
	if i := strings.LastIndex(spec, "="); i >= 0 {
		path = spec[i+1:]
		for _, p := range strings.Split(spec[:i], ",") {
			var v uint64
			if _, err := fmt.Sscanf(strings.TrimSpace(p), "%x", &v); err != nil {
				return fmt.Errorf("-dump-ports 的埠 %q 解不出來", p)
			}
			want[uint16(v)] = true
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, "step\tport\tvalue"); err != nil {
		return err
	}
	n := 0
	for _, w := range m.PortLog {
		if len(want) > 0 && !want[w.Port] {
			continue
		}
		if _, err := fmt.Fprintf(f, "%d\t%03X\t%02X\n", w.Step, w.Port, w.Val); err != nil {
			return err
		}
		n++
	}
	fmt.Printf("I/O 寫入 %d 筆 → %s\n", n, path)
	return nil
}

// biosKeyOf 把一個字元換成 BIOS 鍵盤緩衝區的一筆。
//
// 掃描碼查得到就填，查不到只填 ASCII（掃描碼 0）。**填錯比留 0 糟**：
// 0 是「不是從鍵盤來的」慣例值，程式頂多忽略那一筆；填一個錯的掃描碼會被
// 讀成另一個鍵，而畫面上看起來就只是「按錯了」。
func biosKeyOf(r rune) dos.Key {
	if k, ok := dos.KeyForRune(r); ok {
		return k
	}
	switch r {
	case '\n', '\r':
		return dos.Key{Scan: 0x1C, ASCII: '\r'}
	case 0x1B:
		return dos.Key{Scan: 0x01, ASCII: 0x1B}
	case '\b':
		return dos.Key{Scan: 0x0E, ASCII: '\b'}
	}
	return dos.Key{ASCII: uint8(r)}
}

// writeSpeakerWAV 把喇叭的波形寫成 WAV。
//
// 波形是**一個位元**：兩個位準寫成 0x20／0xE0 而不是 0x00／0xFF，
// 滿幅的方波在多數播放器上會削波，聽起來像壞掉。
func writeSpeakerWAV(m *machine.Machine, path string) error {
	s := m.Speaker
	if len(s) == 0 {
		return fmt.Errorf("喇叭一次都沒動過——這一段沒有聲音")
	}
	const rate = 11025
	sps := machine.StepsPerSecond()
	first, last := s[0].Step, s[len(s)-1].Step
	n := int(float64(last-first) / sps * rate)
	if n <= 0 {
		return fmt.Errorf("波形只有 %d 道指令長，不足一個取樣", last-first)
	}
	pcm := make([]uint8, n)
	j := 0
	for i := range pcm {
		step := first + uint64(float64(i)/rate*sps)
		for j+1 < len(s) && s[j+1].Step <= step {
			j++
		}
		pcm[i] = 0x20
		if s[j].Level != 0 {
			pcm[i] = 0xE0
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var h []byte
	put32 := func(v uint32) { h = binary.LittleEndian.AppendUint32(h, v) }
	put16 := func(v uint16) { h = binary.LittleEndian.AppendUint16(h, v) }
	h = append(h, "RIFF"...)
	put32(uint32(36 + len(pcm)))
	h = append(h, "WAVEfmt "...)
	put32(16)
	put16(1)
	put16(1)
	put32(rate)
	put32(rate)
	put16(1)
	put16(8)
	h = append(h, "data"...)
	put32(uint32(len(pcm)))
	if _, err := f.Write(h); err != nil {
		return err
	}
	_, err = f.Write(pcm)
	return err
}
