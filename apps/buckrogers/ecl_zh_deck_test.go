package buckrogers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Buck repo spec 048: zh-TW and zh-CN put a space between 甲板 and the number
// the original prints as a call of its own.

type zdOpts struct {
	lang          string
	prof          *LayoutProfile
	left, right   uint8
	bottom        uint8
	names, engine bool
}

func zdRun(t *testing.T, o zdOpts, pairs []string, steps []fsStep) *EclTextWatcher {
	t.Helper()
	if o.right == 0 {
		o.right = 38
	}
	if o.left == 0 {
		o.left = 1
	}
	if o.bottom == 0 {
		o.bottom = 22
	}
	w := NewEclTextWatcher(eclFixtureLang(t, o.lang, pairs...))
	w.SetLayout(o.prof)
	if o.names {
		w.SetNames(testNames(t))
	}
	for _, s := range steps {
		e := eclEntry(s.orig, s.clear, s.col, 17)
		e.Left, e.Right, e.Bottom = o.left, o.right, o.bottom
		w.ObserveEntry(e)
		eclSpaceRet(w)
	}
	return w
}

var zhLangs = []string{LangZhTW, LangZhCN, ""}

var deckPairs = []string{"DECK ", "甲板", " WHERE DO YOU GO?", "你們要往哪裡去？"}

func TestZhDeckSpaceBasic(t *testing.T) {
	for _, lang := range zhLangs {
		o := zdOpts{lang: lang}
		// four calls: DECK, number, period, question
		w := zdRun(t, o, deckPairs, []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {".", false, 7}, {" WHERE DO YOU GO?", false, 8}})
		if got := rowText(w.Page(), 17); got != "甲板 5。你們要往哪裡去？" {
			t.Errorf("%q：%q", lang, got)
		}
		// the number and the call after it move one unit right: space 1 unit, 。 follows the 5
		ctl := zdRun(t, zdOpts{lang: LangTest}, []string{"DECK ", "甲板", "5", " 5", ".", "。", " WHERE DO YOU GO?", "你們要往哪裡去？"},
			[]fsStep{{"DECK ", true, 1}, {"5", false, 6}, {".", false, 7}, {" WHERE DO YOU GO?", false, 8}})
		if fsLines(w) != fsLines(ctl) || w.Page().endRow != ctl.Page().endRow || w.Page().endCol != ctl.Page().endCol {
			// ctl's pairs map "5" twice: the first pair wins, so build the equivalence below
			_ = ctl
		}
		if w.Stats.SpaceDropped != 0 || w.Stats.Overflows != 0 {
			t.Errorf("%q：Stats %+v", lang, w.Stats)
		}
		// three calls, no question (ecl.2.32.09000 type)
		w = zdRun(t, o, deckPairs, []fsStep{{"DECK ", true, 1}, {"8", false, 6}, {".", false, 7}})
		if got := rowText(w.Page(), 17); got != "甲板 8。" {
			t.Errorf("%q 三次呼叫：%q", lang, got)
		}
	}
}

// The space is the leading space of the number call's text; every later
// column moves one unit right and nothing else about the page changes: the
// test language with a catalog that maps 5 to " 5" draws the same lines.
func TestZhDeckSpaceEquivalence(t *testing.T) {
	steps := []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {".", false, 7}, {" WHERE DO YOU GO?", false, 8}}
	for _, lang := range []string{LangZhTW, LangZhCN} {
		w := zdRun(t, zdOpts{lang: lang}, deckPairs, steps)
		ctl := zdRun(t, zdOpts{lang: LangTest}, append(append([]string{}, deckPairs...), "5", " 5", ".", "。"), steps)
		if got, want := fsLines(w), fsLines(ctl); got != want {
			t.Errorf("%s：%q，等價對照 %q", lang, got, want)
		}
		if w.Page().endRow != ctl.Page().endRow || w.Page().endCol != ctl.Page().endCol {
			t.Errorf("%s：結束位置 %d,%d 對照 %d,%d", lang, w.Page().endRow, w.Page().endCol, ctl.Page().endRow, ctl.Page().endCol)
		}
	}
}

func TestZhDeckSpaceNotApplied(t *testing.T) {
	type tc struct {
		name  string
		pairs []string
		steps []fsStep
		want  string // text of row 17 ("" = no check)
	}
	cases := []tc{
		{"前一次以編號結尾", []string{"NO ", "編號"}, []fsStep{{"NO ", true, 1}, {"5", false, 6}}, "編號5"},
		{"前一次以句號結尾", []string{"A ", "。"}, []fsStep{{"A ", true, 1}, {"5", false, 6}}, "。5"},
		{"前一次是拉丁字母", []string{"A ", "NEO"}, []fsStep{{"A ", true, 1}, {"5", false, 6}}, "NEO5"},
		{"前一次以第結尾", []string{"AT ", "位於第"}, []fsStep{{"AT ", true, 1}, {"5", false, 6}}, "位於第5"},
		{"本次以字母起首", []string{"DECK ", "甲板"}, []fsStep{{"DECK ", true, 1}, {"A", false, 6}}, "甲板A"},
		{"本次以中文起首", []string{"DECK ", "甲板", " GO", "去"}, []fsStep{{"DECK ", true, 1}, {" GO", false, 6}}, "甲板去"},
		{"第二個數字呼叫", deckPairs, []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {"6", false, 8}}, "甲板 56"},
		{"譯文自帶尾端空白", []string{"DECK ", "甲板 "}, []fsStep{{"DECK ", true, 1}, {"5", false, 7}}, "甲板 5"},
		{"甲板後是原版自畫的空白", []string{"DECK ", "甲板"}, []fsStep{{"DECK ", true, 1}, {" ", false, 6}, {"5", false, 7}}, "甲板 5"},
		{"起點在左欄（換列）", deckPairs, []fsStep{{"DECK ", true, 1}, {"5", false, 1}}, ""},
		{"fresh 呼叫", deckPairs, []fsStep{{"DECK ", true, 1}, {"5", true, 6}}, ""},
		{"p == nil 的非 fresh 分支", deckPairs, []fsStep{{"UNKNOWN", true, 1}, {"5", false, 8}}, ""},
	}
	for _, lang := range zhLangs {
		for _, c := range cases {
			w := zdRun(t, zdOpts{lang: lang}, c.pairs, c.steps)
			if c.name == "p == nil 的非 fresh 分支" || c.name == "fresh 呼叫" {
				// no page of ours before the call: the original shows, nothing is drawn
				if w.Page() != nil {
					t.Errorf("%q %s：不應有頁面", lang, c.name)
				}
				continue
			}
			p := w.Page()
			if p == nil {
				t.Errorf("%q %s：沒有頁面", lang, c.name)
				continue
			}
			for _, l := range p.Lines {
				if c.name == "起點在左欄（換列）" && len(l.Text) > 0 && l.Text[0] == ' ' {
					t.Errorf("%q %s：列首有空白 %+v", lang, c.name, p.Lines)
				}
			}
			if c.want != "" {
				if got := rowText(p, 17); got != c.want {
					t.Errorf("%q %s：%q，應為 %q", lang, c.name, got, c.want)
				}
			}
			if w.Stats.SpaceDropped != 0 {
				t.Errorf("%q %s：SpaceDropped %d", lang, c.name, w.Stats.SpaceDropped)
			}
		}
	}
}

// Rows: 甲板 split over two rows is not recognised; 甲板 at the end of the last of
// several rows is.
func TestZhDeckSpaceRows(t *testing.T) {
	// window of 4 columns (8 units): 你好嗎你好甲板 = 14 units, wraps after 8
	pairs := []string{"DECK ", "你好嗎你好甲板"}
	w := zdRun(t, zdOpts{lang: LangZhTW, right: 4, bottom: 19}, pairs, []fsStep{{"DECK ", true, 1}, {"5", false, 3}})
	if got := rowText(w.Page(), 18); got != "好甲板 5" {
		t.Errorf("多列：row 18 %q，rows %q %q", got, rowText(w.Page(), 17), rowText(w.Page(), 19))
	}
	// 甲 and 板 on different rows: the last row is 板
	pairs = []string{"DECK ", "你好嗎甲板"}
	w = zdRun(t, zdOpts{lang: LangZhTW, right: 4, bottom: 19}, pairs, []fsStep{{"DECK ", true, 1}, {"5", false, 3}})
	if got := rowText(w.Page(), 18); got != "板5" || rowText(w.Page(), 17) != "你好嗎甲" {
		t.Errorf("拆開：row 17 %q row 18 %q", rowText(w.Page(), 17), rowText(w.Page(), 18))
	}
}

func TestZhDeckSpacePlayerNameStays(t *testing.T) {
	noShrink(t) // spec 056 §5.8: this test asserts the step-down order of spec 036/038/045 without shrinking
	party, _ := ReadPartySnapshot(partyMem(2, partyRec{seg: 0x5747, off: 2, name: "CELESTE"}, partyRec{seg: 0x5747, off: 0x200, name: "5BOB"}), testDS)
	ctxCeleste := eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})
	run := func(name string, right uint8, tr fakeTranslit, bottom uint8) (*EclTextWatcher, string) {
		w := NewEclTextWatcher(eclFixtureLang(t, LangZhTW, "DECK ", "甲板"))
		w.SetPlayerNames(NewPlayerNames(tr, nil))
		e := eclEntry("DECK ", true, 1, 17)
		e.Right, e.Bottom = right, bottom
		w.ObserveEntry(e)
		eclSpaceRet(w)
		e2 := eclPlayerEntry(name, false, 6, 17, ctxCeleste)
		e2.Right, e2.Bottom = right, bottom
		w.ObserveEntry(e2)
		return w, rowText(w.Page(), 17)
	}
	tr := fakeTranslit{"CELESTE": "塞萊絲特"}
	if w, got := run("CELESTE", 38, tr, 22); got != "甲板塞萊絲特(CELESTE)" || w.Stats.SpaceDropped != 0 {
		t.Errorf("完整單元：%q %+v", got, w.Stats)
	}
	if _, got := run("CELESTE", 10, tr, 17); got != "甲板塞萊絲特" {
		t.Errorf("只譯名：%q", got)
	}
	if _, got := run("CELESTE", 6, fakeTranslit{"CELESTE": "塞萊絲特塞萊絲特"}, 17); got != "甲板CELESTE" {
		t.Errorf("英文退路：%q", got)
	}
	// the name starts with a digit and has no transliteration: it falls into
	// passthrough with isPlayer still true
	if w, got := run("5BOB", 38, tr, 22); got != "甲板5BOB" || w.Stats.SpaceDropped != 0 {
		t.Errorf("音譯失敗落入 passthrough：%q %+v", got, w.Stats)
	}
}

func TestZhDeckSpaceCatalogHit(t *testing.T) {
	pairs := []string{"DECK ", "甲板", "RED ", "5 張紅卡、1 張綠卡，以及 2 張藍卡。"}
	steps := []fsStep{{"DECK ", true, 1}, {"RED ", false, 6}}
	for _, names := range []bool{false, true} {
		// wide window: one row
		w := zdRun(t, zdOpts{lang: LangZhTW, names: names}, pairs, steps)
		if got := rowText(w.Page(), 17); got != "甲板 5 張紅卡、1 張綠卡，以及 2 張藍卡。" || w.Stats.SpaceDropped != 0 {
			t.Errorf("names=%v：%q %+v", names, got, w.Stats)
		}
		// a window where the sentence has to wrap: the space still goes in front
		// of the 5 and nothing is dropped
		w = zdRun(t, zdOpts{lang: LangZhTW, names: names, right: 8, bottom: 20}, pairs, steps)
		if got := rowText(w.Page(), 17); !strings.HasPrefix(got, "甲板 5") || w.Stats.SpaceDropped != 0 || w.Stats.Overflows != 0 {
			t.Errorf("names=%v 換列：%q %+v", names, got, w.Stats)
		}
	}
}

func TestZhDeckSpaceEngineText(t *testing.T) {
	w := NewEclTextWatcher(eclFixtureLang(t, LangZhTW, "DECK ", "甲板"))
	eng, err := LoadEngineTextFiles(t, "Grenades", "5 顆手榴彈")
	if err != nil {
		t.Fatal(err)
	}
	w.SetEngine(eng)
	for _, s := range []fsStep{{"DECK ", true, 1}, {"Grenades", false, 6}} {
		e := eclEntry(s.orig, s.clear, s.col, 17)
		w.ObserveEntry(e)
		eclSpaceRet(w)
	}
	p := w.Page()
	if p == nil || rowText(p, 17) != "甲板 5 顆手榴彈" || p.Keys[len(p.Keys)-1] != "engine" {
		t.Errorf("引擎翻譯：%q %v", rowText(p, 17), p)
	}
}

// Narrow remaining widths (spec 048 §3.3).  A window of r columns is 2r units;
// the prefix is 4 units (甲板) or 5 (a甲板).
func TestZhDeckSpaceNarrow(t *testing.T) {
	type tc struct {
		name       string
		prefix     string
		right      uint8
		bottom     uint8
		wantRow17  string
		wantStats  func(s EclTextStats) bool
		withPeriod bool
	}
	cases := []tc{
		{"R=4：空白與 。 都放得下", "甲板", 4, 18, "甲板 5。", func(s EclTextStats) bool { return s.SpaceDropped == 0 && s.FullStopDropped == 0 }, true},
		{"R=3：空白放得下，。 退回半形", "a甲板", 4, 18, "a甲板 5.", func(s EclTextStats) bool { return s.SpaceDropped == 0 && s.FullStopDropped == 1 }, true},
		{"R=1：空白把數字推到下一列，退回不含空白", "a甲板", 3, 18, "a甲板5", func(s EclTextStats) bool { return s.SpaceDropped == 1 && s.Overflows == 0 }, false},
		{"R=2：空白填滿，句點換到下一列", "甲板", 3, 18, "甲板 5", func(s EclTextStats) bool { return s.SpaceDropped == 0 && s.FullStopDropped == 1 }, true},
		{"R=2 且末列：句點放不下，窗口變英文", "甲板", 3, 17, "", func(s EclTextStats) bool { return s.Overflows == 1 }, true},
	}
	for _, c := range cases {
		pairs := []string{"DECK ", c.prefix}
		// cursor columns stay inside the window (a column past the right edge
		// makes the call a fresh one)
		steps := []fsStep{{"DECK ", true, 1}, {"5", false, 2}}
		if c.withPeriod {
			steps = append(steps, fsStep{".", false, 3})
		}
		w := zdRun(t, zdOpts{lang: LangZhTW, right: c.right, bottom: c.bottom}, pairs, steps)
		if !c.wantStats(w.Stats) {
			t.Errorf("%s：Stats %+v", c.name, w.Stats)
		}
		if c.wantRow17 != "" {
			if w.Page() == nil {
				t.Errorf("%s：沒有頁面", c.name)
				continue
			}
			if got := rowText(w.Page(), 17); got != c.wantRow17 {
				t.Errorf("%s：row 17 %q，應為 %q", c.name, got, c.wantRow17)
			}
		}
	}
}

// ja, zz, ko keep the pre-048 page; the expectations are literal strings so
// that a flag opened for every language cannot pass by changing both sides.
func TestZhDeckSpaceOtherLanguagesUnchanged(t *testing.T) {
	steps := []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {".", false, 7}}
	type tc struct {
		name  string
		lang  string
		prof  *LayoutProfile
		tr    string
		want  string
		fstop int
	}
	for _, c := range []tc{
		{"ja", LangJa, nil, "甲板", "甲板5。", 1},
		{"zz", LangTest, nil, "甲板", "甲板5.", 0},
		{"ko 無 profile", LangKo, nil, "갑판", "갑판5.", 0},
		{"ko 字元級", LangKo, layoutKoChars, "갑판", "갑판5.", 0},
		{"ko 詞級（規格 046）", LangKo, layoutKo, "갑판", "갑판 5.", 0},
	} {
		w := zdRun(t, zdOpts{lang: c.lang, prof: c.prof}, []string{"DECK ", c.tr}, steps)
		if got := rowText(w.Page(), 17); got != c.want || w.Stats.FullStop != c.fstop || w.Stats.SpaceDropped != 0 {
			t.Errorf("%s：%q %+v，應為 %q", c.name, got, w.Stats, c.want)
		}
	}
}

// The deck rule adds no page state: lastRune and owedSpace stay unset for
// zh, and the page struct has exactly the fields it had before spec 048.
func TestZhDeckSpaceNoPageState(t *testing.T) {
	w := zdRun(t, zdOpts{lang: LangZhTW}, deckPairs, []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {".", false, 7}})
	if p := w.Page(); p.lastRune != 0 || p.owedSpace || p.lastReading != 0 {
		t.Errorf("zh 的頁面狀態被設定：lastRune %q owedSpace %v lastReading %q", p.lastRune, p.owedSpace, p.lastReading)
	}
	var names []string
	rt := reflect.TypeOf(EclTextPage{})
	for i := 0; i < rt.NumField(); i++ {
		names = append(names, rt.Field(i).Name)
	}
	// lastReading is spec 054's (Korean only) and Lang is spec 057's (the
	// shrink table of the page's language); the deck rule added none.
	want := "Generation Left Top Right Bottom TopCol Background Foreground Lines Keys Gone endRow endCol lastRune owedSpace lastReading Lang"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("EclTextPage 欄位：%s，應為 %s", got, want)
	}
}

func TestZhDeckSpaceLanguageTable(t *testing.T) {
	pairs := []string{"A", "甲"}
	for lang, want := range map[string]bool{"": true, LangZhTW: true, LangZhCN: true, LangJa: false, LangKo: false, LangTest: false} {
		if got := eclFixtureLang(t, lang, pairs...).zhDeckSpace(); got != want {
			t.Errorf("lang %q：zhDeckSpace %v，應為 %v", lang, got, want)
		}
	}
	var nilCatalog *EclTextCatalog
	if nilCatalog.zhDeckSpace() {
		t.Error("nil catalog")
	}
}

// Data gate (spec 048 §4.1): the zh translations that end with 甲板 are the
// three deck prompts the spec was written for; a new one needs a review.
func TestZhDeckSpaceKeysGate(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定：以甲板結尾的譯文 key 閘門未檢查")
	}
	want := "ecl.2.32.03956 ecl.2.32.09000 ecl.2.33.08985"
	for _, lang := range []string{LangZhTW, LangZhCN} {
		b, err := os.ReadFile(filepath.Join(root, "text", "ecl-text."+lang+".tsv"))
		if err != nil {
			t.Fatal(err)
		}
		var keys []string
		for _, line := range strings.Split(string(b), "\n")[1:] {
			f := strings.Split(line, "\t")
			if len(f) >= 2 && strings.HasSuffix(f[1], "甲板") {
				keys = append(keys, f[0])
			}
		}
		if got := strings.Join(keys, " "); got != want {
			t.Errorf("%s：以甲板結尾的 key %q，規格 048 只審過 %q", lang, got, want)
		}
	}
}
