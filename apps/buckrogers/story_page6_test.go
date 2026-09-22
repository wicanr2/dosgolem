package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func page6Fixture(t *testing.T) (*StoryPage6Catalog, [][]byte) {
	t.Helper()
	lines := [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}, {11, 12}}
	entries := make([]StoryPage6Identity, 6)
	for i, bytes := range lines {
		entries[i] = StoryPage6Identity{Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.page6.line.%03d", i+1), OriginalLength: uint8(len(bytes)), OriginalSHA256: sha256.Sum256(bytes), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	catalog, err := NewStoryPage6Catalog(entries)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, lines
}
func emitPage6(watcher *StoryPage6Watcher, entry StoryPage6Identity, glyph, column byte, step uint64) {
	args := [7]uint16{1, uint16(glyph), 1, 0, 10, uint16(entry.Row), uint16(column)}
	watcher.ObserveGlyphEntry(entry.Guard, entry.Caller, 0x2222, 0x3333, args, step)
	watcher.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, entry.Caller, 0x2222, 0x3345, step+1)
}
func activatePage6(t *testing.T, watcher *StoryPage6Watcher, catalog *StoryPage6Catalog, lines [][]byte) {
	t.Helper()
	step := uint64(10)
	for i, line := range lines {
		for column, glyph := range line {
			emitPage6(watcher, catalog.entries[i], glyph, byte(column+1), step)
			step += 2
		}
	}
	if !watcher.Active() || len(watcher.Events()) != 6 {
		t.Fatalf("six-line atomic group inactive: %#v", watcher.Events())
	}
}

func TestStoryPage6AtomicRETFAndMeasuredClear(t *testing.T) {
	catalog, lines := page6Fixture(t)
	watcher, _ := NewStoryPage6Watcher(catalog)
	activatePage6(t, watcher, catalog, lines)
	if !watcher.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || watcher.Active() {
		t.Fatal("measured page6 pre-write did not clear")
	}
	if StoryPage6VideoSpanIntersects(0, 8) || !StoryPage6VideoSpanIntersects(0xaa08, 304) {
		t.Fatal("page6 half-open rectangle mismatch")
	}
}

func TestStoryPage6FailClosedABIAndReturn(t *testing.T) {
	catalog, lines := page6Fixture(t)
	for word := 0; word < 7; word++ {
		watcher, _ := NewStoryPage6Watcher(catalog)
		args := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
		args[word] |= 0x100
		watcher.ObserveGlyphEntry(catalog.entries[0].Guard, catalog.entries[0].Caller, 7, 9, args, 10)
		if watcher.Pending() || watcher.Active() {
			t.Fatalf("ABI word %d high byte accepted", word)
		}
	}
	for _, test := range []struct {
		name             string
		previous, caller Address
		opcode           byte
		ss, sp           uint16
		post             uint64
	}{
		{"predecessor", Address{1, 2}, Address{0x0763, 0x04ff}, 0xca, 7, 9 + storyGlyphStackDelta, 11},
		{"opcode", Address{0x0763, 0x03d6}, Address{0x0763, 0x04ff}, 0, 7, 9 + storyGlyphStackDelta, 11},
		{"caller", Address{0x0763, 0x03d6}, Address{1, 2}, 0xca, 7, 9 + storyGlyphStackDelta, 11},
		{"ss", Address{0x0763, 0x03d6}, Address{0x0763, 0x04ff}, 0xca, 8, 9 + storyGlyphStackDelta, 11},
		{"sp", Address{0x0763, 0x03d6}, Address{0x0763, 0x04ff}, 0xca, 7, 9 + storyGlyphStackDelta + 1, 11},
		{"step", Address{0x0763, 0x03d6}, Address{0x0763, 0x04ff}, 0xca, 7, 9 + storyGlyphStackDelta, 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			watcher, _ := NewStoryPage6Watcher(catalog)
			watcher.ObserveGlyphEntry(catalog.entries[0].Guard, catalog.entries[0].Caller, 7, 9, [7]uint16{1, 1, 1, 0, 10, 17, 1}, 10)
			watcher.ObserveVerifiedGlyphReturn(test.previous, test.opcode, test.caller, test.ss, test.sp, test.post)
			if watcher.Pending() || watcher.Active() || len(watcher.Events()) != 0 {
				t.Fatal("bad return survived")
			}
		})
	}
}

func TestStoryPage6FailureMatrixNeverDraws(t *testing.T) {
	catalog, lines := page6Fixture(t)
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, value := range []rune("甲乙丙丁戊己") {
				font.Glyphs[value] = make([]byte, 32)
				text[fmt.Sprintf("story.page6.line.%03d", i+1)] = string(value)
			}
			for _, kind := range []string{"caller", "guard", "mode", "repeat", "style", "order", "hash", "partial", "discontinuity", "unknown", "disjoint", "step_order"} {
				watcher, _ := NewStoryPage6Watcher(catalog)
				identity := catalog.entries[0]
				switch kind {
				case "caller":
					identity.Caller = Address{1, 2}
				case "guard":
					identity.Guard = Address{3, 4}
				case "style":
					identity.Foreground = 9
				case "order":
					identity.Row = 18
				}
				if kind == "hash" {
					emitPage6(watcher, identity, lines[0][0], 1, 1)
					emitPage6(watcher, identity, lines[0][1]^1, 2, 3)
					if watcher.Pending() {
						t.Fatal("completed SHA mismatch retained a candidate")
					}
				} else if kind == "mode" || kind == "repeat" {
					args := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
					if kind == "mode" {
						args[0] = 2
					} else {
						args[2] = 2
					}
					watcher.ObserveGlyphEntry(identity.Guard, identity.Caller, 0x2222, 0x3333, args, 1)
					watcher.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, identity.Caller, 0x2222, 0x3345, 2)
				} else if kind == "step_order" {
					emitPage6(watcher, identity, lines[0][0], 1, 3)
					emitPage6(watcher, identity, lines[0][1], 2, 1)
				} else {
					emitPage6(watcher, identity, lines[0][0], 1, 1)
				}
				if kind == "partial" || kind == "discontinuity" {
					watcher.ObserveExecutionDiscontinuity()
				}
				if kind == "unknown" && watcher.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) {
					t.Fatal("unknown write cleared")
				}
				if kind == "disjoint" && watcher.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0, 8) {
					t.Fatal("disjoint write cleared")
				}
				overlay, err := NewRuntimeStoryPage6Overlay(text, font, scale)
				if err != nil {
					t.Fatal(err)
				}
				if watcher.Active() || len(watcher.Events()) != 0 || overlay.Apply(nil, [256][3]uint8{}) == nil {
					t.Fatalf("%s accepted", kind)
				}
				base := ScaleIndexedRGBA(make([]byte, 320*200), [256][3]uint8{}, scale)
				got, _, drew := overlay.Draw(make([]byte, 320*200), [256][3]uint8{})
				if drew || len(overlay.ActiveKeys()) != 0 || string(got) != string(base) {
					t.Fatalf("%s drew", kind)
				}
			}
		})
	}
}

func TestStoryPage6WriteAndPresenterLifecycleAtBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			catalog, lines := page6Fixture(t)
			watcher, _ := NewStoryPage6Watcher(catalog)
			activatePage6(t, watcher, catalog, lines)
			font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
			text := map[string]string{}
			for i, value := range []rune("甲乙丙丁戊己") {
				font.Glyphs[value] = make([]byte, 32)
				text[fmt.Sprintf("story.page6.line.%03d", i+1)] = string(value)
			}
			overlay, err := NewRuntimeStoryPage6Overlay(text, font, scale)
			if err != nil {
				t.Fatal(err)
			}
			palette := [256][3]uint8{}
			if err := overlay.Apply(watcher.Events(), palette); err != nil {
				t.Fatal(err)
			}
			if watcher.ObserveVideoWrite(Address{1, 2}, storyVideoSegment, 0xaa08, 304) || watcher.ObserveVideoWrite(storyFillInstruction, 0xb800, 0xaa08, 304) || watcher.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0, 8) || !watcher.Active() {
				t.Fatal("unapproved write changed group")
			}
			_, missing, drew := overlay.Draw(make([]byte, 320*200), palette)
			if !drew || len(missing) != 0 || len(overlay.ActiveKeys()) != 6 {
				t.Fatal("active presenter did not draw")
			}
			if !watcher.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || watcher.Active() {
				t.Fatal("measured write not accepted")
			}
			overlay.Clear()
			got, missing, drew := overlay.Draw(make([]byte, 320*200), palette)
			base := ScaleIndexedRGBA(make([]byte, 320*200), palette, scale)
			if drew || len(missing) != 0 || len(overlay.ActiveKeys()) != 0 || string(got) != string(base) {
				t.Fatal("clear left stale output")
			}
		})
	}
}

func TestStoryPage6CatalogREADYAndFontFailClosed(t *testing.T) {
	header := strings.Join(storyPage6EventHeader, "\t") + "\n"
	rows := make([]string, 0, 6)
	text := "key\ttranslation\tsource\n"
	for i, approved := range storyPage6Approved {
		key := fmt.Sprintf("story.page6.line.%03d", i+1)
		rows = append(rows, fmt.Sprintf("%s\t%d\t%d\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t0\t0\tconfirmed\tREADY", key, i+1, approved.length, approved.digest, 17+i))
		text += key + "\t甲\tt\n"
	}
	data := []byte(header + strings.Join(rows, "\n") + "\n")
	if _, _, err := LoadStoryPage6Catalog("events", data, "text", []byte(text)); err != nil {
		t.Fatal(err)
	}
	draft := strings.Replace(string(data), "\tREADY", "\tDRAFT", 1)
	if _, _, err := LoadStoryPage6Catalog("events", []byte(draft), "text", []byte(text)); err == nil {
		t.Fatal("DRAFT accepted")
	}
	for _, scale := range []int{2, 3} {
		missingText := map[string]string{}
		for i := range storyPage6Approved {
			missingText[fmt.Sprintf("story.page6.line.%03d", i+1)] = "缺"
		}
		if _, err := NewRuntimeStoryPage6Overlay(missingText, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, scale); err == nil {
			t.Fatal("missing glyph accepted")
		}
	}
}

func TestStoryPage6PresenterApplyIsAtomicAtBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		text := map[string]string{}
		good := make([]StoryPage6Event, 6)
		for i, value := range []rune("甲乙丙丁戊己") {
			font.Glyphs[value] = make([]byte, 32)
			key := fmt.Sprintf("story.page6.line.%03d", i+1)
			text[key] = string(value)
			good[i] = StoryPage6Event{Generation: 1, EventKey: key, Row: uint8(17 + i), Column: 1}
		}
		badCases := [][]StoryPage6Event{
			func() []StoryPage6Event { out := append([]StoryPage6Event(nil), good...); out[5].Row = 99; return out }(),
			func() []StoryPage6Event {
				out := append([]StoryPage6Event(nil), good...)
				out[4].Generation = 2
				return out
			}(),
			func() []StoryPage6Event {
				out := append([]StoryPage6Event(nil), good...)
				out[5].EventKey = out[4].EventKey
				return out
			}(),
		}
		for _, bad := range badCases {
			overlay, err := NewRuntimeStoryPage6Overlay(text, font, scale)
			if err != nil {
				t.Fatal(err)
			}
			palette := [256][3]uint8{}
			if err := overlay.Apply(bad, palette); err == nil {
				t.Fatal("invalid trailing event accepted")
			}
			base := ScaleIndexedRGBA(make([]byte, 320*200), palette, scale)
			got, _, drew := overlay.Draw(make([]byte, 320*200), palette)
			if drew || len(overlay.ActiveKeys()) != 0 || string(got) != string(base) {
				t.Fatal("failed apply left partial output")
			}
		}
	}
}
