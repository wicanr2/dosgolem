package strikecommander

import (
	"github.com/wicanr2/dosgolem/internal/machine"
)

// Source draining uses the existing sparse deadline, prearmed even when empty.
// Thus it does not depend on FrameEvery, layout, pause or snapshot nextFrame.
// CPU.OpAddr is assigned only on CPU.Step entry (cpu/ops.go), after machine
// Steps++; IRQ/Deliver writes keep that latched origin. At a given Step there
// are at most eight contexts (458's three bits). Pool capacity follows this
// scheduled bound, not an arbitrary number of distinct addresses.
const sourceDrainEvery uint64 = 4096
const sourcePoolCells = 8 * (sourceDrainEvery + 1)

type pendingSourceWrite struct {
	Step, Count uint64
	CS, IP      uint16
	Ctx         machine.VideoWriteCtx
}
type sourceWriteState struct {
	rows      []pendingSourceWrite
	step      uint64
	cs, ip    uint16
	stepKnown bool
	mask      uint8
	indices   [8]int
	drainStep uint64
	armed     bool
}

func (p *Presenter) queueSourceWrite(w machine.VideoWrite) {
	s := &p.sourceWrites
	if !s.stepKnown || s.step != w.Step {
		s.step, s.cs, s.ip, s.stepKnown, s.mask = w.Step, w.CS, w.IP, true, 0
	} else if s.cs != w.CS || s.ip != w.IP {
		panic("strikecommander: VideoWrite OpAddr changed within one Step")
	}
	ctx := uint8(w.Ctx)
	if ctx >= 8 {
		panic("strikecommander: VideoWrite context exceeds 458 bits")
	}
	bit := uint8(1) << ctx
	if s.mask&bit != 0 {
		s.rows[s.indices[ctx]].Count++
		return
	}
	if len(s.rows) == cap(s.rows) {
		panic("strikecommander: source drain deadline/pool invariant violated")
	}
	s.indices[ctx] = len(s.rows)
	s.mask |= bit
	s.rows = append(s.rows, pendingSourceWrite{Step: w.Step, Count: 1, CS: w.CS, IP: w.IP, Ctx: w.Ctx})
}

// Map insertion and diagnostic allocation happen at observation/event/read
// boundaries, never on the VideoWrite call stack. No row is dropped or capped.
func (p *Presenter) drainSourceWrites() {
	for _, w := range p.sourceWrites.rows {
		p.fallbackTrigger(PresentFallbackTrigger{Kind: "write", Steps: w.Step, Last: w.Step, CS: w.CS, IP: w.IP, Ctx: w.Ctx, Count: w.Count})
		k := [2]uint16{w.CS, w.IP}
		if u := p.unattr[k]; u != nil {
			u.Count += int(w.Count)
			u.Last = w.Step
		} else {
			p.unattr[k] = &PresentUnattr{CS: w.CS, IP: w.IP, Ctx: w.Ctx, Count: int(w.Count), First: w.Step, Last: w.Step}
		}
	}
	p.sourceWrites.rows = p.sourceWrites.rows[:0]
	p.sourceWrites.stepKnown = false
	p.sourceWrites.mask = 0
}
func (p *Presenter) armSourceDrain(steps uint64) {
	p.sourceWrites.drainStep, p.sourceWrites.armed = steps, true
}
func (p *Presenter) observeSourceDrain(steps uint64) {
	if !p.sourceWrites.armed || steps >= p.sourceWrites.drainStep+sourceDrainEvery {
		p.drainSourceWrites()
		p.armSourceDrain(steps)
	}
}
func (p *Presenter) sourceDrainWake() uint64 {
	if !p.sourceWrites.armed {
		return 0
	}
	return p.sourceWrites.drainStep + sourceDrainEvery
}
