package machine

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func irq0EndChainFixture(t *testing.T, earlyEOI bool) (*MOO2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	s, m, p := irq0Fixture(t, []byte{0x90})
	vector, ok := s.protectedIRQ0.defaultVector(8)
	if !ok {
		t.Fatal("預設 DOS08h 入口配置失敗")
	}
	// 自製處理器：增加共享資料，再以 PUSH／CB 尾鏈；不翻譯原版 ISR。
	code := []byte{0xff, 0x05, 0, 9, 0, 0}
	if earlyEOI {
		code = append(code, 0xb8, 0x20, 0, 0, 0, 0xe6, 0x20)
	}
	code = append(code, 0x68)
	code = binary.LittleEndian.AppendUint32(code, uint32(vector>>32))
	code = append(code, 0x68)
	code = binary.LittleEndian.AppendUint32(code, uint32(vector))
	code = append(code, 0xcb)
	copy(m.Mem[0x14000:], code)
	return s, m, p
}

func TestProtectedIRQ0EndChainRestoresBothCPUModesAndTicksOnce(t *testing.T) {
	for _, realMode := range []bool{false, true} {
		for _, earlyEOI := range []bool{false, true} {
			t.Run(fmt.Sprintf("實模式%t先EOI%t", realMode, earlyEOI), func(t *testing.T) {
				s, m, p := irq0EndChainFixture(t, earlyEOI)
				c := m.CPU
				for i := range c.R {
					c.R[i] = 0x98760000 + uint32(i)
				}
				c.R[cpu386.ESP] = 0x80000
				c.EFlags = 0x247 | cpu386.DF | 0x100
				if realMode {
					c.EFlags &^= cpu386.IF
				}
				c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x1234, 2, [8]float64{3, 4}
				before := irq0Snapshot(c)
				binary.LittleEndian.PutUint32(m.Mem[0x46c:], 0x22741) // 原版有限 BDA 樣本。
				m.Mem[0x470] = 7
				previous, entered := c.StepHook, false
				c.StepHook = func(c *cpu386.CPU) (bool, error) {
					if s.protectedIRQ0.active && !entered {
						entered = true
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
					before.eip++
				}
				active, failed, started, completed := s.IRQ0State()
				if !entered || irq0Snapshot(c) != before || active || failed || started != 1 || completed != 1 || p.BIOSClock.Deliveries != 1 || p.BIOSClock.InService || p.BIOSClock.Pending {
					t.Fatalf("結束鏈完整恢復／生命週期錯誤：%+v", p.BIOSClock)
				}
				if binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 0x22742 || m.Mem[0x470] != 7 || binary.LittleEndian.Uint32(m.Mem[0x900:]) != 1 {
					t.Fatal("共享資料須保留，BDA 只增加一次")
				}
			})
		}
	}
}

func TestProtectedIRQ0EndChainRolloverAndNonSpecificEOI(t *testing.T) {
	for _, tc := range []struct {
		initial, want uint32
		midnight      byte
	}{
		{0, 1, 0xff}, {0x1800af, 0, 0}, {0x1800b0, 0, 0}, {0xffffffff, 0, 0xff},
	} {
		s, m, p := irq0EndChainFixture(t, false)
		binary.LittleEndian.PutUint32(m.Mem[0x46c:], tc.initial)
		m.Mem[0x470] = 0xff
		p.picInService, p.BIOSClock.Pending = true, true
		if err := m.CPU.Step(); err != nil || s.protectedIRQ0.completed != 1 || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != tc.want || m.Mem[0x470] != tc.midnight || p.BIOSClock.InService || !p.picInService {
			t.Fatalf("午夜／非指定 EOI %X：%v", tc.initial, err)
		}
	}
	// 若處理器先 EOI，BIOS 的第二筆非指定 EOI 應消費當下最高服務位元。
	_, m, p := irq0EndChainFixture(t, true)
	p.picInService, p.BIOSClock.Pending = true, true
	if err := m.CPU.Step(); err != nil || p.picInService || p.BIOSClock.InService {
		t.Fatalf("BIOS EOI 必須沿既有 PIC 優先序：%v", err)
	}
}

func TestProtectedIRQ0EndChainPollutionRejectsBeforeBIOSMutation(t *testing.T) {
	changes := map[string]func(*leProtectedIRQ0, *cpu386.CPU){
		"ESP錯誤":     func(_ *leProtectedIRQ0, c *cpu386.CPU) { c.R[cpu386.ESP]-- },
		"SS錯誤":      func(_ *leProtectedIRQ0, c *cpu386.CPU) { c.Seg[cpu386.SegSS] = 0x10 },
		"堆疊描述子污染":   func(d *leProtectedIRQ0, c *cpu386.CPU) { c.SetDescriptor(d.stackSelector, cpu386.Descriptor{}) },
		"入口描述子污染":   func(d *leProtectedIRQ0, c *cpu386.CPU) { c.SetDescriptor(d.defaultSelector, cpu386.Descriptor{}) },
		"入口位元組污染":   func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.m.Mem[d.defaultBase+8] = 0x90 },
		"錯誤預設slot":  func(d *leProtectedIRQ0, c *cpu386.CPU) { c.EIP = d.defaultBase + 9 },
		"客製保護INT1C": func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.s.dosVectors[0x1c] = 8<<32 | 0x14000 },
		"客製IVT08":   func(d *leProtectedIRQ0, _ *cpu386.CPU) { binary.LittleEndian.PutUint32(d.m.Mem[0x20:], 0x1000100) },
		"客製IVT1C":   func(d *leProtectedIRQ0, _ *cpu386.CPU) { binary.LittleEndian.PutUint32(d.m.Mem[0x70:], 0x1000100) },
		"客製DPMI08":  func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.s.DPMI.SetRealModeVector(8, 0x100, 0) },
		"客製DPMI1C":  func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.s.DPMI.SetRealModeVector(0x1c, 0, 0x100) },
		"預設INT1C污染": func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.m.Mem[d.defaultBase+0x1c] = 0x90 },
		"BDA不足":     func(d *leProtectedIRQ0, _ *cpu386.CPU) { d.m.Mem = d.m.Mem[:0x470] },
	}
	for i := uint32(0); i < 12; i++ {
		index := i
		changes[fmt.Sprintf("框架byte%d", i)] = func(d *leProtectedIRQ0, _ *cpu386.CPU) {
			d.m.Mem[d.stackDescriptor.Base+leIRQ0StackSize-12+index] ^= 1
		}
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s, m, p := irq0EndChainFixture(t, false)
			c, before := m.CPU, irq0Snapshot(m.CPU)
			d := s.protectedIRQ0
			vector, ok := d.defaultVector(0x1c)
			if !ok {
				t.Fatal("預設 INT1C")
			}
			s.dosVectors[0x1c] = vector
			binary.LittleEndian.PutUint32(m.Mem[0x46c:], 123)
			previous, changed := c.StepHook, false
			c.StepHook = func(c *cpu386.CPU) (bool, error) {
				if d.active && c.Seg[cpu386.SegCS] == d.defaultSelector && c.EIP == d.defaultBase+8 && !changed {
					changed = true
					change(d, c)
				}
				return previous(c)
			}
			p.BIOSClock.Pending = true
			if err := c.Step(); err == nil || !changed || !d.failed || d.completed != 0 || irq0Snapshot(c) != before || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 123 || !p.BIOSClock.InService || p.BIOSClock.Deliveries != 1 {
				t.Fatalf("污染未拒絕／恢復，或偷增加 BDA／EOI：%v", err)
			}
			if err := c.Step(); err == nil || irq0Snapshot(c) != before {
				t.Fatal("失敗後不可繼續執行")
			}
		})
	}
}

func TestProtectedIRQ0EndChainKeepsPendingEdge(t *testing.T) {
	s, m, p := irq0EndChainFixture(t, false)
	p.Out8(0x43, 0x34)
	p.Out8(0x40, 2)
	p.Out8(0x40, 0)
	p.BIOSClock.Pending = true
	if err := m.CPU.Step(); err != nil || s.protectedIRQ0.completed != 1 || s.protectedIRQ0.started != 1 || p.BIOSClock.Deliveries != 1 || !p.BIOSClock.Pending || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 1 {
		t.Fatalf("結束鏈不得吃掉待處理邊緣或重入：%v", err)
	}
}
