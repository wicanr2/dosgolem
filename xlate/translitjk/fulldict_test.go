package translitjk

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 044 §5.1: the output invariants over every pure-letter word
// of the full CMU dictionary (first pronunciation).  Needs
// BUCKROGERS_CHT_ROOT; without it the test skips with its reason.

func pureLetterWords(t *testing.T, root string) []string {
	t.Helper()
	m, err := translit.SharedCMU(filepath.Join(root, "text", "cmudict", "cmudict.dict"))
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for w := range m {
		ok := w != ""
		for i := 0; i < len(w); i++ {
			if w[i] < 'a' || w[i] > 'z' {
				ok = false
			}
		}
		if ok {
			words = append(words, strings.ToUpper(w))
		}
	}
	sort.Strings(words)
	return words
}

func TestFullDictionaryInvariants(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定：全字典不變式未檢查")
	}
	words := pureLetterWords(t, root)
	for _, lang := range []string{"ja", "ko"} {
		tr, err := Load(filepath.Join(root, "text"), lang)
		if err != nil {
			t.Fatal(err)
		}
		tr = tr.WithRuleOnly()
		allowed := map[rune]bool{}
		for _, r := range tr.Allowed() {
			allowed[r] = true
		}
		var notOK, viol []string
		for _, w := range words {
			out, _, ok := tr.Transliterate(w, translit.GenderUnknown)
			if !ok {
				notOK = append(notOK, w)
				continue
			}
			if bad := invariantViolation(lang, out, allowed); bad != "" {
				viol = append(viol, w+"="+out+" "+bad)
			}
		}
		t.Logf("%s：檢查 %d 詞，ok=false %d 詞，不變式違反 %d 詞", lang, len(words), len(notOK), len(viol))
		if len(notOK) > 0 {
			n := notOK
			if len(n) > 40 {
				n = n[:40]
			}
			t.Logf("%s ok=false：%s", lang, strings.Join(n, " "))
		}
		if len(viol) > 0 {
			v := viol
			if len(v) > 30 {
				v = v[:30]
			}
			t.Errorf("%s 不變式違反 %d 詞：%s", lang, len(viol), strings.Join(v, "；"))
		}
	}
}

// invariantViolation checks the output invariants of spec 044 J8 and 045 K9.
func invariantViolation(lang, out string, allowed map[rune]bool) string {
	if out == "" {
		return "空輸出"
	}
	for _, r := range out {
		if r != ' ' && r != '-' && !allowed[r] {
			return "字元 " + string(r) + " 不在允許字集"
		}
	}
	if lang == "ja" {
		first, _ := utf8.DecodeRuneInString(out)
		if first == 'ッ' || first == 'ー' || first == 'ン' {
			return "詞首 " + string(first)
		}
		for _, bad := range []string{"ーー", "ンー", "ーッ", "ッー"} {
			if strings.Contains(out, bad) {
				return "含 " + bad
			}
		}
	}
	return ""
}

// The J8 clean-up must stay a safety net: count the words it changes.
func TestJaCleanUpIsASafetyNet(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	m, err := translit.SharedCMU(filepath.Join(root, "text", "cmudict", "cmudict.dict"))
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	var ex []string
	for _, w := range pureLetterWords(t, root) {
		ph, ok := translit.ParsePron(m[strings.ToLower(w)])
		if !ok {
			continue
		}
		syls, ok := Syllabify(ph, lettersOf(w))
		if !ok {
			continue
		}
		raw := jaRenderRaw(syls, nil)
		if jaClean(raw) != raw {
			changed++
			if len(ex) < 20 {
				ex = append(ex, w+"="+raw)
			}
		}
	}
	t.Logf("J8 清理改動 %d 詞：%s", changed, strings.Join(ex, " "))
}
