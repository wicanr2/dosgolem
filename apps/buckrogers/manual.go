// Package buckrogers contains program-specific observation adapters for
// Buck Rogers: Countdown to Doomsday.
package buckrogers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Address is a 16-bit DOS runtime segment:offset address.
type Address struct {
	Segment uint16 `json:"segment"`
	Offset  uint16 `json:"offset"`
}

var (
	manualBegin = Address{Segment: 0x2A33, Offset: 0x01ED}
	manualClear = Address{Segment: 0x026F, Offset: 0x029C}
	manualSteps = [...]struct {
		caller Address
		field  field
	}{
		{Address{0x2A33, 0x021B}, fieldPage},
		{Address{0x2A33, 0x0231}, fieldFollowing},
		{Address{0x2A33, 0x027A}, fieldHeading},
		{Address{0x2A33, 0x02A5}, fieldQuestion},
		{Address{0x2A33, 0x02E2}, fieldOrdinal},
		{Address{0x2A33, 0x0309}, fieldTail},
	}
)

const manualBeginText = "In the Log Book on page"

type field uint8

const (
	fieldPage field = iota
	fieldFollowing
	fieldHeading
	fieldQuestion
	fieldOrdinal
	fieldTail
)

// Question is the visible, answer-free identity of one original manual check.
type Question struct {
	Page        int
	Heading     string
	OrdinalWord string
}

type pendingQuestion struct {
	generation uint64
	stage      int
	page       int
	heading    string
	ordinal    string
	poisoned   bool
}

// Collector implements docs/spec/007-buck-rogers-manual-event-adapter.md.
// The caller must only pass PostCall events after validating the dispatcher
// return guard; Collector intentionally does not inspect CPU or stack state.
type Collector struct {
	generation uint64
	pending    *pendingQuestion
	visible    Question
	hasVisible bool
}

// Generation returns the most recently started manual-question generation.
func (c *Collector) Generation() uint64 { return c.generation }

// Visible returns the last completed question that has not been invalidated.
func (c *Collector) Visible() (Question, bool) { return c.visible, c.hasVisible }

// BeginEntry starts a generation only for the exact proven question prefix.
func (c *Collector) BeginEntry(caller Address, text string) (uint64, bool) {
	if caller != manualBegin || text != manualBeginText || c.generation == ^uint64(0) {
		return 0, false
	}
	c.generation++
	c.pending = &pendingQuestion{generation: c.generation}
	c.visible = Question{}
	c.hasVisible = false
	return c.generation, true
}

// ClearEntry invalidates a completed question but preserves a question that is
// still being printed. Other callers have no effect.
func (c *Collector) ClearEntry(caller Address) {
	if caller == manualClear {
		c.visible = Question{}
		c.hasVisible = false
	}
}

// PostCall consumes one guarded dispatcher return. A stale generation is
// ignored; a malformed event in the current generation poisons that generation.
func (c *Collector) PostCall(generation uint64, caller Address, text string) (Question, bool) {
	p := c.pending
	if p == nil || generation != p.generation || p.poisoned {
		return Question{}, false
	}
	if p.stage >= len(manualSteps) || caller != manualSteps[p.stage].caller {
		p.poisoned = true
		return Question{}, false
	}

	normalized := strings.Join(strings.Fields(text), " ")
	switch manualSteps[p.stage].field {
	case fieldPage:
		if !asciiDigits(normalized) {
			p.poisoned = true
			return Question{}, false
		}
		page, err := strconv.Atoi(normalized)
		if err != nil || page < 1 || page > 999 {
			p.poisoned = true
			return Question{}, false
		}
		p.page = page
	case fieldFollowing:
		if normalized != "following the heading" {
			p.poisoned = true
			return Question{}, false
		}
	case fieldHeading:
		if normalized == "" {
			p.poisoned = true
			return Question{}, false
		}
		p.heading = normalized
	case fieldQuestion:
		if normalized != "what is the" {
			p.poisoned = true
			return Question{}, false
		}
	case fieldOrdinal:
		if normalized == "" {
			p.poisoned = true
			return Question{}, false
		}
		p.ordinal = normalized
	case fieldTail:
		if normalized != "word?" {
			p.poisoned = true
			return Question{}, false
		}
	}
	p.stage++
	if manualSteps[p.stage-1].field != fieldTail {
		return Question{}, false
	}
	if p.page == 0 || p.heading == "" || p.ordinal == "" {
		p.poisoned = true
		return Question{}, false
	}

	q := Question{Page: p.page, Heading: p.heading, OrdinalWord: p.ordinal}
	c.visible, c.hasVisible, c.pending = q, true, nil
	return q, true
}

func asciiDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// DisplayRequest contains only output-side translation data.
type DisplayRequest struct {
	Generation  uint64
	EventKey    string
	TextKey     string
	Translation string
}

type identity struct {
	page    int
	heading string
	ordinal int
}

type catalogEntry struct {
	eventKey    string
	textKey     string
	translation string
}

// Catalog is an immutable exact-match resolver built from the three formal TSV
// inputs. It never contains or accepts manual answers.
type Catalog struct {
	ordinals   map[string]int
	byIdentity map[identity]catalogEntry
}

var (
	ordinalHeader = []string{"number", "ordinal_ascii", "runtime_address", "slot_hex"}
	eventHeader   = []string{"event_key", "record_index", "page", "heading_ascii", "ordinal", "text_key"}
	textHeader    = []string{"key", "translation", "source"}
)

// LoadCatalog validates all inputs before returning a usable resolver.
func LoadCatalog(events, ordinals, translations []byte) (*Catalog, error) {
	ordinalRows, err := readTSV("manual-ordinals.tsv", ordinals, ordinalHeader)
	if err != nil {
		return nil, err
	}
	eventRows, err := readTSV("manual-events.tsv", events, eventHeader)
	if err != nil {
		return nil, err
	}
	textRows, err := readTSV("manual.zh-TW.tsv", translations, textHeader)
	if err != nil {
		return nil, err
	}

	words := make(map[string]int, 10)
	numbers := make(map[int]bool, 10)
	for _, row := range ordinalRows {
		n, err := positiveDecimal(row[0])
		if err != nil || n > 10 {
			return nil, fmt.Errorf("manual-ordinals.tsv: 無效序數 %q", row[0])
		}
		word := row[1]
		if !asciiLower(word) {
			return nil, fmt.Errorf("manual-ordinals.tsv: 無效序數詞 %q", word)
		}
		if numbers[n] || words[word] != 0 {
			return nil, fmt.Errorf("manual-ordinals.tsv: 重複序數或序數詞")
		}
		numbers[n], words[word] = true, n
	}
	if len(words) != 10 {
		return nil, fmt.Errorf("manual-ordinals.tsv: 必須恰有 1–10")
	}
	for n := 1; n <= 10; n++ {
		if !numbers[n] {
			return nil, fmt.Errorf("manual-ordinals.tsv: 缺少序數 %d", n)
		}
	}

	texts := make(map[string]string, len(textRows))
	for _, row := range textRows {
		if _, exists := texts[row[0]]; exists {
			return nil, fmt.Errorf("manual.zh-TW.tsv: 重複文字鍵 %q", row[0])
		}
		texts[row[0]] = row[1]
	}

	byIdentity := make(map[identity]catalogEntry, len(eventRows))
	eventKeys := make(map[string]bool, len(eventRows))
	usedTextKeys := make(map[string]bool, len(eventRows))
	for _, row := range eventRows {
		if _, err := nonnegativeDecimal(row[1]); err != nil {
			return nil, fmt.Errorf("manual-events.tsv: 無效 record index %q", row[1])
		}
		page, err := positiveDecimal(row[2])
		if err != nil {
			return nil, fmt.Errorf("manual-events.tsv: 無效頁碼 %q", row[2])
		}
		ordinal, err := positiveDecimal(row[4])
		if err != nil {
			return nil, fmt.Errorf("manual-events.tsv: 無效序數 %q", row[4])
		}
		if !numbers[ordinal] {
			return nil, fmt.Errorf("manual-events.tsv: 序數不在原版橋接表 %d", ordinal)
		}
		id := identity{page: page, heading: row[3], ordinal: ordinal}
		if _, exists := byIdentity[id]; exists {
			return nil, fmt.Errorf("manual-events.tsv: 重複題目身分")
		}
		if eventKeys[row[0]] {
			return nil, fmt.Errorf("manual-events.tsv: 重複事件鍵 %q", row[0])
		}
		if usedTextKeys[row[5]] {
			return nil, fmt.Errorf("manual-events.tsv: 重複事件文字鍵 %q", row[5])
		}
		translation, exists := texts[row[5]]
		if !exists {
			return nil, fmt.Errorf("manual-events.tsv: 文字鍵不在 catalog %q", row[5])
		}
		eventKeys[row[0]], usedTextKeys[row[5]] = true, true
		byIdentity[id] = catalogEntry{eventKey: row[0], textKey: row[5], translation: translation}
	}
	for key := range texts {
		if !usedTextKeys[key] {
			return nil, fmt.Errorf("manual.zh-TW.tsv: 孤兒文字鍵 %q", key)
		}
	}
	return &Catalog{ordinals: words, byIdentity: byIdentity}, nil
}

// Resolve returns a request only for an exact identity in the current generation.
func (c *Catalog) Resolve(generation, currentGeneration uint64, q Question) (DisplayRequest, bool) {
	if c == nil || generation == 0 || generation != currentGeneration {
		return DisplayRequest{}, false
	}
	ordinal, ok := c.ordinals[q.OrdinalWord]
	if !ok {
		return DisplayRequest{}, false
	}
	entry, ok := c.byIdentity[identity{page: q.Page, heading: q.Heading, ordinal: ordinal}]
	if !ok {
		return DisplayRequest{}, false
	}
	return DisplayRequest{Generation: generation, EventKey: entry.eventKey, TextKey: entry.textKey, Translation: entry.translation}, true
}

func readTSV(name string, data []byte, header []string) ([][]string, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: 不是有效 UTF-8", name)
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return nil, fmt.Errorf("%s: 不接受 BOM", name)
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	r.FieldsPerRecord = len(header)
	gotHeader, err := r.Read()
	if err != nil || !equalStrings(gotHeader, header) {
		return nil, fmt.Errorf("%s: 標頭不符", name)
	}
	var rows [][]string
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, value := range row {
			if value == "" {
				return nil, fmt.Errorf("%s: 欄位不得為空", name)
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: 不得為空", name)
	}
	return rows, nil
}

func positiveDecimal(s string) (int, error) {
	if !asciiDigits(s) {
		return 0, fmt.Errorf("不是 ASCII 十進位")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("不是正整數")
	}
	return n, nil
}

func nonnegativeDecimal(s string) (int, error) {
	if !asciiDigits(s) {
		return 0, fmt.Errorf("不是 ASCII 十進位")
	}
	return strconv.Atoi(s)
}

func asciiLower(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
