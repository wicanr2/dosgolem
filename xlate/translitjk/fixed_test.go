package translitjk

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 044 §5 point 2: the fixed names with their expected output.
// The Buck repo keeps docs/re/phase-308-translit-fixed-names.<lang>.tsv (the
// reviewed expectations), ...-rule-regression.<lang>.tsv (the 85 dictionary
// names in rule-only mode) and phase-308-spec-examples.<lang>.tsv (the
// examples of the rule text).  testdata/ holds verbatim copies of them, of
// the text/ files Load reads and a CMU dictionary subset.  With
// BUCKROGERS_CHT_ROOT the copies are compared with the Buck repo's files;
// -update-fixed regenerates the candidates (review stays "pending" for a row
// whose name and expectation changed) and the copies from it.

var updateFixed = flag.Bool("update-fixed", false,
	"regenerate the fixed-name candidates and the testdata copies (needs a writable BUCKROGERS_CHT_ROOT)")

const phaseTag = "phase-308"

var (
	fixedHeader      = []string{"name", "expected", "tier", "rules", "review"}
	regressionHeader = []string{"name", "expected", "tier", "rules"}
	examplesHeader   = []string{"name", "expected", "rule", "kind"}
	reviewRe         = regexp.MustCompile(`^(pending|ok:[^,\s:]+,[^,\s:]+)$`)
)

// Names of spec 044 §5 point 2 (1) and (4).
var (
	specNames = []string{"ROARKE", "CELESTE", "FLAVIUS", "JANELLE", "PIERRE", "NICOLE STEELE", "ALEXANDER", "JENNIFER",
		"MICHAEL", "WILLIAM", "BUCK", "WILMA", "PORT", "EXIT"}
	syntheticSpelling = []string{"BRELDA", "KORVANTH", "VELDORAN"}
	boundaryNames     = []string{"Z", "A 1.?", "R2D2", "O'BRIEN", "MARY-JANE", "MARY-J", "MARY-", "NICOLE STEELE",
		"WILHELMINA-ROSE", "ANNE  MARIE", "ANNE MARIE SMITH", "HH", "HM"}
)

type fixedRow struct{ name, expected, tier, rules, review string }

func buckRoot() string { return os.Getenv("BUCKROGERS_CHT_ROOT") }

func fixedPath(lang string) string { return filepath.Join("testdata", "fixed-names."+lang+".tsv") }
func regressionPath(lang string) string {
	return filepath.Join("testdata", "rule-regression."+lang+".tsv")
}
func examplesPath(lang string) string { return filepath.Join("testdata", "spec-examples."+lang+".tsv") }

func buckDoc(root, kind, lang string) string {
	return filepath.Join(root, "docs", "re", phaseTag+"-"+kind+"."+lang+".tsv")
}

// traceRow runs one name the way the player-name path does (every layer on).
func traceRow(tr *Transliterator, name string) fixedRow {
	out, tier, ok, rules := tr.TransliterateTrace(name, translit.GenderUnknown)
	if !ok {
		out = ""
	}
	return fixedRow{name: name, expected: out, tier: tier.String(), rules: strings.Join(dedupSort(rules), ",")}
}

func writeTSV(path string, header []string, rows [][]string) error {
	var b strings.Builder
	b.WriteString(strings.Join(header, "\t") + "\n")
	for _, r := range rows {
		b.WriteString(strings.Join(r, "\t") + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func readFixed(path string) ([]fixedRow, error) {
	rows, err := readTSV(path, fixedHeader)
	if err != nil {
		return nil, err
	}
	var out []fixedRow
	for _, r := range rows {
		out = append(out, fixedRow{r[0], r[1], r[2], r[3], r[4]})
	}
	return out, nil
}

// glossaryPool lists the words of the Buck name table (text/name-glossary.tsv)
// that are pure letters, split by whether CMUdict knows them.
func glossaryPool(t *testing.T, root string, tr *Transliterator) (inCMU, notCMU []string) {
	t.Helper()
	rows, err := readTSV(filepath.Join(root, "text", "name-glossary.tsv"),
		[]string{"english", "english_mixed", "chinese", "kind", "person", "basis", "note"})
	if err != nil {
		t.Fatal(err)
	}
	letters := regexp.MustCompile(`^[A-Z]+$`)
	seen := map[string]bool{}
	for _, r := range rows {
		for _, w := range strings.Fields(r[0]) {
			if !letters.MatchString(w) || seen[w] {
				continue
			}
			seen[w] = true
			if _, ok := tr.cmu[strings.ToLower(w)]; ok {
				if _, inDict := tr.dict[w]; !inDict {
					inCMU = append(inCMU, w)
				}
			} else {
				notCMU = append(notCMU, w)
			}
		}
	}
	sort.Strings(inCMU)
	sort.Strings(notCMU)
	return
}

// fixedCandidates builds the fixed-name list: names of the spec, for each rule
// row the first name of the name table that fires it in rule-only mode, the
// tier samples and the boundaries.
func fixedCandidates(t *testing.T, root, lang string, full, ruleOnly *Transliterator) []string {
	t.Helper()
	inCMU, notCMU := glossaryPool(t, root, ruleOnly)
	var names []string
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range specNames {
		add(n)
	}
	first := map[string]string{}
	for _, w := range inCMU {
		_, _, ok, rules := ruleOnly.TransliterateTrace(w, translit.GenderUnknown)
		if !ok {
			continue
		}
		for _, r := range rules {
			if first[r] == "" {
				first[r] = w
			}
		}
	}
	for _, r := range ruleOnly.Rules() {
		if w := first[r]; w != "" {
			add(w)
		}
	}
	// at least five names of the cmudict tier
	n := 0
	for _, x := range names {
		if row := traceRow(full, x); row.tier == "cmudict" {
			n++
		}
	}
	for _, w := range inCMU {
		if n >= 5 {
			break
		}
		if !seen[w] {
			add(w)
			n++
		}
	}
	for _, w := range notCMU {
		if _, _, ok := full.Transliterate(w, translit.GenderUnknown); ok {
			add(w)
		}
	}
	for _, w := range syntheticSpelling {
		if _, known := full.cmu[strings.ToLower(w)]; known {
			t.Fatalf("合成拼寫 %s 在 CMUdict 內，換一個", w)
		}
		add(w)
	}
	for _, w := range boundaryNames {
		add(w)
	}
	return names
}

func dictionaryNames(tr *Transliterator) []string {
	var out []string
	for k := range tr.dict {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cmuSubset lists the words (lower case) the testdata dictionary must hold.
func cmuWords(names []string) map[string]bool {
	set := map[string]bool{}
	for _, n := range names {
		for _, w := range strings.Fields(strings.ToLower(n)) {
			set[w] = true
			for _, seg := range strings.Split(w, "-") {
				set[seg] = true
			}
		}
	}
	return set
}

func readExampleNames(lang string) []string {
	rows, err := readTSV(examplesPath(lang), examplesHeader)
	if err != nil {
		return nil
	}
	var out []string
	for _, r := range rows {
		out = append(out, r[0])
	}
	return out
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// updateData regenerates the candidates and the testdata copies (-update-fixed).
func updateData(t *testing.T, root string) {
	t.Helper()
	text := filepath.Join(root, "text")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join("testdata", "text", "cmudict"), 0o755))
	must(copyFile(filepath.Join(text, "cmudict", "LICENSE"), filepath.Join("testdata", "text", "cmudict", "LICENSE")))
	for _, lang := range []string{"ja", "ko"} {
		must(copyFile(filepath.Join(text, "translit-"+lang+"-names.tsv"), filepath.Join("testdata", "text", "translit-"+lang+"-names.tsv")))
		must(copyFile(filepath.Join(text, "translit-chars."+lang+".tsv"), filepath.Join("testdata", "text", "translit-chars."+lang+".tsv")))
		must(copyFile(buckDoc(root, "spec-examples", lang), examplesPath(lang)))
	}
	// Candidates are generated against the full CMU dictionary of the Buck repo.
	var all []string
	for _, lang := range []string{"ja", "ko"} {
		full, err := Load(text, lang)
		must(err)
		ruleOnly := full.WithRuleOnly()
		names := fixedCandidates(t, root, lang, full, ruleOnly)
		old, _ := readFixed(buckDoc(root, "translit-fixed-names", lang))
		review := map[string]fixedRow{}
		for _, r := range old {
			review[r.name] = r
		}
		var rows [][]string
		for _, n := range names {
			r := traceRow(full, n)
			rev := "pending"
			if o, ok := review[n]; ok && o.expected == r.expected && o.tier == r.tier && o.rules == r.rules {
				rev = o.review
			}
			rows = append(rows, []string{r.name, r.expected, r.tier, r.rules, rev})
		}
		must(writeTSV(buckDoc(root, "translit-fixed-names", lang), fixedHeader, rows))
		must(copyFile(buckDoc(root, "translit-fixed-names", lang), fixedPath(lang)))

		var reg [][]string
		dnames := dictionaryNames(full)
		for _, n := range dnames {
			r := traceRow(ruleOnly, n)
			reg = append(reg, []string{r.name, r.expected, r.tier, r.rules})
		}
		must(writeTSV(buckDoc(root, "translit-rule-regression", lang), regressionHeader, reg))
		must(copyFile(buckDoc(root, "translit-rule-regression", lang), regressionPath(lang)))

		all = append(all, names...)
		all = append(all, dnames...)
		all = append(all, readExampleNames(lang)...)
	}
	// CMU subset: the verbatim lines of every word the tables use.
	want := cmuWords(all)
	f, err := os.Open(filepath.Join(text, "cmudict", "cmudict.dict"))
	must(err)
	defer f.Close()
	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		key := line
		if i := strings.IndexByte(key, ' '); i >= 0 {
			key = key[:i]
		}
		if i := strings.IndexByte(key, '('); i >= 0 {
			key = key[:i]
		}
		if want[key] {
			b.WriteString(line + "\n")
		}
	}
	must(sc.Err())
	must(os.WriteFile(filepath.Join("testdata", "text", "cmudict", "cmudict.dict"), []byte(b.String()), 0o644))
}

func TestFixedNames(t *testing.T) {
	root := buckRoot()
	if *updateFixed {
		if root == "" {
			t.Fatal("-update-fixed 需要 BUCKROGERS_CHT_ROOT（可寫的 Buck repo）")
		}
		updateData(t, root)
	}
	for _, lang := range []string{"ja", "ko"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			tr := loadTD(t, lang)
			rows, err := readFixed(fixedPath(lang))
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) < 50 {
				t.Fatalf("固定名單只有 %d 列", len(rows))
			}
			seen := map[string]bool{}
			tiers := map[string]int{}
			for _, r := range rows {
				if seen[r.name] {
					t.Errorf("重複的名字 %q", r.name)
				}
				seen[r.name] = true
				if !reviewRe.MatchString(r.review) {
					t.Errorf("%q 的 review 欄 %q 不合格式（pending 或 ok:<甲>,<乙>）", r.name, r.review)
				}
				got := traceRow(tr, r.name)
				if got.expected != r.expected || got.tier != r.tier || got.rules != r.rules {
					t.Errorf("%q：得 %q／%s／%q，固定名單為 %q／%s／%q", r.name, got.expected, got.tier, got.rules, r.expected, r.tier, r.rules)
				}
				if (r.expected == "") != (r.tier == "none") {
					t.Errorf("%q：expected 為空與 tier=none 必須一致", r.name)
				}
				tiers[r.tier]++
				// Gender does not matter; the call is deterministic.
				for _, g := range []translit.Gender{translit.GenderUnknown, translit.Male, translit.Female} {
					o, ti, ok := tr.Transliterate(r.name, g)
					if !ok {
						o = ""
					}
					if o != r.expected || ti.String() != r.tier {
						t.Errorf("%q：性別 %v 的輸出 %q／%s 與固定名單不同", r.name, g, o, ti)
					}
				}
			}
			for _, tier := range []string{"dict", "cmudict", "spelling"} {
				// dict-tier samples are the dictionary names (TestDictionaryNames) plus the spec names that are in it
				min := 5
				if tier == "dict" {
					min = 1
				}
				if tiers[tier] < min {
					t.Errorf("tier %s 只有 %d 列，至少 %d", tier, tiers[tier], min)
				}
			}
			t.Logf("%s 固定名單 %d 列（dict %d、cmudict %d、spelling %d、none %d）", lang, len(rows), tiers["dict"], tiers["cmudict"], tiers["spelling"], tiers["none"])
		})
	}
}

// TestFixedNamesAgainstBuck compares the copies with the Buck repo and
// regenerates the candidates from it.
func TestFixedNamesAgainstBuck(t *testing.T) {
	root := buckRoot()
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定：testdata 副本與 Buck repo 的互鎖、固定名單候選重算未檢查")
	}
	same := func(a, b string) {
		t.Helper()
		x, err1 := os.ReadFile(a)
		y, err2 := os.ReadFile(b)
		if err1 != nil || err2 != nil {
			t.Errorf("讀不到 %s 或 %s：%v %v", a, b, err1, err2)
			return
		}
		if string(x) != string(y) {
			t.Errorf("%s 與 %s 不同（以 -update-fixed 重產，或同步 Buck repo）", a, b)
		}
	}
	text := filepath.Join(root, "text")
	same(filepath.Join(text, "cmudict", "LICENSE"), filepath.Join("testdata", "text", "cmudict", "LICENSE"))
	for _, lang := range []string{"ja", "ko"} {
		same(filepath.Join(text, "translit-"+lang+"-names.tsv"), filepath.Join("testdata", "text", "translit-"+lang+"-names.tsv"))
		same(filepath.Join(text, "translit-chars."+lang+".tsv"), filepath.Join("testdata", "text", "translit-chars."+lang+".tsv"))
		same(buckDoc(root, "translit-fixed-names", lang), fixedPath(lang))
		same(buckDoc(root, "translit-rule-regression", lang), regressionPath(lang))
		same(buckDoc(root, "spec-examples", lang), examplesPath(lang))
	}
	// every line of the subset is a verbatim line of the full dictionary
	full := map[string]bool{}
	f, err := os.Open(filepath.Join(text, "cmudict", "cmudict.dict"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		full[sc.Text()] = true
	}
	g, err := os.Open(filepath.Join("testdata", "text", "cmudict", "cmudict.dict"))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	sc = bufio.NewScanner(g)
	n := 0
	for sc.Scan() {
		n++
		if !full[sc.Text()] {
			t.Errorf("子集的這一行不在完整詞典內：%q", sc.Text())
		}
	}
	// the candidates, rebuilt from the current Buck data, are the file's rows
	for _, lang := range []string{"ja", "ko"} {
		fullTr, err := Load(text, lang)
		if err != nil {
			t.Fatal(err)
		}
		names := fixedCandidates(t, root, lang, fullTr, fullTr.WithRuleOnly())
		rows, err := readFixed(fixedPath(lang))
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(names) {
			t.Errorf("%s：候選 %d 列，固定名單 %d 列", lang, len(names), len(rows))
			continue
		}
		for i, nm := range names {
			r := traceRow(fullTr, nm)
			if rows[i].name != nm || rows[i].expected != r.expected || rows[i].tier != r.tier || rows[i].rules != r.rules {
				t.Errorf("%s：第 %d 列候選 %q 與固定名單 %q 不同", lang, i+1, nm, rows[i].name)
			}
		}
	}
	t.Logf("CMUdict 子集 %d 行，全部是完整詞典的原樣行；副本與 Buck repo 逐位元相同", n)
}

// TestDictionaryNames: the project dictionary names come out of the
// dictionary at tier dict; in rule-only mode they match the regression table.
func TestDictionaryNames(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			tr := loadTD(t, lang)
			ro := tr.WithRuleOnly()
			reg, err := readTSV(regressionPath(lang), regressionHeader)
			if err != nil {
				t.Fatal(err)
			}
			if len(reg) != len(tr.dict) {
				t.Errorf("規則回歸表 %d 列，詞典 %d 列", len(reg), len(tr.dict))
			}
			regByName := map[string][]string{}
			for _, r := range reg {
				regByName[r[0]] = r
			}
			for _, n := range dictionaryNames(tr) {
				out, tier, ok := tr.Transliterate(n, translit.GenderUnknown)
				if !ok || tier != translit.TierDict || out != tr.dict[n] {
					t.Errorf("%s：得 %q／%s／%v，詞典為 %q", n, out, tier, ok, tr.dict[n])
				}
				got := traceRow(ro, n)
				r, has := regByName[n]
				if !has {
					t.Errorf("規則回歸表缺 %s", n)
					continue
				}
				if r[1] != got.expected || r[2] != got.tier || r[3] != got.rules {
					t.Errorf("%s 規則模式：得 %q／%s／%q，回歸表為 %q／%s／%q", n, got.expected, got.tier, got.rules, r[1], r[2], r[3])
				}
			}
		})
	}
}

// TestRuleCoverage: every rule id is hit by the fixed names, the rule-only
// dictionary names or the phone table; none is left over (spec 044 §5 point 1).
func TestRuleCoverage(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			tr := loadTD(t, lang)
			hit := map[string]string{}
			mark := func(rules, how string) {
				for _, r := range strings.Split(rules, ",") {
					if r != "" && hit[r] == "" {
						hit[r] = how
					}
				}
			}
			fixed, err := readFixed(fixedPath(lang))
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range fixed {
				mark(r.rules, "固定名單")
			}
			reg, err := readTSV(regressionPath(lang), regressionHeader)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range reg {
				mark(r[3], "規則回歸表")
			}
			for r := range phoneRules(t, lang) {
				mark(r, "音素表")
			}
			valid := map[string]bool{}
			for _, r := range tr.Rules() {
				valid[r] = true
				if hit[r] == "" {
					t.Errorf("規則 %s 沒有任何測試命中", r)
				}
			}
			for r := range hit {
				if !valid[r] {
					t.Errorf("命中了不在 Rules() 內的編號 %s", r)
				}
			}
			counts := map[string]int{}
			for _, h := range hit {
				counts[h]++
			}
			t.Logf("%s：%d 條規則編號；首命中來源 %v", lang, len(tr.Rules()), counts)
		})
	}
}

// TestSpecExamples checks the examples of the rule text, in rule-only mode
// (spec 044 §5 point 1).  kind=rule: the output is the expectation and the
// rule fired ("!<id>": did not fire); kind=idiom: the conventional spelling is
// listed as a difference, so the rule output must differ from it, and a name
// that is in the dictionary gives it at tier dict.
func TestSpecExamples(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			tr := loadTD(t, lang)
			ro := tr.WithRuleOnly()
			rows, err := readTSV(examplesPath(lang), examplesHeader)
			if err != nil {
				t.Fatal(err)
			}
			valid := map[string]bool{}
			for _, r := range tr.Rules() {
				valid[r] = true
			}
			rulesOf := map[string]bool{}
			var bad []string
			idioms := 0
			for _, r := range rows {
				name, want, rule, kind := r[0], r[1], r[2], r[3]
				out, _, ok, rules := ro.TransliterateTrace(name, translit.GenderUnknown)
				if !ok {
					out = ""
				}
				id := strings.TrimPrefix(rule, "!")
				if !valid[id] {
					t.Errorf("%s：規則編號 %q 不在 Rules() 內", name, rule)
				}
				fired := false
				for _, x := range rules {
					if x == id {
						fired = true
					}
				}
				switch kind {
				case "rule":
					rulesOf[id] = true
					if out != want {
						bad = append(bad, fmt.Sprintf("%s 規則模式得 %s，規格為 %s（%s）", name, out, want, rule))
					}
					if strings.HasPrefix(rule, "!") == fired {
						bad = append(bad, fmt.Sprintf("%s：規則 %s 的命中狀態不符（命中=%v）", name, rule, fired))
					}
				case "idiom":
					idioms++
					if out == want {
						bad = append(bad, fmt.Sprintf("%s：標 idiom 但規則模式已得 %s", name, out))
					}
					if d, in := tr.dict[strings.ToUpper(name)]; in {
						if d != want {
							bad = append(bad, fmt.Sprintf("%s：詞典為 %s，慣用寫法列為 %s", name, d, want))
						}
					}
				default:
					t.Errorf("%s：kind %q 不是 rule 或 idiom", name, kind)
				}
			}
			for _, b := range bad {
				t.Error(b)
			}
			t.Logf("%s 例子 %d 列（rule %d、idiom %d），涵蓋 %d 個規則編號", lang, len(rows), len(rows)-idioms, idioms, len(rulesOf))
		})
	}
}
