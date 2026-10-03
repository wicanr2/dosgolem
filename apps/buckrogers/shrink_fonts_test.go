package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
	"github.com/wicanr2/dosgolem/xlate/translitjk"
)

// Spec 056 §3.3 and §5.1: the static availability check.  A character of a
// name unit whose source glyph has ink must still have ink at every
// presenter, language and level, or the overlay reports it missing and the
// page turns into the English original (live_lane.go syncEclText).  A level
// that fails goes out of shrinkLevels.
//
// Inputs (environment gate): the release package, BUCKROGERS_PKG_ROOT (text/
// and font/ in it), or BUCKROGERS_CHT_ROOT with the four font variables the
// other formal tests use.  BUCKROGERS_ZHTW_ETEN_FONT adds the local Eten
// font.  BUCKROGERS_SHRINK_REPORT_DIR, when set, receives the list of
// different source characters whose derived glyphs are the same.  tools/
// package.sh runs this test on the fonts it has just built and fails the
// package unless it ran and passed.

type shrinkFontCase struct {
	label string
	font  *xlate.Font
	runes []rune
}

func shrinkRuneSet(t *testing.T, textDir, lang string, glossary []NameGlossaryEntry) []rune {
	t.Helper()
	set := map[rune]bool{}
	for r := rune(0x21); r < 0x7F; r++ {
		set[r] = true
	}
	set['•'] = true
	for _, e := range glossary {
		for _, r := range e.Chinese + e.English {
			set[r] = true
		}
	}
	chars := func(name string) []rune {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			t.Fatal(err)
		}
		var out []rune
		for i, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
			if i == 0 {
				continue
			}
			if f := strings.Split(line, "\t"); len(f) >= 2 {
				out = append(out, []rune(f[1])...)
			}
		}
		return out
	}
	switch lang {
	case LangZhTW:
		for _, r := range chars("translit-chars.zh-TW.tsv") {
			set[r] = true
		}
	case LangZhCN:
		b, err := os.ReadFile(filepath.Join(textDir, TranslitMapFile(lang)))
		if err != nil {
			t.Fatal(err)
		}
		m, err := LoadTranslitCharMap(TranslitMapFile(lang), b)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range chars("translit-chars.zh-TW.tsv") {
			if cn, ok := m[r]; ok {
				set[cn] = true
			}
		}
	case LangJa, LangKo:
		tr, err := translitjk.Load(textDir, lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range tr.Allowed() {
			set[r] = true
		}
	}
	var out []rune
	for r := range set {
		if !isBlankRune(r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// shrinkLevelReport is what the comparison finds for one level and scale
// (spec 057 §3.5).
type shrinkLevelReport struct {
	Level, Scale int
	Checked      int      // characters with ink in the source
	Empty        []rune   // ink in the source, none (or no glyph) derived
	FullGroups   [][]rune // full-width characters with different source glyphs and the same derived glyph
	HalfGroups   [][]rune // the same for half-width characters
	SrcIdentical [][]rune // full-width characters whose source glyphs are the same (no derivation can part them)
	Skipped      []rune   // characters the source lacks (L0 lacks them too)
}

// shrinkDistinctReports derives the font at every level of the table and scale
// and reports, per level, the characters that are empty and the groups of
// characters whose derived glyphs are the same.  A rune is full-width when it
// is not isHalfwidth; blanks are expected not to be in runes.
func shrinkDistinctReports(font *xlate.Font, runes []rune, levels []shrinkSpec, scale int) []shrinkLevelReport {
	half := halfFontsOf(font)
	var out []shrinkLevelReport
	for _, sp := range levels {
		rep := shrinkLevelReport{Level: sp.Level, Scale: scale}
		fs := shrinkFontsOf(font, sp, scale)
		type entry struct{ src, derived string }
		groups := map[string][]rune{}
		srcSame := map[string][]rune{}
		srcOf := map[rune]string{}
		for _, ru := range runes {
			halfRune := isHalfwidth(ru)
			src, derived := font, fs.Full
			if halfRune {
				src, derived = half.X2, fs.Half
			}
			g, ok := src.Glyphs[ru]
			if !ok || !glyphHasInk(g) {
				rep.Skipped = append(rep.Skipped, ru)
				continue
			}
			rep.Checked++
			srcOf[ru] = string(g)
			if !halfRune {
				srcSame[string(g)] = append(srcSame[string(g)], ru)
			}
			var d []byte
			if derived != nil {
				d = derived.Glyphs[ru]
			}
			if len(d) == 0 || !glyphHasInk(d) {
				rep.Empty = append(rep.Empty, ru)
				continue
			}
			groups[fmt.Sprintf("%v/%x", halfRune, d)] = append(groups[fmt.Sprintf("%v/%x", halfRune, d)], ru)
		}
		var keys []string
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			g := groups[k]
			diff := map[string]bool{}
			for _, ru := range g {
				diff[srcOf[ru]] = true
			}
			if len(diff) < 2 {
				continue
			}
			sort.Slice(g, func(i, j int) bool { return g[i] < g[j] })
			if strings.HasPrefix(k, "true/") {
				rep.HalfGroups = append(rep.HalfGroups, g)
			} else {
				rep.FullGroups = append(rep.FullGroups, g)
			}
		}
		for _, g := range srcSame {
			if len(g) > 1 {
				sort.Slice(g, func(i, j int) bool { return g[i] < g[j] })
				rep.SrcIdentical = append(rep.SrcIdentical, g)
			}
		}
		sort.Slice(rep.SrcIdentical, func(i, j int) bool { return rep.SrcIdentical[i][0] < rep.SrcIdentical[j][0] })
		sort.Slice(rep.Empty, func(i, j int) bool { return rep.Empty[i] < rep.Empty[j] })
		out = append(out, rep)
	}
	return out
}

// shrinkDistinctProblems is the verdict of the static check on one report
// (spec 057 §3.5 (2) and (3)): the characters that came out empty, the groups
// of full-width characters that are the same.  Half-width groups are reported
// only.  (Spec 057 carried a documented exception for ko until spec 058; there
// is none now.)
func shrinkDistinctProblems(label string, rep shrinkLevelReport) []string {
	var out []string
	if len(rep.Empty) > 0 {
		out = append(out, fmt.Sprintf("%s：%d 個字衍生後全空（例 %q）：此等級必須從使用該表的語言移除（規格 057 §3.5 第 3 點）", label, len(rep.Empty), string(rep.Empty[:min(len(rep.Empty), 12)])))
	}
	for _, g := range rep.FullGroups {
		out = append(out, fmt.Sprintf("%s：全形字模相同的不同字 %q（規格 057 §3.5 第 2 點的處置）", label, string(g)))
	}
	return out
}

// The comparison finds groups on a synthetic font: two characters whose
// sources differ by one pixel collapse at 8×8, two characters whose sources are
// the same are excluded, a very different character stays apart, and a blank
// glyph is skipped.  No text/ or font file is involved.
func TestShrinkDistinctSynthetic(t *testing.T) {
	f := &xlate.Font{W: 16, H: 16, Name: "syn", Glyphs: map[rune][]byte{}}
	block := func(x0, y0, x1, y1 int) []byte {
		g := make([]byte, 32)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				g[y*2+x/8] |= 0x80 >> uint(x%8)
			}
		}
		return g
	}
	a := block(2, 2, 14, 6)
	b := append([]byte(nil), a...)
	b[7*2+0] |= 0x40 // one pixel at (1,7): a quarter of a target pixel at 8×8
	f.Glyphs['あ'], f.Glyphs['い'] = a, b
	f.Glyphs['う'] = append([]byte(nil), a...) // the same source as あ
	f.Glyphs['え'] = block(2, 8, 14, 15)       // far from the others
	f.Glyphs['お'] = make([]byte, 32)          // blank
	runes := []rune("あいうえお")
	reps := shrinkDistinctReports(f, runes, shrinkLevels[1:], 2)
	if len(reps) != 1 {
		t.Fatalf("%d reports", len(reps))
	}
	r := reps[0]
	if r.Level != 2 || r.Checked != 4 || len(r.Skipped) != 1 || r.Skipped[0] != 'お' {
		t.Fatalf("report %+v", r)
	}
	if len(r.FullGroups) != 1 || string(r.FullGroups[0]) != "あい" && string(r.FullGroups[0]) != "あいう" {
		t.Errorf("groups %q", r.FullGroups)
	}
	if len(r.SrcIdentical) != 1 || string(r.SrcIdentical[0]) != "あう" {
		t.Errorf("source-identical %q", r.SrcIdentical)
	}
	for _, g := range r.FullGroups {
		for _, ru := range g {
			if ru == 'え' {
				t.Errorf("え is far from the others but is in a group: %q", string(g))
			}
		}
	}
	// Both levels of a table are compared, in the order of the table.
	both := shrinkDistinctReports(f, runes, shrinkLevels, 2)
	if len(both) != 2 || both[0].Level != 1 || both[1].Level != 2 || both[0].Checked != 4 || both[1].Checked != 4 {
		t.Errorf("a two-level table gave %+v", both)
	}
	// The verdict: a group fails the check and so does an empty character.
	if m := shrinkDistinctProblems("syn", r); len(m) != 1 {
		t.Errorf("group: %q", m)
	}
	if m := shrinkDistinctProblems("syn", shrinkLevelReport{Empty: []rune("あ")}); len(m) != 1 {
		t.Errorf("empty character: %q", m)
	}
	if m := shrinkDistinctProblems("syn", shrinkLevelReport{HalfGroups: [][]rune{[]rune("HM")}}); len(m) != 0 {
		t.Errorf("half-width groups are reported only: %q", m)
	}
	// A level that keeps the glyph apart finds no group: the full size.
	big := shrinkSpec{Level: 1, FullLP: 8, HalfLP: 4, X2: shrinkMetrics{FullCell: 16, FullGlyphW: 16, FullGlyphH: 16, HalfW: 8, HalfH: 16}}
	if reps := shrinkDistinctReports(f, runes, []shrinkSpec{big}, 2); len(reps[0].FullGroups) != 0 {
		t.Errorf("full-size derivation merged glyphs: %q", reps[0].FullGroups)
	}
}

// The static availability check (spec 056 §3.3 and §5.1, spec 057 §3.5): for
// every font, scale and level of the table of its language, the characters of
// the name set that the source draws have ink after the derivation, the
// full-width ones are pairwise distinct, and the table holds the levels the
// spec expects.  Pairs of half-width characters are reported only.  It must
// pass (not skip) in tools/package.sh.
func TestShrinkFontAvailability(t *testing.T) {
	var textDir string
	fonts := map[string]string{}
	if root := os.Getenv("BUCKROGERS_PKG_ROOT"); root != "" {
		textDir = filepath.Join(root, "text")
		fonts[LangZhTW] = filepath.Join(root, "font", "buckrogers-unifont.golemfnt")
		for _, l := range []string{LangZhCN, LangJa, LangKo} {
			fonts[l] = filepath.Join(root, "font", "buckrogers-"+l+".golemfnt")
		}
	} else if root := os.Getenv("BUCKROGERS_CHT_ROOT"); root != "" {
		textDir = filepath.Join(root, "text")
		fonts[LangZhTW] = os.Getenv("BUCKROGERS_ZHTW_FONT")
		fonts[LangZhCN] = os.Getenv("BUCKROGERS_ZHCN_FONT")
		fonts[LangJa] = os.Getenv("BUCKROGERS_JA_FONT")
		fonts[LangKo] = os.Getenv("BUCKROGERS_KO_FONT")
		for _, f := range fonts {
			if f == "" {
				t.Skip("四個語言字型的環境變數未設定")
			}
		}
	} else {
		t.Skip("BUCKROGERS_PKG_ROOT、BUCKROGERS_CHT_ROOT 都未設定：縮小字模可用性未檢查")
	}
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: textDir, FontPath: fonts[LangZhTW],
		Langs: []string{LangZhCN, LangJa, LangKo}, LangFonts: map[string]string{LangZhCN: fonts[LangZhCN], LangJa: fonts[LangJa], LangKo: fonts[LangKo]}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.off) != 0 {
		t.Fatalf("語言停用：%v", r.off)
	}
	// Spec 057 §3.5 (4): the expected levels of every language and a lower bound
	// on the number of full-width characters the check covers.
	minFull := map[string]int{LangZhTW: 336, LangZhCN: 336, LangJa: 88, LangKo: 2128}
	type fontCase struct {
		label, lang string
		font        *xlate.Font
		runes       []rune
	}
	var cases []fontCase
	for _, lang := range []string{LangZhTW, LangZhCN, LangJa, LangKo} {
		l := r.lanes[r.laneIndex(lang)]
		set := shrinkRuneSet(t, textDir, lang, l.names.names)
		cases = append(cases, fontCase{lang + " " + filepath.Base(fonts[lang]), lang, l.font, set})
		if lang == LangZhTW {
			if p := os.Getenv("BUCKROGERS_ZHTW_ETEN_FONT"); p != "" {
				f, err := xlate.LoadFont(p)
				if err != nil {
					t.Fatal(err)
				}
				cases = append(cases, fontCase{lang + " " + filepath.Base(p), lang, f, set})
			}
		}
	}
	var report strings.Builder
	checked := 0
	skipped := map[string]map[rune]bool{}
	for _, c := range cases {
		levels := shrinkLevelsFor(c.lang)
		if len(levels) != 2 || levels[0].Level != 1 || levels[1].Level != 2 {
			t.Fatalf("%s：語言 %s 的等級表要有 L1、L2：%+v", c.label, c.lang, levels)
		}
		half := halfFontsOf(c.font)
		if half == nil || half.Err != nil || half.X2 == nil {
			t.Errorf("%s：半形字型不可用", c.label)
			continue
		}
		for _, scale := range []int{2, 3} {
			for _, rep := range shrinkDistinctReports(c.font, c.runes, levels, scale) {
				full := 0
				for _, ru := range c.runes {
					if g, ok := c.font.Glyphs[ru]; ok && glyphHasInk(g) && !isHalfwidth(ru) {
						full++
					}
				}
				if full < minFull[c.lang] {
					t.Errorf("%s：名字字元集的全形字 %d 個，少於下限 %d（字元集載入不全？）", c.label, full, minFull[c.lang])
				}
				checked += rep.Checked
				for _, m := range shrinkDistinctProblems(fmt.Sprintf("%s %d× L%d", c.label, scale, rep.Level), rep) {
					t.Error(m)
				}
				for _, g := range rep.FullGroups {
					fmt.Fprintf(&report, "%s\t%d×\tL%d\t%s\n", c.label, scale, rep.Level, string(g))
				}
				for _, g := range rep.HalfGroups {
					fmt.Fprintf(&report, "%s\t%d×\tL%d\t%s\n", c.label, scale, rep.Level, string(g))
				}
				if skipped[c.label] == nil {
					skipped[c.label] = map[rune]bool{}
				}
				for _, ru := range rep.Skipped {
					skipped[c.label][ru] = true
				}
				t.Logf("%s %d× L%d：檢查 %d 字，衍生後全空 %d，全形字模相同組 %d，半形字模相同組 %d，來源字模相同的全形字 %d 組", c.label, scale, rep.Level, rep.Checked, len(rep.Empty), len(rep.FullGroups), len(rep.HalfGroups), len(rep.SrcIdentical))
			}
		}
	}
	t.Logf("共檢查 %d 個（字、字型、倍率、等級）", checked)
	for label, set := range skipped {
		var rs []rune
		for r := range set {
			rs = append(rs, r)
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
		t.Logf("%s 的來源缺字（L0 也缺，不屬縮小）：%q", label, string(rs))
	}
	if dir := os.Getenv("BUCKROGERS_SHRINK_REPORT_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "shrink-identical-glyphs.tsv"), []byte("font\tscale\tlevel\tcharacters\n"+report.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Negative control (spec 057 §3.5 (5)): the comparison finds the groups of
	// spec 056's Japanese L2 on the formal font, kana of voiced and unvoiced
	// sounds among them, so a comparison that compares nothing cannot pass.
	if li := r.laneIndex(LangJa); li >= 0 {
		l := r.lanes[li]
		set := shrinkRuneSet(t, textDir, LangJa, l.names.names)
		old := shrinkDistinctReports(l.font, set, shrinkLevels[1:], 3)[0]
		found := false
		for _, g := range old.FullGroups {
			if strings.ContainsRune(string(g), 'カ') && strings.ContainsRune(string(g), 'ガ') {
				found = true
			}
		}
		if len(old.FullGroups) == 0 || !found {
			t.Errorf("負向對照失敗：規格 056 的 ja 3× L2 應回報含 カ／ガ 的組，得到 %q", old.FullGroups)
		}
	}
	// The same for ko (spec 058 §5.2): the table of spec 056 draws 게/계 the same
	// at 2× and 갠/갱 at 3×.  The exact numbers of groups (312 and 142 on the name
	// set of the day) are in the phase document, not in the gate: the name set
	// follows text/.
	if li := r.laneIndex(LangKo); li >= 0 {
		l := r.lanes[li]
		set := shrinkRuneSet(t, textDir, LangKo, l.names.names)
		for _, c := range []struct {
			scale int
			a, b  rune
		}{{2, '게', '계'}, {3, '갠', '갱'}} {
			old := shrinkDistinctReports(l.font, set, shrinkLevels[1:], c.scale)[0]
			found := false
			for _, g := range old.FullGroups {
				if strings.ContainsRune(string(g), c.a) && strings.ContainsRune(string(g), c.b) {
					found = true
				}
			}
			t.Logf("ko %d× L2 of spec 056: %d groups with the same glyph", c.scale, len(old.FullGroups))
			if len(old.FullGroups) == 0 || !found {
				t.Errorf("負向對照失敗：規格 056 的 ko %d× L2 應回報含 %c／%c 的組，得到 %d 組", c.scale, c.a, c.b, len(old.FullGroups))
			}
		}
	}
}
