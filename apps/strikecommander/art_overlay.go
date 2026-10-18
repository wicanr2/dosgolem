package strikecommander

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 存讀檔對話框的圖面（strike_commander 規格 004 §3.3 到 §3.5）：每個繪圖事件一列 204 圖面，
// 放在 Overlay 自有的獨立 artLayer。Reference 是解碼的原版色號（不從畫面取樣），Owner 空字串，
// 顯示由 204 的逐格比對決定。

// ArtRect 是一列圖面的 Key 與字帶矩形（1×，[X, X+W) × [Y, Y+H)）。
type ArtRect struct {
	Key        string
	X, Y, W, H int
}

// artEntry 是一列圖面的規劃：事件、字帶矩形、原版基準與輸出解析度的色號圖（無字底放大後再畫中文）。
// Pix 依色盤在 Draw 時建（事件當步沒有色盤），色盤或倍率改變時重建。
type artEntry struct {
	ev    ArtEvent
	rect  ArtRect
	ref   []uint8 // 字帶內的原版色號，逐列
	idx   []uint8 // (W×scale)×(H×scale) 輸出像素的色號
	built bool    // 已以 artPal 建 Pix 並登記到 artLayer
}

// artKey 回 004 §3.3 的 Key：saveload/<元素 x>,<元素 y>/<一般態 k>。
func artKey(ev ArtEvent) string {
	return fmt.Sprintf("saveload/%d,%d/%d", ev.X, ev.Y, normalK(ev.K))
}

// AddDialogArt 依一個繪圖事件規劃一列圖面（004 §3.3）。同 Key 的後畫替換前畫。
// 譯文缺 key、字型缺字記 Miss；超寬、倍率、矩形不合法記 Skip；兩者都不產生圖面，畫面維持英文。
func (o *Overlay) AddDialogArt(ev ArtEvent) {
	e, miss, skip := o.planArt(ev)
	if miss != nil {
		o.Misses = append(o.Misses, *miss)
		return
	}
	if skip != nil {
		o.Skips = append(o.Skips, *skip)
		return
	}
	for i, old := range o.arts {
		if old.rect.Key == e.rect.Key {
			o.arts[i] = e
			return
		}
	}
	o.arts = append(o.arts, e)
}

// ClearArt 移除全部圖面（全螢幕 blit 入口，004 §3.3）。
func (o *Overlay) ClearArt() {
	o.arts = nil
	o.artLayer.ClearArt()
}

// Art 回目前圖面的 Key 與字帶矩形，依加入順序。
func (o *Overlay) Art() []ArtRect {
	out := make([]ArtRect, 0, len(o.arts))
	for _, e := range o.arts {
		out = append(out, e.rect)
	}
	return out
}

// ArtKeys 回目前圖面的 Key 清單。
func (o *Overlay) ArtKeys() []string {
	var out []string
	for _, e := range o.arts {
		out = append(out, e.rect.Key)
	}
	return out
}

// replanArt 在倍率改變後依保留的事件重新規劃（不再記 Miss、Skip；規劃失敗的列移除）。
func (o *Overlay) replanArt() {
	evs := o.arts
	o.arts = nil
	o.artLayer.ClearArt()
	for _, old := range evs {
		if e, miss, skip := o.planArt(old.ev); miss == nil && skip == nil {
			o.arts = append(o.arts, e)
		}
	}
}

// planArt 依 004 §3.3 到 §3.5 規劃一列圖面。
func (o *Overlay) planArt(ev ArtEvent) (*artEntry, *Miss, *Skip) {
	el, ink, ok := elementFor(ev.K)
	if !ok {
		return nil, nil, &Skip{fmt.Sprintf("SAVELOAD k%d", ev.K), "不是有字的元素"}
	}
	if o.scale != 2 && o.scale != 3 {
		return nil, nil, &Skip{el.key, fmt.Sprintf("%d 倍輸出不覆繪", o.scale)}
	}
	if ev.W != el.w || ev.H != el.h || len(ev.Entry) != el.w*el.h || len(ev.Base0) != el.w*el.h {
		return nil, nil, &Skip{el.key, fmt.Sprintf("條目尺寸 %d×%d 不是 %d×%d", ev.W, ev.H, el.w, el.h)}
	}
	tr, ok := o.gfx[el.key]
	if !ok || tr == "" {
		return nil, &Miss{Key: el.key, Reason: MissTranslation}, nil
	}
	text := []rune(tr)
	var missing []rune
	for _, r := range text {
		if !o.font.Wide.Wide(r) || o.font.Font.Glyphs[r] == nil {
			missing = append(missing, r)
		}
	}
	if len(missing) != 0 {
		return nil, &Miss{Key: el.key, Reason: MissGlyph, Runes: missing}, nil
	}
	sc := o.scale
	gx, gy, step, err := glyphOrigin(len(text), sc, el.band, ink, pressedState(ev.K))
	if err != nil {
		return nil, nil, &Skip{el.key, err.Error()}
	}
	bw, bh := el.band.x1-el.band.x0+1, el.band.y1-el.band.y0+1
	rect := ArtRect{Key: artKey(ev), X: ev.X + el.band.x0, Y: ev.Y + el.band.y0, W: bw, H: bh}
	if rect.X < 0 || rect.Y < 0 || rect.X+bw > 320 || rect.Y+bh > 200 {
		return nil, nil, &Skip{el.key, fmt.Sprintf("字帶 (%d,%d) %d×%d 超出畫面", rect.X, rect.Y, bw, bh)}
	}
	ref := make([]uint8, bw*bh)
	for y := 0; y < bh; y++ {
		copy(ref[y*bw:(y+1)*bw], ev.Entry[(el.band.y0+y)*el.w+el.band.x0:])
	}
	c, h := dialogColors(ev)
	bg := dialogBackground(ev)
	ow, oh := bw*sc, bh*sc
	idx := make([]uint8, ow*oh)
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			idx[y*ow+x] = bg[(y/sc)*bw+x/sc]
		}
	}
	font := o.font.Font
	rb := (font.W + 7) / 8
	plot := func(x, y int, col uint8) {
		if x >= 0 && x < ow && y >= 0 && y < oh {
			idx[y*ow+x] = col
		}
	}
	// 高光在筆畫左側：先以 h 在字模左移 1 個輸出像素處畫一次，再以 c 畫字模（004 §3.5）。
	for pass, col := range []uint8{h, c} {
		dx := pass - 1 // 第一趟 −1，第二趟 0
		for i, r := range text {
			g := font.Glyphs[r]
			ox := gx + i*step + dx
			for py := 0; py < font.H; py++ {
				for px := 0; px < font.W; px++ {
					if g[py*rb+px/8]&(0x80>>uint(px%8)) != 0 {
						plot(ox+px, gy+py, col)
					}
				}
			}
		}
	}
	return &artEntry{ev: ev, rect: rect, ref: ref, idx: idx}, nil, nil
}

// stamp 以色盤 pal 建這一列的 204 圖面（CellW 1，每欄一格，004 §3.3）。
func (e *artEntry) stamp(pal [256][3]uint8, scale int) *xlate.Stamp {
	pix := make([]uint8, 4*len(e.idx))
	for i, c := range e.idx {
		p := pal[c]
		pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = p[0], p[1], p[2], 255
	}
	return &xlate.Stamp{
		Key: e.rect.Key, Art: true, X: e.rect.X, Y: e.rect.Y,
		Cells: e.rect.W, CellW: 1, CellH: e.rect.H,
		Reference: e.ref, Pix: pix, PixScale: scale, Order: e.ev.Order,
	}
}

// drawArt 是 Draw 的第 2 步：以當次色盤補建 Pix、以畫格逐格比對，再畫到 out。
func (o *Overlay) drawArt(out, indexed []byte, palette [256][3]uint8) {
	if len(o.arts) == 0 {
		return
	}
	if !o.artPalOK || o.artPal != palette {
		o.artPal, o.artPalOK = palette, true
		for _, e := range o.arts {
			e.built = false
		}
	}
	// 依目前的規劃重新登記：被替換或清掉的 Key 不留在 layer。
	keep := map[string]bool{}
	for _, e := range o.arts {
		keep[e.rect.Key] = true
	}
	for _, s := range append([]*xlate.Stamp(nil), o.artLayer.Art...) {
		if !keep[s.Key] {
			o.artLayer.DropArt(s.Key)
		}
	}
	for _, e := range o.arts {
		if e.built {
			continue
		}
		if err := o.artLayer.AddArt(e.stamp(palette, o.scale)); err != nil {
			o.Skips = append(o.Skips, Skip{e.rect.Key, err.Error()})
		}
		e.built = true
	}
	rgb := make([]byte, 3*len(indexed))
	for i, c := range indexed {
		p := palette[c]
		rgb[3*i], rgb[3*i+1], rgb[3*i+2] = p[0], p[1], p[2]
	}
	o.artLayer.Frame(indexed, rgb)
	o.artLayer.Draw(out, o.scale, nil)
}
