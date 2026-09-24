package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

const (
	menuThreeXFontName = "buckrogers.menu.eten22.v1"
	menuBaseFontName   = "buckrogers.menu.eten16.v1"
	menuFontSourceSHA  = "150c93afaa10f1f09f146c9b67ba6fdca35aa5d13d1b6f965cfdedb33a8a5174"
	menuSealedGroupKey = "buckrogers.menu.scoped.v1"
)

var scopedThreeXMenuKeys = map[string]bool{
	"menu.transition.old.create_new_character":       true,
	"function.menu.option.create_new_character":      true,
	"function.menu.option.add_character_to_team":     true,
	"function.menu.option.load_saved_game":           true,
	"function.menu.option.joystick_mouse_initialize": true,
	"function.menu.option.exit_to_dos":               true,
	"function.menu.selected.create_new_character":    true,
	"function.menu.instruction.choose_function":      true,
	"function.menu.selected.add_character_to_team":   true,
	"function.menu.normal.add_character_to_team":     true,
}

var scopedRaceMenuKeys = map[string]bool{
	"race.screen.prompt": true, "race.option.terran": true,
	"race.option.martian": true, "race.option.venusian": true,
	"race.option.mercurian": true, "race.option.tinker": true,
	"race.option.desert_runner": true, "race.heading.terran": true,
	"race.selection.normal.terran":    true,
	"race.selection.selected.martian": true,
	"race.selection.normal.martian":   true,
}

func validateScopedMenuFontPair(base, derived *xlate.Font) error {
	if base == nil || base.W != 16 || base.H != 16 || derived == nil || derived.W != 22 || derived.H != 22 || derived.Name != menuThreeXFontName {
		return fmt.Errorf("buckrogers: scoped menu requires 16x16 source and named 22x22 output")
	}
	for r, glyph := range base.Glyphs {
		if len(glyph) != 32 {
			return fmt.Errorf("buckrogers: malformed 16px source glyph U+%04X", r)
		}
	}
	for r, glyph := range derived.Glyphs {
		if len(glyph) != 66 {
			return fmt.Errorf("buckrogers: malformed 22px derived glyph U+%04X", r)
		}
	}
	if len(base.Glyphs) != len(derived.Glyphs) {
		return fmt.Errorf("buckrogers: scoped menu font coverage differs")
	}
	// The current manual 16→22 helper implements the READY v1 rule. Validate
	// source lengths before calling it: the helper indexes every glyph.
	expected := manualThreeXFont(base)
	for r, glyph := range expected.Glyphs {
		if !bytes.Equal(derived.Glyphs[r], glyph) {
			return fmt.Errorf("buckrogers: derived glyph U+%04X differs from menu v1", r)
		}
	}
	return nil
}

// BuildScopedThreeXMenuOverlay is the opt-in, whole-batch 3x menu builder.
// The existing 16px BuildMenuOverlay API and all its callers are unchanged.
func BuildScopedThreeXMenuOverlay(entries []MenuOverlayEntry, base, derived *xlate.Font, palette [256][3]uint8, scale int) (*MenuOverlay, error) {
	if err := validateScopedMenuFontPair(base, derived); err != nil {
		return nil, err
	}
	return buildScopedThreeXMenuOverlayValidated(entries, base, derived, palette, scale)
}

// The runtime has already validated the derivation once at construction and
// checks both immutable fingerprints before every Apply. Avoid making a
// second transient font object for each event in that hot path.
func buildScopedThreeXMenuOverlayValidated(entries []MenuOverlayEntry, base, derived *xlate.Font, palette [256][3]uint8, scale int) (*MenuOverlay, error) {
	if scale != 3 {
		return nil, fmt.Errorf("buckrogers: scoped 22px menu requires scale 3")
	}
	for _, entry := range entries {
		if !scopedThreeXMenuKeys[entry.EventKey] {
			return nil, fmt.Errorf("buckrogers: non-menu event %q in scoped 22px batch", entry.EventKey)
		}
	}
	// Build the complete 16px geometry first. No caller can see this private
	// layer if any later 22px ink validation fails.
	overlay, err := BuildMenuOverlay(entries, base, palette, scale)
	if err != nil {
		return nil, err
	}
	for i, stamp := range overlay.Layer.Stamps {
		stamp.Font, stamp.GlyphX, stamp.GlyphY, stamp.GlyphScale = derived, 1, 1, 1
		ink, err := menuInkRect(stamp, scale)
		if err != nil {
			return nil, fmt.Errorf("buckrogers: %s: %w", stamp.Key, err)
		}
		clear := overlay.Events[i].ClearRect
		if ink.Width <= 0 || ink.Height <= 0 || ink.X < clear.X || ink.Y < clear.Y ||
			ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
			return nil, fmt.Errorf("buckrogers: %s 22px ink escapes safe rectangle", stamp.Key)
		}
		overlay.Events[i].InkRect = ink
	}
	return overlay, nil
}

type scopedMenuFonts struct {
	catalog     *MenuCatalog
	source      []byte
	sourceHash  [sha256.Size]byte
	baseHash    [sha256.Size]byte
	derivedHash [sha256.Size]byte
	derived     *xlate.Font
	registry    map[string]*xlate.Font
}

func validateScopedMenuCatalog(catalog *MenuCatalog, rects *MenuOverlayRects) error {
	if catalog == nil || rects == nil || len(catalog.byIdentity) != len(scopedThreeXMenuKeys)+len(scopedRaceMenuKeys) {
		return fmt.Errorf("buckrogers: scoped menu requires the 21 canonical identities")
	}
	seen := map[string]bool{}
	for _, entry := range catalog.byIdentity {
		if !scopedThreeXMenuKeys[entry.eventKey] && !scopedRaceMenuKeys[entry.eventKey] || seen[entry.eventKey] {
			return fmt.Errorf("buckrogers: scoped menu has unknown or duplicate identity %q", entry.eventKey)
		}
		seen[entry.eventKey] = true
		if _, ok := rects.byEvent[entry.eventKey]; !ok {
			return fmt.Errorf("buckrogers: scoped menu lacks rectangle for %q", entry.eventKey)
		}
	}
	return nil
}

// NewScopedMenuRuntimeOverlay authenticates the local GOLEMFNT bytes before
// creating a private font and an exact-catalog runtime. No original asset is
// embedded or emitted. A caller must pass the formal menu-only catalog.
func NewScopedMenuRuntimeOverlay(rects *MenuOverlayRects, catalog *MenuCatalog, sourceGOLEMFNT []byte, scale int) (*RuntimeMenuOverlay, error) {
	expected, _ := hex.DecodeString(menuFontSourceSHA)
	var digest [sha256.Size]byte
	copy(digest[:], expected)
	return newScopedMenuRuntimeOverlay(rects, catalog, sourceGOLEMFNT, scale, digest)
}

func newScopedMenuRuntimeOverlay(rects *MenuOverlayRects, catalog *MenuCatalog, sourceGOLEMFNT []byte, scale int, expected [sha256.Size]byte) (*RuntimeMenuOverlay, error) {
	if scale != 2 && scale != 3 {
		return nil, fmt.Errorf("buckrogers: scoped menu scale must be 2 or 3")
	}
	if err := validateScopedMenuCatalog(catalog, rects); err != nil {
		return nil, err
	}
	source := append([]byte(nil), sourceGOLEMFNT...)
	if sha256.Sum256(source) != expected {
		return nil, fmt.Errorf("buckrogers: scoped menu GOLEMFNT SHA-256 mismatch")
	}
	base, err := decodeScopedMenuFont(source)
	if err != nil {
		return nil, err
	}
	base.Name = menuBaseFontName
	if err := validateScopedMenuSource(base); err != nil {
		return nil, err
	}
	derived := manualThreeXFont(base)
	derived.Name = menuThreeXFontName
	if err := validateScopedMenuFontPair(base, derived); err != nil {
		return nil, err
	}
	baseHash, err := presentation.FontFingerprint(base)
	if err != nil {
		return nil, err
	}
	derivedHash, err := presentation.FontFingerprint(derived)
	if err != nil {
		return nil, err
	}
	overlay, err := NewRuntimeMenuOverlay(rects, base, scale)
	if err != nil {
		return nil, err
	}
	overlay.scoped = &scopedMenuFonts{catalog: catalog, source: source, sourceHash: expected,
		baseHash: baseHash, derivedHash: derivedHash, derived: derived,
		registry: map[string]*xlate.Font{menuBaseFontName: base, menuThreeXFontName: derived}}
	return overlay, nil
}

func validateScopedMenuSource(base *xlate.Font) error {
	if base == nil || base.W != 16 || base.H != 16 {
		return fmt.Errorf("buckrogers: scoped menu source must be 16x16")
	}
	for r, glyph := range base.Glyphs {
		if len(glyph) != 32 {
			return fmt.Errorf("buckrogers: malformed scoped menu source glyph U+%04X", r)
		}
	}
	return nil
}

// Decode only the authenticated, immutable GOLEMFNT byte slice. It mirrors
// xlate.LoadFont's wire format while rejecting duplicate rune records.
func decodeScopedMenuFont(data []byte) (*xlate.Font, error) {
	if len(data) < 16 || string(data[:8]) != "GOLEMFNT" {
		return nil, fmt.Errorf("buckrogers: invalid scoped menu GOLEMFNT header")
	}
	w, h := int(binary.LittleEndian.Uint16(data[8:10])), int(binary.LittleEndian.Uint16(data[10:12]))
	n := int(binary.LittleEndian.Uint32(data[12:16]))
	if w != 16 || h != 16 || n <= 0 || n > (len(data)-16)/37 || len(data) != 16+n*37 {
		return nil, fmt.Errorf("buckrogers: invalid scoped menu GOLEMFNT dimensions or length")
	}
	font := &xlate.Font{W: w, H: h, Glyphs: make(map[rune][]byte, n)}
	for i := 0; i < n; i++ {
		offset := 16 + i*37
		r := rune(binary.LittleEndian.Uint32(data[offset : offset+4]))
		if r < 0 || r > 0x10ffff || r >= 0xd800 && r <= 0xdfff {
			return nil, fmt.Errorf("buckrogers: invalid scoped menu rune U+%04X", r)
		}
		if _, exists := font.Glyphs[r]; exists {
			return nil, fmt.Errorf("buckrogers: duplicate scoped menu rune U+%04X", r)
		}
		font.Glyphs[r] = append([]byte(nil), data[offset+5:offset+37]...)
	}
	return font, nil
}

func (o *RuntimeMenuOverlay) validateScopedMenuFonts() error {
	if o == nil || o.scoped == nil || o.font == nil || o.scoped.derived == nil ||
		o.font.Name != menuBaseFontName || o.scoped.derived.Name != menuThreeXFontName ||
		len(o.scoped.registry) != 2 ||
		o.scoped.registry[menuBaseFontName] != o.font || o.scoped.registry[menuThreeXFontName] != o.scoped.derived ||
		sha256.Sum256(o.scoped.source) != o.scoped.sourceHash {
		return fmt.Errorf("buckrogers: scoped menu source or registry changed")
	}
	baseHash, err := presentation.FontFingerprint(o.font)
	if err != nil || baseHash != o.scoped.baseHash {
		return fmt.Errorf("buckrogers: scoped menu base font fingerprint changed")
	}
	derivedHash, err := presentation.FontFingerprint(o.scoped.derived)
	if err != nil || derivedHash != o.scoped.derivedHash {
		return fmt.Errorf("buckrogers: scoped menu derived font fingerprint changed")
	}
	return nil
}

// ScopedMenuFontFingerprint returns this session's derived-font receipt.
func (o *RuntimeMenuOverlay) ScopedMenuFontFingerprint() ([sha256.Size]byte, error) {
	if err := o.validateScopedMenuFonts(); err != nil {
		return [sha256.Size]byte{}, err
	}
	return o.scoped.derivedHash, nil
}

func (o *RuntimeMenuOverlay) sealScopedMenuLayer(layer *xlate.Layer) error {
	if err := o.validateScopedMenuFonts(); err != nil {
		return err
	}
	if layer == nil {
		return fmt.Errorf("buckrogers: scoped menu layer is nil")
	}
	for _, stamp := range layer.Stamps {
		if stamp == nil || stamp.Font == nil {
			return fmt.Errorf("buckrogers: scoped menu has nil stamp or font")
		}
		want := o.font
		if o.scale == 3 && scopedThreeXMenuKeys[stamp.Key] {
			want = o.scoped.derived
		} else if !scopedRaceMenuKeys[stamp.Key] && !scopedThreeXMenuKeys[stamp.Key] {
			return fmt.Errorf("buckrogers: scoped menu has unknown active key %q", stamp.Key)
		}
		if stamp.Font != want {
			return fmt.Errorf("buckrogers: scoped menu stamp %q uses wrong font", stamp.Key)
		}
	}
	return presentation.WithSealedOrderedLayers(menuSealedGroupKey, 1, 1,
		[]presentation.ActiveLayerSlot{{Name: "menu", Z: 1, Layer: layer}}, o.scoped.registry,
		func(group *presentation.SealedLayerGroup) error {
			for name, want := range map[string][sha256.Size]byte{menuBaseFontName: o.scoped.baseHash, menuThreeXFontName: o.scoped.derivedHash} {
				got, ok := group.FontHash(name)
				if !ok || got != want {
					return fmt.Errorf("buckrogers: sealed menu font %q fingerprint changed", name)
				}
			}
			return nil
		})
}

// SnapshotScopedLayer and RestoreScopedLayer keep the font registry tied to
// this one runtime session. They do not authorize cross-session persistence.
func (o *RuntimeMenuOverlay) SnapshotScopedLayer() ([]byte, error) {
	if o == nil {
		return nil, fmt.Errorf("buckrogers: scoped menu runtime is nil")
	}
	if err := o.sealScopedMenuLayer(o.layer); err != nil {
		return nil, err
	}
	return o.layer.Snapshot()
}

func (o *RuntimeMenuOverlay) RestoreScopedLayer(data []byte) error {
	if err := o.validateScopedMenuFonts(); err != nil {
		return err
	}
	restored := &xlate.Layer{}
	if err := restored.Restore(data, o.scoped.registry); err != nil {
		return err
	}
	if err := o.sealScopedMenuLayer(restored); err != nil {
		return err
	}
	o.layer = restored
	return nil
}
