package cpu386

import "testing"

func TestByteRegisterANDADDAndAliases(t *testing.T) {
	for _, v := range []struct {
		code       []byte
		a, d, w, f uint32
	}{
		{[]byte{0x22, 0xc4}, 0x123456a5, 0, 0x12345604, IF},
		{[]byte{0x22, 0xc4}, 0x123400a5, 0, 0x12340000, IF | PF | ZF},
		{[]byte{0x02, 0xc2}, 0x123456ff, 1, 0x12345600, IF | CF | AF | PF | ZF},
		{[]byte{0x02, 0xc2}, 0x1234567f, 1, 0x12345680, IF | OF | AF | SF},
		{[]byte{0x02, 0xe0}, 0x12340102, 0, 0x12340302, IF | PF},
	} {
		c := New(testBus(v.code))
		c.R[EAX] = v.a
		c.R[EDX] = v.d
		c.EFlags = IF | CF | OF
		if e := c.Step(); e != nil {
			t.Fatal(e)
		}
		if c.R[EAX] != v.w || c.R[EDX] != v.d || c.EFlags != v.f {
			t.Fatalf("code=%x value=%x flags=%x", v.code, c.R[EAX], c.EFlags)
		}
	}
}
