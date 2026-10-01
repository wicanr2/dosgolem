package buckrogers

import (
	"strings"
	"testing"
)

// Buck repo spec 046 §3.4: the space at the start of a continuing call in
// the Korean (word-level) profile.

func eclSpaceRet(w *EclTextWatcher) {
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta)
}

// eclRow joins what the page shows on a row, in the order drawn; a call
// that starts after the previous one is joined without a gap.
func eclRow(p *EclTextPage, row uint8) string {
	var b strings.Builder
	for _, l := range p.Lines {
		if l.Row == row {
			b.WriteString(string(l.Text))
		}
	}
	return b.String()
}

func eclSpaceWatcher(t *testing.T, prof *LayoutProfile, pairs ...string) *EclTextWatcher {
	t.Helper()
	w := NewEclTextWatcher(eclFixture(t, pairs...))
	w.SetLayout(prof)
	return w
}

func TestEclCallStartSpaceKo(t *testing.T) {
	pairs := []string{
		"DECK", "갑판", "DECK ", "갑판", "DONE.", "끝났다.", " WHERE DO YOU GO?", "어디로 가겠습니까?", "WHERE DO YOU GO?", "어디로 가겠습니까?",
		" AND YOU", ", 일지 기록", " THIS ROOM", "이 방은", " IS HIT", "은(는) 공격받았다", " GOES", "간다", "NAME", "",
	}
	type step struct {
		orig  string
		clear bool
		col   uint8
		row   uint8
	}
	cases := []struct {
		name  string
		prof  *LayoutProfile
		steps []step
		want  string
	}{
		{"原文以空白開頭：補空白", layoutKo, []step{{"DECK", true, 1, 17}, {" WHERE DO YOU GO?", false, 4, 17}}, "갑판 어디로 가겠습니까?"},
		{"nil layout 不補", nil, []step{{"DECK", true, 1, 17}, {" WHERE DO YOU GO?", false, 4, 17}}, "갑판어디로 가겠습니까?"},
		{"日文 layout 不補", layoutJa, []step{{"DECK", true, 1, 17}, {" WHERE DO YOU GO?", false, 4, 17}}, "갑판어디로 가겠습니까?"},
		{"詞級但原文沒有空白：不補", layoutKo, []step{{"DECK", true, 1, 17}, {"WHERE DO YOU GO?", false, 4, 17}}, "갑판어디로 가겠습니까?"},
		{"前一次呼叫以空白結尾（欠一個空白）：補", layoutKo, []step{{"DECK ", true, 1, 17}, {"WHERE DO YOU GO?", false, 4, 17}}, "갑판 어디로 가겠습니까?"},
		{"標點記號開頭：黏", layoutKo, []step{{"DECK", true, 1, 17}, {" AND YOU", false, 4, 17}}, "갑판, 일지 기록"},
		{"句點後的指示詞：補", layoutKo, []step{{"DONE.", true, 1, 17}, {" THIS ROOM", false, 4, 17}}, "끝났다. 이 방은"},
		{"原版游標在左欄的換列：不補（列首不留空白）", layoutKo, []step{{"DECK", true, 1, 17}, {" WHERE DO YOU GO?", false, 1, 18}}, ""},
	}
	for _, tc := range cases {
		w := eclSpaceWatcher(t, tc.prof, pairs...)
		var p *EclTextPage
		for _, s := range tc.steps {
			w.ObserveEntry(eclEntry(s.orig, s.clear, s.col, s.row))
			eclSpaceRet(w)
			p = w.Page()
			if p == nil {
				t.Fatalf("%s：%q 沒有命中", tc.name, s.orig)
			}
		}
		if tc.want == "" {
			// New row: no line of the page starts with a space.
			for _, l := range p.Lines {
				if len(l.Text) > 0 && l.Text[0] == ' ' {
					t.Errorf("%s：列首有空白 %+v", tc.name, p.Lines)
				}
			}
			continue
		}
		if got := eclRow(p, 17); got != tc.want {
			t.Errorf("%s：%q，應為 %q", tc.name, got, tc.want)
		}
	}
}

// A continuation after a call we could not translate (the English stays on
// screen): the previous character counts as a Latin letter.
func TestEclCallStartSpaceAfterUntranslated(t *testing.T) {
	pairs := []string{"NAME", "", " IS HIT", "은(는) 공격받았다", " GOES", "간다"}
	for _, tc := range []struct{ orig, want string }{
		{" IS HIT", "은(는) 공격받았다"}, // particle after the English name sticks
		{" GOES", " 간다"},          // a word gets its space
	} {
		w := eclSpaceWatcher(t, layoutKo, pairs...)
		w.ObserveEntry(eclEntry("NAME", true, 1, 17))
		eclSpaceRet(w)
		if w.Page() != nil {
			t.Fatal("未譯呼叫不應建頁")
		}
		w.ObserveEntry(eclEntry(tc.orig, false, 6, 17))
		p := w.Page()
		if p == nil || len(p.Lines) == 0 || string(p.Lines[0].Text) != tc.want {
			t.Errorf("%q：%+v，應為 %q", tc.orig, p, tc.want)
		}
	}
}

// The space is never the reason a window turns into English: without it the
// text is laid out again (SpaceDropped).
func TestEclCallStartSpaceDropped(t *testing.T) {
	pairs := []string{"A", "가", " B", "나"}
	w := eclSpaceWatcher(t, layoutKo, pairs...)
	e1 := eclEntry("A", true, 1, 17)
	e1.Right, e1.Bottom = 2, 17 // 4 half units, one row
	w.ObserveEntry(e1)
	eclSpaceRet(w)
	e2 := eclEntry(" B", false, 2, 17)
	e2.Right, e2.Bottom = 2, 17
	w.ObserveEntry(e2)
	p := w.Page()
	if p == nil || eclRow(p, 17) != "가나" {
		t.Fatalf("空白放不下時應去掉：%+v", p)
	}
	if w.Stats.SpaceDropped != 1 || w.Stats.Overflows != 0 {
		t.Errorf("Stats = %+v", w.Stats)
	}
	// With room for the space it is kept and nothing is counted.
	w2 := eclSpaceWatcher(t, layoutKo, pairs...)
	w2.ObserveEntry(eclEntry("A", true, 1, 17))
	eclSpaceRet(w2)
	w2.ObserveEntry(eclEntry(" B", false, 2, 17))
	if eclRow(w2.Page(), 17) != "가 나" || w2.Stats.SpaceDropped != 0 {
		t.Errorf("有空間時應補空白：%q %+v", eclRow(w2.Page(), 17), w2.Stats)
	}
}

// The word-level gate: the same continuation in the other profiles and in a
// watcher without a profile gives the pre-046 page and counters.
func TestEclCallStartSpaceOtherProfilesUnchanged(t *testing.T) {
	pairs := []string{"DECK", "갑판", " WHERE", "어디로"}
	var base *EclTextPage
	for i, prof := range []*LayoutProfile{nil, layoutJa, layoutKoChars} {
		w := eclSpaceWatcher(t, prof, pairs...)
		w.ObserveEntry(eclEntry("DECK", true, 1, 17))
		eclSpaceRet(w)
		w.ObserveEntry(eclEntry(" WHERE", false, 4, 17))
		p := w.Page()
		if got := eclRow(p, 17); got != "갑판어디로" {
			t.Errorf("profile %d：%q", i, got)
		}
		if w.Stats.SpaceDropped != 0 || p.owedSpace || p.lastRune != 0 {
			t.Errorf("profile %d：不應設定欠空白狀態：%+v %v %q", i, w.Stats, p.owedSpace, p.lastRune)
		}
		if base == nil {
			base = p
		} else if !equalLines(base.Lines, p.Lines) {
			t.Errorf("profile %d 的列與 nil 不同", i)
		}
	}
}

func equalLines(a, b []EclTextLine) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Row != b[i].Row || a[i].Col != b[i].Col || string(a[i].Text) != string(b[i].Text) {
			return false
		}
	}
	return true
}
