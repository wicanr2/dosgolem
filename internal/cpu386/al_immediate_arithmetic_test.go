package cpu386

import "testing"

// FD2.EXE 0x1E282 `add al, 2`（升級成長列 0x1E249 迴圈）第一次在第六章第 2 回合的
// 敵方回合走到；同族的 `sub al, imm8`（0x2836F 等四處）與 `xor al, imm8`
// （0x239D1、0x334F3）一起接。三者都不帶 prefix。
func TestALImmediateArithmeticFlags(t *testing.T) {
	for _, v := range []struct {
		op        byte
		before    uint32
		immediate uint8
		want      uint32
		flags     uint32
	}{
		{0x04, 0x12345601, 2, 0x12345603, PF},
		{0x04, 0x000000ff, 1, 0x00000000, CF | ZF | PF | AF},
		{0x04, 0x0000007f, 1, 0x00000080, OF | SF | AF},
		{0x2c, 0x00000020, 0x20, 0x00000000, ZF | PF},
		{0x2c, 0x00000000, 3, 0x000000fd, CF | SF | AF},
		{0x2c, 0x00000080, 1, 0x0000007f, OF | AF},
		{0x34, 0xabcd0001, 1, 0xabcd0000, ZF | PF},
		{0x34, 0x000000f0, 1, 0x000000f1, SF},
	} {
		code := testBus{v.op, v.immediate}
		c := New(code)
		c.R[EAX] = v.before
		c.EFlags = IF | DF | CF | OF | SF | ZF | AF | PF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != v.want || c.EFlags != (v.flags|IF|DF) || c.EIP != uint32(len(code)) {
			t.Fatalf("op %02X: result=%X flags=%X eip=%X", v.op, c.R[EAX], c.EFlags, c.EIP)
		}
	}
	for _, code := range []testBus{{0x66, 0x04, 1}, {0x66, 0x2c, 1}, {0x66, 0x34, 1}} {
		c := New(code)
		if err := c.Step(); err == nil {
			t.Fatalf("%X 帶 operand-size prefix 應拒絕", code)
		}
	}
}
