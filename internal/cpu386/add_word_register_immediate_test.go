package cpu386

import (
	"bytes"
	"math/bits"
	"testing"
)

func TestADDWordRegisterAllDestinationsAndImmediates(t *testing.T) {
	values := []uint16{0, 1, 15, 16, 127, 128, 240, 255, 256, 0x7f7f, 0x7fff, 0x8000, 0xff7f, 0xfff0, 0xfffe, 0xffff}
	const controls = IF | DF | 2 | 0x200000
	for reg := byte(0); reg < 8; reg++ {
		for _, a := range values {
			for imm := 0; imm < 256; imm++ {
				mem := testBus{0x66, 0x83, 0xc0 | reg, byte(imm), 0xa5}
				c := New(mem)
				c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
				c.R[reg] = 0xabcd0000 | uint32(a)
				c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
				c.EFlags = controls | CF | PF | AF | ZF | SF | OF
				wantR, segments := c.R, c.Seg
				before := append([]byte(nil), mem...)
				b := uint16(int16(int8(byte(imm))))
				sum := uint32(a) + uint32(b)
				result := uint16(sum)
				wantR[reg] = 0xabcd0000 | uint32(result)
				wantFlags := uint32(controls)
				if sum > 0xffff {
					wantFlags |= CF
				}
				if uint32(a&15)+uint32(b&15) > 15 {
					wantFlags |= AF
				}
				signed := int32(int16(a)) + int32(int8(byte(imm)))
				if signed < -32768 || signed > 32767 {
					wantFlags |= OF
				}
				if result == 0 {
					wantFlags |= ZF
				}
				if result >= 0x8000 {
					wantFlags |= SF
				}
				if bits.OnesCount8(byte(result))%2 == 0 {
					wantFlags |= PF
				}
				if err := c.Step(); err != nil || c.EIP != 4 || c.R != wantR || c.Seg != segments || c.EFlags != wantFlags || !bytes.Equal(mem, before) {
					t.Fatalf("目的 %d 值 %X 立即值 %02X：R=%X flags=%X want=%X err=%v", reg, a, imm, c.R, c.EFlags, wantFlags, err)
				}
			}
		}
	}
}

func TestADDWordRegisterRejectedFormsDoNotPublish(t *testing.T) {
	for _, code := range [][]byte{
		{0x66}, {0x66, 0x83}, {0x66, 0x83, 0xc3},
		{0x66, 0x83, 0x00, 1}, {0x66, 0x83, 0x45, 0, 1}, {0x66, 0x83, 0x05, 0, 0, 0, 0, 1},
		{0x26, 0x66, 0x83, 0xc3, 1}, {0x36, 0x66, 0x83, 0xc3, 1},
		{0xf2, 0x66, 0x83, 0xc3, 1}, {0xf3, 0x66, 0x83, 0xc3, 1},
		{0x67, 0x66, 0x83, 0xc3, 1}, {0xf0, 0x66, 0x83, 0xc3, 1},
		{0x66, 0x83, 0xd3, 1}, {0x66, 0x83, 0xdb, 1},
	} {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 0xabcdffff, 5, 6, 7, 8}
		c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
		c.EFlags = 2 | IF | DF | CF | PF | AF | ZF | SF | OF
		r, segments, flags := c.R, c.Seg, c.EFlags
		if err := c.Step(); err == nil || c.R != r || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, code) {
			t.Fatalf("拒絕形狀發布資料：%X R=%X flags=%X err=%v", code, c.R, c.EFlags, err)
		}
	}
}

func TestADDWordRegisterOriginalAndFirstConsumers(t *testing.T) {
	// DOSBox-X 0180:00368B10 的架構樣本；只留最小指令與輸入。
	mem := testBus{0x66, 0x83, 0xc3, 0x18, 0x81, 0xfb, 0xe0, 1, 0, 0, 0x7c, 0x13}
	c := New(mem)
	c.R = [8]uint32{0, 0, 0, 0xf0, 0x3ebb74, 0x3ebba8, 0, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x246
	want, segments := c.R, c.Seg
	want[EBX] = 0x108
	if err := c.Step(); err != nil || c.EIP != 4 || c.R != want || c.Seg != segments || c.EFlags != 0x202 {
		t.Fatalf("原版 ADD 返回：%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 10 || c.R != want || c.EFlags != 0x287 {
		t.Fatalf("原版 CMP 消費：flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 0x1f || c.R != want || c.Seg != segments || c.EFlags != 0x287 {
		t.Fatalf("原版 JL 消費：EIP=%X err=%v", c.EIP, err)
	}
}

func TestADDWordRegisterKeepsOther83Operations(t *testing.T) {
	for _, tc := range []struct {
		code  []byte
		want  uint32
		flags uint32
	}{
		{[]byte{0x83, 0xc3, 1}, 0xabce0000, 2 | IF | AF | SF | PF},
		{[]byte{0x66, 0x83, 0xeb, 1}, 0xabcdfffe, 2 | IF | SF},
		{[]byte{0x66, 0x83, 0xfb, 1}, 0xabcdffff, 2 | IF | SF},
		{[]byte{0x66, 0x83, 0xcb, 1}, 0xabcdffff, 2 | IF | SF | PF},
		{[]byte{0x66, 0x83, 0xe3, 1}, 0xabcd0001, 2 | IF},
	} {
		c := New(testBus(tc.code))
		c.R[EBX], c.EFlags = 0xabcdffff, 2|IF|CF|OF|AF|ZF|SF|PF
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R[EBX] != tc.want || c.EFlags != tc.flags {
			t.Fatalf("既有 83 回歸：%X R=%X flags=%X want=%X err=%v", tc.code, c.R, c.EFlags, tc.flags, err)
		}
	}
}
