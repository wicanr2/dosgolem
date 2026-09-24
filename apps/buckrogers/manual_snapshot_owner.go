package buckrogers

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

const (
	manualSnapshotGroupKey    = "buckrogers.manual.paragraph.v1"
	manualBaseFontIdentity    = "buckrogers.manual.base-16.v1"
	manualDerivedFontIdentity = "buckrogers.manual.derived-22.v1"
)

// ManualFrameTicket binds one already-prepared frame to this owner and its
// lifecycle. Fields are private so a frontend cannot substitute DOS pixels.
type ManualFrameTicket struct {
	owner      *ManualSnapshotOwner
	generation uint64
	epoch      uint64
	frame      host.IndexedFrame
	digest     [sha256.Size]byte
	layersHash [sha256.Size]byte
}

// ManualSnapshotOwner is the only mutable owner of one manual presenter and
// its event consumer. Its methods must be called on the machine-step goroutine.
// It is output-only and never accesses machine, input, answers, or save data.
type ManualSnapshotOwner struct {
	overlay  *RuntimeManualOverlay
	consumer *ManualPresentationConsumer
	base     *xlate.Font
	fontHash [sha256.Size]byte
	baseHash [sha256.Size]byte
	e1       bool
	epoch    uint64
	active   bool
	prepared bool
}

func cloneManualBaseFont(source *xlate.Font) *xlate.Font {
	copy := &xlate.Font{Name: manualBaseFontIdentity, W: source.W, H: source.H, Glyphs: make(map[rune][]byte, len(source.Glyphs))}
	for r, glyph := range source.Glyphs {
		copy.Glyphs[r] = append([]byte(nil), glyph...)
	}
	return copy
}

// NewManualSnapshotOwner pins the supplied font's content for this session.
// Its caller must first verify the local GOLEMFNT and manifest against project
// spec 008 and current catalogs; this constructor does not authenticate files.
func NewManualSnapshotOwner(layout *ManualOverlayLayout, catalog *Catalog, verifiedBase *xlate.Font, scale int) (*ManualSnapshotOwner, error) {
	if verifiedBase == nil || verifiedBase.W != 16 || verifiedBase.H != 16 {
		return nil, fmt.Errorf("buckrogers: manual owner requires verified 16x16 base font")
	}
	base := cloneManualBaseFont(verifiedBase)
	if _, err := presentation.FontFingerprint(base); err != nil {
		return nil, err
	}
	overlay, err := NewRuntimeManualOverlay(layout, catalog, base, scale)
	if err != nil {
		return nil, err
	}
	if scale == 3 {
		// NewRuntimeManualOverlay derives this bitmap but leaves Name empty.
		// Assign identity before Apply can create any text stamp.
		overlay.font.Name = manualDerivedFontIdentity
	} else {
		overlay.font.Name = manualBaseFontIdentity
	}
	fingerprint, err := presentation.FontFingerprint(overlay.font)
	if err != nil {
		return nil, err
	}
	baseFingerprint, err := presentation.FontFingerprint(base)
	if err != nil {
		return nil, err
	}
	if scale == 3 {
		overlay.e1Base = base
	}
	consumer, err := NewManualPresentationConsumer(overlay)
	if err != nil {
		return nil, err
	}
	return &ManualSnapshotOwner{overlay: overlay, consumer: consumer, base: base, fontHash: fingerprint, baseHash: baseFingerprint, e1: scale == 3, epoch: 1, active: true}, nil
}

// SetStyle passes through only the original watcher-observed palette indexes.
func (o *ManualSnapshotOwner) SetStyle(style ManualTextStyle) error {
	if o == nil || !o.active || o.overlay == nil {
		return fmt.Errorf("buckrogers: inactive manual owner")
	}
	if err := o.overlay.SetStyle(style); err != nil {
		return err
	}
	if o.overlay.e1Plan != nil {
		o.overlay.e1Plan = nil
		o.overlay.resetLayers()
		o.overlay.state = manualOverlayCleared
	}
	o.bumpEpoch()
	return nil
}

// Consume preserves the existing append-only consumer and invalidates every
// prepared ticket when one or more lifecycle events were successfully applied.
func (o *ManualSnapshotOwner) Consume(events []ManualPresentationEvent) (int, error) {
	if o == nil || !o.active || o.consumer == nil {
		return 0, fmt.Errorf("buckrogers: inactive manual owner")
	}
	count, err := o.consumer.Consume(events)
	if err != nil {
		// Consumer may have accepted a prefix before rejecting a later event.
		// Retire the owner so no visible prefix can be mistaken for a complete
		// presentation transaction after an error.
		o.Invalidate()
		return count, err
	}
	if count > 0 {
		o.bumpEpoch()
	}
	return count, nil
}

func (o *ManualSnapshotOwner) bumpEpoch() {
	o.prepared = false
	if o.epoch == ^uint64(0) {
		o.active = false
		return
	}
	o.epoch++
}

// Invalidate retires this owner after Restore, stop, source discontinuity, or
// scale replacement. The session must create a new owner before showing manual
// text again; old tickets cannot resurrect a pre-Restore layer.
func (o *ManualSnapshotOwner) Invalidate() {
	if o == nil {
		return
	}
	o.bumpEpoch()
	o.active = false
}

func (o *ManualSnapshotOwner) validateSourceLayers() error {
	if o == nil || !o.active || o.overlay == nil || o.overlay.state != manualOverlayVisible || o.overlay.generation == 0 ||
		o.overlay.background == nil || o.overlay.text == nil || o.overlay.font == nil || o.overlay.layout == nil || !o.overlay.layout.confirmed() ||
		o.overlay.background.W != 320 || o.overlay.background.H != 200 || o.overlay.text.W != 320 || o.overlay.text.H != 200 {
		return fmt.Errorf("buckrogers: manual group is not visible and complete")
	}
	if len(o.overlay.background.Stamps) != o.overlay.layout.rows || len(o.overlay.text.Stamps) != o.overlay.layout.rows {
		return fmt.Errorf("buckrogers: manual group requires all fourteen rows")
	}
	if len(o.overlay.actions) == 0 {
		return fmt.Errorf("buckrogers: visible manual group has no accepted action")
	}
	action := o.overlay.actions[len(o.overlay.actions)-1]
	if action.Generation != o.overlay.generation || action.EventKey == "" || action.TextKey == "" {
		return fmt.Errorf("buckrogers: manual action identity differs from visible group")
	}
	var translation string
	matches := 0
	for _, entry := range o.overlay.catalog.byIdentity {
		if entry.eventKey == action.EventKey && entry.textKey == action.TextKey {
			translation = entry.translation
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("buckrogers: manual action does not identify one catalog entry")
	}
	if o.e1 {
		return o.validateE1SourceLayers(action, translation)
	}
	rows, err := manualRows(translation, o.overlay.layout.columns, o.overlay.layout.rows)
	if err != nil {
		return err
	}
	for row := 0; row < o.overlay.layout.rows; row++ {
		background, text := o.overlay.background.Stamps[row], o.overlay.text.Stamps[row]
		if background == nil || text == nil {
			return fmt.Errorf("buckrogers: nil manual source stamp before Snapshot")
		}
		y := o.overlay.layout.clearY + row*(o.overlay.layout.lineHeight+o.overlay.layout.gap)
		if background.Key != fmt.Sprintf("manual.background.%d", row) || background.X != o.overlay.layout.clearX || background.Y != y ||
			background.Cells != 1 || background.CellW != o.overlay.layout.clearWidth || background.CellH != o.overlay.layout.lineHeight ||
			background.Font != nil || len(background.Text) != 0 || background.Owner != "" || len(background.Transparent) != 0 ||
			background.SwapColors || background.GlyphX != 0 || background.GlyphY != 0 || background.GlyphScale != 0 ||
			text.Key != fmt.Sprintf("manual.%d.%s.%02d", action.Generation, action.TextKey, row) ||
			string(text.Text) != rows[row] || text.Owner != "" || len(text.Transparent) != 0 || text.SwapColors || text.GlyphScale != 0 ||
			text.X != o.overlay.layout.textX || text.Y != y || text.Cells != o.overlay.layout.columns || text.CellW != 8 ||
			text.CellH != o.overlay.layout.lineHeight || text.Font == nil || text.Font != o.overlay.font || text.Font.Name == "" ||
			text.GlyphX != manualGlyphOffset(o.overlay.scale) || text.GlyphY != manualGlyphOffset(o.overlay.scale) {
			return fmt.Errorf("buckrogers: manual row %d geometry or font changed", row)
		}
	}
	fingerprint, err := presentation.FontFingerprint(o.overlay.font)
	if err != nil || fingerprint != o.fontHash {
		return fmt.Errorf("buckrogers: manual font differs from session identity")
	}
	return nil
}

func (o *ManualSnapshotOwner) validateE1SourceLayers(action ManualOverlayAction, translation string) error {
	plan := o.overlay.e1Plan
	if plan == nil || plan.generation != action.Generation || plan.eventKey != action.EventKey ||
		plan.textKey != action.TextKey || plan.translation != translation || plan.digest != plan.hash() ||
		o.overlay.e1Base != o.base || o.overlay.font == o.base {
		return fmt.Errorf("buckrogers: E1 plan identity differs from visible manual group")
	}
	baseHash, err := presentation.FontFingerprint(o.base)
	if err != nil || baseHash != o.baseHash || baseHash != plan.baseSeal {
		return fmt.Errorf("buckrogers: E1 base font changed")
	}
	derivedHash, err := presentation.FontFingerprint(o.overlay.font)
	if err != nil || derivedHash != o.fontHash || derivedHash != plan.derivedSeal {
		return fmt.Errorf("buckrogers: E1 derived font changed")
	}
	expected, err := plan.TextLayer(o.base, o.overlay.font)
	if err != nil {
		return err
	}
	if len(expected.Stamps) != len(o.overlay.text.Stamps) || len(o.overlay.text.FontRegistry) != 2 ||
		o.overlay.text.FontRegistry[o.base.Name] != o.base || o.overlay.text.FontRegistry[o.overlay.font.Name] != o.overlay.font {
		return fmt.Errorf("buckrogers: E1 text registry or row count changed")
	}
	for row := range expected.Stamps {
		background, actual, want := o.overlay.background.Stamps[row], o.overlay.text.Stamps[row], expected.Stamps[row]
		if background == nil || actual == nil {
			return fmt.Errorf("buckrogers: nil E1 manual stamp")
		}
		y := o.overlay.layout.clearY + row*(o.overlay.layout.lineHeight+o.overlay.layout.gap)
		if background.Key != fmt.Sprintf("manual.background.%d", row) || background.X != o.overlay.layout.clearX || background.Y != y ||
			background.Cells != 1 || background.CellW != o.overlay.layout.clearWidth || background.CellH != o.overlay.layout.lineHeight ||
			background.Font != nil || len(background.Text) != 0 || background.Owner != "" || len(background.Transparent) != 0 ||
			background.SwapColors || background.GlyphX != 0 || background.GlyphY != 0 || background.GlyphScale != 0 ||
			background.PixelScale != 0 || len(background.PixelGlyphs) != 0 ||
			actual.Key != want.Key || actual.X != want.X || actual.Y != want.Y || actual.Cells != want.Cells ||
			actual.CellW != want.CellW || actual.CellH != want.CellH || actual.PixelScale != want.PixelScale ||
			actual.Font != nil || len(actual.Text) != 0 || actual.Owner != "" || len(actual.Transparent) != 0 ||
			actual.SwapColors || actual.GlyphX != 0 || actual.GlyphY != 0 || actual.GlyphScale != 0 ||
			len(actual.PixelGlyphs) != len(want.PixelGlyphs) {
			return fmt.Errorf("buckrogers: E1 row %d geometry changed", row)
		}
		for index, glyph := range actual.PixelGlyphs {
			if glyph != want.PixelGlyphs[index] {
				return fmt.Errorf("buckrogers: E1 row %d glyph changed", row)
			}
		}
	}
	return o.overlay.text.ValidatePixelGlyphPlan(3)
}

func digestManualFrame(frame host.IndexedFrame) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write(frame.Indexed)
	for _, rgb := range frame.Palette {
		_, _ = h.Write(rgb[:])
	}
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

func digestManualLayers(overlay *RuntimeManualOverlay) ([sha256.Size]byte, error) {
	background, err := overlay.background.Snapshot()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	text, err := overlay.text.Snapshot()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	h := sha256.New()
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(background)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(background)
	binary.LittleEndian.PutUint64(size[:], uint64(len(text)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(text)
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

// PrepareFrame is called after one same-goroutine machine frame read. It owns a
// copy of that exact input and advances only the output-side Layer.Frame state.
func (o *ManualSnapshotOwner) PrepareFrame(frame host.IndexedFrame) (ManualFrameTicket, error) {
	if o == nil || !o.active {
		return ManualFrameTicket{}, fmt.Errorf("buckrogers: inactive manual owner")
	}
	// A newer frame attempt invalidates every earlier ticket, even when the
	// new input or Layer.Frame later fails. Generation alone is not a frame ID.
	o.bumpEpoch()
	if !o.active {
		return ManualFrameTicket{}, fmt.Errorf("buckrogers: manual frame epoch exhausted")
	}
	if err := o.validateSourceLayers(); err != nil {
		return ManualFrameTicket{}, err
	}
	if frame.Canvas != (host.Canvas{Width: 320, Height: 200}) || len(frame.Indexed) != 320*200 {
		return ManualFrameTicket{}, fmt.Errorf("buckrogers: invalid manual presentation frame")
	}
	private := host.IndexedFrame{Canvas: frame.Canvas, Indexed: append([]byte(nil), frame.Indexed...), Palette: frame.Palette}
	o.overlay.Frame(private.Indexed, private.Palette)
	if err := o.validateSourceLayers(); err != nil {
		return ManualFrameTicket{}, err
	}
	for _, layer := range []*xlate.Layer{o.overlay.background, o.overlay.text} {
		for _, stamp := range layer.Stamps {
			if stamp.State != xlate.Shown {
				return ManualFrameTicket{}, fmt.Errorf("buckrogers: manual stamp not shown after Frame")
			}
		}
	}
	layersHash, err := digestManualLayers(o.overlay)
	if err != nil {
		return ManualFrameTicket{}, err
	}
	o.prepared = true
	return ManualFrameTicket{owner: o, generation: o.overlay.generation, epoch: o.epoch, frame: private, digest: digestManualFrame(private), layersHash: layersHash}, nil
}

type manualTicketSource struct{ frame host.IndexedFrame }

func (s manualTicketSource) ReadPresentationFrame() (host.IndexedFrame, error) { return s.frame, nil }

// Snapshot atomically projects the prepared frozen frame. It does not run
// Layer.Frame or step the machine, and returns no partial RGBA on any failure.
func (o *ManualSnapshotOwner) Snapshot(ticket ManualFrameTicket, scale int) (presentation.LayerPresentationSnapshot, error) {
	result, err := o.snapshot(ticket, scale)
	if err != nil && o != nil && o.active && o.e1 {
		// A failed E1 projection consumes the current frame lease. Repairing a
		// mutable source after failure cannot resurrect its old ticket.
		o.bumpEpoch()
	}
	return result, err
}

func (o *ManualSnapshotOwner) snapshot(ticket ManualFrameTicket, scale int) (presentation.LayerPresentationSnapshot, error) {
	if o == nil || !o.active || !o.prepared || o.overlay == nil || scale != o.overlay.scale || ticket.owner != o ||
		ticket.generation == 0 || ticket.generation != o.overlay.generation || ticket.epoch != o.epoch ||
		ticket.frame.Canvas != (host.Canvas{Width: 320, Height: 200}) || len(ticket.frame.Indexed) != 320*200 ||
		ticket.digest != digestManualFrame(ticket.frame) {
		return presentation.LayerPresentationSnapshot{}, fmt.Errorf("buckrogers: stale or modified manual frame ticket")
	}
	if err := o.validateSourceLayers(); err != nil {
		return presentation.LayerPresentationSnapshot{}, err
	}
	layersHash, err := digestManualLayers(o.overlay)
	if err != nil || layersHash != ticket.layersHash {
		return presentation.LayerPresentationSnapshot{}, fmt.Errorf("buckrogers: manual layers changed after Frame")
	}
	var result presentation.LayerPresentationSnapshot
	fonts := map[string]*xlate.Font{o.overlay.font.Name: o.overlay.font}
	if o.e1 {
		fonts[o.base.Name] = o.base
	}
	err = presentation.WithSealedOrderedLayers(manualSnapshotGroupKey, ticket.generation, ticket.epoch,
		[]presentation.ActiveLayerSlot{
			{Name: "background", Z: 0, Layer: o.overlay.background},
			{Name: "text", Z: 1, Layer: o.overlay.text},
		}, fonts, func(group *presentation.SealedLayerGroup) error {
			if fingerprint, ok := group.FontHash(o.overlay.font.Name); !ok || fingerprint != o.fontHash {
				return fmt.Errorf("buckrogers: sealed manual font identity mismatch")
			}
			if o.e1 {
				if fingerprint, ok := group.FontHash(o.base.Name); !ok || fingerprint != o.baseHash {
					return fmt.Errorf("buckrogers: sealed E1 base font identity mismatch")
				}
			}
			valid := func() bool {
				currentLayersHash, err := digestManualLayers(o.overlay)
				if o.e1 && o.validateSourceLayers() != nil {
					return false
				}
				return o.active && o.prepared && o.overlay.state == manualOverlayVisible &&
					o.overlay.generation == ticket.generation && o.epoch == ticket.epoch && ticket.digest == digestManualFrame(ticket.frame) &&
					err == nil && currentLayersHash == ticket.layersHash
			}
			projected, err := presentation.ProjectSealedLayers(manualTicketSource{frame: ticket.frame}, group, scale, valid)
			if err != nil {
				return err
			}
			result = projected
			return nil
		})
	if err != nil {
		return presentation.LayerPresentationSnapshot{}, err
	}
	if !result.Drew || len(result.Missing) != 0 {
		return presentation.LayerPresentationSnapshot{}, fmt.Errorf("buckrogers: manual group did not draw completely")
	}
	return result, nil
}
