package xlate

import (
	"encoding/binary"
	"fmt"
	"os"
)

// 反鋸齒圖集文字（docs/spec/453-xlate-alpha-atlas-text.md）。
//
// 圖集檔 GOLEMAAF 第 1 版：一個檔是一種字型的一個字級，每字有 8 位元 alpha 填色遮罩，
// 可選的實心描邊外框遮罩。執行期只讀檔與整數運算，不做向量光柵化（§3.3）。

// alphaFontMagic 是 GOLEMAAF 圖集檔的檔頭識別字串（453 §2）。
const alphaFontMagic = "GOLEMAAF"

// alphaFontHeaderLen 是檔頭長度（453 §2）。
const alphaFontHeaderLen = 56

// AlphaGlyph 是圖集內一個字的度量與遮罩（453 §3）。
type AlphaGlyph struct {
	Adv            int
	FX, FY, FW, FH int
	Fill           []byte // len = FW × FH，列優先
	SX, SY, SW, SH int    // HasStroke 為假時全為 0
	Stroke         []byte // HasStroke 為假時為 nil
}

// AlphaFont 是一個字級的圖集（453 §3）。Name 不在檔案內，由呼叫端標記。
type AlphaFont struct {
	Name                string
	Px, Ascent, Descent int
	HasStroke           bool
	StrokeRadius        int
	Source              [32]byte
	Glyphs              map[rune]*AlphaGlyph
}

// aafReader 是嚴格的順序讀取器：每讀一個欄位前先檢查剩餘長度（453 §2）。
type aafReader struct {
	b   []byte
	off int
	err error
}

func (r *aafReader) need(n int64, what string) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || n > int64(len(r.b)-r.off) {
		r.err = fmt.Errorf("xlate: GOLEMAAF 位移 %d 讀 %s 需 %d bytes，只剩 %d", r.off, what, n, len(r.b)-r.off)
		return false
	}
	return true
}

func (r *aafReader) u16(what string) int {
	if !r.need(2, what) {
		return 0
	}
	v := binary.LittleEndian.Uint16(r.b[r.off:])
	r.off += 2
	return int(v)
}

func (r *aafReader) i16(what string) int {
	if !r.need(2, what) {
		return 0
	}
	v := int16(binary.LittleEndian.Uint16(r.b[r.off:]))
	r.off += 2
	return int(v)
}

func (r *aafReader) u32(what string) uint32 {
	if !r.need(4, what) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v
}

// mask 讀 w×h bytes 的遮罩；乘積以 64 位元計算，先與剩餘長度比較才配置（453 §2）。
func (r *aafReader) mask(w, h int, what string) []byte {
	n := int64(w) * int64(h)
	if !r.need(n, what) {
		return nil
	}
	m := make([]byte, n)
	copy(m, r.b[r.off:r.off+int(n)])
	r.off += int(n)
	return m
}

// ParseAlphaFont 嚴格解析一份 GOLEMAAF 第 1 版圖集（453 §2）。任何條件不符都回錯，不回部分結果。
func ParseAlphaFont(b []byte) (*AlphaFont, error) {
	if len(b) < alphaFontHeaderLen {
		return nil, fmt.Errorf("xlate: GOLEMAAF 長度 %d 短於檔頭 %d", len(b), alphaFontHeaderLen)
	}
	if string(b[:8]) != alphaFontMagic {
		return nil, fmt.Errorf("xlate: 輸入不是 %s 圖集", alphaFontMagic)
	}
	r := &aafReader{b: b, off: 8}
	version := r.u16("version")
	px := r.u16("px")
	ascent := r.i16("ascent")
	descent := r.i16("descent")
	flags := r.u16("flags")
	stroke := r.u16("stroke")
	count := r.u32("count")
	if version != 1 {
		return nil, fmt.Errorf("xlate: GOLEMAAF version %d，只支援 1", version)
	}
	if px < 1 || px > 255 {
		return nil, fmt.Errorf("xlate: GOLEMAAF px %d 不在 1..255", px)
	}
	if ascent <= 0 || ascent > px {
		return nil, fmt.Errorf("xlate: GOLEMAAF ascent %d 不在 1..px(%d)", ascent, px)
	}
	if ascent+descent != px {
		return nil, fmt.Errorf("xlate: GOLEMAAF ascent %d + descent %d ≠ px %d", ascent, descent, px)
	}
	if flags&^1 != 0 {
		return nil, fmt.Errorf("xlate: GOLEMAAF flags %#x 有未定義位元", flags)
	}
	hasStroke := flags&1 != 0
	if !hasStroke && stroke != 0 {
		return nil, fmt.Errorf("xlate: GOLEMAAF 無外框遮罩但 stroke = %d", stroke)
	}
	if count < 1 {
		return nil, fmt.Errorf("xlate: GOLEMAAF count 為 0")
	}
	f := &AlphaFont{Px: px, Ascent: ascent, Descent: descent, HasStroke: hasStroke, StrokeRadius: stroke}
	copy(f.Source[:], b[24:56])
	r.off = alphaFontHeaderLen
	f.Glyphs = make(map[rune]*AlphaGlyph)
	prev := int64(-1)
	for i := uint32(0); i < count; i++ {
		cp := r.u32("碼點")
		adv := r.i16("adv")
		fx := r.i16("fx")
		fy := r.i16("fy")
		fw := r.u16("fw")
		fh := r.u16("fh")
		if r.err != nil {
			return nil, r.err
		}
		if cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF) {
			return nil, fmt.Errorf("xlate: GOLEMAAF 第 %d 字碼點 %#x 不合法", i, cp)
		}
		if int64(cp) <= prev {
			return nil, fmt.Errorf("xlate: GOLEMAAF 第 %d 字碼點 U+%04X 未遞增", i, cp)
		}
		prev = int64(cp)
		if adv < 0 {
			return nil, fmt.Errorf("xlate: GOLEMAAF U+%04X adv %d < 0", cp, adv)
		}
		g := &AlphaGlyph{Adv: adv, FX: fx, FY: fy, FW: fw, FH: fh}
		g.Fill = r.mask(fw, fh, "填色遮罩")
		if hasStroke {
			g.SX = r.i16("sx")
			g.SY = r.i16("sy")
			g.SW = r.u16("sw")
			g.SH = r.u16("sh")
			g.Stroke = r.mask(g.SW, g.SH, "外框遮罩")
		}
		if r.err != nil {
			return nil, r.err
		}
		f.Glyphs[rune(cp)] = g
	}
	if r.off != len(b) {
		return nil, fmt.Errorf("xlate: GOLEMAAF 尾端多 %d bytes", len(b)-r.off)
	}
	return f, nil
}

// LoadAlphaFont 讀檔後交給 ParseAlphaFont。要把雜湊綁在實際解析的位元組上時，
// 呼叫端自己讀檔、核對後直接呼叫 ParseAlphaFont。
func LoadAlphaFont(path string) (*AlphaFont, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseAlphaFont(b)
}

// Advance 回字串的前進寬總和；missing 依出現順序列出全部缺字（重複出現的字照列），缺字的前進寬記 0。
func (f *AlphaFont) Advance(rs []rune) (w int, missing []rune) {
	for _, r := range rs {
		g, ok := f.Glyphs[r]
		if !ok {
			missing = append(missing, r)
			continue
		}
		w += g.Adv
	}
	return w, missing
}

// AlphaLayer 選擇圖集的填色層或外框層。
type AlphaLayer int

const (
	AlphaFill AlphaLayer = iota
	AlphaStroke
)

// AlphaRect 是半開矩形 [X0, X1) × [Y0, Y1)，輸出像素。X0 >= X1 或 Y0 >= Y1 為空。
type AlphaRect struct{ X0, Y0, X1, Y1 int }

// Empty 回這個矩形是否為空。
func (r AlphaRect) Empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }

func (r AlphaRect) intersect(o AlphaRect) AlphaRect {
	out := AlphaRect{max(r.X0, o.X0), max(r.Y0, o.Y0), min(r.X1, o.X1), min(r.Y1, o.Y1)}
	if out.Empty() {
		return AlphaRect{}
	}
	return out
}

// glyphLayer 回字形在該層的遮罩位置與內容。
func glyphLayer(g *AlphaGlyph, layer AlphaLayer) (x, y, w, h int, a []byte) {
	if layer == AlphaStroke {
		return g.SX, g.SY, g.SW, g.SH, g.Stroke
	}
	return g.FX, g.FY, g.FW, g.FH, g.Fill
}

// StringMask 算出字串在 layer 的合成遮罩（453 §3.2）。
// 回傳 m 的排列：寬 = box.X1 − box.X0、高 = box.Y1 − box.Y0、列優先，m[(y−box.Y0)*寬 + (x−box.X0)]。
// box 是裁到 clip 之後 alpha > 0 的外接框；字串沒有任何 alpha > 0 時 box 為空、m 為 nil。
// 有缺字時 m 為 nil、box 為空，missing 列出全部缺字。
// layer 為 AlphaStroke 而字型沒有外框層時 m 為 nil、box 為空、noStroke 為真（不畫，呼叫端記錄）。
func StringMask(f *AlphaFont, rs []rune, x, baseline int, layer AlphaLayer, clip AlphaRect) (m []byte, box AlphaRect, missing []rune, noStroke bool) {
	if _, missing = f.Advance(rs); len(missing) > 0 {
		return nil, AlphaRect{}, missing, false
	}
	if layer == AlphaStroke && !f.HasStroke {
		return nil, AlphaRect{}, nil, true
	}
	// 全部字形遮罩的聯集，再裁到 clip；合成只在這個範圍內做（逐像素運算，先裁後合與先合後裁相同）。
	var all AlphaRect
	first := true
	pen := x
	for _, r := range rs {
		g := f.Glyphs[r]
		gx, gy, gw, gh, _ := glyphLayer(g, layer)
		if gw > 0 && gh > 0 {
			gr := AlphaRect{pen + gx, baseline + gy, pen + gx + gw, baseline + gy + gh}
			if first {
				all, first = gr, false
			} else {
				all = AlphaRect{min(all.X0, gr.X0), min(all.Y0, gr.Y0), max(all.X1, gr.X1), max(all.Y1, gr.Y1)}
			}
		}
		pen += g.Adv
	}
	area := all.intersect(clip)
	if first || area.Empty() {
		return nil, AlphaRect{}, nil, false
	}
	aw, ah := area.X1-area.X0, area.Y1-area.Y0
	buf := make([]byte, aw*ah)
	pen = x
	for _, r := range rs {
		g := f.Glyphs[r]
		gx, gy, gw, gh, ga := glyphLayer(g, layer)
		ox, oy := pen+gx, baseline+gy
		for j := 0; j < gh; j++ {
			yy := oy + j
			if yy < area.Y0 || yy >= area.Y1 {
				continue
			}
			for i := 0; i < gw; i++ {
				xx := ox + i
				if xx < area.X0 || xx >= area.X1 {
					continue
				}
				a := int(ga[j*gw+i])
				p := &buf[(yy-area.Y0)*aw+(xx-area.X0)]
				*p = uint8((a*255 + int(*p)*(255-a) + 127) / 255) // 疊合式（453 §3.2 第 4 步）
			}
		}
		pen += g.Adv
	}
	// 縮成 alpha > 0 的外接框。
	bx0, by0, bx1, by1 := aw, ah, -1, -1
	for j := 0; j < ah; j++ {
		for i := 0; i < aw; i++ {
			if buf[j*aw+i] == 0 {
				continue
			}
			bx0, by0, bx1, by1 = min(bx0, i), min(by0, j), max(bx1, i), max(by1, j)
		}
	}
	if bx1 < 0 {
		return nil, AlphaRect{}, nil, false
	}
	bw, bh := bx1-bx0+1, by1-by0+1
	m = make([]byte, bw*bh)
	for j := 0; j < bh; j++ {
		copy(m[j*bw:(j+1)*bw], buf[(by0+j)*aw+bx0:(by0+j)*aw+bx0+bw])
	}
	box = AlphaRect{area.X0 + bx0, area.Y0 + by0, area.X0 + bx1 + 1, area.Y0 + by1 + 1}
	return m, box, nil, false
}

// blendValid 檢查 BlendMask 的輸入；不成立時完全不寫入。
func blendValid(dst []byte, W, H int, m []byte, box AlphaRect) bool {
	if W < 0 || H < 0 || int64(len(dst)) < 4*int64(W)*int64(H) {
		return false
	}
	if box.Empty() {
		return len(m) == 0
	}
	return int64(len(m)) == int64(box.X1-box.X0)*int64(box.Y1-box.Y0)
}

// BlendMask 把遮罩以顏色 c 混進 RGBA 緩衝 dst（寬 W、高 H 像素，H 由參數給，不由 len(dst) 推算）。
// len(dst) < 4·W·H，或 len(m) ≠ (box 寬)·(box 高)（空 box 時 m 必須為空）時完全不寫入。
// 只寫 box ∩ [0,W)×[0,H) 內 alpha > 0 的像素。
func BlendMask(dst []byte, W, H int, m []byte, box AlphaRect, c [3]uint8) {
	blendMask(dst, W, H, m, box, c)
}

// blendMask 同 BlendMask，另回實際寫入像素的緊外接框（DrawAlphaText 的 ink）。
func blendMask(dst []byte, W, H int, m []byte, box AlphaRect, c [3]uint8) (ink AlphaRect) {
	if !blendValid(dst, W, H, m, box) || box.Empty() {
		return AlphaRect{}
	}
	bw := box.X1 - box.X0
	area := box.intersect(AlphaRect{0, 0, W, H})
	if area.Empty() {
		return AlphaRect{}
	}
	ix0, iy0, ix1, iy1 := area.X1, area.Y1, area.X0-1, area.Y0-1
	for y := area.Y0; y < area.Y1; y++ {
		for x := area.X0; x < area.X1; x++ {
			a := int(m[(y-box.Y0)*bw+(x-box.X0)])
			if a == 0 {
				continue
			}
			o := 4 * (y*W + x)
			for k := 0; k < 3; k++ {
				dst[o+k] = uint8((int(c[k])*a + int(dst[o+k])*(255-a) + 127) / 255) // 453 §3.1
			}
			dst[o+3] = 255
			ix0, iy0, ix1, iy1 = min(ix0, x), min(iy0, y), max(ix1, x), max(iy1, y)
		}
	}
	if ix1 < ix0 {
		return AlphaRect{}
	}
	return AlphaRect{ix0, iy0, ix1 + 1, iy1 + 1}
}

// DrawAlphaText = StringMask + BlendMask。回傳的 ink 是「alpha > 0 且在 dst（[0,W)×[0,H)）內」
// 的像素的緊外接框；沒有寫入任何像素時回空框（四欄皆 0）。其餘回傳值同 StringMask。
func DrawAlphaText(dst []byte, W, H int, f *AlphaFont, rs []rune, x, baseline int, layer AlphaLayer, c [3]uint8, clip AlphaRect) (ink AlphaRect, missing []rune, noStroke bool) {
	m, box, missing, noStroke := StringMask(f, rs, x, baseline, layer, clip)
	if len(missing) > 0 || noStroke {
		return AlphaRect{}, missing, noStroke
	}
	return blendMask(dst, W, H, m, box, c), nil, false
}
