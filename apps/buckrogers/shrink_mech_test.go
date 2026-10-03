package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Spec 057 §5: the mechanism tests (tables per language, the page language,
// the derivation threshold, the cache key).

// A rectangular target and the threshold, worked by hand: the 4×4 source
// 1100/1100/0011/0011 at 3×2.  Target pixel (1,0) has exactly half of its area
// inked: it is lit at 1/2 and not at 3/5.
func TestResampleGlyphRectAndThr(t *testing.T) {
	src := []byte{0xC0, 0xC0, 0x30, 0x30}
	if got, want := resampleGlyph(src, 4, 4, 3, 2), []byte{0xC0, 0x60}; string(got) != string(want) {
		t.Errorf("1/2: % x want % x", got, want)
	}
	if got, want := resampleGlyphThr(src, 4, 4, 3, 2, 3, 5), []byte{0x80, 0x20}; string(got) != string(want) {
		t.Errorf("3/5: % x want % x", got, want)
	}
	if string(resampleGlyphThr(src, 4, 4, 3, 2, 1, 3)) != string(resampleGlyph(src, 4, 4, 3, 2)) {
		t.Error("1/3 differs from 1/2 where no pixel has a share in between")
	}
	// The fallback of spec 056 §3.3 on a rectangle: a single pixel lights the
	// pixel with the most ink, the first among equals.
	one := make([]byte, 4)
	one[1] = 0x40 // (1,1)
	if got := shrinkGlyphThr(one, 4, 4, 3, 2, 1, 2); glyphInkCount(got) != 1 {
		t.Errorf("single pixel on a 3×2 target: % x", got)
	}
}

// The derived fonts follow the whole level: a different threshold gives another
// font, a zero threshold is 1/2, the half glyphs do not depend on the threshold,
// an invalid threshold panics, and the font name carries size and threshold.
func TestShrinkThrAndCacheKey(t *testing.T) {
	var ascii strings.Builder
	for r := rune(0x20); r < 0x7f; r++ {
		ascii.WriteRune(r)
	}
	base := halfTestFont("thr", ascii.String()+"中")
	g := make([]byte, 32)
	for i := range g {
		g[i] = byte(i*37 + 11)
	}
	base.Glyphs['中'] = g
	l2 := shrinkLevels[1]
	zero, half := l2, l2
	zero.Thr = shrinkThr{}
	half.Thr = shrinkThr{1, 2}
	third := l2
	third.Thr = shrinkThr{1, 3}
	for _, scale := range []int{2, 3} {
		fz, fh, ft := shrinkFontsOf(base, zero, scale), shrinkFontsOf(base, half, scale), shrinkFontsOf(base, third, scale)
		if string(fz.Full.Glyphs['中']) != string(fh.Full.Glyphs['中']) || string(fz.Half.Glyphs['A']) != string(fh.Half.Glyphs['A']) {
			t.Errorf("%d×: the zero threshold is not 1/2", scale)
		}
		// At 2× a 16×16 source gives 2×2 blocks, whose shares are multiples of
		// 1/4: no pixel lies between 1/3 and 1/2, so the threshold shows at 3×.
		if scale == 3 && string(fz.Full.Glyphs['中']) == string(ft.Full.Glyphs['中']) {
			t.Errorf("%d×: threshold 1/3 gave the glyph of 1/2", scale)
		}
		for r, hg := range fz.Half.Glyphs {
			if string(hg) != string(ft.Half.Glyphs[r]) {
				t.Errorf("%d×: the half glyph %q depends on the threshold", scale, r)
				break
			}
		}
		if fz == ft || fh == ft {
			t.Errorf("%d×: the cache returned one font for two thresholds", scale)
		}
		if shrinkFontsOf(base, third, scale) != ft {
			t.Errorf("%d×: the cache missed the same spec", scale)
		}
		if !strings.Contains(ft.Full.Name, "t1of3") || strings.Contains(ft.Full.Name, "/") {
			t.Errorf("%d×: font name %q", scale, ft.Full.Name)
		}
	}
	// The same level number with another size is another font.
	tall := l2
	tall.X2.FullGlyphH, tall.X2.FullGlyphY = 12, 4
	if shrinkFontsOf(base, tall, 2) == shrinkFontsOf(base, l2, 2) {
		t.Error("one font for two sizes of the same level")
	}
	// An invalid threshold panics at the derivation entry.
	for _, bad := range []shrinkThr{{0, 3}, {3, 0}, {4, 3}, {-1, 2}, {1, -2}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("threshold %+v did not panic", bad)
				}
			}()
			s := l2
			s.Thr = bad
			shrinkFontsOf(base, s, 2)
		}()
	}
}

// withShrinkLevels clears the language tables, withLangShrinkLevels replaces
// one language and restores it, and an empty table switches a language off.
func TestShrinkLevelsHelpers(t *testing.T) {
	def, ja := shrinkLevelsFor(LangZhTW), shrinkLevelsFor(LangJa)
	if len(def) != 2 || len(ja) != 2 || ja[1] == def[1] {
		t.Fatalf("tables: default %d levels, ja %d levels", len(def), len(ja))
	}
	t.Run("replace", func(t *testing.T) {
		withShrinkLevels(t, def[0])
		for _, lang := range []string{"", LangZhTW, LangJa, LangKo} {
			if got := shrinkLevelsFor(lang); len(got) != 1 || got[0] != def[0] {
				t.Errorf("language %q reads %+v after withShrinkLevels", lang, got)
			}
		}
	})
	t.Run("off", func(t *testing.T) {
		noShrink(t)
		for _, lang := range []string{"", LangJa} {
			if len(shrinkLevelsFor(lang)) != 0 {
				t.Errorf("language %q still has levels", lang)
			}
		}
	})
	t.Run("one language", func(t *testing.T) {
		withLangShrinkLevels(t, LangJa)
		if len(shrinkLevelsFor(LangJa)) != 0 || len(shrinkLevelsFor(LangZhTW)) != 2 {
			t.Errorf("ja %d levels, zh-TW %d levels", len(shrinkLevelsFor(LangJa)), len(shrinkLevelsFor(LangZhTW)))
		}
	})
	if got := shrinkLevelsFor(LangJa); len(got) != 2 || got[1] != ja[1] {
		t.Errorf("the ja table was not restored: %+v", got)
	}
}

// ---- the table of a language reaches the layout and the drawing ----

// injectedL2 is a test level whose widths and threshold differ from the
// default L2 and from the Japanese one (full 5, half 3 logical pixels, 10×12
// and 15×16 glyphs, threshold 1/3).
var injectedL2 = shrinkSpec{Level: 2, FullLP: 5, HalfLP: 3, Thr: shrinkThr{1, 3},
	X2: shrinkMetrics{FullCell: 10, FullGlyphW: 10, FullGlyphH: 12, FullInset: 0, HalfW: 6, HalfH: 12, FullGlyphY: 4, HalfGlyphY: 4},
	X3: shrinkMetrics{FullCell: 15, FullGlyphW: 15, FullGlyphH: 16, FullInset: 0, HalfW: 9, HalfH: 18, FullGlyphY: 7, HalfGlyphY: 6}}

// injectedResult is what the scenario of a language gives: the end column of a
// player name and of an NPC annotation (each in a 12-unit window where only L2
// fits), the level of their unit, and what the overlay draws for the player's
// page.
type injectedResult struct {
	playerEnd, npcEnd     uint8
	playerLevel, npcLevel uint8
	cellWs                string // CellW of the stamps of the unit
	fullFont, halfFont    string // glyph sizes of the stamps of the unit
	levelsOfWatcher       int    // FullLP of the L2 the watcher reads
}

func injectedScenario(t *testing.T, lang string) injectedResult {
	t.Helper()
	var res injectedResult
	// The player name セレステ(CELESTE): F 4, H 9; L0 17, L1 (6, 3) 13 units,
	// default and ja L2 (4, 2) 9, ko L2 (5, 2) 10, injected L2 (5, 3) 12.
	party, err := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "CELESTE"}), testDS)
	if err != nil {
		t.Fatal(err)
	}
	w := NewEclTextWatcher(eclFixtureLang(t, lang, "UNUSED FIXTURE", "x"))
	w.SetLayout(LayoutFor(lang))
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "セレステ"}, nil))
	e := eclPlayerEntry("CELESTE", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e.Left, e.Right, e.Top, e.Bottom = 1, 6, 17, 17 // 12 half units
	w.ObserveEntry(e)
	p := w.Page()
	if p == nil || lang != "" && p.Lang != lang { // the loader turns "" into zh-TW
		t.Fatalf("%q: page %+v", lang, p)
	}
	res.playerEnd = p.endCol
	var unit *EclTextLine
	for i := range p.Lines {
		if p.Lines[i].Shrink != 0 {
			unit = &p.Lines[i]
		}
	}
	if unit == nil {
		t.Fatalf("%q: the player name was not shrunk: %s", lang, shrinkDump(p.Lines))
	}
	res.playerLevel = unit.Shrink
	res.levelsOfWatcher = shrinkLevelsFor(w.catalog.langOf())[1].FullLP
	// The overlay of the same language reads the table through the page.
	font := halfTestFont("inj", "セレステ"+"CELESTE()")
	o, err := NewEclTextOverlay(font, 2)
	if err != nil {
		t.Fatal(err)
	}
	if miss := o.Sync([]*EclTextPage{p}, 1, shrinkPalette()); len(miss) != 0 {
		t.Fatalf("%q: missing %q", lang, string(miss))
	}
	var cw []string
	for _, s := range o.layer.Stamps {
		if s.Font == nil || !strings.Contains(s.Font.Name, ".shrink") { // not a stamp of the unit
			continue
		}
		cw = append(cw, fmt.Sprint(s.CellW))
		if isHalfwidth(s.Text[0]) {
			res.halfFont = fmt.Sprintf("%dx%d", s.Font.W, s.Font.H)
		} else {
			res.fullFont = fmt.Sprintf("%dx%d", s.Font.W, s.Font.H)
		}
	}
	res.cellWs = strings.Join(cw, ",")
	// The NPC annotation: the glossary names セレステ.
	g, err := LoadNameGlossary([]byte(pre056GlossaryHead+"CELESTE\t\tセレステ\tfull\tp0\tprinted:x\tn\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	w2 := NewEclTextWatcher(eclFixtureLang(t, lang, "NPC", "セレステ"))
	w2.SetNames(g)
	w2.SetLayout(LayoutFor(lang))
	e2 := eclEntry("NPC", true, 1, 17)
	e2.Left, e2.Right, e2.Top, e2.Bottom = 1, 6, 17, 17
	w2.ObserveEntry(e2)
	p2 := w2.Page()
	if p2 == nil {
		t.Fatalf("%q: no NPC page: %+v", lang, w2.Stats)
	}
	res.npcEnd = p2.endCol
	for _, l := range p2.Lines {
		if l.Shrink != 0 {
			res.npcLevel = l.Shrink
		}
	}
	return res
}

// The injected table reaches the layout of the watcher (NPC annotations and
// player names), the overlay (through the page) and the derived fonts, for the
// language it is set for; the other languages read the default table.  Japanese
// has the same widths as the default table, so only a table of other widths
// shows whether the watcher and the overlay read the table at all.
func TestShrinkInjectedTable(t *testing.T) {
	// Without injection every language fits the unit at L2.  The unit セレステ
	// (CELESTE) has F 4 and H 9: W_s 9 (end column 2 + 9) with the default and the
	// ja cell, 10 with the cell of ko (FullLP 5, HalfLP 2).  The glyphs of the
	// full-width characters are 8×8 (default), 8×14 (ja) and 10×12 (ko).
	for lang, want := range map[string]injectedResult{
		"":       {playerEnd: 11, npcEnd: 11, playerLevel: 2, npcLevel: 2, cellWs: "4,2", fullFont: "8x8", halfFont: "4x8", levelsOfWatcher: 4},
		LangZhTW: {playerEnd: 11, npcEnd: 11, playerLevel: 2, npcLevel: 2, cellWs: "4,2", fullFont: "8x8", halfFont: "4x8", levelsOfWatcher: 4},
		LangZhCN: {playerEnd: 11, npcEnd: 11, playerLevel: 2, npcLevel: 2, cellWs: "4,2", fullFont: "8x8", halfFont: "4x8", levelsOfWatcher: 4},
		LangJa:   {playerEnd: 11, npcEnd: 11, playerLevel: 2, npcLevel: 2, cellWs: "4,2", fullFont: "8x14", halfFont: "4x8", levelsOfWatcher: 4},
		LangKo:   {playerEnd: 12, npcEnd: 12, playerLevel: 2, npcLevel: 2, cellWs: "5,2", fullFont: "10x12", halfFont: "4x8", levelsOfWatcher: 5},
	} {
		if r := injectedScenario(t, lang); r != want {
			t.Errorf("%q: %+v want %+v", lang, r, want)
		}
	}
	// A table injected for one language reaches the watcher, the page and the
	// overlay of that language only; the other languages keep their tables.
	// (The injected L2 has full 5, half 3, so it differs from the table of every
	// language, ko included.)
	for _, inj := range []string{LangJa, LangKo} {
		t.Run("injected "+inj, func(t *testing.T) {
			withLangShrinkLevels(t, inj, shrinkLevels[0], injectedL2)
			checkShrinkGeometry(t, "injected", shrinkLevelsFor(inj))
			r := injectedScenario(t, inj)
			if r.playerEnd != 14 || r.npcEnd != 14 || r.playerLevel != 2 || r.npcLevel != 2 || r.levelsOfWatcher != 5 {
				t.Errorf("%s with the injected table: %+v", inj, r)
			}
			if r.cellWs != "5,3" || r.fullFont != "10x12" || r.halfFont != "6x12" {
				t.Errorf("%s overlay with the injected table: %+v", inj, r)
			}
			for _, lang := range []string{"", LangZhTW, LangZhCN, LangJa, LangKo} {
				if lang == inj {
					continue
				}
				want := injectedScenario(t, lang)
				switch lang {
				case LangKo:
					if want.playerEnd != 12 || want.fullFont != "10x12" {
						t.Errorf("ko changed with the table of %s: %+v", inj, want)
					}
				case LangJa:
					if want.playerEnd != 11 || want.fullFont != "8x14" {
						t.Errorf("ja changed with the table of %s: %+v", inj, want)
					}
				default:
					if want.playerEnd != 11 || want.npcEnd != 11 || want.fullFont != "8x8" {
						t.Errorf("%q changed with the table of %s: %+v", lang, inj, want)
					}
				}
			}
		})
	}
	// The order the languages are loaded in changes nothing: every language
	// gives the same result whatever came before it.
	want := map[string]injectedResult{}
	for _, lang := range []string{"", LangZhTW, LangZhCN, LangJa, LangKo} {
		want[lang] = injectedScenario(t, lang)
	}
	for _, lang := range []string{LangKo, LangJa, LangZhCN, LangZhTW, ""} {
		if got := injectedScenario(t, lang); got != want[lang] {
			t.Errorf("%q: %+v after other languages, %+v before", lang, got, want[lang])
		}
	}
}

// A level the page's table lacks draws nothing and writes no slot (the current
// behaviour, spec 056 §3.6): the unit is not drawn and the slots stay blank.
func TestShrinkMissingLevelDrawsNothing(t *testing.T) {
	for _, lang := range []string{"", LangJa, LangKo} {
		p := shrinkPageLang(lang, EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 3})
		txt, units := eclRowTextShrink(p, 0, 0)
		if len(units) != 0 || strings.ContainsAny(string(txt), "中AB") {
			t.Errorf("%q: unknown level drew %d units: %q", lang, len(units), string(txt))
		}
		o, err := NewEclTextOverlay(halfTestFont("miss", "中AB"), 2)
		if err != nil {
			t.Fatal(err)
		}
		if miss := o.Sync([]*EclTextPage{p}, 1, shrinkPalette()); len(miss) != 0 {
			t.Errorf("%q: missing %q", lang, string(miss))
		}
	}
}

// Spec 057 §5.1 (environment gate): one runtime with the four languages loaded.
// Each lane's watcher reads the table of its language, and the layouts of the
// lanes, in either order, equal the layouts of a runtime that loaded that
// language alone (no table is shared or left behind by another language, and
// switching the language with F4 has nothing to rebuild).
func TestShrinkLanesShareNoTable(t *testing.T) {
	langs := []string{LangZhTW, LangZhCN, LangJa, LangKo}
	dump := func(r *LiveRuntime, lang string) string {
		w, places, players, _, _ := shrinkFormalCorpus(t, r, lang, 300)
		if !reflect.DeepEqual(w.levels(), shrinkLevelsFor(lang)) {
			t.Errorf("%s：watcher 取的表不是 %s 的表", lang, lang)
		}
		var sb strings.Builder
		for _, p := range places {
			c := p.c
			shrinkDumpPlacement(&sb, w.placeText(c.txt, p.key, false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry()))
		}
		for _, c := range players {
			shrinkDumpPlayer(&sb, layoutPlayerName(w.layout, w.levels(), c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b))
		}
		return fmt.Sprintf("%x", sha256.Sum256([]byte(sb.String())))
	}
	all := shrinkFormalLoad(t)
	fwd, rev := map[string]string{}, map[string]string{}
	for _, l := range langs {
		fwd[l] = dump(all, l)
	}
	for i := len(langs) - 1; i >= 0; i-- {
		rev[langs[i]] = dump(all, langs[i])
	}
	for _, l := range langs {
		alone := dump(shrinkFormalLoad(t, l), l)
		if fwd[l] != alone || rev[l] != alone {
			t.Errorf("%s：四語言載入（順 %s、逆 %s）與單獨載入（%s）的版面不同", l, fwd[l][:8], rev[l][:8], alone[:8])
		}
	}
}

// The slots a shrunk unit takes in a row follow the table of the page's
// language (spec 057 §3.2): with a unit three half units wide in that table and
// two in the default one, a later line over the third slot removes the unit
// only where the table says so.
func TestShrunkUnitSlotsFollowPageTable(t *testing.T) {
	unit := EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 2}
	over := EclTextLine{Row: 0, Col: 4, Text: []rune("文")}
	units := func(lang string) int {
		_, kept := eclRowTextShrink(shrinkPageLang(lang, unit, over), 0, 0)
		return len(kept)
	}
	// Default L2 (4, 2): (4 + 2 + 2) / 4 = 2 units; the line at 4 is outside it.
	for _, lang := range []string{"", LangZhTW, LangZhCN, LangJa} {
		if n := units(lang); n != 1 {
			t.Errorf("%q: %d units kept, want 1 (the unit takes 2 half units)", lang, n)
		}
	}
	// The L2 of ko (5, 2): (5 + 2 + 2) / 4 rounded up = 3 units; the line over the
	// third slot removes the unit.
	if n := units(LangKo); n != 0 {
		t.Errorf("ko: %d units kept, want 0 (the unit takes 3 half units)", n)
	}
	// Injected L2 (5, 3): (5 + 3 + 3) / 4 rounded up = 3 units; the line over the
	// third slot removes the unit.
	withLangShrinkLevels(t, LangJa, shrinkLevels[0], injectedL2)
	if n := units(LangJa); n != 0 {
		t.Errorf("ja with the injected table: %d units kept, want 0 (the unit takes 3 half units)", n)
	}
	if n := units(LangZhTW); n != 1 {
		t.Errorf("zh-TW changed with the table of ja: %d units kept", n)
	}
}

// A level the page's table lacks is not looked up in the default table when
// the overlay checks the fonts (spec 057 §3.2): the unit is not drawn and no
// glyph is reported missing, while a table that has the level reports the
// runes the font lacks.
func TestShrinkSyncMissingRunesFollowPageTable(t *testing.T) {
	unit := EclTextLine{Row: 0, Col: 2, Text: []rune("中AB"), Shrink: 2}
	sync := func(lang string) []rune {
		o, err := NewEclTextOverlay(halfTestFont("missing", "AB"), 2) // no glyph for 中
		if err != nil {
			t.Fatal(err)
		}
		return o.Sync([]*EclTextPage{shrinkPageLang(lang, unit)}, 1, shrinkPalette())
	}
	if miss := sync(LangJa); string(miss) != "中" {
		t.Errorf("ja with L2: missing %q, want 中", string(miss))
	}
	withLangShrinkLevels(t, LangJa, shrinkLevels[0]) // ja without L2; the default table still has it
	if miss := sync(LangJa); len(miss) != 0 {
		t.Errorf("ja without L2: missing %q, want none", string(miss))
	}
	if miss := sync(LangZhTW); string(miss) != "中" {
		t.Errorf("zh-TW changed with the table of ja: missing %q", string(miss))
	}
	if miss := sync(LangKo); string(miss) != "中" {
		t.Errorf("ko changed with the table of ja: missing %q", string(miss))
	}
	withLangShrinkLevels(t, LangKo, shrinkLevels[0]) // ko without L2
	if miss := sync(LangKo); len(miss) != 0 {
		t.Errorf("ko without L2: missing %q, want none", string(miss))
	}
}
