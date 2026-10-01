package buckrogers

import (
	"strings"
	"testing"
)

// Buck repo spec 045 §3.4: the space at the start of a continuing
// player-name call in the Korean (word-level) profile.

// koPlayerCall draws "DECK " (the original ends with a space, so the next
// call owes one) and then the player name `name` whose transliteration is
// `kr`, in a one-row window that is `right` columns wide from column 1.
func koPlayerCall(t *testing.T, prof *LayoutProfile, name, kr string, right uint8) (*EclTextWatcher, *EclTextPage) {
	t.Helper()
	party, err := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: name}), testDS)
	if err != nil {
		t.Fatal(err)
	}
	w := eclPlayerWatcher(t, "DECK ", "갑판")
	w.SetLayout(prof)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{name: kr}, nil))
	e1 := eclEntry("DECK ", true, 1, 17)
	e1.Right, e1.Top, e1.Bottom = right, 17, 17
	w.ObserveEntry(e1)
	eclSpaceRet(w)
	col := uint8(4)
	if right < col {
		col = right // the cursor stays inside the window
	}
	e2 := eclPlayerEntry(name, false, col, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
	e2.Right, e2.Top, e2.Bottom = right, 17, 17
	w.ObserveEntry(e2)
	return w, w.Page()
}

func TestEclPlayerNameSpaceKo(t *testing.T) {
	// 갑판 is 4 units; the space 1; 셀레스트(CELESTE) 17; 셀레스트 8; CELESTE 7.
	cases := []struct {
		label         string
		name, kr      string
		right         uint8
		want          string
		spaceDropped  int
		chineseOnly   int
		english       int
		overflows     int
		unitStartsRow bool
	}{
		{"完整單元含空白", "CELESTE", "셀레스트", 11, "갑판 셀레스트(CELESTE)", 0, 0, 0, 0, false},
		{"含空白的只譯名單元", "CELESTE", "셀레스트", 10, "갑판 셀레스트", 0, 1, 0, 0, false},
		{"空白放不下：不含空白的完整單元（窗寬剛好容下）", "CELESTE", "셀레스트", 12, "갑판 셀레스트(CELESTE)", 0, 0, 0, 0, false},
		{"譯名太長：含空白的原文", "CELESTE", "셀레스트셀레스트", 6, "갑판 CELESTE", 0, 0, 1, 0, false},
		{"含空白的原文放不下：不含空白的原文", "CELESTIA", "셀레스트셀레스트", 6, "갑판CELESTIA", 1, 0, 1, 0, false},
		{"全部放不下", "CELESTIA", "셀레스트셀레스트", 3, "", 0, 0, 1, 1, false},
	}
	for _, c := range cases {
		w, p := koPlayerCall(t, layoutKo, c.name, c.kr, c.right)
		got := ""
		if p != nil {
			got = eclRow(p, 17)
		}
		if got != c.want || w.Stats.SpaceDropped != c.spaceDropped || w.Stats.PlayerNameChineseOnly != c.chineseOnly ||
			w.Stats.PlayerNameEnglish != c.english || w.Stats.Overflows != c.overflows {
			t.Errorf("%s：%q（應為 %q）%+v", c.label, got, c.want, w.Stats)
		}
	}
}

// The name is not a particle: a Chinese-only unit that is exactly a particle
// string (DOE → 도) still gets its space (condition 3 of spec 046 §3.4 does
// not apply to player names).
func TestEclPlayerNameSpaceNotGluedLikeParticle(t *testing.T) {
	// 갑판 4 + space 1 + 도(DOE) 2+5 = 12 units.
	for _, c := range []struct {
		right        uint8
		want         string
		chineseOnly  int
		spaceDropped int
	}{
		{7, "갑판 도(DOE)", 0, 0},
		{5, "갑판 도", 1, 0}, // 12 > 10: Chinese only, with its space
		{3, "갑판도", 1, 1},  // 4 + 1 + 2 = 7 > 6: the space is dropped, not the name
	} {
		w, p := koPlayerCall(t, layoutKo, "DOE", "도", c.right)
		if p == nil || eclRow(p, 17) != c.want || w.Stats.PlayerNameChineseOnly != c.chineseOnly || w.Stats.SpaceDropped != c.spaceDropped {
			t.Errorf("right=%d：%q %+v，應為 %q", c.right, eclRow(p, 17), w.Stats, c.want)
		}
	}
}

// After the name unit the last character drawn is the unit's own, and a
// player-name call at the left edge of a row gets no space.
func TestEclPlayerNameSpaceState(t *testing.T) {
	w, p := koPlayerCall(t, layoutKo, "CELESTE", "셀레스트", 20)
	if p == nil || p.lastRune != ')' || p.owedSpace {
		t.Fatalf("name unit state: %+v", p)
	}
	_ = w
	// A call that starts at the left edge of the window (new row): no space.
	party, _ := ReadPartySnapshot(partyMem(1, recCeleste), testDS)
	w = eclPlayerWatcher(t, "DECK ", "갑판")
	w.SetLayout(layoutKo)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "셀레스트"}, nil))
	w.ObserveEntry(eclEntry("DECK ", true, 1, 17))
	eclSpaceRet(w)
	e := eclPlayerEntry("CELESTE", false, 1, 18, eclCtx(party, eclNameCallerB79, 0x81, eclVarSelected, recCeleste))
	w.ObserveEntry(e)
	for _, l := range w.Page().Lines {
		if len(l.Text) > 0 && l.Text[0] == ' ' {
			t.Errorf("列首有空白：%+v", w.Page().Lines)
		}
	}
	// A call that does not continue anything (fresh page) gets no space.
	w = eclPlayerWatcher(t)
	w.SetLayout(layoutKo)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "셀레스트"}, nil))
	w.ObserveEntry(eclPlayerEntry("CELESTE", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if got := eclRow(w.Page(), 17); got != "셀레스트(CELESTE)" || w.Stats.SpaceDropped != 0 {
		t.Errorf("fresh：%q %+v", got, w.Stats)
	}
}

// The other profiles owe no space: the page and counters are those of the
// player-name path before spec 045 (and no leading space anywhere).
func TestEclPlayerNameSpaceOtherProfiles(t *testing.T) {
	for _, prof := range []*LayoutProfile{nil, layoutJa, layoutKoChars} {
		w, p := koPlayerCall(t, prof, "CELESTE", "셀레스트", 20)
		if p == nil || eclRow(p, 17) != "갑판셀레스트(CELESTE)" || w.Stats.SpaceDropped != 0 || p.owedSpace || p.lastRune != 0 {
			t.Errorf("profile %p：%q %+v", prof, eclRow(p, 17), w.Stats)
		}
	}
}

// layoutPlayerName with the space owed, in the order spec 045 §3.4 gives,
// for variants where the order can be told apart (a Chinese-only form wider
// than the full one is not natural, but the order is the contract).
func TestLayoutPlayerNameOrder(t *testing.T) {
	mk := func(s string) AnnotatedText {
		r := []rune(s)
		return AnnotatedText{Text: r, Units: []NameUnit{{0, len(r)}}}
	}
	// width 10 units, one row, the cursor at the start of the row's cells
	const left, right, bottom = 1, 5, 17
	run := func(full, cn string, orig string, space bool, col uint8) playerLayout {
		return layoutPlayerName(layoutKo, []AnnotatedText{mk(full), mk(cn)}, []byte(orig), space, 17, col*2, left*2, right*2+1, bottom)
	}
	_ = run
	// Use the real column coordinates instead: eclUnitLeft/eclUnitRight.
	lay := func(full, cn, orig string, space bool, col uint8) playerLayout {
		return layoutPlayerName(layoutKo, []AnnotatedText{mk(full), mk(cn)}, []byte(orig), space, 17, col, eclUnitLeft(1), eclUnitRight(5), 17)
	}
	start := eclUnitLeft(1) + 2 // two units already drawn on the row: 8 units left
	type want struct {
		kind   int
		spaced bool
		fits   bool
	}
	for _, c := range []struct {
		label          string
		full, cn, orig string
		space          bool
		w              want
	}{
		{"完整單元含空白", "가나다", "가", "ABC", true, want{playerFull, true, true}},
		{"完整單元不含空白（沒有欠空白）", "가나다", "가", "ABC", false, want{playerFull, false, true}},
		{"含空白的完整單元放不下，含空白的只譯名單元", "가나다라", "가나", "ABC", true, want{playerChineseOnly, true, true}},
		{"完整單元含空白放不下、不含空白放得下", "가나다라", "가나다라마", "ABC", true, want{playerFull, false, true}},
		{"兩種含空白都放不下，不含空白的只譯名", "가나다라마", "가나다라", "ABCDEFGHI", true, want{playerChineseOnly, false, true}},
		{"含空白的原文", "가나다라마바", "가나다라마바", "ABCDEFG", true, want{playerEnglish, true, true}},
		{"不含空白的原文", "가나다라마바", "가나다라마바", "ABCDEFGH", true, want{playerEnglish, false, true}},
		{"沒有欠空白時不試含空白的原文", "가나다라마바", "가나다라마바", "ABCDEFGH", false, want{playerEnglish, false, true}},
		{"全部放不下", "가나다라마바", "가나다라마바", "ABCDEFGHIJ", true, want{playerEnglish, false, false}},
	} {
		got := lay(c.full, c.cn, c.orig, c.space, start)
		if got.kind != c.w.kind || got.spaced != c.w.spaced || got.fits != c.w.fits {
			t.Errorf("%s：kind=%d spaced=%v fits=%v，應為 %+v", c.label, got.kind, got.spaced, got.fits, c.w)
		}
		if got.fits && c.w.spaced && !strings.HasPrefix(string(got.lines[0].Text), " ") && got.lines[0].Row == 17 {
			// the space leads the first line of the call unless it was dropped at a row start
			t.Errorf("%s：含空白的候選第一列應以空白起首：%q", c.label, string(got.lines[0].Text))
		}
	}
}
