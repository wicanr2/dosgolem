package host

import (
	"fmt"
	"testing"
)

func TestScaleControllerInitialState(t *testing.T) {
	for _, scale := range []OutputScale{OutputScale2, OutputScale3} {
		t.Run(fmt.Sprintf("scale_%d", scale), func(t *testing.T) {
			controller, err := NewScaleController(scale)
			if err != nil {
				t.Fatal(err)
			}
			state, err := controller.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if state != (ScaleState{ActiveScale: scale, SelectedScale: scale}) {
				t.Fatalf("state=%#v", state)
			}
		})
	}
}

func TestScaleControllerSelectThenApply(t *testing.T) {
	controller, err := NewScaleController(OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := controller.Select(OutputScale3)
	if err != nil {
		t.Fatal(err)
	}
	if state != (ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale3}) {
		t.Fatalf("Select 不得套用倍率：%#v", state)
	}
	state, changed, err := controller.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !changed || state != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) {
		t.Fatalf("Apply=%#v changed=%v", state, changed)
	}
	state, changed, err = controller.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if changed || state != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) {
		t.Fatalf("重複 Apply=%#v changed=%v", state, changed)
	}
}

func TestScaleControllerSelectCanReturnToCurrentScale(t *testing.T) {
	controller, err := NewScaleController(OutputScale3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Select(OutputScale2); err != nil {
		t.Fatal(err)
	}
	state, err := controller.Select(OutputScale3)
	if err != nil {
		t.Fatal(err)
	}
	if state != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) {
		t.Fatalf("return selection=%#v", state)
	}
	state, changed, err := controller.Apply()
	if err != nil || changed || state != (ScaleState{ActiveScale: OutputScale3, SelectedScale: OutputScale3}) {
		t.Fatalf("Apply=%#v changed=%v err=%v", state, changed, err)
	}
}

func TestScaleControllerRejectsInvalidScaleAtomically(t *testing.T) {
	for _, scale := range []OutputScale{0, 1, 4, 255} {
		t.Run(fmt.Sprintf("scale_%d", scale), func(t *testing.T) {
			if controller, err := NewScaleController(scale); controller != nil || err == nil {
				t.Fatalf("New(%d) controller=%#v err=%v", scale, controller, err)
			}
			controller, err := NewScaleController(OutputScale2)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := controller.Select(scale); err == nil {
				t.Fatalf("Select(%d) 必須失敗", scale)
			}
			state, err := controller.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if state != (ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale2}) {
				t.Fatalf("失敗後 state=%#v", state)
			}
		})
	}
}

func TestScaleControllerValueIsolationAndNilReceiver(t *testing.T) {
	controller, err := NewScaleController(OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := controller.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	state.ActiveScale, state.SelectedScale = 0, 0
	if actual, err := controller.Snapshot(); err != nil || actual != (ScaleState{ActiveScale: OutputScale2, SelectedScale: OutputScale2}) {
		t.Fatalf("snapshot 被污染：state=%#v err=%v", actual, err)
	}

	var nilController *ScaleController
	if _, err := nilController.Snapshot(); err == nil {
		t.Fatal("nil Snapshot 必須失敗")
	}
	if _, err := nilController.Select(OutputScale2); err == nil {
		t.Fatal("nil Select 必須失敗")
	}
	if _, _, err := nilController.Apply(); err == nil {
		t.Fatal("nil Apply 必須失敗")
	}
}
