package buckrogers

import "testing"

// withShrinkLevels replaces the level table of spec 056 for one test and
// restores it afterwards.  Tests that use it must not run in parallel.
func withShrinkLevels(t *testing.T, levels ...shrinkSpec) {
	t.Helper()
	old := shrinkLevels
	shrinkLevels = levels
	t.Cleanup(func() { shrinkLevels = old })
}

// noShrink switches the shrink feature off: the layout steps down exactly as
// before spec 056.
func noShrink(t *testing.T) {
	t.Helper()
	withShrinkLevels(t)
}
