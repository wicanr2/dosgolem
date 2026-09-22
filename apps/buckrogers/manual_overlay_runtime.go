package buckrogers

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/xlate"
)

var manualOverlayLayoutHeader = []string{
	"layout_key", "clear_x", "clear_y", "clear_width", "clear_height", "text_x", "text_y",
	"columns", "rows", "capacity", "line_height", "line_spacing", "overflow_policy", "evidence_level",
}

// ManualOverlayLayout is the one confirmed preserved-prompt manual paragraph geometry.
// Its fields are private so callers must load and validate the formal TSV first.
type ManualOverlayLayout struct {
	key                                                    string
	clearX, clearY, clearWidth, clearHeight                int
	textX, textY, columns, rows, capacity, lineHeight, gap int
	overflow, evidence                                     string
}

var confirmedManualOverlayLayout = ManualOverlayLayout{
	key:    "manual.paragraph.body",
	clearX: 7, clearY: 72, clearWidth: 305, clearHeight: 112,
	textX: 16, textY: 72, columns: 36, rows: 14, capacity: 504, lineHeight: 8, gap: 0,
	overflow: "single-page-reject", evidence: "confirmed",
}

func (l *ManualOverlayLayout) confirmed() bool {
	if l == nil || *l != confirmedManualOverlayLayout {
		return false
	}
	textRight := l.textX + l.columns*8
	textBottom := l.textY + l.rows*(l.lineHeight+l.gap)
	clearRight := l.clearX + l.clearWidth
	clearBottom := l.clearY + l.clearHeight
	return textRight <= clearRight && textBottom <= clearBottom && l.capacity == l.columns*l.rows
}

// LoadManualOverlayLayout accepts only the formal, evidence-backed manual body layout.
func LoadManualOverlayLayout(name string, data []byte) (*ManualOverlayLayout, error) {
	rows, err := readTSV(name, data, manualOverlayLayoutHeader)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("%s: 手冊 layout 必須恰有一列", name)
	}
	row := rows[0]
	values := [11]int{}
	for i := range values {
		n, err := nonnegativeDecimal(row[i+1])
		if err != nil {
			return nil, fmt.Errorf("%s: %s 不是非負十進位整數", name, manualOverlayLayoutHeader[i+1])
		}
		values[i] = n
	}
	layout := &ManualOverlayLayout{
		key: row[0], clearX: values[0], clearY: values[1], clearWidth: values[2], clearHeight: values[3],
		textX: values[4], textY: values[5], columns: values[6], rows: values[7], capacity: values[8],
		lineHeight: values[9], gap: values[10], overflow: row[12], evidence: row[13],
	}
	if !layout.confirmed() {
		return nil, fmt.Errorf("%s: 與已確認手冊正文 layout 不符", name)
	}
	return layout, nil
}

type manualOverlayState uint8

const (
	manualOverlayIdle manualOverlayState = iota
	manualOverlayPending
	manualOverlayVisible
	manualOverlayCleared
)

// ManualOverlayAction is output-only metadata for one displayed manual paragraph.
type ManualOverlayAction struct {
	Generation       uint64
	EventKey         string
	TextKey          string
	TranslationRunes int
	ClearRect        PixelRect
	TextRect         PixelRect
}

// RuntimeManualOverlay owns only two output layers: a full-clear background
// and the 36x14 text grid. It never writes machine VRAM or accepts input.
type RuntimeManualOverlay struct {
	layout     *ManualOverlayLayout
	catalog    *Catalog
	font       *xlate.Font
	scale      int
	background *xlate.Layer
	text       *xlate.Layer
	generation uint64
	state      manualOverlayState
	actions    []ManualOverlayAction
	style      *ManualTextStyle
}

// SetStyle supplies the exact original manual-prefix palette indexes observed
// by Watcher. The presenter never samples a synthetic framebuffer to invent a
// foreground colour.
func (o *RuntimeManualOverlay) SetStyle(style ManualTextStyle) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 手冊 presenter 不得為 nil")
	}
	o.style = &style
	return nil
}

func (o *RuntimeManualOverlay) HasStyle() bool { return o != nil && o.style != nil }

// NewRuntimeManualOverlay validates the full catalog font coverage before any
// request can draw. This prevents a partial paragraph from reaching RGBA.
func NewRuntimeManualOverlay(layout *ManualOverlayLayout, catalog *Catalog, font *xlate.Font, scale int) (*RuntimeManualOverlay, error) {
	if !layout.confirmed() {
		return nil, fmt.Errorf("buckrogers: 手冊 layout 無效或未確認")
	}
	if catalog == nil || len(catalog.byIdentity) == 0 {
		return nil, fmt.Errorf("buckrogers: 手冊 presenter 缺少 catalog")
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("buckrogers: 手冊 presenter 字型必須是 16x16")
	}
	if scale != 2 && scale != 3 {
		return nil, fmt.Errorf("buckrogers: 手冊 presenter 倍率必須明示為 2 或 3")
	}
	if err := validateManualCatalogFont(layout, catalog, font); err != nil {
		return nil, err
	}
	if scale == 3 {
		// A 3x text cell is 24 pixels wide. Enlarge CJK ink to 22 pixels
		// so adjacent characters read as a line, while keeping ASCII at
		// its original 16-pixel size. This is an output-only derived font.
		font = manualThreeXFont(font)
	}
	o := &RuntimeManualOverlay{layout: layout, catalog: catalog, font: font, scale: scale}
	o.resetLayers()
	return o, nil
}

func validateManualCatalogFont(layout *ManualOverlayLayout, catalog *Catalog, font *xlate.Font) error {
	required := make(map[rune]bool)
	for _, entry := range catalog.byIdentity {
		if entry.translation == "" || !utf8.ValidString(entry.translation) || len([]rune(entry.translation)) > layout.capacity {
			return fmt.Errorf("buckrogers: 手冊 catalog 有無效譯文")
		}
		for _, r := range entry.translation {
			required[r] = true
		}
	}
	if len(required) == 0 {
		return fmt.Errorf("buckrogers: 手冊 catalog 沒有字型需求")
	}
	runes := make([]rune, 0, len(required))
	for r := range required {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	for _, r := range runes {
		glyph, ok := font.Glyphs[r]
		if !ok || len(glyph) != 32 {
			return fmt.Errorf("buckrogers: 手冊字型缺少有效字模 U+%04X", r)
		}
	}
	return nil
}

func (c *Catalog) containsManualRequest(request DisplayRequest) bool {
	if c == nil {
		return false
	}
	for _, entry := range c.byIdentity {
		if request.EventKey == entry.eventKey && request.TextKey == entry.textKey && request.Translation == entry.translation {
			return true
		}
	}
	return false
}

func (o *RuntimeManualOverlay) resetLayers() {
	o.background = &xlate.Layer{W: 320, H: 200}
	o.text = &xlate.Layer{W: 320, H: 200}
}

// Apply consumes only the CONFORMED manual presentation lifecycle values.
func (o *RuntimeManualOverlay) Apply(event ManualPresentationEvent) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 手冊 presenter 不得為 nil")
	}
	switch event.Kind {
	case ManualPresentationBegin:
		if event.Generation == 0 || event.Request != (DisplayRequest{}) || event.Generation <= o.generation {
			return fmt.Errorf("buckrogers: 手冊 begin generation 無效")
		}
		o.generation, o.state = event.Generation, manualOverlayPending
		o.resetLayers()
		return nil
	case ManualPresentationClear:
		if event.Generation == 0 || event.Request != (DisplayRequest{}) || event.Generation != o.generation {
			return fmt.Errorf("buckrogers: 手冊 clear generation 無效")
		}
		switch o.state {
		case manualOverlayPending:
			return nil
		case manualOverlayVisible:
			o.resetLayers()
			o.state = manualOverlayCleared
			return nil
		default:
			return fmt.Errorf("buckrogers: 手冊 clear 狀態無效")
		}
	case ManualPresentationRequest:
		if event.Generation == 0 || event.Generation != o.generation || event.Request.Generation != event.Generation ||
			o.state != manualOverlayPending {
			return fmt.Errorf("buckrogers: 手冊 request generation 或狀態無效")
		}
		if event.Request.EventKey == "" || event.Request.TextKey == "" || event.Request.Translation == "" ||
			!utf8.ValidString(event.Request.Translation) || len([]rune(event.Request.Translation)) > o.layout.capacity ||
			!o.catalog.containsManualRequest(event.Request) {
			return fmt.Errorf("buckrogers: 手冊 request 不符合正式 catalog")
		}
		background, text, err := o.build(event)
		if err != nil {
			return err
		}
		o.background, o.text, o.state = background, text, manualOverlayVisible
		o.actions = append(o.actions, ManualOverlayAction{
			Generation: event.Generation, EventKey: event.Request.EventKey, TextKey: event.Request.TextKey,
			TranslationRunes: len([]rune(event.Request.Translation)),
			ClearRect:        PixelRect{o.layout.clearX * o.scale, o.layout.clearY * o.scale, o.layout.clearWidth * o.scale, o.layout.clearHeight * o.scale},
			TextRect:         PixelRect{o.layout.textX * o.scale, o.layout.textY * o.scale, o.layout.columns * 8 * o.scale, o.layout.rows * o.layout.lineHeight * o.scale},
		})
		return nil
	default:
		return fmt.Errorf("buckrogers: 未知手冊 presentation event %q", event.Kind)
	}
}

func (o *RuntimeManualOverlay) build(event ManualPresentationEvent) (*xlate.Layer, *xlate.Layer, error) {
	rows, err := manualRows(event.Request.Translation, o.layout.columns, o.layout.rows)
	if err != nil {
		return nil, nil, err
	}
	background := &xlate.Layer{W: 320, H: 200}
	text := &xlate.Layer{W: 320, H: 200}
	for row := 0; row < o.layout.rows; row++ {
		y := o.layout.clearY + row*(o.layout.lineHeight+o.layout.gap)
		background.Add(&xlate.Stamp{
			Key: fmt.Sprintf("manual.background.%d", row), X: o.layout.clearX, Y: y,
			Cells: 1, CellW: o.layout.clearWidth, CellH: o.layout.lineHeight, State: xlate.Pending,
		})
		text.Add(&xlate.Stamp{
			Key: fmt.Sprintf("manual.%d.%s.%02d", event.Generation, event.Request.TextKey, row),
			X:   o.layout.textX, Y: y, Cells: o.layout.columns, CellW: 8, CellH: o.layout.lineHeight,
			Font: o.font, GlyphX: manualGlyphOffset(o.scale), GlyphY: manualGlyphOffset(o.scale),
			Text: []rune(rows[row]), State: xlate.Pending,
		})
	}
	return background, text, nil
}

func manualGlyphOffset(scale int) int {
	if scale == 3 {
		return 1 // 22-pixel glyph in a 24-pixel output cell.
	}
	offset := (8*scale - 16) / 2
	if offset < 0 {
		return 0
	}
	return offset
}

// manualThreeXFont changes only the output bitmap. The source GOLEMFNT and
// the game's indexed framebuffer remain untouched. Each CJK pixel is sampled
// with nearest-neighbour scaling from 16×16 to 22×22; ASCII stays 16×16,
// centred in the derived 22×22 bitmap.
func manualThreeXFont(source *xlate.Font) *xlate.Font {
	const width = 22
	out := &xlate.Font{W: width, H: width, Glyphs: make(map[rune][]byte, len(source.Glyphs))}
	for character, original := range source.Glyphs {
		bitmap := make([]byte, width*3)
		if character <= 0xff {
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if original[y*2+x/8]&(0x80>>uint(x%8)) != 0 {
						setManualThreeXPixel(bitmap, x+3, y+3)
					}
				}
			}
		} else {
			for y := 0; y < width; y++ {
				for x := 0; x < width; x++ {
					sx, sy := x*16/width, y*16/width
					if original[sy*2+sx/8]&(0x80>>uint(sx%8)) != 0 {
						setManualThreeXPixel(bitmap, x, y)
					}
				}
			}
		}
		out.Glyphs[character] = bitmap
	}
	return out
}

func setManualThreeXPixel(bitmap []byte, x, y int) {
	bitmap[y*3+x/8] |= 0x80 >> uint(x%8)
}

// Frame samples only the original indexed framebuffer and palette.
func (o *RuntimeManualOverlay) Frame(indexed []byte, palette [256][3]uint8) {
	if o == nil {
		return
	}
	if o.style == nil {
		rgb := logicalRGB(indexed, palette)
		o.background.Frame(indexed, rgb)
		o.text.Frame(indexed, rgb)
		return
	}
	// The proven body rectangle may already be blank when the request completes.
	// Use the exact original prompt style captured by Watcher instead of applying
	// Layer.Frame's majority-colour heuristic to an all-background rectangle.
	show := func(layer *xlate.Layer, fg uint8) {
		for _, stamp := range layer.Stamps {
			if stamp.State != xlate.Pending && stamp.State != xlate.Shown {
				continue
			}
			// Palette animation changes RGB without changing a valid original
			// stamp. Refresh both pending and shown stamps, but never revive an
			// invalidated state.
			stamp.BG, stamp.FG = palette[o.style.Background], palette[fg]
			if stamp.State == xlate.Pending {
				stamp.State = xlate.Shown
			}
		}
	}
	show(o.background, o.style.Background)
	show(o.text, o.style.Foreground)
}

// Draw composes the background clear layer, then the text layer, over a fresh RGBA baseline.
func (o *RuntimeManualOverlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	rgba := ScaleIndexedRGBA(indexed, palette, o.scale)
	missing := []rune{}
	background := o.background.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	text := o.text.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	return rgba, missing, background || text
}

func (o *RuntimeManualOverlay) ActiveGeneration() uint64 {
	if o == nil {
		return 0
	}
	return o.generation
}

func (o *RuntimeManualOverlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, len(o.text.Stamps))
	for i, stamp := range o.text.Stamps {
		keys[i] = stamp.Key
	}
	return keys
}

func (o *RuntimeManualOverlay) Actions() []ManualOverlayAction {
	if o == nil {
		return nil
	}
	return append([]ManualOverlayAction(nil), o.actions...)
}

func manualRows(text string, columns, rows int) ([]string, error) {
	if text == "" || !utf8.ValidString(text) || columns <= 0 || rows <= 0 || len([]rune(text)) > columns*rows {
		return nil, fmt.Errorf("buckrogers: 手冊 row-major 文字無效")
	}
	runes := []rune(text)
	out := make([]string, rows)
	for row := range out {
		start, end := row*columns, (row+1)*columns
		if start >= len(runes) {
			break
		}
		if end > len(runes) {
			end = len(runes)
		}
		out[row] = string(runes[start:end])
	}
	return out, nil
}
