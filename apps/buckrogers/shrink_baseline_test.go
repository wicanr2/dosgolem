package buckrogers

import (
	"os"
	"strings"
	"testing"
)

// Spec 057 §5.5: the layout digests of the synthetic corpus per language and
// the number of calls, computed on the clean export of dosgolem bba48d9
// (workplace/phase324/zz_baseline_test.go).  The layout does not change in
// spec 057, so the current code must give the same values.
var shrinkLayoutBaseline = map[string]struct {
	calls  int
	digest string
}{
	LangJa:   {59971, "15213337fe31a8b9a53361654a70a4aa4a5ce80285d0d14048d020f49a9eb3a9"},
	LangKo:   {169117, "26c3d75bced46b168b0a26049f6c867e67c96a0999724218ffd8db461725d74c"},
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

// shrinkFontChanged lists the rows of the font baseline that spec 057 changes
// on purpose: the full-width font of the ja L2 at both scales.  The value is
// the digest of the derived font on the formal ja font (frozen).
var shrinkFontChanged = map[string]string{
	"ja\t2\tfull": "cc863e36c0578ef3c6900b6c8d1558fb86082bd66e9440a0586569ec58750a6c",
	"ja\t3\tfull": "2a2f0234a2f4ccfb58936f61d1522c228eab0f1302e0331f899d93e2512ba019",
}

// Spec 057 §5.7: the derived fonts of every formal font, level and scale are
// byte-equal to bba48d9 except the full-width ja L2.  The test looks the rows
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
