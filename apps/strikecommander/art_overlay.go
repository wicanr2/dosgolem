package strikecommander

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 圖形字的圖面（strike_commander 規格 004 §3.3 到 §3.5；主選單與 MENU 片頭選單見規格 008 §3.4 到 §3.6）：
// 每個繪圖事件一列 204 圖面（主選單底板一個事件五列），放在 Overlay 自有的獨立 artLayer。
// Reference 是解碼的原版色號（不從畫面取樣），Owner 空字串，顯示由 204 的逐格比對決定。

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
	idx   []uint8 // (W×scale)×(H×scale) 輸出像素的色號（新路徑只有無字底）
	built bool    // 已以 artPal 建 Pix 並登記到 artLayer
	// text 是 011 §3.6.3 新路徑的字（nil 為舊路徑：字已畫進 idx）。
	text *artText
}

// 主選單圖面的 Key 前綴（008 §3.6）。
const (
	keyMainMenu      = "mainmenu/"
	keyMainMenuPlate = "mainmenu-plate/"
	keyTomenu        = "tomenu/"
)

// artKey 回圖面的 Key（004 §3.3、008 §3.6），依檔別：
//   - SAVELOAD：saveload/<元素 x>,<元素 y>/<一般態 k>；
//   - MAINMENU 按鈕：mainmenu/<x>,<y>/<n>，n = k mod 5；
//   - MAINMENU 底板列：mainmenu-plate/<n>，n 是行號；
//   - TOMENU 按鈕：tomenu/<x>,<y>/<n>，n = k mod 2。
func artKey(ev ArtEvent) string {
	switch ev.File {
	case ArtMainMenuButtons:
		return fmt.Sprintf("%s%d,%d/%d", keyMainMenu, ev.X, ev.Y, ev.K%mmLines)
	case ArtMainMenuPlate:
		return fmt.Sprintf("%s%d", keyMainMenuPlate, ev.Line)
	case ArtTomenuButtons:
		return fmt.Sprintf("%s%d,%d/%d", keyTomenu, ev.X, ev.Y, ev.K%2)
	}
	return fmt.Sprintf("saveload/%d,%d/%d", ev.X, ev.Y, normalK(ev.K))
}

// AddDialogArt 依一個繪圖事件規劃圖面（004 §3.3、008 §3.4）。同 Key 的後畫替換前畫。
// 譯文缺 key、字型缺字記 Miss；超寬、倍率、矩形不合法記 Skip；兩者都不產生圖面，畫面維持英文。
//   - SAVELOAD：規劃之前先依 007 §3.6 移除確認框的圖面（與這次事件是否產生圖面無關）。
//   - MAINMENU 底板：移除 Key 前綴 mainmenu、mainmenu-plate 的舊列，記下事件，先不產生圖面（008 §3.4、§3.6）。
//   - MAINMENU 按鈕：有記下的底板時，先以這個事件的按鈕檔為五行灰字建列（Order 取底板事件），
//     與這個按鈕事件本身是否產生圖面無關，再規劃按鈕本身。
func (o *Overlay) AddDialogArt(ev ArtEvent) {
	switch ev.File {
	case ArtSaveLoad:
		o.dropConfirmArt(ev)
	case ArtMainMenuPlate:
		o.dropArt(func(e *artEntry) bool {
			return strings.HasPrefix(e.rect.Key, keyMainMenu) || strings.HasPrefix(e.rect.Key, keyMainMenuPlate)
		})
		p := ev
		o.platePending = &p
		return
	case ArtMainMenuButtons:
		if p := o.platePending; p != nil {
			o.platePending = nil
			for n := 0; n < mmLines; n++ {
				row := *p
				row.Line, row.Buttons = n, ev.Buttons
				o.addArt(row)
			}
		}
	}
	o.addArt(ev)
}

// addArt 規劃一列圖面並加入（同 Key 替換）。
func (o *Overlay) addArt(ev ArtEvent) {
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

// dropArt 移除符合 drop 的圖面，同時從 artLayer 移除：drawArt 在沒有圖面時直接返回，不會替 layer 對帳。
func (o *Overlay) dropArt(drop func(e *artEntry) bool) {
	keep := o.arts[:0]
	for _, e := range o.arts {
		if drop(e) {
			o.artLayer.DropArt(e.rect.Key)
			continue
		}
		keep = append(keep, e)
	}
	for i := len(keep); i < len(o.arts); i++ {
		o.arts[i] = nil
	}
	o.arts = keep
}

// dropConfirmArt 是 007 §3.6 的生命期規則（防禦；逐欄比對已讓關閉後的確認框停畫），只對 SAVELOAD 的事件：
//  1. k0 的繪圖事件（確認框關閉後整個對話框重畫的第一筆）移除一般態屬於 {15, 16, 17, 19} 的圖面；
//  2. k15 的繪圖事件移除一般態 16 的圖面（新一次確認框）。
//
// 只移除 SAVELOAD 的圖面；主選單的 k0 事件不經這裡（008 §3.1）。
func (o *Overlay) dropConfirmArt(ev ArtEvent) {
	if ev.File != ArtSaveLoad {
		return
	}
	var drop func(n int) bool
	switch ev.K {
	case 0:
		drop = confirmElement
	case 15:
		drop = func(n int) bool { return n == 16 }
	default:
		return
	}
	o.dropArt(func(e *artEntry) bool { return e.ev.File == ArtSaveLoad && drop(normalK(e.ev.K)) })
}

// ClearArt 移除全部圖面與記下的主選單底板（全螢幕 blit 入口，004 §3.3）。
func (o *Overlay) ClearArt() {
	o.arts = nil
	o.platePending = nil
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
// 底板列的事件自帶行號與按鈕檔，照常重建。
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

// planArt 依 004 §3.3 到 §3.5 與 008 §3.2 到 §3.6 規劃一列圖面。
func (o *Overlay) planArt(ev ArtEvent) (*artEntry, *Miss, *Skip) {
	sp, ok := artSpecFor(ev)
	if !ok {
		return nil, nil, &Skip{fmt.Sprintf("%s k%d", ev.File, ev.K), "不是有字的元素"}
	}
	if o.scale != 2 && o.scale != 3 {
		return nil, nil, &Skip{sp.key, fmt.Sprintf("%d 倍輸出不覆繪", o.scale)}
	}
	if ev.W != sp.w || ev.H != sp.h || len(ev.Entry) != sp.w*sp.h || len(ev.Base0) != sp.w*sp.h {
		return nil, nil, &Skip{sp.key, fmt.Sprintf("條目尺寸 %d×%d 不是 %d×%d", ev.W, ev.H, sp.w, sp.h)}
	}
	tr, ok := o.gfx[sp.key]
	if !ok || tr == "" {
		return nil, &Miss{Key: sp.key, Reason: MissTranslation}, nil
	}
	if o.TextArt && o.Text != nil {
		return o.planArtText(ev, sp, []rune(tr)) // strike_commander 規格 011 §3.6.3
	}
	text := []rune(tr)
	var missing []rune
	for _, r := range text {
		if !o.font.Wide.Wide(r) || o.font.Font.Glyphs[r] == nil {
			missing = append(missing, r)
		}
	}
	if len(missing) != 0 {
		return nil, &Miss{Key: sp.key, Reason: MissGlyph, Runes: missing}, nil
	}
	sc := o.scale
	gx, gy, step, err := glyphOrigin(len(text), sc, sp.band, sp.center)
	if err != nil {
		return nil, nil, &Skip{sp.key, err.Error()}
	}
	bw, bh := sp.band.x1-sp.band.x0+1, sp.band.y1-sp.band.y0+1
	rect := ArtRect{Key: sp.rectKey, X: ev.X + sp.band.x0, Y: ev.Y + sp.band.y0, W: bw, H: bh}
	if rect.X < 0 || rect.Y < 0 || rect.X+bw > 320 || rect.Y+bh > 200 {
		return nil, nil, &Skip{sp.key, fmt.Sprintf("字帶 (%d,%d) %d×%d 超出畫面", rect.X, rect.Y, bw, bh)}
	}
	ref := make([]uint8, bw*bh)
	for y := 0; y < bh; y++ {
		copy(ref[y*bw:(y+1)*bw], ev.Entry[(sp.band.y0+y)*sp.w+sp.band.x0:])
	}
	bg, passes, err := artBackground(ev)
	if err != nil {
		return nil, nil, &Skip{sp.key, err.Error()}
	}
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
	// 各趟依序畫，先畫的在下；最後一趟是字色 c（004 §3.5、008 §3.5）。
	for _, p := range passes {
		for i, r := range text {
			g := font.Glyphs[r]
			ox, oy := gx+i*step+p.dx, gy+p.dy
			for py := 0; py < font.H; py++ {
				for px := 0; px < font.W; px++ {
					if g[py*rb+px/8]&(0x80>>uint(px%8)) != 0 {
						plot(ox+px, oy+py, p.col)
					}
				}
			}
		}
	}
	return &artEntry{ev: ev, rect: rect, ref: ref, idx: idx}, nil, nil
}

// stamp 以色盤 pal 建這一列的 204 圖面（CellW 1，每欄一格，004 §3.3）。
// 新路徑（011 §3.6.3）：無字底以色盤轉 RGB、alpha 255，各趟依序以填色層、該趟色號 DrawAlphaText。
func (e *artEntry) stamp(pal [256][3]uint8, scale int) *xlate.Stamp {
	pix := make([]uint8, 4*len(e.idx))
	for i, c := range e.idx {
		p := pal[c]
		pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = p[0], p[1], p[2], 255
	}
	if t := e.text; t != nil {
		ow, oh := e.rect.W*scale, e.rect.H*scale
		clip := xlate.AlphaRect{X0: 0, Y0: 0, X1: ow, Y1: oh}
		t.ink = xlate.AlphaRect{}
		for _, p := range t.passes {
			k, _, _ := xlate.DrawAlphaText(pix, ow, oh, t.font, t.display, t.tx+p.dx, t.baseline+p.dy, xlate.AlphaFill, pal[p.col], clip)
			t.ink = unionRect(t.ink, k)
		}
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
