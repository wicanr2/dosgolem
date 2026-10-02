package cpu386

import "testing"

func independentWordSUBFlags(a, b uint16) uint32 {
	diff := int64(a) - int64(b)
	r := uint16(diff)
	flags := uint32(0)
	if diff < 0 {
		flags |= CF
	}
	if a&15 < b&15 {
		flags |= AF
	}
	if r == 0 {
		flags |= ZF
	}
	if r&0x8000 != 0 {
		flags |= SF
	}
	signed := int32(int16(a)) - int32(int16(b))
	if signed < -32768 || signed > 32767 {
		flags |= OF
	}
	ones := 0
	for bits := uint8(r); bits != 0; bits >>= 1 {
		ones += int(bits & 1)
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}
func TestSUBWordImm16AllOperandsAndIndependentFlags(t *testing.T) {
	mask := uint32(CF | PF | AF | ZF | SF | OF)
	for reg := 0; reg < 8; reg++ {
		bus := testBus{0x66, 0x81, byte(0xe8 + reg), 0x6c, 0x07, 0xa5}
		c := New(bus)
		for a := 0; a < 65536; a++ {
			c.EIP = 0
			c.R = [8]uint32{0x11112222, 0x33334444, 0x55556666, 0x77778888, 0x9999aaaa, 0xbbbbcccc, 0xddddeeee, 0xffff0000}
			c.R[reg] = 0xa5cd0000 | uint32(a)
			c.EFlags = 0x210f57
			c.Seg = [6]uint16{8, 0x10, 0x10, 0, 0x20, 0x10}
			r, seg, flags := c.R, c.Seg, c.EFlags
			want := r
			want[reg] = 0xa5cd0000 | uint32(uint16(int64(a)-1900))
			if err := c.Step(); err != nil || c.EIP != 5 || c.R != want || c.Seg != seg || c.EFlags != flags&^mask|independentWordSUBFlags(uint16(a), 1900) || bus[5] != 0xa5 {
				t.Fatalf("reg%d a=%X r=%X flags=%X err=%v", reg, a, c.R, c.EFlags, err)
			}
		}
		for _, a := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
			for _, b := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
				bus[3], bus[4] = byte(b), byte(b>>8)
				c.EIP = 0
				c.R[reg] = 0xa5cd0000 | uint32(a)
				c.EFlags = IF | DF
				if err := c.Step(); err != nil || c.R[reg] != 0xa5cd0000|uint32(a-b) || c.EFlags != (IF|DF|independentWordSUBFlags(a, b)) {
					t.Fatal("word邊界／完整iw", a, b, err)
				}
			}
		}
	}
}
func TestSUBWordImm16RejectsWithoutArchitecturalWrites(t *testing.T) {
	for _, code := range [][]byte{
		{0x66, 0x81}, {0x66, 0x81, 0xe9}, {0x66, 0x81, 0xe9, 0x6c},
		{0xf3, 0x66, 0x81, 0xe9, 0x6c, 7}, {0xf2, 0x66, 0x81, 0xe9, 0x6c, 7},
		{0x2e, 0x66, 0x81, 0xe9, 0x6c, 7}, {0x67, 0x66, 0x81, 0xe9, 0x6c, 7},
		{0xf0, 0x66, 0x81, 0xe9, 0x6c, 7}, {0x66, 0x81, 0x2d, 0, 0, 0, 0, 0x6c, 7},
		{0x66, 0x81, 0xd1, 0x6c, 7}, {0x66, 0x81, 0xd9, 0x6c, 7},
	} {
		c := New(testBus(code))
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 0x847
		c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x123, 2, [8]float64{3, 4}
		r, seg, flags, stack := c.R, c.Seg, c.EFlags, c.FPUStack
		if err := c.Step(); err == nil || c.R != r || c.Seg != seg || c.EFlags != flags || c.FPUStack != stack || c.FPUDepth != 2 || c.FPUControl != 0x37f || c.FPUStatus != 0x123 {
			t.Fatal("拒絕改核心", code, err)
		}
	}
}
func TestSUBWordImm16ExistingFormsAndActualConsumers(t *testing.T) {
	for _, tc := range []struct {
		code     []byte
		eax, ecx uint32
		want     uint32
	}{
		{[]byte{0x66, 0x81, 0xc1, 1, 0}, 0, 0xa5cd0010, 0xa5cd0011},
		{[]byte{0x66, 0x81, 0xc9, 1, 0}, 0, 0xa5cd0010, 0xa5cd0011},
		{[]byte{0x66, 0x81, 0xe1, 1, 0}, 0, 0xa5cd0010, 0xa5cd0000},
		{[]byte{0x66, 0x83, 0xe9, 1}, 0, 0xa5cd0010, 0xa5cd000f},
		{[]byte{0x81, 0xe9, 0x6c, 7, 0, 0}, 0, 1996, 96},
	} {
		c := New(testBus(tc.code))
		c.R[EAX], c.R[ECX] = tc.eax, tc.ecx
		if err := c.Step(); err != nil || c.R[ECX] != tc.want {
			t.Fatal("既有81／83形狀退化", tc.code, err)
		}
	}
	c := New(testBus{0x66, 0x81, 0xe9, 0x6c, 7, 0x88, 0xc5, 0xc1, 0xe1, 0x10, 0x66, 0x89, 0xd1})
	c.R[EAX], c.R[ECX], c.R[EDX] = 0x2b2a01, 1996, 0x101
	c.EFlags = 0x216
	for i, want := range []struct{ eip, ecx, flags uint32 }{{5, 96, 0x206}, {7, 0x160, 0x206}, {10, 0x1600000, 0x206}, {13, 0x1600101, 0x206}} {
		if err := c.Step(); err != nil || c.EIP != want.eip || c.R[ECX] != want.ecx || c.EFlags != want.flags || c.R[EAX] != 0x2b2a01 || c.R[EDX] != 0x101 {
			t.Fatalf("實際consumer%d r=%X flags=%X err=%v", i, c.R, c.EFlags, err)
		}
	}
}
