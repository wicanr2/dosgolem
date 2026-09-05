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

func TestCompareALAndJumpZero(t *testing.T) {
	mem := testBus{0x3c, 0x7f, 0x74, 0x02, 0xfb, 0xfb, 0xfb}
	c := New(mem)
	c.R[EAX] = 0x1234007f
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x1234007f || c.EFlags&ZF == 0 {
		t.Fatalf("CMP AL altered value or missed ZF: EAX=%08X flags=%08X", c.R[EAX], c.EFlags)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 6 {
		t.Fatalf("taken JZ EIP=%d want 6", c.EIP)
	}

	c = New(mem)
	c.R[EAX] = 0x80
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EFlags&ZF != 0 || c.EFlags&OF == 0 {
		t.Fatalf("CMP AL flags=%08X", c.EFlags)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 4 {
		t.Fatalf("untaken JZ EIP=%d want 4", c.EIP)
	}
}

func TestMoveSegmentToRegisterAndAbsoluteMemory(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{
		0x8c, 0xe8, // mov eax,gs
		0x8c, 0xdb, // mov ebx,ds
		0x8c, 0x05, 0x80, 0x00, 0x00, 0x00, // mov [80h],es
	})
	c := New(mem)
	c.Seg[SegGS] = 0x20
	c.Seg[SegDS] = 0x160
	c.Seg[SegES] = 0x28
	for range 3 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.R[EAX] != 0x20 || c.R[EBX] != 0x160 || mem[0x80] != 0x28 || mem[0x81] != 0 {
		t.Fatalf("segment move mismatch: EAX=%X EBX=%X mem=%02X%02X", c.R[EAX], c.R[EBX], mem[0x81], mem[0x80])
	}

	c = New(testBus{0x8c, 0xf0})
	if err := c.Step(); err == nil {
		t.Fatal("invalid segment encoding was accepted")
	}
}

func TestMoveRegisterToSegmentNarrowForm(t *testing.T) {
	mem := testBus{0x8e, 0xc3} // mov es,bx
	c := New(mem)
	c.R[EBX] = 0x12340160
	c.EFlags = 0x246
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Seg[SegES] != 0x160 || c.EIP != 2 || c.EFlags != 0x246 {
		t.Fatalf("ES=%04X EIP=%d EFLAGS=%08X", c.Seg[SegES], c.EIP, c.EFlags)
	}

	for _, code := range [][]byte{{0x8e, 0xcb}, {0x8e, 0x03}} { // CS destination; memory source
		c = New(testBus(code))
		if err := c.Step(); err == nil {
			t.Fatalf("unsupported 8E form % X was accepted", code)
		}
	}
}

func TestMoveMemorySelectorToSegmentRequiresHostValidation(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x8e, 0x05, 0x40, 0x00, 0x00, 0x00})
	mem[0x40], mem[0x41] = 0x28, 0
	c := New(mem)
	c.EFlags = 0x246
	c.SegmentLoadOK = func(selector uint16, destination int) bool {
		return selector == 0x28 && destination == SegES
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Seg[SegES] != 0x28 || c.EIP != 6 || c.EFlags != 0x246 {
		t.Fatalf("ES=%X EIP=%d flags=%X", c.Seg[SegES], c.EIP, c.EFlags)
	}

	c = New(mem)
	if err := c.Step(); err == nil || c.Seg[SegES] != 0 {
		t.Fatal("unvalidated selector was accepted")
	}
}

func TestESOverrideSegmentWriteUsesDescriptor(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x26, 0x8c, 0x1d, 0x40, 0x00, 0x00, 0x00}) // mov es:[40h],ds
	c := New(mem)
	c.Seg[SegES] = 0x160
	c.Seg[SegDS] = 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[0x60] != 0x68 || mem[0x61] != 0x01 {
		t.Fatalf("descriptor write=%02X%02X", mem[0x61], mem[0x60])
	}

	for _, descriptor := range []Descriptor{
		{Base: 0x20, Limit: 0x40, Writable: true},
		{Base: 0x20, Limit: 0x7f, Writable: false},
	} {
		c = New(testBus(append([]byte(nil), mem[:7]...)))
		c.Seg[SegES] = 0x160
		c.Seg[SegDS] = 0x168
		c.SetDescriptor(0x160, descriptor)
		if err := c.Step(); err == nil {
			t.Fatalf("invalid descriptor %+v was accepted", descriptor)
		}
	}
	c = New(testBus(append([]byte(nil), mem[:7]...)))
	c.Seg[SegES] = 0x160
	c.Seg[SegDS] = 0x168
	if err := c.Step(); err == nil {
		t.Fatal("unknown selector was accepted")
	}
}

func TestMoveWordRegisterToAbsoluteMemory(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x66, 0x89, 0x0d, 0x40, 0x00, 0x00, 0x00})
	c := New(mem)
	c.R[ECX] = 0x12340030
	c.EFlags = 0x246
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[0x40] != 0x30 || mem[0x41] != 0 || c.EIP != 7 || c.EFlags != 0x246 {
		t.Fatalf("word=%02X%02X EIP=%d flags=%X", mem[0x41], mem[0x40], c.EIP, c.EFlags)
	}
}

func TestProtectedModePushRegister(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	mem[0] = 0x56 // push esi
	c := New(mem)
	c.R[ESI] = 0x12345678
	c.R[ESP] = 0x70
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 0x6c || mem[0x8c] != 0x78 || mem[0x8f] != 0x12 {
		t.Fatalf("ESP=%X stack=% X", c.R[ESP], mem[0x8c:0x90])
	}

	c = New(testBus{0x56})
	c.R[ESP] = 3
	if err := c.Step(); err == nil || c.R[ESP] != 3 {
		t.Fatalf("underflow accepted or ESP changed: %X", c.R[ESP])
	}
}

func TestAddSignExtendedByteAndAndByte(t *testing.T) {
	mem := testBus{0x83, 0xc2, 0x0f, 0x80, 0xe2, 0xf0}
	c := New(mem)
	c.R[EDX] = 0x546b0
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDX] != 0x546bf {
		t.Fatalf("ADD EDX=%X", c.R[EDX])
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDX] != 0x546b0 || c.EFlags&CF != 0 || c.EFlags&SF == 0 {
		t.Fatalf("AND EDX=%X flags=%X", c.R[EDX], c.EFlags)
	}

	c = New(testBus{0x83, 0xc0, 0xff})
	c.R[EAX] = 1
	if err := c.Step(); err != nil || c.R[EAX] != 0 {
		t.Fatalf("sign-extended ADD EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestESOverrideWordRead(t *testing.T) {
	mem := testBus{0x66, 0x26, 0x8b, 0x0d, 0x2c, 0x00, 0x00, 0x00}
	c := New(mem)
	c.R[ECX] = 0xabcd0000
	c.Seg[SegES] = 0x28
	c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
		return 0x30, selector == 0x28 && offset == 0x2c
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ECX] != 0xabcd0030 || c.EIP != 8 {
		t.Fatalf("ES word read ECX=%08X EIP=%d", c.R[ECX], c.EIP)
	}

	c = New(mem)
	c.Seg[SegES] = 0x28
	if err := c.Step(); err == nil {
		t.Fatal("missing segment read hook was accepted")
	}
	c = New(mem)
	c.Seg[SegES] = 0x30
	c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) { return 0, false }
	if err := c.Step(); err == nil {
		t.Fatal("rejected segment cell was accepted")
	}
}
