package strikecommander

import "fmt"

// 舊路徑（16×16 與 HUD12 的 1 位元字模）的幾何算式，strike_commander 規格 011 §3.11：(g) 刪除生產碼之前
// 原樣複製到這裡，供單元測試對照（例如 001 §2.6、003 §3.4、009 §3.7 的數值例）。生產碼不再呼叫。

const legacyGlyphPx = 16

// legacyGeom 是舊文字 stamp 的幾何：文字 stamp 的 X、Y、格寬、格高、GlyphX、GlyphY（GlyphX、GlyphY 是輸出像素），
// 與遮蓋範圍 [CX0, CX1)（原版像素）。
type legacyGeom struct {
	X, Y, CellW, CellH, GlyphX, GlyphY int
	CX0, CX1                           int
}

// legacyLabelGeom 是舊 PlanLabel 的幾何（001 §2.6 label、003 §3.4 board）：n 字譯文、scale 2 或 3。
func legacyLabelGeom(l Label, n, scale int) (legacyGeom, error) {
	if scale != 2 && scale != 3 {
		return legacyGeom{}, fmt.Errorf("%d 倍輸出不產生 stamp", scale)
	}
	if n == 0 {
		return legacyGeom{}, fmt.Errorf("譯文是空的")
	}
	cellW := (legacyGlyphPx + scale - 1) / scale
	w := n * cellW
	if w > 320 {
		return legacyGeom{}, fmt.Errorf("譯文 %d 字寬 %d 超過 320", n, w)
	}
	y := l.Y + 1
	if l.Kind == KindBoard {
		x := max(0, l.X1-w)
		g := legacyGeom{X: x, Y: y, CellW: cellW, CellH: boardCellH,
			GlyphX: (cellW*scale - legacyGlyphPx) / 2, GlyphY: (boardCellH*scale - legacyGlyphPx) / 2}
		g.CX0, g.CX1 = max(min(l.X0, x), 0), min(max(l.X1, x+w), 320)
		return g, nil
	}
	x := (l.X0+l.X1)/2 - w/2
	if x < 0 {
		x = 0
	}
	if x+w > 320 {
		x = 320 - w
	}
	g := legacyGeom{X: x, Y: y, CellW: cellW, CellH: stampCellH,
		GlyphX: (cellW*scale - legacyGlyphPx) / 2, GlyphY: (stampCellH*scale - legacyGlyphPx) / 2}
	g.CX0, g.CX1 = max(min(l.X0, x), 0), min(max(l.X1, x+w), 320)
	return g, nil
}

// legacyGenericGlyphY 是 007 §3.9 的垂直通式（16 像素字模）。
func legacyGenericGlyphY(iy0, iy1, by0, bh, scale int) int {
	return clampInt(floorDiv((iy0+iy1+1)*scale-legacyGlyphPx, 2)-by0*scale, 0, bh*scale-legacyGlyphPx)
}

// legacyFlightGeom 是舊 flightLayout（009 §3.7）：n 是譯文字數。不產生 stamp 時回原因。
func legacyFlightGeom(t FlightText, n, scale int) (legacyGeom, string) {
	var g legacyGeom
	switch t.Kind {
	case KindHUD:
		l, r, ok := hudRange(t)
		if !ok {
			return g, fmt.Sprintf("HUD catalog key %q 沒有版面範圍", t.CatKey)
		}
		g.CellW, g.CellH = 4, 4
		w := n * g.CellW
		g.X = t.Anchor()
		if g.X+w-1 > r {
			if l >= t.X0 {
				return g, fmt.Sprintf("HUD 左欄譯文寬 %d 放不下（%d 到 %d）", w, g.X, r)
			}
			g.X = max(l, r+1-w)
			if g.X+w-1 > r {
				return g, fmt.Sprintf("HUD 右欄譯文寬 %d 放不下（%d 到 %d）", w, l, r)
			}
		}
		g.Y = t.Y + 1
		g.CX0, g.CX1 = g.X-1, max(t.X1, g.X+w)+1
	case KindMFD:
		g.CellW, g.CellH = (legacyGlyphPx+scale-1)/scale, 8
		w := n * g.CellW
		l, r := 14, 100
		if t.X0 >= 160 {
			l, r = 219, 305
		}
		if w > r-l+1 {
			return g, fmt.Sprintf("MFD 譯文寬 %d 超過內區 %d 到 %d", w, l, r)
		}
		g.X = t.Anchor()
		if g.X+w-1 > r {
			g.X = max(l, r+1-w)
		}
		g.Y = t.B0
		g.GlyphX = (g.CellW*scale - legacyGlyphPx) / 2
		g.GlyphY = legacyGenericGlyphY(t.Y+1, t.Y+5, t.Y-1, 8, scale)
		g.CX0, g.CX1 = min(t.Anchor(), g.X), max(t.X1, g.X+w)
	case KindPlaque:
		ix0, ix1, ok := inkXRange(t)
		if !ok {
			return g, "橫幅 band 內沒有墨跡"
		}
		g.CellW, g.CellH = (legacyGlyphPx+scale-1)/scale, 9
		w := n * g.CellW
		g.X = floorDiv(ix0+ix1+1-w, 2)
		if g.X < ix0 || g.X+w > ix1+1 {
			return g, fmt.Sprintf("橫幅譯文寬 %d 超過英文墨跡 %d 到 %d", w, ix0, ix1)
		}
		g.Y = t.B0
		g.GlyphX = (g.CellW*scale - legacyGlyphPx) / 2
		g.GlyphY = legacyGenericGlyphY(t.Y, t.Y+4, t.Y-2, 9, scale)
		g.CX0, g.CX1 = min(ix0, g.X), max(ix1+1, g.X+w)
	default:
		return g, fmt.Sprintf("種類 %q 不是飛行種類", t.Kind)
	}
	g.CX0, g.CX1 = max(g.CX0, 0), min(g.CX1, 320)
	return g, ""
}
