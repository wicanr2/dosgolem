package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

var irq1OwnHandler = []byte{0xe4, 0x60, 0xa2, 0, 9, 0, 0, 0xb0, 0x20, 0xe6, 0x20, 0xcf}

func irq1Fixture(t *testing.T, code []byte) (*MOO2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	s, m, _ := mouseCallbackFixture(t, 0)
	c := m.CPU
	copy(m.Mem[0x14000:], code)
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.Seg[cpu386.SegDS] = 0x2509, 0x14000, 8
	if !s.Handle(c, 0x21) {
		t.Fatal("自製IRQ1安裝失敗")
	}
	c.Seg[cpu386.SegDS] = 0x10
	c.EFlags = 0x747
	c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x123, 2, [8]float64{3, 4}
	return s, m, s.DPMI.RealModeIO.(*LEOPLPorts)
}
func TestHardwareIRQ1BothModesAndPrivateStack(t *testing.T) {
	for _, realMode := range []bool{false, true} {
		s, m, p := irq1Fixture(t, irq1OwnHandler)
		c := m.CPU
		before := irq0Snapshot(c)
		clientBlocks := len(s.DPMI.Blocks())
		if err := s.QueueHardwareScan(1); err != nil {
			t.Fatal(err)
		}
		if err := s.QueueHardwareScan(0x81); err != nil {
			t.Fatal(err)
		}
		for index, scan := range []byte{1, 0x81} {
			if realMode {
				r := &cpu.CPU{Flags: cpu.IF | 2}
				r.IP = 0x2345
				r.R[cpu.AX] = 0xabcd
				beforeReal := *r
				if err := p.AdvanceRealMode(r, m); err != nil || r.R != beforeReal.R || r.Seg != beforeReal.Seg || r.IP != beforeReal.IP || r.Flags != beforeReal.Flags {
					t.Fatal(err)
				}
			} else {
				if err := p.AdvanceProtectedMode(c, m); err != nil {
					t.Fatal(err)
				}
			}
			d := p.keyboardIRQ1
			if irq0Snapshot(c) != before || m.Mem[0x900] != scan || d.completed != uint64(index+1) || d.full || d.inService || d.failed || len(s.DPMI.Blocks()) != clientBlocks {
				t.Fatal("兩模式／caller／FPU／真實讀取與EOI不符")
			}
			if d.last.Steps != 5 || !d.last.Returned || len(d.last.IO) != 2 {
				t.Fatalf("處理器觀測不符：%+v", d.last)
			}
		}
		selector, brk := p.keyboardIRQ1.stackSelector, s.DPMI.brk
		if err := s.QueueHardwareScan(1); err != nil {
			t.Fatal(err)
		}
		if err := p.AdvanceProtectedMode(c, m); err != nil || selector != p.keyboardIRQ1.stackSelector || brk != s.DPMI.brk {
			t.Fatal("私有堆疊未重用")
		}
	}
}
func TestHardwareIRQ1GatesAndPIC(t *testing.T) {
	for name, change := range map[string]func(*LEMachine, *LEOPLPorts){
		"CLI":     func(m *LEMachine, p *LEOPLPorts) { m.CPU.EFlags &^= cpu386.IF },
		"遮罩":      func(m *LEMachine, p *LEOPLPorts) { p.picMasks[0] |= 2 },
		"IRQ0服務中": func(m *LEMachine, p *LEOPLPorts) { p.BIOSClock.InService = true },
		"IRQ0執行中": func(m *LEMachine, p *LEOPLPorts) { p.BIOSClock.protectedIRQ0.active = true },
		"IRQ7執行中": func(m *LEMachine, p *LEOPLPorts) { p.realIRQ7 = &leRealIRQ7{active: true} },
	} {
		t.Run(name, func(t *testing.T) {
			s, m, p := irq1Fixture(t, irq1OwnHandler)
			_ = s.QueueHardwareScan(1)
			change(m, p)
			if err := p.AdvanceProtectedMode(m.CPU, m); err != nil || !p.keyboardIRQ1.full || !p.keyboardIRQ1.pending || p.keyboardIRQ1.started != 0 {
				t.Fatal("閘門丟失輸入", err)
			}
		})
	}
	s, m, p := irq1Fixture(t, irq1OwnHandler)
	d := p.keyboardIRQ1
	for i := 0; i < 128; i++ {
		if err := s.QueueHardwareScan(1); err != nil {
			t.Fatal(i, err)
		}
	}
	for _, scan := range []byte{1, 0x81, 0, 2, 0xe0} {
		if err := s.QueueHardwareScan(scan); err == nil {
			t.Fatal("容量／非法碼未拒絕")
		}
	}
	if value, ok := p.In8(0x64); !ok || value != 5 || !d.full {
		t.Fatal("64h不是只讀狀態")
	}
	p.Out8(0x20, 0x0a)
	if v, _ := p.In8(0x20); v&2 == 0 {
		t.Fatal("PIC IRR無IRQ1")
	}
	d.inService = true
	p.picInService = true
	p.BIOSClock.InService = true
	p.Out8(0x20, 0x0b)
	if v, _ := p.In8(0x20); v != 0x83 {
		t.Fatalf("PIC ISR不符：%02X", v)
	}
	for want := byte(0x82); ; want = 0x80 {
		p.Out8(0x20, 0x20)
		v, _ := p.In8(0x20)
		if v != want {
			t.Fatalf("EOI優先序%02X!=%02X", v, want)
		}
		if want == 0x80 {
			break
		}
	}
	if _, ok := p.In8(0x60); !ok || d.full || !d.pending {
		t.Fatal("60h與PIC pending混淆")
	}
	p.Out8(0x61, 0x85)
	if v, _ := p.In8(0x61); v != 0x85 {
		t.Fatal("61h暫存未保持")
	}
	if p.Out8(0x64, 0xad) {
		t.Fatal("未支援controller命令被接受")
	}
	_ = m
}
func TestHardwareIRQ1DefaultBIOSRealFarCall(t *testing.T) {
	code := []byte{0x9c, 0xff, 0x1d, 0x20, 9, 0, 0, 0xcf}
	s, m, p := irq1Fixture(t, code)
	c := m.CPU
	vector, ok := s.protectedIRQ0.defaultVector(9)
	if !ok {
		t.Fatal("default")
	}
	binary.LittleEndian.PutUint32(m.Mem[0x920:], uint32(vector))
	binary.LittleEndian.PutUint16(m.Mem[0x924:], uint16(vector>>32))
	for _, scan := range []byte{1, 0x81} {
		before := irq0Snapshot(c)
		_ = s.QueueHardwareScan(scan)
		if err := p.AdvanceProtectedMode(c, m); err != nil || irq0Snapshot(c) != before {
			t.Fatal(err)
		}
		if m.Keyboard == nil || len(m.Keyboard.Enqueued) != 1 || m.Keyboard.Enqueued[0] != 0x011b || binary.LittleEndian.Uint16(m.Mem[0x41e:]) != 0x011b || p.keyboardIRQ1.failed {
			t.Fatal("default BIOS公開字元／放開契約不符")
		}
	}
	// 下一次真正遠CALL發現已污染的CF入口必須停止。
	m.Mem[uint32(vector)] = 0x90
	_ = s.QueueHardwareScan(1)
	if err := p.AdvanceProtectedMode(c, m); err == nil || !p.keyboardIRQ1.failed {
		t.Fatal("default入口污染未停止")
	}
}
func TestHardwareIRQ1FailureStopsBothModes(t *testing.T) {
	for name, code := range map[string][]byte{
		"未知指令": {0x0f, 0xff},
		"未知埠":  {0xe4, 0x65},
		"缺讀取":  {0xb0, 0x20, 0xe6, 0x20, 0xcf},
		"缺EOI": {0xe4, 0x60, 0xcf},
		"超時":   {0xeb, 0xfe},
	} {
		t.Run(name, func(t *testing.T) {
			s, m, p := irq1Fixture(t, code)
			before := irq0Snapshot(m.CPU)
			_ = s.QueueHardwareScan(1)
			err := p.AdvanceProtectedMode(m.CPU, m)
			if err == nil || !p.keyboardIRQ1.failed || irq0Snapshot(m.CPU) != before || p.keyboardIRQ1.active {
				t.Fatal("失敗後未保持核心並停止")
			}
			micros := p.virtualMicros
			if err := p.AdvanceProtectedMode(m.CPU, m); err == nil || p.virtualMicros != micros {
				t.Fatal("失敗後仍可計時")
			}
			if err := p.AdvanceRealMode(&cpu.CPU{Flags: cpu.IF}, m); err == nil || p.virtualMicros != micros {
				t.Fatal("另一模式未停止")
			}
			if err := s.QueueHardwareScan(1); err == nil {
				t.Fatal("失敗後仍接受輸入")
			}
		})
	}
	for name, change := range map[string]func(*MOO2StartupDOS, *LEMachine, *LEOPLPorts){
		"IVT混用":  func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { m.Mem[36] = 1 },
		"DPMI混用": func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.DPMI.protVec[9] = 8<<32 | 0x14000 },
		"目標無效":   func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.dosVectors[9] = 0x48<<32 | 0x14000 },
	} {
		t.Run(name, func(t *testing.T) {
			s, m, p := irq1Fixture(t, irq1OwnHandler)
			change(s, m, p)
			_ = s.QueueHardwareScan(1)
			if err := p.AdvanceProtectedMode(m.CPU, m); err == nil || !p.keyboardIRQ1.failed {
				t.Fatal(fmt.Sprint(name, err))
			}
		})
	}
}

func TestHardwareIRQ1HigherPriorityIRQ0CanNest(t *testing.T) {
	s, m, p := irq1Fixture(t, append([]byte{0xfb}, irq1OwnHandler...))
	copy(m.Mem[0x15000:], irq0TestHandler)
	s.dosVectors[8] = 8<<32 | 0x15000
	c := m.CPU
	before := irq0Snapshot(c)
	previous := c.StepHook
	triggered := false
	c.StepHook = func(c *cpu386.CPU) (bool, error) {
		if p.keyboardIRQ1.active && !s.protectedIRQ0.active && c.EFlags&cpu386.IF != 0 && !triggered {
			triggered = true
			p.BIOSClock.Pending = true
		}
		return previous(c)
	}
	_ = s.QueueHardwareScan(1)
	if err := p.AdvanceProtectedMode(c, m); err != nil || !triggered || s.protectedIRQ0.completed != 1 || p.keyboardIRQ1.completed != 1 || irq0Snapshot(c) != before {
		t.Fatalf("較高優先IRQ0巢狀錯誤：%v", err)
	}
}
func TestHardwareIRQ1RejectsPrivateFrameChanges(t *testing.T) {
	for _, descriptor := range []bool{false, true} {
		s, m, p := irq1Fixture(t, irq1OwnHandler)
		c := m.CPU
		before := irq0Snapshot(c)
		previous := c.StepHook
		changed := false
		c.StepHook = func(c *cpu386.CPU) (bool, error) {
			if p.keyboardIRQ1.active && !changed {
				changed = true
				d := p.keyboardIRQ1
				if descriptor {
					c.SetDescriptor(d.stackSelector, cpu386.Descriptor{Base: d.stackDescriptor.Base, Limit: 4094, Writable: true})
				} else {
					m.Mem[d.stackDescriptor.Base+4084] ^= 1
				}
			}
			return previous(c)
		}
		_ = s.QueueHardwareScan(1)
		if err := p.AdvanceProtectedMode(c, m); err == nil || !p.keyboardIRQ1.failed || irq0Snapshot(c) != before {
			t.Fatal("私有框架／descriptor污染未停止")
		}
	}
}
