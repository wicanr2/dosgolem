package cpu386

import (
	"bytes"
	"testing"
)

func xorDwordSignedByte(immediate byte) uint32 {
	value := int64(immediate)
	if value >= 128 {
		value = value - 256 + 4294967296
	}
	return uint32(value)
}

func xorDwordIndependent(left, right uint32) uint32 {
	var result uint64
	for bit := uint(0); bit < 32; bit++ {
		weight := uint64(1) << bit
		if uint64(left)/weight%2 != uint64(right)/weight%2 {
			result += weight
		}
	}
	return uint32(result)
}

func xorDwordDefinedFlags(result, initial uint32) uint32 {
	// AF 未定義，這份 oracle 保留它；清除模型在呼叫端另驗。
	flags := initial &^ (CF | PF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if result >= 2147483648 {
		flags |= SF
	}
	ones := 0
	for low := int(byte(result)); low != 0; low /= 2 {
		ones += low % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestXORDwordImmediateAllDestinationsBytesAndIndependentFlags(t *testing.T) {
	values := []uint32{0, 0xffffffff, 0x7fffffff, 0x80000000, 0xfffffbff, 0x12345678, 0x87654321, 0x55555555, 0xaaaaaaaa, 0xffffff80, 0x7f, 0xff}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for register := byte(0); register < 8; register++ {
		for _, left := range values {
			for immediate := 0; immediate < 256; immediate++ {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF, 2 | AF | OF, 2 | ZF | CF} {
					code := []byte{0x83, 0xf0 | register, byte(immediate)}
					c, mem := inImmediateFixture(code)
					c.R[register], c.EFlags = left, initial
					want := snapshotInImmediate(c)
					result := xorDwordIndependent(left, xorDwordSignedByte(byte(immediate)))
					defined := xorDwordDefinedFlags(result, initial)
					want.r[register], want.flags = result, defined&^AF
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("目的%d／來源%X／imm%X／flags%X：%v", register, left, immediate, initial, err)
					}
					if c.EFlags&^AF != defined&^AF || c.EFlags&AF != 0 {
						t.Fatal("五定義旗標與 AF 清除工具模型分開驗")
					}
				}
			}
		}
	}
}

func TestXORDwordImmediateRefusesPrefixesTruncationsMemoryAndUnknownGroups(t *testing.T) {
	for register := byte(0); register < 8; register++ {
		code := []byte{0x83, 0xf0 | register, 0xff}
		cases := [][]byte{code[:0], code[:1], code[:2], {0x83, 0xd0 | register, 0xff}, {0x83, 0xd8 | register, 0xff}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
		}
		for mod := byte(0); mod < 3; mod++ {
			cases = append(cases, []byte{0x83, mod<<6 | 0x30 | register, 0, 0, 0, 0, 0, 0xff})
		}
		for _, instruction := range cases {
			c, mem := inImmediateFixture(instruction)
			c.SegmentRead8 = func(uint16, uint32) (byte, bool) { t.Fatal("拒絕形狀不可讀資料"); return 0, false }
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, instruction) {
				t.Fatalf("拒絕%X：%v", instruction, err)
			}
		}
	}
}

func TestXORDwordImmediateOriginalFullStateAndKnownShift(t *testing.T) {
	code := []byte{0x83, 0xf0, 0xff, 0xc1, 0xe7, 3}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0xfffffbff, 0x7c5, 0, 0x7cc1ef00, 0x2bdacc, 0, 0x6bbc50, 0xe8}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x206
	want := snapshotInImmediate(c)
	want.r[EAX] = 0x400
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("完整初態 XOR：%v", err)
	}
	want.r[EDI], want.flags = 0x740, 0x202
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("既有移位後 XOR 完整 EAX 保持：%v", err)
	}
}

func TestXORDwordImmediateKeepsOther83RegisterGroups(t *testing.T) {
	for _, tc := range []struct {
		modrm byte
		value uint32
		flags uint32
	}{{0xc0, 0xff, 0x207}, {0xc8, 0xffffffff, 0x286}, {0xe0, 0x100, 0x206}, {0xe8, 0x101, 0x213}, {0xf8, 0x100, 0x213}} {
		code := []byte{0x83, tc.modrm, 0xff}
		c, mem := inImmediateFixture(code)
		c.R[EAX], c.EFlags = 0x100, 0x202
		want := snapshotInImmediate(c)
		want.r[EAX], want.flags = tc.value, tc.flags
		if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("既有83 group%X：%v", tc.modrm, err)
		}
	}
}

func TestXORDwordImmediateKeepsExistingXORWidths(t *testing.T) {
	for _, tc := range []struct {
		code []byte
		bits uint
	}{{[]byte{0x35, 0xff, 0xff, 0xff, 0xff}, 32}, {[]byte{0x33, 0xc3}, 32}, {[]byte{0x66, 0x33, 0xc3}, 16}, {[]byte{0x80, 0xf0, 0xff}, 8}} {
		c, mem := inImmediateFixture(tc.code)
		c.R[EAX], c.R[EBX] = 0x12345678, 0xffffffff
		want := snapshotInImmediate(c)
		mask := uint32((uint64(1) << tc.bits) - 1)
		result := xorDwordIndependent(c.R[EAX]&mask, mask)
		want.r[EAX] = c.R[EAX]&^mask | result
		want.flags = xorDwordDefinedFlags(result, c.EFlags) &^ (AF | SF)
		if result >= uint32(uint64(1)<<(tc.bits-1)) {
			want.flags |= SF
		}
		if err := c.Step(); err != nil || c.EIP != uint32(len(tc.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, tc.code) {
			t.Fatalf("既有XOR寬度%d：%v", tc.bits, err)
		}
	}
}
