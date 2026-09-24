package host

import "fmt"

// PlanPanelRoute predicts one PanelController.Route call using values only.
// It has no controller, DOS, window, or input-output capability. A session can
// validate an entire batch before changing its private controller.
func PlanPanelRoute(state PanelState, event PanelEvent) (PanelState, InputRoute, error) {
	if !validOutputScale(state.Scales.ActiveScale) || !validOutputScale(state.Scales.SelectedScale) {
		return PanelState{}, InputRoute{}, fmt.Errorf("host: PanelState 倍率無效")
	}
	next := state
	switch event.Kind {
	case PanelEventOpen:
		if state.Open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板已開啟")
		}
		next.Open = true
		return next, InputRoute{ConsumedByHost: true}, nil
	case PanelEventSelectScale:
		if !state.Open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能選擇倍率")
		}
		if !validOutputScale(event.Scale) {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 不支援的輸出倍率 %d", event.Scale)
		}
		next.Scales.SelectedScale = event.Scale
		return next, InputRoute{ConsumedByHost: true}, nil
	case PanelEventApply:
		if !state.Open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能 Apply")
		}
		next.Scales.ActiveScale = state.Scales.SelectedScale
		next.Open = false
		return next, InputRoute{ConsumedByHost: true}, nil
	case PanelEventCancel:
		if !state.Open {
			return PanelState{}, InputRoute{}, fmt.Errorf("host: 設定面板未開啟，不能 Cancel")
		}
		next.Scales.SelectedScale = state.Scales.ActiveScale
		next.Open = false
		return next, InputRoute{ConsumedByHost: true}, nil
	case PanelEventKeyboard:
		if state.Open {
			return next, InputRoute{ConsumedByHost: true}, nil
		}
		return next, InputRoute{ForwardToDOS: true}, nil
	case PanelEventPointerHostHit:
		return next, InputRoute{ConsumedByHost: true}, nil
	case PanelEventPointerMiss:
		return next, InputRoute{}, nil
	default:
		return PanelState{}, InputRoute{}, fmt.Errorf("host: 未知面板事件 %d", event.Kind)
	}
}
