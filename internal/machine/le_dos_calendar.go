package machine

import (
	"fmt"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

const dosCalendarDayMicros uint64 = 86_400_000_000

type leDOSCalendar struct {
	epoch   time.Time
	ports   *LEOPLPorts
	machine *LEMachine
}

// SetCalendarEpoch 依規格310明示虛擬零時的日期；不得在執行後修改。
func (s *MOO2StartupDOS) SetCalendarEpoch(year, month, day int) error {
	if s == nil || s.FD2StartupDOS == nil || !s.moo2Profile || s.calendar != nil || s.DPMI == nil || s.DPMI.m == nil || s.DPMI.m.CPU == nil {
		return fmt.Errorf("日曆設定缺啟動機器或已設定")
	}
	p, ok := s.DPMI.RealModeIO.(*LEOPLPorts)
	if !ok || p.BIOSClock == nil || p.virtualMicros != 0 || p.BIOSClock.Micros != 0 {
		return fmt.Errorf("日曆只能在已附掛且尚未執行的共用時計設定")
	}
	if year < 1980 || year > 2099 || month < 1 || month > 12 || day < 1 || day > 31 {
		return fmt.Errorf("日曆日期超出DOS範圍")
	}
	epoch := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if epoch.Year() != year || int(epoch.Month()) != month || epoch.Day() != day {
		return fmt.Errorf("日曆日期不存在")
	}
	s.calendar = &leDOSCalendar{epoch: epoch, ports: p, machine: s.DPMI.m}
	return nil
}

// CalendarState 是唯讀初態與時間收據，不回傳內部指標。
func (s *MOO2StartupDOS) CalendarState() (epoch string, micros uint64, configured bool) {
	if s == nil || s.FD2StartupDOS == nil || s.calendar == nil {
		return "", 0, false
	}
	return s.calendar.epoch.Format("2006-01-02"), s.calendar.ports.virtualMicros, true
}

func (s *FD2StartupDOS) getCalendarDate(c *cpu386.CPU) bool {
	d := s.calendar
	if c == nil || !s.moo2Profile || d == nil || s.DPMI == nil || s.DPMI.m != d.machine || d.machine.CPU != c || s.DPMI.RealModeIO != d.ports || d.ports.BIOSClock == nil || d.ports.BIOSClock.Micros != d.ports.virtualMicros {
		return false
	}
	days := d.ports.virtualMicros / dosCalendarDayMicros
	limit := uint64(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Sub(d.epoch) / (24 * time.Hour))
	if days >= limit {
		return false
	}
	date := d.epoch.AddDate(0, 0, int(days))
	// 只替換公開DOS返回欄位；高半部／其他核心與flags保持是平台近似。
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffffff00 | uint32(date.Weekday())
	c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | uint32(date.Year())
	c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(date.Month())<<8 | uint32(date.Day())
	return true
}
