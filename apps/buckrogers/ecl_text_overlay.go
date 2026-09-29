package buckrogers

import (
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/xlate"
)

// EclTextOverlay draws one EclTextPage at a fixed scale.  It owns no DOS
// state: the page is rebuilt whenever the watcher generation changes.
// Rows are drawn as spec 039 width segments (half and full width).
type EclTextOverlay struct {
	layer *xlate.Layer
	fonts segmentFonts
	scale int
	gen   uint64
	cols  [][2]uint8 // background, foreground per stamp
}

func NewEclTextOverlay(font *xlate.Font, scale int) (*EclTextOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: ECL 敘事 presenter 輸入無效")
	}
	full := font
	if scale == 3 {
		full = manualThreeXFont(font)
		full.Name = font.Name + ".ecl.3x22"
	}
	return &EclTextOverlay{layer: &xlate.Layer{W: 320, H: 200}, fonts: familyFonts(font, full, scale), scale: scale}, nil
}

// eclRowText assembles one masked row from start (cell) to right (cell)
// out of the page lines on that row (columns in half units), padded per
// spec 039 §3.1.  Later lines overwrite earlier ones, as before.
func eclRowText(p *EclTextPage, row, start uint8) []rune {
	from, to := int(eclUnitLeft(start)), int(eclUnitRight(p.Right))+1
	slots := make([]rune, to-from) // 0 empty, -1 second unit of a full-width rune
	for _, l := range p.Lines {
		if l.Row != row {
			continue
		}
		u := int(l.Col) - from
		for _, r := range l.Text {
			w := runeUnits(r)
			if u < 0 || u+w > len(slots) {
				u += w
				continue
			}
			for k := u; k < u+w; k++ {
				if slots[k] == -1 && k > 0 {
					slots[k-1] = 0
				}
				if slots[k] > 0 && runeUnits(slots[k]) == 2 && k+1 < len(slots) {
					slots[k+1] = 0
				}
			}
			slots[u] = r
			if w == 2 {
				slots[u+1] = -1
			}
			u += w
		}
	}
	var out []rune
	for u := 0; u < len(slots); {
		if slots[u] > 0 {
			out = append(out, slots[u])
			u += runeUnits(slots[u])
			continue
		}
		e := u
		for e < len(slots) && slots[e] <= 0 {
			e++
		}
		out = appendPadding(out, u, e)
		u = e
	}
	return out
}

// Sync rebuilds the stamps when the watcher moved to a new generation.
// It reports the runes the fonts lack; then nothing is drawn.
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
			miss = append(miss, o.fonts.missingRunes(l.Text)...)
		}
	}
	if len(miss) != 0 {
		return miss
	}
	for pi, p := range pages {
		for row := p.Top; row <= p.Bottom; row++ {
			if !p.Shows(row) {
				continue
			}
			start := p.Left
			if row == p.Top {
				start = p.TopCol
			}
			bg, fg := palette[p.Background], palette[p.Foreground]
			stamps := o.fonts.segmentStamps(fmt.Sprintf("ecl.%d.row.%d", pi, row), int(start)*8, int(row)*8,
				eclRowText(p, row, start), nil, func(int) ([3]uint8, [3]uint8) { return bg, fg })
			for _, s := range stamps {
				o.layer.Stamps = append(o.layer.Stamps, s)
				o.cols = append(o.cols, [2]uint8{p.Background, p.Foreground})
			}
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

// ActiveKeys lists the row keys drawn (segments counted once, spec 039 §3.3).
func (o *EclTextOverlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	k := dedupRowKeys(o.layer.Stamps)
	sort.Strings(k)
	return k
}

func (o *EclTextOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
}
