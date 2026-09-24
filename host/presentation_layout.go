package host

import "fmt"

func presentationGeometry(epoch uint64, scale OutputScale, open bool) MouseLayout {
	s := int(scale)
	chrome := 18 * s
	if open {
		chrome = 92 * s
	}
	return MouseLayout{
		Epoch: epoch, Scale: scale, ChromeHeight: chrome,
		Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 320 * s,
		FrameHeight: chrome + 200*s, PanelOpen: open,
	}
}

// ProjectPresentationLayout applies the fixed 320×200 Ebiten presentation
// geometry to a panel state without touching a window, controller, or bridge.
// It mirrors frontend/ebiten.Game.refreshLayout: closed chrome is 18×scale,
// open chrome is 92×scale, and a changed visible state gets the next epoch.
// The returned changed value is false when current already presents state.
func ProjectPresentationLayout(current MouseLayout, state PanelState) (next MouseLayout, changed bool, err error) {
	if !validOutputScale(state.Scales.ActiveScale) || !validOutputScale(state.Scales.SelectedScale) {
		return MouseLayout{}, false, fmt.Errorf("host: PanelState 倍率無效")
	}
	if current.Epoch == 0 && current != (MouseLayout{}) {
		return MouseLayout{}, false, fmt.Errorf("host: 未初始化 MouseLayout 帶有非零欄位")
	}
	if current.Epoch != 0 && (!validOutputScale(current.Scale) || current != presentationGeometry(current.Epoch, current.Scale, current.PanelOpen)) {
		return MouseLayout{}, false, fmt.Errorf("host: 目前 MouseLayout 與正式呈現幾何不一致")
	}
	if current.Epoch != 0 && current.Scale == state.Scales.ActiveScale && current.PanelOpen == state.Open {
		return current, false, nil
	}
	if current.Epoch == ^uint64(0) {
		return MouseLayout{}, false, fmt.Errorf("host: MouseLayout epoch 溢位")
	}
	next = presentationGeometry(current.Epoch+1, state.Scales.ActiveScale, state.Open)
	if !next.valid() {
		return MouseLayout{}, false, fmt.Errorf("host: 投影出的 MouseLayout 無效")
	}
	return next, true, nil
}
