package cpu386

import (
	"bytes"
	"errors"
	"testing"
)

func TestEnterLevelZeroFrameAndLeave(t *testing.T) {
	mem := testBus(make([]byte, 512))
	copy(mem, []byte{0xc8, 0xac, 0x00, 0x00, 0xc9})
	c := New(mem)
	c.Seg[SegSS] = 0x188
	c.SetDescriptor(0x188, Descriptor{Base: 0x80, Limit: 0x17f, Writable: true})
	c.R[ESP], c.R[EBP], c.EFlags = 0x100, 0x89abcdef, IF|CF|ZF
	if err := c.Step(); err != nil || c.EIP != 4 || c.R[EBP] != 0xfc || c.R[ESP] != 0x50 || c.EFlags != IF|CF|ZF || !bytes.Equal(mem[0x17c:0x180], []byte{0xef, 0xcd, 0xab, 0x89}) {
		t.Fatalf("ENTER frame: EIP=%X EBP=%X ESP=%X flags=%X stack=% X err=%v", c.EIP, c.R[EBP], c.R[ESP], c.EFlags, mem[0x17c:0x180], err)
	}
	if err := c.Step(); err != nil || c.EIP != 5 || c.R[EBP] != 0x89abcdef || c.R[ESP] != 0x100 || c.EFlags != IF|CF|ZF {
		t.Fatalf("LEAVE round-trip: EIP=%X EBP=%X ESP=%X flags=%X err=%v", c.EIP, c.R[EBP], c.R[ESP], c.EFlags, err)
	}
}

func TestEnterLevelZeroDifferentAllocationSize(t *testing.T) {
	mem := testBus(make([]byte, 256))
	copy(mem, []byte{0xc8, 0x10, 0x00, 0x00})
	c := New(mem)
	c.Seg[SegSS] = 0x188
	c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: 255, Writable: true})
	c.R[ESP], c.R[EBP], c.EFlags = 0x80, 0x12345678, IF|CF
	if err := c.Step(); err != nil || c.R[EBP] != 0x7c || c.R[ESP] != 0x6c || c.EFlags != IF|CF || !bytes.Equal(mem[0x7c:0x80], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("ENTER 16-byte frame: EBP=%X ESP=%X flags=%X stack=% X err=%v", c.R[EBP], c.R[ESP], c.EFlags, mem[0x7c:0x80], err)
	}
}

func TestEnterLevelZeroPreflightFailuresPreserveState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     []byte
		limit    uint32
		writable bool
	}{
		{"巢狀層級", []byte{0xc8, 0xac, 0x00, 0x01}, 511, true},
		{"16位前綴", []byte{0x66, 0xc8, 0xac, 0x00, 0x00}, 511, true},
		{"段前綴", []byte{0x3e, 0xc8, 0xac, 0x00, 0x00}, 511, true},
		{"repeat前綴", []byte{0xf3, 0xc8, 0xac, 0x00, 0x00}, 511, true},
		{"ESP下溢", []byte{0xc8, 0xff, 0x00, 0x00}, 511, true},
		{"SS不可寫", []byte{0xc8, 0xac, 0x00, 0x00}, 511, false},
		{"SS越界", []byte{0xc8, 0xac, 0x00, 0x00}, 0xfe, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 512))
			copy(mem, tc.code)
			before := append([]byte(nil), mem...)
			c := New(mem)
			c.Seg[SegSS] = 0x188
			c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: tc.limit, Writable: tc.writable})
			c.R[ESP], c.R[EBP], c.EFlags = 0x100, 0x12345678, IF|CF
			if err := c.Step(); err == nil || c.R[ESP] != 0x100 || c.R[EBP] != 0x12345678 || c.EFlags != IF|CF || !bytes.Equal(mem, before) {
				t.Fatalf("ENTER 未失敗即關閉：ESP=%X EBP=%X flags=%X err=%v", c.R[ESP], c.R[EBP], c.EFlags, err)
			}
		})
	}
}

type enterNoStackReadBus struct{ testBus }

func (b enterNoStackReadBus) Read8(addr uint32) (uint8, error) {
	if addr >= 0x50 {
		return 0, errors.New("ENTER must not read the stack")
	}
	return b.testBus.Read8(addr)
}

func TestEnterLevelZeroDoesNotReadStack(t *testing.T) {
	mem := testBus(make([]byte, 512))
	copy(mem, []byte{0xc8, 0xac, 0x00, 0x00})
	c := New(enterNoStackReadBus{mem})
	c.Seg[SegSS] = 0x188
	c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: 511, Writable: true})
	c.R[ESP], c.R[EBP], c.EFlags = 0x100, 0x12345678, IF|CF
	if err := c.Step(); err != nil || c.R[ESP] != 0x50 || c.R[EBP] != 0xfc || c.EFlags != IF|CF || !bytes.Equal(mem[0xfc:0x100], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("ENTER 額外讀取堆疊或未寫入舊 EBP：ESP=%X EBP=%X flags=%X stack=% X err=%v", c.R[ESP], c.R[EBP], c.EFlags, mem[0xfc:0x100], err)
	}
}
