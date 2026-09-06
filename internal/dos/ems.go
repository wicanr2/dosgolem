package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
)

// EMS 頁配置與映射（`int 67h` AH=41h/43h/44h/45h）。
// 規格：`docs/spec/008`。查詢子集（40h/42h）在 `exec.go`／spec 007。
//
// 機器模型：頁框段 0xE000（4 個實體頁 × 16 KB），頁池 8 頁。
// 頁內容存在這裡（不佔機器記憶體），映射時複製進頁框、換映射／釋放前
// 寫回——複製式分頁，語意等價的前提是「先映射再存取」（spec §3）。

const (
	emsFrameSeg   = 0xE000
	emsPhysPages  = 4
	emsPageSize   = 16 * 1024
	emsTotalPages = 8
)

// ems 是 EMS 狀態。
type ems struct {
	pages   map[uint16][][]byte // handle → 邏輯頁（每頁 16 KB）
	next    uint16
	free    int
	mappedH [emsPhysPages]uint16 // 實體頁現在映射的 handle（0 ＝ 無）
	mappedL [emsPhysPages]uint16 //            與邏輯頁號
}

func newEMS() *ems {
	return &ems{pages: map[uint16][][]byte{}, next: 1, free: emsTotalPages}
}

// writeBack 把實體頁的內容存回它映射的邏輯頁。
func (d *DOS) emsWriteBack(phys int) {
	e := d.ems
	h := e.mappedH[phys]
	if h == 0 {
		return
	}
	base := uint32(emsFrameSeg+uint16(phys)*0x400) * 16
	for i := 0; i < emsPageSize; i++ {
		e.pages[h][e.mappedL[phys]][i] = d.M.Read8(base + uint32(i))
	}
	e.mappedH[phys] = 0
}

func (d *DOS) emsAlloc(c *cpu.CPU) {
	e := d.ems
	n := int(c.R[cpu.BX])
	switch {
	case n == 0:
		setAH(c, 0x87) // 要 0 頁是錯誤
	case n > e.free:
		setAH(c, 0x88) // 頁不夠
	default:
		h := e.next
		e.next++
		e.pages[h] = make([][]byte, n)
		for i := range e.pages[h] {
			e.pages[h][i] = make([]byte, emsPageSize)
		}
		e.free -= n
		c.R[cpu.DX] = h
		setAH(c, 0)
	}
}

func (d *DOS) emsMap(c *cpu.CPU) {
	e := d.ems
	phys, logi, h := int(al(c)), int(c.R[cpu.BX]), c.R[cpu.DX]
	pages, ok := e.pages[h]
	switch {
	case !ok || h == 0:
		setAH(c, 0x83) // 無效 handle
	case phys >= emsPhysPages:
		setAH(c, 0x8B) // 無效實體頁
	case logi >= len(pages):
		setAH(c, 0x8A) // 邏輯頁超出
	default:
		d.emsWriteBack(phys)
		base := uint32(emsFrameSeg+uint16(phys)*0x400) * 16
		d.M.WriteBytes(base, pages[logi])
		e.mappedH[phys], e.mappedL[phys] = h, uint16(logi)
		setAH(c, 0)
	}
}

func (d *DOS) emsRelease(c *cpu.CPU) {
	e := d.ems
	h := c.R[cpu.DX]
	if _, ok := e.pages[h]; !ok || h == 0 {
		setAH(c, 0x83)
		return
	}
	for p := 0; p < emsPhysPages; p++ {
		if e.mappedH[p] == h {
			d.emsWriteBack(p)
		}
	}
	e.free += len(e.pages[h])
	delete(e.pages, h)
	setAH(c, 0)
}

// int67 是 EMS：查詢子集（40h/41h/42h）＋配置與映射（43h/44h/45h）。
// 本機宣稱 8／8 頁——恰好滿足 launcher 的 `cmp bx,8` 探測下限（spec 007 §4）。
func (d *DOS) int67(c *cpu.CPU) {
	switch ah(c) {
	case 0x40: // 取狀態
		setAH(c, 0)
	case 0x41: // 取頁框段 → BX
		setAH(c, 0)
		c.R[cpu.BX] = emsFrameSeg
	case 0x42: // 取頁數：BX ＝ 可用、DX ＝ 總計
		setAH(c, 0)
		c.R[cpu.BX] = uint16(d.ems.free)
		c.R[cpu.DX] = emsTotalPages
	case 0x43:
		d.emsAlloc(c)
	case 0x44:
		d.emsMap(c)
	case 0x45:
		d.emsRelease(c)
	default:
		d.note(0x67, ah(c), al(c))
		setAH(c, 0x84) // 未定義功能
	}
}
