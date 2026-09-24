package host

import "testing"

func TestPlanPanelRouteMatchesController(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		c, err := NewPanelController(scale)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range []PanelEvent{
			{Kind: PanelEventKeyboard},
			{Kind: PanelEventPointerHostHit},
			{Kind: PanelEventPointerMiss},
			{Kind: PanelEventOpen},
			{Kind: PanelEventKeyboard},
			{Kind: PanelEventSelectScale, Scale: OutputScale3},
			{Kind: PanelEventApply},
			{Kind: PanelEventKeyboard},
			{Kind: PanelEventOpen},
			{Kind: PanelEventSelectScale, Scale: OutputScale2},
			{Kind: PanelEventCancel},
			{Kind: PanelEventKeyboard},
		} {
			before, err := c.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			planned, route, err := PlanPanelRoute(before, event)
			if err != nil || before != mustPanelSnapshot(t, c) {
				t.Fatalf("scale=%d event=%+v changed controller during plan: planned=%+v err=%v", scale, event, planned, err)
			}
			got, gotRoute, gotErr := c.Route(event)
			if gotErr != nil || planned != got || route != gotRoute {
				t.Fatalf("scale=%d event=%+v plan=%+v route=%+v err=%v actual=%+v route=%+v err=%v", scale, event, planned, route, err, got, gotRoute, gotErr)
			}
		}
	}
}

func TestPlanPanelRouteRejectsInvalidSequenceWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		open  bool
		event PanelEvent
	}{
		{false, PanelEvent{Kind: PanelEventApply}},
		{false, PanelEvent{Kind: PanelEventCancel}},
		{false, PanelEvent{Kind: PanelEventSelectScale, Scale: OutputScale3}},
		{false, PanelEvent{Kind: PanelEventKind(255)}},
		{true, PanelEvent{Kind: PanelEventOpen}},
		{true, PanelEvent{Kind: PanelEventSelectScale, Scale: OutputScale(4)}},
	} {
		c, err := NewPanelController(OutputScale2)
		if err != nil {
			t.Fatal(err)
		}
		if tc.open {
			if _, _, err := c.Route(PanelEvent{Kind: PanelEventOpen}); err != nil {
				t.Fatal(err)
			}
		}
		before := mustPanelSnapshot(t, c)
		planned, route, planErr := PlanPanelRoute(before, tc.event)
		actual, actualRoute, actualErr := c.Route(tc.event)
		if planErr == nil || actualErr == nil || planned != (PanelState{}) || route != (InputRoute{}) ||
			actual != (PanelState{}) || actualRoute != (InputRoute{}) || mustPanelSnapshot(t, c) != before {
			t.Fatalf("event=%+v open=%v plan=%+v route=%+v err=%v actual=%+v route=%+v err=%v", tc.event, tc.open, planned, route, planErr, actual, actualRoute, actualErr)
		}
	}
}

func TestPlanPanelRouteExhaustiveValidStates(t *testing.T) {
	for _, active := range []OutputScale{OutputScale2, OutputScale3} {
		for _, selected := range []OutputScale{OutputScale2, OutputScale3} {
			for _, open := range []bool{false, true} {
				for _, event := range []PanelEvent{
					{Kind: PanelEventOpen},
					{Kind: PanelEventSelectScale, Scale: OutputScale2},
					{Kind: PanelEventSelectScale, Scale: OutputScale3},
					{Kind: PanelEventApply},
					{Kind: PanelEventCancel},
					{Kind: PanelEventKeyboard},
					{Kind: PanelEventPointerHostHit},
					{Kind: PanelEventPointerMiss},
				} {
					c, err := NewPanelController(active)
					if err != nil {
						t.Fatal(err)
					}
					c.scales.state.SelectedScale = selected
					c.open = open
					before := mustPanelSnapshot(t, c)
					planned, route, planErr := PlanPanelRoute(before, event)
					actual, actualRoute, actualErr := c.Route(event)
					if (planErr == nil) != (actualErr == nil) || planned != actual || route != actualRoute ||
						(planErr != nil && mustPanelSnapshot(t, c) != before) {
						t.Fatalf("active=%d selected=%d open=%v event=%+v plan=%+v/%+v/%v actual=%+v/%+v/%v",
							active, selected, open, event, planned, route, planErr, actual, actualRoute, actualErr)
					}
				}
			}
		}
	}
}

func TestPlanPanelRouteRejectsInvalidSnapshot(t *testing.T) {
	for _, bad := range []PanelState{
		{},
		{Scales: ScaleState{ActiveScale: OutputScale2, SelectedScale: 4}},
		{Scales: ScaleState{ActiveScale: 4, SelectedScale: OutputScale2}},
	} {
		if next, route, err := PlanPanelRoute(bad, PanelEvent{Kind: PanelEventKeyboard}); err == nil ||
			next != (PanelState{}) || route != (InputRoute{}) {
			t.Fatalf("invalid state accepted: state=%+v next=%+v route=%+v err=%v", bad, next, route, err)
		}
	}
}

func mustPanelSnapshot(t *testing.T, c *PanelController) PanelState {
	t.Helper()
	state, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
