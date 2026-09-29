package buckrogers

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// Spec 036 §3.3 (Buck repo): wide text families show named people as
// 中文(ENGLISH).  The glossary only changes what is drawn; catalog keys,
// hashes and the original strings never see the annotation.

// NameGlossaryEntry is one row of text/name-glossary.tsv.
type NameGlossaryEntry struct {
	English, EnglishMixed, Chinese, Kind, Person string
	chinese                                      []rune
}

type nameExclusion struct {
	phrase []rune
	scope  string
}

// NameGlossary holds the names longest Chinese first and the exclusion
// phrases (text/name-glossary-exclude.tsv) that are never annotated.
type NameGlossary struct {
	names      []NameGlossaryEntry
	exclusions []nameExclusion
}

// NameCase picks the English spelling for a family: ECL prints upper case,
// the logbook uses the mixed-case spelling (upper case when it is empty).
type NameCase int

const (
	NameCaseUpper NameCase = iota
	NameCaseMixed
)

// NameTier is one step of the three-step fallback of §3.3.
type NameTier int

const (
	NameTierAll   NameTier = iota // every occurrence annotated
	NameTierFirst                 // only the first occurrence of each person
	NameTierNone                  // no annotation
)

func (t NameTier) String() string {
	switch t {
	case NameTierAll:
		return "all"
	case NameTierFirst:
		return "first"
	}
	return "none"
}

var (
	nameGlossaryHeader = []string{"english", "english_mixed", "chinese", "kind", "person", "basis", "note"}
	nameExcludeHeader  = []string{"phrase", "scope", "note"}
)

// readPlainTSV splits a TSV without quoting (the Python tool reads these
// files with QUOTE_NONE) and allows empty fields.
func readPlainTSV(name string, data []byte, header []string) ([][]string, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: 不是有效 UTF-8", name)
	}
	s := string(data)
	if strings.HasPrefix(s, "\uFEFF") || strings.ContainsRune(s, '\r') {
		return nil, fmt.Errorf("%s: 不允許 BOM 或 CR", name)
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) == 0 || !equalStrings(strings.Split(lines[0], "\t"), header) {
		return nil, fmt.Errorf("%s: 標頭不符", name)
	}
	var rows [][]string
	for i, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) != len(header) {
			return nil, fmt.Errorf("%s:%d: 必須恰有 %d 欄", name, i+2, len(header))
		}
		rows = append(rows, f)
	}
	return rows, nil
}

// LoadNameGlossary reads the glossary and the exclusion list (nil or empty
// exclusion data means no exclusions).
func LoadNameGlossary(glossary, exclude []byte) (*NameGlossary, error) {
	rows, err := readPlainTSV("name-glossary.tsv", glossary, nameGlossaryHeader)
	if err != nil {
		return nil, fmt.Errorf("buckrogers: %w", err)
	}
	g := &NameGlossary{}
	seenEn, seenZh := map[string]bool{}, map[string]bool{}
	for i, r := range rows {
		e := NameGlossaryEntry{English: r[0], EnglishMixed: r[1], Chinese: r[2], Kind: r[3], Person: r[4]}
		switch {
		case e.English == "" || e.English != strings.ToUpper(e.English) || e.Chinese == "" || e.Person == "":
			return nil, fmt.Errorf("buckrogers: name-glossary.tsv:%d 無效", i+2)
		case e.EnglishMixed != "" && strings.ToUpper(e.EnglishMixed) != e.English:
			return nil, fmt.Errorf("buckrogers: name-glossary.tsv:%d english_mixed 與 english 不同拼法", i+2)
		case e.Kind != "full" && e.Kind != "short":
			return nil, fmt.Errorf("buckrogers: name-glossary.tsv:%d kind 無效", i+2)
		case seenEn[e.English] || seenZh[e.Chinese]:
			return nil, fmt.Errorf("buckrogers: name-glossary.tsv:%d 重複", i+2)
		case strings.ContainsAny(e.Chinese, " ()（）"):
			return nil, fmt.Errorf("buckrogers: name-glossary.tsv:%d chinese 含空白或括號", i+2)
		}
		seenEn[e.English], seenZh[e.Chinese] = true, true
		e.chinese = []rune(e.Chinese)
		g.names = append(g.names, e)
	}
	// Longest first; ties keep file order.
	sort.SliceStable(g.names, func(i, j int) bool { return len(g.names[i].chinese) > len(g.names[j].chinese) })
	if len(exclude) != 0 {
		rows, err := readPlainTSV("name-glossary-exclude.tsv", exclude, nameExcludeHeader)
		if err != nil {
			return nil, fmt.Errorf("buckrogers: %w", err)
		}
		for i, r := range rows {
			if r[0] == "" || r[1] == "" {
				return nil, fmt.Errorf("buckrogers: name-glossary-exclude.tsv:%d phrase、scope 不得為空", i+2)
			}
			if _, err := path.Match(r[1], ""); err != nil {
				return nil, fmt.Errorf("buckrogers: name-glossary-exclude.tsv:%d scope 無效", i+2)
			}
			g.exclusions = append(g.exclusions, nameExclusion{phrase: []rune(r[0]), scope: r[1]})
		}
	}
	return g, nil
}

// loadNameGlossary reads the two files from a text directory; a missing
// glossary disables annotation.
func loadNameGlossary(textDir string) (*NameGlossary, error) {
	gb, err := os.ReadFile(filepath.Join(textDir, "name-glossary.tsv"))
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	eb, err := os.ReadFile(filepath.Join(textDir, "name-glossary-exclude.tsv"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return LoadNameGlossary(gb, eb)
}

// NameUnit is a rune span [Start, End) that line breaking must not split.
type NameUnit struct{ Start, End int }

// AnnotatedText is a translation with the inserted annotations marked.
type AnnotatedText struct {
	Tier  NameTier
	Text  []rune
	Units []NameUnit
}

// NameMatch is one glossary hit found in a translation (for scans).
type NameMatch struct {
	Start, End int // rune span of the Chinese name in the input
	Entry      *NameGlossaryEntry
	Paren      bool // already followed by ( or （: never annotated
}

func runesAt(text []rune, i int, p []rune) bool {
	if i+len(p) > len(text) {
		return false
	}
	for k, r := range p {
		if text[i+k] != r {
			return false
		}
	}
	return true
}

// Matches scans left to right, longest name first, skipping exclusion
// phrases whose scope matches key.
func (g *NameGlossary) Matches(text []rune, key string) []NameMatch {
	if g == nil {
		return nil
	}
	skip := make([]int, len(text)) // end of an excluded span starting here, else 0
	for _, ex := range g.exclusions {
		if ok, _ := path.Match(ex.scope, key); !ok || len(ex.phrase) == 0 {
			continue
		}
		for i := 0; i+len(ex.phrase) <= len(text); i++ {
			if runesAt(text, i, ex.phrase) && skip[i] < i+len(ex.phrase) {
				skip[i] = i + len(ex.phrase)
			}
		}
	}
	var out []NameMatch
	for i := 0; i < len(text); {
		if skip[i] != 0 {
			i = skip[i]
			continue
		}
		hit := false
		for k := range g.names {
			e := &g.names[k]
			if !runesAt(text, i, e.chinese) {
				continue
			}
			end := i + len(e.chinese)
			// An excluded phrase starting inside the name wins.
			inner := false
			for j := i + 1; j < end; j++ {
				if skip[j] != 0 {
					inner = true
					break
				}
			}
			if inner {
				continue
			}
			paren := end < len(text) && (text[end] == '(' || text[end] == '（')
			out = append(out, NameMatch{Start: i, End: end, Entry: e, Paren: paren})
			i, hit = end, true
			break
		}
		if !hit {
			i++
		}
	}
	return out
}

func (e *NameGlossaryEntry) english(c NameCase) string {
	if c == NameCaseMixed && e.EnglishMixed != "" {
		return e.EnglishMixed
	}
	return e.English
}

// Annotate builds one tier.  NameTierFirst annotates a person only at its
// first occurrence in text; a name already followed by a parenthesis counts
// as that person's occurrence but is left as is.
func (g *NameGlossary) Annotate(text, key string, c NameCase, tier NameTier) AnnotatedText {
	in := []rune(text)
	out := AnnotatedText{Tier: tier}
	if g == nil || tier == NameTierNone {
		out.Text = in
		return out
	}
	seen := map[string]bool{}
	pos := 0
	for _, m := range g.Matches(in, key) {
		first := !seen[m.Entry.Person]
		seen[m.Entry.Person] = true
		if m.Paren || (tier == NameTierFirst && !first) {
			continue
		}
		out.Text = append(out.Text, in[pos:m.Start]...)
		start := len(out.Text)
		out.Text = append(out.Text, in[m.Start:m.End]...)
		out.Text = append(out.Text, '(')
		out.Text = append(out.Text, []rune(m.Entry.english(c))...)
		out.Text = append(out.Text, ')')
		out.Units = append(out.Units, NameUnit{start, len(out.Text)})
		pos = m.End
	}
	out.Text = append(out.Text, in[pos:]...)
	return out
}

// Variants returns the distinct tiers in fallback order (all → first →
// none); tiers whose text equals the previous one are left out, so a text
// without names yields only one variant.
func (g *NameGlossary) Variants(text, key string, c NameCase) []AnnotatedText {
	var out []AnnotatedText
	for _, t := range []NameTier{NameTierAll, NameTierFirst, NameTierNone} {
		a := g.Annotate(text, key, c, t)
		if len(out) != 0 && string(out[len(out)-1].Text) == string(a.Text) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ChineseFor returns the chinese of the row whose `english` is exactly name
// (spec 038 §3.2: a player named like a glossary person uses its name).
func (g *NameGlossary) ChineseFor(name string) (string, bool) {
	if g == nil {
		return "", false
	}
	for _, e := range g.names {
		if e.English == name {
			return e.Chinese, true
		}
	}
	return "", false
}
