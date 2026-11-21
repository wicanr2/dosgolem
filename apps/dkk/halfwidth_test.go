package dkk

import (
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func TestPackHalfWidth(t *testing.T) {
	for in, want := range map[string][]rune{
		"按 Enter 繼續": {'按', pairRune(' ', 'E'), pairRune('n', 't'), pairRune('e', 'r'), pairRune(' ', ' '), '繼', '續'},
		"造成 10 點":    {'造', '成', pairRune(' ', '1'), pairRune('0', ' '), '點'},
		"長劍 +4":      {'長', '劍', pairRune(' ', '+'), pairRune('4', ' ')},
		"開始 示範":      {'開', '始', ' ', '示', '範'},
		"AC":         {'A', 'C'},
	} {
		got := PackHalfWidth([]rune(in))
		if string(got) != string(want) {
			t.Errorf("%q → %U，要 %U", in, got, want)
		}
	}
}

func TestHalfWidthPairGlyph(t *testing.T) {
	g := func(bits uint16) []byte { // 每列同一個 16 位元圖樣
		b := make([]byte, 32)
		for y := 0; y < 16; y++ {
			b[2*y], b[2*y+1] = byte(bits>>8), byte(bits)
		}
		return b
	}
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for r := rune(0x20); r <= 0x7E; r++ {
		f.Glyphs[r] = g(0)
	}
	f.Glyphs['A'] = g(0x0FF0) // 墨跡 x=4..11
	f.Glyphs['B'] = g(0x0810) // x=4 與 x=11
	AddHalfWidthPairs(f)
	p := f.Glyphs[pairRune('A', 'B')]
	if p[0] != 0xFF || p[1] != 0x81 {
		t.Fatalf("組合字模：%02X %02X，要 FF 81", p[0], p[1])
	}
	if !isPair(pairRune('~', '~')) || isPair('中') {
		t.Fatal("isPair 範圍")
	}
}
