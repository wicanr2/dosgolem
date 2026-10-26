package player

import (
	"math"
	"sort"
	"time"
)

const baseOriginal001 uint64 = 192500
const stepsOriginal001 uint64 = baseOriginal001 * 60

// paceOriginal001 uses only caller-supplied monotonic deltas. Paused updates must not call Step.
type paceOriginal001 struct {
	Acc         float64
	Slow        bool
	dt          []float64
	sat         []bool
	clean       int
	paceOriginalMeasure001     []paceOriginalMeasure001
	slowSeconds float64
}
type paceOriginalMeasure001 struct {
	Seconds float64
	Steps   uint64
}
type paceOriginalStep001 struct {
	N, Cap, FrameSteps uint64
	Acc                float64
	Saturated, Slow    bool
	DT                 float64
}

func (p *paceOriginal001) Reset() { p.Acc = 0 }
func (p *paceOriginal001) Step(dt time.Duration, firstOrResumed bool) paceOriginalStep001 {
	seconds := dt.Seconds()
	if firstOrResumed {
		seconds = 1.0 / 60
		p.Acc = 0
	}
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	p.dt = append(p.dt, seconds)
	if len(p.dt) > 60 {
		p.dt = p.dt[1:]
	}
	d := append([]float64(nil), p.dt...)
	sort.Float64s(d)
	t := d[len(d)/2]
	if len(d)%2 == 0 {
		t = (d[len(d)/2-1] + t) / 2
	}
	f := uint64(math.Round(t * float64(stepsOriginal001)))
	base := max(baseOriginal001, f)
	cap := 2 * base
	if p.Slow {
		cap = base
		p.slowSeconds += seconds
	}
	p.Acc += seconds * float64(stepsOriginal001)
	n := cap
	if p.Acc < float64(cap) {
		n = uint64(math.Floor(p.Acc))
	}
	p.Acc -= float64(n)
	saturated := p.Acc >= float64(cap)
	p.Acc = math.Min(p.Acc, float64(cap))
	p.sat = append(p.sat, saturated)
	if len(p.sat) > 60 {
		p.sat = p.sat[1:]
	}
	if p.Slow {
		if saturated {
			p.clean = 0
		} else {
			p.clean++
		}
		if p.clean >= 120 {
			p.Slow = false
			p.clean = 0
		}
	} else {
		n := 0
		for _, b := range p.sat {
			if b {
				n++
			}
		}
		if n >= 30 {
			p.Slow = true
			p.clean = 0
		}
	}
	return paceOriginalStep001{n, cap, f, p.Acc, saturated, p.Slow, seconds}
}

// Record accepts actual successful step attempts, not requested budget.
func (p *paceOriginal001) Record(seconds float64, steps uint64) {
	p.paceOriginalMeasure001 = append(p.paceOriginalMeasure001, paceOriginalMeasure001{seconds, steps})
	total := 0.0
	for i := len(p.paceOriginalMeasure001) - 1; i >= 0; i-- {
		total += p.paceOriginalMeasure001[i].Seconds
		if total > 2 && i > 0 {
			p.paceOriginalMeasure001 = p.paceOriginalMeasure001[i:]
			break
		}
	}
}
func (p *paceOriginal001) Ratio() float64 {
	var sec float64
	var steps uint64
	for _, m := range p.paceOriginalMeasure001 {
		sec += m.Seconds
		steps += m.Steps
	}
	if sec == 0 {
		return 0
	}
	return float64(steps) / (sec * float64(stepsOriginal001))
}
func (p *paceOriginal001) SlowSeconds() float64 { return p.slowSeconds }
