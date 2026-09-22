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
	if c == nil || c.scales == nil {
		return PanelState{}, InputRoute{}, fmt.Errorf("host: PanelController 不得為 nil")
	}
	switch event.Kind {
	case PanelEventOpen:
		if c.open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板已開啟")
		}
		c.open = true
		return c.after(InputRoute{ConsumedByHost: true})
	case PanelEventSelectScale:
		if !c.open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能選擇倍率")
		}
		if _, err := c.scales.Select(event.Scale); err != nil {
			return PanelState{}, InputRoute{}, err
		}
		return c.after(InputRoute{ConsumedByHost: true})
	case PanelEventApply:
		if !c.open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能 Apply")
		}
		if _, _, err := c.scales.Apply(); err != nil {
			return PanelState{}, InputRoute{}, err
		}
		// 使用者已確認 Apply 後必須自動收合，不論倍率是否實際改變。
		c.open = false
		return c.after(InputRoute{ConsumedByHost: true})
	case PanelEventCancel:
		if !c.open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能 Cancel")
		}
		state, err := c.scales.Snapshot()
		if err != nil {
			return PanelState{}, InputRoute{}, err
		}
		// Select 在失敗時不改 ScaleController，因此只有成功重設 selected 後才收合，
		// 以維持 Cancel 的狀態轉移原子性。
		if _, err := c.scales.Select(state.ActiveScale); err != nil {
			return PanelState{}, InputRoute{}, err
		}
		c.open = false
		return c.after(InputRoute{ConsumedByHost: true})
	case PanelEventKeyboard:
		if c.open {
			return c.after(InputRoute{ConsumedByHost: true})
		}
		return c.after(InputRoute{ForwardToDOS: true})
	case PanelEventPointerHostHit:
		return c.after(InputRoute{ConsumedByHost: true})
	case PanelEventPointerMiss:
		// 未命中 pointer event 的 DOS mouse 轉送仍是 backend 的未決工作；不猜測。
		return c.after(InputRoute{})
	default:
		return PanelState{}, InputRoute{}, fmt.Errorf("host: 未知面板事件 %d", event.Kind)
	}
}

func (c *PanelController) after(route InputRoute) (PanelState, InputRoute, error) {
	state, err := c.Snapshot()
	return state, route, err
}
