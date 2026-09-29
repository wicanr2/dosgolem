package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryPage5Overlay owns only translated RGBA stamps, never DOS state.
type RuntimeStoryPage5Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
	half   *xlate.Font // spec 039 half font (8×16 at 2×, 12×24 at 3×)
}

func NewRuntimeStoryPage5Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage5Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 5 {
		return nil, fmt.Errorf("buckrogers: 第 5 頁 presenter 輸入無效")
	}
	half := halfFontsOf(font).For(scale)
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		if base == "" {
			font.Name = "buckrogers-story-page5-3x22"
		} else {
			font.Name = base + ".page5.3x22"
		}
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 5 頁缺字")
			}
		}
	}
	if err := storyTextCheck(text, segmentFonts{Full: font, Half: half}, 78); err != nil {
		return nil, fmt.Errorf("buckrogers: 第 5 頁：%w", err)
	}
	return &RuntimeStoryPage5Overlay{&xlate.Layer{W: 320, H: 200}, font, text, scale, false, half}, nil
}
func (o *RuntimeStoryPage5Overlay) Apply(es []StoryPage5Event, p [256][3]uint8) error {
	if o == nil || o.active || len(es) != 5 {
		return fmt.Errorf("buckrogers: 第 5 頁 apply 無效")
	}
	gen := es[0].Generation
	if gen == 0 {
		return fmt.Errorf("buckrogers: 第 5 頁 generation 無效")
	}
	seen := map[string]bool{}
	for i, e := range es {
		s := o.text[e.EventKey]
		if e.Generation != gen || seen[e.EventKey] || e.EventKey != fmt.Sprintf("story.page5.line.%03d", i+1) || s == "" || e.Row != uint8(17+i) || e.Column != 1 {
			return fmt.Errorf("buckrogers: 第 5 頁 event 無效")
		}
		seen[e.EventKey] = true
	}
	for _, e := range es {
		for _, stamp := range storyRowStamps(e.EventKey, 8, int(e.Row)*8, 39, []rune(o.text[e.EventKey]), o.storyFonts(), p[0], p[10]) {
			o.layer.Add(stamp)
		}
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryPage5Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 320, 176)
		o.active = false
	}
}
func (o *RuntimeStoryPage5Overlay) Frame(_ []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryPage5Overlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	d := o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss, d
}
func (o *RuntimeStoryPage5Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	return dedupRowKeys(o.layer.Stamps)
}
func (o *RuntimeStoryPage5Overlay) PresentationLayer() *xlate.Layer {
	if o == nil {
		return nil
	}
	return o.layer
}

// storyFonts returns the spec 039 segment fonts of this presenter.
func (o *RuntimeStoryPage5Overlay) storyFonts() segmentFonts {
	off := manualGlyphOffset(o.scale)
	return segmentFonts{Full: o.font, FullX: off, FullY: off, FullGlyphScale: 1, Half: o.half}
}

// storyFontsPlain is storyFonts for full cells drawn without an offset.
func (o *RuntimeStoryPage5Overlay) storyFontsPlain() segmentFonts {
	return segmentFonts{Full: o.font, Half: o.half}
}
