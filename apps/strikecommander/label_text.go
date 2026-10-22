package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 熱點標籤與計分板的反鋸齒文字（strike_commander 規格 011 §3.4、§3.5、§3.6.1、§3.6.2、§3.8、§3.9）。

// NewOverlayText 建一個 overlay（011 §3.9）：標籤、計分板與圖面的中文以圖集 fonts 繪製。fonts 不得為 nil。
// scale 接受 1、2、3；1 倍時永遠不產生 stamp（001 §2.6）。tr 是熱點標籤 catalog（text/opttest_labels），
// gfx 是圖形字 catalog（text/menu_gfx，004 §3.6；可為 nil）。
func NewOverlayText(fonts *TextFonts, tr, gfx map[string]string, scale int) (*Overlay, error) {
	if fonts == nil || scale < 1 || scale > 3 {
		return nil, fmt.Errorf("strikecommander: overlay 輸入無效")
	}
	return &Overlay{layer: &xlate.Layer{W: 320, H: 200}, artLayer: &xlate.Layer{W: 320, H: 200}, tr: tr, gfx: gfx, scale: scale,
		Text: fonts}, nil
}

// textItem 是新路徑一串字的排版結果（011 §3.9）：輸出像素座標。
type textItem struct {
	raw        []rune // N() 前的譯文（CommitStamps.Texts、labels.tsv 的 text）
	display    []rune // N() 後，實際畫的字串
	font       *xlate.AlphaFont
	px         int
	tx         int // 筆位起點（輸出像素）
	baseline   int
	clip       xlate.AlphaRect
	box        xlate.AlphaRect // 排版時以整串遮罩算出的文字框（外框層與填色層的聯集，裁到 clip）
	outline    bool
	outlineIdx uint8 // 外框色號
	fgIdx      uint8 // 字色色號
	e0, e1     int   // 英文範圍 [E0, E1)（原版像素）
}

// PlanText 是新路徑一筆標籤：遮蓋 stamp 與文字項（011 §3.9）。
type PlanText struct {
	Cover  *xlate.Stamp
	Text   textItem
	Kind   Kind
	BG, FG uint8
}

func ceilDiv(a, b int) int { return -floorDiv(-a, b) }

func unionRect(a, b xlate.AlphaRect) xlate.AlphaRect {
	switch {
	case a.Empty():
		return b
	case b.Empty():
		return a
	}
	return xlate.AlphaRect{X0: min(a.X0, b.X0), Y0: min(a.Y0, b.Y0), X1: max(a.X1, b.X1), Y1: max(a.Y1, b.Y1)}
}

// textHorizontal 是 011 §3.5 水平：置中 tx = ⌊((E0 + E1)·s − W) / 2⌋；推回到 [A0·s + m, A1·s − m − W]；
// W > (A1 − A0)·s − 2m 時回錯（Skip）。
func textHorizontal(e0, e1, a0, a1, s, w, m int) (int, error) {
	tx := floorDiv((e0+e1)*s-w, 2)
	lo, hi := a0*s+m, a1*s-m
	if w > hi-lo {
		return 0, fmt.Errorf("譯文寬 %d 輸出像素超過可用寬 %d", w, hi-lo)
	}
	return min(max(tx, lo), hi-w), nil
}

// textVertical 是 011 §3.5 垂直（字帶置中）：字帶 [b0, b1) 原版列，ey = b0·s + ⌊(H − px) / 2⌋，baseline = ey + asc(px)。
// px > H 時回錯（Skip）。
func textVertical(b0, b1, s, px int) (ey, baseline int, err error) {
	h := (b1 - b0) * s
	if px > h {
		return 0, 0, fmt.Errorf("字級 %d 大於字帶高 %d", px, h)
	}
	ey = b0*s + (h-px)/2
	return ey, ey + TextAscent(px), nil
}

// textBox 回字串在 (tx, baseline) 的文字框：外框層（outline 時）與填色層的整串遮罩外接框聯集，裁到 clip。
func textBox(f *xlate.AlphaFont, rs []rune, tx, baseline int, clip xlate.AlphaRect, outline bool) xlate.AlphaRect {
	_, b, _, _ := xlate.StringMask(f, rs, tx, baseline, xlate.AlphaFill, clip)
	if outline {
		_, sb, _, _ := xlate.StringMask(f, rs, tx, baseline, xlate.AlphaStroke, clip)
		b = unionRect(b, sb)
	}
	return b
}

// PlanLabelText 依 011 §3.5、§3.6.1 規劃一筆熱點標籤（board 種類交給 planBoardText，§3.6.2）。text 是 N() 前的譯文。
// 缺字由呼叫端先以 N() 與圖集檢查（Sync 記 Miss）；這裡的錯誤是 Skip。
func PlanLabelText(l Label, text []rune, scale int, fonts *TextFonts) (PlanText, error) {
	if scale != 2 && scale != 3 {
		return PlanText{}, fmt.Errorf("%d 倍輸出不覆繪", scale)
	}
	if len(text) == 0 {
		return PlanText{}, fmt.Errorf("譯文是空的")
	}
	if l.Kind == KindBoard {
		return planBoardText(l, text, scale, fonts)
	}
	px := pxLabel[scale]
	f := fonts.Font(px)
	if f == nil {
		return PlanText{}, fmt.Errorf("圖集缺字級 %d", px)
	}
	disp := NarrowAlnum(text)
	w, miss := f.Advance(disp)
	if len(miss) != 0 {
		return PlanText{}, fmt.Errorf("圖集缺字 %q", string(miss))
	}
	tx, err := textHorizontal(l.X0, l.X1, 0, 320, scale, w, 1)
	if err != nil {
		return PlanText{}, err
	}
	b0, b1 := l.Y+1, l.Y+1+stampCellH
	_, baseline, err := textVertical(b0, b1, scale, px)
	if err != nil {
		return PlanText{}, err
	}
	clip := xlate.AlphaRect{X0: 0, Y0: max((b0-1)*scale, 0), X1: 320 * scale, Y1: min((b1+1)*scale, 200*scale)}
	box := textBox(f, disp, tx, baseline, clip, true)
	cx0, cx1 := l.X0, l.X1
	if !box.Empty() {
		cx0, cx1 = min(cx0, floorDiv(box.X0, scale)), max(cx1, ceilDiv(box.X1, scale))
	}
	cx0, cx1 = max(cx0, 0), min(cx1, 320)
	cover := &xlate.Stamp{Key: l.Key + ".cover", X: cx0, Y: b0, Cells: 1, CellW: cx1 - cx0, CellH: stampCellH, State: xlate.Shown}
	ti := textItem{
		raw: append([]rune(nil), text...), display: disp, font: f, px: px, tx: tx, baseline: baseline, clip: clip, box: box,
		outline: true, outlineIdx: bgIndex, fgIdx: fgIndex, e0: l.X0, e1: l.X1,
	}
	return PlanText{Cover: cover, Text: ti, Kind: KindLabel, BG: bgIndex, FG: fgIndex}, nil
}

// planBoardText 是計分板 GROUND、AIR 的新路徑（011 §3.6.2）：2 倍 16、3 倍 22，字色 10 號，無外框；
// 英文範圍與可用範圍都是游標 [X0, X1)，m = 0；字帶 [Y+1, Y+9) 置中。取代 003 §3.4 的右對齊。
func planBoardText(l Label, text []rune, scale int, fonts *TextFonts) (PlanText, error) {
	px := pxBoard[scale]
	f := fonts.Font(px)
	if f == nil {
		return PlanText{}, fmt.Errorf("圖集缺字級 %d", px)
	}
	disp := NarrowAlnum(text)
	w, miss := f.Advance(disp)
	if len(miss) != 0 {
		return PlanText{}, fmt.Errorf("圖集缺字 %q", string(miss))
	}
	tx, err := textHorizontal(l.X0, l.X1, l.X0, l.X1, scale, w, 0)
	if err != nil {
		return PlanText{}, err
	}
	b0, b1 := l.Y+1, l.Y+1+boardCellH
	_, baseline, err := textVertical(b0, b1, scale, px)
	if err != nil {
		return PlanText{}, err
	}
	clip := xlate.AlphaRect{X0: max(l.X0*scale, 0), Y0: max((b0-1)*scale, 0), X1: min(l.X1*scale, 320*scale), Y1: min((b1+1)*scale, 200*scale)}
	box := textBox(f, disp, tx, baseline, clip, false)
	cx0, cx1 := l.X0, l.X1
	if !box.Empty() {
		cx0, cx1 = min(cx0, floorDiv(box.X0, scale)), max(cx1, ceilDiv(box.X1, scale))
	}
	cx0, cx1 = max(cx0, 0), min(cx1, 320)
	cover := &xlate.Stamp{Key: l.Key + ".cover", X: cx0, Y: b0, Cells: 1, CellW: cx1 - cx0, CellH: boardCellH, State: xlate.Shown}
	ti := textItem{
		raw: append([]rune(nil), text...), display: disp, font: f, px: px, tx: tx, baseline: baseline, clip: clip, box: box,
		outline: false, outlineIdx: bgIndex, fgIdx: boardFGIndex, e0: l.X0, e1: l.X1,
	}
	return PlanText{Cover: cover, Text: ti, Kind: KindBoard, BG: boardBGIndex, FG: boardFGIndex}, nil
}

// drawTextItems 是 011 §3.4 第 4 條的兩趟：先對照畫且有外框的串畫外框層，再對照畫的串畫填色層。
// 回與 items 平行的文字框（兩層 ink 的聯集）；items 中的 nil 不畫、框為空。
func drawTextItems(out []byte, scale int, palette [256][3]uint8, items []*textItem) []xlate.AlphaRect {
	W, H := 320*scale, 200*scale
	ink := make([]xlate.AlphaRect, len(items))
	for i, ti := range items {
		if ti == nil || !ti.outline {
			continue
		}
		k, _, _ := xlate.DrawAlphaText(out, W, H, ti.font, ti.display, ti.tx, ti.baseline, xlate.AlphaStroke, palette[ti.outlineIdx], ti.clip)
		ink[i] = unionRect(ink[i], k)
	}
	for i, ti := range items {
		if ti == nil {
			continue
		}
		k, _, _ := xlate.DrawAlphaText(out, W, H, ti.font, ti.display, ti.tx, ti.baseline, xlate.AlphaFill, palette[ti.fgIdx], ti.clip)
		ink[i] = unionRect(ink[i], k)
	}
	return ink
}

// TextBoxes 回最近一次 Draw 照畫的新路徑串的文字框（輸出像素）；不含 Hidden、miss、Skip（011 §3.9）。
func (o *Overlay) TextBoxes() []xlate.AlphaRect {
	var out []xlate.AlphaRect
	for _, b := range o.lastInk {
		if !b.Empty() {
			out = append(out, b)
		}
	}
	return out
}

// commitItem 是 CommitStamps 的一筆（011 §3.9）：N() 前的譯文與種類。
type commitItem struct {
	Text string
	Kind Kind
}

// commitStamps 組 CommitStamps：Stamps 是每筆 2 個（遮蓋＋文字）的總數，Texts 是 N() 前的譯文（011 §3.9）。
func commitStamps(sc int, items []commitItem) CommitStamps {
	cs := CommitStamps{Scale: sc, Stamps: 2 * len(items)}
	for _, it := range items {
		cs.Texts = append(cs.Texts, it.Text)
		cs.Kinds = append(cs.Kinds, it.Kind)
	}
	return cs
}

// commitStamps 回這個 overlay 目前的 CommitStamps（RunReceipt 的每次提交，011 §3.9）。
func (o *Overlay) commitStamps(sc int) CommitStamps {
	cs := commitStamps(sc, o.commitItems())
	return cs
}

// commitItems 依重建順序回每筆標籤的 commitItem（textItem 的 N() 前譯文與種類）。
func (o *Overlay) commitItems() []commitItem {
	var items []commitItem
	for _, m := range o.matches {
		items = append(items, commitItem{string(m.textItem.raw), o.info[m.cover].kind})
	}
	return items
}

// fontsTSVHeader 是 <前綴>-fonts.tsv 的欄名（同 manifest，011 §3.8）。
const fontsTSVHeader = textTableHeader

// WriteFontsTSV 寫 <dir>/<prefix>-fonts.tsv（011 §3.8）：6 個 file 列（TextFonts.Files 原樣）與最後一列 anchor。
func WriteFontsTSV(dir, prefix string, f *TextFonts, expectedPath string) error {
	if f == nil {
		return fmt.Errorf("strikecommander: 沒有圖集")
	}
	var b strings.Builder
	b.WriteString(fontsTSVHeader + "\n")
	for _, ff := range f.Files {
		fmt.Fprintf(&b, "file\t%s\t%d\t%d\t%d\t%s\n", ff.Name, ff.Px, ff.Count, ff.Bytes, ff.SHA256)
	}
	exp := expectedPath
	if exp == "" {
		exp = "-"
	}
	fmt.Fprintf(&b, "anchor\t%s\t-\t-\t-\t%s\n", f.Anchor, exp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, prefix+"-fonts.tsv"), []byte(b.String()), 0o644)
}
