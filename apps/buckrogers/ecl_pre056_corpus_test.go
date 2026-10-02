package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Spec 056 §5.3, first layer: the synthetic corpus (names, sentences and
// player names from a fixed seed, no game text) and the digest of what the
// layout of dosgolem 72ce242 returns on it.  The file must compile at
// 72ce242 (no reference to anything spec 056 adds): the digest constant was
// computed there, and the frozen copies in ecl_pre056_freeze_test.go are
// checked against the real functions of that commit on the same corpus.

// pre056Geos are the eleven ECL windows of phase-304 §2 (left, top, right,
// bottom in cells).
var pre056Geos = [][4]uint8{
	{1, 17, 38, 22}, {23, 2, 38, 21}, {23, 7, 38, 9}, {23, 11, 38, 21}, {23, 13, 38, 16}, {23, 12, 38, 21},
	{23, 17, 38, 21}, {23, 16, 38, 21}, {23, 10, 38, 21}, {2, 5, 37, 22}, {2, 7, 37, 22},
}

type pre056Lang struct {
	lang      string
	prof      *LayoutProfile
	glossary  string
	names     []string // the display text of the glossary rows, in glossary order
	words     []string // filler
	particles []string // appended to a name (ko)
	players   [][2]string
	leads     []rune // continuation syllables (ko): none, with a final, without
}

const pre056GlossaryHead = "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n"

func pre056Glossary(rows ...[2]string) string {
	s := pre056GlossaryHead
	for i, r := range rows {
		kind := "full"
		if i%3 == 1 {
			kind = "short"
		}
		s += fmt.Sprintf("%s\t\t%s\t%s\tp%d\tprinted:x\tn\n", r[0], r[1], kind, i)
	}
	return s
}

func pre056Langs() []pre056Lang {
	zh := [][2]string{{"BUCK ROGERS", "巴克羅吉斯"}, {"BUCK", "巴克"}, {"WILMA", "威瑪"}, {"GILBERT", "吉爾伯特"},
		{"ALEXANDER WILLIAMS", "亞歷山大威廉"}, {"KANE", "凱恩"}}
	ja := [][2]string{{"BUCK ROGERS", "バック・ロジャース"}, {"BUCK", "バック"}, {"WILMA", "ウィルマ"}, {"GILBERT", "ギルバート"},
		{"ELIAS", "エリアス"}, {"KANE", "ケイン"}}
	ko := [][2]string{{"BUCK ROGERS", "벅로저스"}, {"BUCK", "벅"}, {"WILMA", "윌마"}, {"GILBERT", "길버트"},
		{"ELIAS", "엘리아스"}, {"KANE", "케인"}}
	col := func(rows [][2]string) (out []string) {
		for _, r := range rows {
			out = append(out, r[1])
		}
		return
	}
	return []pre056Lang{
		{lang: LangZhTW, glossary: pre056Glossary(zh...), names: col(zh),
			words:   []string{"你說的話", "我們走吧", "這是一個很長的句子", "在這裡等著", "發現了", "，", "。", "「好」", "(", ")", "甲板", "5", "12", "ABC", "go on"},
			players: [][2]string{{"塞萊絲特", "CELESTE"}, {"弗拉維烏斯", "FLAVIUS"}, {"巴克", "BUCK"}, {"一二", "AB"}, {"亞歷山大威廉", "ALEXANDER WILLIAMS"}}},
		{lang: LangZhCN, glossary: pre056Glossary(zh...), names: col(zh),
			words:   []string{"你说的话", "我们走吧", "这是一个很长的句子", "在这里等着", "发现了", "，", "。", "「好」", "甲板", "7", "ABC"},
			players: [][2]string{{"塞莱丝特", "CELESTE"}, {"弗拉维乌斯", "FLAVIUS"}, {"巴克", "BUCK"}}},
		{lang: LangJa, prof: layoutJa, glossary: pre056Glossary(ja...), names: col(ja),
			words:   []string{"これは長い文章です", "ありがとう", "「はい」", "、", "。", "ー", "行きましょう", "ここで待つ", "(", "12", "ABC", "go on"},
			players: [][2]string{{"セレステ", "CELESTE"}, {"フラビウス", "FLAVIUS"}, {"バック", "BUCK"}, {"アルキメデス", "ARCHIMEDES"}}},
		{lang: LangKo, prof: layoutKo, glossary: pre056Glossary(ko...), names: col(ko),
			words:     []string{"이것은 긴 문장입니다", "안녕하세요", "가자", "여기서 기다려", "발견했다", ",", ".", "「네」", "12", "ABC", "go on", "갑판"},
			particles: []string{"이(가)", "은(는)", "을(를)", "와(과)", "의", "에게", "(으)로", "이", "가", "는"},
			leads:     []rune{0, '각', '가'},
			players:   [][2]string{{"셀레스트", "CELESTE"}, {"플라비우스", "FLAVIUS"}, {"벅", "BUCK"}, {"알키메데스", "ARCHIMEDES"}}},
	}
}

// pre056Sentences builds n sentences from the language's fragments.
func pre056Sentences(l pre056Lang, r *rand.Rand, n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		var sb strings.Builder
		pieces := 2 + r.Intn(5)
		nameAt := r.Intn(pieces)
		for j := 0; j < pieces; j++ {
			if j == nameAt || r.Intn(4) == 0 {
				sb.WriteString(l.names[r.Intn(len(l.names))])
				if len(l.particles) > 0 && r.Intn(2) == 0 {
					sb.WriteString(l.particles[r.Intn(len(l.particles))])
				}
			} else {
				sb.WriteString(l.words[r.Intn(len(l.words))])
			}
			if l.prof != nil && l.prof.word && j < pieces-1 {
				sb.WriteByte(' ')
			} else if l.prof == nil && r.Intn(5) == 0 {
				sb.WriteByte(' ')
			}
		}
		out = append(out, sb.String())
	}
	// A leading space (the layout keeps it as an indent) and a name first.
	out = append(out, " "+l.names[0]+"說", l.names[1]+"，"+l.names[2]+"。")
	return out
}

// pre056Starts lists the starts of a window: every row, every stride-th half
// unit column plus the last one.
func pre056Starts(g [4]uint8, stride int) (out [][2]uint8) {
	for row := g[1]; row <= g[3]; row++ {
		l, rt := int(eclUnitLeft(g[0])), int(eclUnitRight(g[2]))
		for c := l; c <= rt; c += stride {
			out = append(out, [2]uint8{row, uint8(c)})
		}
		if (rt-l)%stride != 0 {
			out = append(out, [2]uint8{row, uint8(rt)})
		}
	}
	return
}

// pre056PlaceCase is one placeText call.
type pre056PlaceCase struct {
	txt         string
	lead        rune
	spaceNeeded bool
	prevRune    rune
	deck        bool
	geo         [4]uint8
	row, col    uint8
}

func (c pre056PlaceCase) entry() EclTextEntry {
	return EclTextEntry{Left: c.geo[0], Top: c.geo[1], Right: c.geo[2], Bottom: c.geo[3]}
}

// page is the page the call continues: only the deck check reads it (the last
// row ends with 甲板).
func (c pre056PlaceCase) page() *EclTextPage {
	if !c.deck {
		return nil
	}
	return &EclTextPage{Lines: []EclTextLine{{Row: c.geo[1], Col: 0, Text: []rune("甲板")}}}
}

func pre056Watcher(t *testing.T, l pre056Lang) *EclTextWatcher {
	t.Helper()
	g, err := LoadNameGlossary([]byte(l.glossary), nil)
	if err != nil {
		t.Fatal(err)
	}
	w := NewEclTextWatcher(eclFixtureLang(t, l.lang, "A", "甲"))
	w.SetNames(g)
	w.SetLayout(l.prof)
	return w
}

// pre056EachPlace walks the placeText corpus of one language.  The states:
// spaceNeeded with prevRune a letter, a space, or none; the continuation
// syllable for ko; the deck page for the zh languages.
func pre056EachPlace(t *testing.T, l pre056Lang, stride int, fn func(c pre056PlaceCase)) {
	r := rand.New(rand.NewSource(56 + int64(len(l.lang))*7 + int64(l.lang[len(l.lang)-1])))
	sentences := pre056Sentences(l, r, 12)
	if l.lang == LangZhTW || l.lang == LangZhCN {
		sentences = append(sentences, "5"+l.names[0]+"說", "12"+l.names[1]+"，"+l.names[2])
	}
	leads := l.leads
	if len(leads) == 0 {
		leads = []rune{0}
	}
	type st struct {
		space bool
		prev  rune
	}
	states := []st{{false, 'A'}, {true, 'A'}, {true, ' '}, {true, 0}}
	for _, g := range pre056Geos {
		starts := pre056Starts(g, stride)
		for si, s := range sentences {
			for k, p := range starts {
				// Every start for a rotating subset of the states keeps the run short.
				for _, lead := range leads {
					for qi, q := range states {
						if (k+qi+si)%2 != 0 && qi != 0 {
							continue
						}
						for _, deck := range []bool{false, true} {
							if deck && ((l.lang != LangZhTW && l.lang != LangZhCN) || s[0] < '0' || s[0] > '9') {
								continue
							}
							fn(pre056PlaceCase{txt: s, lead: lead, spaceNeeded: q.space, prevRune: q.prev, deck: deck, geo: g, row: p[0], col: p[1]})
						}
					}
				}
			}
		}
	}
}

// pre056PlayerCase is one layoutPlayerName call.
type pre056PlayerCase struct {
	zh, en         string
	space          bool
	geo            [4]uint8
	row, col       uint8
	left, right, b uint8
}

func (c pre056PlayerCase) player() []AnnotatedText {
	full := []rune(c.zh + "(" + c.en + ")")
	cn := []rune(c.zh)
	return []AnnotatedText{
		{Tier: NameTierAll, Text: full, Units: []NameUnit{{0, len(full)}}},
		{Tier: NameTierNone, Text: cn, Units: []NameUnit{{0, len(cn)}}},
	}
}

func pre056EachPlayer(l pre056Lang, stride int, fn func(c pre056PlayerCase)) {
	for _, g := range pre056Geos {
		for _, p := range pre056Starts(g, stride) {
			for _, pl := range l.players {
				for _, space := range []bool{false, true} {
					if space && (l.prof == nil || !l.prof.word) {
						continue // the space of spec 045 is Korean only
					}
					fn(pre056PlayerCase{zh: pl[0], en: pl[1], space: space, geo: g, row: p[0], col: p[1],
						left: eclUnitLeft(g[0]), right: eclUnitRight(g[2]), b: g[3]})
				}
			}
		}
	}
}

func pre056DumpLines(sb *strings.Builder, ls []EclTextLine) {
	for _, l := range ls {
		fmt.Fprintf(sb, "[%d,%d,%s]", l.Row, l.Col, string(l.Text))
	}
}

func pre056DumpPlacement(sb *strings.Builder, pl eclPlacement) {
	pre056DumpLines(sb, pl.lines)
	fmt.Fprintf(sb, " %d %d %v %d %v %d %d %d\n", pl.endRow, pl.endCol, pl.fits, pl.tier, pl.marker, pl.spaceDropped, pl.fullStop, pl.fullStopDropped)
}

func pre056DumpPlayer(sb *strings.Builder, c playerLayout) {
	pre056DumpLines(sb, c.lines)
	fmt.Fprintf(sb, " %d %v %d %d %v\n", c.kind, c.spaced, c.endRow, c.endCol, c.fits)
}

type pre056Place func(w *EclTextWatcher, c pre056PlaceCase) eclPlacement
type pre056Player func(prof *LayoutProfile, c pre056PlayerCase) playerLayout

// pre056Digest runs the corpus through place and player and returns the
// SHA-256 of everything they return and the number of calls.
func pre056Digest(t *testing.T, place pre056Place, player pre056Player) (string, int) {
	t.Helper()
	h := sha256.New()
	n := 0
	for _, l := range pre056Langs() {
		w := pre056Watcher(t, l)
		var sb strings.Builder
		fmt.Fprintf(&sb, "== %s\n", l.lang)
		pre056EachPlace(t, l, 4, func(c pre056PlaceCase) {
			pre056DumpPlacement(&sb, place(w, c))
			n++
			if sb.Len() > 1<<20 {
				h.Write([]byte(sb.String()))
				sb.Reset()
			}
		})
		pre056EachPlayer(l, 2, func(c pre056PlayerCase) {
			pre056DumpPlayer(&sb, player(l.prof, c))
			n++
		})
		h.Write([]byte(sb.String()))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), n
}

func pre056PlaceFrozen(w *EclTextWatcher, c pre056PlaceCase) eclPlacement {
	return w.placeTextPre056(c.txt, "ecl.1", false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())
}

func pre056PlayerFrozen(prof *LayoutProfile, c pre056PlayerCase) playerLayout {
	return layoutPlayerNamePre056(prof, c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)
}

// pre056DigestWant is the digest of the corpus at dosgolem 72ce242, computed
// from the real placeText and layoutPlayerName of that commit (git archive
// export, workplace/phase322/exp.sh; the copies gave the same digest there).
const (
	pre056DigestWant  = "0684a4231742fbc32ba65bbb3bb85871e2905e04fb2204a903267be2bcd4c78c"
	pre056DigestCalls = 387666
)

// The frozen copies reproduce the layout of 72ce242 on the synthetic corpus.
func TestPre056FreezeDigest(t *testing.T) {
	got, n := pre056Digest(t, pre056PlaceFrozen, pre056PlayerFrozen)
	if testing.Verbose() {
		t.Logf("pre056 digest %s calls %d", got, n)
	}
	if got != pre056DigestWant || n != pre056DigestCalls {
		t.Errorf("凍結副本與 72ce242 的摘要不同：%s（%d 次）想要 %s（%d 次）", got, n, pre056DigestWant, pre056DigestCalls)
	}
}
