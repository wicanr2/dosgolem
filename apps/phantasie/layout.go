package phantasie

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 版面與疊字（docs/spec/001 §6、§7）。寬度單位是半格 h（4 個原版像素）。

// runSeg 是排好的字串中連續同寬度的一段。
type runSeg struct {
	Runes []rune
	Wide  bool
}

// layoutLine 把譯文排成長度剛好 avail h 的字串（001 §6）：
// 置中時去掉頭尾空白後置中；w ≤ avail 時尾端補 ASCII 空白；w > avail 時由左取不超過 avail h 的最長整字前綴（truncated=true）。
// 譯文含換行或控制字元回錯誤（視為格式錯誤）。
func layoutLine(zh []rune, center bool, avail int, wide WideFunc) (out []rune, truncated bool, err error) {
	for _, r := range zh {
		if r < 0x20 || r == 0x7F {
			return nil, false, fmt.Errorf("譯文含控制字元 U+%04X", r)
		}
	}
	if center {
		i, j := 0, len(zh)
		for i < j && zh[i] == ' ' {
			i++
		}
		for j > i && zh[j-1] == ' ' {
			j--
		}
		zh = zh[i:j]
	}
	w := 0
	for _, r := range zh {
		w += hw(r, wide)
	}
	if w > avail {
		truncated = true
		keep, acc := 0, 0
		for _, r := range zh {
			c := hw(r, wide)
			if acc+c > avail {
				break
			}
			acc += c
			keep++
		}
		zh = zh[:keep]
		w = acc
	}
	left := 0
	if center && w < avail {
		left = (avail - w) / 2
	}
	out = make([]rune, 0, avail)
	for i := 0; i < left; i++ {
		out = append(out, ' ')
	}
	out = append(out, zh...)
	for i := left + w; i < avail; i++ {
		out = append(out, ' ')
	}
	return out, truncated, nil
}

func hw(r rune, wide WideFunc) int {
	if wide != nil && wide(r) {
		return 2
	}
	return 1
}

// splitRuns 把排好的字串切成連續同寬度的段。
func splitRuns(line []rune, wide WideFunc) []runSeg {
	var segs []runSeg
	for _, r := range line {
		w := hw(r, wide) == 2
		if n := len(segs); n > 0 && segs[n-1].Wide == w {
			segs[n-1].Runes = append(segs[n-1].Runes, r)
			continue
		}
		segs = append(segs, runSeg{Runes: []rune{r}, Wide: w})
	}
	return segs
}

// xrange 是半開的像素 x 範圍 [X0, X1)。
type xrange struct{ X0, X1 int }

// mergeRanges 排序並合併相鄰或重疊的範圍。
func mergeRanges(in []xrange) []xrange {
	if len(in) < 2 {
		return in
	}
	s := append([]xrange(nil), in...)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].X0 < s[j-1].X0; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	out := s[:1]
	for _, r := range s[1:] {
		last := &out[len(out)-1]
		if r.X0 <= last.X1 {
			if r.X1 > last.X1 {
				last.X1 = r.X1
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

func intersects(a xrange, rs []xrange) bool {
	for _, r := range rs {
		if a.X0 < r.X1 && r.X0 < a.X1 {
			return true
		}
	}
	return false
}

// buildStamps 由排好的字串產生疊字（001 §7）：每個同寬度段一筆 xlate.Stamp。
// y 是疊字的 Y（Row×8 + DY）；hidden 是已不可見的像素 x 範圍，與之相交的疊字格設 Transparent，全部格都透明的疊字不加入。
func buildStamps(key string, x0, y int, line []rune, wide WideFunc, font *xlate.Font, hidden []xrange) []*xlate.Stamp {
	var out []*xlate.Stamp
	x := x0
	for _, seg := range splitRuns(line, wide) {
		cw := 4
		if seg.Wide {
			cw = 8
		}
		s := &xlate.Stamp{
			Key: key, X: x, Y: y, Cells: len(seg.Runes), CellW: cw, CellH: 8,
			Font: font, GlyphScale: 1, Text: seg.Runes, State: xlate.Pending,
		}
		if len(hidden) > 0 {
			tr := make([]bool, s.Cells)
			all := true
			for i := range tr {
				tr[i] = intersects(xrange{x + i*cw, x + (i+1)*cw}, hidden)
				if !tr[i] {
					all = false
				}
			}
			if all {
				x += s.Cells * cw
				continue
			}
			for _, t := range tr {
				if t {
					s.Transparent = tr
					break
				}
			}
		}
		out = append(out, s)
		x += s.Cells * cw
	}
	return out
}

// availH 回事件的可用寬度（h）：2 × 可見的原版格數（Col + len > 40 時裁切到畫布）。
func availH(col, textLen int) int {
	n := textLen
	if col+n > 40 {
		n = 40 - col
	}
	if n < 0 {
		n = 0
	}
	return 2 * n
}
