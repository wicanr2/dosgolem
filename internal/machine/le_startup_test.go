package machine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func TestFD2StartupDOSOpenReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("mdi.ini\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags = 0x3d00, 0x20, cpu386.CF
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF != 0 || !s.HasHandle(5) {
		t.Fatalf("open EAX=%X flags=%X handle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(5))
	}
	c.R[cpu386.EAX] = 0x3d01
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("write mode EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	copy(bus[0x20:], []byte("../MDI.INI\x00"))
	c.R[cpu386.EAX] = 0x3d00
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 2 || c.EFlags&cpu386.CF == 0 || s.HasHandle(6) {
		t.Fatalf("unsafe path EAX=%X flags=%X nextHandle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(6))
	}
	copy(bus[0x20:], []byte("MISSING.INI\x00"))
	c.R[cpu386.EAX] = 0x3d00
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 2 || c.EFlags&cpu386.CF == 0 || s.HasHandle(6) {
		t.Fatalf("missing path EAX=%X flags=%X nextHandle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(6))
	}
}

func TestFD2StartupDOSDeviceInformation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("open EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.EDX], c.EFlags = 0x4400, 5, 0xa5a5ffff, cpu386.CF
	if !s.Handle(c, 0x21) || c.R[cpu386.EDX] != 0xa5a50000 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("device info EAX=%X EDX=%X flags=%X", c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.EFlags = 0x4400, 6, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 6 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("invalid handle EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.EFlags = 0x4401, 5, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 1 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("unsupported IOCTL EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
}

func TestFD2StartupDOSReadFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 {
		t.Fatalf("open EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0x3f00, 5, 4, 0x40, cpu386.CF
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 4 || c.EFlags&cpu386.CF != 0 || string(bus[0x40:0x44]) != "driv" {
		t.Fatalf("read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x40:0x44])
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 8, 0x44
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 3 || string(bus[0x44:0x47]) != "er\n" {
		t.Fatalf("short read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x44:0x47])
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 8, 0x48
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("EOF EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 0, 0xffffffff
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("zero read EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0x3f00, 6, 1, 0x40, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 6 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("invalid handle EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
}

func TestFD2StartupDOSReadRejectsDestinationRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x40))
	copy(bus[0x10:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0x3f, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x10
	s.Handle(c, 0x21)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 5, 4, 0x3e
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("range EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 5, 4, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 4 || string(bus[0x20:0x24]) != "driv" {
		t.Fatalf("rewound read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x20:0x24])
	}
}

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

func TestFD2StartupDOSInterruptVectors(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	s.realModeVectors[8] = 0xf0001234
	c.Seg[cpu386.SegDS] = 0x160
	c.R[cpu386.EAX] = 0xaaaa2508
	c.R[cpu386.EDX] = 0x12345678
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DOS 2508 flags=%X calls=%d", c.EFlags, s.Calls())
	}

	c.Seg[cpu386.SegES] = 0xffff
	c.R[cpu386.EAX] = 0xbbbb3508
	c.R[cpu386.EBX] = 0xdeadbeef
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x21) || c.Seg[cpu386.SegES] != 0x160 || c.R[cpu386.EBX] != 0x12345678 || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DOS 3508 ES=%X EBX=%X flags=%X calls=%d", c.Seg[cpu386.SegES], c.R[cpu386.EBX], c.EFlags, s.Calls())
	}

	c.R[cpu386.EAX] = 0x0200
	c.R[cpu386.EBX] = 8
	if !s.Handle(c, 0x31) || uint16(c.R[cpu386.ECX]) != 0xf000 || uint16(c.R[cpu386.EDX]) != 0x1234 {
		t.Fatalf("DPMI vector was contaminated ECX=%X EDX=%X", c.R[cpu386.ECX], c.R[cpu386.EDX])
	}
}
