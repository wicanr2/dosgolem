package cpu386

import "testing"

func TestSBBWordRegisterOriginalAndBorrow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		code      testBus
		eax, edx  uint32
		flags     uint32
		wantEAX   uint32
		wantFlags uint32
	}{
		{"原版CF零", testBus{0x66, 0x19, 0xc0}, 0x0600, 0, IF | AF | PF, 0, IF | ZF | PF},
		{"自減借位", testBus{0x66, 0x19, 0xc0}, 0x12340000, 0, IF | CF, 0x1234ffff, IF | CF | AF | SF | PF},
		{"方向及高半部", testBus{0x66, 0x19, 0xd0}, 0xbeef0005, 0x12340002, IF | CF, 0xbeef0002, IF},
		{"16位溢位", testBus{0x66, 0x19, 0xd0}, 0xabcd8000, 0x98767fff, IF | CF, 0xabcd0000, IF | OF | AF | ZF | PF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.code)
			c.R[EAX], c.R[EDX], c.EFlags = tc.eax, tc.edx, tc.flags
			if err := c.Step(); err != nil || c.EIP != 3 || c.R[EAX] != tc.wantEAX || c.R[EDX] != tc.edx || c.EFlags != tc.wantFlags {
				t.Fatalf("66 19 /r: EIP=%X EAX=%X EDX=%X flags=%X err=%v; want EAX=%X flags=%X", c.EIP, c.R[EAX], c.R[EDX], c.EFlags, err, tc.wantEAX, tc.wantFlags)
			}
		})
	}
}

func TestSBBWordRegisterUnsupportedShapes(t *testing.T) {
	for _, code := range []testBus{
		{0x66, 0x19, 0x00},       // 記憶體目的
		{0x66, 0x1b, 0xc0},       // 其他操作碼方向
		{0x3e, 0x66, 0x19, 0xc0}, // 段前綴
		{0xf3, 0x66, 0x19, 0xc0}, // repeat 前綴
	} {
		c := New(code)
		c.R[EAX], c.EFlags = 0x12340005, IF|CF
		if err := c.Step(); err == nil || c.R[EAX] != 0x12340005 || c.EFlags != IF|CF {
			t.Fatalf("未支援 SBB 形狀未拒絕：code=% X EAX=%X flags=%X err=%v", code, c.R[EAX], c.EFlags, err)
		}
	}
}
