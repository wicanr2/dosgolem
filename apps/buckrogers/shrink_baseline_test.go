package buckrogers

import (
	"os"
	"strings"
	"testing"
)

// Spec 057 §5.5 and spec 058 §5.4: the layout digests of the synthetic corpus
// per language and the number of calls.  zh-TW, zh-CN and ja are the values
// computed on the clean export of dosgolem bba48d9 (workplace/phase324/
// zz_baseline_test.go): spec 057 does not change their layout.  ko is the value
// of the layout of spec 058 (the wider L2 cell), recorded with the classification
// of shrink_ko_classify_test.go passing.
var shrinkLayoutBaseline = map[string]struct {
	calls  int
	digest string
}{
	LangJa:   {59971, "15213337fe31a8b9a53361654a70a4aa4a5ce80285d0d14048d020f49a9eb3a9"},
	LangKo:   {169117, "0d621fc6595f43d5ad9ab880cd4e6aac7dc4aa642d64b4c9b84b32ea5f7e00ae"}, // bba48d9: 26c3d75b…
	LangZhCN: {78347, "66019f0eb63effa30b894944c492622f44f11fe21a8d2e3da880a982c75c960e"},
	LangZhTW: {80231, "5a8819d18d023f0b853b4131faac274f9d9737301e63802b584b7b9740d013e1"},
}

func TestShrinkLayoutDigest(t *testing.T) {
	digests, calls := shrinkLayoutDigests(t)
	if len(digests) != len(shrinkLayoutBaseline) {
		t.Fatalf("%d 個語言，基準 %d 個", len(digests), len(shrinkLayoutBaseline))
	}
	for lang, want := range shrinkLayoutBaseline {
		if testing.Verbose() {
			t.Logf("%s: %d 次 %s", lang, calls[lang], digests[lang])
		}
		if digests[lang] != want.digest || calls[lang] != want.calls {
			t.Errorf("%s 的版面摘要與 bba48d9 不同：%s（%d 次）想要 %s（%d 次）", lang, digests[lang], calls[lang], want.digest, want.calls)
		}
	}
}

// shrinkFontChanged lists the rows of the font baseline that the specs after
// 056 change on purpose: the full-width font of the L2 of ja (spec 057) and of
// ko (spec 058) at both scales.  The value is the digest of the derived font on
// the formal font (frozen).
var shrinkFontChanged = map[string]string{
	"ja\t2\tfull": "cc863e36c0578ef3c6900b6c8d1558fb86082bd66e9440a0586569ec58750a6c",
	"ja\t3\tfull": "2a2f0234a2f4ccfb58936f61d1522c228eab0f1302e0331f899d93e2512ba019",
	"ko\t2\tfull": "022926e95a68edd0590bdf891eaf90418a05042352e9ef9866c5457a5fb2731d",
	"ko\t3\tfull": "1fa0b12845617d77e65fc1783c4b902106f077a8ef4cb388bd8dc0ef3f0022ce",
}

// Spec 057 §5.7 and spec 058 §5.5: the derived fonts of every formal font, level
// and scale are byte-equal to bba48d9 except the full-width L2 of ja and ko.  The test looks the rows
// up by the SHA-256 of the font file and skips a font it has no row for (a
// different font does not count as an acceptance).
func TestShrinkFontBaseline(t *testing.T) {
	raw, err := os.ReadFile(shrinkBaselinePath())
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]string{} // sha \t label \t level \t scale \t class
	known := map[string]bool{}
	for i, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if i == 0 {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 6 {
			t.Fatalf("基準表第 %d 列欄位數 %d", i+1, len(c))
		}
		base[strings.Join(c[:5], "\t")] = c[5]
		known[c[0]] = true
	}
	if len(base) != 40 {
		t.Fatalf("基準表 %d 列，預期 40（五個字型、2 等級、2 倍率、全形與半形）", len(base))
	}
	rows := shrinkFontDigestRows(t)
	checked, changed := 0, 0
	for _, r := range rows {
		c := strings.Split(r, "\t")
		sha, label, level, scale, class, got := c[0], c[1], c[2], c[3], c[4], c[5]
		if !known[sha] {
			t.Logf("字型 %s（%s）不在基準表，跳過（不算驗收）", sha[:8], label)
			continue
		}
		want, ok := base[strings.Join(c[:5], "\t")]
		if !ok {
			// A level the baseline does not have: only spec 058 adds one.
			t.Errorf("基準表沒有 %s L%s %s× %s", label, level, scale, class)
			continue
		}
		key := label + "\t" + scale + "\t" + class
		if want2, isChanged := shrinkFontChanged[key]; isChanged && level == "2" {
			changed++
			if got == want {
				t.Errorf("%s L2 %s× %s 應不同於基準，卻相同", label, scale, class)
			}
			if got != want2 {
				t.Errorf("%s L2 %s× %s 摘要 %s，凍結值 %s", label, scale, class, got, want2)
			}
			continue
		}
		checked++
		if got != want {
			t.Errorf("%s L%s %s× %s 與 bba48d9 不同：%s 想要 %s", label, level, scale, class, got, want)
		}
	}
	t.Logf("相同 %d 列，依規格改變 %d 列", checked, changed)
}
