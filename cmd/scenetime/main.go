// Command scenetime 量《三國志》DOS 版演出腳本的等待長度，單位是 BIOS tick。
//
// 靜態讀出來的模型（sangokushi 專案 `docs/re/13` §5）是：
//
//	W        → delay(⌊T×5÷100⌋ + 1)          （檔案 0x4605）
//	delay(n) → 每一圈等 [ds:0x3F6] ÷ 2 個 tick（檔案 0x43EA、0x1F4A、0x1F5E）
//
// 靜態讀完要對拍：這支把 `delay()` 與**整支播放器**都真的在 dosgolem 上跑一遍，
// 回報實際走掉幾個 BIOS tick。tick 由 `IRQ0Every` 驅動，是指令數時鐘——
// 但**要量的東西正好是 tick 數，不是秒**，所以與 `IRQ0Every` 取多少無關。
//
// ⚠ **本專案不含任何原版檔案**，`-exe` 與 `-root` 都由玩家自備。
//
//	go run ./cmd/scenetime -exe path/to/MAIN.EXE -root path/to/dir
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// MAIN.EXE 的基準（都是**檔案位移**，`docs/re/13` §1）。
//
// 映像從檔案 0x400 開始載，所以 root 的 IP ＝ 檔案位移 − 0x400。
//
// ⚠ **DGROUP 不能從檔案位移推。** MZ 檔頭只宣告 24224 bytes，
// 資料段是 PLINK86 的 overlay loader 開機時自己從 `MAIN.EXE` 讀進來的，
// 落點與檔案位移沒有固定關係。所以這裡**先把遊戲跑起來，再到記憶體裡
// 找指令選單那串字**，反推 DGROUP 段——猜錯的話下面每一個 `ds:` 讀到的
// 都是垃圾，而讀垃圾不會報錯。
const (
	ipDelay = 0x43EA - 0x400 // delay(n)
	ipPlay  = 0x4416 - 0x400 // 演出播放器 play(slot)

	dsDelayUnit = 0x3F6  // delay 每一圈等幾個 tick 的來源（× 1/2）
	dsEnable    = 0x220  // 播放器的總開關，0 就整支不播
	dsHandle    = 0xAE2  // PICDATA.DAT 的檔案 handle
	dsMatSeg    = 0x3C66 // 素材緩衝區的段（`B` opcode 把圖讀進這裡）
	dsPicName   = 0xB04  // 字串 "PICDATA.DAT"
	dsMenu      = 0x5431 // 字串 "Move\nWar\nSend…"（拿來驗 DGROUP 對不對）

	// scratchSeg 放我們自己塞的 `int 21h` 小樁，挑在映像之後、640 KB 之內。
	scratchSeg = 0x8000

	// sentinel 是假的返回位址：`ret` 之後 IP 會等於它，我們就知道跑完了。
	sentinel = 0xFFFE
)

func main() {
	exe := flag.String("exe", "", "MAIN.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄（要有 PICDATA.DAT）")
	boot := flag.Uint64("boot", 30_000_000, "先讓遊戲自己跑幾道指令（等資料段載進來）")
	tick := flag.Uint64("tick", 3000, "每幾道指令一次計時器中斷")
	slots := flag.String("slots", "188,218,196,134,208,108,124,150,176,226,230",
		"要跑的演出 slot，逗號分隔")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	img, err := os.ReadFile(*exe)
	die(err)
	m := machine.New()
	die(m.LoadEXE(img))
	m.IRQ0Every = *tick
	d := dos.New(m, *root)
	d.Install()
	d.Stdin = append(d.Stdin, 'y')

	// ---- 先讓 overlay loader 把資料段搬進記憶體 -------------------------
	for m.Steps < *boot && !m.CPU.Halted && !d.Exited {
		if cpu.Addr(m.CPU.Seg[cpu.CS], m.CPU.IP) >= machine.VideoSeg*16 {
			break
		}
		if m.Step() != nil {
			break
		}
	}
	fmt.Printf("開機跑了 %d 道指令，停在 %04X:%04X\n",
		m.Steps, m.CPU.Seg[cpu.CS], m.CPU.IP)

	// root 的 24224 bytes 有沒有被 overlay 蓋掉？蓋掉的話拿檔案反組譯出來的
	// 位址就對不上執行期，而**症狀是「跳到莫名其妙的段」**，不是錯誤訊息。
	{
		var lo, hi = -1, -1
		var n int
		for off := 0; off < 24224; off++ {
			if m.Mem[cpu.Addr(machine.LoadSeg, 0)+uint32(off)] != img[0x400+off] {
				n++
				if lo < 0 {
					lo = off
				}
				hi = off
			}
		}
		fmt.Printf("root 映像與檔案不同的 bytes：%d／24224", n)
		if n > 0 {
			fmt.Printf("，範圍 IP %04X–%04X（檔案 %05X–%05X）", lo, hi, lo+0x400, hi+0x400)
		}
		fmt.Println()
	}

	dgroup, ok := findDGroup(m)
	if !ok {
		fmt.Println("記憶體裡找不到指令選單那串字——資料段還沒載進來")
		os.Exit(1)
	}
	fmt.Printf("int 08h → %04X:%04X   int 1Ch → %04X:%04X   int 1Ah → %04X:%04X\n",
		m.Read16(0x08*4+2), m.Read16(0x08*4),
		m.Read16(0x1C*4+2), m.Read16(0x1C*4),
		m.Read16(0x1A*4+2), m.Read16(0x1A*4))
	fmt.Printf("DGROUP 段 %04X（由 ds:%04X 的指令選單反推）\n", dgroup, dsMenu)
	fmt.Printf("  ds:%04X = %q\n", dsMenu, str(m, dgroup, dsMenu, 34))
	unit = m.Read16(lin(dgroup, dsDelayUnit))
	fmt.Printf("  ds:%04X = %d → delay 每一圈 %d 個 tick\n\n",
		dsDelayUnit, unit, unit/2)

	// ---- delay(n) 實跑 --------------------------------------------------
	fmt.Println("delay(n) 實跑：")
	for n := uint16(1); n <= 4; n++ {
		t, st, why := call(m, dgroup, ipDelay, n)
		fmt.Printf("  delay(%d) → %d tick（模型 %d）%s%s\n", n, t,
			uint32(n)*uint32(unit/2), plural(st), note(why))
	}

	// ---- 整支播放器實跑 -------------------------------------------------
	h, err := openPic(m, d, dgroup)
	if err != nil {
		fmt.Println("\n開 PICDATA.DAT 失敗：", err)
		return
	}
	// **素材緩衝區要自己配。** 正常開機時遊戲會配一段給 `B` opcode 讀圖，
	// 我們是直接跳進播放器的，那一步沒發生過——`ds:3C66` 還是 0，
	// `B` 就會把 3072 bytes 讀到 `0000:0000`，把中斷向量表整片蓋掉。
	// 症狀是「跑一跑跳到莫名其妙的段」，不是任何錯誤訊息。
	mat := m.Read16(lin(dgroup, dsMatSeg))
	fmt.Printf("\nds:%04X（素材緩衝段）＝ %04X", dsMatSeg, mat)
	if mat == 0 {
		seg, err := allocSeg(m, 0x400)
		if err != nil {
			fmt.Println("　配不到記憶體：", err)
			return
		}
		mat = seg
		m.Write16(lin(dgroup, dsMatSeg), mat)
		fmt.Printf(" → 自己配一段 %04X", mat)
	}
	fmt.Println()
	fmt.Printf("PICDATA.DAT handle = %d；整支播放器實跑：\n", h)
	fmt.Println("  slot   W 數  delay 次  等待秒數  每一幀的 tick（60 TPS）")
	for _, s := range parse(*slots) {
		m.Write16(lin(dgroup, dsHandle), h)
		m.Write16(lin(dgroup, dsEnable), 1)
		_, st, why := call(m, dgroup, ipPlay, s)
		w := waits(s)
		// 一幀 ＝ n × 3 個 BIOS tick ÷ 18.2065 Hz，換算成 60 TPS 的 tick。
		var wait uint32
		seq := make([]int, 0, len(ops))
		for _, n := range ops {
			wait += uint32(n) * uint32(unit/2)
			seq = append(seq, int(math.Round(float64(n)*float64(unit/2)*60/tickHz)))
		}
		fmt.Printf("  %4d   %2d      %2d      %6.3f  %v%s\n",
			s, w, len(ops), float64(wait)/tickHz, seq, note(why))
		_ = st
	}
}

// findDGroup 在記憶體裡找指令選單那串字，反推 DGROUP 段。
//
// 只認**段邊界對齊**的落點：DGROUP 一定從一個段的 0 開始，
// 對不齊就表示找到的是別的東西（例如檔案緩衝區裡的同一份 bytes）。
func findDGroup(m *machine.Machine) (uint16, bool) {
	pat := []byte("Move\nWar\nSend\n")
	for i := 0; i+len(pat) < len(m.Mem); i++ {
		if m.Mem[i] != pat[0] {
			continue
		}
		if string(m.Mem[i:i+len(pat)]) != string(pat) {
			continue
		}
		base := int(i) - dsMenu
		if base >= 0 && base%16 == 0 {
			return uint16(base / 16), true
		}
	}
	return 0, false
}

// waits 數這一支腳本有幾個 `W`（＝幾幀）。直接讀檔，不經過遊戲。
func waits(slot uint16) int {
	// 播放器把 512 bytes 讀進 [bp-0x996]，跑完就沒了；重讀一次比較單純。
	// 這裡用同一份規則掃：opcode 'A'..'Y'，參數長度表與 picdata_disasm 相同。
	buf := make([]byte, 512)
	f, err := os.Open(picPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	if _, err := f.ReadAt(buf, int64(slot)*512); err != nil {
		return 0
	}
	args := map[byte]int{'A': 2, 'B': 1, 'C': 5, 'D': 6, 'E': 0, 'F': 1, 'G': 0,
		'H': 0, 'I': 1, 'J': 2, 'K': 1, 'L': 0, 'M': 0, 'N': 1, 'O': 0, 'P': 2,
		'Q': 3, 'R': 0, 'S': 1, 'T': 1, 'U': 3, 'V': 1, 'W': 0, 'X': 1, 'Y': 1, 'Z': 1}
	n := 0
	for i := 0; i < len(buf); {
		a, ok := args[buf[i]]
		if !ok {
			break
		}
		if buf[i] == 'W' {
			n++
		}
		i += 1 + a
	}
	return n
}

var picPath string

// openPic 用遊戲自己的檔名字串發一次 `int 21h AH=3Dh`，拿到真的 handle。
//
// **不自己造 handle**：dosgolem 的檔案表是 `internal/dos` 管的，
// 憑空塞一個號碼進去，`lseek`／`read` 會安靜地失敗。
func openPic(m *machine.Machine, d *dos.DOS, dgroup uint16) (uint16, error) {
	picPath = d.Root + "/" + str(m, dgroup, dsPicName, 16)
	m.Write8(lin(scratchSeg, 0), 0xCD) // int 21h
	m.Write8(lin(scratchSeg, 1), 0x21)
	c := m.CPU
	c.Seg[cpu.CS], c.IP = scratchSeg, 0
	c.Seg[cpu.DS] = dgroup
	c.R[cpu.AX] = 0x3D00
	c.R[cpu.DX] = dsPicName
	if err := m.Step(); err != nil {
		return 0, err
	}
	if c.Flags&cpu.CF != 0 {
		return 0, fmt.Errorf("AX=%04X", c.R[cpu.AX])
	}
	return c.R[cpu.AX], nil
}

// allocSeg 用 `int 21h AH=48h` 要一段記憶體。
func allocSeg(m *machine.Machine, paras uint16) (uint16, error) {
	m.Write8(lin(scratchSeg, 0), 0xCD)
	m.Write8(lin(scratchSeg, 1), 0x21)
	c := m.CPU
	c.Seg[cpu.CS], c.IP = scratchSeg, 0
	c.R[cpu.AX] = 0x4800
	c.R[cpu.BX] = paras
	if err := m.Step(); err != nil {
		return 0, err
	}
	if c.Flags&cpu.CF != 0 {
		return 0, fmt.Errorf("AX=%04X", c.R[cpu.AX])
	}
	return c.R[cpu.AX], nil
}

// call 近呼叫一支函式（右到左推參數，推假返回位址），
// 回傳走掉幾個 BIOS tick、幾道指令、有沒有正常返回。
func call(m *machine.Machine, dgroup, ip uint16, args ...uint16) (uint32, uint64, string) {
	ops = ops[:0]
	c := m.CPU
	c.Seg[cpu.CS], c.IP = machine.LoadSeg, ip
	c.Seg[cpu.DS], c.Seg[cpu.ES] = dgroup, dgroup
	sp := c.R[cpu.SP]
	for i := len(args) - 1; i >= 0; i-- {
		sp -= 2
		m.Write16(lin(c.Seg[cpu.SS], sp), args[i])
	}
	sp -= 2
	m.Write16(lin(c.Seg[cpu.SS], sp), sentinel)
	c.R[cpu.SP] = sp
	c.Halted = false

	t0, s0, why := bdaTicks(m), m.Steps, "跑滿上限"
	for i := 0; i < 20_000_000; i++ {
		if c.Seg[cpu.CS] == machine.LoadSeg && c.IP == sentinel {
			why = ""
			break
		}
		if a := cpu.Addr(c.Seg[cpu.CS], c.IP); a >= machine.VideoSeg*16 {
			why = fmt.Sprintf("跑到 %04X:%04X", c.Seg[cpu.CS], c.IP)
			break
		}
		// 播放器每叫一次 delay()，把參數記下來——**這是 `W` 與 delay 之間
		// 那一段的直接證據**：靜態讀出來的是 ⌊T×5÷100⌋+1，這裡看它實際傳什麼。
		if c.Seg[cpu.CS] == machine.LoadSeg && c.IP == ipDelay {
			ops = append(ops, byte(m.Read16(lin(c.Seg[cpu.SS], c.R[cpu.SP]+2))))
		}
		if err := m.Step(); err != nil {
			why = err.Error()
			break
		}
	}
	return bdaTicks(m) - t0, m.Steps - s0, why
}

// ops 是播放器每一次呼叫 delay() 的參數，call() 每跑一支就重來。
var ops []byte

// unit 是 delay 每一圈等幾個 BIOS tick（＝ `ds:3F6` ÷ 2），開機後才知道。
var unit uint16

// tickHz 是 BIOS 計時器的頻率。
const tickHz = 18.2065

func plural(n uint64) string { return fmt.Sprintf("（%d 道指令）", n) }

func note(why string) string {
	if why == "" {
		return ""
	}
	return "  ⚠ " + why
}

func bdaTicks(m *machine.Machine) uint32 {
	return uint32(m.Read16(0x46C)) | uint32(m.Read16(0x46E))<<16
}

func lin(seg, off uint16) uint32 { return cpu.Addr(seg, off) }

func str(m *machine.Machine, seg, off uint16, n int) string {
	b := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		ch := m.Read8(lin(seg, off+uint16(i)))
		if ch == 0 {
			break
		}
		b = append(b, ch)
	}
	return string(b)
}

func parse(s string) []uint16 {
	var out []uint16
	v, has := 0, false
	for i := 0; i <= len(s); i++ {
		if i < len(s) && s[i] >= '0' && s[i] <= '9' {
			v, has = v*10+int(s[i]-'0'), true
			continue
		}
		if has {
			out = append(out, uint16(v))
		}
		v, has = 0, false
	}
	return out
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
