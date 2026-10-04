package phantasie

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 獨立稽核（docs/spec/005 §5.1）：以原版 FONT 位元圖與目前色號畫面為基準，不使用疊字層自己的顏色狀態當期望值。
// 與 recolor 共用像素粒度的遮罩掃描（scanGroup）與格一致規則，所以兩邊的像素定義相同。

const (
	auditInkPct = 5 // 稽核的墨像素保留率下限：10 × 保留 ≥ 5 × 墨像素（50%）
)

// auditCell 判斷一個原版格是否仍是該原文字模畫出的結果（005 §5.1 的 cellConsistent）。
func auditCell(cs *cellStat, ch byte, bgIdx, fgIdx uint8) bool {
	if ch == ' ' {
		mx := 0
		for _, v := range cs.color {
			if v > mx {
				mx = v
			}
		}
		return 100*mx >= gateSpaceMinPct*cs.n
	}
	if 10*cs.match < 7*cs.n {
		return false
	}
	if cs.ink >= gateMinInk && 10*cs.inkKeep < auditInkPct*cs.ink {
		return false
	}
	return true
}

// modes 由 scan 回呼累計的直方圖求 bgIdx、fgIdx；nfg 為 0 時 ok=false。
type modeAcc struct {
	bg, fg   [256]int
	nbg, nfg int
}

func (m *modeAcc) add(mask, c uint8) {
	if mask == 0 {
		m.bg[c]++
		m.nbg++
	} else {
		m.fg[c]++
		m.nfg++
	}
}

func (m *modeAcc) modes() (bgIdx, fgIdx uint8, ok bool) {
	if m.nfg == 0 {
		return 0, 0, false
	}
	return uint8(argmax(&m.bg)), uint8(argmax(&m.fg)), true
}

// scanRect 對事件矩形的每個像素呼叫 f（含不屬於任何疊字的像素）；y0 是矩形的實際列。
func (o *Overlay) scanRect(col, y0 int, cells []byte, indexed []uint8, f func(k int, m, color uint8)) {
	if o.font == nil || y0 < 0 || y0 >= screenH {
		return
	}
	ex0 := col * 8
	ex1 := ex0 + len(cells)*8
	if ex1 > screenW {
		ex1 = screenW
	}
	for y := y0; y < y0+8 && y < screenH; y++ {
		for x := ex0; x < ex1; x++ {
			k := (x - ex0) / 8
			f(k, o.glyphPixel(cells[k], y-y0, (x-ex0)%8), indexed[y*screenW+x])
		}
	}
}

// AuditStale 回殘字格數（001 §8、005 §5.1 第 1 項）：每個 Shown 疊字的非透明格所對應的原版格，
// 畫面已不是該原文的字模畫出的結果（疊字蓋在已不是原文的畫面上）。與開啟中事件矩形相交的事件組略過
// （A 與 B 之間畫面被半寫，舊疊字的字模檢查必然不成立，不是缺陷）。
func (o *Overlay) AuditStale(indexed []uint8) int {
	stale := 0
	openRec, hasOpen := o.Open()
	for _, key := range o.groupKeys() {
		rec := o.records[key]
		stamps := o.groupStamps(key)
		if rec == nil || (hasOpen && key == openRec.ID) {
			continue
		}
		if hasOpen && groupOverlapsEvent(stamps, openRec) {
			continue
		}
		shown := true
		for _, s := range stamps {
			if s.State != xlate.Shown {
				shown = false
			}
		}
		if !shown {
			continue
		}
		var m modeAcc
		if !o.scanGroup(rec, stamps, indexed, func(_ int, mk, c uint8, _, _ int) { m.add(mk, c) }) {
			continue
		}
		bgIdx, fgIdx, ok := m.modes()
		if !ok {
			continue
		}
		if bgIdx == fgIdx {
			// 事件內一部分格被反白時整組取最多數會底墨同色：改取各格 (底, 墨) 配對的多數（與 recolor 同一規則）。
			if b, f, ok := majorityPair(len(maskCells(rec)), func(g func(k int, m, c uint8)) {
				o.scanGroup(rec, stamps, indexed, func(k int, mk, c uint8, _, _ int) { g(k, mk, c) })
			}); ok {
				bgIdx, fgIdx = b, f
			} else {
				total, same := m.nbg+m.nfg, m.bg[bgIdx]+m.fg[fgIdx]
				if 10*same >= 9*total {
					for _, s := range stamps {
						for i := 0; i < s.Cells; i++ {
							if i >= len(s.Transparent) || !s.Transparent[i] {
								stale++
							}
						}
					}
				}
				continue
			}
		}
		cells := o.scanCells(rec, stamps, indexed, bgIdx, fgIdx)
		for k := range cells {
			if cells[k].n < gateMinPixels {
				continue
			}
			if !auditCell(&cells[k], maskCells(rec)[k], bgIdx, fgIdx) {
				stale++
				if o.AuditDebug != nil {
					o.AuditDebug(fmt.Sprintf("殘字格 key=%s k=%d ch=%q n=%d match=%d ink=%d keep=%d bg=%d fg=%d text=%q", key, k, maskCells(rec)[k], cells[k].n, cells[k].match, cells[k].ink, cells[k].inkKeep, bgIdx, fgIdx, rec.Text))
				}
			}
		}
	}
	return stale
}

// groupOverlapsEvent 回疊字組的矩形是否與事件 rec 的矩形（原版格 8 像素、同一列）相交。
func groupOverlapsEvent(stamps []*xlate.Stamp, rec *EventRecord) bool {
	ex0, ex1 := rec.Col*8, rec.Col*8+len(rec.Cells)*8
	ey0, ey1 := rec.Row*8, rec.Row*8+8
	for _, s := range stamps {
		sx0, sy0, sx1, sy1 := s.Rect()
		if sx0 < ex1 && ex0 < sx1 && sy0 < ey1 && ey0 < sy1 {
			return true
		}
	}
	return false
}

// AuditExposed 回英文外露事件數（005 §5.1 第 2 項）：事件日誌內的事件，其事件矩形目前仍顯示該原文
// （字模遮罩一致檢查），而 Layer 沒有覆蓋該矩形全部像素的非透明疊字。事件日誌與 records 的清理無關。
func (o *Overlay) AuditExposed(indexed []uint8) int {
	exposed, _ := o.AuditEvents(indexed)
	return exposed
}

// laterRanges 回事件 e 之後提交的事件（任何類別）在同一列的矩形 x 範圍：那些像素是後來的事件畫的，
// 不再屬於事件 e（例如輸入欄位後來被另一個事件寫上數字），稽核對事件 e 不計入。
func (o *Overlay) laterRanges(e LogEvent, y0 int) []xrange {
	var out []xrange
	for _, r := range o.rects {
		if r.n > e.N && r.y == y0 {
			out = append(out, xrange{r.x0, r.x1})
		}
	}
	return out
}

// AuditEvents 同 AuditExposed，另回「遮罩對應率不是 100%」的事件數（001 §10 第 1 項）：仍顯示該原文的事件
// （格一致檢查全通過），遮罩為 0 的像素必須全為同一色號、非 0 的像素全為另一色號。
// 被後來事件覆寫的原版格（laterRanges）不計。
func (o *Overlay) AuditEvents(indexed []uint8) (exposed, strict int) {
	var openRec *EventRecord
	if rec, ok := o.Open(); ok {
		openRec = rec
	}
	for _, e := range o.log {
		cells := []byte(e.Text)
		y0 := e.Row * 8
		if stamps := o.groupStamps(e.ID); len(stamps) > 0 {
			if dy, ok := groupDY(stamps, &EventRecord{Row: e.Row}); ok {
				y0 += dy
			}
		}
		if y0 < 0 || y0 >= screenH || e.Col*8 >= screenW {
			continue
		}
		if openRec != nil && openRec.Row*8 == y0 {
			ox0, ox1 := openRec.Col*8, openRec.Col*8+len(openRec.Cells)*8
			if ox0 < e.Col*8+len(cells)*8 && e.Col*8 < ox1 {
				continue
			}
		}
		later := o.laterRanges(e, e.Row*8)
		skip := make([]bool, len(cells))
		kept := 0
		for k := range cells {
			if !intersects(xrange{e.Col*8 + 8*k, e.Col*8 + 8*k + 8}, later) {
				kept++
			} else {
				skip[k] = true
			}
		}
		if kept == 0 {
			continue
		}
		var m modeAcc
		o.scanRect(e.Col, y0, cells, indexed, func(k int, mk, c uint8) {
			if !skip[k] {
				m.add(mk, c)
			}
		})
		bgIdx, fgIdx, ok := m.modes()
		if ok && bgIdx == fgIdx {
			if b, f, ok2 := majorityPair(len(cells), func(g func(k int, m, c uint8)) {
				o.scanRect(e.Col, y0, cells, indexed, func(k int, mk, c uint8) {
					if !skip[k] {
						g(k, mk, c)
					}
				})
			}); ok2 {
				bgIdx, fgIdx = b, f
			}
		}
		if !ok || bgIdx == fgIdx {
			continue
		}
		stats := make([]cellStat, len(cells))
		o.scanRect(e.Col, y0, cells, indexed, func(k int, mk, c uint8) {
			if !skip[k] {
				stats[k].add(mk, c, bgIdx, fgIdx)
			}
		})
		for k := range stats {
			stats[k].finish()
		}
		showing := true
		for k := range stats {
			if stats[k].n == 0 {
				continue
			}
			if !auditCell(&stats[k], cells[k], bgIdx, fgIdx) {
				showing = false
				break
			}
		}
		if !showing {
			continue
		}
		// 遮罩對應率 100% 只對「疊字還在 Layer 內、畫面在疊字之下仍是該原文」的事件判定：已被後來事件取代的舊事件，
		// 它的矩形現在顯示的是別的文字（字模可能是舊字模的超集或子集，例如 + 與 -），不能拿舊事件的字模去量。
		// 只計疊字非透明格範圍內的像素（透明格是玩家輸入等不屬於疊字的格）。
		if rec, stamps := o.records[e.ID], o.groupStamps(e.ID); rec != nil && len(stamps) > 0 {
			// 同一事件可以只反白其中幾格：底色與墨色依各格的反白狀態分開統計，同一狀態內才要求單一色號。
			visualStats := o.scanCells(rec, stamps, indexed, bgIdx, fgIdx)
			if !o.strictMask(rec, stamps, indexed, visualStats, nil) &&
				!o.strictMask(rec, stamps, indexed, visualStats, e.dimCells) {
				strict++
				if o.AuditDebug != nil {
					o.AuditDebug(fmt.Sprintf("遮罩對應率不是 100%% key=%s text=%q", e.ID, e.Text))
				}
			}
		}
		// 覆蓋檢查：事件矩形（扣掉被後來事件覆寫的格）必須被非透明疊字蓋滿。
		var need []xrange
		for k := range cells {
			if !skip[k] {
				a, b := e.Col*8+8*k, e.Col*8+8*k+8
				if b > screenW {
					b = screenW
				}
				if a < b {
					need = append(need, xrange{a, b})
				}
			}
		}
		var cover []xrange
		for _, s := range o.Layer.Stamps {
			if s.Y != y0 {
				continue
			}
			for i := 0; i < s.Cells; i++ {
				if i < len(s.Transparent) && s.Transparent[i] {
					continue
				}
				cover = append(cover, xrange{s.X + i*s.CellW, s.X + (i+1)*s.CellW})
			}
		}
		cover = mergeRanges(cover)
		covered := true
		for _, n := range need {
			cur := n.X0
			for _, r := range cover {
				if r.X0 <= cur && r.X1 > cur {
					cur = r.X1
				}
			}
			if cur < n.X1 {
				covered = false
				break
			}
		}
		if !covered {
			exposed++
			if o.AuditDebug != nil {
				o.AuditDebug(fmt.Sprintf("英文外露 key=%s text=%q 列=%d 欄=%d", e.ID, e.Text, e.Row, e.Col))
			}
		}
	}
	return exposed, strict
}

// strictMask 核對既有像素集合的單色映射。dims 非空時只讓已知變暗格的
// 掃描線 2、4 使用精確清除模型：同格兩線必須同為 0，或反白後的 3。
// 其他像素仍全部按原字模核對，不改定色、粗略顯示判定或覆蓋檢查。
func (o *Overlay) strictMask(rec *EventRecord, stamps []*xlate.Stamp, indexed []uint8, stats []cellStat, dims []bool) bool {
	var c0, c1 [2][256]int
	colors := make(map[int]uint8)
	valid := true
	if len(stamps) == 0 {
		return true
	}
	y0 := stamps[0].Y
	ok := o.scanGroup(rec, stamps, indexed, func(k int, mk, c uint8, _, y int) {
		if k < len(dims) && dims[k] && (y-y0 == 2 || y-y0 == 4) {
			if c != 0 && c != 3 {
				valid = false
			}
			if prior, seen := colors[k]; seen && prior != c {
				valid = false
			}
			colors[k] = c
			return
		}
		sw := 0
		if k < len(stats) && stats[k].swap {
			sw = 1
		}
		if mk == 0 {
			c0[sw][c]++
		} else {
			c1[sw][c]++
		}
	})
	if !ok {
		return true // 沿用無可掃描字模／不一致列時的原有行為。
	}
	for sw := 0; sw < 2; sw++ {
		if distinct(&c0[sw]) > 1 || distinct(&c1[sw]) > 1 {
			return false
		}
	}
	return valid
}

func distinct(h *[256]int) int {
	n := 0
	for _, v := range h {
		if v > 0 {
			n++
		}
	}
	return n
}
