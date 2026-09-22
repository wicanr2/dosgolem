package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
	"strings"
	"testing"
)

func p7(t *testing.T) (*StoryPage7Catalog, [][]byte) {
	b := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}, {11, 12}}
	e := make([]StoryPage7Identity, 6)
	for i, x := range b {
		e[i] = StoryPage7Identity{Sequence: uint8(i + 1), OriginalLength: uint8(len(x)), Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1, EventKey: fmt.Sprintf("story.page7.line.%03d", i+1), OriginalSHA256: sha256.Sum256(x), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive}
	}
	return &StoryPage7Catalog{entries: [6]StoryPage7Identity{e[0], e[1], e[2], e[3], e[4], e[5]}}, b
}
func TestStoryPage7PresenterFailClosedBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		text := map[string]string{}
		events := make([]StoryPage7Event, 6)
		for i, r := range []rune("甲乙丙丁戊己") {
			font.Glyphs[r] = make([]byte, 32)
			k := fmt.Sprintf("story.page7.line.%03d", i+1)
			text[k] = string(r)
			events[i] = StoryPage7Event{1, 0, 0, k, uint8(17 + i), 1}
		}
		if _, e := NewRuntimeStoryPage7Overlay(text, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, scale); e == nil {
			t.Fatal("missing glyph")
		}
		o, e := NewRuntimeStoryPage7Overlay(text, font, scale)
		if e != nil {
			t.Fatal(e)
		}
		for _, bad := range [][]StoryPage7Event{events[:5], func() []StoryPage7Event {
			x := append([]StoryPage7Event(nil), events...)
			x[5].EventKey = x[4].EventKey
			return x
		}(), func() []StoryPage7Event { x := append([]StoryPage7Event(nil), events...); x[4].Row = 99; return x }()} {
			if o.Apply(bad, [256][3]uint8{}) == nil {
				t.Fatal("bad group")
			}
			if len(o.ActiveKeys()) != 0 {
				t.Fatal("partial stamp")
			}
		}
		if e := o.Apply(events, [256][3]uint8{}); e != nil {
			t.Fatal(e)
		}
		if len(o.ActiveKeys()) != 6 {
			t.Fatal("active")
		}
		o.Clear()
		if len(o.ActiveKeys()) != 0 {
			t.Fatal("clear")
		}
	}
}
func TestStoryPage7WatcherFailureMatrix(t *testing.T) {
	c, b := p7(t)
	for _, high := range []bool{false, true} {
		for i := 0; i < 7; i++ {
			w, _ := NewStoryPage7Watcher(c)
			a := [7]uint16{1, uint16(b[0][0]), 1, 0, 10, 17, 1}
			if high {
				a[i] |= 0x100
			} else {
				a[i] = (a[i] + 1) & 255
			}
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 1)
			if !high && i == 1 {
				w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 2)
				emit7(w, c.entries[0], b[0][1], 2, 3)
				if w.p != nil {
					t.Fatal("glyph low byte hash tail")
				}
				continue
			}
			if w.f != nil {
				t.Fatalf("ABI %v/%d", high, i)
			}
		}
	}
	for _, kind := range []string{"caller", "guard", "style", "row", "col", "previous", "opcode", "returncaller", "ss", "sp", "post"} {
		w, _ := NewStoryPage7Watcher(c)
		a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
		caller, guard := Address{0x0763, 0x04ff}, storyGlyphPrimitive
		if kind == "caller" {
			caller = Address{1, 2}
		}
		if kind == "guard" {
			guard = Address{1, 2}
		}
		if kind == "style" {
			a[4] = 9
		}
		if kind == "row" {
			a[5] = 18
		}
		if kind == "col" {
			a[6] = 2
		}
		w.ObserveGlyphEntry(guard, caller, 7, 9, a, 10)
		prev := Address{0x0763, 0x03d6}
		op := byte(0xca)
		ret := caller
		ss, sp, post := uint16(7), uint16(9+storyGlyphStackDelta), uint64(11)
		if kind == "previous" {
			prev = Address{1, 2}
		}
		if kind == "opcode" {
			op = 0
		}
		if kind == "returncaller" {
			ret = Address{1, 2}
		}
		if kind == "ss" {
			ss++
		}
		if kind == "sp" {
			sp++
		}
		if kind == "post" {
			post = 10
		}
		w.ObserveVerifiedGlyphReturn(prev, op, ret, ss, sp, post)
		if w.Active() || w.p != nil || len(w.Events()) != 0 {
			t.Fatal(kind)
		}
	}
	w, _ := NewStoryPage7Watcher(c)
	emit7(w, c.entries[0], b[0][0], 1, 10)
	emit7(w, c.entries[0], b[0][1]^1, 2, 12)
	if w.p != nil {
		t.Fatal("hash tail")
	}
	w.ObserveExecutionDiscontinuity()
	if w.Active() || w.p != nil {
		t.Fatal("discontinuity")
	}
	for _, q := range []struct {
		at        Address
		es, di, n uint16
	}{{Address{1, 2}, storyVideoSegment, 0xaa08, 304}, {storyFillInstruction, 0xb800, 0xaa08, 304}, {storyFillInstruction, storyVideoSegment, 137 * 320, 8}, {storyFillInstruction, storyVideoSegment, 184*320 + 8, 1}} {
		if w.ObserveVideoWrite(q.at, q.es, q.di, q.n) {
			t.Fatal("write")
		}
	}
}
func TestStoryPage7PartialMixedDuplicateAndPresenterZeroDraw(t *testing.T) {
	c, b := p7(t)
	w, _ := NewStoryPage7Watcher(c)
	emit7(w, c.entries[0], b[0][0], 1, 10)
	if w.Active() {
		t.Fatal("partial")
	}
	emit7(w, c.entries[1], b[1][0], 1, 12)
	if w.p != nil || w.Active() {
		t.Fatal("mixed")
	}
	emit7(w, c.entries[0], b[0][0], 1, 20)
	emit7(w, c.entries[0], b[0][1], 2, 22)
	emit7(w, c.entries[0], b[0][0], 1, 24)
	if w.Active() {
		t.Fatal("duplicate")
	}
	emit7(w, c.entries[0], b[0][0], 1, 40)
	emit7(w, c.entries[0], b[0][1], 2, 30)
	if w.p != nil || w.Active() {
		t.Fatal("step backward")
	}
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		text := map[string]string{}
		good := make([]StoryPage7Event, 6)
		for i, r := range []rune("甲乙丙丁戊己") {
			font.Glyphs[r] = make([]byte, 32)
			k := fmt.Sprintf("story.page7.line.%03d", i+1)
			text[k] = string(r)
			good[i] = StoryPage7Event{1, 0, 0, k, uint8(17 + i), 1}
		}
		o, e := NewRuntimeStoryPage7Overlay(text, font, scale)
		if e != nil {
			t.Fatal(e)
		}
		bad := append([]StoryPage7Event(nil), good...)
		bad[5].EventKey = bad[4].EventKey
		if o.Apply(bad, [256][3]uint8{}) == nil {
			t.Fatal("bad presenter")
		}
		base := ScaleIndexedRGBA(make([]byte, 320*200), [256][3]uint8{}, scale)
		got, _, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
		if d || string(base) != string(got) || len(o.ActiveKeys()) != 0 {
			t.Fatal("zero draw")
		}
		if e := o.Apply(good, [256][3]uint8{}); e != nil {
			t.Fatal(e)
		}
		o.Clear()
		got, _, d = o.Draw(make([]byte, 320*200), [256][3]uint8{})
		if d || string(base) != string(got) || len(o.ActiveKeys()) != 0 {
			t.Fatal("stale")
		}
	}
}
func TestStoryPage7CatalogReady(t *testing.T) {
	h := strings.Join(storyPage7EventHeader, "\t") + "\n"
	var r []string
	text := "key\ttranslation\tsource\n"
	for i, a := range storyPage7Approved {
		k := fmt.Sprintf("story.page7.line.%03d", i+1)
		r = append(r, fmt.Sprintf("%s\t%d\t%d\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t0\t0\tconfirmed\tREADY", k, i+1, a.n, a.h, 17+i))
		text += k + "\t甲\tt\n"
	}
	d := []byte(h + strings.Join(r, "\n") + "\n")
	if _, _, e := LoadStoryPage7Catalog("e", d, "t", []byte(text)); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"READY", "0763:04FF"} {
		if _, _, e := LoadStoryPage7Catalog("e", []byte(strings.Replace(string(d), v, "bad", 1)), "t", []byte(text)); e == nil {
			t.Fatal("drift accepted")
		}
	}
	if _, _, e := LoadStoryPage7Catalog("e", d, "t", []byte(strings.Replace(text, "story.page7.line.006\t甲\tt\n", "", 1))); e == nil {
		t.Fatal("missing translation accepted")
	}
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
