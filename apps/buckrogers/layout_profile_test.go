package buckrogers

import (
	"strings"
	"testing"
)

func lineTexts(lines []EclTextLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l.Text)
	}
	return out
}

// Spec 042 §3.4: the default (nil) profile is the pre-042 behaviour, the ja
// profile adds the kinsoku table.
func TestLayoutProfileKinsoku(t *testing.T) {
	ja := LayoutFor(LangJa)
	if ja == nil || LayoutFor(LangZhTW) != nil || LayoutFor(LangZhCN) != nil || LayoutFor(LangEn) != nil {
		t.Fatal("only ja has a layout profile")
	}
	// width 8 units = 4 full-width characters per line.
	cases := []struct {
		name, text string
		want       []string
		wantNil    []string // the default profile's lines
	}{
		{"small kana sticks to the character before", "あいうえゃお",
			[]string{"あいう", "えゃお"}, []string{"あいうえ", "ゃお"}},
		{"long vowel mark", "かきくけーこ",
			[]string{"かきく", "けーこ"}, []string{"かきくけ", "ーこ"}},
		{"middle dot", "アイウエ・オ",
			[]string{"アイウ", "エ・オ"}, []string{"アイウエ", "・オ"}},
		{"opening bracket joins the next token", "あいう「えお",
			[]string{"あいう", "「えお"}, []string{"あいう「", "えお"}},
		{"em dash pair stays together and off the line start", "あいうえ——お",
			[]string{"あいう", "え——お"}, []string{"あいうえ", "——お"}},
		{"closing brackets", "あいうえ】お",
			[]string{"あいう", "え】お"}, []string{"あいうえ", "】お"}},
	}
	for _, c := range cases {
		lines, _, _, ok := layoutEclTextP(ja, []rune(c.text), nil, 0, 0, 0, 7, 20)
		if !ok {
			t.Fatalf("%s: does not fit", c.name)
		}
		if got := lineTexts(lines); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: ja lines = %q, want %q", c.name, got, c.want)
		}
		lines, _, _, ok = layoutEclTextP(nil, []rune(c.text), nil, 0, 0, 0, 7, 20)
		if !ok {
			t.Fatalf("%s: default does not fit", c.name)
		}
		if got := lineTexts(lines); strings.Join(got, "|") != strings.Join(c.wantNil, "|") {
			t.Errorf("%s: default lines = %q, want %q", c.name, got, c.wantNil)
		}
	}
}

// The default profile and the pre-042 entry points are the same function.
func TestLayoutProfileNilIsDefault(t *testing.T) {
	text := []rune("あいうえゃお「かき」く……けこ")
	a, ar, ac, aok := layoutEclTextUnits(text, nil, 3, 4, 2, 15, 20)
	b, br, bc, bok := layoutEclTextP(nil, text, nil, 3, 4, 2, 15, 20)
	if aok != bok || ar != br || ac != bc || strings.Join(lineTexts(a), "|") != strings.Join(lineTexts(b), "|") {
		t.Fatal("nil profile differs from layoutEclTextUnits")
	}
}

// An opening bracket before a name-annotation unit joins the unit's first
// piece (spec 042 §3.4 tokenization).
func TestLayoutProfileOpeningBeforeNameUnit(t *testing.T) {
	ja := LayoutFor(LangJa)
	text := []rune("あいう「ロジャーズ(ROGERS)えお")
	units := []NameUnit{{Start: 4, End: 4 + len([]rune("ロジャーズ(ROGERS)"))}}
	lines, _, _, ok := layoutEclTextP(ja, text, units, 0, 0, 0, 25, 20)
	if !ok {
		t.Fatal("does not fit")
	}
	got := lineTexts(lines)
	for _, l := range got {
		if strings.HasSuffix(l, "「") {
			t.Fatalf("a line ends with an opening bracket: %q", got)
		}
	}
}

// A merged token that would be wider than the line is not merged (the old
// behaviour stays rather than failing the whole layout).
func TestLayoutProfileWideTokenNotMerged(t *testing.T) {
	ja := LayoutFor(LangJa)
	text := []rune("「ABCDEFGHIJKLMNOP")
	lines, _, _, ok := layoutEclTextP(ja, text, nil, 0, 0, 0, 9, 20) // width 10
	if ok {
		// the Latin run is 16 units: wider than the line either way
		t.Fatalf("expected the too-wide Latin run to fail, got %q", lineTexts(lines))
	}
}

func TestAdjustBreak(t *testing.T) {
	ja := LayoutFor(LangJa)
	r := []rune("あいうえゃお")
	if k := ja.adjustBreak(r, 4); k != 3 { // r[4]=ゃ cannot start the second row
		t.Errorf("adjustBreak = %d, want 3", k)
	}
	r = []rune("あい「うえ")
	if k := ja.adjustBreak(r, 3); k != 2 { // r[2]=「 cannot end the first row
		t.Errorf("adjustBreak = %d, want 2", k)
	}
	r = []rune("あいう——え")
	if k := ja.adjustBreak(r, 4); k != 2 { // the dash pair moves whole, with the character before it
		t.Errorf("adjustBreak = %d, want 2", k)
	}
	if k := LayoutFor(LangZhTW).adjustBreak(r, 4); k != 4 {
		t.Errorf("nil profile must not move the break, got %d", k)
	}
	if k := ja.adjustBreak([]rune("ゃゃゃ"), 1); k != 0 { // never below 0
		t.Errorf("adjustBreak = %d, want 0", k)
	}
}

// The logbook body uses the same profile (72 units per line).
func TestLogbookKinsoku(t *testing.T) {
	body := []rune(strings.Repeat("あ", 36) + "ーい")
	pagesJa, err := layoutLogbookUnitsP(LayoutFor(LangJa), body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pagesJa[0]) != 2 || !strings.HasPrefix(pagesJa[0][1], "あー") {
		t.Fatalf("ja lines = %q", pagesJa[0])
	}
	pagesZh, err := layoutLogbookUnits(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pagesZh[0][1], "ー") {
		t.Fatalf("default lines = %q", pagesZh[0])
	}
}
