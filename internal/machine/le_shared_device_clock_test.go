package machine

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// 自製四byte循環與兩個真實CPU；不使用原版素材。
func sharedDMAFixture(t *testing.T, mode byte, rate uint16) (*LEOPLPorts, *LEMachine, *cpu.CPU, *int) {
	t.Helper()
	p, m, _ := c6DMAFixture(t, mode, 2, rate)
	old := m.Mem
	m.Mem = make([]byte, 0x2000)
	copy(m.Mem, old)
	c := cpu386.New(m)
	m.CPU = c
	c.SetDescriptor(8, cpu386.Descriptor{Limit: 0x1fff})
	c.SetDescriptor(0x10, cpu386.Descriptor{Limit: 0x1fff, Writable: true})
	c.Seg = [6]uint16{8, 0x10, 0x10, 0, 0, 0x10}
	c.EIP, c.R[cpu386.ESP], c.EFlags = 0x1000, 0x1800, 2
	m.Mem[0x1000], m.Mem[0x110] = 0x90, 0x90
	calls := new(int)
	c.StepHook = func(*cpu386.CPU) (bool, error) { (*calls)++; return false, nil }
	if !InstallLEBIOSClock(m, p) {
		t.Fatal("共用時鐘安裝")
	}
	r := cpu.New(&dpmiRealBus{m: m, io: p})
	r.Model = cpu.Model80386
	r.Seg[cpu.CS] = 0
	r.IP, r.R[cpu.SP], r.Flags = 0x110, 0x1800, 2
	return p, m, r, calls
}

func sharedStep(t *testing.T, p *LEOPLPorts, m *LEMachine, r *cpu.CPU, real bool) {
	t.Helper()
	if real {
		r.IP = 0x110
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
		if err := r.Step(); err != nil || r.Bus.(*dpmiRealBus).err != nil || r.IP != 0x111 {
			t.Fatalf("實模式NOP：%v %X", err, r.IP)
		}
	} else {
		m.CPU.EIP = 0x1000
		if err := m.CPU.Step(); err != nil || m.CPU.EIP != 0x1001 {
			t.Fatalf("保護模式NOP：%v %X", err, m.CPU.EIP)
		}
	}
}

func TestSharedDMAClockIndependentSamplesAndBothCPUs(t *testing.T) {
	for _, mode := range []byte{0, 0x10, 0x20, 0x30} {
		for _, rate := range []uint16{5000, 22050, 45000} {
			for schedule := 0; schedule < 3; schedule++ {
				p, m, r, calls := sharedDMAFixture(t, mode, rate)
				protected := 0
				channels := uint64(1)
				if mode >= 0x20 {
					channels = 2
				}
				for tick := uint64(1); tick <= 3000; tick++ {
					real := schedule == 1 || schedule == 2 && tick%3 == 0
					if !real {
						protected++
					}
					sharedStep(t, p, m, r, real)
					samples := tick * uint64(rate) * channels / 1000000
					if p.virtualMicros != tick || p.BIOSClock.Micros != tick || *calls != protected ||
						uint64(len(p.PCM)) != samples || p.DMACompletions != samples/2 ||
						p.dma.Current[2] != 0x200+uint16(samples%4) || p.dma.Current[3] != 3-uint16(samples%4) ||
						p.dmaLeft != 2-uint32(samples%2) || p.IRQ7Deliveries != 0 {
						t.Fatalf("mode=%X rate=%d 排程%d tick%d samples%d state=%+v", mode, rate, schedule, tick, samples, p.State())
					}
				}
				for i, b := range p.PCM {
					if b != []byte{0, 0x7f, 0x80, 0xff}[i%4] {
						t.Fatal("來源／重載／有號原始byte")
					}
				}
			}
		}
	}
}

func TestSharedDMAClockTimeConstantMaskResetAndBounds(t *testing.T) {
	for _, mode := range []byte{0, 0x20} {
		p, m, r, _ := sharedDMAFixture(t, mode, 22050)
		for _, w := range [][2]uint16{{0x226, 1}, {0x226, 0}, {0x22c, 0x40}, {0x22c, 211}, {0x22c, 0xc6}, {0x22c, uint16(mode)}, {0x22c, 1}, {0x22c, 0}} {
			if !p.Out8(w[0], byte(w[1])) {
				t.Fatal("TimeConstant")
			}
		}
		for tick := 1; tick <= 4500; tick++ {
			sharedStep(t, p, m, r, tick%2 == 0)
			if len(p.PCM) != tick/45 || p.DMACompletions != uint64(tick/45/2) {
				t.Fatal("TimeConstant重複channels")
			}
		}
		p.Out8(0xa, 5)
		before := p.State()
		pcm := len(p.PCM)
		for i := 0; i < 100; i++ {
			sharedStep(t, p, m, r, i%2 == 0)
		}
		after := p.State()
		if len(p.PCM) != pcm || after.DMACurrent != before.DMACurrent || after.DMA8SampleCredit != before.DMA8SampleCredit {
			t.Fatal("遮罩仍消費來源")
		}
		p.Out8(0xa, 1)
		p.Out8(0x226, 1)
		for i := 0; i < 100; i++ {
			sharedStep(t, p, m, r, i%2 == 0)
		}
		if len(p.PCM) != pcm || p.dmaActive || p.picPending || p.dsp.IRQPending {
			t.Fatal("reset未取消")
		}
	}
	for _, real := range []bool{false, true} {
		p, m, r, _ := sharedDMAFixture(t, 0x20, 22050)
		p.dma.Page[1] = 1
		var err error
		for i := 0; i < 23; i++ {
			if real {
				err = p.AdvanceRealMode(r, m)
			} else {
				m.CPU.EIP = 0x1000
				err = m.CPU.Step()
			}
			if err != nil {
				break
			}
		}
		if err == nil || !strings.Contains(err.Error(), "DMA讀取超界") || len(p.PCM) != 0 || p.DMACompletions != 0 {
			t.Fatal("來源超界未拒絕", real, err)
		}
	}
}

func TestSharedClockProtectedIRQ7StopsWithoutChangingCPU(t *testing.T) {
	for gate := 0; gate < 4; gate++ {
		s, m, p := irq0Fixture(t, irq0TestHandler)
		c := m.CPU
		for i := range c.R {
			c.R[i] = 0x12340000 + uint32(i)
		}
		c.R[cpu386.ESP] = 0x80000
		c.EFlags = cpu386.IF | cpu386.DF | 0x247
		c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x123, 2
		c.FPUStack = [8]float64{2, 3}
		p.picPending, p.dsp.IRQPending, p.picMasks[0] = true, true, 0x78
		binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0x01000100)
		s.DPMI.SetRealModeVector(0x0f, 0x1234, 0x5678)
		s.dosVectors[0x0f] = 8<<32 | 0x14000
		switch gate {
		case 0:
			c.EFlags &^= cpu386.IF
		case 1:
			p.picMasks[0] |= 0x80
		case 2:
			p.picInService = true
		case 3:
			s.protectedIRQ0.active = true
		}
		if err := c.Step(); err != nil || !p.picPending || !p.dsp.IRQPending || p.IRQ7Deliveries != 0 {
			t.Fatalf("閘門%d遺失pending：%v", gate, err)
		}
		c.EFlags |= cpu386.IF
		p.picMasks[0] &^= 0x80
		p.picInService = false
		s.protectedIRQ0.active = false
		before := irq0Snapshot(c)
		stack := append([]byte(nil), m.Mem[0x7ff00:0x80000]...)
		err := c.Step()
		if err == nil || !strings.Contains(err.Error(), "IRQ7 保護模式派送尚未建模") ||
			!strings.Contains(err.Error(), "absolute_ivt0f=01000100 dpmi_rm0f=1234:5678 dos_pm0f=0008:00014000") ||
			irq0Snapshot(c) != before || !bytes.Equal(stack, m.Mem[0x7ff00:0x80000]) || !p.picPending || !p.dsp.IRQPending ||
			p.IRQ7Deliveries != 0 || p.picInService {
			t.Fatalf("IRQ7停止未保留現場：%v", err)
		}
	}
}

func TestSharedClockCountsIRQ0NestedInstructionsOnce(t *testing.T) {
	s, m, p := irq0Fixture(t, irq0TestHandler)
	for _, w := range [][2]uint16{{0x21, 0xf8}, {0xa, 5}, {0xc, 0}, {2, 0}, {2, 2}, {3, 3}, {3, 0}, {0x83, 0}, {0xb, 0x59}, {0xa, 1}, {0x22c, 0x41}, {0x22c, 0x56}, {0x22c, 0x22}, {0x22c, 0xc6}, {0x22c, 0x20}, {0x22c, 1}, {0x22c, 0}, {0x43, 0x34}, {0x40, 0x4e}, {0x40, 0x17}} {
		if !p.Out8(w[0], byte(w[1])) {
			t.Fatal("巢狀clock設定", w)
		}
	}
	for i := 0; i < 10000; i++ {
		m.CPU.EIP = 0x600
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	ticks := p.BIOSClock.Micros
	want := ticks * 44100 / 1000000
	if s.protectedIRQ0.started == 0 || s.protectedIRQ0.started != s.protectedIRQ0.completed || ticks <= 10000 ||
		p.virtualMicros != ticks || uint64(len(p.PCM)) != want || p.DMACompletions != want/2 || p.IRQ7Deliveries != 0 {
		t.Fatalf("巢狀CPU步漏計／重複：ticks=%d state=%+v", ticks, p.State())
	}
}

func TestSharedClockDMA16SingleWordAcrossModes(t *testing.T) {
	p, m, r, _ := sharedDMAFixture(t, 0, 22050)
	m.Mem[0x200], m.Mem[0x201] = 0x34, 0x12
	for _, w := range [][2]uint16{{0x226, 1}, {0x226, 0}, {0x22c, 0x41}, {0x22c, 0x56}, {0x22c, 0x22},
		{0xd4, 5}, {0xd8, 0}, {0xc4, 0}, {0xc4, 1}, {0xc6, 0}, {0xc6, 0}, {0x8b, 0}, {0xd6, 0x49}, {0xd4, 1},
		{0x22c, 0xb0}, {0x22c, 0x30}, {0x22c, 0}, {0x22c, 0}} {
		if !p.Out8(w[0], byte(w[1])) {
			t.Fatal("共用16位元DMA設定", w)
		}
	}
	for tick := 1; tick <= 23; tick++ {
		sharedStep(t, p, m, r, tick%2 == 0)
		if tick < 23 && (len(p.PCM16) != 0 || p.DMA16Completions != 0) {
			t.Fatal("16位元取樣過早")
		}
	}
	if !bytes.Equal(p.PCM16, []byte{0x34, 0x12}) || p.DMA16Completions != 1 || p.IRQ7Deliveries != 0 ||
		!p.dsp.IRQ16Pending || !p.picPending || p.secondaryDMA.Current[2] != 0x101 ||
		p.virtualMicros != 23 || p.BIOSClock.Micros != 23 {
		t.Fatalf("16位元跨模式進度：%+v", p.State())
	}
}
