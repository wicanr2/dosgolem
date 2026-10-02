package buckrogers

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// withShrinkLevels replaces the level table of spec 056 for one test and
// restores it afterwards.  Tests that use it must not run in parallel.
func withShrinkLevels(t *testing.T, levels ...shrinkSpec) {
	t.Helper()
	old := shrinkLevels
	shrinkLevels = levels
	t.Cleanup(func() { shrinkLevels = old })
}

// noShrink switches the shrink feature off: the layout steps down exactly as
// before spec 056.
func noShrink(t *testing.T) {
	t.Helper()
	withShrinkLevels(t)
}

// ---- spec 056 §5.1: levels and glyph derivation ----

// The level table is the table of spec 056 §3.2, number by number, and its
// L0 column is the normal glyph geometry of the code.
func TestShrinkLevelTable(t *testing.T) {
	want := []shrinkSpec{
		{Level: 1, FullLP: 6, HalfLP: 3,
			X2: shrinkMetrics{12, 12, 0, 6, 12, 4, 4}, X3: shrinkMetrics{18, 16, 1, 9, 18, 7, 6}},
		{Level: 2, FullLP: 4, HalfLP: 2,
			X2: shrinkMetrics{8, 8, 0, 4, 8, 8, 8}, X3: shrinkMetrics{12, 11, 0, 6, 12, 12, 12}},
	}
	if len(shrinkLevels) != len(want) {
		t.Fatalf("%d levels", len(shrinkLevels))
	}
	for i, w := range want {
		if shrinkLevels[i] != w {
			t.Errorf("level %d: %+v want %+v", w.Level, shrinkLevels[i], w)
		}
	}
	// L0: the glyph geometry the code draws with today.
	hf := DeriveHalfFonts(halfTestFont("l0", "A"))
	l0 := map[int]shrinkMetrics{
		2: {FullCell: 16, FullGlyph: 16, FullInset: manualGlyphOffset(2), HalfW: hf.X2.W, HalfH: hf.X2.H},
		3: {FullCell: 24, FullGlyph: 22, FullInset: manualGlyphOffset(3), HalfW: hf.X3.W, HalfH: hf.X3.H},
	}
	if l0[2].FullInset != 0 || l0[3].FullInset != 1 || l0[2].HalfW != 8 || l0[2].HalfH != 16 || l0[3].HalfW != 12 || l0[3].HalfH != 24 {
		t.Fatalf("L0 geometry changed: %+v", l0)
	}
	// Every number of a level follows from its LP sizes and the L0 geometry:
	// the cell is LP × scale, the inset ⌊(cell − glyph)/2⌋, the half glyph is
	// half a cell wide and a cell high, and both are bottom aligned.
	for _, s := range shrinkLevels {
		for scale, m := range map[int]shrinkMetrics{2: s.X2, 3: s.X3} {
			if m.FullCell != s.FullLP*scale || m.HalfW != s.HalfLP*scale || m.HalfH != m.FullCell {
				t.Errorf("L%d %d×: cell %d half %d×%d vs LP %d/%d", s.Level, scale, m.FullCell, m.HalfW, m.HalfH, s.FullLP, s.HalfLP)
			}
			if m.FullInset != (m.FullCell-m.FullGlyph)/2 {
				t.Errorf("L%d %d×: inset %d", s.Level, scale, m.FullInset)
			}
			r := 8 * scale
			if m.FullGlyphY != r-m.FullGlyph-l0[scale].FullInset || m.HalfGlyphY != r-m.HalfH {
				t.Errorf("L%d %d×: GlyphY %d/%d", s.Level, scale, m.FullGlyphY, m.HalfGlyphY)
			}
			// The glyph and its stamp cell stay inside the row.
			if m.FullGlyphY+m.FullGlyph > r || m.HalfGlyphY+m.HalfH > r {
				t.Errorf("L%d %d×: glyph leaves the row", s.Level, scale)
			}
		}
	}
}

// W_s: the pixel width rounded up to whole half units.
func TestShrinkUnitUnits(t *testing.T) {
	l1, l2 := shrinkLevels[0], shrinkLevels[1]
	cases := []struct {
		unit   string
		w1, w2 int
	}{
		{"巴克(BUCK)", 8, 5},          // W 10: 30 px → 8, 20 px → 5
		{"塞萊絲特(CELESTE)", 13, 9},    // W 17: 51 px → 13, 34 px → 9
		{"甲", 2, 1},                 // W 2: 6 px = 1.5 units → 2, 4 px → 1
		{"A", 1, 1},                 // W 1: 3 px → 1, 2 px → 1
		{"ABCD", 3, 2},              // W 4: 12 px → 3, 8 px → 2
		{"卡頓•特必安(CARLTON)", 15, 10}, // F 5, H 10 (• is half width): 60 px → 15, 40 px → 10
	}
	for _, c := range cases {
		u := []rune(c.unit)
		if g := shrinkUnitUnits(u, l1); g != c.w1 {
			t.Errorf("%q L1: %d want %d", c.unit, g, c.w1)
		}
		if g := shrinkUnitUnits(u, l2); g != c.w2 {
			t.Errorf("%q L2: %d want %d", c.unit, g, c.w2)
		}
		// Never wider than the normal unit, never narrower than the pixels.
		if shrinkUnitUnits(u, l1) > textUnits(u) || shrinkUnitUnits(u, l1)*halfUnitPx < 0 {
			t.Errorf("%q: shrunk wider than normal", c.unit)
		}
	}
}

// The area rule on a 4×4 source at 3×3, worked by hand: the pixel (1,0) has
// exactly half of its area inked and is lit (≥), so a strict > fails here.
func TestResampleGlyphKnownAnswer(t *testing.T) {
	src := []byte{0xC0, 0xC0, 0x30, 0x30} // 1100 1100 0011 0011
	got := resampleGlyph(src, 4, 4, 3, 3)
	want := []byte{0xC0, 0xE0, 0x60} // 110 111 011
	if string(got) != string(want) {
		t.Fatalf("% x want % x", got, want)
	}
	// Equal sizes copy the glyph.
	g16 := make([]byte, 32)
	for i := range g16 {
		g16[i] = byte(i*37 + 11)
	}
	if string(resampleGlyph(g16, 16, 16, 16, 16)) != string(g16) {
		t.Fatal("identity")
	}
	// Deterministic, and a glyph never grows ink out of nothing.
	a, b := resampleGlyph(g16, 16, 16, 12, 12), resampleGlyph(g16, 16, 16, 12, 12)
	if string(a) != string(b) || len(a) != 12*2 {
		t.Fatal("not deterministic")
	}
	if glyphHasInk(resampleGlyph(make([]byte, 32), 16, 16, 8, 8)) {
		t.Fatal("blank glyph got ink")
	}
}

// A glyph the area rule would blank keeps its pixel with the most ink (spec 056
// §3.3), so every character the source draws stays visible; blank glyphs stay
// blank.  The overlay still reports a rune the source lacks as missing.
func TestDeriveShrinkFontKeepsThinStrokes(t *testing.T) {
	src := &xlate.Font{W: 16, H: 16, Name: "t", Glyphs: map[rune][]byte{}}
	one := make([]byte, 32)
	one[2*3] = 0x20 // the pixel (2,3): a quarter of the target pixel (1,1) at 8×8
	src.Glyphs['點'] = one
	src.Glyphs['滿'] = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	src.Glyphs[' '] = make([]byte, 32)
	// Two single pixels with the same (small) ink in different target pixels: the
	// first in raster order is lit, and only it.
	pair := make([]byte, 32)
	pair[2*3] = 0x20 // (2,3) -> target (1,1)
	pair[2*9] = 0x08 // (4,9) -> target (2,4)
	src.Glyphs['·'] = pair
	d := deriveShrinkFont(src, 8, 8, "t.shrink")
	if g := d.Glyphs['點']; len(g) != 8 || g[1] != 0x40 || glyphInkCount(g) != 1 {
		t.Errorf("single pixel: % x", g)
	}
	if g, ok := d.Glyphs['滿']; !ok || len(g) != 8 || g[0] != 0xff {
		t.Errorf("full glyph %v %v", g, ok)
	}
	if g, ok := d.Glyphs[' ']; !ok || glyphHasInk(g) {
		t.Errorf("blank glyph %v %v", g, ok)
	}
	if g := d.Glyphs['·']; len(g) != 8 || g[1] != 0x40 || glyphInkCount(g) != 1 {
		t.Errorf("tie: % x", g)
	}
	if d.W != 8 || d.H != 8 || d.Name != "t.shrink" {
		t.Errorf("font %+v", d)
	}
	// A glyph the area rule keeps is not touched by the fallback.
	if string(shrinkGlyph(src.Glyphs['滿'], 16, 16, 8, 8)) != string(resampleGlyph(src.Glyphs['滿'], 16, 16, 8, 8)) {
		t.Error("fallback changed a glyph that has ink")
	}
	// A rune the source lacks is missing in the derived font: the overlay
	// reports it and draws nothing.
	base := halfTestFont("miss", "AB中")
	o, _ := NewEclTextOverlay(base, 2)
	withShrinkLevels(t, shrinkLevels[1]) // L2: 8×8 full
	page := &EclTextPage{Left: 0, Top: 0, Right: 9, Bottom: 0, Lines: []EclTextLine{{Row: 0, Col: 0, Text: []rune("缺A"), Shrink: 2}}}
	if miss := o.Sync([]*EclTextPage{page}, 1, [256][3]uint8{}); string(miss) != "缺" {
		t.Fatalf("missing %q", string(miss))
	}
	if o.Active() {
		t.Fatal("drew with a missing glyph")
	}
}

func glyphInkCount(g []byte) (n int) {
	for _, b := range g {
		for ; b != 0; b &= b - 1 {
			n++
		}
	}
	return
}

// ---- spec 056 §5.2: layout ----

func shrinkLine(l EclTextLine) string {
	return string(l.Text) + "@" + itoa(int(l.Row)) + "." + itoa(int(l.Col)) + "/" + itoa(int(l.Shrink))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

func shrinkDump(ls []EclTextLine) string {
	s := ""
	for _, l := range ls {
		s += "[" + shrinkLine(l) + "]"
	}
	return s
}

// A unit that does not fit at the normal size fits at L1, then at L2; the
// lines are in the canonical form of §3.5 and the cursor advances by W_s.
func TestLayoutEclTextLevelSingleUnit(t *testing.T) {
	names := testNames(t)
	a := names.Annotate("一二三四五巴克說。", "ecl.t", NameCaseUpper, NameTierAll)
	if string(a.Text) != "一二三四五巴克(BUCK)說。" || len(a.Units) != 1 {
		t.Fatalf("fixture %q %+v", string(a.Text), a.Units)
	}
	// 10 + unit (10 | L1 8 | L2 5) + 4 = 24 | 22 | 19 units.
	cases := []struct {
		right  uint8
		level  int
		want   string
		endCol uint8
	}{
		{23, 0, "[一二三四五巴克(BUCK)說。@0.0/0]", 24},
		{21, 1, "[一二三四五@0.0/0][巴克(BUCK)@0.10/1][說。@0.18/0]", 22},
		{18, 2, "[一二三四五@0.0/0][巴克(BUCK)@0.10/2][說。@0.15/0]", 19},
	}
	for _, c := range cases {
		ls, _, ec, ok := layoutEclTextLevel(nil, a.Text, a.Units, c.level, 0, 0, 0, c.right, 0)
		if !ok || shrinkDump(ls) != c.want || ec != c.endCol {
			t.Errorf("right %d level %d: %v %s end %d", c.right, c.level, ok, shrinkDump(ls), ec)
		}
	}
	// The levels do not fit where the next one does.
	for _, c := range []struct {
		right uint8
		level int
	}{{21, 0}, {18, 0}, {18, 1}, {17, 2}} {
		if _, _, _, ok := layoutEclTextLevel(nil, a.Text, a.Units, c.level, 0, 0, 0, c.right, 0); ok {
			t.Errorf("right %d level %d fits", c.right, c.level)
		}
	}
	// Level 0, an unknown level and no units are layoutEclTextP itself.
	want, wr, wc, wok := layoutEclTextP(nil, a.Text, a.Units, 0, 0, 0, 23, 0)
	got, gr, gc, gok := layoutEclTextLevel(nil, a.Text, a.Units, 0, 0, 0, 0, 23, 0)
	if shrinkDump(got) != shrinkDump(want) || gr != wr || gc != wc || gok != wok {
		t.Error("level 0 differs from layoutEclTextP")
	}
	got, _, _, _ = layoutEclTextLevel(nil, a.Text, a.Units, 7, 0, 0, 0, 23, 0)
	if shrinkDump(got) != shrinkDump(want) {
		t.Error("unknown level is not level 0")
	}
}

// A shrunk unit is an unbreakable token: it moves to the next row whole, it is
// never cut at an English space, and the closing punctuation that sticks to it
// counts in the width of its token.
func TestLayoutEclTextLevelAtomicUnit(t *testing.T) {
	names := testNames(t)
	// The unit moves whole: row 0 holds 10, the unit (L1: 8) does not fit in 4.
	a := names.Annotate("一二三四五巴克說。", "ecl.t", NameCaseUpper, NameTierAll)
	ls, er, ec, ok := layoutEclTextLevel(nil, a.Text, a.Units, 1, 0, 0, 0, 13, 1)
	if !ok || shrinkDump(ls) != "[一二三四五@0.0/0][巴克(BUCK)@1.0/1][說。@1.8/0]" || er != 1 || ec != 12 {
		t.Errorf("wrap: %v %s end %d,%d", ok, shrinkDump(ls), er, ec)
	}
	// A unit with spaces wider than the line: L0 breaks it at the spaces (spec
	// 036), L1 (W_s 25) fails at width 24 and L2 (17) fits.
	w := names.Annotate("亞歷山大•威廉到了。", "logbook.1", NameCaseMixed, NameTierAll)
	if _, _, _, ok := layoutEclTextLevel(nil, w.Text, w.Units, 0, 0, 0, 0, 23, 1); !ok {
		t.Fatal("L0 control: the wide unit breaks at its space")
	}
	if _, _, _, ok := layoutEclTextLevel(nil, w.Text, w.Units, 1, 0, 0, 0, 23, 1); ok {
		t.Error("L1 broke a unit at a space")
	}
	if ls, _, _, ok := layoutEclTextLevel(nil, w.Text, w.Units, 2, 0, 0, 0, 23, 1); !ok || len(ls) != 2 || ls[0].Shrink != 2 || ls[1].Shrink != 0 {
		t.Errorf("L2 wide unit: %v %s", ok, shrinkDump(ls))
	}
	// Closing punctuation sticks to the unit: 巴克(BUCK)，is one token of
	// 8 + 2 units at L1, so it needs 10 units on the row after 一二三.
	c := names.Annotate("一二三巴克，", "ecl.t", NameCaseUpper, NameTierAll)
	if ls, _, ec, ok := layoutEclTextLevel(nil, c.Text, c.Units, 1, 0, 0, 0, 15, 0); !ok || shrinkDump(ls) != "[一二三@0.0/0][巴克(BUCK)@0.6/1][，@0.14/0]" || ec != 16 {
		t.Errorf("closing: %v %s end %d", ok, shrinkDump(ls), ec)
	}
	if _, _, _, ok := layoutEclTextLevel(nil, c.Text, c.Units, 1, 0, 0, 0, 14, 0); ok {
		t.Error("the comma left its unit")
	}
}

// Two units of one call share the level.
func TestLayoutEclTextLevelTwoUnits(t *testing.T) {
	names := testNames(t)
	a := names.Annotate("巴克與威瑪說", "ecl.t", NameCaseUpper, NameTierAll)
	if len(a.Units) != 2 {
		t.Fatalf("units %+v", a.Units)
	}
	// L0 10+2+11+2 = 25; L1 8+2+9+2 = 21; L2 5+2+6+2 = 15.
	ls, _, ec, ok := layoutEclTextLevel(nil, a.Text, a.Units, 1, 0, 0, 0, 21, 0)
	if !ok || shrinkDump(ls) != "[巴克(BUCK)@0.0/1][與@0.8/0][威瑪(WILMA)@0.10/1][說@0.19/0]" || ec != 21 {
		t.Errorf("two units: %v %s end %d", ok, shrinkDump(ls), ec)
	}
}

// Korean, the word-level profile: the unit and the particle glued to it are
// one token whose width is W_s plus the particle; per level the word-level
// layout fits exactly when the character-level one does (spec 043 §3.4).
func TestLayoutEclTextLevelKoWord(t *testing.T) {
	unit := []rune("셀레스트(CELESTE)")
	text := append(append([]rune{}, unit...), []rune("가 좋다")...)
	units := []NameUnit{{0, len(unit)}}
	// L0 17+2+5 = 24; L1 13+2+5 = 20; L2 9+2+5 = 16.
	for _, c := range []struct {
		right uint8
		level int
		want  string
	}{
		{19, 1, "[셀레스트(CELESTE)@0.0/1][가 좋다@0.13/0]"},
		{15, 2, "[셀레스트(CELESTE)@0.0/2][가 좋다@0.9/0]"},
	} {
		ls, _, _, ok := layoutEclTextLevel(layoutKo, text, units, c.level, 0, 0, 0, c.right, 0)
		if !ok || shrinkDump(ls) != c.want {
			t.Errorf("right %d L%d: %v %s", c.right, c.level, ok, shrinkDump(ls))
		}
	}
	for _, level := range []int{0, 1, 2} {
		for right := uint8(8); right < 30; right++ {
			_, _, _, wok := layoutEclTextLevel(layoutKo, text, units, level, 0, 0, 0, right, 1)
			_, _, _, cok := layoutEclTextLevel(layoutKoChars, text, units, level, 0, 0, 0, right, 1)
			if wok != cok {
				t.Errorf("L%d right %d: word level fits %v, character level %v", level, right, wok, cok)
			}
		}
	}
}

// ---- spec 056 §5.2: the player name (spec 038 §3.3) ----

func playerFixture(zh, en string) []AnnotatedText {
	full := []rune(zh + "(" + en + ")")
	cn := []rune(zh)
	return []AnnotatedText{
		{Tier: NameTierAll, Text: full, Units: []NameUnit{{0, len(full)}}},
		{Tier: NameTierNone, Text: cn, Units: []NameUnit{{0, len(cn)}}},
	}
}

func TestLayoutPlayerNameShrinks(t *testing.T) {
	player := playerFixture("塞萊絲特", "CELESTE") // 17 units; L1 13; L2 9; Chinese only 8
	orig := []byte("CELESTE")
	cases := []struct {
		label        string
		space        bool
		right        uint8
		kind, shrink int
		spaced, fits bool
		endCol       uint8
	}{
		{"fits at the normal size", false, 16, playerFull, 0, false, true, 17},
		{"L1", false, 13, playerFull, 1, false, true, 13},
		{"L2", false, 9, playerFull, 2, false, true, 9},
		{"Chinese only", false, 7, playerChineseOnly, 0, false, true, 8},
		{"English", false, 6, playerEnglish, 0, false, true, 7},
		{"nothing fits", false, 5, playerEnglish, 0, false, false, 0},
		// With the call-start space (spec 045 §3.4) the space is never the
		// reason to shrink less: spaced L1 comes before unspaced L0.
		{"spaced L0", true, 17, playerFull, 0, true, true, 18},
		{"spaced L1 before unspaced L0", true, 16, playerFull, 1, true, true, 14},
		{"spaced L2", true, 9, playerFull, 2, true, true, 10},
		{"spaced Chinese only after the full format", true, 8, playerChineseOnly, 0, true, true, 9},
		{"unspaced Chinese only when the space does not fit either", true, 7, playerChineseOnly, 0, false, true, 8},
	}
	for _, c := range cases {
		got := layoutPlayerName(nil, player, orig, c.space, 0, 0, 0, c.right, 0)
		if got.kind != c.kind || got.shrink != c.shrink || got.spaced != c.spaced || got.fits != c.fits || (c.fits && got.endCol != c.endCol) {
			t.Errorf("%s: kind %d shrink %d spaced %v fits %v end %d (%s)", c.label, got.kind, got.shrink, got.spaced, got.fits, got.endCol, shrinkDump(got.lines))
		}
	}
	// Without shrink levels the order is the one of spec 038 (control).
	noShrink(t)
	got := layoutPlayerName(nil, player, orig, false, 0, 0, 0, 13, 0)
	if got.kind != playerChineseOnly || got.shrink != 0 {
		t.Errorf("control: %+v", got)
	}
	// The input slices are not modified.
	if string(player[0].Text) != "塞萊絲特(CELESTE)" || player[0].Units[0] != (NameUnit{0, 13}) {
		t.Error("input modified")
	}
}

// Pure function: the same call twice gives the same lines.
func TestLayoutPlayerNameShrinkPure(t *testing.T) {
	player := playerFixture("塞萊絲特", "CELESTE")
	a := layoutPlayerName(nil, player, []byte("CELESTE"), true, 0, 2, 0, 13, 0)
	b := layoutPlayerName(nil, player, []byte("CELESTE"), true, 0, 2, 0, 13, 0)
	if shrinkDump(a.lines) != shrinkDump(b.lines) || a.endCol != b.endCol || a.kind != b.kind || a.shrink != b.shrink {
		t.Fatal("not pure")
	}
}

// Through the watcher: a long name is drawn small instead of Chinese only, the
// counters are the ones of spec 056 §3.7 and the step-downs do not count.
func TestEclPlayerNameShrinkWatcher(t *testing.T) {
	party, _ := ReadPartySnapshot(partyMem(2, recFlavius, recCeleste), testDS)
	ctx := eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})
	narrow := func(s string, col uint8) EclTextEntry {
		e := eclPlayerEntry(s, true, col, 17, ctx)
		e.Left, e.Right, e.Top, e.Bottom = 1, 9, 17, 17 // 18 half units
		return e
	}
	// 弗拉維烏斯(FLAVIUS) = 19 units: L1 is 15.
	w := eclPlayerWatcher(t)
	w.ObserveEntry(narrow("FLAVIUS", 1))
	p := w.Page()
	if p == nil || len(p.Lines) != 1 || p.Lines[0].Shrink != 1 || string(p.Lines[0].Text) != "弗拉維烏斯(FLAVIUS)" || p.Lines[0].Col != 2 || p.endCol != 17 {
		t.Fatalf("shrunk: %+v", p)
	}
	if w.Stats.PlayerNames != 1 || w.Stats.PlayerShrunk != [2]int{1, 0} || w.Stats.PlayerNameChineseOnly != 0 || w.Stats.NameShrunk != [2]int{} {
		t.Fatalf("counters %+v", w.Stats)
	}
	// 9 + 9 syllables: L1 21 > 18, L2 14 fits.
	w = eclPlayerWatcher(t)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "塞萊絲特塞萊絲特塞"}, nil))
	w.ObserveEntry(narrow("CELESTE", 1))
	if p := w.Page(); p == nil || p.Lines[0].Shrink != 2 || w.Stats.PlayerShrunk != [2]int{0, 1} || w.Stats.PlayerNames != 1 {
		t.Fatalf("L2: %+v %+v", p, w.Stats)
	}
	// Too long for L2 as well: the step-down of spec 038, no shrink counted.
	w = eclPlayerWatcher(t)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "塞萊絲特塞萊絲特塞萊絲特塞萊"}, nil)) // 14 syllables: L2 is 19
	w.ObserveEntry(narrow("CELESTE", 1))
	if w.Stats.PlayerShrunk != [2]int{} || w.Stats.PlayerNameEnglish != 1 || w.Stats.PlayerNameChineseOnly != 0 {
		t.Fatalf("step-down: %+v", w.Stats)
	}
}

// Korean with the call-start space: a name that fits only with the space left
// out is shrunk with the space instead (spec 056 §3.4); SpaceDropped is not
// counted then.
func TestEclPlayerNameSpaceKoShrink(t *testing.T) {
	// 갑판 4 units, space 1, 셀레스트(CELESTE) 17 (L1 13, L2 9), 셀레스트 8.
	cases := []struct {
		label    string
		right    uint8
		want     string
		shrink   uint8
		counters [2]int
		dropped  int
		cnOnly   int
	}{
		{"fits", 11, "갑판 셀레스트(CELESTE)", 0, [2]int{}, 0, 0},
		{"L1 with the space", 10, "갑판 셀레스트(CELESTE)", 1, [2]int{1, 0}, 0, 0},
		{"L2 with the space instead of Chinese only", 8, "갑판 셀레스트(CELESTE)", 2, [2]int{0, 1}, 0, 0},
		{"Chinese only without the space", 6, "갑판셀레스트", 0, [2]int{}, 1, 1},
	}
	for _, c := range cases {
		w, p := koPlayerCall(t, layoutKo, "CELESTE", "셀레스트", c.right)
		if p == nil {
			t.Errorf("%s: no page %+v", c.label, w.Stats)
			continue
		}
		var shrink uint8
		for _, l := range p.Lines {
			shrink |= l.Shrink
		}
		if eclRow(p, 17) != c.want || shrink != c.shrink || w.Stats.PlayerShrunk != c.counters || w.Stats.SpaceDropped != c.dropped || w.Stats.PlayerNameChineseOnly != c.cnOnly {
			t.Errorf("%s: %q shrink %d %+v", c.label, eclRow(p, 17), shrink, w.Stats)
		}
	}
}

// Through the watcher: an NPC annotation steps down only after the shrink
// levels have been tried, and only the first tier is shrunk.
func TestEclNameShrinkWatcher(t *testing.T) {
	names := testNames(t)
	entry := func(s string, clear bool, col, row uint8) EclTextEntry {
		e := eclEntry(s, clear, col, row)
		e.Left, e.Right, e.Top, e.Bottom = 1, 10, 17, 18 // 20 units × 2 rows
		return e
	}
	// "巴克說巴克好巴克。" with three annotations: L0 does not fit 2×20 (the
	// units do not break), L1 does (8+2+8+2+8+2 over two rows).
	w := NewEclTextWatcher(eclFixture(t, "BUCK TWICE", "巴克說巴克好巴克。"))
	w.SetNames(names)
	w.ObserveEntry(entry("BUCK TWICE", true, 1, 17))
	p := w.Page()
	if p == nil || w.Stats.NameShrunk != [2]int{1, 0} || w.Stats.NameFirstOnly != 0 || w.Stats.NameUnannotated != 0 || w.Stats.Overflows != 0 {
		t.Fatalf("shrunk: %+v %+v", p, w.Stats)
	}
	shrunk := 0
	for _, l := range p.Lines {
		if l.Shrink != 0 {
			shrunk++
			if l.Shrink != 1 || string(l.Text) != "巴克(BUCK)" {
				t.Errorf("unit line %+v", l)
			}
		}
	}
	if shrunk != 3 {
		t.Errorf("%d shrunk units: %s", shrunk, shrinkDump(p.Lines))
	}
	// With the feature off the same call is the first-only step-down of spec 036.
	withShrinkLevels(t)
	w = NewEclTextWatcher(eclFixture(t, "BUCK TWICE", "巴克說巴克好巴克。"))
	w.SetNames(names)
	w.ObserveEntry(entry("BUCK TWICE", true, 1, 17))
	if w.Stats.NameFirstOnly != 1 || w.Stats.NameShrunk != [2]int{} {
		t.Fatalf("control: %+v", w.Stats)
	}
}

// Spec 054 §3.2 and spec 056 §3.5: the reading a shrunk name leaves is the
// Hangul before the English, and the mark that follows takes it.
func TestEclLastReadingShrunkUnit(t *testing.T) {
	party := inlineParty(t, recMarion)
	w := inlineEclWatcher(t, LangKo, layoutKo, nil, " IS HIT", "이(가) 서서히")
	e := eclPlayerEntry("MARION", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e.Right, e.Top, e.Bottom = 6, 17, 18 // 12 units: 마리온(MARION) is 14, L1 is 11
	w.ObserveEntry(e)
	eclSpaceRet(w)
	p := w.Page()
	if p == nil || w.Stats.PlayerShrunk != [2]int{1, 0} || w.Stats.PlayerNameChineseOnly != 0 || p.Lines[0].Shrink != 1 || p.lastReading != '온' {
		t.Fatalf("fixture: %+v %+v", w.Stats, p)
	}
	n := eclEntry(" IS HIT", false, 4, 17)
	n.Right, n.Top, n.Bottom = 6, 17, 18
	w.ObserveEntry(n)
	var all []rune
	for _, l := range w.Page().Lines {
		all = append(all, l.Text...)
	}
	if got := string(all); !strings.Contains(got, "마리온(MARION)이") || strings.Contains(got, "(가)") || w.Stats.MarkersResolved != 1 {
		t.Errorf("%q %+v", got, w.Stats)
	}
}

// ---- spec 056 §5.4: drawing ----

func glyphInk(f *xlate.Font, r rune, x, y int) bool {
	g, ok := f.Glyphs[r]
	if !ok || x < 0 || y < 0 || x >= f.W || y >= f.H {
		return false
	}
	return g[y*((f.W+7)/8)+x/8]&(0x80>>uint(x%8)) != 0
}

type shrinkDraw struct {
	out   []byte
	scale int
	w     int
}

func (d shrinkDraw) px(x, y int) [3]uint8 {
	i := 4 * (y*d.w + x)
	return [3]uint8{d.out[i], d.out[i+1], d.out[i+2]}
}

// shrinkPalette: background of the page (index 0), foreground (10), and the
// unit under the stamps (7) all differ, so padding and fill are visible.
func shrinkPalette() [256][3]uint8 {
	var pal [256][3]uint8
	pal[0] = [3]uint8{1, 2, 3}
	pal[7] = [3]uint8{9, 9, 9}
	pal[10] = [3]uint8{255, 255, 255}
	return pal
}

func shrinkRender(t *testing.T, base *xlate.Font, scale int, p *EclTextPage) (*EclTextOverlay, shrinkDraw) {
	t.Helper()
	o, err := NewEclTextOverlay(base, scale)
	if err != nil {
		t.Fatal(err)
	}
	pal := shrinkPalette()
	if miss := o.Sync([]*EclTextPage{p}, 1, pal); len(miss) != 0 {
		t.Fatalf("missing %q", string(miss))
	}
	indexed := make([]byte, 320*200)
	for i := range indexed {
		indexed[i] = 7
	}
	out, miss := o.Draw(indexed, pal)
	if len(miss) != 0 {
		t.Fatalf("draw missing %q", string(miss))
	}
	return o, shrinkDraw{out: out, scale: scale, w: 320 * scale}
}

func shrinkPage(lines ...EclTextLine) *EclTextPage {
	return &EclTextPage{Left: 0, Top: 0, Right: 5, Bottom: 0, TopCol: 0, Background: 0, Foreground: 10, Lines: lines}
}

// The unit "中AB" at L1 takes 6+3+3 = 12 px = 3 units: its stamps come after
// the normal segments, every pixel is the derived glyph at the position of
// §3.2 (or the background), pixels outside the unit are those of the page
// without it, and the pixels right of the last glyph are background.
func TestShrunkUnitPixels(t *testing.T) {
	base := halfTestFont("shrinkpx", "中文AB")
	for _, scale := range liveScales {
		withShrinkLevels(t, shrinkLevels[0], shrinkLevels[1])
		for _, sp := range shrinkLevels {
			unit := []rune("中AB")
			ws := shrinkUnitUnits(unit, sp) // L1 3, L2 2
			plain := shrinkPage(EclTextLine{Row: 0, Col: 0, Text: []rune("文")}, EclTextLine{Row: 0, Col: 6, Text: []rune("文")})
			with := shrinkPage(EclTextLine{Row: 0, Col: 0, Text: []rune("文")}, EclTextLine{Row: 0, Col: 2, Text: unit, Shrink: uint8(sp.Level)},
				EclTextLine{Row: 0, Col: 6, Text: []rune("文")})
			_, pd := shrinkRender(t, base, scale, plain)
			o, wd := shrinkRender(t, base, scale, with)
			// Stamps: the normal segments first, the unit's after them, with
			// the keys of the row continued.
			var first = -1
			for i, s := range o.layer.Stamps {
				if rowKeyOf(s.Key) != "ecl.0.row.0" {
					t.Fatalf("key %q", s.Key)
				}
				if s.CellW == sp.FullLP || s.CellW == sp.HalfLP {
					if first < 0 {
						first = i
					}
				} else if first >= 0 {
					t.Fatalf("%d× L%d: a normal stamp %q after the shrunk ones", scale, sp.Level, s.Key)
				}
			}
			if first < 0 || len(o.cols) != len(o.layer.Stamps) {
				t.Fatalf("no shrunk stamps or cols out of step: %d %d", len(o.layer.Stamps), len(o.cols))
			}
			if k := o.ActiveKeys(); len(k) != 1 || k[0] != "ecl.0.row.0" {
				t.Fatalf("ActiveKeys %v", k)
			}
			m := sp.metrics(scale)
			fs := shrinkFontsOf(base, sp, scale)
			x0, x1 := 2*halfUnitPx*scale, (2+ws)*halfUnitPx*scale
			pal := shrinkPalette()
			bg, fg := pal[0], pal[10]
			for y := 0; y < 8*scale; y++ {
				for x := 0; x < 320*scale; x++ {
					if x < x0 || x >= x1 {
						// Outside the unit: the page without it.
						if wd.px(x, y) != pd.px(x, y) {
							t.Fatalf("%d× L%d: pixel (%d,%d) outside the unit changed", scale, sp.Level, x, y)
						}
						continue
					}
					want := bg
					px := x - x0 // inside the unit, from its left edge
					switch {
					case px < sp.FullLP*scale: // 中: full cell
						if glyphInk(fs.Full, '中', px-m.FullInset, y-m.FullGlyphY) {
							want = fg
						}
					case px < (sp.FullLP+sp.HalfLP)*scale: // A
						if glyphInk(fs.Half, 'A', px-sp.FullLP*scale, y-m.HalfGlyphY) {
							want = fg
						}
					case px < (sp.FullLP+2*sp.HalfLP)*scale: // B
						if glyphInk(fs.Half, 'B', px-(sp.FullLP+sp.HalfLP)*scale, y-m.HalfGlyphY) {
							want = fg
						}
					}
					if wd.px(x, y) != want {
						t.Fatalf("%d× L%d: unit pixel (%d,%d) = %v want %v", scale, sp.Level, px, y, wd.px(x, y), want)
					}
				}
			}
			// Bottom alignment: the lowest row of the glyph box is the same
			// row as that of a normal full glyph (2×: 15, 3×: 22) and the
			// half box ends at the bottom of the row (2×: 15, 3×: 23).
			l0 := map[int]int{2: 15, 3: 22}[scale]
			if m.FullGlyphY+m.FullGlyph-1 != l0 || m.HalfGlyphY+m.HalfH-1 != 8*scale-1 {
				t.Errorf("%d× L%d: box bottom %d / %d", scale, sp.Level, m.FullGlyphY+m.FullGlyph-1, m.HalfGlyphY+m.HalfH-1)
			}
			// The shrunk glyph pixels: ink counts of the derived glyphs.
			count := 0
			for y := 0; y < 8*scale; y++ {
				for x := x0; x < x1; x++ {
					if wd.px(x, y) == fg {
						count++
					}
				}
			}
			ink := 0
			for _, r := range unit {
				f := fs.Half
				if !isHalfwidth(r) {
					f = fs.Full
				}
				for _, b := range f.Glyphs[r] {
					for ; b != 0; b &= b - 1 {
						ink++
					}
				}
			}
			if count != ink {
				t.Errorf("%d× L%d: %d foreground pixels, the derived glyphs have %d", scale, sp.Level, count, ink)
			}
		}
	}
}

// The unit takes part in the last-writer-wins flow of eclRowText: a later line
// that overwrites one of its slots removes the whole unit; a unit outside
// [start, right] is not drawn and writes no slot; a unit takes the slots of a
// full-width character it cuts.
func TestShrunkUnitSlots(t *testing.T) {
	unit := EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 1} // 3 units: 2..4
	rowOf := func(ls ...EclTextLine) string {
		p := shrinkPage(ls...)
		txt, units := eclRowTextShrink(p, 0, 0)
		return string(txt) + "|" + itoa(len(units))
	}
	pad := func(n int) string { return strings.Repeat("　", n) }
	// The unit alone: its slots are blank padding, the unit is kept.
	if got := rowOf(unit); got != pad(6)+"|1" {
		t.Errorf("alone: %q", got)
	}
	// A later line over one of its slots: the unit goes, the text stays.
	if got := rowOf(unit, EclTextLine{Row: 0, Col: 4, Text: []rune("文")}); got != pad(2)+"文"+pad(3)+"|0" {
		t.Errorf("overwritten by text: %q", got)
	}
	// A later unit over an earlier one removes the earlier one.
	if got := rowOf(unit, EclTextLine{Row: 0, Col: 3, Text: []rune("中AB"), Shrink: 1}); !strings.HasSuffix(got, "|1") {
		t.Errorf("unit over unit: %q", got)
	}
	// An earlier full-width character cut by a unit: the half that stays is
	// cleared (all blank), the unit is kept.
	if got := rowOf(EclTextLine{Row: 0, Col: 1, Text: []rune("文")}, unit); got != pad(6)+"|1" {
		t.Errorf("cut: %q", got)
	}
	if _, units := eclRowTextShrink(shrinkPage(EclTextLine{Row: 0, Col: 1, Text: []rune("文")}, unit), 0, 0); len(units) != 1 {
		t.Error("a unit over a full-width character was dropped")
	}
	// Out of range: left of the row start (top row of a window that starts at
	// cell 1), and right of the right border (cell 5 ends at unit 11).
	p := shrinkPage(EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 1})
	if _, units := eclRowTextShrink(p, 0, 1); len(units) != 1 { // start cell 1 = unit 2
		t.Error("a unit at the start is dropped")
	}
	p = shrinkPage(EclTextLine{Row: 0, Col: 1, Text: []rune("中AB"), Shrink: 1})
	if txt, units := eclRowTextShrink(p, 0, 1); len(units) != 0 || len([]rune(string(txt))) != 5 {
		t.Errorf("left of the start: %d units, text %q", len(units), string(txt))
	}
	p = shrinkPage(EclTextLine{Row: 0, Col: 10, Text: []rune("中AB"), Shrink: 1}) // slots 10..12, the row ends at 11
	if txt, units := eclRowTextShrink(p, 0, 0); len(units) != 0 || string(txt) != pad(6) {
		t.Errorf("right of the border: %d units, text %q", len(units), string(txt))
	}
	// A level that is not in the table draws nothing (and is not a panic).
	p = shrinkPage(EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 9})
	if _, units := eclRowTextShrink(p, 0, 0); len(units) != 0 {
		t.Error("unknown level drawn")
	}
}

// Gone rows and the colors follow the page like any row: the shrunk stamps go
// with their row, and Frame recolors them with the others.
func TestShrunkUnitLifetimeAndColors(t *testing.T) {
	base := halfTestFont("shrinklife", "中文AB")
	for _, scale := range liveScales {
		p := shrinkPage(EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 1})
		o, _ := shrinkRender(t, base, scale, p)
		if !o.Active() {
			t.Fatal("not drawn")
		}
		pal := shrinkPalette()
		pal[0] = [3]uint8{50, 60, 70}
		o.Frame(pal)
		for _, s := range o.layer.Stamps {
			if s.BG != pal[0] || s.FG != pal[10] {
				t.Fatalf("%d×: stamp %q colors %v %v", scale, s.Key, s.BG, s.FG)
			}
		}
		gone := shrinkPage(EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 1})
		gone.Gone = 1 << 0
		o2, _ := NewEclTextOverlay(base, scale)
		if miss := o2.Sync([]*EclTextPage{gone}, 1, shrinkPalette()); len(miss) != 0 || o2.Active() {
			t.Fatalf("%d×: a Gone row is still drawn", scale)
		}
	}
}

// ---- spec 056 §5.2: the rest of the NPC cases ----

// The opening quotation mark of ja stays at the normal size, belongs to the
// plain text before the unit, and travels with the unit when the row breaks.
func TestLayoutEclTextLevelJaOpening(t *testing.T) {
	names := testNames(t)
	a := names.Annotate("一二三「巴克」説", "ecl.t", NameCaseUpper, NameTierAll)
	// 6 + 「 2 + unit (10 | L1 8) + 」 2 + 説 2 = 22 | 20.
	ls, _, ec, ok := layoutEclTextLevel(layoutJa, a.Text, a.Units, 1, 0, 0, 0, 19, 0)
	if !ok || shrinkDump(ls) != "[一二三「@0.0/0][巴克(BUCK)@0.8/1][」説@0.16/0]" || ec != 20 {
		t.Errorf("one row: %v %s end %d", ok, shrinkDump(ls), ec)
	}
	// Row of 14: 一二三「 is 8, the unit (8) does not fit in the 6 left.  「 may not end a
	// row, so it moves down with the unit.
	ls, er, ec, ok := layoutEclTextLevel(layoutJa, a.Text, a.Units, 1, 0, 0, 0, 13, 1)
	if !ok || shrinkDump(ls) != "[一二三@0.0/0][「@1.0/0][巴克(BUCK)@1.2/1][」説@1.10/0]" || er != 1 || ec != 14 {
		t.Errorf("break: %v %s end %d,%d", ok, shrinkDump(ls), er, ec)
	}
	// With only one row (bottom 0) the unit has nowhere to go: the level fails.
	if _, _, _, ok := layoutEclTextLevel(layoutJa, a.Text, a.Units, 1, 0, 0, 0, 13, 0); ok {
		t.Error("a second row was used with bottom 0")
	}
}

// The annotation steps down to the first occurrence only after the shrink
// levels, and the first-only text is not shrunk.
func TestEclNameFirstOnlyIsNotShrunk(t *testing.T) {
	names := testNames(t)
	text := "巴克巴克巴克巴克巴克巴克巴克" // seven times: all-L2 is 7×5 = 35 units, first-only 10 + 6×4 = 34
	run := func(right uint8) *EclTextWatcher {
		w := NewEclTextWatcher(eclFixture(t, "SEVEN", text))
		w.SetNames(names)
		e := eclEntry("SEVEN", true, 1, 17)
		e.Left, e.Right, e.Top, e.Bottom = 1, right, 17, 17
		w.ObserveEntry(e)
		return w
	}
	w := run(17) // 34 units
	p := w.Page()
	if p == nil || w.Stats.NameFirstOnly != 1 || w.Stats.NameShrunk != [2]int{} || w.Stats.NameUnannotated != 0 {
		t.Fatalf("first only: %+v", w.Stats)
	}
	for _, l := range p.Lines {
		if l.Shrink != 0 {
			t.Errorf("the first-only layout was shrunk: %s", shrinkDump(p.Lines))
		}
	}
	if got := eclRow(p, 17); got != "巴克(BUCK)巴克巴克巴克巴克巴克巴克" {
		t.Errorf("row %q", got)
	}
	// 35 units: all annotations at L2 fit.
	if w = run(18); w.Stats.NameShrunk != [2]int{0, 1} || w.Stats.NameFirstOnly != 0 {
		t.Errorf("all L2: %+v", w.Stats)
	}
	// 32 units: the first-only text would fit at L1 (8 + 6×4), but it is never
	// shrunk: the annotation is dropped instead.
	if w = run(16); w.Stats.NameUnannotated != 1 || w.Stats.NameShrunk != [2]int{} || w.Stats.NameFirstOnly != 0 {
		t.Errorf("none: %+v", w.Stats)
	}
}

// zh-TW: the space after 甲板 (spec 048) is kept and the annotation is shrunk
// instead of being dropped; the deck check runs on the layout the tiers chose.
func TestZhDeckSpaceShrinks(t *testing.T) {
	pairs := []string{"DECK ", "甲板", "5 B", "5巴克說"}
	steps := []fsStep{{"DECK ", true, 1}, {"5 B", false, 3}}
	// 甲板 4 + space 1 + 5 1 + unit (10 | L1 8) + 說 2 = 18 | 16 units.
	for _, c := range []struct {
		right   uint8
		want    string
		shrunk  [2]int
		unannot int
	}{
		{9, "甲板 5巴克(BUCK)說", [2]int{}, 0},
		{8, "甲板 5巴克(BUCK)說", [2]int{1, 0}, 0},
	} {
		w := zdRun(t, zdOpts{lang: LangZhTW, names: true, right: c.right, bottom: 17}, pairs, steps)
		p := w.Page()
		if p == nil || eclRow(p, 17) != c.want || w.Stats.NameShrunk != c.shrunk || w.Stats.NameUnannotated != c.unannot || w.Stats.SpaceDropped != 0 {
			t.Errorf("right %d: %v %+v", c.right, p, w.Stats)
		}
	}
	// With the levels off the same call drops the annotation and keeps the space.
	withShrinkLevels(t)
	w := zdRun(t, zdOpts{lang: LangZhTW, names: true, right: 8, bottom: 17}, pairs, steps)
	if p := w.Page(); p == nil || eclRow(p, 17) != "甲板 5巴克說" || w.Stats.NameUnannotated != 1 {
		t.Errorf("control: %v %+v", w.Page(), w.Stats)
	}
}

// ko: the call-start space (spec 046) comes with the shrunk unit; the
// annotation is not dropped to keep the normal size.
func TestEclNameSpaceKoShrink(t *testing.T) {
	g, err := LoadNameGlossary([]byte(pre056GlossaryHead+"BUCK\t\t벅\tfull\tp0\tprinted:x\tn\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	run := func(right uint8) *EclTextWatcher {
		w := NewEclTextWatcher(eclFixtureLang(t, LangKo, "GO ", "가자", " B", "벅이 왔다"))
		w.SetNames(g)
		w.SetLayout(layoutKo)
		for _, s := range []struct {
			o     string
			clear bool
			col   uint8
		}{{"GO ", true, 1}, {" B", false, 3}} {
			e := eclEntry(s.o, s.clear, s.col, 17)
			e.Left, e.Right, e.Top, e.Bottom = 1, right, 17, 17
			w.ObserveEntry(e)
			eclSpaceRet(w)
		}
		return w
	}
	// 가자 4, space 1, 벅(BUCK) 8 (L1 6, L2 4), 이 왔다 7: 20 | 18 | 16 units; without annotation 14.
	for _, c := range []struct {
		right   uint8
		shrunk  [2]int
		unannot int
	}{{10, [2]int{}, 0}, {9, [2]int{1, 0}, 0}, {8, [2]int{0, 1}, 0}} {
		w := run(c.right)
		p := w.Page()
		if p == nil || eclRow(p, 17) != "가자 벅(BUCK)이 왔다" || w.Stats.NameShrunk != c.shrunk || w.Stats.NameUnannotated != c.unannot || w.Stats.SpaceDropped != 0 {
			t.Errorf("right %d: %q %+v", c.right, eclRowOf(p), w.Stats)
		}
	}
}

func eclRowOf(p *EclTextPage) string {
	if p == nil {
		return "<nil>"
	}
	return eclRow(p, 17)
}

// An NPC annotation that ends a call leaves its reading as well (spec 054
// §3.2): the reading is read from the lines of the call in (Row, Col) order, the
// shrunk unit included.
func TestEclLastReadingShrunkNPC(t *testing.T) {
	g, err := LoadNameGlossary([]byte(pre056GlossaryHead+"BUCK\t\t벅\tfull\tp0\tprinted:x\tn\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	w := NewEclTextWatcher(eclFixtureLang(t, LangKo, "HERE B", "여기 벅", " IS", "이(가) 서서히"))
	w.SetNames(g)
	w.SetLayout(layoutKo)
	// 6 units wide: 벅(BUCK) is 8 at the normal size, 6 at L1.
	e := eclEntry("HERE B", true, 1, 17)
	e.Left, e.Right, e.Top, e.Bottom = 1, 3, 17, 20
	w.ObserveEntry(e)
	eclSpaceRet(w)
	p := w.Page()
	if p == nil || w.Stats.NameShrunk != [2]int{1, 0} || p.lastReading != '벅' {
		t.Fatalf("fixture: %+v %+v", w.Stats, p)
	}
	n := eclEntry(" IS", false, 3, 18)
	n.Left, n.Right, n.Top, n.Bottom = 1, 3, 17, 20
	w.ObserveEntry(n)
	var all []rune
	for _, l := range w.Page().Lines {
		all = append(all, l.Text...)
	}
	if got := string(all); !strings.Contains(got, "벅(BUCK)이") || strings.Contains(got, "(가)") || w.Stats.MarkersResolved != 1 {
		t.Errorf("%q %+v", got, w.Stats)
	}
}

// The keys of a row's stamps are unique: the shrunk stamps continue the row's
// numbering after the normal segments (spec 056 §3.6).
func TestShrunkUnitKeysUnique(t *testing.T) {
	base := halfTestFont("keys", "AB中文 ")
	for _, scale := range []int{2, 3} {
		p := shrinkPage(
			EclTextLine{Row: 0, Col: 0, Text: []rune("中A")},
			EclTextLine{Row: 0, Col: 3, Text: []rune("中AB"), Shrink: 1},
			EclTextLine{Row: 0, Col: 6, Text: []rune("文 "), Shrink: 2},
		)
		o, _ := shrinkRender(t, base, scale, p)
		seen := map[string]bool{}
		for _, s := range o.layer.Stamps {
			if seen[s.Key] {
				t.Fatalf("%d×: duplicate stamp key %q", scale, s.Key)
			}
			seen[s.Key] = true
		}
		if len(o.layer.Stamps) < 4 {
			t.Fatalf("%d×: %d stamps", scale, len(o.layer.Stamps))
		}
	}
}

// A space inside a shrunk unit (a long English name) needs no glyph of its own,
// as at the normal size.
func TestShrunkUnitBlankNeedsNoGlyph(t *testing.T) {
	base := halfTestFont("blank", "AB中") // no U+0020 glyph
	o, err := NewEclTextOverlay(base, 2)
	if err != nil {
		t.Fatal(err)
	}
	p := shrinkPage(EclTextLine{Row: 0, Col: 0, Text: []rune("中A B"), Shrink: 1})
	if miss := o.Sync([]*EclTextPage{p}, 1, shrinkPalette()); len(miss) != 0 || !o.Active() {
		t.Fatalf("missing %q active %v", string(miss), o.Active())
	}
}

// The derived fonts come from the sources of spec 056 §3.3: full characters
// from the 16×16 base font, half characters from the 8×16 half font of
// spec 039 at 2× (the 12×24 one is not a source), for both presenters.
func TestShrinkFontsSources(t *testing.T) {
	base := halfTestFont("src", "AB.中文")
	half := halfFontsOf(base)
	if half.X2.W != 8 || half.X2.H != 16 {
		t.Fatalf("half source %d×%d", half.X2.W, half.X2.H)
	}
	for _, sp := range shrinkLevels {
		for _, scale := range []int{2, 3} {
			m := sp.metrics(scale)
			fs := shrinkFontsOf(base, sp, scale)
			for _, r := range "AB." {
				want := shrinkGlyph(half.X2.Glyphs[r], 8, 16, m.HalfW, m.HalfH)
				if got := fs.Half.Glyphs[r]; string(got) != string(want) {
					t.Errorf("L%d %d× half %q: % x want % x", sp.Level, scale, r, got, want)
				}
			}
			for _, r := range "中文" {
				want := shrinkGlyph(base.Glyphs[r], 16, 16, m.FullGlyph, m.FullGlyph)
				if got := fs.Full.Glyphs[r]; string(got) != string(want) {
					t.Errorf("L%d %d× full %q: % x want % x", sp.Level, scale, r, got, want)
				}
			}
			if fs.Half.W != m.HalfW || fs.Half.H != m.HalfH || fs.Full.W != m.FullGlyph || fs.Full.H != m.FullGlyph {
				t.Errorf("L%d %d×: sizes half %d×%d full %d×%d", sp.Level, scale, fs.Half.W, fs.Half.H, fs.Full.W, fs.Full.H)
			}
		}
	}
}

// Counterpart of TestEclPlayerNameSpaceNotGluedLikeParticle with the levels on:
// 갑판 4 + space 1 + 도(DOE) 2+5 = 12 units; L1 is 11 (6 px-units of the unit),
// L2 is 9.
func TestEclPlayerNameSpaceNotGluedShrink(t *testing.T) {
	for _, c := range []struct {
		right        uint8
		want         string
		shrink       uint8
		shrunk       [2]int
		chineseOnly  int
		spaceDropped int
	}{
		{7, "갑판 도(DOE)", 0, [2]int{}, 0, 0},     // 14 units
		{6, "갑판 도(DOE)", 0, [2]int{}, 0, 0},     // 12 units: fits at the normal size
		{5, "갑판 도(DOE)", 2, [2]int{0, 1}, 0, 0}, // 10 units: L1 11 does not fit, L2 9 does
		{4, "갑판 도", 0, [2]int{}, 1, 0},          // 8 units: Chinese only with its space (7)
		{3, "갑판도", 0, [2]int{}, 1, 1},           // 6 units: 7 does not fit, the space goes
	} {
		w, p := koPlayerCall(t, layoutKo, "DOE", "도", c.right)
		var shrink uint8
		if p != nil {
			for _, l := range p.Lines {
				shrink |= l.Shrink
			}
		}
		if p == nil || eclRow(p, 17) != c.want || shrink != c.shrink || w.Stats.PlayerShrunk != c.shrunk || w.Stats.PlayerNameChineseOnly != c.chineseOnly || w.Stats.SpaceDropped != c.spaceDropped {
			t.Errorf("right=%d：%q shrink %d %+v，應為 %q", c.right, eclRowOf(p), shrink, w.Stats, c.want)
		}
	}
}

// Counterpart of TestZhDeckSpacePlayerNameStays: a player name after 甲板 gets
// no space (spec 048 is for numbers) and is drawn small before it steps down.
// 甲板 4 + 塞萊絲特(CELESTE): 17 units at the normal size, 13 at L1, 9 at L2;
// Chinese only is 8.
func TestZhDeckSpacePlayerNameShrinks(t *testing.T) {
	party, _ := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "CELESTE"}), testDS)
	ctx := eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})
	for _, c := range []struct {
		right  uint8
		want   string
		shrunk [2]int
		cnOnly int
	}{
		{11, "甲板塞萊絲特(CELESTE)", [2]int{}, 0},    // 22 units: 21 fits
		{9, "甲板塞萊絲特(CELESTE)", [2]int{1, 0}, 0}, // 18 units: L1 17 fits
		{8, "甲板塞萊絲特(CELESTE)", [2]int{0, 1}, 0}, // 16 units: L1 17 does not, L2 13 does
		{6, "甲板塞萊絲特", [2]int{}, 1},              // 12 units: Chinese only (12)
	} {
		w := NewEclTextWatcher(eclFixtureLang(t, LangZhTW, "DECK ", "甲板"))
		w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "塞萊絲特"}, nil))
		e := eclEntry("DECK ", true, 1, 17)
		e.Right, e.Bottom = c.right, 17
		w.ObserveEntry(e)
		eclSpaceRet(w)
		e2 := eclPlayerEntry("CELESTE", false, 6, 17, ctx)
		e2.Right, e2.Bottom = c.right, 17
		w.ObserveEntry(e2)
		got := "<nil>"
		if p := w.Page(); p != nil {
			got = rowText(p, 17)
		}
		if got != c.want || w.Stats.PlayerShrunk != c.shrunk || w.Stats.PlayerNameChineseOnly != c.cnOnly || w.Stats.SpaceDropped != 0 {
			t.Errorf("right=%d：%q %+v，應為 %q", c.right, got, w.Stats, c.want)
		}
	}
}
