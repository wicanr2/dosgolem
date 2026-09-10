package cpu386

import "testing"

func TestSUBWordImmediatePreservesUpperAndFlags(t *testing.T) {
	for _, v := range []struct {
		before      uint32
		imm         byte
		want, flags uint32
	}{
		{0xabcd0010, 7, 0xabcd0009, PF | AF}, {0x12340000, 1, 0x1234ffff, CF | PF | AF | SF},
		{0xabcd8000, 1, 0xabcd7fff, OF | PF | AF}, {0x12340000, 0xff, 0x12340001, CF | AF},
	} {
		c := New(testBus{0x66, 0x83, 0xef, v.imm})
		c.R[EDI] = v.before
		c.EFlags = IF
		if err := c.Step(); err != nil || c.R[EDI] != v.want || c.EFlags != IF|v.flags {
			t.Fatalf("word SUB=%X flags=%X err=%v", c.R[EDI], c.EFlags, err)
		}
	}
}
