package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
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

func TestStoryPage4PresenterAtomicFailuresDrawNothing(t *testing.T) {
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		text := map[string]string{}
		good := make([]StoryPage4Event, 6)
		for i := range good {
			r := rune('甲' + i)
			font.Glyphs[r] = make([]byte, 32)
			key := fmt.Sprintf("story.page4.line.%03d", i+1)
			text[key] = string(r)
			good[i] = StoryPage4Event{Generation: 1, EventKey: key, Row: uint8(17 + i), Column: 1}
		}
		for _, bad := range [][]StoryPage4Event{func() []StoryPage4Event { x := append([]StoryPage4Event(nil), good...); x[5].Row = 99; return x }(), func() []StoryPage4Event { x := append([]StoryPage4Event(nil), good...); x[4].Generation = 2; return x }(), func() []StoryPage4Event {
			x := append([]StoryPage4Event(nil), good...)
			x[5].EventKey = x[4].EventKey
			return x
		}()} {
			o, e := NewRuntimeStoryPage4Overlay(text, font, scale)
			if e != nil {
				t.Fatal(e)
			}
			if o.Apply(bad, [256][3]uint8{}) == nil {
				t.Fatal("invalid presenter input accepted")
			}
			base := ScaleIndexedRGBA(make([]byte, 320*200), [256][3]uint8{}, scale)
			got, _, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
			if len(o.ActiveKeys()) != 0 || d || string(got) != string(base) {
				t.Fatal("failed apply retained pixels")
			}
		}
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

func TestStoryPage4ReturnMismatchNeverActivates(t *testing.T) {
	c := &StoryPage4Catalog{}
	h := sha256.Sum256([]byte{1})
	c.entries[0] = StoryPage4Identity{Sequence: 1, EventKey: "story.page4.line.001", OriginalLength: 1, OriginalSHA256: h, Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: 17, Column: 1}
	for _, bad := range []struct {
		op     byte
		ss, sp uint16
	}{{0, 7, 27}, {0xca, 8, 27}, {0xca, 7, 28}} {
		w, _ := NewStoryPage4Watcher(c)
		a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
		w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 1)
		w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, bad.op, Address{0x0763, 0x04ff}, bad.ss, bad.sp, 2)
		if w.Active() || len(w.Events()) != 0 {
			t.Fatal("bad return activated")
		}
	}
}

func page4SixLineFixture() (*StoryPage4Catalog, [][]byte) {
	c := &StoryPage4Catalog{}
	lines := make([][]byte, 6)
	for i := range lines {
		lines[i] = []byte{byte(i + 1), byte(i + 17)}
		c.entries[i] = StoryPage4Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page4.line.%03d", i+1), OriginalLength: uint8(len(lines[i])), OriginalSHA256: sha256.Sum256(lines[i]), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	return c, lines
}

func emitPage4(w *StoryPage4Watcher, line StoryPage4Identity, b byte, col uint8, step uint64) {
	a := [7]uint16{1, uint16(b), 1, 0, 10, uint16(line.Row), uint16(col)}
	w.ObserveGlyphEntry(line.Guard, line.Caller, 7, 9, a, step)
	w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, line.Caller, 7, 9+storyGlyphStackDelta, step+1)
}

func TestStoryPage4CompleteLineSHAMismatchNeverLeavesEvents(t *testing.T) {
	c, lines := page4SixLineFixture()
	w, _ := NewStoryPage4Watcher(c)
	// Reach the line hash gate: only its final byte is altered.
	for j, b := range lines[0] {
		if j == len(lines[0])-1 {
			b ^= 0xff
		}
		emitPage4(w, c.entries[0], b, uint8(j+1), uint64(j*2+1))
	}
	if w.Active() || len(w.Events()) != 0 || w.p != nil || len(w.done) != 0 {
		t.Fatal("SHA mismatch retained partial page4 output")
	}
}

func TestStoryPage4ReturnFailureCannotReviveOldEvents(t *testing.T) {
	c, lines := page4SixLineFixture()
	w, _ := NewStoryPage4Watcher(c)
	// A failed return clears the in-flight first glyph.  A new complete group
	// must be its own six lines, not a resurrection of that old candidate.
	a := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(c.entries[0].Guard, c.entries[0].Caller, 7, 9, a, 1)
	w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0, c.entries[0].Caller, 7, 27, 2)
	if w.Active() || len(w.Events()) != 0 {
		t.Fatal("failed return left events")
	}
	for i, line := range lines {
		for j, b := range line {
			emitPage4(w, c.entries[i], b, uint8(j+1), uint64(10+i*10+j*2))
		}
	}
	if !w.Active() || len(w.Events()) != 6 {
		t.Fatal("fresh legal group did not replace failed candidate")
	}
}
