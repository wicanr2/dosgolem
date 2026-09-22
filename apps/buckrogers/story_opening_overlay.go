package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryOpeningOverlay is output-only; it never retains machine state.
type RuntimeStoryOpeningOverlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryOpeningOverlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryOpeningOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 5 {
		return nil, fmt.Errorf("buckrogers: 首屏 presenter 輸入無效")
	}
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != (font.H*((font.W+7)/8)) {
				return nil, fmt.Errorf("buckrogers: 首屏缺字")
			}
		}
	}
	return &RuntimeStoryOpeningOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, text: text, scale: scale}, nil
}
func (o *RuntimeStoryOpeningOverlay) Apply(events []StoryOpeningEvent, p [256][3]uint8) error {
	if o == nil || o.active || len(events) != 5 {
		return fmt.Errorf("buckrogers: 首屏 apply 無效")
	}
	for i, e := range events {
		s := o.text[e.EventKey]
		if s == "" || e.Row != uint8(17+i) || e.Column != 1 {
			return fmt.Errorf("buckrogers: 首屏 event 無效")
		}
		off := manualGlyphOffset(o.scale)
		o.layer.Add(&xlate.Stamp{Key: e.EventKey, X: 8, Y: int(e.Row) * 8, Cells: 39, CellW: 8, CellH: 8, Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1, Text: []rune(s), State: xlate.Shown, BG: p[0], FG: p[10]})
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryOpeningOverlay) Clear() {
	if o != nil {
		o.layer = &xlate.Layer{W: 320, H: 200}
		o.active = false
	}
}
func (o *RuntimeStoryOpeningOverlay) Frame(indexed []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryOpeningOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss, drew
}

// ActiveKeys returns the current, content-safe story event keys.  It is used
// by receipt validation; callers cannot mutate the overlay through it.
func (o *RuntimeStoryOpeningOverlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, stamp := range o.layer.Stamps {
		keys = append(keys, stamp.Key)
	}
	return keys
}
