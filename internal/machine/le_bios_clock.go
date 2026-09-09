package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// LEBIOSClock 為預設BIOS計時服務；指令時間為明示近似，不是實機週期。
type LEBIOSClock struct {
	Micros     uint64
	Deliveries uint64
	Pending    bool
	credit     uint64
	generation uint64
}

func InstallLEBIOSClock(m *LEMachine, p *LEOPLPorts) bool {
	if m == nil || m.CPU == nil || p == nil || p.BIOSClock != nil || len(m.Mem) < 0x471 {
		return false
	}
	p.BIOSClock = &LEBIOSClock{}
	previous := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if err := p.BIOSClock.advance(m, p, c.EFlags&cpu386.IF != 0); err != nil {
			return true, err
		}
		if previous != nil {
			return previous(c)
		}
		return false, nil
	}
	return true
}
func (b *LEBIOSClock) advance(m *LEMachine, p *LEOPLPorts, enabled bool) error {
	if len(m.Mem) < 0x471 {
		return fmt.Errorf("BIOS時鐘資料區不可讀寫")
	}
	b.Micros++
	reload := uint32(65536)
	if p.PIT0.Configured {
		if !p.PIT0.Loaded {
			return b.deliver(m, p, enabled)
		}
		reload = p.PIT0.Reload
	}
	if reload == 0 {
		return fmt.Errorf("PIT重載不可為零")
	}
	if b.generation != p.PIT0.Generation {
		b.credit = 0
		b.generation = p.PIT0.Generation
	}
	b.credit += 1193182
	period := uint64(reload) * 1000000
	if b.credit >= period {
		b.credit %= period
		b.Pending = true
	}
	return b.deliver(m, p, enabled)
}
func (b *LEBIOSClock) deliver(m *LEMachine, p *LEOPLPorts, enabled bool) error {
	if !b.Pending || !enabled || p.picMasks[0]&1 != 0 {
		return nil
	}
	for _, n := range []int{8, 0x1c} {
		if binary.LittleEndian.Uint32(m.Mem[n*4:]) != 0 {
			return fmt.Errorf("BIOS時鐘尚不支援客製INT%02X", n)
		}
	}
	value := binary.LittleEndian.Uint32(m.Mem[0x46c:]) + 1
	if value >= 0x1800b0 {
		value = 0
		m.Mem[0x470]++
	}
	binary.LittleEndian.PutUint32(m.Mem[0x46c:], value)
	b.Pending = false
	b.Deliveries++
	return nil
}
