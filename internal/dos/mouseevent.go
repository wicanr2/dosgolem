package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 事件旗標（`int 33h AX=000Ch` 的遮罩與回呼時的 AX）。
const (
	EvMove       = 0x01
	EvLeftDown   = 0x02
	EvLeftUp     = 0x04
	EvRightDown  = 0x08
	EvRightUp    = 0x10
	EvMiddleDown = 0x20
	EvMiddleUp   = 0x40
)

// cbRetInt 是回呼結束的哨兵中斷。回呼的 far 返回位址指向 stub 段裡
// 這一號的 `CD FF`，handler 一 `retf` 就會執行它，於是控制權回到 handle()。
//
// 選 0FFh 是因為**沒有任何程式會用它**，而且 stub 段本來就每一號都有一格；
// 不必為了收尾另外在 CPU 挖一個鉤子。
const cbRetInt = 0xFF

// cbFrame 是回呼前的 CPU 快照。只存暫存器與旗標——**不整份複製 CPU**，
// 那會把 IntHook、Model 這些設定也一起倒回去。
type cbFrame struct {
	R     [8]uint16
	Seg   [4]uint16
	IP    uint16
	Flags uint16
}

// MouseEvent 依 `AX=000Ch` 登錄的遮罩送一次事件回呼。
//
// **輪詢與回呼是兩條路，遊戲可以只走一條。** GIN3PS 的主選單就是這樣：
// 它用 `AX=3` 拿座標把游標畫出來（所以游標會動，看起來像有在收輸入），
// 但「按下」只從事件回呼進去——不回呼的話按幾次都沒有反應，
// 而且畫面上完全看不出差別。
//
// 回傳 true 表示這一步安排了回呼；呼叫端必須在 CPU 的指令邊界呼叫它。
func (d *DOS) MouseEvent(flags uint16) bool {
	m := &d.Mouse
	if m.EventSeg == 0 && m.EventOff == 0 {
		return false
	}
	if m.EventMask&flags == 0 || d.cbActive {
		return false
	}
	c := d.M.CPU
	d.cbSave = cbFrame{R: c.R, Seg: c.Seg, IP: c.IP, Flags: c.Flags}
	d.cbActive = true

	// 推 far 返回位址（哨兵），驅動的 handler 用 retf 回來。
	push := func(v uint16) {
		c.R[cpu.SP] -= 2
		d.M.Write16(uint32(c.Seg[cpu.SS])*16+uint32(c.R[cpu.SP]), v)
	}
	push(machine.StubSeg)
	push(machine.StubOff(cbRetInt))

	scale := m.XScale
	if scale == 0 {
		scale = uint16(640 / d.M.PixelWidth())
	}
	c.R[cpu.AX] = flags
	c.R[cpu.BX] = m.Buttons
	c.R[cpu.CX] = m.X * scale
	c.R[cpu.DX] = m.Y
	c.R[cpu.SI], c.R[cpu.DI] = 0, 0
	c.Seg[cpu.CS], c.IP = m.EventSeg, m.EventOff
	m.Events = append(m.Events, Poll{X: m.X, Y: m.Y, Buttons: flags, Step: d.M.Steps})
	return true
}

// cbReturn 收尾：把回呼前的暫存器裝回去。
func (d *DOS) cbReturn(c *cpu.CPU) {
	if !d.cbActive {
		return
	}
	c.R, c.Seg, c.IP, c.Flags = d.cbSave.R, d.cbSave.Seg, d.cbSave.IP, d.cbSave.Flags
	d.cbActive = false
}
