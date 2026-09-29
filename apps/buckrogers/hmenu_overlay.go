package buckrogers

import (
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/xlate"
)

// HMenuOverlay draws one HMenuPage at a fixed scale, one stamp per cell so
// each cell keeps its own colors.  Spec 039 §3.3: a half-width cell is 4
// logical pixels wide and uses the half font; X follows the row's unit
// positions.
type HMenuOverlay struct {
	layer *xlate.Layer
	fonts segmentFonts
	scale int
	gen   uint64
	page  *HMenuPage
	cells []HMenuCell // parallel to layer.Stamps
}

func NewHMenuOverlay(font *xlate.Font, scale int) (*HMenuOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: 水平選單 presenter 輸入無效")
	}
	full := font
	if scale == 3 {
		full = manualThreeXFont(font)
		full.Name = font.Name + ".hmenu.3x22"
	}
	return &HMenuOverlay{layer: &xlate.Layer{W: 320, H: 200}, fonts: familyFonts(font, full, scale), scale: scale}, nil
}

// Sync rebuilds after a generation change and reports runes the fonts lack.
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
			miss = append(miss, o.fonts.missingRunes([]rune{c.Rune})...)
		}
	}
	if len(miss) != 0 {
		return miss
	}
	o.page = p
	o.cells = o.cells[:0]
	for _, r := range p.Rows {
		for i, c := range r.Cells {
			o.cells = append(o.cells, c)
			s := &xlate.Stamp{
				Key: fmt.Sprintf("hmenu.%d.%d", r.Row, i), X: int(r.Col)*8 + r.UnitPos(i)*halfUnitPx, Y: int(r.Row) * 8,
				Cells: 1, CellH: 8, Text: []rune{c.Rune}, State: xlate.Shown, BG: palette[c.BG], FG: palette[c.FG],
			}
			if isHalfwidth(c.Rune) {
				s.CellW, s.Font, s.GlyphScale = halfUnitPx, o.fonts.Half, 1
			} else {
				s.CellW, s.Font, s.GlyphX, s.GlyphY, s.GlyphScale = 8, o.fonts.Full, o.fonts.FullX, o.fonts.FullY, 1
			}
			o.layer.Stamps = append(o.layer.Stamps, s)
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

// ActiveKeys lists the stamp keys (one per cell).
func (o *HMenuOverlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	k := dedupRowKeys(o.layer.Stamps)
	sort.Strings(k)
	return k
}

func (o *HMenuOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
