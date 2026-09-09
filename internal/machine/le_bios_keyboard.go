package machine

import (
	"encoding/binary"
	"fmt"
)

type LEBIOSKeyboard struct {
	machine  *LEMachine
	Waiting  bool
	Reads    uint64
	Enqueued []uint16
}

func InstallLEBIOSKeyboard(m *LEMachine) bool {
	if m == nil || m.Keyboard != nil || len(m.Mem) < 0x43e {
		return false
	}
	k := &LEBIOSKeyboard{machine: m}
	if _, _, err := k.positions(); err != nil {
		return false
	}
	m.Keyboard = k
	return true
}
func (k *LEBIOSKeyboard) positions() (uint16, uint16, error) {
	if len(k.machine.Mem) < 0x43e {
		return 0, 0, fmt.Errorf("BIOS按鍵緩衝超界")
	}
	h := binary.LittleEndian.Uint16(k.machine.Mem[0x41a:])
	t := binary.LittleEndian.Uint16(k.machine.Mem[0x41c:])
	if h < 0x1e || h >= 0x3e || t < 0x1e || t >= 0x3e || h&1 != 0 || t&1 != 0 {
		return 0, 0, fmt.Errorf("BIOS按鍵指標無效")
	}
	return h, t, nil
}
func nextBIOSKey(p uint16) uint16 {
	p += 2
	if p == 0x3e {
		p = 0x1e
	}
	return p
}
func (k *LEBIOSKeyboard) Enqueue(key uint16) error {
	h, t, err := k.positions()
	if err != nil {
		return err
	}
	next := nextBIOSKey(t)
	if next == h {
		return fmt.Errorf("BIOS按鍵緩衝已滿")
	}
	binary.LittleEndian.PutUint16(k.machine.Mem[0x400+int(t):], key)
	binary.LittleEndian.PutUint16(k.machine.Mem[0x41c:], next)
	k.Enqueued = append(k.Enqueued, key)
	return nil
}
func (k *LEBIOSKeyboard) ReadEnhanced() (uint16, bool, error) {
	h, t, err := k.positions()
	if err != nil {
		return 0, false, err
	}
	if h == t {
		k.Waiting = true
		return 0, false, nil
	}
	value := binary.LittleEndian.Uint16(k.machine.Mem[0x400+int(h):])
	binary.LittleEndian.PutUint16(k.machine.Mem[0x41a:], nextBIOSKey(h))
	if value&255 == 0xf0 && value>>8 != 0 {
		value &= 0xff00
	}
	k.Waiting = false
	k.Reads++
	return value, true, nil
}
