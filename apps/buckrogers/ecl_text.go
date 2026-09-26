package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Spec 027 (Buck repo): the ECL text-window printer at 0763:056C receives
// the whole Pascal string plus its window, so one content hash identifies
// every script message.  Nothing here reads the machine; LiveRuntime feeds
// values it read from the StepReader.
var (
	eclTextPrinter = Address{Segment: 0x0763, Offset: 0x056C}
	// RETF 12h pops the far return address (4 bytes) and nine words.
	eclTextReturnDelta uint16 = 4 + 0x12
)

type eclTextID struct {
	n   uint8
	sum [32]byte
}

// EclTextCatalog maps an exact original (length, SHA-256) to a key and
// keeps only translated keys; untranslated entries behave as a miss.
type EclTextCatalog struct {
	byID map[eclTextID]string
	text map[string]string
}

// LoadEclTextCatalog reads text/ecl-text-events.tsv and
// text/ecl-text.zh-TW.tsv.  Translation keys must exist in the events file.
func LoadEclTextCatalog(events, translations []byte) (*EclTextCatalog, error) {
	c := &EclTextCatalog{byID: map[eclTextID]string{}, text: map[string]string{}}
	rows, err := readTSV("ecl-text-events.tsv", events, []string{"event_key", "original_length", "original_sha256", "sources"})
	if err != nil {
		return nil, fmt.Errorf("buckrogers: ecl-text-events：%w", err)
	}
	keys := map[string]bool{}
	for _, r := range rows {
		n, err := strconv.Atoi(r[1])
		if err != nil || n <= 0 || n > 255 || keys[r[0]] {
			return nil, fmt.Errorf("buckrogers: ecl-text-events 列 %s 無效", r[0])
		}
		var id eclTextID
		id.n = uint8(n)
		if b, err := hex.DecodeString(r[2]); err != nil || len(b) != 32 {
			return nil, fmt.Errorf("buckrogers: ecl-text-events 列 %s 雜湊無效", r[0])
		} else {
			copy(id.sum[:], b)
		}
		if _, dup := c.byID[id]; dup {
			return nil, fmt.Errorf("buckrogers: ecl-text-events 雜湊重複：%s", r[0])
		}
		keys[r[0]] = true
		c.byID[id] = r[0]
	}
	rows, err = readTSV("ecl-text.zh-TW.tsv", translations, []string{"key", "translation", "source"})
	if err != nil {
		return nil, fmt.Errorf("buckrogers: ecl-text.zh-TW：%w", err)
	}
	for _, r := range rows {
		if !keys[r[0]] || r[1] == "" || c.text[r[0]] != "" {
			return nil, fmt.Errorf("buckrogers: ecl-text.zh-TW 列 %s 無效", r[0])
		}
		c.text[r[0]] = r[1]
	}
	return c, nil
}

// Lookup returns the key and translation for an exact original string.
func (c *EclTextCatalog) Lookup(original []byte) (key, text string, ok bool) {
	if c == nil || len(original) == 0 || len(original) > 255 {
		return "", "", false
	}
	key, ok = c.byID[eclTextID{uint8(len(original)), sha256.Sum256(original)}]
	if !ok {
		return "", "", false
	}
	text, ok = c.text[key]
	return key, text, ok
}

// Translated reports how many catalog keys carry a translation.
func (c *EclTextCatalog) Translated() int {
	if c == nil {
		return 0
	}
	return len(c.text)
}

// EclTextEntry is the value read at 0763:056C.
type EclTextEntry struct {
	Step                     uint64
	SS, SP                   uint16
	Return                   Address
	Original                 []byte
	Clear                    bool
	Background, Foreground   uint8
	Left, Top, Right, Bottom uint8
	CursorCol, CursorRow     uint8
}

// EclTextLine is one laid-out row of Chinese inside the window.
type EclTextLine struct {
	Row, Col uint8
	Text     []rune
}

// EclTextPage is the window's current presentation: the mask rectangle
// (in 8×8 cells, inclusive) and the rows drawn inside it.
type EclTextPage struct {
	Generation               uint64
	Left, Top, Right, Bottom uint8
	Background, Foreground   uint8
	Lines                    []EclTextLine
	Keys                     []string
}

// EclTextWatcher implements spec 027 §3.2–§3.5.
type EclTextWatcher struct {
	catalog *EclTextCatalog
	engine  *EngineTextCatalog // spec 029 fallback; nil disables
	page    *EclTextPage
	endRow  uint8 // Chinese cursor after the last presentation
	endCol  uint8
	inCall  bool
	call    EclTextEntry
	gen     uint64
	Stats   EclTextStats
}

type EclTextStats struct{ Hits, Misses, Overflows, Invalidations, Reentries int }

func NewEclTextWatcher(c *EclTextCatalog) *EclTextWatcher {
	return &EclTextWatcher{catalog: c, gen: 1}
}

// SetEngine installs the spec-029 fallback for strings the ECL catalog misses.
func (w *EclTextWatcher) SetEngine(c *EngineTextCatalog) { w.engine = c }

// Page returns the active presentation or nil.
func (w *EclTextWatcher) Page() *EclTextPage {
	if w == nil {
		return nil
	}
	return w.page
}

func (w *EclTextWatcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.gen
}

// InCall reports whether a hit call is still in flight (for the fast path).
func (w *EclTextWatcher) InCall() bool { return w != nil && w.inCall }

func (w *EclTextWatcher) invalidate() {
	if w.page != nil {
		w.page = nil
		w.gen++
		w.Stats.Invalidations++
	}
	w.inCall = false
}

// ObserveEntry handles CS:IP = 0763:056C.
func (w *EclTextWatcher) ObserveEntry(e EclTextEntry) {
	if w == nil {
		return
	}
	if w.inCall {
		w.Stats.Reentries++
		w.invalidate()
		return
	}
	valid := e.Left <= 0x27 && e.Top <= 0x18 && e.Right <= 0x27 && e.Bottom <= 0x27 &&
		e.Left <= e.Right && e.Top <= e.Bottom && e.Bottom <= 24
	if !valid {
		w.invalidate()
		return
	}
	key, text, ok := w.catalog.Lookup(e.Original)
	if !ok && w.engine != nil {
		if text, ok = w.engine.Translate(string(e.Original)); ok {
			key = "engine"
		}
	}
	if !ok {
		w.Stats.Misses++
		w.invalidate()
		return
	}
	outside := e.CursorCol < e.Left || e.CursorCol > e.Right || e.CursorRow < e.Top || e.CursorRow > e.Bottom
	p := w.page
	sameWindow := p != nil && p.Left == e.Left && p.Right == e.Right && p.Bottom == e.Bottom
	fresh := e.Clear || outside || !sameWindow
	var row, col, first uint8
	switch {
	case e.Clear || outside:
		row, col, first = e.Top, e.Left, e.Top
	case !sameWindow:
		// A continuation after text we did not translate: start where the
		// original will print and never mask the English above it.
		row, col, first = e.CursorRow, e.CursorCol, e.CursorRow
	default:
		row, col, first = w.endRow, w.endCol, p.Top
		if e.CursorCol == e.Left && col != e.Left {
			row, col = row+1, e.Left
		}
		// The English continuation starts at the original cursor; the mask
		// must reach it even when the Chinese sits higher.
		if e.CursorRow < first {
			first = e.CursorRow
		}
	}
	lines, endRow, endCol, fits := layoutEclText([]rune(text), row, col, e.Left, e.Right, e.Bottom)
	if !fits {
		w.Stats.Overflows++
		w.invalidate()
		return
	}
	next := &EclTextPage{Left: e.Left, Top: first, Right: e.Right, Bottom: e.Bottom,
		Background: e.Background, Foreground: e.Foreground}
	if !fresh {
		next.Lines = append(next.Lines, p.Lines...)
		next.Keys = append(next.Keys, p.Keys...)
	}
	next.Lines = append(next.Lines, lines...)
	next.Keys = append(next.Keys, key)
	w.gen++
	next.Generation = w.gen
	w.page = next
	w.endRow, w.endCol = endRow, endCol
	w.inCall = true
	w.call = e
	w.Stats.Hits++
}

// ObserveInstruction closes a hit call at its verified far return.
func (w *EclTextWatcher) ObserveInstruction(at Address, ss, sp uint16) {
	if w == nil || !w.inCall {
		return
	}
	if at == w.call.Return && ss == w.call.SS && sp == w.call.SP+eclTextReturnDelta {
		w.inCall = false
	}
}

// ObserveVideoWrite applies §3.5 rule 3 to one A000 byte offset.
func (w *EclTextWatcher) ObserveVideoWrite(offset uint32) {
	if w == nil || w.page == nil || w.inCall || offset >= 320*200 {
		return
	}
	x, y := offset%320, offset/320
	p := w.page
	if x >= uint32(p.Left)*8 && x < (uint32(p.Right)+1)*8 && y >= uint32(p.Top)*8 && y < (uint32(p.Bottom)+1)*8 {
		w.invalidate()
	}
}

// ObserveDiscontinuity handles restore / observer fault.
func (w *EclTextWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.invalidate()
	}
}

// layoutEclText places text from (row,col) inside [left,right]×[..bottom].
// Latin/digit runs stay together; closing punctuation never starts a line.
func layoutEclText(text []rune, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	var tokens [][]rune
	for i := 0; i < len(text); {
		j := i + 1
		if isEclLatin(text[i]) {
			for j < len(text) && isEclLatin(text[j]) {
				j++
			}
		}
		// Attach trailing closing punctuation to the token before it.
		for j < len(text) && isEclClosing(text[j]) {
			j++
		}
		tokens = append(tokens, text[i:j])
		i = j
	}
	width := int(right) - int(left) + 1
	var lines []EclTextLine
	cur := EclTextLine{Row: row, Col: col}
	used := int(col) - int(left)
	for _, t := range tokens {
		if used+len(t) > width && used > 0 {
			for len(cur.Text) > 0 && cur.Text[len(cur.Text)-1] == ' ' {
				cur.Text = cur.Text[:len(cur.Text)-1]
			}
			if len(cur.Text) > 0 {
				lines = append(lines, cur)
			}
			row++
			cur = EclTextLine{Row: row, Col: left}
			used = 0
			if t[0] == ' ' {
				t = t[1:]
			}
		}
		if row > bottom || len(t) > width {
			return nil, 0, 0, false
		}
		cur.Text = append(cur.Text, t...)
		used += len(t)
	}
	if len(cur.Text) > 0 {
		lines = append(lines, cur)
	}
	end := uint8(int(left) + used)
	if row > bottom {
		return nil, 0, 0, false
	}
	return lines, row, end, true
}

func isEclLatin(r rune) bool {
	return r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '\'' || r == '-')
}

func isEclClosing(r rune) bool {
	return strings.ContainsRune("，。！？：；、」）……》』,.!?:;)", r)
}
