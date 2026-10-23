package machine

import (
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/435-v86-hooks-and-idt-delivery §7 第 2、3、6 項（Machine 側）：合成 GDT、IDT、TSS（仿 §1.1）。
//
//	GDT（segGDT，界限 FFh）：18h 16 位元碼段 基底 70000h；50h 資料 可寫 向下展開 界限 0 基底 50000h（SS0）；
//	                         58h 資料 可寫 向上展開 界限 FFFFh 基底 50000h；10h 386 TSS（忙碌）基底 82000h。
//	IDT（84000h，界限 7FFh）：256 項 386 中斷閘、DPL 0、選擇子 18h、偏移 47DEh+4n；處理常式區填 NOP。
//	TSS：SS0:ESP0 = 50h:1400h，I/O 點陣圖位移 68h（全 0）。
//	V86：程式在 PSP:0100，SS:SP=3000:00D2，DS=01EAh、ES=2000h、FS=28h、GS=30h，IOPL=3、IF=1。

const (
	dIDT     = 0x84000
	dTSS     = 0x82000
	dSS0Base = 0x50000
	dESP0    = 0x1400
	dGateOff = 0x47DE
	dV86SS   = 0x3000
	dV86SP   = 0x00D2
)

func gateOff(vec uint8) uint16 { return dGateOff + 4*uint16(vec) }

// putGate 寫 IDT 第 vec 項：hiFlags 是 P、DPL、型別（例如 8E00h）。
func putGate(m *Machine, vec uint8, sel uint16, off uint32, hiFlags uint32) {
	a := uint32(dIDT) + uint32(vec)*8
	put32(m, a, off&0xFFFF|uint32(sel)<<16)
	put32(m, a+4, off&0xFFFF0000|hiFlags)
}

// installDeliverEnv 在 PE=1 的機器上裝好 GDT、IDT、TSS、TR，並設好 V86 狀態（VM=1）。
func installDeliverEnv(t *testing.T, m *Machine) {
	t.Helper()
	if m.CR0()&cr0PE == 0 {
		mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	}
	putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0x9A, 0)
	putDesc(m, segGDT, 0x50, dSS0Base, 0, 0x96, 0)
	putDesc(m, segGDT, 0x58, dSS0Base, 0xFFFF, 0x92, 0)
	putDesc(m, segGDT, 0x10, dTSS, 0x2068, 0x8B, 0)
	m.SetGDTR(segGDT, 0xFF)
	for v := 0; v < 256; v++ {
		putGate(m, uint8(v), 0x18, uint32(gateOff(uint8(v))), 0x8E00)
	}
	m.SetIDTR(dIDT, 0x7FF)
	for i := uint32(0); i < 0x404; i++ {
		m.Mem[segCode+dGateOff+i] = 0x90
	}
	for i := uint32(0); i < 0x68; i++ {
		m.Mem[dTSS+i] = 0
	}
	put32(m, dTSS+4, dESP0)
	put32(m, dTSS+8, 0x50)
	put32(m, dTSS+0x64, 0x68<<16)
	m.SetTR(0x10, SegCache{Base: dTSS, Limit: 0x2068, Attr: 0x8B})
	mustNil(t, "SetVM", m.SetVM(true))
	c := m.CPU
	c.SetFlags(0x3000 | cpu.IF)
	c.Seg[cpu.SS], c.R[cpu.SP], c.Hi[cpu.SP] = dV86SS, dV86SP, 0
	c.Seg[cpu.DS], c.Seg[cpu.ES], c.FSeg, c.GSeg = 0x01EA, 0x2000, 0x28, 0x30
}

func deliverProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newSysProg(t, code)
	installDeliverEnv(t, m)
	return m
}

// frameAt 讀推入的第 k 個 dword（k 從 0 起：GS、FS、DS、ES、SS、ESP、EFLAGS、CS、EIP、錯誤碼）。
func frameAt(m *Machine, base, esp0 uint32, k int) uint32 {
	return get32(m, base+uint32(uint16(esp0-4*uint32(k+1))))
}

// wantEntered 檢查進入 ring 0 之後的狀態與框架（esp0=1400h、SS0=50h）。
func wantEntered(t *testing.T, m *Machine, what string, vec uint8, hasErr bool, errCode, frameEIP uint32,
	before cpuSnap, ipAfter uint16, ifCleared bool) {
	t.Helper()
	c := m.CPU
	n := 9
	if hasErr {
		n = 10
	}
	want := []uint32{uint32(before.gs), uint32(before.fs), uint32(before.seg[cpu.DS]), uint32(before.seg[cpu.ES]),
		uint32(before.seg[cpu.SS]), uint32(before.hi[cpu.SP])<<16 | uint32(before.r[cpu.SP]),
		uint32(before.fl) | uint32(before.efHi)<<16, uint32(before.seg[cpu.CS]), frameEIP, errCode}
	for k := 0; k < n; k++ {
		if got := frameAt(m, dSS0Base, dESP0, k); got != want[k] {
			t.Errorf("%s：框架第 %d 個 dword %08X，應為 %08X", what, k, got, want[k])
		}
	}
	if !c.Segmented() || m.VM() || m.PMCPL() != 0 || m.User() {
		t.Errorf("%s：應進分段模式 CPL 0、VM=0：segmented=%v VM=%v CPL=%d", what, c.Segmented(), m.VM(), m.PMCPL())
	}
	if c.Seg[cpu.CS] != 0x18 || c.IP != ipAfter || c.SegCache(cpu.CS).Base != segCode {
		t.Errorf("%s：CS:IP=%04X:%04X（基底 %X），應為 0018:%04X", what, c.Seg[cpu.CS], c.IP, c.SegCache(cpu.CS).Base, ipAfter)
	}
	if c.Seg[cpu.SS] != 0x50 || c.SegCache(cpu.SS).Base != dSS0Base ||
		uint32(c.Hi[cpu.SP])<<16|uint32(c.R[cpu.SP]) != dESP0-4*uint32(n) {
		t.Errorf("%s：SS:ESP=%04X:%04X%04X，應為 0050:%08X", what, c.Seg[cpu.SS], c.Hi[cpu.SP], c.R[cpu.SP], dESP0-4*uint32(n))
	}
	if c.Seg[cpu.DS] != 0 || c.Seg[cpu.ES] != 0 || c.FSeg != 0 || c.GSeg != 0 {
		t.Errorf("%s：DS、ES、FS、GS 應為空選擇子：%04X %04X %04X %04X", what, c.Seg[cpu.DS], c.Seg[cpu.ES], c.FSeg, c.GSeg)
	}
	for _, i := range []int{cpu.DS, cpu.ES, cpu.FS, cpu.GS} {
		if c.SegCache(i).Attr&attrP != 0 {
			t.Errorf("%s：段 %d 的快取應不可用", what, i)
		}
	}
	if c.Flag(cpu.IF) == ifCleared || c.Flag(cpu.TF) || c.Flags&cpu.NT != 0 {
		t.Errorf("%s：旗標 %04X（IF 應%s、TF、NT 清）", what, c.Flags, map[bool]string{true: "清", false: "保留"}[ifCleared])
	}
	if c.Flags&0x3000 != before.fl&0x3000 {
		t.Errorf("%s：IOPL 不得改變", what)
	}
}

// cpuSnap 是遞送前後要比對的狀態。
type cpuSnap struct {
	r, hi    [8]uint16
	seg      [4]uint16
	fs, gs   uint16
	ip, fl   uint16
	seg2     bool
	efHi     uint16
	cpl      uint8
	stack    string
	pending  bool
	ticks    uint64
	keys     int
	gdt, idt string
}

func snap(m *Machine) cpuSnap {
	c := m.CPU
	return cpuSnap{r: c.R, hi: c.Hi, seg: c.Seg, fs: c.FSeg, gs: c.GSeg, ip: c.IP, fl: c.Flags, seg2: c.Segmented(),
		efHi: m.EFlagsHi(), cpl: m.PMCPL(), stack: string(m.Mem[dSS0Base : dSS0Base+0x10000]),
		pending: m.irq0Pending, ticks: m.Ticks, keys: len(m.keyQueue),
		gdt: string(m.Mem[segGDT : segGDT+0x100]), idt: string(m.Mem[dIDT : dIDT+0x800])}
}

func TestV86DeliverSoftInt(t *testing.T) {
	// CD 21、閘 DPL 0：#GP(21h*8+2) 送向量 13，框架 EIP 為 CD 起點，推入後 SP=ESP0-40。
	m := deliverProg(t, []byte{0xCD, 0x21})
	b := snap(m)
	mustStep(t, m, "CD 21（閘 DPL 0）")
	wantEntered(t, m, "CD 21（閘 DPL 0）", 13, true, 0x21*8+2, 0x100, b, gateOff(13), true)

	// 閘 DPL 3：直接送向量 nn，框架 EIP 為下一道，無錯誤碼。
	m = deliverProg(t, []byte{0xCD, 0x21})
	putGate(m, 0x21, 0x18, uint32(gateOff(0x21)), 0xEE00)
	b = snap(m)
	mustStep(t, m, "CD 21（閘 DPL 3）")
	wantEntered(t, m, "CD 21（閘 DPL 3）", 0x21, false, 0, 0x102, b, gateOff(0x21), true)

	// IOPL<3 的 CD nn、INTO：#GP(0)，框架 EIP 為起點。
	for _, x := range []struct {
		name string
		code []byte
	}{{"CD 21", []byte{0xCD, 0x21}}, {"INTO", []byte{0xCE}}} {
		m = deliverProg(t, x.code)
		putGate(m, 0x21, 0x18, uint32(gateOff(0x21)), 0xEE00)
		putGate(m, 4, 0x18, uint32(gateOff(4)), 0xEE00)
		m.CPU.SetFlags(cpu.IF | cpu.OF) // IOPL 0
		b = snap(m)
		mustStep(t, m, "IOPL 0 的 "+x.name)
		wantEntered(t, m, "IOPL 0 的 "+x.name, 13, true, 0, 0x100, b, gateOff(13), true)
	}

	// IOPL<3 的 INT 3（CC）不送 #GP(0)：閘 DPL 3 時直接送向量 3（框架 EIP 下一道）；閘 DPL 0 時 #GP(3*8+2)。
	m = deliverProg(t, []byte{0xCC})
	putGate(m, 3, 0x18, uint32(gateOff(3)), 0xEE00)
	m.CPU.SetFlags(cpu.IF)
	b = snap(m)
	mustStep(t, m, "IOPL 0 的 INT 3（DPL 3）")
	wantEntered(t, m, "IOPL 0 的 INT 3（DPL 3）", 3, false, 0, 0x101, b, gateOff(3), true)
	m = deliverProg(t, []byte{0xCC})
	m.CPU.SetFlags(cpu.IF)
	b = snap(m)
	mustStep(t, m, "IOPL 0 的 INT 3（DPL 0）")
	wantEntered(t, m, "IOPL 0 的 INT 3（DPL 0）", 13, true, 3*8+2, 0x100, b, gateOff(13), true)

	// 陷阱閘不清 IF。
	m = deliverProg(t, []byte{0xCD, 0x21})
	putGate(m, 13, 0x18, uint32(gateOff(13)), 0x8F00)
	b = snap(m)
	mustStep(t, m, "陷阱閘")
	wantEntered(t, m, "陷阱閘", 13, true, 0x21*8+2, 0x100, b, gateOff(13), false)
}

func TestV86DeliverExceptions(t *testing.T) {
	cases := []struct {
		name string
		code []byte
		vec  uint8
		prep func(m *Machine)
	}{
		{"div bl", []byte{0xF6, 0xF3}, 0, func(m *Machine) { m.CPU.R[cpu.AX] = 0x1234 }},
		{"div bx 商溢位", []byte{0xF7, 0xF3}, 0, func(m *Machine) { m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = 5, 1 }},
		{"idiv bl", []byte{0xF6, 0xFB}, 0, nil},
		{"aam 0", []byte{0xD4, 0x00}, 0, func(m *Machine) { m.CPU.R[cpu.AX] = 0xFF }},
		{"66 div ebx", []byte{0x66, 0xF7, 0xF3}, 0, nil},
		{"bound 越界", []byte{0x62, 0x06, 0x00, 0x02}, 5, func(m *Machine) {
			m.CPU.R[cpu.AX] = 5
			copy(m.Mem[uint32(m.CPU.Seg[cpu.DS])*16+0x200:], []byte{0, 0, 1, 0})
		}},
	}
	for _, x := range cases {
		m := deliverProg(t, x.code)
		m.CPU.SetFlags(m.CPU.Flags | cpu.CF | cpu.SF | cpu.OF)
		if x.prep != nil {
			x.prep(m)
		}
		b := snap(m)
		mustStep(t, m, x.name)
		// 框架內 EFLAGS 等於指令前的旗標（wantEntered 以 before.fl 比對）。
		wantEntered(t, m, x.name, x.vec, false, 0, 0x100, b, gateOff(x.vec), true)
	}
	// #GP(0)（IOPL<3 的 cli）：錯誤碼 0，框架 EIP 為起點。
	m := deliverProg(t, []byte{0xFA})
	m.CPU.SetFlags(cpu.IF)
	b := snap(m)
	mustStep(t, m, "IOPL 0 的 cli")
	wantEntered(t, m, "IOPL 0 的 cli", 13, true, 0, 0x100, b, gateOff(13), true)
}

func TestV86DeliverIRQ(t *testing.T) {
	// IRQ0：向量 8，無錯誤碼，框架 EIP 為遞送當下的 IP；同一個 Step 接著執行 ring 0 的第一道（NOP）。
	m := deliverProg(t, []byte{0x90, 0x90})
	m.irq0Pending, m.picMask = true, 0
	ticks := 0
	m.SetOnTick(func() { ticks++ })
	b := snap(m)
	mustStep(t, m, "IRQ0")
	wantEntered(t, m, "IRQ0", 8, false, 0, 0x100, b, gateOff(8)+1, true)
	if m.irq0Pending || m.Ticks != b.ticks+1 || ticks != 1 {
		t.Errorf("IRQ0：應消耗 irq0Pending、Ticks+1、onTick 一次：%v %d %d", m.irq0Pending, m.Ticks, ticks)
	}
	// 陷阱閘不清 IF。
	m = deliverProg(t, []byte{0x90})
	putGate(m, 8, 0x18, uint32(gateOff(8)), 0x8F00)
	m.irq0Pending, m.picMask = true, 0
	b = snap(m)
	mustStep(t, m, "IRQ0（陷阱閘）")
	wantEntered(t, m, "IRQ0（陷阱閘）", 8, false, 0, 0x100, b, gateOff(8)+1, false)

	// IRQ1（keyTick、timedKeyTick）：向量 9。
	for _, timed := range []bool{false, true} {
		m = deliverProg(t, []byte{0x90})
		m.WritePhys16(0x09*4+2, 0x1234) // 向量不指 stub
		m.picMask = 0
		if timed {
			m.ScheduleKey(0, 0x1E, false)
		} else {
			m.QueueScan(0x1E)
			m.KeyEvery, m.nextKey = 1, 0
		}
		b = snap(m)
		mustStep(t, m, "IRQ1")
		wantEntered(t, m, "IRQ1", 9, false, 0, 0x100, b, gateOff(9)+1, true)
		if m.KeyIRQs != 1 || m.kbdData != 0x1E {
			t.Errorf("IRQ1（timed=%v）：KeyIRQs=%d kbdData=%02X", timed, m.KeyIRQs, m.kbdData)
		}
	}

	// IF=0、picMask 遮蔽：延後不消耗。
	for _, x := range []struct {
		name string
		prep func(m *Machine)
	}{
		{"IF=0", func(m *Machine) { m.CPU.SetFlags(0x3000) }},
		{"picMask", func(m *Machine) { m.picMask = 0x01 }},
	} {
		m = deliverProg(t, []byte{0x90, 0x90})
		m.irq0Pending, m.picMask = true, 0
		x.prep(m)
		mustStep(t, m, x.name)
		if !m.irq0Pending || m.CPU.Segmented() || m.Ticks != 0 {
			t.Errorf("%s：應延後：pending=%v segmented=%v", x.name, m.irq0Pending, m.CPU.Segmented())
		}
	}
}

// V86 的 ESP=ABCD1234h 時送 IRQ0：進入後 ESP=ESP0-36（高半取自 ESP0），處理常式以 67 前綴的 [esp+4] 讀到框架的 CS。
func TestV86DeliverESPHighHalf(t *testing.T) {
	m := deliverProg(t, []byte{0x90})
	m.CPU.R[cpu.SP], m.CPU.Hi[cpu.SP] = 0x1234, 0xABCD
	copy(m.Mem[segCode+uint32(gateOff(8)):], []byte{0x67, 0x8B, 0x44, 0x24, 0x04}) // mov ax,[esp+4]
	m.irq0Pending, m.picMask = true, 0
	b := snap(m)
	mustStep(t, m, "IRQ0（ESP 高半）")
	wantEntered(t, m, "IRQ0（ESP 高半）", 8, false, 0, 0x100, b, gateOff(8)+5, true)
	if m.CPU.R[cpu.AX] != b.seg[cpu.CS] {
		t.Errorf("[esp+4] 應讀到框架的 CS %04X，實際 %04X", b.seg[cpu.CS], m.CPU.R[cpu.AX])
	}
}

// 推入跨過 SP=0 環繞：SS0=58h（向上展開），ESP0=10h，每個 dword 在 SS0.base+uint16(ESP0-4(k+1))。
func TestV86DeliverStackWrap(t *testing.T) {
	m := deliverProg(t, []byte{0x90})
	put32(m, dTSS+4, 0x10)
	put32(m, dTSS+8, 0x58)
	m.irq0Pending, m.picMask = true, 0
	b := snap(m)
	mustStep(t, m, "IRQ0（環繞）")
	want := []uint32{uint32(b.gs), uint32(b.fs), uint32(b.seg[cpu.DS]), uint32(b.seg[cpu.ES]), uint32(b.seg[cpu.SS]),
		uint32(b.r[cpu.SP]), uint32(b.fl) | uint32(b.efHi)<<16, uint32(b.seg[cpu.CS]), 0x100}
	for k, off := range []uint32{0x0C, 0x08, 0x04, 0x00, 0xFFFC, 0xFFF8, 0xFFF4, 0xFFF0, 0xFFEC} {
		if got := get32(m, dSS0Base+off); got != want[k] {
			t.Errorf("第 %d 個 dword（偏移 %04X）=%08X，應為 %08X", k, off, got, want[k])
		}
	}
	if m.CPU.R[cpu.SP] != 0xFFEC || m.CPU.Hi[cpu.SP] != 0 {
		t.Errorf("SP=%04X%04X，應為 0000FFEC", m.CPU.Hi[cpu.SP], m.CPU.R[cpu.SP])
	}
}

// prepare 每個失敗條件各一例：失敗即關閉、狀態不變（CD 21 在閘 DPL 0 時送向量 13）。
func TestV86DeliverPrepareFailures(t *testing.T) {
	g13 := func(m *Machine, sel uint16, off uint32, hi uint32) { putGate(m, 13, sel, off, hi) }
	o13 := uint32(gateOff(13))
	cases := []struct {
		name     string
		prep     func(m *Machine)
		contains string
	}{
		{"IDT 界限", func(m *Machine) { m.SetIDTR(dIDT, 0x60) }, "IDT 界限"},
		{"286 中斷閘", func(m *Machine) { g13(m, 0x18, o13, 0x8600) }, "閘型別"},
		{"任務閘", func(m *Machine) { g13(m, 0x18, o13, 0x8500) }, "閘型別"},
		{"閘不存在", func(m *Machine) { g13(m, 0x18, o13, 0x0E00) }, "P=0"},
		{"閘選擇子為空", func(m *Machine) { g13(m, 0, o13, 0x8E00) }, "空選擇子"},
		{"閘選擇子 TI=1", func(m *Machine) { g13(m, 0x1C, o13, 0x8E00) }, "TI=1"},
		{"閘選擇子超出 GDT", func(m *Machine) { g13(m, 0x100, o13, 0x8E00) }, "超出 GDT"},
		{"目標不是碼段", func(m *Machine) { g13(m, 0x58, o13, 0x8E00) }, "不是碼段"},
		{"一致碼段", func(m *Machine) { putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0x9E, 0) }, "一致"},
		{"碼段不存在", func(m *Machine) { putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0x1A, 0) }, "不存在"},
		{"碼段 DPL 3", func(m *Machine) { putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0xFA, 0) }, "DPL"},
		{"碼段 D=1", func(m *Machine) { putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0x9A, 4) }, "32 位元碼段"},
		{"閘偏移高半", func(m *Machine) { g13(m, 0x18, o13|0x10000, 0x8E00) }, "高 16"},
		{"TR 無效", func(m *Machine) { m.SetTR(0, SegCache{}) }, "TR"},
		{"SS0 為空", func(m *Machine) { put32(m, dTSS+8, 0) }, "空選擇子"},
		{"SS0 TI=1", func(m *Machine) { put32(m, dTSS+8, 0x54) }, "TI=1"},
		{"SS0 超出 GDT", func(m *Machine) { put32(m, dTSS+8, 0x100) }, "超出 GDT"},
		{"SS0 RPL", func(m *Machine) { put32(m, dTSS+8, 0x51) }, "RPL"},
		{"SS0 DPL", func(m *Machine) { putDesc(m, segGDT, 0x50, dSS0Base, 0, 0xF6, 0) }, "DPL"},
		{"SS0 唯讀", func(m *Machine) { putDesc(m, segGDT, 0x50, dSS0Base, 0, 0x94, 0) }, "可寫資料段"},
		{"SS0 是碼段", func(m *Machine) { put32(m, dTSS+8, 0x18) }, "可寫資料段"},
		{"SS0 不存在", func(m *Machine) { putDesc(m, segGDT, 0x50, dSS0Base, 0, 0x16, 0) }, "不存在"},
		{"SS0 B=1", func(m *Machine) { putDesc(m, segGDT, 0x50, dSS0Base, 0, 0x96, 4) }, "B=1"},
		{"推入超出界限", func(m *Machine) { put32(m, dTSS+4, 0x10) }, "超出界限"},
	}
	for _, x := range cases {
		m := deliverProg(t, []byte{0xCD, 0x21})
		x.prep(m)
		b := snap(m)
		err := m.Step()
		if err == nil || !strings.Contains(err.Error(), x.contains) {
			t.Errorf("%s：應失敗（含 %q），實際 %v", x.name, x.contains, err)
			continue
		}
		a := snap(m)
		if a != b {
			t.Errorf("%s：失敗時狀態不得改變", x.name)
		}
	}
	// IRQ 路徑：失敗時存 irqErr、不消耗（irq0Pending、Ticks），Steps 不加。
	m := deliverProg(t, []byte{0x90})
	m.SetIDTR(dIDT, 0x40)
	m.irq0Pending, m.picMask = true, 0
	b, steps := snap(m), m.Steps
	err := m.Step()
	if err == nil || !strings.Contains(err.Error(), "IDT 界限") || snap(m) != b || m.Steps != steps {
		t.Errorf("IRQ0 的 prepare 失敗：%v（狀態應不變）", err)
	}
}

// 分頁：prepare 讀 IDT 遇到不存在頁回 *PageFaultError（軟體中斷與 IRQ 路徑）；推入目標頁不存在失敗即關閉。
func TestV86DeliverPaging(t *testing.T) {
	setup := func(code []byte) (*Machine, *tables) {
		m, tb := newPagedProg(t, code, fPWU)
		installDeliverEnv(t, m)
		return m, tb
	}
	for _, irq := range []bool{false, true} {
		m, tb := setup([]byte{0xCD, 0x21, 0x90})
		tb.setPTE(dIDT, 0) // IDT 頁不存在
		if irq {
			m.CPU.IP = 0x102
			m.irq0Pending, m.picMask = true, 0
		}
		b := snap(m)
		err := m.Step()
		var pfe *PageFaultError
		if !errors.As(err, &pfe) || pfe.Record.Linear>>12 != dIDT>>12 {
			t.Errorf("irq=%v：讀 IDT 遇到不存在頁應回 *PageFaultError：%v", irq, err)
		}
		if snap(m) != b {
			t.Errorf("irq=%v：失敗時狀態不得改變", irq)
		}
	}
	m, tb := setup([]byte{0xCD, 0x21})
	tb.setPTE(dSS0Base+0x1000, 0) // 推入目標 51000h 起的頁不存在
	b := snap(m)
	err := m.Step()
	var pfe *PageFaultError
	if err == nil || errors.As(err, &pfe) || !strings.Contains(err.Error(), "頁不存在") || snap(m) != b {
		t.Errorf("推入目標頁不存在：應失敗即關閉、狀態不變：%v", err)
	}
	// 對照：頁都在時照常遞送。
	m, _ = setup([]byte{0xCD, 0x21})
	mustStep(t, m, "分頁下的遞送")
	if !m.CPU.Segmented() {
		t.Error("分頁下應照常遞送")
	}
}

// §7 第 3 項：ring 0 iretd 彈出 VM=1 的框架後還原 V86 狀態。IRQ0 的處理常式是 66 CF，同一個 Step 內遞送並返回。
func TestV86ReturnRestores(t *testing.T) {
	m := deliverProg(t, []byte{0x90, 0x90})
	m.CPU.R[cpu.SP], m.CPU.Hi[cpu.SP] = 0x1234, 0xABCD
	m.CPU.SetFlags(0x3000 | cpu.IF | cpu.CF | cpu.ZF)
	copy(m.Mem[segCode+uint32(gateOff(8)):], []byte{0x66, 0xCF})
	m.irq0Pending, m.picMask = true, 0
	b := snap(m)
	mustStep(t, m, "IRQ0 → iretd")
	c := m.CPU
	if c.Segmented() || !m.VM() || !m.User() || c.Seg != b.seg || c.FSeg != b.fs || c.GSeg != b.gs ||
		c.IP != b.ip || c.R[cpu.SP] != 0x1234 || c.Hi[cpu.SP] != 0xABCD || c.Flags != b.fl || m.EFlagsHi() != b.efHi {
		t.Errorf("回 V86 後狀態應還原：segmented=%v VM=%v CS:IP=%04X:%04X SS:ESP=%04X:%04X%04X 段 %v FS=%04X GS=%04X 旗標 %04X",
			c.Segmented(), m.VM(), c.Seg[cpu.CS], c.IP, c.Seg[cpu.SS], c.Hi[cpu.SP], c.R[cpu.SP], c.Seg, c.FSeg, c.GSeg, c.Flags)
	}
	for i := cpu.ES; i <= cpu.GS; i++ {
		if c.SegCache(i) != (SegCache{}) {
			t.Errorf("回 V86 後段快取 %d 應清除", i)
		}
	}
	mustStep(t, m, "回 V86 後繼續執行")
	if c.IP != b.ip+1 {
		t.Errorf("回 V86 後應照 V86 繼續：IP=%04X", c.IP)
	}

	// 失敗例：彈出 VM=0、EIP 高半、EFLAGS 高位元；16 位元 CF。
	for _, x := range []struct {
		name     string
		dw       int // 從 ESP 起的第幾個 dword
		v        uint32
		code     []byte
		contains string
	}{
		{"VM=0", 2, 0x3202, []byte{0x66, 0xCF}, "VM=0"},
		{"EIP 高半", 0, 0x10100, []byte{0x66, 0xCF}, "EIP"},
		{"EFLAGS 高位元", 2, 0x63202, []byte{0x66, 0xCF}, "bit 18"},
		{"16 位元 CF", -1, 0, []byte{0xCF}, "IRET"},
	} {
		m = deliverProg(t, []byte{0x90, 0x90})
		copy(m.Mem[segCode+uint32(gateOff(8)):], []byte{0x90})
		copy(m.Mem[segCode+uint32(gateOff(8))+1:], x.code)
		m.irq0Pending, m.picMask = true, 0
		mustStep(t, m, x.name+"：遞送")
		if x.dw >= 0 {
			put32(m, dSS0Base+uint32(m.CPU.R[cpu.SP])+uint32(4*x.dw), x.v)
		}
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.NT)
		wantStepErr(t, m, x.name, x.contains)
		if !m.CPU.Segmented() || m.VM() {
			t.Errorf("%s：失敗時應留在分段模式", x.name)
		}
	}
}

// §7 第 5 項的埠 60h：V86 下讀埠 60h 兩次得到同一值（JEMM 的 IRQ1 路徑在反射前 in al,60h，不得消耗）。
func TestV86Port60ReadTwice(t *testing.T) {
	m := deliverProg(t, []byte{0xE4, 0x60, 0x88, 0xC3, 0xE4, 0x60}) // in al,60h；mov bl,al；in al,60h
	m.SetKeyPort(0x9E)
	for i := 0; i < 3; i++ {
		mustStep(t, m, "in／mov")
	}
	if uint8(m.CPU.R[cpu.BX]) != 0x9E || uint8(m.CPU.R[cpu.AX]) != 0x9E {
		t.Errorf("兩次讀埠 60h 應同值 9Eh：BL=%02X AL=%02X", uint8(m.CPU.R[cpu.BX]), uint8(m.CPU.R[cpu.AX]))
	}
}

// §7 第 6 項：V86 下 startCallback、periodic 照常執行（以 V86 的 SS:SP 推入）；分段模式下延後不消耗。
func TestV86CallbacksRun(t *testing.T) {
	// 459 §2.3「剖面開，預期改變」：保護模式剖面的滑鼠回呼改經 IRQ12（V86 下經 IDT 74h，見 mouse459_test.go 的
	// TestMouse459V86Delivery）；這個夾具沒有服務層，Step 回 §3.4 條件 1a 的錯誤，不直接遠呼叫。
	m := deliverProg(t, []byte{0x90, 0x90, 0x90})
	m.QueueCallback(QueuedCall{Seg: 0x2000, Off: 0x20})
	wantStepErr(t, m, "V86 回呼（沒有服務層）", "服務層")
	if m.CallbacksMade() != 0 || m.CPU.R[cpu.SP] != dV86SP || m.CallbackPending() != 1 {
		t.Errorf("沒有服務層：不直接遠呼叫、V86 堆疊不動：made=%d SP=%04X", m.CallbacksMade(), m.CPU.R[cpu.SP])
	}
	m = deliverProg(t, []byte{0x90, 0x90, 0x90})
	cs := m.CPU.Seg[cpu.CS]
	m.SetPeriodicFarCall(0x2000, 0x10, 1)
	mustStep(t, m, "V86 periodic")
	mustStep(t, m, "V86 periodic")
	if m.PeriodicCalls() != 1 || m.CPU.R[cpu.SP] != dV86SP-4 ||
		get16(m, dV86SS*16+dV86SP-2) != cs || get16(m, dV86SS*16+dV86SP-4) != 0x101 {
		t.Errorf("V86 periodic 應照常執行並推在 V86 堆疊：calls=%d SP=%04X", m.PeriodicCalls(), m.CPU.R[cpu.SP])
	}
	// 分段模式（遞送進 ring 0 之後）：延後不消耗。
	m = deliverProg(t, []byte{0xCD, 0x21})
	mustStep(t, m, "進 ring 0")
	m.QueueCallback(QueuedCall{Seg: 0x2000, Off: 0x20})
	m.SetPeriodicFarCall(0x2000, 0x10, 1)
	sp, next := m.CPU.R[cpu.SP], m.periodic.next
	for i := 0; i < 3; i++ {
		mustStep(t, m, "ring 0 nop")
	}
	if m.CallbacksMade() != 0 || m.CallbackPending() != 1 || m.PeriodicCalls() != 0 || m.periodic.next != next || m.CPU.R[cpu.SP] != sp {
		t.Errorf("分段模式下回呼應延後不消耗：made=%d pending=%d periodic=%d SP %04X→%04X",
			m.CallbacksMade(), m.CallbackPending(), m.PeriodicCalls(), sp, m.CPU.R[cpu.SP])
	}
}

func get16(m *Machine, a uint32) uint16 { return uint16(m.Mem[a]) | uint16(m.Mem[a+1])<<8 }
