package dos

import (
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// execOverlay 只實作 DOS EXEC 的 AL=03h：載入 MZ 覆疊後返回呼叫端，
// 不建立子行程或 PSP。
func (d *DOS) execOverlay(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 260)
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.R[cpu.AX] = 5
		setCarry(c)
		return
	}
	param := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BX])
	loadSeg := d.M.Read16(param)
	relocSeg := d.M.Read16(param + 2)
	if err := d.M.LoadOverlay(data, loadSeg, relocSeg); err != nil {
		c.R[cpu.AX] = 11
		setCarry(c)
		return
	}
	d.Opened = append(d.Opened, filepath.Base(path))
	c.R[cpu.AX] = 0
	c.R[cpu.DX] = 0
	clearCarry(c)
}
