package buckrogers

import "testing"

// Spec 045 §3.4 moved the player-name layout choice of ObserveEntry into
// layoutPlayerName.  layoutPlayerNamePre045 is the block as it was before
// (spec 038 §3.3), copied verbatim in logic; without an owed space the new
// function must give the same result for every profile and window.

func layoutPlayerNamePre045(prof *LayoutProfile, player []AnnotatedText, original []byte, row, col, left, right, bottom uint8) (kind int, lines []EclTextLine, endRow, endCol uint8, fits bool) {
	chosen := -1
	for i, v := range player {
		if lines, endRow, endCol, fits = layoutEclTextP(prof, v.Text, v.Units, row, col, left, right, bottom); fits {
			chosen = i
			break
		}
	}
	switch chosen {
	case 0:
		kind = playerFull
	case 1:
		kind = playerChineseOnly
	default:
		kind = playerEnglish
		lines, endRow, endCol, fits = layoutEclTextP(prof, []rune(string(original)), nil, row, col, left, right, bottom)
	}
	return
}

func TestLayoutPlayerNameMatchesPre045WithoutSpace(t *testing.T) {
	noShrink(t) // spec 056 §5.8: this test asserts the step-down order of spec 036/038/045 without shrinking
	mk := func(zh, orig string) []AnnotatedText {
		full := []rune(zh + "(" + orig + ")")
		cn := []rune(zh)
		return []AnnotatedText{
			{Tier: NameTierAll, Text: full, Units: []NameUnit{{0, len(full)}}},
			{Tier: NameTierNone, Text: cn, Units: []NameUnit{{0, len(cn)}}},
		}
	}
	names := []struct{ zh, orig string }{
		{"塞萊絲特", "CELESTE"}, {"弗拉維烏斯", "FLAVIUS"}, {"셀레스트", "CELESTE"}, {"セレスト", "CELESTE"},
		{"니콜 스틸", "NICOLE STEELE"}, {"ニコール・スティール", "NICOLE STEELE"}, {"도", "DOE"}, {"가나다라마바사아자차카타파하", "LONGNAME"},
	}
	profs := []struct {
		name string
		p    *LayoutProfile
	}{{"nil", nil}, {"ja", layoutJa}, {"ko", layoutKo}, {"ko-chars", layoutKoChars}}
	windows := [][3]uint8{{1, 38, 22}, {1, 9, 17}, {1, 9, 19}, {5, 12, 18}, {1, 3, 17}}
	n := 0
	for _, pr := range profs {
		for _, nm := range names {
			player := mk(nm.zh, nm.orig)
			for _, win := range windows {
				left, right, bottom := eclUnitLeft(win[0]), eclUnitRight(win[1]), win[2]
				for row := uint8(17); row <= bottom; row++ {
					for col := left; col <= right+1; col++ {
						kind, lines, er, ec, fits := layoutPlayerNamePre045(pr.p, player, []byte(nm.orig), row, col, left, right, bottom)
						got := layoutPlayerName(pr.p, player, []byte(nm.orig), false, row, col, left, right, bottom)
						if got.kind != kind || got.spaced || got.fits != fits || got.endRow != er || got.endCol != ec || !equalLines(got.lines, lines) {
							t.Fatalf("%s %s window %v row %d col %d：kind %d/%d fits %v/%v end %d,%d/%d,%d spaced=%v",
								pr.name, nm.orig, win, row, col, got.kind, kind, got.fits, fits, got.endRow, got.endCol, er, ec, got.spaced)
						}
						n++
					}
				}
			}
		}
	}
	t.Logf("layoutPlayerName（無欠空白）與規格 045 之前的區塊相同：%d 組輸入", n)
}
