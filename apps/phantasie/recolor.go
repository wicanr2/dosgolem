package phantasie

import (
	"github.com/wicanr2/dosgolem/xlate"
)

// 定色與顯示（docs/spec/001 §8）。顏色由原文字模遮罩決定，不用 xlate.Colors 的多數色規則：
// 原版字模是粗體，單格或少數格的墨點可超過一半，「最多的當背景」會把前景與背景對調。

const (
	gateMinPixels   = 24 // 閘門：原版格被非透明格覆蓋的像素少於此數時略過
	gateMinInk      = 6  // 閘門：墨像素不少於此數才檢查墨保留率
	gateSpaceMinPct = 85 // 閘門：全是空白字元的格，同一色號像素的最低比例
)

// glyphPixel 回原版 FONT 中字元 g 在 (掃描線 r, 欄 c) 的像素值（0 為背景，非 0 為墨）；沒有字模時回 0。
// 字模 g 佔 16 bytes：掃描線 r 是 FONT[g×16+2r]、FONT[g×16+2r+1]，高位元在左，每像素 2 位元。
func (o *Overlay) glyphPixel(g byte, r, c int) uint8 {
	if int(g) >= 127 || r < 0 || r > 7 || c < 0 || c > 7 {
		return 0
	}
	b := o.font[int(g)*16+2*r+c/4]
	return (b >> (uint(3-c%4) * 2)) & 3
}

// scanGroup 對事件組的每個像素呼叫 f（k 是原版格序號、m 是遮罩值、color 是畫面色號、x,y 是像素座標）。
// 只計入屬於某筆疊字非透明格範圍、且在事件矩形內的像素（像素粒度，與疊字格和原版格是否對齊無關）。
// y0 是事件組的實際列（取疊字的 Y）；各疊字的 Y 不一致時回 false。
func (o *Overlay) scanGroup(rec *EventRecord, stamps []*xlate.Stamp, indexed []uint8, f func(k int, m, color uint8, x, y int)) bool {
	if len(stamps) == 0 || o.font == nil {
		return false
	}
	y0 := stamps[0].Y
	for _, s := range stamps {
		if s.Y != y0 {
			return false
		}
	}
	ex0 := rec.Col * 8
	ex1 := ex0 + len(rec.Cells)*8
	if ex1 > screenW {
		ex1 = screenW
	}
	for _, s := range stamps {
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			cx0, cx1 := s.X+i*s.CellW, s.X+(i+1)*s.CellW
			if cx0 < ex0 {
				cx0 = ex0
			}
			if cx1 > ex1 {
				cx1 = ex1
			}
			for y := y0; y < y0+8 && y < screenH; y++ {
				if y < 0 {
					continue
				}
				for x := cx0; x < cx1; x++ {
					k := (x - ex0) / 8
					g := rec.Cells[k]
					m := o.glyphPixel(g, y-y0, (x-ex0)%8)
					f(k, m, indexed[y*screenW+x], x, y)
				}
			}
		}
	}
	return true
}

func argmax(h *[256]int) int {
	best, bi := -1, 0
	for i, v := range h {
		if v > best {
			best, bi = v, i
		}
	}
	return bi
}

// Frame 是前端每個呈現幀的入口：先 Layer.Frame（定色與指紋偵測），再對本次由 Pending 轉為 Shown 的疊字
// 所屬的每個事件組執行 recolor（001 §8）。Layer.Add、Clear、Frame 以 l.Stamps[:0] 就地過濾，
// 所以先複製 Pending 疊字的指標（001 §7 實作注意）。indexed 是色號畫面，rgb 是同一幀的 RGB（每像素 3 bytes）。
func (o *Overlay) Frame(indexed, rgb []uint8) {
	var pend []*xlate.Stamp
	for _, s := range o.Layer.Stamps {
		if s.State == xlate.Pending {
			pend = append(pend, s)
		}
	}
	o.Layer.Frame(indexed, rgb)
	seen := map[string]bool{}
	gated := map[string]bool{}
	var keys []string
	for _, s := range pend {
		if s.State != xlate.Shown {
			continue
		}
		if _, ok := o.gate[s]; ok {
			gated[s.Key] = true
		}
		if !seen[s.Key] {
			seen[s.Key] = true
			keys = append(keys, s.Key)
		}
	}
	for _, key := range keys {
		o.recolor(key, indexed, rgb, gated[key])
	}
	if len(o.gate) > 0 {
		o.gate = map[*xlate.Stamp]struct{}{}
	}
}

// recolor 對一個事件組以字模遮罩定 BG、FG（001 §8 步驟 1 至 5）。gated 為真時（restoreShadow 產生的 Pending 疊字）
// 另執行有效性閘門。
func (o *Overlay) recolor(key string, indexed, rgb []uint8, gated bool) {
	rec := o.records[key]
	stamps := o.groupStamps(key)
	if rec == nil || len(stamps) == 0 || o.font == nil {
		return
	}
	var bgH, fgH [256]int
	nbg, nfg := 0, 0
	ok := o.scanGroup(rec, stamps, indexed, func(_ int, m, c uint8, _, _ int) {
		if m == 0 {
			bgH[c]++
			nbg++
		} else {
			fgH[c]++
			nfg++
		}
	})
	if !ok || nfg == 0 {
		o.C.Inc("recolor_fallback")
		return
	}
	bgIdx, fgIdx := uint8(argmax(&bgH)), uint8(argmax(&fgH))
	if bgIdx == fgIdx {
		if gated {
			total, same := nbg+nfg, bgH[bgIdx]+fgH[bgIdx]
			if 10*same >= 9*total {
				o.removeGroup(key)
				o.C.Inc("inconsistent_groups")
				return
			}
		}
		o.C.Inc("recolor_fallback")
		return
	}
	if gated && o.gateGroup(key, rec, stamps, indexed, bgIdx, fgIdx) {
		return
	}
	var bg, fg [3]uint8
	foundBG, foundFG := false, false
	o.scanGroup(rec, stamps, indexed, func(_ int, _, c uint8, x, y int) {
		i := 3 * (y*screenW + x)
		if c == bgIdx && !foundBG {
			bg, foundBG = [3]uint8{rgb[i], rgb[i+1], rgb[i+2]}, true
		}
		if c == fgIdx && !foundFG {
			fg, foundFG = [3]uint8{rgb[i], rgb[i+1], rgb[i+2]}, true
		}
	})
	if !foundBG || !foundFG {
		o.C.Inc("recolor_fallback")
		return
	}
	for _, s := range stamps {
		s.BG, s.FG = bg, fg
	}
}

type cellStat struct {
	n, match, ink, inkKeep int
	color                  [256]int
}

// gateGroup 是有效性閘門（001 §8 步驟 3）：以原版格為單位檢查畫面是否仍是該原文的字模畫出的結果，
// 不一致的格標 Transparent。回傳 true 表示整組已被移除。
func (o *Overlay) gateGroup(key string, rec *EventRecord, stamps []*xlate.Stamp, indexed []uint8, bgIdx, fgIdx uint8) bool {
	cells := make([]cellStat, len(rec.Cells))
	o.scanGroup(rec, stamps, indexed, func(k int, m, c uint8, _, _ int) {
		cs := &cells[k]
		cs.n++
		cs.color[c]++
		if m == 0 && c == bgIdx {
			cs.match++
		}
		if m != 0 {
			cs.ink++
			if c == fgIdx {
				cs.match++
				cs.inkKeep++
			}
		}
	})
	var bad []int
	for k := range cells {
		cs := &cells[k]
		if cs.n < gateMinPixels {
			continue
		}
		if rec.Cells[k] == ' ' {
			mx := 0
			for _, v := range cs.color {
				if v > mx {
					mx = v
				}
			}
			if 100*mx < gateSpaceMinPct*cs.n {
				bad = append(bad, k)
			}
			continue
		}
		if 10*cs.match < 7*cs.n {
			bad = append(bad, k)
			continue
		}
		if cs.ink >= gateMinInk && 10*cs.inkKeep < 3*cs.ink {
			bad = append(bad, k)
		}
	}
	if len(bad) == 0 {
		return false
	}
	ex0 := rec.Col * 8
	for _, k := range bad {
		o.C.Inc("inconsistent_cells")
		px0, px1 := ex0+k*8, ex0+k*8+8
		for _, s := range stamps {
			for i := 0; i < s.Cells; i++ {
				cx0, cx1 := s.X+i*s.CellW, s.X+(i+1)*s.CellW
				if cx0 < px1 && px0 < cx1 {
					setTransparent(s, i)
				}
			}
		}
	}
	all := true
	for _, s := range stamps {
		for i := 0; i < s.Cells; i++ {
			if i >= len(s.Transparent) || !s.Transparent[i] {
				all = false
			}
		}
	}
	if all {
		o.removeGroup(key)
		o.C.Inc("inconsistent_groups")
		return true
	}
	return false
}

// setTransparent 把疊字第 i 格標成透明（陣列不夠長就補）。
func setTransparent(s *xlate.Stamp, i int) {
	if len(s.Transparent) < s.Cells {
		t := make([]bool, s.Cells)
		copy(t, s.Transparent)
		s.Transparent = t
	}
	s.Transparent[i] = true
}
