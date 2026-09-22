package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
	"strings"
	"testing"
)

func page3Fixture(t *testing.T) (*StoryPage3Catalog, [][]byte) {
	t.Helper()
	lines := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}}
	es := make([]StoryPage3Identity, 5)
	for i, b := range lines {
		es[i] = StoryPage3Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page3.line.00%d", i+1), OriginalLength: uint8(len(b)), OriginalSHA256: sha256.Sum256(b), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	c, e := NewStoryPage3Catalog(es)
	if e != nil {
		t.Fatal(e)
	}
	return c, lines
}
func TestStoryPage3PresenterRejectsMixedDuplicateAndMissingGeneration(t *testing.T) {
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	text := map[string]string{}
	for i, r := range []rune("甲乙丙丁戊") {
		f.Glyphs[r] = make([]byte, 32)
		text[fmt.Sprintf("story.page3.line.00%d", i+1)] = string(r)
	}
	good := make([]StoryPage3Event, 5)
	for i := range good {
		good[i] = StoryPage3Event{Generation: 1, EventKey: fmt.Sprintf("story.page3.line.00%d", i+1), Row: uint8(17 + i), Column: 1}
	}
	for _, scale := range []int{2, 3} {
		o, e := NewRuntimeStoryPage3Overlay(text, f, scale)
		if e != nil {
			t.Fatal(e)
		}
		for _, bad := range [][]StoryPage3Event{append([]StoryPage3Event(nil), good[:4]...), func() []StoryPage3Event { x := append([]StoryPage3Event(nil), good...); x[3].Generation = 2; return x }(), func() []StoryPage3Event {
			x := append([]StoryPage3Event(nil), good...)
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
}
func emitPage3(w *StoryPage3Watcher, e StoryPage3Identity, b byte, col uint8, step uint64) {
	a := [7]uint16{1, uint16(b), 1, 0, 10, uint16(e.Row), uint16(col)}
	w.ObserveGlyphEntry(e.Guard, e.Caller, 0x2222, 0x3333, a, step)
	w.ObserveVerifiedGlyphReturn(StoryPage3VerifiedReturn{EntryStep: step, PostCallStep: step + 1, Caller: e.Caller, SS: 0x2222, SP: 0x3345})
}
func TestStoryPage3AtomicRelativeReturnAndMeasuredInvalidation(t *testing.T) {
	c, lines := page3Fixture(t)
	w, _ := NewStoryPage3Watcher(c)
	step := uint64(10)
	for i, line := range lines {
		for j, b := range line {
			emitPage3(w, c.entries[i], b, uint8(1+j), step)
			step += 2
		}
	}
	if !w.Active() || len(w.Events()) != 5 {
		t.Fatalf("atomic group not active: %#v", w.Events())
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() {
		t.Fatal("measured intersecting pre-write did not clear")
	}
	if StoryPage3VideoSpanIntersects(0, 8) {
		t.Fatal("unrelated span intersects")
	}
}
func TestStoryPage3FailsClosedOnReturnAndDiscontinuity(t *testing.T) {
	c, _ := page3Fixture(t)
	w, _ := NewStoryPage3Watcher(c)
	a := [7]uint16{1, 1, 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(storyGlyphPrimitive, c.entries[0].Caller, 7, 9, a, 100)
	w.ObserveVerifiedGlyphReturn(StoryPage3VerifiedReturn{EntryStep: 100, PostCallStep: 101, Caller: c.entries[0].Caller, SS: 7, SP: 9 + storyGlyphStackDelta + 1})
	if w.Pending() || len(w.Events()) != 0 {
		t.Fatal("bad relative stack leaked candidate")
	}
	w.ObserveExecutionDiscontinuity()
	if w.Active() || w.Pending() {
		t.Fatal("discontinuity revived state")
	}
}

func TestStoryPage3WatcherRejectsIdentityDriftWithoutDraw(t *testing.T) {
	c, lines := page3Fixture(t)
	w, _ := NewStoryPage3Watcher(c)
	for _, tc := range []struct {
		name   string
		mutate func(*StoryPage3Identity)
	}{
		{"caller", func(e *StoryPage3Identity) { e.Caller = Address{1, 2} }}, {"guard", func(e *StoryPage3Identity) { e.Guard = Address{3, 4} }}, {"mode", func(e *StoryPage3Identity) { e.Mode = 2 }}, {"repeat", func(e *StoryPage3Identity) { e.Repeat = 2 }}, {"style", func(e *StoryPage3Identity) { e.Foreground = 9 }}, {"order", func(e *StoryPage3Identity) { e.Row = 18 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := c.entries[0]
			tc.mutate(&e)
			emitPage3(w, e, lines[0][0], 1, 10)
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

func TestStoryPage3HashPartialRestoreAndFontMissFailClosed(t *testing.T) {
	c, lines := page3Fixture(t)
	w, _ := NewStoryPage3Watcher(c)
	// A wrong final glyph completes a line but cannot create an event/group.
	for j, b := range lines[0] {
		if j == len(lines[0])-1 {
			b ^= 0xff
		}
		emitPage3(w, c.entries[0], b, uint8(j+1), uint64(10+j))
	}
	if len(w.Events()) != 0 || w.Active() {
		t.Fatal("hash drift emitted")
	}
	// Partial state and restore/discontinuity cannot revive the prior candidate.
	emitPage3(w, c.entries[0], lines[0][0], 1, 30)
	w.ObserveExecutionDiscontinuity()
	if w.Pending() || w.Active() || len(w.Events()) != 0 {
		t.Fatal("restore revived partial")
	}
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, scale := range []int{2, 3} {
		if _, e := NewRuntimeStoryPage3Overlay(map[string]string{"story.page3.line.001": "缺", "story.page3.line.002": "缺", "story.page3.line.003": "缺", "story.page3.line.004": "缺", "story.page3.line.005": "缺"}, f, scale); e == nil {
			t.Fatalf("font miss accepted at %dx", scale)
		}
	}
}

func TestLoadStoryPage3CatalogRejectsNonREADYFixture(t *testing.T) {
	header := strings.Join(storyPage3EventHeader, "\t") + "\n"
	var rows []string
	for i := 1; i <= 5; i++ {
		rows = append(rows, fmt.Sprintf("story.page3.line.00%d\t%d\t1\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t0\t0\tconfirmed\tDRAFT", i, i, strings.Repeat("0", 64), 16+i))
	}
	text := "key\ttranslation\tsource\n"
	for i := 1; i <= 5; i++ {
		text += fmt.Sprintf("story.page3.line.00%d\t甲\tt\n", i)
	}
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			if _, _, e := LoadStoryPage3Catalog("events", []byte(header+strings.Join(rows, "\n")+"\n"), "text", []byte(text)); e == nil {
				t.Fatal("DRAFT catalog accepted before presenter creation")
			}
		})
	}
}

func TestLoadStoryPage3CatalogLocksReviewedIdentity(t *testing.T) {
	base := make([][]string, 5)
	translations := "key\ttranslation\tsource\n"
	for i := range base {
		key := fmt.Sprintf("story.page3.line.%03d", i+1)
		base[i] = []string{key, fmt.Sprint(i + 1), fmt.Sprint(storyPage3Approved[i].length), storyPage3Approved[i].digest,
			"0763:04FF", "0763:026B", "0", "10", fmt.Sprint(17 + i), "1", "0", "0", "confirmed", "READY"}
		translations += key + "\t甲\tt\n"
	}
	load := func(rows [][]string) error {
		var lines []string
		for _, r := range rows {
			lines = append(lines, strings.Join(r, "\t"))
		}
		_, _, err := LoadStoryPage3Catalog("events", []byte(strings.Join(storyPage3EventHeader, "\t")+"\n"+strings.Join(lines, "\n")+"\n"), "text", []byte(translations))
		return err
	}
	if err := load(base); err != nil {
		t.Fatalf("reviewed READY identity rejected: %v", err)
	}
	for _, tc := range []struct {
		name, value string
		field       int
	}{
		{"key", "story.page3.line.999", 0},
		{"sequence", "2", 1},
		{"length", "36", 2},
		{"digest", strings.Repeat("0", 64), 3},
		{"caller", "0763:0500", 4},
		{"guard", "0763:026C", 5},
		{"row", "18", 8},
		{"status", "DRAFT", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := make([][]string, 5)
			for i, r := range base {
				rows[i] = append([]string(nil), r...)
			}
			rows[0][tc.field] = tc.value
			if err := load(rows); err == nil {
				t.Fatal("drifted READY identity accepted")
			}
		})
	}
}

// TestStoryPage3FailureMatrixNeverDraws integrates watcher and presenter at
// both scales: rejected identity/partial/hash/restore inputs cannot revive a stamp.
func TestStoryPage3FailureMatrixNeverDraws(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, r := range []rune("甲乙丙丁戊") {
				f.Glyphs[r] = make([]byte, 32)
				text[fmt.Sprintf("story.page3.line.00%d", i+1)] = string(r)
			}
			o, e := NewRuntimeStoryPage3Overlay(text, f, scale)
			if e != nil {
				t.Fatal(e)
			}
			c, lines := page3Fixture(t)
			for _, kind := range []string{"caller", "guard", "mode", "repeat", "style", "order", "hash", "partial", "restore", "return_caller", "return_ss", "return_sp", "unknown_write"} {
				w, _ := NewStoryPage3Watcher(c)
				e := c.entries[0]
				if kind == "caller" {
					e.Caller = Address{1, 2}
				}
				if kind == "guard" {
					e.Guard = Address{3, 4}
				}
				if kind == "mode" {
					e.Mode = 2
				}
				if kind == "repeat" {
					e.Repeat = 2
				}
				if kind == "style" {
					e.Foreground = 9
				}
				if kind == "order" {
					e.Row = 18
				}
				if kind == "hash" {
					// Complete the line before asserting SHA mismatch. A single
					// bad glyph only proves an unfinished candidate cannot draw.
					for j, b := range lines[0] {
						if j == len(lines[0])-1 {
							b ^= 1
						}
						emitPage3(w, e, b, uint8(j+1), uint64(1+j*2))
					}
					if w.misses != 1 || w.Pending() {
						t.Fatal("completed line did not reach SHA mismatch")
					}
				} else if strings.HasPrefix(kind, "return_") {
					a := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
					w.ObserveGlyphEntry(e.Guard, e.Caller, 0x2222, 0x3333, a, 1)
					r := StoryPage3VerifiedReturn{EntryStep: 1, PostCallStep: 2, Caller: e.Caller, SS: 0x2222, SP: 0x3345}
					switch kind {
					case "return_caller":
						r.Caller = Address{1, 2}
					case "return_ss":
						r.SS++
					case "return_sp":
						r.SP++
					}
					w.ObserveVerifiedGlyphReturn(r)
					w.ObserveExecutionDiscontinuity()
				} else {
					emitPage3(w, e, lines[0][0], 1, 1)
				}
				if kind == "partial" || kind == "restore" {
					w.ObserveExecutionDiscontinuity()
				}
				if kind == "unknown_write" && w.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) {
					t.Fatal("unknown write gained lifecycle authority")
				}
				if len(w.Events()) != 0 || w.Active() || o.Apply(nil, [256][3]uint8{}) == nil {
					t.Fatalf("%s accepted", kind)
				}
				base := ScaleIndexedRGBA(make([]byte, 320*200), [256][3]uint8{}, scale)
				got, _, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
				if d || len(o.ActiveKeys()) != 0 || string(got) != string(base) {
					t.Fatalf("%s drew stale", kind)
				}
			}
		})
	}
}

func TestStoryPage3UnknownWriteHasNoLifecycleAuthorityAtBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			c, lines := page3Fixture(t)
			w, _ := NewStoryPage3Watcher(c)
			step := uint64(1)
			for i, line := range lines {
				for j, b := range line {
					emitPage3(w, c.entries[i], b, uint8(j+1), step)
					step += 2
				}
			}
			if !w.Active() || len(w.Events()) != 5 {
				t.Fatal("complete group did not activate")
			}
			font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, r := range []rune("甲乙丙丁戊") {
				font.Glyphs[r] = make([]byte, 32)
				text[fmt.Sprintf("story.page3.line.00%d", i+1)] = string(r)
			}
			o, err := NewRuntimeStoryPage3Overlay(text, font, scale)
			if err != nil {
				t.Fatal(err)
			}
			palette := [256][3]uint8{}
			if err := o.Apply(w.Events(), palette); err != nil {
				t.Fatal(err)
			}
			if w.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) || !w.Active() {
				t.Fatal("unknown instruction cleared active group")
			}
			if w.ObserveVideoWrite(storyFillInstruction, 0xB800, 0xaa08, 304) || !w.Active() {
				t.Fatal("non-video segment cleared active group")
			}
			indexed := make([]byte, 320*200)
			_, missing, drew := o.Draw(indexed, palette)
			if !drew || len(missing) != 0 || len(o.ActiveKeys()) != 5 {
				t.Fatal("unknown write changed existing output-only group")
			}
			if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() {
				t.Fatal("measured pre-write did not clear watcher")
			}
			o.Clear()
			got, missing, drew := o.Draw(indexed, palette)
			base := ScaleIndexedRGBA(indexed, palette, scale)
			if drew || len(missing) != 0 || len(o.ActiveKeys()) != 0 || string(got) != string(base) {
				t.Fatal("measured pre-write left stale output")
			}
		})
	}
}
