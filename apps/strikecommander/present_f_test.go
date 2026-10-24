package strikecommander

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

type fView struct {
	*pview
	l                      machine.IndexedLayout
	short                  bool
	layoutCalls, intoCalls int
}

func (v *fView) IndexedLayout() machine.IndexedLayout { v.layoutCalls++; return v.l }
func (v *fView) IndexedInto(dst []byte) (machine.IndexedLayout, bool) {
	v.intoCalls++
	if v.short || len(dst) < 64000 {
		return v.l, false
	}
	copy(dst, v.vga)
	return v.l, true
}
func fBoot(r *presRig) *fView {
	v := &fView{pview: r.v, l: machine.IndexedLayout{Kind: machine.IndexedLinear, Mode: 0x13, W: 320, H: 200}}
	r.v.cs, r.v.ip = 0x9999, 0x1234
	r.p.Observe(v)
	return v
}

func TestFInitialLayoutAndInvalid(t *testing.T) {
	r := newPresRig(t, 0)
	if r.p.nextWake() != 0 {
		t.Fatal("missing initial layout wake")
	}
	v := fBoot(r)
	if v.layoutCalls != 1 || !r.p.d.mode13Known {
		t.Fatal("initial layout", v.layoutCalls)
	}
	r.p.Observe(v)
	if v.layoutCalls != 1 {
		t.Fatal("idle step queried layout")
	}
	for _, pending := range []bool{false, true} {
		r := newPresRig(t, 0)
		v := fBoot(r)
		if pending {
			r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
		}
		v.l = machine.IndexedLayout{Kind: machine.IndexedUnchained, Mode: 0x13, W: 320, H: 200}
		if r.p.capture(v, PresRoutine, 0, 0, nil, nil, nil) || r.p.needFallback != pending || r.p.Current().Seq != 0 || r.p.counts.Fallback != 0 {
			t.Fatal("invalid layout changed flag or captured", pending, r.p.needFallback, r.p.Current())
		}
		if r.p.counts.NonLinear13h != 1 || r.p.counts.LayoutCorrected != 1 {
			t.Fatal(r.p.counts)
		}
		if pending {
			r.frame(nil)
			if !r.p.d.layoutCheckPending || r.p.nextWake() != 0 {
				t.Fatal("invalid Frame did not request layout")
			}
			r.v.steps++
			r.p.Observe(v)
			if !r.p.needFallback || r.p.nextWake() != r.p.sourceDrainWake() {
				t.Fatal("invalid retry spins or clears", r.p.nextWake())
			}
		}
	}
	r = newPresRig(t, 0)
	v = fBoot(r)
	v.short = true
	r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
	if r.p.capture(v, PresRoutine, 0, 0, nil, nil, nil) || r.p.counts.IndexedIntoShort != 1 || !r.p.needFallback {
		t.Fatal("short capture changed request")
	}
}

func TestFUnknownFrameAndModeStep(t *testing.T) {
	r := newPresRig(t, 0)
	r.write(8, 9, 0x7777, 0x1111, machine.VWV86)
	r.frame(nil)
	if r.p.Current().Seq != 0 || !r.p.d.layoutCheckPending || r.p.nextWake() != 0 {
		t.Fatal("unknown layout captured")
	}
	r.v.steps++
	fBoot(r)
	if r.p.Current().Kind != PresFallback || r.p.counts.Fallback != 1 || r.p.needFallback {
		t.Fatal("confirmed view failed", r.p.Current())
	}
	r = newPresRig(t, 0)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 777})
	r.p.FrameAt(1000, r.v.vga, r.v.pal)
	rows := r.p.Fallbacks()
	if len(rows) != 1 || len(rows[0].Triggers) != 1 || rows[0].Triggers[0].Kind != "mode" || rows[0].Triggers[0].Steps != 777 || rows[0].Steps != 1000 {
		t.Fatal("mode timestamp was estimated", rows)
	}
}

func TestFFlagClearAndDerivedImages(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	r.p.VideoWrite(machine.VideoWrite{Step: r.v.steps, Offset: 64000, CS: 0x7777, IP: 0x1111, Ctx: machine.VWV86})
	if r.p.needFallback || r.p.counts.Unattributed != 0 || r.p.counts.UnattributedOffscreen != 1 {
		t.Fatal("offscreen write requested fallback")
	}
	r.write(8, 9, 0x7777, 0x1111, machine.VWV86)
	r.win2d(0xfc00, func() { r.write(9, 8, t2dCS, t2dEntry+2, machine.VWV86) })
	if r.p.needFallback || r.p.counts.Fallback != 0 {
		t.Fatal("completion did not consume flag")
	}
	r.frame(nil)
	if r.p.counts.Fallback != 0 {
		t.Fatal("completion caused extra fallback")
	}
	r.write(8, 10, 0x7777, 0x1111, machine.VWV86)
	r.p.derive(PresPalette, r.v.pal, nil, false)
	r.p.derive(PresSubtitle, r.v.pal, nil, false)
	if !r.p.needFallback {
		t.Fatal("derived image consumed flag")
	}
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	if r.p.Current().Kind != PresCU || !r.p.needFallback {
		t.Fatal("BufCU consumed physical VGA request")
	}
}

func TestFContextAttribution(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	shapeCS, _, _, _ := dFrozenEntry(t, "O.shape")
	r.write(1, 9, shapeCS, 0x20, machine.VWV86)
	if r.p.counts.Unattributed != 1 {
		t.Fatal("closed routine CS was attributed")
	}
	dEnter(r, "O.shape", 0xfc00, nil)
	r.write(2, 9, 0x7777, 0x1111, machine.VWV86)
	if r.p.counts.Unattributed != 2 {
		t.Fatal("foreign CS in shape was attributed")
	}
	r.write(3, 9, shapeCS, 0x20, machine.VWV86)
	if r.p.counts.Unattributed != 2 {
		t.Fatal("owned shape write was unattributed")
	}
	dReturn(r, "O.shape", 0xfc00)
	dEnter(r, "O.puts", 0xfc00, nil)
	r.write(4, 9, 0x7777, 0x1111, machine.VWV86)
	if r.p.counts.Unattributed != 2 {
		t.Fatal("puts lost all-CS ownership")
	}
}

func TestFWindowDeferralAndOffscreen(t *testing.T) {
	for _, nested := range []bool{false, true} {
		r := newPresRig(t, 0)
		fBoot(r)
		if nested {
			dEnter(r, "O.puts", 0xfc00, nil)
		}
		dEnter(r, "O.shape", 0xfb00, nil)
		r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
		// puts owns all CS; use an explicit mode event to request fallback.
		r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
		r.frame(nil)
		if r.p.counts.Fallback != 0 || !r.p.f.windowWaiting {
			t.Fatal("writing window did not defer")
		}
		dReturn(r, "O.shape", 0xfb00)
		if nested {
			if r.p.counts.Fallback != 0 {
				t.Fatal("inner close bypassed outer window")
			}
			dReturn(r, "O.puts", 0xfc00)
		}
		if r.p.counts.Fallback != 1 || r.p.Current().Kind != PresFallback {
			t.Fatal("empty stack fallback missing", r.p.Current())
		}
		if !strings.Contains(r.p.Fallbacks()[0].Reasons, "window") {
			t.Fatal(r.p.Fallbacks())
		}
	}
	r := newPresRig(t, 0)
	fBoot(r)
	dEnter(r, "O.shape", 0xfc00, nil)
	r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
	r.frame(nil)
	if r.p.counts.Fallback != 1 || len(r.p.stack) != 1 || r.p.stack[0].wrote {
		t.Fatal("offscreen window blocked fallback")
	}
	dReturn(r, "O.shape", 0xfc00)
	if r.p.counts.Fallback != 1 {
		t.Fatal("offscreen return duplicated capture")
	}
}

func TestFWindowTimeoutWaitsForFrame(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	dEnter(r, "O.shape", 0xfc00, nil)
	start := r.v.steps
	r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
	r.frame(nil)
	r.v.steps, r.v.cs, r.v.ip = start+200000, 0x9999, 0x1234
	r.p.Observe(r.v)
	if r.p.counts.Fallback != 0 || !r.p.f.waitFrame || len(r.p.stack) != 0 {
		t.Fatal("window timeout captured before Frame")
	}
	r.frame(nil)
	if r.p.counts.Fallback != 1 || !strings.Contains(r.p.Fallbacks()[0].Reasons, "timeout") {
		t.Fatal("timeout fallback missing")
	}
}

func TestFCombinedCUWindowDeferral(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	dEnter(r, "O.puts", 0xfc00, nil)
	dEnter(r, "O.shape", 0xfb00, nil)
	r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
	r.frame(nil)
	dReturn(r, "O.shape", 0xfb00)
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x9999, 0x1234)
	if r.p.counts.Fallback != 0 || len(r.p.stack) != 1 {
		t.Fatal("CU return bypassed latched outer-window wait")
	}
	dReturn(r, "O.puts", 0xfc00)
	if r.p.counts.Fallback != 1 {
		t.Fatal("combined delay never captured")
	}
	reasons := r.p.Fallbacks()[0].Reasons
	if !strings.Contains(reasons, "cu") || !strings.Contains(reasons, "window") {
		t.Fatal(reasons)
	}
}

func TestFCUReturnAndTimeout(t *testing.T) {
	for _, ending := range []string{"return", "timeout", "void"} {
		t.Run(ending, func(t *testing.T) {
			r := newPresRig(t, 0)
			fBoot(r)
			r.inject(r.text("GEAR", 10, 0xFFFF, qList))
			r.cuIn(ipCUDiff, 1, tCUSP)
			entry := r.v.steps
			r.write(5, 0, 0x7777, 0x1111, machine.VWV86)
			r.write(6, 0, 0x0098, 0x006c, machine.VWSegmented|machine.VWCode32)
			if r.p.counts.Unattributed != 1 {
				t.Fatal("CU ctx attribution", r.p.counts.Unattributed)
			}
			r.copyFB()
			r.frame(nil)
			if r.p.counts.Fallback != 0 || !r.p.f.cuWaiting {
				t.Fatal("CU Frame captured")
			}
			sp := uint16(tCUSP)
			switch ending {
			case "timeout":
				r.v.steps = entry + 1000000
				r.frame(nil)
				if r.p.counts.CUFallbackTimeout != 1 || r.p.counts.Fallback != 1 {
					t.Fatal("CU timeout", r.p.counts)
				}
				return
			case "void":
				sp = tCUSP - 16
				r.cuIn(ipCUDiff, 1, sp)
				if r.p.counts.CUFallbackTimeout != 1 || r.p.counts.Fallback != 0 {
					t.Fatal("void captured before Frame")
				}
				r.frame(nil)
			}
			r.cuRet(ipCUDiffRet, sp)
			retSteps := r.v.steps
			if r.p.counts.Fallback != 0 {
				t.Fatal("captured at CU return")
			}
			r.frame(nil) // Even a Frame at the return step cannot bypass F5.
			if r.p.counts.Fallback != 0 {
				t.Fatal("Frame bypassed next-observation CU return")
			}
			r.at(0x9999, 0x1234)
			if r.p.counts.Fallback != 1 || r.p.Current().CaptureSteps <= retSteps || r.p.Current().SourceCS != tpBase+segFJemm || r.p.Current().SourceIP != ipCUDiffRet {
				t.Fatal("CU return source/time", r.p.Current(), r.p.counts.Fallback)
			}
		})
	}
}

func TestFSparseLayoutConfirmation(t *testing.T) {
	type sample struct {
		Captures  []PresentInfo
		Counts    PresentCounts
		Verdict   FrameVerdict
		Fallbacks []PresentFallbackRec
	}
	var samples []sample
	for _, sparse := range []bool{false, true} {
		r := newPresRig(t, 0)
		r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
		r.frame(nil)
		v := &fView{pview: r.v, l: machine.IndexedLayout{Kind: machine.IndexedLinear, Mode: 0x13, W: 320, H: 200}}
		r.v.steps++
		r.v.cs, r.v.ip = 0x9999, 0x1234
		filter := machine.StepFilter{IPs: [1024]uint64(r.so.TriggerIPs()), Wake: r.so.NextWake()}
		if sparse && !filter.ShouldObserve(r.v.ip, r.v.steps) {
			t.Fatal("confirmation filtered")
		}
		r.p.Observe(v)
		r.frame(nil)
		samples = append(samples, sample{append([]PresentInfo(nil), r.p.captures...), r.p.Counts(), r.p.LastVerdict(), r.p.Fallbacks()})
	}
	if !reflect.DeepEqual(samples[0], samples[1]) || samples[0].Counts.Fallback != 1 {
		t.Fatal("dense/sparse fallback differs", samples)
	}
}

func TestFPauseEraseAndDualCapture(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	e := dEvent(0)
	dEnter(r, "O.shape", 0xfc00, &e)
	r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
	dReturn(r, "O.shape", 0xfc00)
	r.frame(nil)
	if r.p.counts.Fallback != 0 || !r.p.d.pending {
		t.Fatal("pause fallback")
	}
	e = dEvent(2)
	e.X, e.Y = 134, 127
	dEnter(r, "O.shape", 0xfc00, &e)
	dReturn(r, "O.shape", 0xfc00)
	if r.p.Current().Kind != PresShape || r.p.needFallback || r.p.counts.Fallback != 0 {
		t.Fatal("pause ending kind/flag", r.p.Current())
	}
	for _, deferred := range []bool{false, true} {
		r := newPresRig(t, 0)
		fBoot(r)
		dEnter(r, "O.erase", 0xfc00, nil)
		r.write(5, 9, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
		dReturn(r, "O.erase", 0xfc00)
		r.write(6, 9, 0x7777, 0x1111, machine.VWV86)
		if deferred {
			dEnter(r, "O.shape", 0xfb00, nil)
			r.write(7, 9, dCS(dRow(t, r.p, "O.shape")), 0x30, machine.VWV86)
		}
		r.frame(nil)
		if deferred {
			if r.p.Current().Seq != 0 {
				t.Fatal("deferred flush captured")
			}
			dReturn(r, "O.shape", 0xfb00)
		}
		if r.p.Current().Seq != 1 || r.p.Current().Kind != PresFallback || r.p.counts.Fallback != 1 || r.p.counts.CursorEraseFlush != 1 {
			t.Fatal("flush/fallback was not one dual-counted capture", r.p.Current(), r.p.counts)
		}
	}
	r = newPresRig(t, 0)
	fBoot(r)
	dEnter(r, "O.erase", 0xfc00, nil)
	r.write(5, 9, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
	r.write(6, 9, 0x7777, 0x1111, machine.VWV86)
	r.frame(nil)
	dReturn(r, "O.erase", 0xfc00)
	if r.p.Current().Seq != 0 || !r.p.d.eraseGap {
		t.Fatal("deferred fallback landed on erase return")
	}
	r.frame(nil)
	if r.p.counts.Fallback != 1 || r.p.counts.CursorEraseFlush != 1 || !strings.Contains(r.p.Fallbacks()[0].Reasons, "erase-gap") {
		t.Fatal("erase gap did not flush once", r.p.Fallbacks())
	}
	for _, k := range []int{0, 15} {
		r := newPresRig(t, 0)
		fBoot(r)
		e := dEvent(k)
		dEnter(r, "O.shape", 0xfc00, &e)
		start := r.v.steps
		r.write(3, 9, 0x7777, 0x1111, machine.VWV86)
		dReturn(r, "O.shape", 0xfc00)
		dEnter(r, "O.erase", 0xfc00, nil)
		r.write(4, 9, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
		dReturn(r, "O.erase", 0xfc00)
		r.frame(nil)
		r.v.steps, r.v.cs, r.v.ip = start+200000, 0x9999, 0x1234
		r.p.Observe(r.v)
		if r.p.Current().Seq != 0 || !r.p.needFallback || !r.p.d.eraseGap {
			t.Fatal("pause timeout exposed erase gap", k)
		}
		r.frame(nil)
		if r.p.counts.Fallback != 1 || r.p.counts.CursorEraseFlush != 1 || r.p.Current().Seq != 1 {
			t.Fatal("pause timeout dual flush", k, r.p.Current())
		}
	}
}

func TestFDeferralAbsorbsEarlierRequest(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	dEnter(r, "O.shape", 0xfc00, nil)
	r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
	r.frame(nil)
	if !r.p.f.requested {
		t.Fatal("no prior deferred request")
	}
	e := dEvent(0)
	dEnter(r, "O.shape", 0xfb00, &e)
	dReturn(r, "O.shape", 0xfb00)
	dReturn(r, "O.shape", 0xfc00)
	r.frame(nil)
	e = dEvent(2)
	e.X, e.Y = 134, 127
	dEnter(r, "O.shape", 0xfc00, &e)
	dReturn(r, "O.shape", 0xfc00)
	if r.p.Current().Kind != PresShape || r.p.counts.Fallback != 0 || r.p.needFallback {
		t.Fatal("F6 did not absorb earlier fallback at deferral ending", r.p.Current(), r.p.counts.Fallback)
	}
}

func TestFF7SourcesOwnershipAndWake(t *testing.T) {
	r := newPresRig(t, 0)
	fBoot(r)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 777})
	r.v.steps = 1000
	r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
	r.v.steps = 1001
	r.write(6, 9, 0x7777, 0x1111, machine.VWV86)
	r.frame(nil)
	rows := r.p.Fallbacks()
	if len(rows) != 1 || len(rows[0].Triggers) != 2 || rows[0].Triggers[0].Steps != 777 || rows[0].Triggers[1].Steps != 1000 || rows[0].Triggers[1].Last != 1001 || rows[0].Triggers[1].Count != 2 {
		t.Fatal(rows)
	}
	rows[0].Triggers[0].Mode = 3
	if r.p.Fallbacks()[0].Triggers[0].Mode != 0x13 {
		t.Fatal("diagnostic aliases live state")
	}
	var output []byte
	if err := r.p.WriteFiles("", func(name string, b []byte) error {
		if name == "fallback.tsv" {
			output = append([]byte(nil), b...)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte("\tmode\t777\t777\t")) || !bytes.Contains(output, []byte("\twrite\t1000\t1001\t7777\t1111\t")) {
		t.Fatal(string(output))
	}
	r = newPresRig(t, 0)
	v := fBoot(r)
	r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
	// Explicit raw field state is the independent oracle's input, not a helper result.
	r.p.f.requested, r.p.f.readPending = true, true
	if r.p.nextWake() != 0 {
		t.Fatal("ready fallback not waking")
	}
	a := newSparseAudit(r.so)
	a.oracle(r.so, r.v.steps, 0x1234)
	if !strings.Contains(strings.Join(a.fails, "\n"), "Presenter 延後退路待取像") {
		t.Fatal(a.fails)
	}
	r.p.Observe(v)
	if r.p.counts.Fallback != 1 {
		t.Fatal("ready fallback not consumed")
	}
	// Repeated writes at an existing source do not allocate in the hot path.
	w := machine.VideoWrite{Step: 2000, CS: 0x7777, IP: 0x1111, Offset: 5, Ctx: machine.VWV86}
	r.p.VideoWrite(w)
	if n := testing.AllocsPerRun(100, func() { r.p.VideoWrite(w) }); n != 0 {
		t.Fatal("VideoWrite allocations", n)
	}
}
