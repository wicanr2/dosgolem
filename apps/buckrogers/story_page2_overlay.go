package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryPage2Overlay is output-only and owns no DOS state.
type RuntimeStoryPage2Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryPage2Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage2Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 4 {
		return nil, fmt.Errorf("buckrogers: 第 2 頁 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		if base == "" {
			font.Name = "buckrogers-story-page2-3x22"
		} else {
			font.Name = base + ".page2.3x22"
		}
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 2 頁缺字")
			}
		}
	}
	return &RuntimeStoryPage2Overlay{&xlate.Layer{W: 320, H: 200}, font, text, scale, false}, nil
}
func (o *RuntimeStoryPage2Overlay) Apply(es []StoryPage2Event, p [256][3]uint8) error {
	if o == nil || o.active || len(es) != 4 {
		return fmt.Errorf("buckrogers: 第 2 頁 apply 無效")
	}
	gen := es[0].Generation
	if gen == 0 {
		return fmt.Errorf("buckrogers: 第 2 頁 generation 無效")
	}
	seen := map[string]bool{}
	for i, e := range es {
		s := o.text[e.EventKey]
		if e.Generation != gen || seen[e.EventKey] || e.EventKey != "story.page2.line.00"+string(rune('1'+i)) || s == "" || e.Row != uint8(17+i) || e.Column != 1 {
			return fmt.Errorf("buckrogers: 第 2 頁 event 無效")
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
func (o *RuntimeStoryPage2Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 320, 168)
		o.active = false
	}
}
func (o *RuntimeStoryPage2Overlay) Frame(_ []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryPage2Overlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss, drew
}
func (o *RuntimeStoryPage2Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, s := range o.layer.Stamps {
		keys = append(keys, s.Key)
	}
	return keys
}
func (o *RuntimeStoryPage2Overlay) PresentationLayer() *xlate.Layer {
	if o == nil {
		return nil
	}
	return o.layer
}
