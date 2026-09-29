package buckrogers

import "github.com/wicanr2/dosgolem/xlate"

// Spec 039 §3.3 (Buck repo): the row-group rule for families whose stamps
// are invalidated by Layer.Frame fingerprints and the spec 239 rewrite test
// (menus, and the manual paragraph without a watcher style).  All segment
// stamps of one original row form a group.  When any of them is removed,
// any of their cells turns Transparent, or a new fingerprint failure hits
// an anchor cell while the group has at most two valid anchor cells left,
// the whole row rectangle is cleared.  xlate itself is unchanged: 239 still
// runs per segment, this layer only widens its result to the row.

// rowGroup is one original row drawn as segment stamps.
type rowGroup struct {
	key            string
	x0, y0, x1, y1 int // logical rectangle cleared when the group fails
	segments       int
	// anchors marks the 8×8 cells (from x0) that covered original ink when
	// the group was coloured; nil until then (and after a Restore).
	anchors []bool
}

// rowGroupClear is one group clear, for receipts and tests.
type rowGroupClear struct {
	Key, Reason string // reason: drop, transparent, anchors
}

// rowGroupSet holds the groups of one layer in insertion order.
type rowGroupSet struct {
	groups  []*rowGroup
	cleared []rowGroupClear
}

// add registers a group whose segment stamps have just been added.
func (g *rowGroupSet) add(key string, x0, y0, x1, y1, segments int) {
	g.remove(key)
	g.groups = append(g.groups, &rowGroup{key: key, x0: x0, y0: y0, x1: x1, y1: y1, segments: segments})
}

func (g *rowGroupSet) remove(key string) {
	keep := g.groups[:0]
	for _, gr := range g.groups {
		if gr.key != key {
			keep = append(keep, gr)
		}
	}
	g.groups = keep
}

// reset forgets every group (the layer was replaced).
func (g *rowGroupSet) reset() { g.groups = nil }

// rebuild derives the groups from a restored layer: one group per row key,
// its rectangle the union of its segments.  Anchors are unknown until the
// next colouring, so trigger 3 stays off for restored groups.
func (g *rowGroupSet) rebuild(layer *xlate.Layer) {
	g.groups = nil
	index := map[string]*rowGroup{}
	for _, s := range layer.Stamps {
		x0, y0, x1, y1 := s.Rect()
		k := rowKeyOf(s.Key)
		gr, ok := index[k]
		if !ok {
			gr = &rowGroup{key: k, x0: x0, y0: y0, x1: x1, y1: y1}
			index[k] = gr
			g.groups = append(g.groups, gr)
		}
		gr.segments++
		gr.x0, gr.y0 = min(gr.x0, x0), min(gr.y0, y0)
		gr.x1, gr.y1 = max(gr.x1, x1), max(gr.y1, y1)
	}
}

// segmentsOf lists the layer's stamps of group key.
func segmentsOf(layer *xlate.Layer, key string) []*xlate.Stamp {
	var out []*xlate.Stamp
	for _, s := range layer.Stamps {
		if rowKeyOf(s.Key) == key {
			out = append(out, s)
		}
	}
	return out
}

func anyTransparent(s *xlate.Stamp) bool {
	for i := 0; i < s.Cells && i < len(s.Transparent); i++ {
		if s.Transparent[i] {
			return true
		}
	}
	return false
}

// reconcile applies triggers 1 and 2 (and trigger 3 for the keys in
// flagged) until no group fails.  Call it after every Frame and every Clear
// entry of the owning presenter.
func (g *rowGroupSet) reconcile(layer *xlate.Layer, flagged map[string]bool) {
	for changed := true; changed; {
		changed = false
		for _, gr := range g.groups {
			segs := segmentsOf(layer, gr.key)
			reason := ""
			switch {
			case len(segs) < gr.segments:
				reason = "drop"
			case flagged[gr.key]:
				reason = "anchors"
			default:
				for _, s := range segs {
					if anyTransparent(s) {
						reason = "transparent"
						break
					}
				}
			}
			if reason == "" {
				continue
			}
			layer.Clear(gr.x0, gr.y0, gr.x1, gr.y1)
			g.remove(gr.key)
			g.cleared = append(g.cleared, rowGroupClear{Key: gr.key, Reason: reason})
			changed = true
			break
		}
	}
}

// cellMultiColour reports whether the logical rectangle holds more than
// one palette index (an anchor cell, spec 202 §2.3).
func cellMultiColour(indexed []byte, w, h, x0, y0, x1, y1 int) bool {
	first := -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || x >= w || y < 0 || y >= h {
				continue
			}
			v := int(indexed[y*w+x])
			if first < 0 {
				first = v
			} else if v != first {
				return true
			}
		}
	}
	return false
}

// frame runs Layer.Frame and then the group rules: a group whose segments
// were Pending and are now all Shown gets one colour pair computed over
// the whole row (§3.3 Frame 定色同列共用) and its anchor cells; a shown
// group gets trigger 3; then reconcile runs.
func (g *rowGroupSet) frame(layer *xlate.Layer, indexed []byte, palette [256][3]uint8) {
	type before struct {
		pending bool
		transp  map[*xlate.Stamp][]bool
	}
	w, h := layer.W, layer.H
	if w == 0 {
		w = 320
	}
	if h == 0 {
		h = 200
	}
	states := make(map[string]before, len(g.groups))
	for _, gr := range g.groups {
		b := before{transp: map[*xlate.Stamp][]bool{}}
		for _, s := range segmentsOf(layer, gr.key) {
			if s.State == xlate.Pending {
				b.pending = true
			}
			t := make([]bool, s.Cells)
			for i := range t {
				t[i] = i < len(s.Transparent) && s.Transparent[i]
			}
			b.transp[s] = t
		}
		states[gr.key] = b
	}
	layer.Frame(indexed, logicalRGB(indexed, palette))
	flagged := map[string]bool{}
	for _, gr := range g.groups {
		segs := segmentsOf(layer, gr.key)
		b := states[gr.key]
		if len(segs) != gr.segments {
			continue // trigger 1 in reconcile
		}
		allShown := true
		for _, s := range segs {
			allShown = allShown && s.State == xlate.Shown
		}
		if b.pending && allShown {
			g.colour(gr, segs, indexed, w, h, palette)
			continue
		}
		if gr.anchors == nil {
			continue
		}
		fresh := false
		for _, s := range segs {
			old := b.transp[s]
			for i := 0; i < s.Cells && i < len(s.Transparent); i++ {
				if !s.Transparent[i] || i < len(old) && old[i] {
					continue
				}
				if c := (s.X + i*s.CellW - gr.x0) / 8; c >= 0 && c < len(gr.anchors) && gr.anchors[c] {
					fresh = true
				}
			}
		}
		if !fresh {
			continue
		}
		// Valid anchor cells: anchor 8×8 cells none of whose sub-cells is
		// transparent.
		bad := make([]bool, len(gr.anchors))
		for _, s := range segs {
			for i := 0; i < s.Cells; i++ {
				if i < len(s.Transparent) && s.Transparent[i] {
					if c := (s.X + i*s.CellW - gr.x0) / 8; c >= 0 && c < len(bad) {
						bad[c] = true
					}
				}
			}
		}
		valid := 0
		for c, a := range gr.anchors {
			if a && !bad[c] {
				valid++
			}
		}
		if valid <= 2 {
			flagged[gr.key] = true
		}
	}
	g.reconcile(layer, flagged)
}

// colour gives every segment of gr the colour pair of the whole row and
// records the group's anchor cells, both from the frame that showed it.
func (g *rowGroupSet) colour(gr *rowGroup, segs []*xlate.Stamp, indexed []byte, w, h int, palette [256][3]uint8) {
	var region []uint8
	cells := (gr.x1 - gr.x0 + 7) / 8
	gr.anchors = make([]bool, cells)
	for _, s := range segs {
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			x0 := s.X + i*s.CellW
			for y := s.Y; y < s.Y+s.CellH; y++ {
				for x := x0; x < x0+s.CellW; x++ {
					if x >= 0 && x < w && y >= 0 && y < h {
						region = append(region, indexed[y*w+x])
					}
				}
			}
			if cellMultiColour(indexed, w, h, x0, s.Y, x0+s.CellW, s.Y+s.CellH) {
				if c := (x0 - gr.x0) / 8; c >= 0 && c < cells {
					gr.anchors[c] = true
				}
			}
		}
	}
	bg, fg := xlate.Colors(region)
	swap := len(segs) > 0 && segs[0].SwapColors
	if swap {
		bg, fg = fg, bg
	}
	for _, s := range segs {
		s.BG, s.FG = palette[bg], palette[fg]
	}
}
