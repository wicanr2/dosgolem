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
	lang string // spec 047 §3.1: set at load, "" counts as zh-TW
}

// LoadEclTextCatalog reads text/ecl-text-events.tsv and
// text/ecl-text.zh-TW.tsv.  Translation keys must exist in the events file.
func LoadEclTextCatalog(events, translations []byte) (*EclTextCatalog, error) {
	return LoadEclTextCatalogLang(events, translations, LangZhTW)
}

// LoadEclTextCatalogLang reads text/ecl-text.<lang>.tsv (spec 040 §3.1).
func LoadEclTextCatalogLang(events, translations []byte, lang string) (*EclTextCatalog, error) {
	if lang == "" {
		lang = LangZhTW
	}
	c := &EclTextCatalog{byID: map[eclTextID]string{}, text: map[string]string{}, lang: lang}
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
	name := LangFile("ecl-text", lang)
	rows, err = readLangTSV(name, translations, []string{"key", "translation", "source"}, lang)
	if err != nil {
		return nil, fmt.Errorf("buckrogers: ecl-text.%s：%w", lang, err)
	}
	for _, r := range rows {
		if !keys[r[0]] || r[1] == "" || c.text[r[0]] != "" {
			return nil, fmt.Errorf("buckrogers: ecl-text.%s 列 %s 無效", lang, r[0])
		}
		c.text[r[0]] = r[1]
	}
	return c, nil
}

// fullStop reports whether the language draws a sentence-final period the
// original prints as a call of its own as the full-width 。 (spec 047 §3.1).
// Korean keeps the ASCII period; the test language zz is the control.
func (c *EclTextCatalog) fullStop() bool {
	return c != nil && (c.lang == LangZhTW || c.lang == LangZhCN || c.lang == LangJa)
}

// zhDeckSpace reports whether the language puts a space between 甲板 and the
// number the original prints as a call of its own (spec 048 §3.1): zh-TW and
// zh-CN (the loader has already turned "" into zh-TW).
func (c *EclTextCatalog) zhDeckSpace() bool {
	return c != nil && (c.lang == LangZhTW || c.lang == LangZhCN)
}

// zhDeckDigitCall implements conditions 3 to 5 of spec 048 §3.2 for a
// continuing call: the text starts with an ASCII digit, the call does not
// start at the left edge of the window and the last row drawn on the page
// ends with 甲板.
func zhDeckDigitCall(p *EclTextPage, text string, col, left uint8) bool {
	if p == nil || text == "" || text[0] < '0' || text[0] > '9' || col == left || len(p.Lines) == 0 {
		return false
	}
	return strings.HasSuffix(string(p.Lines[len(p.Lines)-1].Text), "甲板")
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
	// Party is the spec 054 §3.3 snapshot read at the callers whose sentence
	// carries a party member's name; nil: no name is looked up.
	Party *PartySnapshot
}

// EclTextLine is one laid-out row of Chinese inside the window; Col is the
// first half-unit column (spec 039 §3.4: unit = 4 logical pixels).
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
	endRow, endCol           uint8  // Chinese cursor after the last string (endCol in half units)
	// Spec 046 §3.4 (word-level profile only): the last character drawn, and
	// whether the last call ended with a space of the original that the
	// drawn text does not have (the next call starts with that space).
	lastRune  rune
	owedSpace bool
	// Spec 054 §3.2 (Korean only): the syllable the text drawn so far is read
	// as, for the particle mark that starts the next call; 0 when it ends with
	// anything else.
	lastReading rune
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
	layout  *LayoutProfile     // spec 042 §3.4 line-breaking rules; nil is the default
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
	// Spec 046 §3.4: calls whose inserted call-start space had to be left
	// out for the text to fit (the layout with the space failed, the one
	// without succeeded).
	SpaceDropped int
	// Spec 047 §3.2: calls that are a lone period (the original prints the
	// period of "DECK 5." as a call of its own) and were taken for the
	// full-width 。, and those among them that had to stay the original
	// period because 。 did not fit on the row.
	FullStop, FullStopDropped int
	// Spec 054 §3.3: sentences drawn with a party member's reading in place
	// of the English name (InlineNames), those whose reading did not fit and
	// were drawn as before (InlineNameFallback), and calls whose leading
	// Korean particle mark was replaced (MarkersResolved).
	InlineNames, InlineNameFallback, MarkersResolved int
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
		inline    int
	)
	if isPlayer {
		key, ok = "player-name", len(player) != 0
	} else {
		key, text, ok = w.catalog.Lookup(e.Original)
		if !ok && w.engine != nil {
			if text, ok, inline = w.engine.TranslateParty(string(e.Original), w.partyNames(e.Party)); ok {
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
	// Spec 047 §3.2: the period the original prints as a call of its own
	// (exactly "." or ". ") that joins a page of ours is drawn full-width
	// for the languages whose translations end their sentences with 。.
	// A player-name call that fell through to passthrough is not one.
	fullStop := key == "passthrough" && !isPlayer && w.catalog.fullStop() &&
		(string(e.Original) == "." || string(e.Original) == ". ")
	var row, col, first, topCol uint8
	switch {
	case fresh:
		row, col, first, topCol = e.Top, eclUnitLeft(e.Left), e.Top, e.Left
	case p == nil:
		// A continuation after text we did not translate: start where the
		// original will print and never mask the English above it, nor the
		// name or English left of the cursor on its row (§3.4 start column).
		row, col, first, topCol = e.CursorRow, eclUnitLeft(e.CursorCol), e.CursorRow, e.CursorCol
	default:
		row, col, first, topCol = p.endRow, p.endCol, p.Top, p.TopCol
		if e.CursorCol == e.Left && col != eclUnitLeft(e.Left) {
			row, col = row+1, eclUnitLeft(e.Left)
		}
		if e.CursorRow < first {
			first, topCol = e.CursorRow, e.Left
		}
	}
	// Spec 046 §3.4 (Korean, the word-level profile): a call that continues
	// text on the screen starts with a space when the original does (or the
	// call before it ended with one), unless the call starts at the left edge
	// of a row (the layout keeps a space there as an indent).
	spaceNeeded, prevRune := false, rune('A')
	if w.layout != nil && w.layout.word && !fresh && col != eclUnitLeft(e.Left) {
		owed := false
		if p != nil {
			prevRune, owed = p.lastRune, p.owedSpace
		}
		spaceNeeded = (len(e.Original) > 0 && e.Original[0] == ' ' || owed) && prevRune != 0 && prevRune != ' '
	}
	var (
		lines          []EclTextLine
		endRow, endCol uint8
		fits           bool
	)
	if key == "player-name" {
		// Spec 038 §3.3: 中文(英文) → Chinese only → the original English
		// through the existing passthrough.  Every try starts from the same
		// cursor and continuation state.  Spec 045 §3.4: when the call owes
		// a space (spaceNeeded, Korean only) each tier is tried with the
		// space first.
		c := layoutPlayerName(w.layout, player, e.Original, spaceNeeded, row, col, eclUnitLeft(e.Left), eclUnitRight(e.Right), e.Bottom)
		lines, endRow, endCol, fits = c.lines, c.endRow, c.endCol, c.fits
		switch c.kind {
		case playerFull:
			w.Stats.PlayerNames++
		case playerChineseOnly:
			w.Stats.PlayerNames++
			w.Stats.PlayerNameChineseOnly++
		default:
			w.Stats.PlayerNameEnglish++
			if p == nil {
				return
			}
			text, key = string(e.Original), "passthrough"
			w.Stats.Passthrough++
		}
		if fits && spaceNeeded && !c.spaced {
			w.Stats.SpaceDropped++
		}
	} else {
		// Spec 054 §3.2: a continuing Korean call that starts with a particle
		// mark takes the syllable the page was last read as.
		lead := rune(0)
		if w.catalog.ko() && p != nil && !fresh {
			lead = p.lastReading
		}
		// Spec 054 §3.3: the whole placement runs first with the sentence that
		// carries the party member's reading; when it does not fit, once more
		// with the current text.  Nothing is counted until one is chosen.
		pl := w.placeText(text, key, isPlayer, fullStop, fresh, p, lead, spaceNeeded, prevRune, row, col, e)
		if !pl.fits && inline > 0 {
			if plain, pok, _ := w.engine.TranslateParty(string(e.Original), nil); pok {
				w.Stats.InlineNameFallback++
				pl = w.placeText(plain, key, isPlayer, fullStop, fresh, p, lead, spaceNeeded, prevRune, row, col, e)
			}
		} else if inline > 0 {
			w.Stats.InlineNames++
		}
		lines, endRow, endCol, fits = pl.lines, pl.endRow, pl.endCol, pl.fits
		w.Stats.SpaceDropped += pl.spaceDropped
		w.Stats.FullStop += pl.fullStop
		w.Stats.FullStopDropped += pl.fullStopDropped
		if fits && pl.marker {
			w.Stats.MarkersResolved++
		}
		switch pl.tier {
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
	if w.layout != nil && w.layout.word {
		// Spec 046 §3.4: a call that drew nothing (empty original or only
		// spaces dropped by the layout) keeps the state of the page before.
		if p != nil {
			next.lastRune, next.owedSpace = p.lastRune, p.owedSpace
			if !fresh {
				next.lastReading = p.lastReading
			}
		}
		if n := len(lines); n > 0 && len(lines[n-1].Text) > 0 && len(e.Original) > 0 {
			last := lines[n-1].Text[len(lines[n-1].Text)-1]
			next.lastRune = last
			next.owedSpace = e.Original[len(e.Original)-1] == ' ' && last != ' '
			if w.catalog.ko() {
				// The whole call, not only its last row: an annotated name wider
				// than a row breaks at a space inside the annotation.
				var all []rune
				for _, l := range lines {
					all = append(all, l.Text...)
				}
				next.lastReading = koReadingTail(string(all))
			}
		}
	}
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

// ko reports the Korean catalog (spec 054 §3.2 applies to it only).
func (c *EclTextCatalog) ko() bool { return c != nil && c.lang == LangKo }

// partyNames is the spec 054 §3.3 resolver of a call's snapshot: only a
// Korean or Japanese lane with player names has one.
func (w *EclTextWatcher) partyNames(party *PartySnapshot) PartyNameFunc {
	if party == nil || w.engine == nil || w.players == nil || w.engine.lang != LangKo && w.engine.lang != LangJa {
		return nil
	}
	return party.NameFunc(w.players)
}

// eclPlacement is the layout of one non-player call and what it would add to
// the counters, which are only applied for the layout that is chosen.
type eclPlacement struct {
	lines                                   []EclTextLine
	endRow, endCol                          uint8
	fits                                    bool
	tier                                    NameTier
	marker                                  bool // the leading particle mark was replaced
	spaceDropped, fullStop, fullStopDropped int
}

// placeText lays out the text of one call of the ECL catalog, the engine
// catalog or the passthrough.  Every tier starts from the same cursor and
// continuation state; the layout is pure, so a failed try changes nothing
// (spec 036 §3.3).  Whether the call owes a space is decided on txt as it is;
// the leading particle mark is replaced after that (spec 054 §3.2), or the
// glue test would see a bare 이.
func (w *EclTextWatcher) placeText(txt, key string, isPlayer, fullStop, fresh bool, p *EclTextPage, lead rune, spaceNeeded bool, prevRune rune, row, col uint8, e EclTextEntry) (pl eclPlacement) {
	body := txt
	if lead != 0 {
		body, pl.marker = koResolveLeading(txt, lead)
	}
	attempt := func(t string) (ls []EclTextLine, er, ec uint8, ok bool, tier NameTier) {
		variants := []AnnotatedText{{Tier: NameTierNone, Text: []rune(t)}}
		if key != "engine" && key != "passthrough" && w.names != nil {
			variants = w.names.Variants(t, key, NameCaseUpper)
		}
		for i, v := range variants {
			if ls, er, ec, ok = layoutEclTextP(w.layout, v.Text, v.Units, row, col, eclUnitLeft(e.Left), eclUnitRight(e.Right), e.Bottom); ok {
				if i > 0 {
					tier = v.Tier
				}
				return
			}
		}
		return
	}
	done := false
	if fullStop {
		// Spec 047 §3.2: 。 is two units wide.  The layout moves a lone
		// two-unit character to the next row when one unit is left
		// instead of failing, and a full stop never starts a row, so
		// "does not fit" is a failed layout or an end row other than
		// the start row; the original text is then laid out as before.
		pl.fullStop++
		if ls, er, ec, ok, tr := attempt("。"); ok && er == row {
			pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier, done = ls, er, ec, true, tr, true
		} else {
			pl.fullStopDropped++
		}
	}
	if done {
		return
	}
	spaced := spaceNeeded && txt != "" && txt[0] != ' ' && !koGlue(prevRune, txt)
	switch {
	case spaced:
		// Spec 046 §3.4 (5): the space is never the reason a window turns
		// into English; without it the text is tried again.
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(" " + body)
		if !pl.fits {
			if pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body); pl.fits {
				pl.spaceDropped++
			}
		}
	case !isPlayer && !fresh && w.catalog.zhDeckSpace() && zhDeckDigitCall(p, txt, col, eclUnitLeft(e.Left)):
		// Spec 048: a number right after a deck prompt (甲板) gets a
		// space.  The result counts only when the first row drawn is
		// the start row and begins with the space; a space that pushes
		// the number to the next row is dropped, as is one that does
		// not fit.
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(" " + body)
		if !pl.fits || len(pl.lines) == 0 || pl.lines[0].Row != row || len(pl.lines[0].Text) == 0 || pl.lines[0].Text[0] != ' ' {
			if pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body); pl.fits {
				pl.spaceDropped++
			}
		}
	default:
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body)
	}
	return
}

// Outcomes of layoutPlayerName.
const (
	playerFull        = iota // 中文(英文)
	playerChineseOnly        // 中文
	playerEnglish            // the original English, through the passthrough
)

// playerLayout is the layout chosen for a player-name call.
type playerLayout struct {
	kind           int
	spaced         bool // the text starts with the space of spec 045 §3.4
	lines          []EclTextLine
	endRow, endCol uint8
	fits           bool
}

// layoutPlayerName picks the first tier of a player-name call that fits
// (spec 038 §3.3).  With space set (the call continues text and owes a
// space, spec 045 §3.4) the order is: full with space, Chinese only with
// space, full, Chinese only, English with space, English; without it the
// first two (and the English with space) are not tried, which is the order
// spec 038 had before.  The space is a token of its own: it is not inside a
// name unit, so the units shift by one.  When nothing fits, the result is
// the last try (the plain English text) with fits false.
func layoutPlayerName(prof *LayoutProfile, player []AnnotatedText, original []byte, space bool, row, col, left, right, bottom uint8) playerLayout {
	spaced := func(v AnnotatedText) AnnotatedText {
		out := AnnotatedText{Tier: v.Tier, Text: append([]rune{' '}, v.Text...)}
		for _, u := range v.Units {
			out.Units = append(out.Units, NameUnit{u.Start + 1, u.End + 1})
		}
		return out
	}
	try := func(kind int, hasSpace bool, text []rune, units []NameUnit) (playerLayout, bool) {
		ls, er, ec, ok := layoutEclTextP(prof, text, units, row, col, left, right, bottom)
		return playerLayout{kind: kind, spaced: hasSpace, lines: ls, endRow: er, endCol: ec, fits: ok}, ok
	}
	if space {
		for i, v := range player {
			sv := spaced(v)
			if c, ok := try(i, true, sv.Text, sv.Units); ok {
				return c
			}
		}
	}
	for i, v := range player {
		if c, ok := try(i, false, v.Text, v.Units); ok {
			return c
		}
	}
	if space {
		if c, ok := try(playerEnglish, true, append([]rune{' '}, []rune(string(original))...), nil); ok {
			return c
		}
	}
	c, _ := try(playerEnglish, false, []rune(string(original)), nil)
	return c
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

// SetLayout installs the spec 042 §3.4 layout profile (nil: default rules).
func (w *EclTextWatcher) SetLayout(p *LayoutProfile) {
	if w != nil {
		w.layout = p
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
// Columns are half units (spec 039 §3.4): left and right are inclusive unit
// columns, and the returned lines and end column are in units.
func layoutEclText(text []rune, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	return layoutEclTextUnits(text, nil, row, col, left, right, bottom)
}

// layoutEclTextUnits is layoutEclText where each unit (a spec-036 name
// annotation, sorted and disjoint) is one unbreakable token; a unit wider
// than the line breaks only at the spaces of its English part.
func layoutEclTextUnits(text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	return layoutEclTextP(nil, text, units, row, col, left, right, bottom)
}

// layoutEclTextP is layoutEclTextUnits with a per-language layout profile
// (spec 042 §3.4); a nil profile is the pre-042 behaviour.  A word-level
// profile (spec 043 §3.4) lays the call out by words first and, when that
// does not fit, the whole call again at character level, so a window that
// fits at character level never turns into the English original.
func layoutEclTextP(prof *LayoutProfile, text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	if prof != nil && prof.word {
		if lines, endRow, endCol, ok := layoutEclTextTokens(prof, true, text, units, row, col, left, right, bottom); ok {
			return lines, endRow, endCol, true
		}
		prof = prof.charLevel()
	}
	return layoutEclTextTokens(prof, false, text, units, row, col, left, right, bottom)
}

// layoutEclTextTokens is the layout itself; word chooses the word-level
// tokenization of spec 043 §3.4 over the character-level one.
func layoutEclTextTokens(prof *LayoutProfile, word bool, text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	width := int(right) - int(left) + 1
	var tokens [][]rune
	if word {
		tokens = eclWordTokens(prof, text, units, width)
	}
	// The character-level loop below is the pre-043 code, unchanged; it is
	// skipped when the tokens above were built by words.
	for i, u := 0, 0; !word && i < len(text); {
		for u < len(units) && units[u].End <= i {
			u++
		}
		if u < len(units) && units[u].Start == i {
			j := units[u].End
			for j < len(text) && prof.closing(text[j]) {
				j++
			}
			tok := text[i:j]
			if textUnits(tok) <= width {
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
		for j < len(text) && prof.closing(text[j]) {
			j++
		}
		// Never run into the next unit.
		if u < len(units) && j > units[u].Start {
			j = max(units[u].Start, i+1)
		}
		tokens = append(tokens, text[i:j])
		i = j
	}
	tokens = prof.joinOpening(tokens, width)
	var lines []EclTextLine
	cur := EclTextLine{Row: row, Col: col}
	used := int(col) - int(left)
	for _, t := range tokens {
		if used+textUnits(t) > width && used > 0 {
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
		if row > bottom || textUnits(t) > width {
			return nil, 0, 0, false
		}
		cur.Text = append(cur.Text, t...)
		used += textUnits(t)
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

// eclWordTokens is the spec 043 §3.4 tokenization: every space is a token of
// its own; a maximal run of other characters up to the next name unit is one
// word; a name unit together with the run glued to it (a particle) is one
// token when that fits a line.  A word wider than a line is cut into the
// character tokens of the pre-043 rules, so nothing wider than a line
// fails unless a single character is.
func eclWordTokens(prof *LayoutProfile, text []rune, units []NameUnit, width int) [][]rune {
	var tokens [][]rune
	for i, u := 0, 0; i < len(text); {
		for u < len(units) && units[u].End <= i {
			u++
		}
		if u < len(units) && units[u].Start == i {
			end := units[u].End
			stop := len(text)
			if u+1 < len(units) {
				stop = units[u+1].Start
			}
			j := end
			for j < stop && text[j] != ' ' {
				j++
			}
			if textUnits(text[i:j]) <= width {
				tokens = append(tokens, text[i:j])
				i = j
				continue
			}
			j = end
			for j < len(text) && prof.closing(text[j]) {
				j++
			}
			tok := text[i:j]
			if textUnits(tok) <= width {
				tokens = append(tokens, tok)
			} else {
				from := 0
				for k := 1; k < end-i; k++ {
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
		if text[i] == ' ' {
			tokens = append(tokens, text[i:i+1])
			i++
			continue
		}
		next := len(text)
		if u < len(units) {
			// A closing-punctuation run may end inside the next unit (the
			// pre-043 loop has the same quirk); always advance one rune.
			next = max(units[u].Start, i+1)
		}
		j := i
		for j < next && text[j] != ' ' {
			j++
		}
		if textUnits(text[i:j]) <= width {
			tokens = append(tokens, text[i:j])
		} else {
			for k := i; k < j; {
				m := k + 1
				if isEclLatin(text[k]) {
					for m < j && isEclLatin(text[m]) {
						m++
					}
				}
				for m < j && prof.closing(text[m]) {
					m++
				}
				tokens = append(tokens, text[k:m])
				k = m
			}
		}
		i = j
	}
	return tokens
}

// eclUnitLeft and eclUnitRight convert an inclusive window column range in
// 8×8 cells to inclusive half-unit columns (spec 039 §3.4).
func eclUnitLeft(c uint8) uint8  { return c * 2 }
func eclUnitRight(c uint8) uint8 { return c*2 + 1 }

func isEclLatin(r rune) bool {
	return r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '\'' || r == '-')
}

func isEclClosing(r rune) bool {
	return strings.ContainsRune("，。！？：；、」）……》』,.!?:;)", r)
}
