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
	pending *wrapPending     // spec 029 §2.10: first half of a wrapped fragment
	lines   []EngineDispatchLine
	inCall  bool
	callRet Address
	callSS  uint16
	callSP  uint16
	gen     uint64
	Stats   struct{ Hits, Misses, Invalidations int }
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
	if len([]rune(zh)) > len(original) {
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
	if w == nil {
		return
	}
	// §2.10 rule 3: any dispatcher entry ends the wait; only the matching
	// next row may use it.
	pend := w.pending
	w.pending = nil
	if !w.allow[caller] && !w.names[caller] && !w.shared[caller] {
		return
	}
	nameOnly := !w.allow[caller] && w.names[caller]
	row, col, n := int(uint8(args[4])), int(uint8(args[5])), len(original)
	if n == 0 || row > 24 || col+n > 40 {
		return
	}
	// The original repaints these cells: any older line there is stale.
	w.drop(func(l EngineDispatchLine) bool { return !overlapsLine(l, row, col, col+n) })
	w.inCall, w.callRet, w.callSS, w.callSP = true, ret, ss, sp
	var zh string
	var ok bool
	if nameOnly {
		zh, ok = w.catalog.monsterSlot(string(original))
	} else {
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
	if !ok || len([]rune(zh)) > n {
		w.Stats.Misses++
		return
	}
	text := []rune(zh)
	for len(text) < n {
		text = append(text, ' ')
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
	k := min(len(r), n1) // 前段盡量放滿（spec 029 §2.10 修訂）
	if k > n1 || len(r)-k > n2 {
		return false
	}
	pad := func(t []rune, n int) []rune {
		for len(t) < n {
			t = append(t, ' ')
		}
		return t
	}
	w.drop(func(l EngineDispatchLine) bool { return !overlapsLine(l, p.row, p.col, p.col+n1) })
	w.lines = append(w.lines,
		EngineDispatchLine{Row: uint8(p.row), Col: uint8(p.col), Width: uint8(n1), BG: p.bg, FG: p.fg, Text: pad(append([]rune(nil), r[:k]...), n1)},
		EngineDispatchLine{Row: uint8(row), Col: uint8(col), Width: uint8(n2), BG: uint8(args[2]), FG: uint8(args[3]), Text: pad(append([]rune(nil), r[k:]...), n2)})
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
		row := HMenuRow{Row: l.Row, Col: l.Col}
		for _, r := range l.Text {
			row.Cells = append(row.Cells, HMenuCell{Rune: r, BG: l.BG, FG: l.FG})
		}
		p.Rows = append(p.Rows, row)
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
