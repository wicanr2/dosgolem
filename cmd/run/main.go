// run 載入一個 DOS 程式、跑一段指令、把觀察到的東西印出來。
//
// 給「這個 binary 在這台機器上跑得動嗎」用的探路工具：缺哪些服務、
// 開過哪些檔、畫面上有什麼、卡在哪個迴圈。**不認識任何特定的程式。**
//
//	tools/go.sh run ./cmd/run -prog /orig/FOO.COM -root /orig -steps 20000000
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
	prog := flag.String("prog", "", "要跑的 .COM 或 .EXE")
	root := flag.String("root", "", "DOS 看得到的目錄")
	dir := flag.String("dir", "", "DOS 的目前目錄（root 底下的子目錄）")
	steps := flag.Uint64("steps", 20_000_000, "最多跑幾條指令")
	cmdline := flag.String("args", "", "命令列參數，寫進 PSP 的 80h")
	keys := flag.String("keys", "", "排進鍵盤佇列的字元（int 16h 與 IRQ1 都排）")
	find := flag.String("find", "", "把這個檔案的一段內容當指紋，在記憶體裡找它載到哪")
	findOff := flag.Int("find-off", 0, "指紋在該檔案裡的偏移")
	findLen := flag.Int("find-len", 32, "指紋長度")
	loop := flag.Int("loop", 0, "收工前再跑幾條指令，統計落點看它是不是在空轉")
	flag.Parse()
	if *prog == "" || *root == "" {
		flag.Usage()
		os.Exit(2)
	}

	m, d, err := load(*prog, *root, *dir)
	if err != nil {
		die(err)
	}
	if *cmdline != "" {
		setCmdline(m, *cmdline)
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
	report(m, d, runErr, *loop)

	if *find != "" {
		locate(m, *find, *findOff, *findLen)
	}
}

// load 依副檔名決定用哪個載入器。
func load(prog, root, dir string) (*machine.Machine, *dos.DOS, error) {
	data, err := os.ReadFile(prog)
	if err != nil {
		return nil, nil, err
	}
	m := machine.New()
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		err = m.LoadEXE(data)
	} else {
		err = m.LoadCOM(data)
	}
	if err != nil {
		return nil, nil, err
	}
	d := dos.New(m, root)
	d.Dir = dir
	d.Install()
	return m, d, nil
}

func report(m *machine.Machine, d *dos.DOS, runErr error, loop int) {
	c := m.CPU
	fmt.Printf("跑了 %d 條指令，停在 %04X:%04X\n", m.Steps, c.Seg[cpu.CS], c.IP)
	if runErr != nil {
		fmt.Printf("停下來的原因：%v\n", runErr)
	}
	if d.Exited {
		fmt.Printf("程式呼叫了結束服務，回傳碼 %d\n", d.ExitCode)
	}
	fmt.Printf("計時器中斷 %d 次、鍵盤中斷 %d 次（掃描碼還剩 %d）、int 16h 被問 %d 次\n",
		m.Ticks, m.KeyIRQs, len(m.KeyQueue), d.KeyPolls)

	if n := len(d.Opened); n > 0 {
		fmt.Printf("開過的檔（%d）：%s\n", n, strings.Join(d.Opened, "、"))
	}
	if n := len(d.Missing); n > 0 {
		fmt.Printf("找不到的檔（%d）：%s\n", n, strings.Join(d.Missing, "、"))
	}
	if r := d.UnimplementedReport(); len(r) > 0 {
		sort.Strings(r)
		fmt.Println("沒實作的服務：")
		for _, s := range r {
			fmt.Println("  ", s)
		}
	}
	for _, port := range []uint16{0x60, 0x61, 0x64, 0x3DA, 0x388} {
		if n := m.PortsIn[port]; n > 0 {
			fmt.Printf("埠 %03Xh 讀過 %d 次\n", port, n)
		}
	}
	if s := d.Console; len(s) > 0 {
		fmt.Printf("主控台輸出（%d bytes）：\n%s\n", len(s), string(s))
	}

	fmt.Println("文字畫面：")
	for i, line := range m.TextScreen(0) {
		if line != "" {
			fmt.Printf("  %2d |%s\n", i, line)
		}
	}

	if loop > 0 {
		loopReport(m, loop)
	}
}

// loopReport 再跑一段，統計 CS:IP 的落點。落點只有兩三個就是在空轉，
// 而空轉的位置通常直接指出它在等什麼。
func loopReport(m *machine.Machine, n int) {
	seen := map[uint32]int{}
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			break
		}
		seen[uint32(m.CPU.Seg[cpu.CS])<<16|uint32(m.CPU.IP)]++
	}
	type kv struct {
		k uint32
		n int
	}
	top := make([]kv, 0, len(seen))
	for k, c := range seen {
		top = append(top, kv{k, c})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n })
	fmt.Printf("再跑 %d 條指令，落點 %d 個；最常見的：\n", n, len(seen))
	for i := 0; i < len(top) && i < 8; i++ {
		fmt.Printf("   %04X:%04X ×%d\n", top[i].k>>16, top[i].k&0xFFFF, top[i].n)
	}
}

// locate 用一個檔案裡的一段內容當指紋，找出它被載到記憶體的哪裡。
func locate(m *machine.Machine, path string, off, n int) {
	img, err := os.ReadFile(path)
	if err != nil {
		die(err)
	}
	if off < 0 || n <= 0 || off+n > len(img) {
		die(fmt.Errorf("指紋 %d..%d 超出 %s 的 %d bytes", off, off+n, path, len(img)))
	}
	hits := m.Find(img[off : off+n])
	if len(hits) == 0 {
		fmt.Printf("記憶體裡找不到 %s 偏移 %04Xh 起的 %d 個位元組\n", path, off, n)
		return
	}
	for _, a := range hits {
		base := int64(a) - int64(off)
		fmt.Printf("指紋在 %05Xh → 映像基底 %05Xh", a, base)
		if base >= 0 && base+int64(len(img)) <= int64(machine.MemSize) {
			same := 0
			for i, b := range img {
				if m.Mem[base+int64(i)] == b {
					same++
				}
			}
			fmt.Printf("，%d／%d byte 與檔案一致", same, len(img))
		}
		fmt.Println()
	}
}

func setCmdline(m *machine.Machine, s string) {
	psp := uint32(machine.PSPSeg) * 16
	line := " " + s + "\r"
	m.Write8(psp+0x80, uint8(len(line)-1))
	m.WriteBytes(psp+0x81, []byte(line))
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "run:", err)
	os.Exit(1)
}
