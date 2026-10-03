package phantasie

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 位置 oracle（docs/spec/001 §10 第 3 項）：以 25A5 呼叫實際寫進視訊記憶體的足跡判定疊字位置。
// 足跡來自 CaptureVideoWrites 掛的 sub_5E65（rep movsw）：B800 段是偶數掃描線、BA00 段是奇數掃描線，
// 每條掃描線一次複製 Count bytes（每 byte 4 個像素）。

// Footprint 由一次 25A5 呼叫的視訊寫入換算實際寫入的像素矩形 [x0,x1)×[y0,y1)。
// 每條被寫的掃描線的 x 範圍必須相同，掃描線必須連續且剛好 8 條（25A5 一次寫 8 條）；否則 ok=false。
func Footprint(ws []VideoWrite) (x0, y0, x1, y1 int, ok bool) {
	type span struct{ x0, x1 int }
	lines := map[int]span{}
	for _, w := range ws {
		if w.Seg != 0xB800 && w.Seg != 0xBA00 {
			continue
		}
		y := 2 * (int(w.Off) / 80)
		if w.Seg == 0xBA00 {
			y++
		}
		sx0 := (int(w.Off) % 80) * 4
		sx1 := sx0 + int(w.Count)*4
		if cur, has := lines[y]; has {
			if sx0 < cur.x0 {
				cur.x0 = sx0
			}
			if sx1 > cur.x1 {
				cur.x1 = sx1
			}
			lines[y] = cur
			continue
		}
		lines[y] = span{sx0, sx1}
	}
	if len(lines) == 0 {
		return 0, 0, 0, 0, false
	}
	first := true
	var ref span
	for y := range lines {
		if first || y < y0 {
			y0 = y
		}
		if first || y+1 > y1 {
			y1 = y + 1
		}
		first = false
	}
	if y1-y0 != 8 {
		return 0, 0, 0, 0, false
	}
	ref = lines[y0]
	for y := y0; y < y1; y++ {
		s, has := lines[y]
		if !has || s != ref {
			return 0, 0, 0, 0, false
		}
	}
	return ref.x0, y0, ref.x1, y1, true
}

// CheckPosition 檢查一個已提交事件的疊字位置（001 §10 第 3 項）：
// 事件矩形必須等於實際寫入足跡；疊字聯集必須等於事件矩形（不多不少）。Col+len > 40 的裁切事件略過。
// 回傳空字串表示通過，否則是原因。
func CheckPosition(rec *EventRecord, stamps []*xlate.Stamp, ws []VideoWrite) string {
	if rec.Col+len(rec.Text) > 40 || rec.Row >= 25 {
		return ""
	}
	fx0, fy0, fx1, fy1, ok := Footprint(ws)
	if !ok {
		return "寫入足跡不連續或各掃描線的範圍不同"
	}
	ex0, ey0 := rec.Col*8, rec.Row*8
	ex1, ey1 := ex0+len(rec.Text)*8, ey0+8
	if fx0 != ex0 || fy0 != ey0 || fx1 != ex1 || fy1 != ey1 {
		return fmt.Sprintf("事件矩形 [%d,%d)x[%d,%d) 與寫入足跡 [%d,%d)x[%d,%d) 不同", ex0, ex1, ey0, ey1, fx0, fx1, fy0, fy1)
	}
	var cover []xrange
	for _, s := range stamps {
		sx0, sy0, sx1, sy1 := s.Rect()
		if sy0 != fy0 || sy1 != fy1 || sx0 < fx0 || sx1 > fx1 {
			return fmt.Sprintf("疊字矩形 [%d,%d)x[%d,%d) 超出寫入足跡", sx0, sx1, sy0, sy1)
		}
		cover = append(cover, xrange{sx0, sx1})
	}
	cur := fx0
	for _, r := range mergeRanges(cover) {
		if r.X0 > cur {
			break
		}
		if r.X1 > cur {
			cur = r.X1
		}
	}
	if cur != fx1 {
		return fmt.Sprintf("疊字聯集沒有蓋滿寫入足跡（蓋到 x=%d，足跡到 %d）", cur, fx1)
	}
	return ""
}
