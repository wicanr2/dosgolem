package cpu386

import "testing"

func TestANDWordRegisterSignedImmediate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		initial    uint32
		immediate  byte
		want, flag uint32
	}{
		{"清低兩位", 0xabcd1237, 0xfc, 0xabcd1234, 0},
		{"零值", 0xabcd0003, 0xfc, 0xabcd0000, ZF | PF},
		{"負結果", 0xabcdffff, 0x80, 0xabcdff80, SF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(testBus{0x66, 0x83, 0xe7, tc.immediate})
			c.R[EDI], c.R[EAX] = tc.initial, 0xdeadbeef
			c.EFlags = IF | CF | OF | AF
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.R[EDI] != tc.want || c.R[EAX] != 0xdeadbeef || c.EIP != 4 || c.EFlags != tc.flag|IF {
				t.Fatalf("AND word: EDI=%08X EAX=%08X EIP=%X flags=%X", c.R[EDI], c.R[EAX], c.EIP, c.EFlags)
			}
		})
	}
	c := New(testBus{0x26, 0x66, 0x83, 0xe7, 0xfc})
	c.R[EDI] = 0xabcd1237
	if err := c.Step(); err == nil || c.R[EDI] != 0xabcd1237 {
		t.Fatal("不支援的 ES 前綴應拒絕且不改目的暫存器")
	}
}
