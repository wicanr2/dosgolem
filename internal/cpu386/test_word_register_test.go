package cpu386

import (
	"bytes"
	"math/bits"
	"testing"
)

func TestTESTWordAllRegisterPairsAndDefinedFlags(t *testing.T) {
	values := []uint16{0, 1, 2, 0x80, 0x100, 0x101, 0x7fff, 0x8000, 0x8001, 0xffff}
	const controls = IF | DF | 2
	for left := byte(0); left < 8; left++ {
		for right := byte(0); right < 8; right++ {
			for _, a := range values {
				for _, b := range values {
					mem := testBus{0x66, 0x85, 0xc0 | right<<3 | left, 0xa5}
					c := New(mem)
					c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
					c.R[left], c.R[right] = 0x80010000|uint32(a), 0xffff0000|uint32(b)
					c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
					c.EFlags = controls | CF | OF | AF | SF | PF | ZF
					beforeR, beforeSeg := c.R, c.Seg
					beforeData := append([]byte(nil), mem...)
					result := uint16(beforeR[left]) & uint16(beforeR[right])
					want := uint32(controls)
					if result == 0 {
						want |= ZF
					}
					if result&0x8000 != 0 {
						want |= SF
					}
					if bits.OnesCount8(uint8(result))%2 == 0 {
						want |= PF
					}
					if err := c.Step(); err != nil || c.EIP != 3 || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != want || !bytes.Equal(mem, beforeData) {
						t.Fatalf("暫存器 %d/%d 值 %X/%X：flags=%X want=%X err=%v", left, right, beforeR[left], beforeR[right], c.EFlags, want, err)
					}
				}
			}
		}
	}
}

func TestTESTWordAndDwordWidthsStayDistinct(t *testing.T) {
	for _, tc := range []struct {
		code  []byte
		flags uint32
	}{
		{[]byte{0x66, 0x85, 0xc0}, 2 | IF | ZF | PF},
		{[]byte{0x85, 0xc0}, 2 | IF | SF | PF},
	} {
		c := New(testBus(tc.code))
		c.R[EAX], c.EFlags = 0x80000000, 2|IF|CF|AF|OF
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R[EAX] != 0x80000000 || c.EFlags != tc.flags {
			t.Fatalf("位元寬度不符：%X flags=%X err=%v", tc.code, c.EFlags, err)
		}
	}
}

func TestTESTWordRejectedFormsPreserveDataAndFlags(t *testing.T) {
	for _, code := range [][]byte{
		{0x66}, {0x66, 0x85}, {0x66, 0x85, 0x00}, {0x66, 0x85, 0x45, 0},
		{0x26, 0x66, 0x85, 0xc0}, {0x36, 0x66, 0x85, 0xc0},
		{0xf3, 0x66, 0x85, 0xc0}, {0xf2, 0x66, 0x85, 0xc0},
	} {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{0x80001234, 2, 3, 4, 5, 6, 7, 8}
		c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
		c.EFlags = 0xa57
		r, seg := c.R, c.Seg
		if err := c.Step(); err == nil || c.R != r || c.Seg != seg || c.EFlags != 0xa57 || !bytes.Equal(mem, code) {
			t.Fatalf("未知形式未完整拒絕：%X flags=%X err=%v", code, c.EFlags, err)
		}
	}
}

func TestTESTWordOriginalSampleAndFirstBranch(t *testing.T) {
	// 規格 262 的 DOSBox-X 同次架構樣本；只驗 CPU 指令與第一個分支。
	mem := testBus(make([]byte, 0x600000))
	copy(mem[0x248f43:], []byte{0x66, 0x85, 0xc0, 0x0f, 0x85, 0xf5, 0, 0, 0, 0xb8, 0x17, 0xec, 0x38, 0, 0x31, 0xd2})
	c := New(mem)
	c.R = [8]uint32{EAX: 0, EBX: 0xa0, ECX: 0, EDX: 0x508044, ESI: 0, EDI: 0x3a2090, EBP: 0x3ebc06, ESP: 0x3ebbc4}
	c.Seg = [6]uint16{SegCS: 0x180, SegDS: 0x188, SegES: 0x188, SegGS: 0x20, SegSS: 0x188}
	c.SetDescriptor(0x188, Descriptor{Limit: 0x5fffff, Writable: true})
	c.EIP, c.EFlags = 0x248f43, 0x246
	r, seg := c.R, c.Seg
	if err := c.Step(); err != nil || c.EIP != 0x248f46 || c.R != r || c.Seg != seg || c.EFlags != 0x246 {
		t.Fatalf("原版 TEST 樣本不符：flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 0x248f4c || c.R != r || c.Seg != seg || c.EFlags != 0x246 {
		t.Fatalf("原版第一個 JNZ 路徑不符：EIP=%X err=%v", c.EIP, err)
	}
}
