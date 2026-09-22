package buckrogers

import (
	"crypto/sha256"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func storyFixture(t *testing.T) (*StoryOpeningCatalog, [][]byte) {
	t.Helper()
	lines := [][]byte{[]byte("alpha"), []byte("bravo"), []byte("charlie"), []byte("delta"), []byte("echo")}
	entries := make([]StoryOpeningIdentity, len(lines))
	for i, line := range lines {
		entries[i] = StoryOpeningIdentity{
			Sequence: uint8(i + 1), EventKey: "synthetic.story." + string(rune('1'+i)),
			OriginalLength: uint8(len(line)), OriginalSHA256: sha256.Sum256(line),
			Caller: Address{Segment: 0x0763, Offset: 0x04FF}, Guard: storyGlyphPrimitive,
			Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1,
		}
	}
	catalog, err := NewStoryOpeningCatalog(entries)
	if err != nil {
		t.Fatalf("NewStoryOpeningCatalog: %v", err)
	}
	return catalog, lines
}

func emitStoryGlyph(w *StoryOpeningWatcher, entry StoryOpeningIdentity, glyph byte, column uint8, ss, sp uint16, step uint64) {
	args := [7]uint16{uint16(entry.Mode), uint16(glyph), uint16(entry.Repeat), uint16(entry.Background), uint16(entry.Foreground), uint16(entry.Row), uint16(column)}
	w.ObserveGlyphEntry(entry.Guard, entry.Caller, ss, sp, args, step)
	w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: step, PostCallStep: step + 1, Caller: entry.Caller, SS: ss, SP: sp + storyGlyphStackDelta})
}

func emitStoryLines(w *StoryOpeningWatcher, catalog *StoryOpeningCatalog, lines [][]byte, step uint64) {
	for i, line := range lines {
		for j, glyph := range line {
			emitStoryGlyph(w, catalog.entries[i], glyph, uint8(1+j), 0x8123, 0x1000, step)
			step += 2
		}
	}
}

func TestStoryOpeningWatcherCompletesAtomicallyAndInvalidates(t *testing.T) {
	catalog, lines := storyFixture(t)
	w, err := NewStoryOpeningWatcher(catalog)
	if err != nil {
		t.Fatal(err)
	}
	emitStoryLines(w, catalog, lines, 100)
	if !w.Active() || w.Pending() || len(w.Events()) != 5 || w.Generation() != 1 {
		t.Fatalf("完成後狀態 active=%v pending=%v events=%d generation=%d", w.Active(), w.Pending(), len(w.Events()), w.Generation())
	}
	for i, event := range w.Events() {
		if event.EventKey != catalog.entries[i].EventKey || event.Row != uint8(17+i) || event.OriginalLength != uint8(len(lines[i])) {
			t.Fatalf("event[%d]=%+v", i, event)
		}
	}
	if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 0xAB48, 304) {
		t.Fatal("已證實第二頁 span 必須使 active group 失效")
	}
	if w.Active() || w.Pending() || w.Generation() != 2 {
		t.Fatalf("失效後狀態 active=%v pending=%v generation=%d", w.Active(), w.Pending(), w.Generation())
	}
	// 第二頁若只碰巧畫出首行，絕不可重建五行 overlay。
	for j, glyph := range lines[0] {
		emitStoryGlyph(w, catalog.entries[0], glyph, uint8(1+j), 0x8123, 0x1000, uint64(400+j*2))
	}
	if w.Active() || len(w.Events()) != 5 {
		t.Fatalf("部分第二頁文字不應重建 overlay: active=%v events=%d", w.Active(), len(w.Events()))
	}
}

func TestStoryOpeningCatalogRejectsUnprovenShape(t *testing.T) {
	catalog, _ := storyFixture(t)
	entries := append([]StoryOpeningIdentity(nil), catalog.entries[:]...)
	entries[4].Column = 2
	if _, err := NewStoryOpeningCatalog(entries); err == nil {
		t.Fatal("非已證實 column 必須被拒絕")
	}
	entries = append([]StoryOpeningIdentity(nil), catalog.entries[:]...)
	entries[3].Caller = Address{Segment: 0x0763, Offset: 0x0500}
	if _, err := NewStoryOpeningCatalog(entries); err == nil {
		t.Fatal("非已證實 caller 必須被拒絕")
	}
	if _, err := NewStoryOpeningCatalog(entries[:4]); err == nil {
		t.Fatal("非五行 catalog 必須被拒絕")
	}
}

func TestStoryOpeningWatcherRejectsDriftAndNeverLeaksPartialEvents(t *testing.T) {
	catalog, lines := storyFixture(t)
	w, _ := NewStoryOpeningWatcher(catalog)

	// guard 漂移不可進入 collector。
	args := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(Address{Segment: 0x0763, Offset: 0x026C}, catalog.entries[0].Caller, 1, 2, args, 10)
	w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: 10, PostCallStep: 11, Caller: catalog.entries[0].Caller, SS: 1, SP: 2 + storyGlyphStackDelta})
	if w.Pending() || len(w.Events()) != 0 {
		t.Fatal("guard 漂移不可留下候選或 event")
	}

	// 完成第一行後，錯序下一行必須清掉 partial group。
	for j, glyph := range lines[0] {
		emitStoryGlyph(w, catalog.entries[0], glyph, uint8(1+j), 1, 2, uint64(20+j*2))
	}
	emitStoryGlyph(w, catalog.entries[2], lines[2][0], 1, 1, 2, 40)
	if w.Pending() || len(w.Events()) != 0 || w.Drops() == 0 {
		t.Fatalf("錯序必須 fail-closed: pending=%v events=%d drops=%d", w.Pending(), len(w.Events()), w.Drops())
	}

	// hash 不符只能記 miss，不可逸出 event。
	for j, glyph := range lines[0] {
		if j == len(lines[0])-1 {
			glyph ^= 0x01
		}
		emitStoryGlyph(w, catalog.entries[0], glyph, uint8(1+j), 1, 2, uint64(50+j*2))
	}
	if w.Pending() || len(w.Events()) != 0 || w.Misses() != 1 {
		t.Fatalf("hash miss 必須 fail-closed: pending=%v events=%d misses=%d", w.Pending(), len(w.Events()), w.Misses())
	}

	// SS/SP 漂移丟棄 pending。
	w.ObserveGlyphEntry(catalog.entries[0].Guard, catalog.entries[0].Caller, 1, 2, args, 80)
	w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: 80, PostCallStep: 81, Caller: catalog.entries[0].Caller, SS: 1, SP: 2 + storyGlyphStackDelta + 1})
	if w.Pending() || w.Drops() < 2 {
		t.Fatalf("stack 漂移必須丟棄: pending=%v drops=%d", w.Pending(), w.Drops())
	}
}

func TestStoryOpeningWatcherUsesOnlyProvenLowByteABIAndClearsStalePending(t *testing.T) {
	catalog, lines := storyFixture(t)
	w, _ := NewStoryOpeningWatcher(catalog)
	for i, line := range lines {
		for j, glyph := range line {
			args := [7]uint16{0x101, uint16(glyph) | 0x200, 0x301, 0x400, 0x50a, uint16(17+i) | 0x600, uint16(1+j) | 0x700}
			step := uint64(10 + i*100 + j*2)
			w.ObserveGlyphEntry(catalog.entries[i].Guard, catalog.entries[i].Caller, 1, 2, args, step)
			w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: step, PostCallStep: step + 1, Caller: catalog.entries[i].Caller, SS: 1, SP: 2 + storyGlyphStackDelta})
		}
	}
	if !w.Active() || len(w.Events()) != 5 {
		t.Fatalf("高位非零但低位 exact 的 ABI 必須完成：active=%v events=%d", w.Active(), len(w.Events()))
	}

	w, _ = NewStoryOpeningWatcher(catalog)
	args := [7]uint16{1, uint16(lines[0][0]), 1, 0, 10, 17, 1}
	w.ObserveGlyphEntry(catalog.entries[0].Guard, catalog.entries[0].Caller, 1, 2, args, 10)
	// 任意的 later caller visit 並不是 verified return；adapter 的不連續通知會
	// 使舊 frame 永遠不能在晚到的同位址被提交。
	w.ObserveExecutionDiscontinuity()
	w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: 10, PostCallStep: 1000, Caller: catalog.entries[0].Caller, SS: 1, SP: 2 + storyGlyphStackDelta})
	if w.Pending() || len(w.Events()) != 0 || w.Drops() == 0 {
		t.Fatalf("stale pending 必須被不連續事件關閉: pending=%v events=%d drops=%d", w.Pending(), len(w.Events()), w.Drops())
	}
}

func TestStoryOpeningVideoSpanIntersectionBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		di, count uint16
		want      bool
	}{
		{"confirmed-page2", 0xAB48, 304, true},
		{"zero", 0xAB48, 0, false},
		{"left-exclusive", 137*320 + 0, 8, false},
		{"right-touching", 137*320 + 312, 8, true},
		{"above", 135 * 320, 320, false},
		{"below", 176 * 320, 1, false},
		{"crosses-to-left-edge", 135*320 + 319, 10, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StoryOpeningVideoSpanIntersects(test.di, test.count); got != test.want {
				t.Fatalf("StoryOpeningVideoSpanIntersects(%#x,%d)=%v want %v", test.di, test.count, got, test.want)
			}
		})
	}
}

func TestStoryOpeningEventsAreDefensiveCopies(t *testing.T) {
	catalog, lines := storyFixture(t)
	w, _ := NewStoryOpeningWatcher(catalog)
	emitStoryLines(w, catalog, lines, 1)
	first := w.Events()
	first[0].EventKey = "mutated"
	if got := w.Events()[0].EventKey; got == "mutated" {
		t.Fatal("Events 必須回 defensive copy")
	}
}

// TestStoryOpeningETenSafeRectPrototype measures the actual local phase107
// ETen-derived GOLEMFNT, not East Asian Width. It is opt-in because the font
// and DRAFT TSV are project-local licensed inputs, not dosgolem fixtures.
func TestStoryOpeningETenSafeRectPrototype(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("設定 BUCKROGERS_CHT_ROOT 才量測本機倚天字型與 DRAFT 五行")
	}
	font, err := xlate.LoadFont(filepath.Join(root, "workplace/phase107-font/buckrogers-eten-top-pad.golemfnt"))
	if err != nil {
		t.Fatal(err)
	}
	if font.W != 16 || font.H != 16 {
		t.Fatalf("期望 phase107 16x16 字型，得到 %dx%d", font.W, font.H)
	}
	rows := loadStoryPrototypeRows(t, filepath.Join(root, "text/story-opening.zh-TW.tsv"))
	if len(rows) != 5 {
		t.Fatalf("期望五行 DRAFT TSV，得到 %d", len(rows))
	}
	for _, scale := range []int{2, 3} {
		renderFont, glyphOffset := font, (8*scale-16)/2
		if scale == 3 {
			renderFont, glyphOffset = manualThreeXFont(font), 1
		}
		var previous PixelRect
		for line, text := range rows {
			stamp := &xlate.Stamp{X: 8, Y: 136 + line*8, Cells: 39, CellW: 8, CellH: 8, Font: renderFont,
				GlyphX: glyphOffset, GlyphY: glyphOffset, GlyphScale: 1, Text: []rune(text)}
			ink, err := menuInkRect(stamp, scale)
			if err != nil {
				t.Fatalf("%dx line %d: %v", scale, line+1, err)
			}
			clear := PixelRect{X: 8 * scale, Y: (136 + line*8) * scale, Width: 312 * scale, Height: 8 * scale}
			contained := ink.X >= clear.X && ink.Y >= clear.Y && ink.X+ink.Width <= clear.X+clear.Width && ink.Y+ink.Height <= clear.Y+clear.Height
			if !contained || (line > 0 && pixelRectsOverlap(previous, ink)) {
				t.Fatalf("%dx line %d ink=%+v clear=%+v previous=%+v", scale, line+1, ink, clear, previous)
			}
			t.Logf("%dx 第 %d 行：runes=%d ink=%+v clear=%+v", scale, line+1, len([]rune(text)), ink, clear)
			previous = ink
		}
	}
}

func loadStoryPrototypeRows(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]string, 0, 5)
	for _, record := range records[1:] {
		if len(record) != 3 || !strings.HasPrefix(record[0], "story.opening.line.") || record[1] == "" {
			t.Fatalf("無效 DRAFT TSV row: %#v", record)
		}
		rows = append(rows, record[1])
	}
	return rows
}
