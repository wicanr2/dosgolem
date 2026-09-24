package host

import (
	"reflect"
	"testing"
)

type testMouseOutput struct {
	x, y    int
	pressed bool
	calls   []string
}

func (m *testMouseOutput) MoveMouse(x, y int)    { m.x, m.y = x, y; m.calls = append(m.calls, "move") }
func (m *testMouseOutput) PressMouse(button int) { m.pressed = true; m.calls = append(m.calls, "down") }
func (m *testMouseOutput) ReleaseMouse(button int) {
	m.pressed = false
	m.calls = append(m.calls, "up")
}

func mouseLayout(epoch uint64, scale OutputScale, open bool) MouseLayout {
	return MouseLayout{Epoch: epoch, Scale: scale, ChromeHeight: 18 * int(scale), Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 320 * int(scale), FrameHeight: 218 * int(scale), PanelOpen: open}
}

func mouseEvent(kind MouseEventKind, target MouseTarget, x, y int) MouseEvent {
	return MouseEvent{Kind: kind, Button: 0, Target: target, X: x, Y: y}
}

func mustMouseBridge(t *testing.T, output MouseOutput) *MouseBridge {
	t.Helper()
	b, err := NewMouseBridge(output)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustApplyMouse(t *testing.T, b *MouseBridge, layout MouseLayout) {
	t.Helper()
	if err := b.ApplyLayout(layout); err != nil {
		t.Fatal(err)
	}
}

func wantMouseRoute(t *testing.T, got MouseRoute, host, dos, cleanup bool, reason string) {
	t.Helper()
	if got.ConsumedByHost != host || got.ForwardedToDOS != dos || got.Cleanup != cleanup || got.Reason != reason {
		t.Fatalf("route=%+v want host=%t dos=%t cleanup=%t reason=%q", got, host, dos, cleanup, reason)
	}
}

func TestMouseBridgeCornersAndCanvasUpAtBothScales(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		layout := mouseLayout(1, scale, false)
		for _, point := range [][2]int{{0, layout.ChromeHeight}, {layout.FrameWidth - 1, layout.ChromeHeight}, {0, layout.FrameHeight - 1}, {layout.FrameWidth - 1, layout.FrameHeight - 1}} {
			output := &testMouseOutput{}
			bridge := mustMouseBridge(t, output)
			mustApplyMouse(t, bridge, layout)
			wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, point[0], point[1])), false, true, false, "canvas-down-forwarded")
			wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventUp, MouseTargetCanvas, point[0], point[1])), false, true, true, "up-release")
			wantX, wantY := point[0]/int(scale), (point[1]-layout.ChromeHeight)/int(scale)
			if output.x != wantX || output.y != wantY || output.pressed || len(output.calls) != 4 {
				t.Fatalf("%d× point=%v output=%+v", scale, point, output)
			}
		}
	}
}

func TestMouseBridgeRejectsBoundariesAndNonCanvasTargets(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		layout := mouseLayout(1, scale, false)
		for _, point := range [][2]int{{320 * int(scale), layout.ChromeHeight}, {0, layout.ChromeHeight + 200*int(scale)}, {-1, layout.ChromeHeight}, {0, -1}} {
			output := &testMouseOutput{}
			bridge := mustMouseBridge(t, output)
			mustApplyMouse(t, bridge, layout)
			wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, point[0], point[1])), false, false, false, "outside-canvas-rejected")
			if len(output.calls) != 0 {
				t.Fatalf("%d× boundary %v touched DOS", scale, point)
			}
		}
		for _, target := range []MouseTarget{MouseTargetHost, MouseTargetOutside, MouseTarget(99)} {
			output := &testMouseOutput{}
			bridge := mustMouseBridge(t, output)
			mustApplyMouse(t, bridge, layout)
			x, y := 100*int(scale), layout.ChromeHeight+82*int(scale)
			wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, x, y)), false, true, false, "canvas-down-forwarded")
			oldX, oldY := output.x, output.y
			wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventUp, target, x, y)), false, true, true, "non-canvas-target-release")
			if output.x != oldX || output.y != oldY || output.pressed || len(output.calls) != 3 || output.calls[2] != "up" {
				t.Fatalf("%d× target=%d remapped DOS: %+v", scale, target, output)
			}
		}
	}
}

func TestMouseBridgeCleanupAcrossLayoutAndHostCapture(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		layout := mouseLayout(1, scale, false)
		for _, tc := range []struct {
			name   string
			event  MouseEvent
			next   MouseLayout
			reason string
		}{
			{"outside", mouseEvent(MouseEventUp, MouseTargetOutside, layout.FrameWidth, layout.ChromeHeight), layout, "non-canvas-target-release"},
			{"panel", mouseEvent(MouseEventUp, MouseTargetCanvas, 100*int(scale), layout.ChromeHeight+82*int(scale)), mouseLayout(2, scale, true), "epoch-changed-release"},
			{"focus", MouseEvent{Kind: MouseEventFocusLost}, mouseLayout(2, scale, true), "focus-lost-release"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				output := &testMouseOutput{}
				bridge := mustMouseBridge(t, output)
				mustApplyMouse(t, bridge, layout)
				x, y := 100*int(scale), layout.ChromeHeight+82*int(scale)
				wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, x, y)), false, true, false, "canvas-down-forwarded")
				oldX, oldY := output.x, output.y
				if tc.next.Epoch != layout.Epoch {
					mustApplyMouse(t, bridge, tc.next)
				}
				wantMouseRoute(t, bridge.Handle(tc.next, tc.event), false, true, true, tc.reason)
				if output.x != oldX || output.y != oldY || output.pressed || len(output.calls) != 3 || output.calls[2] != "up" {
					t.Fatalf("%d× %s leaked Move: %+v", scale, tc.name, output)
				}
			})
		}
		output := &testMouseOutput{}
		bridge := mustMouseBridge(t, output)
		mustApplyMouse(t, bridge, layout)
		wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetHost, 10, 5)), true, false, false, "host-down-consumed")
		wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventUp, MouseTargetCanvas, 100*int(scale), layout.ChromeHeight+82*int(scale))), true, false, false, "host-captured-up-consumed")
		if len(output.calls) != 0 || bridge.HostCaptured() {
			t.Fatalf("%d× host capture leaked: %+v", scale, output)
		}
	}
}

func TestMouseBridgePanelAndFailuresFailClosed(t *testing.T) {
	output := &testMouseOutput{}
	bridge := mustMouseBridge(t, output)
	layout := mouseLayout(2, OutputScale2, true)
	mustApplyMouse(t, bridge, layout)
	wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, 200, 200)), true, false, false, "panel-open-consumed")
	wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventUp, MouseTargetCanvas, 200, 200)), true, false, false, "host-captured-up-consumed")
	if len(output.calls) != 0 {
		t.Fatal("panel input reached DOS")
	}

	closed := mouseLayout(3, OutputScale2, false)
	mustApplyMouse(t, bridge, closed)
	wantMouseRoute(t, bridge.Handle(layout, mouseEvent(MouseEventDown, MouseTargetCanvas, 200, 200)), false, false, false, "stale-layout-epoch-rejected")
	wantMouseRoute(t, bridge.Handle(closed, MouseEvent{Kind: MouseEventDown, Button: 1, Target: MouseTargetCanvas, X: 200, Y: 200}), false, false, false, "non-left-rejected")
	wantMouseRoute(t, bridge.Handle(closed, MouseEvent{Kind: MouseEventKind(99), Button: 0}), false, false, false, "unknown-event-rejected")
	if err := bridge.ApplyLayout(MouseLayout{Epoch: 4, Scale: OutputScale2, Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 639, FrameHeight: 436}); err == nil {
		t.Fatal("invalid layout accepted")
	}
	maxInt := int(^uint(0) >> 1)
	if err := bridge.ApplyLayout(MouseLayout{Epoch: 4, Scale: OutputScale2, Canvas: Canvas{Width: maxInt, Height: 1}, FrameWidth: maxInt, FrameHeight: 436}); err == nil {
		t.Fatal("overflowing width accepted")
	}
	if err := bridge.ApplyLayout(MouseLayout{Epoch: 4, Scale: OutputScale2, ChromeHeight: 2, Canvas: Canvas{Width: 1, Height: maxInt / 2}, FrameWidth: 2, FrameHeight: maxInt}); err == nil {
		t.Fatal("overflowing chrome plus height accepted")
	}
	if got, ok := bridge.Layout(); !ok || got != closed {
		t.Fatalf("invalid layout replaced current: %+v %t", got, ok)
	}
	if len(output.calls) != 0 {
		t.Fatalf("failure path touched DOS: %+v", output)
	}

	if _, err := NewMouseBridge(nil); err == nil {
		t.Fatal("nil output accepted")
	}
}

func TestMouseBridgeHostCaptureBlocksSecondCanvasDownAcrossEpoch(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		output := &testMouseOutput{}
		bridge := mustMouseBridge(t, output)
		first := mouseLayout(1, scale, false)
		mustApplyMouse(t, bridge, first)
		wantMouseRoute(t, bridge.Handle(first, mouseEvent(MouseEventDown, MouseTargetHost, 10, 5)), true, false, false, "host-down-consumed")
		canvasDown := mouseEvent(MouseEventDown, MouseTargetCanvas, 100*int(scale), first.ChromeHeight+82*int(scale))
		wantMouseRoute(t, bridge.Handle(first, canvasDown), true, false, false, "host-captured-down-consumed")
		second := mouseLayout(2, scale, false)
		mustApplyMouse(t, bridge, second)
		canvasDown.Y = second.ChromeHeight + 82*int(scale)
		wantMouseRoute(t, bridge.Handle(second, canvasDown), true, false, false, "host-captured-down-consumed")
		wantMouseRoute(t, bridge.Handle(second, mouseEvent(MouseEventUp, MouseTargetCanvas, canvasDown.X, canvasDown.Y)), true, false, false, "host-captured-up-consumed")
		if len(output.calls) != 0 || bridge.HostCaptured() || bridge.Pressed() {
			t.Fatalf("%d× host capture leaked across epoch: %+v", scale, output)
		}
	}
}

func TestMouseBridgeSnapshotIncludesPressedEpochAndDoesNotRoute(t *testing.T) {
	output := &testMouseOutput{}
	bridge := mustMouseBridge(t, output)
	if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{}) {
		t.Fatalf("new bridge snapshot=%+v", got)
	}
	first := mouseLayout(1, OutputScale2, false)
	mustApplyMouse(t, bridge, first)
	if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{Current: first, HasCurrent: true}) {
		t.Fatalf("layout snapshot=%+v", got)
	}
	before := len(output.calls)
	copy := bridge.Snapshot()
	copy.Current.Canvas.Width = 1
	copy.Current.Epoch = 99
	copy.Pressed = true
	copy.PressedEpoch = 99
	copy.HostCaptured = true
	if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{Current: first, HasCurrent: true}) || len(output.calls) != before {
		t.Fatalf("mutated snapshot changed bridge or output: %+v calls=%v", got, output.calls)
	}
	wantMouseRoute(t, bridge.Handle(first, mouseEvent(MouseEventDown, MouseTargetCanvas, 200, 100)), false, true, false, "canvas-down-forwarded")
	pressed := MouseBridgeSnapshot{Current: first, HasCurrent: true, Pressed: true, PressedEpoch: 1}
	if got := bridge.Snapshot(); got != pressed || len(output.calls) != before+2 {
		t.Fatalf("accepted Down snapshot=%+v calls=%v", got, output.calls)
	}
	second := mouseLayout(2, OutputScale2, true)
	mustApplyMouse(t, bridge, second)
	pressed.Current = second
	if got := bridge.Snapshot(); got != pressed || len(output.calls) != before+2 {
		t.Fatalf("cross-epoch snapshot=%+v calls=%v", got, output.calls)
	}
	wantMouseRoute(t, bridge.Handle(second, mouseEvent(MouseEventUp, MouseTargetCanvas, 200, 100)), false, true, true, "epoch-changed-release")
	if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{Current: second, HasCurrent: true}) ||
		!reflect.DeepEqual(output.calls, []string{"move", "down", "up"}) {
		t.Fatalf("cross-epoch release snapshot=%+v calls=%v", got, output.calls)
	}
	// A fresh press followed by focus loss clears the press epoch without
	// relying on a current-layout Up, and Snapshot itself adds no output.
	closed := mouseLayout(3, OutputScale2, false)
	mustApplyMouse(t, bridge, closed)
	wantMouseRoute(t, bridge.Handle(closed, mouseEvent(MouseEventDown, MouseTargetCanvas, 200, 100)), false, true, false, "canvas-down-forwarded")
	if got := bridge.Snapshot(); !got.Pressed || got.PressedEpoch != 3 {
		t.Fatalf("new press epoch=%+v", got)
	}
	wantMouseRoute(t, bridge.Handle(closed, MouseEvent{Kind: MouseEventFocusLost}), false, true, true, "focus-lost-release")
	if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{Current: closed, HasCurrent: true}) ||
		!reflect.DeepEqual(output.calls, []string{"move", "down", "up", "move", "down", "up"}) {
		t.Fatalf("focus release snapshot=%+v calls=%v", got, output.calls)
	}
}

func TestMouseBridgeSnapshotHostCaptureAndNil(t *testing.T) {
	var nilBridge *MouseBridge
	if got := nilBridge.Snapshot(); got != (MouseBridgeSnapshot{}) {
		t.Fatalf("nil bridge snapshot=%+v", got)
	}
	for _, cleanup := range []MouseEventKind{MouseEventUp, MouseEventFocusLost} {
		output := &testMouseOutput{}
		bridge := mustMouseBridge(t, output)
		first := mouseLayout(1, OutputScale3, false)
		second := mouseLayout(2, OutputScale3, true)
		mustApplyMouse(t, bridge, first)
		wantMouseRoute(t, bridge.Handle(first, mouseEvent(MouseEventDown, MouseTargetHost, 10, 5)), true, false, false, "host-down-consumed")
		want := MouseBridgeSnapshot{Current: first, HasCurrent: true, HostCaptured: true}
		if got := bridge.Snapshot(); got != want || len(output.calls) != 0 {
			t.Fatalf("host capture snapshot=%+v calls=%v", got, output.calls)
		}
		mustApplyMouse(t, bridge, second)
		want.Current = second
		if got := bridge.Snapshot(); got != want {
			t.Fatalf("host capture across epoch snapshot=%+v", got)
		}
		event := mouseEvent(cleanup, MouseTargetCanvas, 300, 150)
		if cleanup == MouseEventUp {
			wantMouseRoute(t, bridge.Handle(second, event), true, false, false, "host-captured-up-consumed")
		} else {
			wantMouseRoute(t, bridge.Handle(second, event), false, false, false, "unmatched-up-rejected")
		}
		if got := bridge.Snapshot(); got != (MouseBridgeSnapshot{Current: second, HasCurrent: true}) || len(output.calls) != 0 {
			t.Fatalf("host cleanup kind=%d snapshot=%+v calls=%v", cleanup, got, output.calls)
		}
	}
}

func TestMouseBridgeSnapshotContainsOnlyValueFields(t *testing.T) {
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
		case reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Map,
			reflect.Slice, reflect.Chan, reflect.Func:
			t.Fatalf("snapshot leaks reference capability: %s", typ)
		}
	}
	check(reflect.TypeOf(MouseBridgeSnapshot{}))
}
