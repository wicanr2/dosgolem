package buckrogers

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Spec 058 §3.4: what the wider L2 cell of ko changes in the layout.  Every
// call is laid out twice by the same program, once with the table of ko and
// once with the ko table replaced by the one of spec 056 (withLangShrinkLevels),
// and the pair is put into exactly one class:
//
//	1   the old call did not fit or was not at L2: the outputs are byte-equal;
//	2a  the old call was at L2 and the new one is too, with the same tier and
//	    space handling: 2a-i the lines are the same and every position follows
//	    from the wider unit (computed here from the text of the units), or
//	    2a-ii the rows are laid out differently and the layout still satisfies
//	    the invariants (a) to (d) of the spec;
//	2b  the new call lands on a later tier or on no shrink: counted by landing;
//	2c  the old call was at L2 and the new one does not fit: must be 0;
//	3   the old call was not at L2 and the new one is: must be 0.

// koCallOut is one call in a comparable form.
type koCallOut struct {
	fits           bool
	shrink         int
	kind           string // the tier and the space handling of the call
	lines          []EclTextLine
	endRow, endCol uint8
	whole          string // every field, for the byte-equal comparison
}

func koOutOfPlace(pl eclPlacement) koCallOut {
	kind := fmt.Sprintf("tier=%d sd=%d marker=%v fs=%d/%d", pl.tier, pl.spaceDropped, pl.marker, pl.fullStop, pl.fullStopDropped)
	return koCallOut{fits: pl.fits, shrink: pl.shrink, kind: kind, lines: pl.lines, endRow: pl.endRow, endCol: pl.endCol,
		whole: fmt.Sprintf("%s|%d,%d|%v|%d|%s", shrinkLinesKey(pl.lines), pl.endRow, pl.endCol, pl.fits, pl.shrink, kind)}
}

func koOutOfPlayer(c playerLayout) koCallOut {
	kind := fmt.Sprintf("kind=%d spaced=%v", c.kind, c.spaced)
	return koCallOut{fits: c.fits, shrink: c.shrink, kind: kind, lines: c.lines, endRow: c.endRow, endCol: c.endCol,
		whole: fmt.Sprintf("%s|%d,%d|%v|%d|%s", shrinkLinesKey(c.lines), c.endRow, c.endCol, c.fits, c.shrink, kind)}
}

// koUnitCount is F and H of a shrunk unit by the width class of its characters.
func koUnitCount(text []rune) (f, h int) {
	for _, r := range text {
		if isHalfwidth(r) {
			h++
		} else {
			f++
		}
	}
	return
}

// koDelta is how many half units wider a unit is at the new L2 than at the old
// (5F + 2H against 4F + 2H, rounded up to whole half units).
func koDelta(text []rune) int {
	f, h := koUnitCount(text)
	return (5*f+2*h+3)/4 - (4*f+2*h+3)/4
}

// koLineWidth is the width in half units of a line: a shrunk unit takes
// ceil((fullLP F + 2 H) / 4), any other line the sum of its characters.
func koLineWidth(l EclTextLine, fullLP int) int {
	if l.Shrink == 0 {
		return textUnits(l.Text)
	}
	f, h := koUnitCount(l.Text)
	return (fullLP*f + 2*h + 3) / 4
}

// koLayoutProblem checks the invariants (b), (c) and (d) of spec 058 §3.4 on
// one output: adjacent lines of a row touch, every line lies inside the
// window, and the rows never decrease.
func koLayoutProblem(o koCallOut, fullLP int, left, right uint8) string {
	ls := append([]EclTextLine(nil), o.lines...)
	for i := 1; i < len(o.lines); i++ {
		if o.lines[i].Row < o.lines[i-1].Row {
			return fmt.Sprintf("rows decrease at line %d", i)
		}
	}
	sort.SliceStable(ls, func(i, j int) bool {
		if ls[i].Row != ls[j].Row {
			return ls[i].Row < ls[j].Row
		}
		return ls[i].Col < ls[j].Col
	})
	for i, l := range ls {
		w := koLineWidth(l, fullLP)
		if int(l.Col) < int(left) || int(l.Col)+w-1 > int(right) {
			return fmt.Sprintf("line %d [%d,%d,%q] width %d outside the window %d..%d", i, l.Row, l.Col, string(l.Text), w, left, right)
		}
		if i > 0 && ls[i-1].Row == l.Row && int(l.Col) != int(ls[i-1].Col)+koLineWidth(ls[i-1], fullLP) {
			return fmt.Sprintf("line %d [%d,%d] does not follow line %d", i, l.Row, l.Col, i-1)
		}
	}
	return ""
}

// koFlat is the characters of an output in reading order, blanks removed, and
// the sequence of its shrunk units.
func koFlat(o koCallOut) (text, units string) {
	ls := append([]EclTextLine(nil), o.lines...)
	sort.SliceStable(ls, func(i, j int) bool {
		if ls[i].Row != ls[j].Row {
			return ls[i].Row < ls[j].Row
		}
		return ls[i].Col < ls[j].Col
	})
	var tb, ub strings.Builder
	for _, l := range ls {
		for _, r := range l.Text {
			if !isBlankRune(r) {
				tb.WriteRune(r)
			}
		}
		if l.Shrink != 0 {
			fmt.Fprintf(&ub, "[%d:%s]", l.Shrink, string(l.Text))
		}
	}
	return tb.String(), ub.String()
}

// koTally counts the classes of one corpus.
type koTally struct {
	calls, c1, c2ai, c2aiiRow, c2aiiBreak, c2b, c2c, c3 int
	landing                                             map[string]int
	oldShrink, newShrink                                [3]int
	bad                                                 []string
}

func (v *koTally) addBad(format string, args ...any) {
	if len(v.bad) < 8 {
		v.bad = append(v.bad, fmt.Sprintf(format, args...))
	}
}

func (v *koTally) String() string {
	var keys []string
	for k := range v.landing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var ls []string
	for _, k := range keys {
		ls = append(ls, fmt.Sprintf("%s=%d", k, v.landing[k]))
	}
	return fmt.Sprintf("呼叫=%d 第1條相同=%d 2a-i=%d 2a-ii(換列)=%d 2a-ii(斷行不同)=%d 2b=%d 2c=%d 第3條=%d 舊L1/L2=%d/%d 新L1/L2=%d/%d 2b落點[%s] 違反=%d",
		v.calls, v.c1, v.c2ai, v.c2aiiRow, v.c2aiiBreak, v.c2b, v.c2c, v.c3, v.oldShrink[1], v.oldShrink[2], v.newShrink[1], v.newShrink[2], strings.Join(ls, "; "), len(v.bad))
}

// classify puts one pair of outputs into its class.
func (v *koTally) classify(label string, left, right uint8, old, cur koCallOut) {
	v.calls++
	if old.shrink >= 0 && old.shrink <= 2 {
		v.oldShrink[old.shrink]++
	}
	if cur.shrink >= 0 && cur.shrink <= 2 {
		v.newShrink[cur.shrink]++
	}
	if !old.fits || old.shrink != 2 {
		if cur.shrink == 2 {
			v.c3++
			v.addBad("%s: class 3: old shrink %d fits %v, new at L2", label, old.shrink, old.fits)
		}
		if cur.whole != old.whole {
			v.addBad("%s: class 1 differs:\n old %s\n new %s", label, old.whole, cur.whole)
		}
		v.c1++
		return
	}
	if !cur.fits {
		v.c2c++
		v.addBad("%s: class 2c: the old call is at L2, the new one does not fit", label)
		return
	}
	if cur.shrink != 2 || cur.kind != old.kind {
		v.c2b++
		if v.landing == nil {
			v.landing = map[string]int{}
		}
		v.landing[fmt.Sprintf("{%s}->{shrink %d, %s}", old.kind, cur.shrink, cur.kind)]++
		return
	}
	// Class 2a.
	same := len(cur.lines) == len(old.lines)
	for i := 0; same && i < len(cur.lines); i++ {
		a, b := cur.lines[i], old.lines[i]
		same = a.Row == b.Row && a.Shrink == b.Shrink && string(a.Text) == string(b.Text)
	}
	if same && cur.endRow == old.endRow {
		// 2a-i: every position follows from the units before it on its row.
		sum, endExtra := map[uint8]int{}, 0
		for i, l := range cur.lines {
			if want := int(old.lines[i].Col) + sum[l.Row]; int(l.Col) != want {
				v.addBad("%s: class 2a-i: line %d [%d,%d,%q] at column %d, expected %d", label, i, l.Row, l.Col, string(l.Text), l.Col, want)
				break
			}
			if l.Shrink != 0 {
				sum[l.Row] += koDelta(l.Text)
			}
		}
		endExtra = sum[old.endRow]
		if int(cur.endCol) != int(old.endCol)+endExtra {
			v.addBad("%s: class 2a-i: endCol %d, expected %d + %d", label, cur.endCol, old.endCol, endExtra)
		}
		v.c2ai++
		return
	}
	// 2a-ii: the rows are laid out differently.
	ct, cu := koFlat(cur)
	ot, ou := koFlat(old)
	if ct != ot || cu != ou {
		v.addBad("%s: class 2a-ii (a): text differs:\n old %q %s\n new %q %s", label, ot, ou, ct, cu)
	}
	if m := koLayoutProblem(cur, 5, left, right); m != "" {
		v.addBad("%s: class 2a-ii new: %s", label, m)
	}
	if m := koLayoutProblem(old, 4, left, right); m != "" {
		v.addBad("%s: class 2a-ii old: %s", label, m)
	}
	rowsOf := func(o koCallOut) string {
		var sb strings.Builder
		for _, l := range o.lines {
			if l.Shrink != 0 {
				fmt.Fprintf(&sb, "%d,", l.Row)
			}
		}
		return sb.String()
	}
	if rowsOf(cur) != rowsOf(old) {
		v.c2aiiRow++
	} else {
		v.c2aiiBreak++
	}
}

func (v *koTally) check(t *testing.T, name string) {
	t.Helper()
	t.Logf("%s: %s", name, v)
	if v.c2c != 0 || v.c3 != 0 || len(v.bad) != 0 {
		t.Errorf("%s：2c=%d 第3條=%d，違反 %d 筆（§3.4 第 3 條與 2c 必須為 0；非 0 時停止實作並修訂規格）：\n%s", name, v.c2c, v.c3, len(v.bad), strings.Join(v.bad, "\n"))
	}
}

// koPairs lays the synthetic corpus of one language out with the current table
// and with the table of spec 056 for that language.
func koSynthetic(t *testing.T, lang string) (place, player *koTally) {
	t.Helper()
	var pl pre056Lang
	for _, l := range pre056Langs() {
		if l.lang == lang {
			pl = l
		}
	}
	w := pre056Watcher(t, pl)
	var curPlace, oldPlace []koCallOut
	var cases []pre056PlaceCase
	pre056EachPlace(t, pl, 4, func(c pre056PlaceCase) {
		cases = append(cases, c)
		curPlace = append(curPlace, koOutOfPlace(adapterPlace(w, c)))
	})
	var pcases []pre056PlayerCase
	var curPlayer, oldPlayer []koCallOut
	pre056EachPlayer(pl, 2, func(c pre056PlayerCase) {
		pcases = append(pcases, c)
		curPlayer = append(curPlayer, koOutOfPlayer(adapterPlayer(lang, pl.prof, c)))
	})
	t.Run("table of spec 056", func(t *testing.T) {
		withLangShrinkLevels(t, lang, shrinkLevels...)
		for _, c := range cases {
			oldPlace = append(oldPlace, koOutOfPlace(adapterPlace(w, c)))
		}
		for _, c := range pcases {
			oldPlayer = append(oldPlayer, koOutOfPlayer(adapterPlayer(lang, pl.prof, c)))
		}
	})
	place, player = &koTally{}, &koTally{}
	for i, c := range cases {
		place.classify(fmt.Sprintf("placeText %q geo %v at %d,%d", c.txt, c.geo, c.row, c.col), eclUnitLeft(c.geo[0]), eclUnitRight(c.geo[2]), oldPlace[i], curPlace[i])
	}
	for i, c := range pcases {
		player.classify(fmt.Sprintf("layoutPlayerName %q/%q geo %v at %d,%d", c.zh, c.en, c.geo, c.row, c.col), c.left, c.right, oldPlayer[i], curPlayer[i])
	}
	return
}

func TestShrinkKoClassifySynthetic(t *testing.T) {
	place, player := koSynthetic(t, LangKo)
	place.check(t, "ko placeText（合成語料）")
	player.check(t, "ko layoutPlayerName（合成語料）")
	// The corpus has to reach the classes, or it proves nothing.
	if place.c1 == 0 || place.c2ai == 0 || place.c2b == 0 || player.c1 == 0 || player.c2ai == 0 || player.c2b == 0 {
		t.Errorf("語料沒有涵蓋各類：placeText %v playerLayout %v", place, player)
	}
}

// The same on the formal text/ catalog of ko (environment gate): at most 2,000
// sampled calls per window (seed 56).
func TestShrinkKoClassifyFormalText(t *testing.T) {
	r := shrinkFormalLoad(t, LangKo)
	w, places, players, nSent, nPlayers := shrinkFormalCorpus(t, r, LangKo, 2000)
	var curPlace, oldPlace, curPlayer, oldPlayer []koCallOut
	for _, p := range places {
		c := p.c
		curPlace = append(curPlace, koOutOfPlace(w.placeText(c.txt, p.key, false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())))
	}
	for _, c := range players {
		curPlayer = append(curPlayer, koOutOfPlayer(layoutPlayerName(w.layout, w.levels(), c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)))
	}
	t.Run("table of spec 056", func(t *testing.T) {
		withLangShrinkLevels(t, LangKo, shrinkLevels...)
		for _, p := range places {
			c := p.c
			oldPlace = append(oldPlace, koOutOfPlace(w.placeText(c.txt, p.key, false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())))
		}
		for _, c := range players {
			oldPlayer = append(oldPlayer, koOutOfPlayer(layoutPlayerName(w.layout, w.levels(), c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)))
		}
	})
	place, player := &koTally{}, &koTally{}
	for i, p := range places {
		place.classify(fmt.Sprintf("placeText %s geo %v at %d,%d", p.key, p.c.geo, p.c.row, p.c.col), eclUnitLeft(p.c.geo[0]), eclUnitRight(p.c.geo[2]), oldPlace[i], curPlace[i])
	}
	for i, c := range players {
		player.classify(fmt.Sprintf("layoutPlayerName %q/%q geo %v at %d,%d", c.zh, c.en, c.geo, c.row, c.col), c.left, c.right, oldPlayer[i], curPlayer[i])
	}
	place.check(t, fmt.Sprintf("ko placeText（text/ 語料，句子 %d）", nSent))
	player.check(t, fmt.Sprintf("ko layoutPlayerName（text/ 語料，玩家名 %d）", nPlayers))
}
