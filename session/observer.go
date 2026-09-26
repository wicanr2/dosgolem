package session

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// StepObserver watches a sealed session one instruction at a time
// (docs/spec/238).  It only ever receives a read-only StepView and value-type
// video writes; it can neither mutate nor retain the owned machine.
type StepObserver interface {
	BeforeStep(v StepView) error
	VideoWrite(w machine.VideoWrite)
	// Frame is called on each vertical retrace with copies of the indexed
	// frame and palette; mutating them does not touch the machine.
	Frame(indexed []byte, palette [256][3]uint8)
}

// StepView is valid only inside the BeforeStep call it was passed to.  A
// retained view reads zero and makes the next BeforeStep an ObserverFault.
type StepView struct {
	o     *Owner
	token uint64
}

func (v StepView) live() bool {
	if v.o == nil {
		return false
	}
	if !v.o.viewOpen || v.o.viewToken != v.token {
		v.o.viewMisuse = true
		return false
	}
	return true
}

func (v StepView) reg(r int) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.R[r]
}

func (v StepView) seg(s int) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.Seg[s]
}

func (v StepView) Steps() uint64 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Steps
}
func (v StepView) CS() uint16 { return v.seg(cpu.CS) }
func (v StepView) DS() uint16 { return v.seg(cpu.DS) }
func (v StepView) ES() uint16 { return v.seg(cpu.ES) }
func (v StepView) SS() uint16 { return v.seg(cpu.SS) }
func (v StepView) IP() uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.IP
}
func (v StepView) AX() uint16 { return v.reg(cpu.AX) }
func (v StepView) BX() uint16 { return v.reg(cpu.BX) }
func (v StepView) CX() uint16 { return v.reg(cpu.CX) }
func (v StepView) DX() uint16 { return v.reg(cpu.DX) }
func (v StepView) SP() uint16 { return v.reg(cpu.SP) }
func (v StepView) BP() uint16 { return v.reg(cpu.BP) }
func (v StepView) SI() uint16 { return v.reg(cpu.SI) }
func (v StepView) DI() uint16 { return v.reg(cpu.DI) }
func (v StepView) Flags() uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.Flags
}

// Palette returns a copy of the current DAC palette.
func (v StepView) Palette() [256][3]uint8 {
	if !v.live() {
		return [256][3]uint8{}
	}
	return v.o.machine.Palette()
}

// Read8 and Read16 never have side effects (machine.Peek8).
func (v StepView) Read8(a uint32) uint8 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Peek8(a)
}
func (v StepView) Read16(a uint32) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Peek16(a)
}

// errProgramExited stops the observed loop at the first instruction after the
// program has exited.  It is not an error of the program or the observer.
var errProgramExited = errors.New("session: 程式已結束")

// runObserved is Advance's observer path.  Faults from the observer are
// returned as observerErr so Advance can classify them apart from original
// program faults.
func (o *Owner) runObserved(budget uint64) (stop machine.Stop, rawErr, observerErr error) {
	// videoPanic latches a panic raised inside a Step (video write or
	// frame callback); the Step completes and the next attempt stops.
	var videoPanic error
	o.machine.ObserveVideoWrites(func(w machine.VideoWrite) {
		if videoPanic != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				videoPanic = fmt.Errorf("session: 觀測器 VideoWrite panic：%v", r)
			}
		}()
		o.observer.VideoWrite(w)
	})
	o.machine.SetOnFrame(func() {
		if videoPanic != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				videoPanic = fmt.Errorf("session: 觀測器 Frame panic：%v", r)
			}
		}()
		o.observer.Frame(o.machine.Indexed(), o.machine.Palette())
	})
	defer func() {
		o.machine.ObserveVideoWrites(nil)
		o.machine.SetOnFrame(nil)
	}()
	inObserver := false
	observe := func(m *machine.Machine) error {
		// The receipt runner stops as soon as the program exits; without this
		// check the CPU would keep stepping undefined memory after exit.
		if o.dos.Exited {
			return errProgramExited
		}
		if videoPanic != nil {
			observerErr = videoPanic
			return videoPanic
		}
		if o.viewMisuse {
			observerErr = errors.New("session: 觀測器在回呼外使用了 StepView")
			return observerErr
		}
		o.viewToken++
		o.viewOpen = true
		inObserver = true
		e := o.observer.BeforeStep(StepView{o: o, token: o.viewToken})
		inObserver = false
		o.viewOpen = false
		if e != nil {
			observerErr = fmt.Errorf("session: 觀測器 BeforeStep：%w", e)
			return observerErr
		}
		return nil
	}
	// A BeforeStep panic happens before that instruction's Step, so unwinding
	// the whole loop here leaves the machine exactly where it was.  Recovering
	// once around the loop instead of once per step keeps the hot path cheap.
	func() {
		defer func() {
			if r := recover(); r != nil {
				if !inObserver {
					panic(r) // not the observer's panic: never reclassify it
				}
				o.viewOpen = false
				observerErr = fmt.Errorf("session: 觀測器 BeforeStep panic：%v", r)
				stop, rawErr = machine.StopBudget, observerErr
			}
		}()
		stop, rawErr = o.machine.RunUntilObserved(nil, budget, observe)
	}()
	if errors.Is(rawErr, errProgramExited) {
		// Advance classifies the exited DOS as ProgramStopped.
		rawErr = nil
	}
	if observerErr == nil && videoPanic != nil {
		observerErr = videoPanic
	}
	if observerErr == nil && o.viewMisuse {
		observerErr = errors.New("session: 觀測器在回呼外使用了 StepView")
	}
	if observerErr != nil && rawErr == observerErr {
		rawErr = nil
	}
	return stop, rawErr, observerErr
}
