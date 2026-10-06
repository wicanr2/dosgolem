package xlate

import (
	"bytes"
	"testing"
)

func TestDrawWithBackgroundFallbackAndGlyph(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alpha byte
		size  int
		allow bool
		want  [3]byte
	}{
		{"opaque", 255, 36, true, [3]byte{9, 8, 7}},
		{"transparent", 0, 36, true, [3]byte{1, 2, 3}},
		{"translucent", 254, 36, true, [3]byte{1, 2, 3}},
		{"short", 255, 35, true, [3]byte{1, 2, 3}},
		{"denied", 255, 36, false, [3]byte{1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Stamp{Key: "eligible", Cells: 1, CellW: 1, CellH: 1, State: Shown, Text: []rune{'X'}, FG: [3]byte{1, 2, 3}, BG: [3]byte{1, 2, 3}, Font: &Font{W: 1, H: 1, Name: "test", Glyphs: map[rune][]byte{'X': {128}}}, GlyphScale: 1}
			l := Layer{W: 1, H: 1, Stamps: []*Stamp{s}}
			a := make([]byte, tc.size)
			for i := 0; i+3 < len(a); i += 4 {
				copy(a[i:i+4], []byte{9, 8, 7, tc.alpha})
			}
			before := bytes.Clone(a)
			snap, _ := l.Snapshot()
			dst := make([]byte, 36)
			if !l.DrawWithBackground(dst, 3, nil, a, func(q *Stamp) bool { return tc.allow && q == s }) {
				t.Fatal("no text")
			}
			if !bytes.Equal(dst[:4], []byte{1, 2, 3, 255}) {
				t.Fatal("glyph lost when FG equals BG", dst[:4])
			}
			for i := 4; i < len(dst); i += 4 {
				want := append(tc.want[:], 255)
				if !bytes.Equal(dst[i:i+4], want) {
					t.Fatalf("background[%d]=%v want%v", i, dst[i:i+4], want)
				}
			}
			after, _ := l.Snapshot()
			if !bytes.Equal(snap, after) || !bytes.Equal(a, before) {
				t.Fatal("input or snapshot changed")
			}
			legacy := make([]byte, 36)
			fallback := make([]byte, 36)
			l.Draw(legacy, 3, nil)
			l.DrawWithBackground(fallback, 3, nil, a, nil)
			if !bytes.Equal(legacy, fallback) {
				t.Fatal("legacy mismatch")
			}
		})
	}
}

func TestDrawWithBackgroundPreservesHiddenCells(t *testing.T) {
	s := &Stamp{Cells: 2, CellW: 1, CellH: 1, State: Shown, Transparent: []bool{true, false}, BG: [3]byte{1, 2, 3}}
	l := Layer{W: 2, H: 1, Stamps: []*Stamp{s}}
	bg := make([]byte, 72)
	for i := 0; i < len(bg); i += 4 {
		copy(bg[i:i+4], []byte{9, 8, 7, 255})
	}
	dst := make([]byte, 72)
	calls := 0
	l.DrawWithBackground(dst, 3, nil, bg, func(*Stamp) bool { calls++; return true })
	for y := 0; y < 3; y++ {
		for x := 0; x < 6; x++ {
			i := 4 * (y*6 + x)
			want := []byte{0, 0, 0, 0}
			if x >= 3 {
				want = []byte{9, 8, 7, 255}
			}
			if !bytes.Equal(dst[i:i+4], want) {
				t.Fatal("hidden cell changed")
			}
		}
	}
	if calls != 1 {
		t.Fatal("callback count", calls)
	}
	s.State = Pending
	clear(dst)
	l.DrawWithBackground(dst, 3, nil, bg, func(*Stamp) bool { t.Fatal("pending callback"); return true })
	if !bytes.Equal(dst, make([]byte, 72)) {
		t.Fatal("pending drawn")
	}
	if l.DrawWithBackground(dst[:71], 3, nil, bg, nil) || l.DrawWithBackground(dst, 0, nil, bg, nil) {
		t.Fatal("bad output accepted")
	}
}
