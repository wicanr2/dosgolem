package dkk

import "github.com/wicanr2/dosgolem/xlate"

// 規格 005：中文句中的可列印 ASCII 兩兩合成一個組合字，佔一個 8×8 字格。
const pairBase = 0xF0000

func isHalf(r rune) bool { return r >= 0x20 && r <= 0x7E }

func pairRune(a, b rune) rune { return pairBase + a<<7 + b }

func isPair(r rune) bool { return r >= pairBase+0x20<<7 && r < pairBase+0x80<<7 }

// PackHalfWidth 依規格 005 §2 合併連續 ASCII；譯文全是 ASCII 時原樣回傳。
func PackHalfWidth(rs []rune) []rune {
	wide := false
	for _, r := range rs {
		if r >= 0x80 {
			wide = true
			break
		}
	}
	if !wide {
		return rs
	}
	out := make([]rune, 0, len(rs))
	for i := 0; i < len(rs); {
		if !isHalf(rs[i]) {
			out = append(out, rs[i])
			i++
			continue
		}
		j := i
		for j < len(rs) && isHalf(rs[j]) {
			j++
		}
		if j-i == 1 && rs[i] == ' ' {
			out = append(out, ' ')
			i = j
			continue
		}
		for k := i; k < j; k += 2 {
			b := rune(' ')
			if k+1 < j {
				b = rs[k+1]
			}
			out = append(out, pairRune(rs[k], b))
		}
		i = j
	}
	return out
}

// AddHalfWidthPairs 在 16×16 字型補上全部組合字（規格 005 §2）。倚天 ASCII 墨跡在 x=4..11。
func AddHalfWidthPairs(f *xlate.Font) {
	if f == nil || f.W != 16 {
		return
	}
	for a := rune(0x20); a <= 0x7E; a++ {
		ga, okA := f.Glyphs[a]
		for b := rune(0x20); b <= 0x7E; b++ {
			gb, okB := f.Glyphs[b]
			if !okA || !okB {
				continue
			}
			g := make([]byte, len(ga))
			for y := 0; y+1 < len(g); y += 2 {
				l := uint16(ga[y])<<8 | uint16(ga[y+1])
				r := uint16(gb[y])<<8 | uint16(gb[y+1])
				v := (l<<4)&0xFF00 | (r>>4)&0x00FF
				g[y], g[y+1] = byte(v>>8), byte(v)
			}
			f.Glyphs[pairRune(a, b)] = g
		}
	}
}
