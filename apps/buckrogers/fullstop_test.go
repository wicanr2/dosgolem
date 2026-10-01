package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// Buck repo spec 047 §3.2: the period the original prints as a call of its
// own is drawn as 。 for zh-TW, zh-CN and ja; ko and the test language zz
// are the controls.

func eclFixtureLang(t *testing.T, lang string, pairs ...string) *EclTextCatalog {
	t.Helper()
	ev := "event_key\toriginal_length\toriginal_sha256\tsources\n"
	tr := "key\ttranslation\tsource\n"
	for i := 0; i+1 < len(pairs); i += 2 {
		k := fmt.Sprintf("ecl.t.%02d", i/2)
		ev += fmt.Sprintf("%s\t%d\t%x\tECL1:0:%d\n", k, len(pairs[i]), sha256.Sum256([]byte(pairs[i])), i)
		if pairs[i+1] != "" {
			tr += fmt.Sprintf("%s\t%s\tecl-batch-editorial\n", k, pairs[i+1])
		}
	}
	c, err := LoadEclTextCatalogLang([]byte(ev), []byte(tr), lang)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type fsStep struct {
	orig  string
	clear bool
	col   uint8
}

// fsRun plays the steps on one row of a window with the given width (in
// columns, two half units each) and bottom row, like the deck prompt:
// "DECK " (translated), a number and a period (both passthrough).
func fsRun(t *testing.T, lang string, left, right, bottom uint8, pairs []string, steps []fsStep) *EclTextWatcher {
	t.Helper()
	w := NewEclTextWatcher(eclFixtureLang(t, lang, pairs...))
	for _, s := range steps {
		e := eclEntry(s.orig, s.clear, s.col, 17)
		e.Left, e.Right, e.Bottom = left, right, bottom
		w.ObserveEntry(e)
		eclSpaceRet(w)
	}
	return w
}

func fsLines(w *EclTextWatcher) string {
	p := w.Page()
	if p == nil {
		return "<no page>"
	}
	s := ""
	for _, l := range p.Lines {
		s += fmt.Sprintf("r%d.%d:%s|", l.Row, l.Col, string(l.Text))
	}
	return s
}

func TestEclFullStopLoneCall(t *testing.T) {
	pairs := []string{"DECK ", "甲板"}
	for _, period := range []string{".", ". "} {
		steps := []fsStep{{"DECK ", true, 1}, {"5", false, 6}, {period, false, 7}}
		ctl := fsRun(t, LangTest, 1, 38, 22, pairs, steps)
		for _, lang := range []string{LangZhTW, LangZhCN, LangJa, ""} {
			w := fsRun(t, lang, 1, 38, 22, pairs, steps)
			if got := rowText(w.Page(), 17); got != "甲板5。" {
				t.Errorf("%q %q：%q，應為 甲板5。", lang, period, got)
			}
			if w.Stats.FullStop != 1 || w.Stats.FullStopDropped != 0 {
				t.Errorf("%q %q：Stats %+v", lang, period, w.Stats)
			}
			// The call counts as before: two misses taken as passthrough, one hit.
			a, b := w.Stats, ctl.Stats
			a.FullStop, a.FullStopDropped = 0, 0
			if a != b {
				t.Errorf("%q %q：計數 %+v 與對照 %+v 不同", lang, period, a, b)
			}
			if k := w.Page().Keys; len(k) != 3 || k[2] != "passthrough" {
				t.Errorf("%q %q：key %v", lang, period, k)
			}
		}
		// Korean keeps the original period, like the control.
		for _, lang := range []string{LangKo, LangTest} {
			w := fsRun(t, lang, 1, 38, 22, pairs, steps)
			if got, want := fsLines(w), fsLines(ctl); got != want {
				t.Errorf("%q %q：%q，應與對照 %q 相同", lang, period, got, want)
			}
			if w.Stats.FullStop != 0 || w.Stats.FullStopDropped != 0 {
				t.Errorf("%q %q：Stats %+v", lang, period, w.Stats)
			}
		}
	}
}

func TestEclFullStopNeedsAPageOfOurs(t *testing.T) {
	pairs := []string{"DECK ", "甲板"}
	for _, lang := range []string{LangZhTW, LangJa} {
		// The text before is untranslated English: nothing of ours in the
		// window, the original period stays.
		w := fsRun(t, lang, 1, 38, 22, pairs, []fsStep{{"UNKNOWN", true, 1}, {".", false, 8}})
		if w.Page() != nil || w.Stats.FullStop != 0 {
			t.Errorf("%s：未譯文字後的句點不應處理：%+v %+v", lang, w.Page(), w.Stats)
		}
		// A fresh call clears the page of the window first: no page, no change.
		w = fsRun(t, lang, 1, 38, 22, pairs, []fsStep{{"DECK ", true, 1}, {".", true, 1}})
		if w.Page() != nil || w.Stats.FullStop != 0 {
			t.Errorf("%s：fresh 的句點不應處理：%+v %+v", lang, w.Page(), w.Stats)
		}
	}
}

func TestEclFullStopPlayerNameStays(t *testing.T) {
	// A player whose name is a period: the player-name branch misses (no
	// transliteration) and falls into passthrough; that is not the original's
	// lone period.
	party, _ := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "."}), testDS)
	w := eclPlayerWatcher(t)
	w.catalog.lang = LangZhTW
	w.ObserveEntry(eclPlayerEntry("UNUSED FIXTURE", true, 1, 17, nil))
	eclSpaceRet(w)
	w.ObserveEntry(eclPlayerEntry(".", false, 8, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if w.Stats.FullStop != 0 || lastLine(w.Page()) != "." {
		t.Errorf("玩家名 . 不應處理：%q %+v", lastLine(w.Page()), w.Stats)
	}
}

func TestEclFullStopRoom(t *testing.T) {
	pairs := []string{"A", "甲", "B", "甲a", "C", "甲乙"}
	type tc struct {
		name              string
		first             string
		period            string
		bottom            uint8
		wantFullStop      int
		wantDropped       int
		wantText17        string // "" for do not check (compared with the control)
		wantSameAsControl bool
	}
	// Window of two columns: 4 half units wide.
	cases := []tc{
		{"剩 2 單位：放得下", "A", ".", 17, 1, 0, "甲。", false},
		{"剩 2 單位：. 加空白", "A", ". ", 17, 1, 0, "甲。", false},
		{"剩 1 單位：退回原文", "B", ".", 17, 1, 1, "", true},
		{"剩 1 單位：退回原文，下一列", "B", ".", 18, 1, 1, "", true},
		{"剩 0 單位：換列的結果等於改動前", "C", ".", 18, 1, 1, "", true},
		{"剩 0 單位且末列：與改動前相同", "C", ".", 17, 1, 1, "", true},
	}
	for _, c := range cases {
		steps := []fsStep{{c.first, true, 1}, {c.period, false, 2}}
		ctl := fsRun(t, LangTest, 1, 2, c.bottom, pairs, steps)
		w := fsRun(t, LangZhTW, 1, 2, c.bottom, pairs, steps)
		if w.Stats.FullStop != c.wantFullStop || w.Stats.FullStopDropped != c.wantDropped {
			t.Errorf("%s：Stats %+v", c.name, w.Stats)
		}
		if c.wantText17 != "" {
			if got := rowText(w.Page(), 17); got != c.wantText17 {
				t.Errorf("%s：%q，應為 %q", c.name, got, c.wantText17)
			}
		}
		if c.wantSameAsControl {
			if got, want := fsLines(w), fsLines(ctl); got != want {
				t.Errorf("%s：%q，應與未啟用語言相同 %q", c.name, got, want)
			}
			if w.Stats.Overflows != ctl.Stats.Overflows {
				t.Errorf("%s：溢出 %d，對照 %d", c.name, w.Stats.Overflows, ctl.Stats.Overflows)
			}
		}
	}
}

// A call that is not exactly one period, and every call of a sequence
// without a lone period, give the page and the counters of the control.
func TestEclFullStopGate(t *testing.T) {
	pairs := []string{"DECK ", "甲板", " HIT", "命中。"}
	sequences := [][]fsStep{
		{{"DECK ", true, 1}, {"5.", false, 6}},
		{{"DECK ", true, 1}, {"...", false, 6}},
		{{"DECK ", true, 1}, {"!", false, 6}, {"?", false, 7}},
		{{"DECK ", true, 1}, {"x.", false, 6}, {" HIT", false, 8}},
		{{"DECK ", true, 1}, {"  .", false, 6}},
		{{"DECK ", true, 1}, {" .", false, 6}},
	}
	for i, steps := range sequences {
		ctl := fsRun(t, LangTest, 1, 38, 22, pairs, steps)
		for _, lang := range []string{LangZhTW, LangZhCN, LangJa, LangKo} {
			w := fsRun(t, lang, 1, 38, 22, pairs, steps)
			if got, want := fsLines(w), fsLines(ctl); got != want {
				t.Errorf("序列 %d %s：%q，應與對照 %q 相同", i, lang, got, want)
			}
			if w.Stats != ctl.Stats {
				t.Errorf("序列 %d %s：Stats %+v 與對照 %+v 不同", i, lang, w.Stats, ctl.Stats)
			}
		}
	}
}

func TestEclCatalogLangNormalised(t *testing.T) {
	pairs := []string{"A", "甲"}
	for lang, want := range map[string]bool{"": true, LangZhTW: true, LangZhCN: true, LangJa: true, LangKo: false, LangTest: false} {
		if got := eclFixtureLang(t, lang, pairs...).fullStop(); got != want {
			t.Errorf("lang %q：fullStop %v，應為 %v", lang, got, want)
		}
	}
	if !eclFixture(t, pairs...).fullStop() {
		t.Error("LoadEclTextCatalog 預設應為 zh-TW")
	}
	var nilCatalog *EclTextCatalog
	if nilCatalog.fullStop() {
		t.Error("nil catalog")
	}
}
