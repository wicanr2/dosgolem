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
	base  *xlate.Font // the 16×16 font the shrink fonts of spec 056 derive from
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
	return &EclTextOverlay{layer: &xlate.Layer{W: 320, H: 200}, fonts: familyFonts(font, full, scale), base: font, scale: scale}, nil
}

// eclRowText assembles one masked row from start (cell) to right (cell)
// out of the page lines on that row (columns in half units), padded per
// spec 039 §3.1.  Later lines overwrite earlier ones, as before.  A shrunk
// name unit (spec 056) is not part of the text: its slots are blank here.
func eclRowText(p *EclTextPage, row, start uint8) []rune {
	t, _ := eclRowTextShrink(p, row, start)
	return t
}

// eclRowTextShrink is eclRowText that also returns the shrunk name units that
// survive on the row, in order (spec 056 §3.6).  A unit takes W_s slots and
// takes part in the same last-writer-wins flow: the full-width halves it cuts
// are cleared as for any text, a later line that overwrites one of its slots
// removes the whole unit, and a unit that does not lie inside [start, right]
// is not drawn and writes no slot.
func eclRowTextShrink(p *EclTextPage, row, start uint8) ([]rune, []EclTextLine) {
	from, to := int(eclUnitLeft(start)), int(eclUnitRight(p.Right))+1
	slots := make([]rune, to-from)   // 0 empty, -1 second unit of a full-width rune
	owner := make([]int, len(slots)) // 1 + the index of the shrunk unit that holds the slot, 0 none
	var units []EclTextLine
	var alive []bool
	kill := func(k int) {
		if !alive[k] {
			return
		}
		alive[k] = false
		for i := range owner {
			if owner[i] == k+1 {
				owner[i] = 0
			}
		}
	}
	cut := func(k int) {
		if owner[k] != 0 {
			kill(owner[k] - 1)
		}
		if slots[k] == -1 && k > 0 {
			slots[k-1] = 0
		}
		if slots[k] > 0 && runeUnits(slots[k]) == 2 && k+1 < len(slots) {
			slots[k+1] = 0
		}
	}
	for _, l := range p.Lines {
		if l.Row != row {
			continue
		}
		u := int(l.Col) - from
		if l.Shrink != 0 {
			spec, ok := shrinkSpecFor(int(l.Shrink))
			if !ok {
				continue
			}
			w := shrinkUnitUnits(l.Text, spec)
			if u < 0 || u+w > len(slots) {
				continue
			}
			for k := u; k < u+w; k++ {
				cut(k)
				slots[k] = 0
			}
			units = append(units, l)
			alive = append(alive, true)
			for k := u; k < u+w; k++ {
				owner[k] = len(units)
			}
			continue
		}
		for _, r := range l.Text {
			w := runeUnits(r)
			if u < 0 || u+w > len(slots) {
				u += w
				continue
			}
			for k := u; k < u+w; k++ {
				cut(k)
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
	var keep []EclTextLine
	for k, l := range units {
		if alive[k] {
			keep = append(keep, l)
		}
	}
	return out, keep
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
			if l.Shrink != 0 {
				if spec, ok := shrinkSpecFor(int(l.Shrink)); ok {
					miss = append(miss, missingShrinkRunes(shrinkFontsOf(o.base, spec, o.scale), l.Text)...)
				}
				continue
			}
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
			rowKey := fmt.Sprintf("ecl.%d.row.%d", pi, row)
			text, shrunk := eclRowTextShrink(p, row, start)
			stamps := o.fonts.segmentStamps(rowKey, int(start)*8, int(row)*8, text, nil, func(int) ([3]uint8, [3]uint8) { return bg, fg })
			// Spec 056 §3.6: the shrunk units come after the normal segments,
			// so the background fill of those does not cover them.
			for _, l := range shrunk {
				if spec, ok := shrinkSpecFor(int(l.Shrink)); ok {
					stamps = append(stamps, shrinkStamps(rowKey, len(stamps), int(l.Col), int(row)*8, l.Text, spec, o.scale,
						shrinkFontsOf(o.base, spec, o.scale), bg, fg)...)
				}
			}
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
