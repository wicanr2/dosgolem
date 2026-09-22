package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func page2Fixture(t *testing.T) (*StoryPage2Catalog, [][]byte) {
	t.Helper()
	lines := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}
	es := make([]StoryPage2Identity, 4)
	for i, b := range lines {
		es[i] = StoryPage2Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("p2.%d", i+1), OriginalLength: uint8(len(b)), OriginalSHA256: sha256.Sum256(b), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	c, e := NewStoryPage2Catalog(es)
	if e != nil {
		t.Fatal(e)
	}
	return c, lines
}
func emitPage2(w *StoryPage2Watcher, e StoryPage2Identity, b byte, col uint8, step uint64) {
	a := [7]uint16{1, uint16(b), 1, 0, 10, uint16(e.Row), uint16(col)}
	w.ObserveGlyphEntry(e.Guard, e.Caller, 0x2222, 0x3333, a, step)
	w.ObserveVerifiedGlyphReturn(StoryPage2VerifiedReturn{EntryStep: step, PostCallStep: step + 1, Caller: e.Caller, SS: 0x2222, SP: 0x3345})
}
func TestStoryPage2AtomicRelativeReturnAndMeasuredInvalidation(t *testing.T) {
	c, lines := page2Fixture(t)
	w, _ := NewStoryPage2Watcher(c)
	step := uint64(10)
	for i, line := range lines {
		for j, b := range line {
			emitPage2(w, c.entries[i], b, uint8(1+j), step)
			step += 2
		}
	}
	if !w.Active() || len(w.Events()) != 4 {
		t.Fatalf("atomic group not active: %#v", w.Events())
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() {
		t.Fatal("measured intersecting pre-write did not clear")
	}
	if StoryPage2VideoSpanIntersects(0, 8) {
		t.Fatal("unrelated span intersects")
	}
}
func TestStoryPage2FailsClosedOnReturnAndDiscontinuity(t *testing.T) {
	c, _ := page2Fixture(t)
	w, _ := NewStoryPage2Watcher(c)
	a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(storyGlyphPrimitive, c.entries[0].Caller, 7, 9, a, 100)
	w.ObserveVerifiedGlyphReturn(StoryPage2VerifiedReturn{EntryStep: 100, PostCallStep: 101, Caller: c.entries[0].Caller, SS: 7, SP: 9 + storyGlyphStackDelta + 1})
	if w.Pending() || len(w.Events()) != 0 {
		t.Fatal("bad relative stack leaked candidate")
	}
	w.ObserveExecutionDiscontinuity()
	if w.Active() || w.Pending() {
		t.Fatal("discontinuity revived state")
	}
}
