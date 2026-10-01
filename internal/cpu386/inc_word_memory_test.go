package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"testing"
)

func incWordMemoryExpectedFlags(value uint16, controls uint32) uint32 {
	result := uint16(uint32(value) + 1)
	flags := controls &^ (PF | AF | ZF | SF | OF)
	if value&15 == 15 {
		flags |= AF
	}
	if int32(int16(value))+1 > 32767 {
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

func TestINCWordMemoryFlagsAndExactWidth(t *testing.T) {
	values := []uint16{0, 1, 14, 15, 16, 127, 128, 255, 256, 0x7f7f, 0x7fff, 0x8000, 0xff7f, 0xfff0, 0xfffe, 0xffff}
	for _, value := range values {
		for _, carry := range []uint32{0, CF} {
			for _, stale := range []uint32{0, PF | AF | ZF | SF | OF} {
				mem := make(testBus, 128)
				copy(mem, []byte{0x66, 0xff, 0x05, 32, 0, 0, 0})
				binary.LittleEndian.PutUint16(mem[96:98], value)
				mem[95], mem[98], mem[99] = 0xa5, 0x5a, 0xaa
				c := New(mem)
				c.Seg = [6]uint16{0, 0x188, 0x190, 0x198, 0x1a0, 0x188}
				// 限制恰好只有目的兩 bytes，誤用 dword 讀寫會拒絕。
				c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 33, Writable: true})
				c.R = [8]uint32{0x11110000, 0x22220000, 0x33330000, 0x44440000, 0x55550000, 0x66660000, 0x77770000, 0x88880000}
				c.EFlags = 2 | IF | DF | 0x200000 | carry | stale
				regs, segs := c.R, c.Seg
				expected := append([]byte(nil), mem...)
				binary.LittleEndian.PutUint16(expected[96:98], value+1)
				flags := incWordMemoryExpectedFlags(value, c.EFlags)
				if err := c.Step(); err != nil || c.EIP != 7 || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, expected) {
					t.Fatalf("word INC 值=%X CF=%X flags=%X want=%X err=%v", value, carry, c.EFlags, flags, err)
				}
			}
		}
	}
}

func TestINCWordMemoryAddressSegmentsAndAliases(t *testing.T) {
	for _, tc := range []struct {
		code     []byte
		reg      int
		value    uint32
		stack    bool
		extraReg int
		extra    uint32
	}{
		{[]byte{0x66, 0xff, 0x00}, EAX, 32, false, -1, 0},
		{[]byte{0x66, 0xff, 0x47, 0xf0}, EDI, 48, false, -1, 0},
		{[]byte{0x66, 0xff, 0x45, 0xf0}, EBP, 48, true, -1, 0},
		{[]byte{0x66, 0xff, 0x04, 0x24}, ESP, 32, true, -1, 0},
		{[]byte{0x66, 0xff, 0x44, 0x73, 4}, EBX, 12, false, ESI, 8},
		{[]byte{0x66, 0xff, 0x04, 0x95, 16, 0, 0, 0}, EDX, 4, false, -1, 0},
		{[]byte{0x66, 0xff, 0x87, 48, 0, 0, 0}, EDI, 0xfffffff0, false, -1, 0},
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
		c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 63, Writable: true})
		c.SetDescriptor(0x190, Descriptor{Base: 128, Limit: 63, Writable: true})
		binary.LittleEndian.PutUint16(mem[96:98], 0x8000)
		binary.LittleEndian.PutUint16(mem[160:162], 0x7fff)
		address := 96
		if tc.stack {
			address = 160
		}
		value := binary.LittleEndian.Uint16(mem[address : address+2])
		c.EFlags = 2 | IF | CF | OF | ZF
		regs, segs := c.R, c.Seg
		expected := append([]byte(nil), mem...)
		binary.LittleEndian.PutUint16(expected[address:address+2], value+1)
		flags := incWordMemoryExpectedFlags(value, c.EFlags)
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, expected) {
			t.Fatalf("word INC 地址 code=%X R=%X flags=%X want=%X err=%v", tc.code, c.R, c.EFlags, flags, err)
		}
	}
}

type incWordFailureBus struct {
	testBus
	readFail, writeFail uint32
	writes              []uint32
}

func (b *incWordFailureBus) Read8(address uint32) (uint8, error) {
	if address == b.readFail {
		return 0, fmt.Errorf("受控讀取失敗")
	}
	return b.testBus.Read8(address)
}
func (b *incWordFailureBus) Write8(address uint32, value uint8) error {
	b.writes = append(b.writes, address)
	if address == b.writeFail {
		return fmt.Errorf("受控寫入失敗")
	}
	return b.testBus.Write8(address, value)
}

func TestINCWordMemoryFailureDoesNotPublishFlags(t *testing.T) {
	for _, tc := range []struct {
		name                string
		limit               uint32
		writable            bool
		readFail, writeFail uint32
		partial             bool
	}{
		{"唯讀", 65, false, ^uint32(0), ^uint32(0), false},
		{"完整目的界線", 64, true, ^uint32(0), ^uint32(0), false},
		{"讀取首 byte", 65, true, 64, ^uint32(0), false},
		{"讀取末 byte", 65, true, 65, ^uint32(0), false},
		{"寫入首 byte", 65, true, ^uint32(0), 64, false},
		{"寫入末 byte 的既有部分寫入模型", 65, true, ^uint32(0), 65, true},
	} {
		mem := make(testBus, 80)
		copy(mem, []byte{0x66, 0xff, 0x05, 64, 0, 0, 0})
		binary.LittleEndian.PutUint16(mem[64:66], 0x00ff)
		bus := &incWordFailureBus{testBus: mem, readFail: tc.readFail, writeFail: tc.writeFail}
		c := New(bus)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: tc.limit, Writable: tc.writable})
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		regs, segs, flags := c.R, c.Seg, c.EFlags
		expected := append([]byte(nil), mem...)
		if tc.partial {
			expected[64] = 0
		}
		if err := c.Step(); err == nil || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, expected) {
			t.Fatalf("word INC 拒絕 %s：flags=%X err=%v", tc.name, c.EFlags, err)
		}
		wantWrites := 0
		if tc.writeFail == 64 {
			wantWrites = 1
		}
		if tc.partial {
			wantWrites = 2
		}
		if len(bus.writes) != wantWrites {
			t.Fatalf("%s 寫入次數=%d want=%d", tc.name, len(bus.writes), wantWrites)
		}
	}
}

func TestINCWordMemoryRejectedForms(t *testing.T) {
	cases := [][]byte{{0x66, 0xff}, {0x66, 0xff, 0x05, 64}, {0x66, 0xff, 0x04}, {0x66, 0xff, 0x45}}
	for group := byte(0); group < 8; group++ {
		cases = append(cases, []byte{0x66, 0xff, 0xc0 | group<<3})
		if group > 1 {
			cases = append(cases, []byte{0x66, 0xff, group<<3 | 5, 64, 0, 0, 0})
		}
	}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
		cases = append(cases, []byte{prefix, 0x66, 0xff, 0x05, 64, 0, 0, 0})
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		regs, segs, flags := c.R, c.Seg, c.EFlags
		expected := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, expected) {
			t.Fatalf("word INC 未審查形式須拒絕 code=%X err=%v", code, err)
		}
	}
}

func TestINCWordMemoryOriginalSampleAndA1(t *testing.T) {
	// 只重定位 DS:[EAX+4] 及 A1 資料地址，保留原版值與兩條指令的語意。
	mem := make(testBus, 160)
	copy(mem, []byte{0x66, 0xff, 0x40, 4, 0xa1, 40, 0, 0, 0})
	c := New(mem)
	c.R = [8]uint32{28, 0x1df, 0, 0x508050, 0x3ebb74, 0x3ebba8, 0, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.SetDescriptor(0x180, Descriptor{Limit: 159})
	c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 95, Writable: true})
	binary.LittleEndian.PutUint32(mem[104:108], 28)
	c.EFlags = 0x282
	regs, segs := c.R, c.Seg
	expected := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint16(expected[96:98], 1)
	if err := c.Step(); err != nil || c.EIP != 4 || c.R != regs || c.Seg != segs || c.EFlags != 0x202 || !bytes.Equal(mem, expected) {
		t.Fatalf("原版 word INC：flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 9 || c.R != regs || c.Seg != segs || c.EFlags != 0x202 || !bytes.Equal(mem, expected) {
		t.Fatalf("原版 A1 載入：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
}
