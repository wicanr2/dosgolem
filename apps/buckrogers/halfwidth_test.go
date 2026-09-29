package buckrogers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 039 §5.1–§5.2 unit tests for the half-width model shared by the
// part-1 families (ECL, horizontal menu, engine dispatcher, logbook, story
// pages, action bar, body icon).  Every font here is synthetic.

// halfTestFont is a synthetic 16×16 base: half-width glyphs keep their ink
// in columns 4–11 with a distinct pattern per row; full-width glyphs fill
// the cell.  U+3000 has no glyph (like both working fonts).
func halfTestFont(name string, runes string) *xlate.Font {
	f := &xlate.Font{W: 16, H: 16, Name: name, Glyphs: map[rune][]byte{}}
	for _, r := range runes {
		g := make([]byte, 32)
		for y := 0; y < 16; y++ {
			if isHalfwidth(r) {
				row := uint16(byte(int(r)*7+y*13)|0x81) << 4 // columns 4 and 11 always inked
				g[2*y], g[2*y+1] = byte(row>>8), byte(row)
			} else {
				g[2*y], g[2*y+1] = 0xff, 0xff
			}
		}
		f.Glyphs[r] = g
	}
	return f
}

func TestHalfUnitsAndSegments(t *testing.T) {
	type seg struct {
		text string
		unit int
		half bool
	}
	cases := []struct {
		in    string
		units int
		segs  []seg
	}{
		{"RAM", 3, []seg{{"RAM", 0, true}}},                                 // pure English, odd
		{"中文", 4, []seg{{"中文", 0, false}}},                                  // pure Chinese
		{"中A文", 5, []seg{{"中", 0, false}, {"A", 2, true}, {"文", 3, false}}}, // mixed, odd
		{"NEO 中", 6, []seg{{"NEO ", 0, true}, {"中", 4, false}}},             // content space is half width
		{"卡頓•特必安(CARLTON TURABIAN)", 29, []seg{{"卡頓", 0, false}, {"•", 4, true}, {"特必安", 5, false}, {"(CARLTON TURABIAN)", 11, true}}},
		{"", 0, nil},
	}
	for _, c := range cases {
		r := []rune(c.in)
		if u := textUnits(r); u != c.units || stringUnits(c.in) != c.units {
			t.Errorf("%q: units %d want %d", c.in, u, c.units)
		}
		got := splitSegments(r, nil)
		if len(got) != len(c.segs) {
			t.Fatalf("%q: %d segments %+v", c.in, len(got), got)
		}
		for i, s := range got {
			if string(r[s.Start:s.End]) != c.segs[i].text || s.Unit != c.segs[i].unit || s.Half != c.segs[i].half {
				t.Errorf("%q seg %d: %+v want %+v", c.in, i, s, c.segs[i])
			}
		}
	}
	// A foreground change splits a segment of one width class.
	fg := []int{10, 15, 10, 10}
	got := splitSegments([]rune("(M)移"), func(i int) int { return fg[i] })
	if len(got) != 4 || got[1].Unit != 1 || got[3].Half {
		t.Fatalf("colour split %+v", got)
	}
	if segmentKey("ecl.0.row.17", 2) != "ecl.0.row.17#2" || rowKeyOf("ecl.0.row.17#2") != "ecl.0.row.17" || rowKeyOf("plain") != "plain" {
		t.Fatal("segment key")
	}
	if cellsForUnits(19) != 10 || cellsForUnits(20) != 10 || cellsForUnits(0) != 0 {
		t.Fatal("cells for units")
	}
	if fitUnits([]rune("中文A字"), 5) != 3 || fitUnits([]rune("中文A字"), 6) != 3 || fitUnits([]rune("中文A字"), 7) != 4 {
		t.Fatal("fitUnits moves a full-width character whole")
	}
}

func TestHalfPaddingOrder(t *testing.T) {
	cases := []struct {
		in     string
		target int
		want   string
	}{
		{"中A", 8, "中A 　　"}, // odd content: one U+0020 first, then U+3000
		{"中", 7, "中　　 "},   // odd end: U+3000, then one U+0020
		{"AB", 2, "AB"},    // already full
		{"A", 1, "A"},      // odd target, no padding
		{"中文字", 4, "中文字"},  // wider than the target: unchanged
		{"", 4, "　　"},      // empty
		{"A", 3, "A  "},    // from 1 to 3: U+0020 to 2, then U+0020 for the odd end
	}
	for _, c := range cases {
		if got := string(padUnits([]rune(c.in), c.target)); got != c.want {
			t.Errorf("pad(%q,%d) = %q want %q", c.in, c.target, got, c.want)
		}
	}
	// A gap that starts and ends on odd units (between two lines).
	if got := string(appendPadding(nil, 3, 7)); got != " 　 " {
		t.Errorf("odd gap %q", got)
	}
}

func TestHalfFontDerivation(t *testing.T) {
	base := halfTestFont("base", "A•中 ")
	h := DeriveHalfFonts(base)
	if h.Err != nil || h.X2.Name != "base.half8x16" || h.X3.Name != "base.half12x24" ||
		h.X2.W != 8 || h.X2.H != 16 || h.X3.W != 12 || h.X3.H != 24 {
		t.Fatalf("derive %+v", h)
	}
	if _, ok := h.X2.Glyphs['中']; ok {
		t.Fatal("full-width glyph in the half font")
	}
	for _, r := range "A• " {
		src, x2, x3 := base.Glyphs[r], h.X2.Glyphs[r], h.X3.Glyphs[r]
		for y := 0; y < 16; y++ {
			for x := 0; x < 8; x++ {
				want := src[2*y+(x+4)/8]&(0x80>>uint((x+4)%8)) != 0
				if got := x2[y]&(0x80>>uint(x)) != 0; got != want {
					t.Fatalf("%q 2× (%d,%d)", r, x, y)
				}
			}
		}
		// 3×: nearest neighbour ×1.5, x from ⌊2x/3⌋ and y from ⌊2y/3⌋.
		for y := 0; y < 24; y++ {
			for x := 0; x < 12; x++ {
				want := x2[2*y/3]&(0x80>>uint(2*x/3)) != 0
				if got := x3[2*y+x/8]&(0x80>>uint(x%8)) != 0; got != want {
					t.Fatalf("%q 3× (%d,%d)", r, x, y)
				}
			}
		}
	}
	if n := DeriveHalfFonts(halfTestFont("", "A")); n.Err != nil || n.X2.Name != "buckrogers.half8x16" || n.X3.Name != "buckrogers.half12x24" {
		t.Fatalf("empty base name %+v", n)
	}
	// Ink outside columns 4–11 fails the derivation (column 3 and column 12).
	for _, bad := range [][2]byte{{0x10, 0x00}, {0x00, 0x08}} {
		f := halfTestFont("bad", "AB")
		f.Glyphs['B'][10], f.Glyphs['B'][11] = bad[0], bad[1]
		if h := DeriveHalfFonts(f); h.Err == nil || h.X2 != nil || !strings.Contains(h.Err.Error(), "U+0042") {
			t.Fatalf("ink %x accepted: %+v", bad, h)
		}
	}
	// One derivation per base font: every presenter shares the pointers.
	if a, b := halfFontsOf(base), halfFontsOf(base); a != b || a.X2 == nil {
		t.Fatal("half fonts not shared")
	}
}

// §3.1: U+3000 and U+0020 need no glyph in any family pre-check.
func TestHalfBlankNeedsNoGlyph(t *testing.T) {
	base := halfTestFont("blank", "中A")
	f := segmentFonts{Full: base, Half: halfFontsOf(base).For(2)}
	if miss := f.missingRunes([]rune("中 　A")); len(miss) != 0 {
		t.Fatalf("missing %q", string(miss))
	}
	if miss := f.missingRunes([]rune("B字")); string(miss) != "B字" {
		t.Fatalf("missing %q", string(miss))
	}
	if miss := (segmentFonts{Full: base}).missingRunes([]rune("A ")); string(miss) != "A" {
		t.Fatalf("no half font: %q", string(miss))
	}
	for _, scale := range liveScales {
		// ECL
		ecl, _ := NewEclTextOverlay(base, scale)
		page := &EclTextPage{Left: 1, Top: 17, Right: 10, Bottom: 17, TopCol: 1, Lines: []EclTextLine{{Row: 17, Col: 2, Text: []rune("中　A ")}}}
		if miss := ecl.Sync([]*EclTextPage{page}, 1, [256][3]uint8{}); len(miss) != 0 {
			t.Fatalf("ECL %d×: %q", scale, string(miss))
		}
		// Horizontal menu and dispatcher (same presenter).
		hm, _ := NewHMenuOverlay(base, scale)
		row := newHMenuRow(24, 0, []HMenuCell{{'中', 0, 10}, {'　', 0, 10}, {' ', 0, 10}})
		if miss := hm.Sync(&HMenuPage{Rows: []HMenuRow{row}}, 1, [256][3]uint8{}); len(miss) != 0 {
			t.Fatalf("hmenu %d×: %q", scale, string(miss))
		}
		// Story pages.
		if err := storyTextCheck(map[string]string{"k": "中　A "}, segmentFonts{Full: base, Half: halfFontsOf(base).For(scale)}, 78); err != nil {
			t.Fatalf("story %d×: %v", scale, err)
		}
	}
	// Logbook.
	c, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.5\t中　A。\tx\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	w := NewLogbookWatcher(c, nil)
	w.ObserveEntry([]byte(" as logbook entry 5."), 1, 17, 38, 22)
	lb, _ := NewLogbookOverlay(halfTestFont("lb", "中A。:5"), 2)
	if miss := lb.Sync(w, [256][3]uint8{}); len(miss) != 0 {
		t.Fatalf("logbook: %q", string(miss))
	}
}

// §5.2: 2× and 3× half glyph pixels through a real presenter (ECL row
// "A中"): the half cell is 4 logical pixels, the glyph starts at its left
// edge with no offset, and the full-width cell follows at unit 1.
func TestHalfGlyphPixelsThroughPresenter(t *testing.T) {
	base := halfTestFont("px", "A中")
	var pal [256][3]uint8
	pal[10] = [3]uint8{255, 255, 255}
	for _, scale := range liveScales {
		o, _ := NewEclTextOverlay(base, scale)
		page := &EclTextPage{Left: 0, Top: 0, Right: 3, Bottom: 0, TopCol: 0, Foreground: 10, Lines: []EclTextLine{{Row: 0, Col: 0, Text: []rune("A中")}}}
		if miss := o.Sync([]*EclTextPage{page}, 1, pal); len(miss) != 0 {
			t.Fatal(string(miss))
		}
		// "A中" = 3 units; padding ' ' to 4 and U+3000 to 8 (cells 0–3).
		st := o.layer.Stamps
		if len(st) != 4 || st[0].CellW != 4 || st[1].X != 4 || st[1].CellW != 8 || st[2].Text[0] != ' ' || st[2].X != 12 || st[3].X != 16 {
			t.Fatalf("segments %+v %+v %+v %+v", st[0], st[1], st[2], st[3])
		}
		out, miss := o.Draw(make([]byte, 320*200), pal)
		if len(miss) != 0 {
			t.Fatal(string(miss))
		}
		half := halfFontsOf(base).For(scale)
		W := 320 * scale
		for y := 0; y < 8*scale; y++ {
			for x := 0; x < 4*scale; x++ {
				ink := half.Glyphs['A'][y*((half.W+7)/8)+x/8]&(0x80>>uint(x%8)) != 0
				if got := out[4*(y*W+x)] == 255; got != ink {
					t.Fatalf("%d× A pixel (%d,%d) got %v want %v", scale, x, y, got, ink)
				}
			}
		}
		// The full-width glyph starts in the next half cell (x = 4 logical).
		if out[4*((8*scale/2)*W+4*scale+8*scale/2)] != 255 {
			t.Fatalf("%d× 中 not at unit 1", scale)
		}
	}
}

func TestHalfCapacityBoundaries(t *testing.T) {
	// ECL: a one-cell window holds two half units.
	win := func(text string) bool {
		_, _, _, ok := layoutEclText([]rune(text), 17, eclUnitLeft(1), eclUnitLeft(1), eclUnitRight(1), 17)
		return ok
	}
	if !win("AB") || win("ABC") || !win("中") || win("中A") {
		t.Fatal("ECL one-cell window")
	}
	// Horizontal menu: 2×(40−start column) units; the item separator is one.
	w := NewHMenuWatcher(hmenuFixture(t, "Yes", "(Y)", "No", "否(N)"))
	e := hmenuEntry("Yes", 1, [2]uint8{1, 3})
	e.Col = 38 // 4 units: "(Y)" is 3
	w.ObserveEntry(e)
	if p := w.Page(); p == nil || p.Rows[0].Units() != 4 || p.Rows[0].Cells[3].Rune != ' ' {
		t.Fatalf("hmenu fits: %+v", w.Page())
	}
	w.ObserveInstruction(e.Return, e.SS, e.SP+hmenuReturnDelta)
	e = hmenuEntry("Yes No", 1, [2]uint8{1, 3}, [2]uint8{5, 6})
	e.Col = 32 // 16 units: "(Y)" + " " + "否(N)" = 3+1+5 = 9
	w.ObserveEntry(e)
	if p := w.Page(); p == nil || p.Rows[0].Pos[4] != 4 {
		t.Fatalf("hmenu separator: %+v", w.Page())
	}
	w.ObserveInstruction(e.Return, e.SS, e.SP+hmenuReturnDelta)
	e.Col = 36 // 8 units < 9
	w.ObserveEntry(e)
	if w.Page() != nil || w.Stats.Overflows != 1 {
		t.Fatal("hmenu overflow drew")
	}
	// Logbook: body 72 units a line, title row 76 units.
	if p, err := LayoutLogbook(strings.Repeat("中", 36)); err != nil || len(p[0]) != 1 {
		t.Fatalf("72 units: %v %v", p, err)
	}
	if p, err := LayoutLogbook(strings.Repeat("中", 35) + "AB"); err != nil || len(p[0]) != 1 {
		t.Fatalf("70+2 units: %v %v", p, err)
	}
	if p, err := LayoutLogbook(strings.Repeat("中", 36) + "A"); err != nil || len(p[0]) != 2 {
		t.Fatalf("73 units: %v %v", p, err)
	}
	for title, ok := range map[string]bool{strings.Repeat("中", 36) + "A": true, strings.Repeat("中", 37): false} {
		_, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.9\t正文。\tx\nlogbook.9.title\t"+title+"\tx\n"), nil)
		if (err == nil) != ok { // "9: " is 3 units: 3+73 = 76, 3+74 = 77
			t.Errorf("title %d units: err=%v", stringUnits(title), err)
		}
	}
	// Story pages: 78 units (page 8: 76, page 9: 40).
	fonts := segmentFonts{Full: halfTestFont("st", "中A"), Half: halfFontsOf(halfTestFont("st2", "A")).X2}
	for limit, s := range map[int]string{78: strings.Repeat("中", 39), 76: strings.Repeat("中", 37) + "AA", 40: strings.Repeat("中", 19) + "AA"} {
		if err := storyTextCheck(map[string]string{"k": s}, fonts, limit); err != nil {
			t.Errorf("%d: %v", limit, err)
		}
		if err := storyTextCheck(map[string]string{"k": s + "A"}, fonts, limit); err == nil {
			t.Errorf("%d+1 accepted", limit)
		}
	}
	p9 := halfTestFont("p9", "中A")
	if _, err := NewRuntimeStoryPage9Overlay(map[string]string{"story.page9.line.001": strings.Repeat("中", 19) + "AA"}, p9, 2); err != nil {
		t.Fatalf("page 9 40 units: %v", err)
	}
	if _, err := NewRuntimeStoryPage9Overlay(map[string]string{"story.page9.line.001": strings.Repeat("中", 20) + "A"}, p9, 2); err == nil {
		t.Fatal("page 9 41 units accepted")
	}
}

// §3.4 dispatcher general path: Chinese ≤ original length × 2 half units.
func TestHalfDispatcherGeneralCapacity(t *testing.T) {
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: engineTSV("frag", "Abc", "Abd"),
		FragmentText:   engineZh("frag", "Abc", "中文字", "Abd", "中文字A"),
		ItemEvents:     engineTSV("item", "Zqx"),
		ItemText:       engineZh("item", "Zqx", "物"),
	})
	if err != nil {
		t.Fatal(err)
	}
	caller := CodeKey{Segment: 0x0763, Offset: 0x1282}
	ret := Address{0x0763, 0x1282}
	w := NewEngineDispatchWatcher(c, map[CodeKey]bool{caller: true})
	w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 3, 1}, []byte("Abc")) // 6 ≤ 6
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 4, 1}, []byte("Abd")) // 7 > 6
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	if l, ok := lineAt(w, 3); !ok || l.Width != 3 || string(l.Text) != "中文字" {
		t.Fatalf("fits: %+v", l)
	}
	if _, ok := lineAt(w, 4); ok || w.Stats.Misses != 1 {
		t.Fatal("7 units in 3 cells drawn")
	}
}

// §3.4 table rows: the last column is right-aligned to the original right
// edge with at least one unit before it; otherwise English.
func TestHalfTableLastColumnRightEdge(t *testing.T) {
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: engineTSV("frag", "Abc"),
		FragmentText:   engineZh("frag", "Abc", "一二三四五六七八九十"), // 20 units
		ItemEvents:     engineTSV("item", "Zqx"),
		ItemText:       engineZh("item", "Zqx", "物"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range []int{6, 7, 20} {
		in := "Abc" + strings.Repeat(" ", sp) + "123"
		zh, ok := c.Translate(in)
		if !ok || stringUnits(zh) != 2*len(in) || !strings.HasSuffix(zh, "123") || !strings.HasPrefix(zh, "一二三四五六七八九十") {
			t.Fatalf("%d spaces: %q %v", sp, zh, ok)
		}
		// The digits end exactly on the original right edge.
		r := []rune(zh)
		if textUnits(r[:len(r)-3]) != 2*len(in)-3 {
			t.Fatalf("%d spaces: digits start at %d", sp, textUnits(r[:len(r)-3]))
		}
	}
	// 20 + 1 + 3 = 24 = 2×12 fits with exactly one unit; 2×11 does not.
	if zh, _ := c.Translate("Abc" + strings.Repeat(" ", 6) + "123"); string([]rune(zh)[10:]) != " 123" {
		t.Fatalf("one-unit gap: %q", zh)
	}
	if _, ok := c.Translate("Abc" + strings.Repeat(" ", 5) + "123"); ok {
		t.Fatal("no gap accepted (the original English must show)")
	}
}

// §3.5: dispatcher yield covers the overlay's own cells (Width), not the
// number of runes.
func TestHalfDispatcherYieldRange(t *testing.T) {
	r := &LiveRuntime{indexed: make([]byte, 320*200)}
	out := ScaleIndexedRGBA(r.indexed, r.palette, 2)
	var cells []HMenuCell
	for _, ch := range padUnits([]rune("AB"), 20) { // Width 10 cells, 11 runes
		cells = append(cells, HMenuCell{Rune: ch})
	}
	row := newHMenuRow(5, 10, cells)
	if len(row.Cells) != 11 || row.WidthCells() != 10 {
		t.Fatalf("row %+v", row)
	}
	paint := func(col int) {
		o := 4 * ((5*8*2+3)*640 + col*8*2 + 3)
		out[o] = 9
	}
	paint(20) // first cell after the line: not ours
	if keep := r.untouchedRows(out, 2, &HMenuPage{Rows: []HMenuRow{row}}); len(keep) != 1 {
		t.Fatal("pixel after the line counted")
	}
	paint(19) // last cell of the line
	if keep := r.untouchedRows(out, 2, &HMenuPage{Rows: []HMenuRow{row}}); len(keep) != 0 {
		t.Fatal("pixel in the last cell ignored")
	}
}

// §3.3: horizontal menu keeps one stamp per rune with its own colour and
// key; half cells are 4 pixels at the unit position.
func TestHalfHMenuStampsPerRune(t *testing.T) {
	w := NewHMenuWatcher(hmenuFixture(t, "Move", "移動(M)"))
	w.ObserveEntry(hmenuEntry("Move", 0, [2]uint8{1, 4}))
	base := halfTestFont("hm", "移動(M)")
	o, _ := NewHMenuOverlay(base, 2)
	var pal [256][3]uint8
	pal[10], pal[15] = [3]uint8{10, 10, 10}, [3]uint8{15, 15, 15}
	if miss := o.Sync(w.Page(), 1, pal); len(miss) != 0 {
		t.Fatal(string(miss))
	}
	want := []struct {
		x, cw int
		fg    uint8
	}{{0, 8, 10}, {8, 8, 10}, {16, 4, 10}, {20, 4, 15}, {24, 4, 10}, {28, 4, 10}}
	for i, c := range want {
		s := o.layer.Stamps[i]
		if s.Key != fmt.Sprintf("hmenu.24.%d", i) || s.X != c.x || s.CellW != c.cw || s.FG != pal[c.fg] {
			t.Fatalf("stamp %d: %+v", i, s)
		}
	}
	// 移動 (4 units) + (M) (3) = 7 units; ' ' to 8, then 36 U+3000 to 80.
	if n := len(o.layer.Stamps); n != 5+1+36 || o.layer.Stamps[5].Text[0] != ' ' || o.layer.Stamps[6].X != 32 {
		t.Fatalf("stamps %d", n)
	}
}

// §3.3 row groups on the action bar (spec 215 per-rune stamps): a dropped
// segment, a transparent cell or an anchored fingerprint change clears the
// whole label.
func TestHalfActionBarRowGroupClears(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	var pal [256][3]uint8
	pal[15] = [3]uint8{255, 255, 255}
	setup := func() (*RuntimeActionBarOverlay, []byte) {
		o, err := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &style)
		if err != nil {
			t.Fatal(err)
		}
		o.ObserveAnchorEvent("career.screen.remaining_points.heading")
		e, r := actionEventFor(c, "career", "action.add", "normal")
		if err := o.Apply(e, r, pal); err != nil {
			t.Fatal(err)
		}
		indexed := make([]byte, 320*200)
		for x := 0; x < 32; x += 3 { // original label ink in every cell
			indexed[195*320+x] = 15
		}
		o.Frame(indexed, pal)
		if len(o.ActiveKeys()) != 1 || len(o.layer.Stamps) != 5 {
			t.Fatalf("setup keys %v stamps %d", o.ActiveKeys(), len(o.layer.Stamps))
		}
		return o, indexed
	}
	// 1+2: a clear of one half cell ('A' at x 20–24) makes that segment
	// transparent and drops it; the group clears whole.
	o, indexed := setup()
	o.layer.Clear(21, 192, 23, 200)
	o.Frame(indexed, pal)
	if len(o.ActiveKeys()) != 0 || len(o.layer.Stamps) != 0 {
		t.Fatalf("partial clear: keys %v stamps %d", o.ActiveKeys(), len(o.layer.Stamps))
	}
	// 3: the original rewrites an anchored cell of one segment.
	o, indexed = setup()
	indexed[195*320+22] = 7
	for i := 0; i < 3; i++ { // xlate: three consecutive misses
		o.Frame(indexed, pal)
	}
	if len(o.ActiveKeys()) != 0 || len(o.layer.Stamps) != 0 {
		t.Fatalf("fingerprint: keys %v stamps %d", o.ActiveKeys(), len(o.layer.Stamps))
	}
	// Control: an unchanged frame keeps the group.
	o, indexed = setup()
	o.Frame(indexed, pal)
	if len(o.ActiveKeys()) != 1 {
		t.Fatal("unchanged frame cleared the group")
	}
}

// §3.4 body icon (009): capacity cells × 2 half units; a half-width label is
// drawn as width segments.
func TestHalfBodyIconCapacityUnits(t *testing.T) {
	var key string
	for k, id := range bodyTestCatalog().byKey {
		if id.Capacity == 3 && id.Rect.Width == 24 {
			key = k
			break
		}
	}
	for text, ok := range map[string]bool{"界界界": true, "界界AB": true, "界界界A": false, "界A(B)": true, "界A(BC)": false} {
		c := bodyTestCatalog()
		id := c.byKey[key]
		id.Translation = text
		c.byKey[key] = id
		var all string
		for _, x := range c.byKey {
			all += x.Translation
		}
		font := halfTestFont("body", all+c.save.PrefixText+c.save.SuffixText)
		_, err := NewRuntimeBodyIconOverlay(c, font, 2, BodyIconMove)
		if (err == nil) != ok {
			t.Errorf("%q (%d units in 3 cells): err=%v", text, stringUnits(text), err)
		}
	}
}
