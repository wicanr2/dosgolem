package buckrogers

import (
	"sort"
	"strconv"
)

// Helpers for the spec-036 static scan (cmd/buckrogers-name-scan).

// LoadNameGlossaryDir reads name-glossary.tsv and name-glossary-exclude.tsv
// from a text directory; nil without error when the glossary is absent.
func LoadNameGlossaryDir(textDir string) (*NameGlossary, error) { return loadNameGlossary(textDir) }

// LoadNameGlossaryDirLang is LoadNameGlossaryDir for one language: zh-TW
// reads name-glossary.tsv, another language name-glossary.<lang>.tsv and
// name-glossary-exclude.<lang>.tsv from the same directory.
func LoadNameGlossaryDirLang(textDir, lang string) (*NameGlossary, error) {
	return loadNameGlossaryLang(textDir, textDir, lang)
}

// Keys lists the translated keys in order.
func (c *EclTextCatalog) Keys() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.text))
	for k := range c.text {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Text returns the translation of a key.
func (c *EclTextCatalog) Text(key string) string { return c.text[key] }

// LayoutEclAnnotated lays one tier out in a window from (row, col) and
// reports the rows used and whether it fits.  Columns are 8×8 cells; the
// layout itself runs in half units (spec 039 §3.4).
func LayoutEclAnnotated(a AnnotatedText, row, col, left, right, bottom uint8) (int, bool) {
	return LayoutEclAnnotatedLang(LangZhTW, a, row, col, left, right, bottom)
}

// LayoutEclAnnotatedLang is LayoutEclAnnotated with the layout profile of a
// language (spec 042 §3.4, spec 043 §3.4).
func LayoutEclAnnotatedLang(lang string, a AnnotatedText, row, col, left, right, bottom uint8) (int, bool) {
	lines, _, _, ok := layoutEclTextP(LayoutFor(lang), a.Text, a.Units, row, eclUnitLeft(col), eclUnitLeft(left), eclUnitRight(right), bottom)
	return len(lines), ok
}

// LayoutLogbookAnnotated lays one logbook tier out; err means over 3 pages.
func LayoutLogbookAnnotated(a AnnotatedText) ([][]string, error) {
	return LayoutLogbookAnnotatedLang(LangZhTW, a)
}

// LayoutLogbookAnnotatedLang is LayoutLogbookAnnotated with the layout
// profile of a language.
func LayoutLogbookAnnotatedLang(lang string, a AnnotatedText) ([][]string, error) {
	return layoutLogbookUnitsP(LayoutFor(lang), a.Text, a.Units)
}

// LogbookEntries lists the entry numbers in order.
func (c *LogbookCatalog) LogbookEntries() []int {
	var out []int
	for n := range c.entries {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// Entry returns the laid-out entry.
func (c *LogbookCatalog) Entry(n int) LogbookEntry { return c.entries[n] }

// TitleRow is the whole title row as the panel draws it.
func (c *LogbookCatalog) TitleRow(n int, title string) string {
	return fillLogbook(c.titleFmt, strconv.Itoa(n), title)
}

// LogbookTitleUnits is the title row width limit in half units.
const LogbookTitleUnits = logbookTitleUnits

// TextUnits is the spec 039 width of s in half units.
func TextUnits(s string) int { return stringUnits(s) }

// ReadCatalogRows reads a key/translation/source catalog into key → text.
func ReadCatalogRows(name string, data []byte) (map[string]string, error) {
	rows, err := readTSV(name, data, []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r[0]] = r[1]
	}
	return out, nil
}
