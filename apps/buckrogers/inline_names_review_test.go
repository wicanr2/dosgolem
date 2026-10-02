package buckrogers

import (
	"strings"
	"testing"
)

// Buck repo spec 054: cases the independent review of the implementation found
// the first tests could not tell apart from a wrong implementation.

func TestKoReadingTailMoreCases(t *testing.T) {
	for _, c := range []struct {
		s    string
		want rune
		why  string
	}{
		{"한(A)과(B)", '과', "兩個加註：取最後一個括號之前的音節"},
		{"한(A)과 (B)", 0, "括號前是空白"},
		{"스(A\x7f)", 0, "0x7F 不是可列印字元"},
		{"스(A\x1f)", 0, "0x1F 不是可列印字元"},
		{"스( )", '스', "只有空白的加註：0x20 可列印"},
		{"스(~)", '스', "0x7E 可列印"},
	} {
		if got := koReadingTail(c.s); got != c.want {
			t.Errorf("koReadingTail(%q) = %q，應為 %q（%s）", c.s, got, c.want, c.why)
		}
	}
}

// A table row whose front became wider than the original columns with the
// reading of a name is drawn as before, not dropped (spec 054 §3.3).
func TestTranslatePartyTableRowWideReadingGivesWay(t *testing.T) {
	c := inlineCatalog(t, LangJa, map[string]string{"Hits ": "命中"}, map[string]string{"NEO WARRIOR": "NEO 戦士"}, map[string]string{"Bolt": "ボルト"})
	row := "Hits FLAVIUS  7"
	short := partyReadings(map[string]string{"FLAVIUS": "フラビ"})
	wide := partyReadings(map[string]string{"FLAVIUS": strings.Repeat("フラビウス", 4)})
	plain, pok := c.Translate(row)
	if !pok {
		t.Fatalf("fixture：%q 應能翻譯", row)
	}
	if got, ok, inline := c.TranslateParty(row, short); !ok || inline != 1 || got == plain {
		t.Errorf("短讀音：%q %v %d（現行 %q）", got, ok, inline, plain)
	}
	if got, ok, inline := c.TranslateParty(row, wide); !ok || inline != 0 || got != plain {
		t.Errorf("寬讀音應退回現行輸出：%q %v %d，現行 %q", got, ok, inline, plain)
	}
}

// Template sentences: the mark after a slot is resolved on the filled value,
// and a member's name in a plain slot is replaced like anywhere else.
func TestTranslatePartyTemplates(t *testing.T) {
	frags := map[string]string{"Drop ": "-", " forever? ": "-", "Use antidote on ": "-"}
	tpl := map[string]string{
		"tpl.drop":     "{0}을(를) 영구히 버리겠습니까?",
		"tpl.antidote": "{0}이(가) 해독제를 쓴다",
	}
	c := joiningCatalog(t, LangKo, frags, tpl)
	names := partyReadings(map[string]string{"FLAVIUS": "플라비우스", "ROARKE": "로앙"})
	for _, tc := range []struct {
		in, want string
		inline   int
	}{
		{"Drop Bolt Gun forever? ", "볼트건을 영구히 버리겠습니까?", 0},
		{"Drop Gun Bolt forever? ", "건볼트를 영구히 버리겠습니까?", 0},
		{"Use antidote on FLAVIUS", "플라비우스가 해독제를 쓴다", 1},
		{"Use antidote on ROARKE", "로앙이 해독제를 쓴다", 1},
		{"Use antidote on XYZ", "XYZ이(가) 해독제를 쓴다", 0},
		{"Use antidote on NEO WARRIOR", "NEO 전사가 해독제를 쓴다", 0},
	} {
		got, ok, inline := c.TranslateParty(tc.in, names)
		if !ok || got != tc.want || inline != tc.inline {
			t.Errorf("%q → %q %v %d，應為 %q %d", tc.in, got, ok, inline, tc.want, tc.inline)
		}
		if plain, _ := c.Translate(tc.in); tc.inline == 0 && plain != got {
			t.Errorf("%q：沒有換名時應與 Translate 相同：%q 對 %q", tc.in, got, plain)
		}
	}
}

// The slot is compared exactly: case matters.
func TestTranslatePartyNameCaseSensitive(t *testing.T) {
	c := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	names := partyReadings(map[string]string{"FLAVIUS": "플라비우스"})
	if got, ok, inline := c.TranslateParty("Flavius makes his tactics roll.", names); !ok || inline != 0 || !strings.HasPrefix(got, "Flavius") {
		t.Errorf("大小寫不同不應換名：%q %v %d", got, ok, inline)
	}
}

// --- ECL: wrapped annotation, Chinese-only tier, counters ---------------------

// An annotated name wider than a row breaks at a space inside the annotation;
// the syllable before the "(" is on the row above, and the mark that follows
// still takes it (the last row alone would give none).
func TestEclLastReadingAcrossWrappedAnnotation(t *testing.T) {
	rec := partyRec{seg: 0x57A7, off: 0x0009, name: "NICOLE STEELE", gender: 1}
	party := inlineParty(t, rec)
	w := inlineEclWatcher(t, LangKo, layoutKo, nil, " IS HIT", "이(가) 서서히")
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"NICOLE STEELE": "니콜 스틸"}, nil))
	e := eclPlayerEntry("NICOLE STEELE", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e.Right, e.Top, e.Bottom = 10, 17, 19
	w.ObserveEntry(e)
	eclSpaceRet(w)
	p := w.Page()
	if p == nil || w.Stats.PlayerNames != 1 || w.Stats.PlayerNameChineseOnly != 0 || w.Stats.PlayerNameEnglish != 0 {
		t.Fatalf("fixture：名字應以完整單元畫出：%+v %+v", w.Stats, p)
	}
	last := p.Lines[len(p.Lines)-1]
	if len(p.Lines) < 2 || strings.Contains(string(last.Text), "(") {
		t.Fatalf("fixture：加註應拆成多列且最後一列不含左括號：%+v", p.Lines)
	}
	if p.lastReading != '틸' {
		t.Errorf("lastReading %q，應為 틸", p.lastReading)
	}
	n := eclEntry(" IS HIT", false, 6, 17)
	n.Right, n.Top, n.Bottom = 10, 17, 19
	w.ObserveEntry(n)
	var all []rune
	for _, l := range w.Page().Lines {
		all = append(all, l.Text...)
	}
	if got := string(all); !strings.Contains(got, "이") || strings.Contains(got, "(가)") || w.Stats.MarkersResolved != 1 {
		t.Errorf("틸 之後應取 이：%q %+v", got, w.Stats)
	}
}

// A name drawn as its reading only (the annotation did not fit) leaves the
// reading's last syllable.
func TestEclLastReadingChineseOnlyTier(t *testing.T) {
	noShrink(t) // spec 056 §5.8: this test asserts the step-down order of spec 036/038/045 without shrinking
	party := inlineParty(t, recMarion)
	w := inlineEclWatcher(t, LangKo, layoutKo, nil, " IS HIT", "이(가) 서서히")
	e := eclPlayerEntry("MARION", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e.Right, e.Top, e.Bottom = 6, 17, 18 // 12 units: 마리온(MARION) is 14, 마리온 is 6
	w.ObserveEntry(e)
	eclSpaceRet(w)
	if w.Stats.PlayerNameChineseOnly != 1 || w.Page().lastReading != '온' {
		t.Fatalf("fixture：應只畫讀音，lastReading 為 온：%+v %q", w.Stats, w.Page().lastReading)
	}
	n := eclEntry(" IS HIT", false, 4, 17)
	n.Right, n.Top, n.Bottom = 6, 17, 18
	w.ObserveEntry(n)
	if got := eclRow(w.Page(), 17); got != "마리온이" || w.Stats.MarkersResolved != 1 {
		t.Errorf("%q %+v", got, w.Stats)
	}
}

// Counters that must stay put: a sentence without a member's name that does
// not fit is an ordinary overflow, and a mark that was resolved on a page that
// then does not fit is not counted.
func TestEclInlineCountersStayPut(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	w := inlineEclWatcher(t, LangKo, layoutKo, eng)
	w.ObserveEntry(inlineEntry("MARCUS makes his tactics roll.", true, 1, inlineParty(t, recFlavius), 6))
	if w.Page() != nil || w.Stats.Overflows != 1 || w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 0 || w.Stats.MarkersResolved != 0 {
		t.Errorf("隊員名不在句中的溢位：%+v", w.Stats)
	}
	// The name takes the row; the call that follows does not fit after it.
	party := inlineParty(t, recCeleste)
	v := inlineEclWatcher(t, LangKo, layoutKo, nil, " IS HIT", "이(가) 서서히")
	first := eclPlayerEntry("CELESTE", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	first.Right, first.Top, first.Bottom = 9, 17, 17 // 18 units: 셀레스트(CELESTE) is 17
	v.ObserveEntry(first)
	eclSpaceRet(v)
	if v.Stats.PlayerNames != 1 || v.Stats.PlayerNameChineseOnly != 0 {
		t.Fatalf("fixture：%+v", v.Stats)
	}
	next := eclEntry(" IS HIT", false, 8, 17)
	next.Right, next.Top, next.Bottom = 9, 17, 17
	v.ObserveEntry(next)
	if v.Stats.Overflows != 1 || v.Stats.MarkersResolved != 0 {
		t.Errorf("放不下的續接不計 MarkersResolved：%+v", v.Stats)
	}
}

// --- dispatcher: the boundary and the missing pieces --------------------------

// 52 half units are the cells of the original: a reading that makes the line
// exactly that wide is drawn, one syllable more gives way to the current text.
func TestDispatchInlineNameExactBoundary(t *testing.T) {
	frags := map[string]string{" makes ": "은(는)", "his": "자신의", " tactics roll.": "전술 판정을 한다"}
	eng := inlineCatalog(t, LangKo, frags, inlineKoMonsters, inlineKoItems)
	const sentence = "AB makes his tactics roll." // 26 cells, 52 units
	for _, tc := range []struct {
		syllables      int
		names, fallbck int
	}{{13, 1, 0}, {14, 0, 1}} {
		party := inlineParty(t, recAb)
		w := inlineDispatcher(t, LangKo, eng, fakeTranslit{"AB": strings.Repeat("가", tc.syllables-1) + "파"})
		dispatchSentence(w, inlineNameCaller, sentence, party)
		units := stringUnits(strings.TrimRight(string(w.Lines()[0].Text), " 　"))
		if w.Stats.InlineNames != tc.names || w.Stats.InlineNameFallback != tc.fallbck || w.Stats.Hits != 1 {
			t.Errorf("%d 音節：%+v（行寬 %d 單位）", tc.syllables, w.Stats, units)
		}
		if tc.names == 1 && units != 52 {
			t.Errorf("%d 音節：應剛好 52 單位，得 %d", tc.syllables, units)
		}
	}
}

// A translation that is too wide without any name in it is an old miss.
func TestDispatchInlineNoNameWideIsPlainMiss(t *testing.T) {
	long := map[string]string{}
	for k, v := range koFragments {
		long[k] = v
	}
	long[" tactics roll."] = "아주아주아주아주아주아주아주 긴 번역문입니다 정말로 매우 길어요"
	eng := inlineCatalog(t, LangKo, long, inlineKoMonsters, inlineKoItems)
	w := inlineDispatcher(t, LangKo, eng, inlineReadings)
	dispatchSentence(w, inlineNameCaller, "MARCUS makes his tactics roll.", inlineParty(t, recFlavius))
	if w.Stats.Misses != 1 || w.Stats.InlineNameFallback != 0 || w.Stats.InlineNames != 0 {
		t.Errorf("%+v", w.Stats)
	}
}

// A dispatcher without player names looks nobody up.
func TestDispatchInlineNameNeedsPlayers(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	w := inlineDispatcher(t, LangKo, eng, inlineReadings)
	w.SetPlayerNames(nil)
	dispatchSentence(w, inlineNameCaller, "FLAVIUS makes his tactics roll.", inlineParty(t, recFlavius))
	if got := inlineLine(w); got != "FLAVIUS은(는) 자신의 전술 판정을 한다." || w.Stats.InlineNames != 0 {
		t.Errorf("%q %+v", got, w.Stats)
	}
}
