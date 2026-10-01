package cpu386

import (
	"bytes"
	"math/bits"
	"testing"
)

func xorWordRegisterExpected(a, b uint16, controls uint32) (uint16, uint32) {
	var result uint16
	for bit := uint(0); bit < 16; bit++ {
		if ((a>>bit)&1)+((b>>bit)&1) == 1 {
			result |= 1 << bit
		}
	}
	flags := controls &^ (CF | PF | AF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if int16(result) < 0 {
		flags |= SF
	}
	if bits.OnesCount8(byte(result))%2 == 0 {
		flags |= PF
	}
	return result, flags
}

func TestXORWordRegisterAllPairsAndFlags(t *testing.T) {
	values := []uint16{0, 1, 15, 16, 255, 256, 0x7fff, 0x8000, 0xff00, 0xffff, 0x55aa, 0xaa55, 0xff7f, 0x7f7f, 0xabcd, 0x1234}
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			for _, a := range values {
				for _, b := range values {
					for _, stale := range []uint32{0, CF | PF | AF | ZF | SF | OF} {
						mem := testBus{0x66, 0x31, 0xc0 | src<<3 | dst}
						c := New(mem)
						c.R = [8]uint32{0x11110000, 0x22220000, 0x33330000, 0x44440000, 0x55550000, 0x66660000, 0x77770000, 0x88880000}
						c.R[dst] |= uint32(a)
						if src != dst {
							c.R[src] |= uint32(b)
						}
						c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
						c.EFlags = 2 | IF | DF | 0x200000 | stale
						expected := c.R
						segs := c.Seg
						memory := append([]byte(nil), mem...)
						result, flags := xorWordRegisterExpected(uint16(c.R[dst]), uint16(c.R[src]), c.EFlags)
						expected[dst] = expected[dst]&0xffff0000 | uint32(result)
						if err := c.Step(); err != nil || c.EIP != 3 || c.R != expected || c.Seg != segs || c.EFlags&^AF != flags || !bytes.Equal(mem, memory) {
							t.Fatalf("word XOR 目的=%d 來源=%d a=%X b=%X R=%X flags=%X want=%X err=%v", dst, src, a, b, c.R, c.EFlags, flags, err)
						}
						// AF 是未定義旗標；這項只驗現有清 AF 模型，不能作硬體逐值證據。
						if c.EFlags&AF != 0 {
							t.Fatalf("word XOR 未定義 AF 的現有模型須清除：flags=%X", c.EFlags)
						}
					}
				}
			}
		}
	}
}

func TestXORWordRegisterRejectedForms(t *testing.T) {
	cases := [][]byte{{0x66, 0x31}}
	for mod := byte(0); mod < 3; mod++ {
		cases = append(cases, []byte{0x66, 0x31, mod<<6 | 7, 64, 0, 0, 0})
	}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
		cases = append(cases, []byte{prefix, 0x66, 0x31, 0xff})
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		regs, segs, flags := c.R, c.Seg, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("word XOR 未審查形式須拒絕 code=%X err=%v", code, err)
		}
	}
}

func TestXORWordRegisterOriginalHighHalfAndMOVES(t *testing.T) {
	// 只重定位代碼，保留原版完整 EDI、旗標與下一 MOV ES 的 null selector。
	mem := testBus{0x66, 0x31, 0xff, 0x8e, 0xc7, 0xcd, 0x2f}
	c := New(mem)
	c.R = [8]uint32{0xf1684, 0x36df20, 1, 5, 0x3ebad0, 0, 0x3ebb20, 0x38ec2b}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x246
	expected := c.R
	expected[EDI] = 0x380000
	segs := c.Seg
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 3 || c.R != expected || c.Seg != segs || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
		t.Fatalf("原版 word XOR 高半部／旗標：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	segs[SegES] = 0
	if err := c.Step(); err != nil || c.EIP != 5 || c.R != expected || c.Seg != segs || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
		t.Fatalf("原版 MOV ES 消費低 word：R=%X seg=%X flags=%X err=%v", c.R, c.Seg, c.EFlags, err)
	}
}

func TestXORWordRegisterKeepsByteAndDwordForms(t *testing.T) {
	for _, tc := range []struct {
		code  []byte
		want  uint32
		flags uint32
	}{
		{[]byte{0x66, 0x31, 0xd8}, 0x12340003, 0x206},
		{[]byte{0x31, 0xd8}, 0xb9f90003, 0x286},
		{[]byte{0x30, 0xd8}, 0x12340003, 0x206},
	} {
		mem := testBus(append([]byte(nil), tc.code...))
		c := New(mem)
		c.R[EAX], c.R[EBX], c.EFlags = 0x12340002, 0xabcd0001, 2|IF|CF|AF|OF
		regs := c.R
		regs[EAX] = tc.want
		before := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.EFlags != tc.flags || !bytes.Equal(mem, before) {
			t.Fatalf("word／byte／dword XOR code=%X R=%X flags=%X err=%v", tc.code, c.R, c.EFlags, err)
		}
	}
}
