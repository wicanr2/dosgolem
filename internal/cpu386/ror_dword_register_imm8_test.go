package cpu386

import (
	"bytes"
	"testing"
)

// 逐步算術循環避免重用實作的合併位移公式。
func rorDwordRegisterOracle(value uint32, immediate byte, initial uint32) (uint32, uint32) {
	count := int(immediate % 32)
	flags := initial
	result := uint64(value)
	for n := 0; n < count; n++ {
		low := result % 2
		result = result/2 + low*0x80000000
	}
	if count != 0 {
		flags &^= CF
		flags |= uint32(result / 0x80000000)
	}
	if count == 1 {
		flags &^= OF
		if result/0x80000000 != (result/0x40000000)%2 {
			flags |= OF
		}
	}
	// 多位OF保留只驗工具模型，硬體未定義。
	return uint32(result), flags
}

func TestRORDwordRegisterImmediateAllRegistersCountsAndBits(t *testing.T) {
	values := []uint32{0, 1, 0xffffffff, 0x7fffffff, 0x80000000, 0x80000001, 0x02000000, 0xffff0000, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for reg := byte(0); reg < 8; reg++ {
		for imm := 0; imm < 256; imm++ {
			for _, value := range values {
				for flags := uint32(0); flags < 8; flags++ {
					code := []byte{0xc1, 0xc8 | reg, byte(imm)}
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
					want.r[reg], want.flags = rorDwordRegisterOracle(value, byte(imm), c.EFlags)
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("reg=%d count=%d value=%X flags=%X：%v got=%+v want=%+v", reg, imm, value, flags, err, snapshotInImmediate(c), want)
					}
				}
			}
		}
	}
}

func TestRORDwordRegisterImmediateRejectsWithoutPublishing(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xc1, 0xc8 | reg, 8}
		var cases [][]byte
		for n := 0; n < len(code); n++ {
			cases = append(cases, append([]byte(nil), code[:n]...))
		}
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
		{0xc1, 0x0d, 32, 0, 0, 0, 8},
		{0xc1, 0x48, 32, 8},
		{0xc1, 0x88, 32, 0, 0, 0, 8},
		{0xd3, 0xc8},
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

func TestRORDwordRegisterImmediateOriginalStateAndMOVConsumer(t *testing.T) {
	code := []byte{0xc1, 0xca, 0x10, 0x66, 0x8b, 0xc2}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0x347010, 0x6bb370, 0xaff0aff, 0x74a, 0x2bdb70, 0x2215, 0x711120, 0x347094}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x297
	want := snapshotInImmediate(c)
	want.flags = 0x296
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始ROR完整後態：%v", err)
	}
	want.r[EAX] = 0x340aff
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("MOV AX,DX消費及完整保持：%v", err)
	}
}

func TestRORDwordRegisterImmediateKeepsD1ROR(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		for _, value := range []uint32{0, 1, 0xffffffff, 0x40000000, 0x80000000, 0x80000001} {
			c, mem := inImmediateFixture([]byte{0xd1, 0xc8 | reg})
			c.R[reg] = value
			want := snapshotInImmediate(c)
			want.r[reg], want.flags = rorDwordRegisterOracle(value, 1, c.EFlags)
			if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xd1, 0xc8 | reg}) {
				t.Fatalf("既有D1 ROR reg=%d value=%X：%v", reg, value, err)
			}
		}
	}
}
