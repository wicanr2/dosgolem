package strikecommander

import (
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestFPendingLayoutRecoveryWithoutFallback(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			t.Run(fmt.Sprintf("fallback%v/sparse%v", fallback, sparse), func(t *testing.T) {
				r := newPresRig(t, 0)
				v := fBoot(r)
				e := dEvent(15)
				dEnter(r, "O.shape", 0xfc00, &e)
				r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
				if fallback {
					r.write(6, 9, 0x7777, 0x1111, machine.VWV86)
				}
				dReturn(r, "O.shape", 0xfc00)
				if !r.p.d.pending || r.p.needFallback != fallback || r.p.d.eraseGap {
					t.Fatal("fixture did not establish owned pending without gap")
				}
				// Direct mode change: no BIOS notification or fallback flag injection.
				v.l.Mode, r.v.mode = 3, 3
				r.so.Labels.OnClearArt(r.v.steps)
				r.v.steps++
				r.p.Observe(v)
				if r.p.d.mode13Known || !r.p.d.pending || r.p.needFallback != fallback || r.p.Current().Seq != 0 {
					t.Fatal("initial completion did not retain pending after discovering illegal layout")
				}
				pendingNoSpin(t, r, v, sparse)
				for retry := 0; retry < 2; retry++ {
					before := v.layoutCalls
					r.frame(nil)
					if !r.p.d.layoutCheckPending || r.p.nextWake() != 0 {
						t.Fatal("pending Frame did not request bounded layout confirmation")
					}
					r.v.steps++
					r.p.Observe(v)
					if v.layoutCalls != before+1 || r.p.d.layoutCheckPending {
						t.Fatal("illegal Frame must cause exactly one live query")
					}
					pendingNoSpin(t, r, v, sparse)
				}
				v.l.Mode, r.v.mode = 0x13, 0x13 // no ModeChange notification
				r.frame(nil)
				if r.p.nextWake() != 0 || r.p.Current().Seq != 0 {
					t.Fatal("restored layout was not deferred to a live observation")
				}
				r.v.steps++
				r.p.Observe(v)
				if !r.p.d.mode13Known || r.p.d.pending || r.p.needFallback || r.p.Current().Seq != 1 || v.intoCalls != 1 {
					t.Fatal("restored layout failed to capture exactly once", r.p.Current(), v.intoCalls)
				}
			})
		}
	}
}
