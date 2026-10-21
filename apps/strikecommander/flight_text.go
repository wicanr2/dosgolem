package strikecommander

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 飛行字串（HUD、MFD、訊息橫幅）的反鋸齒新路徑（strike_commander 規格 011 §3.6.4 到 §3.6.6；分期 (e)）。
// FlightOverlay.Text 非 nil 時一律走新路徑。

// NewFlightOverlayText 建一個走新路徑的飛行 overlay，不需要舊字型（011 §3.9）。scale 1 到 3；1 不畫，2 不畫 HUD。
func NewFlightOverlayText(fonts *TextFonts, cats FlightCatalogs, scale int) (*FlightOverlay, error) {
	if scale < 1 || scale > 3 {
		return nil, fmt.Errorf("strikecommander: 飛行 overlay 倍率 %d 不支援", scale)
	}
	if fonts == nil {
		return nil, fmt.Errorf("strikecommander: 飛行 overlay 缺圖集")
	}
	return &FlightOverlay{Text: fonts, cats: cats, scale: scale, HUDScaleOff: scale != 3, masks: map[string][]bool{}}, nil
}

// flightTextRange 回 011 §3.1 表的英文範圍 [e0, e1)、可用範圍 [a0, a1)、邊距 m 與是否畫外框；E′（換回範圍）同英文範圍。
func flightTextRange(t FlightText) (e0, e1, a0, a1, m int, outline bool, why string) {
	switch t.Kind {
	case KindHUD:
		l, r, ok := hudRange(t)
		if !ok {
			return 0, 0, 0, 0, 0, false, fmt.Sprintf("HUD catalog key %q 沒有版面範圍", t.CatKey)
		}
		return t.Anchor(), t.X1, l, r + 1, 1, true, ""
	case KindMFD:
		a0, a1 := 14, 101
		if t.X0 >= 160 {
			a0, a1 = 219, 306
		}
		if t.K > 0 {
			a0 = t.Anchor() // 011 §3.1：k > 0 的左界取 Xs[k]，數字前綴不會被中文蓋到
		}
		return t.Anchor(), t.X1, a0, a1, 0, false, ""
	case KindPlaque:
		ix0, ix1, ok := inkXRange(t)
		if !ok {
			return 0, 0, 0, 0, 0, false, "橫幅 band 內沒有墨跡"
		}
		return ix0, ix1 + 1, ix0, ix1 + 1, 0, false, ""
	}
	return 0, 0, 0, 0, 0, false, fmt.Sprintf("種類 %q 不是飛行種類", t.Kind)
}

// flightPx 回飛行字串的字級（011 §3.1）。
func flightPx(k Kind, scale int) int {
	switch k {
	case KindHUD:
		return pxHUD[scale]
	case KindMFD:
		return pxMFD[scale]
	}
	return pxPlaque[scale]
}

// planText 是 plan 的新路徑（011 §3.5、§3.6.4 到 §3.6.6）。
func (o *FlightOverlay) planText(i int, t FlightText) (flightEntry, *Miss, *Skip) {
	if o.scale < 2 || (t.Kind == KindHUD && o.scale != 3) {
		return flightEntry{}, nil, errNoStamp
	}
	tr, ok := o.cats.Table(t.Kind)[t.CatKey]
	if !ok || tr == "" {
		return flightEntry{}, &Miss{Key: t.Key, Reason: MissTranslation}, nil
	}
	text := []rune(tr)
	sc := o.scale
	px := flightPx(t.Kind, sc)
	f := o.Text.Font(px)
	if f == nil {
		return flightEntry{}, nil, &Skip{t.Key, fmt.Sprintf("圖集缺字級 %d", px)}
	}
	disp := NarrowAlnum(text)
	w, miss := f.Advance(disp)
	if len(miss) != 0 {
		return flightEntry{}, &Miss{Key: t.Key, Reason: MissGlyph, Runes: miss}, nil
	}
	if !validRows(t.Before, t.BH) || !validRows(t.After, t.BH) {
		return flightEntry{}, nil, &Skip{t.Key, "前後列缺或長度不符"}
	}
	e0, e1, a0, a1, m, outline, why := flightTextRange(t)
	if why != "" {
		return flightEntry{}, nil, &Skip{t.Key, why}
	}
	// 字色與無墨跡判定在 E′ 內（011 §3.5）。
	fg, n := inkMode(t, e0, e1)
	if n == 0 {
		return flightEntry{}, nil, &Skip{t.Key, "遮蓋範圍內沒有墨跡"}
	}
	tx, err := textHorizontal(e0, e1, a0, a1, sc, w, m)
	if err != nil {
		return flightEntry{}, nil, &Skip{t.Key, err.Error()}
	}
	b0, b1 := t.B0, t.B0+t.BH
	_, baseline, err := textVertical(b0, b1, sc, px)
	if err != nil {
		return flightEntry{}, nil, &Skip{t.Key, err.Error()}
	}
	clip := xlate.AlphaRect{X0: max(a0*sc, 0), Y0: max((b0-1)*sc, 0), X1: min(a1*sc, 320*sc), Y1: min((b1+1)*sc, 200*sc)}
	box := textBox(f, disp, tx, baseline, clip, outline)
	cx0, cx1 := e0, e1
	if !box.Empty() {
		cx0, cx1 = min(cx0, floorDiv(box.X0, sc)), max(cx1, ceilDiv(box.X1, sc))
	}
	cx0, cx1 = max(cx0, 0), min(cx1, 320)
	e := flightEntry{idx: i, t: t, text: text, cx0: cx0, cx1: cx1, fg: fg, outline: outline}
	e.cover = &xlate.Stamp{Key: t.Key + ".cover", X: cx0, Y: t.B0, Cells: 1, CellW: cx1 - cx0, CellH: t.BH, State: xlate.Shown}
	e.ti = &textItem{raw: append([]rune(nil), text...), display: disp, font: f, px: px, tx: tx, baseline: baseline, clip: clip,
		box: box, outline: outline, outlineIdx: bgIndex, fgIdx: fg, e0: e0, e1: e1}
	return e, nil, nil
}

// drawText 是 Draw 的新路徑（011 §3.9）：照畫串的遮蓋先以背景圖換回 → 照畫 HUD 串的外框層 → 照畫串的填色層。
func (o *FlightOverlay) drawText(out []byte, indexed []byte, palette [256][3]uint8, bad []bool) {
	o.lastInk = make([]xlate.AlphaRect, len(o.entries))
	var bitems []bgItem
	layer := &xlate.Layer{W: 320, H: 200}
	items := make([]*textItem, len(o.entries))
	for i := range o.entries {
		e := &o.entries[i]
		if bad[i] {
			o.Hidden = append(o.Hidden, e.t.Key)
			continue
		}
		bitems = append(bitems, bgItem{label: e.idx, key: e.t.Key, kind: e.t.Kind,
			x0: e.cx0, x1: e.cx1, y0: e.t.B0, rows: e.t.BH, before: e.t.Before, after: e.t.After})
		e.cover.BG, e.cover.FG = palette[bgIndex], palette[e.fg]
		layer.Stamps = append(layer.Stamps, e.cover)
		items[i] = e.ti
	}
	if len(bitems) == 0 {
		return
	}
	bg := labelBackground(out, indexed, palette, o.scale, bitems, nil)
	layer.DrawWithBackground(out, o.scale, func(rune) {}, bg, func(*xlate.Stamp) bool { return true })
	o.lastInk = drawTextItems(out, o.scale, palette, items)
}

// TextBoxes 回最近一次 Draw 照畫的新路徑串的文字框（輸出像素）；不含 Hidden、miss、Skip（011 §3.9）。
func (o *FlightOverlay) TextBoxes() []xlate.AlphaRect {
	var out []xlate.AlphaRect
	for _, b := range o.lastInk {
		if !b.Empty() {
			out = append(out, b)
		}
	}
	return out
}

// commitStamps 回升格時的 CommitStamps（011 §3.9）：每筆 2 個，Texts 是 N() 前的譯文。
func (o *FlightOverlay) commitStamps(sc int) CommitStamps {
	var items []commitItem
	for _, e := range o.entries {
		items = append(items, commitItem{string(e.text), e.t.Kind})
	}
	return commitStamps(sc, items)
}
