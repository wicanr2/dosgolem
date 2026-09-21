// Package host contains output-only host presentation state. It never
// forwards input to DOS or holds a machine reference.
package host

import "fmt"

// OutputScale is an explicitly supported integer output multiplier.
type OutputScale uint8

const (
	OutputScale2 OutputScale = 2
	OutputScale3 OutputScale = 3
)

// ScaleState is a value-only snapshot of host scale selection state.
// Select changes SelectedScale; only Apply may change ActiveScale.
type ScaleState struct {
	ActiveScale   OutputScale
	SelectedScale OutputScale
}

// ScaleController implements the confirmed select-then-Apply host contract.
// It deliberately contains no backend, machine, input, or persistence state.
type ScaleController struct {
	state ScaleState
}

// NewScaleController starts with the selected scale equal to the active scale.
func NewScaleController(initial OutputScale) (*ScaleController, error) {
	if !validOutputScale(initial) {
		return nil, fmt.Errorf("host: 不支援的輸出倍率 %d", initial)
	}
	return &ScaleController{state: ScaleState{ActiveScale: initial, SelectedScale: initial}}, nil
}

// Snapshot returns a value copy of the controller state.
func (c *ScaleController) Snapshot() (ScaleState, error) {
	if c == nil {
		return ScaleState{}, fmt.Errorf("host: ScaleController 不得為 nil")
	}
	return c.state, nil
}

// Select updates only the pending host scale. It never applies output changes.
func (c *ScaleController) Select(scale OutputScale) (ScaleState, error) {
	if c == nil {
		return ScaleState{}, fmt.Errorf("host: ScaleController 不得為 nil")
	}
	if !validOutputScale(scale) {
		return ScaleState{}, fmt.Errorf("host: 不支援的輸出倍率 %d", scale)
	}
	c.state.SelectedScale = scale
	return c.state, nil
}

// Apply atomically commits the pending selection. changed reports whether the
// output scale value actually changed; it does not imply a frontend redraw.
func (c *ScaleController) Apply() (state ScaleState, changed bool, err error) {
	if c == nil {
		return ScaleState{}, false, fmt.Errorf("host: ScaleController 不得為 nil")
	}
	if !validOutputScale(c.state.ActiveScale) || !validOutputScale(c.state.SelectedScale) {
		return ScaleState{}, false, fmt.Errorf("host: ScaleController 狀態無效")
	}
	changed = c.state.ActiveScale != c.state.SelectedScale
	c.state.ActiveScale = c.state.SelectedScale
	return c.state, changed, nil
}

func validOutputScale(scale OutputScale) bool {
	return scale == OutputScale2 || scale == OutputScale3
}
