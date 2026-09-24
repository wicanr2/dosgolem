package host

import "fmt"

// PanelEventKind 是已經由 host hit test 分類完成的事件。它刻意沒有螢幕座標、
// DOS scan code 或任何 machine reference；那些是視窗後端與未來轉送層的責任。
type PanelEventKind uint8

const (
	PanelEventOpen PanelEventKind = iota + 1
	PanelEventSelectScale
	PanelEventApply
	PanelEventCancel
	PanelEventKeyboard
	PanelEventPointerHostHit
	PanelEventPointerMiss
)

// PanelEvent 是純 host 事件。SelectScale 只有 PanelEventSelectScale 才讀取 Scale。
type PanelEvent struct {
	Kind  PanelEventKind
	Scale OutputScale
}

// InputRoute 是 host 對一個事件的決定。ForwardToDOS 只是許可訊號；此 package
// 不會自行把任何輸入送到 BIOS queue、IRQ 或 DOS mouse API。
type InputRoute struct {
	ConsumedByHost bool
	ForwardToDOS   bool
}

// PanelState 是設定面板的記憶體中狀態。它不表示跨重啟持久化設定。
type PanelState struct {
	Open   bool
	Scales ScaleState
}

// PanelController 實作已確認的 host 操作契約：host hit 一律消費；面板開啟時
// 鍵盤不送 DOS；Apply 提交倍率後自動收合並讓之後的鍵盤恢復可轉送；Cancel 則捨棄
// 暫選值，重設為 active 倍率後收合。
type PanelController struct {
	scales *ScaleController
	open   bool
}

func NewPanelController(initial OutputScale) (*PanelController, error) {
	scales, err := NewScaleController(initial)
	if err != nil {
		return nil, err
	}
	return &PanelController{scales: scales}, nil
}

func (c *PanelController) Snapshot() (PanelState, error) {
	if c == nil || c.scales == nil {
		return PanelState{}, fmt.Errorf("host: PanelController 不得為 nil")
	}
	scales, err := c.scales.Snapshot()
	if err != nil {
		return PanelState{}, err
	}
	return PanelState{Open: c.open, Scales: scales}, nil
}

// Route 套用一個已分類 host event 並回傳其消費／轉送結果。所有 host hit 與面板
// 開啟時的鍵盤都保證 ForwardToDOS=false；未知或時序不合法事件不改狀態。
func (c *PanelController) Route(event PanelEvent) (PanelState, InputRoute, error) {
	state, err := c.Snapshot()
	if err != nil {
		return PanelState{}, InputRoute{}, err
	}
	next, route, err := PlanPanelRoute(state, event)
	if err != nil {
		return PanelState{}, InputRoute{}, err
	}
	// The value-only plan has already validated the whole transition. Nothing
	// reaches DOS here; the controller commits its own two private fields only.
	c.scales.state = next.Scales
	c.open = next.Open
	return next, route, nil
}
