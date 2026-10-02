package cpu386

import (
	"bytes"
	"testing"
)

// 整數乘法／整除與低byte逐bit計數，不重用實作位移或旗標函式。
func shlDwordOneOracle(value, initial uint32) (uint32, uint32) {
	result := (uint64(value) * 2) % 0x100000000
	carry := uint64(value) / 0x80000000
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	flags |= uint32(carry)
	if result/0x80000000 != carry {
		flags |= OF
	}
	if result >= 0x80000000 {
		flags |= SF
	}
	if result == 0 {
		flags |= ZF
	}
	ones := 0
	for n := result % 256; n > 0; n /= 2 {
		ones += int(n % 2)
	}
	if ones%2 == 0 {
		flags |= PF
	}
	// AF未定義，清除只驗既有工具模型。
	return uint32(result), flags
}

func TestSHLDwordOneAllRegistersLowWordsBitsAndArithmeticFlags(t *testing.T) {
	values := []uint32{0x7fffffff, 0x80000000, 0x80000001, 0xffffffff, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for value := uint32(0); value < 65536; value++ {
		values = append(values, value)
	}
	masks := [6]uint32{CF, PF, AF, ZF, SF, OF}
	for reg := byte(0); reg < 8; reg++ {
		for _, value := range values {
			for pattern := 0; pattern < 64; pattern++ {
				code := []byte{0xd1, 0xe0 | reg}
				c, mem := inImmediateFixture(code)
				c.R[reg], c.EFlags = value, 2|IF|DF|0x200000
				for bit, flag := range masks {
					if pattern&(1<<bit) != 0 {
						c.EFlags |= flag
					}
				}
				want := snapshotInImmediate(c)
				want.r[reg], want.flags = shlDwordOneOracle(value, c.EFlags)
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
					t.Fatalf("reg=%d value=%X flags=%d：%v got=%+v want=%+v", reg, value, pattern, err, snapshotInImmediate(c), want)
				}
			}
		}
	}
}

func TestSHLDwordOneRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xd1, 0xe0 | reg}
		cases := [][]byte{nil, {0xd1}}
		for _, prefix := range []byte{0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
		}
		for _, instruction := range cases {
			c, mem := inImmediateFixture(instruction)
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
				t.Fatalf("截短／前綴 %X：%v", instruction, err)
			}
		}
	}
	for _, code := range [][]byte{
		{0xd1, 0x25, 32, 0, 0, 0}, {0xd1, 0x60, 32}, {0xd1, 0xa0, 32, 0, 0, 0},
		{0xd1, 0xd8}, {0xd1, 0xf0},
	} {
		c, mem := subByteMemoryFixture(code)
		c.R[EAX] = 0
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("未知記憶體／group %X：%v", code, err)
		}
	}
}

func TestSHLDwordOneOriginalFullStatePair(t *testing.T) {
	code := []byte{0xd1, 0xe0, 0xd1, 0xe3}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0, 0x3d6978, 0x8000, 1, 0x2723e0, 0x2723f4, 1, 0x325048}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 2
	want := snapshotInImmediate(c)
	want.flags = 0x46
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始EAX單位SHL完整保持：%v", err)
	}
	want.r[EBX], want.flags = 2, 2
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始EBX單位SHL完整保持：%v", err)
	}
}

func TestSHLDwordOneKeepsExistingShiftAndRotatePaths(t *testing.T) {
	for _, test := range []struct {
		code          []byte
		value, result uint32
		flags         uint32
	}{
		{[]byte{0x66, 0xd1, 0xe0}, 0x98764000, 0x98768000, 0xE86},
		{[]byte{0xc1, 0xe0, 1}, 0x80000001, 2, 0xE03},
		{[]byte{0xd3, 0xe0}, 0x80000001, 2, 0xE03},
		{[]byte{0xd1, 0xc8}, 1, 0x80000000, 0xED7},
		{[]byte{0xd1, 0xd0}, 0x80000001, 3, 0xED7},
		{[]byte{0xd1, 0xe8}, 0x80000001, 0x40000000, 0xE07},
		{[]byte{0xd1, 0xf8}, 0x80000001, 0xc0000000, 0x687},
	} {
		c, mem := inImmediateFixture(test.code)
		c.R[EAX], c.R[ECX] = test.value, 1
		want := snapshotInImmediate(c)
		want.r[EAX], want.flags = test.result, test.flags
		if err := c.Step(); err != nil || c.EIP != uint32(len(test.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, test.code) {
			t.Fatalf("既有指令 %X：%v got=%+v want=%+v", test.code, err, snapshotInImmediate(c), want)
		}
	}
}
