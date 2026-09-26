package buckrogers

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/xlate"
)

// StepReader is the read-only per-instruction view a live runtime needs.
// dosgolem's session.StepView satisfies it; nothing here can mutate the
// machine.
type StepReader interface {
	Steps() uint64
	CS() uint16
	IP() uint16
	SS() uint16
	SP() uint16
	Read8(a uint32) uint8
	Read16(a uint32) uint16
	Palette() [256][3]uint8
}

var (
	dispatchEntry = Address{Segment: 0x0763, Offset: 0x0424}
	clearCells    = Address{Segment: 0x026F, Offset: 0x029C}
)

// LiveMenuRuntime is the menu family (menu, gender, class, roster, character
// sheet, name prompt, career and technical skill screens) of the receipt
// runner, packaged for a live session.  It keeps one presenter per output
// scale so the host can switch 2×／3× without replaying the game.
type LiveMenuRuntime struct {
	watcher    *MenuRequestWatcher
	presenters map[int]*RuntimeMenuOverlay
	fault      error
}

// NewLiveMenuRuntime builds the runtime from an already merged catalog and
// safe-rect set; font is the 16×16 GOLEMFNT used by the receipt runner.
func NewLiveMenuRuntime(catalog *MenuCatalog, rects *MenuOverlayRects, font *xlate.Font) (*LiveMenuRuntime, error) {
	if catalog == nil || rects == nil || font == nil {
		return nil, errors.New("buckrogers: live menu 輸入不得為 nil")
	}
	if err := ValidateMenuOverlayCoverage(catalog, rects); err != nil {
		return nil, err
	}
	r := &LiveMenuRuntime{watcher: NewMenuRequestWatcher(catalog), presenters: map[int]*RuntimeMenuOverlay{}}
	for _, scale := range []int{2, 3} {
		p, err := NewRuntimeMenuOverlay(rects, font, scale)
		if err != nil {
			return nil, err
		}
		r.presenters[scale] = p
	}
	return r, nil
}

// BeforeStep mirrors the receipt runner's per-instruction menu logic: a
// proven clear call, a dispatcher entry, or otherwise a possible guarded
// return that completes an event.
func (r *LiveMenuRuntime) BeforeStep(v StepReader) error {
	if r.fault != nil {
		return r.fault
	}
	at := Address{Segment: v.CS(), Offset: v.IP()}
	ss, sp := v.SS(), v.SP()
	switch at {
	case clearCells:
		bottom, right := v.Read8(linear(ss, sp+4)), v.Read8(linear(ss, sp+6))
		top, left := v.Read8(linear(ss, sp+8)), v.Read8(linear(ss, sp+10))
		for _, scale := range []int{2, 3} {
			if err := r.presenters[scale].ClearTextCells(bottom, right, top, left); err != nil {
				return r.failWith(err)
			}
		}
	case dispatchEntry:
		caller := Address{Segment: v.Read16(linear(ss, sp+2)), Offset: v.Read16(linear(ss, sp))}
		var args [6]uint16
		for i := range args {
			args[i] = v.Read16(linear(ss, sp+4+uint16(i)*2))
		}
		base := linear(args[1], args[0])
		original := make([]byte, v.Read8(base))
		for i := range original {
			original[i] = v.Read8(base + 1 + uint32(i))
		}
		r.watcher.ObserveDispatchEntry(caller, ss, sp, args, original, v.Steps())
	default:
		events, requests := r.watcher.EventCount(), r.watcher.RequestCount()
		r.watcher.ObserveInstruction(at, ss, sp, v.Steps())
		if r.watcher.EventCount() > events && r.watcher.RequestCount() > requests {
			event, eventOK := r.watcher.LastEvent()
			request, requestOK := r.watcher.LastRequest()
			if !eventOK || !requestOK {
				return r.failWith(errors.New("buckrogers: live menu request 計數前進但沒有內容"))
			}
			palette := v.Palette()
			for _, scale := range []int{2, 3} {
				if err := r.presenters[scale].Apply(event, request, palette); err != nil {
					return r.failWith(fmt.Errorf("buckrogers: live menu %d× apply：%w", scale, err))
				}
			}
		}
	}
	return nil
}

// Frame forwards one vertical retrace to every presenter.
func (r *LiveMenuRuntime) Frame(indexed []byte, palette [256][3]uint8) {
	for _, scale := range []int{2, 3} {
		r.presenters[scale].Frame(indexed, palette)
	}
}

// Draw composes the overlay for one scale onto a scaled RGBA copy.
func (r *LiveMenuRuntime) Draw(indexed []byte, palette [256][3]uint8, scale int) ([]byte, []rune, bool, error) {
	p, ok := r.presenters[scale]
	if !ok {
		return nil, nil, false, fmt.Errorf("buckrogers: live menu 不支援 %d×", scale)
	}
	rgba, missing, drew := p.Draw(indexed, palette)
	return rgba, missing, drew, nil
}

// Requests returns the display requests produced so far (for receipts).
func (r *LiveMenuRuntime) Requests() []DisplayRequest { return r.watcher.Requests() }

func (r *LiveMenuRuntime) failWith(err error) error {
	if r.fault == nil {
		r.fault = err
	}
	return r.fault
}

// linear matches the receipt runner, which addresses with cpu.Addr.
func linear(seg, off uint16) uint32 { return cpu.Addr(seg, off) }
