package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// HMenuOverlay draws one HMenuPage at a fixed scale, one stamp per cell so
// each cell keeps its own colors.
type HMenuOverlay struct {
	layer *xlate.Layer
	font  *xlate.Font
	scale int
	gen   uint64
	page  *HMenuPage
	cells []HMenuCell // parallel to layer.Stamps
}

func NewHMenuOverlay(font *xlate.Font, scale int) (*HMenuOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: 水平選單 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		font.Name = base + ".hmenu.3x22"
	}
	return &HMenuOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, scale: scale}, nil
}

// Sync rebuilds after a generation change and reports runes the font lacks.
func (o *HMenuOverlay) Sync(p *HMenuPage, gen uint64, palette [256][3]uint8) []rune {
	if o == nil || gen == o.gen {
		return nil
	}
	o.gen = gen
	o.layer.Stamps = nil
	o.page = nil
	if p == nil {
		return nil
	}
	var miss []rune
	for _, r := range p.Rows {
		for _, c := range r.Cells {
			if _, ok := o.font.Glyphs[c.Rune]; !ok && c.Rune != ' ' {
				miss = append(miss, c.Rune)
			}
		}
	}
	if len(miss) != 0 {
		return miss
	}
	o.page = p
	off := manualGlyphOffset(o.scale)
	o.cells = o.cells[:0]
	for _, r := range p.Rows {
		for i, c := range r.Cells {
			o.cells = append(o.cells, c)
			o.layer.Stamps = append(o.layer.Stamps, &xlate.Stamp{
				Key: fmt.Sprintf("hmenu.%d.%d", r.Row, i), X: (int(r.Col) + i) * 8, Y: int(r.Row) * 8,
				Cells: 1, CellW: 8, CellH: 8, Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1,
				Text: []rune{c.Rune}, State: xlate.Shown, BG: palette[c.BG], FG: palette[c.FG],
			})
		}
	}
	return nil
}

func (o *HMenuOverlay) Frame(palette [256][3]uint8) {
	if o == nil || o.page == nil {
		return
	}
	for i, s := range o.layer.Stamps {
		c := o.cells[i]
		s.BG, s.FG = palette[c.BG], palette[c.FG]
	}
}

func (o *HMenuOverlay) Active() bool { return o != nil && len(o.layer.Stamps) != 0 }

func (o *HMenuOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
