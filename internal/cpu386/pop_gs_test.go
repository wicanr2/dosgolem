package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestPopGSUsesDwordStackAndLoadsSelector(t *testing.T) {
	mem := testBus(make([]byte, 96))
	copy(mem, []byte{0x0f, 0xa9})
	binary.LittleEndian.PutUint32(mem[48:52], 0xabcd0040)
	before := append([]byte(nil), mem...)
	c := New(mem)
	c.R[ESP], c.EFlags = 16, IF|CF|ZF
	c.Seg[SegSS], c.Seg[SegGS] = 0x28, 0x20
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
	c.SetDescriptor(0x40, Descriptor{Base: 0, Limit: 95})
	if err := c.Step(); err != nil || c.Seg[SegGS] != 0x40 || c.R[ESP] != 20 || c.EIP != 2 || c.EFlags != IF|CF|ZF || !bytes.Equal(mem, before) {
		t.Fatalf("POP GS=%X ESP=%X EIP=%X flags=%X err=%v", c.Seg[SegGS], c.R[ESP], c.EIP, c.EFlags, err)
	}
}

func TestPopGSFailuresPreserveStackAndSegment(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     []byte
		esp      uint32
		selector uint32
		limit    uint32
	}{
		{"selector未登錄", []byte{0x0f, 0xa9}, 16, 0x48, 63},
		{"SS越界", []byte{0x0f, 0xa9}, 16, 0x40, 18},
		{"ESP溢位", []byte{0x0f, 0xa9}, ^uint32(0) - 2, 0x40, 63},
		{"16位前綴", []byte{0x66, 0x0f, 0xa9}, 16, 0x40, 63},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 96))
			copy(mem, tc.code)
			binary.LittleEndian.PutUint32(mem[48:52], tc.selector)
			before := append([]byte(nil), mem...)
			c := New(mem)
			c.R[ESP], c.EFlags = tc.esp, IF|CF|ZF
			c.Seg[SegSS], c.Seg[SegGS] = 0x28, 0x20
			c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: tc.limit, Writable: true})
			c.SetDescriptor(0x40, Descriptor{Base: 0, Limit: 95})
			if err := c.Step(); err == nil || c.Seg[SegGS] != 0x20 || c.R[ESP] != tc.esp || c.EFlags != IF|CF|ZF || !bytes.Equal(mem, before) {
				t.Fatalf("POP GS 失敗未保留狀態: GS=%X ESP=%X flags=%X err=%v", c.Seg[SegGS], c.R[ESP], c.EFlags, err)
			}
		})
	}
}
