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
	var cases []shrinkFontCase
	for _, lang := range []string{LangZhTW, LangZhCN, LangJa, LangKo} {
		l := r.lanes[r.laneIndex(lang)]
		set := shrinkRuneSet(t, textDir, lang, l.names.names)
		cases = append(cases, shrinkFontCase{lang + " " + filepath.Base(fonts[lang]), l.font, set})
		if lang == LangZhTW {
			if p := os.Getenv("BUCKROGERS_ZHTW_ETEN_FONT"); p != "" {
				f, err := xlate.LoadFont(p)
				if err != nil {
					t.Fatal(err)
				}
				cases = append(cases, shrinkFontCase{lang + " " + filepath.Base(p), f, set})
			}
		}
	}
	if len(shrinkLevels) == 0 {
		t.Fatal("shrinkLevels 是空表：沒有縮小等級可檢查")
	}
	var report strings.Builder
	checked, skippedSrc := 0, 0
	skipped := map[string]map[rune]bool{}
	for _, c := range cases {
		half := halfFontsOf(c.font)
		if half == nil || half.Err != nil || half.X2 == nil {
			t.Errorf("%s：半形字型不可用", c.label)
			continue
		}
		for _, scale := range []int{2, 3} {
			for _, sp := range shrinkLevels {
				fs := shrinkFontsOf(c.font, sp, scale)
				if fs.Half == nil {
					t.Errorf("%s %d× L%d：半形縮小字型缺", c.label, scale, sp.Level)
					continue
				}
				groups := map[string][]rune{} // derived glyph (with class) -> source characters
				srcOf := map[rune]string{}
				var empty []rune
				for _, ru := range c.runes {
					src, derived := c.font, fs.Full
					if isHalfwidth(ru) {
						src, derived = half.X2, fs.Half
					}
					g, ok := src.Glyphs[ru]
					if !ok || !glyphHasInk(g) {
						skippedSrc++ // the source lacks it: L0 lacks it too, spec 056 §3.3
						if skipped[c.label] == nil {
							skipped[c.label] = map[rune]bool{}
						}
						skipped[c.label][ru] = true
						continue
					}
					checked++
					d, ok := derived.Glyphs[ru]
					if !ok || !glyphHasInk(d) {
						empty = append(empty, ru)
						continue
					}
					key := fmt.Sprintf("%v/%x", isHalfwidth(ru), d)
					groups[key] = append(groups[key], ru)
					srcOf[ru] = fmt.Sprintf("%x", g)
				}
				if len(empty) > 0 {
					t.Errorf("%s %d× L%d：%d 個字衍生後全空（例 %q）：此等級必須從 shrinkLevels 移除", c.label, scale, sp.Level, len(empty), string(empty[:min(len(empty), 12)]))
				}
				var keys []string
				for k := range groups {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				pairs, sets := 0, 0
				for _, k := range keys {
					g := groups[k]
					if len(g) < 2 {
						continue
					}
					diff := map[string]bool{}
					for _, ru := range g {
						diff[srcOf[ru]] = true
					}
					if len(diff) < 2 {
						continue
					}
					sets++
					n := len(g)
					pairs += n * (n - 1) / 2
					fmt.Fprintf(&report, "%s\t%d×\tL%d\t%s\n", c.label, scale, sp.Level, string(g))
				}
				t.Logf("%s %d× L%d：檢查 %d 字，衍生後全空 %d，相同字模的不同來源字 %d 組（%d 對）", c.label, scale, sp.Level, len(c.runes), len(empty), sets, pairs)
			}
		}
	}
	t.Logf("共檢查 %d 個（字、字型、倍率、等級）；來源缺字略過 %d", checked, skippedSrc)
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
}
