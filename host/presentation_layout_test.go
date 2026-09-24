package host

import "testing"

func TestProjectPresentationLayoutMirrorsClosedAndOpenEbitenGeometry(t *testing.T) {
	closed := PanelState{Scales: ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale2}}
	first, changed, err := ProjectPresentationLayout(MouseLayout{}, closed)
	if err != nil || !changed || first != (MouseLayout{
		Epoch: 1, Scale: OutputScale2, ChromeHeight: 36,
		Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 640, FrameHeight: 436,
	}) {
		t.Fatalf("closed projection = (%+v, %t, %v)", first, changed, err)
	}
	open := closed
	open.Open = true
	opened, changed, err := ProjectPresentationLayout(first, open)
	if err != nil || !changed || opened != (MouseLayout{
		Epoch: 2, Scale: OutputScale2, ChromeHeight: 184,
		Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 640, FrameHeight: 584, PanelOpen: true,
	}) {
		t.Fatalf("open projection = (%+v, %t, %v)", opened, changed, err)
	}
	applied := open
	applied.Open = false
	applied.Scales.ActiveScale, applied.Scales.SelectedScale = OutputScale3, OutputScale3
	three, changed, err := ProjectPresentationLayout(opened, applied)
	if err != nil || !changed || three != (MouseLayout{
		Epoch: 3, Scale: OutputScale3, ChromeHeight: 54,
		Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 960, FrameHeight: 654,
	}) {
		t.Fatalf("Apply projection = (%+v, %t, %v)", three, changed, err)
	}
	unchanged, changed, err := ProjectPresentationLayout(three, applied)
	if err != nil || changed || unchanged != three {
		t.Fatalf("unchanged projection = (%+v, %t, %v)", unchanged, changed, err)
	}
}

func TestProjectPresentationLayoutRejectsNoncanonicalNoop(t *testing.T) {
	state := PanelState{Scales: ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale2}}
	wrong := MouseLayout{Epoch: 1, Scale: OutputScale2, Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 640, FrameHeight: 400}
	if _, changed, err := ProjectPresentationLayout(wrong, state); err == nil || changed {
		t.Fatalf("noncanonical current layout = (changed=%t, err=%v), want rejection", changed, err)
	}
	opened := state
	opened.Open = true
	if _, changed, err := ProjectPresentationLayout(wrong, opened); err == nil || changed {
		t.Fatalf("noncanonical source on transition = (changed=%t, err=%v), want rejection", changed, err)
	}
	zeroEpochWithFields := MouseLayout{Scale: OutputScale2}
	if _, changed, err := ProjectPresentationLayout(zeroEpochWithFields, state); err == nil || changed {
		t.Fatalf("malformed uninitialized source = (changed=%t, err=%v), want rejection", changed, err)
	}
}
