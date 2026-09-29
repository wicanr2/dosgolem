package buckrogers

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

const testManualPanel = "key\ttranslation\tsource\nmanual.english.long\t英文原文第{0}字（請用大寫輸入）：\truntime-interface\nmanual.english.short\t英文第{0}字（大寫）：\truntime-interface\n"

func testManualEnglishCatalog(paragraph string) *Catalog {
	return &Catalog{byIdentity: map[identity]catalogEntry{
		{page: 1, heading: "Alpha", ordinal: 3}: {eventKey: "manual.page1.alpha.word3", textKey: "t1", translation: paragraph},
		{page: 2, heading: "Beta", ordinal: 1}:  {eventKey: "manual.page2.beta.word1", textKey: "t2", translation: "短。"},
	}}
}

func fontCovering(extra string, skip rune) *xlate.Font {
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for r := rune(0x21); r <= 0x7E; r++ {
		f.Glyphs[r] = make([]byte, 32)
	}
	for _, r := range extra + testManualPanel + "「」" {
		f.Glyphs[r] = make([]byte, 32)
	}
	delete(f.Glyphs, skip)
	return f
}

func TestManualEnglishTemplates(t *testing.T) {
	if _, _, err := manualEnglishTemplates([]byte(testManualPanel)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"英文第字：", "英文第{0}{0}字：", "英文第{0}字{x}："} {
		panel := strings.Replace(testManualPanel, "英文第{0}字（大寫）：", bad, 1)
		if _, _, err := manualEnglishTemplates([]byte(panel)); err == nil {
			t.Fatalf("template %q accepted", bad)
		}
	}
}

func TestManualEnglishLoadRejectsBadFiles(t *testing.T) {
	c := testManualEnglishCatalog("段落。")
	fonts := []*xlate.Font{fontCovering("段落。短", 0)}
	head := "# provenance\nevent_key\tordinal\twords\n"
	good := "manual.page1.alpha.word3\t3\t\"Aa Bb CC\n"
	if m, err := LoadManualEnglish([]byte(head+good), c, []byte(testManualPanel), fonts); err != nil || m.Len() != 1 {
		t.Fatalf("good file: %v", err)
	}
	for name, body := range map[string]string{
		"event-key":   "manual.unknown.word3\t3\tAa Bb CC\n",
		"duplicate":   good + good,
		"ordinal":     "manual.page1.alpha.word3\t2\tAa Bb\n",
		"word-count":  "manual.page1.alpha.word3\t3\tAa Bb\n",
		"ascii":       "manual.page1.alpha.word3\t3\tAa Bé CC\n",
		"word-length": "manual.page1.alpha.word3\t3\tAa Bb ABCDEFGHI\n",
		"format":      "manual.page1.alpha.word3\t3\n",
	} {
		_, err := LoadManualEnglish([]byte(head+body), c, []byte(testManualPanel), fonts)
		if err == nil || !strings.HasPrefix(err.Error(), "manual-english: ") {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(err.Error(), "Aa") || strings.Contains(err.Error(), "CC") {
			t.Fatalf("%s: error leaks words", name)
		}
	}
}

func TestManualEnglishExcludesPerQuestion(t *testing.T) {
	head := "event_key\tordinal\twords\n"
	body := "manual.page1.alpha.word3\t3\tAa Bb CC\nmanual.page2.beta.word1\t1\tQQ\n"
	// Missing glyph for 'Q' excludes only the second question.
	c := testManualEnglishCatalog("段落。")
	m, err := LoadManualEnglish([]byte(head+body), c, []byte(testManualPanel), []*xlate.Font{fontCovering("段落。短", 'Q')})
	if err != nil || m.Len() != 1 || m.Excluded["manual.page2.beta.word1"] != "font" {
		t.Fatalf("font exclusion: %v %+v", err, m)
	}
	// A 13-row paragraph leaves no room: layout exclusion.
	c = testManualEnglishCatalog(strings.Repeat("字", 36*13))
	m, err = LoadManualEnglish([]byte(head+body), c, []byte(testManualPanel), []*xlate.Font{fontCovering("字短", 0)})
	if err != nil || m.Excluded["manual.page1.alpha.word3"] != "layout" {
		t.Fatalf("layout exclusion: %v %+v", err, m)
	}
}

func TestManualEnglishLayout(t *testing.T) {
	long, short, _ := manualEnglishTemplates([]byte(testManualPanel))
	// Short paragraph: long form on row k+1, last word bracketed.
	rows, ok := manualEnglishLayout(2, []string{"One", "two", "THREE"}, long, short)
	if !ok || len(rows) != 1 || rows[0].row != 3 || !strings.HasSuffix(string(rows[0].text), "one two 「THREE」") && !strings.HasSuffix(string(rows[0].text), "One two 「THREE」") {
		t.Fatalf("long: %+v", rows)
	}
	// Long words wrap by word; the bracketed unit never splits.
	words := []string{"Aaaaaaaa", "Bbbbbbbb", "Cccccccc", "Dddddddd", "Eeeeeeee", "FFFFFFFF"}
	rows, ok = manualEnglishLayout(0, words, long, short)
	if !ok || len(rows) < 2 {
		t.Fatalf("wrap: %+v", rows)
	}
	for _, r := range rows {
		if textUnits(r.text) > 72 {
			t.Fatalf("row too wide: %d units", textUnits(r.text))
		}
	}
	if last := string(rows[len(rows)-1].text); !strings.HasSuffix(last, "「FFFFFFFF」") {
		t.Fatalf("bracket split: %q", last)
	}
	// Only row 13 left: long form does not fit, short form does.
	rows, ok = manualEnglishLayout(12, words, long, short)
	if !ok || len(rows) != 1 || rows[0].row != 13 || !strings.HasPrefix(string(rows[0].text), "英文第6字") {
		t.Fatalf("short: %+v", rows)
	}
	if _, ok := manualEnglishLayout(13, words, long, short); ok {
		t.Fatal("no room accepted")
	}
}

func TestManualEnglishFollowsVisibleRequest(t *testing.T) {
	c := testManualEnglishCatalog("段落。")
	font := fontCovering("段落。短", 0)
	m, err := LoadManualEnglish([]byte("event_key\tordinal\twords\nmanual.page1.alpha.word3\t3\tAa Bb CC\n"), c, []byte(testManualPanel), []*xlate.Font{font})
	if err != nil {
		t.Fatal(err)
	}
	layout := confirmedManualOverlayLayout
	man := &RuntimeManualOverlay{layout: &layout, font: font, scale: 2, state: manualOverlayVisible, generation: 5,
		actions: []ManualOverlayAction{{Generation: 5, EventKey: "manual.page1.alpha.word3"}}}
	var p manualEnglishPresenter
	var pal [256][3]uint8
	if p.sync(m, man, pal) {
		t.Fatal("drew without style")
	}
	man.style = &ManualTextStyle{Background: 0, Foreground: 10}
	if !p.sync(m, man, pal) || len(dedupRowKeys(p.layer.Stamps)) != 1 {
		t.Fatalf("visible: %+v", p.layer)
	}
	for _, s := range p.layer.Stamps {
		if s.Y != 72+8*2 {
			t.Fatalf("visible row y=%d", s.Y)
		}
	}
	for _, s := range p.layer.Stamps {
		if strings.Contains(s.Key, "CC") || strings.Contains(s.Key, "Aa") {
			t.Fatal("stamp key leaks words")
		}
	}
	// Cleared: actions still hold the old entry, but nothing is visible.
	man.state = manualOverlayCleared
	if g, _, ok := man.VisibleRequest(); ok || g != 0 {
		t.Fatal("VisibleRequest true after clear")
	}
	if p.sync(m, man, pal) || p.layer != nil {
		t.Fatal("presenter kept rows after clear")
	}
	// A new request not in the excerpt shows nothing.
	man.state, man.generation = manualOverlayVisible, 6
	man.actions = append(man.actions, ManualOverlayAction{Generation: 6, EventKey: "manual.page2.beta.word1"})
	if p.sync(m, man, pal) {
		t.Fatal("drew for a question outside the excerpt")
	}
	// Generation mismatch (pending new request) shows nothing.
	man.generation = 7
	if _, _, ok := man.VisibleRequest(); ok {
		t.Fatal("stale action accepted")
	}
}
