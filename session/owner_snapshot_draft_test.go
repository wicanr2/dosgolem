//go:build draft_session_snapshot

package session

// This excluded DRAFT models an owner-private presentation ticket with
// synthetic pixels. It uses committed package code and same-package test
// helpers/private fields, not a public API or production Snapshot contract.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

type draftSnapshotInput struct {
	frame host.IndexedFrame
	err   error
	reads int
}

func (s *draftSnapshotInput) ReadPresentationFrame() (host.IndexedFrame, error) {
	s.reads++
	return s.frame, s.err
}

type draftSnapshotTicket struct {
	owner      *draftSnapshotCandidate
	session    *Owner
	frameID    uint64
	inputEpoch uint64
	scale      host.OutputScale
}

type draftSnapshotCandidate struct {
	owner         *Owner
	input         *draftSnapshotInput
	frame         host.IndexedFrame
	frameHash     [32]byte
	frameID       uint64
	prepared      bool
	slots         []presentation.ActiveLayerSlot
	fonts         map[string]*xlate.Font
	sealedSlots   []presentation.ActiveLayerSlot
	sealedFonts   map[string]*xlate.Font
	sealedBytes   [][]byte
	fontHashes    map[string][32]byte
	onProjectRead func()
}

type draftFrozenSource struct {
	frame  host.IndexedFrame
	onRead func()
}

func (s draftFrozenSource) ReadPresentationFrame() (host.IndexedFrame, error) {
	if s.onRead != nil {
		s.onRead()
	}
	return s.frame, nil
}

func newDraftSnapshotCandidate(t *testing.T) *draftSnapshotCandidate {
	t.Helper()
	o := startSyntheticOwner(t, []byte{0x90, 0x90, 0x90})
	font := &xlate.Font{Name: "draft.snapshot.font", W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	background := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{{Key: "background", X: 0, Y: 0, Cells: 2, CellW: 1, CellH: 1, State: xlate.Shown, BG: [3]uint8{10, 20, 30}}}}
	text := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{{Key: "text", X: 0, Y: 0, Cells: 1, CellW: 1, CellH: 1, State: xlate.Shown, Font: font, Text: []rune{'中'}, BG: [3]uint8{10, 20, 30}, FG: [3]uint8{200, 210, 220}}}}
	return &draftSnapshotCandidate{
		owner: o,
		input: &draftSnapshotInput{frame: host.IndexedFrame{Canvas: host.Canvas{Width: 2, Height: 1}, Indexed: []byte{0, 0}}},
		slots: []presentation.ActiveLayerSlot{{Name: "background", Z: 0, Layer: background}, {Name: "text", Z: 1, Layer: text}},
		fonts: map[string]*xlate.Font{font.Name: font},
	}
}

func (d *draftSnapshotCandidate) fault(err error) error {
	d.prepared = false
	return d.owner.ReportDrawFault(err)
}

func draftCloneFont(source *xlate.Font) *xlate.Font {
	clone := &xlate.Font{Name: source.Name, W: source.W, H: source.H, Glyphs: make(map[rune][]byte, len(source.Glyphs))}
	for r, bitmap := range source.Glyphs {
		clone.Glyphs[r] = append([]byte(nil), bitmap...)
	}
	return clone
}

func draftFrameHash(frame host.IndexedFrame) [32]byte {
	h := sha256.New()
	var dims [16]byte
	binary.LittleEndian.PutUint64(dims[:8], uint64(frame.Canvas.Width))
	binary.LittleEndian.PutUint64(dims[8:], uint64(frame.Canvas.Height))
	_, _ = h.Write(dims[:])
	_, _ = h.Write(frame.Indexed)
	for _, rgb := range frame.Palette {
		_, _ = h.Write(rgb[:])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (d *draftSnapshotCandidate) sealLayers() error {
	if len(d.slots) != 2 || d.slots[0].Name != "background" || d.slots[0].Z != 0 ||
		d.slots[1].Name != "text" || d.slots[1].Z != 1 {
		return errors.New("draft: incomplete or unordered active group")
	}
	// First use the committed seal to reject source pointer/registry drift.
	if err := presentation.WithSealedOrderedLayers("draft.snapshot.group", d.owner.generation, d.frameID,
		d.slots, d.fonts, func(*presentation.SealedLayerGroup) error { return nil }); err != nil {
		return err
	}
	fonts := make(map[string]*xlate.Font, len(d.fonts))
	hashes := make(map[string][32]byte, len(d.fonts))
	for name, font := range d.fonts {
		clone := draftCloneFont(font)
		fingerprint, err := presentation.FontFingerprint(clone)
		if err != nil {
			return err
		}
		fonts[name], hashes[name] = clone, fingerprint
	}
	slots := make([]presentation.ActiveLayerSlot, 0, len(d.slots))
	data := make([][]byte, 0, len(d.slots))
	for _, slot := range d.slots {
		encoded, err := slot.Layer.Snapshot()
		if err != nil {
			return err
		}
		clone := &xlate.Layer{}
		if err := clone.Restore(encoded, fonts); err != nil {
			return err
		}
		encoded, err = clone.Snapshot()
		if err != nil {
			return err
		}
		slots = append(slots, presentation.ActiveLayerSlot{Name: slot.Name, Z: slot.Z, Layer: clone})
		data = append(data, encoded)
	}
	d.sealedSlots, d.sealedFonts, d.sealedBytes, d.fontHashes = slots, fonts, data, hashes
	return nil
}

func (d *draftSnapshotCandidate) frozenLayersUnchanged() bool {
	if len(d.sealedSlots) != 2 || len(d.sealedBytes) != 2 || len(d.sealedFonts) != len(d.fontHashes) {
		return false
	}
	if d.sealedSlots[0].Name != "background" || d.sealedSlots[0].Z != 0 ||
		d.sealedSlots[1].Name != "text" || d.sealedSlots[1].Z != 1 ||
		draftFrameHash(d.frame) != d.frameHash {
		return false
	}
	for name, font := range d.sealedFonts {
		fingerprint, err := presentation.FontFingerprint(font)
		want, ok := d.fontHashes[name]
		if err != nil || !ok || fingerprint != want {
			return false
		}
	}
	for i, slot := range d.sealedSlots {
		current, err := slot.Layer.Snapshot()
		if err != nil || !bytes.Equal(current, d.sealedBytes[i]) {
			return false
		}
	}
	return true
}

// prepare stands in for the not-yet-specified Advance -> Layer.Frame -> frame
// seal boundary. Its separate frameID is deliberately not Owner's input epoch
// or SourceGeneration. It freezes both pixels and source layers/font now.
func (d *draftSnapshotCandidate) prepare() (draftSnapshotTicket, error) {
	if d.owner.Status().Phase != PhaseRunning || d.frameID == ^uint64(0) {
		return draftSnapshotTicket{}, d.fault(errors.New("draft: cannot prepare frame"))
	}
	d.prepared = false
	d.frameID++
	provider, err := host.NewPresentationSnapshotProvider(d.input)
	if err != nil {
		return draftSnapshotTicket{}, d.fault(err)
	}
	frame, err := provider.Snapshot()
	if err != nil {
		return draftSnapshotTicket{}, d.fault(err)
	}
	view, err := d.owner.View()
	if err != nil {
		return draftSnapshotTicket{}, d.fault(err)
	}
	if err := d.sealLayers(); err != nil {
		return draftSnapshotTicket{}, d.fault(err)
	}
	d.frame = host.IndexedFrame{Canvas: frame.Canvas, Indexed: frame.Indexed, Palette: frame.Palette}
	d.frameHash = draftFrameHash(d.frame)
	d.prepared = true
	return draftSnapshotTicket{owner: d, session: d.owner, frameID: d.frameID, inputEpoch: view.SourceGeneration, scale: view.Panel.Scales.ActiveScale}, nil
}

func (d *draftSnapshotCandidate) snapshot(ticket draftSnapshotTicket, scale host.OutputScale) (presentation.LayerPresentationSnapshot, error) {
	if d.owner.Status().Phase != PhaseRunning {
		return presentation.LayerPresentationSnapshot{}, errors.New("draft: owner not running")
	}
	view, err := d.owner.View()
	if err != nil {
		return presentation.LayerPresentationSnapshot{}, d.fault(err)
	}
	if !d.prepared || ticket.owner != d || ticket.session != d.owner || ticket.frameID == 0 || ticket.frameID != d.frameID ||
		ticket.inputEpoch != view.SourceGeneration || ticket.scale != scale || scale != view.Panel.Scales.ActiveScale ||
		(scale != host.OutputScale2 && scale != host.OutputScale3) {
		return presentation.LayerPresentationSnapshot{}, d.fault(errors.New("draft: stale or cross-scale frame ticket"))
	}
	if !d.frozenLayersUnchanged() {
		return presentation.LayerPresentationSnapshot{}, d.fault(errors.New("draft: prepared layer or font changed"))
	}
	var shot presentation.LayerPresentationSnapshot
	err = presentation.WithSealedOrderedLayers("draft.snapshot.group", ticket.inputEpoch, ticket.frameID,
		d.sealedSlots, d.sealedFonts, func(group *presentation.SealedLayerGroup) error {
			valid := func() bool {
				return d.prepared && d.frameID == ticket.frameID && d.owner.Status().Phase == PhaseRunning &&
					d.owner == ticket.session && d.owner.generation == ticket.inputEpoch && d.frozenLayersUnchanged()
			}
			projected, err := presentation.ProjectSealedLayers(
				draftFrozenSource{frame: d.frame, onRead: d.onProjectRead}, group, int(scale), valid)
			if err != nil {
				return err
			}
			shot = projected
			return nil
		})
	if err != nil {
		return presentation.LayerPresentationSnapshot{}, d.fault(err)
	}
	return shot, nil
}

func draftApplySnapshotScale(t *testing.T, o *Owner, scale host.OutputScale) {
	t.Helper()
	r := deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventOpen}}, nil)
	consumePausedTurn(t, o, r.Epoch)
	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventSelectScale, Scale: scale}}, nil)
	consumePausedTurn(t, o, r.Epoch)
	r = deliverOwner(t, o, []host.PanelEvent{{Kind: host.PanelEventApply}}, nil)
	consumePausedTurn(t, o, r.Epoch)
}

func TestDraftSnapshotOwnerFrozenFramesAcrossScaleChangesNoStepsOrAliases(t *testing.T) {
	d := newDraftSnapshotCandidate(t)
	initial, err := d.prepare()
	if err != nil {
		t.Fatal(err)
	}
	first, err := d.snapshot(initial, host.OutputScale2)
	if err != nil || !first.Drew {
		t.Fatalf("first 2x shot = (%+v, %v)", first, err)
	}
	second, err := d.snapshot(initial, host.OutputScale2)
	if err != nil || !bytes.Equal(first.RGBA, second.RGBA) || d.input.reads != 1 {
		t.Fatalf("repeat shot = (%+v, %v), source reads=%d", second, err, d.input.reads)
	}
	want2 := append([]byte(nil), second.RGBA...)
	// The ticket must render the pixels, layers, and font sealed at prepare;
	// changing caller-owned source values must not recolor a repeated draw.
	d.input.frame.Indexed[0] = 1
	d.slots[1].Layer.Stamps[0].FG = [3]uint8{255, 0, 0}
	d.fonts["draft.snapshot.font"].Glyphs['中'][0] = 0
	frozen, err := d.snapshot(initial, host.OutputScale2)
	if err != nil || !bytes.Equal(frozen.RGBA, want2) || frozen.Frame.Indexed[0] != 0 || d.input.reads != 1 {
		t.Fatalf("source drift affected frozen ticket: err=%v equal=%t indexed=%v reads=%d", err, bytes.Equal(frozen.RGBA, want2), frozen.Frame.Indexed, d.input.reads)
	}
	first.RGBA[0] ^= 0xff
	first.Frame.Indexed[0] ^= 0xff
	if !bytes.Equal(second.RGBA, want2) || second.Frame.Indexed[0] != 0 {
		t.Fatal("returned buffers alias each other")
	}
	d.input.frame.Indexed[0] = 0
	d.slots[1].Layer.Stamps[0].FG = [3]uint8{200, 210, 220}
	d.fonts["draft.snapshot.font"].Glyphs['中'][0] = 0x80
	draftApplySnapshotScale(t, d.owner, host.OutputScale3)
	three, err := d.prepare()
	if err != nil {
		t.Fatal(err)
	}
	shot3, err := d.snapshot(three, host.OutputScale3)
	if err != nil || !shot3.Drew || shot3.Scale != 3 {
		t.Fatalf("3x shot = (%+v, %v)", shot3, err)
	}
	draftApplySnapshotScale(t, d.owner, host.OutputScale2)
	back, err := d.prepare()
	if err != nil {
		t.Fatal(err)
	}
	shot2, err := d.snapshot(back, host.OutputScale2)
	if err != nil || !bytes.Equal(shot2.RGBA, want2) || d.input.reads != 3 || d.owner.machine.Steps != 0 {
		t.Fatalf("2x return err=%v equal=%t reads=%d steps=%d", err, bytes.Equal(shot2.RGBA, want2), d.input.reads, d.owner.machine.Steps)
	}
}

func TestDraftSnapshotOwnerRejectsStaleScaleAndFailedSources(t *testing.T) {
	t.Run("old 2x ticket after Apply", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		old, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		draftApplySnapshotScale(t, d.owner, host.OutputScale3)
		shot, err := d.snapshot(old, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("stale shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
		if _, again := d.snapshot(old, host.OutputScale2); again == nil || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatal("failed candidate re-read or closed twice")
		}
	})
	t.Run("same-value owner swap", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		old, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		d.owner = startSyntheticOwner(t, []byte{0x90, 0x90, 0x90})
		if d.owner.generation != old.inputEpoch || ownerPanel(t, d.owner).Scales.ActiveScale != old.scale {
			t.Fatal("swap did not preserve same-value counterexample")
		}
		shot, err := d.snapshot(old, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("cross-owner shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
	t.Run("2x to 3x to 2x ABA", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		old, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		draftApplySnapshotScale(t, d.owner, host.OutputScale3)
		draftApplySnapshotScale(t, d.owner, host.OutputScale2)
		if ownerPanel(t, d.owner).Scales.ActiveScale != old.scale || d.owner.generation == old.inputEpoch {
			t.Fatal("round-trip did not create same-value ABA counterexample")
		}
		shot, err := d.snapshot(old, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("ABA shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
	for _, tc := range []struct {
		name       string
		breakGroup func(*draftSnapshotCandidate)
	}{
		{"missing text", func(d *draftSnapshotCandidate) { d.slots = d.slots[:1] }},
		{"wrong order", func(d *draftSnapshotCandidate) { d.slots[0], d.slots[1] = d.slots[1], d.slots[0] }},
		{"bad font registry", func(d *draftSnapshotCandidate) { d.fonts = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newDraftSnapshotCandidate(t)
			tc.breakGroup(d)
			ticket, err := d.prepare()
			if err == nil || ticket != (draftSnapshotTicket{}) || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 || d.owner.machine.Steps != 0 {
				t.Fatalf("bad group prepare = (%+v, %v), status=%+v", ticket, err, d.owner.Status())
			}
		})
	}
	t.Run("private sealed font tamper", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		ticket, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		d.sealedFonts["draft.snapshot.font"].Glyphs['中'][0] = 0
		shot, err := d.snapshot(ticket, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("tampered shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
	t.Run("private frozen frame tamper", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		ticket, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		d.frame.Indexed[0] ^= 1
		shot, err := d.snapshot(ticket, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("frame tamper shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
	t.Run("new frame invalidates old ticket", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		old, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.prepare(); err != nil {
			t.Fatal(err)
		}
		shot, err := d.snapshot(old, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("old-frame shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
	t.Run("source fault before ticket", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		d.input.err = fmt.Errorf("synthetic source failure")
		ticket, err := d.prepare()
		if err == nil || ticket != (draftSnapshotTicket{}) || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 || d.owner.machine.Steps != 0 {
			t.Fatalf("source prepare = (%+v, %v), status=%+v", ticket, err, d.owner.Status())
		}
	})
	t.Run("frame invalidated while projecting", func(t *testing.T) {
		d := newDraftSnapshotCandidate(t)
		ticket, err := d.prepare()
		if err != nil {
			t.Fatal(err)
		}
		d.onProjectRead = func() { d.frameID++ }
		shot, err := d.snapshot(ticket, host.OutputScale2)
		if err == nil || len(shot.RGBA) != 0 || d.owner.Status().Phase != PhaseFailed || d.owner.closer.(*countingCloser).calls != 1 {
			t.Fatalf("invalidation shot = (%+v, %v), status=%+v", shot, err, d.owner.Status())
		}
	})
}
