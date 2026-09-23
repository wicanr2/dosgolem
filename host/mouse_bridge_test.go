package host

import "testing"

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
