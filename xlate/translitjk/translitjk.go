// Package translitjk transliterates English names into katakana (ja) or
// Hangul (ko) for the player-name display (Buck repo specs 044 and 045).
package translitjk

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Transliterator is the loaded read-only data of one language; it can be
// used by several goroutines.
type Transliterator struct {
	lang     string
	dict     map[string]string
	cmu      map[string][]string
	allowed  map[rune]bool
	ruleOnly bool
}

// Load reads translit-<lang>-names.tsv, translit-chars.<lang>.tsv and
// cmudict/cmudict.dict under textDir (spec 044 §3.1).
func Load(textDir, lang string) (*Transliterator, error) {
	if lang != "ja" && lang != "ko" {
		return nil, fmt.Errorf("translitjk: 不支援的語言 %q（只有 ja、ko）", lang)
	}
	t := &Transliterator{lang: lang, dict: map[string]string{}, allowed: map[rune]bool{}}
	rows, err := readTSV(filepath.Join(textDir, "translit-chars."+lang+".tsv"), []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		for _, ch := range r[1] {
			t.allowed[ch] = true
		}
	}
	if len(t.allowed) == 0 {
		return nil, fmt.Errorf("translit-chars.%s.tsv 沒有任何字", lang)
	}
	rows, err = readTSV(filepath.Join(textDir, "translit-"+lang+"-names.tsv"), []string{"english", "translation", "basis"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		en, tr := r[0], r[1]
		if en == "" || en != strings.ToUpper(en) || strings.ContainsAny(en, " \t") {
			return nil, fmt.Errorf("translit-%s-names.tsv：english %q 要全大寫且不含空白", lang, en)
		}
		if _, dup := t.dict[en]; dup {
			return nil, fmt.Errorf("translit-%s-names.tsv：english %q 重複", lang, en)
		}
		if err := t.checkText(tr); err != nil {
			return nil, fmt.Errorf("translit-%s-names.tsv：%s 的 translation %q：%w", lang, en, tr, err)
		}
		t.dict[en] = tr
	}
	t.cmu, err = translit.SharedCMU(filepath.Join(textDir, "cmudict", "cmudict.dict"))
	if err != nil {
		return nil, err
	}
	return t, nil
}

// checkText checks a dictionary translation: every character in the allowed
// set (U+0020 and "-" excepted); ja has neither, ko only single inner spaces.
func (t *Transliterator) checkText(s string) error {
	if s == "" {
		return fmt.Errorf("空")
	}
	if t.lang == "ja" && strings.ContainsAny(s, " -") {
		return fmt.Errorf("ja 的 translation 不得含空白或連字號")
	}
	if t.lang == "ko" {
		if strings.Contains(s, "-") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.Contains(s, "  ") {
			return fmt.Errorf("ko 的 translation 首尾與連續空白、連字號都不行")
		}
	}
	for _, ch := range s {
		if ch != ' ' && ch != '-' && !t.allowed[ch] {
			return fmt.Errorf("U+%04X 不在允許字集內", ch)
		}
	}
	return nil
}

func readTSV(path string, header []string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows [][]string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if first {
			first = false
			if line != strings.Join(header, "\t") {
				return nil, fmt.Errorf("%s: 標頭必須為 %s", path, strings.Join(header, "/"))
			}
			continue
		}
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != len(header) {
			return nil, fmt.Errorf("%s: 欄數 %d，應為 %d：%q", path, len(f), len(header), line)
		}
		rows = append(rows, f)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if first {
		return nil, fmt.Errorf("%s: 空檔", path)
	}
	return rows, nil
}

// WithRuleOnly returns a copy with the dictionary layer off (tests only).
func (t *Transliterator) WithRuleOnly() *Transliterator {
	c := *t
	c.ruleOnly = true
	return &c
}

// Allowed lists the allowed characters, ascending (U+0020 and "-" not in it).
func (t *Transliterator) Allowed() []rune {
	var out []rune
	for r := range t.allowed {
		if r != ' ' && r != '-' {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Rules lists every rule id of the language.
func (t *Transliterator) Rules() []string {
	if t.lang == "ja" {
		return jaRules()
	}
	return koRules()
}

func (t *Transliterator) wordSep() string {
	if t.lang == "ja" {
		return "・"
	}
	return " "
}

func (t *Transliterator) hyphen() string {
	if t.lang == "ja" {
		return "・"
	}
	return "-"
}

// Transliterate implements the player-name transliterator (spec 044 §3.1).
func (t *Transliterator) Transliterate(name string, g translit.Gender) (string, translit.Tier, bool) {
	out, tier, ok, _ := t.TransliterateTrace(name, g)
	return out, tier, ok
}

// TransliterateTrace is Transliterate with the ids of the rules that fired
// (only in rule-only mode and for the cmudict and spelling layers).
func (t *Transliterator) TransliterateTrace(name string, g translit.Gender) (out string, tier translit.Tier, ok bool, rules []string) {
	tr := &tracer{}
	out, tier, ok = t.transliterate(name, tr)
	return out, tier, ok, tr.ids
}

func (t *Transliterator) transliterate(name string, tr *tracer) (string, translit.Tier, bool) {
	words := strings.Fields(strings.ToUpper(name))
	if len(words) == 0 {
		return "", translit.TierNone, false
	}
	parts := make([]string, 0, len(words))
	tier := translit.TierDict
	for _, w := range words {
		letters := 0
		for i := 0; i < len(w); i++ {
			c := w[i]
			switch {
			case c >= 'A' && c <= 'Z':
				letters++
			case c == '\'' || c == '-':
			default:
				return "", translit.TierNone, false
			}
		}
		if letters < 2 {
			return "", translit.TierNone, false
		}
		s, wt, ok := t.word(w, tr)
		if !ok || s == "" {
			return "", translit.TierNone, false
		}
		for _, ch := range s {
			if ch != ' ' && ch != '-' && !t.allowed[ch] {
				return "", translit.TierNone, false
			}
		}
		parts = append(parts, s)
		if wt < tier {
			tier = wt
		}
	}
	tr.add(map[string]string{"ja": "J9", "ko": "K10"}[t.lang])
	return strings.Join(parts, t.wordSep()), tier, true
}

// word transliterates one word cut by spaces (§3.2).
func (t *Transliterator) word(w string, tr *tracer) (string, translit.Tier, bool) {
	if !t.ruleOnly {
		if s, ok := t.dict[w]; ok {
			return s, translit.TierDict, true
		}
	}
	if pron, ok := t.cmu[strings.ToLower(w)]; ok {
		if ph, ok := translit.ParsePron(pron); ok {
			if s, ok := t.convert(ph, lettersOf(w), tr); ok {
				return s, translit.TierCMUdict, true
			}
			return "", translit.TierNone, false
		}
	}
	if strings.Contains(w, "-") {
		var parts []string
		tier := translit.TierDict
		for _, seg := range strings.Split(w, "-") {
			if len(lettersOf(seg)) < 2 {
				return "", translit.TierNone, false
			}
			s, st, ok := t.word(seg, tr)
			if !ok {
				return "", translit.TierNone, false
			}
			parts = append(parts, s)
			if st < tier {
				tier = st
			}
		}
		return strings.Join(parts, t.hyphen()), tier, true
	}
	ph := translit.SpellingPhones(lettersOf(w))
	s, ok := t.convert(ph, lettersOf(w), tr)
	return s, translit.TierSpelling, ok
}

func lettersOf(w string) string {
	var b strings.Builder
	for i := 0; i < len(w); i++ {
		c := w[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c >= 'a' && c <= 'z' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func (t *Transliterator) convert(ph []translit.Phone, letters string, tr *tracer) (string, bool) {
	syls, ok := Syllabify(ph, letters)
	if !ok {
		return "", false
	}
	if t.lang == "ja" {
		return jaRender(syls, tr)
	}
	return koRender(syls, letters, tr)
}

// FromPhones renders ARPAbet phonemes with their spelling directly (tests,
// spec 044 §5), with the rule ids that fired.
func (t *Transliterator) FromPhones(ph []translit.Phone, letters string) (string, []string, bool) {
	tr := &tracer{}
	s, ok := t.convert(ph, letters, tr)
	return s, tr.ids, ok
}

func dedupSort(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
