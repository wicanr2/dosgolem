package session

import (
	"errors"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestOwnerStartsSealedBootingWithoutOriginalOrInstalledDOS(t *testing.T) {
	o, err := New(Config{InitialScale: host.OutputScale2})
	if err != nil {
		t.Fatal(err)
	}
	if got := o.Status(); got.Phase != PhaseBooting || got.FirstFault != nil || got.CloseError != nil {
		t.Fatalf("new status = %+v, want clean Booting", got)
	}
}

func TestOwnerFirstDrawFaultClosesExactlyOnceAndLatchesCause(t *testing.T) {
	o := newTestOwner(t, nil)
	first := errors.New("first draw fault")
	if err := o.ReportDrawFault(first); !errors.Is(err, first) {
		t.Fatalf("first ReportDrawFault = %v, want first fault", err)
	}
	if got := o.Status(); got.Phase != PhaseFailed || !errors.Is(got.FirstFault, first) {
		t.Fatalf("after first fault = %+v, want Failed with first cause", got)
	}
	second := errors.New("later draw fault")
	if err := o.ReportDrawFault(second); !errors.Is(err, first) || errors.Is(err, second) {
		t.Fatalf("later ReportDrawFault = %v, want only first fault", err)
	}
	if err := o.Close(); !errors.Is(err, nil) {
		t.Fatalf("repeated Close = %v, want nil", err)
	}
	if o.closer.(*countingCloser).calls != 1 {
		t.Fatalf("close calls = %d, want 1", o.closer.(*countingCloser).calls)
	}
}

func TestOwnerCloseClosesAnOwnedOSResourceAndIsIdempotent(t *testing.T) {
	// internal/dos.DOS.Close has no exported handle-count observer.  This test
	// therefore verifies the owner invokes a real Close exactly once without
	// claiming it is a DOS handle receipt; production wires dosCloser directly.
	f, err := os.CreateTemp(t.TempDir(), "session-close-*")
	if err != nil {
		t.Fatal(err)
	}
	o := newTestOwner(t, f)
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed resource Write error = %v, want os.ErrClosed", err)
	}
	if err := o.Close(); err != nil {
		t.Fatalf("second Close = %v, want nil", err)
	}
	if got := o.Status(); got.Phase != PhaseClosed || got.FirstFault != nil || got.CloseError != nil {
		t.Fatalf("after Close = %+v, want clean Closed", got)
	}
}

func TestOwnerPreservesCloseErrorWithoutReplacingFirstFault(t *testing.T) {
	closeErr := errors.New("close failed")
	o := newTestOwner(t, &countingCloser{err: closeErr})
	fault := errors.New("draw failed")
	err := o.ReportDrawFault(fault)
	if !errors.Is(err, fault) || !errors.Is(err, closeErr) {
		t.Fatalf("ReportDrawFault = %v, want joined fault and close error", err)
	}
	got := o.Status()
	if got.Phase != PhaseFailed || !errors.Is(got.FirstFault, fault) || !errors.Is(got.CloseError, closeErr) {
		t.Fatalf("status = %+v, want preserved fault and close error", got)
	}
	if err := o.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("repeated Close = %v, want close error", err)
	}
	if o.closer.(*countingCloser).calls != 1 {
		t.Fatalf("close calls = %d, want 1", o.closer.(*countingCloser).calls)
	}
}

func TestOwnerFailsClosedForNilFault(t *testing.T) {
	o := newTestOwner(t, nil)
	if err := o.ReportDrawFault(nil); err == nil {
		t.Fatal("nil draw fault unexpectedly accepted")
	}
	if got := o.Status(); got.Phase != PhaseFailed || got.FirstFault == nil {
		t.Fatalf("nil draw fault status = %+v, want failed with concrete cause", got)
	}

}

func TestOwnerClosedRejectsLaterFaultWithoutRevivalOrReclose(t *testing.T) {
	closer := &countingCloser{}
	o := newTestOwner(t, closer)
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	late := errors.New("late draw fault")
	if err := o.ReportDrawFault(late); !errors.Is(err, late) {
		t.Fatalf("closed ReportDrawFault = %v, want rejection", err)
	}
	if got := o.Status(); got.Phase != PhaseClosed || got.FirstFault != nil {
		t.Fatalf("closed status = %+v, want unchanged Closed", got)
	}
	if closer.calls != 1 {
		t.Fatalf("close calls = %d, want 1", closer.calls)
	}
}

func TestOwnerAdvanceSyntheticCOMRecordsMachineDeltaAndBudgetStop(t *testing.T) {
	o := startSyntheticOwner(t, []byte{0x90, 0x90, 0x90})
	if epoch, err := o.acceptTurn(); err != nil || epoch != 1 {
		t.Fatalf("acceptTurn = (%d, %v), want (1, nil)", epoch, err)
	}
	receipt, err := o.Advance(2)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Epoch != 1 || receipt.Budget != 2 ||
		receipt.MachineStepsBefore != 0 || receipt.MachineStepsAfter != 2 || receipt.Steps != 2 ||
		!receipt.HasRawStop || receipt.RawStop != machine.StopBudget || receipt.RawError != nil ||
		receipt.Phase != PhaseRunning || receipt.Reason != StopReasonBudgetExhausted {
		t.Fatalf("receipt = %+v, want epoch 1, 2-step budget receipt", receipt)
	}
}

func TestOwnerAdvanceSyntheticCOMClassifiesDOSExitBeforeRawBudget(t *testing.T) {
	// mov ax,4c03h / int 21h: the raw loop result is StopBudget, but DOS has
	// already observed a normal program exit.
	o := startSyntheticOwner(t, []byte{0xB8, 0x03, 0x4C, 0xCD, 0x21})
	if _, err := o.acceptTurn(); err != nil {
		t.Fatal(err)
	}
	receipt, err := o.Advance(2)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.HasRawStop || receipt.RawStop != machine.StopBudget || receipt.RawError != nil ||
		receipt.Steps != 2 || receipt.Reason != StopReasonProgramStopped || receipt.Phase != PhaseStopped {
		t.Fatalf("exit receipt = %+v, want ProgramStopped despite raw StopBudget", receipt)
	}
	if !o.dos.Exited || o.dos.ExitCode != 3 {
		t.Fatalf("DOS exit = (%t, %d), want (true, 3)", o.dos.Exited, o.dos.ExitCode)
	}
	again, err := o.Advance(9)
	if err != nil || again.Steps != 0 || again.HasRawStop || again.Reason != StopReasonProgramStopped || again.Phase != PhaseStopped {
		t.Fatalf("stopped Advance = (%+v, %v), want zero-step ProgramStopped", again, err)
	}
}

func TestOwnerAdvanceSyntheticCOMPreservesRawMachineErrorOverStopBudget(t *testing.T) {
	// 63h is unsupported by the default 8086 model.  Machine.Step counts the
	// failed attempt, while RunUntil still reports raw StopBudget with an error.
	o := startSyntheticOwner(t, []byte{0x90, 0x63})
	closer := &countingCloser{}
	o.closer = closer
	if _, err := o.acceptTurn(); err != nil {
		t.Fatal(err)
	}
	receipt, err := o.Advance(8)
	if err == nil || receipt.RawError == nil {
		t.Fatalf("Advance = (%+v, %v), want raw machine error", receipt, err)
	}
	if !errors.Is(err, receipt.RawError) || !receipt.HasRawStop || receipt.RawStop != machine.StopBudget ||
		receipt.Steps != 2 || receipt.Reason != StopReasonOriginalFault || receipt.Phase != PhaseFailed {
		t.Fatalf("error receipt = (%+v, %v), want raw error priority and two attempts", receipt, err)
	}
	if closer.calls != 1 {
		t.Fatalf("close calls = %d, want 1", closer.calls)
	}
	again, againErr := o.Advance(1)
	if againErr == nil || again.Steps != 0 || again.Reason != StopReasonOriginalFault || again.Phase != PhaseFailed {
		t.Fatalf("failed Advance = (%+v, %v), want zero-step latched original fault", again, againErr)
	}
}

func TestOwnerAdvanceRejectsZeroBudgetBeforeMachineStep(t *testing.T) {
	o := startSyntheticOwner(t, []byte{0x90})
	if _, err := o.acceptTurn(); err != nil {
		t.Fatal(err)
	}
	receipt, err := o.Advance(0)
	if err == nil || receipt.Steps != 0 || receipt.HasRawStop ||
		receipt.Reason != StopReasonFrontendFault || receipt.Phase != PhaseFailed || o.machine.Steps != 0 {
		t.Fatalf("zero-budget Advance = (%+v, %v), want failed pre-machine rejection", receipt, err)
	}
}

func TestOwnerDeliverPanelBatchesPauseAndQuarantineKeyboard(t *testing.T) {
	o := startSyntheticOwner(t, []byte{0x90, 0x90, 0x90, 0x90, 0x90})
	enter := dos.Key{Scan: 0x1C, ASCII: 0x0D}
	escape := dos.Key{Scan: 0x01, ASCII: 0x1B}

	r := deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventOpen}}, []dos.Key{enter})
	if r.Epoch != 1 || !r.HostChanged || !r.Paused || r.DOSCallsCommitted != 0 || o.dos.KeysPending() != 0 {
		t.Fatalf("Open receipt = %+v, BIOS=%d", r, o.dos.KeysPending())
	}
	consumePausedTurn(t, o, 1)

	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}}, []dos.Key{escape})
	if r.Epoch != 2 || !r.Paused || r.DOSCallsCommitted != 0 || o.dos.KeysPending() != 0 {
		t.Fatalf("Select receipt = %+v, BIOS=%d", r, o.dos.KeysPending())
	}
	consumePausedTurn(t, o, 2)

	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventCancel}}, []dos.Key{enter})
	if r.Epoch != 3 || !r.Paused || r.DOSCallsCommitted != 0 || o.dos.KeysPending() != 0 {
		t.Fatalf("Cancel receipt = %+v, BIOS=%d", r, o.dos.KeysPending())
	}
	consumePausedTurn(t, o, 3)
	state := ownerPanel(t, o)
	if state.Open || state.Scales.ActiveScale != host.OutputScale2 || state.Scales.SelectedScale != host.OutputScale2 {
		t.Fatalf("Cancel panel state = %+v, want closed 2×", state)
	}

	r = deliverOwner(t, o, nil, []dos.Key{enter, escape})
	if r.Epoch != 4 || r.Paused || r.DOSCallsCommitted != 2 || o.dos.KeysPending() != 2 {
		t.Fatalf("closed-key receipt = %+v, BIOS=%d", r, o.dos.KeysPending())
	}
	if tick, err := o.Advance(1); err != nil || tick.Steps != 1 || tick.Reason != StopReasonBudgetExhausted {
		t.Fatalf("closed Advance = (%+v, %v), want one machine step", tick, err)
	}

	// Reopen, choose 3×, then Apply plus Enter.  Apply closes the panel but
	// is still a host-transition batch, so Enter remains host-owned.
	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventOpen}}, nil)
	if r.Epoch != 5 || !r.Paused {
		t.Fatalf("second Open receipt = %+v", r)
	}
	consumePausedTurn(t, o, 5)
	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}}, nil)
	if r.Epoch != 6 || !r.Paused {
		t.Fatalf("second Select receipt = %+v", r)
	}
	consumePausedTurn(t, o, 6)
	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventApply}}, []dos.Key{enter})
	if r.Epoch != 7 || !r.Paused || r.DOSCallsCommitted != 0 || o.dos.KeysPending() != 2 {
		t.Fatalf("Apply receipt = %+v, BIOS=%d", r, o.dos.KeysPending())
	}
	consumePausedTurn(t, o, 7)
	state = ownerPanel(t, o)
	if state.Open || state.Scales.ActiveScale != host.OutputScale3 || state.Scales.SelectedScale != host.OutputScale3 {
		t.Fatalf("Apply panel state = %+v, want closed 3×", state)
	}
}

func TestOwnerDeliverRejectsMixedPointerBeforeAnyDOSOrEpochEffect(t *testing.T) {
	o := startSyntheticOwner(t, []byte{0x90, 0x90})
	start := ownerPanel(t, o)
	pointer := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	receipt, err := o.Deliver(CapturedUpdate{
		StartedPanel: start,
		PanelEvents:  []host.PanelEvent{{Kind: host.PanelEventOpen}},
		PointerDown:  &pointer,
		BIOSKeys:     []dos.Key{{Scan: 0x1C, ASCII: 0x0D}},
	})
	if err == nil || receipt.Epoch != 0 || receipt.Phase != PhaseFailed ||
		o.dos.KeysPending() != 0 || len(o.dos.Mouse.Events) != 0 || o.machine.Steps != 0 {
		t.Fatalf("mixed Deliver = (%+v, %v), BIOS=%d mouse=%v steps=%d; want pre-commit failure only",
			receipt, err, o.dos.KeysPending(), o.dos.Mouse.Events, o.machine.Steps)
	}
	if after := ownerPanel(t, o); after != start {
		t.Fatalf("failed Deliver mutated panel: before=%+v after=%+v", start, after)
	}
}

func TestOwnerDeliverPointerAndFocusRoutesAgainstSealedLayout(t *testing.T) {
	down := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	up := host.MouseEvent{Kind: host.MouseEventUp, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}

	t.Run("cross-turn canvas Down then Up", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90, 0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		r, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down})
		if err != nil || r.Epoch != 1 || r.Paused || r.DOSCallsCommitted != 2 || !o.mouse.Pressed() || o.dos.Mouse.Buttons != 1 {
			t.Fatalf("Down Deliver = (%+v, %v), mouse=%+v", r, err, o.dos.Mouse)
		}
		consumeRunningTurn(t, o, 1)
		r, err = o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerUp: &up})
		if err != nil || r.Epoch != 2 || r.DOSCallsCommitted != 2 || o.mouse.Pressed() || o.dos.Mouse.Buttons != 0 {
			t.Fatalf("Up Deliver = (%+v, %v), mouse=%+v", r, err, o.dos.Mouse)
		}
	})

	t.Run("same-turn Down plus Up", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		r, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down, PointerUp: &up})
		if err != nil || r.Epoch != 1 || r.DOSCallsCommitted != 4 || o.mouse.Pressed() || o.dos.Mouse.Buttons != 0 {
			t.Fatalf("same-turn Deliver = (%+v, %v), mouse=%+v", r, err, o.dos.Mouse)
		}
	})

	t.Run("non-canvas Up and focus loss are release-only", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input func(*Owner) CapturedUpdate
		}{
			{"outside-Up", func(o *Owner) CapturedUpdate {
				outside := up
				outside.Target = host.MouseTargetOutside
				return CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerUp: &outside}
			}},
			{"focus-lost", func(o *Owner) CapturedUpdate {
				return CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, FocusLost: true}
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
				if _, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down}); err != nil {
					t.Fatal(err)
				}
				consumeRunningTurn(t, o, 1)
				r, err := o.Deliver(tc.input(o))
				if err != nil || r.Epoch != 2 || r.DOSCallsCommitted != 1 || o.mouse.Pressed() || o.dos.Mouse.Buttons != 0 {
					t.Fatalf("release Deliver = (%+v, %v), mouse=%+v", r, err, o.dos.Mouse)
				}
			})
		}
	})

	t.Run("open panel consumes pointer without DOS", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		openOwnerPanelForTest(t, o)
		r, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down})
		if err != nil || r.Epoch != 1 || !r.Paused || r.DOSCallsCommitted != 0 || o.mouse.Pressed() || !o.mouse.HostCaptured() || len(o.dos.Mouse.Events) != 0 {
			t.Fatalf("open-panel Deliver = (%+v, %v), mouse=%+v", r, err, o.dos.Mouse)
		}
		consumePausedTurn(t, o, 1)
	})
}

func TestOwnerDeliverRejectsStalePointerLayoutBeforeDOSAction(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	down := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	stale := sessionLayout(2, host.OutputScale2, false)
	receipt, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: stale, PointerDown: &down})
	if err == nil || receipt.Epoch != 0 || receipt.Phase != PhaseFailed || o.dos.KeysPending() != 0 || len(o.dos.Mouse.Events) != 0 || o.machine.Steps != 0 {
		t.Fatalf("stale layout Deliver = (%+v, %v), mouse=%+v steps=%d", receipt, err, o.dos.Mouse, o.machine.Steps)
	}
}

func TestOwnerDeliverRejectsPanelTransitionBeforeItCanStalePrivateLayout(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	start := ownerPanel(t, o)
	beforeMouse := o.mouse.Snapshot()
	receipt, err := o.Deliver(CapturedUpdate{
		StartedPanel: start,
		PanelEvents:  []host.PanelEvent{{Kind: host.PanelEventOpen}},
		BIOSKeys:     []dos.Key{{Scan: 0x1C, ASCII: 0x0D}},
	})
	if err == nil || receipt.Epoch != 0 || receipt.Phase != PhaseFailed ||
		o.dos.KeysPending() != 0 || len(o.dos.Mouse.Events) != 0 || o.machine.Steps != 0 {
		t.Fatalf("Open without layout projection = (%+v, %v), BIOS=%d mouse=%v steps=%d",
			receipt, err, o.dos.KeysPending(), o.dos.Mouse.Events, o.machine.Steps)
	}
	if after := ownerPanel(t, o); after != start || o.mouse.Snapshot() != beforeMouse || o.layout.PanelOpen {
		t.Fatalf("rejected Open left stale state: panel=%+v mouse=%+v layout=%+v", after, o.mouse.Snapshot(), o.layout)
	}
}

func TestOwnerDeliverRejectsExistingPanelLayoutDriftBeforePointerDOSAction(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	// Reproduce the old post-Open state without exercising Deliver: panel is
	// open while the owner and bridge still carry its old closed layout.
	if _, _, err := o.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	down := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	receipt, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down})
	if err == nil || receipt.Epoch != 0 || receipt.Phase != PhaseFailed ||
		o.dos.KeysPending() != 0 || len(o.dos.Mouse.Events) != 0 || o.machine.Steps != 0 {
		t.Fatalf("drifted panel pointer Deliver = (%+v, %v), BIOS=%d mouse=%v steps=%d",
			receipt, err, o.dos.KeysPending(), o.dos.Mouse.Events, o.machine.Steps)
	}
}

func TestOwnerDeliverRejectsConflictingHostHitAndCanvasDownBeforeDOSAction(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	down := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	receipt, err := o.Deliver(CapturedUpdate{
		StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down,
		PanelEvents: []host.PanelEvent{{Kind: host.PanelEventPointerHostHit}},
	})
	if err == nil || receipt.Epoch != 0 || receipt.Phase != PhaseFailed ||
		o.dos.KeysPending() != 0 || len(o.dos.Mouse.Events) != 0 || o.machine.Steps != 0 {
		t.Fatalf("conflicting host hit Deliver = (%+v, %v), BIOS=%d mouse=%v steps=%d",
			receipt, err, o.dos.KeysPending(), o.dos.Mouse.Events, o.machine.Steps)
	}
}

func TestOwnerDeliverFocusLostReleasesPressedMouseWithStaleCapturedLayout(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	down := host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: 200, Y: 100, Target: host.MouseTargetCanvas}
	if _, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &down}); err != nil {
		t.Fatal(err)
	}
	consumeRunningTurn(t, o, 1)
	stale := sessionLayout(2, host.OutputScale2, false)
	receipt, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: stale, FocusLost: true})
	if err != nil || receipt.Epoch != 2 || receipt.DOSCallsCommitted != 1 || o.mouse.Pressed() || o.dos.Mouse.Buttons != 0 {
		t.Fatalf("stale-layout FocusLost = (%+v, %v), mouse=%+v", receipt, err, o.dos.Mouse)
	}
}

func TestOwnerDeliverReceiptCountsCommittedCallsNotUnchangedMoveEvents(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	// Make the DOS callback log observable. The synthetic cursor begins at
	// (0,0), so this canvas point submits MoveMouse(0,0), which DOS correctly
	// suppresses, followed by the left-button press callback.
	o.dos.Mouse.Handler.Set = true
	o.dos.Mouse.Handler.Mask = dos.EvMove | dos.EvLeftDown
	downAtInitialPosition := host.MouseEvent{
		Kind: host.MouseEventDown, Button: 0, X: 0, Y: o.layout.ChromeHeight,
		Target: host.MouseTargetCanvas,
	}
	receipt, err := o.Deliver(CapturedUpdate{
		StartedPanel: ownerPanel(t, o), Layout: o.layout, PointerDown: &downAtInitialPosition,
	})
	if err != nil || receipt.DOSCallsCommitted != 2 {
		t.Fatalf("unchanged-position Down = (%+v, %v), want two committed calls", receipt, err)
	}
	if o.dos.Mouse.X != 0 || o.dos.Mouse.Y != 0 || len(o.dos.Mouse.Events) != 1 ||
		o.dos.Mouse.Events[0].Buttons != dos.EvLeftDown {
		t.Fatalf("unchanged MoveMouse must not claim a move event: mouse=%+v events=%+v", o.dos.Mouse, o.dos.Mouse.Events)
	}
}

type countingCloser struct {
	calls int
	err   error
}

func (c *countingCloser) Close() error {
	c.calls++
	return c.err
}

func newTestOwner(t *testing.T, closer resourceCloser) *Owner {
	t.Helper()
	o, err := New(Config{InitialScale: host.OutputScale2})
	if err != nil {
		t.Fatal(err)
	}
	if closer == nil {
		closer = &countingCloser{}
	}
	o.closer = closer
	return o
}

func startSyntheticOwner(t *testing.T, code []byte) *Owner {
	t.Helper()
	o := newTestOwner(t, nil)
	if err := o.machine.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	return o
}

func startSyntheticOwnerWithLayout(t *testing.T, code []byte, layout host.MouseLayout) *Owner {
	t.Helper()
	o, err := New(Config{InitialScale: layout.Scale, InitialLayout: layout})
	if err != nil {
		t.Fatal(err)
	}
	o.closer = &countingCloser{}
	if err := o.machine.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	return o
}

func sessionLayout(epoch uint64, scale host.OutputScale, open bool) host.MouseLayout {
	s := int(scale)
	chrome := 18 * s
	if open {
		chrome = 92 * s
	}
	return host.MouseLayout{Epoch: epoch, Scale: scale, ChromeHeight: chrome,
		Canvas: host.Canvas{Width: 320, Height: 200}, FrameWidth: 320 * s,
		FrameHeight: chrome + 200*s, PanelOpen: open}
}

func openOwnerPanelForTest(t *testing.T, o *Owner) {
	t.Helper()
	state, _, err := o.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen})
	if err != nil {
		t.Fatal(err)
	}
	next := sessionLayout(o.layout.Epoch+1, state.Scales.ActiveScale, state.Open)
	if err := o.mouse.ApplyLayout(next); err != nil {
		t.Fatal(err)
	}
	o.layout = next
}

func ownerPanel(t *testing.T, o *Owner) host.PanelState {
	t.Helper()
	state, err := o.panel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func deliverOwner(t *testing.T, o *Owner, events []host.PanelEvent, keys []dos.Key) InputReceipt {
	t.Helper()
	receipt, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), PanelEvents: events, BIOSKeys: keys})
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func consumePausedTurn(t *testing.T, o *Owner, epoch uint64) {
	t.Helper()
	receipt, err := o.Advance(0)
	if err != nil || receipt.Epoch != epoch || receipt.Steps != 0 || receipt.Reason != StopReasonPanelPaused || receipt.Phase != PhaseRunning {
		t.Fatalf("paused Advance = (%+v, %v), want epoch %d zero-step PanelPaused", receipt, err, epoch)
	}
}

func consumeRunningTurn(t *testing.T, o *Owner, epoch uint64) {
	t.Helper()
	receipt, err := o.Advance(1)
	if err != nil || receipt.Epoch != epoch || receipt.Steps != 1 || receipt.Reason != StopReasonBudgetExhausted || receipt.Phase != PhaseRunning {
		t.Fatalf("running Advance = (%+v, %v), want epoch %d one-step budget receipt", receipt, err, epoch)
	}
}
