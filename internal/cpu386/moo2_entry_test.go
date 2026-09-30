package cpu386

import "testing"

func TestMOO2StartupByteSUBRegister(t *testing.T) {
	for _, tc := range []struct {
		code      []byte
		eax, ebx  uint32
		wantEAX   uint32
		wantFlags uint32
	}{
		{[]byte{0x28, 0xc0}, 0x1234567f, 0, 0x12345600, ZF | PF},
		{[]byte{0x28, 0xd8}, 0x12345601, 2, 0x123456ff, CF | SF | PF | AF},
	} {
		c := New(testBus(tc.code))
		c.R[EAX], c.R[EBX] = tc.eax, tc.ebx
		c.EFlags = IF | CF | ZF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != tc.wantEAX || c.EFlags&(CF|ZF|SF|PF|AF|OF) != tc.wantFlags || c.EFlags&IF == 0 || c.EIP != 2 {
			t.Fatalf("SUB byte 暫存器結果錯誤：EAX=%X flags=%X EIP=%X", c.R[EAX], c.EFlags, c.EIP)
		}
	}
	c := New(testBus{0x28, 0x00})
	if err := c.Step(); err == nil {
		t.Fatal("尚未支援的 SUB byte memory 形狀必須拒絕")
	}
}

func TestDSOverrideBeforeMOVEDXImmediate(t *testing.T) {
	c := New(testBus{0x3e, 0xba, 0x50, 0xa1, 0x1c, 0x00})
	c.R[EDX], c.EFlags = 0xdeadbeef, IF|CF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 6 || c.R[EDX] != 0x1ca150 || c.EFlags != IF|CF {
		t.Fatalf("DS 覆寫不應改變立即數 MOV：EIP=%X EDX=%X flags=%X", c.EIP, c.R[EDX], c.EFlags)
	}
}

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
