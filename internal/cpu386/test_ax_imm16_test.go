package cpu386

import (
	"bytes"
	"testing"
)

func TestTESTAXImm16OriginalShape(t *testing.T) {
	mem := testBus{0x66, 0xa9, 0x89, 0xcf}
	before := append([]byte(nil), mem...)
	c := New(mem)
	c.R[EAX] = 0xabcd_bb00
	c.EFlags = 0x206
	if err := c.Step(); err != nil || c.EIP != 4 || c.R[EAX] != 0xabcd_bb00 || c.EFlags != 0x286 || !bytes.Equal(mem, before) {
		t.Fatalf("TEST AX,CF89 EIP=%X EAX=%X flags=%X err=%v", c.EIP, c.R[EAX], c.EFlags, err)
	}
}

func TestTESTAXImm16FlagsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		code      []byte
		eax       uint32
		wantFlags uint32
		wantErr   bool
	}{
		{"零結果", []byte{0x66, 0xa9, 0, 0}, 0x1234ffff, ZF | PF, false},
		{"奇偶清除", []byte{0x66, 0xa9, 1, 0}, 0x12340001, 0, false},
		{"立即數截短", []byte{0x66, 0xa9, 0x89}, 0x1234bb00, CF | OF | SF | AF, true},
		{"段覆寫拒絕", []byte{0x3e, 0x66, 0xa9, 0x89, 0xcf}, 0x1234bb00, CF | OF | SF | AF, true},
		{"repeat 拒絕", []byte{0xf3, 0x66, 0xa9, 0x89, 0xcf}, 0x1234bb00, CF | OF | SF | AF, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(tc.code)
			before := append([]byte(nil), mem...)
			c := New(mem)
			c.R[EAX], c.EFlags = tc.eax, CF|OF|SF|AF
			err := c.Step()
			if (err != nil) != tc.wantErr || c.R[EAX] != tc.eax || c.EFlags != tc.wantFlags || !bytes.Equal(mem, before) {
				t.Fatalf("TEST AX 立即數 EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
			}
		})
	}
}
