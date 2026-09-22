package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryPage3Overlay is output-only and owns no DOS state.
type RuntimeStoryPage3Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryPage3Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage3Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 5 {
		return nil, fmt.Errorf("buckrogers: 第 3 頁 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		if base == "" {
			font.Name = "buckrogers-story-page3-3x22"
		} else {
			font.Name = base + ".page3.3x22"
		}
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 3 頁缺字")
			}
		}
	}
	return &RuntimeStoryPage3Overlay{&xlate.Layer{W: 320, H: 200}, font, text, scale, false}, nil
}
func (o *RuntimeStoryPage3Overlay) Apply(es []StoryPage3Event, p [256][3]uint8) error {
	if o == nil || o.active || len(es) != 5 {
		return fmt.Errorf("buckrogers: 第 3 頁 apply 無效")
	}
	gen := es[0].Generation
	if gen == 0 {
		return fmt.Errorf("buckrogers: 第 3 頁 generation 無效")
	}
	seen := map[string]bool{}
	for i, e := range es {
		s := o.text[e.EventKey]
		if e.Generation != gen || seen[e.EventKey] || e.EventKey != "story.page3.line.00"+string(rune('1'+i)) || s == "" || e.Row != uint8(17+i) || e.Column != 1 {
			return fmt.Errorf("buckrogers: 第 3 頁 event 無效")
		}
		seen[e.EventKey] = true
	}
	for _, e := range es {
		s := o.text[e.EventKey]
		off := manualGlyphOffset(o.scale)
		o.layer.Add(&xlate.Stamp{Key: e.EventKey, X: 8, Y: int(e.Row) * 8, Cells: 39, CellW: 8, CellH: 8, Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1, Text: []rune(s), State: xlate.Shown, BG: p[0], FG: p[10]})
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryPage3Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 320, 176)
		o.active = false
	}
}
func (o *RuntimeStoryPage3Overlay) Frame(_ []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryPage3Overlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss, drew
}
func (o *RuntimeStoryPage3Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, s := range o.layer.Stamps {
		keys = append(keys, s.Key)
	}
	return keys
}
func (o *RuntimeStoryPage3Overlay) PresentationLayer() *xlate.Layer {
	if o == nil {
		return nil
	}
	return o.layer
}
