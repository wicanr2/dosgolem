package buckrogers

import (
	"sort"
	"strconv"
)

// Helpers for the spec-036 static scan (cmd/buckrogers-name-scan).

// LoadNameGlossaryDir reads name-glossary.tsv and name-glossary-exclude.tsv
// from a text directory; nil without error when the glossary is absent.
func LoadNameGlossaryDir(textDir string) (*NameGlossary, error) { return loadNameGlossary(textDir) }

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
// reports the rows used and whether it fits.
func LayoutEclAnnotated(a AnnotatedText, row, col, left, right, bottom uint8) (int, bool) {
	lines, _, _, ok := layoutEclTextUnits(a.Text, a.Units, row, col, left, right, bottom)
	return len(lines), ok
}

// LayoutLogbookAnnotated lays one logbook tier out; err means over 3 pages.
func LayoutLogbookAnnotated(a AnnotatedText) ([][]string, error) {
	return layoutLogbookUnits(a.Text, a.Units)
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

// LogbookTitleCells is the title row width limit.
const LogbookTitleCells = logbookTitleCells

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
