package host

import (
	"fmt"
	"reflect"
)

// MouseTarget is the host hit-test classification captured for a pointer
// event. The host backend owns that classification; MouseBridge never imports
// a window toolkit or a DOS machine.
type MouseTarget uint8

const (
	MouseTargetCanvas MouseTarget = iota + 1
	MouseTargetHost
	MouseTargetOutside
)

// MouseEventKind is the small event surface needed by the generic bridge.
type MouseEventKind uint8

const (
	MouseEventDown MouseEventKind = iota + 1
	MouseEventUp
	MouseEventFocusLost
)

// MouseEvent uses host logical pixels. Button 0 is the DOS left button.
type MouseEvent struct {
	Kind   MouseEventKind
	Button int
	X, Y   int
	Target MouseTarget
}

// MouseLayout is immutable input for one host update. Canvas is in DOS logical
// pixels; ChromeHeight, FrameWidth and FrameHeight are output logical pixels.
// Backends must use the same value for hit testing and Handle.
type MouseLayout struct {
	Epoch                   uint64
	Scale                   OutputScale
	ChromeHeight            int
	Canvas                  Canvas
	FrameWidth, FrameHeight int
	PanelOpen               bool
}

func (s MouseLayout) valid() bool {
	if s.Epoch == 0 || !validOutputScale(s.Scale) || s.ChromeHeight < 0 || !s.Canvas.valid() {
		return false
	}
	maxInt := int(^uint(0) >> 1)
	scale := int(s.Scale)
	if s.Canvas.Width > maxInt/scale || s.Canvas.Height > maxInt/scale {
		return false
	}
	width, height := s.Canvas.Width*scale, s.Canvas.Height*scale
	if s.ChromeHeight > maxInt-height {
		return false
	}
	return s.FrameWidth >= width && s.FrameHeight >= s.ChromeHeight+height
}

func (s MouseLayout) canvasContains(x, y int) bool {
	return s.valid() && x >= 0 && x < s.Canvas.Width*int(s.Scale) &&
		y >= s.ChromeHeight && y < s.ChromeHeight+s.Canvas.Height*int(s.Scale)
}

// MouseOutput is the only write capability given to MouseBridge. An adapter
// may call DOS mouse APIs, but the bridge itself holds no machine reference.
type MouseOutput interface {
	MoveMouse(x, y int)
	PressMouse(button int)
	ReleaseMouse(button int)
}

// MouseRoute records whether a host event was consumed or reached MouseOutput.
// Cleanup means an already accepted left press was released exactly once.
type MouseRoute struct {
	ConsumedByHost bool
	ForwardedToDOS bool
	Cleanup        bool
	Reason         string
}

// MouseBridgeSnapshot is a value copy of one bridge's routing state. It
// includes the epoch of an accepted DOS press, which callers cannot recover
// from the current layout after ApplyLayout. It contains no output capability
// or mutable pointer. Like Handle, Snapshot is used on the owning goroutine.
type MouseBridgeSnapshot struct {
	Current      MouseLayout
	HasCurrent   bool
	Pressed      bool
	PressedEpoch uint64
	HostCaptured bool
}

// MouseBridge owns one generic DOS left-button lifecycle. It is deliberately
// independent of Ebitengine, PanelController, games and dosgolem.Machine.
type MouseBridge struct {
	output       MouseOutput
	current      MouseLayout
	hasCurrent   bool
	pressed      bool
	pressedEpoch uint64
	hostCaptured bool
}

func NewMouseBridge(output MouseOutput) (*MouseBridge, error) {
	if nilMouseOutput(output) {
		return nil, fmt.Errorf("host: MouseOutput 不得為 nil")
	}
	return &MouseBridge{output: output}, nil
}

func nilMouseOutput(output MouseOutput) bool {
	if output == nil {
		return true
	}
	v := reflect.ValueOf(output)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// ApplyLayout adopts only a strictly newer valid layout. A rejected layout
// leaves the active layout and any accepted DOS press intact.
func (b *MouseBridge) ApplyLayout(layout MouseLayout) error {
	if b == nil || b.output == nil {
		return fmt.Errorf("host: MouseBridge 不得為 nil")
	}
	if !layout.valid() {
		return fmt.Errorf("host: MouseLayout 無效")
	}
	if b.hasCurrent && layout.Epoch <= b.current.Epoch {
		return fmt.Errorf("host: MouseLayout epoch 必須嚴格遞增")
	}
	b.current, b.hasCurrent = layout, true
	return nil
}

func (b *MouseBridge) Layout() (MouseLayout, bool) {
	if b == nil {
		return MouseLayout{}, false
	}
	return b.current, b.hasCurrent
}

func (b *MouseBridge) Pressed() bool {
	return b != nil && b.pressed
}

func (b *MouseBridge) HostCaptured() bool {
	return b != nil && b.hostCaptured
}

// Snapshot reads the complete routing state in one call without changing it
// or touching MouseOutput. A nil bridge returns the zero-value snapshot.
func (b *MouseBridge) Snapshot() MouseBridgeSnapshot {
	if b == nil {
		return MouseBridgeSnapshot{}
	}
	return MouseBridgeSnapshot{
		Current:      b.current,
		HasCurrent:   b.hasCurrent,
		Pressed:      b.pressed,
		PressedEpoch: b.pressedEpoch,
		HostCaptured: b.hostCaptured,
	}
}

// Handle processes an event against exactly the supplied current layout.
// Focus loss intentionally ignores layout freshness so it can release an
// accepted DOS button even after a resize or presentation transition.
func (b *MouseBridge) Handle(layout MouseLayout, event MouseEvent) MouseRoute {
	if b == nil || b.output == nil {
		return MouseRoute{Reason: "nil-bridge-rejected"}
	}
	if event.Kind == MouseEventFocusLost {
		b.hostCaptured = false
		return b.releaseOnly("focus-lost-release")
	}
	if !b.hasCurrent || layout != b.current {
		return MouseRoute{Reason: "stale-layout-epoch-rejected"}
	}
	if event.Kind != MouseEventDown && event.Kind != MouseEventUp {
		return MouseRoute{Reason: "unknown-event-rejected"}
	}
	if event.Button != 0 {
		if layout.PanelOpen {
			return MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
		}
		return MouseRoute{Reason: "non-left-rejected"}
	}
	if event.Kind == MouseEventUp {
		if b.pressed {
			if b.pressedEpoch != layout.Epoch {
				return b.releaseOnly("epoch-changed-release")
			}
			if event.Target != MouseTargetCanvas || layout.PanelOpen || !layout.canvasContains(event.X, event.Y) {
				return b.releaseOnly("non-canvas-target-release")
			}
			b.output.MoveMouse(event.X/int(layout.Scale), (event.Y-layout.ChromeHeight)/int(layout.Scale))
			return b.releaseOnly("up-release")
		}
		if b.hostCaptured {
			b.hostCaptured = false
			return MouseRoute{ConsumedByHost: true, Reason: "host-captured-up-consumed"}
		}
		if layout.PanelOpen {
			return MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
		}
		return MouseRoute{Reason: "unmatched-up-rejected"}
	}

	if b.pressed {
		return MouseRoute{Reason: "duplicate-down-rejected"}
	}
	if b.hostCaptured {
		return MouseRoute{ConsumedByHost: true, Reason: "host-captured-down-consumed"}
	}
	if layout.PanelOpen {
		b.hostCaptured = true
		return MouseRoute{ConsumedByHost: true, Reason: "panel-open-consumed"}
	}
	if event.Target == MouseTargetHost {
		b.hostCaptured = true
		return MouseRoute{ConsumedByHost: true, Reason: "host-down-consumed"}
	}
	if event.Target != MouseTargetCanvas || !layout.canvasContains(event.X, event.Y) {
		return MouseRoute{Reason: "outside-canvas-rejected"}
	}
	b.output.MoveMouse(event.X/int(layout.Scale), (event.Y-layout.ChromeHeight)/int(layout.Scale))
	b.output.PressMouse(0)
	b.pressed, b.pressedEpoch = true, layout.Epoch
	return MouseRoute{ForwardedToDOS: true, Reason: "canvas-down-forwarded"}
}

func (b *MouseBridge) releaseOnly(reason string) MouseRoute {
	if !b.pressed {
		return MouseRoute{Reason: "unmatched-up-rejected"}
	}
	b.output.ReleaseMouse(0)
	b.pressed, b.pressedEpoch = false, 0
	return MouseRoute{ForwardedToDOS: true, Cleanup: true, Reason: reason}
}
