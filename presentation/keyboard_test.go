package presentation

import (
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestKeyboardBridgeForwardsOnlyWhenPanelClosed(t *testing.T) {
	m := machine.New()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := NewKeyboardBridge(panel, m)
	if err != nil {
		t.Fatal(err)
	}

	state, route, err := bridge.DeliverDOSScan(0x1e) // A
	if err != nil {
		t.Fatal(err)
	}
	if state.Open || !route.ForwardToDOS || route.ConsumedByHost {
		t.Fatalf("closed route=%+v state=%+v", route, state)
	}
	if got, want := m.KeyCodes(), []uint8{0x1e, 0x9e}; !reflect.DeepEqual(got, want) {
		t.Fatalf("closed KeyCodes=%#v, want %#v", got, want)
	}

	if _, _, err := panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	before := m.KeyCodes()
	state, route, err = bridge.DeliverDOSScan(0x30) // B
	if err != nil {
		t.Fatal(err)
	}
	if !state.Open || !route.ConsumedByHost || route.ForwardToDOS {
		t.Fatalf("open route=%+v state=%+v", route, state)
	}
	if got := m.KeyCodes(); !reflect.DeepEqual(got, before) {
		t.Fatalf("open panel queued DOS keyboard events: got %#v, before %#v", got, before)
	}
}

func TestKeyboardBridgeRejectsNilDependencies(t *testing.T) {
	m := machine.New()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKeyboardBridge(nil, m); err == nil {
		t.Fatal("nil panel unexpectedly accepted")
	}
	if _, err := NewKeyboardBridge(panel, nil); err == nil {
		t.Fatal("nil machine unexpectedly accepted")
	}
	var bridge *KeyboardBridge
	if _, _, err := bridge.DeliverDOSScan(0x1e); err == nil {
		t.Fatal("nil bridge unexpectedly delivered a scan")
	}
}
