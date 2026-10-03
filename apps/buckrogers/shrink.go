package buckrogers

import (
	"fmt"
	"sync"

	"github.com/wicanr2/dosgolem/xlate"
)

// Buck repo spec 056: a name unit (the "中文(英文)" annotation of spec 036 or
// the player name of spec 038) that does not fit its ECL window is drawn
// smaller before the layout steps down to Chinese only or to English.
//
// A level is a fixed fraction of the normal glyph size.  The layout treats a
// shrunk unit as W_s half units of an opaque placeholder (so the existing
// tokenization, word level, kinsoku and joinOpening run unchanged) and the
// lines are expanded afterwards: each shrunk unit becomes an EclTextLine of
// its own with Shrink set.

// shrinkMetrics are one presenter's numbers of a level (spec 056 §3.2):
// output pixels.  FullCell is the width (and height) of a full cell,
// FullGlyph the square glyph, FullInset the x offset of a full glyph; HalfW and
// HalfH the half glyph; the Y values are the GlyphY of a bottom-aligned stamp.
type shrinkMetrics struct {
	FullCell, FullGlyphW, FullGlyphH, FullInset int
	HalfW, HalfH                                int
	FullGlyphY, HalfGlyphY                      int
}

// shrinkThr is the area threshold n/d of the full-width glyphs of a level (spec
// 057 §3.2): a target pixel is lit when its ink area × d ≥ n × its area.  The
// zero value is 1/2, the rule of spec 056.
type shrinkThr struct{ N, D int }

// get returns the normalized threshold.  A value that is neither zero nor
// 0 < n ≤ d is a table error and panics (TestShrinkLevelTable reports it first).
func (t shrinkThr) get() (n, d int) {
	if t == (shrinkThr{}) {
		return 1, 2
	}
	if t.N <= 0 || t.D <= 0 || t.N > t.D {
		panic(fmt.Sprintf("buckrogers: shrink threshold %d/%d", t.N, t.D))
	}
	return t.N, t.D
}

// shrinkSpec is one level.  FullLP and HalfLP are the logical pixels a full or
// half character takes (a normal one takes 8 and 4).  Thr applies to the
// full-width glyphs only; the half-width ones always use 1/2.
type shrinkSpec struct {
	Level          int
	FullLP, HalfLP int
	Thr            shrinkThr
	X2, X3         shrinkMetrics
}

// shrinkLevels is the level table of spec 056 §3.2, in the order they are
// tried.  It is a variable so that tests can replace it (an empty table
// switches the feature off); tests that do must not run in parallel.
var shrinkLevels = []shrinkSpec{
	{Level: 1, FullLP: 6, HalfLP: 3,
		X2: shrinkMetrics{FullCell: 12, FullGlyphW: 12, FullGlyphH: 12, FullInset: 0, HalfW: 6, HalfH: 12, FullGlyphY: 4, HalfGlyphY: 4},
		X3: shrinkMetrics{FullCell: 18, FullGlyphW: 16, FullGlyphH: 16, FullInset: 1, HalfW: 9, HalfH: 18, FullGlyphY: 7, HalfGlyphY: 6}},
	{Level: 2, FullLP: 4, HalfLP: 2,
		X2: shrinkMetrics{FullCell: 8, FullGlyphW: 8, FullGlyphH: 8, FullInset: 0, HalfW: 4, HalfH: 8, FullGlyphY: 8, HalfGlyphY: 8},
		X3: shrinkMetrics{FullCell: 12, FullGlyphW: 11, FullGlyphH: 11, FullInset: 0, HalfW: 6, HalfH: 12, FullGlyphY: 12, HalfGlyphY: 12}},
}

// shrinkLangLevels are the level tables of the languages that do not use the
// default table (spec 057 §3.2).  A key with an empty table switches the
// shrinking of that language off.  Only tests write it.
var shrinkLangLevels = map[string][]shrinkSpec{
	LangJa: jaShrinkLevels,
	LangKo: koShrinkLevels,
}

// jaShrinkLevels is the Japanese table of spec 057 §3.3: L1 as in spec 056; L2
// keeps the cell (FullLP 4, HalfLP 2) and draws the full-width glyph taller so
// that the dakuten and handakuten of the kana stay apart.
var jaShrinkLevels = []shrinkSpec{
	shrinkLevels[0],
	{Level: 2, FullLP: 4, HalfLP: 2,
		X2: shrinkMetrics{FullCell: 8, FullGlyphW: 8, FullGlyphH: 14, FullInset: 0, HalfW: 4, HalfH: 8, FullGlyphY: 2, HalfGlyphY: 8},
		X3: shrinkMetrics{FullCell: 12, FullGlyphW: 12, FullGlyphH: 16, FullInset: 0, HalfW: 6, HalfH: 12, FullGlyphY: 7, HalfGlyphY: 12}},
}

// koShrinkLevels is the Korean table of spec 058 §3.3: L1 as in spec 056; L2
// takes a cell of 5 logical pixels (the half cell stays 2) and draws the
// full-width glyph 10×12 (2×) or 15×16 (3×) with the threshold 1/3, so that
// no two syllables of the name set come out as the same glyph and the double
// stroke of ㅔ and ㅐ stays.
var koShrinkLevels = []shrinkSpec{
	shrinkLevels[0],
	{Level: 2, FullLP: 5, HalfLP: 2, Thr: shrinkThr{N: 1, D: 3},
		X2: shrinkMetrics{FullCell: 10, FullGlyphW: 10, FullGlyphH: 12, FullInset: 0, HalfW: 4, HalfH: 8, FullGlyphY: 4, HalfGlyphY: 8},
		X3: shrinkMetrics{FullCell: 15, FullGlyphW: 15, FullGlyphH: 16, FullInset: 0, HalfW: 6, HalfH: 12, FullGlyphY: 7, HalfGlyphY: 12}},
}

// shrinkLevelsFor returns the level table of a language: its own table when it
// has one, else the default (spec 057 §3.2).
func shrinkLevelsFor(lang string) []shrinkSpec {
	if t, ok := shrinkLangLevels[lang]; ok {
		return t
	}
	return shrinkLevels
}

// metrics returns the numbers for a presenter scale (2 or 3).
func (s shrinkSpec) metrics(scale int) shrinkMetrics {
	if scale == 3 {
		return s.X3
	}
	return s.X2
}

// shrinkSpecIn finds a level of a table.
func shrinkSpecIn(levels []shrinkSpec, level int) (shrinkSpec, bool) {
	for _, s := range levels {
		if s.Level == level {
			return s, true
		}
	}
	return shrinkSpec{}, false
}

// shrinkPlaceholderBase starts the plane-15 private-use runes that stand for
// the unit n of a call in a shrink layout; they are one half unit wide and
// never reach a font.
const (
	shrinkPlaceholderBase = 0xF0000
	shrinkPlaceholderMax  = 0xF0FFF
)

func isShrinkPlaceholder(r rune) bool {
	return r >= shrinkPlaceholderBase && r <= shrinkPlaceholderMax
}

// shrinkUnitUnits is W_s: the half units a unit takes at the level
// (spec 056 §3.2), the pixel width rounded up to whole half units.
func shrinkUnitUnits(unit []rune, s shrinkSpec) int {
	px := 0
	for _, r := range unit {
		if isHalfwidth(r) {
			px += s.HalfLP
		} else {
			px += s.FullLP
		}
	}
	return (px + halfUnitPx - 1) / halfUnitPx
}

// layoutEclTextLevel is layoutEclTextP with every unit drawn at the given
// shrink level.  Level 0, an unknown level and a text without units are
// layoutEclTextP itself.  The units are sorted and disjoint (the contract of
// layoutEclTextUnits).
func layoutEclTextLevel(prof *LayoutProfile, levels []shrinkSpec, text []rune, units []NameUnit, level int, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	spec, ok := shrinkSpecIn(levels, level)
	if level == 0 || !ok || len(units) == 0 || len(units) > shrinkPlaceholderMax-shrinkPlaceholderBase {
		return layoutEclTextP(prof, text, units, row, col, left, right, bottom)
	}
	t2 := make([]rune, 0, len(text))
	u2 := make([]NameUnit, 0, len(units))
	pos := 0
	for k, u := range units {
		t2 = append(t2, text[pos:u.Start]...)
		start := len(t2)
		for i, n := 0, shrinkUnitUnits(text[u.Start:u.End], spec); i < n; i++ {
			t2 = append(t2, shrinkPlaceholderBase+rune(k))
		}
		u2 = append(u2, NameUnit{start, len(t2)})
		pos = u.End
	}
	t2 = append(t2, text[pos:]...)
	lines, endRow, endCol, fits := layoutEclTextP(prof, t2, u2, row, col, left, right, bottom)
	if !fits {
		return nil, 0, 0, false
	}
	return expandShrinkLines(lines, text, units, level), endRow, endCol, true
}

// expandShrinkLines turns the placeholder runs of laid-out lines back into
// the canonical form of spec 056 §3.5: one EclTextLine per shrunk unit and
// one per run of other text between them.
func expandShrinkLines(lines []EclTextLine, text []rune, units []NameUnit, level int) []EclTextLine {
	var out []EclTextLine
	for _, l := range lines {
		col := int(l.Col)
		var run []rune
		runCol := col
		flush := func() {
			if len(run) > 0 {
				out = append(out, EclTextLine{Row: l.Row, Col: uint8(runCol), Text: run})
				run = nil
			}
		}
		for i := 0; i < len(l.Text); {
			r := l.Text[i]
			if isShrinkPlaceholder(r) {
				j := i
				for j < len(l.Text) && l.Text[j] == r {
					j++
				}
				flush()
				u := units[int(r-shrinkPlaceholderBase)]
				out = append(out, EclTextLine{Row: l.Row, Col: uint8(col), Text: append([]rune(nil), text[u.Start:u.End]...), Shrink: uint8(level)})
				col += j - i
				i = j
				runCol = col
				continue
			}
			if len(run) == 0 {
				runCol = col
			}
			run = append(run, r)
			col += runeUnits(r)
			i++
		}
		flush()
	}
	return out
}

// resampleInk is the ink area of every target pixel of the glyph src (sw×sh,
// rows of (sw+7)/8 bytes, most significant bit first) at tw×th, in the
// integer units of spec 056 §3.3: horizontally a source pixel is tw wide and a
// target pixel sw wide (vertically th and sh), so a target pixel has area sw×sh.
func resampleInk(src []byte, sw, sh, tw, th int) []int {
	sb := (sw + 7) / 8
	areas := make([]int, tw*th)
	overlap := func(a0, a1, b0, b1 int) int {
		lo, hi := max(a0, b0), min(a1, b1)
		if hi > lo {
			return hi - lo
		}
		return 0
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			ink := 0
			for j := 0; j < sh; j++ {
				oy := overlap(y*sh, (y+1)*sh, j*th, (j+1)*th)
				if oy == 0 {
					continue
				}
				for i := 0; i < sw; i++ {
					if src[j*sb+i/8]&(0x80>>uint(i%8)) == 0 {
						continue
					}
					ink += oy * overlap(x*sw, (x+1)*sw, i*tw, (i+1)*tw)
				}
			}
			areas[y*tw+x] = ink
		}
	}
	return areas
}

// resampleGlyph is the area rule of spec 056 §3.3: the glyph src at tw×th.  A
// target pixel is lit when the ink among the source pixels it overlaps covers
// at least half of it (2×ink ≥ sw×sh).  With equal sizes the glyph is copied.
func resampleGlyph(src []byte, sw, sh, tw, th int) []byte {
	return resampleGlyphThr(src, sw, sh, tw, th, 1, 2)
}

// resampleGlyphThr is resampleGlyph with the threshold n/d (spec 057 §3.2): a
// target pixel is lit when its ink area × d ≥ n × its area.
func resampleGlyphThr(src []byte, sw, sh, tw, th, n, d int) []byte {
	tb := (tw + 7) / 8
	out := make([]byte, th*tb)
	for i, ink := range resampleInk(src, sw, sh, tw, th) {
		if d*ink >= n*sw*sh {
			x, y := i%tw, i/tw
			out[y*tb+x/8] |= 0x80 >> uint(x%8)
		}
	}
	return out
}

// shrinkGlyph is resampleGlyph with the rule of spec 056 §3.3 for strokes the
// area rule loses (a dot, an apex): a glyph that has ink and would come out
// blank lights the one target pixel with the most ink (the first in raster
// order among equals), so every character the source draws stays visible.
func shrinkGlyph(src []byte, sw, sh, tw, th int) []byte {
	return shrinkGlyphThr(src, sw, sh, tw, th, 1, 2)
}

// shrinkGlyphThr is shrinkGlyph with the threshold n/d.
func shrinkGlyphThr(src []byte, sw, sh, tw, th, n, d int) []byte {
	out := resampleGlyphThr(src, sw, sh, tw, th, n, d)
	if glyphHasInk(out) || !glyphHasInk(src) {
		return out
	}
	best, at := 0, -1
	for i, ink := range resampleInk(src, sw, sh, tw, th) {
		if ink > best {
			best, at = ink, i
		}
	}
	if at >= 0 {
		out[(at/tw)*((tw+7)/8)+(at%tw)/8] |= 0x80 >> uint((at%tw)%8)
	}
	return out
}

func glyphHasInk(g []byte) bool {
	for _, b := range g {
		if b != 0 {
			return true
		}
	}
	return false
}

// deriveShrinkFont resamples every glyph of src to tw×th with shrinkGlyphThr
// (threshold n/d), so a glyph with ink in src has ink in the result.
func deriveShrinkFont(src *xlate.Font, tw, th int, name string, n, d int) *xlate.Font {
	out := &xlate.Font{W: tw, H: th, Name: name, Glyphs: make(map[rune][]byte, len(src.Glyphs))}
	sb := (src.W + 7) / 8
	for r, g := range src.Glyphs {
		if len(g) != src.H*sb {
			continue
		}
		out.Glyphs[r] = shrinkGlyphThr(g, src.W, src.H, tw, th, n, d)
	}
	return out
}

// shrinkFontSet is the fonts one level draws with at one presenter scale.
// Half is nil when the half fonts of the base failed their ink check.
type shrinkFontSet struct{ Full, Half *xlate.Font }

type shrinkFontKey struct {
	base  *xlate.Font
	spec  shrinkSpec // the whole level: size, threshold and logical widths (spec 057 §3.2)
	scale int
}

var shrinkFontCache = struct {
	sync.Mutex
	m map[shrinkFontKey]*shrinkFontSet
}{m: map[shrinkFontKey]*shrinkFontSet{}}

// shrinkFontsOf derives the fonts of a level for base, the 16×16 font the
// overlay was built with (spec 056 §3.3): full characters from base, half
// characters from the 8×16 half font of spec 039.
func shrinkFontsOf(base *xlate.Font, s shrinkSpec, scale int) *shrinkFontSet {
	key := shrinkFontKey{base, s, scale}
	shrinkFontCache.Lock()
	defer shrinkFontCache.Unlock()
	if f, ok := shrinkFontCache.m[key]; ok {
		return f
	}
	m := s.metrics(scale)
	name := base.Name
	if name == "" {
		name = "buckrogers"
	}
	tn, td := s.Thr.get()
	f := &shrinkFontSet{Full: deriveShrinkFont(base, m.FullGlyphW, m.FullGlyphH, fmt.Sprintf("%s.shrink%d.x%d.full%dx%d.t%dof%d", name, s.Level, scale, m.FullGlyphW, m.FullGlyphH, tn, td), tn, td)}
	if h := halfFontsOf(base); h != nil && h.Err == nil && h.X2 != nil {
		f.Half = deriveShrinkFont(h.X2, m.HalfW, m.HalfH, fmt.Sprintf("%s.shrink%d.x%d.half%dx%d", name, s.Level, scale, m.HalfW, m.HalfH), 1, 2)
	}
	shrinkFontCache.m[key] = f
	return f
}

// missingShrinkRunes lists the runes of a shrunk unit the derived fonts of
// the level lack (blank padding characters never need a glyph).
func missingShrinkRunes(fs *shrinkFontSet, t []rune) []rune {
	var miss []rune
	for _, r := range t {
		if isBlankRune(r) {
			continue
		}
		font := fs.Full
		if isHalfwidth(r) {
			font = fs.Half
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

// shrinkStamps builds the stamps of one shrunk unit whose first half unit is
// the absolute column col of the row at logical y (spec 056 §3.6): segments
// by width class, bottom-aligned glyphs, keys continuing the row's numbering
// from n.
func shrinkStamps(rowKey string, n int, col, y int, unit []rune, s shrinkSpec, scale int, fs *shrinkFontSet, bg, fg [3]uint8) []*xlate.Stamp {
	m := s.metrics(scale)
	var out []*xlate.Stamp
	x := col * halfUnitPx
	for _, seg := range splitSegments(unit, nil) {
		st := &xlate.Stamp{Key: segmentKey(rowKey, n+len(out)), X: x, Y: y, Cells: seg.End - seg.Start, CellH: 8,
			Text: append([]rune(nil), unit[seg.Start:seg.End]...), State: xlate.Shown, BG: bg, FG: fg, GlyphScale: 1}
		if seg.Half {
			st.CellW, st.Font, st.GlyphX, st.GlyphY = s.HalfLP, fs.Half, 0, m.HalfGlyphY
		} else {
			st.CellW, st.Font, st.GlyphX, st.GlyphY = s.FullLP, fs.Full, m.FullInset, m.FullGlyphY
		}
		out = append(out, st)
		x += st.Cells * st.CellW
	}
	return out
}
