package machine

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

func clockFixture(t *testing.T) (*LEMachine, *LEOPLPorts) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x500)}
	m.CPU = cpu386.New(m)
	p := NewLEOPLPorts()
	if !InstallLEBIOSClock(m, p) {
		t.Fatal("安裝")
	}
	return m, p
}
func TestBIOSClockPeriodMaskAndRollover(t *testing.T) {
	m, p := clockFixture(t)
	advance := func(n int, on bool) {
		t.Helper()
		for i := 0; i < n; i++ {
			if err := p.BIOSClock.advance(m, p, on); err != nil {
				t.Fatal(err)
			}
		}
	}
	advance(54925, true)
	if p.BIOSClock.Deliveries != 0 {
		t.Fatal("過早tick")
	}
	advance(1, true)
	if p.BIOSClock.Deliveries != 1 || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 1 {
		t.Fatal("分數週期")
	}
	p.Out8(0x21, 0xf9)
	advance(200000, true)
	if !p.BIOSClock.Pending || p.BIOSClock.Deliveries != 1 {
		t.Fatal("mask")
	}
	p.Out8(0x20, 0x0a)
	if v, ok := p.In8(0x20); !ok || v&1 == 0 {
		t.Fatal("IRQ0 IRR")
	}
	p.Out8(0x21, 0xf8)
	advance(1, false)
	if p.BIOSClock.Deliveries != 1 {
		t.Fatal("IF")
	}
	advance(1, true)
	if p.BIOSClock.Deliveries != 2 {
		t.Fatal("pending未合併")
	}
	binary.LittleEndian.PutUint32(m.Mem[0x46c:], 0x1800af)
	p.BIOSClock.Pending = true
	advance(1, true)
	if binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 0 || m.Mem[0x470] != 1 {
		t.Fatal("午夜")
	}
	for _, n := range []int{8, 0x1c} {
		binary.LittleEndian.PutUint32(m.Mem[n*4:], 0x1234)
		p.BIOSClock.Pending = true
		if err := p.BIOSClock.advance(m, p, true); err == nil {
			t.Fatal("客製handler未拒絕")
		}
		binary.LittleEndian.PutUint32(m.Mem[n*4:], 0)
	}
}
func TestBIOSClockReloadAndCPUHook(t *testing.T) {
	m, p := clockFixture(t)
	calls := 0
	// 安裝前已有的執行期hook必须被呼叫，並只計一次保護模式步進。
	m2 := &LEMachine{Mem: make([]byte, 0x500)}
	m2.CPU = cpu386.New(m2)
	m2.CPU.StepHook = func(c *cpu386.CPU) (bool, error) { calls++; return true, nil }
	p2 := NewLEOPLPorts()
	InstallLEBIOSClock(m2, p2)
	if err := m2.CPU.Step(); err != nil || calls != 1 || p2.BIOSClock.Micros != 1 {
		t.Fatal("hook組合")
	}
	p.Out8(0x43, 0x36)
	p.Out8(0x40, 0x4e)
	p.Out8(0x40, 0x17) // 5966
	for i := 0; i < 5000; i++ {
		if err := p.BIOSClock.advance(m, p, true); err != nil {
			t.Fatal(err)
		}
	}
	if p.BIOSClock.Deliveries != 0 {
		t.Fatal("5966過早")
	}
	p.BIOSClock.advance(m, p, true)
	if p.BIOSClock.Deliveries != 1 {
		t.Fatal("5966週期")
	}
	p.Out8(0x43, 0x36)
	p.Out8(0x40, 0)
	p.Out8(0x40, 0)
	for i := 0; i < 54925; i++ {
		p.BIOSClock.advance(m, p, true)
	}
	if p.BIOSClock.Deliveries != 1 {
		t.Fatal("重載相位")
	}
	p.BIOSClock.advance(m, p, true)
	if p.BIOSClock.Deliveries != 2 {
		t.Fatal("重載完成")
	}
}
