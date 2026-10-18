package strikecommander

import "fmt"

// 墨跡框核對（001 §2.8）：每筆熱點標籤在 puts 入口與配對的 blit 入口各讀一次畫外緩衝的
// 列範圍 [y′−2, y′+17)，y′ 是 puts 入口時的 [DS:obj+12h]。第 y 列的位址是
// Seg:[DS:List + 2y] + x（blit 本身就這樣定址，3C3C:0061 的 mov si, ss:[bx]）。

type inkCapture struct {
	yPrime int
	before []byte // 列 yPrime-2 起 19 列，每列 320 bytes；超出 [0,200) 的列為 0
}

// Point 是一個原版像素座標。
type Point struct{ X, Y int }

// InkResult 是一筆熱點標籤的核對結果。
type InkResult struct {
	Key     string
	Skipped string // 非空表示略過及原因
	Diffs   int    // 差異像素數
	Outside int    // 落在墨跡框外的差異像素數
	// OutsideAt 列出框外差異像素（最多 64 個），供人判讀（例如滑鼠游標的十字）。
	OutsideAt []Point
	Top       bool // 上緣列 Y+1 有差異
	Left      bool // 左緣行 X0 有差異
	Right     bool // 右緣行 X1−1 有差異
}

// OK 回核對通過：沒有略過、差異全在框內、上左右三緣各有差異。
func (r InkResult) OK() bool {
	return r.Skipped == "" && r.Diffs > 0 && r.Outside == 0 && r.Top && r.Left && r.Right
}

func (r InkResult) String() string {
	if r.Skipped != "" {
		return fmt.Sprintf("%q 略過：%s", r.Key, r.Skipped)
	}
	return fmt.Sprintf("%q 差異 %d、框外 %d %v、上緣 %t、左緣 %t、右緣 %t、通過 %t",
		r.Key, r.Diffs, r.Outside, r.OutsideAt, r.Top, r.Left, r.Right, r.OK())
}

const inkRows = 19 // [y′−2, y′+17)

func readRows(v View, ds, seg, list uint16, y0, y1 int) []byte {
	out := make([]byte, (y1-y0)*320)
	for y := y0; y < y1; y++ {
		if y < 0 || y >= 200 {
			continue
		}
		off := rd16(v, ds, list+uint16(2*y))
		row := out[(y-y0)*320:]
		for x := 0; x < 320; x++ {
			row[x] = v.Read8(lin(seg, off+uint16(x)))
		}
	}
	return out
}

func (o *LabelObserver) inkCheck(v View, ds uint16, l Label) InkResult {
	r := InkResult{Key: l.Key}
	yp := l.ink.yPrime
	if l.ink.before == nil {
		r.Skipped = "puts 入口時沒有讀緩衝"
		return r
	}
	if l.Y != yp {
		r.Skipped = fmt.Sprintf("第一次 putc 的 Y %d 不等於 puts 入口的 y′ %d", l.Y, yp)
		return r
	}
	if o.CursorY >= 0 && o.CursorY-5 < yp+17 && yp-2 < o.CursorY+6 {
		r.Skipped = fmt.Sprintf("滑鼠游標列 %d 落在核對列 [%d, %d)", o.CursorY, yp-2, yp+17)
		return r
	}
	after := readRows(v, ds, l.Seg, l.List, yp-2, yp+17)
	bx0, by0, bx1, by1 := l.Box()
	for i := range after {
		if after[i] == l.ink.before[i] {
			continue
		}
		x, y := i%320, yp-2+i/320
		r.Diffs++
		if x < bx0 || x >= bx1 || y < by0 || y >= by1 {
			r.Outside++
			if len(r.OutsideAt) < 64 {
				r.OutsideAt = append(r.OutsideAt, Point{x, y})
			}
			continue
		}
		if y == by0 {
			r.Top = true
		}
		if x == bx0 {
			r.Left = true
		}
		if x == bx1-1 {
			r.Right = true
		}
	}
	return r
}
