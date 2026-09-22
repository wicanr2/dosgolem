package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestStoryPage4SixLineAtomicAndSixRowClear(t *testing.T) {
	c := &StoryPage4Catalog{}
	lines := make([][]byte, 6)
	for i := range lines {
		lines[i] = []byte{byte(i + 1)}
		c.entries[i] = StoryPage4Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page4.line.%03d", i+1), OriginalLength: 1, OriginalSHA256: sha256.Sum256(lines[i]), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	w, e := NewStoryPage4Watcher(c)
	if e != nil {
		t.Fatal(e)
	}
	for i := range lines {
		a := [7]uint16{1, uint16(lines[i][0]), 1, 0, 10, uint16(17 + i), 1}
		w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, uint64(i+1))
		w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, uint64(i+2))
	}
	if !w.Active() || len(w.Events()) != 6 {
		t.Fatal("six rows not atomic")
	}
	if !StoryPage4VideoSpanIntersects(uint16(176*320+8), 1) || StoryPage4VideoSpanIntersects(uint16(184*320+8), 1) {
		t.Fatal("six-row boundary")
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() {
		t.Fatal("measured clear")
	}
}

func TestStoryPage4RejectsHighWordAndPartial(t *testing.T) {
	c := &StoryPage4Catalog{}
	h := sha256.Sum256([]byte{1})
	c.entries[0] = StoryPage4Identity{Sequence: 1, EventKey: "story.page4.line.001", OriginalLength: 1, OriginalSHA256: h, Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: 17, Column: 1}
	w, _ := NewStoryPage4Watcher(c)
	a := [7]uint16{0x101, 1, 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 1)
	if w.Active() || len(w.Events()) != 0 {
		t.Fatal("high word accepted")
	}
	w.ObserveExecutionDiscontinuity()
	if w.Active() {
		t.Fatal("restore revived")
	}
}
