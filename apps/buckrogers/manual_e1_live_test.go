package buckrogers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Buck repo spec 053: E1 on the live path.  The synthetic tests build the
// presenter and the lane by hand; the formal tests (BUCKROGERS_CHT_ROOT) run
// the real catalog.  No test text is a manual word.

func manualE1LiveCatalog(texts ...string) *Catalog {
	c := &Catalog{byIdentity: map[identity]catalogEntry{}}
	for i, text := range texts {
		n := strconv.Itoa(i + 1)
		c.byIdentity[identity{page: 1, heading: "Fixture" + n, ordinal: i + 1}] = catalogEntry{
			eventKey: "manual.fixture.word" + n, textKey: "manual.fixture" + n, translation: text,
		}
	}
	return c
}

// manualE1LiveLane builds a zh-TW lane with only the manual family.
func manualE1LiveLane(t *testing.T, catalog *Catalog, base *xlate.Font) *liveLane {
	t.Helper()
	l := &liveLane{lang: LangZhTW, font: base, manCatalog: catalog,
		resets: map[string]int{}, rebuilds: map[string]int{}, skips: map[string]int{}}
	layout := loadManualOverlayLayout(t)
	w := NewWatcher(catalog)
	for i, scale := range liveScales {
		p, err := NewRuntimeManualOverlayLang(layout, catalog, base, scale, LangZhTW)
		if err != nil {
			t.Fatal(err)
		}
		l.manPres[i] = p
		l.setupManualE1(i)
		consumer, err := NewManualPresentationConsumer(p)
		if err != nil {
			t.Fatal(err)
		}
		if l.manSync[i], err = NewManualPresentationBridge(w, consumer); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

func manualE1LiveEvents(w *Watcher, generation uint64, entry catalogEntry) {
	w.presentation = append(w.presentation,
		ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: generation},
		ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: generation,
			Request: DisplayRequest{Generation: generation, EventKey: entry.eventKey, TextKey: entry.textKey}})
}

func manualE1LiveTexts() []string {
	// 33 full-width characters take 66 of the 72 units; the word after them
	// is split by the fixed cells and moved whole by E1.
	return []string{
		strings.Repeat("字", 33) + "ABCDEFGH" + "字字",
		"繁中 ALPHA 與 BETA 並列。",
	}
}

func TestManualE1LivePreflightEnablesOnlyZhTW3x(t *testing.T) {
	texts := manualE1LiveTexts()
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	if l.manE1 != manualE1On || !l.manualE1Active() {
		t.Fatalf("zh-TW 3×：status=%q active=%v", l.manE1, l.manualE1Active())
	}
	for i, scale := range liveScales {
		if on := l.manPres[i].e1Base != nil; on != (scale == 3) {
			t.Errorf("%d×：e1Base=%v", scale, on)
		}
	}
	if len(l.manE1Rows) != len(texts) {
		t.Errorf("rows=%v", l.manE1Rows)
	}
	// Other languages never get E1 (spec 053 §3.1).
	for _, lang := range []string{LangZhCN, LangJa, LangKo, LangTest} {
		o := &liveLane{lang: lang, font: base, manCatalog: catalog}
		layout := loadManualOverlayLayout(t)
		p, err := NewRuntimeManualOverlayLang(layout, catalog, base, 3, lang)
		if err != nil {
			t.Fatal(err)
		}
		o.manPres[1] = p
		o.setupManualE1(1)
		if p.e1Base != nil || o.manE1 != "" {
			t.Errorf("%s 不得啟用 E1：status=%q", lang, o.manE1)
		}
	}
}

func TestManualE1LiveAppliesWholeWords(t *testing.T) {
	texts := manualE1LiveTexts()
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	var key catalogEntry
	for _, e := range catalog.byIdentity {
		if e.translation == texts[0] {
			key = e
		}
	}
	draw := func(on bool) []byte {
		p := l.manPres[1]
		saved := p.e1Base
		if !on {
			p.e1Base = nil
		}
		defer func() { p.e1Base = saved }()
		p.state, p.generation = manualOverlayCleared, 0
		if err := p.SetStyle(ManualTextStyle{Background: 0, Foreground: 10}); err != nil {
			t.Fatal(err)
		}
		if err := p.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1}); err != nil {
			t.Fatal(err)
		}
		req := DisplayRequest{Generation: 1, EventKey: key.eventKey, TextKey: key.textKey}
		if err := p.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: req}); err != nil {
			t.Fatal(err)
		}
		if on != (p.e1Plan != nil) {
			t.Fatalf("on=%v e1Plan=%v", on, p.e1Plan != nil)
		}
		var palette [256][3]uint8
		palette[10] = [3]uint8{85, 255, 85}
		indexed := make([]byte, 320*200)
		p.Frame(indexed, palette)
		rgba, missing, ok := p.Draw(indexed, palette)
		if len(missing) != 0 || !ok || rgba == nil {
			t.Fatalf("draw on=%v missing=%q ok=%v", on, string(missing), ok)
		}
		if err := p.Apply(ManualPresentationEvent{Kind: ManualPresentationClear, Generation: 1}); err != nil {
			t.Fatal(err)
		}
		if len(p.ActiveKeys()) != 0 {
			t.Fatal("離頁後 ActiveKeys 應為空")
		}
		return rgba
	}
	if string(draw(true)) == string(draw(false)) {
		t.Fatal("跨列英文詞：E1 與固定格應不同")
	}
}

func TestManualE1LivePreflightFailureKeepsFixedCells(t *testing.T) {
	// 15 words of 60 letters: the fixed cells need 13 rows, E1 needs one row
	// per word, 15 rows.
	word := strings.Repeat("W", 60)
	words := make([]string, 15)
	for i := range words {
		words[i] = word
	}
	texts := []string{strings.Join(words, " "), "繁中。"}
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	if l.manPres[1].e1Base != nil || l.manE1 != "off(preflight:1)" {
		t.Fatalf("status=%q e1Base=%v", l.manE1, l.manPres[1].e1Base != nil)
	}
	if l.manPres[1] == nil || l.manPres[0].e1Base != nil {
		t.Fatal("固定格 presenter 應照常建立")
	}
}

func TestManualE1LiveKeywordCoexistence(t *testing.T) {
	texts := manualE1LiveTexts()
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	plan := []manualEnglishRow{{row: 3, text: []rune("X")}}
	m := &ManualEnglish{plans: map[string][]manualEnglishRow{
		"manual.fixture.word1": plan, "manual.fixture.word2": plan}, Excluded: map[string]string{}}
	l.checkManualE1Keyword(m)
	if l.manE1 != manualE1On || l.manPres[1].e1Base == nil {
		t.Fatalf("k' ≤ k 時 E1 應保持：%q", l.manE1)
	}
	// A question without a keyword row is not checked, even when E1 is longer.
	l.manE1Rows["manual.fixture.word1"] = 99
	l.checkManualE1Keyword(&ManualEnglish{plans: map[string][]manualEnglishRow{"manual.fixture.word2": plan}})
	if l.manE1 != manualE1On {
		t.Fatalf("沒有關鍵字列的題不應檢查：%q", l.manE1)
	}
	l.checkManualE1Keyword(m)
	if l.manE1 != "off(keyword:1)" || l.manPres[1].e1Base != nil {
		t.Fatalf("k' > k 時 E1 應整體關閉：%q", l.manE1)
	}
}

func TestManualE1LiveRuntimeFallback(t *testing.T) {
	texts := manualE1LiveTexts()
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	w := l.manSync[1].watcher
	var entry catalogEntry
	for _, e := range catalog.byIdentity {
		if e.translation == texts[0] {
			entry = e
		}
	}
	manualE1LiveEvents(w, 1, entry)
	// Inject: an E1 base of the wrong size makes the plan fail; the fixed
	// cells are untouched (spec 053 §5.1).
	good := l.manPres[1].e1Base
	l.manPres[1].e1Base = &xlate.Font{Name: "bad", W: 8, H: 8, Glyphs: map[rune][]byte{}}
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	if l.manE1 != manualE1Runtime || l.manPres[1].e1Base != nil || l.resets["manual"] != 0 {
		t.Fatalf("status=%q e1Base=%v resets=%v", l.manE1, l.manPres[1].e1Base != nil, l.resets)
	}
	if len(l.manPres[1].ActiveKeys()) == 0 || len(l.manPres[0].ActiveKeys()) == 0 {
		t.Fatal("退路後兩個倍率都應顯示")
	}
	_ = good

	// A failure that is not E1's: both attempts fail, E1 stays as it was and
	// the reset is counted once (no retry loop).
	l2 := manualE1LiveLane(t, catalog, base)
	w2 := l2.manSync[1].watcher
	w2.presentation = append(w2.presentation,
		ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1},
		ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1,
			Request: DisplayRequest{Generation: 1, EventKey: "no.such.key", TextKey: "no.such"}})
	l2.syncManual(ManualTextStyle{}, false, len(w2.presentation))
	if l2.manPres[1].e1Base == nil || l2.manE1 != manualE1On || l2.resets["manual"] != 2 {
		t.Fatalf("status=%q e1Base=%v resets=%v", l2.manE1, l2.manPres[1].e1Base != nil, l2.resets)
	}
}

// The formal tests below load the real catalog and the local work font.

func manualE1FormalRuntime(t *testing.T) *LiveRuntime {
	t.Helper()
	text, font := liveLangInputs(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: font})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestManualE1LiveFormalCatalogPreflight(t *testing.T) {
	r := manualE1FormalRuntime(t)
	l := r.lanes[0]
	if !strings.Contains(r.DebugSummary(), " manual-e1=on") || !l.manualE1Active() {
		t.Fatalf("zh-TW 3× 應啟用 E1：%q", l.manE1)
	}
	if len(l.manE1Rows) != 39 {
		t.Fatalf("預檢題數 = %d，應為 39", len(l.manE1Rows))
	}
	if l.manPres[0].e1Base != nil {
		t.Fatal("2× 不得啟用 E1")
	}
	// k' ≤ k for every question (spec 053 §3.4): equal or shorter, counted.
	same, shorter, longer := 0, 0, 0
	for _, e := range l.manCatalog.byIdentity {
		paragraph, err := manualRows(e.translation, manualEnglishColumns, manualEnglishLastRow+1)
		if err != nil {
			t.Fatal(err)
		}
		k, e1 := manualUsedRows(paragraph), l.manE1Rows[e.eventKey]
		switch {
		case e1 == k:
			same++
		case e1 < k:
			shorter++
		default:
			longer++
		}
	}
	t.Logf("E1 列數與固定格：相同 %d、較少 %d、較多 %d", same, shorter, longer)
	if longer != 0 {
		t.Errorf("%d 題的 E1 列數大於固定格（關鍵字列並存條件不成立）", longer)
	}
}

func TestManualE1LiveFormalComposeSurvivesBrokenDraw(t *testing.T) {
	r := manualE1FormalRuntime(t)
	l := r.lanes[0]
	p := l.manPres[1]
	var entry catalogEntry
	for _, e := range p.catalog.byIdentity {
		entry = e
		break
	}
	if err := p.SetStyle(ManualTextStyle{Background: 0, Foreground: 10}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []ManualPresentationEvent{
		{Kind: ManualPresentationBegin, Generation: 1},
		{Kind: ManualPresentationRequest, Generation: 1, Request: DisplayRequest{Generation: 1, EventKey: entry.eventKey, TextKey: entry.textKey}},
	} {
		if err := p.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	var palette [256][3]uint8
	palette[10] = [3]uint8{85, 255, 85}
	indexed := make([]byte, 320*200)
	r.Frame(indexed, palette)
	out, drawn, err := r.ComposeWith(indexed, palette, 3)
	if err != nil || !drawn || len(out) != 960*600*4 {
		t.Fatalf("compose err=%v drawn=%v len=%d", err, drawn, len(out))
	}
	// Break the derived font after the plan was sealed: DrawChecked fails.
	for _, g := range p.font.Glyphs {
		for i := range g {
			g[i] ^= 0xff
		}
		break
	}
	p.font.Name = "changed"
	before := l.skips["manual"]
	out2, _, err := r.ComposeWith(indexed, palette, 3)
	if err != nil || len(out2) != 960*600*4 {
		t.Fatalf("compose err=%v len=%d", err, len(out2))
	}
	if l.skips["manual"] != before+1 {
		t.Errorf("skips[manual] = %d，應加一", l.skips["manual"])
	}
}

func TestManualE1LiveFormalKeywordRowsCoexist(t *testing.T) {
	r := manualE1FormalRuntime(t)
	text := filepath.Join(os.Getenv("BUCKROGERS_CHT_ROOT"), "text")
	if _, err := os.Stat(filepath.Join(text, "manual-english-panel.zh-TW.tsv")); err != nil {
		t.Skip("沒有 manual-english-panel.zh-TW.tsv")
	}
	// Placeholder words only: "Wxyz" is not a word of the manual.
	var b strings.Builder
	b.WriteString("event_key\tordinal\twords\n")
	l := r.lanes[0]
	for id, e := range l.manCatalog.byIdentity {
		if id.ordinal < 1 || id.ordinal > 40 {
			continue
		}
		b.WriteString(e.eventKey + "\t" + strconv.Itoa(id.ordinal) + "\t" + strings.TrimSpace(strings.Repeat("Wxyz ", id.ordinal)) + "\n")
	}
	path := filepath.Join(t.TempDir(), "excerpt.tsv")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.SetManualEnglish(path); err != nil {
		t.Fatal(err)
	}
	if r.manEng == nil || r.manEng.Len() == 0 {
		t.Fatalf("摘錄應載入：off=%q", r.manEngOff)
	}
	if !strings.Contains(r.DebugSummary(), " manual-e1=on") {
		t.Fatalf("摘錄載入後 E1 應保持：%s", r.DebugSummary())
	}
	p := l.manPres[1]
	if err := p.SetStyle(ManualTextStyle{Background: 0, Foreground: 10}); err != nil {
		t.Fatal(err)
	}
	shown := 0
	var palette [256][3]uint8
	for _, e := range l.manCatalog.byIdentity {
		if _, has := r.manEng.plans[e.eventKey]; !has {
			continue
		}
		gen := uint64(shown + 1)
		for _, ev := range []ManualPresentationEvent{
			{Kind: ManualPresentationBegin, Generation: gen},
			{Kind: ManualPresentationRequest, Generation: gen, Request: DisplayRequest{Generation: gen, EventKey: e.eventKey, TextKey: e.textKey}},
		} {
			if err := p.Apply(ev); err != nil {
				t.Fatal(err)
			}
		}
		if p.e1Plan == nil {
			t.Fatalf("E1 應啟用於 %s", e.textKey)
		}
		if !r.manEngPres[1].sync(r.manEng, p, palette) {
			t.Fatalf("E1 下關鍵字列應照畫：%s", e.textKey)
		}
		if err := p.Apply(ManualPresentationEvent{Kind: ManualPresentationClear, Generation: gen}); err != nil {
			t.Fatal(err)
		}
		shown++
	}
	if shown != r.manEng.Len() {
		t.Errorf("畫出 %d 題，摘錄有 %d 題", shown, r.manEng.Len())
	}
}
