package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

func TestLEBIOSData(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x2000)}
	m.CPU = cpu386.New(m)
	m.Mem[0x3ff] = 0xab
	m.Mem[0x500] = 0xcd
	if err := InstallDOS4GWBIOSData(m); err != nil {
		t.Fatal(err)
	}
	var got uint32
	for i := uint32(0); i < 4; i++ {
		v, ok := m.CPU.ReadSegment8(0x40, 0x63+i)
		if !ok {
			t.Fatal("BIOS read rejected")
		}
		got |= uint32(v) << (8 * i)
	}
	if got != 0x302903d4 || m.Mem[0x3ff] != 0xab || m.Mem[0x500] != 0xcd {
		t.Fatalf("BIOS value %08x", got)
	}
	if err := InstallDOS4GWBIOSData(m); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestLEBIOSDataRejectsOverlap(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x2000)}
	m.CPU = cpu386.New(m)
	m.Mem[0x499] = 1
	if err := InstallDOS4GWBIOSData(m); err == nil {
		t.Fatal("overlap accepted")
	}
	if m.Mem[0x400] != 0 || len(m.CPU.Descriptors) != 0 {
		t.Fatal("failure changed state")
	}
}
