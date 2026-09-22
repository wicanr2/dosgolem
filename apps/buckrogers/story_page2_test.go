package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
	"strings"
	"testing"
)

func page2Fixture(t *testing.T) (*StoryPage2Catalog, [][]byte) {
	t.Helper()
	lines := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}
	es := make([]StoryPage2Identity, 4)
	for i, b := range lines {
		es[i] = StoryPage2Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page2.line.00%d", i+1), OriginalLength: uint8(len(b)), OriginalSHA256: sha256.Sum256(b), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
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
	good := make([]StoryPage2Event, 4)
	for i := range good {
		good[i] = StoryPage2Event{Generation: 1, EventKey: fmt.Sprintf("story.page2.line.00%d", i+1), Row: uint8(17 + i), Column: 1}
	}
	for _, scale := range []int{2, 3} {
		o, e := NewRuntimeStoryPage2Overlay(text, f, scale)
		if e != nil {
			t.Fatal(e)
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

func TestStoryPage2HashPartialRestoreAndFontMissFailClosed(t *testing.T) {
	c, lines := page2Fixture(t)
	w, _ := NewStoryPage2Watcher(c)
	// A wrong final glyph completes a line but cannot create an event/group.
	for j, b := range lines[0] {
		if j == len(lines[0])-1 {
			b ^= 0xff
		}
		emitPage2(w, c.entries[0], b, uint8(j+1), uint64(10+j))
	}
	if len(w.Events()) != 0 || w.Active() {
		t.Fatal("hash drift emitted")
	}
	// Partial state and restore/discontinuity cannot revive the prior candidate.
	emitPage2(w, c.entries[0], lines[0][0], 1, 30)
	w.ObserveExecutionDiscontinuity()
	if w.Pending() || w.Active() || len(w.Events()) != 0 {
		t.Fatal("restore revived partial")
	}
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, scale := range []int{2, 3} {
		if _, e := NewRuntimeStoryPage2Overlay(map[string]string{"story.page2.line.001": "缺", "story.page2.line.002": "缺", "story.page2.line.003": "缺", "story.page2.line.004": "缺"}, f, scale); e == nil {
			t.Fatalf("font miss accepted at %dx", scale)
		}
	}
}

func TestLoadStoryPage2CatalogRejectsNonREADYFixture(t *testing.T) {
	header := strings.Join(storyPage2EventHeader, "\t") + "\n"
	var rows []string
	for i := 1; i <= 4; i++ {
		rows = append(rows, fmt.Sprintf("story.page2.line.00%d\t%d\t1\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t0\t0\tconfirmed\tDRAFT", i, i, strings.Repeat("0", 64), 16+i))
	}
	text := "key\ttranslation\tsource\n"
	for i := 1; i <= 4; i++ {
		text += fmt.Sprintf("story.page2.line.00%d\t甲\tt\n", i)
	}
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			if _, _, e := LoadStoryPage2Catalog("events", []byte(header+strings.Join(rows, "\n")+"\n"), "text", []byte(text)); e == nil {
				t.Fatal("DRAFT catalog accepted before presenter creation")
			}
		})
	}
}

func TestLoadStoryPage2CatalogLocksReviewedIdentity(t *testing.T) {
	base := make([][]string, 4)
	translations := "key\ttranslation\tsource\n"
	for i := range base {
		key := fmt.Sprintf("story.page2.line.%03d", i+1)
		base[i] = []string{key, fmt.Sprint(i + 1), fmt.Sprint(storyPage2Approved[i].length), storyPage2Approved[i].digest,
			"0763:04FF", "0763:026B", "0", "10", fmt.Sprint(17 + i), "1", "0", "0", "confirmed", "READY"}
		translations += key + "\t甲\tt\n"
	}
	load := func(rows [][]string) error {
		var lines []string
		for _, r := range rows {
			lines = append(lines, strings.Join(r, "\t"))
		}
		_, _, err := LoadStoryPage2Catalog("events", []byte(strings.Join(storyPage2EventHeader, "\t")+"\n"+strings.Join(lines, "\n")+"\n"), "text", []byte(translations))
		return err
	}
	if err := load(base); err != nil {
		t.Fatalf("reviewed READY identity rejected: %v", err)
	}
	for _, tc := range []struct {
		name, value string
		field       int
	}{
		{"key", "story.page2.line.999", 0},
		{"sequence", "2", 1},
		{"length", "36", 2},
		{"digest", strings.Repeat("0", 64), 3},
		{"caller", "0763:0500", 4},
		{"guard", "0763:026C", 5},
		{"row", "18", 8},
		{"status", "DRAFT", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := make([][]string, 4)
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

// TestStoryPage2FailureMatrixNeverDraws integrates watcher and presenter at
// both scales: rejected identity/partial/hash/restore inputs cannot revive a stamp.
func TestStoryPage2FailureMatrixNeverDraws(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, r := range []rune("甲乙丙丁") {
				f.Glyphs[r] = make([]byte, 32)
				text[fmt.Sprintf("story.page2.line.00%d", i+1)] = string(r)
			}
			o, e := NewRuntimeStoryPage2Overlay(text, f, scale)
			if e != nil {
				t.Fatal(e)
			}
			c, lines := page2Fixture(t)
			for _, kind := range []string{"caller", "guard", "mode", "repeat", "style", "order", "hash", "partial", "restore", "return_caller", "return_ss", "return_sp", "unknown_write"} {
				w, _ := NewStoryPage2Watcher(c)
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
						emitPage2(w, e, b, uint8(j+1), uint64(1+j*2))
					}
					if w.misses != 1 || w.Pending() {
						t.Fatal("completed line did not reach SHA mismatch")
					}
				} else if strings.HasPrefix(kind, "return_") {
					a := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
					w.ObserveGlyphEntry(e.Guard, e.Caller, 0x2222, 0x3333, a, 1)
					r := StoryPage2VerifiedReturn{EntryStep: 1, PostCallStep: 2, Caller: e.Caller, SS: 0x2222, SP: 0x3345}
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
					emitPage2(w, e, lines[0][0], 1, 1)
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

func TestStoryPage2UnknownWriteHasNoLifecycleAuthorityAtBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			c, lines := page2Fixture(t)
			w, _ := NewStoryPage2Watcher(c)
			step := uint64(1)
			for i, line := range lines {
				for j, b := range line {
					emitPage2(w, c.entries[i], b, uint8(j+1), step)
					step += 2
				}
			}
			if !w.Active() || len(w.Events()) != 4 {
				t.Fatal("complete group did not activate")
			}
			font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, r := range []rune("甲乙丙丁") {
				font.Glyphs[r] = make([]byte, 32)
				text[fmt.Sprintf("story.page2.line.00%d", i+1)] = string(r)
			}
			o, err := NewRuntimeStoryPage2Overlay(text, font, scale)
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
			if !drew || len(missing) != 0 || len(o.ActiveKeys()) != 4 {
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
