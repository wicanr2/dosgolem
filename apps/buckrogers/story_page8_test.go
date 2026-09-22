package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func page8TestCatalog(t *testing.T) (*StoryPage8Catalog, [][]byte) {
	t.Helper()
	lengths := []int{38, 37, 33, 22}
	raw := make([][]byte, 4)
	entries := make([]StoryPage8Identity, 4)
	for i, length := range lengths {
		raw[i] = make([]byte, length)
		for j := range raw[i] {
			raw[i][j] = byte(0x20 + i*40 + j)
		}
		entries[i] = StoryPage8Identity{
			Sequence: uint8(i + 1), OriginalLength: uint8(length), Mode: 1, Repeat: 1,
			Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1,
			EventKey: fmt.Sprintf("story.page8.line.%03d", i+1), OriginalSHA256: sha256.Sum256(raw[i]),
			Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive,
		}
	}
	return &StoryPage8Catalog{entries: [4]StoryPage8Identity{entries[0], entries[1], entries[2], entries[3]}}, raw
}

func emit8(w *StoryPage8Watcher, identity StoryPage8Identity, glyph byte, column uint8, step uint64) {
	args := [7]uint16{1, uint16(glyph), 1, 0, 10, uint16(identity.Row), uint16(column)}
	w.ObserveGlyphEntry(identity.Guard, identity.Caller, 7, 9, args, step)
	w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, identity.Caller, 7, 9+storyGlyphStackDelta, step+1)
}

func emitPage8Group(w *StoryPage8Watcher, catalog *StoryPage8Catalog, raw [][]byte) {
	step := uint64(10)
	for i, line := range raw {
		for j, glyph := range line {
			emit8(w, catalog.entries[i], glyph, uint8(j+1), step)
			step += 2
		}
	}
}

func TestStoryPage8WatcherCommitsOnlyAfterAll130Edges(t *testing.T) {
	catalog, raw := page8TestCatalog(t)
	w, err := NewStoryPage8Watcher(catalog)
	if err != nil {
		t.Fatal(err)
	}
	step, edges := uint64(10), 0
	for i, line := range raw {
		for j, glyph := range line {
			emit8(w, catalog.entries[i], glyph, uint8(j+1), step)
			step += 2
			edges++
			if edges < 130 && (w.Active() || len(w.Events()) != 0) {
				t.Fatalf("edge %d 過早提交", edges)
			}
		}
	}
	if edges != 130 || !w.Active() || len(w.Events()) != 4 {
		t.Fatalf("edges=%d active=%v events=%d", edges, w.Active(), len(w.Events()))
	}
	for i, event := range w.Events() {
		if event.EventKey != fmt.Sprintf("story.page8.line.%03d", i+1) || event.Row != uint8(17+i) || event.Column != 1 || event.EntryStep >= event.PostCallStep || (i > 0 && w.Events()[i-1].PostCallStep >= event.EntryStep) {
			t.Fatalf("event %d 無效: %#v", i, event)
		}
	}
}

func TestStoryPage8WatcherFailClosedMatrix(t *testing.T) {
	catalog, raw := page8TestCatalog(t)
	assertClosed := func(t *testing.T, run func(*StoryPage8Watcher)) {
		t.Helper()
		w, _ := NewStoryPage8Watcher(catalog)
		run(w)
		if w.Active() || len(w.Events()) != 0 {
			t.Fatalf("失敗案例不得提交: active=%v events=%d", w.Active(), len(w.Events()))
		}
	}
	abiNames := [...]string{"mode", "glyph", "repeat", "background", "foreground/style", "row", "column"}
	for i := 0; i < 7; i++ {
		i := i
		t.Run("ABI high "+abiNames[i], func(t *testing.T) {
			assertClosed(t, func(w *StoryPage8Watcher) {
				args := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
				args[i] |= 0x100
				w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, args, 10)
			})
		})
		t.Run("ABI low "+abiNames[i], func(t *testing.T) {
			assertClosed(t, func(w *StoryPage8Watcher) {
				args := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
				args[i] = (args[i] + 1) & 0xff
				w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, args, 10)
				if i == 1 {
					w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
					step := uint64(12)
					for column, glyph := range raw[0][1:] {
						emit8(w, catalog.entries[0], glyph, uint8(column+2), step)
						step += 2
					}
				}
			})
		})
	}

	for _, tc := range []struct {
		name string
		run  func(*StoryPage8Watcher)
	}{
		{"caller", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{1, 2}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
		}},
		{"guard", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(Address{1, 2}, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
		}},
		{"RETF previous", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{1, 2}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
		}},
		{"RETF opcode", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xcb, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
		}},
		{"RETF caller", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{1, 2}, 7, 9+storyGlyphStackDelta, 11)
		}},
		{"SS", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 8, 9+storyGlyphStackDelta, 11)
		}},
		{"SP", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta+1, 11)
		}},
		{"return equal", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 10)
		}},
		{"return backward", func(w *StoryPage8Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 9)
		}},
		{"entry not increasing", func(w *StoryPage8Watcher) {
			emit8(w, catalog.entries[0], raw[0][0], 1, 10)
			emit8(w, catalog.entries[0], raw[0][1], 2, 11)
		}},
		{"last glyph hash", func(w *StoryPage8Watcher) {
			step := uint64(10)
			for j, glyph := range raw[0] {
				if j == len(raw[0])-1 {
					glyph ^= 1
				}
				emit8(w, catalog.entries[0], glyph, uint8(j+1), step)
				step += 2
			}
		}},
		{"partial", func(w *StoryPage8Watcher) { emit8(w, catalog.entries[0], raw[0][0], 1, 10) }},
		{"mixed", func(w *StoryPage8Watcher) {
			emit8(w, catalog.entries[0], raw[0][0], 1, 10)
			emit8(w, catalog.entries[1], raw[1][0], 1, 12)
		}},
		{"duplicate", func(w *StoryPage8Watcher) {
			step := uint64(10)
			for j, glyph := range raw[0] {
				emit8(w, catalog.entries[0], glyph, uint8(j+1), step)
				step += 2
			}
			emit8(w, catalog.entries[0], raw[0][0], 1, step)
		}},
		{"discontinuity", func(w *StoryPage8Watcher) {
			emit8(w, catalog.entries[0], raw[0][0], 1, 10)
			w.ObserveExecutionDiscontinuity()
			emit8(w, catalog.entries[0], raw[0][1], 2, 12)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { assertClosed(t, tc.run) })
	}
}

func TestStoryPage8RowAwareInvalidationAndDiscontinuity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		di, count uint16
		want      bool
	}{
		{"empty", 136*320 + 8, 0, false},
		{"x7", 136*320 + 7, 1, false},
		{"x8", 136*320 + 8, 1, true},
		{"x311", 136*320 + 311, 1, true},
		{"x312", 136*320 + 312, 1, false},
		{"before top", 136*320 - 304, 304, false},
		{"after bottom", 168*320 + 8, 1, false},
		{"cross row into x7", 136*320 + 319, 8, false},
		{"cross row into x8", 136*320 + 319, 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := StoryPage8VideoSpanIntersects(tc.di, tc.count); got != tc.want {
				t.Fatalf("intersects(%#x,%d)=%v, want %v", tc.di, tc.count, got, tc.want)
			}
		})
	}

	catalog, raw := page8TestCatalog(t)
	w, _ := NewStoryPage8Watcher(catalog)
	emit8(w, catalog.entries[0], raw[0][0], 1, 10)
	for _, write := range []struct {
		at            Address
		es, di, count uint16
	}{
		{Address{1, 2}, storyVideoSegment, 136*320 + 8, 1},
		{storyFillInstruction, 0xb800, 136*320 + 8, 1},
		{storyFillInstruction, storyVideoSegment, 136*320 + 312, 1},
	} {
		if w.ObserveVideoWrite(write.at, write.es, write.di, write.count) || w.pending == nil {
			t.Fatal("未知／不相交 write 不得清 pending")
		}
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 136*320+8, 1) || w.pending != nil || w.Active() {
		t.Fatal("相交 write 必須清 pending")
	}

	w, _ = NewStoryPage8Watcher(catalog)
	emitPage8Group(w, catalog, raw)
	oldGeneration := w.Generation()
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xaa08, 304) || w.Active() || len(w.Events()) != 0 || w.Generation() != oldGeneration+1 {
		t.Fatal("active 相交 write 未原子失效")
	}
	emitPage8Group(w, catalog, raw)
	oldGeneration = w.Generation()
	w.ObserveExecutionDiscontinuity()
	if w.Active() || len(w.Events()) != 0 || w.Generation() != oldGeneration+1 {
		t.Fatal("active discontinuity 未原子失效")
	}
}

func page8CatalogFixture() (string, string) {
	events := strings.Join(storyPage8EventHeader, "\t") + "\n"
	translations := "key\ttranslation\tsource\n"
	for i, approved := range storyPage8Approved {
		key := fmt.Sprintf("story.page8.line.%03d", i+1)
		events += fmt.Sprintf("%s\t%d\t%d\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t%d\t%d\tconfirmed\tREADY\n", key, i+1, approved.length, approved.hash, 17+i, approved.entryStep, approved.postCallStep)
		translations += key + "\t甲\truntime-editorial\n"
	}
	return events, translations
}

func TestStoryPage8StrictReadyCatalog(t *testing.T) {
	events, translations := page8CatalogFixture()
	catalog, text, err := LoadStoryPage8Catalog("events", []byte(events), "translations", []byte(translations))
	if err != nil || catalog == nil || len(text) != 4 {
		t.Fatalf("有效 READY catalog 被拒絕: catalog=%v text=%d err=%v", catalog != nil, len(text), err)
	}
	for _, tc := range []struct {
		name, events, translations string
	}{
		{"DRAFT", strings.Replace(events, "READY", "DRAFT", 1), translations},
		{"evidence", strings.Replace(events, "confirmed", "hypothesis", 1), translations},
		{"caller", strings.Replace(events, "0763:04FF", "0763:0500", 1), translations},
		{"guard", strings.Replace(events, "0763:026B", "0763:026C", 1), translations},
		{"style", strings.Replace(events, "\t0\t10\t17\t1\t", "\t0\t9\t17\t1\t", 1), translations},
		{"row", strings.Replace(events, "\t0\t10\t17\t1\t", "\t0\t10\t18\t1\t", 1), translations},
		{"column", strings.Replace(events, "\t0\t10\t17\t1\t", "\t0\t10\t17\t2\t", 1), translations},
		{"hash", strings.Replace(events, storyPage8Approved[0].hash, strings.Repeat("0", 64), 1), translations},
		{"entry provenance", strings.Replace(events, fmt.Sprint(storyPage8Approved[0].entryStep), "1", 1), translations},
		{"post provenance", strings.Replace(events, fmt.Sprint(storyPage8Approved[0].postCallStep), "2", 1), translations},
		{"missing translation", events, strings.Replace(translations, "story.page8.line.004\t甲\truntime-editorial\n", "", 1)},
		{"translation provenance", events, strings.Replace(translations, "runtime-editorial", "manual", 1)},
		{"orphan translation", events, strings.Replace(translations, "story.page8.line.004", "story.page8.line.999", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := LoadStoryPage8Catalog("events", []byte(tc.events), "translations", []byte(tc.translations)); err == nil {
				t.Fatal("非 READY catalog 被接受")
			}
		})
	}

	identities := make([]StoryPage8Identity, 4)
	for i, approved := range storyPage8Approved {
		digest, _ := hex.DecodeString(approved.hash)
		var hash [32]byte
		copy(hash[:], digest)
		identities[i] = StoryPage8Identity{
			Sequence: uint8(i + 1), OriginalLength: approved.length, Mode: 1, Repeat: 1,
			Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1,
			EntryStep: approved.entryStep, PostCallStep: approved.postCallStep,
			EventKey: fmt.Sprintf("story.page8.line.%03d", i+1), OriginalSHA256: hash,
			Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive,
		}
	}
	if _, err := NewStoryPage8Catalog(identities); err != nil {
		t.Fatal(err)
	}
	identities[0].EntryStep++
	if _, err := NewStoryPage8Catalog(identities); err == nil {
		t.Fatal("公開建構子接受 provenance drift")
	}
}
