package session

import (
	"errors"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/host"
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
