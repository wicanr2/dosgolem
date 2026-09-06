package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EMS 最小集（`docs/spec/014`）：LIM/EMS 4.0 的 `int 67h`。
//
// 1994 年的 DOS 遊戲把放不進 640 KB 的資料放這裡：源平合戰的
// `Opendat.gp` 有 853 KB、`Enddat.gp` 有 1,013 KB。**偵測不到 EMS 的話
// 程式不會報錯，它只是不去讀那些檔**——開場停在 logo，看起來像卡住。

// emsPageSize 是一頁 16 KB；page frame 有四個實體頁。
const emsPageSize = 16 * 1024

// emsTotalPages 是宣告的總頁數（8 MB）。
const emsTotalPages = 512

// emsHandle 是一個 EMS handle 擁有的邏輯頁。
type emsHandle struct {
	pages [][]byte
}

// ems 是 EMS 的狀態。
type ems struct {
	handles map[uint16]*emsHandle
	next    uint16

	// frame[p] 是實體頁 p 目前映著誰（handle 與邏輯頁），nil ＝ 空。
	// saved 是 AH=47h 存下來的那一份。
	frame, saved [4]*emsMapping
}

type emsMapping struct {
	h    uint16
	page uint16
}

// emsPages 回傳目前配出去的總頁數。
func (e *ems) used() int {
	n := 0
	for _, h := range e.handles {
		n += len(h.pages)
	}
	return n
}

func (d *DOS) emsState() *ems {
	if d.ems == nil {
		d.ems = &ems{handles: map[uint16]*emsHandle{}, next: 1}
	}
	return d.ems
}

// emsFrameAddr 是實體頁 p 在 1 MB 空間裡的位址。
func emsFrameAddr(p int) uint32 {
	return uint32(machine.EMSFrameSeg)*16 + uint32(p)*emsPageSize
}

// emsFlush 把實體頁 p 的內容寫回它映著的邏輯頁。
//
// ⚠ **不寫回的話「換頁再換回來」會拿到舊資料**，而且完全不報錯——
// 遊戲的資料看起來就是隨機少了一塊。
func (d *DOS) emsFlush(p int) {
	e := d.emsState()
	m := e.frame[p]
	if m == nil {
		return
	}
	h, ok := e.handles[m.h]
	if !ok || int(m.page) >= len(h.pages) {
		return
	}
	copy(h.pages[m.page], d.M.Mem[emsFrameAddr(p):emsFrameAddr(p)+emsPageSize])
}

// emsLoad 把邏輯頁的內容搬進實體頁 p。
func (d *DOS) emsLoad(p int, handle, page uint16) {
	e := d.emsState()
	h := e.handles[handle]
	copy(d.M.Mem[emsFrameAddr(p):emsFrameAddr(p)+emsPageSize], h.pages[page])
	e.frame[p] = &emsMapping{h: handle, page: page}
}

// emsCall 是 `int 67h` 的分派（經 `int F6h` trampoline 進來）。
//
// **回傳慣例是 AH ＝ 0 成功、非 0 是錯誤碼**，不是 DOS 的 CF。
func (d *DOS) emsCall(c *cpu.CPU) {
	e := d.emsState()
	switch ah(c) {
	case 0x40: // Get Status
		setAH(c, 0)

	case 0x41: // Get Page Frame Segment
		c.R[cpu.BX] = machine.EMSFrameSeg
		setAH(c, 0)

	case 0x42: // Get Page Counts：BX ＝ 未配置、DX ＝ 總數
		c.R[cpu.BX] = uint16(emsTotalPages - e.used())
		c.R[cpu.DX] = emsTotalPages
		setAH(c, 0)

	case 0x43: // Allocate Pages：BX ＝ 頁數 → DX ＝ handle
		want := int(c.R[cpu.BX])
		if want == 0 || want > emsTotalPages-e.used() {
			setAH(c, 0x87) // 頁數不足
			return
		}
		h := &emsHandle{pages: make([][]byte, want)}
		for i := range h.pages {
			h.pages[i] = make([]byte, emsPageSize)
		}
		id := e.next
		e.next++
		e.handles[id] = h
		c.R[cpu.DX] = id
		setAH(c, 0)

	case 0x44: // Map Handle Page：AL ＝ 實體頁、BX ＝ 邏輯頁、DX ＝ handle
		p := int(al(c))
		if p > 3 {
			setAH(c, 0x8B) // 實體頁超範圍
			return
		}
		h, ok := e.handles[c.R[cpu.DX]]
		if !ok {
			setAH(c, 0x83) // handle 無效
			return
		}
		d.emsFlush(p)
		if c.R[cpu.BX] == 0xFFFF { // 解除映射
			e.frame[p] = nil
			setAH(c, 0)
			return
		}
		if int(c.R[cpu.BX]) >= len(h.pages) {
			setAH(c, 0x8A) // 邏輯頁超範圍
			return
		}
		d.emsLoad(p, c.R[cpu.DX], c.R[cpu.BX])
		setAH(c, 0)

	case 0x45: // Release Handle
		id := c.R[cpu.DX]
		if _, ok := e.handles[id]; !ok {
			setAH(c, 0x83)
			return
		}
		for p := 0; p < 4; p++ {
			if m := e.frame[p]; m != nil && m.h == id {
				d.emsFlush(p)
				e.frame[p] = nil
			}
		}
		delete(e.handles, id)
		setAH(c, 0)

	case 0x46: // Get Version
		setAL(c, 0x40) // 4.0
		setAH(c, 0)

	case 0x47: // Save Page Map
		e.saved = e.frame
		setAH(c, 0)

	case 0x48: // Restore Page Map
		for p := 0; p < 4; p++ {
			d.emsFlush(p)
		}
		for p := 0; p < 4; p++ {
			m := e.saved[p]
			if m == nil {
				e.frame[p] = nil
				continue
			}
			if h, ok := e.handles[m.h]; ok && int(m.page) < len(h.pages) {
				d.emsLoad(p, m.h, m.page)
			}
		}
		setAH(c, 0)

	case 0x4B: // Get Handle Count
		c.R[cpu.BX] = uint16(len(e.handles))
		setAH(c, 0)

	case 0x4C: // Get Pages for Handle
		h, ok := e.handles[c.R[cpu.DX]]
		if !ok {
			setAH(c, 0x83)
			return
		}
		c.R[cpu.BX] = uint16(len(h.pages))
		setAH(c, 0)

	case 0x58: // Get Mappable Physical Address Array
		// AL=00：把陣列寫到 ES:DI（每項 ＝ 段 ＋ 實體頁號）；AL=01：只回項數。
		// **這是 EMS 4.0 程式問「page frame 長什麼樣」的方式**，
		// 回 84h（沒這功能）它就當成不能用。
		if al(c) == 0x00 {
			dst := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI])
			for p := 0; p < 4; p++ {
				d.M.Write16(dst+uint32(p)*4, machine.EMSFrameSeg+uint16(p)*(emsPageSize/16))
				d.M.Write16(dst+uint32(p)*4+2, uint16(p))
			}
		}
		c.R[cpu.CX] = 4
		setAH(c, 0)

	default:
		d.note(0x67, ah(c), al(c))
		setAH(c, 0x84) // 沒有這個功能
	}
}
