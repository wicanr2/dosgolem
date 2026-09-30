package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestORMemoryImmediateOriginalAndSigned(t *testing.T) {
	mem := testBus(make([]byte, 192))
	copy(mem, []byte{0x83, 0x0e, 0x01, 0x83, 0x4d, 0xfc, 0x80})
	binary.LittleEndian.PutUint32(mem[76:80], 0x90)
	binary.LittleEndian.PutUint32(mem[140:144], 0x10)
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x28, 0x30
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
	c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: 31, Writable: true})
	c.R[ESI], c.R[EBP], c.EFlags = 12, 16, 0x297
	if err := c.Step(); err != nil || binary.LittleEndian.Uint32(mem[76:80]) != 0x91 || c.EFlags != 0x202 || c.EIP != 3 {
		t.Fatalf("原版形狀 OR 值=%X flags=%X EIP=%X err=%v", mem[76:80], c.EFlags, c.EIP, err)
	}
	if err := c.Step(); err != nil || binary.LittleEndian.Uint32(mem[140:144]) != 0xffffff90 || c.EFlags != 0x286 || c.EIP != 7 {
		t.Fatalf("SS sign extension OR 值=%X flags=%X EIP=%X err=%v", mem[140:144], c.EFlags, c.EIP, err)
	}
	if binary.LittleEndian.Uint32(mem[76:80]) != 0x91 || c.R[ESI] != 12 || c.R[EBP] != 16 {
		t.Fatal("非目的記憶體或暫存器被改動")
	}
}

func TestORMemoryImmediateSegmentFailuresPreserveState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit uint32
		write bool
	}{
		{"越界", 13, true},
		{"唯讀", 31, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 128))
			copy(mem, []byte{0x83, 0x0e, 0x01})
			binary.LittleEndian.PutUint32(mem[76:80], 0x90)
			before := append([]byte(nil), mem...)
			c := New(mem)
			c.Seg[SegDS], c.R[ESI], c.EFlags = 0x28, 12, 0x297
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: tc.limit, Writable: tc.write})
			if err := c.Step(); err == nil || c.EFlags != 0x297 || !bytes.Equal(mem, before) {
				t.Fatalf("段失敗後改變記憶體或旗標: %v", err)
			}
		})
	}
}
