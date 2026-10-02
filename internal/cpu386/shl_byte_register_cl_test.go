package cpu386

import (
	"bytes"
	"testing"
)

// 較寬乘法與模256獨立推導結果，定義旗標與工具近似分開處理。
func shlByteCLOracle(value, cl byte, initial uint32) (byte, uint32, uint32) {
	count := uint(cl & 31)
	if count == 0 {
		return value, initial, ^uint32(0)
	}
	product := uint64(value) * (uint64(1) << count)
	result := byte(product % 256)
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	defined := PF | ZF | SF
	var carry uint32
	if count <= 8 {
		carry = uint32((uint64(value) * (uint64(1) << (count - 1)) / 128) % 2)
	}
	if count < 8 {
		defined |= CF
	}
	flags |= carry
	if result == 0 {
		flags |= ZF
	}
	if result >= 128 {
		flags |= SF
	}
	ones := 0
	for n := int(result); n > 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	if count == 1 {
		defined |= OF
		if uint32(result)/128 != carry {
			flags |= OF
		}
	} else {
		flags |= initial & OF
	}
	// 非零 AF 清除、多位 OF 保留、>=8 CF 僅為既有工具模型。
	return result, flags, defined
}

func TestSHLByteRegisterCLAllValuesCountsAndAliases(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		for value := 0; value < 256; value++ {
			for cl := 0; cl < 256; cl++ {
				if reg == 1 && value != cl {
					continue
				}
				for pattern := 0; pattern < 16; pattern++ {
					code := []byte{0xd2, 0xe0 | reg}
					c, mem := inImmediateFixture(code)
					c.R[ECX] = c.R[ECX]&0xffffff00 | uint32(cl)
					host, shift := int(reg%4), uint(0)
					if reg >= 4 {
						shift = 8
					}
					c.R[host] = c.R[host]&^(0xff<<shift) | uint32(value)<<shift
					c.EFlags = 2 | IF | DF | 0x200000
					if pattern&1 != 0 {
						c.EFlags |= CF
					}
					if pattern&2 != 0 {
						c.EFlags |= OF
					}
					if pattern&4 != 0 {
						c.EFlags |= AF
					}
					if pattern&8 != 0 {
						c.EFlags |= SF | ZF | PF
					}
					want := snapshotInImmediate(c)
					result, flags, defined := shlByteCLOracle(byte(value), byte(cl), c.EFlags)
					want.r[host] = want.r[host]&^(0xff<<shift) | uint32(result)<<shift
					want.flags = flags
					if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("reg=%d value=%X CL=%X pattern=%d：%v got=%+v want=%+v", reg, value, cl, pattern, err, snapshotInImmediate(c), want)
					}
					if c.EFlags&defined != flags&defined {
						t.Fatal("定義旗標")
					}
					if cl&31 != 0 {
						if c.EFlags&AF != 0 || cl&31 > 1 && c.EFlags&OF != want.flags&OF {
							t.Fatal("未定義旗標工具模型")
						}
					}
				}
			}
		}
	}
}

func TestSHLByteRegisterCLRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xd2, 0xe0 | reg}
		cases := [][]byte{nil, {0xd2}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
		}
		for group := byte(0); group < 8; group++ {
			if group != 4 {
				cases = append(cases, []byte{0xd2, 0xc0 | group<<3 | reg})
			}
		}
		for _, instruction := range cases {
			c, mem := inImmediateFixture(instruction)
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
				t.Fatalf("截短／前綴／未知 group %X：%v", instruction, err)
			}
		}
	}
	for _, code := range [][]byte{{0xd2, 0x25, 32, 0, 0, 0}, {0xd2, 0x60, 32}, {0xd2, 0xa0, 32, 0, 0, 0}} {
		c, mem := subByteMemoryFixture(code)
		c.R[EAX] = 0
		c.R[ECX] = 2
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		mem[288] = 0x81
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("記憶體形狀 %X：%v", code, err)
		}
	}
}

func TestSHLByteRegisterCLKeepsExistingByteShiftsAndDword(t *testing.T) {
	for _, test := range []struct {
		code          []byte
		value, result uint32
		flags         uint32
	}{
		{[]byte{0xd0, 0xe5}, 0x12348102, 0x12340202, 0xA03},
		{[]byte{0xc0, 0xe5, 2}, 0x12348102, 0x12340402, 0x202},
		{[]byte{0xd0, 0xed}, 0x12348102, 0x12344002, 0xA03},
		{[]byte{0xc0, 0xed, 2}, 0x12348102, 0x12342002, 0x202},
		{[]byte{0xd3, 0xe1}, 0x12348102, 0x48d20408, 0x202},
	} {
		c, mem := inImmediateFixture(test.code)
		c.R[ECX] = test.value
		c.EFlags = 0x202
		want := snapshotInImmediate(c)
		want.r[ECX] = test.result
		want.flags = test.flags
		if err := c.Step(); err != nil || c.EIP != uint32(len(test.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, test.code) {
			t.Fatalf("既有形狀 %X：%v got=%+v want=%+v", test.code, err, snapshotInImmediate(c), want)
		}
	}
}

func TestSHLByteRegisterCLOriginalState(t *testing.T) {
	code := []byte{0xd2, 0xe5}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0xf91c, 0x102, 0, 0x7cc1ef00, 0x2bdac8, 0, 0x609070, 0x6bbc60}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x202
	want := snapshotInImmediate(c)
	want.r[ECX] = 0x402
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始 SHL 完整後態：%v", err)
	}
}
