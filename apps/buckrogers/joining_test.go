package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Buck repo spec 046: the Korean and Japanese join rules.

func TestKoGlue(t *testing.T) {
	cases := []struct {
		a    rune
		out  string
		want bool
		why  string
	}{
		{'S', "은(는)", true, "括號並列形接拉丁字母"},
		{'S', "은(는) 공격", true, "記號後接空白"},
		{')', "의 피해", true, "名字註記括號後接助詞"},
		{'점', "의 피해", true, "助詞接韓文音節"},
		{'」', "라고", false, "라고 不在詞記號清單"},
		{'」', "이라고 말했다", true, "引號後接引用助詞"},
		{'.', "은(는)", false, "A 以句點結尾，詞記號不黏"},
		{'다', ", 일지", true, "標點記號黏，與前字無關"},
		{'」', ", 일지", true, "引號後的逗號黏"},
		{'.', ", ", true, "句點後的逗號黏"},
		{'%', "의", false, "% 不是可附著的前字"},
		{'다', "...하지만", false, "... 開頭不是標點記號"},
		{'.', "이 방은", false, "이 是指示詞，不在清單"},
		{'S', "이 방은", false, "이 是指示詞，名字後也不黏"},
		{'S', "이(가)", true, "이(가) 黏"},
		{'S', "이동한다", false, "이 後接韓文音節不是記號"},
		{'S', "들은", true, "들은 在清單"},
		{'S', "들이받기 성공!", false, "들이 後接韓文音節"},
		{'S', "에서 온", true, "에서"},
		{'S', "에서의", true, "에서의"},
		{'S', "가스", false, "가 後接韓文音節"},
		{'S', "은 blinded", true, "裸露的 은 後接空白"},
		{'S', "", false, "空輸出"},
		{'3', "점의 피해.", false, "점 不是記號"},
		{'S', ". 설치를 확인해 주십시오.", true, "句點加空白開頭"},
	}
	for _, c := range cases {
		if got := koGlue(c.a, c.out); got != c.want {
			t.Errorf("koGlue(%q, %q) = %v，應為 %v（%s）", c.a, c.out, got, c.want, c.why)
		}
	}
}

func TestJaParticleStart(t *testing.T) {
	cases := map[string]bool{
		"は自分の戦術判定を行う。": true, "はそれをかわした": true, "はもうない": true,
		"が操縦を": true, "を持つ": true, "に続く": true, "の武器": true, "と戦う": true, "で破壊された": true, "へ向かう": true,
		"できる": false, "ときどき": false, "ところが": false, "とても強い": false,
		"もういっぱいだ": false, "もっと": false, "も使えない": true, "も": true,
		"や": true, "から来た": true, "からだ": false, "までの": false, "まで": true, "より遅く": true,
		"点のダメージ": false, "": false,
	}
	for out, want := range cases {
		if got := jaParticleStart(out); got != want {
			t.Errorf("jaParticleStart(%q) = %v，應為 %v", out, got, want)
		}
	}
}

// koCatalog is the Korean fixture: the fragments of the sentences the spec
// 046 evidence is about, with Korean translations.
func joiningCatalog(t *testing.T, lang string, tr map[string]string, tpl map[string]string) *EngineTextCatalog {
	t.Helper()
	var origs, pairs []string
	for o, z := range tr {
		origs = append(origs, o)
		pairs = append(pairs, o, z)
	}
	sort.Strings(origs)
	// engineZh wants pairs in any order; keep deterministic anyway.
	h := func(s string) string { x := sha256.Sum256([]byte(s)); return fmt.Sprintf("frag.%x", x[:6]) }
	var ev, tx strings.Builder
	ev.WriteString("event_key\tsignature\n")
	tx.WriteString("key\ttranslation\tsource\n")
	keys := make([]string, 0, len(tpl))
	for k := range tpl {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sigs := map[string]string{
		"tpl.antidote": h("Use antidote on ") + "|_",
		"tpl.drop":     h("Drop ") + "|*|" + h(" forever? "),
		"tpl.logbook":  h(" and you record ") + "|_|" + h(" as logbook entry ") + "|_",
		"tpl.needs":    h("Needs ") + "|_|" + h(" points in ") + "|_",
		"tpl.dup":      h("Dup ") + "|_",
	}
	for _, k := range keys {
		fmt.Fprintf(&ev, "%s\t%s\n", k, sigs[k])
		fmt.Fprintf(&tx, "%s\t%s\tecl-batch-editorial\n", k, tpl[k])
	}
	files := EngineTextFiles{
		Lang:           lang,
		FragmentEvents: engineTSV("frag", origs...),
		FragmentText:   engineZh("frag", pairs...),
		ItemEvents:     engineTSV("item", "Bolt", "Gun"),
		ItemText:       engineZh("item", "Bolt", "볼트", "Gun", "건"),
		MonsterEvents:  engineTSV("monster", "NEO WARRIOR"),
		MonsterText:    engineZh("monster", "NEO WARRIOR", "NEO 전사"),
	}
	if len(keys) > 0 {
		files.TemplateEvents, files.TemplateText = []byte(ev.String()), []byte(tx.String())
	}
	c, err := LoadEngineTextCatalog(files)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var koFragments = map[string]string{
	" makes ": "은(는)", "his": "자신의", " tactics roll.": "전술 판정을 한다.",
	"(from behind) ": "(뒤에서)", "Hitting for ": "명중:", " points ": "점", "of damage": "의 피해",
	" is hit FOR ": "은(는) 공격을 받아", " points of Damage.": "점의 피해.",
	" and you record ": "그리고 기록한다:", " as logbook entry ": "을(를) 일지 항목",
	"Use antidote on ": "", "Drop ": "", " forever? ": "", "Needs ": "", " points in ": "",
	"Dup ": "", "Fires ": "발사:", " at ": "에게",
}

func init() {
	for k, v := range koFragments {
		if v == "" {
			delete(koFragments, k)
			koFragments[k] = "-" // fragments of the templates need some text
		}
	}
}

func TestJoinKoSentences(t *testing.T) {
	c := joiningCatalog(t, LangKo, koFragments, nil)
	cases := []struct{ in, want, why string }{
		{"FLAVIUS makes his tactics roll.", "FLAVIUS은(는) 자신의 전술 판정을 한다.", "A 與 B：片段間補空白，名字與助詞緊接"},
		{"(from behind) Hitting for 3 points of damage", "(뒤에서) 명중: 3 점의 피해", "括號後、片段與數字、助詞 의 黏接"},
		{"ROARKE is hit FOR 3 points of Damage.", "ROARKE은(는) 공격을 받아 3 점의 피해.", "名字加助詞片段"},
		{"Hitting for  2 points of damage", "명중: 2 점의 피해", "`_` 自帶空白不重複補"},
		{"Fires Bolt Gun", "발사: 볼트건", "F 與 I 相鄰補空白"},
		{"Fires Bolt Gun at NEO WARRIOR", "발사: 볼트건에게 NEO 전사", "I 後接助詞 에게 黏、F 與 `_` 補空白"},
		{"ROARKE makes his tactics roll.x", "ROARKE은(는) 자신의 전술 판정을 한다.x", "`_` 緊接在片段後，原文無空白不補"},
	}
	for _, tc := range cases {
		got, ok := c.Translate(tc.in)
		if !ok || got != tc.want {
			t.Errorf("%s：%q → %q %v，應為 %q", tc.why, tc.in, got, ok, tc.want)
		}
	}
	// Another language of the same texts is not touched by the Korean rule.
	z := joiningCatalog(t, LangZhTW, koFragments, nil)
	if got, _ := z.Translate("FLAVIUS makes his tactics roll."); got != "FLAVIUS 은(는)자신의전술 판정을 한다." {
		t.Errorf("zh-TW 串接不應改變：%q", got)
	}
}

func TestJoinJaOnlyChangesNameParticle(t *testing.T) {
	ja := map[string]string{" makes ": "は", "his": "自分の", " tactics roll.": "戦術判定を行う。", " points ": "点", " is hit FOR ": "は", " points of Damage.": "点のダメージを受けた。", " is": "もう"}
	c := joiningCatalog(t, LangJa, ja, nil)
	cases := []struct{ in, want, why string }{
		{"FLAVIUS makes his tactics roll.", "FLAVIUSは自分の戦術判定を行う。", "拉丁字母名字加助詞：不補空白"},
		{"ROARKE is hit FOR 3 points of Damage.", "ROARKEは 3 点のダメージを受けた。", "E 型態不改：數字與片段的空白保留（左貼右空）"},
		{"3 points of Damage.", "3 点のダメージを受けた。", "數字結尾的 `_` 與 F 之間：現行規則（空白在 `_` 內）"},
		{"FLAVIUS is", "FLAVIUS もう", "もう 是普通詞，不當助詞"},
	}
	for _, tc := range cases {
		got, ok := c.Translate(tc.in)
		if !ok || got != tc.want {
			t.Errorf("%s：%q → %q %v，應為 %q", tc.why, tc.in, got, ok, tc.want)
		}
	}
}

func TestTemplateFieldPeriod(t *testing.T) {
	tplKo := map[string]string{
		"tpl.logbook":  ", 일지 항목 {1}번으로 기록한다",
		"tpl.antidote": "{0}에게 해독제 사용",
		"tpl.drop":     "{0}을(를) 영구히 버리겠습니까?",
		"tpl.needs":    "{1}에 {0}점이 필요하다",
	}
	tplJa := map[string]string{
		"tpl.logbook":  "、日誌{1}番に記録した",
		"tpl.antidote": "{0}に解毒剤を使う",
		"tpl.drop":     "{0}を永久に捨てるか？",
		"tpl.needs":    "{1}は{0}点必要だ",
	}
	frags := map[string]string{" and you record ": "a", " as logbook entry ": "b", "Use antidote on ": "c", "Drop ": "d", " forever? ": "e", "Needs ": "f", " points in ": "g"}
	ko := joiningCatalog(t, LangKo, frags, tplKo)
	ja := joiningCatalog(t, LangJa, frags, tplJa)
	zh := joiningCatalog(t, LangZhTW, frags, map[string]string{"tpl.logbook": "，已記入手札，編號 {1}", "tpl.antidote": "對{0}使用解毒劑"})
	type tc struct{ in, ko, ja, zh string }
	for _, c := range []tc{
		{" and you record IT as logbook entry 38.", ", 일지 항목 38번으로 기록한다.", "、日誌38番に記録した。", "，已記入手札，編號 38。"},
		{" and you record IT as logbook entry 38", ", 일지 항목 38번으로 기록한다", "、日誌38番に記録した", "，已記入手札，編號 38"},
		{" and you record IT as logbook entry NO.", ", 일지 항목 NO.번으로 기록한다", "、日誌NO.番に記録した", "，已記入手札，編號 NO."},
		{"Use antidote on 5.", "5에게 해독제 사용.", "5に解毒剤を使う。", "對5使用解毒劑。"},
		{"Use antidote on FLAVIUS", "FLAVIUS에게 해독제 사용", "FLAVIUSに解毒剤を使う", "對FLAVIUS使用解毒劑"},
		{"Needs 5 points in 2.", "2.에 5점이 필요하다", "2.は5点必要だ", ""},
	} {
		if got, _ := ko.Translate(c.in); got != c.ko && c.zh != "" || c.zh == "" && !strings.Contains(got, "필요하다") {
			t.Errorf("ko %q → %q，應為 %q", c.in, got, c.ko)
		}
		if got, _ := ja.Translate(c.in); got != c.ja && c.zh != "" {
			t.Errorf("ja %q → %q，應為 %q", c.in, got, c.ja)
		}
		if c.zh != "" {
			if got, _ := zh.Translate(c.in); got != c.zh {
				t.Errorf("zh-TW %q → %q，應為 %q（規格 047 §3.3）", c.in, got, c.zh)
			}
		}
	}
	// `drop`: the field is a fragment-origin slot; nothing is stripped or added.
	if got, _ := ko.Translate("Drop Bolt Gun forever? "); !strings.HasSuffix(got, "?") || strings.Contains(got, "..") {
		t.Errorf("tpl.drop：%q", got)
	}
}

func TestTemplateLoadRejectsRepeatedField(t *testing.T) {
	frags := map[string]string{"Dup ": "d"}
	for _, lang := range []string{LangKo, LangJa} {
		_, err := func() (c *EngineTextCatalog, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("%v", r)
				}
			}()
			var origs, pairs []string
			for o, z := range frags {
				origs = append(origs, o)
				pairs = append(pairs, o, z)
			}
			h := func(s string) string { x := sha256.Sum256([]byte(s)); return fmt.Sprintf("frag.%x", x[:6]) }
			return LoadEngineTextCatalog(EngineTextFiles{
				Lang: lang, FragmentEvents: engineTSV("frag", origs...), FragmentText: engineZh("frag", pairs...),
				ItemEvents: engineTSV("item", "Bolt"), ItemText: engineZh("item", "Bolt", "x"),
				TemplateEvents: []byte("event_key\tsignature\ntpl.dup\t" + h("Dup ") + "|_\n"),
				TemplateText:   []byte("key\ttranslation\tsource\ntpl.dup\t{0} と {0}\tecl-batch-editorial\n"),
			})
		}()
		if err == nil || !strings.Contains(err.Error(), "重複引用") {
			t.Errorf("%s：重複引用同一欄位的範本應拒載：%v", lang, err)
		}
	}
	// zh-TW keeps accepting it (the path is not touched).
	h := func(s string) string { x := sha256.Sum256([]byte(s)); return fmt.Sprintf("frag.%x", x[:6]) }
	if _, err := LoadEngineTextCatalog(EngineTextFiles{
		Lang: LangZhTW, FragmentEvents: engineTSV("frag", "Dup "), FragmentText: engineZh("frag", "Dup ", "d"),
		ItemEvents: engineTSV("item", "Bolt"), ItemText: engineZh("item", "Bolt", "x"),
		TemplateEvents: []byte("event_key\tsignature\ntpl.dup\t" + h("Dup ") + "|_\n"),
		TemplateText:   []byte("key\ttranslation\tsource\ntpl.dup\t{0}{0}\tecl-batch-editorial\n"),
	}); err != nil {
		t.Errorf("zh-TW 不應拒載：%v", err)
	}
}

// Out-of-range placeholders stay literal in the Korean and Japanese path.
func TestTemplateOutOfRangeStaysLiteral(t *testing.T) {
	frags := map[string]string{"Use antidote on ": "c"}
	for _, lang := range []string{LangKo, LangJa} {
		c := joiningCatalog(t, lang, frags, map[string]string{"tpl.antidote": "{0} {3}"})
		if got, ok := c.Translate("Use antidote on X"); !ok || got != "X {3}" {
			t.Errorf("%s：%q %v", lang, got, ok)
		}
	}
}

// Spec 046 §3.7: the Korean font has no U+3002.
func TestMonsterSlotPeriod(t *testing.T) {
	for lang, want := range map[string]string{LangKo: "NEO 전사.", LangJa: "NEO 전사。", LangZhTW: "NEO 전사。", LangZhCN: "NEO 전사。"} {
		c := joiningCatalog(t, lang, map[string]string{"x": "y"}, nil)
		if got, ok := c.monsterSlot("NEO WARRIOR."); !ok || got != want {
			t.Errorf("%s monsterSlot：%q %v，應為 %q", lang, got, ok, want)
		}
		if got, ok := c.monsterSlot("NEO WARRIOR!"); !ok || got != "NEO 전사!" {
			t.Errorf("%s 其他標點不變：%q", lang, got)
		}
		// The two translateLine entries (whole string and per part).
		if got, _ := c.Translate("NEO WARRIOR."); got != want {
			t.Errorf("%s Translate 整串：%q", lang, got)
		}
	}
}

// The test language has no rule of its own and gives the answers of the code
// before spec 046 (frozen copy, digest from the real function at cc242ea).
// zh-TW and zh-CN follow the pre-046 code except for the template field of
// spec 047 §3.3 (engine_zh047_test.go); ja and ko have their own frozen copy
// (engine_freeze046_test.go).
func TestEngineJoinUnchangedForOtherLanguages(t *testing.T) {
	for _, lang := range []string{LangTest} {
		c := engineFreezeCatalog(t, lang)
		if got := engineFreezeDigest(c, func(c *EngineTextCatalog, s string) (string, bool) { return c.translateLine(s) }); got != engineFreezeWant {
			t.Errorf("lang=%q：translateLine 摘要 %s，應為 %s", lang, got, engineFreezeWant)
		}
	}
	// The corpus exercises the joiner, the templates and the monster slot.
	c := engineFreezeCatalog(t, "")
	var ok, tpl int
	for _, s := range engineFreezeCorpus() {
		if z, good := frozenTranslateLine(c, s); good {
			ok++
			if strings.Contains(z, "手札") || strings.Contains(z, "解毒") || strings.Contains(z, "丟棄") {
				tpl++
			}
		}
	}
	t.Logf("語料 %d 個，可翻譯 %d、範本 %d", len(engineFreezeCorpus()), ok, tpl)
	if ok < 500 || tpl == 0 {
		t.Error("語料涵蓋不足")
	}
}

// Spec 046 §3.1: the Korean and Japanese rules are keyed by language only;
// a catalog built without a language is zh-TW.
func TestEngineCatalogLang(t *testing.T) {
	for in, want := range map[string]string{"": LangZhTW, LangKo: LangKo, LangJa: LangJa, LangZhCN: LangZhCN, LangTest: LangTest} {
		if c := engineFreezeCatalog(t, in); c.lang != want {
			t.Errorf("Lang %q → catalog lang %q，應為 %q", in, c.lang, want)
		}
	}
}

// The word marks of the Korean join and KO_LATIN_SUFFIXES of the static
// checker are related by two fixed differences (spec 046 §3.2).
func TestKoWordMarksVersusLatinSuffixes(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	b, err := os.ReadFile(filepath.Join(root, "tools", "lang_check.py"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)KO_LATIN_SUFFIXES\s*=\s*frozenset\(\s*"([^"]+)"\.split\(\)\)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("解析失敗：找不到 KO_LATIN_SUFFIXES 的 frozenset(\"…\".split()) 字面形式")
	}
	suffixes := map[string]bool{}
	for _, w := range strings.Fields(string(m[1])) {
		suffixes[w] = true
	}
	marks := map[string]bool{}
	for _, w := range koWordMarks {
		marks[w] = true
	}
	var onlySuffix, onlyMark []string
	for w := range suffixes {
		if !marks[w] {
			onlySuffix = append(onlySuffix, w)
		}
	}
	for w := range marks {
		if !suffixes[w] {
			onlyMark = append(onlyMark, w)
		}
	}
	sort.Strings(onlySuffix)
	sort.Strings(onlyMark)
	if got := strings.Join(onlySuffix, " "); got != "이" {
		t.Errorf("KO_LATIN_SUFFIXES 減詞記號 = %q，應為 \"이\"", got)
	}
	wantMark := []string{"(으)로", "도", "만", "와(과)", "은(는)", "을(를)", "이(가)"}
	sort.Strings(wantMark)
	if !reflect.DeepEqual(onlyMark, wantMark) {
		t.Errorf("詞記號減 KO_LATIN_SUFFIXES = %q，應為括號並列形、도、만", onlyMark)
	}
}
