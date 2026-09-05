package cpu386

import "testing"

type testBus []byte

func (b testBus) Read8(addr uint32) (uint8, error)      { return b[addr], nil }
func (b testBus) Write8(addr uint32, value uint8) error { b[addr] = value; return nil }

func TestFD2EntryPrefixInstructionShapes(t *testing.T) {
	mem := testBus(make([]byte, 0x200))
	copy(mem, []byte{
		0xfb, 0x83, 0xe4, 0xfc, 0x8b, 0xdc,
		0x89, 0x1d, 0x80, 0x00, 0x00, 0x00,
		0x66, 0xb8, 0x24, 0x00, 0x66, 0xa3, 0x84, 0x00, 0x00, 0x00,
		0xbb, 0x52, 0x41, 0x48, 0x50, 0x2b, 0xc0, 0xb4, 0x30, 0xcd, 0x21,
	})
	c := New(mem)
	c.R[ESP] = 0x103
	interrupt := false
	c.IntHook = func(cpu *CPU, number uint8) bool { interrupt = number == 0x21; return true }
	for !interrupt {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.R[ESP] != 0x100 || c.R[EBX] != 0x50484152 || uint8(c.R[EAX]>>8) != 0x30 || c.EFlags&IF == 0 {
		t.Fatalf("unexpected registers: %+v flags=%08X", c.R, c.EFlags)
	}
	if mem[0x80] != 0 || mem[0x81] != 1 || mem[0x84] != 0x24 || mem[0x85] != 0 {
		t.Fatalf("unexpected memory writes")
	}
}

func TestRelativeJumpAndUnknownOpcode(t *testing.T) {
	mem := testBus{0xeb, 2, 0xff, 0xff, 0xfb}
	c := New(mem)
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 4 {
		t.Fatalf("EIP=%d", c.EIP)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	c.EIP = 2
	if err := c.Step(); err == nil {
		t.Fatal("expected unknown opcode error")
	}
}
