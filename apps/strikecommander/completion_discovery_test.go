package strikecommander

import (
	"testing"
)

// Count the original synthetic memory port, not a second signature verifier.
type completionReadView struct {
	*fv
	reads int
}

func (v *completionReadView) Read8(a uint32) uint8 { v.reads++; return v.fv.Read8(a) }
func completionObserve(w *world, v *completionReadView, step uint64, base, ip, cx uint16) {
	v.steps, v.cs, v.ip, v.ss, v.sp, v.cx = step, base+segFJemm, ip, tSS, tCUSP, cx
	w.o.Observe(v)
}
func completionKnown(t *testing.T, o *FlightObserver, base uint16, step uint64) {
	t.Helper()
	b, ok := o.Base()
	e := o.cu.Entry()
	if !ok || b != base || !e.InProgress || e.Steps != step {
		t.Fatalf("current CU was not discovered/accepted: base=%04x ok=%v entry=%+v", b, ok, e)
	}
	if o.Counts().HitPuts != 0 {
		t.Fatal("CU discovery fabricated a puts")
	}
}
func TestCompletionDiscoveryFrontendAndDefaultOff(t *testing.T) {
	for _, base := range []uint16{0x0300, 0x0780} {
		old := newWorld(t, base)
		v := &completionReadView{fv: old.v}
		completionObserve(old, v, 1000000, base, ipCUDiff, 1)
		if _, ok := old.o.Base(); ok || v.reads != 0 || old.o.Counts().HitCUDiff != 0 || old.o.Generation() != 0 {
			t.Fatal("legacy default changed before puts", old.o.Counts(), v.reads)
		}
		log, dropped := old.o.Log()
		if len(log) != 0 || dropped != 0 {
			t.Fatal("opt-off created diagnostics")
		}
		for _, ip := range []uint16{ipCUDiff, ipCUPlain} {
			w := newWorld(t, base)
			w.o.discoverCompletionCU = true
			view := &completionReadView{fv: w.v}
			completionObserve(w, view, 1000100, base, ip, 1)
			completionKnown(t, w.o, base, 1000100)
			if len(w.o.Visible()) != 0 || len(w.o.Pending()) != 0 || len(w.o.Candidates()) != 0 || w.o.Generation() != 0 {
				t.Fatal("discovery manufactured text")
			}
		}
	}
	r, err := NewFrontendRuntime(runtimeTestConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.so.Flight.discoverCompletionCU {
		t.Fatal("frontend without HD collector did not opt in")
	}
	lease, err := r.Prepare(FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1, HD: false})
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	if !r.so.Flight.discoverCompletionCU {
		t.Fatal("HD-off Prepare changed observer opt-in")
	}
}
func TestCompletionDiscoveryFullSignaturesAndEntryGuards(t *testing.T) {
	for i := range flightBaseRows {
		row := flightBaseRows[i]
		t.Run(row.name, func(t *testing.T) {
			w := newWorld(t, 0x0300)
			w.o.discoverCompletionCU = true
			w.v.mem[lin(w.base+row.seg, row.off+uint16(len(row.bytes)-1))] ^= 1
			completionObserve(w, &completionReadView{fv: w.v}, 1000000, w.base, ipCUDiff, 1)
			if _, ok := w.o.Base(); ok || w.o.cu.Entry().InProgress {
				t.Fatal("bad full signature row accepted", row.name)
			}
		})
	}
	for _, which := range []string{"puts", "jemm-tail", "only-jemm", "wrong-CS", "return-only", "CH-nonzero"} {
		t.Run(which, func(t *testing.T) {
			w := newWorld(t, 0x0300)
			w.o.discoverCompletionCU = true
			base, ip, cx := w.base, uint16(ipCUDiff), uint16(1)
			switch which {
			case "puts":
				w.v.mem[lin(base+segFText, ipFPuts+uint16(len(sigPutsEntry)-1))] ^= 1
			case "jemm-tail":
				w.v.mem[lin(base+segFJemm, 0x0090+uint16(len(sigJemm)-1))] ^= 1
			case "only-jemm":
				w.v.mem = map[uint32]uint8{}
				w.v.wsig(base+segFJemm, 0x0090, sigJemm, 0)
			case "wrong-CS":
				base += 0x1000
			case "return-only":
				ip = ipCUDiffRet
			case "CH-nonzero":
				cx = 0x0201
			}
			completionObserve(w, &completionReadView{fv: w.v}, 1000000, base, ip, cx)
			if _, ok := w.o.Base(); ok || w.o.cu.Entry().InProgress {
				t.Fatal("unqualified CU discovered STRIKE", which)
			}
		})
	}
}
func TestCompletionDiscoveryFailedCandidateThrottleAndForeignIsolation(t *testing.T) {
	w := newWorld(t, 0x0300)
	w.o.discoverCompletionCU = true
	v := &completionReadView{fv: w.v}
	bad := lin(w.base+segFGlyph, 0x0184)
	original := v.mem[bad]
	v.mem[bad] ^= 1
	completionObserve(w, v, 1000000, w.base, ipCUDiff, 1)
	if _, ok := w.o.Base(); ok || w.o.baseKnown || w.o.lastVerify != 0 {
		t.Fatal("unknown failure polluted established base")
	}
	v.mem[bad] = original
	v.reads = 0
	completionObserve(w, v, 1000000+flightReverifySteps-1, w.base, ipCUDiff, 1)
	if _, ok := w.o.Base(); ok || v.reads != 0 {
		t.Fatal("failed unknown candidate read before throttle", v.reads)
	}
	completionObserve(w, v, 1000000+flightReverifySteps, w.base, ipCUDiff, 1)
	completionKnown(t, w.o, w.base, 1000000+flightReverifySteps)
	if w.o.failedCompletionBase.valid {
		t.Fatal("success did not clear failed candidate")
	}
	// A JEMM-only foreign candidate must not damage an already established owner.
	w.o.visible = []FlightText{{Key: "visible", CatKey: "visible"}}
	w.o.pending = []FlightText{{Key: "pending"}}
	w.o.cands = []FlightText{{Key: "candidate"}}
	w.o.gen = 7
	before, last := w.o.cu.Entry(), w.o.lastVerify
	foreign := uint16(0x1400)
	v.wsig(foreign+segFJemm, 0x0090, sigJemm, 0)
	completionObserve(w, v, 1600000, foreign, ipCUDiff, 1)
	if b, ok := w.o.Base(); !ok || b != w.base || w.o.lastVerify != last || w.o.cu.Entry() != before || w.o.Generation() != 7 || len(w.o.visible) != 1 || len(w.o.pending) != 1 || len(w.o.cands) != 1 {
		t.Fatal("foreign failure poisoned valid base")
	}
	if !w.o.failedCompletionBase.valid || w.o.failedCompletionBase.base != foreign {
		t.Fatal("foreign failed candidate was not retained separately")
	}
	// A distinct fully signed base is tried immediately, not throttled by foreign.
	next := uint16(0x2100)
	installFlightSigs(v.fv, next)
	completionObserve(w, v, 1600001, next, ipCUDiff, 1)
	completionKnown(t, w.o, next, 1600001)
	if w.o.Generation() != 8 || len(w.o.visible) != 0 || len(w.o.pending) != 0 || len(w.o.cands) != 0 {
		t.Fatal("new base did not use original reset semantics")
	}
}
func TestCompletionDiscoveryEmptyVisibleRevocationAndRecovery(t *testing.T) {
	w := newWorld(t, 0x0300)
	w.o.discoverCompletionCU = true
	v := &completionReadView{fv: w.v}
	completionObserve(w, v, 1000000, w.base, ipCUDiff, 1)
	completionKnown(t, w.o, w.base, 1000000)
	bad := lin(w.base+segFText, ipFPuts)
	old := v.mem[bad]
	v.mem[bad] ^= 1
	// Even the duplicate-entry path must perform full validation before accepting it.
	completionObserve(w, v, 1000001, w.base, ipCUDiff, 1)
	if _, ok := w.o.Base(); ok {
		t.Fatal("empty visible set suppressed CU signature revocation")
	}
	if in, await := w.o.CUState(); in || await {
		t.Fatal("revocation retained CU state")
	}
	if w.o.Counts().EntriesDuplicate != 0 {
		t.Fatal("failed signature reached duplicate acceptance")
	}
	if w.o.lastVerify != 1000001 {
		t.Fatal("disabled owner did not record failed verify time")
	}
	v.mem[bad] = old
	v.reads = 0
	completionObserve(w, v, 1000001+flightReverifySteps-1, w.base, ipCUDiff, 1)
	if _, ok := w.o.Base(); ok || v.reads != 0 {
		t.Fatal("disabled same-base retry bypassed throttle", v.reads)
	}
	completionObserve(w, v, 1000001+flightReverifySteps, w.base, ipCUDiff, 1)
	completionKnown(t, w.o, w.base, 1000001+flightReverifySteps)
}
func TestCompletionDiscoveryOriginalCUStateMachine(t *testing.T) {
	w := newWorld(t, 0x0300)
	w.o.discoverCompletionCU = true
	v := &completionReadView{fv: w.v}
	completionObserve(w, v, 1000000, w.base, ipCUDiff, 1)
	completionObserve(w, v, 1000001, w.base, ipCUDiff, 1)
	if w.o.cu.Entry().Steps != 1000000 || w.o.Counts().EntriesDuplicate != 1 {
		t.Fatal("duplicate replaced original entry")
	}
	v.steps, v.cs, v.ip, v.ss, v.sp = 1000010, w.base+segFJemm, ipCUDiffRet, tSS, tCUSP+2
	w.o.Observe(v)
	if !w.o.cu.Entry().InProgress {
		t.Fatal("wrong return cleared original entry")
	}
	v.steps, v.sp = 1000011, tCUSP
	w.o.Observe(v)
	if !w.o.cu.Entry().Awaiting {
		t.Fatal("matching return was not accepted")
	}
	w.o.Frame(nil, 1000012)
	if w.o.Counts().Promotions != 1 || len(w.o.Visible()) != 0 || w.o.Generation() != 0 {
		t.Fatal("empty original promotion changed text semantics")
	}
	v.steps, v.ip, v.cx = 1000020, ipMU, 0
	w.o.Observe(v)
	if !w.o.VGAWindow() {
		t.Fatal("MU0 did not preserve original VGA flag")
	}
	completionObserve(w, v, 1000030, w.base, ipCUDiff, 0x0201)
	if !w.o.VGAWindow() || w.o.cu.Entry().InProgress {
		t.Fatal("CH-nonzero changed collections/CU")
	}
	completionObserve(w, v, 1000040, w.base, ipCUDiff, 1)
	if w.o.VGAWindow() {
		t.Fatal("new VGA CU did not clear MU0 state")
	}
	w.o.Frame(nil, 1000040+cuTimeoutSteps)
	if w.o.cu.Entry().InProgress || w.o.Counts().Timeouts != 1 {
		t.Fatal("timeout did not use original tracker")
	}
	completionObserve(w, v, 1000041+cuTimeoutSteps, w.base, ipCUDiff, 1)
	completionKnown(t, w.o, w.base, 1000041+cuTimeoutSteps)
}
