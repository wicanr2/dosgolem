package dos

import "fmt"

// 滑鼠事件回呼的相容名稱。
//
// 事件遮罩的位元定義與 `bios.go` 的 `Event*` 相同；這裡的 `Ev*` 是早期
// 的名字，留著讓既有的呼叫端（`cmd/probe`、`cmd/clickgrid`、apps）
// 不必一起改。
//
// ⚠ **回呼是排進機器的佇列，不是當場跳過去**（`internal/machine/callback.go`）。
// 早期版本在服務層裡直接改 CS:IP 再靠一個哨兵中斷收尾；那條路只有在
// 「剛好在指令邊界」時才成立，而 MouseEvent 是從測試腳本與 oracle 呼叫的，
// 不在指令邊界上。
const (
	EvMove       = EventMove
	EvLeftDown   = EventLeftDown
	EvLeftUp     = EventLeftUp
	EvRightDown  = EventRightDown
	EvRightUp    = EventRightUp
	EvMiddleDown = 1 << 5
	EvMiddleUp   = 1 << 6
)

// MouseEvent 發一次事件給遊戲登記的處理常式。
//
// 回 true 表示真的排進佇列了（遊戲有登記，而且遮罩開著這一種事件）。
// **回 false 不代表出錯**——多數程式只登記自己在意的那幾種。
func (d *DOS) MouseEvent(flags uint16) bool {
	m := &d.Mouse
	if !m.Handler.Set || m.Handler.Mask&flags == 0 {
		return false
	}
	d.fireMouseEvent(flags)
	return true
}

// callbackBudget 是同步派送等回呼返回的指令上限。
//
// 沒有上限的話，回呼裡的無窮迴圈會變成整個測試掛住不動——而掛住不動
// 與「這一支還在算」看起來一樣。
const callbackBudget = 1_000_000

// RunMouseEvent 發一次事件並**跑到回呼返回為止**，回呼看到的 SI/DI
// 是這一次位移的 mickey 數。
//
// 與 MouseEvent 的差別只有時機：MouseEvent 排進佇列、由下一個指令邊界
// 送出去（跑分那條路要的行為）；這一支適合「發完就要看結果」的呼叫端
// （測試、逐點對拍）。回呼返回時暫存器由 machine.FinishCallback 整份還原，
// 兩條路都一樣。
//
// 回 (false, nil) 表示遊戲沒登記或遮罩沒開這一種事件——**那不是錯誤**。
func (d *DOS) RunMouseEvent(flags uint16, dx, dy int16) (bool, error) {
	m := &d.Mouse
	if !m.Handler.Set || m.Handler.Mask&flags == 0 {
		return false, nil
	}
	before := d.M.CallbacksMade()
	discards := d.M.MouseIRQ12Discards()
	d.fireMouseEventMickeys(flags, dx, dy)
	for i := 0; i < callbackBudget; i++ {
		if err := d.M.Step(); err != nil {
			return false, fmt.Errorf("執行 int 33h 回呼 %04X:%04X：%w",
				m.Handler.Seg, m.Handler.Off, err)
		}
		if d.M.CallbacksMade() > before && !d.M.CallbackActive() {
			return true, nil
		}
		// 459 §3.8：保護模式剖面的事件被丟棄（登記在遞送前失效，或 AX=0000h 清空）＝與「遊戲沒登記」同義。
		if d.M.MouseIRQ12Discards() > discards {
			return false, nil
		}
	}
	if d.M.MouseIRQ12Active() {
		pending, isr4, isr2, uir, queued, _ := d.M.MouseIRQ12()
		st := d.M.PICStats()
		return false, fmt.Errorf("int 33h 回呼 %04X:%04X 在 %d 道指令內沒有返回（IRQ12：待處理=%t ISR4=%t ISR2=%t UIR=%t "+
			"從片 IMR=%02Xh 佇列 %d 筆；docs/spec/459 §3.8）",
			m.Handler.Seg, m.Handler.Off, callbackBudget, pending, isr4, isr2, uir, st.SlaveIMR, queued)
	}
	return false, fmt.Errorf("int 33h 回呼 %04X:%04X 在 %d 道指令內沒有返回",
		m.Handler.Seg, m.Handler.Off, callbackBudget)
}
