package buckrogers

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Spec 029 §2.3: dispatcher 0763:0424 strings from allow-listed callers are
// translated with the engine catalog and drawn in the original cells.
var engineDispatchReturnDelta uint16 = 4 + 0x0C // RETF 0Ch

// EngineDispatchLine is one dispatcher string drawn in the original cells.
// Width is in 8×8 cells; Text is padded to 2×Width half units (spec 039
// §3.1, §3.4).
type EngineDispatchLine struct {
	Row, Col, Width uint8
	BG, FG          uint8
	Text            []rune
}

type EngineDispatchWatcher struct {
	catalog *EngineTextCatalog
	allow   map[CodeKey]bool
	names   map[CodeKey]bool // callers where only exact monster names are drawn
	shared  map[CodeKey]bool // spec 029 §2.5: callers shared with older families
	ecl     *EclTextCatalog  // spec 029 §2.9; nil skips the step
	players *PlayerNames     // spec 038 §3.3 combat name column; nil keeps English
	pending *wrapPending     // spec 029 §2.10: first half of a wrapped fragment
	lines   []EngineDispatchLine
	inCall  bool
	callRet Address
	callSS  uint16
	callSP  uint16
	gen     uint64
	Stats   struct{ Hits, Misses, Invalidations int }
	// PartyStats counts spec 038 §3.4 lines wider than the original name
	// and lines stepped down to Chinese only (diagnostics).
	PartyStats struct{ Extended, ChineseOnly int }
	// headers is the spec 039 §3.4 欄名列 white list (nil: none);
	// HeaderStats counts anchored lines and lines whose original column
	// starts differ from the list (laid out by the general path).
	headers     *HeaderColumns
	HeaderStats struct{ Anchored, Mismatches int }
}

// SetHeaderColumns installs the header white list after its load check
// against the engine catalog.
func (w *EngineDispatchWatcher) SetHeaderColumns(h *HeaderColumns) error {
	if err := h.ValidateDispatcher(w.catalog); err != nil {
		return err
	}
	w.headers = h
	return nil
}

// headerText is the spec 039 §3.4 欄名列 layout of a general-path hit: a
// listed fragment whose original column starts (from the original bytes)
// equal the list is anchored; otherwise ok is false and the caller keeps
// the general layout.
func (w *EngineDispatchWatcher) headerText(original []byte, zh string, n int) ([]rune, bool) {
	cols, listed := w.headers.dispatcherColumns(w.catalog, string(original))
	if !listed {
		return nil, false
	}
	if !equalInts(tokenStarts(original), cols) {
		w.HeaderStats.Mismatches++
		return nil, false
	}
	out, err := anchorColumns([]rune(zh), cols, 2*n)
	if err != nil {
		w.HeaderStats.Mismatches++
		return nil, false
	}
	w.HeaderStats.Anchored++
	return out, true
}

// LoadEngineDispatchCallers parses text/engine-dispatch-callers.tsv and
// rejects any caller an existing family already owns.
func LoadEngineDispatchCallers(data []byte, owned map[CodeKey]bool) (map[CodeKey]bool, error) {
	rows, err := readTSV("engine-dispatch-callers.tsv", data, []string{"caller", "note"})
	if err != nil {
		return nil, err
	}
	out := map[CodeKey]bool{}
	for _, r := range rows {
		a, err := ParseCodeKey(r[0])
		if err != nil {
			return nil, err
		}
		if owned[a] {
			return nil, fmt.Errorf("buckrogers: 呼叫端 %s 已屬既有家族", r[0])
		}
		out[a] = true
	}
	return out, nil
}

func NewEngineDispatchWatcher(c *EngineTextCatalog, allow map[CodeKey]bool) *EngineDispatchWatcher {
	return &EngineDispatchWatcher{catalog: c, allow: allow, names: map[CodeKey]bool{}, gen: 1}
}

// SetNameCallers installs callers shared with other families: there only a
// string that is exactly a monster name is drawn (other families claim
// strings by exact hash, so a monster name is never theirs).
func (w *EngineDispatchWatcher) SetNameCallers(names map[CodeKey]bool) { w.names = names }

// SetPlayerNames installs the spec-038 resolver for the combat name column.
func (w *EngineDispatchWatcher) SetPlayerNames(p *PlayerNames) { w.players = p }

// NeedsParty reports whether an entry from caller uses the party snapshot
// (spec 038 §3.1: only name-related dispatcher entries take one).
func (w *EngineDispatchWatcher) NeedsParty(caller CodeKey) bool {
	if w == nil || w.allow[caller] {
		return false
	}
	return caller == partyNameCaller && w.names[caller] || partyPanelOnly(caller)
}

// SetEclCatalog gives the watcher the spec 027 catalog for §2.9.
func (w *EngineDispatchWatcher) SetEclCatalog(c *EclTextCatalog) { w.ecl = c }

// eclPrompt implements spec 029 §2.9: an ECL string printed through the
// dispatcher with trailing 0x20 spaces.
func (w *EngineDispatchWatcher) eclPrompt(original []byte) (string, bool) {
	if w.ecl == nil {
		return "", false
	}
	body := bytes.TrimRight(original, " ")
	if len(body) == len(original) || len(body) == 0 {
		return "", false
	}
	_, zh, ok := w.ecl.Lookup(body)
	if !ok {
		return "", false
	}
	zh += strings.Repeat(" ", len(original)-len(body))
	// Spec 039 §3.4: half units against the original length × 2.
	if stringUnits(zh) > 2*len(original) {
		return "", false
	}
	return zh, true
}

// SetSharedCallers installs spec 029 §2.5 callers: each must be owned by an
// older family and absent from the other lists. Overlap with the older
// family is settled at compose time (§2.6.1).
func (w *EngineDispatchWatcher) SetSharedCallers(shared, owned map[CodeKey]bool) error {
	for k := range shared {
		if !owned[k] {
			return fmt.Errorf("buckrogers: 共用呼叫端 %s 不屬既有家族", k)
		}
		if w.allow[k] || w.names[k] {
			return fmt.Errorf("buckrogers: 共用呼叫端 %s 與其他清單重疊", k)
		}
	}
	w.shared = shared
	return nil
}

func (w *EngineDispatchWatcher) Lines() []EngineDispatchLine { return w.lines }
func (w *EngineDispatchWatcher) Generation() uint64          { return w.gen }
func (w *EngineDispatchWatcher) InCall() bool                { return w != nil && w.inCall }

func (w *EngineDispatchWatcher) drop(keep func(EngineDispatchLine) bool) {
	n := 0
	for _, l := range w.lines {
		if keep(l) {
			w.lines[n] = l
			n++
		}
	}
	if n != len(w.lines) {
		w.lines = w.lines[:n]
		w.gen++
		w.Stats.Invalidations++
	}
}

func overlapsLine(l EngineDispatchLine, row, c0, c1 int) bool {
	return int(l.Row) == row && int(l.Col) < c1 && c0 < int(l.Col)+int(l.Width)
}

// wrapPending is S1 of spec 029 §2.10, waiting for the next row.
type wrapPending struct {
	s1       []byte
	row, col int
	bg, fg   uint8
	caller   CodeKey
}

// ObserveEntry handles a dispatcher entry (args as read by the menu family).
func (w *EngineDispatchWatcher) ObserveEntry(caller CodeKey, ss, sp uint16, ret Address, args [6]uint16, original []byte) {
	w.ObserveEntryParty(caller, ss, sp, ret, args, original, nil, nil)
}

// partyDecision is the spec-038 outcome for one dispatcher entry.
type partyDecision struct {
	handled bool   // a party member: never the monster-name lookup
	text    []rune // nil keeps the original (English)
	width   int    // cells the overlay covers (≥ the original length)
	chinese bool   // §3.4 stepped down to Chinese only
}

// partyName implements spec 038 §3.2–§3.4 for the name callers 235A, 21DE
// and 0388: the string pointer args[1]:args[0] must be one of the first N
// party records and the string must equal that record's name.
// handled=false means the pointer is not a member (monster record or
// elsewhere): 235A keeps the current monster-name lookup, 21DE/0388 draw
// nothing.  video reads A000 for the extension-cell check (nil: the check
// fails).
func (w *EngineDispatchWatcher) partyName(caller CodeKey, args [6]uint16, original []byte, party *PartySnapshot, video MemReader) partyDecision {
	if caller != partyNameCaller && !partyPanelOnly(caller) || party == nil {
		return partyDecision{}
	}
	m := party.MemberAt(args[1], args[0])
	if m == nil || m.Name != string(original) {
		return partyDecision{}
	}
	d := partyDecision{handled: true}
	row, col, n := int(uint8(args[4])), int(uint8(args[5])), len(original)
	if col == battleNameColumn && caller == partyNameCaller {
		// §3.3: Chinese only, inside the original cells (spec 039: units).
		if zh, ok := w.players.Chinese(m.Name, m.Gender); ok && stringUnits(zh) <= 2*n {
			d.text, d.width = []rune(zh), n
		}
		return d
	}
	scr, ok := partyPanelAt(row, col)
	if !ok {
		return d // other columns/rows (item title, training page): English
	}
	zh, ok := w.players.Chinese(m.Name, m.Gender)
	if !ok {
		return d
	}
	avail := scr.last - col + 1
	cn := []rune(zh)
	var cands [][]rune
	if scr.full {
		cands = append(cands, []rune(zh+"("+string(original)+")"))
	}
	cands = append(cands, cn)
	extOK := true
	for i, c := range cands {
		// Spec 039 §3.4: the candidate fits in ≤ avail×2 half units and
		// covers ⌈units/2⌉ cells.
		if textUnits(c) > 2*avail {
			continue
		}
		l := cellsForUnits(textUnits(c))
		if l > n {
			if !extOK {
				continue // §3.4: after a failed check only the original cells
			}
			if !extensionClear(video, row, col+n, col+l, uint8(args[2])) {
				extOK = false
				continue
			}
		}
		d.text, d.width = c, max(l, n)
		d.chinese = scr.full && i == len(cands)-1
		return d
	}
	return d
}

// Spec 038 §3.4: the two callers that only ever draw party members.  They
// are program constants, not rows of engine-dispatch-name-callers.tsv
// (that file means "not a member: look up a monster name").
var (
	partySelectedCaller = CodeKey{Unit: 0x2BA60, Offset: 0x21DE}
	partyCursorCaller   = CodeKey{Unit: 0x27BBE, Offset: 0x0388}
)

func partyPanelOnly(caller CodeKey) bool {
	return caller == partySelectedCaller || caller == partyCursorCaller
}

// partyScreen is one row of the spec 038 §3.4 table: last is the last
// background cell usable by the overlay; full selects 中文(英文).
type partyScreen struct {
	last int
	full bool
}

// partyPanelAt maps a start column and row to the §3.4 table.
func partyPanelAt(row, col int) (partyScreen, bool) {
	switch {
	case col == 17 && row >= 4 && row <= 9: // exploration party column
		return partyScreen{last: 33}, true
	case col == 1 && row >= 4 && row <= 9: // full-width party table, Pick Character
		return partyScreen{last: 33, full: true}, true
	case col == 8 && row == 1: // character sheet title
		return partyScreen{last: 27, full: true}, true
	}
	return partyScreen{}, false
}

// extensionClear reports whether cells [c0, c1) of text row row are all
// background bg in A000 (mode 13h, 320 bytes a line).
func extensionClear(video MemReader, row, c0, c1 int, bg uint8) bool {
	if video == nil || c1 > 40 || row > 24 {
		return false
	}
	for y := row * 8; y < row*8+8; y++ {
		for x := c0 * 8; x < c1*8; x++ {
			if video.Read8(0xA0000+uint32(y*320+x)) != bg {
				return false
			}
		}
	}
	return true
}

// ObserveEntryParty is ObserveEntry with the spec-038 party snapshot taken
// at this entry (nil: no snapshot) and A000 for the §3.4 extension check.
func (w *EngineDispatchWatcher) ObserveEntryParty(caller CodeKey, ss, sp uint16, ret Address, args [6]uint16, original []byte, party *PartySnapshot, video MemReader) {
	if w == nil {
		return
	}
	// §2.10 rule 3: any dispatcher entry ends the wait; only the matching
	// next row may use it.
	pend := w.pending
	w.pending = nil
	panelOnly := partyPanelOnly(caller) && !w.allow[caller] && !w.names[caller] && !w.shared[caller]
	if !w.allow[caller] && !w.names[caller] && !w.shared[caller] && !panelOnly {
		return
	}
	nameOnly := !w.allow[caller] && w.names[caller] || panelOnly
	row, col, n := int(uint8(args[4])), int(uint8(args[5])), len(original)
	if n == 0 || row > 24 || col+n > 40 {
		return
	}
	var d partyDecision
	if nameOnly {
		d = w.partyName(caller, args, original, party, video)
		if panelOnly && d.text == nil {
			// §3.4: 21DE/0388 draw only members in the table; the original
			// writes of this call invalidate any overlay there as before.
			return
		}
	}
	width := max(n, d.width)
	// The original repaints these cells: any older line there is stale.
	w.drop(func(l EngineDispatchLine) bool { return !overlapsLine(l, row, col, col+width) })
	w.inCall, w.callRet, w.callSS, w.callSP = true, ret, ss, sp
	var zh string
	var ok bool
	switch {
	case d.handled:
		if d.text != nil {
			w.lines = append(w.lines, EngineDispatchLine{Row: uint8(row), Col: uint8(col), Width: uint8(width),
				BG: uint8(args[2]), FG: uint8(args[3]), Text: padUnits(d.text, 2*width)})
			w.gen++
			w.Stats.Hits++
			if d.width > n {
				w.PartyStats.Extended++
			}
			if d.chinese {
				w.PartyStats.ChineseOnly++
			}
		} else {
			w.Stats.Misses++
		}
		return
	case nameOnly:
		zh, ok = w.catalog.monsterSlot(string(original))
	default:
		zh, ok = w.catalog.Translate(string(original))
	}
	if !ok && !nameOnly {
		zh, ok = w.eclPrompt(original)
	}
	if !ok && !nameOnly && pend != nil && pend.caller == caller && row == pend.row+1 {
		if w.joinWrapped(pend, row, col, args, original) {
			w.gen++
			w.Stats.Hits++
			return
		}
	}
	if !ok && !nameOnly {
		w.pending = &wrapPending{s1: append([]byte(nil), original...), row: row, col: col,
			bg: uint8(args[2]), fg: uint8(args[3]), caller: caller}
	}
	// Spec 039 §3.4: the Chinese takes at most the original length × 2 half
	// units; Width = max(n, ⌈units/2⌉) = n on this path.
	if !ok || stringUnits(zh) > 2*n {
		w.Stats.Misses++
		return
	}
	text := padUnits([]rune(zh), 2*n)
	if a, ok := w.headerText(original, zh, n); ok {
		text = a
	}
	w.lines = append(w.lines, EngineDispatchLine{Row: uint8(row), Col: uint8(col), Width: uint8(n),
		BG: uint8(args[2]), FG: uint8(args[3]), Text: text})
	w.gen++
	w.Stats.Hits++
}

// joinWrapped implements spec 029 §2.10 rule 2: S1+" "+S2 or S1+S2 is one
// fragment; its Chinese is split across the two rows.
func (w *EngineDispatchWatcher) joinWrapped(p *wrapPending, row, col int, args [6]uint16, s2 []byte) bool {
	var zh string
	var ok bool
	for _, joined := range []string{string(p.s1) + " " + string(s2), string(p.s1) + string(s2)} {
		if zh, ok = w.catalog.wholeFragment(joined); ok {
			break
		}
	}
	if !ok {
		return false
	}
	r := []rune(zh)
	n1, n2 := len(p.s1), len(s2)
	// 前段盡量放滿（spec 029 §2.10 修訂）：spec 039 以 2×len(S1) 半形單位計，
	// 放不下的全形字整字移到後段。
	k := fitUnits(r, 2*n1)
	if textUnits(r[k:]) > 2*n2 {
		return false
	}
	w.drop(func(l EngineDispatchLine) bool { return !overlapsLine(l, p.row, p.col, p.col+n1) })
	w.lines = append(w.lines,
		EngineDispatchLine{Row: uint8(p.row), Col: uint8(p.col), Width: uint8(n1), BG: p.bg, FG: p.fg, Text: padUnits(r[:k], 2*n1)},
		EngineDispatchLine{Row: uint8(row), Col: uint8(col), Width: uint8(n2), BG: uint8(args[2]), FG: uint8(args[3]), Text: padUnits(r[k:], 2*n2)})
	return true
}

func (w *EngineDispatchWatcher) ObserveInstruction(at Address, ss, sp uint16) {
	if w != nil && w.inCall && at == w.callRet && ss == w.callSS && sp == w.callSP+engineDispatchReturnDelta {
		w.inCall = false
	}
}

func (w *EngineDispatchWatcher) ObserveVideoWrite(offset uint32) {
	if w == nil || w.inCall || len(w.lines) == 0 && w.pending == nil || offset >= 320*200 {
		return
	}
	row, col := int(offset/320/8), int(offset%320/8)
	if p := w.pending; p != nil && row == p.row && col >= p.col && col < p.col+len(p.s1) {
		w.pending = nil // §2.10 rule 3
	}
	w.drop(func(l EngineDispatchLine) bool { return !overlapsLine(l, row, col, col+1) })
}

func (w *EngineDispatchWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.inCall = false
		w.pending = nil
		w.drop(func(EngineDispatchLine) bool { return false })
	}
}

// Page converts the lines to the per-cell form the menu presenter draws.
func (w *EngineDispatchWatcher) Page() *HMenuPage {
	if w == nil || len(w.lines) == 0 {
		return nil
	}
	p := &HMenuPage{}
	for _, l := range w.lines {
		var cells []HMenuCell
		for _, r := range l.Text {
			cells = append(cells, HMenuCell{Rune: r, BG: l.BG, FG: l.FG})
		}
		p.Rows = append(p.Rows, newHMenuRow(l.Row, l.Col, cells))
	}
	return p
}

var engineCallerRE = regexp.MustCompile(`\b([0-9A-F]{4}):([0-9A-F]{4})\b`)

// OwnedCallers collects every segment:offset that appears in an existing
// family's events file, so the allow list cannot claim them. Overlay
// segments are read as the units loaded there in character creation.
func OwnedCallers(files map[string][]byte) map[CodeKey]bool {
	out := map[CodeKey]bool{}
	for name, b := range files {
		if strings.HasPrefix(name, "engine-") || strings.HasPrefix(name, "ecl-") || strings.HasPrefix(name, "hmenu") || strings.HasPrefix(name, "item-") {
			continue
		}
		for _, m := range engineCallerRE.FindAllStringSubmatch(string(b), -1) {
			var a Address
			fmt.Sscanf(m[1]+":"+m[2], "%04X:%04X", &a.Segment, &a.Offset)
			out[LegacyCodeKey(a)] = true
		}
	}
	return out
}
