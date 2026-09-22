package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func p7(t *testing.T) (*StoryPage7Catalog, [][]byte) {
	b := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}, {11, 12}}
	e := make([]StoryPage7Identity, 6)
	for i, x := range b {
		e[i] = StoryPage7Identity{uint8(i + 1), uint8(len(x)), fmt.Sprintf("story.page7.line.%03d", i+1), sha256.Sum256(x), Address{0x0763, 0x04ff}, storyGlyphPrimitive}
	}
	c, err := NewStoryPage7Catalog(e)
	if err != nil {
		t.Fatal(err)
	}
	return c, b
}
func emit7(w *StoryPage7Watcher, e StoryPage7Identity, b byte, col uint8, s uint64) {
	a := [7]uint16{1, uint16(b), 1, 0, 10, uint16(17 + int(e.Sequence) - 1), uint16(col)}
	w.ObserveGlyphEntry(e.Guard, e.Caller, 7, 9, a, s)
	w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, e.Caller, 7, 9+storyGlyphStackDelta, s+1)
}
func TestStoryPage7AtomicABIAndRowAwareWrite(t *testing.T) {
	c, b := p7(t)
	for i := 0; i < 7; i++ {
		w, _ := NewStoryPage7Watcher(c)
		a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
		a[i] |= 0x100
		w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 1)
		if w.f != nil {
			t.Fatal("ABI high")
		}
	}
	w, _ := NewStoryPage7Watcher(c)
	s := uint64(1)
	for i, x := range b {
		for j, v := range x {
			emit7(w, c.entries[i], v, uint8(j+1), s)
			s += 2
		}
	}
	if !w.Active() || len(w.Events()) != 6 {
		t.Fatal("atomic")
	}
	for _, q := range []struct{ d, n uint16 }{{137 * 320, 8}, {136 * 320, 8}, {184*320 + 8, 1}} {
		if w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, q.d, q.n) {
			t.Fatal("margin")
		}
	}
	if !w.Active() {
		t.Fatal("unexpected clear")
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 136*320+319, 2) || w.Active() {
		t.Fatal("cross row")
	}
}
