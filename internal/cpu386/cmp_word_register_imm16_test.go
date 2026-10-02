package cpu386

import (
	"bytes"
	"testing"
)

func TestCMPWordImm16AllWordsAndRegisters(t *testing.T) {
	mask := uint32(CF | PF | AF | ZF | SF | OF)
	for reg := 0; reg < 8; reg++ {
		mem := testBus{0x66, 0x81, byte(0xf8 + reg), 0xd4, 0, 0xa5}
		c := newWordADDCPU312(mem)
		for a := 0; a < 65536; a++ {
			c.EIP = 0
			c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
			c.R[reg] = 0xabcd0000 | uint32(a)
			c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
			c.EFlags = 0x210f57
			r, seg, f, fpu := c.R, c.Seg, c.EFlags, snapshotWordADDFPU312(c)
			if err := c.Step(); err != nil || c.EIP != 5 || c.R != r || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != f&^mask|independentWordSUBFlags(uint16(a), 212) || mem[5] != 0xa5 {
				t.Fatalf("reg%d a%X flags%X err%v", reg, a, c.EFlags, err)
			}
		}
		for _, a := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
			for _, b := range []uint16{0, 1, 15, 16, 0x7fff, 0x8000, 0xffff} {
				mem[3], mem[4] = byte(b), byte(b>>8)
				c.EIP = 0
				c.R[reg] = 0xabcd0000 | uint32(a)
				c.EFlags = IF | DF
				r := c.R
				if err := c.Step(); err != nil || c.R != r || c.EFlags != IF|DF|independentWordSUBFlags(a, b) {
					t.Fatal("完整iw／word邊界", a, b, err)
				}
			}
		}
	}
}
func TestCMPWordImm16ActualJLAndExistingForms(t *testing.T) {
	for _, a := range []uint16{1, 2, 211, 212, 213, 0, 0x7fff, 0x8000, 0xffff} {
		for _, b := range []uint16{212, 0, 0x7fff, 0x8000, 0xffff} {
			mem := testBus{0x66, 0x81, 0xf9, byte(b), byte(b >> 8), 0x0f, 0x8c, 0x45, 0xff, 0xff, 0xff}
			c := newWordADDCPU312(mem)
			c.R[ECX] = 0xabcd0000 | uint32(a)
			c.EFlags = 0x293
			r, seg, fpu := c.R, c.Seg, snapshotWordADDFPU312(c)
			if err := c.Step(); err != nil || c.EIP != 5 || c.R != r || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != 0x202|independentWordSUBFlags(a, b) {
				t.Fatal("CMP flags", a, b, err)
			}
			flags := c.EFlags
			want := uint32(11)
			if int16(a) < int16(b) {
				delta := uint32(0xffffff45)
				want += delta
			}
			if err := c.Step(); err != nil || c.EIP != want || c.R != r || c.Seg != seg || snapshotWordADDFPU312(c) != fpu || c.EFlags != flags {
				t.Fatal("JL有號分支與核心保持", a, b, c.EIP, err)
			}
		}
	}
	for _, code := range [][]byte{
		{0x66, 0x81, 0xe9, 0xd4, 0}, {0x66, 0x81, 0xc1, 0xd4, 0},
		{0x66, 0x81, 0xc9, 0xd4, 0}, {0x66, 0x81, 0xe1, 0xd4, 0},
		{0x81, 0xf9, 0xd4, 0, 0, 0}, {0x66, 0x81, 0x3d, 64, 0, 0, 0, 0xd4, 0},
	} {
		mem := testBus(make([]byte, 128))
		copy(mem, code)
		c := newWordADDCPU312(mem)
		c.Seg[SegDS] = 0x30
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: false})
		c.R[ECX] = 1
		c.EFlags = 0x293
		mem[96] = 1
		if err := c.Step(); err != nil {
			t.Fatal("既有81形狀", code, err)
		}
		group := (code[2] >> 3) & 7
		if code[0] == 0x81 {
			if c.R[ECX] != 1 || c.EFlags != 0x297 {
				t.Fatal("dword CMP", c.R, c.EFlags)
			}
		} else if group == 7 {
			if c.R[ECX] != 1 || c.EFlags != 0x297 || mem[96] != 1 {
				t.Fatal("word memory CMP")
			}
		} else if group == 5 {
			if uint16(c.R[ECX]) != 0xff2d {
				t.Fatal("word SUB")
			}
		} else if group == 0 {
			if c.R[ECX] != 213 {
				t.Fatal("word ADD")
			}
		} else if group == 1 {
			if c.R[ECX] != 213 {
				t.Fatal("word OR")
			}
		} else if group == 4 {
			if c.R[ECX] != 0 {
				t.Fatal("word AND")
			}
		}
	}
}
func TestCMPWordImm16RejectsWithoutCoreWrites(t *testing.T) {
	for _, code := range [][]byte{
		{0x66, 0x81}, {0x66, 0x81, 0xf9}, {0x66, 0x81, 0xf9, 0xd4},
		{0xf3, 0x66, 0x81, 0xf9, 0xd4, 0}, {0xf2, 0x66, 0x81, 0xf9, 0xd4, 0},
		{0x3e, 0x66, 0x81, 0xf9, 0xd4, 0}, {0x67, 0x66, 0x81, 0xf9, 0xd4, 0},
		{0xf0, 0x66, 0x81, 0xf9, 0xd4, 0}, {0x66, 0x66, 0x81, 0xf9, 0xd4, 0},
		{0x66, 0x81, 0xd1, 0xd4, 0}, {0x66, 0x81, 0xd9, 0xd4, 0}, {0x66, 0x81, 0xf1, 0xd4, 0},
	} {
		mem := testBus(code)
		c := newWordADDCPU312(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 0x847
		r, seg, f, fpu := c.R, c.Seg, c.EFlags, snapshotWordADDFPU312(c)
		snapshot := append([]byte(nil), mem...)
		if c.Step() == nil || c.R != r || c.Seg != seg || c.EFlags != f || snapshotWordADDFPU312(c) != fpu || !bytes.Equal(snapshot, mem) {
			t.Fatalf("拒絕後核心被改 %X", code)
		}
	}
}
