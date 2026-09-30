package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Spec 028 (Buck repo): 37F1:0243 draws a horizontal menu line from its
// parent's frame.  Menus are composed at run time, so each item is looked
// up on its own and the line is rebuilt in Chinese only when every item
// has a translation.
var (
	hmenuPrinter            = CodeKey{Unit: 0x2BA60, Offset: 0x0243} // spec 032
	hmenuReturnDelta uint16 = 4 + 4                                  // RETF 4
)

type HMenuCatalog struct {
	byID map[eclTextID]string
	text map[string]string
}

func LoadHMenuCatalog(events, translations []byte) (*HMenuCatalog, error) {
	return LoadHMenuCatalogLang(events, translations, LangZhTW)
}

// LoadHMenuCatalogLang reads text/hmenu.<lang>.tsv (spec 040 §3.1).
func LoadHMenuCatalogLang(events, translations []byte, lang string) (*HMenuCatalog, error) {
	c := &HMenuCatalog{byID: map[eclTextID]string{}, text: map[string]string{}}
	rows, err := readTSV("hmenu-item-events.tsv", events, []string{"event_key", "original_length", "original_sha256"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		n, err := strconv.Atoi(r[1])
		b, herr := hex.DecodeString(r[2])
		if err != nil || n <= 0 || n > 255 || herr != nil || len(b) != 32 {
			return nil, fmt.Errorf("buckrogers: hmenu 事件 %s 無效", r[0])
		}
		var id eclTextID
		id.n = uint8(n)
		copy(id.sum[:], b)
		if _, dup := c.byID[id]; dup {
			return nil, fmt.Errorf("buckrogers: hmenu 雜湊重複 %s", r[0])
		}
		c.byID[id] = r[0]
	}
	keys := map[string]bool{}
	for _, k := range c.byID {
		keys[k] = true
	}
	rows, err = readLangTSV(LangFile("hmenu", lang), translations, []string{"key", "translation", "source"}, lang)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if !keys[r[0]] || r[1] == "" || c.text[r[0]] != "" {
			return nil, fmt.Errorf("buckrogers: hmenu 譯文 %s 無效", r[0])
		}
		c.text[r[0]] = r[1]
	}
	return c, nil
}

func (c *HMenuCatalog) lookup(item string) (string, bool) {
	if c == nil || item == "" || len(item) > 255 {
		return "", false
	}
	k, ok := c.byID[eclTextID{uint8(len(item)), sha256.Sum256([]byte(item))}]
	if !ok {
		// ECL menus keep items in capitals and the engine prints them with
		// only the first letter capitalised ("ORDER FOOD" -> "Order food").
		up := strings.ToUpper(item)
		if up == item {
			return "", false
		}
		if k, ok = c.byID[eclTextID{uint8(len(up)), sha256.Sum256([]byte(up))}]; !ok {
			return "", false
		}
	}
	t, ok := c.text[k]
	return t, ok
}

// HMenuEntry is the parent-frame data read at 37F1:0243.
type HMenuEntry struct {
	SS, SP      uint16
	Return      Address
	Text        []byte
	Row, Col    uint8
	Items       [][2]uint8 // 1-based inclusive character ranges
	Selected    uint8
	Normal, Hot uint8 // DS:6B46, DS:6B47 palette indices
}

// HMenuCell is one drawn cell; colors are palette indices.
type HMenuCell struct {
	Rune   rune
	BG, FG uint8
}

type HMenuRow struct {
	Row, Col uint8
	Cells    []HMenuCell // from Col to column 39
	// Pos is each cell's offset from Col in half units (spec 039 §3.3); a
	// nil Pos means every cell is one full cell (older callers).
	Pos []int
}

// UnitPos is the half-unit offset of cell i from Col.
func (r HMenuRow) UnitPos(i int) int {
	if r.Pos != nil {
		return r.Pos[i]
	}
	return 2 * i
}

// Units is the row width in half units.
func (r HMenuRow) Units() int {
	if len(r.Cells) == 0 {
		return 0
	}
	if r.Pos == nil {
		return 2 * len(r.Cells)
	}
	last := len(r.Cells) - 1
	return r.Pos[last] + runeUnits(r.Cells[last].Rune)
}

// WidthCells is the number of 8×8 cells the row covers.
func (r HMenuRow) WidthCells() int { return cellsForUnits(r.Units()) }

// newHMenuRow fills Pos from the cells' widths.
func newHMenuRow(row, col uint8, cells []HMenuCell) HMenuRow {
	pos := make([]int, len(cells))
	u := 0
	for i, c := range cells {
		pos[i] = u
		u += runeUnits(c.Rune)
	}
	return HMenuRow{Row: row, Col: col, Cells: cells, Pos: pos}
}

// padHMenuCells pads cells to target half units with spec 039 §3.1
// padding in the given colors.
func padHMenuCells(cells []HMenuCell, target int, bg, fg uint8) []HMenuCell {
	u := 0
	for _, c := range cells {
		u += runeUnits(c.Rune)
	}
	for _, r := range appendPadding(nil, u, target) {
		cells = append(cells, HMenuCell{r, bg, fg})
	}
	return cells
}

// HMenuPage holds one row per original row the menu used ('@' breaks).
type HMenuPage struct {
	Rows []HMenuRow
}

type HMenuStats struct {
	Hits, Misses, Overflows, Invalidations, Reentries int
	LastInvalidOffset                                 uint32
	Returns                                           int
	LastEntrySP, LastReturnSP                         uint16
}

type HMenuWatcher struct {
	catalog *HMenuCatalog
	page    *HMenuPage
	inCall  bool
	call    HMenuEntry
	gen     uint64
	Stats   HMenuStats
}

func NewHMenuWatcher(c *HMenuCatalog) *HMenuWatcher { return &HMenuWatcher{catalog: c, gen: 1} }

func (w *HMenuWatcher) Page() *HMenuPage   { return w.page }
func (w *HMenuWatcher) Generation() uint64 { return w.gen }
func (w *HMenuWatcher) InCall() bool       { return w != nil && w.inCall }

func (w *HMenuWatcher) invalidate() {
	if w.page != nil {
		w.page = nil
		w.gen++
		w.Stats.Invalidations++
	}
	w.inCall = false
}

func (w *HMenuWatcher) ObserveEntry(e HMenuEntry) {
	if w == nil {
		return
	}
	if w.inCall {
		w.Stats.Reentries++
		w.invalidate()
		return
	}
	page, why := w.build(e)
	if page == nil {
		if why == "overflow" {
			w.Stats.Overflows++
		} else {
			w.Stats.Misses++
		}
		w.invalidate()
		w.inCall = false
		return
	}
	w.page = page
	w.gen++
	w.Stats.LastEntrySP = e.SP
	w.inCall = true
	w.call = e
	w.Stats.Hits++
}

func (w *HMenuWatcher) build(e HMenuEntry) (*HMenuPage, string) {
	n := len(e.Text)
	if n == 0 || len(e.Items) == 0 || e.Col > 39 {
		return nil, "shape"
	}
	breaks := strings.Count(string(e.Text), "@")
	if int(e.Row)+breaks > 24 {
		return nil, "shape"
	}
	rows := make([][]HMenuCell, breaks+1)
	// Original column of character index c (1-based): the row restarts at
	// the start column after each '@'.
	startCol := make([]int, breaks+1)
	for i := range startCol {
		startCol[i] = -1
	}
	colOf := func(c int) int {
		last := strings.LastIndexByte(string(e.Text[:c-1]), '@')
		return int(e.Col) + (c - 1) - (last + 1)
	}
	for i, r := range e.Items {
		if r[0] < 1 || r[1] < r[0] || int(r[1]) > n {
			return nil, "shape"
		}
		raw := string(e.Text[r[0]-1 : r[1]])
		if strings.Contains(raw, "@") {
			return nil, "shape"
		}
		t, ok := w.catalog.lookup(strings.TrimSpace(raw))
		if !ok {
			return nil, "miss"
		}
		line := strings.Count(string(e.Text[:r[0]-1]), "@")
		if startCol[line] < 0 {
			startCol[line] = colOf(int(r[0]))
		}
		if len(rows[line]) > 0 {
			rows[line] = append(rows[line], HMenuCell{' ', 0, e.Normal})
		}
		sel := uint8(i+1) == e.Selected && e.Hot != 0
		for _, ch := range t {
			switch {
			case sel:
				rows[line] = append(rows[line], HMenuCell{ch, e.Hot, 0})
			case ch < 0x80 && (ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'):
				rows[line] = append(rows[line], HMenuCell{ch, 0, e.Hot})
			default:
				rows[line] = append(rows[line], HMenuCell{ch, 0, e.Normal})
			}
		}
	}
	page := &HMenuPage{}
	for i, cells := range rows {
		if startCol[i] < 0 {
			continue
		}
		// Spec 039 §3.4: the row holds 2×(40−start column) half units; the
		// item separator is one unit.
		width := 2 * (40 - startCol[i])
		u := 0
		for _, c := range cells {
			u += runeUnits(c.Rune)
		}
		if width <= 0 || u > width {
			return nil, "overflow"
		}
		cells = padHMenuCells(cells, width, 0, e.Normal)
		page.Rows = append(page.Rows, newHMenuRow(e.Row+uint8(i), uint8(startCol[i]), cells))
	}
	return page, ""
}

func (w *HMenuWatcher) ObserveInstruction(at Address, ss, sp uint16) {
	if w != nil && w.inCall && at == w.call.Return && ss == w.call.SS && sp == w.call.SP+hmenuReturnDelta {
		w.inCall = false
		w.Stats.Returns++
		w.Stats.LastReturnSP = sp
	}
}

func (w *HMenuWatcher) ObserveVideoWrite(offset uint32) {
	if w == nil || w.page == nil || w.inCall || offset >= 320*200 {
		return
	}
	x, y := offset%320, offset/320
	for _, r := range w.page.Rows {
		if y/8 == uint32(r.Row) && x >= uint32(r.Col)*8 {
			w.Stats.LastInvalidOffset = offset
			w.invalidate()
			return
		}
	}
}

func (w *HMenuWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.invalidate()
	}
}
