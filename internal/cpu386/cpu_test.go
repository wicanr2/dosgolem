package cpu386

import (
	"bytes"
	"testing"
)

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

func TestNearRelativeJump(t *testing.T) {
	mem := testBus{0xe9, 0x10, 0x00, 0x00, 0x00}
	c := New(mem)
	if err := c.Step(); err != nil || c.EIP != 0x15 {
		t.Fatalf("near JMP EIP=%X err=%v", c.EIP, err)
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

func TestNearJumpZero(t *testing.T) {
	mem := testBus{0x0f, 0x84, 0x03, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff}
	c := New(mem)
	c.EFlags |= ZF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 9 {
		t.Fatalf("taken near JZ EIP=%d", c.EIP)
	}
	c = New(mem)
	if err := c.Step(); err != nil || c.EIP != 6 {
		t.Fatalf("untaken near JZ EIP=%d err=%v", c.EIP, err)
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

func TestMoveSegmentFromESDescriptorMemory(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x26, 0x8e, 0x1d, 0x40, 0x00, 0x00, 0x00})
	mem[0x60], mem[0x61] = 0x30, 0
	c := New(mem)
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	c.SegmentLoadOK = func(selector uint16, destination int) bool {
		return selector == 0x30 && destination == SegDS
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Seg[SegDS] != 0x30 || c.EIP != 7 {
		t.Fatalf("DS=%X EIP=%d", c.Seg[SegDS], c.EIP)
	}
}

func TestMoveDwordFromDSIndexedMemory(t *testing.T) {
	mem := testBus{0x8b, 0x06}
	c := New(mem)
	c.Seg[SegDS] = 0x30
	c.R[ESI] = 4
	c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
		data := []byte{0x78, 0x56, 0x34, 0x12}
		if selector != 0x30 || offset < 4 || offset >= 8 {
			return 0, false
		}
		return data[offset-4], true
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x12345678 || c.EIP != 2 {
		t.Fatalf("EAX=%X EIP=%d", c.R[EAX], c.EIP)
	}
}

func TestMoveDwordFromDSDisp8AndORRegister(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x8b, 0x43, 0x02, 0x0b, 0xc0})
	mem[0x32], mem[0x33], mem[0x34], mem[0x35] = 0xcc, 0xcb, 0x03, 0
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f})
	c.R[EBX] = 0x10
	if err := c.Step(); err != nil || c.R[EAX] != 0x3cbcc {
		t.Fatalf("MOV EAX=%X err=%v", c.R[EAX], err)
	}
	if err := c.Step(); err != nil || c.R[EAX] != 0x3cbcc || c.EFlags&ZF != 0 || c.EFlags&CF != 0 {
		t.Fatalf("OR EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
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

func TestProtectedModePushES(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	mem[0] = 0x06
	c := New(mem)
	c.R[ESP] = 0x50
	c.Seg[SegSS], c.Seg[SegES] = 0x168, 0x160
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 0x4c || !bytes.Equal(mem[0x6c:0x70], []byte{0x60, 0x01, 0, 0}) || c.Seg[SegES] != 0x160 {
		t.Fatalf("ESP=%X stack=% X ES=%X", c.R[ESP], mem[0x6c:0x70], c.Seg[SegES])
	}
}

func TestProtectedModeSegmentTransferAndIndirectCALL(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x1e, 0x07, 0xff, 0xd0})
	c := New(mem)
	c.R[ESP], c.R[EAX] = 0x50, 0x40
	c.Seg[SegSS], c.Seg[SegDS], c.Seg[SegES] = 0x168, 0x160, 0x28
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
	for range 3 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.Seg[SegES] != 0x160 || c.R[ESP] != 0x4c || c.EIP != 0x40 || !bytes.Equal(mem[0x6c:0x70], []byte{4, 0, 0, 0}) {
		t.Fatalf("ES=%X ESP=%X EIP=%X return=% X", c.Seg[SegES], c.R[ESP], c.EIP, mem[0x6c:0x70])
	}
}

func TestProtectedModePopDS(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	mem[0] = 0x1f
	mem[0x70], mem[0x71] = 0x60, 0x01
	c := New(mem)
	c.R[ESP] = 0x50
	c.Seg[SegSS], c.Seg[SegDS] = 0x168, 0x30
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Seg[SegDS] != 0x160 || c.R[ESP] != 0x54 {
		t.Fatalf("DS=%X ESP=%X", c.Seg[SegDS], c.R[ESP])
	}

	c = New(mem)
	c.R[ESP] = 0x50
	c.Seg[SegSS], c.Seg[SegDS] = 0x168, 0x30
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err == nil || c.Seg[SegDS] != 0x30 || c.R[ESP] != 0x50 {
		t.Fatalf("unknown selector changed state: err=%v DS=%X ESP=%X", err, c.Seg[SegDS], c.R[ESP])
	}
}

func TestProtectedModeNearCALL(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0xe8, 0x10, 0x00, 0x00, 0x00})
	c := New(mem)
	c.R[ESP] = 0x50
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 0x15 || c.R[ESP] != 0x4c || !bytes.Equal(mem[0x6c:0x70], []byte{5, 0, 0, 0}) {
		t.Fatalf("EIP=%X ESP=%X return=% X", c.EIP, c.R[ESP], mem[0x6c:0x70])
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

func TestImmediateORCompareAndShortJNZ(t *testing.T) {
	mem := testBus{0x0d, 0x20, 0x20, 0x20, 0x20, 0x3d, 0x6e, 0x6f, 0x38, 0x37, 0x75, 0x02, 0xff, 0xff, 0xfb}
	c := New(mem)
	c.R[EAX] = 0x00010000
	for range 3 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.R[EAX] != 0x20212020 || c.EIP != 14 || c.EFlags&ZF != 0 {
		t.Fatalf("EAX=%X EIP=%d flags=%X", c.R[EAX], c.EIP, c.EFlags)
	}
}

func TestCompareRegisterAndShortJAE(t *testing.T) {
	mem := testBus{0x3b, 0xf7, 0x73, 0x02, 0xff, 0xff, 0xfb}
	c := New(mem)
	c.R[ESI], c.R[EDI] = 0x10, 0x20
	if err := c.Step(); err != nil || c.EFlags&CF == 0 || c.EFlags&ZF != 0 {
		t.Fatalf("CMP flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 4 {
		t.Fatalf("untaken JAE EIP=%X err=%v", c.EIP, err)
	}

	c = New(mem)
	c.R[ESI], c.R[EDI] = 0x20, 0x20
	if err := c.Step(); err != nil || c.EFlags&CF != 0 || c.EFlags&ZF == 0 {
		t.Fatalf("equal CMP flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 6 {
		t.Fatalf("taken JAE EIP=%X err=%v", c.EIP, err)
	}
}

func TestCompareDSByteLoadAndShortJA(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x38, 0x46, 0x01, 0x77, 0x02, 0xff, 0xff, 0x8a, 0x46, 0x01})
	mem[0x25] = 0x20
	c := New(mem)
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x3f})
	c.R[ESI], c.R[EAX] = 4, 0x10
	if err := c.Step(); err != nil || c.EFlags&(CF|ZF) != 0 {
		t.Fatalf("CMP flags=%X err=%v", c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 7 {
		t.Fatalf("taken JA EIP=%X err=%v", c.EIP, err)
	}
	if err := c.Step(); err != nil || c.R[EAX] != 0x20 {
		t.Fatalf("MOV AL,[ESI+1] EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestCompareDSByteAndIncrementPreservesCarry(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x80, 0x3e, 0x00, 0x80, 0x7e, 0xff, 0x00, 0x41})
	mem[0x24] = 0x00
	mem[0x23] = 0x7f
	c := New(mem)
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x3f})
	c.R[ESI], c.R[ECX] = 4, 0xffffffff
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || mem[0x24] != 0 {
		t.Fatalf("zero CMP changed memory or flags: err=%v flags=%X byte=%X", err, c.EFlags, mem[0x24])
	}
	if err := c.Step(); err != nil || c.EFlags&ZF != 0 || c.EFlags&CF != 0 || mem[0x23] != 0x7f {
		t.Fatalf("nonzero CMP changed memory or flags: err=%v flags=%X byte=%X", err, c.EFlags, mem[0x23])
	}
	c.EFlags |= CF
	if err := c.Step(); err != nil || c.R[ECX] != 0 || c.EFlags&CF == 0 || c.EFlags&ZF == 0 {
		t.Fatalf("INC ECX=%X flags=%X err=%v", c.R[ECX], c.EFlags, err)
	}
}

func TestLODSBAndMOVSBUseSegmentDescriptors(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0xac, 0xa4})
	mem[0x24], mem[0x25] = 0x5a, 0xa5
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x30, 0x160
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x3f})
	c.SetDescriptor(0x160, Descriptor{Base: 0x60, Limit: 0x3f, Writable: true})
	c.R[EAX], c.R[ESI], c.R[EDI] = 0x12345600, 4, 3
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x1234565a || c.R[ESI] != 5 {
		t.Fatalf("LODSB EAX=%X ESI=%X", c.R[EAX], c.R[ESI])
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[0x63] != 0xa5 || c.R[ESI] != 6 || c.R[EDI] != 4 {
		t.Fatalf("MOVSB dst=%X ESI=%X EDI=%X", mem[0x63], c.R[ESI], c.R[EDI])
	}
}

func TestMoveByteRegisterAndREPSTOSD(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x8a, 0xd1, 0xf3, 0xab})
	c := New(mem)
	c.R[EAX], c.R[ECX], c.R[EDX], c.R[EDI] = 0x12345678, 2, 0xaabbcc00, 4
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x1f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDX] != 0xaabbcc02 || c.R[ECX] != 2 {
		t.Fatalf("MOV DL,CL EDX=%X ECX=%X", c.R[EDX], c.R[ECX])
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	want := []byte{0x78, 0x56, 0x34, 0x12, 0x78, 0x56, 0x34, 0x12}
	if !bytes.Equal(mem[0x24:0x2c], want) || c.R[ECX] != 0 || c.R[EDI] != 12 {
		t.Fatalf("REP STOSD dst=% X ECX=%X EDI=%X", mem[0x24:0x2c], c.R[ECX], c.R[EDI])
	}
}

func TestREPSTOSBAndAccumulatorImmediateALU(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0xf3, 0xaa, 0x05, 0x0f, 0x00, 0x00, 0x00, 0x24, 0xf0})
	c := New(mem)
	c.R[EAX], c.R[ECX], c.R[EDI] = 0x546b0, 3, 4
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x1f, Writable: true})
	for range 3 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(mem[0x24:0x27], []byte{0xb0, 0xb0, 0xb0}) || c.R[ECX] != 0 || c.R[EDI] != 7 || c.R[EAX] != 0x546b0 {
		t.Fatalf("bytes=% X ECX=%X EDI=%X EAX=%X", mem[0x24:0x27], c.R[ECX], c.R[EDI], c.R[EAX])
	}
	if c.EFlags&CF != 0 || c.EFlags&ZF != 0 || c.EFlags&SF == 0 {
		t.Fatalf("AND AL flags=%X", c.EFlags)
	}
}

func TestESRelativeByteReadWithSignedDisp8(t *testing.T) {
	mem := testBus{0x26, 0x8a, 0x4f, 0xff}
	c := New(mem)
	c.R[EDI] = 0x81
	c.R[ECX] = 0x12345678
	c.Seg[SegES] = 0x28
	c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
		return 0, selector == 0x28 && offset == 0x80
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ECX] != 0x12345600 || c.EIP != 4 {
		t.Fatalf("ECX=%X EIP=%d", c.R[ECX], c.EIP)
	}
}

func TestCLDAndREPESCASB(t *testing.T) {
	mem := testBus{0xfc, 0xf3, 0xae}
	c := New(mem)
	c.EFlags |= DF
	c.R[EAX] = 0x20
	c.R[ECX] = 4
	c.R[EDI] = 0x80
	c.Seg[SegES] = 0x28
	data := map[uint32]uint8{0x80: 0x20, 0x81: 0x20, 0x82: 'X'}
	c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
		v, ok := data[offset]
		return v, selector == 0x28 && ok
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EFlags&DF != 0 {
		t.Fatal("CLD did not clear DF")
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDI] != 0x83 || c.R[ECX] != 1 || c.EFlags&ZF != 0 {
		t.Fatalf("EDI=%X ECX=%X flags=%X", c.R[EDI], c.R[ECX], c.EFlags)
	}

	c = New(testBus{0xf3, 0xae})
	c.R[ECX], c.R[EDI] = 0, 0x81
	c.Seg[SegES] = 0x99
	if err := c.Step(); err != nil || c.R[EDI] != 0x81 {
		t.Fatalf("zero-count SCASB read memory or moved EDI: err=%v EDI=%X", err, c.R[EDI])
	}
}

func TestLEARegisterSignedDisp8DoesNotReadMemory(t *testing.T) {
	mem := testBus{0x8d, 0x77, 0xff}
	c := New(mem)
	c.R[EDI] = 0x81
	c.EFlags = 0x246
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESI] != 0x80 || c.EIP != 3 || c.EFlags != 0x246 {
		t.Fatalf("ESI=%X EIP=%d flags=%X", c.R[ESI], c.EIP, c.EFlags)
	}
}

func TestBufferFinalizeInstructions(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0x2a, 0xc0, 0xaa, 0x5e, 0x4f})
	c := New(mem)
	c.R[EAX], c.R[EDI], c.R[ESP] = 0x20, 0x40, 0x70
	c.Seg[SegES], c.Seg[SegSS] = 0x160, 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0xff, Writable: true})
	mem[0x70], mem[0x71], mem[0x72], mem[0x73] = 0x78, 0x56, 0x34, 0x12
	for range 4 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.R[EAX] != 0 || mem[0x40] != 0 || c.R[ESI] != 0x12345678 || c.R[ESP] != 0x74 || c.R[EDI] != 0x40 {
		t.Fatalf("EAX=%X ESI=%X ESP=%X EDI=%X", c.R[EAX], c.R[ESI], c.R[ESP], c.R[EDI])
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
