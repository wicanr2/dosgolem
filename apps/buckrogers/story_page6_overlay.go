package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryPage6Overlay owns RGBA stamps only; it has no DOS-state access.
type RuntimeStoryPage6Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryPage6Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage6Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 6 {
		return nil, fmt.Errorf("buckrogers: 第 6 頁 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		if base == "" {
			font.Name = "buckrogers-story-page6-3x22"
		} else {
			font.Name = base + ".page6.3x22"
		}
	}
	for _, line := range text {
		for _, runeValue := range line {
			if glyph, ok := font.Glyphs[runeValue]; !ok || len(glyph) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 6 頁缺字")
			}
		}
	}
	return &RuntimeStoryPage6Overlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, text: text, scale: scale}, nil
}
func (overlay *RuntimeStoryPage6Overlay) Apply(events []StoryPage6Event, palette [256][3]uint8) error {
	if overlay == nil || overlay.active || len(events) != 6 {
		return fmt.Errorf("buckrogers: 第 6 頁 apply 無效")
	}
	generation := events[0].Generation
	if generation == 0 {
		return fmt.Errorf("buckrogers: 第 6 頁 generation 無效")
	}
	seen := map[string]bool{}
	for i, event := range events {
		if event.Generation != generation || seen[event.EventKey] || event.EventKey != fmt.Sprintf("story.page6.line.%03d", i+1) || event.Row != uint8(17+i) || event.Column != 1 || overlay.text[event.EventKey] == "" {
			return fmt.Errorf("buckrogers: 第 6 頁 event 無效")
		}
		seen[event.EventKey] = true
	}
	for _, event := range events {
		offset := manualGlyphOffset(overlay.scale)
		overlay.layer.Add(&xlate.Stamp{Key: event.EventKey, X: 8, Y: int(event.Row) * 8, Cells: 39, CellW: 8, CellH: 8, Font: overlay.font, GlyphX: offset, GlyphY: offset, GlyphScale: 1, Text: []rune(overlay.text[event.EventKey]), State: xlate.Shown, BG: palette[0], FG: palette[10]})
	}
	overlay.active = true
	return nil
}
func (overlay *RuntimeStoryPage6Overlay) Clear() {
	if overlay != nil {
		overlay.layer.Clear(8, 136, 320, 184)
		overlay.active = false
	}
}
func (overlay *RuntimeStoryPage6Overlay) Frame(_ []byte, palette [256][3]uint8) {
	if overlay != nil {
		for _, stamp := range overlay.layer.Stamps {
			stamp.BG = palette[0]
			stamp.FG = palette[10]
		}
	}
}
func (overlay *RuntimeStoryPage6Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	if overlay == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, palette, overlay.scale)
	var missing []rune
	drew := overlay.layer.Draw(out, overlay.scale, func(value rune) { missing = append(missing, value) })
	return out, missing, drew
}
func (overlay *RuntimeStoryPage6Overlay) ActiveKeys() []string {
	if overlay == nil {
		return nil
	}
	out := make([]string, 0, len(overlay.layer.Stamps))
	for _, stamp := range overlay.layer.Stamps {
		out = append(out, stamp.Key)
	}
	return out
}
func (overlay *RuntimeStoryPage6Overlay) PresentationLayer() *xlate.Layer {
	if overlay == nil {
		return nil
	}
	return overlay.layer
}
