package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSUBMemoryImmediateWidthsAndSegments(t *testing.T) {
	for _, tc := range []struct {
		before, imm, want, flags uint32
		short                    bool
	}{
		{3000, 2280, 720, 0, false}, {0, 1, 0xffffffff, CF | SF | PF | AF, false},
		{0x80000000, 1, 0x7fffffff, OF | AF | PF, false}, {0, 0xff, 1, CF | AF, true},
		{12, 6, 6, PF, true},
	} {
		for _, mode := range []int{0, 1, 2} {
			mem := testBus(make([]byte, 192))
			op := byte(0x81)
			if tc.short {
				op = 0x83
			}
			code := []byte{op, 0x28}
			if mode == 1 {
				code = []byte{op, 0x2c, 0x24}
			}
			if mode == 2 {
				code = []byte{op, 0x6d, 0xfc}
			}
			if tc.short {
				code = append(code, byte(tc.imm))
			} else {
				code = binary.LittleEndian.AppendUint32(code, tc.imm)
			}
			copy(mem, code)
			c := New(mem)
			c.Seg[SegDS] = 0x28
			c.Seg[SegSS] = 0x30
			c.R[EAX] = 12
			c.R[ESP] = 12
			c.R[EBP] = 16
			c.EFlags = IF
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
			c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: 31, Writable: true})
			pos := 76
			if mode != 0 {
				pos = 140
			}
			binary.LittleEndian.PutUint32(mem[pos:], tc.before)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if binary.LittleEndian.Uint32(mem[pos:]) != tc.want || c.EFlags != IF|tc.flags {
				t.Fatalf("結果不符 mode=%d imm=%x got=%x flags=%x", mode, tc.imm, binary.LittleEndian.Uint32(mem[pos:]), c.EFlags)
			}
			for _, limit := range []uint32{13, 31} {
				c.EIP = 0
				c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: limit, Writable: false})
				c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: limit, Writable: false})
				before := append([]byte(nil), mem...)
				flags := c.EFlags
				if c.Step() == nil || !bytes.Equal(before, mem) || c.EFlags != flags {
					t.Fatal("失敗未保留資料／旗標")
				}
			}
		}
	}
}
