package buckrogers

// Spec 034 (Buck repo): an English keyword row under the manual paragraph.
// The excerpt file is local-only (ignored workplace); its words are never
// logged, summarised or used as stamp keys.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

const (
	manualEnglishColumns = 36
	manualEnglishLastRow = 13 // 0-based rows of the 36×14 manual grid
	manualEnglishMaxWord = 8
)

type manualEnglishRow struct {
	row  int
	text []rune
}

// ManualEnglish holds the precomputed rows for each question it can show.
type ManualEnglish struct {
	plans    map[string][]manualEnglishRow
	Excluded map[string]string // event key -> reason code (no content)
}

// manualEnglishTemplates reads text/manual-english-panel.zh-TW.tsv.
func manualEnglishTemplates(data []byte) (long, short string, err error) {
	rows, err := readTSV("manual-english-panel.zh-TW.tsv", data, []string{"key", "translation", "source"})
	if err != nil {
		return "", "", err
	}
	for _, r := range rows {
		if strings.Count(r[1], "{0}") != 1 || strings.Count(r[1], "{") != 1 || strings.Count(r[1], "}") != 1 {
			return "", "", fmt.Errorf("manual-english-panel.zh-TW.tsv: %s 必須恰有一個 {0}", r[0])
		}
		switch r[0] {
		case "manual.english.long":
			long = r[1]
		case "manual.english.short":
			short = r[1]
		}
	}
	if long == "" || short == "" {
		return "", "", fmt.Errorf("manual-english-panel.zh-TW.tsv: 缺少模板")
	}
	return long, short, nil
}

// manualEnglishLayout places the keyword row(s) below a paragraph of k rows
// (spec 034 §3.4).  Spec 039 §3.4: widths are half units, 72 per row; words
// are separated by one half-width space and each bracket of 「WORD」 takes
// two units.  It returns false when even the short form does not fit.
func manualEnglishLayout(k int, words []string, long, short string) ([]manualEnglishRow, bool) {
	const width = 2 * manualEnglishColumns
	n := len(words)
	last := "「" + words[n-1] + "」"
	first := k + 1
	if first > manualEnglishLastRow {
		return nil, false
	}
	units := append(append([]string{}, words[:n-1]...), last)
	var lines [][]rune
	cur := []rune(strings.Replace(long, "{0}", strconv.Itoa(n), 1))
	fits := textUnits(cur) <= width
	for _, u := range units {
		ur := []rune(u)
		if textUnits(ur) > width {
			fits = false
			break
		}
		gap := 1
		if len(cur) == 0 {
			gap = 0
		}
		if textUnits(cur)+gap+textUnits(ur) > width {
			lines = append(lines, cur)
			cur, gap = nil, 0
		}
		if gap == 1 {
			cur = append(cur, ' ')
		}
		cur = append(cur, ur...)
	}
	lines = append(lines, cur)
	if fits && first+len(lines)-1 <= manualEnglishLastRow {
		out := make([]manualEnglishRow, len(lines))
		for i, l := range lines {
			out[i] = manualEnglishRow{row: first + i, text: l}
		}
		return out, true
	}
	s := []rune(strings.Replace(short, "{0}", strconv.Itoa(n), 1) + last)
	if textUnits(s) > width {
		return nil, false
	}
	return []manualEnglishRow{{row: first, text: s}}, true
}

// manualEnglishCovered reports whether fonts draw every rune of plan.
// U+0020／U+3000 need no glyph.  A font narrower than 16 pixels is a spec
// 039 half font and is checked against half-width runes only; the others
// are checked against every rune.
func manualEnglishCovered(plan []manualEnglishRow, fonts []*xlate.Font) bool {
	for _, f := range fonts {
		if f == nil {
			return false
		}
		halfOnly := f.W < 16
		for _, pr := range plan {
			for _, ch := range pr.text {
				if isBlankRune(ch) || halfOnly && !isHalfwidth(ch) {
					continue
				}
				if _, ok := f.Glyphs[ch]; !ok {
					return false
				}
			}
		}
	}
	return true
}

func printableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7E {
			return false
		}
	}
	return s != ""
}

// LoadManualEnglish validates the local excerpt. A file-level problem
// returns an error carrying only a reason code; per-question problems
// (fonts, layout) exclude that question.
func LoadManualEnglish(data []byte, catalog *Catalog, panel []byte, fonts []*xlate.Font) (*ManualEnglish, error) {
	long, short, err := manualEnglishTemplates(panel)
	if err != nil {
		return nil, fmt.Errorf("manual-english: template")
	}
	// Plain tab split: English words may contain double quotes, which the
	// shared CSV-based readTSV would reject.
	var rows [][]string
	header := false
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 || f[0] == "" || f[1] == "" || f[2] == "" {
			return nil, fmt.Errorf("manual-english: format")
		}
		if !header {
			if f[0] != "event_key" || f[1] != "ordinal" || f[2] != "words" {
				return nil, fmt.Errorf("manual-english: format")
			}
			header = true
			continue
		}
		rows = append(rows, f)
	}
	if !header {
		return nil, fmt.Errorf("manual-english: format")
	}
	byEvent := map[string]catalogEntry{}
	ordinal := map[string]int{}
	for id, e := range catalog.byIdentity {
		byEvent[e.eventKey], ordinal[e.eventKey] = e, id.ordinal
	}
	m := &ManualEnglish{plans: map[string][]manualEnglishRow{}, Excluded: map[string]string{}}
	seen := map[string]bool{}
	for _, r := range rows {
		key := r[0]
		e, ok := byEvent[key]
		if !ok || seen[key] {
			return nil, fmt.Errorf("manual-english: event-key")
		}
		seen[key] = true
		n, err := strconv.Atoi(r[1])
		if err != nil || n != ordinal[key] {
			return nil, fmt.Errorf("manual-english: ordinal")
		}
		words := strings.Split(r[2], " ")
		if len(words) != n {
			return nil, fmt.Errorf("manual-english: word-count")
		}
		for _, w := range words {
			if !printableASCII(w) {
				return nil, fmt.Errorf("manual-english: ascii")
			}
		}
		if len(words[n-1]) > manualEnglishMaxWord {
			return nil, fmt.Errorf("manual-english: word-length")
		}
		paragraph, err := manualRows(e.translation, manualEnglishColumns, manualEnglishLastRow+1)
		if err != nil {
			m.Excluded[key] = "paragraph"
			continue
		}
		k := manualUsedRows(paragraph)
		plan, ok := manualEnglishLayout(k, words, long, short)
		if !ok {
			m.Excluded[key] = "layout"
			continue
		}
		if !manualEnglishCovered(plan, fonts) {
			m.Excluded[key] = "font"
			continue
		}
		m.plans[key] = plan
	}
	return m, nil
}

// Len is the number of questions that can show a keyword row.
func (m *ManualEnglish) Len() int {
	if m == nil {
		return 0
	}
	return len(m.plans)
}

// manualEnglishPresenter draws one scale's keyword rows, mirroring the
// manual presenter's visible request (spec 034 §3.3).
type manualEnglishPresenter struct {
	layer *xlate.Layer
	gen   uint64
	key   string
}

func (p *manualEnglishPresenter) sync(m *ManualEnglish, man *RuntimeManualOverlay, palette [256][3]uint8) bool {
	gen, key, ok := man.VisibleRequest()
	plan, has := m.plans[key]
	if !ok || !has || !man.HasStyle() {
		p.layer, p.gen, p.key = nil, 0, ""
		return false
	}
	if p.layer == nil || gen != p.gen || key != p.key {
		p.layer, p.gen, p.key = &xlate.Layer{W: 320, H: 200}, gen, key
		lay := man.layout
		fonts := man.segmentFonts()
		for i, r := range plan {
			// Spec 039 §3.3: pad to 72 units and draw as segments.
			text := padUnits(r.text, 2*lay.columns)
			for _, s := range fonts.segmentStamps(fmt.Sprintf("manual.english.%d.%d", gen, i), lay.textX,
				lay.clearY+r.row*(lay.lineHeight+lay.gap), text, nil,
				func(int) ([3]uint8, [3]uint8) { return [3]uint8{}, [3]uint8{} }) {
				s.CellH = lay.lineHeight
				p.layer.Add(s)
			}
		}
	}
	for _, s := range p.layer.Stamps {
		s.BG, s.FG = palette[man.style.Background], palette[man.style.Foreground]
	}
	return true
}

// manualUsedRows is the number of rows a laid-out paragraph occupies: the
// index of its last non-empty row plus one.  The keyword row of spec 034
// starts on the row after it.
func manualUsedRows(rows []string) int {
	k := 0
	for i, line := range rows {
		if line != "" {
			k = i + 1
		}
	}
	return k
}
