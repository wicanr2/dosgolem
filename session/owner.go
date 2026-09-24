// Package session owns one sealed DOS session lifecycle.
//
// It intentionally exposes phase and fault values only.  The machine, DOS,
// panel, and bridges stay private so an open frontend configuration cannot
// rebind one half of a turn to another target.
package session

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
)

// Phase is the monotonic lifecycle of one sealed session.
type Phase uint8

const (
	PhaseBooting Phase = iota + 1
	PhaseRunning
	PhaseStopped
	PhaseFailed
	PhaseClosed
)

// InstructionBudget is an explicit, finite instruction-attempt limit for one
// accepted running turn.  The caller owns the concrete limit; zero is never a
// valid running-turn budget.
type InstructionBudget uint64

// StopReason classifies the session-level result without discarding the raw
// machine stop code kept by TickReceipt.
type StopReason uint8

const (
	StopReasonUnknown StopReason = iota
	StopReasonPanelPaused
	StopReasonBudgetExhausted
	StopReasonProgramStopped
	StopReasonPredicateStopped
	StopReasonBreakpointStopped
	StopReasonOriginalFault
	StopReasonObserverFault
	StopReasonFrontendFault
)

// TickReceipt records one Advance attempt.  Steps is a machine Step-attempt
// delta, not a count of successfully executed instructions.  RawStop and
// RawError are deliberately retained for audit: in particular, RunUntil may
// return StopBudget together with a non-nil error.
type TickReceipt struct {
	Epoch              uint64
	Budget             InstructionBudget
	MachineStepsBefore uint64
	MachineStepsAfter  uint64
	Steps              uint64
	RawStop            machine.Stop
	HasRawStop         bool
	RawError           error
	Phase              Phase
	Reason             StopReason
}

// Config contains presentation-only construction choices.  In particular it
// does not accept an existing machine, DOS, panel, or bridge.
type Config struct {
	InitialScale host.OutputScale
}

// Status is a value-only lifecycle receipt.  It deliberately exposes neither
// an owned resource nor a mutable routing capability.
type Status struct {
	Phase      Phase
	FirstFault error
	CloseError error
}

// Owner privately owns the resources which must share a session lifetime.
// All methods are for the one frontend goroutine named by spec 019.
type Owner struct {
	machine  *machine.Machine
	dos      *dos.DOS
	panel    *host.PanelController
	mouse    *host.MouseBridge
	keyboard *presentation.KeyboardBridge
	closer   resourceCloser

	phase          Phase
	epoch          uint64
	turnPending    bool
	terminalReason StopReason
	firstFault     error
	closeErr       error
	closed         bool
}

// New creates a new sealed, synthetic-ready owner.  It neither loads an
// original executable nor accepts one; later boot work must remain within this
// owner rather than exposing these resources.
func New(cfg Config) (*Owner, error) {
	panel, err := host.NewPanelController(cfg.InitialScale)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	d := dos.New(m, "")
	// DOS.Install must follow LoadEXE.  This narrow owner has no original
	// boot yet, so Booting deliberately does not install DOS or permit steps.
	// A future private boot must LoadEXE, then Install, then enter Running.
	keepDOS := false
	defer func() {
		if !keepDOS {
			d.Close()
		}
	}()
	mouse, err := host.NewMouseBridge(d)
	if err != nil {
		return nil, err
	}
	keyboard, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, d)
	if err != nil {
		return nil, err
	}
	o, err := newOwner(m, d, panel, mouse, keyboard, dosCloser{dos: d})
	if err != nil {
		return nil, err
	}
	keepDOS = true
	return o, nil
}

// ReportDrawFault synchronously latches the first inspectable Draw fault.
// It is intentionally independent of any later Update call.
func (o *Owner) ReportDrawFault(err error) error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if err == nil {
		err = errors.New("session: ReportDrawFault 需要非 nil error")
	}
	if o.phase != PhaseFailed && o.phase != PhaseClosed {
		o.terminalReason = StopReasonFrontendFault
	}
	return o.fail(err)
}

// Advance consumes the one privately accepted running turn and returns an
// auditable machine receipt.  This is deliberately the synthetic/no-observer
// slice: bare Machine.RunUntil does not dispatch oracle.OnCall hooks.  Public
// input delivery and an observer-aware runner are intentionally not invented
// here; a later sealed Deliver implementation must perform the READY
// captured-update/prepare/commit contract before calling acceptTurn.
func (o *Owner) Advance(budget InstructionBudget) (TickReceipt, error) {
	if o == nil {
		return TickReceipt{Reason: StopReasonFrontendFault}, errors.New("session: Owner 不得為 nil")
	}
	receipt := TickReceipt{Epoch: o.epoch, Budget: budget, Phase: o.phase, Reason: o.terminalReason}
	switch o.phase {
	case PhaseStopped:
		receipt.Reason = StopReasonProgramStopped
		return receipt, nil
	case PhaseFailed:
		return receipt, errors.Join(o.firstFault, o.closeErr)
	case PhaseClosed:
		receipt.Reason = StopReasonFrontendFault
		return receipt, errors.Join(errors.New("session: 已關閉的 owner 不可 Advance"), o.closeErr)
	case PhaseBooting:
		return o.frontendFaultReceipt(receipt, errors.New("session: Booting owner 尚未進入 Running"))
	case PhaseRunning:
		// Continue below.
	default:
		return o.frontendFaultReceipt(receipt, errors.New("session: 未知 owner phase"))
	}
	if !o.turnPending {
		return o.frontendFaultReceipt(receipt, errors.New("session: Advance 前必須先由 Deliver 接納一個回合"))
	}
	o.turnPending = false
	if budget == 0 {
		return o.frontendFaultReceipt(receipt, errors.New("session: InstructionBudget 必須為正數"))
	}
	if o.dos.Exited {
		o.phase = PhaseStopped
		o.terminalReason = StopReasonProgramStopped
		receipt.Phase = o.phase
		receipt.Reason = o.terminalReason
		return receipt, nil
	}

	receipt.MachineStepsBefore = o.machine.Steps
	rawStop, rawErr := o.machine.RunUntil(nil, uint64(budget))
	receipt.HasRawStop = true
	receipt.RawStop = rawStop
	receipt.RawError = rawErr
	receipt.MachineStepsAfter = o.machine.Steps
	if receipt.MachineStepsAfter < receipt.MachineStepsBefore {
		return o.originalFaultReceipt(receipt, errors.New("session: Machine.Steps 不得倒退"))
	}
	receipt.Steps = receipt.MachineStepsAfter - receipt.MachineStepsBefore
	if receipt.Steps > uint64(budget) {
		return o.originalFaultReceipt(receipt, errors.New("session: Machine.Steps 差分超過 InstructionBudget"))
	}
	if rawErr != nil {
		return o.originalFaultReceipt(receipt, rawErr)
	}
	if o.dos.Exited {
		o.phase = PhaseStopped
		o.terminalReason = StopReasonProgramStopped
		receipt.Phase = o.phase
		receipt.Reason = o.terminalReason
		return receipt, nil
	}
	switch rawStop {
	case machine.StopBudget:
		if receipt.Steps == uint64(budget) {
			receipt.Phase = o.phase
			receipt.Reason = StopReasonBudgetExhausted
			return receipt, nil
		}
		return o.originalFaultReceipt(receipt, errors.New("session: StopBudget 未耗盡 InstructionBudget"))
	case machine.StopPredicate:
		// No observer/predicate contract is installed in this slice.  A future
		// owner may expose PredicateStopped only after that contract is READY.
		return o.originalFaultReceipt(receipt, errors.New("session: 未宣告的 machine predicate stop"))
	case machine.StopBreakpoint:
		return o.originalFaultReceipt(receipt, errors.New("session: 未宣告的 machine breakpoint stop"))
	default:
		return o.originalFaultReceipt(receipt, fmt.Errorf("session: 未知 machine stop %v", rawStop))
	}
}

// Close performs the owned resource close at most once.  A close after a
// fault preserves that first fault; a normal close moves directly to Closed.
func (o *Owner) Close() error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if !o.closed {
		o.closed = true
		if o.closer == nil {
			o.closeErr = errors.New("session: 缺少私有資源關閉器")
		} else {
			o.closeErr = o.closer.Close()
		}
		if o.phase != PhaseFailed {
			o.phase = PhaseClosed
		}
	}
	return o.closeErr
}

// Status returns immutable lifecycle state; it cannot be used to reach an
// owned machine, DOS, panel, or bridge.
func (o *Owner) Status() Status {
	if o == nil {
		return Status{}
	}
	return Status{Phase: o.phase, FirstFault: o.firstFault, CloseError: o.closeErr}
}

// startLoadedMachine is the private half of a future boot path.  Its caller
// must already have loaded the executable into o.machine; keeping it private
// prevents a frontend from supplying or retaining a mutable machine target.
func (o *Owner) startLoadedMachine() error {
	if o == nil {
		return errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseBooting {
		return o.fail(errors.New("session: 只有 Booting owner 可啟動"))
	}
	o.dos.Install()
	o.phase = PhaseRunning
	return nil
}

// acceptTurn is the minimal owner-only epoch gate.  It deliberately contains
// no made-up input routing: the future Deliver method must call it only after
// the complete captured-update prepare/commit operation succeeds.
func (o *Owner) acceptTurn() (uint64, error) {
	if o == nil {
		return 0, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseRunning {
		if o.phase == PhaseStopped || o.phase == PhaseClosed {
			return o.epoch, fmt.Errorf("session: phase %d 不可接納新回合", o.phase)
		}
		return o.epoch, o.fail(errors.New("session: 非 Running owner 不可接納新回合"))
	}
	if o.turnPending {
		return o.epoch, o.fail(errors.New("session: 前一回合尚未 Advance"))
	}
	if o.epoch == ^uint64(0) {
		return o.epoch, o.fail(errors.New("session: epoch 溢位"))
	}
	o.epoch++
	o.turnPending = true
	return o.epoch, nil
}

func (o *Owner) frontendFaultReceipt(receipt TickReceipt, cause error) (TickReceipt, error) {
	o.terminalReason = StopReasonFrontendFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	receipt.Reason = o.terminalReason
	return receipt, err
}

func (o *Owner) originalFaultReceipt(receipt TickReceipt, cause error) (TickReceipt, error) {
	o.terminalReason = StopReasonOriginalFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	receipt.Reason = o.terminalReason
	return receipt, err
}

func (o *Owner) fail(err error) error {
	if o.phase == PhaseFailed {
		return errors.Join(o.firstFault, o.closeErr)
	}
	if o.phase == PhaseClosed {
		return errors.Join(err, o.closeErr)
	}
	o.firstFault = err
	o.phase = PhaseFailed
	return errors.Join(err, o.Close())
}

type resourceCloser interface {
	Close() error
}

type dosCloser struct{ dos *dos.DOS }

func (c dosCloser) Close() error {
	if c.dos == nil {
		return errors.New("session: 私有 DOS 不得為 nil")
	}
	c.dos.Close()
	return nil
}

func newOwner(m *machine.Machine, d *dos.DOS, panel *host.PanelController, mouse *host.MouseBridge, keyboard *presentation.KeyboardBridge, closer resourceCloser) (*Owner, error) {
	if m == nil || d == nil || panel == nil || mouse == nil || keyboard == nil || closer == nil {
		return nil, errors.New("session: 封閉 owner 的私有資源不得為 nil")
	}
	return &Owner{machine: m, dos: d, panel: panel, mouse: mouse, keyboard: keyboard, closer: closer, phase: PhaseBooting}, nil
}
