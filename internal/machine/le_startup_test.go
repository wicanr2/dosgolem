package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

type startupBus []byte

func (b startupBus) Read8(addr uint32) (uint8, error)      { return b[addr], nil }
func (b startupBus) Write8(addr uint32, value uint8) error { b[addr] = value; return nil }

func TestFD2StartupDOS(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	c.R[cpu386.EAX] = 0x3000
	c.R[cpu386.EBX] = 0x50484152
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0x1606 {
		t.Fatalf("DOS version response EAX=%08X", c.R[cpu386.EAX])
	}
	if c.Seg[cpu386.SegDS] != 0x160 || c.Seg[cpu386.SegES] != 0x28 || c.Seg[cpu386.SegGS] != 0x20 || c.Seg[cpu386.SegSS] != 0x160 {
		t.Fatalf("startup selectors DS=%X ES=%X GS=%X SS=%X", c.Seg[cpu386.SegDS], c.Seg[cpu386.SegES], c.Seg[cpu386.SegGS], c.Seg[cpu386.SegSS])
	}
	if value, ok := c.SegmentRead16(0x28, 0x2c); !ok || value != 0x30 {
		t.Fatalf("ES environment cell value=%X ok=%v", value, ok)
	}
	if _, ok := c.SegmentRead16(0x28, 0x2e); ok {
		t.Fatal("unknown ES environment cell was accepted")
	}
	if !c.SegmentLoadOK(0x28, cpu386.SegDS) || !c.SegmentLoadOK(0x28, cpu386.SegES) || c.SegmentLoadOK(0x28, cpu386.SegSS) {
		t.Fatal("PSP selector load destinations mismatch")
	}
	if !c.SegmentLoadOK(0x30, cpu386.SegDS) || !c.SegmentLoadOK(0x30, cpu386.SegES) || !c.SegmentLoadOK(0x30, cpu386.SegFS) || c.SegmentLoadOK(0x30, cpu386.SegSS) {
		t.Fatal("environment selector load destinations mismatch")
	}
	for offset, want := range minimalFD2Environment {
		if got, ok := c.SegmentRead8(0x30, uint32(offset)); !ok || got != want {
			t.Fatalf("environment[%d]=%X ok=%v want %X", offset, got, ok, want)
		}
	}
	if _, ok := c.SegmentRead8(0x30, uint32(len(minimalFD2Environment))); ok {
		t.Fatal("environment out-of-range read accepted")
	}
	c.R[cpu386.EAX] = 0xff00
	c.R[cpu386.EDX] = 0x78
	if !s.Handle(c, 0x21) || c.R[cpu386.EAX] != 0x4734ffff || c.Seg[cpu386.SegGS] != 0x20 {
		t.Fatalf("DOS/4G response EAX=%08X GS=%04X", c.R[cpu386.EAX], c.Seg[cpu386.SegGS])
	}
	c.R[cpu386.EAX] = 0x2c00
	for want := uint32(0); want < 3; want++ {
		if !s.Handle(c, 0x21) || uint8(c.R[cpu386.EDX]>>8) != uint8(want) {
			t.Fatalf("DOS time second=%d EDX=%08X", want, c.R[cpu386.EDX])
		}
		c.R[cpu386.EAX] = 0x2c00
	}
	c.R[cpu386.EAX] = 0x2900
	if s.Handle(c, 0x21) || s.Calls() != 5 {
		t.Fatal("unknown post-startup call was accepted")
	}
}

func TestFD2StartupDOSRejectsWrongOrder(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	c.R[cpu386.EAX] = 0xff00
	c.R[cpu386.EDX] = 0x78
	if s.Handle(c, 0x21) || s.Calls() != 0 {
		t.Fatal("out-of-order DOS/4G call was accepted")
	}
}

func TestFD2StartupDPMIGetRealModeInterruptVector(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	s.realModeVectors[8] = 0xf0001234
	c.R[cpu386.EAX] = 0xaaaa0200
	c.R[cpu386.EBX] = 0xbbbb0008
	c.R[cpu386.ECX] = 0xcccc0000
	c.R[cpu386.EDX] = 0xdddd0000
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x31) || c.R[cpu386.ECX] != 0xccccf000 || c.R[cpu386.EDX] != 0xdddd1234 || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DPMI 0200 ECX=%X EDX=%X flags=%X calls=%d", c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags, s.Calls())
	}

	c.R[cpu386.EAX] = 0x0201
	if s.Handle(c, 0x31) {
		t.Fatal("unknown DPMI function was accepted")
	}
}
