package translitjk

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// copyTextDir copies testdata/text into a temp dir for the tests that damage it.
func copyTextDir(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join("testdata", "text")
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func rewrite(t *testing.T, path string, f func(string) string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(f(string(b))), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join("testdata", "text"), "zh-TW"); err == nil {
		t.Error("lang zh-TW 應回錯誤")
	}
	for _, lang := range []string{"ja", "ko"} {
		names := "translit-" + lang + "-names.tsv"
		chars := "translit-chars." + lang + ".tsv"
		cases := []struct {
			label string
			mut   func(dir string)
			want  string
		}{
			{"缺詞典", func(d string) { os.Remove(filepath.Join(d, names)) }, ""},
			{"缺允許字集", func(d string) { os.Remove(filepath.Join(d, chars)) }, ""},
			{"缺 cmudict", func(d string) { os.RemoveAll(filepath.Join(d, "cmudict")) }, ""},
			{"詞典標頭不符", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string { return strings.Replace(s, "english\t", "name\t", 1) })
			}, "標頭"},
			{"english 重複", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string {
					lines := strings.Split(s, "\n")
					return strings.Join(append(lines[:len(lines)-1], lines[1], ""), "\n")
				})
			}, "重複"},
			{"english 小寫", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string { return strings.Replace(s, "\nAARON\t", "\naaron\t", 1) })
			}, "全大寫"},
			{"translation 為空", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string {
					lines := strings.Split(s, "\n")
					f := strings.Split(lines[1], "\t")
					f[1] = ""
					lines[1] = strings.Join(f, "\t")
					return strings.Join(lines, "\n")
				})
			}, "空"},
			{"translation 含允許字集外的字", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string {
					lines := strings.Split(s, "\n")
					f := strings.Split(lines[1], "\t")
					f[1] = "漢"
					lines[1] = strings.Join(f, "\t")
					return strings.Join(lines, "\n")
				})
			}, "允許字集"},
			{"欄數不符", func(d string) {
				rewrite(t, filepath.Join(d, names), func(s string) string { return s + "X\tY\n" })
			}, "欄數"},
		}
		for _, c := range cases {
			dir := copyTextDir(t)
			c.mut(dir)
			_, err := Load(dir, lang)
			if err == nil {
				t.Errorf("%s %s：應回錯誤", lang, c.label)
			} else if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s %s：錯誤 %q 應含 %q", lang, c.label, err, c.want)
			}
		}
	}
	// spaces: none in ja, single inner ones in ko; never a hyphen
	for _, c := range []struct {
		lang, text string
		ok         bool
	}{
		{"ja", "アイ", true}, {"ja", "ア イ", false}, {"ja", "ア-イ", false},
		{"ko", "가 나", true}, {"ko", "가나", true}, {"ko", " 가", false}, {"ko", "가 ", false}, {"ko", "가  나", false}, {"ko", "가-나", false},
		{"ko", "", false}, {"ja", "漢", false},
	} {
		if err := loadTD(t, c.lang).checkText(c.text); (err == nil) != c.ok {
			t.Errorf("%s checkText(%q)：err=%v，期望可用=%v", c.lang, c.text, err, c.ok)
		}
	}
}

func TestAllowedSet(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		tr := loadTD(t, lang)
		rows, err := readTSV(filepath.Join("testdata", "text", "translit-chars."+lang+".tsv"), []string{"key", "translation", "source"})
		if err != nil {
			t.Fatal(err)
		}
		a := tr.Allowed()
		if len(a) != len(rows) {
			t.Errorf("%s：Allowed() %d 個，檔案 %d 列", lang, len(a), len(rows))
		}
		for i, r := range a {
			if r == ' ' || r == '-' {
				t.Errorf("%s：Allowed() 含 %q", lang, r)
			}
			if i > 0 && a[i-1] >= r {
				t.Errorf("%s：Allowed() 不是升冪：%U %U", lang, a[i-1], r)
			}
		}
	}
}

func TestBoundaryNames(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		tr := loadTD(t, lang)
		for _, n := range []string{"Z", "A 1.?", "R2D2", "MARY-J", "MARY-", "-MARY", "HM", "HH", "", "   ", "名前", "JOSÉ", "H"} {
			if out, tier, ok := tr.Transliterate(n, translit.GenderUnknown); ok || out != "" || tier != translit.TierNone {
				t.Errorf("%s %q：應為 ok=false，得 %q／%s／%v", lang, n, out, tier, ok)
			}
		}
		for _, n := range []string{"O'BRIEN", "MARY-JANE", "NICOLE STEELE", "ANNE MARIE SMITH", "mary-jane", "Nicole   Steele"} {
			if out, _, ok := tr.Transliterate(n, translit.GenderUnknown); !ok || out == "" {
				t.Errorf("%s %q：應有結果", lang, n)
			}
		}
		a, _, _ := tr.Transliterate("ANNE  MARIE", translit.GenderUnknown)
		b, _, _ := tr.Transliterate("anne marie", translit.GenderUnknown)
		if a == "" || a != b {
			t.Errorf("%s：連續空白與大小寫應與單一空白相同：%q 對 %q", lang, a, b)
		}
		// words are joined as the language does: ja with the middle dot, ko with a space; hyphen segments by - (ko) or the dot (ja)
		out, _, _ := tr.Transliterate("NICOLE STEELE", translit.GenderUnknown)
		sep := map[string]string{"ja": "・", "ko": " "}[lang]
		if strings.Count(out, sep) != 1 {
			t.Errorf("%s NICOLE STEELE：%q 應含恰好一個 %q", lang, out, sep)
		}
		out, _, _ = tr.Transliterate("MARY-JANE", translit.GenderUnknown)
		hy := map[string]string{"ja": "・", "ko": "-"}[lang]
		if strings.Count(out, hy) != 1 {
			t.Errorf("%s MARY-JANE：%q 應含恰好一個 %q", lang, out, hy)
		}
	}
}

// An output character outside the allowed set makes the whole name ok=false.
func TestOutputOutsideAllowedSet(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		dir := copyTextDir(t)
		full := loadTD(t, lang)
		ro := full.WithRuleOnly()
		const probe = "TURABIAN"
		out, _, ok := ro.Transliterate(probe, translit.GenderUnknown)
		if !ok {
			t.Fatalf("%s %s 應有結果", lang, probe)
		}
		// the dictionary shrinks to one row so that no translation needs the removed character
		names := filepath.Join(dir, "translit-"+lang+"-names.tsv")
		var one string
		for k, v := range full.dict {
			one = k + "\t" + v + "\tmachine-reviewed\n"
			_ = k
			break
		}
		if err := os.WriteFile(names, []byte("english\ttranslation\tbasis\n"+one), 0o644); err != nil {
			t.Fatal(err)
		}
		oneKey := strings.Split(one, "\t")[0]
		oneVal := strings.Split(one, "\t")[1]
		var drop rune
		for _, r := range out {
			if r != ' ' && r != '-' && !strings.ContainsRune(oneVal, r) {
				drop = r
				break
			}
		}
		if drop == 0 {
			t.Fatalf("%s：找不到可移除的字", lang)
		}
		rewrite(t, filepath.Join(dir, "translit-chars."+lang+".tsv"), func(s string) string {
			var keep []string
			for _, l := range strings.Split(s, "\n") {
				if f := strings.Split(l, "\t"); len(f) > 1 && strings.ContainsRune(f[1], drop) && f[0] != "key" {
					continue
				}
				keep = append(keep, l)
			}
			return strings.Join(keep, "\n")
		})
		tr, err := Load(dir, lang)
		if err != nil {
			t.Fatal(err)
		}
		if got, tier, ok := tr.Transliterate(probe, translit.GenderUnknown); ok || got != "" || tier != translit.TierNone {
			t.Errorf("%s：缺 %U 時 %s 應 ok=false，得 %q／%s／%v", lang, drop, probe, got, tier, ok)
		}
		if _, _, ok := tr.Transliterate(oneKey, translit.GenderUnknown); !ok {
			t.Errorf("%s：詞典名 %s 應仍可用", lang, oneKey)
		}
		// a multi-word name fails as a whole when one word does
		if got, _, ok := tr.Transliterate("NICOLE "+probe, translit.GenderUnknown); ok {
			t.Errorf("%s：整名應 ok=false，得 %q", lang, got)
		}
	}
}

func TestRuleOnlyIsolation(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		tr := loadTD(t, lang)
		ro := tr.WithRuleOnly()
		if tr.ruleOnly || !ro.ruleOnly {
			t.Fatalf("%s：WithRuleOnly 必須回傳副本且不改原物件", lang)
		}
		// PIERRE: rules give ピエア / 피에르 form, the dictionary the conventional one
		d, dt, _ := tr.Transliterate("PIERRE", translit.GenderUnknown)
		r, rt, _ := ro.Transliterate("PIERRE", translit.GenderUnknown)
		if dt != translit.TierDict || rt != translit.TierCMUdict || d == r {
			t.Errorf("%s PIERRE：詞典 %q／%s，規則 %q／%s", lang, d, dt, r, rt)
		}
		// a dictionary hit fires nothing but the composition rule (J9, K10)
		if _, _, _, rules := tr.TransliterateTrace("PIERRE", translit.GenderUnknown); len(rules) != 1 || rules[0] != map[string]string{"ja": "J9", "ko": "K10"}[lang] {
			t.Errorf("%s：詞典命中只應有組成規則，得 %v", lang, rules)
		}
	}
}

// Spec 044 §3.1: two goroutines Load the same path; the dictionary is parsed
// once, so both get the same table.  Run with -race.
func TestConcurrentLoad(t *testing.T) {
	var wg sync.WaitGroup
	trs := make([]*Transliterator, 4)
	errs := make([]error, 4)
	for i := range trs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			trs[i], errs[i] = Load(filepath.Join("testdata", "text"), []string{"ja", "ko"}[i%2])
		}(i)
	}
	wg.Wait()
	for i := range trs {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if reflect.ValueOf(trs[i].cmu).Pointer() != reflect.ValueOf(trs[0].cmu).Pointer() {
			t.Errorf("第 %d 次 Load 的 CMUdict 對照表不是同一份", i)
		}
	}
	// the loaded value is usable from many goroutines
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				trs[0].Transliterate("NICOLE STEELE", translit.Female)
				trs[1].Transliterate("MARY-JANE", translit.Male)
			}
		}()
	}
	wg.Wait()
}
