package session

import (
	"errors"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/host"
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
