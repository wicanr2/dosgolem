package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

type RuntimeStoryPage9Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   string
	scale  int
	active bool
}

func NewRuntimeStoryPage9Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage9Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 1 || text["story.page9.line.001"] == "" {
		return nil, fmt.Errorf("buckrogers: 第 9 頁 presenter 輸入無效")
	}
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	translation := text["story.page9.line.001"]
	if len([]rune(translation)) > 20 {
		return nil, fmt.Errorf("buckrogers: 第 9 頁譯文超過安全矩形")
	}
	for _, r := range translation {
		glyph, ok := font.Glyphs[r]
		if !ok || len(glyph) != font.H*((font.W+7)/8) {
			return nil, fmt.Errorf("buckrogers: 第 9 頁缺字")
		}
	}
	return &RuntimeStoryPage9Overlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, text: translation, scale: scale}, nil
}

func (o *RuntimeStoryPage9Overlay) Apply(event StoryPage9Event, palette [256][3]uint8) error {
	if o == nil || o.active || event.Generation == 0 || event.EntryStep >= event.PostCallStep ||
		event.EventKey != "story.page9.line.001" || event.Row != 17 || event.Column != 1 {
		return fmt.Errorf("buckrogers: 第 9 頁 event 無效")
	}
	o.layer.Add(&xlate.Stamp{Key: event.EventKey, X: 8, Y: 136, Cells: 20, CellW: 8, CellH: 8,
		Font: o.font, GlyphX: manualGlyphOffset(o.scale), GlyphY: manualGlyphOffset(o.scale), GlyphScale: 1,
		Text: []rune(o.text), State: xlate.Shown, BG: palette[0], FG: palette[10]})
	o.active = true
	return nil
}

func (o *RuntimeStoryPage9Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 168, 144)
		o.active = false
	}
}
func (o *RuntimeStoryPage9Overlay) Frame(_ []byte, palette [256][3]uint8) {
	if o == nil {
		return
	}
	for _, stamp := range o.layer.Stamps {
		stamp.BG, stamp.FG = palette[0], palette[10]
	}
}
func (o *RuntimeStoryPage9Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, palette, o.scale)
	var missing []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { missing = append(missing, r) })
	return out, missing, drew
}
func (o *RuntimeStoryPage9Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, s := range o.layer.Stamps {
		keys = append(keys, s.Key)
	}
	return keys
}
