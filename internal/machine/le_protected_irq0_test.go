package machine

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func irq0Fixture(t *testing.T, code []byte) (*MOO2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	s, m, _ := mouseCallbackFixture(t, 0)
	copy(m.Mem[0x14000:], code)
	c := m.CPU
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.Seg[cpu386.SegDS] = 0x2508, 0x14000, 8
	if !s.Handle(c, 0x21) {
		t.Fatal("安裝自製 IRQ0")
	}
	c.Seg[cpu386.SegDS] = 0x10
	return s, m, s.DPMI.RealModeIO.(*LEOPLPorts)
}

func irq0Snapshot(c *cpu386.CPU) leMouseCallbackSnapshot {
	return leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
}

// 自製處理器寫入共享資料、EOI、修改 EAX，再 IRETD；不是原版 ISR 的翻譯。
var irq0TestHandler = []byte{0xff, 0x05, 0, 9, 0, 0, 0xb8, 0x20, 0, 0, 0, 0xe6, 0x20, 0xb8, 0x78, 0x56, 0x34, 0x12, 0xcf}

func TestProtectedIRQ0DOSVectorsRoundTripAndReject(t *testing.T) {
	s, m, p := irq0Fixture(t, irq0TestHandler)
	c := m.CPU
	for number := 0; number < 256; number++ {
		// 每個預設向量都必須非空且能保存、替換、恢復；08h 也先復原未掛接狀態。
		s.dosVectors[number] = 0
		c.R[cpu386.EAX], c.EFlags = 0xabcd3500|uint32(number), 0x247
		before := irq0Snapshot(c)
		if !s.Handle(c, 0x21) {
			t.Fatal(number)
		}
		vector := s.dosVectors[number]
		before.r[cpu386.EBX], before.seg[cpu386.SegES], before.flags = uint32(vector), uint16(vector>>32), 0x246
		if vector == 0 || irq0Snapshot(c) != before {
			t.Fatalf("35h 完整返回 %02X", number)
		}
		value, ok := c.ReadSegment8(c.Seg[cpu386.SegES], c.R[cpu386.EBX])
		if !ok || value != 0xcf {
			t.Fatal("非空入口必須可讀")
		}
		for _, target := range []uint64{8<<32 | 0x14000, vector} {
			c.R[cpu386.EAX], c.R[cpu386.EDX], c.Seg[cpu386.SegDS], c.EFlags = 0x2500|uint32(number), uint32(target), uint16(target>>32), 0x247
			before = irq0Snapshot(c)
			before.flags &^= cpu386.CF
			if !s.Handle(c, 0x21) || s.dosVectors[number] != target || irq0Snapshot(c) != before {
				t.Fatal("25h 往返不符")
			}
		}
	}
	c.SetDescriptor(0x20, cpu386.Descriptor{Base: 0x1000, Limit: 0xfff})
	for _, target := range []uint64{0, 0x20<<32 | 0x10, 8<<32 | uint64(leIRQ0Return), 8<<32 | uint64(leMouseCallbackReturn), 0x30<<32 | 0x14000, 8<<32 | 0xffffffff} {
		c.R[cpu386.EAX], c.R[cpu386.EDX], c.Seg[cpu386.SegDS] = 0x2508, uint32(target), uint16(target>>32)
		before, vector := irq0Snapshot(c), s.dosVectors[8]
		if s.Handle(c, 0x21) || irq0Snapshot(c) != before || s.dosVectors[8] != vector {
			t.Fatalf("無效向量 %X 未原子拒絕", target)
		}
	}
	p.BIOSClock.Pending = true
	if err := p.BIOSClock.advance(m, p, true); err != nil || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 1 || s.protectedIRQ0.started != 0 {
		t.Fatal("恢復預設08h 後仍應走 BIOS")
	}
}

func TestProtectedIRQ0DispatchFullStateAndBothCPUModes(t *testing.T) {
	for _, realMode := range []bool{false, true} {
		s, m, p := irq0Fixture(t, irq0TestHandler)
		c := m.CPU
		for i := range c.R {
			c.R[i] = 0x98760000 + uint32(i)
		}
		c.R[cpu386.ESP] = 0x80000
		c.EFlags = 0x247 | cpu386.DF | 0x100
		if realMode {
			c.EFlags &^= cpu386.IF
		}
		c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x123, 2, [8]float64{3, 4}
		before := irq0Snapshot(c)
		entered := false
		previous := c.StepHook
		c.StepHook = func(c *cpu386.CPU) (bool, error) {
			if s.protectedIRQ0.active && !entered {
				entered = true
				if c.EFlags&(cpu386.IF|0x100) != 0 || c.EFlags&cpu386.DF == 0 || c.Seg[cpu386.SegSS] == before.seg[cpu386.SegSS] || c.R[cpu386.ESP] != leIRQ0StackSize-12 {
					t.Fatal("入口旗標／私有堆疊不符")
				}
				frame := s.protectedIRQ0.frame
				if binary.LittleEndian.Uint32(frame[8:]) != before.flags&^(cpu386.IF|0x100) {
					t.Fatal("核心框架不可冒充外層 IF")
				}
				c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0, 0, 0, [8]float64{}
			}
			return previous(c)
		}
		p.BIOSClock.Pending = true
		if realMode {
			r := &cpu.CPU{Flags: cpu.IF | 2}
			r.R[cpu.AX], r.R[cpu.SP], r.Seg[cpu.CS], r.IP = 0x1234, 0xff00, 0x100, 0x10
			savedR, savedSeg, savedIP, savedFlags := r.R, r.Seg, r.IP, r.Flags
			if err := p.AdvanceRealMode(r, m); err != nil || r.R != savedR || r.Seg != savedSeg || r.IP != savedIP || r.Flags != savedFlags {
				t.Fatalf("實模式來源被改寫：%v", err)
			}
		} else {
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			before.eip++ // IRQ 返回後正常執行外層 NOP。
		}
		if !entered || irq0Snapshot(c) != before || binary.LittleEndian.Uint32(m.Mem[0x900:]) != 1 {
			t.Fatalf("原版處理器／完整恢復不符，real=%t", realMode)
		}
		active, failed, started, completed := s.IRQ0State()
		if active || failed || started != 1 || completed != 1 || p.BIOSClock.InService || p.BIOSClock.Deliveries != 1 || p.BIOSClock.Micros != 6 {
			t.Fatalf("轉送生命週期不符：%+v", p.BIOSClock)
		}
	}
}

func TestProtectedIRQ0IFMaskEOIAndNonReentrance(t *testing.T) {
	// STI 後仍不能重入；不 EOI 的外層返回也不能偷偷清服務狀態。
	s, m, p := irq0Fixture(t, []byte{0xfb, 0xff, 0x05, 0, 9, 0, 0, 0xcf})
	for _, enabled := range []bool{false, true} {
		p.picMasks[0] = 0xf9
		p.BIOSClock.Pending = true
		if err := p.BIOSClock.advance(m, p, enabled); err != nil || s.protectedIRQ0.started != 0 {
			t.Fatal("遮罩不可派送")
		}
	}
	p.picMasks[0] = 0xf8
	if err := p.BIOSClock.advance(m, p, false); err != nil || s.protectedIRQ0.started != 0 {
		t.Fatal("來源 IF 不可忽略")
	}
	p.Out8(0x43, 0x34)
	p.Out8(0x40, 2)
	p.Out8(0x40, 0)
	p.picInService = true // 較高優先 IRQ0 可執行，但其 EOI 不可清 IRQ7。
	if err := p.BIOSClock.advance(m, p, true); err != nil || !p.BIOSClock.InService || s.protectedIRQ0.completed != 1 {
		t.Fatal("返回不可偷清 EOI")
	}
	if err := p.BIOSClock.advance(m, p, true); err != nil || s.protectedIRQ0.started != 1 || !p.BIOSClock.Pending {
		t.Fatal("STI／待處理邊緣不得重入")
	}
	p.Out8(0x20, 0x0b)
	if value, ok := p.In8(0x20); !ok || value != 0x81 {
		t.Fatal("PIC ISR bit0／bit7")
	}
	p.Out8(0x20, 0x20)
	if p.BIOSClock.InService || !p.picInService {
		t.Fatal("EOI 優先清 IRQ0，不可誤清 IRQ7")
	}
	p.Out8(0x20, 0x20)
	if p.picInService {
		t.Fatal("後續 EOI 應沿 IRQ7 契約")
	}
}

func TestProtectedIRQ0ReturnPollutionAndBoundedFailure(t *testing.T) {
	for index := 0; index < 14; index++ {
		s, m, p := irq0Fixture(t, []byte{0xcf})
		c := m.CPU
		before := irq0Snapshot(c)
		previous := c.StepHook
		c.StepHook = func(c *cpu386.CPU) (bool, error) {
			if d := s.protectedIRQ0; d.active {
				if index < 12 {
					m.Mem[d.stackDescriptor.Base+leIRQ0StackSize-12+uint32(index)] ^= 1
				} else if index == 12 {
					c.R[cpu386.ESP]--
				} else {
					c.SetDescriptor(d.stackSelector, cpu386.Descriptor{})
				}
			}
			return previous(c)
		}
		p.BIOSClock.Pending = true
		if err := c.Step(); err == nil || !s.protectedIRQ0.failed || s.protectedIRQ0.completed != 0 || irq0Snapshot(c) != before {
			t.Fatalf("返回污染 %d 未拒絕／恢復", index)
		}
		if err := c.Step(); err == nil || irq0Snapshot(c) != before {
			t.Fatal("失敗後不可繼續正式執行")
		}
	}
	for _, code := range [][]byte{{0x0f, 0x0b}, {0xeb, 0xfe}} {
		s, m, p := irq0Fixture(t, code)
		before := irq0Snapshot(m.CPU)
		p.BIOSClock.Pending = true
		err := m.CPU.Step()
		if err == nil || !strings.Contains(err.Error(), "IRQ0") || !s.protectedIRQ0.failed || irq0Snapshot(m.CPU) != before {
			t.Fatal("未知 opcode／有界迴圈必須拒絕並恢復外層")
		}
	}
}

func TestProtectedIRQ0MouseCallbackCoexists(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	code := append([]byte(nil), irq0TestHandler...)
	code[2], code[3] = 0x50, 9
	copy(m.Mem[0x14000:], code)
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.Seg[cpu386.SegDS] = 0x2508, 0x14000, 8
	if !s.Handle(c, 0x21) {
		t.Fatal("安裝 IRQ0")
	}
	c.Seg[cpu386.SegDS] = 0x10
	if err := s.InjectMouseEvent(657, 189, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Step(); err != nil || !s.mouseCallback.active {
		t.Fatal("第一個滑鼠回呼應先開始")
	}
	if err := s.InjectMouseEvent(658, 190, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	c.EFlags |= cpu386.IF
	p := s.DPMI.RealModeIO.(*LEOPLPorts)
	p.BIOSClock.Pending = true
	if err := c.Step(); err != nil || s.protectedIRQ0.completed != 1 || !s.mouseCallback.active || len(s.mouseCallback.queue) != 1 || binary.LittleEndian.Uint32(m.Mem[0x950:]) != 1 {
		t.Fatalf("IRQ0 不得吞掉活動滑鼠框架或佇列：%v", err)
	}
	for i := 0; i < 7; i++ {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if s.mouseCallback.active || s.mouseCallback.completed != 1 || len(s.mouseCallback.queue) != 1 {
		t.Fatal("IRQ0 返回後原滑鼠遠返回必須仍可完成")
	}
}

func TestProtectedIRQ0RejectsUnmodeledChains(t *testing.T) {
	s, m, p := irq0Fixture(t, irq0TestHandler)
	s.dosVectors[0x1c] = 8<<32 | 0x14000
	p.BIOSClock.Pending = true
	if err := m.CPU.Step(); err == nil || s.protectedIRQ0.started != 0 || !p.BIOSClock.Pending {
		t.Fatal("未建模的保護模式 INT1C 不可默默跳過")
	}
	s.dosVectors[0x1c] = 0
	vector, ok := s.protectedIRQ0.defaultVector(8)
	if !ok {
		t.Fatal("預設入口")
	}
	// 自製近 JMP 轉到合成核心入口；完整實／保護模式鏈仍明確拒絕。
	c := m.CPU
	previous := c.StepHook
	c.StepHook = func(c *cpu386.CPU) (bool, error) {
		if s.protectedIRQ0.active {
			c.Seg[cpu386.SegCS], c.EIP = uint16(vector>>32), uint32(vector)
		}
		return previous(c)
	}
	if err := c.Step(); err == nil || !strings.Contains(err.Error(), "核心鏈") || !s.protectedIRQ0.failed {
		t.Fatal("合成入口不得冒充完整核心鏈")
	}
}

func TestProtectedIRQ0ClockDiagnosticStable(t *testing.T) {
	_, _, a := irq0Fixture(t, irq0TestHandler)
	_, _, b := irq0Fixture(t, irq0TestHandler)
	if first, second := fmt.Sprintf("%+v", a.BIOSClock), fmt.Sprintf("%+v", b.BIOSClock); first != second || strings.Contains(first, "0xc") || strings.Contains(first, "protectedIRQ0") {
		t.Fatal("診斷不可包含橋接器主機指標")
	}
}

func TestProtectedIRQ0BlocksLowerIRQ7UntilEOI(t *testing.T) {
	_, m, p := irq0Fixture(t, irq0TestHandler)
	r := cpu.New(&Machine{Mem: m.Mem})
	r.Flags, r.R[cpu.SP], r.IP = cpu.IF|2, 0x9000, 0x1234
	binary.LittleEndian.PutUint32(m.Mem[0x0f*4:], 0x01000100)
	p.picMasks[0], p.picPending, p.BIOSClock.InService = 0x78, true, true
	if err := p.AdvanceRealMode(r, m); err != nil || p.IRQ7Deliveries != 0 || r.IP != 0x1234 || r.R[cpu.SP] != 0x9000 {
		t.Fatal("IRQ0 尚服務時不得派送較低優先 IRQ7")
	}
	p.Out8(0x20, 0x20)
	if err := p.AdvanceRealMode(r, m); err != nil || p.IRQ7Deliveries != 1 || r.Seg[cpu.CS] != 0x100 || r.IP != 0x100 || r.R[cpu.SP] != 0x8ffa || !p.picInService {
		t.Fatalf("IRQ0 EOI 後必須保留既有實模式 IRQ7：%v", err)
	}
}

func TestProtectedIRQ0RejectsForeignCPUAndCorruptDefaults(t *testing.T) {
	s, m, p := irq0Fixture(t, irq0TestHandler)
	other := cpu386.New(m)
	other.SetDescriptor(8, cpu386.Descriptor{Limit: 0xffffffff})
	other.Seg[cpu386.SegDS], other.R[cpu386.EDX] = 8, 0x14000
	for _, ax := range []uint32{0x3508, 0x2508} {
		other.R[cpu386.EAX] = ax
		before, vector := irq0Snapshot(other), s.dosVectors[8]
		if s.Handle(other, 0x21) || irq0Snapshot(other) != before || s.dosVectors[8] != vector {
			t.Fatal("不可跨到其他 CPU 的描述子與向量")
		}
	}
	for _, number := range []uint8{8, 0x1c} {
		vector, ok := s.protectedIRQ0.defaultVector(number)
		if !ok {
			t.Fatal("預設入口")
		}
		s.dosVectors[number] = vector
		m.Mem[uint32(vector)] = 0x90
		p.BIOSClock.Pending = true
		if err := p.BIOSClock.advance(m, p, true); err == nil || !p.BIOSClock.Pending || p.BIOSClock.Deliveries != 0 {
			t.Fatal("被污染的預設入口不可偷走 BIOS 計數")
		}
		m.Mem[uint32(vector)] = 0xcf
	}
}
