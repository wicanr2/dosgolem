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
	"image"
	"image/color"
	"image/png"
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
	cpuModel := flag.String("cpu", "386", "CPU 機型：8086、186 或 386。386 會讓偵測 CPU 的程式改走 32 位元路徑，而 0x66 只有子集（`docs/spec/012-cpu-386-subset`）")
	scratch := flag.String("scratch", "", "可寫的暫存層目錄（`docs/spec/009-scratch-writes`）；空字串＝寫入只記帳不落地")
	steps := flag.Uint64("steps", 20_000_000, "最多跑幾條指令")
	cmdline := flag.String("args", "", "命令列參數，寫進 PSP 的 80h")
	keys := flag.String("keys", "", "排進鍵盤佇列的字元（int 16h 與 IRQ1 都排）")
	find := flag.String("find", "", "把這個檔案的一段內容當指紋，在記憶體裡找它載到哪")
	findOff := flag.Int("find-off", 0, "指紋在該檔案裡的偏移")
	findLen := flag.Int("find-len", 32, "指紋長度")
	memops := flag.Bool("memops", false, "印出 AH=48h／49h／4Ah 的呼叫紀錄與 EXEC 紀錄（查子行程是不是蓋到父行程）")
	traceExit := flag.Int("trace-after-exit", 0, "第一個 EXEC 起來的子行程結束後，逐條印出接下來幾條指令的 CS:IP 與位元組")
	watch := flag.String("watch", "", "監看一段線性位址的寫入，格式 lo-hi（十六進位），每次寫入印出 CS:IP")
	traceTail := flag.Int("trace-tail", 0, "記住最後幾條指令的 CS:IP、位元組與暫存器，收工時印出來（查程式為什麼結束）")
	pngOut := flag.String("png", "", "收工時把目前畫面存成 PNG（平面模式與 mode 13h）")
	watchRead := flag.String("watch-read", "", "監看一段線性位址的讀取，格式 lo-hi（十六進位）；收工時印出被讀過的位址範圍與讀取者")
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
	d.Scratch = *scratch
	if *memops {
		d.Calls = map[dos.Call]int{}
	}
	switch *cpuModel {
	case "386":
	case "186":
		m.CPU.Model = cpu.Model80186
		m.CPU.SetFlags(m.CPU.Flags) // 旗標改套 8086／80186 的固定位元
	case "8086":
		m.CPU.Model = cpu.Model8086
		m.CPU.SetFlags(m.CPU.Flags)
	default:
		die(fmt.Errorf("-cpu 只接受 8086、186 或 386：%q", *cpuModel))
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

	if *watch != "" {
		var lo, hi uint32
		if _, err := fmt.Sscanf(*watch, "%x-%x", &lo, &hi); err != nil {
			die(fmt.Errorf("-watch 格式是 lo-hi：%v", err))
		}
		m.WatchWrites(lo, hi, func(a uint32, old, v uint8) {
			c := m.CPU
			fmt.Printf("watch 步 %d  %05X: %02X→%02X  由 %04X:%04X  SS:SP=%04X:%04X  EXEC 數=%d\n",
				m.Steps, a, old, v, c.Seg[cpu.CS], c.IP, c.Seg[cpu.SS], c.R[cpu.SP], len(d.ExecLog))
		})
	}
	readers := map[uint32]map[string]int{}
	if *watchRead != "" {
		var lo, hi uint32
		if _, err := fmt.Sscanf(*watchRead, "%x-%x", &lo, &hi); err != nil {
			die(fmt.Errorf("-watch-read 格式是 lo-hi：%v", err))
		}
		m.WatchReads(lo, hi, func(a uint32, _ uint8) {
			c := m.CPU
			k := fmt.Sprintf("%04X:%04X", c.Seg[cpu.CS], c.IP)
			if readers[a] == nil {
				readers[a] = map[string]int{}
			}
			readers[a][k]++
		})
	}
	var runErr error
	traced := 0
	tail := make([]string, 0, *traceTail)
	for m.Steps < *steps && !d.Exited {
		if *traceTail > 0 {
			c := m.CPU
			a := cpu.Addr(c.Seg[cpu.CS], c.IP)
			line := fmt.Sprintf("%04X:%04X  % X  AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X SP=%04X DS=%04X ES=%04X",
				c.Seg[cpu.CS], c.IP, bytesAt(m, a, 6), c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX],
				c.R[cpu.SI], c.R[cpu.DI], c.R[cpu.SP], c.Seg[cpu.DS], c.Seg[cpu.ES])
			if len(tail) == *traceTail {
				tail = tail[1:]
			}
			tail = append(tail, line)
		}
		if runErr = m.Step(); runErr != nil {
			break
		}
		if *traceExit > traced && len(d.ExecLog) > 0 && d.ExecLog[0].Exit != 0xFF {
			c := m.CPU
			a := cpu.Addr(c.Seg[cpu.CS], c.IP)
			fmt.Printf("trace %4d  %04X:%04X  % X  AX=%04X BX=%04X CX=%04X DX=%04X SP=%04X BP=%04X DS=%04X ES=%04X SS=%04X F=%04X\n",
				traced, c.Seg[cpu.CS], c.IP, bytesAt(m, a, 6), c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX],
				c.R[cpu.SP], c.R[cpu.BP], c.Seg[cpu.DS], c.Seg[cpu.ES], c.Seg[cpu.SS], c.Flags)
			traced++
		}
	}
	report(m, d, runErr, *loop)
	if *pngOut != "" {
		if err := savePNG(m, *pngOut); err != nil {
			fmt.Println("PNG：", err)
		}
	}
	if len(readers) > 0 {
		addrs := make([]uint32, 0, len(readers))
		for a := range readers {
			addrs = append(addrs, a)
		}
		sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })
		fmt.Printf("讀過 %d 個位址，%05X–%05X；讀取者：", len(addrs), addrs[0], addrs[len(addrs)-1])
		who := map[string]int{}
		for _, a := range addrs {
			for k, n := range readers[a] {
				who[k] += n
			}
		}
		for k, n := range who {
			fmt.Printf(" %s×%d", k, n)
		}
		fmt.Println()
	}
	for i, line := range tail {
		fmt.Printf("tail %4d  %s\n", i-len(tail), line)
	}
	if *memops {
		reportMem(d)
	}

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
		m.Ticks, m.KeyIRQs, m.KeyQueueLen(), d.KeyPolls)

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

// reportMem 印出記憶體服務與 EXEC 的呼叫紀錄，依發生順序。
func reportMem(d *dos.DOS) {
	fmt.Println("記憶體服務：")
	for _, op := range d.MemOps {
		fmt.Printf("   步 %10d  AH=%02Xh BX=%04X ES=%04X → AX=%04X ok=%v\n",
			op.Step, op.Fn, op.BX, op.ES, op.AX, op.OK)
	}
	fmt.Println("中斷服務呼叫次數：")
	keys := make([]dos.Call, 0, len(d.Calls))
	for k := range d.Calls {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Int != keys[j].Int {
			return keys[i].Int < keys[j].Int
		}
		if keys[i].AH != keys[j].AH {
			return keys[i].AH < keys[j].AH
		}
		return keys[i].AL < keys[j].AL
	})
	for _, k := range keys {
		fmt.Printf("   int %02Xh AH=%02Xh AL=%02Xh ×%d\n", k.Int, k.AH, k.AL, d.Calls[k])
	}
	fmt.Println("EXEC：")
	for _, e := range d.ExecLog {
		fmt.Printf("   %s（%s）PSP=%04X exit=%02X\n", e.Name, e.Base, e.PSP, e.Exit)
	}
}

func bytesAt(m *machine.Machine, a uint32, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = m.Read8(a + uint32(i))
	}
	return b
}

// savePNG 把目前畫面存成 PNG：平面模式走 PlanarRGB，mode 13h 走色號＋DAC。
func savePNG(m *machine.Machine, path string) error {
	w, h, rgb := m.PlanarRGB()
	if rgb == nil {
		if m.VideoMode() != 0x13 {
			return fmt.Errorf("目前是 %02Xh 模式，只支援平面模式與 13h", m.VideoMode())
		}
		w, h = 320, 200
		raw, pal := m.VideoRaw(), m.Palette()
		rgb = make([]uint8, w*h*3)
		for i := 0; i < w*h && i < len(raw); i++ {
			c := pal[raw[i]]
			rgb[i*3], rgb[i*3+1], rgb[i*3+2] = c[0], c[1], c[2]
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Set(i%w, i/w, color.RGBA{rgb[i*3], rgb[i*3+1], rgb[i*3+2], 255})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Printf("畫面 %d×%d → %s\n", w, h, path)
	if _, _, px := m.Planar(); px != nil {
		hist := [16]int{}
		for _, p := range px {
			hist[p&15]++
		}
		pal := m.Palette()
		fmt.Print("平面色號分布與對應顏色：")
		for i, n := range hist {
			if n > 0 {
				c := pal[m.VGA.DACIndex(uint8(i))]
				fmt.Printf(" %d:%d→#%02X%02X%02X", i, n, c[0], c[1], c[2])
			}
		}
		fmt.Println()
	}
	return png.Encode(f, img)
}
