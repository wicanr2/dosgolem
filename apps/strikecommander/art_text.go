package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 圖面（存讀檔、確認框、主選單、底板灰字、MENU 片頭選單）的反鋸齒字（strike_commander 規格 011 §3.6.3）。

// artText 是一列圖面新路徑的字：N() 後的字串、字級、相對字帶的筆位與 em 上緣（輸出像素）、趟表。
type artText struct {
	raw, display []rune
	font         *xlate.AlphaFont
	px           int
	tx, ey       int
	baseline     int
	passes       []glyphPass
	ink          xlate.AlphaRect // 最近一次建 Pix 時各趟 ink 的聯集（相對字帶）
}

// artPx 回圖面的字級（011 §3.1）：TOMENU 2 倍 22、3 倍 32，其他 16、22。
func artPx(f ArtFile, scale int) int {
	if f == ArtTomenuButtons {
		return pxTomenu[scale]
	}
	return pxArt[scale]
}

// artPressedCenter 回一般態事件 ev 對應按下態的置中框（存讀檔：墨跡框下移 1 列）。
// 主選單與 TOMENU 兩態共用置中框（008 §3.2），回 false。
func artPressedCenter(ev ArtEvent) (box, bool) {
	if ev.File != ArtSaveLoad || pressedState(ev.K) || !pressedState(ev.K+1) {
		return box{}, false
	}
	_, ink, ok := elementFor(ev.K + 1)
	return ink, ok
}

// artEy 是 011 §3.6.3 垂直第 4 步：ey = clamp(⌊((iy0 + iy1 + 1)·s − px) / 2⌋ − by0·s, lo, hi)（相對字帶）。
func artEy(center, band box, scale, px, lo, hi int) int {
	return clampInt(floorDiv((center.y0+center.y1+1)*scale-px, 2)-band.y0*scale, lo, hi)
}

// artTextGeom 回 011 §3.6.3 的 tx、ey（相對字帶，輸出像素）；不放得下時回錯（Skip）。
func artTextGeom(ev ArtEvent, sp artSpec, passes []glyphPass, f *xlate.AlphaFont, disp []rune, scale int) (tx, ey int, err error) {
	bw, bh := sp.band.x1-sp.band.x0+1, sp.band.y1-sp.band.y0+1
	w, _ := f.Advance(disp)
	m := 0
	var dys []int
	for _, p := range passes {
		m = max(m, p.dx, -p.dx)
		dys = append(dys, p.dy)
	}
	if len(dys) == 0 {
		dys = []int{0}
	}
	// 水平：置中框先減字帶起始欄；可用範圍是字帶，m = 字樣趟的最大 |dx|。
	x0, x1 := sp.center.x0-sp.band.x0, sp.center.x1-sp.band.x0
	tx = floorDiv((x0+x1+1)*scale-w, 2)
	lo, hi := m, bw*scale-m
	if w > hi-lo {
		return 0, 0, fmt.Errorf("譯文寬 %d 輸出像素超過字帶寬 %d − 2×%d", w, bw*scale, m)
	}
	tx = min(max(tx, lo), hi-w)
	// 垂直：墨跡感知夾住（第 1 到 4 步）與按下態保底（第 5 步）。
	it, ib, ok, _ := textInkRows(f, disp, false)
	if !ok {
		it, ib = 0, 0
	}
	vlo, vhi := artVerticalRange(it, ib, bh*scale, dys)
	if vlo > vhi {
		return 0, 0, fmt.Errorf("墨跡列 %d..%d（趟 dy %v）放不進字帶高 %d", it, ib-1, dys, bh*scale)
	}
	ey = artEy(sp.center, sp.band, scale, f.Px, vlo, vhi)
	if pc, ok := artPressedCenter(ev); ok && pc.y0+pc.y1 > sp.center.y0+sp.center.y1 {
		eyP := artEy(pc, sp.band, scale, f.Px, vlo, vhi)
		if eyP-ey < 1 && ey-1 >= vlo {
			ey--
		}
	}
	return tx, ey, nil
}

// planArtText 是 planArt 的新路徑（011 §3.6.3）：缺字記 MissGlyph；趟表、版面或字帶不合記 Skip。
// idx 只放無字底；字在 stamp() 以色盤建 Pix 時畫。
func (o *Overlay) planArtText(ev ArtEvent, sp artSpec, text []rune) (*artEntry, *Miss, *Skip) {
	sc := o.scale
	px := artPx(ev.File, sc)
	f := o.Text.Font(px)
	if f == nil {
		return nil, nil, &Skip{sp.key, fmt.Sprintf("圖集缺字級 %d", px)}
	}
	disp := NarrowAlnum(text)
	if _, miss := f.Advance(disp); len(miss) != 0 {
		return nil, &Miss{Key: sp.key, Reason: MissGlyph, Runes: miss}, nil
	}
	bg, passes, err := artBackground(ev)
	if err != nil {
		return nil, nil, &Skip{sp.key, err.Error()}
	}
	tx, ey, err := artTextGeom(ev, sp, passes, f, disp, sc)
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
	ow, oh := bw*sc, bh*sc
	idx := make([]uint8, ow*oh)
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			idx[y*ow+x] = bg[(y/sc)*bw+x/sc]
		}
	}
	t := &artText{raw: append([]rune(nil), text...), display: disp, font: f, px: px, tx: tx, ey: ey,
		baseline: ey + TextAscent(px), passes: append([]glyphPass(nil), passes...)}
	return &artEntry{ev: ev, rect: rect, ref: ref, idx: idx, text: t}, nil, nil
}

// ArtReportEntry 是 arts.tsv 的一列（011 §3.8）：圖面 Key 與事件、新路徑的版面（相對字帶，輸出像素）。
type ArtReportEntry struct {
	Key               string
	File              ArtFile
	G, K, X, Y, Order int
	Path              string
	Display           string
	Px, TxOut, Ey     int
	Ink               xlate.AlphaRect
}

// ArtReport 依圖面順序列出目前的圖面（011 §3.8 arts.tsv）。
func (o *Overlay) ArtReport() []ArtReportEntry {
	var out []ArtReportEntry
	for _, e := range o.arts {
		t := e.text
		out = append(out, ArtReportEntry{Key: e.rect.Key, File: e.ev.File, G: e.ev.G, K: e.ev.K, X: e.ev.X, Y: e.ev.Y, Order: e.ev.Order,
			Path: "new", Display: string(t.display), Px: t.px, TxOut: t.tx, Ey: t.ey, Ink: t.ink})
	}
	return out
}

// artsTSVHeader 是 <前綴>-s<倍率>-arts.tsv 的欄名（011 §3.8）。
var artsTSVHeader = []string{"key", "file", "g", "k", "x", "y", "order", "display", "px", "tx_out", "ey", "ink_x0", "ink_y0", "ink_x1", "ink_y1", "path"}

// ArtsTSV 編 arts.tsv（011 §3.8；path 欄一律 new）。
func ArtsTSV(es []ArtReportEntry) []byte {
	var b strings.Builder
	b.WriteString(strings.Join(artsTSVHeader, "\t") + "\n")
	for _, e := range es {
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t", e.Key, e.File, e.G, e.K, e.X, e.Y, e.Order)
		ink := e.Ink
		if ink.Empty() {
			ink = xlate.AlphaRect{}
		}
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\tnew\n", e.Display, e.Px, e.TxOut, e.Ey, ink.X0, ink.Y0, ink.X1, ink.Y1)
	}
	return []byte(b.String())
}

// writeArtsTSV 寫各倍率的 <prefix>-s<倍率>-arts.tsv；arts 為 nil 時不寫。
func writeArtsTSV(dir, prefix string, scales []int, arts map[int][]ArtReportEntry) error {
	if arts == nil {
		return nil
	}
	for _, sc := range scales {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-arts.tsv", prefix, sc)), ArtsTSV(arts[sc]), 0o644); err != nil {
			return err
		}
	}
	return nil
}
