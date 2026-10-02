package buckrogers

import (
	"testing"
)

// Buck repo spec 054 in the ECL text window: the party member's reading in a
// sentence the engine catalog translates (§3.3), and the particle mark that
// starts a call after a name (§3.2).

var (
	recRoarke = partyRec{seg: 0x5777, off: 0x0003, name: "ROARKE", gender: 0}
	recMarion = partyRec{seg: 0x5787, off: 0x0004, name: "MARION", gender: 0}
	recAb     = partyRec{seg: 0x5797, off: 0x0006, name: "AB", gender: 0}
)

var inlineReadings = fakeTranslit{
	"FLAVIUS": "플라비우스", "ROARKE": "로앙", "CELESTE": "셀레스트", "MARION": "마리온",
	"AB": "아브라함스키안아브라함스키안",
}

func inlineParty(t *testing.T, recs ...partyRec) *PartySnapshot {
	t.Helper()
	s, err := ReadPartySnapshot(partyMem(len(recs), recs...), testDS)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// inlineEclWatcher is the Korean (or other) ECL watcher with the engine
// catalog of the lane and the player names of the readings above.
func inlineEclWatcher(t *testing.T, lang string, prof *LayoutProfile, eng *EngineTextCatalog, pairs ...string) *EclTextWatcher {
	t.Helper()
	w := NewEclTextWatcher(eclFixtureLang(t, lang, append([]string{"UNUSED FIXTURE", "미사용"}, pairs...)...))
	w.SetLayout(prof)
	w.SetEngine(eng)
	w.SetPlayerNames(NewPlayerNames(inlineReadings, nil))
	return w
}

// inlineEntry is a call of the sentence callers: a window of one row unless
// bottom says otherwise, the snapshot those callers read in party.
func inlineEntry(orig string, clear bool, col uint8, party *PartySnapshot, right uint8) EclTextEntry {
	e := eclEntry(orig, clear, col, 17)
	e.Right, e.Top, e.Bottom, e.Party = right, 17, 17, party
	return e
}

func TestEclInlineNameKo(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	party := inlineParty(t, recFlavius, recRoarke)
	for _, tc := range []struct {
		orig, with, without string
		inline              int
	}{
		{"FLAVIUS makes his tactics roll.", "플라비우스는 자신의 전술 판정을 한다.", "FLAVIUS은(는) 자신의 전술 판정을 한다.", 1},
		{"ROARKE makes his tactics roll.", "로앙은 자신의 전술 판정을 한다.", "ROARKE은(는) 자신의 전술 판정을 한다.", 1},
		{"MARCUS makes his tactics roll.", "MARCUS은(는) 자신의 전술 판정을 한다.", "MARCUS은(는) 자신의 전술 판정을 한다.", 0},
		{"GIANT makes his tactics roll.", "거인은 자신의 전술 판정을 한다.", "거인은 자신의 전술 판정을 한다.", 0},
	} {
		w := inlineEclWatcher(t, LangKo, layoutKo, eng)
		w.ObserveEntry(inlineEntry(tc.orig, true, 1, party, 38))
		p := w.Page()
		if p == nil || eclRow(p, 17) != tc.with || w.Stats.InlineNames != tc.inline || w.Stats.InlineNameFallback != 0 || w.Stats.Hits != 1 || w.Stats.Overflows != 0 {
			t.Errorf("%q 有快照：%q %+v，應為 %q", tc.orig, eclRow(p, 17), w.Stats, tc.with)
		}
		w = inlineEclWatcher(t, LangKo, layoutKo, eng)
		w.ObserveEntry(inlineEntry(tc.orig, true, 1, nil, 38))
		if p := w.Page(); p == nil || eclRow(p, 17) != tc.without || w.Stats.InlineNames != 0 {
			t.Errorf("%q 無快照：%q %+v，應為 %q", tc.orig, eclRow(w.Page(), 17), w.Stats, tc.without)
		}
	}
}

// A lane that has no player names, or whose watcher has no engine catalog,
// takes nothing from the snapshot.
func TestEclInlineNameNeedsPlayersAndEngine(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	party := inlineParty(t, recFlavius)
	w := inlineEclWatcher(t, LangKo, layoutKo, eng)
	w.SetPlayerNames(nil)
	w.ObserveEntry(inlineEntry("FLAVIUS makes his tactics roll.", true, 1, party, 38))
	if got := eclRow(w.Page(), 17); got != "FLAVIUS은(는) 자신의 전술 판정을 한다." || w.Stats.InlineNames != 0 {
		t.Errorf("沒有玩家名：%q %+v", got, w.Stats)
	}
}

func TestEclInlineNameJa(t *testing.T) {
	eng := inlineCatalog(t, LangJa, inlineJaFrags, map[string]string{"NEO WARRIOR": "NEO 戦士"}, map[string]string{"Bolt": "ボルト"})
	w := inlineEclWatcher(t, LangJa, layoutJa, eng)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"FLAVIUS": "フラビウス"}, nil))
	w.ObserveEntry(inlineEntry("FLAVIUS makes his tactics roll.", true, 1, inlineParty(t, recFlavius), 38))
	if got := eclRow(w.Page(), 17); got != "フラビウスは自分の戦術判定を行う。" || w.Stats.InlineNames != 1 {
		t.Errorf("ja：%q %+v", got, w.Stats)
	}
}

// The zh lanes have player names too, and they get the same snapshot; their
// pages, row by row, and counters are those of a call without one.
func TestEclInlineNameZhUnchanged(t *testing.T) {
	party := inlineParty(t, recFlavius, recRoarke)
	for _, lang := range []string{LangZhTW, LangZhCN, LangTest, ""} {
		eng := engineFreezeCatalog(t, lang)
		run := func(p *PartySnapshot) (*EclTextWatcher, []string) {
			w := inlineEclWatcher(t, lang, LayoutFor(lang), eng)
			var rows []string
			for _, s := range []string{"FLAVIUS makes his tactics roll.", "(from behind) Hitting for 3 points of damage", "ROARKE", "Bolt Gun"} {
				e := inlineEntry(s, true, 1, p, 38)
				e.Bottom = 22
				w.ObserveEntry(e)
				eclSpaceRet(w)
				if pg := w.Page(); pg != nil {
					rows = append(rows, eclRow(pg, 17))
				} else {
					rows = append(rows, "<none>")
				}
			}
			return w, rows
		}
		with, rowsWith := run(party)
		without, rowsWithout := run(nil)
		if with.Stats != without.Stats {
			t.Errorf("lang %q：計數不同 %+v 對 %+v", lang, with.Stats, without.Stats)
		}
		for i := range rowsWith {
			if rowsWith[i] != rowsWithout[i] {
				t.Errorf("lang %q 第 %d 次：%q 對 %q", lang, i, rowsWith[i], rowsWithout[i])
			}
		}
		if with.Stats.InlineNames != 0 || with.Stats.InlineNameFallback != 0 || with.Stats.MarkersResolved != 0 {
			t.Errorf("lang %q 不應有 054 計數：%+v", lang, with.Stats)
		}
	}
}

// The reading that is too wide for the window gives way to the current text
// (the English name with its mark), and only when that does not fit either the
// window falls back as before.
func TestEclInlineNameWidthFallback(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	party := inlineParty(t, recAb)
	const sentence = "AB makes his tactics roll."
	with, _, inline := eng.TranslateParty(sentence, party.NameFunc(NewPlayerNames(inlineReadings, nil)))
	plain, _ := eng.Translate(sentence)
	if inline != 1 || stringUnits(with) <= stringUnits(plain) {
		t.Fatalf("fixture：讀音版 %q（%d 單位）應比現行 %q（%d）寬", with, stringUnits(with), plain, stringUnits(plain))
	}
	// Columns are two units each.
	cols := func(units int) uint8 { return uint8((units + 1) / 2) }
	right := 1 + cols(stringUnits(plain)) - 1
	if 2*int(cols(stringUnits(plain))) >= stringUnits(with) {
		t.Fatalf("fixture：窗寬無法區分兩個版本")
	}
	w := inlineEclWatcher(t, LangKo, layoutKo, eng)
	w.ObserveEntry(inlineEntry(sentence, true, 1, party, right))
	if p := w.Page(); p == nil || eclRow(p, 17) != plain || w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 1 || w.Stats.Overflows != 0 || w.Stats.Hits != 1 {
		t.Errorf("退回現行輸出：%q %+v，應為 %q", eclRow(w.Page(), 17), w.Stats, plain)
	}
	// Narrower than the current text too: the old fallback (the page is
	// dropped), and the counter says the second text was tried.
	w = inlineEclWatcher(t, LangKo, layoutKo, eng)
	w.ObserveEntry(inlineEntry(sentence, true, 1, party, right-3))
	if w.Page() != nil || w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 1 || w.Stats.Overflows != 1 || w.Stats.Hits != 0 {
		t.Errorf("兩個版本都放不下：%+v", w.Stats)
	}
	// Without the snapshot nothing about the names is counted.
	w = inlineEclWatcher(t, LangKo, layoutKo, eng)
	w.ObserveEntry(inlineEntry(sentence, true, 1, nil, right))
	if w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 0 || eclRow(w.Page(), 17) != plain {
		t.Errorf("無快照：%q %+v", eclRow(w.Page(), 17), w.Stats)
	}
}

// --- the particle mark that starts a call -------------------------------------

// namedThen draws the player-name call of name (kr is its reading) and then a
// call of the ECL catalog that starts with a mark, as the original does for a
// name and the sentence that goes on.
func namedThen(t *testing.T, name string, rec partyRec, next string, pairs ...string) (*EclTextWatcher, *EclTextPage) {
	t.Helper()
	party := inlineParty(t, rec)
	w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
	e := eclPlayerEntry(name, true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e.Right, e.Top, e.Bottom = 38, 17, 17
	w.ObserveEntry(e)
	eclSpaceRet(w)
	n := eclEntry(next, false, 20, 17)
	n.Right, n.Top, n.Bottom = 38, 17, 17
	w.ObserveEntry(n)
	return w, w.Page()
}

func TestEclLeadingMarkAfterPlayerName(t *testing.T) {
	pairs := []string{" IS HIT", "이(가) 서서히", " GOES", "은(는) 간다", "ENDS", "(으)로 끝난다"}
	for _, tc := range []struct {
		name  string
		rec   partyRec
		next  string
		want  string
		note  string
	}{
		{"CELESTE", recCeleste, " IS HIT", "셀레스트(CELESTE)가 서서히", "無尾音取 가，名字與助詞之間沒有空白"},
		{"MARION", recMarion, " IS HIT", "마리온(MARION)이 서서히", "有尾音取 이：解析成單獨的 이 也不插空白（先判斷空白，後換標記）"},
		{"MARION", recMarion, " GOES", "마리온(MARION)은 간다", "은(는) 有尾音"},
		{"CELESTE", recCeleste, " GOES", "셀레스트(CELESTE)는 간다", "은(는) 無尾音"},
		{"CELESTE", recCeleste, "ENDS", "셀레스트(CELESTE)로 끝난다", "原文沒有空白的續接：(으)로 無尾音取 로"},
		{"MARION", recMarion, "ENDS", "마리온(MARION)으로 끝난다", "(으)로 有尾音取 으로"},
	} {
		w, p := namedThen(t, tc.name, tc.rec, tc.next, pairs...)
		if p == nil || eclRow(p, 17) != tc.want || w.Stats.MarkersResolved != 1 || w.Stats.SpaceDropped != 0 {
			t.Errorf("%s：%q %+v，應為 %q", tc.note, eclRow(p, 17), w.Stats, tc.want)
		}
	}
}

// The mark stays when the last thing drawn cannot be read as a syllable, when
// the name went through as English, and on a page that starts here.
func TestEclLeadingMarkKept(t *testing.T) {
	pairs := []string{"TURN 5", "차례 5", "DONE.", "끝났다.", " IS HIT", "이(가) 서서히", "STOP", "멈춰", "NAME", "이름"}
	t.Run("digit", func(t *testing.T) {
		w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
		w.ObserveEntry(eclEntry("TURN 5", true, 1, 17))
		eclSpaceRet(w)
		w.ObserveEntry(eclEntry(" IS HIT", false, 8, 17))
		if got := eclRow(w.Page(), 17); got != "차례 5이(가) 서서히" || w.Stats.MarkersResolved != 0 {
			t.Errorf("數字之後：%q %+v", got, w.Stats)
		}
	})
	t.Run("period", func(t *testing.T) {
		w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
		w.ObserveEntry(eclEntry("DONE.", true, 1, 17))
		eclSpaceRet(w)
		w.ObserveEntry(eclEntry(" IS HIT", false, 8, 17))
		if got := eclRow(w.Page(), 17); got != "끝났다. 이(가) 서서히" || w.Stats.MarkersResolved != 0 {
			t.Errorf("句點之後：%q %+v", got, w.Stats)
		}
	})
	t.Run("english name", func(t *testing.T) {
		// A window one row high and too narrow for the reading draws the
		// English name through the passthrough: lastReading is 0.
		party := inlineParty(t, recAb)
		w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
		first := eclEntry("STOP", true, 1, 17)
		first.Right, first.Top, first.Bottom = 8, 17, 17
		w.ObserveEntry(first)
		eclSpaceRet(w)
		e := eclPlayerEntry("AB", false, 4, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
		e.Right, e.Top, e.Bottom = 8, 17, 17
		w.ObserveEntry(e)
		eclSpaceRet(w)
		if p := w.Page(); p == nil || w.Stats.PlayerNameEnglish != 1 || p.lastReading != 0 {
			t.Fatalf("fixture：名字應退回英文，lastReading 為 0：%+v %+v", w.Stats, p)
		}
	})
	t.Run("fresh page", func(t *testing.T) {
		w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
		w.ObserveEntry(eclEntry("NAME", true, 1, 17))
		eclSpaceRet(w)
		if w.Page().lastReading != '름' {
			t.Fatalf("lastReading %q", w.Page().lastReading)
		}
		// Clearing starts a page of its own: nothing carries over.
		w.ObserveEntry(eclEntry(" IS HIT", true, 1, 17))
		if got := eclRow(w.Page(), 17); got != "이(가) 서서히" || w.Stats.MarkersResolved != 0 {
			t.Errorf("清除後的新頁：%q %+v", got, w.Stats)
		}
	})
}

// A call that draws nothing keeps what the page was last read as, and a window
// change (a page of its own) does not inherit it.
func TestEclLastReadingState(t *testing.T) {
	pairs := []string{"NAME", "이름", " IS HIT", "이(가) 서서히"}
	w := inlineEclWatcher(t, LangKo, layoutKo, nil, pairs...)
	w.ObserveEntry(eclEntry("NAME", true, 1, 17))
	eclSpaceRet(w)
	if r := w.Page().lastReading; r != '름' {
		t.Fatalf("lastReading %q", r)
	}
	// An empty call draws nothing.
	w.ObserveEntry(eclEntry("", false, 6, 17))
	eclSpaceRet(w)
	if r := w.Page().lastReading; r != '름' {
		t.Errorf("空呼叫應保留前值：%q", r)
	}
	w.ObserveEntry(eclEntry(" IS HIT", false, 6, 17))
	if got := eclRow(w.Page(), 17); got != "이름이 서서히" || w.Stats.MarkersResolved != 1 {
		t.Errorf("%q %+v", got, w.Stats)
	}
	// The last character decides: after the call that ends with a space or a
	// Latin letter the value is 0.
	for _, tc := range []struct{ orig, tr string }{{"Z", "이름."}, {"Y", "이름Y"}} {
		v := inlineEclWatcher(t, LangKo, layoutKo, nil, "NAME", "이름", tc.orig, tc.tr)
		v.ObserveEntry(eclEntry("NAME", true, 1, 17))
		eclSpaceRet(v)
		v.ObserveEntry(eclEntry(tc.orig, false, 6, 17))
		if r := v.Page().lastReading; r != 0 {
			t.Errorf("%q 之後 lastReading 應為 0：%q", tc.tr, r)
		}
	}
}

// Japanese and zh pages never carry a reading.
func TestEclLastReadingKoOnly(t *testing.T) {
	for _, tc := range []struct {
		lang string
		prof *LayoutProfile
		text string
	}{{LangJa, layoutJa, "テスト"}, {LangZhTW, nil, "測試"}, {LangKo, layoutKoChars, "테스트"}} {
		w := inlineEclWatcher(t, tc.lang, tc.prof, nil, "NAME", tc.text)
		w.ObserveEntry(eclEntry("NAME", true, 1, 17))
		if r := w.Page().lastReading; r != 0 {
			t.Errorf("%s／%v：lastReading 應為 0：%q", tc.lang, tc.prof != nil && tc.prof.word, r)
		}
	}
}
