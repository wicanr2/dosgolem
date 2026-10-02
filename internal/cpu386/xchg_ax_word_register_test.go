package cpu386

import (
	"bytes"
	"testing"
)

// 整除拆出兩個舊高低word，再交叉組合，旗標直接保存原值。
func checkXCHGAXWord(t *testing.T, reg byte, a, b, initial uint32) {
	code := []byte{0x66, 0x90 + reg}
	c, mem := inImmediateFixture(code)
	c.R[EAX], c.R[reg], c.EFlags = a, b, initial
	want := snapshotInImmediate(c)
	want.r[EAX], want.r[reg] = a/65536*65536+b%65536, b/65536*65536+a%65536
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("對側=%d EAX=%X R=%X 初旗標=%X：%v got=%+v want=%+v", reg, a, b, initial, err, snapshotInImmediate(c), want)
	}
}

func TestXCHGAXWordAllRegistersAndLowWords(t *testing.T) {
	for reg := byte(1); reg < 8; reg++ {
		for word := uint32(0); word < 65536; word++ {
			for _, other := range []uint32{word, 65535 - word} {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					checkXCHGAXWord(t, reg, 0x98760000+word, 0xa55a0000+other, initial)
				}
			}
		}
	}
}

func TestXCHGAXWordBitsEdgesAndAllFlagBits(t *testing.T) {
	values := []uint32{0, 1, 127, 128, 255, 256, 0x7fff, 0x8000, 0x8001, 0xfffe, 0xffff, 0xaaaa, 0x5555}
	for bit := uint(0); bit < 16; bit++ {
		values = append(values, uint32(1)<<bit, 65535-(uint32(1)<<bit))
	}
	masks := [6]uint32{CF, PF, AF, ZF, SF, OF}
	for reg := byte(1); reg < 8; reg++ {
		for _, a := range values {
			for _, b := range values {
				for pattern := 0; pattern < 64; pattern++ {
					initial := uint32(2 | IF | DF | 0x200000)
					for bit, mask := range masks {
						if pattern&(1<<bit) != 0 {
							initial |= mask
						}
					}
					checkXCHGAXWord(t, reg, 0x98760000+a, 0xa55a0000+b, initial)
				}
			}
		}
		// 含AF與非算術旗標，各bit及補集都不得改動。
		for bit := uint(0); bit < 32; bit++ {
			for _, initial := range []uint32{uint32(1) << bit, ^(uint32(1) << bit)} {
				checkXCHGAXWord(t, reg, 0x12347fff, 0xabcd8000, initial)
			}
		}
	}
}

func TestXCHGAXWordRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(1); reg < 8; reg++ {
		code := []byte{0x66, 0x90 + reg}
		cases := [][]byte{nil, {0x66}, {0x90 + reg}, {0x66, 0x90}, {0x66, 0x87, 0xc0 | reg}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
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

func TestXCHGAXWordOriginalFullStateAndRORConsumer(t *testing.T) {
	code := []byte{0x66, 0x93, 0xc1, 0xcb, 8}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0xa0a0a2e, 0x2e0a40c0, 0x2e0a2e0a, 0x2e0a0a0a, 0x2bdb6c, 0x6ee4, 0x70e2d2, 0x3471a0}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x206
	want := snapshotInImmediate(c)
	want.r[EAX], want.r[EBX] = 0xa0a0a0a, 0x2e0a0a2e
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原版完整XCHG與高16位／全部旗標保持：%v", err)
	}
	// 用逐次整除循環保存來源消費，避免以CPU的位移公式作期望。
	for i := 0; i < 8; i++ {
		v := want.r[EBX]
		want.r[EBX] = v/2 + v%2*0x80000000
	}
	if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原版ROR EBX,8完整消費：%v got=%+v want=%+v", err, snapshotInImmediate(c), want)
	}
}

func TestXCHGAXWordKeepsExistingByteDwordAndNOP(t *testing.T) {
	for _, test := range []struct {
		code []byte
		a, b uint32
	}{
		{[]byte{0x87, 0xd8}, 0xaabbccdd, 0x11223344},
		{[]byte{0x86, 0xd8}, 0x112233dd, 0xaabbcc44},
		{[]byte{0x90}, 0x11223344, 0xaabbccdd},
	} {
		c, mem := inImmediateFixture(test.code)
		c.R[EAX], c.R[EBX] = 0x11223344, 0xaabbccdd
		want := snapshotInImmediate(c)
		want.r[EAX], want.r[EBX] = test.a, test.b
		if err := c.Step(); err != nil || c.EIP != uint32(len(test.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, test.code) {
			t.Fatalf("既有指令 %X：%v got=%+v want=%+v", test.code, err, snapshotInImmediate(c), want)
		}
	}
}
