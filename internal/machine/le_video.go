package machine

import "github.com/wicanr2/dosgolem/internal/cpu386"

// LEVideo 是明示安裝的 mode13 子集；BIOS預設DAC未知，不冒稱完整VGA。
type LEVideo struct {
	machine  *LEMachine
	ports    *LEOPLPorts
	Mode     byte
	ModeSets uint64
}

func InstallLEVideo(m *LEMachine, p *LEOPLPorts) bool {
	if m == nil || p == nil || m.Video != nil {
		return false
	}
	m.Video = &LEVideo{machine: m, ports: p}
	return true
}
func (v *LEVideo) Handle(c *cpu386.CPU) bool {
	if uint16(c.R[cpu386.EAX]) != 0x13 || len(v.machine.Mem) < 0xb0000 {
		return false
	}
	clear(v.machine.Mem[0xa0000:0xb0000])
	v.machine.Mem[0x449] = 0x13
	v.machine.Mem[0x44a] = 40
	v.machine.Mem[0x44b] = 0
	v.Mode = 0x13
	v.ModeSets++
	return true
}
func (v *LEVideo) Indexed() []byte {
	if v.Mode != 0x13 || len(v.machine.Mem) < 0xafa00 {
		return nil
	}
	return append([]byte(nil), v.machine.Mem[0xa0000:0xafa00]...)
}
func (v *LEVideo) Palette() [256][3]byte { return v.ports.device.Palette() }
