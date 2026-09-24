package host

import "fmt"

// MouseActionKind is one already-classified MouseOutput call. It is a value
// only: a session owner may validate a complete plan before it commits any
// action to its private DOS adapter.
type MouseActionKind uint8

const (
	MouseActionMove MouseActionKind = iota + 1
	MouseActionPress
	MouseActionRelease
)

// MouseAction has the exact arguments that MouseBridge.Handle would send to
// MouseOutput. Move uses DOS logical coordinates; press and release use the
// existing host button number (currently left button 0).
type MouseAction struct {
	Kind   MouseActionKind
	X, Y   int
	Button int
}

// MouseRoutePlan is the pure result of one MouseBridge.Handle-equivalent
// routing decision. Neither it nor PlanMouseRoute owns an output capability.
type MouseRoutePlan struct {
	Next        MouseBridgeSnapshot
	Route       MouseRoute
	Actions     [2]MouseAction
	ActionCount uint8
}

func (p *MouseRoutePlan) add(action MouseAction) {
	if p.ActionCount >= uint8(len(p.Actions)) {
		panic("host: one MouseBridge event cannot produce more than two actions")
	}
	p.Actions[p.ActionCount] = action
	p.ActionCount++
}

// PlanMouseRoute predicts one Handle call from a complete value snapshot.
// It deliberately preserves every existing MouseRoute reason, including
// accepted host no-ops and normal rejected routes. Invalid snapshots are not
// normal route results: they are rejected before any action can be planned.
func PlanMouseRoute(snapshot MouseBridgeSnapshot, layout MouseLayout, event MouseEvent) (MouseRoutePlan, error) {
	if err := validateMouseRouteSnapshot(snapshot); err != nil {
		return MouseRoutePlan{}, err
	}
	plan := MouseRoutePlan{Next: snapshot}
	release := func(reason string) {
		if !plan.Next.Pressed {
			plan.Route = MouseRoute{Reason: "unmatched-up-rejected"}
			return
		}
		plan.add(MouseAction{Kind: MouseActionRelease, Button: 0})
		plan.Next.Pressed, plan.Next.PressedEpoch = false, 0
		plan.Route = MouseRoute{ForwardedToDOS: true, Cleanup: true, Reason: reason}
	}

	// Handle intentionally permits focus cleanup even after a new layout made
	// the caller's event layout stale.
	if event.Kind == MouseEventFocusLost {
		plan.Next.HostCaptured = false
		release("focus-lost-release")
		return plan, nil
	}
	if !snapshot.HasCurrent || layout != snapshot.Current {
		plan.Route = MouseRoute{Reason: "stale-layout-epoch-rejected"}
		return plan, nil
	}
	if event.Kind != MouseEventDown && event.Kind != MouseEventUp {
		plan.Route = MouseRoute{Reason: "unknown-event-rejected"}
		return plan, nil
	}
	if event.Button != 0 {
		if layout.PanelOpen {
			plan.Route = MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
		} else {
			plan.Route = MouseRoute{Reason: "non-left-rejected"}
		}
		return plan, nil
	}
	if event.Kind == MouseEventUp {
		if plan.Next.Pressed {
			if plan.Next.PressedEpoch != layout.Epoch {
				release("epoch-changed-release")
				return plan, nil
			}
			if event.Target != MouseTargetCanvas || layout.PanelOpen || !layout.canvasContains(event.X, event.Y) {
				release("non-canvas-target-release")
				return plan, nil
			}
			plan.add(MouseAction{Kind: MouseActionMove,
				X: event.X / int(layout.Scale), Y: (event.Y - layout.ChromeHeight) / int(layout.Scale)})
			release("up-release")
			return plan, nil
		}
		if plan.Next.HostCaptured {
			plan.Next.HostCaptured = false
			plan.Route = MouseRoute{ConsumedByHost: true, Reason: "host-captured-up-consumed"}
		} else if layout.PanelOpen {
			plan.Route = MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
		} else {
			plan.Route = MouseRoute{Reason: "unmatched-up-rejected"}
		}
		return plan, nil
	}

	if plan.Next.Pressed {
		plan.Route = MouseRoute{Reason: "duplicate-down-rejected"}
		return plan, nil
	}
	if plan.Next.HostCaptured {
		plan.Route = MouseRoute{ConsumedByHost: true, Reason: "host-captured-down-consumed"}
		return plan, nil
	}
	if layout.PanelOpen {
		plan.Next.HostCaptured = true
		plan.Route = MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
		return plan, nil
	}
	if event.Target == MouseTargetHost {
		plan.Next.HostCaptured = true
		plan.Route = MouseRoute{ConsumedByHost: true, Reason: "host-down-consumed"}
		return plan, nil
	}
	if event.Target != MouseTargetCanvas || !layout.canvasContains(event.X, event.Y) {
		plan.Route = MouseRoute{Reason: "outside-canvas-rejected"}
		return plan, nil
	}
	plan.add(MouseAction{Kind: MouseActionMove, X: event.X / int(layout.Scale), Y: (event.Y - layout.ChromeHeight) / int(layout.Scale)})
	plan.add(MouseAction{Kind: MouseActionPress, Button: 0})
	plan.Next.Pressed, plan.Next.PressedEpoch = true, layout.Epoch
	plan.Route = MouseRoute{ForwardedToDOS: true, Reason: "canvas-down-forwarded"}
	return plan, nil
}

func validateMouseRouteSnapshot(snapshot MouseBridgeSnapshot) error {
	if snapshot.HasCurrent && !snapshot.Current.valid() {
		return fmt.Errorf("host: MouseBridgeSnapshot 的 current layout 無效")
	}
	if !snapshot.HasCurrent && (snapshot.Pressed || snapshot.PressedEpoch != 0 || snapshot.HostCaptured) {
		return fmt.Errorf("host: 無 current layout 的 MouseBridgeSnapshot 狀態不一致")
	}
	if snapshot.Pressed && snapshot.PressedEpoch == 0 {
		return fmt.Errorf("host: 已按下的 MouseBridgeSnapshot 缺少 pressed epoch")
	}
	if !snapshot.Pressed && snapshot.PressedEpoch != 0 {
		return fmt.Errorf("host: 未按下的 MouseBridgeSnapshot 不得保留 pressed epoch")
	}
	if snapshot.Pressed && snapshot.HostCaptured {
		return fmt.Errorf("host: MouseBridgeSnapshot 不得同時為 pressed 與 host captured")
	}
	return nil
}
