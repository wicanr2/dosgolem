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
	half   *xlate.Font // spec 039 half font (8×16 at 2×, 12×24 at 3×)
}

func NewRuntimeStoryPage9Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage9Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 1 || text["story.page9.line.001"] == "" {
		return nil, fmt.Errorf("buckrogers: 第 9 頁 presenter 輸入無效")
	}
	half := halfFontsOf(font).For(scale)
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	translation := text["story.page9.line.001"]
	for _, r := range translation {
		glyph, ok := font.Glyphs[r]
		if !ok || len(glyph) != font.H*((font.W+7)/8) {
			return nil, fmt.Errorf("buckrogers: 第 9 頁缺字")
		}
	}
	if err := storyTextCheck(map[string]string{"story.page9.line.001": translation}, segmentFonts{Full: font, Half: half}, 40); err != nil {
		return nil, fmt.Errorf("buckrogers: 第 9 頁：%w", err)
	}
	return &RuntimeStoryPage9Overlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, text: translation, scale: scale, half: half}, nil
}

func (o *RuntimeStoryPage9Overlay) Apply(event StoryPage9Event, palette [256][3]uint8) error {
	if o == nil || o.active || event.Generation == 0 || event.EntryStep >= event.PostCallStep ||
		event.EventKey != "story.page9.line.001" || event.Row != 17 || event.Column != 1 {
		return fmt.Errorf("buckrogers: 第 9 頁 event 無效")
	}
	for _, stamp := range storyRowStamps(event.EventKey, 8, 136, 20, []rune(o.text), o.storyFonts(), palette[0], palette[10]) {
		o.layer.Add(stamp)
	}
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
	return dedupRowKeys(o.layer.Stamps)
}

// storyFonts returns the spec 039 segment fonts of this presenter.
func (o *RuntimeStoryPage9Overlay) storyFonts() segmentFonts {
	off := manualGlyphOffset(o.scale)
	return segmentFonts{Full: o.font, FullX: off, FullY: off, FullGlyphScale: 1, Half: o.half}
}

// storyFontsPlain is storyFonts for full cells drawn without an offset.
func (o *RuntimeStoryPage9Overlay) storyFontsPlain() segmentFonts {
	return segmentFonts{Full: o.font, Half: o.half}
}
