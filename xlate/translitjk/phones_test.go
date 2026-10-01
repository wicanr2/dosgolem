package translitjk

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// loadTD loads the language from the copy of the Buck repo's text/ kept in
// testdata/text (the copy is hash-locked to the Buck repo in fixed_test.go).
func loadTD(t testing.TB, lang string) *Transliterator {
	t.Helper()
	tr, err := Load(filepath.Join("testdata", "text"), lang)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func mustPhones(t testing.TB, pron string) []translit.Phone {
	t.Helper()
	ph, ok := translit.ParsePron(strings.Fields(pron))
	if !ok {
		t.Fatalf("不是 ARPAbet：%q", pron)
	}
	return ph
}

// phoneCase feeds ARPAbet straight into the back ends (Buck repo spec 044
// §5 point 2 (2): the rule rows no word of the name table reaches).  The
// expected strings follow the rule tables of specs 044 and 045; where a word
// is a real English name the spec's example form is the expectation.
type phoneCase struct {
	letters string
	pron    string
	ja, ko  string // "" = not asserted for that language
	note    string
}

var phoneCases = []phoneCase{
	{"yield", "Y IY1 L D", "イールド", "일드", "J2.Y J2.YIY J4.L J4.D / K2.Y K8.coda K7.eu；無單字輸入"},
	{"yu", "Y UW1", "ユー", "유", "J2.YUW / K2.Y"},
	{"woo", "W UW1", "ウー", "우", "J2.WUW / K2.W"},
	{"regular", "R EH1 G Y AH0 L ER0", "レギュラー", "레귤러", "J3.AHu / K3.UW K5"},
	{"billiard", "B IH1 L Y ER0 D", "ビリャード", "빌려드", "J3.ya / K3.y"},
	{"kyel", "K Y EH1 L", "キェル", "", "J3.ye"},
	{"myeet", "M Y IY1 T", "ミート", "미트", "J3.yi"},
	{"kyoto", "K Y OW1 T OW0", "キョート", "쿄토", "J3.yo"},
	{"rich", "R IH1 CH", "リッチ", "리치", "J4.CH J6.a / K7.CH"},
	{"smooth", "S M UW1 DH", "スムーズ", "스무드", "J4.DH"},
	{"woods", "W UH1 D Z", "ウッズ", "우즈", "J4.DZ J6.a / K7.DZ"},
	{"ahmed", "AA1 HH M AH0 D", "アフメッド", "아흐머드", "J4.HH J6.b / K7.HH"},
	{"humphrey", "HH AH1 M F R IY0", "ハンフリー", "험프리", "J4.M.BPF"},
	{"nkrumah", "N K R UW1 M AH0", "ヌクルーマ", "느크루마", "J4.N.initial / K8.eu"},
	{"hank", "HH AE1 NG K", "ハンク", "행크", "J4.NG.KGT"},
	{"singer", "S IH1 NG ER0", "シンガー", "싱어", "J4.NG.vowel"},
	{"ash", "AE1 SH", "アシュ", "애시", "J4.SH / K7.SH"},
	{"watts", "W AA1 T S", "ワッツ", "와츠", "J4.TS J6.a / K7.TS"},
	{"love", "L AH1 V", "ラブ", "러브", "J4.V"},
	{"kwnat", "K W N AA1 T", "クウナット", "크우낫", "J4.W / K2.W.end；無單字輸入"},
	{"ylan", "Y L AA1 N", "イラン", "일란", "J4.Y / K2.Y.end"},
	{"krnat", "K R N AA1 T", "クルナット", "크르낫", "J4.R / K2.R.alone；無單字輸入"},
	{"bezh", "B EH1 ZH", "ベジ", "베지", "J4.ZH"},
	{"claire", "K L EH1 R", "クレア", "클레어", "J5.EH / K4.eo"},
	{"pierce", "P IH1 R S", "ピアス", "피어스", "J5.IH"},
	{"moore", "M UH1 R", "ムア", "무어", "J5.UH；慣用ムーア交詞典"},
	{"tyre", "T AY1 R", "タイア", "타이", "J5.diph"},
	{"mbeki", "M B EH1 K IY0", "ムベキー", "므베키", "J4.M / K8.eu"},
}

func TestFromPhones(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		tr := loadTD(t, lang).WithRuleOnly()
		for _, c := range phoneCases {
			want := c.ja
			if lang == "ko" {
				want = c.ko
			}
			if want == "" {
				continue
			}
			got, _, ok := tr.FromPhones(mustPhones(t, c.pron), c.letters)
			if !ok || got != want {
				t.Errorf("%s %s [%s]：得 %q（ok=%v），應為 %q（%s）", lang, c.letters, c.pron, got, ok, want, c.note)
			}
		}
	}
}

// phoneRules is the set of rule ids the phone table fires, per language.
func phoneRules(t testing.TB, lang string) map[string]bool {
	t.Helper()
	tr := loadTD(t, lang).WithRuleOnly()
	set := map[string]bool{}
	for _, c := range phoneCases {
		_, rules, _ := tr.FromPhones(mustPhones(t, c.pron), c.letters)
		for _, r := range rules {
			set[r] = true
		}
	}
	return set
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
