package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSSWordStore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   []byte
		offset uint32
	}{
		{"base", []byte{0x66, 0x36, 0x89, 0x07}, 0x20},
		{"positive", []byte{0x66, 0x36, 0x89, 0x57, 2}, 0x22},
		{"negative", []byte{0x36, 0x66, 0x89, 0x57, 0xfe}, 0x1e},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 0x200))
			copy(mem, tc.code)
			c := New(mem)
			c.Seg[SegDS] = 0x160
			c.Seg[SegSS] = 0x168
			c.SetDescriptor(0x160, Descriptor{Base: 0x40, Limit: 0x3f, Writable: true})
			c.SetDescriptor(0x168, Descriptor{Base: 0x100, Limit: 0x3f, Writable: true})
			c.R[EDI] = 0x20
			c.R[EAX] = 0xabcd1234
			c.R[EDX] = 0xffff1234
			c.EFlags = 0xed7
			regs, segs, flags := c.R, c.Seg, c.EFlags
			want := append([]byte(nil), mem...)
			binary.LittleEndian.PutUint16(want[0x100+tc.offset:], 0x1234)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(mem, want) || c.R != regs || c.Seg != segs || c.EFlags != flags || c.EIP != uint32(len(tc.code)) {
				t.Fatal("store changed width, segment, registers, flags or EIP")
			}
		})
	}
}
func TestSSWordStoreRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		code []byte
		desc *Descriptor
	}{
		{"missing", []byte{0x66, 0x36, 0x89, 7}, nil},
		{"readonly", []byte{0x66, 0x36, 0x89, 7}, &Descriptor{Base: 0x100, Limit: 0x3f}},
		{"cross_limit", []byte{0x66, 0x36, 0x89, 7}, &Descriptor{Base: 0x100, Limit: 0x20, Writable: true}},
		{"duplicate", []byte{0x66, 0x36, 0x36, 0x89, 7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"conflict", []byte{0x66, 0x26, 0x36, 0x89, 7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"dword", []byte{0x36, 0x89, 7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"register", []byte{0x66, 0x36, 0x89, 0xc7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"sib", []byte{0x66, 0x36, 0x89, 4, 0x24}, &Descriptor{Limit: 0xff, Writable: true}},
		{"absolute", []byte{0x66, 0x36, 0x89, 5, 0, 0, 0, 0}, &Descriptor{Limit: 0xff, Writable: true}},
		{"disp32", []byte{0x66, 0x36, 0x89, 0x87, 0, 0, 0, 0}, &Descriptor{Limit: 0xff, Writable: true}},
		{"repeat", []byte{0xf3, 0x66, 0x36, 0x89, 7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"other_opcode", []byte{0x66, 0x36, 0x8b, 7}, &Descriptor{Limit: 0xff, Writable: true}},
		{"es_store", []byte{0x66, 0x26, 0x89, 7}, &Descriptor{Limit: 0xff, Writable: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 0x200))
			copy(mem, tc.code)
			c := New(mem)
			c.Seg[SegSS] = 0x168
			if tc.desc != nil {
				c.SetDescriptor(0x168, *tc.desc)
			}
			c.R[EDI] = 0x20
			c.R[EAX] = 0x1234
			c.EFlags = 0xed7
			before := append([]byte(nil), mem...)
			regs, segs, flags := c.R, c.Seg, c.EFlags
			if err := c.Step(); err == nil {
				t.Fatal("unsupported store accepted")
			}
			if !bytes.Equal(before, mem) || c.R != regs || c.Seg != segs || c.EFlags != flags {
				t.Fatal("rejected instruction mutated data")
			}
		})
	}
}
