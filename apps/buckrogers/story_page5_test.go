package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
	"strings"
	"testing"
)

func page5Fixture(t *testing.T) (*StoryPage5Catalog, [][]byte) {
	t.Helper()
	lines := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}}
	es := make([]StoryPage5Identity, 5)
	for i, b := range lines {
		es[i] = StoryPage5Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page5.line.%03d", i+1), OriginalLength: uint8(len(b)), OriginalSHA256: sha256.Sum256(b), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	c, e := NewStoryPage5Catalog(es)
	if e != nil {
		t.Fatal(e)
	}
	return c, lines
}
func emitPage5(w *StoryPage5Watcher, e StoryPage5Identity, b byte, col uint8, step uint64) {
	a := [7]uint16{1, uint16(b), 1, 0, 10, uint16(e.Row), uint16(col)}
	w.ObserveGlyphEntry(e.Guard, e.Caller, 0x2222, 0x3333, a, step)
	w.ObserveVerifiedGlyphReturn(StoryPage5VerifiedReturn{EntryStep: step, PostCallStep: step + 1, Caller: e.Caller, SS: 0x2222, SP: 0x3345})
}

func TestStoryPage5AtomicTwoStageReturnAndClear(t *testing.T) {
	c, lines := page5Fixture(t)
	w, _ := NewStoryPage5Watcher(c)
	step := uint64(10)
	for i, line := range lines {
		for j, b := range line {
			emitPage5(w, c.entries[i], b, uint8(1+j), step)
			step += 2
		}
	}
	if !w.Active() || len(w.Events()) != 5 {
		t.Fatalf("atomic group not active: %#v", w.Events())
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() {
		t.Fatal("measured intersecting pre-write did not clear")
	}
	if StoryPage5VideoSpanIntersects(0, 8) {
		t.Fatal("unrelated span intersects")
	}
}
func TestStoryPage5RejectsReturnHighWordAndDiscontinuity(t *testing.T) {
	c, lines := page5Fixture(t)
	for i := 0; i < 7; i++ {
		w, _ := NewStoryPage5Watcher(c)
		a := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
		a[i] |= 0x100
		w.ObserveGlyphEntry(c.entries[0].Guard, c.entries[0].Caller, 7, 9, a, 100)
		w.ObserveVerifiedGlyphReturn(StoryPage5VerifiedReturn{EntryStep: 100, PostCallStep: 101, Caller: c.entries[0].Caller, SS: 7, SP: 9 + storyGlyphStackDelta})
		if w.Pending() || w.Active() || len(w.Events()) != 0 || w.drops != 1 {
			t.Fatalf("ABI word %d high byte accepted", i)
		}
	}
	w, _ := NewStoryPage5Watcher(c)
	a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(storyGlyphPrimitive, c.entries[0].Caller, 7, 9, a, 100)
	w.ObserveVerifiedGlyphReturn(StoryPage5VerifiedReturn{EntryStep: 100, PostCallStep: 101, Caller: c.entries[0].Caller, SS: 7, SP: 9 + storyGlyphStackDelta + 1})
	if w.Pending() {
		t.Fatal("bad stack leaked")
	}
	w.ObserveExecutionDiscontinuity()
	if w.Active() || w.Pending() {
		t.Fatal("discontinuity revived")
	}
}
func TestStoryPage5FailureMatrixNeverDraws(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, r := range []rune("甲乙丙丁戊") {
				f.Glyphs[r] = make([]byte, 32)
				text[fmt.Sprintf("story.page5.line.%03d", i+1)] = string(r)
			}
			o, e := NewRuntimeStoryPage5Overlay(text, f, scale)
			if e != nil {
				t.Fatal(e)
			}
			c, lines := page5Fixture(t)
			for _, kind := range []string{"caller", "guard", "mode", "repeat", "style", "order", "hash", "partial", "restore", "return_caller", "return_ss", "return_sp", "unknown_write"} {
				w, _ := NewStoryPage5Watcher(c)
				id := c.entries[0]
				switch kind {
				case "caller":
					id.Caller = Address{1, 2}
				case "guard":
					id.Guard = Address{3, 4}
				case "mode":
					id.Mode = 2
				case "repeat":
					id.Repeat = 2
				case "style":
					id.Foreground = 9
				case "order":
					id.Row = 18
				}
				if kind == "hash" {
					for j, b := range lines[0] {
						if j == len(lines[0])-1 {
							b ^= 1
						}
						emitPage5(w, id, b, uint8(j+1), uint64(j+1))
					}
				} else if strings.HasPrefix(kind, "return_") {
					a := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
					w.ObserveGlyphEntry(id.Guard, id.Caller, 0x2222, 0x3333, a, 1)
					r := StoryPage5VerifiedReturn{EntryStep: 1, PostCallStep: 2, Caller: id.Caller, SS: 0x2222, SP: 0x3345}
					if kind == "return_caller" {
						r.Caller = Address{1, 2}
					} else if kind == "return_ss" {
						r.SS++
					} else {
						r.SP++
					}
					w.ObserveVerifiedGlyphReturn(r)
				} else {
					emitPage5(w, id, lines[0][0], 1, 1)
				}
				if kind == "partial" || kind == "restore" {
					w.ObserveExecutionDiscontinuity()
				}
				if kind == "unknown_write" && w.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) {
					t.Fatal("unknown write cleared")
				}
				if len(w.Events()) != 0 || w.Active() || o.Apply(nil, [256][3]uint8{}) == nil {
					t.Fatalf("%s accepted", kind)
				}
				base := ScaleIndexedRGBA(make([]byte, 320*200), [256][3]uint8{}, scale)
				got, _, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
				if d || len(o.ActiveKeys()) != 0 || string(got) != string(base) {
					t.Fatalf("%s drew", kind)
				}
			}
		})
	}
}
func TestStoryPage5CatalogLocksREADY(t *testing.T) {
	header := strings.Join(storyPage5EventHeader, "\t") + "\n"
	var rows []string
	text := "key\ttranslation\tsource\n"
	for i := range storyPage5Approved {
		key := fmt.Sprintf("story.page5.line.%03d", i+1)
		rows = append(rows, fmt.Sprintf("%s\t%d\t%d\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t0\t0\tproven\tREADY", key, i+1, storyPage5Approved[i].length, storyPage5Approved[i].digest, 17+i))
		text += key + "\t甲\tt\n"
	}
	if _, _, e := LoadStoryPage5Catalog("events", []byte(header+strings.Join(rows, "\n")+"\n"), "text", []byte(text)); e != nil {
		t.Fatal(e)
	}
	rows[0] = strings.Replace(rows[0], "\tREADY", "\tDRAFT", 1)
	if _, _, e := LoadStoryPage5Catalog("events", []byte(header+strings.Join(rows, "\n")+"\n"), "text", []byte(text)); e == nil {
		t.Fatal("DRAFT catalog accepted")
	}
}
