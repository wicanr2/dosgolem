package buckrogers

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// LoadActionBarOverlayRects loads geometry without choosing a normal-label color policy.
func LoadActionBarOverlayRects(name string, data []byte) (*MenuOverlayRects, error) {
	rects, err := LoadMenuOverlayRects(name, data)
	if err != nil {
		return nil, err
	}
	for key, rect := range rects.byEvent {
		if rect.y != 192 || rect.height != 8 || rect.drawX != rect.x || rect.drawY != rect.y ||
			rect.width <= 0 || rect.width%8 != 0 || rect.capacity != rect.width/8 ||
			rect.lines != 1 || rect.overflow != "single-line-reject" {
			return nil, fmt.Errorf("%s: %s 不符合 action-bar exact band", name, key)
		}
	}
	return rects, nil
}

// ValidateActionBarOverlayCoverage requires all 16 normal/focus request identities.
// The three-cell Add source slot has one proven blank cell before Subtract, so
// its presentation-only clear rectangle is explicitly widened by one cell.
func ValidateActionBarOverlayCoverage(catalog *ActionBarRequestCatalog, rects *MenuOverlayRects) error {
	if catalog == nil || rects == nil {
		return fmt.Errorf("buckrogers: action overlay catalog 與安全矩形不得為空")
	}
	events := make(map[string]actionRequestIdentity, len(catalog.byIdentity))
	for id, request := range catalog.byIdentity {
		events[request.EventKey] = id
	}
	for key, id := range events {
		r, ok := rects.byEvent[key]
		if !ok {
			return fmt.Errorf("buckrogers: action catalog event %q 缺少安全矩形", key)
		}
		wantWidth := int(id.x1 - id.x0)
		if strings.Contains(key, ".action.add.") {
			wantWidth += 8
		}
		if r.x != int(id.x0) || r.y != int(id.y0) || r.width != wantWidth ||
			r.height != int(id.y1-id.y0) || r.drawX != r.x || r.drawY != r.y || r.capacity != r.width/8 {
			return fmt.Errorf("buckrogers: action rect %q 與 exact identity 不符", key)
		}
	}
	for key := range rects.byEvent {
		if _, ok := events[key]; !ok {
			return fmt.Errorf("buckrogers: 孤兒 action 安全矩形 %q", key)
		}
	}
	return nil
}

// ActionBarNormalStyle is explicit so no caller can silently discard the
// preserved mnemonic-letter color contract.
type ActionBarNormalStyle struct {
	RuneForegrounds []uint8
}

// HotkeyPreservingActionBarNormalStyle implements the confirmed display rule:
// only the Latin mnemonic is white; parentheses and Traditional Chinese are
// rendered with the original normal-label color.
func HotkeyPreservingActionBarNormalStyle() ActionBarNormalStyle {
	return ActionBarNormalStyle{RuneForegrounds: []uint8{10, 10, 10, 15, 10}}
}

func actionBarRuneAdvance(r rune) int {
	if r < 128 {
		return 4
	}
	return 8
}

// actionBarGlyphLayout preserves the established 2× pixels and all ASCII
// mnemonic glyphs. At 3× only CJK glyphs use a 22×22 nearest-neighbour raster
// inside their 24×24 output cell, leaving a one-pixel inset and reducing the
// inter-glyph gap from eight to two pixels. This remains inside spec 215's
// logical 8-pixel-high action-bar band after scaling.
func actionBarGlyphLayout(font *xlate.Font, r rune, scale int) (*xlate.Font, int, int, int, error) {
	offset := (8*scale - 16) / 2
	if scale != 3 || r < 128 {
		return font, offset, offset, 0, nil
	}
	source, ok := font.Glyphs[r]
	if !ok || len(source) != 32 {
		return nil, 0, 0, 0, fmt.Errorf("buckrogers: action overlay 缺少有效字模 U+%04X", r)
	}
	return &xlate.Font{W: 22, H: 22, Glyphs: map[rune][]byte{r: scaleActionBarGlyph22(source)}}, 1, 1, 1, nil
}

// scaleActionBarGlyph22 expands a 16×16 GOLEMFNT raster to an exact 22×22
// bitmap without changing the source font or its 2× rendering path.
func scaleActionBarGlyph22(source []byte) []byte {
	const sourceWidth, targetWidth = 16, 22
	out := make([]byte, targetWidth*((targetWidth+7)/8))
	for y := 0; y < targetWidth; y++ {
		sy := y * sourceWidth / targetWidth
		for x := 0; x < targetWidth; x++ {
			sx := x * sourceWidth / targetWidth
			if source[sy*2+sx/8]&(0x80>>uint(sx%8)) != 0 {
				out[y*3+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}
	return out
}

type ActionBarOverlay struct {
	Layer             *xlate.Layer
	EventKey, TextKey string
	ClearRect         PixelRect
	Background        uint8
	RuneForegrounds   []uint8
	TranslationRunes  int
}

func (c *ActionBarRequestCatalog) entryFor(event ActionBarEvent) (actionBarEntry, bool) {
	if c == nil || c.events == nil {
		return actionBarEntry{}, false
	}
	for _, entry := range c.events.byScreen[event.Screen] {
		if event.EventKey == entry.screen+"."+entry.key+"."+event.Variant {
			return entry, true
		}
	}
	return actionBarEntry{}, false
}

// BuildActionBarOverlay builds a presentation-only multi-color stamp group.
// Focus colors are exact evidence; normal colors must be explicitly supplied.
func BuildActionBarOverlay(catalog *ActionBarRequestCatalog, rects *MenuOverlayRects,
	event ActionBarEvent, request DisplayRequest, font *xlate.Font, palette [256][3]uint8,
	scale int, normal *ActionBarNormalStyle) (*ActionBarOverlay, error) {

	want, ok := catalog.Resolve(event)
	if !ok || want != request {
		return nil, fmt.Errorf("buckrogers: action overlay request 與 exact event 不符")
	}
	if err := ValidateActionBarOverlayCoverage(catalog, rects); err != nil {
		return nil, err
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("buckrogers: action overlay 字型必須是 16x16")
	}
	if scale != 2 && scale != 3 {
		return nil, fmt.Errorf("buckrogers: action overlay 倍率必須明示為 2 或 3")
	}
	entry, ok := catalog.entryFor(event)
	if !ok {
		return nil, fmt.Errorf("buckrogers: action overlay 找不到事件來源")
	}
	r := rects.byEvent[event.EventKey]
	runes := []rune(request.Translation)
	logicalWidth := 0
	for _, runeValue := range runes {
		logicalWidth += actionBarRuneAdvance(runeValue)
	}
	if len(runes) == 0 || logicalWidth > r.width {
		return nil, fmt.Errorf("buckrogers: action overlay 譯文容量不符")
	}
	foregrounds := make([]uint8, len(runes))
	background := entry.focusBG
	if event.Variant == "focus" {
		if normal != nil {
			return nil, fmt.Errorf("buckrogers: focus 不接受 normal 配色策略")
		}
		for i := range foregrounds {
			foregrounds[i] = entry.focusFG
		}
	} else if event.Variant == "normal" {
		background = entry.normalBG
		if normal == nil || len(normal.RuneForegrounds) != len(runes) {
			return nil, fmt.Errorf("buckrogers: normal 必須逐 rune 明示配色")
		}
		for i, color := range normal.RuneForegrounds {
			if color != entry.normalFirstFG && color != entry.normalRestFG {
				return nil, fmt.Errorf("buckrogers: normal rune %d 使用未證實色號 %d", i, color)
			}
			foregrounds[i] = color
		}
		confirmed := HotkeyPreservingActionBarNormalStyle().RuneForegrounds
		if len(foregrounds) != len(confirmed) {
			return nil, fmt.Errorf("buckrogers: normal 快捷字母顯示契約長度不符")
		}
		for i := range confirmed {
			if foregrounds[i] != confirmed[i] {
				return nil, fmt.Errorf("buckrogers: normal 快捷字母配色契約漂移")
			}
		}
	} else {
		return nil, fmt.Errorf("buckrogers: 未知 action variant %q", event.Variant)
	}
	for _, runeValue := range runes {
		glyph, found := font.Glyphs[runeValue]
		if !found || len(glyph) != 32 {
			return nil, fmt.Errorf("buckrogers: action overlay 缺少有效字模 U+%04X", runeValue)
		}
	}
	layer := &xlate.Layer{W: 320, H: 200}
	stampFor := func(index, x int) (*xlate.Stamp, error) {
		runeValue := runes[index]
		glyphFont, glyphX, glyphY, glyphScale, err := actionBarGlyphLayout(font, runeValue, scale)
		if err != nil {
			return nil, err
		}
		return &xlate.Stamp{Key: fmt.Sprintf("%s#%d", event.EventKey, index), X: x, Y: r.y, Cells: 1,
			CellW: actionBarRuneAdvance(runeValue), CellH: 8, Font: glyphFont, GlyphX: glyphX, GlyphY: glyphY,
			GlyphScale: glyphScale, Text: []rune{runeValue}, State: xlate.Shown,
			BG: palette[background], FG: palette[foregrounds[index]]}, nil
	}
	base, err := stampFor(0, r.x)
	if err != nil {
		return nil, err
	}
	layer.Add(base)
	drawX := r.drawX + actionBarRuneAdvance(runes[0])
	for i := 1; i < len(runes); i++ {
		stamp, err := stampFor(i, drawX)
		if err != nil {
			return nil, err
		}
		layer.Add(stamp)
		drawX += actionBarRuneAdvance(runes[i])
	}
	clear := PixelRect{r.x * scale, r.y * scale, r.width * scale, r.height * scale}
	for _, stamp := range layer.Stamps {
		ink, err := menuInkRect(stamp, scale)
		if err != nil || ink.X < clear.X || ink.Y < clear.Y || ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
			return nil, fmt.Errorf("buckrogers: action overlay 字模墨跡超出安全矩形")
		}
	}
	return &ActionBarOverlay{Layer: layer, EventKey: event.EventKey, TextKey: request.TextKey,
		ClearRect: clear, Background: background, RuneForegrounds: append([]uint8(nil), foregrounds...), TranslationRunes: len(runes)}, nil
}
