package cpu386

import (
	"bytes"
	"testing"
)

// 逐步算術循環避免重用實作的合併位移公式。
func rolDwordRegisterOracle(value uint32, immediate byte, initial uint32) (uint32, uint32) {
	count := int(immediate & 31)
	flags := initial
	result := uint64(value)
	var carry uint64
	for n := 0; n < count; n++ {
		carry = result / 0x80000000
		result = (result*2 + carry) % 0x100000000
	}
	if count != 0 {
		flags &^= CF
		flags |= uint32(carry)
	}
	if count == 1 {
		flags &^= OF
		if (result / 0x80000000) != carry {
			flags |= OF
		}
	}
	// 多位 OF 保留只驗工具模型，硬體未定義。
	return uint32(result), flags
}

func TestROLDwordRegisterImmediateAllRegistersCountsAndBits(t *testing.T) {
	values := []uint32{0, 1, 0xffffffff, 0x7fffffff, 0x80000000, 0x80000001, 0x02000000, 0xffff0000, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for reg := byte(0); reg < 8; reg++ {
		for imm := 0; imm < 256; imm++ {
			for _, value := range values {
				for flags := uint32(0); flags < 8; flags++ {
					code := []byte{0xc1, 0xc0 | reg, byte(imm)}
					c, mem := inImmediateFixture(code)
					c.R[reg] = value
					c.EFlags = 2 | IF | DF | 0x200000
					if flags&4 != 0 {
						c.EFlags |= PF | AF | ZF | SF
					}
					if flags&1 != 0 {
						c.EFlags |= CF
					}
					if flags&2 != 0 {
						c.EFlags |= OF
					}
					want := snapshotInImmediate(c)
					want.r[reg], want.flags = rolDwordRegisterOracle(value, byte(imm), c.EFlags)
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("reg=%d count=%d value=%X flags=%X：%v got=%+v want=%+v", reg, imm, value, flags, err, snapshotInImmediate(c), want)
					}
				}
			}
		}
	}
}

func TestROLDwordRegisterImmediateRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xc1, 0xc0 | reg, 8}
		var cases [][]byte
		for n := 0; n < len(code); n++ {
			cases = append(cases, append([]byte(nil), code[:n]...))
		}
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
		{0xc1, 0x05, 32, 0, 0, 0, 8},
		{0xc1, 0x40, 32, 8},
		{0xc1, 0x80, 32, 0, 0, 0, 8},
		{0xd1, 0xc0},
		{0xd3, 0xc0},
	} {
		c, mem := subByteMemoryFixture(code)
		c.R[EAX] = 0
		// 段均有效，拒絕不能靠未知 selector 掩蓋。
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("未知記憶體／其他 opcode %X：%v", code, err)
		}
	}
}

func TestROLDwordRegisterImmediateOriginalState(t *testing.T) {
	code := []byte{0xc1, 0xc0, 8}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0x2000000, 0x60906c, 0x272610, 0x3ff09, 0x2bda90, 0x74723, 0x6bce4c, 0x609070}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x286
	want := snapshotInImmediate(c)
	want.r[EAX] = 2
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始 ROL 完整後態：%v", err)
	}
}

func TestROLDwordRegisterImmediateKeepsWordAndROR(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x66, 0xc1, 0xc0, 8})
	c.R[EAX] = 0x98760102
	want := snapshotInImmediate(c)
	want.r[EAX] = 0x98760201
	want.flags = want.flags&^CF | CF
	if err := c.Step(); err != nil || c.EIP != 4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x66, 0xc1, 0xc0, 8}) {
		t.Fatalf("既有 word ROL：%v", err)
	}
	c, mem = inImmediateFixture([]byte{0xc1, 0xc8, 8})
	c.R[EAX] = 0x98760102
	want = snapshotInImmediate(c)
	want.r[EAX] = 0x02987601
	want.flags &^= CF
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xc1, 0xc8, 8}) {
		t.Fatalf("既有 dword ROR：%v", err)
	}
}
