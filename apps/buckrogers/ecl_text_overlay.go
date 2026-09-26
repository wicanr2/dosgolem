package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// EclTextOverlay draws one EclTextPage at a fixed scale.  It owns no DOS
// state: the page is rebuilt whenever the watcher generation changes.
type EclTextOverlay struct {
	layer *xlate.Layer
	font  *xlate.Font
	scale int
	gen   uint64
	cols  [][2]uint8 // background, foreground per stamp
}

func NewEclTextOverlay(font *xlate.Font, scale int) (*EclTextOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: ECL 敘事 presenter 輸入無效")
	}
	if scale == 3 {
		base := font.Name
		font = manualThreeXFont(font)
		font.Name = base + ".ecl.3x22"
	}
	return &EclTextOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, scale: scale}, nil
}

// Sync rebuilds the stamps when the watcher moved to a new generation.
// It reports the runes the font lacks; then nothing is drawn.
func (o *EclTextOverlay) Sync(pages []*EclTextPage, gen uint64, palette [256][3]uint8) []rune {
	if o == nil || gen == o.gen {
		return nil
	}
	o.gen = gen
	o.layer.Stamps = nil
	o.cols = nil
	var miss []rune
	for _, p := range pages {
		for _, l := range p.Lines {
			for _, r := range l.Text {
				if _, ok := o.font.Glyphs[r]; !ok && r != ' ' {
					miss = append(miss, r)
				}
			}
		}
	}
	if len(miss) != 0 {
		return miss
	}
	off := manualGlyphOffset(o.scale)
	for pi, p := range pages {
		cells := int(p.Right) - int(p.Left) + 1
		for row := p.Top; row <= p.Bottom; row++ {
			if !p.Shows(row) {
				continue
			}
			text := make([]rune, cells)
			for i := range text {
				text[i] = ' '
			}
			for _, l := range p.Lines {
				if l.Row == row {
					copy(text[int(l.Col)-int(p.Left):], l.Text)
				}
			}
			o.layer.Stamps = append(o.layer.Stamps, &xlate.Stamp{
				Key: fmt.Sprintf("ecl.%d.row.%d", pi, row), X: int(p.Left) * 8, Y: int(row) * 8,
				Cells: cells, CellW: 8, CellH: 8, Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1,
				Text: text, State: xlate.Shown, BG: palette[p.Background], FG: palette[p.Foreground],
			})
			o.cols = append(o.cols, [2]uint8{p.Background, p.Foreground})
		}
	}
	return nil
}

// Frame follows palette changes (fades) without touching lifetime.
func (o *EclTextOverlay) Frame(palette [256][3]uint8) {
	if o == nil {
		return
	}
	for i, s := range o.layer.Stamps {
		s.BG, s.FG = palette[o.cols[i][0]], palette[o.cols[i][1]]
	}
}

func (o *EclTextOverlay) Active() bool { return o != nil && len(o.layer.Stamps) != 0 }

func (o *EclTextOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
