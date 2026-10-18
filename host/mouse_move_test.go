package host

import (
	"reflect"
	"testing"
)

// docs/spec/451 §3.1 第 1、2 項：移動事件的五列判定與 DOS 座標換算。

func moveEvent(target MouseTarget, x, y int) MouseEvent {
	return mouseEvent(MouseEventMove, target, x, y)
}

// TestPlanMouseRouteMoveRows 直接把合法快照餵給 PlanMouseRoute，§2.1 每一列至少一例。
func TestPlanMouseRouteMoveRows(t *testing.T) {
	closed := mouseLayout(1, OutputScale2, false) // chrome 36，畫布 x 0..639、y 36..435
	open := mouseLayout(1, OutputScale2, true)
	base := MouseBridgeSnapshot{Current: closed, HasCurrent: true}
	w, top, bottom := closed.Canvas.Width*int(closed.Scale), closed.ChromeHeight, closed.ChromeHeight+closed.Canvas.Height*int(closed.Scale)
	inX, inY := 200, 100 // DOS (100, 32)
	inMove := []MouseAction{{Kind: MouseActionMove, X: 100, Y: 32}}

	pressedSame := base
	pressedSame.Pressed, pressedSame.PressedEpoch = true, closed.Epoch
	newer := mouseLayout(2, OutputScale2, false)
	pressedOld := MouseBridgeSnapshot{Current: newer, HasCurrent: true, Pressed: true, PressedEpoch: 1}
	pressedOpen := MouseBridgeSnapshot{Current: open, HasCurrent: true, Pressed: true, PressedEpoch: open.Epoch}
	captured := base
	captured.HostCaptured = true
	capturedOpen := MouseBridgeSnapshot{Current: open, HasCurrent: true, HostCaptured: true}
	panel := MouseBridgeSnapshot{Current: open, HasCurrent: true}
	button1 := moveEvent(MouseTargetCanvas, inX, inY)
	button1.Button = 1

	for _, tc := range []struct {
		name     string
		snapshot MouseBridgeSnapshot
		layout   MouseLayout
		event    MouseEvent
		route    MouseRoute
		actions  []MouseAction
	}{
		{"row1-no-current", MouseBridgeSnapshot{}, closed, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{Reason: "stale-layout-epoch-rejected"}, nil},
		{"row1-layout-differs", base, newer, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{Reason: "stale-layout-epoch-rejected"}, nil},
		{"row2-host-captured", captured, closed, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{ConsumedByHost: true, Reason: "host-captured-move-consumed"}, nil},
		{"row2-host-captured-and-panel-open", capturedOpen, open, moveEvent(MouseTargetCanvas, inX, open.ChromeHeight+10),
			MouseRoute{ConsumedByHost: true, Reason: "host-captured-move-consumed"}, nil},
		{"row3-panel-open", panel, open, moveEvent(MouseTargetCanvas, inX, open.ChromeHeight+10),
			MouseRoute{ConsumedByHost: true, Reason: "panel-open-move-consumed"}, nil},
		{"row4-target-host-inside", base, closed, moveEvent(MouseTargetHost, inX, inY),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row4-target-outside", base, closed, moveEvent(MouseTargetOutside, inX, inY),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row4-x-minus-1", base, closed, moveEvent(MouseTargetCanvas, -1, inY),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row4-x-width", base, closed, moveEvent(MouseTargetCanvas, w, inY),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row4-y-chrome-minus-1", base, closed, moveEvent(MouseTargetCanvas, inX, top-1),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row4-y-bottom", base, closed, moveEvent(MouseTargetCanvas, inX, bottom),
			MouseRoute{Reason: "outside-canvas-move-rejected"}, nil},
		{"row5-inner-edges", base, closed, moveEvent(MouseTargetCanvas, w-1, top),
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, []MouseAction{{Kind: MouseActionMove, X: 319, Y: 0}}},
		// 另兩個內緣（x = 0、y = ChromeHeight + 高 × 倍率 − 1），規格未要求，補上讓 canvasContains 四邊都有內外兩側。
		{"row5-other-inner-edges", base, closed, moveEvent(MouseTargetCanvas, 0, bottom-1),
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, []MouseAction{{Kind: MouseActionMove, X: 0, Y: 199}}},
		{"row5-inside", base, closed, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, inMove},
		{"row5-button-1", base, closed, button1,
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, inMove},
		{"row5-pressed-same-epoch", pressedSame, closed, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, inMove},
		{"row5-pressed-older-epoch", pressedOld, newer, moveEvent(MouseTargetCanvas, inX, inY),
			MouseRoute{ForwardedToDOS: true, Reason: "canvas-move-forwarded"}, inMove},
		{"row3-pressed-panel-open", pressedOpen, open, moveEvent(MouseTargetCanvas, inX, open.ChromeHeight+10),
			MouseRoute{ConsumedByHost: true, Reason: "panel-open-move-consumed"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanMouseRoute(tc.snapshot, tc.layout, tc.event)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Route != tc.route {
				t.Fatalf("route=%+v，要 %+v", plan.Route, tc.route)
			}
			if got := plan.Actions[:plan.ActionCount]; len(got) != len(tc.actions) || (len(got) != 0 && !reflect.DeepEqual(got, tc.actions)) {
				t.Fatalf("actions=%+v，要 %+v", got, tc.actions)
			}
			// 移動事件不改 Pressed、PressedEpoch、HostCaptured（§2.1），任何一列都一樣。
			if plan.Next != tc.snapshot {
				t.Fatalf("Next=%+v，要不變 %+v", plan.Next, tc.snapshot)
			}
		})
	}
}

// TestPlanMouseRouteMoveMatchesHandle 照 TestPlanMouseRouteMatchesHandleMatrix 的做法，
// 以事件序列與 ApplyLayout 把 MouseBridge 帶到各可達狀態，比較 Handle 與 PlanMouseRoute。
func TestPlanMouseRouteMoveMatchesHandle(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		t.Run("scale", func(t *testing.T) {
			out := &planMouseOutput{}
			bridge := mustMouseBridge(t, out)
			current := mouseLayout(1, scale, false)
			mustApplyMouse(t, bridge, current)
			snapshot := bridge.Snapshot()
			s := int(scale)
			run := func(name string, event MouseEvent, reason string) {
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
				if got.Reason != reason {
					t.Fatalf("%s reason=%q，要 %q", name, got.Reason, reason)
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
			at := func(x, y int) (int, int) { return x * s, current.ChromeHeight + y*s }

			// 無按鍵。
			x, y := at(100, 82)
			run("idle-move", moveEvent(MouseTargetCanvas, x, y), "canvas-move-forwarded")
			run("idle-move-outside-frame", moveEvent(MouseTargetCanvas, current.FrameWidth, y), "outside-canvas-move-rejected")
			run("idle-move-target-outside", moveEvent(MouseTargetOutside, x, y), "outside-canvas-move-rejected")
			run("idle-move-target-host", moveEvent(MouseTargetHost, x, y), "outside-canvas-move-rejected")
			right := moveEvent(MouseTargetCanvas, x+s, y)
			right.Button = 1
			run("idle-move-button-1", right, "canvas-move-forwarded")

			// Pressed（同 epoch）：拖曳。
			run("press", mouseEvent(MouseEventDown, MouseTargetCanvas, x, y), "canvas-down-forwarded")
			x, y = at(150, 90)
			run("drag-move", moveEvent(MouseTargetCanvas, x, y), "canvas-move-forwarded")
			run("drag-move-outside", moveEvent(MouseTargetCanvas, -1, y), "outside-canvas-move-rejected")

			// Pressed 後換 layout（不同 epoch），再開面板。
			apply(mouseLayout(2, scale, false))
			x, y = at(160, 75)
			run("pressed-old-epoch-move", moveEvent(MouseTargetCanvas, x, y), "canvas-move-forwarded")
			apply(mouseLayout(3, scale, true))
			x, y = at(160, 75)
			run("pressed-panel-open-move", moveEvent(MouseTargetCanvas, x, y), "panel-open-move-consumed")
			run("pressed-epoch-release", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y), "epoch-changed-release")
			run("panel-open-move", moveEvent(MouseTargetCanvas, x, y), "panel-open-move-consumed")

			// HostCaptured，之後開面板。
			apply(mouseLayout(4, scale, false))
			run("host-down", mouseEvent(MouseEventDown, MouseTargetHost, 10, 5), "host-down-consumed")
			x, y = at(20, 30)
			run("captured-move", moveEvent(MouseTargetCanvas, x, y), "host-captured-move-consumed")
			apply(mouseLayout(5, scale, true))
			x, y = at(20, 30)
			run("captured-panel-open-move", moveEvent(MouseTargetCanvas, x, y), "host-captured-move-consumed")
			run("captured-up", mouseEvent(MouseEventUp, MouseTargetCanvas, x, y), "host-captured-up-consumed")
			run("after-capture-panel-move", moveEvent(MouseTargetCanvas, x, y), "panel-open-move-consumed")

			// 過期 layout。
			stale := mouseLayout(4, scale, false)
			before := len(out.actions)
			plan, err := PlanMouseRoute(snapshot, stale, moveEvent(MouseTargetCanvas, x, y))
			actual := bridge.Handle(stale, moveEvent(MouseTargetCanvas, x, y))
			if err != nil || plan.Route != actual || actual.Reason != "stale-layout-epoch-rejected" || plan.Next != bridge.Snapshot() || plan.ActionCount != 0 || len(out.actions) != before {
				t.Fatalf("stale move plan=%+v actual=%+v err=%v", plan, actual, err)
			}
		})
	}
}

// TestMouseMoveDOSCoordinates 驗 §3.1 第 2 項的換算：先減 chrome 再除倍率。
func TestMouseMoveDOSCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		layout MouseLayout
		x, y   int
		want   MouseAction
	}{
		{"2x-chrome-24", MouseLayout{Epoch: 1, Scale: OutputScale2, ChromeHeight: 24, Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 640, FrameHeight: 24 + 400},
			321, 175, MouseAction{Kind: MouseActionMove, X: 160, Y: 75}},
		// 先除再減得 (0, 1)，漏減 chrome 得 (0, 9)。
		{"3x-chrome-25", MouseLayout{Epoch: 1, Scale: OutputScale3, ChromeHeight: 25, Canvas: Canvas{Width: 320, Height: 200}, FrameWidth: 960, FrameHeight: 25 + 600},
			2, 27, MouseAction{Kind: MouseActionMove, X: 0, Y: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.layout.valid() {
				t.Fatalf("layout 無效：%+v", tc.layout)
			}
			event := moveEvent(MouseTargetCanvas, tc.x, tc.y)
			snapshot := MouseBridgeSnapshot{Current: tc.layout, HasCurrent: true}
			plan, err := PlanMouseRoute(snapshot, tc.layout, event)
			if err != nil {
				t.Fatal(err)
			}
			if plan.ActionCount != 1 || plan.Actions[0] != tc.want {
				t.Fatalf("plan actions=%+v，要 %+v", plan.Actions[:plan.ActionCount], tc.want)
			}
			out := &planMouseOutput{}
			bridge := mustMouseBridge(t, out)
			mustApplyMouse(t, bridge, tc.layout)
			route := bridge.Handle(tc.layout, event)
			if route.Reason != "canvas-move-forwarded" || len(out.actions) != 1 || out.actions[0] != tc.want {
				t.Fatalf("Handle route=%+v actions=%+v，要 %+v", route, out.actions, tc.want)
			}
		})
	}
}
