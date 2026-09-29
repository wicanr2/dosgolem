package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 039 §5.2 unit tests for the part-2 families: menus (001／018／020／
// 021), the scoped 3× main menu (004), the manual paragraph (005, 2× and the
// fixed-grid 3×), the manual English row (034) and the manual snapshot owner.
// Every font and text here is synthetic.

const halfPart2Runes = "離開至 DOS地球甲乙丙丁中文字繁段落ABCXRM"

// halfPart2Rects has one 8-cell row and a second row overlapping its right
// part, for the partial-overlap trigger.
const halfPart2Rects = "event_key\tx\ty\twidth\theight\tdraw_x\tdraw_y\tcapacity_cells\tline_count\toverflow_policy\n" +
	"race.option.terran\t8\t24\t64\t8\t8\t24\t8\t1\tsingle-line-reject\n" +
	"race.heading.terran\t24\t24\t48\t8\t24\t24\t6\t1\tsingle-line-reject\n"

func halfPart2Menu(t *testing.T, scale int) *RuntimeMenuOverlay {
	t.Helper()
	rects, err := LoadMenuOverlayRects("rects.tsv", []byte(halfPart2Rects))
	if err != nil {
		t.Fatal(err)
	}
	o, err := NewRuntimeMenuOverlay(rects, halfTestFont("part2", halfPart2Runes), scale)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func halfPart2Event(t *testing.T) TextEvent { return fixtureMenuEvent(t) } // row 3, column 1, 8 cells

func halfPart2Heading(t *testing.T) TextEvent {
	e := fixtureMenuEvent(t)
	e.Column, e.OriginalLength = 3, 6
	return e
}

// halfPart2Frame is a 320×200 frame of colour 1 whose row-3 cells listed in
// inked (8×8 columns from x=8) carry a colour-2 dot, i.e. anchor cells.
func halfPart2Frame(inked ...int) []byte {
	f := bytes.Repeat([]byte{1}, 320*200)
	for _, c := range inked {
		f[(24+3)*320+8+c*8+3] = 2
	}
	return f
}

func halfPart2Palette() [256][3]uint8 {
	var p [256][3]uint8
	p[1], p[2], p[3] = [3]uint8{10, 20, 30}, [3]uint8{200, 210, 220}, [3]uint8{90, 90, 90}
	return p
}

func TestHalfPart2BlankNeedsNoGlyph(t *testing.T) {
	font := halfTestFont("blank", halfPart2Runes) // no U+0020／U+3000 glyphs
	delete(font.Glyphs, ' ')
	var palette [256][3]uint8
	entry := MenuOverlayEntry{EventKey: "e", TextKey: "t", Translation: "地　球 AB", X: 8, Y: 24, Width: 64, Height: 8,
		DrawX: 8, DrawY: 24, Capacity: 8, LineCount: 1, Overflow: "single-line-reject"}
	if _, err := BuildMenuOverlay([]MenuOverlayEntry{entry}, font, palette, 2); err != nil {
		t.Fatalf("menu blank precheck: %v", err)
	}
	for _, scale := range []int{2, 3} {
		exit, _ := NewRuntimePostJoinExitPromptOverlay(font, scale)
		if err := exit.Apply(PostJoinExitPromptGeneration{1, postJoinExitQ1, "離開至 DOS", 96}, palette); err != nil {
			t.Fatalf("021 blank precheck %dx: %v", scale, err)
		}
		skill, _ := NewRuntimeSkillExitOverlay(font, scale)
		if err := skill.Apply(SkillExitGeneration{Generation: 1, EventKey: "k", Translation: "甲　乙 丙"}, palette); err != nil {
			t.Fatalf("020 blank precheck %dx: %v", scale, err)
		}
	}
	catalog := manualOverlayCatalog("繁　中 AB")
	if _, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, font, 2); err != nil {
		t.Fatalf("manual blank precheck: %v", err)
	}
	if !manualEnglishCovered([]manualEnglishRow{{text: []rune("中 　A")}}, []*xlate.Font{font, halfFontsOf(font).X2}) {
		t.Fatal("034 blank precheck")
	}
	delete(font.Glyphs, 'D')
	delete(halfFontCache.m, font)
	if _, err := BuildMenuOverlay([]MenuOverlayEntry{entry}, font, palette, 2); err != nil {
		t.Fatalf("unused missing half glyph rejected: %v", err)
	}
	exit, _ := NewRuntimePostJoinExitPromptOverlay(font, 2)
	if err := exit.Apply(PostJoinExitPromptGeneration{1, postJoinExitQ1, "離開至 DOS", 96}, palette); err == nil {
		t.Fatal("021 accepted a missing half glyph")
	}
}

func TestHalfPart2CapacityUnits(t *testing.T) {
	font := halfTestFont("cap", halfPart2Runes)
	var palette [256][3]uint8
	menu := func(text string) error {
		e := MenuOverlayEntry{EventKey: "e", TextKey: "t", Translation: text, X: 8, Y: 24, Width: 64, Height: 8,
			DrawX: 8, DrawY: 24, Capacity: 8, LineCount: 1, Overflow: "single-line-reject"}
		_, err := BuildMenuOverlay([]MenuOverlayEntry{e}, font, palette, 2)
		return err
	}
	// 8 cells = 16 units: seven full characters (9 runes) plus "AB" fit,
	// one more half character does not.
	if err := menu("地球地球地球地AB"); err != nil {
		t.Fatalf("16 units rejected: %v", err)
	}
	if err := menu("地球地球地球地ABC"); err == nil {
		t.Fatal("17 units accepted")
	}
	// Prefix cells count two units each.
	e := MenuOverlayEntry{EventKey: "e", TextKey: "t", Translation: "地球地球地球AB", X: 8, Y: 24, Width: 64, Height: 8,
		DrawX: 16, DrawY: 24, Capacity: 7, LineCount: 1, Overflow: "single-line-reject"}
	if _, err := BuildMenuOverlay([]MenuOverlayEntry{e}, font, palette, 2); err != nil {
		t.Fatalf("prefix + 14 units rejected: %v", err)
	}
	e.Translation += "C"
	if _, err := BuildMenuOverlay([]MenuOverlayEntry{e}, font, palette, 2); err == nil {
		t.Fatal("prefix + 15 units (17 in all) accepted")
	}
	exit, _ := NewRuntimePostJoinExitPromptOverlay(font, 2)
	if err := exit.Apply(PostJoinExitPromptGeneration{1, postJoinExitQ1, "甲乙丙丁甲乙丙丁甲乙丙AB", 96}, palette); err != nil {
		t.Fatalf("021 24 units rejected: %v", err)
	}
	if err := exit.Apply(PostJoinExitPromptGeneration{2, postJoinExitQ1, "甲乙丙丁甲乙丙丁甲乙丙ABC", 96}, palette); err == nil {
		t.Fatal("021 25 units accepted")
	}
	skill, _ := NewRuntimeSkillExitOverlay(font, 2)
	long := strings.Repeat("甲", 32) + "AB" // 66 units = 33 cells
	if err := skill.Apply(SkillExitGeneration{Generation: 1, EventKey: "k", Translation: long}, palette); err != nil {
		t.Fatalf("020 66 units rejected: %v", err)
	}
	if err := skill.Apply(SkillExitGeneration{Generation: 2, EventKey: "k", Translation: long + "C"}, palette); err == nil {
		t.Fatal("020 67 units accepted")
	}
	// Manual: 72 units per row, a full-width character that does not fit
	// moves whole, at most 14 rows.
	rows, err := manualRows(strings.Repeat("A", 71)+"字B", 36, 14)
	if err != nil || rows[0] != strings.Repeat("A", 71) || rows[1] != "字B" {
		t.Fatalf("manual whole-character wrap: %q %v", rows[:2], err)
	}
	if _, err := manualRows(strings.Repeat("字", 504), 36, 14); err != nil {
		t.Fatalf("504 full characters rejected: %v", err)
	}
	if _, err := manualRows(strings.Repeat("A", 1008), 36, 14); err != nil {
		t.Fatalf("1008 half characters rejected: %v", err)
	}
	for _, bad := range []string{strings.Repeat("字", 505), strings.Repeat("A", 1009), strings.Repeat("A", 71) + strings.Repeat("字", 36*13+1)} {
		if _, err := manualRows(bad, 36, 14); err == nil {
			t.Fatalf("manual accepted %d runes over 14 rows", len([]rune(bad)))
		}
	}
	// 034: the long form is measured in units.
	long34, short34, _ := manualEnglishTemplates([]byte(testManualPanel))
	// 45 runes (two rows by rune count) but 60 units: one row.
	words := []string{"Aaaaaaaa", "Bbbbbbbb", "CCCCCCCC"}
	out, ok := manualEnglishLayout(0, words, long34, short34)
	if !ok || len(out) != 1 || len(out[0].text) <= 36 || textUnits(out[0].text) > 72 || !strings.HasSuffix(string(out[0].text), "「CCCCCCCC」") {
		t.Fatalf("034 long form in units: %+v", out)
	}
}

// halfPart2Segments lists (text, unit offset, half) of a menu stamp list.
func halfPart2Segments(stamps []*xlate.Stamp, x int) string {
	var b strings.Builder
	for _, s := range stamps {
		fmt.Fprintf(&b, "[%s@%d/%d]", string(s.Text), (s.X-x)/halfUnitPx, s.CellW)
	}
	return b.String()
}

func TestHalfPart2MenuSegmentsAndGlyphPixels(t *testing.T) {
	font := halfTestFont("pix", halfPart2Runes)
	half := halfFontsOf(font)
	var palette [256][3]uint8
	palette[0], palette[14] = [3]uint8{0, 0, 0}, [3]uint8{255, 255, 0}
	entry := MenuOverlayEntry{EventKey: "function.menu.option.exit_to_dos", TextKey: "menu.exit_to_dos", Translation: "離開至 DOS",
		Background: 0, Foreground: 14, X: 72, Y: 104, Width: 96, Height: 8, DrawX: 72, DrawY: 104, Capacity: 12,
		LineCount: 1, Overflow: "single-line-reject"}
	for _, scale := range []int{2, 3} {
		built, err := BuildMenuOverlay([]MenuOverlayEntry{entry}, font, palette, scale)
		if err != nil {
			t.Fatal(err)
		}
		if got := halfPart2Segments(built.Layer.Stamps, 72); got != "[離開至@0/8][ DOS@6/4][　　　　　　　@10/8]" {
			t.Fatalf("%dx segments %s", scale, got)
		}
		seg := built.Layer.Stamps[1]
		if seg.Key != "function.menu.option.exit_to_dos#1" || seg.Font != half.For(scale) || seg.GlyphX != 0 || seg.GlyphY != 0 || seg.GlyphScale != 1 {
			t.Fatalf("%dx half segment %+v", scale, seg)
		}
		for _, s := range built.Layer.Stamps {
			if rowKeyOf(s.Key) != entry.EventKey {
				t.Fatalf("segment key %q", s.Key)
			}
		}
		rgba := make([]byte, 320*scale*200*scale*4)
		built.Layer.Draw(rgba, scale, nil)
		// 'D' is the second rune of the half segment: logical x = 72 + 7×4.
		g := half.For(scale)
		for gy := 0; gy < g.H; gy++ {
			for gx := 0; gx < g.W; gx++ {
				ink := g.Glyphs['D'][gy*((g.W+7)/8)+gx/8]&(0x80>>uint(gx%8)) != 0
				i := (((104*scale)+gy)*320*scale + (72+28)*scale + gx) * 4
				if got := rgba[i] == 255 && rgba[i+2] == 0; got != ink {
					t.Fatalf("%dx 'D' pixel (%d,%d) ink=%v got=%v", scale, gx, gy, ink, got)
				}
			}
		}
	}
}

func TestHalfPart2MenuReplaceLeavesNoSegment(t *testing.T) {
	o := halfPart2Menu(t, 2)
	e := halfPart2Event(t)
	pal := halfPart2Palette()
	frame := halfPart2Frame(0, 1, 2, 3, 4, 5, 6, 7)
	if err := o.Apply(e, DisplayRequest{EventKey: "race.option.terran", TextKey: "a", Translation: "地A球BC字"}, pal); err != nil {
		t.Fatal(err)
	}
	o.Frame(frame, pal)
	if n := len(o.layer.Stamps); n < 4 {
		t.Fatalf("long translation has %d segments", n)
	}
	if err := o.Apply(e, DisplayRequest{EventKey: "race.option.terran", TextKey: "b", Translation: "地"}, pal); err != nil {
		t.Fatal(err)
	}
	o.Frame(frame, pal)
	if keys := o.ActiveKeys(); len(keys) != 1 || keys[0] != "race.option.terran" || len(o.layer.Stamps) != 1 {
		t.Fatalf("stale segments survived: keys=%v stamps=%d", keys, len(o.layer.Stamps))
	}
	fresh := halfPart2Menu(t, 2)
	if err := fresh.Apply(e, DisplayRequest{EventKey: "race.option.terran", TextKey: "b", Translation: "地"}, pal); err != nil {
		t.Fatal(err)
	}
	fresh.Frame(frame, pal)
	a, _, _ := o.Draw(frame, pal)
	b, _, _ := fresh.Draw(frame, pal)
	if !bytes.Equal(a, b) {
		t.Fatal("replacement differs from a fresh single output")
	}
}

func TestHalfPart2MenuFrameColoursShared(t *testing.T) {
	o := halfPart2Menu(t, 2)
	pal := halfPart2Palette()
	frame := halfPart2Frame(0, 1, 2, 3, 4, 5, 6, 7)
	// The half segment "A" covers x 16..19; make its own region mostly
	// colour 3 so that per-segment colouring would differ.
	for y := 24; y < 32; y++ {
		for x := 16; x < 20; x++ {
			frame[y*320+x] = 3
		}
	}
	if err := o.Apply(halfPart2Event(t), DisplayRequest{EventKey: "race.option.terran", TextKey: "a", Translation: "地A球"}, pal); err != nil {
		t.Fatal(err)
	}
	o.Frame(frame, pal)
	var region []uint8
	for y := 24; y < 32; y++ {
		region = append(region, frame[y*320+8:y*320+72]...)
	}
	bg, fg := xlate.Colors(region)
	if len(o.layer.Stamps) != 5 {
		t.Fatalf("segments=%d", len(o.layer.Stamps))
	}
	for _, s := range o.layer.Stamps {
		if s.State != xlate.Shown || s.BG != pal[bg] || s.FG != pal[fg] {
			t.Fatalf("segment %s colours %v/%v want %v/%v", s.Key, s.BG, s.FG, pal[bg], pal[fg])
		}
	}
}

// halfPart2Shown applies "地A球" (segments 地｜A｜球｜␠｜　×5) and shows it on
// frame.
func halfPart2Shown(t *testing.T, frame []byte) *RuntimeMenuOverlay {
	t.Helper()
	o := halfPart2Menu(t, 2)
	pal := halfPart2Palette()
	if err := o.Apply(halfPart2Event(t), DisplayRequest{EventKey: "race.option.terran", TextKey: "a", Translation: "地A球"}, pal); err != nil {
		t.Fatal(err)
	}
	o.Frame(frame, pal)
	if len(o.ActiveKeys()) != 1 {
		t.Fatal("row not shown")
	}
	return o
}

func halfPart2Change(frame []byte, x0, x1 int) []byte {
	out := append([]byte(nil), frame...)
	for y := 24; y < 32; y++ {
		for x := x0; x < x1; x++ {
			out[y*320+x] = 3
		}
	}
	return out
}

func TestHalfPart2MenuRowGroupTriggers(t *testing.T) {
	pal := halfPart2Palette()
	all := halfPart2Frame(0, 1, 2, 3, 4, 5, 6, 7)
	lastReason := func(o *RuntimeMenuOverlay) string {
		if len(o.groups.cleared) == 0 {
			return ""
		}
		return o.groups.cleared[len(o.groups.cleared)-1].Reason
	}
	cases := []struct {
		name   string
		run    func(o *RuntimeMenuOverlay)
		reason string
	}{
		{"segment removed by fingerprint (half segment)", func(o *RuntimeMenuOverlay) {
			changed := halfPart2Change(all, 16, 20)
			for i := 0; i < 3; i++ {
				o.Frame(changed, pal)
			}
		}, "drop"},
		{"Clear makes one cell transparent", func(o *RuntimeMenuOverlay) {
			if err := o.ClearTextCells(3, 6, 3, 6); err != nil { // x 48..55, inside the U+3000 segment
				t.Fatal(err)
			}
		}, "transparent"},
		{"live ClearRect makes one cell transparent", func(o *RuntimeMenuOverlay) {
			o.clearRect(48, 24, 56, 32)
		}, "transparent"},
		{"later row over the right part (Clear drops covered segments)", func(o *RuntimeMenuOverlay) {
			if err := o.Apply(halfPart2Heading(t), DisplayRequest{EventKey: "race.heading.terran", TextKey: "h", Translation: "地球"}, pal); err != nil {
				t.Fatal(err)
			}
			if keys := o.ActiveKeys(); len(keys) != 1 || keys[0] != "race.heading.terran" {
				t.Fatalf("new row missing: %v", keys)
			}
		}, "drop"},
		{"Add partial overlap makes one cell transparent", func(o *RuntimeMenuOverlay) {
			o.layer.Add(&xlate.Stamp{Key: "other", X: 48, Y: 24, Cells: 1, CellW: 8, CellH: 8, State: xlate.Shown})
			o.Frame(all, pal)
		}, "transparent"},
		{"fingerprint on one cell, many anchors", func(o *RuntimeMenuOverlay) {
			changed := halfPart2Change(all, 48, 56)
			for i := 0; i < 3; i++ {
				o.Frame(changed, pal)
			}
		}, "transparent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := halfPart2Shown(t, all)
			c.run(o)
			for _, k := range o.ActiveKeys() {
				if k == "race.option.terran" {
					t.Fatalf("row survived: %v", o.ActiveKeys())
				}
			}
			for _, s := range o.layer.Stamps {
				if rowKeyOf(s.Key) == "race.option.terran" {
					t.Fatalf("segment %s survived", s.Key)
				}
			}
			if got := lastReason(o); got != c.reason {
				t.Fatalf("reason %q want %q (%+v)", got, c.reason, o.groups.cleared)
			}
		})
	}
	t.Run("group anchors at most two", func(t *testing.T) {
		// Only 8×8 cells 5 and 6 (x 48..63, both inside the U+3000
		// segment) are anchors; a fingerprint failure on cell 5 leaves one
		// valid anchor in the group.  xlate's per-segment 239 does not drop
		// the segment here (C has one cell, one valid anchor left).
		sparse := halfPart2Frame(5, 6)
		o := halfPart2Shown(t, sparse)
		changed := halfPart2Change(sparse, 48, 56)
		for i := 0; i < 3; i++ {
			o.Frame(changed, pal)
		}
		if len(o.ActiveKeys()) != 0 || lastReason(o) != "anchors" {
			t.Fatalf("keys=%v cleared=%+v", o.ActiveKeys(), o.groups.cleared)
		}
	})
	t.Run("control", func(t *testing.T) {
		o := halfPart2Shown(t, all)
		for i := 0; i < 5; i++ {
			o.Frame(all, pal)
		}
		if err := o.ClearTextCells(10, 30, 10, 20); err != nil {
			t.Fatal(err)
		}
		if len(o.ActiveKeys()) != 1 || len(o.layer.Stamps) != 5 || len(o.groups.cleared) != 0 {
			t.Fatalf("control changed: %v %+v", o.ActiveKeys(), o.groups.cleared)
		}
	})
}

// halfPart2ScopedFixture is scopedMenuFixture with a translation that has a
// half-width character (2 cells = 4 units: 地｜X␠).
func halfPart2ScopedFixture(t *testing.T) (*MenuCatalog, *MenuOverlayRects, map[string]TextEvent) {
	t.Helper()
	_, rects, events := scopedMenuFixture(t)
	var b strings.Builder
	b.WriteString(strings.Join(menuEventHeader, "\t") + "\n")
	for i, key := range scopedMenuFixtureKeys {
		e := events[key]
		fmt.Fprintf(&b, "%s\t%d\tmenu.synthetic\t2\t%x\t37F1:1856\t%d\t%d\t12\t9\n", key, i+1, e.OriginalSHA256, e.Background, e.Foreground)
	}
	catalog, err := LoadMenuCatalog([]byte(b.String()), []byte("key\ttranslation\tsource\nmenu.synthetic\t地X\tsynthetic\n"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog, rects, events
}

func TestHalfPart2ScopedMenuHalfSegmentsAndSeal(t *testing.T) {
	catalog, rects, events := halfPart2ScopedFixture(t)
	source := scopedSyntheticGOLEMFNT()
	if _, err := newScopedMenuRuntimeOverlay(rects, catalog, source, 2, sha256.Sum256(source)); err == nil {
		t.Fatal("scoped runtime accepted 2x")
	}
	o, err := newScopedMenuRuntimeOverlay(rects, catalog, source, 3, sha256.Sum256(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(o.scoped.registry) != 3 || o.scoped.half == nil || o.scoped.half.Name != menuHalfFontName ||
		o.scoped.half.W != 12 || o.scoped.half.H != 24 || o.scoped.registry[menuHalfFontName] != o.scoped.half {
		t.Fatalf("half font registry: %+v", o.scoped.registry)
	}
	key := "function.menu.selected.create_new_character"
	request, ok := catalog.Resolve(events[key])
	if !ok {
		t.Fatal("resolve")
	}
	if err := o.Apply(events[key], request, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	if got := halfPart2Segments(o.layer.Stamps, 72); got != "[地@0/8][X @2/4]" {
		t.Fatalf("segments %s", got)
	}
	full, half := o.layer.Stamps[0], o.layer.Stamps[1]
	if full.Font != o.scoped.derived || full.GlyphX != 1 || half.Font != o.scoped.half || half.GlyphX != 0 || half.GlyphY != 0 {
		t.Fatal("per-segment font switch")
	}
	if keys := o.ActiveKeys(); len(keys) != 1 || keys[0] != key {
		t.Fatalf("active keys %v", keys)
	}
	for _, s := range o.layer.Stamps {
		s.State = xlate.Shown
	}
	snapshot, err := o.SnapshotScopedLayer()
	if err != nil {
		t.Fatalf("seal rejected half segment: %v", err)
	}
	indexed := make([]byte, 320*200)
	before, _, _ := o.Draw(indexed, [256][3]uint8{})
	if err := o.RestoreScopedLayer(snapshot); err != nil {
		t.Fatal(err)
	}
	after, _, _ := o.Draw(indexed, [256][3]uint8{})
	if !bytes.Equal(before, after) || o.layer.Stamps[1].Font != o.scoped.half {
		t.Fatal("restore changed half font identity or RGBA")
	}
	// A half segment whose font is not the registered half font is refused.
	foreign := DeriveHalfFonts(o.font).X3 // same name and bytes, other pointer
	for _, bad := range []*xlate.Font{o.font, o.scoped.derived, foreign} {
		o.layer.Stamps[1].Font = bad
		if _, err := o.SnapshotScopedLayer(); err == nil {
			t.Fatalf("seal accepted half segment font %q", bad.Name)
		}
	}
	o.layer.Stamps[1].Font = o.scoped.half
	o.scoped.registry[menuHalfFontName] = foreign
	if _, err := o.SnapshotScopedLayer(); err == nil {
		t.Fatal("seal accepted a same-name half font pointer")
	}
	o.scoped.registry[menuHalfFontName] = o.scoped.half
	o.scoped.half.Glyphs['X'][0] ^= 0x80
	if _, err := o.SnapshotScopedLayer(); err == nil {
		t.Fatal("seal accepted half font fingerprint drift")
	}
	o.scoped.half.Glyphs['X'][0] ^= 0x80
	if _, err := o.SnapshotScopedLayer(); err != nil {
		t.Fatalf("repaired half font rejected: %v", err)
	}
}

func TestHalfPart2ManualSegmentsAndRowGroups(t *testing.T) {
	text := "RAM繁中" + strings.Repeat("字", 40) + " AB"
	catalog := manualOverlayCatalog(text)
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, manualOverlayFont(catalog), scale)
		if err != nil {
			t.Fatal(err)
		}
		if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1}); err != nil {
			t.Fatal(err)
		}
		if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: manualOverlayRequest(catalog, 1)}); err != nil {
			t.Fatal(err)
		}
		keys := o.ActiveKeys()
		if len(keys) != 14 {
			t.Fatalf("%dx rows=%d", scale, len(keys))
		}
		units := map[string]int{}
		for _, s := range o.text.Stamps {
			u := s.Cells
			if s.CellW == 8 {
				u *= 2
			} else if s.Font != o.half || s.GlyphX != 0 || s.GlyphY != 0 {
				t.Fatalf("%dx half segment font/offset", scale)
			}
			units[rowKeyOf(s.Key)] += u
		}
		for _, k := range keys {
			if units[k] != 72 {
				t.Fatalf("%dx row %s units=%d", scale, k, units[k])
			}
		}
		// Row 0 = "RAM" + 34 full + 1 more? 3 + 68 = 71 units, the next full
		// character moves whole; row 13 is 36 U+3000.
		row0 := ""
		for _, s := range segmentsOf(o.text, keys[0]) {
			row0 += string(s.Text)
		}
		if !strings.HasPrefix(row0, "RAM繁中") || textUnits([]rune(strings.TrimRight(row0, "　 "))) != 71 {
			t.Fatalf("%dx row 0 %q", scale, row0)
		}
		last := segmentsOf(o.text, keys[13])
		if len(last) != 1 || string(last[0].Text) != strings.Repeat("　", 36) {
			t.Fatalf("%dx empty row stamp %+v", scale, last)
		}
		// No style: a fingerprint failure on one cell clears the whole row.
		palette, indexed := manualOverlayPaletteAndFrame()
		o.Frame(indexed, palette)
		changed := append([]byte(nil), indexed...)
		for y := 72; y < 80; y++ {
			for x := 40; x < 48; x++ {
				changed[y*320+x] ^= 0x55
			}
		}
		for i := 0; i < 3; i++ {
			o.Frame(changed, palette)
		}
		after := o.ActiveKeys()
		if len(after) != 13 || after[0] == keys[0] || len(segmentsOf(o.text, keys[0])) != 0 {
			t.Fatalf("%dx row group: %v", scale, after)
		}
	}
}

func TestHalfPart2ManualOwnerSealAcceptsHalfSegments(t *testing.T) {
	text := "RAM繁中段落 AB"
	catalog := manualOverlayCatalog(text)
	font := manualOverlayFont(catalog)
	owner, err := NewManualSnapshotOwner(loadManualOverlayLayout(t), catalog, font, 2)
	if err != nil {
		t.Fatal(err)
	}
	if owner.half == nil || owner.half.Name != manualBaseFontIdentity+".half8x16" || owner.overlay.half != owner.half {
		t.Fatalf("owner half font %+v", owner.half)
	}
	if n, err := owner.Consume([]ManualPresentationEvent{{Kind: ManualPresentationBegin, Generation: 3},
		{Kind: ManualPresentationRequest, Generation: 3, Request: manualOverlayRequest(catalog, 3)}}); err != nil || n != 2 {
		t.Fatalf("consume=%d err=%v", n, err)
	}
	palette, indexed := manualOverlayPaletteAndFrame()
	frame := host.IndexedFrame{Canvas: host.Canvas{Width: 320, Height: 200}, Indexed: indexed, Palette: palette}
	ticket, err := owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	shot, err := owner.Snapshot(ticket, 2)
	if err != nil || !shot.Drew {
		t.Fatalf("sealed 2x with half segments: %v", err)
	}
	var halfSeg *xlate.Stamp
	for _, s := range owner.overlay.text.Stamps {
		if s.CellW == halfUnitPx {
			halfSeg = s
			break
		}
	}
	if halfSeg == nil || halfSeg.Font != owner.half {
		t.Fatal("no half segment")
	}
	for _, mutate := range []func() func(){
		func() func() { // font outside the registry
			old := halfSeg.Font
			halfSeg.Font = DeriveHalfFonts(owner.base).X2
			return func() { halfSeg.Font = old }
		},
		func() func() { // units no longer sum to 72
			halfSeg.Cells--
			return func() { halfSeg.Cells++ }
		},
		func() func() { // cell width outside {4, 8}
			halfSeg.CellW = 6
			return func() { halfSeg.CellW = halfUnitPx }
		},
		func() func() { // half font fingerprint drift
			owner.half.Glyphs['A'][0] ^= 0x80
			return func() { owner.half.Glyphs['A'][0] ^= 0x80 }
		},
	} {
		undo := mutate()
		if _, err := owner.PrepareFrame(frame); err == nil {
			t.Fatal("owner accepted a changed half segment")
		}
		undo()
		ticket, err = owner.PrepareFrame(frame)
		if err != nil {
			t.Fatalf("repaired layer rejected: %v", err)
		}
	}
	if _, err := owner.Snapshot(ticket, 2); err != nil {
		t.Fatal(err)
	}
}
