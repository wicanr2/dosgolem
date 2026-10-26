package player

import (
	"math"
	"sort"
	"time"
)

const BaseSteps uint64 = 192500
const StepsPerSecond uint64 = BaseSteps * 60

// Pace uses only caller-supplied monotonic deltas. Paused updates must not call Step.
type Pace struct {
	Acc         float64
	Slow        bool
	dt          []float64
	sat         []bool
	clean       int
	measure     []measure
	slowSeconds float64
}
type measure struct {
	Seconds float64
	Steps   uint64
}
type PaceStep struct {
	N, Cap, FrameSteps uint64
	Acc                float64
	Saturated, Slow    bool
	DT                 float64
}

func (p *Pace) Reset() { p.Acc = 0 }
func (p *Pace) Step(dt time.Duration, firstOrResumed bool) PaceStep {
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
	f := uint64(math.Round(t * float64(StepsPerSecond)))
	base := max(BaseSteps, f)
	cap := 2 * base
	if p.Slow {
		cap = base
		p.slowSeconds += seconds
	}
	p.Acc += seconds * float64(StepsPerSecond)
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
	return PaceStep{n, cap, f, p.Acc, saturated, p.Slow, seconds}
}

// Record accepts actual successful step attempts, not requested budget.
func (p *Pace) Record(seconds float64, steps uint64) {
	p.measure = append(p.measure, measure{seconds, steps})
	total := 0.0
	for i := len(p.measure) - 1; i >= 0; i-- {
		total += p.measure[i].Seconds
		if total > 2 && i > 0 {
			p.measure = p.measure[i:]
			break
		}
	}
}
func (p *Pace) Ratio() float64 {
	var sec float64
	var steps uint64
	for _, m := range p.measure {
		sec += m.Seconds
		steps += m.Steps
	}
	if sec == 0 {
		return 0
	}
	return float64(steps) / (sec * float64(StepsPerSecond))
}
func (p *Pace) SlowSeconds() float64 { return p.slowSeconds }
