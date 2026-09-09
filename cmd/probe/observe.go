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

// obsSetup 在機器造好、還沒開跑之前掛上。掛鉤點一。
func obsSetup(m *machine.Machine) {
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
