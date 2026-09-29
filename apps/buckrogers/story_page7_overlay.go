package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

type RuntimeStoryPage7Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
	half   *xlate.Font // spec 039 half font (8×16 at 2×, 12×24 at 3×)
}

func NewRuntimeStoryPage7Overlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryPage7Overlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 6 {
		return nil, fmt.Errorf("buckrogers: 第 7 頁 presenter 輸入無效")
	}
	half := halfFontsOf(font).For(scale)
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("buckrogers: 第 7 頁缺字")
			}
		}
	}
	if err := storyTextCheck(text, segmentFonts{Full: font, Half: half}, 78); err != nil {
		return nil, fmt.Errorf("buckrogers: 第 7 頁：%w", err)
	}
	return &RuntimeStoryPage7Overlay{&xlate.Layer{W: 320, H: 200}, font, text, scale, false, half}, nil
}
func (o *RuntimeStoryPage7Overlay) Apply(es []StoryPage7Event, p [256][3]uint8) error {
	if o == nil || o.active || len(es) != 6 {
		return fmt.Errorf("buckrogers: 第 7 頁 apply 無效")
	}
	g := es[0].Generation
	if g == 0 {
		return fmt.Errorf("buckrogers: 第 7 頁 generation 無效")
	}
	seen := map[string]bool{}
	for i, e := range es {
		k := fmt.Sprintf("story.page7.line.%03d", i+1)
		if e.Generation != g || e.EntryStep >= e.PostCallStep || (i > 0 && es[i-1].PostCallStep >= e.EntryStep) || seen[e.EventKey] || e.EventKey != k || e.Row != uint8(17+i) || e.Column != 1 || o.text[k] == "" {
			return fmt.Errorf("buckrogers: 第 7 頁 event 無效")
		}
		seen[k] = true
	}
	for _, e := range es {
		for _, stamp := range storyRowStamps(e.EventKey, 8, int(e.Row)*8, 39, []rune(o.text[e.EventKey]), o.storyFonts(), p[0], p[10]) {
			o.layer.Add(stamp)
		}
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryPage7Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 320, 184)
		o.active = false
	}
}
func (o *RuntimeStoryPage7Overlay) Frame(_ []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryPage7Overlay) Draw(i []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(i, p, o.scale)
	var m []rune
	d := o.layer.Draw(out, o.scale, func(r rune) { m = append(m, r) })
	return out, m, d
}
func (o *RuntimeStoryPage7Overlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	return dedupRowKeys(o.layer.Stamps)
}

// storyFonts returns the spec 039 segment fonts of this presenter.
func (o *RuntimeStoryPage7Overlay) storyFonts() segmentFonts {
	off := manualGlyphOffset(o.scale)
	return segmentFonts{Full: o.font, FullX: off, FullY: off, FullGlyphScale: 1, Half: o.half}
}

// storyFontsPlain is storyFonts for full cells drawn without an offset.
func (o *RuntimeStoryPage7Overlay) storyFontsPlain() segmentFonts {
	return segmentFonts{Full: o.font, Half: o.half}
}
