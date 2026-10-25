package strikecommander

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/session"
)

// 014 v14 §3.2. P17 bytes: workplace/flicker/probe5/out/p17-sigs.txt.
// Segment values retain the original EXE-relative address space.
const (
	progMENU   presProg = 2
	progSTRIKE presProg = 3
)

func presentSig(s string) (out [16]byte) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		panic("invalid static presentation signature")
	}
	copy(out[:], b)
	return
}
func presentDRoutines() []presRoutine {
	restore := presentSig("c812000056571e8cd88ec08b7e0c8b5e")
	fill := presentSig("558bec83ec025657fc8b46088ae08b5e")
	local := presentSig("558bec83ec0856578b5e068e078b7f02")
	erase := presentSig("c812000056571ec47e108b5e1443d1e3")
	puts := presentSig("558bec83ec0656578b76068b7e08ff74")
	blit := presentSig("558bec83ec0656571e06fc8b5e068b76")
	shape := presentSig("c852010056571efc16071e0fa98b7606")
	var rows []presRoutine
	add := func(name string, prog presProg, seg, en, ret, hi uint16, sig [16]byte, kind PresentKind, all, completion, isblit bool) {
		rows = append(rows, presRoutine{name: name, prog: prog, seg: seg, entryIP: en, retIP: ret, ipLo: en, ipHi: hi, sig: sig, signed: true, kind: kind, allCS: all, completion: completion, blit: isblit, timeout: presTimeoutDefault})
	}
	add("blit", progOPTTEST, 0x39fb, 0, 0x8b, 0x8b, blit, PresBlit, false, true, true)
	add("O.shape", progOPTTEST, 0x3a97, 0, 0xf0, 0x24d, shape, PresShape, false, true, false)
	add("O.puts", progOPTTEST, 0x3942, 0x34b, 0x3d3, 0x3d3, puts, PresPuts, true, true, false)
	add("O.restore", progOPTTEST, 0x3b13, 6, 0xea, 0xea, restore, PresRoutine, false, true, false)
	add("O.fill", progOPTTEST, 0x39f6, 4, 0x4f, 0x4f, fill, PresRoutine, false, true, false)
	add("O.local", progOPTTEST, 0x39ed, 2, 0x92, 0x92, local, PresRoutine, false, true, false)
	add("O.erase", progOPTTEST, 0x3aff, 0xe, 0x144, 0x144, erase, PresRoutine, false, false, false)
	add("M.restore", progMENU, 0x1bdb, 0xe, 0xf2, 0xf2, restore, PresRoutine, false, true, false)
	add("M.fill", progMENU, 0x1b35, 8, 0x53, 0x53, fill, PresRoutine, false, true, false)
	add("M.local", progMENU, 0x1b2c, 6, 0x96, 0x96, local, PresRoutine, false, true, false)
	add("M.blit", progMENU, 0x1b3a, 4, 0x8f, 0x8f, blit, PresBlit, false, true, true)
	add("M.puts", progMENU, 0x1a82, 0x33f, 0x3c7, 0x3c7, puts, PresPuts, true, true, false)
	add("M.shape", progMENU, 0x1ba2, 2, 0xf2, 0x24f, shape, PresShape, false, true, false)
	add("M.erase", progMENU, 0x1bc8, 6, 0x13c, 0x13c, erase, PresRoutine, false, false, false)
	add("M.zoom", progMENU, 0x03bf, 6, 0xf0, 0xf0, presentSig("558bec1efcc5760ac47e068a1446b6ff"), PresRoutine, false, true, false)
	rows[len(rows)-1].timeout = 1_000_000
	add("M.rle", progMENU, 0x100a, 0xc, 0x3e9, 0x3e9, presentSig("c81c00005756061e9cfcc5760616078d"), PresRoutine, false, true, false)
	add("M.memcpy", progMENU, 0, 0x34bf, 0x34e2, 0x34e2, presentSig("558bec56578cdac47e06c5760a8b4e0e"), PresRoutine, false, true, false)
	add("S.restore", progSTRIKE, 0x616e, 0xa, 0xee, 0xee, restore, PresRoutine, false, true, false)
	add("S.fill", progSTRIKE, 0x602a, 0xa, 0x55, 0x55, fill, PresRoutine, false, true, false)
	add("S.local", progSTRIKE, 0x6021, 8, 0x98, 0x98, local, PresRoutine, false, true, false)
	add("S.erase", progSTRIKE, 0x615b, 2, 0x138, 0x138, erase, PresRoutine, false, false, false)
	add("S.blit", progSTRIKE, 0x602f, 6, 0x91, 0x91, blit, PresBlit, false, true, true)
	add("S.puts", progSTRIKE, 0x5f7a, 0x345, 0x3cd, 0x3cd, puts, PresPuts, true, true, false)
	add("S.shape", progSTRIKE, 0x60f2, 4, 0xf4, 0x251, shape, PresShape, false, true, false)
	return rows
}

type RoutineCount struct{ Opened, Returned, WroteVGA, DuplicateEntry, DuplicateDelivered, ReturnWithoutEntry, ReturnDuplicateDelivered, Timeout, Skipped uint64 }

func (p *Presenter) updateRoutine(name string, fn func(*RoutineCount)) {
	if p.counts.Routines == nil {
		p.counts.Routines = map[string]RoutineCount{}
	}
	c := p.counts.Routines[name]
	fn(&c)
	p.counts.Routines[name] = c
}
func (p *Presenter) routineEntryDuplicate(r *presRoutine, legal bool) {
	p.updateRoutine(r.name, func(c *RoutineCount) {
		if legal {
			c.DuplicateDelivered++
		} else {
			c.DuplicateEntry++
		}
	})
}

type presReturn struct {
	r            *presRoutine
	cs, ss, sp   uint16
	deliv, steps uint64
	valid        bool
}
type presCaptureCU struct {
	key, captured, returnVW, returnState uint64
	returned, residue, promoted          bool
}
type deferMode uint8

const (
	deferIdle deferMode = iota
	deferK0
	deferK15
)

type presentDState struct {
	menu                                                                                               bool
	optCandidate, strikeCandidate                                                                      uint16
	optCandidateOK, strikeCandidateOK                                                                  bool
	returns                                                                                            [64]presReturn
	mode                                                                                               deferMode
	start                                                                                              uint64
	group, x, y                                                                                        int
	load, yes, no, pending, eraseGap, flushDeferred, flushReadPending, mode13Known, layoutCheckPending bool
	endKind                                                                                            PresentKind
	verdict                                                                                            FrameVerdict
	verdicts                                                                                           []func(FrameVerdict)
	cuCapture                                                                                          presCaptureCU
	flightCommit                                                                                       FlightCommit
	flightCommitValid                                                                                  bool
}

func (p *Presenter) dispatchD(v PresentView, ip uint16) {
	cs := v.CS()
	for i := range p.routines {
		r := &p.routines[i]
		if ip != r.entryIP && ip != r.retIP {
			continue
		}
		if ip == r.entryIP {
			if r.prog == progAbs {
				if cs == r.seg {
					p.entry(v, r)
				}
				continue
			}
			if r.signed {
				ok := true
				for j, b := range r.sig {
					if v.Read8(lin(cs, ip+uint16(j))) != b {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
			}
			switch r.prog {
			case progOPTTEST:
				base, known := p.labels.Base()
				if known {
					if cs != base+r.seg {
						continue
					}
				} else {
					base = cs - r.seg
					if why := p.labels.checkBase(v, base); why != "" {
						p.candidateFailure(v, r, why)
						continue
					}
					p.d.optCandidate, p.d.optCandidateOK = base, true
				}
			case progSTRIKE:
				if p.flight == nil {
					continue
				}
				base, known := p.flight.Base()
				if known {
					if cs != base+r.seg {
						continue
					}
				} else {
					base = cs - r.seg
					if why := p.flight.verifyRows(v, base); why != "" {
						p.candidateFailure(v, r, why)
						continue
					}
					p.d.strikeCandidate, p.d.strikeCandidateOK = base, true
				}
			case progMENU:
				if r.name == "M.memcpy" && !p.d.menu {
					continue
				}
				p.d.menu = true
			}
			p.entry(v, r)
			if len(p.stack) != 0 {
				w := p.stack[len(p.stack)-1]
				for j := range p.queue {
					if p.queue[j].art && p.queue[j].ev.Steps == v.Steps() {
						p.queue[j].win = w
					}
				}
			}
		} else if p.returnCSAllowed(r, cs, v.Steps()) {
			p.ret(v, r)
		}
	}
}
func (p *Presenter) candidateFailure(v PresentView, r *presRoutine, why string) {
	p.counts.CandidateBaseFailed++
	p.logWindow(PresentWindowRec{Steps: v.Steps(), Event: "candidate-base-failed: " + why, Routine: r.name, CS: v.CS(), SS: v.SS(), SP: v.SP()})
}
func (p *Presenter) returnCSAllowed(r *presRoutine, cs uint16, steps uint64) bool {
	if r.prog == progAbs {
		return cs == r.seg
	}
	if r.prog == progOPTTEST {
		if base, ok := p.labels.Base(); ok {
			return cs == base+r.seg
		}
	}
	if r.prog == progSTRIKE && p.flight != nil {
		if base, ok := p.flight.Base(); ok {
			return cs == base+r.seg
		}
	}
	for _, w := range p.stack {
		if w.r == r && w.cs == cs {
			return true
		}
	}
	for _, h := range p.d.returns {
		if h.valid && h.r == r && h.cs == cs && steps-h.steps <= presTimeoutDefault {
			return true
		}
	}
	return false
}
func (p *Presenter) rememberReturn(w *presWindow, deliv, steps uint64) {
	idx := -1
	oldest := uint64(^uint64(0))
	oldidx := 0
	for i, h := range p.d.returns {
		if h.valid && h.r == w.r && h.ss == w.ss && h.sp == w.sp {
			idx = i
			break
		}
		if !h.valid && idx < 0 {
			idx = i
		}
		if h.valid && h.steps < oldest {
			oldest = h.steps
			oldidx = i
		}
	}
	if idx < 0 {
		idx = oldidx
		p.counts.ReturnHistoryEvicted++
	}
	p.d.returns[idx] = presReturn{r: w.r, cs: w.cs, ss: w.ss, sp: w.sp, deliv: deliv, steps: steps, valid: true}
}
func (p *Presenter) duplicateReturn(r *presRoutine, cs, ss, sp uint16, deliv, steps uint64) bool {
	for i := range p.d.returns {
		h := &p.d.returns[i]
		if h.valid && h.r == r && h.cs == cs && h.ss == ss && h.sp == sp && steps-h.steps <= presTimeoutDefault && deliv > h.deliv {
			h.deliv = deliv
			return true
		}
	}
	return false
}

func (p *Presenter) deferArt(ev ArtEvent) {
	if ev.File != ArtSaveLoad || ev.G < 0 || ev.G > 1 {
		return
	}
	switch ev.K {
	case 0:
		p.d.mode = deferK0
		p.absorbFallbackInDeferral()
		p.d.start = ev.Steps
		p.d.group, p.d.x, p.d.y = ev.G, ev.X, ev.Y
		p.d.load, p.d.yes, p.d.no = false, false, false
	case 15:
		p.d.mode = deferK15
		p.absorbFallbackInDeferral()
		p.d.start = ev.Steps
		p.d.group = ev.G
		p.d.yes, p.d.no = false, false
	case 4, 5:
		if p.d.mode == deferK0 && ev.G == p.d.group && ev.X == p.d.x+54 && ev.Y == p.d.y+97 {
			p.d.load = true
		}
	}
}
func (p *Presenter) endDeferral(kind PresentKind, steps uint64) {
	if p.d.mode == deferIdle {
		return
	}
	p.d.mode = deferIdle
	p.d.endKind = kind
	p.d.start = steps
}
func (p *Presenter) clearDeferral(steps uint64) {
	if p.d.mode == deferK15 {
		p.endDeferral(PresBlit, steps)
	}
}
func (p *Presenter) deferWindowClosed(w *presWindow, steps uint64) {
	for _, it := range p.queue {
		if !it.art || it.win != w || it.ev.File != ArtSaveLoad || it.ev.G != p.d.group || it.ev.Steps < p.d.start {
			continue
		}
		ev := it.ev
		if p.d.mode == deferK0 {
			if (ev.K == 2 || ev.K == 3) && !p.d.load && ev.X == p.d.x+54 && ev.Y == p.d.y+97 {
				p.endDeferral(w.kind, steps)
			} else if ev.K == 1 && p.d.load {
				p.endDeferral(w.kind, steps)
			}
		}
		if p.d.mode == deferK15 {
			if ev.K == 17 || ev.K == 18 {
				p.d.yes = true
			}
			if ev.K == 19 || ev.K == 20 {
				p.d.no = true
			}
			if p.d.yes && p.d.no {
				p.endDeferral(w.kind, steps)
			}
		}
	}
}
func (p *Presenter) observeDeferral(v PresentView) {
	if p.d.mode != deferIdle && v.Steps()-p.d.start >= presTimeoutDefault {
		p.counts.ModeTimeout++
		p.endDeferral(PresRoutine, v.Steps())
	}
}

// pendingCaptureReady is read-only. A retained pending flag cannot make a
// rejected physical capture spin; F4's layout-check wake remains independent.
func (p *Presenter) pendingCaptureReady() bool {
	if p.d.mode != deferIdle || len(p.stack) != 0 || !p.d.mode13Known || (p.d.eraseGap && !p.d.flushReadPending) {
		return false
	}
	if p.needFallback && p.f.requested && (p.f.waitFrame || p.cuOpen || (p.f.returnPending && p.steps <= p.f.returnStep)) {
		return false
	}
	return true
}
func (p *Presenter) finishPending(v PresentView) {
	if !p.pendingCaptureReady() {
		return
	}
	if p.d.pending || p.d.flushDeferred {
		kind := p.d.endKind
		if kind == "" {
			kind = PresRoutine
		}
		if p.d.flushDeferred {
			kind = PresRoutine
		}
		if p.capture(v, kind, v.CS(), v.IP(), nil, nil, p.flightSet()) {
			p.scheduleRecheck()
		}
	}
}
func (p *Presenter) cursorFrame(indexed []byte, palette [256][3]uint8) {
	if !p.d.eraseGap {
		return
	}
	if p.d.mode != deferIdle {
		p.d.pending = true
		return
	}
	if p.writingWindow() || p.cuOpen {
		p.d.flushDeferred = true
		p.d.flushReadPending = false
		return
	}
	if !p.d.mode13Known {
		p.d.flushDeferred = true
		p.d.flushReadPending = true
		p.d.layoutCheckPending = true
		return
	}
	if len(indexed) != 64000 {
		return
	}
	kind := PresRoutine
	if p.needFallback {
		kind = PresFallback
	}
	p.capture(nil, kind, 0, 0, indexed, &palette, p.flightSet())
}
func (p *Presenter) writingWindow() bool {
	for _, w := range p.stack {
		if w.wrote {
			return true
		}
	}
	return false
}

// presentationWake is integrated by sparse.go; the audit uses independent fields.
func (p *Presenter) presentationWake() uint64 {
	if p.d.layoutCheckPending {
		return 0
	}
	if p.pendingCaptureReady() && (p.d.pending || p.d.flushReadPending) {
		return 0
	}
	if p.d.mode != deferIdle {
		return p.d.start + presTimeoutDefault
	}
	return session.NoWake
}

func presentK0Rows(ev ArtEvent) []ArtEvent {
	if ev.W != 182 || ev.H != 131 || len(ev.Entry) != ev.W*ev.H || len(ev.Base0) != ev.W*ev.H {
		return nil
	}
	var out []ArtEvent
	for _, r := range []struct{ k, x, y, w int }{{2, 54, 97, 75}, {6, 10, 112, 80}, {8, 94, 112, 80}} {
		e := ev
		e.K = r.k
		e.X += r.x
		e.Y += r.y
		e.W, e.H = r.w, 12
		e.Entry = make([]byte, e.W*e.H)
		e.Base0 = make([]byte, e.W*e.H)
		for y := 0; y < 12; y++ {
			copy(e.Entry[y*e.W:(y+1)*e.W], ev.Entry[(y+r.y)*ev.W+r.x:(y+r.y)*ev.W+r.x+e.W])
			copy(e.Base0[y*e.W:(y+1)*e.W], ev.Base0[(y+r.y)*ev.W+r.x:(y+r.y)*ev.W+r.x+e.W])
		}
		out = append(out, e)
	}
	return out
}

type VerdictClass uint8

const (
	VerdictEligible VerdictClass = iota
	VerdictExempt
	VerdictStale
	VerdictNoImage
)

type ExemptReason uint8

const (
	ExemptNone ExemptReason = iota
	ExemptWindowWriting
	ExemptInCU
	ExemptDeferral
	ExemptEraseGapDeferred
	ExemptAfterCUVoid
	ExemptPendingCapture
)

type FrameVerdict struct {
	Steps, Seq    uint64
	Class         VerdictClass
	Reason        ExemptReason
	Diff          int
	FrameComplete bool
}

// LastVerdict 回最近一次 FrameAt 的值；尚無回掃時為 VerdictNoImage、Steps 0。
func (p *Presenter) LastVerdict() FrameVerdict { return p.d.verdict }

// OnVerdict 在每次回掃處理完呈現像後通知；回呼只能呼叫 Compose 或讀取方法。
func (p *Presenter) OnVerdict(fn func(FrameVerdict)) { p.d.verdicts = append(p.d.verdicts, fn) }
func (p *Presenter) verdictFrame(indexed []byte) {
	v := FrameVerdict{Steps: p.steps, FrameComplete: true, Class: VerdictNoImage}
	if p.cur != nil {
		v.Seq = p.cur.info.Seq
	}
	switch {
	case p.writingWindow():
		v.Reason = ExemptWindowWriting
		v.FrameComplete = false
	case p.cuOpen:
		v.Reason = ExemptInCU
		v.FrameComplete = false
	case p.d.mode != deferIdle:
		v.Reason = ExemptDeferral
	case len(p.stack) != 0 && (p.epWrote || p.d.pending):
		v.Reason = ExemptPendingCapture
	case p.d.flushDeferred:
		v.Reason = ExemptEraseGapDeferred
	case p.staleVoid:
		v.Reason = ExemptAfterCUVoid
	}
	rec := PresentStale{Steps: p.steps, Seq: v.Seq}
	if p.cur != nil {
		rec.Kind = p.cur.info.Kind
	}
	if v.Reason != ExemptNone {
		v.Class = VerdictExempt
		if v.Reason == ExemptAfterCUVoid {
			p.counts.StaleAfterCUVoid++
			rec.Class = "after-cu-void"
			if p.cur != nil {
				rec.Diff, rec.X0, rec.Y0, rec.X1, rec.Y1 = diffBox(p.cur.indexed, indexed)
			}
			p.staleRec(rec)
		}
	} else {
		p.counts.StaleChecked++
		if p.cur == nil {
			p.counts.StaleNoImage++
		} else if bytes.Equal(p.cur.indexed, indexed) {
			v.Class = VerdictEligible
		} else {
			v.Class = VerdictStale
			p.counts.Stale++
			rec.Class = "stale"
			rec.Diff, rec.X0, rec.Y0, rec.X1, rec.Y1 = diffBox(p.cur.indexed, indexed)
			v.Diff = rec.Diff
			p.staleRec(rec)
		}
	}
	p.d.verdict = v
	if p.frontend != nil {
		p.frontend.onVerdict(v, indexed, p.frontendPalette)
	}
	for _, fn := range p.d.verdicts {
		fn(v)
	}
}

func (p *Presenter) routineCountsText() string {
	var b bytes.Buffer
	names := make([]string, 0, len(p.counts.Routines))
	for name := range p.counts.Routines {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "routine %s %+v\n", name, p.counts.Routines[name])
	}
	return b.String()
}
func (p *Presenter) CountsText() []byte {
	p.drainSourceWrites()
	return append(p.countsTextBase(), []byte(p.routineCountsText())...)
}
