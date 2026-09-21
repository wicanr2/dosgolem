package buckrogers

import (
	"fmt"

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

// ValidateActionBarOverlayCoverage requires all 16 normal/focus request identities
// to have exact Phase 74 geometry and rejects orphan rectangles.
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
		if r.x != int(id.x0) || r.y != int(id.y0) || r.width != int(id.x1-id.x0) ||
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

// ActionBarNormalStyle has no default. Callers must explicitly select one
// proven palette index for every translated rune.
type ActionBarNormalStyle struct {
	RuneForegrounds []uint8
}

type ActionBarOverlay struct {
	Layer             *xlate.Layer
	EventKey, TextKey string
	ClearRect         PixelRect
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
	if len(runes) == 0 || len(runes) > r.capacity {
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
	} else {
		return nil, fmt.Errorf("buckrogers: 未知 action variant %q", event.Variant)
	}
	for _, runeValue := range runes {
		glyph, found := font.Glyphs[runeValue]
		if !found || len(glyph) != 32 {
			return nil, fmt.Errorf("buckrogers: action overlay 缺少有效字模 U+%04X", runeValue)
		}
	}
	offset := (8*scale - 16) / 2
	layer := &xlate.Layer{W: 320, H: 200}
	base := &xlate.Stamp{Key: event.EventKey + "#0", X: r.x, Y: r.y, Cells: r.capacity,
		CellW: 8, CellH: 8, Font: font, GlyphX: offset, GlyphY: offset,
		Text: []rune{runes[0]}, State: xlate.Shown, BG: palette[background], FG: palette[foregrounds[0]]}
	layer.Add(base)
	for i := 1; i < len(runes); i++ {
		stamp := &xlate.Stamp{Key: fmt.Sprintf("%s#%d", event.EventKey, i), X: r.drawX + i*8, Y: r.y,
			Cells: 1, CellW: 8, CellH: 8, Font: font, GlyphX: offset, GlyphY: offset,
			Text: []rune{runes[i]}, State: xlate.Shown, BG: palette[background], FG: palette[foregrounds[i]]}
		layer.Add(stamp)
	}
	clear := PixelRect{r.x * scale, r.y * scale, r.width * scale, r.height * scale}
	for _, stamp := range layer.Stamps {
		ink, err := menuInkRect(stamp, scale)
		if err != nil || ink.X < clear.X || ink.Y < clear.Y || ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
			return nil, fmt.Errorf("buckrogers: action overlay 字模墨跡超出安全矩形")
		}
	}
	return &ActionBarOverlay{Layer: layer, EventKey: event.EventKey, TextKey: request.TextKey,
		ClearRect: clear, RuneForegrounds: append([]uint8(nil), foregrounds...), TranslationRunes: len(runes)}, nil
}
