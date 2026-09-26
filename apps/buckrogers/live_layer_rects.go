package buckrogers

import "github.com/wicanr2/dosgolem/xlate"

// layerRects lists the logical 320×200 rectangles of a layer's stamps.
func layerRects(l *xlate.Layer) []PixelRect {
	if l == nil {
		return nil
	}
	out := make([]PixelRect, 0, len(l.Stamps))
	for _, s := range l.Stamps {
		x0, y0, x1, y1 := s.Rect()
		out = append(out, PixelRect{x0, y0, x1 - x0, y1 - y0})
	}
	return out
}

// LayerRects report where each presenter currently has stamps, so the live
// runtime can let the newest writer win over a stale menu stamp.
func (o *RuntimeSkillExitOverlay) LayerRects() []PixelRect          { return layerRects(o.layer) }
func (o *RuntimePostJoinExitPromptOverlay) LayerRects() []PixelRect { return layerRects(o.layer) }
func (o *RuntimePostJoinMenuOverlay) LayerRects() []PixelRect       { return layerRects(o.layer) }

// ClearRect removes the menu family's stamps inside a logical rectangle at
// every scale.  The live runtime calls it when another family has just drawn
// there: the original has overwritten those cells, but the menu layer only
// notices on its next frame-hash check.
func (r *LiveMenuRuntime) ClearRect(rect PixelRect) {
	for _, scale := range []int{2, 3} {
		r.presenters[scale].layer.Clear(rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height)
	}
}

// SafeLogicalRects lists the body-icon stamps in logical 320×200 pixels.
func (o *RuntimeBodyIconOverlay) SafeLogicalRects() []PixelRect {
	out := make([]PixelRect, 0, len(o.rects))
	for _, k := range o.ActiveKeys() {
		out = append(out, o.rects[k])
	}
	return out
}
