package parity

import "testing"

func TestScenarioValidate(t *testing.T) {
	good := Scenario{Schema: 1, Game: "fd2", Name: "title", State: "near-state", Cycles: "fixed 12000", Timeline: "wait:1", ExpectedCapture: "title.png"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.State = "confirmed"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected unsupported evidence level error")
	}
	bad = good
	bad.Game = "rich2"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected wrong game error")
	}
}
