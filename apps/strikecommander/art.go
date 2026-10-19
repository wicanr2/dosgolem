package strikecommander

import "fmt"

// 存讀檔對話框 SAVELOAD.SHP 的元素、字帶、墨跡色與無字底（strike_commander 規格 004 §2.2、§3.4、§3.5；
// 確認框 k15 到 k20 與 SAVE／LOAD 字帶下移見規格 007 §3.2 到 §3.5、§3.9）。座標都是 1×、相對元素左上角、含端點。

// box 是含端點的矩形。
type box struct{ x0, y0, x1, y1 int }

func (b box) in(x, y int) bool { return x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 }

// shift 回下移 dy 列的矩形。
func (b box) shift(dy int) box { return box{b.x0, b.y0 + dy, b.x1, b.y1 + dy} }

// artElement 是 004 §2.2 表（與 007 §3.2 表）的一列（一般態）。按下態用一般態那一列，墨跡框下移 1 列。
type artElement struct {
	n    int    // 一般態的 k
	key  string // 圖形字 catalog 的 key（英文字面，004 §3.6）
	w, h int    // 元素尺寸
	band box    // 字帶
	ink  box    // 墨跡框
}

// artElements 是 004 §2.2 與 007 §3.2 的表，以一般態 k 為索引。k1 是一般態（LOAD GAME 標題條），不是 k0 的按下態。
// k2、k4 的字帶是 y 2 到 10（007 §3.9：確認框 k15 的底列 y 128 壓到原本的第 1 列）。
// k15 的字帶只含提示字 ARE YOU SURE；框內自帶的 YES、NO 不設字帶（畫面上那裡是 k17、k19，007 §3.2）。
var artElements = map[int]artElement{
	0:  {0, "SAVE GAME", 182, 131, box{44, 6, 139, 17}, box{47, 9, 133, 14}},
	1:  {1, "LOAD GAME", 107, 12, box{8, 0, 103, 11}, box{11, 3, 102, 8}},
	2:  {2, "SAVE", 75, 12, box{20, 2, 59, 10}, box{24, 3, 51, 7}},
	4:  {4, "LOAD", 75, 12, box{20, 2, 59, 10}, box{22, 3, 53, 7}},
	6:  {6, "CANCEL", 80, 12, box{12, 1, 67, 9}, box{15, 3, 63, 7}},
	8:  {8, "QUIT TO DOS", 80, 12, box{2, 1, 77, 9}, box{3, 3, 74, 7}},
	15: {15, "ARE YOU SURE", 132, 44, box{18, 11, 112, 19}, box{24, 13, 106, 17}},
	16: {16, "OVERWRITE", 95, 9, box{0, 0, 94, 8}, box{14, 2, 78, 6}},
	17: {17, "YES", 46, 12, box{4, 1, 41, 9}, box{14, 3, 34, 7}},
	19: {19, "NO", 44, 12, box{4, 1, 39, 9}, box{15, 3, 30, 7}},
}

// pressedState 是按下態的 k 集合 {3, 5, 7, 9, 18, 20}（004 §2.2、007 §3.2）。k18 是 k17 的、k20 是 k19 的。
// 不以奇偶判斷（k1、k15 是一般態）。
func pressedState(k int) bool {
	return k == 3 || k == 5 || k == 7 || k == 9 || k == 18 || k == 20
}

// confirmElement 回一般態 n 是否為確認框的元素 {15, 16, 17, 19}（007 §3.2）。
func confirmElement(n int) bool { return n == 15 || n == 16 || n == 17 || n == 19 }

// normalK 回 k 的一般態：按下態取 k−1，其餘取 k（004 §3.3 的 Key）。
func normalK(k int) int {
	if pressedState(k) {
		return k - 1
	}
	return k
}

// elementFor 回 k 的元素列與該態的墨跡框（按下態下移 1 列）。k 不在 0 到 9、15 到 20 時回 false。
func elementFor(k int) (artElement, box, bool) {
	el, ok := artElements[normalK(k)]
	if !ok {
		return artElement{}, box{}, false
	}
	ink := el.ink
	if pressedState(k) {
		ink = ink.shift(1)
	}
	return el, ink, true
}

// ArtCatalogKeys 回對照中的 10 個圖形字 key，依一般態 k 排序（LintArt 用；004 §3.6、007 §3.7）。
func ArtCatalogKeys() []string {
	var out []string
	for _, n := range []int{0, 1, 2, 4, 6, 8, 15, 16, 17, 19} {
		out = append(out, artElements[n].key)
	}
	return out
}

// inkColors0 是 0 組的墨跡色 F（004 §3.4）：RE 015 §5 的筆畫色與反鋸齒 15，加上 198。
// 勘誤（主代理裁定，2026-10-19）後 F 只用於字色 c 的計數，不再用於判定墨跡像素。
var inkColors0 = [256]bool{15: true, 20: true, 26: true, 34: true, 45: true, 49: true, 58: true, 66: true,
	74: true, 82: true, 93: true, 198: true, 199: true, 200: true}

// highlight0 是 0 組的高光色號（004 §3.5）。
const highlight0 = 15

// bgTitle、bgButton 是 0 組的底色集合 B（004 勘誤：墨跡像素改為底色補集）。
// 標題 k0、k1 的底是 13 夾 9 的刷紋；按鈕 k2 到 k9 另有 8（RE 015 §5、shape06-grid.txt）。
// bgConfirm 是確認框 k15 到 k20 的 B（007 §3.3）：4、5 是貫穿墨跡框內外的亮帶（底紋，不是字內高光），
// 10 是板面底色。四個元素 0 組墨跡框內 F 以外的色號只有這五個。
var (
	bgTitle   = [256]bool{9: true, 13: true}
	bgButton  = [256]bool{8: true, 9: true, 13: true}
	bgConfirm = [256]bool{4: true, 5: true, 8: true, 9: true, 10: true}
)

// bgColors 回一般態 n 的底色集合 B。
func bgColors(n int) *[256]bool {
	switch {
	case n == 0 || n == 1:
		return &bgTitle
	case confirmElement(n):
		return &bgConfirm
	}
	return &bgButton
}

// judgeBox 回 k 的墨跡判定框（004 勘誤）：按鈕與確認框（007 §3.3）是該態的墨跡框；標題 k0、k1 的墨跡框往下延 1 列，
// 含字下的陰影列（色號 8、4）。k0 右緣另延到 134，含 E 字右側落在墨跡框外的陰影（第二次勘誤）。
// k0 為 (47,9)–(134,15)、k1 為 (11,3)–(102,9)。
func judgeBox(k int) box {
	_, ink, _ := elementFor(k)
	if n := normalK(k); n == 0 || n == 1 {
		ink.y1++
		if n == 0 {
			ink.x1 = 134
		}
	}
	return ink
}

// inkMask 依 004 §3.4（勘誤後）回元素上的墨跡像素（w×h，逐列）：0 組條目 base0 在判定框內、
// 色號不屬於底色集合 B 的位置。21 組的墨跡像素是同一批位置（依位置認定，不換算色號），所以兩組共用這份遮罩。
func inkMask(base0 []uint8, w, h int, ink box, bg *[256]bool) []bool {
	m := make([]bool, w*h)
	for y := ink.y0; y <= ink.y1; y++ {
		for x := ink.x0; x <= ink.x1; x++ {
			if x >= 0 && x < w && y >= 0 && y < h && !bg[base0[y*w+x]] {
				m[y*w+x] = true
			}
		}
	}
	return m
}

// inpaint 回字帶的無字底（band 寬×高，逐列）與退回原規則的墨跡位置（元素索引）。
// 墨跡像素換成同一列、字帶內、非墨跡、而且 0 組條目 base0 色號屬於 bg 的最近位置的色號（21 組取同位置
// 的色號），左右等距取左；該列找不到這種位置時退回同一列最近的非墨跡像素（004 §3.4 規則 4），
// 並記入回傳的退回清單。其他像素不變。找鄰點時看原圖，不看已回填的像素。
func inpaint(entry, base0 []uint8, mask []bool, w int, band box, bg *[256]bool) ([]uint8, []int) {
	bw, bh := band.x1-band.x0+1, band.y1-band.y0+1
	out := make([]uint8, bw*bh)
	var fallback []int
	near := func(ey, ex int, ok func(i int) bool) (uint8, bool) {
		for d := 1; d < bw; d++ {
			if l := ex - d; l >= band.x0 && ok(ey*w+l) {
				return entry[ey*w+l], true
			}
			if r := ex + d; r <= band.x1 && ok(ey*w+r) {
				return entry[ey*w+r], true
			}
		}
		return 0, false
	}
	for y := 0; y < bh; y++ {
		ey := band.y0 + y
		for x := 0; x < bw; x++ {
			ex := band.x0 + x
			c := entry[ey*w+ex]
			if mask[ey*w+ex] {
				if v, ok := near(ey, ex, func(i int) bool { return !mask[i] && bg[base0[i]] }); ok {
					c = v
				} else {
					fallback = append(fallback, ey*w+ex)
					if v, ok := near(ey, ex, func(i int) bool { return !mask[i] }); ok {
						c = v
					}
				}
			}
			out[y*bw+x] = c
		}
	}
	return out, fallback
}

// mostColor 回 cs 中出現最多的色號，同數取色號小者；cs 空時回 false。
func mostColor(count *[256]int) (uint8, bool) {
	best := -1
	for c := 0; c < 256; c++ {
		if count[c] > 0 && (best < 0 || count[c] > count[best]) {
			best = c
		}
	}
	return uint8(best), best >= 0
}

// textColors 依 004 §3.5 與 007 §3.4 回字色 c、高光 h 與是否畫高光 hasH。
//   - c：墨跡像素中 0 組色號屬於 F 的位置，本條目同位置色號出現最多者（勘誤：F 只用於這裡）。
//   - h：「0 組墨跡框內色號 15 的位置」在本條目同位置色號中最多者；0 組即 15。
//   - 0 組墨跡框內沒有 15（k19、k20 的 NO）時不畫高光，0 組與 21 組都適用（007 §3.4）。
func textColors(entry, base0 []uint8, mask []bool, w int, ink box) (c, h uint8, hasH bool) {
	var cc [256]int
	for i, on := range mask {
		if on && inkColors0[base0[i]] {
			cc[entry[i]]++
		}
	}
	c, _ = mostColor(&cc)
	var hc [256]int
	for y := ink.y0; y <= ink.y1; y++ {
		for x := ink.x0; x <= ink.x1; x++ {
			if base0[y*w+x] == highlight0 {
				hc[entry[y*w+x]]++
			}
		}
	}
	h, hasH = mostColor(&hc)
	return c, h, hasH
}

// glyphOrigin 依 004 §3.5 與 007 §3.9 回第一個字模的左上角（相對字帶，輸出像素）與每字的步距。
//   - 每字格寬 cellW = ceil(16 / scale) 原版像素，字模在格內水平置中（同 PlanLabel 的 GlyphX）；
//   - 文字寬 W = n × cellW × scale，tx = floor(((x0 + x1 + 1) × scale − W) / 2)，x0、x1 是墨跡框相對字帶；
//   - 垂直用 007 §3.9 的通式（取代 004 的公式）：ty = clamp(floor(((iy0 + iy1 + 1) × scale − 16) / 2) − by0 × scale,
//     0, 字帶高 × scale − 16)。iy0、iy1 是該態的墨跡框（按下態已下移，不再另加 scale），by0 是字帶起始列。
//
// 文字寬超過字帶寬回錯（記 Skip）。tx 夾在字帶內（實作細節：現行譯文不會用到）。
func glyphOrigin(n, scale int, band, ink box) (gx, gy, step int, err error) {
	cellW := (glyphPx + scale - 1) / scale
	step = cellW * scale
	W := n * step
	bw, bh := (band.x1-band.x0+1)*scale, (band.y1-band.y0+1)*scale
	if W > bw {
		return 0, 0, 0, fmt.Errorf("譯文 %d 字寬 %d 輸出像素超過字帶寬 %d", n, W, bw)
	}
	if bh < glyphPx {
		return 0, 0, 0, fmt.Errorf("字帶高 %d 輸出像素放不下 16 像素字模", bh)
	}
	x0, x1 := ink.x0-band.x0, ink.x1-band.x0
	tx := ((x0+x1+1)*scale - W) / 2
	tx = max(0, min(tx, bw-W))
	// (iy0 + iy1 + 1) × scale − 16 對表中每一列都是正數（最小是 k16 2 倍的 2），Go 的 / 向零取整即 floor。
	ty := ((ink.y0+ink.y1+1)*scale-glyphPx)/2 - band.y0*scale
	ty = max(0, min(ty, bh-glyphPx))
	return tx + (step-glyphPx)/2, ty, step, nil
}

// LintArt 依 004 §3.6 檢查圖形字 catalog（text/menu_gfx.<lang>.tsv）：欄名、欄數、key 唯一；
// 對照中的 10 個 key（007 §3.7）都在、譯文非空、每字都是字型收入的全形字。catalog 其餘 key（主選單）不報錯。
// 不走 Lint：它會把 LABL 以外的 key 全報孤兒。
func LintArt(b []byte, wide *WideTable) LintReport {
	var rep LintReport
	bad := func(format string, a ...any) { rep.Errors = append(rep.Errors, fmt.Sprintf(format, a...)) }
	header, rows, err := splitCatalog(b)
	if err != nil {
		bad("%v", err)
		return rep
	}
	if header != catalogHeader {
		bad("第 1 列欄名要是 key、translation、source")
	}
	seen := map[string]catalogRow{}
	for _, r := range rows {
		if r.cols != 3 {
			bad("第 %d 列欄數 %d，要 3", r.line, r.cols)
			continue
		}
		if first, dup := seen[r.key]; dup {
			bad("第 %d 列 key %q 與第 %d 列重複", r.line, r.key, first.line)
			continue
		}
		seen[r.key] = r
	}
	for _, k := range ArtCatalogKeys() {
		r, ok := seen[k]
		if !ok {
			bad("缺圖形字 key %q", k)
			continue
		}
		if r.tr == "" {
			bad("第 %d 列 key %q 譯文是空的", r.line, k)
			continue
		}
		for _, c := range r.tr {
			switch {
			case !wide.Has(c):
				bad("第 %d 列 key %q 缺字 %q（U+%04X）", r.line, k, c, c)
			case !wide.Wide(c):
				bad("第 %d 列 key %q 含半形字 %q（U+%04X）", r.line, k, c, c)
			}
		}
	}
	return rep
}

// dialogMask 回事件條目的墨跡遮罩與該態的元素、墨跡框（004 §3.4 與勘誤）：兩組都由 0 組條目 Base0
// 在判定框內依位置認定。回傳的墨跡框是 §2.2 的框（高光 h 與水平置中仍用它）。
func dialogMask(ev ArtEvent) (artElement, box, []bool) {
	el, ink, _ := elementFor(ev.K)
	return el, ink, inkMask(ev.Base0, el.w, el.h, judgeBox(ev.K), bgColors(el.n))
}

// dialogBackground 回事件條目在字帶內的無字底（字帶寬×高，逐列，004 §3.4）。
func dialogBackground(ev ArtEvent) []uint8 {
	bgPix, _ := dialogBackgroundFallback(ev)
	return bgPix
}

// dialogBackgroundFallback 同 dialogBackground，另回退回原規則 4 的墨跡位置（元素索引），供測試記 log。
func dialogBackgroundFallback(ev ArtEvent) ([]uint8, []int) {
	el, _, mask := dialogMask(ev)
	return inpaint(ev.Entry, ev.Base0, mask, el.w, el.band, bgColors(el.n))
}

// dialogColors 回事件條目的字色 c、高光 h 與是否畫高光（004 §3.5、007 §3.4）。
func dialogColors(ev ArtEvent) (c, h uint8, hasH bool) {
	el, ink, mask := dialogMask(ev)
	return textColors(ev.Entry, ev.Base0, mask, el.w, ink)
}
