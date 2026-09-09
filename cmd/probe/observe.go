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
)

// obsStep 每一道指令之前叫一次。掛鉤點四。
//
// **這裡要便宜。** 主迴圈一秒跑上千萬次，多一個 map 查詢就會讓
// 十億道的觀測從一分鐘變成十分鐘。
func obsStep(m *machine.Machine) {
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
