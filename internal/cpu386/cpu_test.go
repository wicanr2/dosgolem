package cpu386

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestStepHookHandledUnhandledAndError(t *testing.T) {
	c := New(testBus{0xfb})
	c.StepHook = func(cpu *CPU) (bool, error) {
		cpu.EIP = 7
		return true, nil
	}
	if err := c.Step(); err != nil || c.EIP != 7 {
		t.Fatalf("handled hook EIP=%d err=%v", c.EIP, err)
	}

	c = New(testBus{0xfb})
	c.StepHook = func(*CPU) (bool, error) { return false, nil }
	if err := c.Step(); err != nil || c.EIP != 1 {
		t.Fatalf("unhandled hook did not decode opcode: EIP=%d err=%v", c.EIP, err)
	}

	want := errors.New("runtime hook failure")
	c = New(testBus{0xfb})
	c.StepHook = func(*CPU) (bool, error) { return true, want }
	if err := c.Step(); !errors.Is(err, want) || c.EIP != 0 {
		t.Fatalf("hook error=%v EIP=%d", err, c.EIP)
	}
}

func TestRegisterTEST32(t *testing.T) {
	c := New(testBus{0x85, 0xc0})
	c.R[EAX] = 0x80000000
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x80000000 || c.EFlags&SF == 0 || c.EFlags&ZF != 0 || c.EFlags&(CF|OF) != 0 {
		t.Fatalf("nonzero TEST EAX=%X flags=%X", c.R[EAX], c.EFlags)
	}

	c = New(testBus{0x85, 0xc8})
	c.R[EAX], c.R[ECX] = 1, 2
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 1 || c.R[ECX] != 2 || c.EFlags&ZF == 0 {
		t.Fatalf("zero TEST EAX=%X ECX=%X flags=%X", c.R[EAX], c.R[ECX], c.EFlags)
	}

	c = New(testBus{0x85, 0x00})
	if err := c.Step(); err == nil {
		t.Fatal("memory TEST shape was accepted")
	}
}

func TestByteTESTAtBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xf6, 0x45, 0xfc, 0x02})
	mem[0x20] = 0x02
	c := New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBP], c.EFlags = 0x24, CF|OF|AF
	if err := c.Step(); err != nil || c.EFlags&(CF|OF|ZF) != 0 || mem[0x20] != 0x02 {
		t.Fatalf("nonzero TEST byte flags=%X value=%X err=%v", c.EFlags, mem[0x20], err)
	}

	mem[0x20] = 0
	c = New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBP] = 0x24
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 {
		t.Fatalf("zero TEST byte flags=%X err=%v", c.EFlags, err)
	}

	c = New(testBus{0xf6, 0x45, 0xfc, 0x02})
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 3, Writable: true})
	c.R[EBP] = 0x24
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range TEST byte was accepted")
	}
}

func TestByteTESTAtBaseDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x60))
	copy(mem, []byte{0xf6, 0x83, 0xf0, 0xff, 0xff, 0xff, 0x02})
	mem[0x20] = 0x02
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f})
	c.R[EBX], c.EFlags = 0x30, CF|OF|AF
	if err := c.Step(); err != nil || c.R[EBX] != 0x30 || c.EFlags&(CF|OF|ZF) != 0 || mem[0x20] != 0x02 {
		t.Fatalf("nonzero TEST EBX=%X flags=%X value=%X err=%v", c.R[EBX], c.EFlags, mem[0x20], err)
	}

	mem[0x20] = 0
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f})
	c.R[EBX] = 0x30
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 {
		t.Fatalf("zero TEST flags=%X err=%v", c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0xf6, 0x85, 0xfc, 0xff, 0xff, 0xff, 0x80})
	mem[0x1c] = 0x80
	c = New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Limit: 0x3f})
	c.R[EBP] = 0x20
	if err := c.Step(); err != nil || c.EFlags&SF == 0 {
		t.Fatalf("SS TEST flags=%X err=%v", c.EFlags, err)
	}

	c = New(testBus{0xf6, 0x83, 0xf0, 0xff, 0xff, 0xff, 0x02})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 6})
	c.R[EBX] = 0x30
	if err := c.Step(); err == nil || c.R[EBX] != 0x30 {
		t.Fatalf("out-of-range TEST EBX=%X err=%v", c.R[EBX], err)
	}
}

func TestByteTESTAtSIBDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x60))
	copy(mem, []byte{0xf6, 0x44, 0x03, 0xff, 0x40})
	mem[0x2f] = 0x40
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x5f, Writable: true})
	c.R[EBX], c.R[EAX], c.EFlags = 0x20, 0x10, CF|OF|AF
	if err := c.Step(); err != nil || c.EFlags&(CF|OF|AF|ZF) != 0 || mem[0x2f] != 0x40 {
		t.Fatalf("nonzero TEST byte flags=%X value=%X err=%v", c.EFlags, mem[0x2f], err)
	}

	mem[0x2f] = 0
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x5f})
	c.R[EBX], c.R[EAX] = 0x20, 0x10
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 {
		t.Fatalf("zero TEST byte flags=%X err=%v", c.EFlags, err)
	}

	c = New(testBus{0xf6, 0x44, 0x03, 0x01, 0x40})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 4})
	c.R[EBX], c.R[EAX] = 0x20, 0x10
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range SIB TEST byte was accepted")
	}

	c = New(testBus{0xf6, 0x44, 0x23, 0x01, 0x40})
	if err := c.Step(); err == nil {
		t.Fatal("unapproved SIB TEST byte was accepted")
	}
}

func TestByteTESTRegister(t *testing.T) {
	for _, test := range []struct {
		name string
		code []byte
		reg  int
		set  uint32
		zero bool
	}{
		{name: "DL zero", code: []byte{0xf6, 0xc2, 0x80}, reg: EDX, set: 0xa5a5007f, zero: true},
		{name: "AH nonzero", code: []byte{0xf6, 0xc4, 0x80}, reg: EAX, set: 0x12348000},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := New(testBus(test.code))
			c.R[test.reg], c.EFlags = test.set, CF|OF|AF
			before := c.R[test.reg]
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.R[test.reg] != before || (c.EFlags&ZF != 0) != test.zero || c.EFlags&(CF|OF|AF) != 0 {
				t.Fatalf("register=%X flags=%X", c.R[test.reg], c.EFlags)
			}
		})
	}

	c := New(testBus{0xf6, 0xda})
	if err := c.Step(); err == nil {
		t.Fatal("unapproved F6 register group was accepted")
	}
}

func TestAbsoluteByteANDOR(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x80, 0x25, 0x20, 0, 0, 0, 0xf8, 0x80, 0x0d, 0x20, 0, 0, 0, 4})
	mem[0x20] = 0xff
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || mem[0x20] != 0xf8 {
		t.Fatalf("AND byte=%X err=%v", mem[0x20], err)
	}
	if err := c.Step(); err != nil || mem[0x20] != 0xfc || c.EFlags&ZF != 0 {
		t.Fatalf("OR byte=%X flags=%X err=%v", mem[0x20], c.EFlags, err)
	}
}

func TestByteORAtSIBDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x60))
	copy(mem, []byte{0x80, 0x4c, 0x03, 0xff, 0x40})
	mem[0x2f] = 0x01
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x5f, Writable: true})
	c.R[EBX], c.R[EAX], c.EFlags = 0x20, 0x10, CF|OF|AF
	if err := c.Step(); err != nil || mem[0x2f] != 0x41 || c.EFlags&(CF|OF|AF|ZF) != 0 {
		t.Fatalf("OR byte=%X flags=%X err=%v", mem[0x2f], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x60))
	copy(mem, []byte{0x80, 0x4c, 0x03, 0xff, 0x40})
	mem[0x2f] = 1
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x5f})
	c.R[EBX], c.R[EAX] = 0x20, 0x10
	if err := c.Step(); err == nil || mem[0x2f] != 1 {
		t.Fatalf("read-only OR byte=%X err=%v", mem[0x2f], err)
	}

	c = New(testBus{0x80, 0x4c, 0x23, 0x01, 0x40})
	if err := c.Step(); err == nil {
		t.Fatal("unapproved SIB OR byte was accepted")
	}
}

func TestByteORAtBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x60))
	copy(mem, []byte{0x80, 0x4b, 0xfc, 0x08})
	mem[0x2c] = 0x40
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f, Writable: true})
	c.R[EBX], c.EFlags = 0x30, CF|OF|AF
	if err := c.Step(); err != nil || mem[0x2c] != 0x48 || c.EFlags&(CF|OF|AF|ZF) != 0 {
		t.Fatalf("OR byte=%X flags=%X err=%v", mem[0x2c], c.EFlags, err)
	}

	mem[0x2c] = 0x40
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f})
	c.R[EBX] = 0x30
	if err := c.Step(); err == nil || mem[0x2c] != 0x40 {
		t.Fatalf("read-only OR byte=%X err=%v", mem[0x2c], err)
	}
}

func TestCompareDwordAtBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x83, 0x7b, 0x0c, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.R[EBX] = 0x10
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[EBX] != 0x10 {
		t.Fatalf("CMP EBX=%X flags=%X err=%v", c.R[EBX], c.EFlags, err)
	}
}

func TestCompareDwordAtBaseDisp8Immediate32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x81, 0x7d, 0xfc, 0x78, 0x56, 0x34, 0x12})
	copy(mem[0x20:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBP] = 0x24
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[EBP] != 0x24 {
		t.Fatalf("CMP base+disp8 imm32 EBP=%X flags=%X err=%v", c.R[EBP], c.EFlags, err)
	}

	c = New(testBus{0x81, 0x7d, 0xfc, 0x78, 0x56, 0x34, 0x12})
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 6, Writable: true})
	c.R[EBP] = 0x24
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range base+disp8 imm32 CMP was accepted")
	}
}

func TestCompareDwordAtBaseDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x83, 0xbb, 0xfc, 0xff, 0xff, 0xff, 0xff})
	copy(mem[0x20:], []byte{0xff, 0xff, 0xff, 0xff})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBX] = 0x24
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[EBX] != 0x24 {
		t.Fatalf("CMP base+disp32 EBX=%X flags=%X err=%v", c.R[EBX], c.EFlags, err)
	}

	c = New(testBus{0x83, 0xbb, 0xfc, 0xff, 0xff, 0xff, 0xff})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 6, Writable: true})
	c.R[EBX] = 0x24
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range base+disp32 CMP was accepted")
	}
}

func TestStoreDwordAtBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x89, 0x58, 0x04})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.R[EAX], c.R[EBX] = 0x10, 0x12345678
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x14:0x18], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("store=% X err=%v", mem[0x14:0x18], err)
	}
}

func TestStoreDwordAtBaseDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x89, 0x83, 0xfc, 0xff, 0xff, 0xff})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBX], c.R[EAX] = 0x24, 0x12345678
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("base+disp32 store=% X err=%v", mem[0x20:0x24], err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0x89, 0x83, 0xfc, 0xff, 0xff, 0xff})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f})
	c.R[EBX], c.R[EAX] = 0x24, 0x12345678
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x20:0x24], []byte{0, 0, 0, 0}) {
		t.Fatalf("read-only base+disp32 store=% X err=%v", mem[0x20:0x24], err)
	}
}

func TestStoreRegisterToStackDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x89, 0x4c, 0x24, 0xfc})
	c := New(mem)
	c.R[ESP], c.R[ECX] = 0x20, 0x12345678
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x1c:0x20], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("stack store=% X err=%v", mem[0x1c:0x20], err)
	}
}

func TestLoadRegisterFromAbsoluteAddress(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x8b, 0x15, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || c.R[EDX] != 0x12345678 {
		t.Fatalf("EDX=%X err=%v", c.R[EDX], err)
	}
}

func TestLoadDwordFromBaseScaledIndex(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x8b, 0x04, 0xb0})
	copy(mem[0x40:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x7f})
	c.R[EAX], c.R[ESI], c.EFlags = 0x20, 8, CF|ZF|OF
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345678 || c.R[ESI] != 8 || c.EFlags != CF|ZF|OF {
		t.Fatalf("MOV EAX=%X ESI=%X flags=%X err=%v", c.R[EAX], c.R[ESI], c.EFlags, err)
	}

	c = New(testBus{0x8b, 0x04, 0xb0})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 2})
	c.R[EAX], c.R[ESI] = 0x20, 8
	if err := c.Step(); err == nil || c.R[EAX] != 0x20 {
		t.Fatalf("out-of-range scaled load EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestStoreDwordToBaseScaledIndex(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x89, 0x14, 0x98})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x7f, Writable: true})
	c.R[EAX], c.R[EBX], c.R[EDX], c.EFlags = 0x20, 8, 0x12345678, CF|ZF|OF
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x40:0x44], []byte{0x78, 0x56, 0x34, 0x12}) || c.R[EAX] != 0x20 || c.R[EBX] != 8 || c.R[EDX] != 0x12345678 || c.EFlags != CF|ZF|OF {
		t.Fatalf("store=% X EAX=%X EBX=%X EDX=%X flags=%X err=%v", mem[0x40:0x44], c.R[EAX], c.R[EBX], c.R[EDX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x80))
	copy(mem, []byte{0x89, 0x14, 0x98})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x7f})
	c.R[EAX], c.R[EBX], c.R[EDX] = 0x20, 8, 0x12345678
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x40:0x44], []byte{0, 0, 0, 0}) {
		t.Fatalf("read-only store=% X err=%v", mem[0x40:0x44], err)
	}
}

func TestRegisterMOV16(t *testing.T) {
	c := New(testBus{0x66, 0x8b, 0xca})
	c.R[ECX], c.R[EDX], c.EFlags = 0xaabbccdd, 0x11223344, CF|ZF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ECX] != 0xaabb3344 || c.R[EDX] != 0x11223344 || c.EFlags != CF|ZF {
		t.Fatalf("MOV CX,DX ECX=%X EDX=%X flags=%X", c.R[ECX], c.R[EDX], c.EFlags)
	}
}

func TestStoreRegisterMOV16(t *testing.T) {
	c := New(testBus{0x66, 0x89, 0xc3})
	c.R[EAX], c.R[EBX], c.EFlags = 0x11223344, 0xaabbccdd, CF|ZF|OF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x11223344 || c.R[EBX] != 0xaabb3344 || c.EFlags != CF|ZF|OF {
		t.Fatalf("MOV BX,AX EAX=%X EBX=%X flags=%X", c.R[EAX], c.R[EBX], c.EFlags)
	}
}

func TestLoadRegisterFromStackDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x8b, 0x4c, 0x24, 0x04})
	copy(mem[0x24:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.R[ESP], c.R[ECX] = 0x20, 0xaabbccdd
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.R[ECX] != 0x12345678 {
		t.Fatalf("MOV ECX=%X err=%v", c.R[ECX], err)
	}
}

func TestLoadRegisterFromStackDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x50))
	copy(mem, []byte{0x8b, 0x94, 0x24, 0x20, 0x00, 0x00, 0x00})
	copy(mem[0x30:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.R[ESP], c.R[EDX], c.EFlags = 0x10, 0xdeadbeef, CF|ZF
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x4f, Writable: true})
	if err := c.Step(); err != nil || c.R[EDX] != 0x12345678 || c.R[ESP] != 0x10 || c.EFlags != CF|ZF {
		t.Fatalf("positive MOV EDX=%X ESP=%X flags=%X err=%v", c.R[EDX], c.R[ESP], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0x8b, 0x8c, 0x24, 0xf8, 0xff, 0xff, 0xff})
	copy(mem[0x18:], []byte{0xef, 0xcd, 0xab, 0x89})
	c = New(mem)
	c.R[ESP] = 0x20
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.R[ECX] != 0x89abcdef || c.R[ESP] != 0x20 {
		t.Fatalf("negative MOV ECX=%X ESP=%X err=%v", c.R[ECX], c.R[ESP], err)
	}
}

func TestLoadRegisterFromBaseDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x8b, 0x83, 0xfc, 0xff, 0xff, 0xff})
	copy(mem[0x20:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBX], c.R[EAX] = 0x24, 0xdeadbeef
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345678 || c.R[EBX] != 0x24 {
		t.Fatalf("base+disp32 load EAX=%X EBX=%X err=%v", c.R[EAX], c.R[EBX], err)
	}

	c = New(testBus{0x8b, 0x83, 0xfc, 0xff, 0xff, 0xff})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 5, Writable: true})
	c.R[EBX], c.R[EAX] = 0x24, 0xdeadbeef
	if err := c.Step(); err == nil || c.R[EAX] != 0xdeadbeef {
		t.Fatalf("out-of-range base+disp32 load EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestLEAStackDisp8(t *testing.T) {
	c := New(testBus{0x8d, 0x44, 0x24, 0xfc})
	c.R[ESP] = 0x20
	if err := c.Step(); err != nil || c.R[EAX] != 0x1c {
		t.Fatalf("LEA EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestLEAStackDisp32(t *testing.T) {
	c := New(testBus{0x8d, 0x84, 0x24, 0x08, 0x01, 0x00, 0x00})
	c.R[ESP], c.EFlags = 0x1000, CF|ZF
	if err := c.Step(); err != nil || c.R[EAX] != 0x1108 || c.R[ESP] != 0x1000 || c.EFlags != CF|ZF {
		t.Fatalf("positive LEA EAX=%X ESP=%X flags=%X err=%v", c.R[EAX], c.R[ESP], c.EFlags, err)
	}

	c = New(testBus{0x8d, 0x8c, 0x24, 0xf8, 0xff, 0xff, 0xff})
	c.R[ESP] = 0x1000
	if err := c.Step(); err != nil || c.R[ECX] != 0xff8 {
		t.Fatalf("negative LEA ECX=%X err=%v", c.R[ECX], err)
	}
}

func TestStoreRegisterIndirect(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x89, 0x10})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.R[EAX], c.R[EDX] = 0x20, 0x12345678
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("store=% X err=%v", mem[0x20:0x24], err)
	}
}

func TestStoreImmediateDwordAbsolute(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0xc7, 0x05, 0x20, 0, 0, 0, 0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("store=% X err=%v", mem[0x20:0x24], err)
	}
}

func TestStoreImmediateDwordAtBaseDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xc7, 0x83, 0xfc, 0xff, 0xff, 0xff, 0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBX] = 0x24
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("immediate base+disp32 store=% X err=%v", mem[0x20:0x24], err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0xc7, 0x83, 0xfc, 0xff, 0xff, 0xff, 0x78, 0x56, 0x34, 0x12})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f})
	c.R[EBX] = 0x24
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x20:0x24], []byte{0, 0, 0, 0}) {
		t.Fatalf("read-only immediate base+disp32 store=% X err=%v", mem[0x20:0x24], err)
	}
}

func TestSubtractRegisterAbsolute(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x2b, 0x05, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{4, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.R[EAX] = 7
	if err := c.Step(); err != nil || c.R[EAX] != 3 || c.EFlags&ZF != 0 {
		t.Fatalf("SUB EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestRegisterSHL32(t *testing.T) {
	c := New(testBus{0xc1, 0xe0, 2})
	c.R[EAX] = 0x40000001
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 4 || c.EFlags&CF == 0 || c.EFlags&ZF != 0 {
		t.Fatalf("SHL EAX=%X flags=%X", c.R[EAX], c.EFlags)
	}
}

func TestRegisterRCLAndROR32ByOne(t *testing.T) {
	preserved := uint32(SF | ZF | AF | PF)
	for _, test := range []struct {
		name      string
		value     uint32
		carryIn   uint32
		want      uint32
		wantFlags uint32
	}{
		{name: "DOS success", value: 5, want: 5, wantFlags: preserved},
		{name: "DOS error", value: 2, carryIn: CF, want: 0x80000002, wantFlags: preserved | CF | OF},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := New(testBus{0xd1, 0xd0, 0xd1, 0xc8})
			c.R[EAX], c.EFlags = test.value, preserved|test.carryIn
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.R[EAX] != test.want || c.EFlags&(SF|ZF|AF|PF|CF|OF) != test.wantFlags {
				t.Fatalf("EAX=%X flags=%X", c.R[EAX], c.EFlags)
			}
		})
	}

	for _, code := range [][]byte{{0xd1, 0x10}, {0xd1, 0xe0}, {0x66, 0xd1, 0xd0}} {
		c := New(testBus(code))
		if err := c.Step(); err == nil {
			t.Fatalf("未授權 D1 形狀 % X 被接受", code)
		}
	}
}

func TestRegisterADD32(t *testing.T) {
	c := New(testBus{0x01, 0xc6})
	c.R[ESI], c.R[EAX] = 3, 5
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESI] != 8 || c.R[EAX] != 5 || c.EFlags&ZF != 0 {
		t.Fatalf("ADD ESI=%X EAX=%X flags=%X", c.R[ESI], c.R[EAX], c.EFlags)
	}
}

func TestAddRegisterStackDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x03, 0x44, 0x24, 0x08})
	copy(mem[0x28:], []byte{5, 0, 0, 0})
	c := New(mem)
	c.R[ESP], c.R[EAX] = 0x20, 7
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.R[EAX] != 12 || c.EFlags&ZF != 0 {
		t.Fatalf("ADD EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestRegisterCMP32(t *testing.T) {
	c := New(testBus{0x39, 0xc3})
	c.R[EBX], c.R[EAX] = 7, 7
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[EBX] != 7 || c.R[EAX] != 7 {
		t.Fatalf("CMP EBX=%X EAX=%X flags=%X err=%v", c.R[EBX], c.R[EAX], c.EFlags, err)
	}
}

func TestCompareRegisterAbsoluteDword(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x3b, 0x0d, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{7, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	c.R[ECX] = 7
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[ECX] != 7 {
		t.Fatalf("CMP ECX=%X flags=%X err=%v", c.R[ECX], c.EFlags, err)
	}
}

func TestJBEShort(t *testing.T) {
	for _, flags := range []uint32{CF, ZF} {
		c := New(testBus{0x76, 2, 0xfb, 0xfb})
		c.EFlags = flags
		if err := c.Step(); err != nil || c.EIP != 4 || c.EFlags != flags {
			t.Fatalf("taken JBE flags=%X EIP=%X err=%v", flags, c.EIP, err)
		}
	}
	c := New(testBus{0x76, 2, 0xfb, 0xfb})
	if err := c.Step(); err != nil || c.EIP != 2 {
		t.Fatalf("untaken JBE EIP=%X err=%v", c.EIP, err)
	}
}

func TestJLShort(t *testing.T) {
	for _, flags := range []uint32{SF, OF} {
		c := New(testBus{0x7c, 2, 0xfb, 0xfb})
		c.EFlags = flags
		if err := c.Step(); err != nil || c.EIP != 4 || c.EFlags != flags {
			t.Fatalf("taken JL flags=%X EIP=%X err=%v", flags, c.EIP, err)
		}
	}
	for _, flags := range []uint32{0, SF | OF} {
		c := New(testBus{0x7c, 2, 0xfb, 0xfb})
		c.EFlags = flags
		if err := c.Step(); err != nil || c.EIP != 2 || c.EFlags != flags {
			t.Fatalf("untaken JL flags=%X EIP=%X err=%v", flags, c.EIP, err)
		}
	}
}

func TestRegisterSUB32(t *testing.T) {
	c := New(testBus{0x29, 0xc4})
	c.R[ESP], c.R[EAX] = 9, 4
	if err := c.Step(); err != nil || c.R[ESP] != 5 || c.R[EAX] != 4 || c.EFlags&ZF != 0 {
		t.Fatalf("SUB ESP=%X EAX=%X flags=%X err=%v", c.R[ESP], c.R[EAX], c.EFlags, err)
	}
}

func TestNegRegister32(t *testing.T) {
	c := New(testBus{0xf7, 0xd8})
	c.R[EAX] = 5
	if err := c.Step(); err != nil || c.R[EAX] != 0xfffffffb || c.EFlags&CF == 0 {
		t.Fatalf("NEG EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}

	c = New(testBus{0xf7, 0xd8})
	if err := c.Step(); err != nil || c.R[EAX] != 0 || c.EFlags&ZF == 0 || c.EFlags&CF != 0 {
		t.Fatalf("NEG zero EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}

	c = New(testBus{0xf7, 0xd8})
	c.R[EAX] = 0x80000000
	if err := c.Step(); err != nil || c.R[EAX] != 0x80000000 || c.EFlags&OF == 0 {
		t.Fatalf("NEG min EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}

	c = New(testBus{0xf7, 0xc0})
	if err := c.Step(); err == nil {
		t.Fatal("unsupported F7 group was accepted")
	}
}

func TestNotRegister32(t *testing.T) {
	c := New(testBus{0xf7, 0xd1})
	c.R[ECX], c.EFlags = 0xfffffff5, CF|ZF|OF
	if err := c.Step(); err != nil || c.R[ECX] != 0x0a || c.EFlags != CF|ZF|OF {
		t.Fatalf("NOT ECX=%X flags=%X err=%v", c.R[ECX], c.EFlags, err)
	}
}

func TestDecrementAbsoluteDword(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0xff, 0x0d, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{1, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.EFlags = CF
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0, 0, 0, 0}) || c.EFlags&ZF == 0 || c.EFlags&CF == 0 {
		t.Fatalf("DEC value=% X flags=%X err=%v", mem[0x20:0x24], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x30))
	copy(mem, []byte{0xff, 0x0d, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{1, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	c.EFlags = CF | OF
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x20:0x24], []byte{1, 0, 0, 0}) || c.EFlags != CF|OF {
		t.Fatalf("read-only DEC value=% X flags=%X err=%v", mem[0x20:0x24], c.EFlags, err)
	}
}

func TestDecrementDwordAtBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x60))
	copy(mem, []byte{0xff, 0x4b, 0xfc})
	copy(mem[0x2c:], []byte{1, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f, Writable: true})
	c.R[EBX], c.EFlags = 0x30, CF
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x2c:0x30], []byte{0, 0, 0, 0}) || c.EFlags&ZF == 0 || c.EFlags&CF == 0 {
		t.Fatalf("DEC value=% X flags=%X err=%v", mem[0x2c:0x30], c.EFlags, err)
	}

	copy(mem[0x2c:], []byte{0, 0, 0, 0x80})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f, Writable: true})
	c.R[EBX] = 0x30
	if err := c.Step(); err != nil || binary.LittleEndian.Uint32(mem[0x2c:0x30]) != 0x7fffffff || c.EFlags&OF == 0 {
		t.Fatalf("DEC overflow value=%X flags=%X err=%v", binary.LittleEndian.Uint32(mem[0x2c:0x30]), c.EFlags, err)
	}

	copy(mem[0x2c:], []byte{1, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x5f})
	c.R[EBX] = 0x30
	if err := c.Step(); err == nil || binary.LittleEndian.Uint32(mem[0x2c:0x30]) != 1 {
		t.Fatalf("read-only DEC value=%X err=%v", binary.LittleEndian.Uint32(mem[0x2c:0x30]), err)
	}
}

func TestIncrementAbsoluteDword(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0xff, 0x05, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{0xff, 0xff, 0xff, 0xff})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.EFlags = CF
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0, 0, 0, 0}) || c.EFlags&ZF == 0 || c.EFlags&CF == 0 {
		t.Fatalf("INC value=% X flags=%X err=%v", mem[0x20:0x24], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x30))
	copy(mem, []byte{0xff, 0x05, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{1, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	c.EFlags = CF | OF
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x20:0x24], []byte{1, 0, 0, 0}) || c.EFlags != CF|OF {
		t.Fatalf("read-only INC value=% X flags=%X err=%v", mem[0x20:0x24], c.EFlags, err)
	}
}

func TestAndEAXImmediate32(t *testing.T) {
	c := New(testBus{0x25, 0xff, 0xff, 0x00, 0x00})
	c.R[EAX], c.EFlags = 0x12345678, CF|OF|AF
	if err := c.Step(); err != nil || c.R[EAX] != 0x5678 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("AND EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestAndRegisterImmediate32(t *testing.T) {
	c := New(testBus{0x81, 0xe2, 0xff, 0xff, 0x00, 0x00})
	c.R[EDX], c.EFlags = 0x12345678, CF|OF|AF
	if err := c.Step(); err != nil || c.R[EDX] != 0x5678 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("AND EDX=%X flags=%X err=%v", c.R[EDX], c.EFlags, err)
	}
}

func TestCompareRegisterImmediate32(t *testing.T) {
	for _, tc := range []struct {
		name  string
		left  uint32
		flags uint32
	}{
		{name: "less", left: 0x10, flags: CF | SF},
		{name: "equal", left: 0x20, flags: ZF},
		{name: "greater", left: 0x30, flags: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(testBus{0x81, 0xfb, 0x20, 0x00, 0x00, 0x00})
			c.R[EBX], c.EFlags = tc.left, OF|AF
			if err := c.Step(); err != nil || c.R[EBX] != tc.left || c.EFlags&(CF|ZF|SF) != tc.flags {
				t.Fatalf("CMP EBX=%X flags=%X err=%v", c.R[EBX], c.EFlags, err)
			}
		})
	}
}

func TestJumpGreaterShort(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  []byte
		flags uint32
		want  uint32
	}{
		{name: "greater", code: []byte{0x7f, 0x03}, want: 5},
		{name: "equal", code: []byte{0x7f, 0x03}, flags: ZF, want: 2},
		{name: "less", code: []byte{0x7f, 0x03}, flags: SF, want: 2},
		{name: "negative", code: []byte{0x7f, 0xfe}, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(testBus(tc.code))
			c.EFlags = tc.flags
			if err := c.Step(); err != nil || c.EIP != tc.want || c.EFlags != tc.flags {
				t.Fatalf("JG EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
			}
		})
	}
}

func TestSubtractRegisterImmediate32(t *testing.T) {
	c := New(testBus{0x81, 0xec, 0x18, 0x01, 0x00, 0x00})
	c.R[ESP] = 0x1000
	if err := c.Step(); err != nil || c.R[ESP] != 0xee8 || c.EFlags&CF != 0 {
		t.Fatalf("SUB ESP=%X flags=%X err=%v", c.R[ESP], c.EFlags, err)
	}

	c = New(testBus{0x81, 0xe8, 0x01, 0x00, 0x00, 0x00})
	c.R[EAX] = 0
	if err := c.Step(); err != nil || c.R[EAX] != 0xffffffff || c.EFlags&CF == 0 || c.EFlags&SF == 0 {
		t.Fatalf("borrow SUB EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestPushAbsoluteDword(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xff, 0x35, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[ESP] = 0x40
	if err := c.Step(); err != nil || c.R[ESP] != 0x3c || !bytes.Equal(mem[0x3c:0x40], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("PUSH ESP=%X stack=% X err=%v", c.R[ESP], mem[0x3c:0x40], err)
	}

	mem = testBus([]byte{0xff, 0x35, 0x20, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 5, Writable: true})
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 5, Writable: true})
	c.R[ESP] = 4
	if err := c.Step(); err == nil || c.R[ESP] != 4 {
		t.Fatalf("out-of-range PUSH ESP=%X err=%v", c.R[ESP], err)
	}
}

func TestPushBaseDisp8Dword(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xff, 0x75, 0x08})
	copy(mem[0x18:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBP], c.R[ESP] = 0x10, 0x40
	if err := c.Step(); err != nil || c.R[ESP] != 0x3c || !bytes.Equal(mem[0x3c:0x40], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("PUSH ESP=%X stack=% X err=%v", c.R[ESP], mem[0x3c:0x40], err)
	}

	mem = testBus([]byte{0xff, 0x75, 0x08})
	c = New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 2, Writable: true})
	c.R[EBP], c.R[ESP] = 0x10, 4
	if err := c.Step(); err == nil || c.R[ESP] != 4 {
		t.Fatalf("out-of-range PUSH ESP=%X err=%v", c.R[ESP], err)
	}
}

func TestPushBaseDword(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xff, 0x33})
	copy(mem[0x18:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EBX], c.R[ESP] = 0x18, 0x40
	if err := c.Step(); err != nil || c.R[ESP] != 0x3c || !bytes.Equal(mem[0x3c:0x40], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("PUSH ESP=%X stack=% X err=%v", c.R[ESP], mem[0x3c:0x40], err)
	}

	mem = testBus([]byte{0xff, 0x33})
	c = New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 1, Writable: true})
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 1, Writable: true})
	c.R[EBX], c.R[ESP] = 0x18, 4
	if err := c.Step(); err == nil || c.R[ESP] != 4 {
		t.Fatalf("out-of-range PUSH ESP=%X err=%v", c.R[ESP], err)
	}

	mem = testBus(make([]byte, 0x20))
	copy(mem, []byte{0xff, 0x33})
	copy(mem[0x10:], []byte{0x78, 0x56, 0x34, 0x12})
	c = New(mem)
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x1f})
	c.R[EBX], c.R[ESP] = 0x10, 0x20
	if err := c.Step(); err == nil || c.R[ESP] != 0x20 {
		t.Fatalf("read-only stack PUSH ESP=%X err=%v", c.R[ESP], err)
	}
}

func TestIncrementBaseDword(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0xff, 0x03})
	copy(mem[0x10:], []byte{0xff, 0xff, 0xff, 0x7f})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	c.R[EBX], c.EFlags = 0x10, CF
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x10:0x14], []byte{0, 0, 0, 0x80}) || c.EFlags&CF == 0 || c.EFlags&OF == 0 || c.EFlags&SF == 0 {
		t.Fatalf("INC value=% X flags=%X err=%v", mem[0x10:0x14], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x20))
	copy(mem, []byte{0xff, 0x03})
	copy(mem[0x10:], []byte{0x34, 0x12, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x1f})
	c.R[EBX] = 0x10
	if err := c.Step(); err == nil || !bytes.Equal(mem[0x10:0x14], []byte{0x34, 0x12, 0, 0}) {
		t.Fatalf("read-only INC value=% X err=%v", mem[0x10:0x14], err)
	}
}

func TestIncrementRegisterByte(t *testing.T) {
	tests := []struct {
		name      string
		code      byte
		before    uint32
		want      uint32
		wantFlags uint32
	}{
		{name: "AL overflow", code: 0xc0, before: 0x1234567f, want: 0x12345680, wantFlags: CF | OF | SF | AF},
		{name: "AH wraps", code: 0xc4, before: 0x1234ff78, want: 0x12340078, wantFlags: CF | ZF | AF | PF},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := New(testBus{0xfe, test.code})
			c.R[EAX], c.EFlags = test.before, CF
			if err := c.Step(); err != nil || c.R[EAX] != test.want || c.EFlags&(CF|OF|SF|ZF|AF|PF) != test.wantFlags {
				t.Fatalf("INC EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
			}
		})
	}

	c := New(testBus{0xfe, 0x00})
	if err := c.Step(); err == nil {
		t.Fatal("memory FE /0 was accepted")
	}
}

func TestPushFlags32(t *testing.T) {
	mem := testBus(make([]byte, 0x20))
	mem[0] = 0x9c
	c := New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.R[ESP], c.EFlags = 0x20, 0x246
	if err := c.Step(); err != nil || c.R[ESP] != 0x1c || !bytes.Equal(mem[0x1c:0x20], []byte{0x46, 0x02, 0, 0}) || c.EFlags != 0x246 {
		t.Fatalf("PUSHFD ESP=%X stack=% X flags=%X err=%v", c.R[ESP], mem[0x1c:0x20], c.EFlags, err)
	}

	c = New(testBus{0x9c})
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0})
	c.R[ESP], c.EFlags = 4, 0x246
	if err := c.Step(); err == nil || c.R[ESP] != 4 || c.EFlags != 0x246 {
		t.Fatalf("failed PUSHFD ESP=%X flags=%X err=%v", c.R[ESP], c.EFlags, err)
	}
}

func TestPopFlags32(t *testing.T) {
	mem := testBus(make([]byte, 0x20))
	mem[0] = 0x9d
	copy(mem[0x10:], []byte{0x46, 0x02, 0x00, 0x00})
	c := New(mem)
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.R[ESP], c.EFlags = 0x10, CF|OF
	if err := c.Step(); err != nil || c.R[ESP] != 0x14 || c.EFlags != 0x246 {
		t.Fatalf("POPFD ESP=%X flags=%X err=%v", c.R[ESP], c.EFlags, err)
	}

	c = New(testBus{0x9d})
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0, Limit: 0})
	c.R[ESP], c.EFlags = 4, 0x246
	if err := c.Step(); err == nil || c.R[ESP] != 4 || c.EFlags != 0x246 {
		t.Fatalf("failed POPFD ESP=%X flags=%X err=%v", c.R[ESP], c.EFlags, err)
	}
}

func TestClearInterruptFlag(t *testing.T) {
	c := New(testBus{0xfa})
	c.EFlags = IF | CF | ZF
	if err := c.Step(); err != nil || c.EFlags != CF|ZF {
		t.Fatalf("CLI flags=%X err=%v", c.EFlags, err)
	}
}

func TestStoreSegmentAbsoluteWord(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x66, 0x8c, 0x1d, 0x20, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x22], []byte{0x60, 0x01}) {
		t.Fatalf("MOV segment word=% X err=%v", mem[0x20:0x22], err)
	}

	mem = testBus([]byte{0x66, 0x8c, 0x1d, 0x20, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 6, Writable: true})
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range segment word store was accepted")
	}
}

func TestStoreSegmentRegister16(t *testing.T) {
	c := New(testBus{0x66, 0x8c, 0xc2})
	c.Seg[SegES] = 0x1234
	c.R[EDX], c.EFlags = 0xaabbccdd, CF|ZF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDX] != 0xaabb1234 || c.Seg[SegES] != 0x1234 || c.EFlags != CF|ZF {
		t.Fatalf("MOV DX,ES EDX=%X ES=%X flags=%X", c.R[EDX], c.Seg[SegES], c.EFlags)
	}
}

func TestLoadSegmentRegister16(t *testing.T) {
	c := New(testBus{0x66, 0x8e, 0xdb})
	c.R[EBX] = 0xaaaa0160
	c.SegmentLoadOK = func(selector uint16, destination int) bool {
		return selector == 0x160 && destination == SegDS
	}
	if err := c.Step(); err != nil || c.Seg[SegDS] != 0x160 {
		t.Fatalf("MOV DS,BX DS=%X err=%v", c.Seg[SegDS], err)
	}

	c = New(testBus{0x66, 0x8e, 0xdb})
	c.R[EBX] = 0x160
	c.SegmentLoadOK = func(uint16, int) bool { return false }
	if err := c.Step(); err == nil || c.Seg[SegDS] != 0 {
		t.Fatalf("rejected MOV DS,BX DS=%X err=%v", c.Seg[SegDS], err)
	}
}

func TestLoadSegmentAbsoluteWord(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x66, 0x8e, 0x05, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{0x28, 0})
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x160, 0x30
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	c.SetDescriptor(0x28, Descriptor{Base: 0, Limit: 0x2f})
	if err := c.Step(); err != nil || c.Seg[SegES] != 0x28 {
		t.Fatalf("MOV ES=%X err=%v", c.Seg[SegES], err)
	}

	mem[0x20], mem[0x21] = 0x99, 0
	c = New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x160, 0x30
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	if err := c.Step(); err == nil || c.Seg[SegES] != 0x30 {
		t.Fatalf("invalid selector MOV ES=%X err=%v", c.Seg[SegES], err)
	}
}

func TestRegisterCMP8(t *testing.T) {
	c := New(testBus{0x38, 0xf3})
	c.R[EBX], c.R[EDX] = 0x11, 0x1100
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || c.R[EBX] != 0x11 || c.R[EDX] != 0x1100 {
		t.Fatalf("CMP BL=%X DH=%X flags=%X err=%v", c.reg8(3), c.reg8(6), c.EFlags, err)
	}
}

func TestMoveEAXFromAbsoluteAddress(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0xa1, 0x20, 0, 0, 0})
	copy(mem[0x20:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345678 {
		t.Fatalf("MOV EAX=%X err=%v", c.R[EAX], err)
	}

	c = New(mem[:0x22])
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x21, Writable: true})
	c.R[EAX] = 0xabcdef01
	if err := c.Step(); err == nil || c.R[EAX] != 0xabcdef01 {
		t.Fatalf("out-of-range MOV EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestMoveDwordFromAbsoluteIndexedSIB(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x8b, 0x14, 0x85, 0x20, 0, 0, 0})
	copy(mem[0x28:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f})
	c.R[EAX], c.R[EDX] = 2, 0xabcdef01
	if err := c.Step(); err != nil || c.R[EDX] != 0x12345678 {
		t.Fatalf("indexed MOV EDX=%X err=%v", c.R[EDX], err)
	}

	mem = testBus([]byte{0x8b, 0x14, 0x85, 0x20, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 6})
	c.R[EAX], c.R[EDX] = 2, 0xabcdef01
	if err := c.Step(); err == nil || c.R[EDX] != 0xabcdef01 {
		t.Fatalf("out-of-range indexed MOV EDX=%X err=%v", c.R[EDX], err)
	}
}

func TestMoveDwordToAbsoluteIndexedSIB(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x89, 0x1c, 0x85, 0x20, 0, 0, 0})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EAX], c.R[EBX] = 2, 0x12345678
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x28:0x2c], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("indexed MOV store=% X err=%v", mem[0x28:0x2c], err)
	}

	mem = testBus([]byte{0x89, 0x1c, 0x85, 0x20, 0, 0, 0})
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 6, Writable: true})
	c.R[EAX], c.R[EBX] = 2, 0x12345678
	if err := c.Step(); err == nil {
		t.Fatal("out-of-range indexed MOV was accepted")
	}
}

func TestMoveImmediateDwordToSIBAddress(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xc7, 0x04, 0x01, 0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	c.R[EAX], c.R[ECX] = 0x10, 0x10
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("SIB store=% X", mem[0x20:0x24])
	}

	c = New(testBus{0xc7, 0x04, 0x25})
	if err := c.Step(); err == nil {
		t.Fatal("unsupported displacement-only SIB was accepted")
	}
}

func TestMoveImmediateDwordToStackSIB(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0xc7, 0x04, 0x24, 0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.R[ESP] = 0x20
	c.Seg[SegDS], c.Seg[SegSS] = 0x30, 0x38
	c.SetDescriptor(0x30, Descriptor{Base: 0x40, Limit: 0x3f, Writable: true})
	c.SetDescriptor(0x38, Descriptor{Base: 0, Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x20:0x24], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("stack SIB=% X err=%v", mem[0x20:0x24], err)
	}

	mem = testBus(make([]byte, 0x20))
	copy(mem, []byte{0xc7, 0x04, 0x24, 1, 0, 0, 0})
	c = New(mem)
	c.R[ESP] = 0x1e
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	if err := c.Step(); err == nil || mem[0x1e] != 0 || mem[0x1f] != 0 {
		t.Fatalf("out-of-range stack SIB=% X err=%v", mem[0x1e:], err)
	}
}

func TestPushSignExtendedByte(t *testing.T) {
	mem := testBus(make([]byte, 0x20))
	copy(mem, []byte{0x6a, 0x80})
	c := New(mem)
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.R[ESP] = 0x10
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 0x0c || !bytes.Equal(mem[0x0c:0x10], []byte{0x80, 0xff, 0xff, 0xff}) {
		t.Fatalf("PUSH ESP=%X value=% X", c.R[ESP], mem[0x0c:0x10])
	}
}

func TestPushImmediateDword(t *testing.T) {
	mem := testBus(make([]byte, 0x20))
	copy(mem, []byte{0x68, 0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.R[ESP] = 0x10
	if err := c.Step(); err != nil || c.R[ESP] != 0x0c || !bytes.Equal(mem[0x0c:0x10], []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("PUSH ESP=%X value=% X err=%v", c.R[ESP], mem[0x0c:0x10], err)
	}

	c = New(boundedTestBus{0x68, 1, 2, 3})
	c.R[ESP] = 0x10
	if err := c.Step(); err == nil || c.R[ESP] != 0x10 {
		t.Fatalf("truncated PUSH ESP=%X err=%v", c.R[ESP], err)
	}
}

func TestLeave32(t *testing.T) {
	mem := testBus(make([]byte, 0x20))
	mem[0] = 0xc9
	copy(mem[0x10:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x1f, Writable: true})
	c.R[EBP], c.R[ESP] = 0x10, 2
	if err := c.Step(); err != nil || c.R[EBP] != 0x12345678 || c.R[ESP] != 0x14 {
		t.Fatalf("LEAVE EBP=%X ESP=%X err=%v", c.R[EBP], c.R[ESP], err)
	}
}

type testBus []byte

func (b testBus) Read8(addr uint32) (uint8, error)      { return b[addr], nil }
func (b testBus) Write8(addr uint32, value uint8) error { b[addr] = value; return nil }

type boundedTestBus []byte

func (b boundedTestBus) Read8(addr uint32) (uint8, error) {
	if addr >= uint32(len(b)) {
		return 0, errors.New("out of range")
	}
	return b[addr], nil
}

func (b boundedTestBus) Write8(addr uint32, value uint8) error {
	if addr >= uint32(len(b)) {
		return errors.New("out of range")
	}
	b[addr] = value
	return nil
}

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

func TestOutputALToImmediatePort(t *testing.T) {
	c := New(testBus{0xe6, 0x43})
	c.R[EAX], c.EFlags = 0x12345636, CF|ZF
	var port uint16
	var value uint8
	c.PortOut = func(p uint16, v uint8) bool { port, value = p, v; return true }
	if err := c.Step(); err != nil || port != 0x43 || value != 0x36 || c.R[EAX] != 0x12345636 || c.EFlags != CF|ZF {
		t.Fatalf("OUT port=%X value=%X EAX=%X flags=%X err=%v", port, value, c.R[EAX], c.EFlags, err)
	}

	c = New(testBus{0xe6, 0x43})
	if err := c.Step(); err == nil {
		t.Fatal("OUT without consumer was accepted")
	}
}

func TestShortJumpBelow(t *testing.T) {
	c := New(testBus{0x72, 0xfe})
	c.EFlags = CF | ZF
	if err := c.Step(); err != nil || c.EIP != 0 || c.EFlags != CF|ZF {
		t.Fatalf("taken JB EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}

	c = New(testBus{0x72, 0xfe})
	c.EFlags = ZF
	if err := c.Step(); err != nil || c.EIP != 2 || c.EFlags != ZF {
		t.Fatalf("untaken JB EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
}

func TestShortJumpLessOrEqual(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags uint32
		want  uint32
	}{
		{name: "equal", flags: ZF | CF, want: 0},
		{name: "less", flags: SF, want: 0},
		{name: "greater", flags: CF, want: 2},
		{name: "same signed flags", flags: SF | OF, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := New(testBus{0x7e, 0xfe})
			c.EFlags = test.flags
			if err := c.Step(); err != nil || c.EIP != test.want || c.EFlags != test.flags {
				t.Fatalf("JLE EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
			}
		})
	}
}

func TestShortJumpGreaterOrEqual(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags uint32
		want  uint32
	}{
		{name: "positive", want: 0},
		{name: "both signed flags", flags: SF | OF, want: 0},
		{name: "less SF", flags: SF, want: 2},
		{name: "less OF", flags: OF, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := New(testBus{0x7d, 0xfe})
			c.EFlags = test.flags
			if err := c.Step(); err != nil || c.EIP != test.want || c.EFlags != test.flags {
				t.Fatalf("JGE EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
			}
		})
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

func TestMoveDwordFromDSBaseMemory(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x8b, 0x0b})
	copy(mem[0x18:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0x2f})
	c.R[EBX], c.R[ECX], c.EFlags = 0x18, 0xa5a5a5a5, CF|ZF|OF
	if err := c.Step(); err != nil || c.R[ECX] != 0x12345678 || c.R[EBX] != 0x18 || c.EFlags != CF|ZF|OF {
		t.Fatalf("MOV ECX=%X EBX=%X flags=%X err=%v", c.R[ECX], c.R[EBX], c.EFlags, err)
	}

	c = New(testBus{0x8b, 0x0b})
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 1})
	c.R[EBX], c.R[ECX] = 0x18, 0xa5a5a5a5
	if err := c.Step(); err == nil || c.R[ECX] != 0xa5a5a5a5 {
		t.Fatalf("out-of-range MOV ECX=%X err=%v", c.R[ECX], err)
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

func TestMoveImmediateByteToDSRegisterMemory(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0xc6, 0x03, 0x02})
	c := New(mem)
	c.R[EBX] = 4
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || mem[0x24] != 2 || c.R[EBX] != 4 {
		t.Fatalf("byte=%X EBX=%X err=%v", mem[0x24], c.R[EBX], err)
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

func TestX87InitAndStoreControlWordToStack(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0xdb, 0xe3, 0xd9, 0x3c, 0x24})
	c := New(mem)
	c.R[ESP] = 4
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x3f, Writable: true})
	c.FPUControl, c.FPUDepth = 0xffff, 3
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.FPUControl != 0x037f || c.FPUDepth != 0 {
		t.Fatalf("FNINIT control=%X depth=%d", c.FPUControl, c.FPUDepth)
	}
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x24:0x26], []byte{0x7f, 0x03}) || c.R[ESP] != 4 {
		t.Fatalf("FNSTCW bytes=% X ESP=%X err=%v", mem[0x24:0x26], c.R[ESP], err)
	}
}

func TestX87LoadControlPushZerosWaitAndReturn(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	copy(mem, []byte{0xd9, 0x2d, 0x40, 0x00, 0x00, 0x00, 0xd9, 0xee, 0xd9, 0xee, 0x9b, 0xc3})
	mem[0x40], mem[0x41] = 0x7f, 0x12
	mem[0x70], mem[0x71] = 0x50, 0x00
	c := New(mem)
	c.R[ESP] = 0x50
	c.Seg[SegDS], c.Seg[SegSS] = 0x160, 0x168
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 0xff})
	c.SetDescriptor(0x168, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	for range 5 {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.FPUControl != 0x127f || c.FPUDepth != 2 || c.EIP != 0x50 || c.R[ESP] != 0x54 {
		t.Fatalf("control=%X depth=%d EIP=%X ESP=%X", c.FPUControl, c.FPUDepth, c.EIP, c.R[ESP])
	}
}

func TestCompareHighByteImmediateDoesNotWriteBack(t *testing.T) {
	c := New(testBus{0x80, 0xfc, 0x03})
	c.R[EAX] = 0x0003037f
	if err := c.Step(); err != nil || c.R[EAX] != 0x0003037f || c.EFlags&ZF == 0 {
		t.Fatalf("CMP AH EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestAndByteBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x80, 0x63, 0x0c, 0xfc})
	mem[0x1c] = 0xff
	c := New(mem)
	c.R[EBX], c.EFlags = 0x10, CF|OF|AF
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || mem[0x1c] != 0xfc || c.R[EBX] != 0x10 || c.EFlags&(CF|OF|AF|ZF|SF) != SF {
		t.Fatalf("DS AND value=%X EBX=%X flags=%X err=%v", mem[0x1c], c.R[EBX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0x80, 0x65, 0xfc, 0x0f})
	mem[0x1c] = 0xf0
	c = New(mem)
	c.R[EBP] = 0x20
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || mem[0x1c] != 0 || c.R[EBP] != 0x20 || c.EFlags&ZF == 0 {
		t.Fatalf("SS AND value=%X EBP=%X flags=%X err=%v", mem[0x1c], c.R[EBP], c.EFlags, err)
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

func TestOrALImmediate8(t *testing.T) {
	c := New(testBus{0x0c, 0x03})
	c.R[EAX], c.EFlags = 0x12345680, CF|OF|AF
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345683 || c.EFlags&(CF|OF|AF|ZF|SF) != SF {
		t.Fatalf("OR AL EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestOrRegister8Immediate8(t *testing.T) {
	c := New(testBus{0x80, 0xca, 0x40, 0x80, 0xcc, 0x80})
	c.R[EDX], c.R[EAX], c.EFlags = 0x12345603, 0x11223344, CF|OF|AF
	if err := c.Step(); err != nil || c.R[EDX] != 0x12345643 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("OR DL EDX=%X flags=%X err=%v", c.R[EDX], c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.R[EAX] != 0x1122b344 || c.EFlags&SF == 0 {
		t.Fatalf("OR AH EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestOrBaseDisp8RegisterDword(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x09, 0x43, 0x0c})
	copy(mem[0x1c:], []byte{0x00, 0x01, 0x00, 0x00})
	c := New(mem)
	c.R[EBX], c.R[EAX], c.EFlags = 0x10, 3, CF|OF|AF
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x1c:0x20], []byte{3, 1, 0, 0}) || c.R[EBX] != 0x10 || c.R[EAX] != 3 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("DS OR memory=% X EBX=%X EAX=%X flags=%X err=%v", mem[0x1c:0x20], c.R[EBX], c.R[EAX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0x09, 0x55, 0xfc})
	copy(mem[0x1c:], []byte{0x00, 0x00, 0x00, 0x80})
	c = New(mem)
	c.R[EBP], c.R[EDX] = 0x20, 1
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x1c:0x20], []byte{1, 0, 0, 0x80}) || c.EFlags&SF == 0 {
		t.Fatalf("SS OR memory=% X flags=%X err=%v", mem[0x1c:0x20], c.EFlags, err)
	}
}

func TestOrRegister8BaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x0a, 0x45, 0xfc})
	mem[0x1c] = 0x40
	c := New(mem)
	c.R[EBP], c.R[EAX], c.EFlags = 0x20, 3, CF|OF|AF
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x43 || mem[0x1c] != 0x40 || c.R[EBP] != 0x20 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("OR AL EAX=%X source=%X EBP=%X flags=%X err=%v", c.R[EAX], mem[0x1c], c.R[EBP], c.EFlags, err)
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

func TestCompareDSAbsoluteByteImmediate(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x80, 0x3d, 0x40, 0x00, 0x00, 0x00, 0x00})
	mem[0x60] = 0
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || mem[0x60] != 0 {
		t.Fatalf("absolute CMP flags=%X byte=%X err=%v", c.EFlags, mem[0x60], err)
	}
}

func TestXORByteRegisters(t *testing.T) {
	mem := testBus([]byte{0x30, 0xdb})
	c := New(mem)
	c.R[EBX] = 0xa5ff12ef
	c.EFlags = CF | OF | AF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EBX] != 0xa5ff1200 || c.EFlags&ZF == 0 || c.EFlags&(CF|OF|AF) != 0 {
		t.Fatalf("XOR BL,BL EBX=%08X flags=%X", c.R[EBX], c.EFlags)
	}
}

func TestMOVZXAbsoluteDSWord(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x0f, 0xb7, 0x05, 0x40, 0x00, 0x00, 0x00})
	mem[0x60], mem[0x61] = 0x7f, 0x12
	c := New(mem)
	c.R[EAX] = 0xffffffff
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x127f {
		t.Fatalf("MOVZX EAX,[disp32] EAX=%08X err=%v", c.R[EAX], err)
	}
}

func TestMOVZXRegisterWord(t *testing.T) {
	flags := uint32(CF | PF | AF | ZF | SF | OF)
	c := New(testBus{0x0f, 0xb7, 0xc3})
	c.R[EAX], c.R[EBX], c.EFlags = 0xffffffff, 0xa5a5127f, flags
	if err := c.Step(); err != nil || c.R[EAX] != 0x127f || c.R[EBX] != 0xa5a5127f || c.EFlags != flags {
		t.Fatalf("MOVZX EAX=%X EBX=%X flags=%X err=%v", c.R[EAX], c.R[EBX], c.EFlags, err)
	}

	c = New(testBus{0x0f, 0xb7, 0xc0})
	c.R[EAX], c.EFlags = 0xffff8001, flags
	if err := c.Step(); err != nil || c.R[EAX] != 0x8001 || c.EFlags != flags {
		t.Fatalf("MOVZX same register EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestMOVZXByteFromESI(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x0f, 0xb6, 0x16})
	mem[0x20] = 0xa5
	c := New(mem)
	c.R[ESI], c.R[EDX], c.EFlags = 0x20, 0xffffffff, CF|ZF
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || c.R[EDX] != 0xa5 || c.R[ESI] != 0x20 || mem[0x20] != 0xa5 || c.EFlags != CF|ZF {
		t.Fatalf("MOVZX EDX=%X ESI=%X source=%X flags=%X err=%v", c.R[EDX], c.R[ESI], mem[0x20], c.EFlags, err)
	}
}

func TestMOVZXByteFromEAX(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x0f, 0xb6, 0x00})
	mem[0x20] = 0xa5
	c := New(mem)
	c.R[EAX], c.EFlags = 0x20, CF|ZF
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x2f})
	if err := c.Step(); err != nil || c.R[EAX] != 0xa5 || c.EFlags != CF|ZF {
		t.Fatalf("MOVZX EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestMOVZXRegisterByte(t *testing.T) {
	flags := uint32(CF | ZF | OF)
	c := New(testBus{0x0f, 0xb6, 0xd4})
	c.R[EAX], c.R[EDX], c.EFlags = 0x1234a578, 0xffffffff, flags
	if err := c.Step(); err != nil || c.R[EDX] != 0xa5 || c.R[EAX] != 0x1234a578 || c.EFlags != flags {
		t.Fatalf("MOVZX EDX,AH EAX=%X EDX=%X flags=%X err=%v", c.R[EAX], c.R[EDX], c.EFlags, err)
	}

	c = New(testBus{0x0f, 0xb6, 0xc0})
	c.R[EAX], c.EFlags = 0xffffffa5, flags
	if err := c.Step(); err != nil || c.R[EAX] != 0xa5 || c.EFlags != flags {
		t.Fatalf("MOVZX EAX,AL EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestStoreRegister8ToBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x88, 0x45, 0xfc})
	c := New(mem)
	c.R[EBP], c.R[EAX], c.EFlags = 0x20, 0x123456a5, CF|ZF
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || mem[0x1c] != 0xa5 || c.R[EBP] != 0x20 || c.R[EAX] != 0x123456a5 || c.EFlags != CF|ZF {
		t.Fatalf("MOV byte value=%X EBP=%X EAX=%X flags=%X err=%v", mem[0x1c], c.R[EBP], c.R[EAX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x40))
	copy(mem, []byte{0x88, 0x63, 0x04})
	c = New(mem)
	c.R[EBX], c.R[EAX] = 0x20, 0x0000b400
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || mem[0x24] != 0xb4 || c.R[EBX] != 0x20 {
		t.Fatalf("MOV AH value=%X EBX=%X err=%v", mem[0x24], c.R[EBX], err)
	}
}

func TestStoreRegister8ToBase(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x88, 0x23})
	c := New(mem)
	c.R[EBX], c.R[EAX], c.EFlags = 0x18, 0x0000a500, CF|ZF|OF
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x2f, Writable: true})
	if err := c.Step(); err != nil || mem[0x18] != 0xa5 || c.R[EBX] != 0x18 || c.R[EAX] != 0x0000a500 || c.EFlags != CF|ZF|OF {
		t.Fatalf("MOV AH value=%X EBX=%X EAX=%X flags=%X err=%v", mem[0x18], c.R[EBX], c.R[EAX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x20))
	copy(mem, []byte{0x88, 0x03})
	c = New(mem)
	c.R[EBX], c.R[EAX] = 0x10, 0xa5
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x1f})
	if err := c.Step(); err == nil || mem[0x10] != 0 {
		t.Fatalf("read-only MOV value=%X err=%v", mem[0x10], err)
	}
}

func TestStoreRegister8ToSIBDisp32(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x88, 0xa4, 0xb3, 0xf0, 0xff, 0xff, 0xff})
	c := New(mem)
	c.R[EBX], c.R[ESI], c.R[EAX], c.EFlags = 0x20, 0x0c, 0x0000a500, CF|ZF|OF
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil || mem[0x40] != 0xa5 || c.R[EBX] != 0x20 || c.R[ESI] != 0x0c || c.EFlags != CF|ZF|OF {
		t.Fatalf("SIB MOV AH value=%X flags=%X err=%v", mem[0x40], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x80))
	copy(mem, []byte{0x88, 0xb4, 0x34, 0x18, 0, 0, 0})
	c = New(mem)
	c.R[ESP], c.R[ESI], c.R[EDX] = 0x20, 0x10, 0x00005a00
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil || mem[0x48] != 0x5a {
		t.Fatalf("stack SIB MOV value=%X err=%v", mem[0x48], err)
	}

	mem = testBus(make([]byte, 0x80))
	copy(mem, []byte{0x88, 0xb4, 0x34, 0x18, 0, 0, 0})
	c = New(mem)
	c.R[ESP], c.R[ESI], c.R[EDX] = 0x20, 0x10, 0x00005a00
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Limit: 0x7f})
	if err := c.Step(); err == nil || mem[0x48] != 0 {
		t.Fatalf("read-only SIB MOV value=%X err=%v", mem[0x48], err)
	}
}

func TestStoreImmediateDwordBaseDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xc7, 0x45, 0xf8, 0xff, 0xff, 0xff, 0xff})
	c := New(mem)
	c.R[EBP], c.EFlags = 0x20, CF|ZF
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || !bytes.Equal(mem[0x18:0x1c], []byte{0xff, 0xff, 0xff, 0xff}) || c.R[EBP] != 0x20 || c.EFlags != CF|ZF {
		t.Fatalf("MOV immediate memory=% X EBP=%X flags=%X err=%v", mem[0x18:0x1c], c.R[EBP], c.EFlags, err)
	}
}

func TestX87StartupSelfTestSequence(t *testing.T) {
	mem := testBus(make([]byte, 0x100))
	code := []byte{
		0x66, 0x50, 0x9b, 0xdb, 0xe3, 0xd9, 0xe8, 0xd9, 0xee, 0xde, 0xf9,
		0xd9, 0xc0, 0xd9, 0xe0, 0xde, 0xd9, 0x9b, 0xdf, 0xe0, 0xb0, 0x02,
		0x9e, 0x0f, 0x84, 0x02, 0x00, 0x00, 0x00, 0xb0, 0x03, 0x9b, 0xdb,
		0xe3, 0x9b, 0xd9, 0x2c, 0x24, 0x66, 0x87, 0x04, 0x24, 0x66, 0x58, 0xc3,
	}
	copy(mem, code)
	mem[0x90] = 0x60
	c := New(mem)
	c.R[EAX], c.R[ESP] = 0x127f, 0x90
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0xff, Writable: true})
	for steps := 0; c.EIP != 0x60 && steps < 30; steps++ {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.EIP != 0x60 || c.R[EAX] != 0x0103 || c.R[ESP] != 0x94 {
		t.Fatalf("self-test return EIP=%X EAX=%X ESP=%X", c.EIP, c.R[EAX], c.R[ESP])
	}
	if c.FPUControl != 0x127f || c.FPUStatus != 0 || c.FPUDepth != 0 {
		t.Fatalf("self-test FPU control=%X status=%X depth=%d", c.FPUControl, c.FPUStatus, c.FPUDepth)
	}
}

func TestXchgRegisterStackDisp8(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x87, 0x4c, 0x24, 0x04})
	copy(mem[0x24:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.R[ESP], c.R[ECX] = 0x20, 0xaabbccdd
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.R[ECX] != 0x12345678 || !bytes.Equal(mem[0x24:0x28], []byte{0xdd, 0xcc, 0xbb, 0xaa}) {
		t.Fatalf("XCHG ECX=%X stack=% X err=%v", c.R[ECX], mem[0x24:0x28], err)
	}
}

func TestReturnImmediate32(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0xc2, 0x04, 0x00})
	copy(mem[0x20:], []byte{0x34, 0x12, 0, 0})
	c := New(mem)
	c.R[ESP] = 0x20
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.EIP != 0x1234 || c.R[ESP] != 0x28 {
		t.Fatalf("RET EIP=%X ESP=%X err=%v", c.EIP, c.R[ESP], err)
	}
}

func TestPushPopFS(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x0f, 0xa0, 0x0f, 0xa1})
	c := New(mem)
	c.R[ESP] = 0x60
	c.Seg[SegSS], c.Seg[SegFS] = 0x30, 0x38
	c.SetDescriptor(0x30, Descriptor{Limit: 0x7f, Writable: true})
	c.SetDescriptor(0x38, Descriptor{Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[ESP] != 0x5c {
		t.Fatalf("PUSH FS ESP=%X err=%v", c.R[ESP], err)
	}
	value, _ := c.read16(0x5c)
	if value != 0x38 {
		t.Fatalf("PUSH FS stack=%X", value)
	}
	c.Seg[SegFS] = 0
	if err := c.Step(); err != nil || c.R[ESP] != 0x60 || c.Seg[SegFS] != 0x38 {
		t.Fatalf("POP FS FS=%X ESP=%X err=%v", c.Seg[SegFS], c.R[ESP], err)
	}
}

func TestPopNullFSLeavesUnusableSelector(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x0f, 0xa1})
	c := New(mem)
	c.R[ESP] = 0x20
	c.Seg[SegSS], c.Seg[SegFS] = 0x30, 0x38
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.Seg[SegFS] != 0 || c.R[ESP] != 0x24 {
		t.Fatalf("POP null FS=%X ESP=%X err=%v", c.Seg[SegFS], c.R[ESP], err)
	}
	if _, ok := c.readSegment8(c.Seg[SegFS], 0); ok {
		t.Fatal("null FS unexpectedly resolved memory")
	}
}

func TestGroup83SubtractRegister(t *testing.T) {
	mem := testBus([]byte{0x83, 0xec, 0x04})
	c := New(mem)
	c.R[ESP] = 0x100
	if err := c.Step(); err != nil || c.R[ESP] != 0xfc {
		t.Fatalf("SUB ESP,4 ESP=%X flags=%X err=%v", c.R[ESP], c.EFlags, err)
	}
}

func TestGroup83CompareRegister(t *testing.T) {
	c := New(testBus{0x83, 0xf8, 0x10})
	c.R[EAX] = 0
	if err := c.Step(); err != nil || c.R[EAX] != 0 || c.EFlags&SF == 0 || c.EFlags&ZF != 0 {
		t.Fatalf("CMP EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}

	c = New(testBus{0x83, 0xfb, 0xff})
	c.R[EBX] = 0xffffffff
	if err := c.Step(); err != nil || c.R[EBX] != 0xffffffff || c.EFlags&ZF == 0 {
		t.Fatalf("signed immediate CMP EBX=%X flags=%X err=%v", c.R[EBX], c.EFlags, err)
	}
}

func TestGroup83CompareAbsoluteDSDword(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x83, 0x3d, 0x40, 0x00, 0x00, 0x00, 0x00})
	c := New(mem)
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 {
		t.Fatalf("CMP [disp32],0 flags=%X err=%v", c.EFlags, err)
	}
}

func TestGroup83CompareStackDisp8Dword(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x83, 0x7c, 0x24, 0x04, 0x00})
	c := New(mem)
	c.R[ESP] = 0x20
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Limit: 0x3f, Writable: true})
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 {
		t.Fatalf("CMP stack flags=%X err=%v", c.EFlags, err)
	}
}

func TestSETZRegisterByte(t *testing.T) {
	for _, test := range []struct {
		flags uint32
		want  uint8
	}{{ZF, 1}, {0, 0}} {
		c := New(testBus{0x0f, 0x94, 0xc4})
		c.R[EAX], c.EFlags = 0x12345678, test.flags
		if err := c.Step(); err != nil || c.reg8(4) != test.want || c.R[EAX]&0xffff00ff != 0x12340078 || c.EFlags != test.flags {
			t.Fatalf("SETZ EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
		}
	}
}

func TestSETNZRegisterByte(t *testing.T) {
	for _, test := range []struct {
		flags uint32
		want  uint8
	}{{ZF | CF, 0}, {CF, 1}} {
		c := New(testBus{0x0f, 0x95, 0xc0})
		c.R[EAX], c.EFlags = 0x12345678, test.flags
		if err := c.Step(); err != nil || c.reg8(0) != test.want || c.R[EAX]&0xffffff00 != 0x12345600 || c.EFlags != test.flags {
			t.Fatalf("SETNZ EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
		}
	}

	c := New(testBus{0x0f, 0x95, 0x00})
	if err := c.Step(); err == nil {
		t.Fatal("memory SETNZ was accepted")
	}
}

func TestLFSAbsolutePointer(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x0f, 0xb4, 0x05, 0x40, 0x00, 0x00, 0x00})
	copy(mem[0x60:], []byte{0x78, 0x56, 0x34, 0x12, 0x38, 0x00})
	c := New(mem)
	c.Seg[SegDS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	c.SetDescriptor(0x38, Descriptor{Base: 0x100, Limit: 0xff})
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345678 || c.Seg[SegFS] != 0x38 {
		t.Fatalf("LFS EAX=%X FS=%X err=%v", c.R[EAX], c.Seg[SegFS], err)
	}
}

func TestLFSRejectsUnknownSelectorAtomically(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x0f, 0xb4, 0x05, 0x40, 0x00, 0x00, 0x00})
	copy(mem[0x60:], []byte{0x78, 0x56, 0x34, 0x12, 0x48, 0x00})
	c := New(mem)
	c.R[EAX], c.Seg[SegFS], c.Seg[SegDS] = 0xaabbccdd, 0x38, 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err == nil || c.R[EAX] != 0xaabbccdd || c.Seg[SegFS] != 0x38 {
		t.Fatalf("LFS atomic EAX=%X FS=%X err=%v", c.R[EAX], c.Seg[SegFS], err)
	}
}

func TestMoveDwordToEBPDisp8UsesSS(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x89, 0x45, 0xfc})
	c := New(mem)
	c.R[EAX], c.R[EBP] = 0x12345678, 0x50
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f, Writable: true})
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	low, _ := c.read16(0x6c)
	high, _ := c.read16(0x6e)
	value := uint32(low) | uint32(high)<<16
	if value != 0x12345678 {
		t.Fatalf("MOV [EBP-4],EAX value=%X", value)
	}
}

func TestXORDwordRegisters(t *testing.T) {
	c := New(testBus([]byte{0x31, 0xf6}))
	c.R[ESI] = 0x12345678
	c.EFlags = CF | OF | AF
	if err := c.Step(); err != nil || c.R[ESI] != 0 || c.EFlags&ZF == 0 || c.EFlags&(CF|OF|AF) != 0 {
		t.Fatalf("XOR ESI,ESI ESI=%X flags=%X err=%v", c.R[ESI], c.EFlags, err)
	}
}

func TestMoveDwordFromEBPDisp8UsesSS(t *testing.T) {
	mem := testBus(make([]byte, 0x90))
	copy(mem, []byte{0x8b, 0x45, 0xfc})
	copy(mem[0x6c:], []byte{0x78, 0x56, 0x34, 0x12})
	c := New(mem)
	c.R[EBP] = 0x50
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x12345678 {
		t.Fatalf("MOV EAX,[EBP-4] EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestCompareESByteAtEAX(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x26, 0x80, 0x38, 0x00})
	mem[0x60] = 0
	c := New(mem)
	c.R[EAX], c.Seg[SegES] = 0x40, 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.EFlags&ZF == 0 || mem[0x60] != 0 {
		t.Fatalf("CMP ES:[EAX],0 flags=%X byte=%X err=%v", c.EFlags, mem[0x60], err)
	}
}

func TestSubtractStackDwordFromRegister(t *testing.T) {
	mem := testBus(make([]byte, 0x90))
	copy(mem, []byte{0x2b, 0x45, 0xfc})
	copy(mem[0x6c:], []byte{0x34, 0x12, 0x00, 0x00})
	c := New(mem)
	c.R[EAX], c.R[EBP] = 0x1234, 0x50
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 0x20, Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[EAX] != 0 || c.EFlags&ZF == 0 {
		t.Fatalf("SUB EAX,[EBP-4] EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}
}

func TestSubtractStackDwordRejectsReadWithoutCommit(t *testing.T) {
	c := New(testBus([]byte{0x2b, 0x45, 0xfc}))
	c.R[EAX], c.R[EBP], c.Seg[SegSS] = 0x1234, 0x50, 0x30
	if err := c.Step(); err == nil || c.R[EAX] != 0x1234 {
		t.Fatalf("SUB stack atomic EAX=%X err=%v", c.R[EAX], err)
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

func TestDSBaseByteRead(t *testing.T) {
	mem := testBus(make([]byte, 0x30))
	copy(mem, []byte{0x8a, 0x23})
	mem[0x18] = 0xa5
	c := New(mem)
	c.R[EBX], c.R[EAX], c.EFlags = 0x18, 0x12345678, CF|ZF|OF
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x2f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x1234a578 || c.R[EBX] != 0x18 || c.EFlags != CF|ZF|OF {
		t.Fatalf("MOV AH EAX=%X EBX=%X flags=%X err=%v", c.R[EAX], c.R[EBX], c.EFlags, err)
	}

	c = New(testBus{0x8a, 0x03})
	c.R[EBX], c.R[EAX] = 0x18, 0x12345678
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 1})
	if err := c.Step(); err == nil || c.R[EAX] != 0x12345678 {
		t.Fatalf("out-of-range MOV EAX=%X err=%v", c.R[EAX], err)
	}
}

func TestSIBDisp32ByteRead(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x8a, 0xa4, 0xb3, 0xf0, 0xff, 0xff, 0xff})
	mem[0x40] = 0xa5
	c := New(mem)
	c.R[EBX], c.R[ESI], c.R[ESP], c.R[EAX], c.EFlags = 0x20, 0x0c, 0x70, 0x12345678, CF|ZF|OF
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x1234a578 || c.EFlags != CF|ZF|OF {
		t.Fatalf("SIB MOV AH EAX=%X flags=%X err=%v", c.R[EAX], c.EFlags, err)
	}

	mem = testBus(make([]byte, 0x80))
	copy(mem, []byte{0x8a, 0x84, 0x34, 0x18, 0, 0, 0})
	mem[0x48] = 0x5a
	c = New(mem)
	c.R[ESP], c.R[ESI], c.R[EAX] = 0x20, 0x10, 0x12345678
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Limit: 0x7f})
	if err := c.Step(); err != nil || c.R[EAX] != 0x1234565a {
		t.Fatalf("stack SIB MOV EAX=%X err=%v", c.R[EAX], err)
	}

	c = New(testBus{0x8a, 0x84, 0x34, 0x18, 0, 0, 0})
	c.R[ESP], c.R[ESI], c.R[EAX] = 0x20, 0x10, 0x12345678
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x168, Descriptor{Limit: 6})
	if err := c.Step(); err == nil || c.R[EAX] != 0x12345678 {
		t.Fatalf("out-of-range SIB MOV EAX=%X err=%v", c.R[EAX], err)
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

func TestREPNESCASB(t *testing.T) {
	c := New(testBus{0xf2, 0xae})
	c.R[EAX], c.R[ECX], c.R[EDI] = 0, 5, 0x20
	c.Seg[SegES] = 0x28
	data := []byte{'A', 'B', 0, 'C'}
	c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
		if selector != 0x28 || offset < 0x20 || offset-0x20 >= uint32(len(data)) {
			return 0, false
		}
		return data[offset-0x20], true
	}
	if err := c.Step(); err != nil || c.R[EDI] != 0x23 || c.R[ECX] != 2 || c.EFlags&ZF == 0 {
		t.Fatalf("REPNE SCASB EDI=%X ECX=%X flags=%X err=%v", c.R[EDI], c.R[ECX], c.EFlags, err)
	}

	c = New(testBus{0xf2, 0xaa})
	if err := c.Step(); err == nil {
		t.Fatal("REPNE STOSB was accepted")
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
