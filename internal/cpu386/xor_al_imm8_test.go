package cpu386

import (
	"bytes"
	"testing"
)

func TestXORALImmediateAllOperandsAndIndependentFlags(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				code := []byte{0x34, byte(b)}
				c, mem := inImmediateFixture(code)
				c.R[EAX] = 0xa1b2c300 | uint32(a)
				c.EFlags = initial
				want := snapshotInImmediate(c)
				result := xorByteRegisterResult(byte(a), byte(b))
				want.r[EAX] = 0xa1b2c300 | uint32(result)
				defined := xorByteRegisterDefinedFlags(result, initial)
				want.flags = defined &^ AF
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
					t.Fatalf("AL=%X imm=%X：%v", a, b, err)
				}
				if c.EFlags&^AF != defined&^AF || c.EFlags&AF != 0 {
					t.Fatal("五定義旗標與獨立AF近似")
				}
			}
		}
	}
}

func TestXORALImmediateRejectsWithoutCommit(t *testing.T) {
	cases := [][]byte{{}, {0x34}}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
		cases = append(cases, []byte{prefix, 0x34, 1})
	}
	for _, code := range cases {
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("截短／前綴%X：%v", code, err)
		}
	}
}

func TestXORALImmediateOriginalStateAndHighBits(t *testing.T) {
	for _, high := range []uint32{0, 0xfedcba00} {
		c, mem := inImmediateFixture([]byte{0x34, 1})
		c.R = [8]uint32{high | 1, 0x57f, 0, 1, 0x2bdb6c, 0x2bdbb4, 0x3259a0, 0x3d6978}
		c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
		c.EFlags = 0x297
		want := snapshotInImmediate(c)
		want.r[EAX], want.flags = high, 0x246
		if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x34, 1}) {
			t.Fatalf("原始AL／高24位／完整狀態：%v", err)
		}
	}
}
