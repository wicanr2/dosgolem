package buckrogers

import (
	"errors"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// LiveRuntime is the Buck Rogers overlay observer for a live session.  It
// receives the same per-step, video-write, and frame events as the receipt
// runner and keeps the last retraced frame so the host can compose it at
// either output scale.  Families are added one at a time with a runner
// parity check each; today it carries the menu family only.
type LiveRuntime struct {
	menu      *LiveMenuRuntime
	indexed   []byte
	palette   [256][3]uint8
	hasFrame  bool
	frameSeen uint64
}

func NewLiveRuntime(menu *LiveMenuRuntime) (*LiveRuntime, error) {
	if menu == nil {
		return nil, errors.New("buckrogers: live runtime 需要選單家族")
	}
	return &LiveRuntime{menu: menu}, nil
}

func (r *LiveRuntime) BeforeStep(v StepReader) error { return r.menu.BeforeStep(v) }

func (r *LiveRuntime) VideoWrite(machine.VideoWrite) {}

func (r *LiveRuntime) Frame(indexed []byte, palette [256][3]uint8) {
	r.menu.Frame(indexed, palette)
	r.indexed, r.palette, r.hasFrame = indexed, palette, true
	r.frameSeen++
}

// Compose returns the last retraced frame with the overlay at scale, or
// false before the first retrace.
func (r *LiveRuntime) Compose(scale int) ([]byte, bool, error) {
	if !r.hasFrame {
		return nil, false, nil
	}
	rgba, missing, _, err := r.menu.Draw(r.indexed, r.palette, scale)
	if err != nil {
		return nil, false, err
	}
	if len(missing) != 0 {
		return nil, false, errors.New("buckrogers: live runtime 缺字")
	}
	return rgba, true, nil
}

// Frames counts retraces observed so far.
func (r *LiveRuntime) Frames() uint64 { return r.frameSeen }
