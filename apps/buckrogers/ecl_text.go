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
	// Player is the spec-038 context read at the entry; nil disables the
	// player-name check for this call.
	Player *EclPlayerContext
}

// EclTextLine is one laid-out row of Chinese inside the window.
type EclTextLine struct {
	Row, Col uint8
	Text     []rune
}

// EclTextPage is one window's presentation: the mask rectangle (in 8×8
// cells, inclusive) and the rows drawn inside it.
type EclTextPage struct {
	Generation               uint64
	Left, Top, Right, Bottom uint8
	TopCol                   uint8 // first masked column of the Top row (spec 027 §3.4)
	Background, Foreground   uint8
	Lines                    []EclTextLine
	Keys                     []string
	Gone                     uint32 // rows removed by later original writes (bit = row)
	endRow, endCol           uint8  // Chinese cursor after the last string
}

// Shows reports whether the page still masks the given row.
func (p *EclTextPage) Shows(row uint8) bool { return p.Gone&(1<<row) == 0 }

func (p *EclTextPage) live() bool {
	for _, l := range p.Lines {
		if p.Shows(l.Row) {
			return true
		}
	}
	return false
}

func (p *EclTextPage) sameWindow(e EclTextEntry) bool {
	return p.Left == e.Left && p.Right == e.Right && p.Bottom == e.Bottom
}

func (p *EclTextPage) intersects(l, t, r, b uint8) bool {
	return p.Left <= r && l <= p.Right && p.Top <= b && t <= p.Bottom
}

// EclTextWatcher implements spec 027 §3.2–§3.7: one page per text window.
type EclTextWatcher struct {
	catalog *EclTextCatalog
	engine  *EngineTextCatalog // spec 029 fallback; nil disables
	names   *NameGlossary      // spec 036 annotation; nil disables
	players *PlayerNames       // spec 038 player names; nil disables
	pages   []*EclTextPage
	inCall  bool
	call    EclTextEntry
	gen     uint64
	Stats   EclTextStats
}

type EclTextStats struct {
	Hits, Misses, Overflows, Invalidations, Reentries, Passthrough int
	// Spec 036 §3.3: hits whose name annotation had to step down to
	// "first occurrence only" or "none" to fit.  Counted apart from
	// Overflows (the whole window falls back to English).
	NameFirstOnly, NameUnannotated int
	// Spec 038 §3.3: player-name calls drawn as 中文(英文), stepped down to
	// Chinese only, or stepped down to the original English.
	PlayerNames, PlayerNameChineseOnly, PlayerNameEnglish int
}

func NewEclTextWatcher(c *EclTextCatalog) *EclTextWatcher {
	return &EclTextWatcher{catalog: c, gen: 1}
}

// SetNames installs the spec-036 glossary; only ECL catalog hits are
// annotated (engine translations and passthrough are narrow families).
func (w *EclTextWatcher) SetNames(g *NameGlossary) { w.names = g }

// SetPlayerNames installs the spec-038 resolver for player names.
func (w *EclTextWatcher) SetPlayerNames(p *PlayerNames) { w.players = p }

// SetEngine installs the spec-029 fallback for strings the ECL catalog misses.
func (w *EclTextWatcher) SetEngine(c *EngineTextCatalog) { w.engine = c }

// Pages returns the active presentations.
func (w *EclTextWatcher) Pages() []*EclTextPage {
	if w == nil {
		return nil
	}
	return w.pages
}

// Page returns the first active presentation (tests and single-window use).
func (w *EclTextWatcher) Page() *EclTextPage {
	if w == nil || len(w.pages) == 0 {
		return nil
	}
	return w.pages[0]
}

func (w *EclTextWatcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.gen
}

// InCall reports whether a hit call is still in flight (for the fast path).
func (w *EclTextWatcher) InCall() bool { return w != nil && w.inCall }

func (w *EclTextWatcher) drop(keep func(*EclTextPage) bool) {
	n := 0
	for _, p := range w.pages {
		if keep(p) {
			w.pages[n] = p
			n++
		}
	}
	if n != len(w.pages) {
		w.pages = w.pages[:n]
		w.gen++
		w.Stats.Invalidations++
	}
}

// removeRows implements the row-level invalidation of §3.7: rows of other
// pages inside [t,b] whose columns meet [l,r] stop masking.
func (w *EclTextWatcher) removeRows(l, t, r, b uint8, except *EclTextPage) {
	changed := false
	n := 0
	for _, p := range w.pages {
		if p != except && p.intersects(l, t, r, b) {
			for row := max(t, p.Top); row <= min(b, p.Bottom); row++ {
				// The Top row masks only [TopCol, Right] (§3.7 revision).
				if row == p.Top && r < p.TopCol {
					continue
				}
				if p.Shows(row) {
					p.Gone |= 1 << row
					changed = true
				}
			}
			if !p.live() {
				continue
			}
		}
		w.pages[n] = p
		n++
	}
	if changed || n != len(w.pages) {
		w.pages = w.pages[:n]
		w.gen++
		w.Stats.Invalidations++
	}
}

func (w *EclTextWatcher) invalidate() {
	w.drop(func(*EclTextPage) bool { return false })
	w.inCall = false
}

func (w *EclTextWatcher) window(e EclTextEntry) (int, *EclTextPage) {
	for i, p := range w.pages {
		if p.sameWindow(e) {
			return i, p
		}
	}
	return -1, nil
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
	outside := e.CursorCol < e.Left || e.CursorCol > e.Right || e.CursorRow < e.Top || e.CursorRow > e.Bottom
	fresh := e.Clear || outside
	idx, p := w.window(e)
	if fresh {
		// The original clears this window: rows it overlaps are stale.
		if p != nil {
			w.drop(func(q *EclTextPage) bool { return q != p })
		}
		w.removeRows(e.Left, e.Top, e.Right, e.Bottom, nil)
		idx, p = w.window(e)
	}
	// Spec 038 §3.2: a player name goes before the 027 catalog and 029.
	var player []AnnotatedText
	isPlayer := false
	if w.players != nil {
		if m, hit := eclPlayerName(e.Player, e.Original); hit {
			isPlayer = true
			if zh, ok := w.players.Chinese(m.Name, m.Gender); ok {
				full := []rune(zh + "(" + string(e.Original) + ")")
				cn := []rune(zh)
				player = []AnnotatedText{
					{Tier: NameTierAll, Text: full, Units: []NameUnit{{0, len(full)}}},
					{Tier: NameTierNone, Text: cn, Units: []NameUnit{{0, len(cn)}}},
				}
			}
		}
	}
	var (
		key, text string
		ok        bool
	)
	if isPlayer {
		key, ok = "player-name", len(player) != 0
	} else {
		key, text, ok = w.catalog.Lookup(e.Original)
		if !ok && w.engine != nil {
			if text, ok = w.engine.Translate(string(e.Original)); ok {
				key = "engine"
			}
		}
	}
	if !ok {
		w.Stats.Misses++
		if p == nil {
			// Nothing of ours in this window: the original shows as is.
			return
		}
		// §3.7: a continuation we cannot translate joins the page verbatim.
		text, key = string(e.Original), "passthrough"
		w.Stats.Passthrough++
	}
	var row, col, first, topCol uint8
	switch {
	case fresh:
		row, col, first, topCol = e.Top, e.Left, e.Top, e.Left
	case p == nil:
		// A continuation after text we did not translate: start where the
		// original will print and never mask the English above it, nor the
		// name or English left of the cursor on its row (§3.4 start column).
		row, col, first, topCol = e.CursorRow, e.CursorCol, e.CursorRow, e.CursorCol
	default:
		row, col, first, topCol = p.endRow, p.endCol, p.Top, p.TopCol
		if e.CursorCol == e.Left && col != e.Left {
			row, col = row+1, e.Left
		}
		if e.CursorRow < first {
			first, topCol = e.CursorRow, e.Left
		}
	}
	var (
		lines          []EclTextLine
		endRow, endCol uint8
		fits           bool
	)
	if key == "player-name" {
		// Spec 038 §3.3: 中文(英文) → Chinese only → the original English
		// through the existing passthrough.  Every try starts from the same
		// cursor and continuation state.
		chosen := -1
		for i, v := range player {
			if lines, endRow, endCol, fits = layoutEclTextUnits(v.Text, v.Units, row, col, e.Left, e.Right, e.Bottom); fits {
				chosen = i
				break
			}
		}
		switch chosen {
		case 0:
			w.Stats.PlayerNames++
		case 1:
			w.Stats.PlayerNames++
			w.Stats.PlayerNameChineseOnly++
		default:
			w.Stats.PlayerNameEnglish++
			if p == nil {
				return
			}
			text, key = string(e.Original), "passthrough"
			w.Stats.Passthrough++
			lines, endRow, endCol, fits = layoutEclText([]rune(text), row, col, e.Left, e.Right, e.Bottom)
		}
	} else {
		variants := []AnnotatedText{{Tier: NameTierNone, Text: []rune(text)}}
		if key != "engine" && key != "passthrough" && w.names != nil {
			variants = w.names.Variants(text, key, NameCaseUpper)
		}
		// Every tier starts from the same cursor and continuation state; the
		// layout is pure, so a failed try changes nothing (spec 036 §3.3).
		var tier NameTier
		for i, v := range variants {
			if lines, endRow, endCol, fits = layoutEclTextUnits(v.Text, v.Units, row, col, e.Left, e.Right, e.Bottom); fits {
				if i > 0 {
					tier = v.Tier
				}
				break
			}
		}
		switch tier {
		case NameTierFirst:
			w.Stats.NameFirstOnly++
		case NameTierNone:
			w.Stats.NameUnannotated++
		}
	}
	if !fits {
		w.Stats.Overflows++
		if p != nil {
			w.drop(func(q *EclTextPage) bool { return q != p })
		}
		return
	}
	next := &EclTextPage{Left: e.Left, Top: first, Right: e.Right, Bottom: e.Bottom, TopCol: topCol,
		Background: e.Background, Foreground: e.Foreground, endRow: endRow, endCol: endCol}
	if p != nil {
		next.Lines = append(next.Lines, p.Lines...)
		next.Keys = append(next.Keys, p.Keys...)
		// Rows this call prints on mask again.
		next.Gone = p.Gone &^ (^uint32(0) << min(row, e.CursorRow))
	}
	next.Lines = append(next.Lines, lines...)
	next.Keys = append(next.Keys, key)
	w.gen++
	next.Generation = w.gen
	if idx >= 0 {
		w.pages[idx] = next
	} else {
		w.pages = append(w.pages, next)
	}
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

// ObserveVideoWrite applies §3.5 rule 3 per page.
func (w *EclTextWatcher) ObserveVideoWrite(offset uint32) {
	if w == nil || len(w.pages) == 0 || w.inCall || offset >= 320*200 {
		return
	}
	col, row := uint8(offset%320/8), uint8(offset/320/8)
	w.removeRows(col, row, col, row, nil)
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
	return layoutEclTextUnits(text, nil, row, col, left, right, bottom)
}

// layoutEclTextUnits is layoutEclText where each unit (a spec-036 name
// annotation, sorted and disjoint) is one unbreakable token; a unit wider
// than the line breaks only at the spaces of its English part.
func layoutEclTextUnits(text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	width := int(right) - int(left) + 1
	var tokens [][]rune
	for i, u := 0, 0; i < len(text); {
		for u < len(units) && units[u].End <= i {
			u++
		}
		if u < len(units) && units[u].Start == i {
			j := units[u].End
			for j < len(text) && isEclClosing(text[j]) {
				j++
			}
			tok := text[i:j]
			if len(tok) <= width {
				tokens = append(tokens, tok)
			} else {
				// Break before each space inside the unit; the space leads
				// the next piece and is dropped at a line start.
				from := 0
				for k := 1; k < units[u].End-i; k++ {
					if tok[k] == ' ' {
						tokens = append(tokens, tok[from:k])
						from = k
					}
				}
				tokens = append(tokens, tok[from:])
			}
			i = j
			continue
		}
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
		// Never run into the next unit.
		if u < len(units) && j > units[u].Start {
			j = max(units[u].Start, i+1)
		}
		tokens = append(tokens, text[i:j])
		i = j
	}
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
