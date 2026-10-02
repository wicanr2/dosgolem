package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type wordADDFPU312 struct {
	control, status uint16
	stack           [8]float64
	depth           uint8
}

func snapshotWordADDFPU312(c *CPU) wordADDFPU312 {
	return wordADDFPU312{c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
}
func newWordADDCPU312(bus Bus) *CPU {
	c := New(bus)
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x1234, 0x5678, 2
	c.FPUStack = [8]float64{1.25, -2.5, 3.75, 4.5, 5.25, 6.75, 7.5, 8.25}
	return c
}

func independentWordADDFlags312(a, b uint16) uint32 {
	sum := uint32(a) + uint32(b)
	r := uint16(sum)
	f := uint32(0)
	if sum > 65535 {
		f |= CF
	}
	if uint32(a&15)+uint32(b&15) > 15 {
		f |= AF
	}
	if r == 0 {
		f |= ZF
	}
	if r&0x8000 != 0 {
		f |= SF
	}
	signed := int32(int16(a)) + int32(int16(b))
	if signed < -32768 || signed > 32767 {
		f |= OF
	}
	ones := 0
	for bits := uint8(r); bits != 0; bits >>= 1 {
		ones += int(bits & 1)
	}
	if ones%2 == 0 {
		f |= PF
	}
	return f
}
func TestADDWordMemorySourceAllWordsAndRegisters(t *testing.T) {
	mask := uint32(CF | PF | AF | ZF | SF | OF)
	for reg := 0; reg < 8; reg++ {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, 0x03, byte(5 | reg<<3), 64, 0, 0, 0})
		c := newWordADDCPU312(mem)
		c.Seg = [6]uint16{8, 0x30, 0x30, 0, 0x20, 0x30}
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: false})
		mem[95], mem[98] = 0xa5, 0x5a
		binary.LittleEndian.PutUint16(mem[96:], 2)
		snapshot := append([]byte(nil), mem...)
		for a := 0; a < 65536; a++ {
			c.EIP = 0
			c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
			c.R[reg] = 0xabcd0000 | uint32(a)
			c.EFlags = 0x210f57
			r, seg, f, fpu := c.R, c.Seg, c.EFlags, snapshotWordADDFPU312(c)
			want := r
			want[reg] = 0xabcd0000 | uint32(uint16(a+2))
			if err := c.Step(); err != nil || c.EIP != 7 || c.R != want || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != f&^mask|independentWordADDFlags312(uint16(a), 2) || !bytes.Equal(mem, snapshot) {
				t.Fatalf("reg%d a=%X r=%X flags=%X err=%v", reg, a, c.R, c.EFlags, err)
			}
		}
		for _, a := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
			for _, b := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
				binary.LittleEndian.PutUint16(mem[96:], b)
				c.EIP = 0
				c.R[reg] = 0xabcd0000 | uint32(a)
				c.EFlags = IF | DF
				snapshot = append(snapshot[:0], mem...)
				if err := c.Step(); err != nil || c.R[reg] != 0xabcd0000|uint32(a+b) || c.EFlags != IF|DF|independentWordADDFlags312(a, b) || !bytes.Equal(mem, snapshot) {
					t.Fatal("word邊界或只讀來源", a, b, err)
				}
			}
		}
	}
}
func TestADDWordMemorySourceAddressingAndConsumers(t *testing.T) {
	for _, v := range []struct {
		code   []byte
		seg    int
		offset uint32
	}{
		{[]byte{0x66, 0x03, 0x02}, SegDS, 64},
		{[]byte{0x66, 0x03, 0x45, 0xff}, SegSS, 64},
		{[]byte{0x66, 0x03, 0x04, 0x24}, SegSS, 64},
		{[]byte{0x66, 0x03, 0x04, 0x8d, 48, 0, 0, 0}, SegDS, 64},
		{[]byte{0x66, 0x03, 0x44, 0x8d, 0xff}, SegSS, 80},
		{[]byte{0x66, 0x03, 0x83, 66, 0, 0, 0}, SegDS, 64},
		{[]byte{0x66, 0x03, 0x05, 65, 0, 0, 0}, SegDS, 65},
	} {
		mem := testBus(make([]byte, 256))
		copy(mem, v.code)
		c := newWordADDCPU312(mem)
		c.Seg[SegDS], c.Seg[SegSS] = 0x30, 0x40
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: false})
		c.SetDescriptor(0x40, Descriptor{Base: 64, Limit: 95, Writable: false})
		c.R = [8]uint32{0xbeef000f, 4, 64, 0xfffffffe, 64, 65, 7, 8}
		c.EFlags = IF | DF
		base := uint32(32)
		if v.seg == SegSS {
			base = 64
		}
		binary.LittleEndian.PutUint16(mem[base+v.offset:], 2)
		// 另一個segment在同offset放不同值，不能把SS當DS。
		other := uint32(64)
		if base == 64 {
			other = 32
		}
		binary.LittleEndian.PutUint16(mem[other+v.offset:], 0x1234)
		r, seg, fpu := c.R, c.Seg, snapshotWordADDFPU312(c)
		want := r
		want[EAX] = 0xbeef0011
		snapshot := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != uint32(len(v.code)) || c.R != want || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != IF|DF|PF|AF || !bytes.Equal(mem, snapshot) {
			t.Fatalf("code%X r%X flags%X err%v", v.code, c.R, c.EFlags, err)
		}
	}
	mem := testBus(make([]byte, 0x29bf00))
	copy(mem, []byte{0x66, 0x03, 0x05, 0xa4, 0xbe, 0x29, 0, 0x66, 0xa3, 0xa2, 0xbe, 0x29, 0})
	c := newWordADDCPU312(mem)
	c.Seg[SegDS] = 0x188
	c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: uint32(len(mem) - 1), Writable: true})
	c.R = [8]uint32{15, 0, 0x2bdcdc, 8, 0x2bdbd4, 0x2bdbe0, 0x284324, 0x2bdca4}
	c.EFlags = 0x202
	copy(mem[0x29bea0:], []byte{15, 0, 0, 0, 2, 0, 2, 0})
	if err := c.Step(); err != nil || c.R[EAX] != 17 || c.EFlags != 0x216 {
		t.Fatal("原版ADD形狀", err)
	}
	r, seg, f, fpu := c.R, c.Seg, c.EFlags, snapshotWordADDFPU312(c)
	if err := c.Step(); err != nil || c.EIP != 13 || c.R != r || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != f || !bytes.Equal(mem[0x29bea0:0x29bea8], []byte{15, 0, 17, 0, 2, 0, 2, 0}) {
		t.Fatal("真正66 A3寫回", err)
	}
	for _, code := range [][]byte{{0x66, 0x03, 0xc1}, {0x03, 0xc1}, {0x66, 0x01, 0x05, 64, 0, 0, 0}} {
		mem := testBus(make([]byte, 128))
		copy(mem, code)
		c := newWordADDCPU312(mem)
		c.Seg[SegDS] = 0x30
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: true})
		c.R[EAX], c.R[ECX] = 0xbeef000f, 2
		binary.LittleEndian.PutUint16(mem[96:], 2)
		if err := c.Step(); err != nil {
			t.Fatal("既有ADD回歸", err)
		}
		if code[0] == 3 {
			if c.R[EAX] != 0xbeef0011 {
				t.Fatal(c.R)
			}
		} else if code[1] == 1 {
			if binary.LittleEndian.Uint16(mem[96:]) != 17 || c.R[EAX] != 0xbeef000f {
				t.Fatal(c.R)
			}
		} else if c.R[EAX] != 0xbeef0011 {
			t.Fatal(c.R)
		}
	}
}
func TestADDWordMemorySourceRejectsWithoutWrites(t *testing.T) {
	for _, v := range []struct {
		code    []byte
		missing bool
		limit   uint32
		size    int
	}{
		{[]byte{0x66, 0x03}, false, 95, 128},
		{[]byte{0x66, 0x03, 0x05, 64, 0, 0}, false, 95, 128},
		{[]byte{0x66, 0x03, 0x04}, false, 95, 128},
		{[]byte{0x66, 0x03, 0x05, 64, 0, 0, 0}, true, 95, 128},
		{[]byte{0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 64, 128},
		{[]byte{0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 97},
		{[]byte{0x66, 0x03, 0x05, 0xff, 0xff, 0xff, 0xff}, false, 0xffffffff, 128},
		{[]byte{0xf3, 0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 128},
		{[]byte{0xf2, 0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 128},
		{[]byte{0x3e, 0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 128},
		{[]byte{0x67, 0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 128},
		{[]byte{0xf0, 0x66, 0x03, 0x05, 64, 0, 0, 0}, false, 95, 128},
	} {
		mem := testBus(make([]byte, v.size))
		copy(mem, v.code)
		// 將截短指令放在bus末端，零填充不能假裝disp已讀完整。
		c := newWordADDCPU312(mem)
		if len(v.code) < 7 && (len(v.code) < 3 || v.code[2] != 0x04) {
			c = newWordADDCPU312(testBus(v.code))
			mem = testBus(v.code)
		}
		if len(v.code) == 3 && v.code[2] == 0x04 {
			c = newWordADDCPU312(testBus(v.code))
			mem = testBus(v.code)
		}
		c.Seg[SegDS] = 0x30
		if !v.missing {
			c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: v.limit, Writable: false})
		}
		c.R = [8]uint32{0xbeef000f, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 0x847
		r, seg, f, fpu := c.R, c.Seg, c.EFlags, snapshotWordADDFPU312(c)
		snapshot := append([]byte(nil), mem...)
		if c.Step() == nil || c.R != r || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != f || !bytes.Equal(mem, snapshot) {
			t.Fatalf("拒絕或核心保持失敗 code%X", v.code)
		}
	}
}
