package cpu386

import "testing"

func TestWordCMPAbsoluteFromMOO2Entry(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dx, source uint16
		flags      uint32
	}{
		{"less", 1, 2, CF | SF},
		{"equal", 2, 2, ZF},
		{"greater", 3, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 64))
			copy(mem, []byte{0x66, 0x3b, 0x15, 32, 0, 0, 0})
			mem[32], mem[33] = byte(tc.source), byte(tc.source>>8)
			c := New(mem)
			c.Seg[SegDS] = 0x160
			c.SetDescriptor(0x160, Descriptor{Limit: 63})
			c.R[EDX] = 0xabcd0000 | uint32(tc.dx)
			c.EFlags = IF | CF | ZF | SF | OF
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.EIP != 7 || c.R[EDX] != 0xabcd0000|uint32(tc.dx) || mem[32] != byte(tc.source) || c.EFlags&(CF|ZF|SF|OF) != tc.flags {
				t.Fatalf("CMP state: EIP=%X EDX=%X flags=%X source=%X", c.EIP, c.R[EDX], c.EFlags, mem[32])
			}
			c.EIP = 0
			c.SetDescriptor(0x160, Descriptor{Limit: 32})
			if err := c.Step(); err == nil {
				t.Fatal("descriptor 越界讀取未拒絕")
			}
		})
	}
}
