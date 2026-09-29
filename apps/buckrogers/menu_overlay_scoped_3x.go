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
	// menuHalfFontName is the spec 039 §3.2 12×24 half font derived from
	// the SHA-verified 16×16 base.
	menuHalfFontName   = menuBaseFontName + ".half12x24"
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

// validateScopedMenuHalfFont checks the spec 039 §3.4 half font of the
// scoped runtime: 12×24, named after the base, one glyph per half-width
// character of the base and byte-equal to the versioned derivation.  It has
// no 22×22 restriction.
func validateScopedMenuHalfFont(base, half *xlate.Font) error {
	if base == nil || half == nil || half.W != 12 || half.H != 24 {
		return fmt.Errorf("buckrogers: scoped menu requires a 12x24 half font")
	}
	expected := DeriveHalfFonts(base)
	if expected.Err != nil {
		return expected.Err
	}
	if half.Name == "" || half.Name != expected.X3.Name {
		return fmt.Errorf("buckrogers: scoped menu half font name differs")
	}
	if len(expected.X3.Glyphs) != len(half.Glyphs) {
		return fmt.Errorf("buckrogers: scoped menu half font coverage differs")
	}
	for r, glyph := range expected.X3.Glyphs {
		if got, ok := half.Glyphs[r]; !ok || len(got) != 48 || !bytes.Equal(got, glyph) {
			return fmt.Errorf("buckrogers: half glyph U+%04X differs from derivation", r)
		}
	}
	return nil
}

// BuildScopedThreeXMenuOverlay is the opt-in, whole-batch 3x menu builder.
// The existing 16px BuildMenuOverlay API and all its callers are unchanged;
// half-width segments (spec 039) use the 12×24 half font of the same base.
func BuildScopedThreeXMenuOverlay(entries []MenuOverlayEntry, base, derived *xlate.Font, palette [256][3]uint8, scale int) (*MenuOverlay, error) {
	if err := validateScopedMenuFontPair(base, derived); err != nil {
		return nil, err
	}
	if h := halfFontsOf(base); h.Err != nil {
		return nil, h.Err
	} else if err := validateScopedMenuHalfFont(base, h.X3); err != nil {
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
	// Spec 039 §3.4: switch fonts per segment.  Full segments take the
	// 22-point derived font; half segments keep the 12×24 half font that
	// BuildMenuOverlay derived from the same base pointer.
	half := halfFontsOf(base).X3
	byEvent := map[string][]*xlate.Stamp{}
	for _, stamp := range overlay.Layer.Stamps {
		if stamp.CellW == halfUnitPx {
			if half == nil || stamp.Font != half {
				return nil, fmt.Errorf("buckrogers: %s half segment lacks the 12x24 half font", stamp.Key)
			}
		} else {
			stamp.Font, stamp.GlyphX, stamp.GlyphY, stamp.GlyphScale = derived, 1, 1, 1
		}
		k := rowKeyOf(stamp.Key)
		byEvent[k] = append(byEvent[k], stamp)
	}
	for i := range overlay.Events {
		ink, err := menuInkRectAll(byEvent[overlay.Events[i].EventKey], scale)
		if err != nil {
			return nil, fmt.Errorf("buckrogers: %s: %w", overlay.Events[i].EventKey, err)
		}
		clear := overlay.Events[i].ClearRect
		if ink.Width <= 0 || ink.Height <= 0 || ink.X < clear.X || ink.Y < clear.Y ||
			ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
			return nil, fmt.Errorf("buckrogers: %s 22px ink escapes safe rectangle", overlay.Events[i].EventKey)
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
	halfHash    [sha256.Size]byte
	half        *xlate.Font
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
	// Spec 039 §3.4: the scoped runtime only accepts 3×.
	if scale != 3 {
		return nil, fmt.Errorf("buckrogers: scoped menu scale must be 3")
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
	halves := halfFontsOf(base)
	if halves.Err != nil {
		return nil, halves.Err
	}
	half := halves.X3
	if err := validateScopedMenuHalfFont(base, half); err != nil {
		return nil, err
	}
	halfHash, err := presentation.FontFingerprint(half)
	if err != nil {
		return nil, err
	}
	overlay, err := NewRuntimeMenuOverlay(rects, base, scale)
	if err != nil {
		return nil, err
	}
	overlay.scoped = &scopedMenuFonts{catalog: catalog, source: source, sourceHash: expected,
		baseHash: baseHash, derivedHash: derivedHash, derived: derived, halfHash: halfHash, half: half,
		registry: map[string]*xlate.Font{menuBaseFontName: base, menuThreeXFontName: derived, menuHalfFontName: half}}
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
	if o == nil || o.scoped == nil || o.font == nil || o.scoped.derived == nil || o.scoped.half == nil ||
		o.font.Name != menuBaseFontName || o.scoped.derived.Name != menuThreeXFontName || o.scoped.half.Name != menuHalfFontName ||
		len(o.scoped.registry) != 3 ||
		o.scoped.registry[menuBaseFontName] != o.font || o.scoped.registry[menuThreeXFontName] != o.scoped.derived ||
		o.scoped.registry[menuHalfFontName] != o.scoped.half || halfFontsOf(o.font).X3 != o.scoped.half ||
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
	halfHash, err := presentation.FontFingerprint(o.scoped.half)
	if err != nil || halfHash != o.scoped.halfHash {
		return fmt.Errorf("buckrogers: scoped menu half font fingerprint changed")
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
		// Spec 039 §3.3: key checks use the original row key.
		key := rowKeyOf(stamp.Key)
		if !scopedRaceMenuKeys[key] && !scopedThreeXMenuKeys[key] {
			return fmt.Errorf("buckrogers: scoped menu has unknown active key %q", stamp.Key)
		}
		// Full segments use the 22-point font on the ten scoped keys (the
		// base otherwise); half segments always use the half font.
		var want *xlate.Font
		switch stamp.CellW {
		case 8:
			want = o.font
			if o.scale == 3 && scopedThreeXMenuKeys[key] {
				want = o.scoped.derived
			}
		case halfUnitPx:
			want = o.scoped.half
		default:
			return fmt.Errorf("buckrogers: scoped menu stamp %q has cell width %d", stamp.Key, stamp.CellW)
		}
		if stamp.Font != want {
			return fmt.Errorf("buckrogers: scoped menu stamp %q uses wrong font", stamp.Key)
		}
	}
	return presentation.WithSealedOrderedLayers(menuSealedGroupKey, 1, 1,
		[]presentation.ActiveLayerSlot{{Name: "menu", Z: 1, Layer: layer}}, o.scoped.registry,
		func(group *presentation.SealedLayerGroup) error {
			for name, want := range map[string][sha256.Size]byte{menuBaseFontName: o.scoped.baseHash, menuThreeXFontName: o.scoped.derivedHash, menuHalfFontName: o.scoped.halfHash} {
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
	o.groups.rebuild(restored)
	return nil
}
