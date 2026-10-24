package strikecommander

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func pendingAdvance(r *presRig, v *fView, sparse bool, target uint64) {
	r.v.cs, r.v.ip = 0x9999, 0x1234
	f := machine.StepFilter{IPs: [1024]uint64(r.so.TriggerIPs()), Wake: r.p.nextWake()}
	for r.v.steps < target {
		r.v.steps++
		if !sparse || f.ShouldObserve(r.v.ip, r.v.steps) {
			r.p.Observe(v)
			f.Wake = r.p.nextWake()
		}
	}
}

func pendingNoSpin(t *testing.T, r *presRig, v *fView, sparse bool) {
	t.Helper()
	if !r.p.d.pending || r.p.presentationWake() == 0 || r.p.nextWake() == 0 {
		t.Fatal("blocked pending must remain without immediate wake", r.p.d.pending, r.p.nextWake())
	}
	a := newSparseAudit(r.so)
	a.oracle(r.so, r.v.steps+1, 0x1234)
	if strings.Contains(strings.Join(a.fails, "\n"), "結束或游標待取像") {
		t.Fatal("independent pending oracle ignores capture blocker", a.fails)
	}
	layout, into, invalid := v.layoutCalls, v.intoCalls, r.p.counts.NonLinear13h
	pendingAdvance(r, v, sparse, r.v.steps+20)
	if v.layoutCalls != layout || v.intoCalls != into || r.p.counts.NonLinear13h != invalid {
		t.Fatal("blocked pending repeatedly attempted capture", v.layoutCalls-layout, v.intoCalls-into, r.p.counts.NonLinear13h-invalid)
	}
}

func TestFPendingCaptureBlockers(t *testing.T) {
	for _, scenario := range []string{"CU-ClearArt", "invalid-timeout"} {
		for _, sparse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/sparse%v", scenario, sparse), func(t *testing.T) {
				r := newPresRig(t, 0)
				v := fBoot(r)
				if scenario == "CU-ClearArt" {
					r.inject(r.text("GEAR", 10, 0xffff, qList))
					r.cuIn(ipCUDiff, 1, tCUSP)
					r.write(5, 9, 0x7777, 0x1111, machine.VWV86)
					r.frame(nil)
				} else {
					v.l.Mode, r.v.mode = 3, 3
					r.p.VideoModeChange(machine.ModeChange{Mode: 3, Step: r.v.steps})
				}
				e := dEvent(15)
				e.Steps = r.v.steps
				r.so.Labels.OnArt(e)
				if !r.p.d.pending || !r.p.f.absorbed {
					t.Fatal("request was not absorbed")
				}
				seq := r.p.Current().Seq
				if scenario == "CU-ClearArt" {
					r.so.Labels.OnClearArt(r.v.steps)
					r.v.steps++
					r.p.VideoWrite(machine.VideoWrite{Step: r.v.steps, CS: 0x7777, IP: 0x1111, Offset: 6, Ctx: machine.VWV86})
					r.p.Observe(v)
					if len(r.p.sourceWrites.rows) != 1 || !r.p.cuOpen || r.p.Current().Seq != seq {
						t.Fatal("blocked CU pending attempted capture/drained source")
					}
					pendingNoSpin(t, r, v, sparse)
					r.cuRet(ipCUDiffRet, tCUSP)
					if r.p.Current().Seq != seq {
						t.Fatal("return-step Frame boundary captured too early")
					}
					r.v.steps++
					r.p.Observe(v)
				} else {
					pendingAdvance(r, v, sparse, e.Steps+200000)
					if r.p.counts.ModeTimeout != 1 || r.p.Current().Seq != seq {
						t.Fatal("invalid timeout changed image or missed timeout")
					}
					pendingNoSpin(t, r, v, sparse)
					// F4: an invalid Frame still requests the next live layout query.
					r.frame(nil)
					if !r.p.d.layoutCheckPending || r.p.nextWake() != 0 {
						t.Fatal("invalid Frame lost its layout confirmation wake")
					}
					r.v.steps++
					r.p.Observe(v)
					pendingNoSpin(t, r, v, sparse)
					v.l.Mode, r.v.mode = 0x13, 0x13 // direct change, no BIOS notification
					r.frame(nil)
					if r.p.nextWake() != 0 {
						t.Fatal("corrected layout cannot be confirmed")
					}
					r.v.steps++
					r.p.Observe(v)
				}
				if r.p.d.pending || r.p.needFallback || r.p.Current().Seq != seq+1 {
					t.Fatal("safe completion did not capture exactly once", r.p.Current(), r.p.d.pending)
				}
			})
		}
	}
}
