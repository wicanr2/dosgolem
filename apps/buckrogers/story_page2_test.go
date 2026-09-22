package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
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
func TestStoryPage2PresenterRejectsMixedDuplicateAndMissingGeneration(t *testing.T) {
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	text := map[string]string{}
	for i, r := range []rune("甲乙丙丁") {
		f.Glyphs[r] = make([]byte, 32)
		text[fmt.Sprintf("story.page2.line.00%d", i+1)] = string(r)
	}
	o, e := NewRuntimeStoryPage2Overlay(text, f, 2)
	if e != nil {
		t.Fatal(e)
	}
	good := make([]StoryPage2Event, 4)
	for i := range good {
		good[i] = StoryPage2Event{Generation: 1, EventKey: fmt.Sprintf("story.page2.line.00%d", i+1), Row: uint8(17 + i), Column: 1}
	}
	for _, bad := range [][]StoryPage2Event{append([]StoryPage2Event(nil), good[:3]...), func() []StoryPage2Event { x := append([]StoryPage2Event(nil), good...); x[3].Generation = 2; return x }(), func() []StoryPage2Event {
		x := append([]StoryPage2Event(nil), good...)
		x[3].EventKey = x[2].EventKey
		return x
	}()} {
		if o.Apply(bad, [256][3]uint8{}) == nil {
			t.Fatal("accepted invalid atomic generation")
		}
		if len(o.ActiveKeys()) != 0 {
			t.Fatal("invalid apply drew")
		}
	}
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

func TestStoryPage2WatcherRejectsIdentityDriftWithoutDraw(t *testing.T) {
	c, lines := page2Fixture(t)
	w, _ := NewStoryPage2Watcher(c)
	for _, tc := range []struct {
		name   string
		mutate func(*StoryPage2Identity)
	}{
		{"caller", func(e *StoryPage2Identity) { e.Caller = Address{1, 2} }}, {"guard", func(e *StoryPage2Identity) { e.Guard = Address{3, 4} }}, {"mode", func(e *StoryPage2Identity) { e.Mode = 2 }}, {"repeat", func(e *StoryPage2Identity) { e.Repeat = 2 }}, {"style", func(e *StoryPage2Identity) { e.Foreground = 9 }}, {"order", func(e *StoryPage2Identity) { e.Row = 18 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := c.entries[0]
			tc.mutate(&e)
			emitPage2(w, e, lines[0][0], 1, 10)
			if len(w.Events()) != 0 || w.Active() {
				t.Fatal("drift emitted")
			}
			w.ObserveExecutionDiscontinuity()
		})
	}
	if w.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) {
		t.Fatal("unknown write invalidated")
	}
}
