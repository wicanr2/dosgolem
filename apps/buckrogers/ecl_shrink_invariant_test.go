package buckrogers

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Spec 056 §3.8 item 1 and §5.3, first layer: the current placeText and
// layoutPlayerName against the frozen copies of 72ce242.  A call the old
// layout placed at the first state it tried (all annotations at normal size,
// or the full player name) must come out the same, shrink level included.
// Every other difference has to be one of the two expected classes:
//   a: the old layout stepped down (first occurrence only, none, Chinese
//      only, English) and the new one draws the units smaller;
//   b: the old layout gave up the leading space and the new one keeps it and
//      draws the units smaller.
// A call with no level that fits comes out as the old one.

func shrinkLinesKey(ls []EclTextLine) string {
	var sb strings.Builder
	for _, l := range ls {
		fmt.Fprintf(&sb, "[%d,%d,%d,%s]", l.Row, l.Col, l.Shrink, string(l.Text))
	}
	return sb.String()
}

// shrinkLevelOf is the one level the shrunk lines of a call share (0: none);
// ok is false when two levels are mixed.
func shrinkLevelOf(ls []EclTextLine) (lv int, ok bool) {
	for _, l := range ls {
		if l.Shrink == 0 {
			continue
		}
		if lv != 0 && lv != int(l.Shrink) {
			return lv, false
		}
		lv = int(l.Shrink)
	}
	return lv, true
}

type invTally struct {
	calls, first, same, a, b, bad int
	shrunk                        [3]int // shrink calls by level (index 1, 2)
	fits, overflow                int     // new layouts that fit and that did not
	badSamples                    []string
}

func (v *invTally) addBad(format string, args ...any) {
	v.bad++
	if len(v.badSamples) < 8 {
		v.badSamples = append(v.badSamples, fmt.Sprintf(format, args...))
	}
}

func (v *invTally) String() string {
	return fmt.Sprintf("calls=%d 舊版第一狀態 L0（逐位元組相同）=%d 降段或溢出而輸出相同=%d 類 a=%d 類 b=%d 縮小 L1=%d L2=%d 放得下=%d 溢出=%d 違反=%d",
		v.calls, v.first, v.same, v.a, v.b, v.shrunk[1], v.shrunk[2], v.fits, v.overflow, v.bad)
}

func (v *invTally) check(t *testing.T, label string) {
	t.Helper()
	t.Logf("%s: %s", label, v)
	if v.bad != 0 {
		t.Errorf("%s：%d 筆違反 §3.8 第 1 條：\n%s", label, v.bad, strings.Join(v.badSamples, "\n"))
	}
}

// classifyPlace compares one placeText call with its frozen copy.
func (v *invTally) classifyPlace(c pre056PlaceCase, old, cur eclPlacement) {
	v.calls++
	if cur.fits {
		v.fits++
	} else {
		v.overflow++
	}
	desc := fmt.Sprintf("%q start (%d,%d) geo %v lead %q space %v prev %q deck %v\n  old %s fits %v tier %d dropped %d\n  new %s fits %v tier %d dropped %d shrink %d",
		c.txt, c.row, c.col, c.geo, c.lead, c.spaceNeeded, c.prevRune, c.deck,
		shrinkLinesKey(old.lines), old.fits, old.tier, old.spaceDropped,
		shrinkLinesKey(cur.lines), cur.fits, cur.tier, cur.spaceDropped, cur.shrink)
	if lv, ok := shrinkLevelOf(cur.lines); !ok || lv != cur.shrink {
		v.addBad("一次呼叫的縮小等級不一致：%s", desc)
		return
	}
	identical := old.fits == cur.fits && old.tier == cur.tier && old.marker == cur.marker &&
		old.spaceDropped == cur.spaceDropped && old.fullStop == cur.fullStop && old.fullStopDropped == cur.fullStopDropped &&
		(!old.fits || (old.endRow == cur.endRow && old.endCol == cur.endCol && shrinkLinesKey(old.lines) == shrinkLinesKey(cur.lines)))
	if old.fits && old.tier == NameTierAll && old.spaceDropped == 0 {
		if !identical || cur.shrink != 0 {
			v.addBad("舊版第一狀態 L0 的呼叫輸出不同：%s", desc)
			return
		}
		v.first++
		return
	}
	if identical && cur.shrink == 0 {
		v.same++
		return
	}
	switch {
	case !cur.fits || cur.shrink == 0 || cur.tier != NameTierAll:
		v.addBad("不屬 a、b 類（新版沒有縮小地放得下，卻與舊版不同）：%s", desc)
	case cur.marker != old.marker || cur.fullStop != old.fullStop || cur.fullStopDropped != old.fullStopDropped:
		v.addBad("縮小改變了其他計數：%s", desc)
	case cur.spaceDropped == old.spaceDropped:
		v.a++
		v.shrunk[cur.shrink]++
	case cur.spaceDropped < old.spaceDropped:
		v.b++
		v.shrunk[cur.shrink]++
	default:
		v.addBad("SpaceDropped 增加：%s", desc)
	}
}

// classifyPlayer compares one layoutPlayerName call with its frozen copy.
func (v *invTally) classifyPlayer(c pre056PlayerCase, old, cur playerLayout) {
	v.calls++
	if cur.fits {
		v.fits++
	} else {
		v.overflow++
	}
	desc := fmt.Sprintf("%s(%s) space %v start (%d,%d) geo %v\n  old %s kind %d spaced %v fits %v\n  new %s kind %d spaced %v fits %v shrink %d",
		c.zh, c.en, c.space, c.row, c.col, c.geo,
		shrinkLinesKey(old.lines), old.kind, old.spaced, old.fits,
		shrinkLinesKey(cur.lines), cur.kind, cur.spaced, cur.fits, cur.shrink)
	if lv, ok := shrinkLevelOf(cur.lines); !ok || lv != cur.shrink {
		v.addBad("一次呼叫的縮小等級不一致：%s", desc)
		return
	}
	identical := old.fits == cur.fits && (!old.fits || (old.kind == cur.kind && old.spaced == cur.spaced &&
		old.endRow == cur.endRow && old.endCol == cur.endCol && shrinkLinesKey(old.lines) == shrinkLinesKey(cur.lines)))
	if old.fits && old.kind == playerFull && old.spaced == c.space {
		if !identical || cur.shrink != 0 {
			v.addBad("舊版第一狀態 L0 的呼叫輸出不同：%s", desc)
			return
		}
		v.first++
		return
	}
	if identical && cur.shrink == 0 {
		v.same++
		return
	}
	switch {
	case !cur.fits || cur.shrink == 0 || cur.kind != playerFull:
		v.addBad("不屬 a、b 類（新版沒有縮小地放得下，卻與舊版不同）：%s", desc)
	case c.space && old.fits && old.kind == playerFull && !old.spaced && cur.spaced:
		v.b++
		v.shrunk[cur.shrink]++
	case old.fits && old.kind == playerFull && old.spaced && !c.space:
		v.addBad("無空白呼叫的舊結果不應帶空白：%s", desc)
	default:
		v.a++
		v.shrunk[cur.shrink]++
	}
}

// Spec 056 §5.3: the synthetic corpus of the digest test, new against old.
func TestShrinkInvariantSynthetic(t *testing.T) {
	var all, allPlayers invTally
	for _, l := range pre056Langs() {
		w := pre056Watcher(t, l)
		var place, player invTally
		pre056EachPlace(t, l, 4, func(c pre056PlaceCase) {
			cur := w.placeText(c.txt, "ecl.1", false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())
			old := pre056PlaceFrozen(w, c)
			place.classifyPlace(c, old, cur)
		})
		pre056EachPlayer(l, 2, func(c pre056PlayerCase) {
			cur := layoutPlayerName(l.prof, c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)
			player.classifyPlayer(c, pre056PlayerFrozen(l.prof, c), cur)
		})
		place.check(t, l.lang+" placeText")
		player.check(t, l.lang+" layoutPlayerName")
		for _, x := range [][2]*invTally{{&all, &place}, {&allPlayers, &player}} {
			x[0].calls += x[1].calls
			x[0].first += x[1].first
			x[0].same += x[1].same
			x[0].a += x[1].a
			x[0].b += x[1].b
		}
	}
	// The corpus must exercise the new code: some calls shrink, many are untouched.
	if all.a == 0 || allPlayers.a == 0 || all.first == 0 || allPlayers.first == 0 {
		t.Errorf("語料沒有涵蓋縮小：NPC %+v 玩家 %+v", all, allPlayers)
	}
}

// Spec 056 §5.3 and §3.8 item 1 on the formal text/ catalogs (environment
// gate): every sentence with a name unit of the four languages, and the
// player names the lanes can show, in the windows of phase-304 §2.  At most
// 2,000 sampled calls per language and window (seed 56); no digest, text/
// changes with proofreading, the receipt records the text/ commit.
func TestShrinkInvariantFormalText(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	fonts := map[string]string{LangZhCN: os.Getenv("BUCKROGERS_ZHCN_FONT"), LangJa: os.Getenv("BUCKROGERS_JA_FONT"), LangKo: os.Getenv("BUCKROGERS_KO_FONT")}
	tw := os.Getenv("BUCKROGERS_ZHTW_FONT")
	if root == "" || tw == "" || fonts[LangZhCN] == "" || fonts[LangJa] == "" || fonts[LangKo] == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 與四個語言字型環境變數未設定")
	}
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: root + "/text", FontPath: tw,
		Langs: []string{LangZhCN, LangJa, LangKo}, LangFonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	const perGeo = 2000
	var sumPlace, sumPlayer invTally
	for _, lang := range []string{LangZhTW, LangZhCN, LangJa, LangKo} {
		i := r.laneIndex(lang)
		if i < 0 {
			t.Fatalf("語言 %s 沒有載入：%v", lang, r.off)
		}
		l := r.lanes[i]
		w, names := l.ecl, l.names
		if w == nil || names == nil || l.players == nil {
			t.Fatalf("%s：ecl %v names %v players %v", lang, w != nil, names != nil, l.players != nil)
		}
		type sentence struct{ key, text string }
		var ss []sentence
		for _, k := range w.catalog.Keys() {
			txt := w.catalog.text[k]
			if v := names.Variants(txt, k, NameCaseUpper); len(v) > 0 && len(v[0].Units) > 0 {
				ss = append(ss, sentence{k, txt})
			}
		}
		if len(ss) == 0 {
			t.Fatalf("%s：沒有含名字單元的句子", lang)
		}
		rnd := rand.New(rand.NewSource(56))
		sort.Slice(ss, func(a, b int) bool { return ss[a].key < ss[b].key })
		var place invTally
		var leads = []rune{0}
		if lang == LangKo {
			leads = []rune{0, '각', '가'}
		}
		for _, g := range pre056Geos {
			for n := 0; n < perGeo; n++ {
				s := ss[rnd.Intn(len(ss))]
				c := pre056PlaceCase{txt: s.text, lead: leads[rnd.Intn(len(leads))], geo: g,
					row: g[1] + uint8(rnd.Intn(int(g[3]-g[1])+1)),
					col: eclUnitLeft(g[0]) + uint8(rnd.Intn(int(eclUnitRight(g[2])-eclUnitLeft(g[0]))+1))}
				if w.layout != nil && w.layout.word && rnd.Intn(2) == 0 {
					c.spaceNeeded, c.prevRune = true, []rune{'A', ' ', 0}[rnd.Intn(3)]
				}
				if w.catalog.zhDeckSpace() && s.text != "" && s.text[0] >= '0' && s.text[0] <= '9' {
					c.deck = rnd.Intn(2) == 0
				}
				cur := w.placeText(c.txt, s.key, false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())
				old := w.placeTextPre056(c.txt, s.key, false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())
				place.classifyPlace(c, old, cur)
			}
		}
		place.check(t, lang+" placeText（text/ 語料，句子 "+fmt.Sprint(len(ss))+"）")
		// Player names: every glossary person and a few plain names.
		var pl [][2]string
		seen := map[string]bool{}
		add := func(en string) {
			if seen[en] {
				return
			}
			seen[en] = true
			for _, g := range []translit.Gender{translit.Male, translit.Female} {
				if zh, ok := l.players.Chinese(en, g); ok && zh != "" {
					pl = append(pl, [2]string{zh, en})
					return
				}
			}
		}
		for _, e := range names.names {
			add(e.English)
		}
		for _, en := range []string{"CELESTE", "FLAVIUS", "MARION", "ARCHIMEDES", "ALEXANDER", "ELIZABETH", "FREDERICK", "BARTHOLOMEW", "AB", "X"} {
			add(en)
		}
		if len(pl) == 0 {
			t.Fatalf("%s：沒有可顯示的玩家名", lang)
		}
		var player invTally
		for _, g := range pre056Geos {
			for n := 0; n < perGeo; n++ {
				p := pl[rnd.Intn(len(pl))]
				c := pre056PlayerCase{zh: p[0], en: p[1], geo: g,
					row:  g[1] + uint8(rnd.Intn(int(g[3]-g[1])+1)),
					col:  eclUnitLeft(g[0]) + uint8(rnd.Intn(int(eclUnitRight(g[2])-eclUnitLeft(g[0]))+1)),
					left: eclUnitLeft(g[0]), right: eclUnitRight(g[2]), b: g[3]}
				c.space = w.layout != nil && w.layout.word && rnd.Intn(2) == 0
				cur := layoutPlayerName(w.layout, c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)
				player.classifyPlayer(c, layoutPlayerNamePre056(w.layout, c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b), cur)
			}
		}
		player.check(t, lang+" layoutPlayerName（text/ 語料，玩家名 "+fmt.Sprint(len(pl))+"）")
		for _, x := range [][2]*invTally{{&sumPlace, &place}, {&sumPlayer, &player}} {
			x[0].calls += x[1].calls
			x[0].first += x[1].first
			x[0].same += x[1].same
			x[0].a += x[1].a
			x[0].b += x[1].b
		}
	}
	t.Logf("合計 NPC：呼叫 %d、第一狀態 L0 %d、輸出相同 %d、類 a %d、類 b %d", sumPlace.calls, sumPlace.first, sumPlace.same, sumPlace.a, sumPlace.b)
	t.Logf("合計玩家名：呼叫 %d、第一狀態 L0 %d、輸出相同 %d、類 a %d、類 b %d", sumPlayer.calls, sumPlayer.first, sumPlayer.same, sumPlayer.a, sumPlayer.b)
}
