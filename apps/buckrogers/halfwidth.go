package buckrogers

import (
	"fmt"
	"strings"
	"sync"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 039 (Buck repo): Latin letters, digits, spaces and the interpunct
// inside overlay text are half width.  Widths are counted in half units of
// 4 logical pixels: U+0020–U+007E and U+2022 take one unit, every other
// character two.  A row is drawn as segments of one width class (and one
// foreground); half segments use CellW=4 and the half font derived below,
// full segments keep CellW=8 and the family's own font.  xlate is not
// changed: every segment is an ordinary stamp.

// halfUnitPx is one half unit in logical pixels.
const halfUnitPx = 4

// ideographicSpace pads full cells (spec 039 §3.1); drawGlyphs fills only
// the background for it, as for U+0020.
const ideographicSpace = '　'

// isHalfwidth reports whether r takes one half unit (spec 039 §3.1).
func isHalfwidth(r rune) bool { return r >= 0x20 && r <= 0x7E || r == 0x2022 }

// runeUnits is the width of r in half units.
func runeUnits(r rune) int {
	if isHalfwidth(r) {
		return 1
	}
	return 2
}

// textUnits is the width of t in half units.
func textUnits(t []rune) int {
	n := 0
	for _, r := range t {
		n += runeUnits(r)
	}
	return n
}

// stringUnits is textUnits for a string.
func stringUnits(s string) int {
	n := 0
	for _, r := range s {
		n += runeUnits(r)
	}
	return n
}

// cellsForUnits is the number of 8×8 cells u half units cover.
func cellsForUnits(u int) int { return (u + 1) / 2 }

// isBlankRune reports the two padding characters that need no glyph.
func isBlankRune(r rune) bool { return r == ' ' || r == ideographicSpace }

// appendPadding appends spec 039 §3.1 padding to t, whose content ends at
// half unit from (relative to a cell-aligned row start), up to unit to: one
// U+0020 to reach an even unit, U+3000 per full cell, and one U+0020 more
// when to is odd.
func appendPadding(t []rune, from, to int) []rune {
	u := from
	if u < to && u%2 == 1 {
		t = append(t, ' ')
		u++
	}
	for u+2 <= to {
		t = append(t, ideographicSpace)
		u += 2
	}
	if u < to {
		t = append(t, ' ')
	}
	return t
}

// padUnits returns a copy of t padded to target half units.
func padUnits(t []rune, target int) []rune {
	out := append([]rune(nil), t...)
	return appendPadding(out, textUnits(t), target)
}

// fitUnits returns the longest prefix of t whose width is at most max
// units; a full-width character that does not fit moves whole.
func fitUnits(t []rune, max int) int {
	u := 0
	for i, r := range t {
		u += runeUnits(r)
		if u > max {
			return i
		}
	}
	return len(t)
}

// textSegment is a run of runes [Start, End) of one width class and one
// color; Unit is its offset in half units from the row start.
type textSegment struct {
	Start, End int
	Unit       int
	Half       bool
}

// splitSegments cuts t where the width class or, when color is not nil,
// color(i) changes (spec 039 §3.3).
func splitSegments(t []rune, color func(i int) int) []textSegment {
	var out []textSegment
	u := 0
	for i, r := range t {
		h := isHalfwidth(r)
		if i == 0 || h != out[len(out)-1].Half || color != nil && color(i) != color(i-1) {
			out = append(out, textSegment{Start: i, End: i, Unit: u, Half: h})
		}
		out[len(out)-1].End = i + 1
		u += runeUnits(r)
	}
	return out
}

// segmentKey is the stamp key of segment n of a row (spec 039 §3.3).
func segmentKey(row string, n int) string { return fmt.Sprintf("%s#%d", row, n) }

// rowKeyOf strips the "#<segment>" suffix of segmentKey.
func rowKeyOf(key string) string {
	if i := strings.LastIndexByte(key, '#'); i >= 0 {
		return key[:i]
	}
	return key
}

// dedupRowKeys lists the row keys of stamps once each, in first-seen order.
func dedupRowKeys(stamps []*xlate.Stamp) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range stamps {
		k := rowKeyOf(s.Key)
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// segmentFonts is how a family draws its two width classes.
type segmentFonts struct {
	Full           *xlate.Font
	FullX, FullY   int
	FullGlyphScale int
	Half           *xlate.Font // nil when the derivation failed
}

// missingRunes lists the runes of t the fonts cannot draw.  U+0020 and
// U+3000 never need a glyph (spec 039 §3.1).
func (f segmentFonts) missingRunes(t []rune) []rune {
	var miss []rune
	for _, r := range t {
		if isBlankRune(r) {
			continue
		}
		font := f.Full
		if isHalfwidth(r) {
			font = f.Half
		}
		if font == nil {
			miss = append(miss, r)
			continue
		}
		if g, ok := font.Glyphs[r]; !ok || len(g) != font.H*((font.W+7)/8) {
			miss = append(miss, r)
		}
	}
	return miss
}

// segmentStamps builds one Shown stamp per segment of a row whose first
// cell is at logical (x, y).  color (may be nil) splits segments by
// foreground; bgfg gives each segment's colors from its first rune index.
func (f segmentFonts) segmentStamps(key string, x, y int, t []rune, color func(i int) int, bgfg func(i int) ([3]uint8, [3]uint8)) []*xlate.Stamp {
	segs := splitSegments(t, color)
	out := make([]*xlate.Stamp, 0, len(segs))
	for n, s := range segs {
		bg, fg := bgfg(s.Start)
		st := &xlate.Stamp{Key: segmentKey(key, n), X: x + s.Unit*halfUnitPx, Y: y, Cells: s.End - s.Start,
			CellH: 8, Text: append([]rune(nil), t[s.Start:s.End]...), State: xlate.Shown, BG: bg, FG: fg}
		if s.Half {
			st.CellW, st.Font, st.GlyphScale = halfUnitPx, f.Half, 1
		} else {
			st.CellW, st.Font, st.GlyphX, st.GlyphY, st.GlyphScale = 8, f.Full, f.FullX, f.FullY, f.FullGlyphScale
		}
		out = append(out, st)
	}
	return out
}

// HalfFonts are the spec 039 §3.2 half fonts derived from one base font:
// 8×16 for 2× and 12×24 for 3×.  Err is set when the base font failed the
// ink check; then both fonts are nil and every half character is missing.
type HalfFonts struct {
	X2, X3 *xlate.Font
	Err    error
}

// For returns the half font used at scale (2 or 3).
func (h *HalfFonts) For(scale int) *xlate.Font {
	if h == nil {
		return nil
	}
	if scale == 3 {
		return h.X3
	}
	return h.X2
}

// halfInkLeft and halfInkRight are the base columns a half glyph must stay
// inside (spec 039 §3.2).
const (
	halfInkLeft  = 4
	halfInkRight = 11
)

// DeriveHalfFonts derives the half fonts from a 16×16 base font that has
// not been through spec 031's glyph replacement.  Every half-width glyph's
// ink must lie in columns 4–11 of the base.
func DeriveHalfFonts(base *xlate.Font) *HalfFonts {
	if base == nil || base.W != 16 || base.H != 16 {
		return &HalfFonts{Err: fmt.Errorf("buckrogers: 半形衍生需要 16×16 底字型")}
	}
	name := base.Name
	if name == "" {
		name = "buckrogers"
	}
	x2 := &xlate.Font{W: 8, H: 16, Name: name + ".half8x16", Glyphs: map[rune][]byte{}}
	x3 := &xlate.Font{W: 12, H: 24, Name: name + ".half12x24", Glyphs: map[rune][]byte{}}
	for r, g := range base.Glyphs {
		if !isHalfwidth(r) {
			continue
		}
		if len(g) != 32 {
			return &HalfFonts{Err: fmt.Errorf("buckrogers: 半形衍生 U+%04X 字模長度 %d", r, len(g))}
		}
		half := make([]byte, 16)
		for y := 0; y < 16; y++ {
			row := uint16(g[2*y])<<8 | uint16(g[2*y+1])
			if row&^uint16(0xFF<<(15-halfInkRight)) != 0 {
				return &HalfFonts{Err: fmt.Errorf("buckrogers: 半形字 U+%04X 墨跡超出第 %d–%d 欄", r, halfInkLeft, halfInkRight)}
			}
			half[y] = byte(row >> (15 - halfInkRight))
		}
		x2.Glyphs[r] = half
		big := make([]byte, 24*2)
		for y := 0; y < 24; y++ {
			src := half[2*y/3]
			for x := 0; x < 12; x++ {
				if src&(0x80>>uint(2*x/3)) != 0 {
					big[2*y+x/8] |= 0x80 >> uint(x%8)
				}
			}
		}
		x3.Glyphs[r] = big
	}
	return &HalfFonts{X2: x2, X3: x3}
}

// halfFontCache keeps one derivation per base font pointer, so every
// presenter of a session shares the same half-font pointers and names
// (spec 039 §3.3).
var halfFontCache = struct {
	sync.Mutex
	m map[*xlate.Font]*HalfFonts
}{m: map[*xlate.Font]*HalfFonts{}}

// halfFontsOf returns the session's half fonts for base, deriving them on
// first use.  The base font must not be modified afterwards.
func halfFontsOf(base *xlate.Font) *HalfFonts {
	halfFontCache.Lock()
	defer halfFontCache.Unlock()
	if h, ok := halfFontCache.m[base]; ok {
		return h
	}
	h := DeriveHalfFonts(base)
	halfFontCache.m[base] = h
	return h
}

// HalfFontsOf is the shared half-font derivation for callers outside the
// package (receipt runners, font registries).
func HalfFontsOf(base *xlate.Font) *HalfFonts { return halfFontsOf(base) }

// familyFonts builds the segmentFonts of a family drawing with full at
// scale, whose full cells use the offset of manualGlyphOffset.
func familyFonts(base, full *xlate.Font, scale int) segmentFonts {
	off := manualGlyphOffset(scale)
	return segmentFonts{Full: full, FullX: off, FullY: off, FullGlyphScale: 1, Half: halfFontsOf(base).For(scale)}
}
