package presentation

import (
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
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

func TestKeyboardBridgeBIOSDeliveryRespectsPanelFocus(t *testing.T) {
	m := machine.New()
	bios := dos.New(m, ".")
	bios.Install()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	key := dos.Key{Scan: 0x1c, ASCII: 0x0d}
	_, route, err := bridge.DeliverBIOSKey(key)
	if err != nil || !route.ForwardToDOS || bios.KeysPending() != 1 {
		t.Fatalf("closed BIOS route=%+v pending=%d err=%v", route, bios.KeysPending(), err)
	}
	if _, _, err := panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	before := bios.KeysPending()
	_, route, err = bridge.DeliverBIOSKey(key)
	if err != nil || !route.ConsumedByHost || route.ForwardToDOS || bios.KeysPending() != before {
		t.Fatalf("open BIOS route=%+v pending=%d before=%d err=%v", route, bios.KeysPending(), before, err)
	}
	if _, err := NewKeyboardBridgeWithBIOS(panel, m, nil); err == nil {
		t.Fatal("nil BIOS unexpectedly accepted")
	}
	other := machine.New()
	otherDOS := dos.New(other, ".")
	if _, err := NewKeyboardBridgeWithBIOS(panel, m, otherDOS); err == nil {
		t.Fatal("different machine BIOS unexpectedly accepted")
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

func TestKeyboardBridgeBIOSPreflightAndMutableMachineAlias(t *testing.T) {
	m := machine.New()
	bios := dos.New(m, ".")
	bios.Install()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	otherPanel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := NewKeyboardBridge(panel, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := plain.ValidateBIOSForPanel(panel); err == nil {
		t.Fatal("bridge without BIOS passed preflight")
	}
	bridge, err := NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.ValidateBIOSForPanel(panel); err != nil {
		t.Fatal(err)
	}
	if err := bridge.ValidateBIOSForPanel(otherPanel); err == nil {
		t.Fatal("different panel passed preflight")
	}
	if err := bridge.ValidateBIOSForPanel(nil); err == nil {
		t.Fatal("nil panel passed preflight")
	}
	var nilBridge *KeyboardBridge
	if err := nilBridge.ValidateBIOSForPanel(panel); err == nil {
		t.Fatal("nil bridge passed preflight")
	}
	if err := new(KeyboardBridge).ValidateBIOSForPanel(panel); err == nil {
		t.Fatal("zero bridge passed preflight")
	}
	for _, invalid := range []struct {
		name          string
		bridge        KeyboardBridge
		deliverReject bool
	}{
		{"missing bridge panel", KeyboardBridge{machine: m, bios: bios}, true},
		{"different bridge panel", KeyboardBridge{panel: otherPanel, machine: m, bios: bios}, false},
		{"missing bridge machine", KeyboardBridge{panel: panel, bios: bios}, true},
		{"missing bridge BIOS", KeyboardBridge{panel: panel, machine: m}, true},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			beforePanel, err := panel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			beforeOther, err := otherPanel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			beforeKeys := m.BIOSKeyCount()
			if err := invalid.bridge.ValidateBIOSForPanel(panel); err == nil {
				t.Fatal("invalid internal bridge passed preflight")
			}
			if invalid.deliverReject {
				if _, _, err := invalid.bridge.DeliverBIOSKey(dos.Key{Scan: 0x1c, ASCII: 0x0d}); err == nil {
					t.Fatal("invalid internal bridge delivered a key")
				}
			}
			afterPanel, err := panel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			afterOther, err := otherPanel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(afterPanel, beforePanel) || !reflect.DeepEqual(afterOther, beforeOther) || m.BIOSKeyCount() != beforeKeys {
				t.Fatalf("rejected bridge changed panel or BIOS queue: main=%+v→%+v other=%+v→%+v keys=%d→%d", beforePanel, afterPanel, beforeOther, afterOther, beforeKeys, m.BIOSKeyCount())
			}
		})
	}
	before, err := panel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	otherMachine := machine.New()
	bios.M = otherMachine // DOS.M is public and can change after construction.
	if err := bridge.ValidateBIOSForPanel(panel); err == nil {
		t.Fatal("changed BIOS machine passed preflight")
	}
	if _, _, err := bridge.DeliverBIOSKey(dos.Key{Scan: 0x1c, ASCII: 0x0d}); err == nil {
		t.Fatal("changed BIOS machine accepted a key")
	}
	after, err := panel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) || m.BIOSKeyCount() != 0 || otherMachine.BIOSKeyCount() != 0 {
		t.Fatalf("failed delivery changed panel/queues: before=%+v after=%+v queues=%d/%d", before, after, m.BIOSKeyCount(), otherMachine.BIOSKeyCount())
	}
}
