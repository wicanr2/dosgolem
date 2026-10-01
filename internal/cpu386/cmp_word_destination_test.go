package cpu386

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

func cmpWordDestinationExpectedFlags(a, b uint16, controls uint32) uint32 {
	result := a - b
	flags := controls &^ (CF | PF | AF | ZF | SF | OF)
	if a < b {
		flags |= CF
	}
	if a&15 < b&15 {
		flags |= AF
	}
	signed := int32(int16(a)) - int32(int16(b))
	if signed < -32768 || signed > 32767 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if int16(result) < 0 {
		flags |= SF
	}
	if bits.OnesCount8(byte(result))%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestCMPWordDestinationAllRegistersAndFlags(t *testing.T) {
	values := []uint16{0, 1, 15, 16, 24, 127, 128, 255, 256, 0x7f7f, 0x7fff, 0x8000, 0xff7f, 0xfff0, 0xfffe, 0xffff}
	for src := byte(0); src < 8; src++ {
		for dst := byte(0); dst < 8; dst++ {
			for _, a := range values {
				for _, b := range values {
					mem := testBus{0x66, 0x39, 0xc0 | src<<3 | dst, 0xa5}
					c := New(mem)
					c.R = [8]uint32{0x11110000, 0x22220000, 0x33330000, 0x44440000, 0x55550000, 0x66660000, 0x77770000, 0x88880000}
					c.R[dst] |= uint32(a)
					if src != dst {
						c.R[src] |= uint32(b)
					}
					c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
					c.EFlags = 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF
					regs, segments := c.R, c.Seg
					before := append([]byte(nil), mem...)
					flags := cmpWordDestinationExpectedFlags(uint16(c.R[dst]), uint16(c.R[src]), c.EFlags)
					if err := c.Step(); err != nil || c.EIP != 3 || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
						t.Fatalf("word 目的 %d 來源 %d：R=%X flags=%X want=%X err=%v", dst, src, c.R, c.EFlags, flags, err)
					}
				}
			}
		}
	}
}

func TestCMPWordMemoryAllSourcesExactlyTwoBytes(t *testing.T) {
	values := []uint16{0, 1, 15, 16, 24, 127, 128, 255, 256, 0x7f7f, 0x7fff, 0x8000, 0xff7f, 0xfff0, 0xfffe, 0xffff}
	for reg := byte(0); reg < 8; reg++ {
		for _, a := range values {
			for _, b := range values {
				mem := make(testBus, 160)
				copy(mem, []byte{0x66, 0x39, reg<<3 | 5, 32, 0, 0, 0})
				binary.LittleEndian.PutUint16(mem[96:98], a)
				mem[98], mem[99] = 0x5a, 0xa5
				c := New(mem)
				c.Seg[SegDS] = 0x188
				// descriptor 恰好只包含目的兩 bytes；若誤讀 dword 必須失敗。
				c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 33})
				c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
				c.R[reg] = 0xabcd0000 | uint32(b)
				c.EFlags = 2 | IF | DF | CF | PF | AF | ZF | SF | OF
				regs, segments := c.R, c.Seg
				before := append([]byte(nil), mem...)
				flags := cmpWordDestinationExpectedFlags(a, b, c.EFlags)
				if err := c.Step(); err != nil || c.EIP != 7 || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
					t.Fatalf("word 記憶體來源 %d：flags=%X want=%X err=%v", reg, c.EFlags, flags, err)
				}
			}
		}
	}
}

func TestCMPWordMemoryAddressSegmentsAndAliases(t *testing.T) {
	for _, tc := range []struct {
		code     []byte
		reg      int
		value    uint32
		stack    bool
		extraReg int
		extra    uint32
	}{
		{[]byte{0x66, 0x39, 0x00}, EAX, 32, false, -1, 0},
		{[]byte{0x66, 0x39, 0x47, 0xf0}, EDI, 48, false, -1, 0},
		{[]byte{0x66, 0x39, 0x45, 0xf0}, EBP, 48, true, -1, 0},
		{[]byte{0x66, 0x39, 0x04, 0x24}, ESP, 32, true, -1, 0},
		{[]byte{0x66, 0x39, 0x44, 0x73, 4}, EBX, 12, false, ESI, 8},
		{[]byte{0x66, 0x39, 0x04, 0x95, 16, 0, 0, 0}, EDX, 4, false, -1, 0},
		{[]byte{0x66, 0x39, 0x87, 48, 0, 0, 0}, EDI, 0xfffffff0, false, -1, 0},
	} {
		mem := make(testBus, 224)
		copy(mem, tc.code)
		c := New(mem)
		c.R = [8]uint32{0xabcd0018, 2, 3, 4, 5, 6, 7, 8}
		c.R[tc.reg] = tc.value
		if tc.extraReg >= 0 {
			c.R[tc.extraReg] = tc.extra
		}
		c.Seg[SegDS], c.Seg[SegSS] = 0x188, 0x190
		c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 63})
		c.SetDescriptor(0x190, Descriptor{Base: 128, Limit: 63})
		binary.LittleEndian.PutUint16(mem[96:98], 0x8000)
		binary.LittleEndian.PutUint16(mem[160:162], 0x7fff)
		address := 96
		if tc.stack {
			address = 160
		}
		binary.LittleEndian.PutUint16(mem[address:address+2], 37)
		c.EFlags = 2 | IF | CF | OF
		regs, segments := c.R, c.Seg
		before := append([]byte(nil), mem...)
		flags := cmpWordDestinationExpectedFlags(37, uint16(c.R[EAX]), c.EFlags)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("word 地址消費 code=%X R=%X flags=%X want=%X err=%v", tc.code, c.R, c.EFlags, flags, err)
		}
	}
}

func TestCMPWordDestinationRejectedFormsAndPartialRead(t *testing.T) {
	cases := [][]byte{{0x66, 0x39}, {0x66, 0x39, 0x05, 64}, {0x66, 0x39, 0x04}, {0x66, 0x39, 0x45}}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
		cases = append(cases, []byte{prefix, 0x66, 0x39, 0x05, 64, 0, 0, 0})
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 2 | IF | CF | OF | PF | AF | ZF | SF
		regs, flags := c.R, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("word 截短／未知前綴須拒絕：code=%X err=%v", code, err)
		}
	}
	for available := 0; available < 2; available++ {
		for _, descriptorFailure := range []bool{false, true} {
			mem := make(testBus, 66)
			copy(mem, []byte{0x66, 0x39, 0x05, 64, 0, 0, 0})
			c := New(mem)
			c.R[EAX], c.EFlags = 24, 2|IF|CF|OF|ZF
			c.Seg[SegDS] = 0x188
			if descriptorFailure {
				c.SetDescriptor(0x188, Descriptor{Limit: uint32(63 + available)})
			} else {
				c.SetDescriptor(0x188, Descriptor{Limit: 65})
				c.Bus = mem[:64+available]
			}
			regs, segments, flags := c.R, c.Seg, c.EFlags
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
				t.Fatalf("word 部分讀取須拒絕：available=%d descriptor=%t err=%v", available, descriptorFailure, err)
			}
		}
	}
}

func TestCMPWordDestinationOriginalSampleAndJL(t *testing.T) {
	// 資料目的重定位至 DS:32，保留原版 word CMP、JL 與其位移。
	mem := make(testBus, 160)
	copy(mem, []byte{0x66, 0x39, 0x07, 0x7c, 3, 0x66, 0x89, 0x07})
	c := New(mem)
	c.R = [8]uint32{0, 0x1df, 1, 0x9f, 0x3ebb50, 0x3ebba8, 0x451054, 32}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.SetDescriptor(0x180, Descriptor{Limit: 159})
	c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 95})
	c.EFlags = 0x213
	regs, segments := c.R, c.Seg
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 3 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
		t.Fatalf("原版 word CMP：flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 5 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
		t.Fatalf("原版 JL 不取分支：EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
}

func TestCMPWordDestinationKeeps3BDirection(t *testing.T) {
	for _, tc := range []struct {
		code  []byte
		flags uint32
	}{
		{[]byte{0x66, 0x39, 0xc8}, 0x297},
		{[]byte{0x66, 0x3b, 0xc8}, 0x206},
		{[]byte{0x66, 0x3b, 0x0d, 64, 0, 0, 0}, 0x206},
	} {
		mem := make(testBus, 80)
		copy(mem, tc.code)
		c := New(mem)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: 79})
		c.R[EAX], c.R[ECX], c.EFlags = 0xabcd0000, 0xffff0018, 0x246
		regs := c.R
		before := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.EFlags != tc.flags || !bytes.Equal(mem, before) {
			t.Fatalf("word 39／3B 方向：code=%X flags=%X err=%v", tc.code, c.EFlags, err)
		}
	}
}
