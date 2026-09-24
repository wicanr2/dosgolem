//go:build draft_observer_advance_seam

package buckrogers

// This excluded DRAFT is a synthetic stop-order receipt only. It neither
// installs Watcher nor adds a session.Owner API. In particular, it records the
// missing Oracle guard/hook loop instead of claiming raw observation is a
// Buck Rogers player-path equivalent.

import (
	"errors"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type draftAdvanceSeamReceipt struct {
	StepsBefore, StepsAfter uint64
	RawStop                 machine.Stop
	HasRawStop              bool
	RawError                error
	ObserverCalls           int
	PreExited, PreHalted    bool
}

type draftAdvanceSeam struct {
	m       *machine.Machine
	d       *dos.DOS
	observe machine.PreStepObserver
}

func newDraftAdvanceSeam(t *testing.T, code []byte, observe machine.PreStepObserver) (*draftAdvanceSeam, func()) {
	t.Helper()
	m := machine.New()
	if err := m.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, ".")
	d.Install()
	return &draftAdvanceSeam{m: m, d: d, observe: observe}, func() { d.Close() }
}

// run records the candidate owner ordering: budget validation, pre-existing
// DOS exit/HLT, then exactly the generic pre-step seam and raw machine result.
// It deliberately has no predicate, breakpoint, Oracle hook, stub, or guard.
func (r *draftAdvanceSeam) run(budget uint64) (receipt draftAdvanceSeamReceipt) {
	receipt = draftAdvanceSeamReceipt{StepsBefore: r.m.Steps}
	defer func() { receipt.StepsAfter = r.m.Steps }()
	if budget == 0 || r.d.Exited || r.m.CPU.Halted {
		receipt.PreExited, receipt.PreHalted = r.d.Exited, r.m.CPU.Halted
		return
	}
	observe := r.observe
	if observe != nil {
		original := observe
		observe = func(m *machine.Machine) error {
			receipt.ObserverCalls++
			return original(m)
		}
	}
	receipt.RawStop, receipt.RawError = r.m.RunUntilObserved(nil, budget, observe)
	receipt.HasRawStop = true
	return
}

func TestDraftObserverAdvanceSeamSyntheticStopExitFaultOrder(t *testing.T) {
	t.Run("budget observes before each attempted step", func(t *testing.T) {
		r, close := newDraftAdvanceSeam(t, []byte{0x90, 0x90, 0x90}, func(*machine.Machine) error { return nil })
		defer close()
		got := r.run(2)
		if !got.HasRawStop || got.RawStop != machine.StopBudget || got.RawError != nil || got.ObserverCalls != 2 || got.StepsAfter-got.StepsBefore != 2 {
			t.Fatalf("budget receipt = %+v", got)
		}
	})
	t.Run("exit flag coexists with raw budget; owner must classify", func(t *testing.T) {
		r, close := newDraftAdvanceSeam(t, []byte{0xB8, 0x09, 0x4C, 0xCD, 0x21}, func(*machine.Machine) error { return nil })
		defer close()
		got := r.run(2)
		if !got.HasRawStop || got.RawStop != machine.StopBudget || got.RawError != nil || !r.d.Exited || got.ObserverCalls != 2 || got.StepsAfter-got.StepsBefore != 2 {
			t.Fatalf("exit receipt = %+v exited=%t", got, r.d.Exited)
		}
	})
	t.Run("CPU error retains raw error after its observer", func(t *testing.T) {
		r, close := newDraftAdvanceSeam(t, []byte{0x63}, func(*machine.Machine) error { return nil })
		defer close()
		got := r.run(1)
		if !got.HasRawStop || got.RawStop != machine.StopBudget || got.RawError == nil || got.ObserverCalls != 1 || got.StepsAfter-got.StepsBefore != 1 {
			t.Fatalf("CPU-fault receipt = %+v", got)
		}
	})
	t.Run("observer fault prevents its first attempted step", func(t *testing.T) {
		fault := errors.New("injected observer fault")
		r, close := newDraftAdvanceSeam(t, []byte{0x90}, func(*machine.Machine) error { return fault })
		defer close()
		got := r.run(1)
		if !got.HasRawStop || got.RawStop != machine.StopBudget || !errors.Is(got.RawError, fault) || got.ObserverCalls != 1 || got.StepsAfter != got.StepsBefore {
			t.Fatalf("observer-fault receipt = %+v", got)
		}
	})
	t.Run("pre-existing exit and HLT suppress the seam", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			set  func(*draftAdvanceSeam)
		}{
			{"exit", func(r *draftAdvanceSeam) { r.d.Exited = true }},
			{"HLT", func(r *draftAdvanceSeam) { r.m.CPU.Halted = true }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r, close := newDraftAdvanceSeam(t, []byte{0x90}, func(*machine.Machine) error { return nil })
				defer close()
				tc.set(r)
				got := r.run(1)
				if got.HasRawStop || got.RawError != nil || got.ObserverCalls != 0 || got.StepsAfter != got.StepsBefore {
					t.Fatalf("pre-existing %s receipt = %+v", tc.name, got)
				}
			})
		}
	})
}

func TestDraftObserverAdvanceSeamDocumentsOracleGuardGap(t *testing.T) {
	r, close := newDraftAdvanceSeam(t, []byte{0x90}, nil)
	defer close()
	// Oracle.RunUntil rejects A0000..FFFFF before firing OnCall. The generic
	// seam has no guard parameter, so it observes and attempts an instruction
	// there. This is a counterexample to direct production reuse, not desired
	// session behavior.
	r.m.CPU.Seg[cpu.CS], r.m.CPU.IP = machine.VideoSeg, 0
	r.observe = func(*machine.Machine) error { return nil }
	got := r.run(1)
	if !got.HasRawStop || got.ObserverCalls != 1 || got.StepsAfter != got.StepsBefore+1 {
		t.Fatalf("raw guard-gap receipt = %+v", got)
	}
}
