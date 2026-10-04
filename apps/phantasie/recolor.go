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
	mask := maskCells(rec)
	ex1 := ex0 + len(mask)*8
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
					g := mask[k]
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

// majorityPair 對事件的每個原版格各自求 (底, 墨) 色號配對（遮罩為 0 與非 0 的像素各取最多數的色號），
// 回出現最多的配對（同數取最左格者）。底與墨同色或沒有墨像素的格不計。最多數配對與其對調配對（反白狀態）
// 合計要涵蓋至少一半有墨的格，否則視為沒有可靠的組色（例如整組幾乎同一色，只有個別格有雜訊）。
// scan 對每個像素呼叫一次 g。
func majorityPair(n int, scan func(g func(k int, m, c uint8))) (bg, fg uint8, ok bool) {
	type hist struct {
		bg, fg   [256]int
		nbg, nfg int
	}
	hs := make([]hist, n)
	scan(func(k int, m, c uint8) {
		if k < 0 || k >= n {
			return
		}
		if m == 0 {
			hs[k].bg[c]++
			hs[k].nbg++
		} else {
			hs[k].fg[c]++
			hs[k].nfg++
		}
	})
	count := map[[2]uint8]int{}
	var order [][2]uint8
	inked := 0
	for k := range hs {
		if hs[k].nbg == 0 || hs[k].nfg == 0 {
			continue
		}
		inked++
		p := [2]uint8{uint8(argmax(&hs[k].bg)), uint8(argmax(&hs[k].fg))}
		if p[0] == p[1] {
			continue
		}
		if count[p] == 0 {
			order = append(order, p)
		}
		count[p]++
	}
	best := 0
	var bp [2]uint8
	for _, p := range order {
		if count[p] > best {
			best, bp = count[p], p
		}
	}
	if best == 0 || 2*(best+count[[2]uint8{bp[1], bp[0]}]) < inked {
		return 0, 0, false
	}
	return bp[0], bp[1], true
}

// MaxBusyFrames 是 Frame 因畫面操作進行中而連續延後的上限。
const MaxBusyFrames = 600

// Frame 是前端每個呈現幀的入口：先 Layer.Frame（定色與指紋偵測），再對本次由 Pending 轉為 Shown 的疊字
// 所屬的每個事件組執行 recolor（001 §8）。Layer.Add、Clear、Frame 以 l.Stamps[:0] 就地過濾，
// 所以先複製 Pending 疊字的指標（001 §7 實作注意）。indexed 是色號畫面，rgb 是同一幀的 RGB（每像素 3 bytes）。
func (o *Overlay) Frame(indexed, rgb []uint8) {
	if o.Busy != nil && o.Busy() {
		if o.busyFrames < MaxBusyFrames {
			o.busyFrames++
			o.C.Inc("frame_deferred")
			return
		}
		o.C.Inc("frame_busy_forced")
	}
	o.busyFrames = 0
	// recolor 把橫跨不同反白狀態的疊字切開時，新疊字是 Pending，同一幀內再跑一輪讓它們定色（最多 3 輪）。
	for round := 0; round < 3; round++ {
		o.resplit = false
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
		if !o.resplit {
			break
		}
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
		// 整組取最多數時底與墨同色：事件內有一部分格被反白（兩種狀態的像素數接近）時會發生。
		// 改取各格自己的 (底, 墨) 配對中最多的一組當組色。
		b, f, ok := majorityPair(len(maskCells(rec)), func(g func(k int, m, c uint8)) {
			o.scanGroup(rec, stamps, indexed, func(k int, m, c uint8, _, _ int) { g(k, m, c) })
		})
		if !ok {
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
		bgIdx, fgIdx = b, f
	}
	cells := o.scanCells(rec, stamps, indexed, bgIdx, fgIdx)
	if gated && o.gateGroup(key, rec, stamps, cells) {
		return
	}
	if o.splitByState(key, rec, stamps, cells) {
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
		if stampSwap(rec, cells, s) {
			s.BG, s.FG = fg, bg
		} else {
			s.BG, s.FG = bg, fg
		}
	}
}

type cellStat struct {
	n, match, ink, inkKeep int
	color                  [256]int
	// 原始計數：add 累計，finish 之後得到 swap、match、inkKeep。
	bg0, bgF, inkFG, inkBG int
	swap                   bool
}

// add 累計一個像素。bgIdx、fgIdx 是事件組的背景與前景色號。
func (cs *cellStat) add(m, c, bgIdx, fgIdx uint8) {
	cs.n++
	cs.color[c]++
	if m == 0 {
		if c == bgIdx {
			cs.bg0++
		} else if c == fgIdx {
			cs.bgF++
		}
		return
	}
	cs.ink++
	if c == fgIdx {
		cs.inkFG++
	} else if c == bgIdx {
		cs.inkBG++
	}
}

// finish 判定這一格是否被反白：遮罩為 0 的像素多數是組前景色，且墨像素多數是組背景色，就是與組色相反的狀態
// （原版對一段文字做 invert，同一事件可以只反白其中幾格）。底與墨都是前景色的格（純色填滿）不算反白。
// 反白格以對調後的底色與墨色計 match 與 inkKeep。
func (cs *cellStat) finish() {
	cs.swap = cs.bgF > cs.bg0 && cs.inkBG >= cs.inkFG
	if cs.swap {
		cs.match, cs.inkKeep = cs.bgF+cs.inkBG, cs.inkBG
	} else {
		cs.match, cs.inkKeep = cs.bg0+cs.inkFG, cs.inkFG
	}
}

// scanCells 以組色統計事件組每個原版格，並判定各格的反白狀態。
func (o *Overlay) scanCells(rec *EventRecord, stamps []*xlate.Stamp, indexed []uint8, bgIdx, fgIdx uint8) []cellStat {
	cells := make([]cellStat, len(maskCells(rec)))
	o.scanGroup(rec, stamps, indexed, func(k int, m, c uint8, _, _ int) { cells[k].add(m, c, bgIdx, fgIdx) })
	for k := range cells {
		cells[k].finish()
	}
	return cells
}

// cellOf 回疊字第 i 格所屬的原版格序號（以該格左緣計，夾在事件範圍內）。
func cellOf(rec *EventRecord, s *xlate.Stamp, i int) int {
	k := (s.X + i*s.CellW - rec.Col*8) / 8
	if s.X+i*s.CellW < rec.Col*8 || k < 0 {
		k = 0
	}
	if k >= len(maskCells(rec)) {
		k = len(maskCells(rec)) - 1
	}
	return k
}

// stampSwap 回疊字是否整片處於反白狀態（非透明格多數）。疊字已依狀態切開時，各格狀態一致。
func stampSwap(rec *EventRecord, cells []cellStat, s *xlate.Stamp) bool {
	sw, n := 0, 0
	for i := 0; i < s.Cells; i++ {
		if i < len(s.Transparent) && s.Transparent[i] {
			continue
		}
		n++
		if cells[cellOf(rec, s, i)].swap {
			sw++
		}
	}
	return 2*sw > n
}

// splitByState 把橫跨不同反白狀態的疊字依狀態切開（001 §8 的逐格狀態）。有切開就以新疊字（Pending）取代原疊字
// 並回 true，Frame 會立刻再跑一輪定色。透明格沿用前一格的狀態，不另外切。
// 只切沒有實體像素路徑、Text 與格一一對應的疊字；其他情形不切（整片以多數狀態定色）。
func (o *Overlay) splitByState(key string, rec *EventRecord, stamps []*xlate.Stamp, cells []cellStat) bool {
	type piece struct{ a, b int }
	split := false
	repl := map[*xlate.Stamp][]*xlate.Stamp{}
	for _, s := range stamps {
		if s.PixelScale != 0 || len(s.PixelGlyphs) > 0 || len(s.Text) != s.Cells || s.Cells < 2 {
			continue
		}
		// 每格狀態：透明格沿用前一個非透明格，前導的透明格沿用第一個非透明格。
		st := make([]bool, s.Cells)
		known := make([]bool, s.Cells)
		for i := range st {
			if !(i < len(s.Transparent) && s.Transparent[i]) {
				st[i], known[i] = cells[cellOf(rec, s, i)].swap, true
			}
		}
		first := -1
		for i := range known {
			if known[i] {
				first = i
				break
			}
		}
		if first < 0 {
			continue
		}
		for i := 0; i < first; i++ {
			st[i] = st[first]
		}
		for i := first + 1; i < s.Cells; i++ {
			if !known[i] {
				st[i] = st[i-1]
			}
		}
		ps := []piece{{0, 1}}
		for i := 1; i < s.Cells; i++ {
			if st[i] != st[i-1] {
				ps = append(ps, piece{i, i + 1})
			} else {
				ps[len(ps)-1].b = i + 1
			}
		}
		if len(ps) < 2 {
			continue
		}
		split = true
		var news []*xlate.Stamp
		for _, p := range ps {
			ns := &xlate.Stamp{
				Key: s.Key, Owner: s.Owner, X: s.X + p.a*s.CellW, Y: s.Y, Cells: p.b - p.a, CellW: s.CellW, CellH: s.CellH,
				Font: s.Font, GlyphX: s.GlyphX, GlyphY: s.GlyphY, GlyphScale: s.GlyphScale,
				Text: append([]rune(nil), s.Text[p.a:p.b]...), SwapColors: s.SwapColors, State: xlate.Pending,
			}
			if len(s.Transparent) > p.a {
				hi := p.b
				if hi > len(s.Transparent) {
					hi = len(s.Transparent)
				}
				ns.Transparent = make([]bool, ns.Cells)
				copy(ns.Transparent, s.Transparent[p.a:hi])
			}
			if _, ok := o.gate[s]; ok {
				o.gate[ns] = struct{}{}
			}
			news = append(news, ns)
		}
		repl[s] = news
	}
	if !split {
		return false
	}
	out := make([]*xlate.Stamp, 0, len(o.Layer.Stamps)+2)
	for _, s := range o.Layer.Stamps {
		if news, ok := repl[s]; ok {
			out = append(out, news...)
			delete(o.gate, s)
			continue
		}
		out = append(out, s)
	}
	o.Layer.Stamps = out
	o.C.Inc("recolor_split")
	o.resplit = true
	return true
}

// gateGroup 是有效性閘門（001 §8 步驟 3）：以原版格為單位檢查畫面是否仍是該原文的字模畫出的結果，
// 不一致的格標 Transparent。cells 是 scanCells 的結果（反白格以對調色計）。回傳 true 表示整組已被移除。
func (o *Overlay) gateGroup(key string, rec *EventRecord, stamps []*xlate.Stamp, cells []cellStat) bool {
	var bad []int
	for k := range cells {
		cs := &cells[k]
		if cs.n < gateMinPixels {
			continue
		}
		if maskCells(rec)[k] == ' ' {
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
