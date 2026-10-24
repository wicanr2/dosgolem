package strikecommander

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestDDeferralEraseTimeout(t *testing.T) {
	for _, k := range []int{0, 15} {
		for _, ending := range []string{"frame", "draw"} {
			t.Run(fmt.Sprintf("k%d/%s", k, ending), func(t *testing.T) {
				r := newPresRig(t, 0)
				e := dEvent(k)
				dEnter(r, "O.shape", 0xfc00, &e)
				start := r.v.steps
				r.write(8, 3, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
				dReturn(r, "O.shape", 0xfc00)
				dEnter(r, "O.erase", 0xfc00, nil)
				r.write(8, 0, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
				dReturn(r, "O.erase", 0xfc00)
				if !r.p.d.eraseGap {
					t.Fatal("paused erase lost its gap")
				}
				r.frame(nil) // The deferral still owns the frame; it cannot flush yet.
				r.v.steps, r.v.cs, r.v.ip = start+200000, 0x9999, 0x1234
				r.p.Observe(r.v)
				if r.p.Current().Seq != 0 || r.p.d.mode != deferIdle || r.p.counts.ModeTimeout != 1 {
					t.Fatal("timeout captured erase gap", r.p.Current(), r.p.d.mode)
				}
				if ending == "frame" {
					r.frame(nil)
					if r.p.counts.CursorEraseFlush != 1 {
						t.Fatal("missing cursor flush")
					}
				} else {
					dEnter(r, "O.shape", 0xfc00, nil)
					r.write(8, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
					dReturn(r, "O.shape", 0xfc00)
				}
				if r.p.Current().Seq != 1 || r.p.d.eraseGap || r.p.d.pending {
					t.Fatal("ending did not capture once", r.p.Current())
				}
			})
		}
	}
}

func TestDVoidFirstFrameLifetime(t *testing.T) {
	for _, reason := range []ExemptReason{ExemptWindowWriting, ExemptInCU, ExemptDeferral, ExemptPendingCapture, ExemptEraseGapDeferred} {
		t.Run(fmt.Sprint(reason), func(t *testing.T) {
			r := newPresRig(t, 0)
			r.p.capture(r.v, PresRoutine, 0, 0, nil, nil, nil)
			r.p.flight = nil
			r.v.vga[12] ^= 1
			r.p.staleVoid = true
			switch reason {
			case ExemptWindowWriting:
				r.p.stack = []*presWindow{{wrote: true}}
			case ExemptInCU:
				r.p.cuOpen = true
			case ExemptDeferral:
				r.p.d.mode = deferK0
			case ExemptPendingCapture:
				r.p.stack, r.p.epWrote = []*presWindow{{}}, true
			case ExemptEraseGapDeferred:
				r.p.d.flushDeferred = true
			}
			r.p.FrameAt(100, r.v.vga, r.v.pal)
			if v := r.p.LastVerdict(); v.Reason != reason || r.p.staleVoid {
				t.Fatal("void survived its first Frame or changed priority", reason, v, r.p.staleVoid)
			}
			r.p.stack, r.p.epWrote, r.p.cuOpen = nil, false, false
			r.p.d.mode, r.p.d.flushDeferred = deferIdle, false
			r.p.FrameAt(101, r.v.vga, r.v.pal)
			if v := r.p.LastVerdict(); v.Class != VerdictStale || v.Diff != 1 {
				t.Fatal("second Frame got an unrelated void exemption", v)
			}
		})
	}
	// The classifier may update diagnostics, but it cannot consume semantic flags.
	r := newPresRig(t, 0)
	r.p.staleVoid = true
	r.p.verdictFrame(r.v.vga)
	if !r.p.staleVoid {
		t.Fatal("classifier consumed void flag")
	}
}

func TestDStaleRowsMatchVerdicts(t *testing.T) {
	r := newPresRig(t, 0)
	stale, after := 0, 0
	r.p.OnVerdict(func(v FrameVerdict) {
		if v.Class == VerdictStale {
			stale++
		}
		if v.Reason == ExemptAfterCUVoid {
			after++
		}
	})
	r.frame(nil)
	if len(r.p.stale) != 0 || r.p.counts.StaleNoImage != 1 {
		t.Fatal("NoImage became a stale row")
	}
	r.p.capture(r.v, PresRoutine, 0, 0, nil, nil, nil)
	r.p.staleVoid = true
	r.frame(nil)
	if len(r.p.stale) != 1 || r.p.stale[0].Class != "after-cu-void" || r.p.stale[0].Diff != 0 {
		t.Fatal("equal AfterCUVoid row missing", r.p.stale)
	}
	r.v.vga[42] ^= 1
	r.p.staleVoid = true
	r.frame(nil)
	r.frame(nil)
	rowsStale, rowsAfter := 0, 0
	for _, row := range r.p.stale {
		switch row.Class {
		case "stale":
			rowsStale++
		case "after-cu-void":
			rowsAfter++
		default:
			t.Fatal("unexpected stale row class", row.Class)
		}
	}
	if rowsStale != stale || rowsAfter != after || stale != 1 || after != 2 || r.p.counts.StaleAfterCUVoid != 2 {
		t.Fatal("verdict/row partition differs", rowsStale, stale, rowsAfter, after, r.p.counts)
	}
}

func dUnknownBases(r *presRig, name string) {
	r.so.Labels.baseKnown, r.so.Labels.baseOK, r.so.Flight.baseOK = false, false, false
	if name != "O.shape" {
		return
	}
	r.v.w8(tpBase+segText, ipPutsEntry, sigPutsEntry...)
	for _, row := range baseRows {
		b, wild := append([]int(nil), row.bytes...), 0
		for j := range b {
			if b[j] < 0 {
				b[j] = int((tpBase+segText)>>(8*wild)) & 255
				wild++
			}
		}
		r.v.w8(tpBase+row.seg, row.off, b...)
	}
}

func TestDUnknownReturnCSBranches(t *testing.T) {
	for _, name := range []string{"O.shape", "S.shape", "M.shape"} {
		for _, sparse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/sparse%v", name, sparse), func(t *testing.T) {
				r := newPresRig(t, 0)
				dUnknownBases(r, name)
				cs, _, ret, _ := dFrozenEntry(t, name)
				observeRet := func(atCS, sp uint16) {
					r.v.steps += 100
					r.v.cs, r.v.ip, r.v.ss, r.v.sp = atCS, ret, tSS, sp
					f := machine.StepFilter{IPs: [1024]uint64(r.so.TriggerIPs()), Wake: r.p.nextWake()}
					if sparse && !f.ShouldObserve(ret, r.v.steps) {
						t.Fatal("return filtered out")
					}
					r.p.Observe(r.v)
				}
				// Neither an open CS nor a recently returned CS: ignore diagnostics.
				observeRet(cs, 0xfc00)
				if r.p.counts.ReturnWithoutEntry != 0 {
					t.Fatal("unknown CS diagnosed")
				}
				dEnter(r, name, 0xfc00, nil)
				if len(r.p.stack) != 1 {
					t.Fatal("entry did not open", r.p.windows)
				}
				// Open CS authorizes the matching return even without an observer base.
				observeRet(cs, 0xfc00)
				if r.p.counts.Routines[name].Returned != 1 {
					t.Fatal("open CS rejected")
				}
				r.v.deliv++
				observeRet(cs, 0xfc00)
				if r.p.counts.ReturnDuplicateDelivered != 1 {
					t.Fatal("recent CS rejected")
				}
				observeRet(cs, 0xfb00)
				if r.p.counts.ReturnWithoutEntry != 1 {
					t.Fatal("recent CS lost orphan diagnostic")
				}
				observeRet(cs+1, 0xfa00)
				if r.p.counts.ReturnWithoutEntry != 1 {
					t.Fatal("unrecorded CS diagnosed")
				}
			})
		}
	}
}

func TestDEndingPendingWake(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		r := newPresRig(t, 0)
		fBoot(r) // establish the legal layout required by a physical capture
		e := dEvent(15)
		e.Steps = r.v.steps
		r.so.Labels.OnArt(e)
		r.write(7, 3, 0x9999, 0x1234, machine.VWV86)
		r.so.Labels.OnClearArt(r.v.steps + 1)
		if r.p.d.mode != deferIdle || !r.p.d.pending {
			t.Fatal("no ending pending state")
		}
		if got := r.p.nextWake(); got != 0 {
			t.Fatal("ending pending wake", got)
		}
		a := newSparseAudit(r.so)
		a.oracle(r.so, r.v.steps+1, 0x1234)
		if !strings.Contains(strings.Join(a.fails, "\n"), "Presenter 結束或游標待取像") {
			t.Fatal("independent pending oracle missed state", a.fails)
		}
		f := machine.StepFilter{Wake: r.p.nextWake()}
		r.v.steps++
		r.v.cs, r.v.ip = 0x9999, 0x1234
		if sparse && !f.ShouldObserve(r.v.ip, r.v.steps) {
			t.Fatal("pending filtered")
		}
		r.p.Observe(r.v)
		if r.p.Current().Seq != 1 || r.p.d.pending {
			t.Fatal("pending capture not consumed")
		}
	}
}
