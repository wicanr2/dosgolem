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
	c.R[cpu386.EAX] = 0xff00
	c.R[cpu386.EDX] = 0x78
	if !s.Handle(c, 0x21) || c.R[cpu386.EAX] != 0x4734ffff || c.Seg[cpu386.SegGS] != 0x20 {
		t.Fatalf("DOS/4G response EAX=%08X GS=%04X", c.R[cpu386.EAX], c.Seg[cpu386.SegGS])
	}
	if s.Handle(c, 0x21) || s.Calls() != 2 {
		t.Fatal("extra startup call was accepted")
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
