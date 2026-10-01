package cpu386

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

// 以借位、有號較寬差與低 byte 同位建立旗標預期，不沿用 sub32 的位元公式。
func cmpMemoryExpectedFlags(a, b, controls uint32) uint32 {
	result := a - b
	flags := controls &^ (CF | PF | AF | ZF | SF | OF)
	if a < b {
		flags |= CF
	}
	if a&15 < b&15 {
		flags |= AF
	}
	signed := int64(int32(a)) - int64(int32(b))
	if signed < -2147483648 || signed > 2147483647 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if int32(result) < 0 {
		flags |= SF
	}
	if bits.OnesCount8(byte(result))%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestCMPMemoryRegisterAllSourcesAndFlags(t *testing.T) {
	values := []uint32{0, 1, 15, 16, 24, 127, 128, 255, 256, 0x7fff, 0x7fffffff, 0x80000000, 0xffffff7f, 0xfffffff0, 0xfffffffe, 0xffffffff}
	for reg := byte(0); reg < 8; reg++ {
		for _, a := range values {
			for _, b := range values {
				mem := make(testBus, 160)
				copy(mem, []byte{0x39, reg<<3 | 5, 32, 0, 0, 0})
				binary.LittleEndian.PutUint32(mem[96:100], a)
				c := New(mem)
				c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
				c.R[reg] = b
				c.Seg = [6]uint16{0, 0x188, 2, 3, 4, 5}
				// 目的資料段唯讀，CMP 必須仍可讀取並保持資料。
				c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 95})
				c.EFlags = 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF
				regs, segments := c.R, c.Seg
				before := append([]byte(nil), mem...)
				wantFlags := cmpMemoryExpectedFlags(a, b, c.EFlags)
				if err := c.Step(); err != nil || c.EIP != 6 || c.R != regs || c.Seg != segments || c.EFlags != wantFlags || !bytes.Equal(mem, before) {
					t.Fatalf("來源 %d memory=%X R=%X：flags=%X want=%X err=%v", reg, a, b, c.EFlags, wantFlags, err)
				}
			}
		}
	}
}

func TestCMPMemoryRegisterAddressConsumersAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		code     []byte
		reg      int
		value    uint32
		stack    bool
		extraReg int
		extra    uint32
	}{
		{[]byte{0x39, 0x08}, EAX, 32, false, -1, 0},
		{[]byte{0x39, 0x4f, 0xf0}, EDI, 48, false, -1, 0},
		{[]byte{0x39, 0x4d, 0xf0}, EBP, 48, true, -1, 0},
		{[]byte{0x39, 0x0c, 0x24}, ESP, 32, true, -1, 0},
		{[]byte{0x39, 0x4c, 0x73, 4}, EBX, 12, false, ESI, 8},
		{[]byte{0x39, 0x0c, 0x95, 16, 0, 0, 0}, EDX, 4, false, -1, 0},
		{[]byte{0x39, 0x8e, 0xf0, 0xff, 0xff, 0xff}, ESI, 48, false, -1, 0},
		{[]byte{0x39, 0x8f, 48, 0, 0, 0}, EDI, 0xfffffff0, false, -1, 0},
		{[]byte{0x39, 0x09}, ECX, 32, false, -1, 0},
	} {
		mem := make(testBus, 224)
		copy(mem, tc.code)
		c := New(mem)
		c.R = [8]uint32{1, 24, 3, 4, 5, 6, 7, 8}
		c.R[tc.reg] = tc.value
		if tc.extraReg >= 0 {
			c.R[tc.extraReg] = tc.extra
		}
		c.Seg[SegDS], c.Seg[SegSS] = 0x188, 0x190
		c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 63})
		c.SetDescriptor(0x190, Descriptor{Base: 128, Limit: 63})
		address := 96
		if tc.stack {
			address = 160
		}
		// 另一段相同偏移放不同值，驗證 DS／SS 選擇。
		binary.LittleEndian.PutUint32(mem[96:100], 0x87654321)
		binary.LittleEndian.PutUint32(mem[160:164], 0x12345678)
		binary.LittleEndian.PutUint32(mem[address:address+4], 37)
		c.EFlags = 2 | IF | CF | OF
		regs, segments := c.R, c.Seg
		before := append([]byte(nil), mem...)
		flags := cmpMemoryExpectedFlags(37, c.R[ECX], c.EFlags)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("位址消費 code=%X R=%X flags=%X want=%X err=%v", tc.code, c.R, c.EFlags, flags, err)
		}
	}
}

func TestCMPMemoryRegisterRejectsPrefixesTruncationAndPartialRead(t *testing.T) {
	cases := [][]byte{
		{0x39}, {0x39, 0x0d}, {0x39, 0x0d, 64, 0, 0},
		{0x39, 0x0c}, {0x39, 0x4c, 0x24}, {0x39, 0x4d},
	}
	cases = append(cases, []byte{0xf2, 0x39, 0xc8})
	for _, prefix := range []byte{0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
		cases = append(cases, []byte{prefix, 0x39, 0x0d, 64, 0, 0, 0})
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 24, 3, 4, 5, 6, 7, 8}
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		regs, flags := c.R, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("截短／前綴須拒絕：code=%X flags=%X err=%v", code, c.EFlags, err)
		}
	}
	// 分別讓完整目的最後一個 byte 越過 descriptor，以及 bus 的 0–3 bytes 可讀。
	for available := 0; available < 4; available++ {
		for _, descriptorFailure := range []bool{false, true} {
			mem := make(testBus, 68)
			copy(mem, []byte{0x39, 0x0d, 64, 0, 0, 0})
			c := New(mem)
			c.R[ECX], c.EFlags = 24, 2|IF|CF|ZF|OF
			c.Seg[SegDS] = 0x188
			if descriptorFailure {
				c.SetDescriptor(0x188, Descriptor{Limit: uint32(63 + available)})
			} else {
				c.Bus = mem[:64+available]
				c.SetDescriptor(0x188, Descriptor{Limit: 67})
			}
			regs, segments, flags := c.R, c.Seg, c.EFlags
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
				t.Fatalf("目的完整讀取失敗須保存狀態：available=%d descriptor=%t err=%v", available, descriptorFailure, err)
			}
		}
	}
}

func TestCMPMemoryRegisterOriginalSampleAndJGE(t *testing.T) {
	// 把原版資料目的移至 DS:32，保留 CMP 與相對 JGE 位移。
	mem := make(testBus, 160)
	copy(mem, []byte{0x39, 0x0d, 32, 0, 0, 0, 0x0f, 0x8d, 1, 2, 0, 0})
	c := New(mem)
	c.R = [8]uint32{0, 24, 0, 0, 0x3ebb74, 0x3ebba8, 0xa5940, 0x3cfe38}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.SetDescriptor(0x180, Descriptor{Limit: 159})
	c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 95})
	c.EFlags = 0x246
	regs, segments := c.R, c.Seg
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 6 || c.EFlags != 0x297 || c.R != regs || c.Seg != segments || !bytes.Equal(mem, before) {
		t.Fatalf("原版 CMP 樣本：flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 12 || c.EFlags != 0x297 || c.R != regs || c.Seg != segments || !bytes.Equal(mem, before) {
		t.Fatalf("原版 JGE 不取分支：EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
}

func TestCMPMemoryRegisterKeepsRegisterAnd3BDirections(t *testing.T) {
	for _, tc := range []struct {
		code  []byte
		flags uint32
	}{
		{[]byte{0x39, 0xc8}, 0x297},
		{[]byte{0x3b, 0xc8}, 0x206},
		{[]byte{0x3b, 0x0d, 64, 0, 0, 0}, 0x206},
	} {
		mem := make(testBus, 80)
		copy(mem, tc.code)
		c := New(mem)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: 79})
		c.R[ECX], c.EFlags = 24, 0x246
		regs := c.R
		before := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.EFlags != tc.flags || !bytes.Equal(mem, before) {
			t.Fatalf("既有 39／3B 方向：code=%X flags=%X err=%v", tc.code, c.EFlags, err)
		}
	}
}
