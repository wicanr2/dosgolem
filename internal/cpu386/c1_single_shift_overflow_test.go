package cpu386

import (
	"bytes"
	"testing"
)

// 用逐步倍增／整除取得移出bit，不共用CPU移位或旗標實作。
func c1ShiftOracle(group byte, value, initial uint32, immediate byte) (uint32, uint32) {
	count := int(immediate % 32)
	if count == 0 {
		return value, initial
	}
	result, carry := uint64(value), uint64(0)
	for i := 0; i < count; i++ {
		if group == 4 {
			carry, result = result/0x80000000, result*2%0x100000000
		} else {
			carry = result % 2
			sign := uint64(0)
			if group == 7 {
				sign = result / 0x80000000 * 0x80000000
			}
			result = result/2 + sign
		}
	}
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	flags |= uint32(carry)
	if count == 1 && (group == 4 && result/0x80000000 != carry || group == 5 && value >= 0x80000000) {
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
	// 非零AF、多位OF未定義，清除只驗現行工具模型。
	return uint32(result), flags
}

func TestC1ShiftAllGroupsRegistersImmediateCountsAndOverflow(t *testing.T) {
	values := []uint32{0, 1, 0x7fffffff, 0x80000000, 0x80000001, 0xffffffff, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for _, group := range []byte{4, 5, 7} {
		for reg := byte(0); reg < 8; reg++ {
			for immediate := 0; immediate < 256; immediate++ {
				for _, value := range values {
					for pattern := 0; pattern < 8; pattern++ {
						code := []byte{0xc1, 0xc0 | group*8 | reg, byte(immediate)}
						c, mem := inImmediateFixture(code)
						c.R[reg], c.EFlags = value, 2|IF|DF|0x200000
						if pattern&1 != 0 {
							c.EFlags |= CF
						}
						if pattern&2 != 0 {
							c.EFlags |= OF
						}
						if pattern&4 != 0 {
							c.EFlags |= PF | AF | ZF | SF
						}
						want := snapshotInImmediate(c)
						want.r[reg], want.flags = c1ShiftOracle(group, value, c.EFlags, byte(immediate))
						if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
							t.Fatalf("group=%d reg=%d imm=%X value=%X flags=%d：%v got=%+v want=%+v", group, reg, immediate, value, pattern, err, snapshotInImmediate(c), want)
						}
					}
				}
			}
		}
	}
}

func TestC1ShiftRejectsTruncationPrefixesAndUnknownGroups(t *testing.T) {
	for _, group := range []byte{4, 5, 7} {
		for reg := byte(0); reg < 8; reg++ {
			code := []byte{0xc1, 0xc0 | group*8 | reg, 1}
			cases := [][]byte{nil, {0xc1}, code[:2]}
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
	}
	for _, code := range [][]byte{{0xc1, 0xd0, 1}, {0xc1, 0xd8, 1}, {0xc1, 0xf0, 1}} {
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("未知group %X：%v", code, err)
		}
	}
}
