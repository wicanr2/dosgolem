package parity

import (
	"path/filepath"
	"testing"
)

func TestVersionedFD2ScenariosValidate(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "scenarios", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("找不到受版控的 FD2 場景")
	}
	for _, path := range paths {
		if _, err := LoadScenario(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestScenarioValidateRejectsUnsupportedStateAndWrongGame(t *testing.T) {
	good := Scenario{Schema: 2, Game: "fd2", Name: "title", State: "near-state", OriginalRunner: "dosgolem", Cycles: "fixed 12000", Timeline: "wait:1", ExpectedCapture: "title.png"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.State = "confirmed"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected unsupported evidence level error")
	}
	bad = good
	bad.OriginalRunner = "dosbox-bootstrap"
	bad.State = "same-state"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected bootstrap runner same-state error")
	}
	bad = good
	bad.OriginalRunner = "dosbox-x"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected unsupported original runner error")
	}
	bad = good
	bad.Game = "rich2"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected wrong game error")
	}
	bad = good
	bad.Regions = []Region{{Name: "map", X: 0, Y: 0, Width: Width, Height: Height}, {Name: "map", X: 0, Y: 0, Width: 1, Height: 1}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected duplicate region error")
	}
	bad = good
	bad.Regions = []Region{{Name: "outside", X: Width, Y: 0, Width: 1, Height: 1}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected out-of-bounds region error")
	}
}
