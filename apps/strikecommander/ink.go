package strikecommander

import "fmt"

// 墨跡框核對（001 §2.8，label）：每筆熱點標籤在 puts 入口與配對的 blit 入口各讀一次畫外緩衝的
// 列範圍 [y′−2, y′+17)，y′ 是 puts 入口時的 [DS:obj+12h]。第 y 列的位址是
// Seg:[DS:List + 2y] + x（blit 本身就這樣定址，3C3C:0061 的 mov si, ss:[bx]）。

type inkCapture struct {
	yPrime int    // puts 入口時的 [DS:obj+12h]（label、board 都記）
	before []byte // label 且 InkCheck 時：列 yPrime-2 起 19 列，每列 320 bytes；超出 [0,200) 的列為 0
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
	return diffRows(l, l.ink.before, after, yp-2)
}

// diffRows 比較同一段列的前後兩份（列 rowY0 起，每列 320 bytes），以 l.Box() 判斷框內外與上、左、右三緣。
// label 與 board 共用。
func diffRows(l Label, before, after []byte, rowY0 int) InkResult {
	r := InkResult{Key: l.Key}
	bx0, by0, bx1, by1 := l.Box()
	for i := range after {
		if after[i] == before[i] {
			continue
		}
		x, y := i%320, rowY0+i/320
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

// board 的前列、後列與核對（003 §3.2）：
//   - 前列在 puts 入口取，後列在同一筆 puts 返回時（IP 03D3h、SP 等於入口 SP）立刻取，範圍都是
//     [Y+1, Y+9) × [0, 320)。puts 在 03CE 結束字元迴圈，返回時所有 putc 都已同步返回。
//   - 不在配對 blit 時取：GROUND 與 AIR 畫在同一帶，blit 時 GROUND 的核對會看到 AIR 的墨跡。
//   - 墨跡框 [X0, X1) × [Y+1, Y+8)，字高 7 列（RE 014 §1）；第 Y+8 列在讀取範圍內、框外。
//   - 核對不看滑鼠游標：游標在標籤畫完之後、blit 之前才畫進緩衝（003 §2.3）。
const (
	boardRows = 8 // 前後列的列數 [Y+1, Y+9)
	boardInkH = 7 // 墨跡框高 [Y+1, Y+8)
)

// rowsReturn 在 label 與 board 標籤的 puts 返回時讀後列（列數依種類 refRows：label 14、board 8；006 §3.1 第 2 條），
// board 另做墨跡核對。第一次 putc 的 y 與 puts 入口的 y 不同時前後列作廢：board 之後不產生 stamp（Overlay 記 skip），
// label 照 001 畫實心底（Overlay 記 BGFallbacks）。BoardInk 只對 board 算，label 維持零值。
func (o *LabelObserver) rowsReturn(v View, l *Label) {
	yp := l.ink.yPrime
	if l.Y != yp {
		l.Before, l.After = nil, nil
		if l.Kind == KindBoard {
			l.BoardInk = InkResult{Key: l.Key, Skipped: fmt.Sprintf("第一次 putc 的 Y %d 不等於 puts 入口的 y′ %d，前後列作廢", l.Y, yp)}
		}
		o.note(v.Steps(), string(l.Kind), "%s 標籤 %q：第一次 putc 的 Y %d 不等於 puts 入口的 y′ %d，前後列作廢", l.Kind, l.Key, l.Y, yp)
		return
	}
	l.After = readRows(v, v.DS(), l.Seg, l.List, l.Y+1, l.Y+1+refRows(l.Kind))
	if l.Kind == KindBoard {
		l.BoardInk = diffRows(*l, l.Before, l.After, l.Y+1)
	}
}
