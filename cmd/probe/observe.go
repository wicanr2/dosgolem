package main

// 觀測用的附加旗標：`-dump-ems`／`-opl-log`／`-poke-file`／`-watch-file`／
// `-xms-file`／`-read-watch*`／`-cpuhz`。
//
// **刻意放在獨立檔案。** 這幾支是 yuan（源平合戰）那條線加出來的，
// 與 main.go 的主流程只有四個掛鉤點；混進 main.go 之後，上游每改一次
// 旗標區就要重解一次衝突。分開放的話合併只會動到那四行。

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

var (
	obsCPUHz = flag.Uint64("cpuhz", 0,
		"模擬的 CPU 時脈（Hz），**設了就改走週期時鐘**：一道指令算一格會把繪圖算得太便宜。"+
			"這個值就是「在假裝哪一台機器」，與 DOSBox 的 cycles 同一顆旋鈕（0 ＝ 不改，維持指令數時鐘）")
	obsDumpEMS = flag.String("dump-ems", "",
		"跑完把每一個 EMS handle 的每一頁寫成 `<目錄>/ems-<handle>-<頁>.bin`，"+
			"並印出頁數與 page frame 現在映著誰。page frame 只看得到此刻那四頁，"+
			"要在整份 EMS 裡找東西得用這個")
	obsOPLLog = flag.String("opl-log", "",
		"把 OPL2（AdLib）暫存器寫入序列寫到這個檔：每行 `步數 暫存器 值`（十六進位）。"+
			"音訊 parity 對的是這串「樂譜」，不是波形（`docs/spec/004` §6）")
	obsDumpVGM = flag.String("dump-vgm", "",
		"把 OPL（AdLib／OPL3）暫存器寫入序列倒成 VGM，外部播放器可以離線合成"+
			"（`docs/spec/190-opl-vgm-dump`）。時間基準取機器自己的 `IRQ0Every × PITHz`。"+
			"⚠ **要配 `-adlib`**：AdLib 預設不存在，偵測失敗的程式整段跳過音樂路徑，"+
			"而空序列與「這個程式沒有音樂」長得一模一樣")
	obsVGMClearOn = flag.String("vgm-clear-on-open", "",
		"開到這個檔的時候把 OPL 寫入序列清掉（不分大小寫，不含路徑）。"+
			"**這是把一段 OPL 輸出框到某一首曲子上唯一的錨**：開機會連放好幾首而且"+
			"暫存器串是接起來的，事後從序列裡找分界只能用猜的。"+
			"⚠ 給了卻沒開到那個檔就**不寫 VGM**——不然錄到的是別首，"+
			"而它照樣產出一份聽起來完全正常的音檔")
	obsVGMStepsPerSec = flag.Float64("vgm-steps-per-second", 0,
		"蓋掉 -dump-vgm 的時間基準（指令／秒）。0 ＝ 用機器現在的設定。"+
			"程式中途改了 PIT 分頻**而且**間隔被 -tick 釘死時，機器速度變過而"+
			"這裡只取一個快照——那種情況自己算一個傳進來")
	obsPokeFile = flag.String("poke-file", "",
		"從檔案讀 -poke 的腳本（一行一筆或整串分號分隔）。"+
			"命令列單一參數上限 128 KB，塗整份圖形這種大量 poke 放檔案裡")
	obsWatchFile = flag.String("watch-file", "",
		"把 -watch 留下的寫入**全部**寫到這個檔（畫面上只列得下最後幾筆）")
	obsXMSFile = flag.String("xms-file", "",
		"把每一次 XMS move **全部**寫到這個檔（畫面上只列 30 筆）")
	obsBCRing = flag.String("bc-ring", "",
		"位元碼追蹤：執行到 `<seg>:<off>` 時記下 `DS:SI`（直譯器的位元碼 PC），"+
			"跑完印出最後 N 筆相異值（格式 `<seg>:<off>[:N]`，N 預設 40）。"+
			"回答「剛剛那句話是哪一支腳本印的」——"+
			"監看訊息緩衝只看得到最內層那支複製位元組的小函式，看不到誰叫它")
	obsBCSeen = flag.String("bc-seen", "",
		"位元碼追蹤（集合版）：`<seg>:<off>:<起>-<迄>`，在 [起,迄) 這段步數裡"+
			"把執行到 `<seg>:<off>` 時的 `DS:SI` **全部**收成集合，跑完照第一次"+
			"看到的順序印出來。`-bc-ring` 只留最後 N 筆，會被最內圈的緊迴圈灌滿；"+
			"要看「這段時間有哪幾支腳本跑過」用這個")
	obsBCSub = flag.String("bc-sub", "",
		"位元碼追蹤（副程式版）：`<seg>:<off>:<起>-<迄>[:N]`，在 [起,迄) 這段步數裡"+
			"每次執行到 `<seg>:<off>` 就看 `BX`——直譯器跑一支位元碼副程式時，"+
			"`BX` 是那一支的**對照表位址**（`xlat` 用的就是它），"+
			"所以 `BX` 變了就是換了一支。輸出是換手的時間軸（最多 N 筆，預設 2000）。"+
			"`-bc-seen` 回答「跑過哪些位元碼位址」，這個回答「那些位址屬於誰、誰叫誰」")
	obsPollLog = flag.Int("poll-log", 0,
		"印出遊戲**按鍵不為零時**讀到的滑鼠輪詢（最多 N 筆，0 ＝ 不印）。"+
			"回答「這一次點擊遊戲到底看到了什麼」——"+
			"腳本點擊沒反應時，畫面上分不出三件事："+
			"座標送錯了、按住的期間遊戲一次都沒問、"+
			"還是問到了但那個位置沒有東西可點")
	obsReadWatch = flag.String("read-watch", "",
		"監看一段線性位址的**讀取**：`<lo>-<hi>`（十六進位）。配 -read-watch-grain 與 -read-watch-file 用")
	obsReadGrain = flag.Int("read-watch-grain", 1,
		"讀取監看的粒度：位址除以它之後**值變了才記一筆**。"+
			"素材是一格一格排的時候，設成一格的大小就會直接印出「用了第幾格」")
	obsReadFile = flag.String("read-watch-file", "",
		"讀取監看的紀錄寫到這個檔（一行：步數 位址 格號 CS:IP）")
)

// obsRead 是讀取監看攔到的一次讀。
type obsRead struct {
	step   uint64
	addr   uint32
	cell   int
	cs, ip uint16
}

var obsReads []obsRead

// 位元碼追蹤的狀態。`-bc-ring` 沒給時 bcOn 是 false，obsStep 只多一個比較。
var (
	bcOn         bool
	bcSeg, bcOff uint16
	bcKeep       int
	bcRing       []uint32
	bcLast       uint32

	bcSeenOn    bool
	bcSeenSeg   uint16
	bcSeenOff   uint16
	bcSeenFrom  uint64
	bcSeenTo    uint64
	bcSeenSet   map[uint32]uint64
	bcSeenOrder []uint32

	bcSubOn   bool
	bcSubSeg  uint16
	bcSubOff  uint16
	bcSubFrom uint64
	bcSubTo   uint64
	bcSubKeep int
	bcSubLast uint32
	bcSubLog  []bcSubHit
)

// bcSubHit 是一次「換了一支位元碼副程式」。
type bcSubHit struct {
	step   uint64
	ds, bx uint16 // 對照表位址：ds:bx
	si     uint16 // 換手當下的位元碼 PC
}

// obsStep 每一道指令之前叫一次。掛鉤點四。
//
// **這裡要便宜。** 主迴圈一秒跑上千萬次，多一個 map 查詢就會讓
// 十億道的觀測從一分鐘變成十分鐘。
func obsStep(m *machine.Machine) {
	if bcSeenOn && m.Steps >= bcSeenFrom && m.Steps < bcSeenTo {
		if cs, ip := m.CPU.Op(); cs == bcSeenSeg && ip == bcSeenOff {
			v := uint32(m.CPU.Seg[cpu.DS])<<16 | uint32(m.CPU.R[cpu.SI])
			if _, ok := bcSeenSet[v]; !ok {
				bcSeenSet[v] = m.Steps
				bcSeenOrder = append(bcSeenOrder, v)
			}
		}
	}
	if bcSubOn && m.Steps >= bcSubFrom && m.Steps < bcSubTo {
		if cs, ip := m.CPU.Op(); cs == bcSubSeg && ip == bcSubOff {
			ds, bx := m.CPU.Seg[cpu.DS], m.CPU.R[cpu.BX]
			v := uint32(ds)<<16 | uint32(bx)
			if v != bcSubLast {
				bcSubLast = v
				if len(bcSubLog) < bcSubKeep {
					bcSubLog = append(bcSubLog,
						bcSubHit{step: m.Steps, ds: ds, bx: bx, si: m.CPU.R[cpu.SI]})
				}
			}
		}
	}
	if !bcOn {
		return
	}
	cs, ip := m.CPU.Op()
	if cs != bcSeg || ip != bcOff {
		return
	}
	v := uint32(m.CPU.Seg[cpu.DS])<<16 | uint32(m.CPU.R[cpu.SI])
	if v == bcLast {
		return // 同一個 PC 連續命中不記第二筆
	}
	bcLast = v
	bcRing = append(bcRing, v)
	if len(bcRing) > bcKeep {
		bcRing = bcRing[len(bcRing)-bcKeep:]
	}
}

// obsSetup 在機器造好、還沒開跑之前掛上。掛鉤點一。
func obsSetup(m *machine.Machine) {
	if *obsBCSeen != "" {
		f := strings.Split(*obsBCSeen, ":")
		if len(f) != 3 {
			die(fmt.Errorf("-bc-seen 要 <seg>:<off>:<起>-<迄>，收到 %q", *obsBCSeen))
		}
		seg, err1 := strconv.ParseUint(f[0], 16, 16)
		off, err2 := strconv.ParseUint(f[1], 16, 16)
		var from, to uint64
		_, err3 := fmt.Sscanf(f[2], "%d-%d", &from, &to)
		if err1 != nil || err2 != nil || err3 != nil || to <= from {
			die(fmt.Errorf("-bc-seen 讀不出來：%q", *obsBCSeen))
		}
		bcSeenSeg, bcSeenOff = uint16(seg), uint16(off)
		bcSeenFrom, bcSeenTo = from, to
		bcSeenSet = map[uint32]uint64{}
		bcSeenOn = true
	}
	if *obsBCSub != "" {
		f := strings.Split(*obsBCSub, ":")
		if len(f) != 3 && len(f) != 4 {
			die(fmt.Errorf("-bc-sub 要 <seg>:<off>:<起>-<迄>[:N]，收到 %q", *obsBCSub))
		}
		seg, err1 := strconv.ParseUint(f[0], 16, 16)
		off, err2 := strconv.ParseUint(f[1], 16, 16)
		var from, to uint64
		_, err3 := fmt.Sscanf(f[2], "%d-%d", &from, &to)
		if err1 != nil || err2 != nil || err3 != nil || to <= from {
			die(fmt.Errorf("-bc-sub 讀不出來：%q", *obsBCSub))
		}
		bcSubKeep = 2000
		if len(f) == 4 {
			if n, err := strconv.Atoi(f[3]); err == nil && n > 0 {
				bcSubKeep = n
			}
		}
		bcSubSeg, bcSubOff = uint16(seg), uint16(off)
		bcSubFrom, bcSubTo = from, to
		bcSubOn = true
	}
	if *obsBCRing != "" {
		f := strings.Split(*obsBCRing, ":")
		if len(f) < 2 {
			die(fmt.Errorf("-bc-ring 要 <seg>:<off>[:N]，收到 %q", *obsBCRing))
		}
		var seg, off uint64
		var err1, err2 error
		seg, err1 = strconv.ParseUint(f[0], 16, 16)
		off, err2 = strconv.ParseUint(f[1], 16, 16)
		if err1 != nil || err2 != nil {
			die(fmt.Errorf("-bc-ring 的位址讀不出來：%q", *obsBCRing))
		}
		bcKeep = 40
		if len(f) > 2 {
			if n, err := strconv.Atoi(f[2]); err == nil && n > 0 {
				bcKeep = n
			}
		}
		bcSeg, bcOff, bcOn = uint16(seg), uint16(off), true
	}
	if *obsCPUHz > 0 {
		m.CPUHz = *obsCPUHz
		m.CycleClock = true
		m.RecalcIRQ0()
	}
	if *obsReadWatch == "" {
		return
	}
	var lo, hi uint32
	if _, err := fmt.Sscanf(*obsReadWatch, "%x-%x", &lo, &hi); err != nil {
		die(err)
	}
	grain := *obsReadGrain
	if grain < 1 {
		grain = 1
	}
	last := -1
	m.WatchReads(lo, hi, func(a uint32, _ uint8) {
		cell := int(a-lo) / grain
		// **同一格連讀不記第二筆。** 一張圖被讀幾千次，每一次都記
		// 會把「換到哪一格」淹掉——要看的是切換，不是次數。
		if cell == last {
			return
		}
		last = cell
		if len(obsReads) < 200000 {
			obsReads = append(obsReads,
				obsRead{m.Steps, a, cell, m.CPU.Seg[cpu.CS], m.CPU.IP})
		}
	})
}

// obsPokeScript 把 -poke-file 的內容併進 -poke 的腳本。掛鉤點二。
func obsPokeScript(inline string) string {
	if *obsPokeFile == "" {
		return inline
	}
	b, err := os.ReadFile(*obsPokeFile)
	if err != nil {
		die(err)
	}
	var parts []string
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" && !strings.HasPrefix(ln, "#") {
			parts = append(parts, ln)
		}
	}
	if inline != "" {
		parts = append(parts, inline)
	}
	return strings.Join(parts, ";")
}

// obsReport 跑完之後落檔。掛鉤點三。`writes` 是 -watch 攔到的那一串，
// 型別由呼叫端給（main 裡是區域型別），所以用一個小介面接。
func obsReport(m *machine.Machine, d *dos.DOS, watch func(w *bufio.Writer)) {
	// PIT 的分頻決定計時器多快。**印出來**：分頻被寫成一個小數字時，
	// 症狀是機器淹在中斷裡出不來，而那看起來像「程式當掉」。
	fmt.Printf("\nPIT 通道 0 分頻 %d", m.PITDiv)
	if m.CycleClock {
		fmt.Printf("（週期時鐘 %d Hz，每 %d 個週期一次 IRQ0）", m.CPUHz, m.CycPerIRQ0())
	}
	fmt.Println()
	obsReportPolls(m, d)
	if bcSeenOn {
		fmt.Printf("位元碼 PC（%04X:%04X，步數 %d–%d，共 %d 個相異值，照第一次看到的順序）：\n",
			bcSeenSeg, bcSeenOff, bcSeenFrom, bcSeenTo, len(bcSeenOrder))
		for _, v := range bcSeenOrder {
			fmt.Printf("  %04X:%04X  #%d\n", v>>16, v&0xFFFF, bcSeenSet[v])
		}
	}
	if bcSubOn {
		fmt.Printf("位元碼副程式換手（%04X:%04X 上的 BX，步數 %d–%d，共 %d 筆）：\n",
			bcSubSeg, bcSubOff, bcSubFrom, bcSubTo, len(bcSubLog))
		for _, h := range bcSubLog {
			fmt.Printf("  #%d  對照表 %04X:%04X  PC %04X:%04X\n",
				h.step, h.ds, h.bx, h.ds, h.si)
		}
	}
	if bcOn {
		fmt.Printf("位元碼 PC（%04X:%04X 上最後 %d 筆相異值，舊 → 新）：\n",
			bcSeg, bcOff, len(bcRing))
		for _, v := range bcRing {
			fmt.Printf("  %04X:%04X\n", v>>16, v&0xFFFF)
		}
	}
	if *obsOPLLog != "" {
		if err := obsWrite(*obsOPLLog, func(w *bufio.Writer) {
			fmt.Fprintln(w, "# dosgolem OPL2 log：步數 暫存器 值")
			for _, o := range m.OPL {
				fmt.Fprintf(w, "%d %02x %02x\n", o.Step, o.Reg, o.Val)
			}
		}); err != nil {
			fmt.Println("opl-log 寫檔失敗:", err)
		} else {
			fmt.Printf("\nOPL2 暫存器序列寫到 %s（%d 筆）\n", *obsOPLLog, len(m.OPL))
		}
	}
	if *obsDumpVGM != "" && *obsVGMClearOn != "" && !obsVGMCleared {
		// **沒命中就不要寫。** 錄到的會是別首，而它照樣產出一份聽起來
		// 完全正常的音檔，檔名卻寫著這一首——那種錯沒有人會發現。
		fmt.Printf("\ndump-vgm 跳過：整段執行沒有開過 %q，框不出這一首\n", *obsVGMClearOn)
	} else if *obsDumpVGM != "" {
		f, err := os.Create(*obsDumpVGM)
		if err != nil {
			fmt.Println("dump-vgm 建檔失敗:", err)
		} else {
			// 印的要是**真正用到的**那個基準，不是機器現在的——旗標蓋掉之後
			// 兩者不同，而印錯的那一行會讓人以為蓋掉沒生效。
			base := *obsVGMStepsPerSec
			if base <= 0 {
				base = m.StepsPerSecondNow()
			}
			st, err := m.OPLVGM(f, base)
			f.Close()
			if err != nil {
				fmt.Println("dump-vgm:", err)
			} else {
				// **秒數要印出來。** 「錄到 0 筆」與「錄到 3 萬筆」看檔案大小
				// 就分得出來；「曲速差四倍」只有這個數字看得出來。
				fmt.Printf("\nVGM → %s（%s，%d 筆寫入，%.1f 秒，時間基準 %.0f 指令／秒）\n",
					*obsDumpVGM, st.Chip, st.Writes, st.Seconds, base)
			}
		}
	}
	if *obsWatchFile != "" && watch != nil {
		if err := obsWrite(*obsWatchFile, watch); err != nil {
			fmt.Println("watch-file 寫檔失敗:", err)
		} else {
			fmt.Printf("\n監看紀錄寫到 %s\n", *obsWatchFile)
		}
	}
	if *obsReadFile != "" {
		if err := obsWrite(*obsReadFile, func(w *bufio.Writer) {
			for _, r := range obsReads {
				fmt.Fprintf(w, "%d %05x %d %04x:%04x\n", r.step, r.addr, r.cell, r.cs, r.ip)
			}
		}); err != nil {
			fmt.Println("read-watch-file 寫檔失敗:", err)
		} else {
			fmt.Printf("\n讀取監看寫到 %s（%d 筆）\n", *obsReadFile, len(obsReads))
		}
	}
	if *obsXMSFile != "" {
		if err := obsWrite(*obsXMSFile, func(w *bufio.Writer) {
			fmt.Fprintln(w, "# dosgolem XMS move：步數 長度 來源handle:位移 目的handle:位移 位元數")
			for _, x := range d.XMSMoves {
				fmt.Fprintf(w, "%d %d %04x:%08x %04x:%08x %d\n",
					x.Step, x.Len, x.SrcH, x.SrcOff, x.DstH, x.DstOff, x.Bits)
			}
		}); err != nil {
			fmt.Println("xms-file 寫檔失敗:", err)
		} else {
			fmt.Printf("\nXMS move 寫到 %s（%d 筆）\n", *obsXMSFile, len(d.XMSMoves))
		}
	}
	if *obsDumpEMS != "" {
		if err := d.DumpEMS(*obsDumpEMS); err != nil {
			fmt.Println("dump-ems 寫檔失敗:", err)
		} else {
			fmt.Printf("\nEMS 攤到 %s\n%s", *obsDumpEMS, d.EMSLayout())
		}
	}
}

// obsVGMCleared 記 -vgm-clear-on-open 有沒有真的命中。
var obsVGMCleared bool

// obsSetupDOS 是掛鉤點四：要 DOS 那一層才掛得上的觀測。
//
// `OnOpen` 是「把一段執行期產物框到某個檔上」唯一的錨（`oracle.OnFileOpen`
// 的註解講的是同一件事）。這裡用它把 OPL 的寫入序列切在曲檔被開啟的那一刻。
func obsSetupDOS(m *machine.Machine, d *dos.DOS) {
	if *obsVGMClearOn == "" {
		return
	}
	want := strings.ToLower(*obsVGMClearOn)
	prev := d.OnOpen
	d.OnOpen = func(name string) {
		if prev != nil {
			prev(name)
		}
		if strings.ToLower(name) == want && !obsVGMCleared {
			// **只清序列，不清暫存器檔**：驅動開機時把預設音色灌進全部
			// 18 個 operator，之後的曲子是疊在那個狀態上的。
			//
			// **只切第一次。** 同一首可能被開第二次（logh3 的開頭動畫會
			// 重播一輪），每次都切的話留下的是**最後那一小段**——
			// 而那一段照樣合法、照樣播得出來，只是短得莫名其妙。
			// 「這一首從哪裡開始」的答案是第一次載入，不是最後一次。
			m.ClearOPL()
			obsVGMCleared = true
		}
	}
}

func obsWrite(path string, body func(*bufio.Writer)) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	body(w)
	return w.Flush()
}

// obsReportPolls 印出按鍵不為零的滑鼠輪詢。
//
// 腳本點擊「沒反應」時畫面上看不出是哪一種失敗：座標送錯、按住的期間
// 遊戲一次都沒問（輪詢率在不同畫面差三個數量級）、或是問到了但那裡
// 沒有可點的東西。前兩種在這份清單裡立刻分得開——沒有任何一筆就是
// 遊戲沒看到，有筆但座標不對就是送錯了。
func obsReportPolls(m *machine.Machine, d *dos.DOS) {
	if *obsPollLog <= 0 {
		return
	}
	// 座標倍率也要印。`int 33h AX=3` 回的 CX 是 `X × 倍率`，倍率由視訊
	// 模式決定；倍率錯了遊戲收到的就是別的位置，而畫面上只會看到
	// 「點了沒反應」。
	scale := int(d.Mouse.XScale)
	how := "-xscale 指定"
	if scale == 0 {
		how = "依視訊模式自動決定"
		if w := m.PixelWidth(); w > 0 {
			scale = 640 / w
		} else {
			scale = 2
		}
	}
	fmt.Printf("視訊模式 %02Xh，畫面寬 %d，滑鼠水平倍率 %d（%s；回報的 CX ＝ X × 倍率）\n",
		m.VideoMode(), m.PixelWidth(), scale, how)
	n := 0
	for _, p := range d.Mouse.Polls {
		if p.Buttons == 0 {
			continue
		}
		n++
		if n <= *obsPollLog {
			fmt.Printf("[輪詢] #%d 遊戲讀到 CX=%d DX=%d 鍵=%02X（注入的是 %d,%d）\n",
				p.Step, int(p.X)*scale, p.Y, p.Buttons, p.X, p.Y)
		}
	}
	fmt.Printf("滑鼠輪詢裡按鍵不為零的共 %d 筆（總共 %d 筆）\n", n, len(d.Mouse.Polls))
	if n == 0 {
		fmt.Println("  ⚠ 一筆都沒有：按住的期間遊戲沒有讀滑鼠，或按鍵根本沒送出去")
	}
}
