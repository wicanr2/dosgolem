package presentation

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

func TestLayerSnapshotProjectsActiveLayerWithoutMutatingIt(t *testing.T) {
	m := machine.New()
	setDAC(m, 1, 0x3f, 0, 0)
	setDAC(m, 2, 0, 0x3f, 0)
	setPixel(m, 0, 0, 1)
	setPixel(m, 1, 1, 1)
	setPixel(m, 2, 1, 2)
	setPixel(m, 1, 2, 1)
	setPixel(m, 2, 2, 2)

	font := &xlate.Font{Name: "test", W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	layer := &xlate.Layer{W: 320, H: 200}
	layer.Add(&xlate.Stamp{Key: "active", X: 1, Y: 1, Cells: 1, CellW: 2, CellH: 2, Font: font, Text: []rune{'中'}, State: xlate.Pending})
	// Lifecycle ownership stays outside the presenter. It settles the active
	// stamp before the read-only snapshot provider is called.
	indexed := m.Indexed()
	layer.Frame(indexed, rgb(indexed, m.Palette()))
	before, err := layer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	source, err := NewMachineFrameSource(m)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewLayerSnapshotProvider(source, layer, map[string]*xlate.Font{"test": font})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := provider.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Drew || len(snapshot.RGBA) != 320*2*200*2*4 {
		t.Fatalf("Drew=%v RGBA length=%d", snapshot.Drew, len(snapshot.RGBA))
	}
	after, err := layer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read-only presentation snapshot mutated active xlate layer")
	}

	snapshot.Frame.Indexed[0] = 0
	snapshot.RGBA[0] = 0
	if got := m.Read8(machine.VideoSeg * 16); got != 1 {
		t.Fatalf("machine framebuffer changed to %#x", got)
	}
	later, err := provider.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if later.Frame.Indexed[0] != 1 || later.RGBA[0] == 0 {
		t.Fatal("later presentation snapshot reused caller-mutable buffers")
	}
}

func TestLayerSnapshotFailsClosedForUnregisteredOrMismatchedLayer(t *testing.T) {
	m := machine.New()
	source, err := NewMachineFrameSource(m)
	if err != nil {
		t.Fatal(err)
	}
	unnamed := &xlate.Font{W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	layer := &xlate.Layer{W: 320, H: 200, Stamps: []*xlate.Stamp{{Key: "bad", Font: unnamed}}}
	provider, err := NewLayerSnapshotProvider(source, layer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Snapshot(2); err == nil {
		t.Fatal("unnamed active font unexpectedly produced a snapshot")
	}
	wrongSize := &xlate.Layer{W: 319, H: 200}
	provider, err = NewLayerSnapshotProvider(source, wrongSize, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Snapshot(2); err == nil {
		t.Fatal("mismatched layer size unexpectedly produced a snapshot")
	}
	if _, err := provider.Snapshot(0); err == nil {
		t.Fatal("zero scale unexpectedly produced a snapshot")
	}
	if _, err := NewLayerSnapshotProvider(source, nil, nil); err == nil {
		t.Fatal("nil layer unexpectedly accepted")
	}
}

func TestLayerSnapshotRejectsPhysicalGlyphBeforeFrameRead(t *testing.T) {
	font := &xlate.Font{Name: "physical", W: 1, H: 1, Glyphs: map[rune][]byte{'A': {0x80}}}
	layer := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{{
		X: 0, Y: 0, Cells: 2, CellW: 1, CellH: 1, State: xlate.Shown,
		PixelScale: 3, PixelGlyphs: []xlate.PixelGlyph{{Rune: 'A', Font: font, SrcW: 1, SrcH: 1}},
	}}}
	source := &sealedCountingSource{frame: host.IndexedFrame{Canvas: host.Canvas{Width: 2, Height: 1}, Indexed: []byte{0, 0}}}
	provider, err := NewLayerSnapshotProvider(source, layer, map[string]*xlate.Font{font.Name: font})
	if err != nil {
		t.Fatal(err)
	}
	shot, err := provider.Snapshot(3)
	if err == nil || source.reads != 0 || len(shot.RGBA) != 0 {
		t.Fatalf("legacy projection silently accepted physical glyph: err=%v reads=%d rgba=%d", err, source.reads, len(shot.RGBA))
	}
}

func setDAC(m *machine.Machine, index, r, g, b uint8) {
	m.DAC[index*3], m.DAC[index*3+1], m.DAC[index*3+2] = r, g, b
}

func setPixel(m *machine.Machine, x, y int, value uint8) {
	m.Write8(machine.VideoSeg*16+uint32(y*machine.VideoWidth+x), value)
}

func rgb(indexed []uint8, palette [256][3]uint8) []uint8 {
	out := make([]uint8, len(indexed)*3)
	for i, color := range indexed {
		copy(out[i*3:i*3+3], palette[color][:])
	}
	return out
}
