package r5asset

import (
	"os"
	"testing"
)

// Enabled only by an explicitly recorded disposable source overlay. The
// normal loader contains no fault switch, VM callback or injectable policy.
func TestInjectedWholeBatchLateFailureRollback(t *testing.T) {
	mode := os.Getenv("SC_R5_INJECTED")
	if mode == "" {
		t.Skip("explicit test-only failure overlay")
	}
	sourceRoot, _ := fixtureRoots(t, true, false)
	l, o, base := loaderRig(t, 0, sourceRoot, Limits{})
	e := l.Load()
	if e == nil {
		t.Fatal("injected failure published")
	}
	v, ok := e.(Error)
	if !ok {
		t.Fatal(e)
	}
	if mode == "late" && v.Code != Decode {
		t.Fatal("wrong late failure", e)
	}
	if mode == "close" && v.Code != Closed {
		t.Fatal("wrong Close failure", e)
	}
	if o.OwnedBytes() != base+l.limits.ControlBytes || l.Stats().Published || l.Stats().Loading {
		t.Fatal("late whole rollback", l.Stats())
	}
}
