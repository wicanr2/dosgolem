package host

import "testing"

func mustPanel(t *testing.T, c *PanelController) PanelState {
	t.Helper()
	state, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func mustRoute(t *testing.T, c *PanelController, event PanelEvent) (PanelState, InputRoute) {
	t.Helper()
	state, route, err := c.Route(event)
	if err != nil {
		t.Fatalf("Route(%#v): %v", event, err)
	}
	return state, route
}

func TestPanelKeyboardIsolationAndApplyAutoCollapse(t *testing.T) {
	c, err := NewPanelController(OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	state, route := mustRoute(t, c, PanelEvent{Kind: PanelEventKeyboard})
	if state.Open || route != (InputRoute{ForwardToDOS: true}) {
		t.Fatalf("closed keyboard state=%#v route=%#v", state, route)
	}
	state, route = mustRoute(t, c, PanelEvent{Kind: PanelEventOpen})
	if !state.Open || route != (InputRoute{ConsumedByHost: true}) {
		t.Fatalf("open state=%#v route=%#v", state, route)
	}
	state, route = mustRoute(t, c, PanelEvent{Kind: PanelEventKeyboard})
	if !state.Open || route != (InputRoute{ConsumedByHost: true}) {
		t.Fatalf("open keyboard state=%#v route=%#v", state, route)
	}
	state, route = mustRoute(t, c, PanelEvent{Kind: PanelEventSelectScale, Scale: OutputScale3})
	if state.Scales != (ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale3}) || route != (InputRoute{ConsumedByHost: true}) {
		t.Fatalf("select state=%#v route=%#v", state, route)
	}
	state, route = mustRoute(t, c, PanelEvent{Kind: PanelEventApply})
	if state.Open || state.Scales != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) || route != (InputRoute{ConsumedByHost: true}) {
		t.Fatalf("Apply state=%#v route=%#v", state, route)
	}
	state, route = mustRoute(t, c, PanelEvent{Kind: PanelEventKeyboard})
	if state.Open || route != (InputRoute{ForwardToDOS: true}) {
		t.Fatalf("post-Apply keyboard state=%#v route=%#v", state, route)
	}
}

func TestPanelHostPointerHitsNeverForwardToDOS(t *testing.T) {
	c, err := NewPanelController(OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []PanelEvent{
		{Kind: PanelEventPointerHostHit},
		{Kind: PanelEventOpen},
		{Kind: PanelEventPointerHostHit},
		{Kind: PanelEventSelectScale, Scale: OutputScale3},
		{Kind: PanelEventApply},
	} {
		_, route := mustRoute(t, c, event)
		if !route.ConsumedByHost || route.ForwardToDOS {
			t.Fatalf("host event %#v route=%#v", event, route)
		}
	}
	_, route := mustRoute(t, c, PanelEvent{Kind: PanelEventPointerMiss})
	if route.ConsumedByHost || route.ForwardToDOS {
		t.Fatalf("pointer miss must stay undecided: %#v", route)
	}
}

func TestPanelApplyCurrentScaleStillAutoCollapses(t *testing.T) {
	c, err := NewPanelController(OutputScale3)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = mustRoute(t, c, PanelEvent{Kind: PanelEventOpen})
	state, route := mustRoute(t, c, PanelEvent{Kind: PanelEventApply})
	if state.Open || state.Scales != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) || route != (InputRoute{ConsumedByHost: true}) {
		t.Fatalf("same-scale Apply state=%#v route=%#v", state, route)
	}
}

func TestPanelRejectsIllegalEventsAtomically(t *testing.T) {
	c, err := NewPanelController(OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []PanelEvent{
		{Kind: PanelEventSelectScale, Scale: OutputScale3},
		{Kind: PanelEventApply},
	} {
		before := mustPanel(t, c)
		if _, _, err := c.Route(event); err == nil {
			t.Fatalf("event %#v 必須失敗", event)
		}
		if after := mustPanel(t, c); after != before {
			t.Fatalf("event %#v changed state: before=%#v after=%#v", event, before, after)
		}
	}
	if _, _, err := c.Route(PanelEvent{Kind: PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []PanelEvent{
		{Kind: PanelEventOpen},
		{Kind: PanelEventSelectScale, Scale: 99},
		{Kind: PanelEventKind(99)},
	} {
		before := mustPanel(t, c)
		if _, _, err := c.Route(event); err == nil {
			t.Fatalf("event %#v 必須失敗", event)
		}
		if after := mustPanel(t, c); after != before {
			t.Fatalf("event %#v changed state: before=%#v after=%#v", event, before, after)
		}
	}
	var nilController *PanelController
	if _, _, err := nilController.Route(PanelEvent{Kind: PanelEventKeyboard}); err == nil {
		t.Fatal("nil controller 必須失敗")
	}
}
