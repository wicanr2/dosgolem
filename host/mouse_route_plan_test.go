package host

import (
	"reflect"
	"testing"
)

type planMouseOutput struct{ actions []MouseAction }

func (o *planMouseOutput) MoveMouse(x, y int) {
	o.actions = append(o.actions, MouseAction{Kind: MouseActionMove, X: x, Y: y})
}
func (o *planMouseOutput) PressMouse(button int) {
	o.actions = append(o.actions, MouseAction{Kind: MouseActionPress, Button: button})
}
func (o *planMouseOutput) ReleaseMouse(button int) {
	o.actions = append(o.actions, MouseAction{Kind: MouseActionRelease, Button: button})
}

func TestPlanMouseRouteMatchesHandleMatrix(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		t.Run("scale", func(t *testing.T) {
			out := &planMouseOutput{}
			bridge := mustMouseBridge(t, out)
			first := mouseLayout(1, scale, false)
			mustApplyMouse(t, bridge, first)
			snapshot := bridge.Snapshot()
			current := first
			run := func(name string, event MouseEvent) {
				t.Helper()
				before := len(out.actions)
				plan, err := PlanMouseRoute(snapshot, current, event)
				if err != nil {
					t.Fatalf("%s plan: %v", name, err)
				}
				got := bridge.Handle(current, event)
				if plan.Route != got || plan.Next != bridge.Snapshot() || !reflect.DeepEqual(plan.Actions[:plan.ActionCount], out.actions[before:]) {
					t.Fatalf("%s plan=%+v actual route=%+v snapshot=%+v actions=%+v", name, plan, got, bridge.Snapshot(), out.actions[before:])
				}
				snapshot = plan.Next
			}
			apply := func(next MouseLayout) {
				t.Helper()
				mustApplyMouse(t, bridge, next)
				snapshot.Current, snapshot.HasCurrent = next, true
				current = next
				if snapshot != bridge.Snapshot() {
					t.Fatalf("layout snapshot drift: plan=%+v actual=%+v", snapshot, bridge.Snapshot())
				}
			}

			x, y := 100*int(scale), first.ChromeHeight+82*int(scale)
			run("canvas-down", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			run("duplicate-down", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			run("canvas-up", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y))
			run("unmatched-up", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y))
			run("outside-down", mouseEvent(MouseEventDown, MouseTargetCanvas, current.FrameWidth, y))
			run("unmapped-target", mouseEvent(MouseEventDown, MouseTargetOutside, x, y))
			run("non-left", MouseEvent{Kind: MouseEventDown, Button: 1, Target: MouseTargetCanvas, X: x, Y: y})
			run("unknown", MouseEvent{Kind: MouseEventKind(99), Button: 0})

			run("host-down", mouseEvent(MouseEventDown, MouseTargetHost, 10, 5))
			run("host-captured-down", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			run("host-captured-up", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y))

			apply(mouseLayout(2, scale, false))
			x, y = 100*int(scale), current.ChromeHeight+82*int(scale)
			run("press-before-epoch-change", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			apply(mouseLayout(3, scale, true))
			run("epoch-changed-release", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y))
			run("panel-down", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			run("panel-up", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y))

			apply(mouseLayout(4, scale, false))
			x, y = 100*int(scale), current.ChromeHeight+82*int(scale)
			run("press-before-focus", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			stale := mouseLayout(3, scale, false)
			before := len(out.actions)
			plan, err := PlanMouseRoute(snapshot, stale, mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			actual := bridge.Handle(stale, mouseEvent(MouseEventDown, MouseTargetCanvas, x, y))
			if err != nil || plan.Route != actual || plan.Next != bridge.Snapshot() || plan.ActionCount != 0 || len(out.actions) != before {
				t.Fatalf("stale plan=%+v err=%v", plan, err)
			}
			run("focus-release", MouseEvent{Kind: MouseEventFocusLost})
		})
	}
}

func TestPlanMouseRouteMatchesHandleCornersAtBothScales(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		layout := mouseLayout(1, scale, false)
		for _, point := range [][2]int{{0, layout.ChromeHeight}, {layout.FrameWidth - 1, layout.ChromeHeight}, {0, layout.FrameHeight - 1}, {layout.FrameWidth - 1, layout.FrameHeight - 1}} {
			out := &planMouseOutput{}
			bridge := mustMouseBridge(t, out)
			mustApplyMouse(t, bridge, layout)
			snapshot := bridge.Snapshot()
			for _, event := range []MouseEvent{
				mouseEvent(MouseEventDown, MouseTargetCanvas, point[0], point[1]),
				mouseEvent(MouseEventUp, MouseTargetCanvas, point[0], point[1]),
			} {
				before := len(out.actions)
				plan, err := PlanMouseRoute(snapshot, layout, event)
				if err != nil {
					t.Fatal(err)
				}
				actual := bridge.Handle(layout, event)
				if plan.Route != actual || plan.Next != bridge.Snapshot() || !reflect.DeepEqual(plan.Actions[:plan.ActionCount], out.actions[before:]) {
					t.Fatalf("%d× point=%v event=%+v plan=%+v actual=%+v output=%+v", scale, point, event, plan, actual, out.actions[before:])
				}
				snapshot = plan.Next
			}
		}
	}
}

func TestPlanMouseRouteRejectsImpossibleSnapshotsWithoutActions(t *testing.T) {
	layout := mouseLayout(1, OutputScale2, false)
	for _, snapshot := range []MouseBridgeSnapshot{
		{Pressed: true, PressedEpoch: 1},
		{Current: layout, HasCurrent: true, Pressed: true},
		{Current: layout, HasCurrent: true, PressedEpoch: 1},
		{Current: layout, HasCurrent: true, Pressed: true, PressedEpoch: 1, HostCaptured: true},
		{Current: MouseLayout{Epoch: 1}, HasCurrent: true},
	} {
		if plan, err := PlanMouseRoute(snapshot, layout, mouseEvent(MouseEventDown, MouseTargetCanvas, 100, 100)); err == nil || plan != (MouseRoutePlan{}) {
			t.Fatalf("invalid snapshot accepted: plan=%+v err=%v", plan, err)
		}
	}
}

func TestMouseRoutePlanContainsOnlyValues(t *testing.T) {
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		t.Helper()
		switch typ.Kind() {
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				check(typ.Field(i).Type)
			}
		case reflect.Array:
			check(typ.Elem())
		case reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
			t.Fatalf("route plan leaks reference capability: %s", typ)
		}
	}
	check(reflect.TypeOf(MouseAction{}))
	check(reflect.TypeOf(MouseRoutePlan{}))
}
