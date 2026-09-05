// psys 把 DOS 版 UCSD p-System 的載入器（`PSYSTEM.COM`）跑起來，
// 看它走到哪裡、缺哪些服務。
//
// 這是探路用的，不是對外的 API。目標是把 p-machine 直譯器
// （`SYSTEM.PME.86`）帶到可以當 oracle 的狀態。
//
//	tools/go.sh run ./cmd/psys -com /orig/PSYSTEM.COM -root /orig -steps 20000000
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	com := flag.String("com", "", "PSYSTEM.COM 的路徑")
	root := flag.String("root", "", "磁碟映像所在目錄（會當成目前目錄）")
	steps := flag.Uint64("steps", 50_000_000, "最多跑幾條指令")
	args := flag.String("args", "", "命令列參數，寫進 PSP")
	pme := flag.String("pme", "", "SYSTEM.PME.86 的路徑；在記憶體裡找它載到哪")
	trace := flag.Int("trace", 0, "找到 PME 之後，再記錄幾條 p-code")
	keys := flag.String("keys", "", "排進 BIOS 鍵盤佇列的字元")
	flag.Parse()
	if *com == "" || *root == "" {
		flag.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(*com)
	if err != nil {
		die(err)
	}
	m := machine.New()
	if err := m.LoadCOM(data); err != nil {
		die(err)
	}
	d := dos.New(m, *root)
	d.Dir = ""
	d.Install()
	if *args != "" {
		setCmdline(m, *args)
	}
	if *keys != "" {
		d.TypeKeys(*keys)
		if err := m.TypeScan(*keys); err != nil {
			die(err)
		}
	}

	var runErr error
	for m.Steps < *steps && !d.Exited {
		if runErr = m.Step(); runErr != nil {
			break
		}
	}

	if *pme != "" && *trace > 0 && runErr == nil {
		if img, err := os.ReadFile(*pme); err == nil {
			if base, ok := findPME(m, img); ok {
				tracePCode(m, d, base, *trace, *steps)
			}
		}
	}

	c := m.CPU
	fmt.Printf("跑了 %d 條指令，停在 %04X:%04X；int 16h 被問了 %d 次；鍵盤中斷送了 %d 次，掃描碼還剩 %d 個\n",
		m.Steps, c.Seg[cpu.CS], c.IP, d.KeyPolls, m.KeyIRQs, len(m.KeyQueue))
	if runErr != nil {
		fmt.Printf("停下來的原因：%v\n", runErr)
	}
	{
		lin := uint32(c.Seg[cpu.CS])*16 + uint32(c.IP)
		fmt.Printf("停在實體 %05Xh，前後 32 byte：% X | % X\n",
			lin, m.Mem[lin-16:lin], m.Mem[lin:lin+16])
			for _, port := range []uint16{0x60, 0x61, 0x64, 0x20, 0x3DA} {
			if n := m.PortsIn[port]; n > 0 {
				fmt.Printf("埠 %03Xh 讀過 %d 次\n", port, n)
			}
		}
		fmt.Printf("向量 08h→%04X:%04X  09h→%04X:%04X  1Ch→%04X:%04X  16h→%04X:%04X\n",
			m.Read16(0x08*4+2), m.Read16(0x08*4),
			m.Read16(0x09*4+2), m.Read16(0x09*4),
			m.Read16(0x1C*4+2), m.Read16(0x1C*4),
			m.Read16(0x16*4+2), m.Read16(0x16*4))
		// 最後一萬條指令都在哪：只記 (cs,ip)，看迴圈有多大
		seen := map[uint32]int{}
		for i := 0; i < 20000; i++ {
			if err := m.Step(); err != nil {
				break
			}
			seen[uint32(m.CPU.Seg[cpu.CS])<<16|uint32(m.CPU.IP)]++
		}
		type kv struct {
			k uint32
			n int
		}
		var top []kv
		for k, n := range seen {
			top = append(top, kv{k, n})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n })
		fmt.Printf("再跑兩萬條指令，落點 %d 個；最常見的幾個：\n", len(seen))
		for i := 0; i < len(top) && i < 12; i++ {
			fmt.Printf("   %04X:%04X ×%d\n", top[i].k>>16, top[i].k&0xFFFF, top[i].n)
		}
	}
	if d.Exited {
		fmt.Printf("程式呼叫了結束服務，回傳碼 %d\n", d.ExitCode)
	}
	if n := len(d.Opened); n > 0 {
		fmt.Printf("開過的檔（%d）：%s\n", n, strings.Join(d.Opened, "、"))
	}
	if n := len(d.Missing); n > 0 {
		fmt.Printf("找不到的檔（%d）：%s\n", n, strings.Join(d.Missing, "、"))
	}
	if r := d.UnimplementedReport(); len(r) > 0 {
		fmt.Println("沒實作的服務：")
		sort.Strings(r)
		for _, s := range r {
			fmt.Println("  ", s)
		}
	}
	if s := d.Console; len(s) > 0 {
		fmt.Printf("主控台輸出（%d bytes）：\n%s\n", len(s), string(s))
	}
	if *pme != "" {
		locatePME(m, *pme)
	}

	mode := m.Read8(0x449)
	cols := int(m.Read16(0x44A))
	page := m.Read16(0x44E)
	fmt.Printf("BIOS 視訊模式 %02Xh，%d 欄，作用頁偏移 %04Xh\n", mode, cols, page)
	if cols == 0 {
		cols = 80
	}
	fmt.Println(textScreen(m, cols, uint32(page)))
}

// textScreen 把 B800 的文字頁讀成 80×25。p-System 走的是文字模式，
// 畫面不在 VGA 的 320×200 緩衝區裡。
func textScreen(m *machine.Machine, cols int, page uint32) string {
	var b strings.Builder
	blank := true
	for row := 0; row < 25; row++ {
		line := make([]byte, 0, cols)
		for col := 0; col < cols; col++ {
			ch := m.Read8(0xB8000 + page + uint32(row*cols+col)*2)
			if ch < 0x20 || ch > 0x7E {
				ch = ' '
			} else {
				blank = false
			}
			line = append(line, ch)
		}
		b.WriteString("|" + strings.TrimRight(string(line), " ") + "\n")
	}
	if blank {
		return "（整頁是空白）"
	}
	return b.String()
}

// locatePME 在記憶體裡找 p-machine 直譯器載到哪。
//
// 直譯器是從 .VOL 裡搬進記憶體的，載入位址不是靜態看得出來的。
// 拿它的前幾十個 byte 當指紋掃一遍實體記憶體最直接；
// 找到之後 dispatch 表與所有狀態變數的位址就都跟著定了。
func locatePME(m *machine.Machine, path string) {
	img, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "psys: 讀不到 PME 映像：", err)
		return
	}
	if _, ok := findPME(m, img); !ok {
		fmt.Println("記憶體裡找不到 PME 的遮罩表")
		return
	}
	// 檔頭那張位址表在載入時會被改寫，不能當指紋。
	// `0x1fb6` 起是遮罩表（第 n 項 = (1<<n)−1），純常數、不會被重定位。
	const maskTable = 0x1fb6
	sig := img[maskTable : maskTable+34]
	var hits []uint32
	for a := uint32(0); a+uint32(len(sig)) < machine.MemSize; a++ {
		if m.Mem[a] != sig[0] || m.Mem[a+1] != sig[1] {
			continue
		}
		if string(m.Mem[a:a+uint32(len(sig))]) == string(sig) {
			hits = append(hits, a-maskTable)
		}
	}
	if len(hits) == 0 {
		fmt.Println("記憶體裡找不到 PME 的遮罩表")
		return
	}
	for _, base := range hits {
		same := 0
		for i, b := range img {
			if m.Mem[base+uint32(i)] == b {
				same++
			}
		}
		const dispatch = 0x1d56
		fmt.Printf("PME 映像基底 %05Xh，%d／%d byte 與磁碟上的一致（差 %d）\n",
			base, same, len(img), len(img)-same)
		fmt.Printf("  dispatch 表在 %05Xh；若 cs 指著它，cs = %04Xh（實際 cs = %04Xh）\n",
			base+dispatch, (base+dispatch)>>4, m.CPU.Seg[cpu.CS])
		fmt.Printf("  opcode 9Eh 的表項 = %04Xh（磁碟上是 026Eh）\n", m.Read16(base+dispatch+0x9E*2))
		// 檔頭那 18 個 word 載入前後的差
		fmt.Print("  檔頭 word 0..17：")
		for i := 0; i < 18; i++ {
			disk := uint16(img[i*2]) | uint16(img[i*2+1])<<8
			live := m.Read16(base + uint32(i)*2)
			if disk == live {
				fmt.Printf(" %04X", live)
			} else {
				fmt.Printf(" %04X→%04X", disk, live)
			}
		}
		fmt.Println()

		// 差異落在哪：每 256 byte 一格
		fmt.Print("  差異分布（每格 256 byte，. = 全同，數字 = 幾個 byte 不同）：\n   ")
		for blk := 0; blk*256 < len(img); blk++ {
			n := 0
			for i := blk * 256; i < (blk+1)*256 && i < len(img); i++ {
				if m.Mem[base+uint32(i)] != img[i] {
					n++
				}
			}
			switch {
			case n == 0:
				fmt.Print(".")
			case n < 10:
				fmt.Printf("%d", n)
			default:
				fmt.Print("#")
			}
			if blk%64 == 63 {
				fmt.Print("\n   ")
			}
		}
		fmt.Println()
		// 假說：載入器把磁碟上 1D56h 起的 dispatch 表搬到映像偏移 0，
		// 這樣 `jmp word ptr cs:[di]` 不必帶位移就能用，
		// 而表項（映像相對偏移）當成 cs:offset 也剛好正確。
		copied := 0
		for i := 0; i < 512; i++ {
			if m.Mem[base+uint32(i)] == img[dispatch+i] {
				copied++
			}
		}
		fmt.Printf("  映像偏移 0..1FFh 與磁碟 %04Xh 起的 dispatch 表：%d／512 byte 相同\n",
			dispatch, copied)

		// 其餘差異逐一列出來，看是不是重定位。
		fmt.Println("  512h 之後的差異：")
		shown := 0
		for i := 512; i+1 < len(img); i += 2 {
			d := uint16(img[i]) | uint16(img[i+1])<<8
			l := m.Read16(base + uint32(i))
			if d == l {
				continue
			}
			if shown < 24 {
				fmt.Printf("    %04Xh: %04X → %04X（差 %+d）\n", i, d, l, int(l)-int(d))
			}
			shown++
		}
		fmt.Printf("    共 %d 個 word 不同\n", shown)
	}
}

// setCmdline 把命令列參數寫進 PSP 的 80h。
func setCmdline(m *machine.Machine, s string) {
	psp := uint32(machine.PSPSeg) * 16
	line := " " + s + "\r"
	m.Write8(psp+0x80, uint8(len(line)-1))
	m.WriteBytes(psp+0x81, []byte(line))
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "psys:", err)
	os.Exit(1)
}


// findPME 用遮罩表當指紋找出 PME 映像的基底。
//
// 檔頭那張位址表在載入時會被 dispatch 表蓋掉，不能當指紋；
// `0x1fb6` 起的遮罩表（第 n 項 = (1<<n)−1）是純常數，不會被動到。
func findPME(m *machine.Machine, img []byte) (uint32, bool) {
	const maskTable = 0x1fb6
	sig := img[maskTable : maskTable+34]
	for a := uint32(0); a+uint32(len(sig)) < machine.MemSize; a++ {
		if m.Mem[a] != sig[0] || m.Mem[a+1] != sig[1] {
			continue
		}
		if string(m.Mem[a:a+uint32(len(sig))]) == string(sig) {
			return a - maskTable, true
		}
	}
	return 0, false
}

// tracePCode 記錄原版直譯器實際執行的 p-code。
//
// 判準是「控制權剛落到某個 dispatch 目標」：載入器把 dispatch 表搬到映像偏移 0，
// 所以表項就是 cs 相對的常式位址。進到常式時 `lodsb` 已經走過，
// 所以剛執行的那個 opcode 在 `ds:si−1`，si 就是 IPC。
func tracePCode(m *machine.Machine, d *dos.DOS, base uint32, want int, budget uint64) {
	seg := uint16(base >> 4)
	if uint32(seg)*16 != base {
		fmt.Printf("PME 基底 %05Xh 不是段對齊，追不了\n", base)
		return
	}
	targets := map[uint16]bool{}
	for op := 0; op < 256; op++ {
		targets[m.Read16(base+uint32(op)*2)] = true
	}

	type row struct {
		codeSeg, ipc uint16
		op           uint8
		sp           uint16
		tos          uint16
	}
	var rows []row
	limit := m.Steps + budget
	for len(rows) < want && m.Steps < limit && !d.Exited {
		if err := m.Step(); err != nil {
			fmt.Println("追蹤中斷：", err)
			break
		}
		c := m.CPU
		if c.Seg[cpu.CS] != seg || !targets[c.IP] {
			continue
		}
		ds, si := c.Seg[cpu.DS], c.R[cpu.SI]
		rows = append(rows, row{ds, si - 1,
			m.Read8(uint32(ds)*16 + uint32(si-1)),
			c.R[cpu.SP],
			m.Read16(uint32(c.Seg[cpu.SS])*16 + uint32(c.R[cpu.SP]))})
	}

	fmt.Printf("\np-code 軌跡（%d 條，PME 段 %04Xh）：\n", len(rows), seg)
	fmt.Println("   #  code seg:IPC  opcode  SP    TOS")
	for i, r := range rows {
		fmt.Printf("  %3d  %04X:%04X    %02X      %04X  %04X\n",
			i, r.codeSeg, r.ipc, r.op, r.sp, r.tos)
	}
}
