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
// KeysPendingBefore/After bracket DOS keyboard-queue depth around the turn:
// early rejections report the unchanged snapshot, the stepping path reports
// the post-step depth, so input-consumption evidence never needs a second
// machine read.
type TickReceipt struct {
	Epoch              uint64
	Budget             InstructionBudget
	MachineStepsBefore uint64
	MachineStepsAfter  uint64
	Steps              uint64
	KeysPendingBefore  int
	KeysPendingAfter   int
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
	// SourceGeneration, StartedPanel, and Layout must come from one Owner.View
	// result. Zero is never a compatibility token: accepting it would permit a
	// same-value ABA source to bypass the sealed owner check.
	SourceGeneration uint64
	PanelEvents      []host.PanelEvent
	PointerDown      *host.MouseEvent
	PointerUp        *host.MouseEvent
	FocusLost        bool
	BIOSKeys         []dos.Key
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

// View is the immutable routing-input snapshot of a sealed Running owner. It
// contains neither an owned target nor a mutable bridge.  It is deliberately
// not a presentation-pixel snapshot.
type View struct {
	Phase            Phase
	Panel            host.PanelState
	Layout           host.MouseLayout
	Epoch            uint64
	SourceGeneration uint64
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
		if _, changed, err := host.ProjectPresentationLayout(cfg.InitialLayout, state); err != nil || changed {
			return nil, errors.New("session: InitialLayout 與正式呈現幾何不一致")
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
// prevalidated panel transitions then project and install their next private
// layout after the batch's pointer/focus portion has committed.
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
	panelEvents []preparedPanelEvent
	mouseEvents []preparedMouseEvent
	biosKeys    []dos.Key
	hostChanged bool
	paused      bool
}

type preparedPanelEvent struct {
	event         host.PanelEvent
	final         host.PanelState
	layout        host.MouseLayout
	layoutChanged bool
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
	if update.SourceGeneration == 0 || update.SourceGeneration != o.generation {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的 SourceGeneration 與 owner 不一致")
	}
	// Every batch's three capture values must be one View result, even when it
	// contains no pointer. FocusLost retains MouseBridge's release exception
	// for its event routing, but cannot use a separately stale batch layout.
	if update.Layout != o.layout {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的 layout 與 owner 不一致")
	}
	if err := o.keyboard.ValidateBIOSForPanel(o.panel); err != nil {
		return deliveredPlan{}, err
	}
	base, err := o.panel.Snapshot()
	if err != nil {
		return deliveredPlan{}, err
	}
	if !o.layoutSet {
		return deliveredPlan{}, errors.New("session: Running owner 缺少私有 layout")
	}
	canonical, changed, err := host.ProjectPresentationLayout(o.layout, base)
	if err != nil || changed || canonical != o.layout {
		if err == nil {
			err = errors.New("session: owner 私有 layout 未按目前 panel 正規化")
		}
		return deliveredPlan{}, err
	}
	baseMouse := o.mouse.Snapshot()
	if !baseMouse.HasCurrent || baseMouse.Current != o.layout {
		return deliveredPlan{}, errors.New("session: owner mouse snapshot 與私有 layout 不一致")
	}
	if update.StartedPanel != base {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的起點 panel 與 owner 不一致")
	}
	plan := deliveredPlan{owner: o, generation: o.generation, base: base, final: base, baseMouse: baseMouse, layout: o.layout}
	plan.finalMouse = plan.baseMouse
	hasPointer := update.PointerDown != nil || update.PointerUp != nil || update.FocusLost
	if hasPointer {
		// MouseBridge deliberately permits FocusLost to release an existing
		// press even after its internal press epoch has changed. The batch layout
		// itself was already required above to be the current View layout.
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
	if update.PointerDown != nil && update.PointerDown.Target != host.MouseTargetHost && len(update.PanelEvents) != 0 {
		return deliveredPlan{}, errors.New("session: 同批 panel route 與非 host Down 分類矛盾")
	}
	projectedLayout := plan.layout
	for _, event := range update.PanelEvents {
		next, route, err := host.PlanPanelRoute(plan.final, event)
		if err != nil {
			return deliveredPlan{}, err
		}
		if route.ForwardToDOS {
			return deliveredPlan{}, errors.New("session: PanelEventKeyboard 必須以 BIOSKeys 明示交付")
		}
		layout, layoutChanged := projectedLayout, false
		if o.layoutSet {
			layout, layoutChanged, err = host.ProjectPresentationLayout(projectedLayout, next)
			if err != nil {
				return deliveredPlan{}, err
			}
		}
		plan.final = next
		plan.panelEvents = append(plan.panelEvents, preparedPanelEvent{
			event: event, final: next, layout: layout, layoutChanged: layoutChanged,
		})
		if layoutChanged {
			projectedLayout = layout
		}
		plan.hostChanged = plan.hostChanged || route.ConsumedByHost
	}
	if o.layoutSet && projectedLayout != plan.layout {
		// A sealed owner always applies this value in commit after the matching
		// prevalidated panel route. It is intentionally private: callers only
		// bring the captured layout for their pointer event.
		plan.finalMouse.Current = projectedLayout
		plan.finalMouse.HasCurrent = true
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
	for _, prepared := range plan.panelEvents {
		state, route, err := o.panel.Route(prepared.event)
		if err != nil || route.ForwardToDOS || state != prepared.final {
			if err == nil {
				err = errors.New("session: 預檢的 panel commit 發生分歧")
			}
			return o.frontendFaultInputReceipt(receipt, err)
		}
		if prepared.layoutChanged {
			if err := o.mouse.ApplyLayout(prepared.layout); err != nil {
				panic("session: 預檢的 layout commit 發生不可能的分歧: " + err.Error())
			}
			o.layout = prepared.layout
			o.layoutSet = true
		}
	}
	final, err := o.panel.Snapshot()
	if err != nil || final != plan.final {
		if err == nil {
			err = errors.New("session: panel commit 結果與純 plan 不一致")
		}
		return o.frontendFaultInputReceipt(receipt, err)
	}
	if o.mouse.Snapshot() != plan.finalMouse || (o.layoutSet && o.layout != plan.finalMouse.Current) {
		panic("session: 預檢的 mouse/layout commit 發生不可能的分歧")
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
	receipt.KeysPendingBefore = o.dos.KeysPending()
	receipt.KeysPendingAfter = receipt.KeysPendingBefore
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
	if !o.turnPaused && budget == 0 {
		return o.frontendFaultReceipt(receipt, errors.New("session: InstructionBudget 必須為正數"))
	}
	if o.generation == ^uint64(0) {
		return o.frontendFaultReceipt(receipt, errors.New("session: Advance 前 SourceGeneration 溢位"))
	}
	o.turnPending = false
	if o.turnPaused {
		o.turnPaused = false
		o.generation++
		receipt.Phase = o.phase
		receipt.Reason = StopReasonPanelPaused
		return receipt, nil
	}
	if o.dos.Exited {
		o.phase = PhaseStopped
		o.terminalReason = StopReasonProgramStopped
		o.generation++
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
	receipt.KeysPendingAfter = o.dos.KeysPending()
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
		o.generation++
		receipt.Phase = o.phase
		receipt.Reason = o.terminalReason
		return receipt, nil
	}
	switch rawStop {
	case machine.StopBudget:
		if receipt.Steps == uint64(budget) {
			o.generation++
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

// View returns a value-only input-capture source. It is valid only while the
// owner is Running and its private layout is canonical for the current panel.
// A snapshot/layout failure is a synchronous owner fault; the returned View is
// always zero on error, so no stale partial source can be reused.
func (o *Owner) View() (View, error) {
	if o == nil {
		return View{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseRunning {
		return View{}, fmt.Errorf("session: phase %d 不可取得 View，請讀取 Status", o.phase)
	}
	if !o.layoutSet {
		return View{}, o.fail(errors.New("session: Running owner 缺少私有 layout"))
	}
	panel, err := o.panel.Snapshot()
	if err != nil {
		return View{}, o.fail(fmt.Errorf("session: 讀取 owner panel snapshot: %w", err))
	}
	canonical, changed, err := host.ProjectPresentationLayout(o.layout, panel)
	if err != nil || changed || canonical != o.layout {
		if err == nil {
			err = errors.New("session: owner 私有 layout 未按目前 panel 正規化")
		}
		return View{}, o.fail(err)
	}
	return View{Phase: o.phase, Panel: panel, Layout: o.layout, Epoch: o.epoch, SourceGeneration: o.generation}, nil
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
	if o.generation == ^uint64(0) {
		return o.fail(errors.New("session: 啟動前 SourceGeneration 溢位"))
	}
	o.dos.Install()
	o.phase = PhaseRunning
	o.generation++
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
	if o.epoch == ^uint64(0) || o.generation == ^uint64(0) {
		return o.epoch, o.fail(errors.New("session: epoch 或 SourceGeneration 溢位"))
	}
	o.epoch++
	o.turnPending = true
	o.turnPaused = paused
	o.generation++
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
