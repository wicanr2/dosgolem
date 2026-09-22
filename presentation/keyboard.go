package presentation

import (
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// KeyboardBridge connects the already-classified host panel state to the DOS
// hardware keyboard queue. It accepts a DOS scan code, not a window-backend
// key: Ebitengine key mapping and all pointer/mouse forwarding remain outside
// this small generic bridge.
//
// When the panel is open, DeliverDOSScan consumes the event and must not touch
// the machine. Once the panel is closed, it queues the normal make/break pair
// through Machine.QueueKey. It never steps the machine.
type KeyboardBridge struct {
	panel   *host.PanelController
	machine *machine.Machine
}

// NewKeyboardBridge constructs the keyboard-only input bridge. The supplied
// PanelController owns the already-confirmed host focus and Apply/Cancel
// behavior; this bridge owns no persistence or window state.
func NewKeyboardBridge(panel *host.PanelController, m *machine.Machine) (*KeyboardBridge, error) {
	if panel == nil {
		return nil, fmt.Errorf("presentation: KeyboardBridge 的 panel 不得為 nil")
	}
	if m == nil {
		return nil, fmt.Errorf("presentation: KeyboardBridge 的 machine 不得為 nil")
	}
	return &KeyboardBridge{panel: panel, machine: m}, nil
}

// DeliverDOSScan applies the panel's keyboard focus rule and queues scan only
// after it explicitly permits DOS forwarding. The returned state and route are
// the PanelController's receipt for this one input event.
func (b *KeyboardBridge) DeliverDOSScan(scan uint8) (host.PanelState, host.InputRoute, error) {
	if b == nil || b.panel == nil || b.machine == nil {
		return host.PanelState{}, host.InputRoute{}, fmt.Errorf("presentation: KeyboardBridge 不得為 nil")
	}
	state, route, err := b.panel.Route(host.PanelEvent{Kind: host.PanelEventKeyboard})
	if err != nil {
		return host.PanelState{}, host.InputRoute{}, err
	}
	if route.ForwardToDOS {
		b.machine.QueueKey(scan)
	}
	return state, route, nil
}
