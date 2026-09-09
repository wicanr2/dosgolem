package cpu386

import "testing"

func TestWordMULUpperHalvesAndAlias(t *testing.T) {
	for _, v := range []struct{ a, b, lo, hi uint32 }{{0, 123, 0, 0}, {0xffff, 0xffff, 1, 0xfffe}, {0x1234, 0x10, 0x2340, 1}, {12, 3, 36, 0}} {
		for _, reg := range []byte{EBX, EDX} {
			c := New(testBus([]byte{0x66, 0xf7, 0xe0 | reg}))
			c.R[EAX] = 0xaaaa0000 | v.a
			c.R[EDX] = 0xbbbb0000 | v.b
			c.R[EBX] = 0xcccc0000 | v.b
			c.EFlags = IF | ZF | CF | OF
			if e := c.Step(); e != nil {
				t.Fatal(e)
			}
			flags := uint32(IF | ZF)
			if v.hi != 0 {
				flags |= CF | OF
			}
			if c.R[EAX] != 0xaaaa0000|v.lo || c.R[EDX] != 0xbbbb0000|v.hi || c.R[EBX] != 0xcccc0000|v.b || c.EFlags != flags {
				t.Fatalf("結果或高位不符：%x %x flags=%x", c.R[EAX], c.R[EDX], c.EFlags)
			}
		}
	}
}
