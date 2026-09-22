package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var validManualEvents = []struct {
	caller Address
	text   string
}{
	{Address{0x2A33, 0x021B}, "41"},
	{Address{0x2A33, 0x0231}, "following the heading"},
	{Address{0x2A33, 0x027A}, "Technical Skills"},
	{Address{0x2A33, 0x02A5}, "what is the"},
	{Address{0x2A33, 0x02E2}, "second"},
	{Address{0x2A33, 0x0309}, "word?"},
}

func startCollector(t *testing.T, c *Collector) uint64 {
	t.Helper()
	g, ok := c.BeginEntry(manualBegin, manualBeginText)
	if !ok {
		t.Fatal("精確題首應建立 generation")
	}
	return g
}

func feedCollector(c *Collector, generation uint64, events []struct {
	caller Address
	text   string
}) (Question, bool) {
	var q Question
	var ok bool
	for _, event := range events {
		q, ok = c.PostCall(generation, event.caller, event.text)
	}
	return q, ok
}

func TestCollectorPhase18Sequence(t *testing.T) {
	c := &Collector{}
	g := startCollector(t, c)
	for _, event := range validManualEvents[:2] {
		c.PostCall(g, event.caller, event.text)
	}
	c.ClearEntry(manualClear)
	q, ok := feedCollector(c, g, validManualEvents[2:])
	want := Question{Page: 41, Heading: "Technical Skills", OrdinalWord: "second"}
	if !ok || q != want {
		t.Fatalf("完成題目 = %#v, %v；要 %#v, true", q, ok, want)
	}
	if visible, ok := c.Visible(); !ok || visible != want {
		t.Fatalf("visible = %#v, %v", visible, ok)
	}
}

func TestCollectorBeginAndClearLifecycle(t *testing.T) {
	c := &Collector{}
	g := startCollector(t, c)
	if _, ok := feedCollector(c, g, validManualEvents); !ok {
		t.Fatal("第一題應完成")
	}
	if _, ok := c.BeginEntry(Address{0xFFFF, 0xFFFF}, manualBeginText); ok {
		t.Fatal("未知 begin 不應建立 generation")
	}
	if _, ok := c.Visible(); !ok {
		t.Fatal("未知 begin 不應清除 visible")
	}
	g2 := startCollector(t, c)
	if g2 != 2 {
		t.Fatalf("第二個 generation = %d", g2)
	}
	if _, ok := c.Visible(); ok {
		t.Fatal("精確 begin 應立即清除 visible")
	}
	if _, ok := feedCollector(c, g2, validManualEvents); !ok {
		t.Fatal("第二題應完成")
	}
	c.ClearEntry(manualClear)
	if _, ok := c.Visible(); ok {
		t.Fatal("精確 clear 應清除 visible")
	}
}

func TestCollectorFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		first struct {
			caller Address
			text   string
		}
	}{
		{"亂序", validManualEvents[2]},
		{"未知 caller", struct {
			caller Address
			text   string
		}{Address{0xFFFF, 0xFFFF}, "41"}},
		{"非 ASCII 頁碼", struct {
			caller Address
			text   string
		}{validManualEvents[0].caller, "４１"}},
		{"零頁碼", struct {
			caller Address
			text   string
		}{validManualEvents[0].caller, "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Collector{}
			g := startCollector(t, c)
			c.PostCall(g, tc.first.caller, tc.first.text)
			if _, ok := feedCollector(c, g, validManualEvents); ok {
				t.Fatal("poisoned generation 不得提交")
			}
			g2 := startCollector(t, c)
			if _, ok := feedCollector(c, g2, validManualEvents); !ok {
				t.Fatal("下一個精確 begin 應復原")
			}
		})
	}
}

func TestCollectorWrongLiteralPoisonsGeneration(t *testing.T) {
	c := &Collector{}
	g := startCollector(t, c)
	c.PostCall(g, validManualEvents[0].caller, validManualEvents[0].text)
	c.PostCall(g, validManualEvents[1].caller, "following a similar heading")
	if _, ok := feedCollector(c, g, validManualEvents[2:]); ok {
		t.Fatal("錯誤固定文句後不得提交")
	}
}

func TestCollectorMissingDuplicateAndStale(t *testing.T) {
	c := &Collector{}
	g1 := startCollector(t, c)
	if _, ok := feedCollector(c, g1, validManualEvents[:5]); ok {
		t.Fatal("缺題尾不得提交")
	}
	g2 := startCollector(t, c)
	c.PostCall(g2, validManualEvents[0].caller, validManualEvents[0].text)
	c.PostCall(g2, validManualEvents[0].caller, validManualEvents[0].text)
	if _, ok := feedCollector(c, g2, validManualEvents[1:]); ok {
		t.Fatal("重複 page 應 poison")
	}
	g3 := startCollector(t, c)
	if _, ok := c.PostCall(g2, validManualEvents[0].caller, validManualEvents[0].text); ok {
		t.Fatal("stale generation 不得提交")
	}
	if _, ok := feedCollector(c, g3, validManualEvents); !ok {
		t.Fatal("stale generation 不得污染新題")
	}
}

func ordinalFixture() string {
	var b strings.Builder
	b.WriteString("number\tordinal_ascii\truntime_address\tslot_hex\n")
	words := []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth"}
	for i, word := range words {
		fmt.Fprintf(&b, "%d\t%s\t0EC0:%04X\t00\n", i+1, word, 0x33AE+i*0x13)
	}
	return b.String()
}

const eventFixture = "event_key\trecord_index\tpage\theading_ascii\tordinal\ttext_key\n" +
	"manual.page34.deimos_prison.word10\t32\t34\tDeimos Prison\t10\tmanual.log.49.deimos_prison\n"

const textFixture = "key\ttranslation\tsource\n" +
	"manual.log.49.deimos_prison\t繁中段落\tmanual-and-runtime\n"

func loadFixture(t *testing.T, events, ordinals, texts string) *Catalog {
	t.Helper()
	c, err := LoadCatalog([]byte(events), []byte(ordinals), []byte(texts))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	return c
}

func TestCatalogExactResolveAndDisplayFields(t *testing.T) {
	c := loadFixture(t, eventFixture, ordinalFixture(), textFixture)
	r, ok := c.Resolve(1, 1, Question{34, "Deimos Prison", "tenth"})
	if !ok || r.TextKey != "manual.log.49.deimos_prison" || r.Translation != "繁中段落" {
		t.Fatalf("resolve = %#v, %v", r, ok)
	}
	typ := reflect.TypeOf(DisplayRequest{})
	if typ.NumField() != 4 {
		t.Fatalf("DisplayRequest 欄位數 = %d，要 4", typ.NumField())
	}
	for i, want := range []string{"Generation", "EventKey", "TextKey", "Translation"} {
		if typ.Field(i).Name != want {
			t.Fatalf("欄位 %d = %s，要 %s", i, typ.Field(i).Name, want)
		}
	}
}

func TestCatalogResolveFailsClosed(t *testing.T) {
	c := loadFixture(t, eventFixture, ordinalFixture(), textFixture)
	for _, tc := range []struct {
		generation, current uint64
		q                   Question
	}{
		{0, 0, Question{34, "Deimos Prison", "tenth"}},
		{1, 2, Question{34, "Deimos Prison", "tenth"}},
		{1, 1, Question{34, "deimos prison", "tenth"}},
		{1, 1, Question{34, "Deimos Prison", "10th"}},
		{1, 1, Question{41, "Technical Skills", "second"}},
	} {
		if got, ok := c.Resolve(tc.generation, tc.current, tc.q); ok {
			t.Errorf("不應命中：%#v", got)
		}
	}
}

func TestCatalogRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name, events, ordinals, texts string
	}{
		{"BOM", eventFixture, ordinalFixture(), "\ufeff" + textFixture},
		{"重複 identity", eventFixture + strings.SplitN(eventFixture, "\n", 2)[1], ordinalFixture(), textFixture},
		{"重複 event key", eventFixture + "manual.page34.deimos_prison.word10\t33\t35\tOther\t9\tmanual.other\n", ordinalFixture(), textFixture + "manual.other\t其他\ttest\n"},
		{"重複事件文字鍵", eventFixture + "manual.other\t33\t35\tOther\t9\tmanual.log.49.deimos_prison\n", ordinalFixture(), textFixture},
		{"重複 catalog key", eventFixture, ordinalFixture(), textFixture + "manual.log.49.deimos_prison\t重複\ttest\n"},
		{"孤兒 catalog", eventFixture, ordinalFixture(), textFixture + "manual.orphan\t孤兒\ttest\n"},
		{"ordinal 缺號", eventFixture, strings.Replace(ordinalFixture(), "10\ttenth\t0EC0:3459\t00\n", "", 1), textFixture},
		{"ordinal 多對一", eventFixture, strings.Replace(ordinalFixture(), "10\ttenth", "9\ttenth", 1), textFixture},
		{"事件 ordinal 不在橋接表", strings.Replace(eventFixture, "\t10\tmanual", "\t11\tmanual", 1), ordinalFixture(), textFixture},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadCatalog([]byte(tc.events), []byte(tc.ordinals), []byte(tc.texts)); err == nil {
				t.Fatal("應拒絕無效輸入")
			}
		})
	}
	if _, err := LoadCatalog([]byte(eventFixture), []byte(ordinalFixture()), append([]byte(textFixture), 0xFF)); err == nil {
		t.Fatal("應拒絕無效 UTF-8")
	}
}

func TestFormalProjectCatalog(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, "text", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	c, err := LoadCatalog(read("manual-events.tsv"), read("manual-ordinals.tsv"), read("manual.zh-TW.tsv"))
	if err != nil {
		t.Fatalf("正式 TSV：%v", err)
	}
	if got := len(c.byIdentity); got != 39 {
		t.Fatalf("正式手冊題目數 = %d，要 39", got)
	}
	if _, ok := c.Resolve(1, 1, Question{34, "Deimos Prison", "tenth"}); !ok {
		t.Fatal("正式 Deimos 映射應命中")
	}
	if got, ok := c.Resolve(2, 2, Question{41, "Technical Skills", "second"}); !ok || got.EventKey != "manual.page41.technical_skills.word2" || got.TextKey != "manual.rules.technical_skills" {
		t.Fatalf("正式 Technical Skills 映射 = %#v, %v；要精確命中", got, ok)
	}
	if got, ok := c.Resolve(3, 3, Question{42, "Roll.", "fourth"}); !ok || got.EventKey != "manual.page42.roll.word4" || got.TextKey != "manual.rules.roll" {
		t.Fatalf("正式 Roll 映射 = %#v, %v；要精確第 39 題", got, ok)
	}
}
