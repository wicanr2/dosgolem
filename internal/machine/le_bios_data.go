package machine

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// InstallDOS4GWBIOSData 顯式建立既有 BIOS 初始資料；不覆寫 LE 程式內容。
func InstallDOS4GWBIOSData(m *LEMachine) error {
	if m == nil || m.CPU == nil || len(m.Mem) < 0x500 {
		return fmt.Errorf("machine: BIOS 資料區未映射")
	}
	if _, exists := m.CPU.Descriptors[0x40]; exists {
		return fmt.Errorf("machine: BIOS selector 0040 已存在")
	}
	for _, b := range m.Mem[0x400:0x500] {
		if b != 0 {
			return fmt.Errorf("machine: BIOS 資料區與既有資料衝突")
		}
	}
	bios := &Machine{Mem: m.Mem}
	bios.initBDA()
	m.CPU.SetDescriptor(0x40, cpu386.Descriptor{Base: 0x400, Limit: 0xfff, Writable: true})
	return nil
}
