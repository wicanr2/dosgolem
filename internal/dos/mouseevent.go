package dos

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
