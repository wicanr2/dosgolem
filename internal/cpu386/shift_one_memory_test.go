package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 規格190：D1 /4的預期結果依Intel SDM定義旗標；AF只驗既有清零政策。
func TestSHL32ByOneRegisterFlags(t *testing.T) {
	for _, tc := range []struct {
		name               string
		value, want, flags uint32
	}{
		{"zero", 0, 0, ZF | PF},
		{"one", 1, 2, 0},
		{"sign-crossing", 0x40000000, 0x80000000, OF | SF | PF},
		{"carry-zero", 0x80000000, 0, CF | OF | ZF | PF},
		{"carry-no-overflow", 0xc0000001, 0x80000002, CF | SF},
		{"all-bits", 0xffffffff, 0xfffffffe, CF | SF},
		{"even-low-parity", 0x83, 0x106, PF},
		{"positive-overflow", 0x7fffffff, 0xfffffffe, OF | SF},
		{"negative-to-positive", 0x80000001, 2, CF | OF},
	} {
		for reg := byte(0); reg < 8; reg++ {
			t.Run(tc.name+string(rune('0'+reg)), func(t *testing.T) {
				c := New(testBus{0xd1, 0xe0 | reg})
				c.R = [8]uint32{11, 12, 13, 14, 15, 16, 17, 18}
				c.R[reg] = tc.value
				wantRegs := c.R
				wantRegs[reg] = tc.want
				c.EFlags = IF | DF | AF | CF | OF | SF | ZF | PF
				if err := c.Step(); err != nil {
					t.Fatal(err)
				}
				if c.R != wantRegs || c.EIP != 2 || c.EFlags != IF|DF|tc.flags {
					t.Fatalf("regs=%x eip=%x flags=%x want=%x", c.R, c.EIP, c.EFlags, IF|DF|tc.flags)
				}
			})
		}
	}
}

func TestSHL32ByOneMemoryAddressAndWidth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   []byte
		target int
	}{
		{"original-esp-disp8", []byte{0xd1, 0x64, 0x24, 8}, 136},
		{"esp-negative-disp8", []byte{0xd1, 0x64, 0x24, 0xf8}, 120},
		{"esp-negative-disp32", []byte{0xd1, 0xa4, 0x24, 0xf0, 0xff, 0xff, 0xff}, 112},
		{"esp-no-disp", []byte{0xd1, 0x24, 0x24}, 128},
		{"ebx-ds-disp8", []byte{0xd1, 0x63, 8}, 360},
		{"scaled-index-ds", []byte{0xd1, 0x64, 0x8b, 0xf8}, 356},
		{"absolute-ds", []byte{0xd1, 0x25, 0xb0, 0, 0, 0}, 432},
		{"no-base-sib-ds", []byte{0xd1, 0x24, 0x4d, 100, 0, 0, 0}, 362},
		{"ebp-ss-disp8", []byte{0xd1, 0x65, 0xf8}, 136},
		{"scaled-index-ss", []byte{0xd1, 0x64, 0x8d, 8}, 164},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 512))
			copy(mem, tc.code)
			binary.LittleEndian.PutUint32(mem[tc.target:], 0x40000001)
			wantMem := append(testBus(nil), mem...)
			binary.LittleEndian.PutUint32(wantMem[tc.target:], 0x80000002)
			c := New(mem)
			c.R = [8]uint32{8, 3, 4, 96, 64, 80, 1, 2}
			wantRegs := c.R
			c.Seg[SegSS], c.Seg[SegDS] = 0x30, 0x38
			c.SetDescriptor(0x30, Descriptor{Base: 64, Limit: 255, Writable: true})
			c.SetDescriptor(0x38, Descriptor{Base: 256, Limit: 255, Writable: true})
			c.EFlags = IF | DF | AF | CF
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(mem, wantMem) || c.R != wantRegs || c.EIP != uint32(len(tc.code)) ||
				c.EFlags != IF|DF|OF|SF {
				t.Fatalf("target=%x value=%x regs=%x eip=%x flags=%x", tc.target,
					binary.LittleEndian.Uint32(mem[tc.target:]), c.R, c.EIP, c.EFlags)
			}
		})
	}
}

func TestSHL32ByOneRejectsPrefixesAndMemoryFaults(t *testing.T) {
	for _, code := range [][]byte{
		{0x66, 0xd1, 0x64, 0x24, 8},
		{0x26, 0xd1, 0x64, 0x24, 8},
		{0xf3, 0xd1, 0x64, 0x24, 8},
		{0xf2, 0xd1, 0x64, 0x24, 8},
		{0x67, 0xd1, 0x64, 0x24, 8},
		{0xd1, 0x6c, 0x24, 8},
		{0xd1, 0xc0},
	} {
		mem := testBus(bytes.Repeat([]byte{0xa5}, 256))
		copy(mem, code)
		before := append(testBus(nil), mem...)
		c := New(mem)
		c.R[ESP] = 64
		beforeRegs := c.R
		c.Seg[SegSS] = 0x30
		c.SetDescriptor(0x30, Descriptor{Limit: 255, Writable: true})
		c.EFlags = IF | CF | OF | AF
		if c.Step() == nil || !bytes.Equal(mem, before) || c.R != beforeRegs || c.EFlags != IF|CF|OF|AF {
			t.Fatalf("未拒絕或發布部分狀態：%x", code)
		}
	}
	for _, code := range [][]byte{{0xd1, 0x64}, {0xd1, 0x64, 0x24}, {0xd1, 0xa4, 0x24, 1, 0, 0}} {
		mem := testBus(append([]byte(nil), code...))
		before := append(testBus(nil), mem...)
		c := New(mem)
		c.EFlags = IF | CF
		if c.Step() == nil || !bytes.Equal(mem, before) || c.EFlags != IF|CF {
			t.Fatalf("未拒絕截斷指令：%x", code)
		}
	}
	for _, tc := range []struct {
		name       string
		descriptor Descriptor
		install    bool
	}{
		{"read-only", Descriptor{Limit: 255}, true},
		{"four-byte-limit", Descriptor{Limit: 74, Writable: true}, true},
		{"missing-descriptor", Descriptor{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 256))
			copy(mem, []byte{0xd1, 0x64, 0x24, 8})
			before := append(testBus(nil), mem...)
			c := New(mem)
			c.R[ESP] = 64
			c.Seg[SegSS] = 0x30
			if tc.install {
				c.SetDescriptor(0x30, tc.descriptor)
			}
			c.EFlags = IF | CF | OF | AF
			if c.Step() == nil || !bytes.Equal(mem, before) || c.R[ESP] != 64 || c.EFlags != IF|CF|OF|AF {
				t.Fatal("記憶體錯誤未拒絕或已發布結果")
			}
		})
	}
}
