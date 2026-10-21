package strikecommander

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 飛行字串的版面、背景、外框與出圖比對（strike_commander 規格 009 §3.6.5、§3.7、§3.8）。
// 每個倍率一個 FlightOverlay；座標是原版像素，GlyphX、GlyphY 是輸出像素，GlyphScale 為 1。

// FlightCover 是一筆字串的遮蓋範圍 [X0, X1) × [Y0, Y1)（原版像素，§3.11）。
type FlightCover struct {
	Key            string
	Kind           Kind
	X0, Y0, X1, Y1 int
}

// FlightEntry 是 Report 的一筆（§3.11）：遮蓋範圍加版面、字色、外框與前後列、Ref。
type FlightEntry struct {
	FlightCover
	CatKey                                      string
	Text                                        string
	K                                           int
	Xs                                          []int
	TextX0, TextX1, Y, B0, BH                   int // 原版的 X0、X1、Y 與 band
	FG                                          uint8
	Outline                                     bool
	TX, TY, Cells, CellW, CellH, GlyphX, GlyphY int // 文字 stamp
	Hidden                                      bool
	Before, After, Ref                          []byte

	// strike_commander 規格 011 §3.8 新增：Path 為 "new" 時上面的文字 stamp 欄不適用（flight.tsv 填 -）。
	// 座標是輸出像素；Ink 是最近一次 Draw 的文字框。
	Path                string
	Display             string
	Px, TxOut, Baseline int
	Clip, Ink           xlate.AlphaRect
	E0, E1              int
}

type flightEntry struct {
	idx      int // 在最近一次 Sync 的 texts 中的索引
	t        FlightText
	text     []rune
	cx0, cx1 int
	fg       uint8
	outline  bool
	cover    *xlate.Stamp
	stamp    *xlate.Stamp
	mask     []bool // 外框用的字形遮罩（重建時快取，§3.8 第 4 條）
	// ti 是 strike_commander 規格 011 新路徑的文字項（stamp、mask 為 nil）。
	ti *textItem
}

// FlightOverlay 依可見集合規劃飛行字串的 stamp 並出圖。
type FlightOverlay struct {
	hud   *HUDFont
	label *LabelFont
	cats  FlightCatalogs
	scale int

	gen     uint64
	built   bool
	entries []flightEntry
	lastBad []bool
	masks   map[string][]bool

	// OutlineAll 是樣張用的實驗旗標：MFD、橫幅也畫外框（§4.2 樣張；不進驗收）。建立後、第一次 Sync 前設定。
	// 新路徑不理會它（011 §3.9）。
	OutlineAll bool

	// Text 是 011 的圖集；非 nil 時走反鋸齒新路徑（NewFlightOverlayText）。lastInk 與 entries 平行。
	Text    *TextFonts
	lastInk []xlate.AlphaRect

	Hidden      []string
	Misses      []Miss
	Skips       []Skip
	HUDScaleOff bool
}

// NewFlightOverlay 建一個倍率的 overlay。scale 1 到 3；1 不產生 stamp，2 不產生 HUD stamp。
// label 不得為 nil；scale 3 時 hud 不得為 nil。
func NewFlightOverlay(hud *HUDFont, label *LabelFont, cats FlightCatalogs, scale int) (*FlightOverlay, error) {
	if scale < 1 || scale > 3 {
		return nil, fmt.Errorf("strikecommander: 飛行 overlay 倍率 %d 不支援", scale)
	}
	if label == nil || label.Font == nil {
		return nil, fmt.Errorf("strikecommander: 飛行 overlay 缺 16×16 字型")
	}
	if scale == 3 && (hud == nil || hud.Font == nil) {
		return nil, fmt.Errorf("strikecommander: 3 倍飛行 overlay 缺 HUD12 字型")
	}
	return &FlightOverlay{hud: hud, label: label, cats: cats, scale: scale, HUDScaleOff: scale != 3, masks: map[string][]bool{}}, nil
}

// Scale 回倍率。
func (o *FlightOverlay) Scale() int { return o.scale }

// Sync 在 generation 改變時依可見集合重建 stamp；generation 沒變時不重建（Misses、Skips 不再追加），
// 以這次的 texts（同索引、同種類與 catalog key）更新觀察字串、前後列、Ref、Xs、X1 與由它們算出的版面與字色。
func (o *FlightOverlay) Sync(texts []FlightText, gen uint64) {
	if o.built && gen == o.gen {
		for i := range o.entries {
			e := &o.entries[i]
			if e.idx >= len(texts) {
				continue
			}
			t := texts[e.idx]
			if t.Kind != e.t.Kind || t.CatKey != e.t.CatKey {
				continue
			}
			if ne, miss, skip := o.plan(e.idx, t); miss == nil && skip == nil {
				*e = ne
			}
		}
		return
	}
	o.built, o.gen = true, gen
	o.entries, o.lastBad, o.lastInk = nil, nil, nil
	for i, t := range texts {
		e, miss, skip := o.plan(i, t)
		switch {
		case miss != nil:
			o.Misses = append(o.Misses, *miss)
		case skip != nil:
			if skip.Reason != "" {
				o.Skips = append(o.Skips, *skip)
			}
		default:
			o.entries = append(o.entries, e)
		}
	}
}

// errNoStamp 表示不產生 stamp 且不記 Skip（1 倍、HUD 在 2 倍，§3.7.1）。
var errNoStamp = &Skip{}

// plan 規劃一筆的兩個 stamp。不產生 stamp 時回 miss 或 skip（skip.Reason 為空表示不記）。
func (o *FlightOverlay) plan(i int, t FlightText) (flightEntry, *Miss, *Skip) {
	if o.Text != nil {
		return o.planText(i, t)
	}
	if o.scale < 2 || (t.Kind == KindHUD && o.scale != 3) {
		return flightEntry{}, nil, errNoStamp
	}
	tr, ok := o.cats.Table(t.Kind)[t.CatKey]
	if !ok {
		return flightEntry{}, &Miss{Key: t.Key, Reason: MissTranslation}, nil
	}
	text := []rune(tr)
	if len(text) == 0 {
		return flightEntry{}, &Miss{Key: t.Key, Reason: MissTranslation}, nil
	}
	font, wide := o.label.Font, o.label.Wide
	if t.Kind == KindHUD {
		font, wide = o.hud.Font, o.hud.Wide
	}
	var miss []rune
	for _, r := range text {
		if !wide.Wide(r) || font.Glyphs[r] == nil {
			miss = append(miss, r)
		}
	}
	if len(miss) != 0 {
		return flightEntry{}, &Miss{Key: t.Key, Reason: MissGlyph, Runes: miss}, nil
	}
	if !validRows(t.Before, t.BH) || !validRows(t.After, t.BH) {
		return flightEntry{}, nil, &Skip{t.Key, "前後列缺或長度不符"}
	}
	g, why := flightLayout(t, len(text), o.scale)
	if why != "" {
		return flightEntry{}, nil, &Skip{t.Key, why}
	}
	fg, n := inkMode(t, g.cx0, g.cx1)
	if n == 0 {
		return flightEntry{}, nil, &Skip{t.Key, "遮蓋範圍內沒有墨跡"}
	}
	e := flightEntry{idx: i, t: t, text: text, cx0: g.cx0, cx1: g.cx1, fg: fg,
		outline: t.Kind == KindHUD || o.OutlineAll}
	e.cover = &xlate.Stamp{Key: t.Key + ".cover", X: g.cx0, Y: t.B0, Cells: 1, CellW: g.cx1 - g.cx0, CellH: t.BH, State: xlate.Shown}
	e.stamp = &xlate.Stamp{Key: t.Key + ".text", X: g.tx, Y: g.ty, Cells: len(text), CellW: g.cellW, CellH: g.cellH,
		Font: font, GlyphX: g.glyphX, GlyphY: g.glyphY, GlyphScale: 1, Text: append([]rune(nil), text...), State: xlate.Shown}
	if e.outline {
		mk := fmt.Sprintf("%s|%d|%d|%d|%d|%d|%d|%s", t.Kind, g.tx, g.ty, g.cellW, g.cellH, g.glyphX, g.glyphY, tr)
		m, ok := o.masks[mk]
		if !ok {
			m = glyphMask(e.stamp, o.scale)
			o.masks[mk] = m
		}
		e.mask = m
	}
	return e, nil, nil
}

// flightGeom 是一筆的版面（原版像素；glyphX、glyphY 是輸出像素）。
type flightGeom struct {
	tx, ty, cellW, cellH, glyphX, glyphY int
	cx0, cx1                             int
}

// hudRange 回 §3.7.1 表的左界 L、右界 R（含）。
func hudRange(t FlightText) (l, r int, ok bool) {
	switch t.CatKey {
	case "GEAR":
		return 183, 213, true
	case "FLAPS":
		return 183, 212, true
	case "BRAKE":
		return 183, 211, true
	case "DGFT", "STRF", "SRM":
		return 118, 149, true
	case "NONE":
		return t.Anchor(), 149, true
	}
	return 0, 0, false
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func clampInt(v, lo, hi int) int { return max(lo, min(v, hi)) }

// genericGlyphY 是 007 §3.9 的垂直通式：墨跡 [iy0, iy1] 的中心，band 起點 by0、列數 bh。
func genericGlyphY(iy0, iy1, by0, bh, scale int) int {
	return clampInt(floorDiv((iy0+iy1+1)*scale-glyphPx, 2)-by0*scale, 0, bh*scale-glyphPx)
}

// flightLayout 是 §3.7 的版面。n 是譯文字數。不產生 stamp 時回原因。
func flightLayout(t FlightText, n, scale int) (flightGeom, string) {
	var g flightGeom
	switch t.Kind {
	case KindHUD:
		l, r, ok := hudRange(t)
		if !ok {
			return g, fmt.Sprintf("HUD catalog key %q 沒有版面範圍", t.CatKey)
		}
		g.cellW, g.cellH = 4, 4
		w := n * g.cellW
		g.tx = t.Anchor()
		if g.tx+w-1 > r {
			if l >= t.X0 {
				return g, fmt.Sprintf("HUD 左欄譯文寬 %d 放不下（%d 到 %d）", w, g.tx, r)
			}
			g.tx = max(l, r+1-w)
			if g.tx+w-1 > r {
				return g, fmt.Sprintf("HUD 右欄譯文寬 %d 放不下（%d 到 %d）", w, l, r)
			}
		}
		g.ty = t.Y + 1
		g.cx0, g.cx1 = g.tx-1, max(t.X1, g.tx+w)+1
	case KindMFD:
		g.cellW, g.cellH = (glyphPx+scale-1)/scale, 8
		w := n * g.cellW
		l, r := 14, 100
		if t.X0 >= 160 {
			l, r = 219, 305
		}
		if w > r-l+1 {
			return g, fmt.Sprintf("MFD 譯文寬 %d 超過內區 %d 到 %d", w, l, r)
		}
		g.tx = t.Anchor()
		if g.tx+w-1 > r {
			g.tx = max(l, r+1-w)
		}
		g.ty = t.B0
		g.glyphX = (g.cellW*scale - glyphPx) / 2
		g.glyphY = genericGlyphY(t.Y+1, t.Y+5, t.Y-1, 8, scale)
		g.cx0, g.cx1 = min(t.Anchor(), g.tx), max(t.X1, g.tx+w)
	case KindPlaque:
		ix0, ix1, ok := inkXRange(t)
		if !ok {
			return g, "橫幅 band 內沒有墨跡"
		}
		g.cellW, g.cellH = (glyphPx+scale-1)/scale, 9
		w := n * g.cellW
		g.tx = floorDiv(ix0+ix1+1-w, 2)
		if g.tx < ix0 || g.tx+w > ix1+1 {
			return g, fmt.Sprintf("橫幅譯文寬 %d 超過英文墨跡 %d 到 %d", w, ix0, ix1)
		}
		g.ty = t.B0
		g.glyphX = (g.cellW*scale - glyphPx) / 2
		g.glyphY = genericGlyphY(t.Y, t.Y+4, t.Y-2, 9, scale)
		g.cx0, g.cx1 = min(ix0, g.tx), max(ix1+1, g.tx+w)
	default:
		return g, fmt.Sprintf("種類 %q 不是飛行種類", t.Kind)
	}
	g.cx0, g.cx1 = max(g.cx0, 0), min(g.cx1, 320)
	return g, ""
}

// inkXRange 回 band 內前列 ≠ 後列的點的 x 範圍 [ix0, ix1]（§3.7.3）。
func inkXRange(t FlightText) (ix0, ix1 int, ok bool) {
	ix0, ix1 = 320, -1
	for i := range t.After {
		if i < len(t.Before) && t.Before[i] != t.After[i] {
			x := i % 320
			ix0, ix1 = min(ix0, x), max(ix1, x)
		}
	}
	return ix0, ix1, ix1 >= 0
}

// inkMode 是 §3.8 第 2 條的字色：遮蓋範圍內、前列 ≠ 後列的點在後列的色號眾數，同數取小。n 是墨跡點數。
func inkMode(t FlightText, cx0, cx1 int) (fg uint8, n int) {
	var hist [256]int
	for r := 0; r < t.BH; r++ {
		for x := max(cx0, 0); x < min(cx1, 320); x++ {
			i := r*320 + x
			if t.Before[i] != t.After[i] {
				hist[t.After[i]]++
				n++
			}
		}
	}
	best := -1
	for c := 0; c < 256; c++ {
		if hist[c] > 0 && (best < 0 || hist[c] > hist[best]) {
			best = c
		}
	}
	if best < 0 {
		return 0, 0
	}
	return uint8(best), n
}

// flightMismatch 是 §3.6.5 的逐筆比對：遮蓋範圍（裁到畫面）內 indexed 逐點等於 Ref 的對應列（列位移 y − b0）。
// 不改既有的 mismatched（它寫死 y − Y − 1）。
func (o *FlightOverlay) flightMismatch(indexed []byte) []bool {
	bad := make([]bool, len(o.entries))
	for i, e := range o.entries {
		t := e.t
		if len(indexed) < 64000 || t.Ref == nil || len(t.Ref) != t.BH*320 {
			bad[i] = true
			continue
		}
	scan:
		for y := max(t.B0, 0); y < min(t.B0+t.BH, 200); y++ {
			row := (y - t.B0) * 320
			for x := e.cx0; x < e.cx1; x++ {
				if indexed[y*320+x] != t.Ref[row+x] {
					bad[i] = true
					break scan
				}
			}
		}
	}
	return bad
}

// Match 回比對不符的觀察字串（不出圖，不改 Hidden）。
func (o *FlightOverlay) Match(indexed []byte) []string {
	var out []string
	for i, b := range o.flightMismatch(indexed) {
		if b {
			out = append(out, o.entries[i].t.Key)
		}
	}
	return out
}

// Draw 在 out（原版放大圖，已畫過 OPTTEST 的 Overlay.Draw）上就地畫飛行字串（§3.8 第 5 條的合成順序）：
// 逐筆比對，不符的兩個 stamp 都不畫、記入 Hidden；照畫的以 labelBackground 換回墨跡（不畫外框），
// HUD（與實驗旗標下的 MFD、橫幅）以快取遮罩畫 0 號色外框，再畫文字 stamp。
func (o *FlightOverlay) Draw(out []byte, indexed []byte, palette [256][3]uint8) {
	bad := o.flightMismatch(indexed)
	o.lastBad = bad
	o.Hidden = nil
	if len(o.entries) == 0 || o.scale < 2 {
		o.lastInk = nil
		return
	}
	if o.Text != nil {
		o.drawText(out, indexed, palette, bad)
		return
	}
	var items []bgItem
	var shown []*flightEntry
	for i := range o.entries {
		e := &o.entries[i]
		if bad[i] {
			o.Hidden = append(o.Hidden, e.t.Key)
			continue
		}
		items = append(items, bgItem{label: e.idx, key: e.t.Key, kind: e.t.Kind,
			x0: e.cx0, x1: e.cx1, y0: e.t.B0, rows: e.t.BH, before: e.t.Before, after: e.t.After})
		shown = append(shown, e)
	}
	if len(shown) == 0 {
		return
	}
	bg := labelBackground(out, indexed, palette, o.scale, items, nil)
	for k, e := range shown {
		if e.outline {
			drawOutlineMask(bg, o.scale, palette[bgIndex], items[k], e.mask)
		}
	}
	layer := &xlate.Layer{W: 320, H: 200}
	for _, e := range shown {
		e.cover.BG, e.cover.FG = palette[bgIndex], palette[e.fg]
		e.stamp.BG, e.stamp.FG = palette[bgIndex], palette[e.fg]
		layer.Stamps = append(layer.Stamps, e.cover, e.stamp)
	}
	layer.DrawWithBackground(out, o.scale, func(rune) {}, bg, func(*xlate.Stamp) bool { return true })
}

// drawOutlineMask 是 §3.8 第 4 條的外框：字形遮罩 mask（glyphMask 在重建時的結果）以 8 鄰域膨脹一個輸出像素，
// 減去字形，落在遮蓋範圍輸出像素內者設成 c。既有的 drawOutline 不改（006 使用）。
func drawOutlineMask(bg []byte, scale int, c [3]uint8, it bgItem, mask []bool) {
	W, H := 320*scale, 200*scale
	if len(mask) != W*H {
		return
	}
	for y := max(it.y0*scale, 0); y < min((it.y0+it.rows)*scale, H); y++ {
		for x := max(it.x0*scale, 0); x < min(it.x1*scale, W); x++ {
			if mask[y*W+x] {
				continue
			}
			hit := false
			for dy := -1; dy <= 1 && !hit; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if (dx != 0 || dy != 0) && nx >= 0 && nx < W && ny >= 0 && ny < H && mask[ny*W+nx] {
						hit = true
						break
					}
				}
			}
			if hit {
				i := 4 * (y*W + x)
				bg[i], bg[i+1], bg[i+2], bg[i+3] = c[0], c[1], c[2], 255
			}
		}
	}
}

// Covers 回目前每筆（含 Hidden）的遮蓋範圍，供 CheckComposite 的允許區。
func (o *FlightOverlay) Covers() []FlightCover {
	var out []FlightCover
	for _, e := range o.entries {
		out = append(out, FlightCover{Key: e.t.Key, Kind: e.t.Kind, X0: e.cx0, Y0: e.t.B0, X1: e.cx1, Y1: e.t.B0 + e.t.BH})
	}
	return out
}

// Report 依重建順序列出每筆（§3.11 的 FlightEntry），Hidden 取最近一次 Draw。切片是複本。
func (o *FlightOverlay) Report() []FlightEntry {
	var out []FlightEntry
	for i, e := range o.entries {
		t := e.t
		fe := FlightEntry{
			FlightCover: FlightCover{Key: t.Key, Kind: t.Kind, X0: e.cx0, Y0: t.B0, X1: e.cx1, Y1: t.B0 + t.BH},
			CatKey:      t.CatKey, Text: string(e.text), K: t.K, Xs: append([]int(nil), t.Xs...),
			TextX0: t.X0, TextX1: t.X1, Y: t.Y, B0: t.B0, BH: t.BH, FG: e.fg, Outline: e.outline,
			Before: append([]byte(nil), t.Before...), After: append([]byte(nil), t.After...), Ref: append([]byte(nil), t.Ref...),
			Path: "old",
		}
		if ti := e.ti; ti != nil {
			// 011 §3.8：新路徑沒有文字 stamp，填新欄。
			fe.Path, fe.Display, fe.Px, fe.TxOut, fe.Baseline, fe.Clip, fe.E0, fe.E1 = "new", string(ti.display), ti.px, ti.tx, ti.baseline, ti.clip, ti.e0, ti.e1
			if i < len(o.lastInk) {
				fe.Ink = o.lastInk[i]
			}
		} else {
			fe.TX, fe.TY, fe.Cells, fe.CellW, fe.CellH = e.stamp.X, e.stamp.Y, e.stamp.Cells, e.stamp.CellW, e.stamp.CellH
			fe.GlyphX, fe.GlyphY = e.stamp.GlyphX, e.stamp.GlyphY
		}
		if i < len(o.lastBad) {
			fe.Hidden = o.lastBad[i]
		}
		out = append(out, fe)
	}
	return out
}
