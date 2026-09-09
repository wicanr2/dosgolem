package cpu386

import "testing"

func TestShiftCLCountsSignedAndFlags(t *testing.T) {
	for _, v := range []struct {
		mod        byte
		a, n, w, f uint32
	}{
		{0xfa, 0x80000001, 0, 0x80000001, IF | CF | OF},
		{0xfa, 0x80000001, 1, 0xc0000000, IF | CF | SF | PF},
		{0xfa, 0x80000001, 31, 0xffffffff, IF | OF | SF | PF},
		{0xfa, 0x80000001, 32, 0x80000001, IF | CF | OF},
		{0xfa, 0x80000001, 33, 0xc0000000, IF | CF | SF | PF},
		{0xea, 0x80000001, 1, 0x40000000, IF | CF | OF | PF},
		{0xe2, 0x40000000, 1, 0x80000000, IF | OF | SF | PF},
		{0xf9, 33, 33, 16, IF | CF},
	} {
		m := testBus([]byte{0xd3, v.mod})
		c := New(m)
		c.R[EDX] = v.a
		c.R[ECX] = v.n
		c.EFlags = IF | CF | OF
		if e := c.Step(); e != nil {
			t.Fatal(e)
		}
		if c.R[v.mod&7] != v.w || c.EFlags != v.f {
			t.Fatalf("mod=%x n=%d value=%x flags=%x", v.mod, v.n, c.R[v.mod&7], c.EFlags)
		}
	}
	for _, code := range [][]byte{{0x66, 0xd3, 0xfa}, {0xf3, 0xd3, 0xfa}, {0xd3, 0x3a}} {
		c := New(testBus(code))
		c.R[EDX] = 123
		c.R[ECX] = 1
		c.EFlags = IF | CF
		if c.Step() == nil || c.R[EDX] != 123 || c.EFlags != IF|CF {
			t.Fatal("未拒絕不支援形式")
		}
	}
}
