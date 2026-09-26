package buckrogers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 030 (Buck repo): when the original says an event is recorded as
// logbook entry N, show the Chinese entry in an overlay panel instead of
// sending the player to the manual.

const (
	logbookCols      = 36
	logbookBodyRows  = 19
	logbookMaxPages  = 3
	logbookTop       = 2 // text row of the title (y = 16)
	logbookLeft      = 1 // text column (x = 8)
	logbookFirstBody = logbookTop + 1
	logbookPageRow   = logbookTop + 1 + logbookBodyRows // row 22
)

type LogbookEntry struct {
	Title string
	Pages [][]string // pages of body lines
}

type LogbookCatalog struct {
	entries map[int]LogbookEntry
}

// LayoutLogbook splits a body (paragraphs separated by the two characters
// `\n`) into pages of 19 lines × 36 cells with the spec-027 wrap rules.
func LayoutLogbook(body string) ([][]string, error) {
	var lines []string
	for _, para := range strings.Split(body, `\n`) {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		ls, _, _, ok := layoutEclText([]rune(para), 0, 0, 0, logbookCols-1, 255)
		if !ok {
			return nil, fmt.Errorf("buckrogers: 手札段落無法排版")
		}
		for _, l := range ls {
			lines = append(lines, string(l.Text))
		}
	}
	var pages [][]string
	for len(lines) > 0 {
		n := logbookBodyRows
		if n > len(lines) {
			n = len(lines)
		}
		pages = append(pages, lines[:n])
		lines = lines[n:]
	}
	if len(pages) == 0 || len(pages) > logbookMaxPages {
		return nil, fmt.Errorf("buckrogers: 手札頁數 %d 超出範圍", len(pages))
	}
	return pages, nil
}

func LoadLogbookCatalog(data []byte) (*LogbookCatalog, error) {
	rows, err := readTSV("logbook.zh-TW.tsv", data, []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	bodies, titles := map[int]string{}, map[int]string{}
	for _, r := range rows {
		k := strings.TrimPrefix(r[0], "logbook.")
		title := strings.HasSuffix(k, ".title")
		k = strings.TrimSuffix(k, ".title")
		n, err := strconv.Atoi(k)
		if err != nil || n < 1 || n > 71 || r[1] == "" {
			return nil, fmt.Errorf("buckrogers: 手札列 %s 無效", r[0])
		}
		if title {
			titles[n] = r[1]
		} else {
			bodies[n] = r[1]
		}
	}
	c := &LogbookCatalog{entries: map[int]LogbookEntry{}}
	for n, b := range bodies {
		pages, err := LayoutLogbook(b)
		if err != nil {
			return nil, fmt.Errorf("第 %d 則：%w", n, err)
		}
		c.entries[n] = LogbookEntry{Title: titles[n], Pages: pages}
	}
	return c, nil
}

var logbookTail = regexp.MustCompile(`( as logbook entry ) *([0-9]{1,2})\.$`)

type LogbookWatcher struct {
	catalog *LogbookCatalog
	engine  *EngineTextCatalog
	open    int // entry number, 0 = closed
	page    int
	window  [4]uint8 // left, top, right, bottom of the text window
	tlBy    map[[2]uint16]bool
	gen     uint64
	Stats   struct{ Opens, Closes int }
}

func NewLogbookWatcher(c *LogbookCatalog, e *EngineTextCatalog) *LogbookWatcher {
	return &LogbookWatcher{catalog: c, engine: e, gen: 1}
}

func (w *LogbookWatcher) Generation() uint64 { return w.gen }

// Open returns the entry and page shown, or 0.
func (w *LogbookWatcher) Open() (int, int) { return w.open, w.page }

func (w *LogbookWatcher) close() {
	if w.open != 0 {
		w.open, w.page = 0, 0
		w.gen++
		w.Stats.Closes++
	}
	w.tlBy = nil
}

// ObserveEntry runs at every 0763:056C entry (spec 030 §3.2, §3.3-1).
func (w *LogbookWatcher) ObserveEntry(original []byte, left, top, right, bottom uint8) {
	if w == nil {
		return
	}
	w.close()
	m := logbookTail.FindSubmatch(original)
	if m == nil {
		return
	}
	if w.engine != nil {
		if _, ok := w.engine.frag[engineIDOf(string(m[1]))]; !ok {
			return
		}
	}
	n, _ := strconv.Atoi(string(m[2]))
	if _, ok := w.catalog.entries[n]; !ok {
		return
	}
	w.open, w.page = n, 0
	w.window = [4]uint8{left, top, right, bottom}
	w.gen++
	w.Stats.Opens++
}

// ObserveVideoWrite closes the panel when one writer clears the whole text
// window (writes both its top-left and bottom-right cells).
func (w *LogbookWatcher) ObserveVideoWrite(cs, ip uint16, offset uint32) {
	if w == nil || w.open == 0 || offset >= 320*200 {
		return
	}
	col, row := uint8(offset%320/8), uint8(offset/320/8)
	who := [2]uint16{cs, ip}
	if col == w.window[0] && row == w.window[1] {
		if w.tlBy == nil {
			w.tlBy = map[[2]uint16]bool{}
		}
		w.tlBy[who] = true
	}
	if col == w.window[2] && row == w.window[3] && w.tlBy[who] {
		w.close()
	}
}

func (w *LogbookWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.close()
	}
}

// Turn flips pages from the host; it reports whether the panel took the key.
func (w *LogbookWatcher) Turn(delta int) bool {
	if w == nil || w.open == 0 {
		return false
	}
	pages := len(w.catalog.entries[w.open].Pages)
	p := w.page + delta
	if p < 0 || p >= pages {
		return true
	}
	w.page = p
	w.gen++
	return true
}

// LogbookOverlay draws the panel at one scale.
type LogbookOverlay struct {
	layer *xlate.Layer
	font  *xlate.Font
	scale int
	gen   uint64
}

func NewLogbookOverlay(font *xlate.Font, scale int) (*LogbookOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: 手札 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		font.Name = base + ".logbook.3x22"
	}
	return &LogbookOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, scale: scale}, nil
}

func (o *LogbookOverlay) Sync(w *LogbookWatcher, palette [256][3]uint8) []rune {
	if o == nil || w == nil || w.gen == o.gen {
		return nil
	}
	o.gen = w.gen
	o.layer.Stamps = nil
	n, page := w.Open()
	if n == 0 {
		return nil
	}
	e := w.catalog.entries[n]
	rows := map[int]string{logbookTop: fmt.Sprintf("手札第 %d 則：%s", n, e.Title)}
	for i, l := range e.Pages[page] {
		rows[logbookFirstBody+i] = l
	}
	if len(e.Pages) > 1 {
		rows[logbookPageRow] = fmt.Sprintf("第 %d／%d 頁　PgDn／PgUp 翻頁", page+1, len(e.Pages))
		rows[logbookPageRow] = strings.ReplaceAll(rows[logbookPageRow], "　", " ")
	}
	var miss []rune
	off := manualGlyphOffset(o.scale)
	for r := logbookTop; r <= logbookPageRow; r++ {
		text := []rune(rows[r])
		for _, ch := range text {
			if _, ok := o.font.Glyphs[ch]; !ok && ch != ' ' {
				miss = append(miss, ch)
			}
		}
		for len(text) < logbookCols+2 {
			text = append(text, ' ')
		}
		fg := palette[10]
		if r == logbookTop {
			fg = palette[15]
		}
		o.layer.Stamps = append(o.layer.Stamps, &xlate.Stamp{
			Key: fmt.Sprintf("logbook.%d", r), X: logbookLeft * 8, Y: r * 8, Cells: logbookCols + 2, CellW: 8, CellH: 8,
			Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1, Text: text[:logbookCols+2], State: xlate.Shown,
			BG: palette[0], FG: fg,
		})
	}
	if len(miss) != 0 {
		o.layer.Stamps = nil
	}
	return miss
}

func (o *LogbookOverlay) Active() bool { return o != nil && len(o.layer.Stamps) != 0 }

func (o *LogbookOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
