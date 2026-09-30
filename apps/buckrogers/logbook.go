package buckrogers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 030 (Buck repo): when the original says an event is recorded as
// logbook entry N, show the Chinese entry in an overlay panel instead of
// sending the player to the manual.

const (
	logbookCols      = 36 // body cells; spec 039: 72 half units a line
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
	// Spec 036 §3.3: the annotation tier chosen for the body and the title.
	BodyTier, TitleTier NameTier
	titles              []AnnotatedText // title tiers, re-chosen when the template changes
}

// logbookTitleCells is the width of the title row (spec 036 §3.3);
// logbookTitleUnits and logbookBodyUnits are the spec 039 limits in half
// units.
const (
	logbookTitleCells = logbookCols + 2
	logbookTitleUnits = 2 * logbookTitleCells
	logbookBodyUnits  = 2 * logbookCols
)

type LogbookCatalog struct {
	entries map[int]LogbookEntry
	// Panel title and page-row templates; {0} and {1} are filled in.
	titleFmt, pageFmt string
}

// LoadLogbookPanelText reads text/logbook-panel.zh-TW.tsv (keys
// logbook.panel.title with {0}=entry number, {1}=title; logbook.panel.page
// with {0}=page, {1}=pages).
func (c *LogbookCatalog) LoadLogbookPanelText(data []byte) error {
	return c.LoadLogbookPanelTextLang(data, LangZhTW)
}

// LoadLogbookPanelTextLang reads text/logbook-panel.<lang>.tsv (spec 040).
func (c *LogbookCatalog) LoadLogbookPanelTextLang(data []byte, lang string) error {
	rows, err := readLangTSV(LangFile("logbook-panel", lang), data, []string{"key", "translation", "source"}, lang)
	if err != nil {
		return err
	}
	for _, r := range rows {
		ok := strings.Contains(r[1], "{0}") && strings.Contains(r[1], "{1}")
		switch {
		case r[0] == "logbook.panel.title" && ok:
			c.titleFmt = r[1]
		case r[0] == "logbook.panel.page" && ok:
			c.pageFmt = r[1]
		default:
			return fmt.Errorf("buckrogers: 手札面板列 %s 無效", r[0])
		}
	}
	return c.chooseTitles()
}

// chooseTitles picks, per entry, the first title tier whose whole row fits
// in 76 half units (38 cells, spec 039 §3.4); an entry whose unannotated
// title is still too wide fails.
func (c *LogbookCatalog) chooseTitles() error {
	for n, e := range c.entries {
		ok := false
		for _, v := range e.titles {
			if stringUnits(fillLogbook(c.titleFmt, strconv.Itoa(n), string(v.Text))) <= logbookTitleUnits {
				e.Title, e.TitleTier, ok = string(v.Text), v.Tier, true
				break
			}
		}
		if !ok {
			return fmt.Errorf("buckrogers: 手札第 %d 則標題超過 %d 單位", n, logbookTitleUnits)
		}
		c.entries[n] = e
	}
	return nil
}

func fillLogbook(f string, a, b string) string {
	return strings.NewReplacer("{0}", a, "{1}", b).Replace(f)
}

// LayoutLogbook splits a body (paragraphs separated by the two characters
// `\n`) into pages of 19 lines × 72 half units (spec 039 §3.4) with the
// spec-027 wrap rules.
func LayoutLogbook(body string) ([][]string, error) {
	return layoutLogbookUnits([]rune(body), nil)
}

// layoutLogbookUnits is LayoutLogbook for an annotated body: units never
// cross a paragraph break, and each stays one token (spec 036 §3.3).
func layoutLogbookUnits(body []rune, units []NameUnit) ([][]string, error) {
	var lines []string
	for start := 0; start <= len(body); {
		end := start
		for end < len(body) && !(body[end] == '\\' && end+1 < len(body) && body[end+1] == 'n') {
			end++
		}
		a, b := start, end
		for a < b && isLogbookSpace(body[a]) {
			a++
		}
		for b > a && isLogbookSpace(body[b-1]) {
			b--
		}
		start = end + 2
		if a == b {
			continue
		}
		var us []NameUnit
		for _, u := range units {
			if u.Start >= a && u.End <= b {
				us = append(us, NameUnit{u.Start - a, u.End - a})
			}
		}
		ls, _, _, ok := layoutEclTextUnits(body[a:b], us, 0, 0, 0, logbookBodyUnits-1, 255)
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

func isLogbookSpace(r rune) bool { return unicode.IsSpace(r) }

// LoadLogbookCatalog reads text/logbook.zh-TW.tsv.  With a glossary it lays
// out the three annotation tiers of spec 036 §3.3 once and keeps the first
// that fits in three pages; nothing is re-laid out at run time.
func LoadLogbookCatalog(data []byte, names *NameGlossary) (*LogbookCatalog, error) {
	return LoadLogbookCatalogLang(data, names, LangZhTW)
}

// LoadLogbookCatalogLang reads text/logbook.<lang>.tsv (spec 040 §3.1).
func LoadLogbookCatalogLang(data []byte, names *NameGlossary, lang string) (*LogbookCatalog, error) {
	rows, err := readLangTSV(LangFile("logbook", lang), data, []string{"key", "translation", "source"}, lang)
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
	c := &LogbookCatalog{entries: map[int]LogbookEntry{}, titleFmt: "{0}: {1}", pageFmt: "{0}/{1}"}
	for n, b := range bodies {
		key := "logbook." + strconv.Itoa(n)
		var pages [][]string
		var tier NameTier
		for _, v := range names.Variants(b, key, NameCaseMixed) {
			if pages, err = layoutLogbookUnits(v.Text, v.Units); err == nil {
				tier = v.Tier
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("第 %d 則：%w", n, err)
		}
		e := LogbookEntry{Title: titles[n], Pages: pages, BodyTier: tier}
		if titles[n] != "" {
			e.titles = names.Variants(titles[n], key+".title", NameCaseMixed)
		} else {
			e.titles = []AnnotatedText{{Tier: NameTierNone}}
		}
		c.entries[n] = e
	}
	if err := c.chooseTitles(); err != nil {
		return nil, err
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
	colors  [2]uint8 // background, foreground of the call that opened it
	tlBy    map[[2]uint16]bool
	kbHead  uint16 // BDA keyboard head when the panel opened (spec 030 §3.3-4)
	kbArmed bool
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
	w.kbArmed = false
}

// BDAKeyHead is the linear address of the BIOS keyboard buffer head.
const BDAKeyHead = 0x41A

// ArmKeyHead records the keyboard buffer head read at the step the panel
// opened (spec 030 §3.3-4). It does nothing while the panel is closed.
func (w *LogbookWatcher) ArmKeyHead(head uint16) {
	if w == nil || w.open == 0 {
		return
	}
	w.kbHead, w.kbArmed = head, true
}

// ObserveKeyHead closes the panel once the game has taken a key from the
// buffer: the head differs from the one recorded at open. Inequality, not
// order, because the head wraps inside the ring.
func (w *LogbookWatcher) ObserveKeyHead(head uint16) {
	if w == nil || w.open == 0 || !w.kbArmed || head == w.kbHead {
		return
	}
	w.close()
}

// ObserveEntry runs at every 0763:056C entry (spec 030 §3.2, §3.3-1).
func (w *LogbookWatcher) ObserveEntry(original []byte, left, top, right, bottom uint8) {
	w.ObserveEntryColors(original, left, top, right, bottom, 0, 10)
}

// ObserveEntryColors is ObserveEntry with the call's colours; the panel
// uses them because the palette differs between screens.
func (w *LogbookWatcher) ObserveEntryColors(original []byte, left, top, right, bottom, bg, fg uint8) {
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
	w.colors = [2]uint8{bg, fg}
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
	fonts segmentFonts
	scale int
	gen   uint64
	// TitleErrors counts titles wider than the 38-cell row (spec 036 §3.3
	// guard); load-time selection should make this impossible.
	TitleErrors int
	LastError   string
}

func NewLogbookOverlay(font *xlate.Font, scale int) (*LogbookOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: 手札 presenter 輸入無效")
	}
	full := font
	if scale == 3 {
		full = manualThreeXFont(font)
		full.Name = font.Name + ".logbook.3x22"
	}
	return &LogbookOverlay{layer: &xlate.Layer{W: 320, H: 200}, fonts: familyFonts(font, full, scale), scale: scale}, nil
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
	rows := map[int]string{logbookTop: fillLogbook(w.catalog.titleFmt, strconv.Itoa(n), e.Title)}
	for i, l := range e.Pages[page] {
		rows[logbookFirstBody+i] = l
	}
	if len(e.Pages) > 1 {
		rows[logbookPageRow] = fillLogbook(w.catalog.pageFmt, strconv.Itoa(page+1), strconv.Itoa(len(e.Pages)))
	}
	var miss []rune
	bg, fg := palette[w.colors[0]], palette[w.colors[1]]
	for r := logbookTop; r <= logbookPageRow; r++ {
		text := []rune(rows[r])
		miss = append(miss, o.fonts.missingRunes(text)...)
		units := logbookTitleUnits
		if u := textUnits(text); u > units {
			// Never cut a title silently: record it and draw it whole.
			o.TitleErrors++
			o.LastError = fmt.Sprintf("手札第 %d 則第 %d 列 %d 單位超過 %d 單位", n, r, u, units)
			units = u + u%2
		}
		o.layer.Stamps = append(o.layer.Stamps, o.fonts.segmentStamps(fmt.Sprintf("logbook.%d", r), logbookLeft*8, r*8,
			padUnits(text, units), nil, func(int) ([3]uint8, [3]uint8) { return bg, fg })...)
	}
	if len(miss) != 0 {
		o.layer.Stamps = nil
	}
	return miss
}

// Rect is the panel rectangle in logical pixels.
func (o *LogbookOverlay) Rect() (x0, y0, x1, y1 int) {
	return logbookLeft * 8, logbookTop * 8, (logbookLeft + logbookCols + 2) * 8, (logbookPageRow + 1) * 8
}

func (o *LogbookOverlay) Active() bool { return o != nil && len(o.layer.Stamps) != 0 }

func (o *LogbookOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
