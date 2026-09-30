package cpu386

import (
	"bytes"
	"testing"
)

func TestCMPWordEBPDisplacementUsesSS(t *testing.T) {
	mem := testBus(make([]byte, 512))
	copy(mem, []byte{0x66, 0x3b, 0x4d, 0xce}) // CMP CX, word SS:[EBP-32h]
	mem[0x14e], mem[0x14f] = 1, 0
	mem[0x4e], mem[0x4f] = 0, 0 // DS 同偏移不同值，防止誤用 DS。
	before := append([]byte(nil), mem...)
	c := New(mem)
	c.Seg[SegSS], c.Seg[SegDS] = 0x30, 0x28
	c.SetDescriptor(0x30, Descriptor{Base: 0x100, Limit: 0xff, Writable: true})
	c.SetDescriptor(0x28, Descriptor{Base: 0, Limit: 0xff})
	c.R[EBP], c.R[ECX], c.EFlags = 0x80, 0xabcd0000, 0x246
	if err := c.Step(); err != nil || c.EIP != 4 || c.R[ECX] != 0xabcd0000 || c.R[EBP] != 0x80 || c.EFlags != 0x297 || !bytes.Equal(mem, before) {
		t.Fatalf("CMP CX,SS:[EBP-32h] EIP=%X ECX=%X EBP=%X flags=%X err=%v", c.EIP, c.R[ECX], c.R[EBP], c.EFlags, err)
	}
}

func TestCMPWordNonEBPMemoryUsesDS(t *testing.T) {
	mem := testBus(make([]byte, 512))
	copy(mem, []byte{0x66, 0x3b, 0x08}) // CMP CX, word DS:[EAX]
	mem[0xd0], mem[0xd1] = 2, 0
	mem[0x150], mem[0x151] = 0, 0
	before := append([]byte(nil), mem...)
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x28, 0x30
	c.SetDescriptor(0x28, Descriptor{Base: 0x80, Limit: 0xff})
	c.SetDescriptor(0x30, Descriptor{Base: 0x100, Limit: 0xff})
	c.R[EAX], c.R[ECX], c.EFlags = 0x50, 0x12340003, 0x297
	if err := c.Step(); err != nil || c.EIP != 3 || c.R[ECX] != 0x12340003 || c.EFlags != 0x202 || !bytes.Equal(mem, before) {
		t.Fatalf("CMP CX,DS:[EAX] EIP=%X ECX=%X flags=%X err=%v", c.EIP, c.R[ECX], c.EFlags, err)
	}
}

func TestCMPWordMemoryFailuresPreserveOperandsAndFlags(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     []byte
		ssLimit  uint32
		selector uint16
	}{
		{"SS邊界", []byte{0x66, 0x3b, 0x4d, 0xce}, 0x4e, 0x30},
		{"SS未登錄", []byte{0x66, 0x3b, 0x4d, 0xce}, 0xff, 0x40},
		{"位移截短", []byte{0x66, 0x3b, 0x4d}, 0xff, 0x30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 512))
			copy(mem, tc.code)
			mem[0x14e] = 1
			before := append([]byte(nil), mem...)
			if tc.name == "位移截短" {
				mem = testBus(tc.code)
				before = append([]byte(nil), mem...)
			}
			c := New(mem)
			c.Seg[SegSS] = tc.selector
			c.SetDescriptor(0x30, Descriptor{Base: 0x100, Limit: tc.ssLimit})
			c.R[EBP], c.R[ECX], c.EFlags = 0x80, 0xabcd0000, 0x246
			if err := c.Step(); err == nil || c.R[ECX] != 0xabcd0000 || c.R[EBP] != 0x80 || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
				t.Fatalf("CMP 失敗未保留狀態：ECX=%X EBP=%X flags=%X err=%v", c.R[ECX], c.R[EBP], c.EFlags, err)
			}
		})
	}
}
