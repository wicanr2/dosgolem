package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 以整除延伸符號、逐位比較與位元計數核對，不呼叫CPU運算或旗標函式。
func xorWordImmediateOracle(value uint32, imm byte, initial uint32) (uint32, uint32) {
	source := uint32(imm)
	if source >= 128 {
		source += 65536 - 256
	}
	word := value % 65536
	result := uint32(0)
	for weight := uint32(1); weight < 65536; weight *= 2 {
		if word/weight%2 != source/weight%2 {
			result += weight
		}
	}
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if result >= 32768 {
		flags |= SF
	}
	ones := 0
	for n := result % 256; n > 0; n /= 2 {
		ones += int(n % 2)
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return value/65536*65536 + result, flags
}

func checkXORWordImmediate(t *testing.T, reg byte, value uint32, imm byte, initial uint32) {
	code := []byte{0x66, 0x83, 0xf0 | reg, imm}
	c, mem := inImmediateFixture(code)
	c.R[reg], c.EFlags = value, initial
	want := snapshotInImmediate(c)
	want.r[reg], want.flags = xorWordImmediateOracle(value, imm, initial)
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("目的=%d 值=%X 立即值=%X 初旗標=%X：%v got=%+v want=%+v", reg, value, imm, initial, err, snapshotInImmediate(c), want)
	}
}

func TestXORWordImmediateAllWordsAndImmediates(t *testing.T) {
	for value := uint32(0); value < 65536; value++ {
		for imm := 0; imm < 256; imm++ {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				checkXORWordImmediate(t, EDI, 0x98760000+value, byte(imm), initial)
			}
		}
	}
}

func TestXORWordImmediateAllRegistersBitsAndFlags(t *testing.T) {
	values := []uint32{0, 1, 127, 128, 255, 256, 0x7fff, 0x8000, 0x8001, 0xfffe, 0xffff, 0xaaaa, 0x5555}
	for bit := uint(0); bit < 16; bit++ {
		values = append(values, uint32(1)<<bit, 65535-(uint32(1)<<bit))
	}
	masks := [6]uint32{CF, PF, AF, ZF, SF, OF}
	for reg := byte(0); reg < 8; reg++ {
		for _, value := range values {
			for imm := 0; imm < 256; imm++ {
				for pattern := 0; pattern < 64; pattern++ {
					initial := uint32(2 | IF | DF | 0x200000)
					for bit, mask := range masks {
						if pattern&(1<<bit) != 0 {
							initial |= mask
						}
					}
					checkXORWordImmediate(t, reg, 0xa55a0000+value, byte(imm), initial)
				}
			}
		}
	}
}

func TestXORWordImmediateRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0x66, 0x83, 0xf0 | reg, 1}
		cases := [][]byte{nil, {0x66}, {0x66, 0x83}, {0x66, 0x83, 0xf0 | reg},
			{0x66, 0x83, 0x35, 32, 0, 0, 0, 1}, {0x66, 0x83, 0x70 | reg, 32, 1}, {0x66, 0x83, 0xb0 | reg, 32, 0, 0, 0, 1}}
		for _, prefix := range []byte{0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
		}
		for _, instruction := range cases {
			c, mem := inImmediateFixture(instruction)
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
				t.Fatalf("未知／截短 %X：%v", instruction, err)
			}
		}
	}
}

func TestXORWordImmediateOriginalFullStateAndPUSHConsumer(t *testing.T) {
	code := []byte{0x66, 0x83, 0xf7, 1, 0x57, 0x50}
	mem := make(testBus, 0x272408)
	copy(mem, code)
	binary.LittleEndian.PutUint32(mem[0x2723f8:0x2723fc], 0x246752)
	binary.LittleEndian.PutUint32(mem[0x2723fc:0x272400], 0x3d6978)
	c := New(mem)
	c.R = [8]uint32{0x325048, 0x3d6978, 0x3d69c0, 1, 0x272400, 0xfc4, 0, 0}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: uint32(len(mem) - 1), Writable: true})
	c.EFlags = 0x46
	want := snapshotInImmediate(c)
	before := append([]byte(nil), mem...)
	want.r[EDI], want.flags = 1, 2
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原版完整word XOR：%v", err)
	}
	want.r[ESP] = 0x2723fc
	binary.LittleEndian.PutUint32(before[0x2723fc:0x272400], 1)
	if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原版PUSH EDI真實完整dword消費：%v", err)
	}
	want.r[ESP] = 0x2723f8
	binary.LittleEndian.PutUint32(before[0x2723f8:0x2723fc], 0x325048)
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原版PUSH EAX保持抽樣：%v", err)
	}
}

func TestXORWordImmediateKeepsExistingArithmeticPaths(t *testing.T) {
	for _, test := range []struct {
		code          []byte
		result, flags uint32
	}{
		{[]byte{0x66, 0x83, 0xc0, 1}, 0x98760001, 0x602},
		{[]byte{0x66, 0x83, 0xe8, 1}, 0x9876ffff, 0x697},
		{[]byte{0x66, 0x83, 0xf8, 1}, 0x98760000, 0x697},
		{[]byte{0x66, 0x83, 0xc8, 1}, 0x98760001, 0x602},
		{[]byte{0x66, 0x83, 0xe0, 1}, 0x98760000, 0x646},
		{[]byte{0x66, 0x31, 0xd8}, 0x98760002, 0x602},
		{[]byte{0x83, 0xf0, 1}, 0x98760001, 0x682},
		{[]byte{0x80, 0xf0, 1}, 0x98760001, 0x602},
	} {
		c, mem := inImmediateFixture(test.code)
		c.R[EAX], c.R[EBX] = 0x98760000, 2
		want := snapshotInImmediate(c)
		want.r[EAX], want.flags = test.result, test.flags
		if err := c.Step(); err != nil || c.EIP != uint32(len(test.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, test.code) {
			t.Fatalf("既有指令 %X：%v got=%+v want=%+v", test.code, err, snapshotInImmediate(c), want)
		}
	}
}
