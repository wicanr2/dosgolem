package strikecommander

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// PresentFallbackTrigger aggregates a pending source; Steps/Last are exact
// VideoWrite.Step or ModeChange.Step, never a retained StepView timestamp.
type PresentFallbackTrigger struct {
	Kind        string // "write" or "mode"
	Steps, Last uint64
	CS, IP      uint16
	Ctx         machine.VideoWriteCtx
	Mode        uint8
	Count       uint64
}

// PresentFallbackRec describes one successful physical-VGA fallback capture.
// SourceCS/IP in PresentInfo remain the capture point; triggers are separate.
type PresentFallbackRec struct {
	Steps, Seq uint64
	Deferred   bool
	Reasons    string
	Triggers   []PresentFallbackTrigger
}

type fallbackTriggerKey struct {
	kind   uint8
	cs, ip uint16
	ctx    machine.VideoWriteCtx
	mode   uint8
}
type presentFState struct {
	absorbed                                bool // F6: deferral ending consumes an earlier request as its own completion.
	layoutKnown                             bool
	layout                                  machine.IndexedLayout
	requested, readPending, waitFrame       bool
	cuWaiting, returnPending, windowWaiting bool
	returnObserved                          bool
	cuEntry, returnStep                     uint64
	returnCS, returnIP                      uint16
	reasons                                 uint8
	sources                                 map[fallbackTriggerKey]PresentFallbackTrigger
}

const (
	fReasonLayout uint8 = 1 << iota
	fReasonWindow
	fReasonCU
	fReasonPause
	fReasonErase
	fReasonTimeout
)

func linear13(l machine.IndexedLayout) bool { return l.Kind == machine.IndexedLinear && l.Mode == 0x13 }

func (p *Presenter) fallbackTrigger(t PresentFallbackTrigger) {
	if t.Count == 0 {
		t.Count = 1
		t.Last = t.Steps
	}
	key := fallbackTriggerKey{cs: t.CS, ip: t.IP, ctx: t.Ctx, mode: t.Mode}
	if t.Kind == "mode" {
		key.kind = 1
	}
	if p.f.sources == nil {
		p.f.sources = make(map[fallbackTriggerKey]PresentFallbackTrigger)
	}
	if old, ok := p.f.sources[key]; ok {
		old.Last = t.Last
		old.Count += t.Count
		p.f.sources[key] = old
	} else {
		p.f.sources[key] = t
	}
	p.needFallback = true
	if p.d.mode != deferIdle {
		p.d.pending = true
		p.f.reasons |= fReasonPause
	}
}

func (p *Presenter) absorbFallbackInDeferral() {
	if p.needFallback {
		p.f.absorbed = true
		p.f.waitFrame = false
		p.f.reasons |= fReasonPause
		p.d.pending = true
	}
}

// updateLayout is used only with a live view (or a mode notification below).
// Direct VGA unchaining has no ModeChange event; an actual capture corrects the
// tracked assumption, as required by 014 F4. Frame cannot query a StepView.
func (p *Presenter) updateLayout(l machine.IndexedLayout) {
	if p.f.layoutKnown && p.f.layout != l {
		p.counts.LayoutCorrected++
	}
	p.f.layoutKnown, p.f.layout = true, l
	p.d.mode13Known = linear13(l)
}
func (p *Presenter) observeLayout(v PresentView) {
	if !p.f.layoutKnown || p.d.layoutCheckPending {
		p.updateLayout(v.IndexedLayout())
		p.d.layoutCheckPending = false
	}
}
func (p *Presenter) captureLayout(v PresentView) bool {
	p.updateLayout(v.IndexedLayout())
	if !p.d.mode13Known {
		p.counts.NonLinearLayout++ // retained compatibility diagnostic
		p.counts.NonLinear13h++
		return false // U18: neither create nor clear needFallback.
	}
	return true
}
func (p *Presenter) fallbackMode(c machine.ModeChange) {
	// A BIOS mode 13h set implies its normal chained layout. This is an explicit
	// tracked assumption until a live IndexedLayout query confirms/corrects it.
	p.f.layoutKnown = true
	p.f.layout = machine.IndexedLayout{Kind: machine.IndexedLinear, Mode: c.Mode, W: 320, H: 200}
	p.d.mode13Known = c.Mode == 0x13
	p.fallbackTrigger(PresentFallbackTrigger{Kind: "mode", Steps: c.Step, Mode: c.Mode})
}

func (p *Presenter) deferFallbackCU() {
	p.f.cuWaiting = true
	p.f.readPending = false
	p.f.reasons |= fReasonCU
	if p.flight != nil {
		p.f.cuEntry = p.flight.cu.Entry().Steps
	}
}

func (p *Presenter) latchFallbackWindow() {
	if p.writingWindow() {
		p.f.windowWaiting = true
		p.f.reasons |= fReasonWindow
	}
}

// fallbackFrame runs before cursor flush, palette/subtitle derivation and the
// verdict. Only a writing window blocks a fallback; offscreen windows do not.
func (p *Presenter) fallbackFrame(indexed []byte, pal [256][3]uint8) {
	if !p.needFallback {
		return
	}
	if p.d.mode != deferIdle {
		p.d.pending = true
		p.f.reasons |= fReasonPause
		return
	}
	p.f.requested = true
	p.f.waitFrame = false
	p.latchFallbackWindow()
	if !p.f.layoutKnown || !p.d.mode13Known {
		p.counts.NonLinear13h++
		p.d.layoutCheckPending = true
		p.f.readPending = true
		p.f.reasons |= fReasonLayout
		return
	}
	if p.cuOpen {
		p.deferFallbackCU()
		return
	}
	if p.f.returnPending && !p.f.returnObserved {
		return
	} // F5: wait for the next observation after return.
	if p.writingWindow() || (p.f.windowWaiting && len(p.stack) != 0) {
		p.f.readPending = true
		p.f.reasons |= fReasonWindow
		return
	}
	if len(indexed) != 64000 {
		return
	}
	p.capture(nil, PresFallback, 0, 0, indexed, &pal, p.flightSet())
}

func (p *Presenter) fallbackObserve(v PresentView) {
	if p.f.returnPending && v.Steps() > p.f.returnStep {
		p.f.returnObserved = true
	}
	if !p.needFallback || !p.f.requested || p.f.waitFrame || p.d.mode != deferIdle {
		return
	}
	p.latchFallbackWindow()
	if p.cuOpen {
		p.deferFallbackCU()
		return
	}
	if p.writingWindow() || (p.f.windowWaiting && len(p.stack) != 0) {
		return
	}
	if p.d.eraseGap && !p.d.flushReadPending {
		p.f.reasons |= fReasonErase
		return
	}
	if p.f.returnPending && v.Steps() <= p.f.returnStep {
		return
	}
	if !p.f.readPending && !p.f.returnPending {
		return
	}
	// A failed layout confirmation is retried at the next Frame, not every step.
	if !p.d.mode13Known {
		p.f.readPending = false
		return
	}
	cs, ip := v.CS(), v.IP()
	if p.f.returnPending {
		cs, ip = p.f.returnCS, p.f.returnIP
	}
	if p.capture(v, PresFallback, cs, ip, nil, nil, p.flightSet()) {
		p.scheduleRecheck()
	}
}

func (p *Presenter) fallbackCU(e cuEvent) {
	if !p.needFallback || !p.f.cuWaiting {
		return
	}
	if e.Entry.Steps != p.f.cuEntry {
		return
	}
	switch e.Kind {
	case cuEvReturn:
		if e.Outcome == cuReturnAccepted {
			p.f.cuWaiting = false
			p.f.returnPending = true
			p.f.returnObserved = false
			p.f.returnStep = e.V.Steps()
			p.f.returnCS, p.f.returnIP = e.V.CS(), e.IP
		}
	case cuEvTimeout, cuEvVoided:
		p.counts.CUFallbackTimeout++
		p.f.cuWaiting, p.f.returnPending, p.f.readPending = false, false, false
		p.f.waitFrame = true
		p.f.reasons |= fReasonTimeout
	}
}
func (p *Presenter) fallbackWindowTimeout() {
	if p.needFallback && p.f.requested && p.f.windowWaiting {
		if !p.f.absorbed {
			p.f.waitFrame = true
			p.f.readPending = false
		}
		p.f.windowWaiting = false
		p.f.reasons |= fReasonTimeout
	}
}

func (p *Presenter) fallbackWake() uint64 {
	if !p.f.layoutKnown {
		return 0
	}
	if p.f.requested && !p.f.waitFrame && p.d.mode13Known && p.d.mode == deferIdle && !p.cuOpen && !p.writingWindow() && (!p.f.windowWaiting || len(p.stack) == 0) &&
		(p.f.returnPending || p.f.readPending) && (!p.d.eraseGap || p.d.flushReadPending) {
		return 0
	}
	return session.NoWake
}

func (p *Presenter) fallbackCaptured(img *presentImage, flush bool) {
	if flush {
		p.counts.CursorEraseFlush++
	}
	if img.info.Kind == PresFallback {
		p.counts.Fallback++
		r := PresentFallbackRec{Steps: p.steps, Seq: img.info.Seq, Deferred: p.f.reasons != 0}
		for _, row := range []struct {
			bit  uint8
			name string
		}{{fReasonLayout, "layout"}, {fReasonWindow, "window"}, {fReasonCU, "cu"}, {fReasonPause, "deferral"}, {fReasonErase, "erase-gap"}, {fReasonTimeout, "timeout"}} {
			if p.f.reasons&row.bit != 0 {
				if r.Reasons != "" {
					r.Reasons += ","
				}
				r.Reasons += row.name
			}
		}
		for _, t := range p.f.sources {
			r.Triggers = append(r.Triggers, t)
		}
		sort.Slice(r.Triggers, func(i, j int) bool {
			a, b := r.Triggers[i], r.Triggers[j]
			if a.Steps != b.Steps {
				return a.Steps < b.Steps
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			if a.CS != b.CS {
				return a.CS < b.CS
			}
			if a.IP != b.IP {
				return a.IP < b.IP
			}
			if a.Ctx != b.Ctx {
				return a.Ctx < b.Ctx
			}
			return a.Mode < b.Mode
		})
		if len(p.fallbacks) < presMaxRecords {
			p.fallbacks = append(p.fallbacks, r)
		}
	}
}
func (p *Presenter) clearFallback() {
	p.needFallback = false
	l, known, sources := p.f.layout, p.f.layoutKnown, p.f.sources
	clear(sources)
	p.f = presentFState{layout: l, layoutKnown: known, sources: sources}
}

// Fallbacks returns owned diagnostic copies. It does not establish CU identity.
func (p *Presenter) Fallbacks() []PresentFallbackRec {
	p.drainSourceWrites()
	out := append([]PresentFallbackRec(nil), p.fallbacks...)
	for i := range out {
		out[i].Triggers = append([]PresentFallbackTrigger(nil), out[i].Triggers...)
	}
	return out
}
func (p *Presenter) fallbackFiles() []byte {
	var b bytes.Buffer
	b.WriteString("steps\tseq\tdeferred\treasons\ttrigger\tfirst\tlast\tcs\tip\tctx\tmode\tcount\n")
	for _, r := range p.fallbacks {
		for _, s := range r.Triggers {
			fmt.Fprintf(&b, "%d\t%d\t%t\t%s\t%s\t%d\t%d\t%04X\t%04X\t%d\t%02X\t%d\n", r.Steps, r.Seq, r.Deferred, strings.ReplaceAll(r.Reasons, "\t", " "), s.Kind, s.Steps, s.Last, s.CS, s.IP, s.Ctx, s.Mode, s.Count)
		}
	}
	return b.Bytes()
}
