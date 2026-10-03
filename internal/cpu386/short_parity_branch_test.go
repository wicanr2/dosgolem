package cpu386

import (
	"bytes"
	"testing"
)

func TestShortParityBranches(t *testing.T) {
	for _, tc := range []struct {
		name      string
		op, delta byte
		flags     uint32
		want      uint32
	}{
		{"JP_taken", 0x7a, 0x18, PF, 26},
		{"JP_fallthrough", 0x7a, 0x18, CF | ZF | SF | OF, 2},
		{"JNP_original", 0x7b, 0x18, CF | ZF | SF | OF, 26},
		{"JNP_fallthrough", 0x7b, 0x18, PF, 2},
		{"JP_negative", 0x7a, 0xfe, PF, 0},
		{"JNP_negative", 0x7b, 0xfe, 0, 0},
		{"JP_positive_limit", 0x7a, 0x7f, PF, 129},
		{"JNP_negative_limit_wrap", 0x7b, 0x80, 0, 0xffffff82},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, []byte{tc.op, tc.delta})
			c.EFlags = tc.flags | IF | DF | 2
			c.R[EAX] = 123
			c.FPUStack[0] = 7
			c.FPUDepth = 1
			before := *c
			source := append([]byte(nil), mem...)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.EIP != tc.want || c.EFlags != before.EFlags || c.R != before.R || c.FPUStack != before.FPUStack || c.FPUDepth != before.FPUDepth || !bytes.Equal(mem, source) {
				t.Fatalf("EIP=%08x want=%08x flags=%x", c.EIP, tc.want, c.EFlags)
			}
		})
	}
	mem, c := newFPU(t, []byte{0x7a, 0xf4})
	c.EIP = 32
	copy(mem[32:], []byte{0x7a, 0xf4})
	c.EFlags |= PF
	if err := c.Step(); err != nil || c.EIP != 22 {
		t.Fatalf("原JP負位移 EIP=%d err=%v", c.EIP, err)
	}
}

func TestShortParityBranchConsumesX87C2(t *testing.T) {
	for _, tc := range []struct {
		name  string
		angle float64
		want  uint32
	}{
		{"有限弧度", 0, 31},
		{"超範圍", 0x1p63, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFPU(t, []byte{0xd9, 0xff, 0xdf, 0xe0, 0x9e, 0x7b, 0x18})
			c.FPUStack[0] = tc.angle
			c.FPUDepth = 1
			c.FPUStatus = 0x0400
			for i := 0; i < 4; i++ {
				if err := c.Step(); err != nil {
					t.Fatalf("step%d: %v", i, err)
				}
			}
			if c.EIP != tc.want {
				t.Fatalf("EIP=%d want=%d status=%x flags=%x", c.EIP, tc.want, c.FPUStatus, c.EFlags)
			}
		})
	}
}

func TestShortParityBranchesRejectPrefixesAndTruncation(t *testing.T) {
	for _, op := range []byte{0x7a, 0x7b} {
		for _, prefix := range []byte{0x66, 0x26, 0xf2, 0xf3} {
			mem, c := newFPU(t, []byte{prefix, op, 0x18})
			c.EFlags = PF | IF | CF | 2
			flags := c.EFlags
			source := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || c.EFlags != flags || !bytes.Equal(mem, source) {
				t.Fatalf("prefix=%x op=%x err=%v", prefix, op, err)
			}
		}
		c := New(testBus([]byte{op}))
		c.EFlags = PF | IF | CF | 2
		flags := c.EFlags
		if err := c.Step(); err == nil || c.EFlags != flags {
			t.Fatalf("截斷 op=%x err=%v", op, err)
		}
	}
}
