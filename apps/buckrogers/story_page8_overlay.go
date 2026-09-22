package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

type RuntimeStoryPage8Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryPage8Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage8Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 4 {
		return nil, fmt.Errorf("buckrogers: 第 8 頁 presenter 輸入無效")
	}
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	for i := 1; i <= 4; i++ {
		key := fmt.Sprintf("story.page8.line.%03d", i)
		translation := text[key]
		if translation == "" {
			return nil, fmt.Errorf("buckrogers: 第 8 頁缺譯文")
		}
		for _, r := range translation {
			glyph, ok := font.Glyphs[r]
			if !ok || len(glyph) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 8 頁缺字")
			}
		}
	}
	return &RuntimeStoryPage8Overlay{
		layer: &xlate.Layer{W: 320, H: 200}, font: font, text: text, scale: scale,
	}, nil
}

func (o *RuntimeStoryPage8Overlay) Apply(events []StoryPage8Event, palette [256][3]uint8) error {
	if o == nil || o.active || len(events) != 4 {
		return fmt.Errorf("buckrogers: 第 8 頁 apply 無效")
	}
	generation := events[0].Generation
	if generation == 0 {
		return fmt.Errorf("buckrogers: 第 8 頁 generation 無效")
	}
	seen := make(map[string]bool, 4)
	for i, event := range events {
		key := fmt.Sprintf("story.page8.line.%03d", i+1)
		if event.Generation != generation || event.EntryStep >= event.PostCallStep ||
			(i > 0 && events[i-1].PostCallStep >= event.EntryStep) || seen[event.EventKey] ||
			event.EventKey != key || event.Row != uint8(17+i) || event.Column != 1 || o.text[key] == "" {
			return fmt.Errorf("buckrogers: 第 8 頁 event 無效")
		}
		seen[event.EventKey] = true
	}
	offset := manualGlyphOffset(o.scale)
	for _, event := range events {
		o.layer.Add(&xlate.Stamp{
			Key: event.EventKey, X: 8, Y: int(event.Row) * 8,
			Cells: 38, CellW: 8, CellH: 8,
			Font: o.font, GlyphX: offset, GlyphY: offset, GlyphScale: 1,
			Text: []rune(o.text[event.EventKey]), State: xlate.Shown,
			BG: palette[0], FG: palette[10],
		})
	}
	o.active = true
	return nil
}

func (o *RuntimeStoryPage8Overlay) Clear() {
	if o == nil {
		return
	}
	o.layer.Clear(8, 136, 312, 168)
	o.active = false
}

func (o *RuntimeStoryPage8Overlay) Frame(_ []byte, palette [256][3]uint8) {
	if o == nil {
		return
	}
	for _, stamp := range o.layer.Stamps {
		stamp.BG = palette[0]
		stamp.FG = palette[10]
	}
}

func (o *RuntimeStoryPage8Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, palette, o.scale)
	var missing []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { missing = append(missing, r) })
	return out, missing, drew
}

func (o *RuntimeStoryPage8Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, stamp := range o.layer.Stamps {
		keys = append(keys, stamp.Key)
	}
	return keys
}
