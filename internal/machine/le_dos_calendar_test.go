package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func calendarFixture(t *testing.T) (*MOO2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	s, m, _ := mouseCallbackFixture(t, 0)
	c := m.CPU
	c.R = [8]uint32{0xabcd2aa8, 0x12345678, 0x98765432, 0xfeedbeef, 0x80000, 0x34567890, 0x456789ab, 0x56789abc}
	c.EFlags = 0x847
	c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x123, 2, [8]float64{3, 4}
	return s, m, s.DPMI.RealModeIO.(*LEOPLPorts)
}
func TestDOSCalendarKnownDatesAndMidnight(t *testing.T) {
	for _, tc := range []struct {
		y, m, d             int
		days                uint64
		wy, wm, wd, weekday uint32
	}{
		{1980, 1, 1, 0, 1980, 1, 1, 2},
		{1996, 1, 1, 0, 1996, 1, 1, 1},
		{1996, 1, 1, 31, 1996, 2, 1, 4},
		{2000, 2, 28, 0, 2000, 2, 28, 1},
		{2000, 2, 28, 1, 2000, 2, 29, 2},
		{2000, 2, 28, 2, 2000, 3, 1, 3},
		{1999, 12, 31, 1, 2000, 1, 1, 6},
		{2099, 12, 31, 0, 2099, 12, 31, 4},
	} {
		s, m, p := calendarFixture(t)
		if err := s.SetCalendarEpoch(tc.y, tc.m, tc.d); err != nil {
			t.Fatal(err)
		}
		p.virtualMicros, p.BIOSClock.Micros = tc.days*dosCalendarDayMicros+123456, tc.days*dosCalendarDayMicros+123456
		for repeat := 0; repeat < 2; repeat++ {
			c := m.CPU
			c.R[cpu386.EAX] = 0xabcd2aa8
			before := irq0Snapshot(c)
			mem := append([]byte(nil), m.Mem...)
			state := p.State()
			calls, timeCalls := s.calls, s.timeCalls
			if !s.Handle(c, 0x21) {
				t.Fatal("日期拒絕")
			}
			want := before
			want.r[cpu386.EAX] = 0xabcd2a00 | tc.weekday
			want.r[cpu386.ECX] = 0x12340000 | tc.wy
			want.r[cpu386.EDX] = 0x98760000 | tc.wm<<8 | tc.wd
			if irq0Snapshot(c) != want || !bytes.Equal(mem, m.Mem) || p.State() != state || s.calls != calls || s.timeCalls != timeCalls {
				t.Fatal("完整核心／記憶體／時間或返回欄位錯誤")
			}
		}
	}
	for _, real := range []bool{false, true} {
		s, m, p := calendarFixture(t)
		if err := s.SetCalendarEpoch(2000, 2, 28); err != nil {
			t.Fatal(err)
		}
		p.virtualMicros, p.BIOSClock.Micros = dosCalendarDayMicros-1, dosCalendarDayMicros-1
		m.CPU.EFlags = 2
		if !s.Handle(m.CPU, 0x21) || uint16(m.CPU.R[cpu386.EDX]) != 0x021c || uint8(m.CPU.R[cpu386.EAX]) != 1 {
			t.Fatal("午夜前")
		}
		if real {
			r := &cpu.CPU{Flags: 2}
			if err := p.AdvanceRealMode(r, m); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := p.AdvanceProtectedMode(m.CPU, m); err != nil {
				t.Fatal(err)
			}
		}
		m.CPU.R[cpu386.EAX] = 0x2a00
		if !s.Handle(m.CPU, 0x21) || uint16(m.CPU.R[cpu386.EDX]) != 0x021d || uint8(m.CPU.R[cpu386.EAX]) != 2 || p.virtualMicros != dosCalendarDayMicros || p.BIOSClock.Micros != dosCalendarDayMicros {
			t.Fatal("兩模式跨日")
		}
	}
}
func TestDOSCalendarInvalidSetupAndSources(t *testing.T) {
	for _, date := range [][3]int{{1979, 1, 1}, {2100, 1, 1}, {1996, 0, 1}, {1996, 13, 1}, {1996, 1, 0}, {1996, 1, 32}, {1996, 4, 31}, {1995, 2, 29}, {2000, 2, 30}} {
		s, m, p := calendarFixture(t)
		before := irq0Snapshot(m.CPU)
		state := p.State()
		if s.SetCalendarEpoch(date[0], date[1], date[2]) == nil || s.calendar != nil || irq0Snapshot(m.CPU) != before || p.State() != state {
			t.Fatal("無效設定改狀態")
		}
	}
	if NewMOO2StartupDOS(nil).SetCalendarEpoch(1996, 1, 1) == nil {
		t.Fatal("未附掛設定")
	}
	for _, change := range []func(*MOO2StartupDOS, *LEMachine, *LEOPLPorts){
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.moo2Profile = false },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { p.virtualMicros = 1 },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { p.BIOSClock.Micros = 1 },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { p.BIOSClock = nil },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.DPMI.RealModeIO = nil },
	} {
		s, m, p := calendarFixture(t)
		change(s, m, p)
		before := irq0Snapshot(m.CPU)
		if s.SetCalendarEpoch(1996, 1, 1) == nil || s.calendar != nil || irq0Snapshot(m.CPU) != before {
			t.Fatal("無效来源設定")
		}
	}
	for _, change := range []func(*MOO2StartupDOS, *LEMachine, *LEOPLPorts){
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.calendar = nil },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.moo2Profile = false },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { p.virtualMicros = 1 },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { p.BIOSClock = nil },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.DPMI.RealModeIO = NewLEOPLPorts() },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) { s.DPMI.m = &LEMachine{} },
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) {
			p.virtualMicros, p.BIOSClock.Micros = dosCalendarDayMicros, dosCalendarDayMicros
		},
		func(s *MOO2StartupDOS, m *LEMachine, p *LEOPLPorts) {
			p.virtualMicros, p.BIOSClock.Micros = ^uint64(0), ^uint64(0)
		},
	} {
		s, m, p := calendarFixture(t)
		if err := s.SetCalendarEpoch(2099, 12, 31); err != nil {
			t.Fatal(err)
		}
		if s.SetCalendarEpoch(1996, 1, 1) == nil {
			t.Fatal("重複設定")
		}
		change(s, m, p)
		before := irq0Snapshot(m.CPU)
		mem := append([]byte(nil), m.Mem...)
		if s.Handle(m.CPU, 0x21) || irq0Snapshot(m.CPU) != before || !bytes.Equal(mem, m.Mem) {
			t.Fatal("無效来源回傳日期或改核心")
		}
	}
	s, m, _ := calendarFixture(t)
	before := irq0Snapshot(m.CPU)
	if NewFD2StartupDOS(nil).Handle(m.CPU, 0x21) || irq0Snapshot(m.CPU) != before || s.Handle(m.CPU, 0x22) {
		t.Fatal("其他程式／INT接受")
	}
}
func TestDOSCalendarActualINTAndConsumer(t *testing.T) {
	s, m, p := calendarFixture(t)
	c := m.CPU
	if err := s.SetCalendarEpoch(1996, 1, 1); err != nil {
		t.Fatal(err)
	}
	copy(m.Mem[0x600:], []byte{0xcd, 0x21, 0x66, 0x81, 0xe9, 0x6c, 0x07, 0x88, 0xc5, 0xc1, 0xe1, 0x10, 0x66, 0x89, 0xd1})
	c.IntHook = s.Handle
	c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0, 0, 2
	if err := c.Step(); err != nil || c.EIP != 0x602 || c.R[cpu386.ECX] != 1996 || c.R[cpu386.EDX] != 0x101 || c.R[cpu386.EAX] != 0xabcd2a01 {
		t.Fatalf("實際INT返回：%v %+v", err, c.R)
	}
	if err := c.Step(); err != nil || c.EIP != 0x607 || c.R[cpu386.ECX] != 96 {
		t.Fatalf("原版形狀SUB consumer：err=%v eip=%X r=%X", err, c.EIP, c.R)
	}
	if err := c.Step(); err != nil || c.EIP != 0x609 || uint8(c.R[cpu386.ECX]>>8) != 1 {
		t.Fatal("weekday consumer")
	}
	if p.virtualMicros != 3 || p.BIOSClock.Micros != 3 {
		t.Fatal("查日期補推時鐘")
	}
}
