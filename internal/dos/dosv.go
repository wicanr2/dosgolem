package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// DOS/V 最小集（`docs/spec/010`，READY）。
//
// 原則是**讓原版驅動（DOSJP）自己提供 DOS/V 服務**，機器層只補
// 「一台 DOS/V 機器本來就該有、而且 DOSJP 落腳前要問的」東西。

// dbcsTableOff 是 DBCS 前導位元組表在 StubSeg 裡的位移。
// 內容：`81 9F E0 FC 00 00`（Shift-JIS 前導範圍，雙 0 結束）。
const dbcsTableOff = 0x40

// dbcs 是 `int 21h AH=63h`。量測證據與邊界見 `docs/spec/010`。
func (d *DOS) dbcs(c *cpu.CPU) {
	at := uint32(machine.StubSeg)*16 + dbcsTableOff
	switch al(c) {
	case 0x00: // 取 DBCS 向量表 → DS:SI
		// 第一次被問的時候把預設表放好（Shift-JIS：81–9F、E0–FC）。
		if d.M.Read8(at) == 0 && d.M.Read8(at+1) == 0 {
			d.M.WriteBytes(at, []byte{0x81, 0x9F, 0xE0, 0xFC, 0x00, 0x00})
		}
		c.Seg[cpu.DS] = machine.StubSeg
		c.R[cpu.SI] = dbcsTableOff
		clearCarry(c)
	case 0x01: // 設 DBCS 向量表 ← DS:SI：真的抄回來，之後 AL=00 回抄過的
		src := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
		for i := 0; i < 32; i++ {
			b := d.M.Read8(src + uint32(i))
			d.M.Write8(at+uint32(i), b)
			if b == 0 && i > 0 && d.M.Read8(at+uint32(i)-1) == 0 {
				break // 雙 0 結束
			}
		}
		clearCarry(c)
	default:
		d.note(0x21, 0x63, al(c))
		clearCarry(c)
	}
}
