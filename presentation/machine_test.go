package presentation

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestMachineFrameSourceProvidesIndependentPresentationInput(t *testing.T) {
	m := machine.New()
	m.Write8(machine.VideoSeg*16+7, 0x2a)
	m.DAC[0x2a*3] = 0x12
	m.DAC[0x2a*3+1] = 0x23
	m.DAC[0x2a*3+2] = 0x34

	source, err := NewMachineFrameSource(m)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := host.NewPresentationSnapshotProvider(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := provider.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Canvas.Width != machine.VideoWidth || snapshot.Canvas.Height != machine.VideoHigh {
		t.Fatalf("Canvas=%+v, want %dx%d", snapshot.Canvas, machine.VideoWidth, machine.VideoHigh)
	}
	if got := snapshot.Indexed[7]; got != 0x2a {
		t.Fatalf("Indexed[7]=%#x, want 0x2a", got)
	}
	if got, want := snapshot.Palette[0x2a], [3]uint8{0x49, 0x8e, 0xd3}; got != want {
		t.Fatalf("Palette[0x2a]=%#v, want %#v", got, want)
	}

	// Mutating the host-owned snapshot must not affect machine VRAM or a later
	// presentation read.
	snapshot.Indexed[7] = 0
	if got := m.Read8(machine.VideoSeg*16 + 7); got != 0x2a {
		t.Fatalf("machine VRAM changed to %#x", got)
	}
	later, err := provider.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(later.Indexed[7:8], []byte{0x2a}) {
		t.Fatalf("later frame changed: %#x", later.Indexed[7])
	}
}

func TestMachineFrameSourceRejectsNil(t *testing.T) {
	if _, err := NewMachineFrameSource(nil); err == nil {
		t.Fatal("NewMachineFrameSource(nil) unexpectedly succeeded")
	}
	var source *MachineFrameSource
	if _, err := source.ReadPresentationFrame(); err == nil {
		t.Fatal("nil source unexpectedly read a frame")
	}
}
