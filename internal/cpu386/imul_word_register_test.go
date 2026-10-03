package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestIMULWordRegisterSourcesAndSignedBoundaries(t *testing.T) {
	values := []int16{-32768, -32767, -256, -129, -2, -1, 0, 1, 2, 127, 128, 255, 256, 32766, 32767}
	for reg := byte(0); reg < 8; reg++ {
		for _, a := range values {
			for _, b := range values {
				for _, flags := range []uint32{2 | IF, 2 | IF | DF | 0x200000 | CF | OF | SF | ZF | AF | PF} {
					mem := testBus{0x66, 0xf7, 0xe8 | reg, 0xa5}
					c := New(mem)
					c.R = [8]uint32{0xaaaa0000, 0xbbbb0000, 0xcccc0000, 0xdddd0000, 0xeeee0000, 0xffff0000, 0x12340000, 0x56780000}
					c.R[EAX] |= uint32(uint16(a))
					if reg != EAX {
						c.R[reg] |= uint32(uint16(b))
					}
					c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
					c.EFlags = flags
					wantR, segments := c.R, c.Seg
					before := append([]byte(nil), mem...)
					// 使用較寬有號運算與符號延伸判準，驗證完整積及兩個定義旗標。
					product := int64(a) * int64(int16(uint16(c.R[reg])))
					lo, hi := uint16(product), uint16(uint64(product)>>16)
					wantR[EAX] = wantR[EAX]&0xffff0000 | uint32(lo)
					wantR[EDX] = wantR[EDX]&0xffff0000 | uint32(hi)
					wantFlags := flags &^ (CF | OF)
					if product != int64(int16(lo)) {
						wantFlags |= CF | OF
					}
					if err := c.Step(); err != nil || c.EIP != 3 || c.R != wantR || c.Seg != segments || c.EFlags != wantFlags || !bytes.Equal(mem, before) {
						t.Fatalf("來源 %d，AX=%d b=%d：R=%X want=%X flags=%X want=%X err=%v", reg, a, b, c.R, wantR, c.EFlags, wantFlags, err)
					}
				}
			}
		}
	}
}

// 358：word NEG已接通；舊NEG負例明確加segment prefix，其他群組保持。
func TestIMULWordRegisterRejectedFormsDoNotPublish(t *testing.T) {
	for _, code := range [][]byte{
		{0x66}, {0x66, 0xf7},
		{0x66, 0xf7, 0x28}, {0x66, 0xf7, 0x6d, 0}, {0x66, 0xf7, 0x2d, 0, 0, 0, 0},
		{0x26, 0x66, 0xf7, 0xeb}, {0x36, 0x66, 0xf7, 0xeb},
		{0xf2, 0x66, 0xf7, 0xeb}, {0xf3, 0x66, 0xf7, 0xeb},
		{0x67, 0x66, 0xf7, 0xeb}, {0xf0, 0x66, 0xf7, 0xeb},
		{0x66, 0xf7, 0xd3}, {0x3e, 0x66, 0xf7, 0xdb}, {0x66, 0xf7, 0xf3}, {0x66, 0xf7, 0xfb},
	} {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{0xaaaa0001, 2, 0xbbbbffff, 5, 6, 7, 8, 9}
		c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
		c.EFlags = 2 | IF | CF | OF | AF | PF | ZF | SF
		regs, segments, flags := c.R, c.Seg, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("拒絕不得發布結果：code=%X R=%X flags=%X err=%v", code, c.R, c.EFlags, err)
		}
	}
}

// 最小 A3 使用點移到測試資料位址 64，保留 word 積由完整 EAX 存值的語意。
func TestIMULWordRegisterOriginalSampleAndStore(t *testing.T) {
	mem := make(testBus, 80)
	copy(mem, []byte{0x66, 0xf7, 0xeb, 0xa3, 64, 0, 0, 0, 0x33, 0xdb})
	c := New(mem)
	c.R = [8]uint32{1, 0, 0, 5, 0x3ebb74, 0x3ebba8, 0, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x246
	regs, segments := c.R, c.Seg
	regs[EAX] = 5
	if err := c.Step(); err != nil || c.EIP != 3 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 {
		t.Fatalf("原版乘積與定義旗標：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 8 || binary.LittleEndian.Uint32(mem[64:68]) != 5 || c.R != regs || c.EFlags != 0x246 {
		t.Fatalf("A3 消費：R=%X mem=%X err=%v", c.R, mem[64:68], err)
	}
	regs[EBX] = 0
	if err := c.Step(); err != nil || c.EIP != 10 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 {
		t.Fatalf("XOR 重新定義算術旗標：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
}

func TestIMULWordRegisterKeepsExistingMultiplyForms(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		a, b, lo, hi uint32
	}{
		{[]byte{0x66, 0xf7, 0xe3}, 0xaaaaffff, 0xffff, 0xaaaa0001, 0xbbbbfffe},
		{[]byte{0xf7, 0xe3}, 0xffff, 0xffff, 0xfffe0001, 0},
		{[]byte{0x0f, 0xaf, 0xc3}, 0xfffffffe, 3, 0xfffffffa, 0xbbbb0000},
		{[]byte{0x6b, 0xc3, 0xfe}, 1, 3, 0xfffffffa, 0xbbbb0000},
		{[]byte{0x69, 0xc3, 0xfe, 0xff, 0xff, 0xff}, 1, 3, 0xfffffffa, 0xbbbb0000},
	} {
		c := New(testBus(tc.code))
		c.R[EAX], c.R[EBX], c.R[EDX] = tc.a, tc.b, 0xbbbb0000
		if err := c.Step(); err != nil || c.R[EAX] != tc.lo || c.R[EDX] != tc.hi {
			t.Fatalf("既有乘法回歸：code=%X R=%X err=%v", tc.code, c.R, err)
		}
	}
}
