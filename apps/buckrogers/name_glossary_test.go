package buckrogers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

const testGlossary = "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n" +
	"BUCK ROGERS\tBuck Rogers\t巴克羅吉斯\tfull\tbuck-rogers\tprinted:x\tn\n" +
	"BUCK\tBuck\t巴克\tshort\tbuck-rogers\tprinted:x\tn\n" +
	"WILMA\tWilma\t威瑪\tshort\twilma-deering\tprinted:x\tn\n" +
	"GILBERT\t\t吉爾伯特\tfull\tgilbert\txinhua\tn\n" +
	"JIM\t\t吉姆\tfull\tjim\txinhua\tn\n" +
	"SCOT.DOS\tScot.dos\t斯科特\tfull\tscot\txinhua\tn\n" +
	"ALEXANDER WILLIAMS\tAlexander Williams\t亞歷山大•威廉\tfull\talexander-williams\tprinted:x\tn\n" +
	"WILLIAMS\tWilliams\t威廉\tshort\talexander-williams\tprinted:x\tn\n"

const testExclude = "phrase\tscope\tnote\n" +
	"吉姆博火箭酒吧\t*\t店名\n" +
	"斯科特海鮮坊\tecl.*\t店名（測試用）\n"

func testNames(t *testing.T) *NameGlossary {
	t.Helper()
	g, err := LoadNameGlossary([]byte(testGlossary), []byte(testExclude))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func annotate(g *NameGlossary, text, key string, c NameCase, tier NameTier) string {
	return string(g.Annotate(text, key, c, tier).Text)
}

func TestNameGlossaryLoadRejects(t *testing.T) {
	head := "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n"
	for _, bad := range []string{
		head + "Buck\t\t巴克\tfull\tb\tx\tn\n",                           // not upper case
		head + "BUCK\tBock\t巴克\tfull\tb\tx\tn\n",                       // mixed is another spelling
		head + "BUCK\t\t巴克\tlong\tb\tx\tn\n",                           // kind
		head + "BUCK\t\t巴克\tfull\tb\tx\n",                              // columns
		head + "BUCK\t\t巴克\tfull\tb\tx\tn\nBUCK\t\t巴\tfull\tb\tx\tn\n", // duplicate
		"english\tchinese\n",
	} {
		if _, err := LoadNameGlossary([]byte(bad), nil); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestNameAnnotateLongestExclusionParenCase(t *testing.T) {
	g := testNames(t)
	// Longest first: the full name wins over its short form.
	if got := annotate(g, "巴克羅吉斯對巴克說。", "ecl.1", NameCaseUpper, NameTierAll); got != "巴克羅吉斯(BUCK ROGERS)對巴克(BUCK)說。" {
		t.Fatalf("longest %q", got)
	}
	if got := annotate(g, "亞歷山大•威廉與威廉", "logbook.3", NameCaseMixed, NameTierAll); got != "亞歷山大•威廉(Alexander Williams)與威廉(Williams)" {
		t.Fatalf("separator name %q", got)
	}
	// Exclusion phrases are skipped whole; scope follows the key.
	if got := annotate(g, "吉姆博火箭酒吧裡的吉姆", "ecl.1", NameCaseUpper, NameTierAll); got != "吉姆博火箭酒吧裡的吉姆(JIM)" {
		t.Fatalf("exclusion %q", got)
	}
	if got := annotate(g, "斯科特海鮮坊", "ecl.6.99", NameCaseUpper, NameTierAll); got != "斯科特海鮮坊" {
		t.Fatalf("scoped exclusion %q", got)
	}
	if got := annotate(g, "斯科特海鮮坊", "logbook.2", NameCaseMixed, NameTierAll); got != "斯科特(Scot.dos)海鮮坊" {
		t.Fatalf("out-of-scope exclusion %q", got)
	}
	// A name already followed by a parenthesis is left as is.
	for _, s := range []string{"威瑪(WILMA)來了", "威瑪（Wilma）來了"} {
		if got := annotate(g, s, "ecl.1", NameCaseUpper, NameTierAll); got != s {
			t.Fatalf("paren %q", got)
		}
	}
	// Case follows the family; an empty mixed spelling falls back to upper case.
	if got := annotate(g, "巴克與吉爾伯特", "logbook.1", NameCaseMixed, NameTierAll); got != "巴克(Buck)與吉爾伯特(GILBERT)" {
		t.Fatalf("mixed %q", got)
	}
	if got := annotate(g, "巴克與吉爾伯特", "ecl.1", NameCaseUpper, NameTierAll); got != "巴克(BUCK)與吉爾伯特(GILBERT)" {
		t.Fatalf("upper %q", got)
	}
	// Nil glossary: no annotation.
	var none *NameGlossary
	if got := annotate(none, "巴克", "ecl.1", NameCaseUpper, NameTierAll); got != "巴克" {
		t.Fatalf("nil %q", got)
	}
}

func TestNameVariantsTiers(t *testing.T) {
	g := testNames(t)
	v := g.Variants("巴克羅吉斯叫巴克和威瑪，威瑪(WILMA)回答威瑪。", "ecl.1", NameCaseUpper)
	want := []string{
		"巴克羅吉斯(BUCK ROGERS)叫巴克(BUCK)和威瑪(WILMA)，威瑪(WILMA)回答威瑪(WILMA)。",
		// Same person (full or short form) only once; the parenthesised
		// occurrence counts as later.
		"巴克羅吉斯(BUCK ROGERS)叫巴克和威瑪(WILMA)，威瑪(WILMA)回答威瑪。",
		"巴克羅吉斯叫巴克和威瑪，威瑪(WILMA)回答威瑪。",
	}
	if len(v) != 3 {
		t.Fatalf("variants %d", len(v))
	}
	for i := range want {
		if string(v[i].Text) != want[i] || v[i].Tier != NameTier(i) {
			t.Fatalf("tier %d: %q", i, string(v[i].Text))
		}
	}
	// Units cover exactly the inserted annotations.
	if u := v[1].Units; len(u) != 2 || string(v[1].Text[u[0].Start:u[0].End]) != "巴克羅吉斯(BUCK ROGERS)" ||
		string(v[1].Text[u[1].Start:u[1].End]) != "威瑪(WILMA)" {
		t.Fatalf("units %+v", u)
	}
	// Identical tiers collapse.
	if v := g.Variants("巴克來了。", "ecl.1", NameCaseUpper); len(v) != 2 || v[1].Tier != NameTierNone {
		t.Fatalf("collapse %+v", v)
	}
	if v := g.Variants("沒有人名。", "ecl.1", NameCaseUpper); len(v) != 1 {
		t.Fatalf("no names %+v", v)
	}
}

func TestLayoutKeepsNameUnitTogether(t *testing.T) {
	g := testNames(t)
	a := g.Annotate("一二三四五巴克(X)", "ecl.1", NameCaseUpper, NameTierAll) // already paren: no unit
	if len(a.Units) != 0 {
		t.Fatal("paren produced a unit")
	}
	a = g.Annotate("一二三四五巴克說。", "ecl.1", NameCaseUpper, NameTierAll)
	// Width 10: "一二三四五巴克(BUCK)" is 13 cells; without units the break
	// would fall inside the annotation.
	lines, _, _, ok := layoutEclTextUnits(a.Text, a.Units, 0, 0, 0, 9, 9)
	if !ok || len(lines) != 2 || string(lines[0].Text) != "一二三四五" || string(lines[1].Text) != "巴克(BUCK)說。" {
		t.Fatalf("unit split: %+v", lines)
	}
	plain, _, _, _ := layoutEclText(a.Text, 0, 0, 0, 9, 9)
	if string(plain[0].Text) == "一二三四五" {
		t.Fatal("control: plain layout should have broken inside the annotation")
	}
	// A unit wider than the line breaks only at English spaces.
	a = g.Annotate("亞歷山大•威廉到了。", "logbook.1", NameCaseMixed, NameTierAll)
	lines, _, _, ok = layoutEclTextUnits(a.Text, a.Units, 0, 0, 0, 19, 9)
	if !ok || len(lines) != 2 || string(lines[0].Text) != "亞歷山大•威廉(Alexander" || string(lines[1].Text) != "Williams)到了。" {
		t.Fatalf("wide unit: %+v", lines)
	}
}

func TestEclNameTiersFallback(t *testing.T) {
	// Window of 10 columns × 2 rows.
	entry := func(s string, clear bool, col, row uint8) EclTextEntry {
		e := eclEntry(s, clear, col, row)
		e.Left, e.Right, e.Top, e.Bottom = 1, 10, 17, 18
		return e
	}
	finish := func(w *EclTextWatcher, e EclTextEntry) {
		w.ObserveInstruction(e.Return, e.SS, e.SP+eclTextReturnDelta)
	}
	names := testNames(t)

	// Tier all fits.
	w := NewEclTextWatcher(eclFixture(t, "HI BUCK", "巴克來。"))
	w.SetNames(names)
	w.ObserveEntry(entry("HI BUCK", true, 1, 17))
	if p := w.Page(); p == nil || len(p.Lines) != 1 || string(p.Lines[0].Text) != "巴克(BUCK)來。" || w.Stats.NameFirstOnly+w.Stats.NameUnannotated != 0 {
		t.Fatalf("tier all: %+v %+v", p, w.Stats)
	}

	// "巴克(BUCK)說巴克(BUCK)好。" does not fit 2×10; first-only does.
	w = NewEclTextWatcher(eclFixture(t, "BUCK TWICE", "巴克說巴克好巴克。"))
	w.SetNames(names)
	w.ObserveEntry(entry("BUCK TWICE", true, 1, 17))
	p := w.Page()
	if p == nil || w.Stats.NameFirstOnly != 1 || w.Stats.Overflows != 0 {
		t.Fatalf("first-only: %+v", w.Stats)
	}
	fa := names.Annotate("巴克說巴克好巴克。", "ecl.t.00", NameCaseUpper, NameTierFirst)
	want, er, ec, _ := layoutEclTextUnits(fa.Text, fa.Units, 17, 1, 1, 10, 18)
	if len(p.Lines) != len(want) || p.endRow != er || p.endCol != ec {
		t.Fatalf("first-only layout %+v", p)
	}

	// Continuation: the retries start from the page's end state, and the
	// page keeps its top, start column and earlier lines.  Window 10×3.
	tall := func(s string, clear bool, col, row uint8) EclTextEntry {
		e := entry(s, clear, col, row)
		e.Bottom = 19
		return e
	}
	w = NewEclTextWatcher(eclFixture(t, "OK", "好，", "WILMA TALKS", "威瑪說威瑪好。"))
	w.SetNames(names)
	w.ObserveEntry(tall("OK", true, 1, 17))
	finish(w, tall("OK", true, 1, 17))
	w.ObserveEntry(tall("WILMA TALKS", false, 3, 17))
	p = w.Page()
	// Tier all needs a fourth row; first-only fits in three.
	if p == nil || w.Stats.NameFirstOnly != 1 || p.Top != 17 || p.TopCol != 1 || len(p.Lines) != 3 {
		t.Fatalf("continuation: %+v %+v", p, w.Stats)
	}
	if string(p.Lines[0].Text) != "好，" || string(p.Lines[1].Text) != "威瑪(WILMA)說" || p.Lines[1].Row != 18 ||
		string(p.Lines[2].Text) != "威瑪好。" || p.endRow != 19 || p.endCol != 5 {
		t.Fatalf("continuation lines %+v end %d,%d", p.Lines, p.endRow, p.endCol)
	}

	// Down to no annotation.
	w = NewEclTextWatcher(eclFixture(t, "LONG", "一二三四五六七巴克八九十一二三。"))
	w.SetNames(names)
	w.ObserveEntry(entry("LONG", true, 1, 17))
	if w.Page() == nil || w.Stats.NameUnannotated != 1 || w.Stats.NameFirstOnly != 0 {
		t.Fatalf("none: %+v", w.Stats)
	}

	// All three overflow: the spec-027 overflow, not a downgrade.
	w = NewEclTextWatcher(eclFixture(t, "HUGE", strings.Repeat("字", 19)+"巴克"))
	w.SetNames(names)
	w.ObserveEntry(entry("HUGE", true, 1, 17))
	if w.Page() != nil || w.Stats.Overflows != 1 || w.Stats.NameFirstOnly+w.Stats.NameUnannotated != 0 {
		t.Fatalf("overflow: %+v", w.Stats)
	}

	// Engine fallback text is a narrow family: never annotated.
	w = NewEclTextWatcher(eclFixture(t, "X", "巴克"))
	w.SetNames(names)
	w.ObserveEntry(entry("X", true, 1, 17))
	if p := w.Page(); p == nil || string(p.Lines[0].Text) != "巴克(BUCK)" {
		t.Fatalf("catalog hit %+v", p)
	}
	finish(w, entry("X", true, 1, 17))
	w.ObserveEntry(entry("巴克", false, 1, 18)) // passthrough of original bytes
	if p := w.Page(); p == nil || p.Keys[len(p.Keys)-1] != "passthrough" || string(p.Lines[len(p.Lines)-1].Text) != "巴克" {
		t.Fatalf("passthrough annotated %+v", p)
	}
}

func TestLogbookNameTiers(t *testing.T) {
	names := testNames(t)
	// 36 × 19 × 3 = 2052 cells.  With every annotation the body needs a
	// fourth page; first-only fits.
	body := strings.Repeat("巴克", 300) + strings.Repeat("字", 2052-600-10)
	panel := "key\ttranslation\tsource\nlogbook.panel.title\t手札第 {0} 則：{1}\tx\nlogbook.panel.page\t第 {0}／{1} 頁\tx\n"
	// Title "吉爾伯特與亞歷山大•威廉" + prefix "手札第 9 則：" (8): tier all is 43 cells,
	// first-only the same, none 20 — chosen independently of the body.
	data := "key\ttranslation\tsource\nlogbook.9\t" + body + "\tx\nlogbook.9.title\t吉爾伯特與亞歷山大•威廉\tx\n" +
		"logbook.10\t巴克與威瑪。\tx\nlogbook.10.title\t巴克\tx\n"
	c, err := LoadLogbookCatalog([]byte(data), names)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLogbookPanelText([]byte(panel)); err != nil {
		t.Fatal(err)
	}
	e := c.entries[9]
	if e.BodyTier != NameTierFirst || len(e.Pages) != 3 || !strings.HasPrefix(e.Pages[0][0], "巴克(Buck)巴克巴克") {
		t.Fatalf("body tier %v pages %d", e.BodyTier, len(e.Pages))
	}
	if e.TitleTier != NameTierNone || e.Title != "吉爾伯特與亞歷山大•威廉" {
		t.Fatalf("title %v %q", e.TitleTier, e.Title)
	}
	e = c.entries[10]
	if e.BodyTier != NameTierAll || e.Pages[0][0] != "巴克(Buck)與威瑪(Wilma)。" || e.TitleTier != NameTierAll || e.Title != "巴克(Buck)" {
		t.Fatalf("entry 10 %+v", e)
	}
	// A title too wide even without annotation fails static validation.
	if _, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.9\t正文。\tx\nlogbook.9.title\t"+strings.Repeat("長", 40)+"\tx\n"), names); err == nil {
		t.Fatal("over-wide title accepted")
	}
	c2, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.9\t正文。\tx\nlogbook.9.title\t"+strings.Repeat("長", 31)+"\tx\n"), names)
	if err != nil {
		t.Fatal(err) // "9: " + 31 = 34 with the default template
	}
	if err := c2.LoadLogbookPanelText([]byte(panel)); err == nil {
		t.Fatal("title over 38 cells after the panel template accepted") // 8 + 31 = 39
	}
}

func TestLogbookOverlayTitleGuard(t *testing.T) {
	c, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.5\t正文。\tx\nlogbook.5.title\t短\tx\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Bypass load-time selection to exercise the draw-time guard.
	e := c.entries[5]
	e.Title = strings.Repeat("長", 40)
	c.entries[5] = e
	w := NewLogbookWatcher(c, nil)
	w.ObserveEntry([]byte(" as logbook entry 5."), 1, 17, 38, 22)
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, r := range "長正文。:5 " {
		font.Glyphs[r] = make([]byte, 32)
	}
	o, err := NewLogbookOverlay(font, 2)
	if err != nil {
		t.Fatal(err)
	}
	o.Sync(w, [256][3]uint8{})
	if o.TitleErrors != 1 || o.LastError == "" {
		t.Fatalf("guard %d %q", o.TitleErrors, o.LastError)
	}
	s := o.layer.Stamps[0]
	if s.Cells != len(s.Text) || string(s.Text) != "5: "+strings.Repeat("長", 40) {
		t.Fatalf("title truncated: %d %q", s.Cells, string(s.Text))
	}
}

// Static scan of the formal catalog (needs the Buck repo): every title fits
// and every body fits in three pages at some tier.
func TestFormalLogbookNameTiers(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT not set")
	}
	names, err := loadNameGlossary(filepath.Join(root, "text"))
	if err != nil || names == nil {
		t.Fatalf("glossary %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "text", "logbook.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadLogbookCatalog(b, names)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := os.ReadFile(filepath.Join(root, "text", "logbook-panel.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLogbookPanelText(pb); err != nil {
		t.Fatal(err)
	}
	for n, e := range c.entries {
		if w := len([]rune(fillLogbook(c.titleFmt, strconv.Itoa(n), e.Title))); w > logbookTitleCells {
			t.Fatalf("entry %d title %d cells", n, w)
		}
		if len(e.Pages) == 0 || len(e.Pages) > logbookMaxPages {
			t.Fatalf("entry %d pages %d", n, len(e.Pages))
		}
	}
}
