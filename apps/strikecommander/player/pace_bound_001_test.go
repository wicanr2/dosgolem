package player

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

// The oracle is the frozen pre-fix source in pace_original_oracle_001_test.go.
// Its identifiers alone are renamed; the checker verifies byte-for-byte reversal.
func TestPaceStableOriginal001(t *testing.T) {
	for _, hz := range []float64{20, 50, 59.94, 60, 144} {
		t.Run(fmt.Sprintf("%.2fHz", hz), func(t *testing.T) {
			var current Pace
			var original paceOriginal001
			dt := time.Duration(float64(time.Second) / hz)
			for i := 0; i < 3600; i++ {
				got := current.Step(dt, i == 0)
				want := original.Step(dt, i == 0)
				current.Record(got.DT, got.N)
				original.Record(want.DT, want.N)
				if got != PaceStep(want) || current.Ratio() != original.Ratio() || current.SlowSeconds() != original.SlowSeconds() {
					t.Fatalf("STABLE_ORIGINAL hz=%v update=%d got=%+v want=%+v", hz, i, got, want)
				}
				if got.Slow || got.Saturated {
					t.Fatalf("stable frequency misclassified: %+v", got)
				}
			}
		})
	}
}

func TestPaceStallOriginal001(t *testing.T) {
	var current Pace
	var original paceOriginal001
	for i := 0; i < 181; i++ {
		dt := time.Second / 60
		if i == 60 {
			dt = time.Second
		}
		got, want := current.Step(dt, i == 0), original.Step(dt, i == 0)
		current.Record(got.DT, got.N)
		original.Record(want.DT, want.N)
		if got != PaceStep(want) || current.Ratio() != original.Ratio() || current.SlowSeconds() != original.SlowSeconds() {
			t.Fatalf("STALL_ORIGINAL update=%d got=%+v want=%+v", i, got, want)
		}
		if i == 0 && (got.DT != 1.0/60 || got.N != 192500 || got.Cap != 385000 || got.Acc != 0) {
			t.Fatalf("first independent hand values: %+v", got)
		}
		if i == 60 && (got.DT != 1 || got.N != 385000 || got.Cap != 385000 || got.Acc != 385000 || !got.Saturated || got.Slow) {
			t.Fatalf("stall independent hand values: %+v", got)
		}
	}
}

func TestPaceClosedLoopBound001(t *testing.T) {
	// Fixed fixtures, never inferred host throughput. Each profile feeds its own N
	// into its next dt. All 12,000 updates run; the old-source control fails the
	// product bound, not the disposable prototype's 100M stop threshold.
	for _, rate := range []float64{2000000, 5000000, 10000000, 11550000, 15000000} {
		for _, overhead := range []float64{0, 0.002} {
			t.Run(fmt.Sprintf("rate%.0f_overhead%.3f", rate, overhead), func(t *testing.T) {
				var p Pace
				dt := time.Second / 60
				var maximum uint64
				for i := 0; i < 12000; i++ {
					s := p.Step(dt, i == 0)
					if s.N > 1155000 || s.Cap > 1155000 || s.Acc > 1155000 {
						t.Fatalf("PACE_FIXED_BOUND update=%d N=%d cap=%d acc=%v", i, s.N, s.Cap, s.Acc)
					}
					if s.N > s.Cap || s.Acc < 0 || s.Acc > float64(s.Cap) || math.IsNaN(s.Acc) || math.IsInf(s.Acc, 0) {
						t.Fatalf("pacing invariant update=%d %+v", i, s)
					}
					maximum = max(maximum, s.N)
					dt = time.Duration((float64(s.N)/rate + overhead) * float64(time.Second))
				}
				t.Logf("12000 updates complete; maximum N=%d", maximum)
			})
		}
	}
}

func TestPaceResumeExistingHistory001(t *testing.T) {
	var p Pace
	for i := 0; i < 60; i++ {
		s := p.Step(time.Second/60, i == 0)
		p.Record(s.DT, s.N)
	}
	for i := 0; i < 30; i++ {
		s := p.Step(time.Second, false)
		p.Record(s.DT, s.N)
	}
	trueCount := 0
	for _, saturated := range p.sat {
		if saturated {
			trueCount++
		}
	}
	if !p.Slow || p.Acc != 1155000 || p.clean != 0 || len(p.dt) != 60 || len(p.sat) != 60 || trueCount != 30 || len(p.measure) == 0 {
		t.Fatalf("resume requires real existing slow/debt/history: %+v", p)
	}
	dts := append([]float64(nil), p.dt...)
	sats := append([]bool(nil), p.sat...)
	measures := append([]measure(nil), p.measure...)
	ratio := p.Ratio()
	slowSeconds := p.SlowSeconds()
	r := p.Step(time.Hour, true)
	// DT is replaced before both median and debt accounting. The existing 30
	// one-second samples survive; median=(1+1/60)/2 gives F=5,871,250.
	if r.DT != 1.0/60 || r.N != 192500 || r.Acc != 0 || r.FrameSteps != 5871250 || r.Cap != 1155000 || r.Saturated || !r.Slow || p.clean != 1 {
		t.Fatalf("RESUME_EXISTING_HISTORY independent values: %+v clean=%d", r, p.clean)
	}
	wantDT := append(dts[1:], 1.0/60)
	wantSat := append(sats[1:], false)
	if !reflect.DeepEqual(p.dt, wantDT) || !reflect.DeepEqual(p.sat, wantSat) || !reflect.DeepEqual(p.measure, measures) || p.Ratio() != ratio {
		t.Fatal("resume must preserve dt/saturation/measurement history; only normal append/eviction applies")
	}
	if math.Abs(p.SlowSeconds()-(slowSeconds+1.0/60)) > 1e-10 {
		t.Fatal("resume counted paused hour in SlowSeconds")
	}
	for clean := 2; clean <= 120; clean++ {
		s := p.Step(time.Second/60, false)
		if s.Saturated || s.Acc < 0 || s.Acc >= 1 {
			t.Fatalf("unexpected recovery saturation/debt at clean %d: %+v", clean, s)
		}
		if clean < 120 && (!s.Slow || p.clean != clean) {
			t.Fatalf("Slow exited early at clean %d: %+v", clean, s)
		}
		if clean == 120 && (s.Slow || p.Slow || p.clean != 0) {
			t.Fatalf("Slow did not exit on 120th clean update: %+v clean=%d", s, p.clean)
		}
	}
}
