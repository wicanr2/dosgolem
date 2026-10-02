package cpu386

import (
	"bytes"
	"testing"
)

// 逐 bit 比較建立獨立 byte 結果，不呼叫 CPU 的 XOR 路徑。
func xorByteRegisterResult(a, b byte) byte {
	var result byte
	for bit := uint(0); bit < 8; bit++ {
		mask := byte(1 << bit)
		if (a&mask != 0) != (b&mask != 0) {
			result |= mask
		}
	}
	return result
}

// 只驗公開契約定義的五旗標；AF 的清除模型另驗。
func xorByteRegisterDefinedFlags(result byte, initial uint32) uint32 {
	flags := initial &^ (CF | PF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if result >= 128 {
		flags |= SF
	}
	ones := 0
	for value := int(result); value > 0; value /= 2 {
		ones += value % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestXORByteRegisterImmediateAllOperandsAndDestinations(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		register := int(reg % 4)
		shift := uint(0)
		if reg >= 4 {
			shift = 8
		}
		mask := uint32(255) << shift
		for a := 0; a < 256; a++ {
			for b := 0; b < 256; b++ {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					code := []byte{0x80, 0xf0 | reg, byte(b)}
					c, mem := inImmediateFixture(code)
					c.R[register] = (c.R[register] &^ mask) | (uint32(a) << shift)
					c.EFlags = initial
					want := snapshotInImmediate(c)
					result := xorByteRegisterResult(byte(a), byte(b))
					want.r[register] = (want.r[register] &^ mask) | (uint32(result) << shift)
					defined := xorByteRegisterDefinedFlags(result, initial)
					want.flags = defined &^ AF
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("reg=%d a=%X b=%X：%v", reg, a, b, err)
					}
					if c.EFlags&^AF != defined&^AF {
						t.Fatal("定義五旗標")
					}
					if c.EFlags&AF != 0 {
						t.Fatal("AF 清除只驗工具模型")
					}
				}
			}
		}
	}
}

func TestXORByteRegisterImmediateRejectsWithoutCommit(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0x80, 0xf0 | reg, 0xff}
		cases := [][]byte{code[:0], code[:1], code[:2], {0x80, 0xd0 | reg, 0xff}, {0x80, 0xd8 | reg, 0xff}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
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
}

func TestXORByteRegisterImmediateOriginalRawState(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x80, 0xf1, 0xff})
	c.R = [8]uint32{0x80000080, 0x6bbcf9, 0x10, 0x28f70880, 0x2bda90, 0x8a3dc220, 0x6bcc64, 0x6bbc8c}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x282
	want := snapshotInImmediate(c)
	want.r[ECX] = 0x6bbc06
	want.flags = 0x206
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x80, 0xf1, 0xff}) {
		t.Fatalf("原始 CL／完整外層狀態：%v", err)
	}
}

func TestXORByteRegisterImmediateKeepsMemoryShapeAndOtherGroups(t *testing.T) {
	c, mem := subByteMemoryFixture([]byte{0x80, 0x35, 32, 0, 0, 0, 0xff})
	mem[288] = 0xf9
	want := snapshotInImmediate(c)
	want.flags = xorByteRegisterDefinedFlags(6, c.EFlags) &^ AF
	before := append([]byte(nil), mem...)
	before[288] = 6
	if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("既有記憶體 XOR：%v", err)
	}
	for _, group := range []byte{0, 1, 4, 5, 7} {
		c, mem := inImmediateFixture([]byte{0x80, 0xc0 | group<<3, 8})
		c.R[EAX] = 0xaabbcc16
		want := snapshotInImmediate(c)
		result := byte(0x16)
		switch group {
		case 0:
			result += 8
			want.flags = addByteMemoryFlags(0x16, 8, c.EFlags)
		case 1:
			result = 0x1e
			want.flags = xorByteRegisterDefinedFlags(result, c.EFlags) &^ AF
		case 4:
			result = 0
			want.flags = xorByteRegisterDefinedFlags(result, c.EFlags) &^ AF
		case 5:
			result -= 8
			want.flags = subByteMemoryFlags(0x16, 8, c.EFlags)
		case 7:
			want.flags = subByteMemoryFlags(0x16, 8, c.EFlags)
		}
		want.r[EAX] = (want.r[EAX] & 0xffffff00) | uint32(result)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("既有 group %d：%v", group, err)
		}
	}
}
