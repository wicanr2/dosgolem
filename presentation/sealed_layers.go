package presentation

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"sort"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/xlate"
)

// ActiveLayerSlot is consumed synchronously by SealOrderedLayers. The returned
// group retains neither Layer nor Font pointers supplied by the caller.
type ActiveLayerSlot struct {
	Name  string
	Z     int
	Layer *xlate.Layer
}

type sealedLayerSlot struct {
	name string
	z    int
	data []byte
}

// SealedLayerGroup owns private layer bytes and font copies. Its value is not
// a lifecycle token: the owner must additionally validate its generation and
// epoch before and after each frame read.
type SealedLayerGroup struct {
	active     bool
	key        string
	generation uint64
	epoch      uint64
	canvas     host.Canvas
	slots      []sealedLayerSlot
	fonts      map[string]*xlate.Font
	fontHashes map[string][sha256.Size]byte
	digest     [sha256.Size]byte
}

// FontFingerprint validates all bitmap lengths and hashes a font independently
// of Go map order. A name is part of its identity, not a substitute for bytes.
func FontFingerprint(font *xlate.Font) ([sha256.Size]byte, error) {
	if font == nil || font.Name == "" || font.W <= 0 || font.H <= 0 || font.W > 4096 || font.H > 4096 {
		return [sha256.Size]byte{}, fmt.Errorf("presentation: invalid named font")
	}
	rowBytes := (font.W + 7) / 8
	if rowBytes > int(^uint(0)>>1)/font.H {
		return [sha256.Size]byte{}, fmt.Errorf("presentation: font bitmap length overflow")
	}
	glyphBytes := rowBytes * font.H
	runes := make([]rune, 0, len(font.Glyphs))
	for r, glyph := range font.Glyphs {
		if len(glyph) != glyphBytes {
			return [sha256.Size]byte{}, fmt.Errorf("presentation: malformed glyph U+%04X", r)
		}
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	h := sha256.New()
	writeSealedString(h, font.Name)
	writeSealedU64(h, uint64(font.W))
	writeSealedU64(h, uint64(font.H))
	writeSealedU64(h, uint64(len(runes)))
	for _, r := range runes {
		writeSealedU64(h, uint64(r))
		_, _ = h.Write(font.Glyphs[r])
	}
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func writeSealedU64(h hash.Hash, value uint64) {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], value)
	_, _ = h.Write(data[:])
}

func writeSealedString(h hash.Hash, value string) {
	writeSealedU64(h, uint64(len(value)))
	_, _ = h.Write([]byte(value))
}

func cloneSealedFont(font *xlate.Font) *xlate.Font {
	copy := &xlate.Font{Name: font.Name, W: font.W, H: font.H, Glyphs: make(map[rune][]byte, len(font.Glyphs))}
	for r, glyph := range font.Glyphs {
		copy.Glyphs[r] = append([]byte(nil), glyph...)
	}
	return copy
}

// WithSealedOrderedLayers validates source stamps before Snapshot, copies all
// layer/font data, and grants a group only for the callback's dynamic extent.
// Capturing the pointer cannot keep its projection lease alive.
func WithSealedOrderedLayers(key string, generation, epoch uint64, slots []ActiveLayerSlot, fonts map[string]*xlate.Font, callback func(*SealedLayerGroup) error) error {
	if callback == nil {
		return fmt.Errorf("presentation: sealed group callback is nil")
	}
	group, err := sealOrderedLayers(key, generation, epoch, slots, fonts)
	if err != nil {
		return err
	}
	group.active = true
	defer func() { group.active = false }()
	return callback(group)
}

func sealOrderedLayers(key string, generation, epoch uint64, slots []ActiveLayerSlot, fonts map[string]*xlate.Font) (*SealedLayerGroup, error) {
	if key == "" || generation == 0 || epoch == 0 || len(slots) == 0 {
		return nil, fmt.Errorf("presentation: invalid sealed group identity")
	}
	group := &SealedLayerGroup{key: key, generation: generation, epoch: epoch, fonts: make(map[string]*xlate.Font, len(fonts)), fontHashes: make(map[string][sha256.Size]byte, len(fonts))}
	for name, font := range fonts {
		if name == "" || font == nil || font.Name != name {
			return nil, fmt.Errorf("presentation: invalid font registry %q", name)
		}
		fingerprint, err := FontFingerprint(font)
		if err != nil {
			return nil, err
		}
		group.fonts[name] = cloneSealedFont(font)
		group.fontHashes[name] = fingerprint
	}
	seen := make(map[string]bool, len(slots))
	for i, slot := range slots {
		if slot.Name == "" || seen[slot.Name] || slot.Layer == nil || slot.Layer.W <= 0 || slot.Layer.H <= 0 || i > 0 && slot.Z <= slots[i-1].Z {
			return nil, fmt.Errorf("presentation: invalid or unordered layer slot")
		}
		seen[slot.Name] = true
		canvas := host.Canvas{Width: slot.Layer.W, Height: slot.Layer.H}
		if i == 0 {
			group.canvas = canvas
		} else if canvas != group.canvas {
			return nil, fmt.Errorf("presentation: layer canvas mismatch")
		}
		for _, stamp := range slot.Layer.Stamps {
			if stamp == nil {
				return nil, fmt.Errorf("presentation: nil source stamp before Snapshot")
			}
			if stamp.Font != nil && (stamp.Font.Name == "" || fonts[stamp.Font.Name] != stamp.Font) {
				return nil, fmt.Errorf("presentation: source stamp font is not exactly registered")
			}
			for _, glyph := range stamp.PixelGlyphs {
				if glyph.Font == nil || glyph.Font.Name == "" || fonts[glyph.Font.Name] != glyph.Font {
					return nil, fmt.Errorf("presentation: source pixel glyph font is not exactly registered")
				}
			}
		}
		// A caller supplies the canonical font registry to this seal operation;
		// do not mutate its source layer merely to satisfy xlate's pixel-plan
		// preflight. Snapshot reads the shallow layer value synchronously.
		snapshotLayer := *slot.Layer
		snapshotLayer.FontRegistry = fonts
		data, err := snapshotLayer.Snapshot()
		if err != nil {
			return nil, fmt.Errorf("presentation: snapshot layer %q: %w", slot.Name, err)
		}
		group.slots = append(group.slots, sealedLayerSlot{name: slot.Name, z: slot.Z, data: data})
	}
	group.digest = digestSealedGroup(group)
	return group, nil
}

func digestSealedGroup(group *SealedLayerGroup) [sha256.Size]byte {
	h := sha256.New()
	writeSealedString(h, group.key)
	writeSealedU64(h, group.generation)
	writeSealedU64(h, group.epoch)
	writeSealedU64(h, uint64(group.canvas.Width))
	writeSealedU64(h, uint64(group.canvas.Height))
	for _, slot := range group.slots {
		writeSealedString(h, slot.name)
		writeSealedU64(h, uint64(slot.z))
		writeSealedU64(h, uint64(len(slot.data)))
		_, _ = h.Write(slot.data)
	}
	names := make([]string, 0, len(group.fonts))
	for name := range group.fonts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writeSealedString(h, name)
		fingerprint, err := FontFingerprint(group.fonts[name])
		if err != nil {
			writeSealedString(h, "invalid")
		} else {
			_, _ = h.Write(fingerprint[:])
		}
	}
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Identity returns values only, never the group's layer or font storage.
func (g *SealedLayerGroup) Identity() (string, uint64, uint64) {
	if g == nil || !g.active {
		return "", 0, 0
	}
	return g.key, g.generation, g.epoch
}

// FontHash returns the sealed content fingerprint for one registered name.
func (g *SealedLayerGroup) FontHash(name string) ([sha256.Size]byte, bool) {
	if g == nil || !g.active {
		return [sha256.Size]byte{}, false
	}
	value, ok := g.fontHashes[name]
	return value, ok
}

func (g *SealedLayerGroup) preflight(scale int) ([]*xlate.Layer, error) {
	if g == nil || !g.active || g.key == "" || g.generation == 0 || g.epoch == 0 || scale <= 0 || len(g.slots) == 0 || g.canvas.Width <= 0 || g.canvas.Height <= 0 || g.digest != digestSealedGroup(g) {
		return nil, fmt.Errorf("presentation: invalid or modified sealed group")
	}
	for name, font := range g.fonts {
		fingerprint, err := FontFingerprint(font)
		if err != nil || fingerprint != g.fontHashes[name] {
			return nil, fmt.Errorf("presentation: sealed font identity changed")
		}
	}
	copies := make([]*xlate.Layer, 0, len(g.slots))
	for i, slot := range g.slots {
		if slot.name == "" || len(slot.data) == 0 || i > 0 && slot.z <= g.slots[i-1].z {
			return nil, fmt.Errorf("presentation: invalid sealed slot")
		}
		layer := &xlate.Layer{}
		if err := layer.Restore(slot.data, g.fonts); err != nil {
			return nil, fmt.Errorf("presentation: restore sealed slot: %w", err)
		}
		if layer.W != g.canvas.Width || layer.H != g.canvas.Height {
			return nil, fmt.Errorf("presentation: sealed canvas mismatch")
		}
		if err := validateSealedStamps(layer, g.fonts, scale); err != nil {
			return nil, err
		}
		copies = append(copies, layer)
	}
	return copies, nil
}

func validateSealedStamps(layer *xlate.Layer, fonts map[string]*xlate.Font, scale int) error {
	if err := layer.ValidatePixelGlyphPlan(scale); err != nil {
		return fmt.Errorf("presentation: invalid sealed pixel glyph plan: %w", err)
	}
	for _, stamp := range layer.Stamps {
		if stamp == nil || stamp.State != xlate.Shown || stamp.X < 0 || stamp.Y < 0 || stamp.Cells <= 0 || stamp.CellW <= 0 || stamp.CellH <= 0 || stamp.GlyphX < 0 || stamp.GlyphY < 0 || stamp.GlyphScale < 0 || len(stamp.Text) > stamp.Cells || len(stamp.Transparent) > stamp.Cells || stamp.X >= layer.W || stamp.Y >= layer.H || stamp.Cells > (layer.W-stamp.X)/stamp.CellW || stamp.CellH > layer.H-stamp.Y {
			return fmt.Errorf("presentation: invalid sealed stamp geometry or state")
		}
		if stamp.Font == nil {
			if len(stamp.Text) != 0 {
				return fmt.Errorf("presentation: text without font")
			}
			continue
		}
		if stamp.PixelScale != 0 {
			continue // The checked physical plan validates its own font and crop.
		}
		font := stamp.Font
		if font.Name == "" || fonts[font.Name] != font {
			return fmt.Errorf("presentation: sealed stamp font not registered")
		}
		k := stamp.GlyphScale
		if k == 0 {
			k = scale / 3
			if k < 1 {
				k = 1
			}
		}
		if stamp.CellW > int(^uint(0)>>1)/scale || stamp.CellH > int(^uint(0)>>1)/scale || font.W > (stamp.CellW*scale-stamp.GlyphX)/k || font.H > (stamp.CellH*scale-stamp.GlyphY)/k {
			return fmt.Errorf("presentation: glyph outside text-safe cell")
		}
		for _, r := range stamp.Text {
			if r != ' ' && r != '　' {
				if _, ok := font.Glyphs[r]; !ok {
					return fmt.Errorf("presentation: missing sealed glyph U+%04X", r)
				}
			}
		}
	}
	return nil
}

// ProjectSealedLayers preflights all layers before its one FrameSource read.
// valid is the owner's callback-scoped lease check, invoked before and after
// that read. Errors return a zero snapshot, never partially drawn RGBA.
func ProjectSealedLayers(source host.FrameSource, group *SealedLayerGroup, scale int, valid func() bool) (LayerPresentationSnapshot, error) {
	if valid == nil || !valid() {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: stale sealed group owner")
	}
	copies, err := group.preflight(scale)
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	frames, err := host.NewPresentationSnapshotProvider(source)
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	frame, err := frames.Snapshot()
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	if !valid() || !group.active || group.digest != digestSealedGroup(group) {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: sealed group changed while reading frame")
	}
	if frame.Canvas != group.canvas {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: frame canvas differs from sealed layers")
	}
	rgba, err := scaleIndexedRGBA(frame, scale)
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	missing := []rune{}
	drew := false
	for _, layer := range copies {
		layerDrew, err := layer.DrawChecked(rgba, scale, func(r rune) { missing = append(missing, r) })
		if err != nil {
			return LayerPresentationSnapshot{}, fmt.Errorf("presentation: checked sealed draw: %w", err)
		}
		if layerDrew {
			drew = true
		}
	}
	if len(missing) != 0 {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: glyph missing after preflight")
	}
	return LayerPresentationSnapshot{Frame: frame, Scale: scale, RGBA: rgba, Drew: drew}, nil
}
