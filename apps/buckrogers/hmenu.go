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
	hmenuPrinter            = Address{Segment: 0x37F1, Offset: 0x0243}
	hmenuReturnDelta uint16 = 4 + 4 // RETF 4
)

type HMenuCatalog struct {
	byID map[eclTextID]string
	text map[string]string
}

func LoadHMenuCatalog(events, translations []byte) (*HMenuCatalog, error) {
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
	rows, err = readTSV("hmenu.zh-TW.tsv", translations, []string{"key", "translation", "source"})
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
		return "", false
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

type HMenuPage struct {
	Row, Col uint8
	Cells    []HMenuCell // from Col to column 39
}

type HMenuStats struct{ Hits, Misses, Overflows, Invalidations, Reentries int }

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
	w.inCall = true
	w.call = e
	w.Stats.Hits++
}

func (w *HMenuWatcher) build(e HMenuEntry) (*HMenuPage, string) {
	n := len(e.Text)
	if n == 0 || len(e.Items) == 0 || e.Row > 24 || e.Col > 39 || strings.IndexByte(string(e.Text), '@') >= 0 {
		return nil, "shape"
	}
	var cells []HMenuCell
	for i, r := range e.Items {
		if r[0] < 1 || r[1] < r[0] || int(r[1]) > n {
			return nil, "shape"
		}
		item := strings.TrimSpace(string(e.Text[r[0]-1 : r[1]]))
		t, ok := w.catalog.lookup(item)
		if !ok {
			return nil, "miss"
		}
		if i > 0 {
			cells = append(cells, HMenuCell{' ', 0, e.Normal})
		}
		sel := uint8(i+1) == e.Selected && e.Hot != 0
		for _, ch := range t {
			switch {
			case sel:
				cells = append(cells, HMenuCell{ch, e.Hot, 0})
			case ch < 0x80 && (ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'):
				cells = append(cells, HMenuCell{ch, 0, e.Hot})
			default:
				cells = append(cells, HMenuCell{ch, 0, e.Normal})
			}
		}
	}
	width := 40 - int(e.Col)
	if len(cells) > width {
		return nil, "overflow"
	}
	for len(cells) < width {
		cells = append(cells, HMenuCell{' ', 0, e.Normal})
	}
	return &HMenuPage{Row: e.Row, Col: e.Col, Cells: cells}, ""
}

func (w *HMenuWatcher) ObserveInstruction(at Address, ss, sp uint16) {
	if w != nil && w.inCall && at == w.call.Return && ss == w.call.SS && sp == w.call.SP+hmenuReturnDelta {
		w.inCall = false
	}
}

func (w *HMenuWatcher) ObserveVideoWrite(offset uint32) {
	if w == nil || w.page == nil || w.inCall || offset >= 320*200 {
		return
	}
	x, y := offset%320, offset/320
	if y/8 == uint32(w.page.Row) && x >= uint32(w.page.Col)*8 {
		w.invalidate()
	}
}

func (w *HMenuWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.invalidate()
	}
}
