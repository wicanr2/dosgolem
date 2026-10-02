package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func negByteRegisterFlags(value byte, initial uint32) uint32 {
	result := (256 - int(value)) % 256
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if value != 0 {
		flags |= CF
	}
	if value%16 != 0 {
		flags |= AF
	}
	if value == 128 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if result >= 128 {
		flags |= SF
	}
	ones := 0
	for n := result; n > 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestNEGByteRegisterAllValuesAndInitialArithmeticFlags(t *testing.T) {
	masks := [6]uint32{CF, PF, AF, ZF, SF, OF}
	for reg := byte(0); reg < 8; reg++ {
		for value := 0; value < 256; value++ {
			for pattern := 0; pattern < 64; pattern++ {
				code := []byte{0xf6, 0xd8 | reg}
				c, mem := inImmediateFixture(code)
				host, shift := int(reg%4), uint(0)
				if reg >= 4 {
					shift = 8
				}
				c.R[host] = c.R[host]&^(0xff<<shift) | uint32(value)<<shift
				c.EFlags = 2 | IF | DF | 0x200000
				for bit, flag := range masks {
					if pattern&(1<<bit) != 0 {
						c.EFlags |= flag
					}
				}
				want := snapshotInImmediate(c)
				want.r[host] = want.r[host]&^(0xff<<shift) | uint32((256-value)%256)<<shift
				want.flags = negByteRegisterFlags(byte(value), c.EFlags)
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
					t.Fatalf("reg=%d value=%X initial=%d：%v got=%+v want=%+v", reg, value, pattern, err, snapshotInImmediate(c), want)
				}
			}
		}
	}
}

func TestNEGByteRegisterRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xf6, 0xd8 | reg}
		cases := [][]byte{nil, {0xf6}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
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
		{0xf6, 0x1d, 32, 0, 0, 0}, {0xf6, 0x58, 32}, {0xf6, 0x98, 32, 0, 0, 0},
		{0xf6, 0xc8}, {0xf6, 0xd0}, {0xf6, 0xe8}, {0xf6, 0xf8},
	} {
		c, mem := subByteMemoryFixture(code)
		c.R[EAX] = 0
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		mem[288] = 0x80
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("記憶體／未知 group %X：%v", code, err)
		}
	}
}

func TestNEGByteRegisterKeepsTESTDIVAndDwordNEG(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0xf6, 0xc1, 0x80})
	c.R[ECX] = 0xffffff76
	want := snapshotInImmediate(c)
	want.flags = testDwordRegisterDefinedFlags(0, c.EFlags) &^ AF
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf6, 0xc1, 0x80}) {
		t.Fatalf("既有 byte TEST：%v", err)
	}
	c, mem = inImmediateFixture([]byte{0xf6, 0xf1})
	c.R[EAX] = 0x98760064
	c.R[ECX] = 0x1234000a
	want = snapshotInImmediate(c)
	want.r[EAX] = 0x9876000a
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf6, 0xf1}) {
		t.Fatalf("既有 byte DIV：%v", err)
	}
	c, mem = inImmediateFixture([]byte{0xf7, 0xd9})
	c.R[ECX] = 1
	want = snapshotInImmediate(c)
	want.r[ECX] = 0xffffffff
	want.flags = subByteMemoryFlags(0, 1, c.EFlags) | SF
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf7, 0xd9}) {
		t.Fatalf("既有 dword NEG：%v", err)
	}
	c, mem = subByteMemoryFixture([]byte{0xf7, 0x5c, 0x24, 4})
	c.R[ESP] = 32
	binary.LittleEndian.PutUint32(mem[804:], 1)
	before := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint32(before[804:], 0xffffffff)
	want = snapshotInImmediate(c)
	want.flags = subByteMemoryFlags(0, 1, c.EFlags) | SF
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("既有堆疊 NEG：%v", err)
	}
}

func TestNEGByteRegisterOriginalStateAndMOVConsumer(t *testing.T) {
	code := []byte{0xf6, 0xd9, 0x8a, 0xd1}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0x230001, 0xfffffff6, 0x458f000, 0x3ff09, 0x2bdac0, 0x800, 0x6cd700, 0x601bf0}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x297
	want := snapshotInImmediate(c)
	want.r[ECX] = 0xffffff0a
	want.flags = 0x217
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want {
		t.Fatalf("原始 NEG 完整後態：%v", err)
	}
	want.r[EDX] = 0x458f00a
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始 MOV 消費：%v", err)
	}
}
