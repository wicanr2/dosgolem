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

// CapturedUpdate is the value-only input captured for one future frontend
// update. This controlled slice accepts already-classified panel events,
// pointer events, and explicitly mapped BIOS keys. Panel transitions that
// require a new mouse layout fail before commit until the owner has a private
// layout-projection contract.
type CapturedUpdate struct {
	StartedPanel host.PanelState
	Layout       host.MouseLayout
	PanelEvents  []host.PanelEvent
	PointerDown  *host.MouseEvent
	PointerUp    *host.MouseEvent
	FocusLost    bool
	BIOSKeys     []dos.Key
}

// InputReceipt records a successfully accepted update. Epoch is an accepted
// batch ordinal, not a DOS step count. A paused panel batch still has one.
type InputReceipt struct {
	Epoch       uint64
	Phase       Phase
	HostChanged bool
	// DOSCallsCommitted counts committed MouseOutput and BIOS key calls, not
	// DOS mouse callbacks/events. In particular, a committed MoveMouse call
	// whose coordinate is unchanged intentionally produces no DOS move event.
	DOSCallsCommitted uint64
	Paused            bool
}

// Config contains value-only host construction choices. In particular it does
// not accept an existing machine, DOS, panel, or bridge.
type Config struct {
	InitialScale  host.OutputScale
	InitialLayout host.MouseLayout
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
	turnPaused     bool
	generation     uint64
	committing     bool
	layout         host.MouseLayout
	layoutSet      bool
	terminalReason StopReason
	firstFault     error
	closeErr       error
	closed         bool
}

// New creates a new sealed, synthetic-ready owner.  It neither loads an
// original executable nor accepts one; later boot work must remain within this
// owner rather than exposing these resources.
func New(cfg Config) (*Owner, error) {
	if cfg.InitialLayout.Epoch == 0 && cfg.InitialLayout != (host.MouseLayout{}) {
		return nil, errors.New("session: InitialLayout 非零時必須有 layout epoch")
	}
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
	if cfg.InitialLayout.Epoch != 0 {
		state, err := panel.Snapshot()
		if err != nil {
			return nil, err
		}
		if cfg.InitialLayout.Scale != state.Scales.ActiveScale || cfg.InitialLayout.PanelOpen != state.Open {
			return nil, errors.New("session: InitialLayout 與初始 panel 狀態不一致")
		}
		if err := mouse.ApplyLayout(cfg.InitialLayout); err != nil {
			return nil, err
		}
	}
	keyboard, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, d)
	if err != nil {
		return nil, err
	}
	o, err := newOwner(m, d, panel, mouse, keyboard, dosCloser{dos: d})
	if err != nil {
		return nil, err
	}
	if cfg.InitialLayout.Epoch != 0 {
		o.layout, o.layoutSet = cfg.InitialLayout, true
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

// Deliver accepts one value-only update through a pure prepare and exclusive
// commit.  It deliberately does not accept mutable bridges or a machine from
// its caller.  Pointer/focus transport uses the owner-private captured layout;
// a pointer batch that also changes panel layout remains fail-closed until an
// approved owner-private layout-projection contract exists.
func (o *Owner) Deliver(update CapturedUpdate) (InputReceipt, error) {
	if o == nil {
		return InputReceipt{Phase: PhaseFailed}, errors.New("session: Owner 不得為 nil")
	}
	receipt := InputReceipt{Epoch: o.epoch, Phase: o.phase}
	if o.committing {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: Commit 期間不得重入 Deliver"))
	}
	if o.phase != PhaseRunning {
		if o.phase == PhaseStopped || o.phase == PhaseClosed {
			return receipt, fmt.Errorf("session: phase %d 不可接納新回合", o.phase)
		}
		return o.frontendFaultInputReceipt(receipt, errors.New("session: 非 Running owner 不可 Deliver"))
	}
	if o.turnPending {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: 前一回合尚未 Advance"))
	}
	plan, err := o.prepare(update)
	if err != nil {
		return o.frontendFaultInputReceipt(receipt, err)
	}
	return o.commit(plan)
}

type deliveredPlan struct {
	owner       *Owner
	generation  uint64
	base, final host.PanelState
	baseMouse   host.MouseBridgeSnapshot
	finalMouse  host.MouseBridgeSnapshot
	layout      host.MouseLayout
	panelEvents []host.PanelEvent
	mouseEvents []preparedMouseEvent
	biosKeys    []dos.Key
	hostChanged bool
	paused      bool
}

type preparedMouseEvent struct {
	event       host.MouseEvent
	route       host.MouseRoute
	next        host.MouseBridgeSnapshot
	actionCount uint8
}

// prepare performs no Route, Handle, ApplyLayout, DeliverBIOSKey, DOS write,
// or machine step.  It only consumes immutable snapshots and pure planners.
func (o *Owner) prepare(update CapturedUpdate) (deliveredPlan, error) {
	if o.epoch == ^uint64(0) || o.generation == ^uint64(0) {
		return deliveredPlan{}, errors.New("session: epoch 或 generation 溢位")
	}
	if err := o.keyboard.ValidateBIOSForPanel(o.panel); err != nil {
		return deliveredPlan{}, err
	}
	base, err := o.panel.Snapshot()
	if err != nil {
		return deliveredPlan{}, err
	}
	if update.StartedPanel != base {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的起點 panel 與 owner 不一致")
	}
	plan := deliveredPlan{owner: o, generation: o.generation, base: base, final: base, baseMouse: o.mouse.Snapshot(), layout: o.layout}
	plan.finalMouse = plan.baseMouse
	if o.layoutSet && (o.layout.Scale != base.Scales.ActiveScale || o.layout.PanelOpen != base.Open ||
		!plan.baseMouse.HasCurrent || plan.baseMouse.Current != o.layout) {
		return deliveredPlan{}, errors.New("session: owner layout、mouse snapshot 與 panel 不一致")
	}
	hasPointer := update.PointerDown != nil || update.PointerUp != nil || update.FocusLost
	if hasPointer {
		if !o.layoutSet {
			return deliveredPlan{}, errors.New("session: pointer/focus Deliver 缺少 owner 私有 layout")
		}
		// MouseBridge deliberately permits FocusLost to release an existing
		// press even when its event carries an old layout. Down and Up do not
		// have that exception: their captured layout must still be current.
		if (update.PointerDown != nil || update.PointerUp != nil) && update.Layout != o.layout {
			return deliveredPlan{}, errors.New("session: pointer/focus CapturedUpdate 的 layout 與 owner 不一致")
		}
		if !plan.baseMouse.HasCurrent || plan.baseMouse.Current != o.layout {
			return deliveredPlan{}, errors.New("session: owner mouse snapshot 與私有 layout 不一致")
		}
		if update.PointerDown != nil {
			if update.PointerDown.Kind != host.MouseEventDown {
				return deliveredPlan{}, errors.New("session: PointerDown 必須是 MouseEventDown")
			}
			if err := planMouseEvent(&plan, update.Layout, *update.PointerDown); err != nil {
				return deliveredPlan{}, err
			}
		}
		if update.PointerUp != nil {
			if update.PointerUp.Kind != host.MouseEventUp {
				return deliveredPlan{}, errors.New("session: PointerUp 必須是 MouseEventUp")
			}
			if err := planMouseEvent(&plan, update.Layout, *update.PointerUp); err != nil {
				return deliveredPlan{}, err
			}
		}
		if update.FocusLost {
			if err := planMouseEvent(&plan, update.Layout, host.MouseEvent{Kind: host.MouseEventFocusLost}); err != nil {
				return deliveredPlan{}, err
			}
		}
	}
	if update.PointerDown != nil && update.PointerDown.Target == host.MouseTargetCanvas {
		for _, event := range update.PanelEvents {
			if event.Kind == host.PanelEventPointerHostHit {
				return deliveredPlan{}, errors.New("session: 同批 pointer host-hit 與 canvas Down 分類矛盾")
			}
		}
	}
	for _, event := range update.PanelEvents {
		next, route, err := host.PlanPanelRoute(plan.final, event)
		if err != nil {
			return deliveredPlan{}, err
		}
		if route.ForwardToDOS {
			return deliveredPlan{}, errors.New("session: PanelEventKeyboard 必須以 BIOSKeys 明示交付")
		}
		plan.final = next
		plan.panelEvents = append(plan.panelEvents, event)
		plan.hostChanged = plan.hostChanged || route.ConsumedByHost
	}
	if o.layoutSet && (plan.base.Open != plan.final.Open || plan.base.Scales.ActiveScale != plan.final.Scales.ActiveScale) {
		return deliveredPlan{}, errors.New("session: panel layout transition 尚缺 owner 私有 projection")
	}
	plan.paused = plan.base.Open || plan.final.Open || plan.hostChanged
	if !plan.paused {
		plan.biosKeys = append(plan.biosKeys, update.BIOSKeys...)
	}
	return plan, nil
}

func planMouseEvent(plan *deliveredPlan, layout host.MouseLayout, event host.MouseEvent) error {
	if event.Target != host.MouseTargetCanvas && event.Target != host.MouseTargetHost && event.Target != host.MouseTargetOutside && event.Kind != host.MouseEventFocusLost {
		return errors.New("session: pointer target 未分類")
	}
	next, err := host.PlanMouseRoute(plan.finalMouse, layout, event)
	if err != nil {
		return err
	}
	switch next.Route.Reason {
	case "stale-layout-epoch-rejected", "unknown-event-rejected", "non-left-rejected", "nil-bridge-rejected":
		return fmt.Errorf("session: pointer route 在 commit 前拒絕：%s", next.Route.Reason)
	}
	plan.mouseEvents = append(plan.mouseEvents, preparedMouseEvent{event: event, route: next.Route, next: next.Next, actionCount: next.ActionCount})
	plan.finalMouse = next.Next
	return nil
}

// commit rechecks every source before its first output. The owner never leaks
// panel, bridge, DOS, or machine mutators; after this check it executes only
// prevalidated mouse actions and explicit BIOS key pushes.
func (o *Owner) commit(plan deliveredPlan) (InputReceipt, error) {
	receipt := InputReceipt{Epoch: o.epoch, Phase: o.phase}
	if plan.owner != o || plan.generation != o.generation || o.committing {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: stale 或重入的 Deliver plan"))
	}
	current, err := o.panel.Snapshot()
	if err != nil || current != plan.base || o.mouse.Snapshot() != plan.baseMouse || (o.layoutSet && o.layout != plan.layout) {
		if err == nil {
			err = errors.New("session: Deliver plan 的 panel source 已漂移")
		}
		return o.frontendFaultInputReceipt(receipt, err)
	}
	if err := o.keyboard.ValidateBIOSForPanel(o.panel); err != nil {
		return o.frontendFaultInputReceipt(receipt, err)
	}
	o.committing = true
	defer func() { o.committing = false }()
	for _, prepared := range plan.mouseEvents {
		route := o.mouse.Handle(plan.layout, prepared.event)
		if route != prepared.route || o.mouse.Snapshot() != prepared.next {
			panic("session: prevalidated mouse commit 發生不可能的分歧")
		}
	}
	expected := plan.base
	for _, event := range plan.panelEvents {
		var planErr error
		expected, _, planErr = host.PlanPanelRoute(expected, event)
		if planErr != nil {
			return o.frontendFaultInputReceipt(receipt, planErr)
		}
		state, route, err := o.panel.Route(event)
		if err != nil || route.ForwardToDOS || state != expected {
			if err == nil {
				err = errors.New("session: 預檢的 panel commit 發生分歧")
			}
			return o.frontendFaultInputReceipt(receipt, err)
		}
	}
	final, err := o.panel.Snapshot()
	if err != nil || final != plan.final {
		if err == nil {
			err = errors.New("session: panel commit 結果與純 plan 不一致")
		}
		return o.frontendFaultInputReceipt(receipt, err)
	}
	for _, key := range plan.biosKeys {
		o.dos.PushKey(key)
	}
	o.epoch++
	o.generation++
	o.turnPending = true
	o.turnPaused = plan.paused
	keyCalls := uint64(len(plan.biosKeys))
	return InputReceipt{Epoch: o.epoch, Phase: o.phase, HostChanged: plan.hostChanged,
		DOSCallsCommitted: keyCalls + mouseActionCount(plan.mouseEvents), Paused: plan.paused}, nil
}

func mouseActionCount(events []preparedMouseEvent) uint64 {
	var count uint64
	for _, prepared := range events {
		count += uint64(prepared.actionCount)
	}
	return count
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
	if o.turnPaused {
		o.turnPaused = false
		receipt.Phase = o.phase
		receipt.Reason = StopReasonPanelPaused
		return receipt, nil
	}
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
	return o.acceptTurnWithPause(false)
}

func (o *Owner) acceptTurnWithPause(paused bool) (uint64, error) {
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
	o.turnPaused = paused
	return o.epoch, nil
}

func (o *Owner) frontendFaultInputReceipt(receipt InputReceipt, cause error) (InputReceipt, error) {
	o.terminalReason = StopReasonFrontendFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	return receipt, err
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
