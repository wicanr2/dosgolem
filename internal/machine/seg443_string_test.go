package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/443-segmented-32bit-code-and-stack §3 第 8、9 項：D=1 碼段的 32 位元定址字串指令與一步上限。
//
// 環境：d1Prog（程式在 70000h，堆疊在 1000h 起）；DS 基底 11000h、界限 2EFFFh；ES 基底 40000h、界限 1FFFFh；
// FS 基底 5F000h、界限 1FFFFh（只用偏移 10000h 以上）。ESI、EDI 起點都大於 FFFFh，證明以 32 位元定址。

const (
	strDS = 0x11000
	strES = 0x40000
	strFS = 0x5F000
)

func strProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := d1Prog(t, code)
	m.CPU.SetSegCache(cpu.DS, dataCache(strDS, 0x2EFFF, 0x93))
	m.CPU.SetSegCache(cpu.ES, dataCache(strES, 0x1FFFF, 0x93))
	m.CPU.SetSegCache(cpu.FS, dataCache(strFS, 0x1FFFF, 0x93))
	return m
}

// strRef 是獨立的參考：依 op、寬度、DF 算出一次迭代後的 ESI、EDI 與要比對的資料。
type strCase struct {
	name   string
	code   []byte
	size   uint32 // 1、2、4
	src    uint32 // 來源段基底（前綴覆寫時為 FS）
	df     bool
	moves  bool // movs
	cmp    bool // cmps
	stos   bool
	lods   bool
	scas   bool
	usesSI bool
	usesDI bool
}

func TestSeg443StringOnce(t *testing.T) {
	var cases []strCase
	for _, df := range []bool{false, true} {
		for _, w := range []struct {
			pre  []byte
			odd  bool
			size uint32
		}{{nil, false, 1}, {[]byte{0x66}, true, 2}, {nil, true, 4}} {
			for _, k := range []struct {
				name                         string
				op                           byte
				moves, cmp, stos, lods, scas bool
			}{
				{"movs", 0xA4, true, false, false, false, false},
				{"cmps", 0xA6, false, true, false, false, false},
				{"stos", 0xAA, false, false, true, false, false},
				{"lods", 0xAC, false, false, false, true, false},
				{"scas", 0xAE, false, false, false, false, true},
			} {
				if w.size == 2 && w.pre == nil {
					continue
				}
				op := k.op
				if w.odd {
					op |= 1
				}
				c := strCase{name: k.name, size: w.size, src: strDS, df: df, moves: k.moves, cmp: k.cmp, stos: k.stos,
					lods: k.lods, scas: k.scas, usesSI: k.moves || k.cmp || k.lods, usesDI: !k.lods}
				c.code = append(append([]byte{}, w.pre...), op)
				cases = append(cases, c)
				if c.usesSI { // 來源段前綴（FS）
					o := c
					o.name += "（64 前綴）"
					o.src = strFS
					o.code = append([]byte{0x64}, c.code...)
					cases = append(cases, o)
				}
				if c.stos || c.scas { // 目的段不可覆寫：64 前綴仍用 ES
					o := c
					o.name += "（64 前綴，目的仍為 ES）"
					o.code = append([]byte{0x64}, c.code...)
					cases = append(cases, o)
				}
			}
		}
	}
	const esi0, edi0 = 0x00010010, 0x00010020
	srcVal := []byte{0x05, 0x81, 0x22, 0x93}
	dstVal := []byte{0x07, 0x11, 0x22, 0x93}
	for _, c := range cases {
		m := strProg(t, c.code)
		f := uint16(0)
		if c.df {
			f = cpu.DF
		}
		m.CPU.SetFlags(f)
		setReg32(m, cpu.SI, esi0)
		setReg32(m, cpu.DI, edi0)
		setReg32(m, cpu.AX, 0x99887766)
		setReg32(m, cpu.CX, 0x00030005)
		copy(m.Mem[c.src+esi0:], srcVal)
		copy(m.Mem[strES+edi0:], dstVal)
		copy(m.Mem[strDS+esi0+0x100:], []byte{0xEE}) // 不相干的位置
		mustStep(t, m, c.name)
		n := int(c.size)
		d := c.size
		if c.df {
			d = -c.size
		}
		wantSI, wantDI := uint32(esi0), uint32(edi0)
		if c.usesSI {
			wantSI += d
		}
		if c.usesDI {
			wantDI += d
		}
		what := c.name + " % X DF=%v"
		if reg32(m.CPU, cpu.SI) != wantSI || reg32(m.CPU, cpu.DI) != wantDI || reg32(m.CPU, cpu.CX) != 0x00030005 {
			t.Errorf(what+"：ESI=%08X EDI=%08X ECX=%08X，應為 %08X %08X 00030005", c.code, c.df,
				reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), reg32(m.CPU, cpu.CX), wantSI, wantDI)
		}
		acc := []byte{0x66, 0x77, 0x88, 0x99}
		switch {
		case c.moves:
			want := append(append([]byte{}, srcVal[:n]...), dstVal[n:]...)
			if got := m.Mem[strES+edi0 : strES+edi0+4]; !bytes.Equal(got, want) {
				t.Errorf(what+"：ES:EDI=% X，應為 % X", c.code, c.df, got, want)
			}
		case c.stos:
			want := append(append([]byte{}, acc[:n]...), dstVal[n:]...)
			if got := m.Mem[strES+edi0 : strES+edi0+4]; !bytes.Equal(got, want) {
				t.Errorf(what+"：ES:EDI=% X，應為 % X", c.code, c.df, got, want)
			}
			if m.Mem[strFS+edi0&0xFFFF] != 0 {
				t.Errorf(what+"：不得寫 FS", c.code, c.df)
			}
		case c.lods:
			want := uint32(0x99887766)
			for i := 0; i < n; i++ {
				want = want&^(0xFF<<(8*i)) | uint32(srcVal[i])<<(8*i)
			}
			if reg32(m.CPU, cpu.AX) != want {
				t.Errorf(what+"：EAX=%08X，應為 %08X", c.code, c.df, reg32(m.CPU, cpu.AX), want)
			}
		case c.cmp, c.scas:
			// 參考：以 n bytes 寬的無號比較算 CF、ZF（[ESI]／累加器 − [EDI]）。
			var a, b uint32
			for i := 0; i < n; i++ {
				b |= uint32(dstVal[i]) << (8 * i)
				if c.cmp {
					a |= uint32(srcVal[i]) << (8 * i)
				} else {
					a |= uint32(acc[i]) << (8 * i)
				}
			}
			if m.CPU.Flag(cpu.CF) != (a < b) || m.CPU.Flag(cpu.ZF) != (a == b) {
				t.Errorf(what+"：CF=%v ZF=%v，應為 %v %v", c.code, c.df, m.CPU.Flag(cpu.CF), m.CPU.Flag(cpu.ZF), a < b, a == b)
			}
		}
	}
}

func TestSeg443StringRep(t *testing.T) {
	// rep movsb：ECX 高半非 0（10003h 次），DF=0。
	m := strProg(t, []byte{0xF3, 0xA4})
	setReg32(m, cpu.SI, 0x00010000)
	setReg32(m, cpu.DI, 0x00002000)
	setReg32(m, cpu.CX, 0x00010003)
	for i := uint32(0); i < 0x10003; i++ {
		m.Mem[strDS+0x10000+i] = byte(i*7 + i>>8)
	}
	mustStep(t, m, "rep movsb ECX=10003h")
	if reg32(m.CPU, cpu.CX) != 0 || reg32(m.CPU, cpu.SI) != 0x20003 || reg32(m.CPU, cpu.DI) != 0x12003 || m.CPU.IP != 2 {
		t.Errorf("rep movsb：ECX=%08X ESI=%08X EDI=%08X IP=%04X", reg32(m.CPU, cpu.CX), reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), m.CPU.IP)
	}
	if !bytes.Equal(m.Mem[strES+0x2000:strES+0x12003], m.Mem[strDS+0x10000:strDS+0x20003]) {
		t.Error("rep movsb：目的內容不等於來源")
	}
	// rep movsd，DF=1。
	m = strProg(t, []byte{0xF3, 0xA5})
	m.CPU.SetFlags(cpu.DF)
	setReg32(m, cpu.SI, 0x00010100)
	setReg32(m, cpu.DI, 0x00010200)
	setReg32(m, cpu.CX, 3)
	for i := uint32(0); i < 16; i++ {
		m.Mem[strDS+0x100F8+i] = byte(0xA0 + i)
	}
	mustStep(t, m, "rep movsd DF=1")
	if reg32(m.CPU, cpu.SI) != 0x100F4 || reg32(m.CPU, cpu.DI) != 0x101F4 || reg32(m.CPU, cpu.CX) != 0 {
		t.Errorf("rep movsd DF=1：ESI=%08X EDI=%08X ECX=%08X", reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), reg32(m.CPU, cpu.CX))
	}
	if !bytes.Equal(m.Mem[strES+0x101F8:strES+0x10204], m.Mem[strDS+0x100F8:strDS+0x10104]) {
		t.Error("rep movsd DF=1：目的內容不對")
	}
	// rep 的 ECX=0：什麼都不做。
	m = strProg(t, []byte{0xF3, 0xAA})
	setReg32(m, cpu.DI, 0x10000)
	setReg32(m, cpu.CX, 0)
	mustStep(t, m, "rep stosb ECX=0")
	if reg32(m.CPU, cpu.DI) != 0x10000 || m.Mem[strES+0x10000] != 0 {
		t.Error("rep ECX=0：不得執行任何迭代")
	}
	// repe cmpsb 提前結束：前 3 bytes 相同，第 4 個不同。ECX 高半非 0。
	m = strProg(t, []byte{0xF3, 0xA6})
	setReg32(m, cpu.SI, 0x10000)
	setReg32(m, cpu.DI, 0x10000)
	setReg32(m, cpu.CX, 0x00020000)
	copy(m.Mem[strDS+0x10000:], []byte{1, 2, 3, 4, 5})
	copy(m.Mem[strES+0x10000:], []byte{1, 2, 3, 9, 5})
	mustStep(t, m, "repe cmpsb")
	if reg32(m.CPU, cpu.CX) != 0x1FFFC || reg32(m.CPU, cpu.SI) != 0x10004 || m.CPU.Flag(cpu.ZF) {
		t.Errorf("repe cmpsb：ECX=%08X ESI=%08X ZF=%v，應為 1FFFC 10004 false", reg32(m.CPU, cpu.CX), reg32(m.CPU, cpu.SI), m.CPU.Flag(cpu.ZF))
	}
	// repne scasd：第 3 個 dword 等於 EAX。
	m = strProg(t, []byte{0xF2, 0xAF})
	setReg32(m, cpu.DI, 0x10000)
	setReg32(m, cpu.CX, 10)
	setReg32(m, cpu.AX, 0xCAFEBABE)
	copy(m.Mem[strES+0x10008:], []byte{0xBE, 0xBA, 0xFE, 0xCA})
	mustStep(t, m, "repne scasd")
	if reg32(m.CPU, cpu.CX) != 7 || reg32(m.CPU, cpu.DI) != 0x1000C || !m.CPU.Flag(cpu.ZF) {
		t.Errorf("repne scasd：ECX=%d EDI=%08X ZF=%v，應為 7 1000C true", reg32(m.CPU, cpu.CX), reg32(m.CPU, cpu.DI), m.CPU.Flag(cpu.ZF))
	}
	// DOSBoxCost：每次迭代 Cycles 加 1（另加本道的 1）。
	m = strProg(t, []byte{0xF3, 0xA4})
	m.CPU.DOSBoxCost = true
	setReg32(m, cpu.SI, 0x10000)
	setReg32(m, cpu.DI, 0x10000)
	setReg32(m, cpu.CX, 5)
	before := m.CPU.Cycles
	mustStep(t, m, "DOSBoxCost rep movsb")
	if got := m.CPU.Cycles - before; got != 6 {
		t.Errorf("DOSBoxCost：Cycles 增加 %d，應為 6（1＋5 次迭代）", got)
	}
}

// 第 k 次迭代失敗：前 k−1 次已寫，第 k 次未寫，ESI、EDI、ECX 停在第 k 次之前。
func TestSeg443StringFault(t *testing.T) {
	const k = 4
	for _, c := range []struct {
		name  string
		setup func(*Machine)
		want  string
	}{
		{"目的超出 ES 界限", func(m *Machine) { m.CPU.SetSegCache(cpu.ES, dataCache(strES, 0x1000B, 0x93)) }, "超出界限"},
		{"來源超出 DS 界限", func(m *Machine) { m.CPU.SetSegCache(cpu.DS, dataCache(strDS, 0x1000D, 0x93)) }, "超出界限"},
	} {
		m := strProg(t, []byte{0xF3, 0xA5})
		c.setup(m)
		setReg32(m, cpu.SI, 0x10000)
		setReg32(m, cpu.DI, 0x10000)
		setReg32(m, cpu.CX, 10)
		for i := 0; i < 40; i++ {
			m.Mem[strDS+0x10000+uint32(i)] = byte(0x40 + i)
			m.Mem[strES+0x10000+uint32(i)] = 0xEE // 哨兵：第 k 次迭代不得寫入（含寫入 0）
		}
		wantStepErr(t, m, c.name, c.want)
		done := uint32(4 * (k - 1))
		if !bytes.Equal(m.Mem[strES+0x10000:strES+0x10000+done], m.Mem[strDS+0x10000:strDS+0x10000+done]) {
			t.Errorf("%s：前 %d 次迭代應已寫入", c.name, k-1)
		}
		if got := m.Mem[strES+0x10000+done : strES+0x10000+done+4]; !bytes.Equal(got, []byte{0xEE, 0xEE, 0xEE, 0xEE}) {
			t.Errorf("%s：第 %d 次迭代不得寫入：% X", c.name, k, got)
		}
		if reg32(m.CPU, cpu.SI) != 0x10000+done || reg32(m.CPU, cpu.DI) != 0x10000+done || reg32(m.CPU, cpu.CX) != 10-(k-1) {
			t.Errorf("%s：ESI=%08X EDI=%08X ECX=%d，應停在第 %d 次之前", c.name, reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), reg32(m.CPU, cpu.CX), k)
		}
	}
	// cmps 讀失敗時旗標不變。
	m := strProg(t, []byte{0xA6})
	m.CPU.SetSegCache(cpu.DS, dataCache(strDS, 0xFFFF, 0x93))
	setReg32(m, cpu.SI, 0x10000)
	setReg32(m, cpu.DI, 0x10000)
	m.Mem[strES+0x10000] = 0x01
	m.CPU.SetFlags(cpu.ZF | cpu.SF)
	flags := m.CPU.Flags
	wantStepErr(t, m, "cmpsb 來源讀失敗", "超出界限")
	if m.CPU.Flags != flags {
		t.Errorf("cmps 讀失敗：旗標 %04X→%04X 不得改變", flags, m.CPU.Flags)
	}
}

// 一步上限（§2.5）：單道 rep 在一次 Step 內最多 2^20 次迭代。
func TestSeg443RepStepLimit(t *testing.T) {
	big := func(t *testing.T, code []byte) *Machine {
		m := d1Prog(t, code)
		m.SetA20(true)
		m.CPU.SetSegCache(cpu.DS, dataCache(0x100000, 0x1FFFFF, 0x93))
		m.CPU.SetSegCache(cpu.ES, dataCache(0x300000, 0x1FFFFF, 0x93))
		setReg32(m, cpu.SI, 0)
		setReg32(m, cpu.DI, 0)
		return m
	}
	const lim = 1 << 20
	// rep movsb ECX=100001h：第一步後 ECX=1、ESI／EDI 前進 2^20、IP=opIP；第二步完成。
	m := big(t, []byte{0xF3, 0xA4})
	setReg32(m, cpu.CX, lim+1)
	m.Ext[0x100000-ExtBase+0x00] = 0x11
	m.Ext[0x100000-ExtBase+lim-1] = 0x22
	m.Ext[0x100000-ExtBase+lim] = 0x33
	steps := m.Steps
	mustStep(t, m, "rep movsb 第一步")
	if reg32(m.CPU, cpu.CX) != 1 || reg32(m.CPU, cpu.SI) != lim || reg32(m.CPU, cpu.DI) != lim || m.CPU.IP != 0 || m.Steps != steps+1 {
		t.Errorf("第一步：ECX=%X ESI=%X EDI=%X IP=%04X Steps+%d，應為 1、%X、%X、0000、1", reg32(m.CPU, cpu.CX),
			reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), m.CPU.IP, m.Steps-steps, lim, lim)
	}
	if m.Ext[0x300000-ExtBase] != 0x11 || m.Ext[0x300000-ExtBase+lim-1] != 0x22 || m.Ext[0x300000-ExtBase+lim] != 0 {
		t.Error("第一步：已完成的 2^20 次迭代應已提交，第 2^20＋1 次尚未執行")
	}
	mustStep(t, m, "rep movsb 第二步")
	if reg32(m.CPU, cpu.CX) != 0 || m.CPU.IP != 2 || m.Ext[0x300000-ExtBase+lim] != 0x33 {
		t.Errorf("第二步：ECX=%X IP=%04X，應為 0、0002", reg32(m.CPU, cpu.CX), m.CPU.IP)
	}
	// repe cmpsb：第 2^20 次迭代不同，條件已不成立，不續跑。
	m = big(t, []byte{0xF3, 0xA6})
	setReg32(m, cpu.CX, lim+1)
	m.Ext[0x100000-ExtBase+lim-1] = 1
	mustStep(t, m, "repe cmpsb 於上限處結束")
	if reg32(m.CPU, cpu.CX) != 1 || m.CPU.IP != 2 || m.CPU.Flag(cpu.ZF) {
		t.Errorf("repe cmpsb：ECX=%X IP=%04X ZF=%v，應為 1、0002、false", reg32(m.CPU, cpu.CX), m.CPU.IP, m.CPU.Flag(cpu.ZF))
	}
	// repe cmpsb：條件仍成立時續跑。
	m = big(t, []byte{0xF3, 0xA6})
	setReg32(m, cpu.CX, lim+1)
	mustStep(t, m, "repe cmpsb 續跑")
	if reg32(m.CPU, cpu.CX) != 1 || m.CPU.IP != 0 {
		t.Errorf("repe cmpsb 續跑：ECX=%X IP=%04X，應為 1、0000", reg32(m.CPU, cpu.CX), m.CPU.IP)
	}
	// ECX ≤ 2^20：一道一步。
	m = big(t, []byte{0xF3, 0xA4})
	setReg32(m, cpu.CX, lim)
	mustStep(t, m, "rep movsb ECX=2^20")
	if reg32(m.CPU, cpu.CX) != 0 || m.CPU.IP != 2 {
		t.Errorf("ECX=2^20：ECX=%X IP=%04X，應為 0、0002", reg32(m.CPU, cpu.CX), m.CPU.IP)
	}
}
