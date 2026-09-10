package cpu386

import "testing"

func TestCarryFlagControlPreservesOtherState(t *testing.T) {
	for _, flags := range []uint32{0, 0xffffffff} {
		c := New(testBus{0xf8, 0xf9})
		c.EFlags = flags
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		before := c.R
		if err := c.Step(); err != nil || c.EFlags != flags&^CF || c.R != before {
			t.Fatalf("CLC不符：%v", err)
		}
		if err := c.Step(); err != nil || c.EFlags != flags|CF || c.R != before {
			t.Fatalf("STC不符：%v", err)
		}
	}
}
