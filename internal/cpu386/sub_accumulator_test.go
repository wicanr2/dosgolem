package cpu386

import "testing"

func TestSUBAccumulatorImmediateWidthsAndFlags(t *testing.T) {
	for _, v := range []struct {
		word                           bool
		before, immediate, want, flags uint32
	}{
		{false, 0x2000, 0xab0, 0x1550, PF},
		{false, 0, 1, 0xffffffff, CF | PF | AF | SF},
		{false, 0x80000000, 1, 0x7fffffff, OF | PF | AF},
		{false, 5, 5, 0, ZF | PF},
		{true, 0x12340000, 1, 0x1234ffff, CF | PF | AF | SF},
		{true, 0xabcd8000, 1, 0xabcd7fff, OF | PF | AF},
	} {
		code := testBus{0x2d, byte(v.immediate), byte(v.immediate >> 8)}
		if v.word {
			code = append(testBus{0x66}, code...)
		} else {
			code = append(code, byte(v.immediate>>16), byte(v.immediate>>24))
		}
		c := New(code)
		c.R[EAX] = v.before
		c.EFlags = IF | DF | CF | OF | SF | ZF | AF | PF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != v.want || c.EFlags != (v.flags|IF|DF) || c.EIP != uint32(len(code)) {
			t.Fatalf("SUB result=%X flags=%X eip=%X", c.R[EAX], c.EFlags, c.EIP)
		}
	}
}
