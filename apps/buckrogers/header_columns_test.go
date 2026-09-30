package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 039 §5.2 欄名列 unit tests.  Text, hashes and fonts are synthetic;
// the geometry mirrors the career-skill heading (row 4, column 21, 18
// cells, anchors 2,8,14 → data columns 23, 29, 35).

const headerTestOriginal = "hdr-fixture-18-cel" // 18 bytes, synthetic

func headerTestCatalog(t *testing.T, translation string) (*MenuCatalog, TextEvent) {
	t.Helper()
	h := sha256.Sum256([]byte(headerTestOriginal))
	events := "event_key\tsequence\ttext_key\toriginal_length\toriginal_sha256\tcaller\tbackground\tforeground\trow\tcolumn\n" +
		fmt.Sprintf("career.screen.columns.heading\t1\tcareer.screen.columns\t18\t%x\t2684:085C\t0\t10\t4\t21\n", h)
	texts := "key\ttranslation\tsource\ncareer.screen.columns\t" + translation + "\truntime-interface\n"
	c, err := LoadCareerSkillCatalog([]byte(events), []byte(texts))
	if err != nil {
		t.Fatal(err)
	}
	e := TextEvent{EntryStep: 10, PostCallStep: 20, Caller: Address{0x2684, 0x085C}, OriginalLength: 18,
		OriginalSHA256: h, Background: 0, Foreground: 10, Row: 4, Column: 21}
	return c, e
}

const headerTestRects = "event_key\tx\ty\twidth\theight\tdraw_x\tdraw_y\tcapacity_cells\tline_count\toverflow_policy\n" +
	"career.screen.columns.heading\t168\t32\t144\t8\t168\t32\t18\t1\tsingle-line-reject\n"

func headerTestList(t *testing.T, rows ...string) *HeaderColumns {
	t.Helper()
	h, err := LoadHeaderColumns("header-columns.tsv", []byte("family\tkey\tcols\tevidence\n"+strings.Join(rows, "\n")+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHeaderColumnsMenuAnchor(t *testing.T) {
	catalog, event := headerTestCatalog(t, "點數 加值 總計")
	list := headerTestList(t, "menu\tcareer.screen.columns.heading\t2,8,14\tscreen")
	if err := list.ValidateMenu(catalog); err != nil {
		t.Fatal(err)
	}
	rects, err := LoadMenuOverlayRects("rects.tsv", []byte(headerTestRects))
	if err != nil {
		t.Fatal(err)
	}
	font := halfTestFont("hdr", "點數加值總計")
	var palette [256][3]uint8
	palette[0], palette[10] = [3]uint8{0, 0, 0}, [3]uint8{0, 255, 0}
	request, ok := catalog.Resolve(event)
	if !ok {
		t.Fatal("resolve")
	}
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeMenuOverlay(rects, font, scale)
		if err != nil {
			t.Fatal(err)
		}
		o.SetHeaderColumns(list)
		if err := o.Apply(event, request, palette); err != nil {
			t.Fatal(err)
		}
		if len(o.layer.Stamps) != 1 {
			t.Fatalf("%dx: %d segments, want 1", scale, len(o.layer.Stamps))
		}
		s := o.layer.Stamps[0]
		if s.Key != "career.screen.columns.heading#0" || s.CellW != 8 || textUnits(s.Text) != 36 {
			t.Fatalf("%dx stamp %q cellW=%d units=%d", scale, s.Key, s.CellW, textUnits(s.Text))
		}
		// Each column name starts at the data column below it.
		for r, col := range map[rune]int{'點': 23, '加': 29, '總': 35} {
			i := strings.IndexRune(string(s.Text), r)
			if x := s.X + len([]rune(string(s.Text)[:i]))*s.CellW; x != col*8 {
				t.Fatalf("%dx %c at logical x %d, want %d", scale, r, x, col*8)
			}
		}
		s.State = xlate.Shown // Apply leaves the stamp Pending until Frame
		rgba := make([]byte, 320*scale*200*scale*4)
		o.layer.Draw(rgba, scale, nil)
		green := func(x, y int) bool { i := (y*320*scale + x) * 4; return rgba[i+1] == 255 && rgba[i] == 0 }
		off := menuGlyphOffset(scale)
		y := 32*scale + off
		for _, col := range []int{23, 29, 35} {
			if !green(col*8*scale+off, y) || green(col*8*scale-1, y) {
				t.Fatalf("%dx column %d: ink does not start at the column", scale, col)
			}
		}
		if green(22*8*scale+off, y) || green(27*8*scale+off, y) || green(33*8*scale+off, y) {
			t.Fatalf("%dx: ink between columns", scale)
		}
	}
	// Without the list the row is the general layout (spaces are half units).
	o, _ := NewRuntimeMenuOverlay(rects, font, 2)
	if err := o.Apply(event, request, palette); err != nil {
		t.Fatal(err)
	}
	if len(o.layer.Stamps) != 5 {
		t.Fatalf("general layout: %d segments", len(o.layer.Stamps))
	}
}

func TestHeaderColumnsLoadCheckFailures(t *testing.T) {
	head := "family\tkey\tcols\tevidence\n"
	for name, body := range map[string]string{
		"family":     "table\tk\t0,4\tx\n",
		"uc":         "dispatcher\tfrag.x.uc\t0,4\tx\n",
		"duplicate":  "menu\tk\t0,4\tx\nmenu\tk\t0,5\tx\n",
		"increasing": "menu\tk\t4,4\tx\n",
		"number":     "menu\tk\t0,a\tx\n",
		"negative":   "menu\tk\t-1,4\tx\n",
		"empty":      "menu\tk\t0,4\t\n",
	} {
		if _, err := LoadHeaderColumns("h.tsv", []byte(head+body)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	menuCases := []struct{ name, zh, cols string }{
		{"token count", "點數 加值 總計", "2,8"},
		{"column width", "點數 加值 總計", "2,4,14"}, // 4 units + 1 > 2·2
		{"last column", "點數 加值 總計", "2,8,17"},  // 34 + 4 > 36
		{"double space", "點數  加值 總計", "2,8,14"},
		{"leading space", " 點數 加值 總計", "2,8,14"},
		{"trailing space", "點數 加值 總計 ", "2,8,14"},
		{"key", "點數 加值 總計", ""},
	}
	for _, c := range menuCases {
		catalog, _ := headerTestCatalog(t, c.zh)
		row := "menu\tcareer.screen.columns.heading\t" + c.cols + "\tx"
		if c.cols == "" {
			row = "menu\tcareer.screen.missing\t2,8,14\tx"
		}
		if err := headerTestList(t, row).ValidateMenu(catalog); err == nil {
			t.Errorf("menu %s accepted", c.name)
		}
	}
	// The boundary itself passes: one unit between columns, last column
	// ending exactly at the original length × 2.
	catalog, _ := headerTestCatalog(t, "點數 加 總計")
	if err := headerTestList(t, "menu\tcareer.screen.columns.heading\t2,5,16\tx").ValidateMenu(catalog); err != nil {
		t.Errorf("boundary rejected: %v", err)
	}
	eng := headerTestEngine(t)
	for name, row := range map[string]string{
		"events":      "dispatcher\tfrag.000000000000\t0,17,32\tx",
		"width":       "dispatcher\t" + headerTestFrag + "\t0,2,32\tx",
		"last column": "dispatcher\t" + headerTestFrag + "\t0,17,37\tx",
		"no text":     "dispatcher\t" + headerTestUntranslated + "\t0,17,32\tx",
	} {
		if err := headerTestList(t, row).ValidateDispatcher(eng); err == nil {
			t.Errorf("dispatcher %s accepted", name)
		}
	}
	if err := headerTestList(t, "dispatcher\t"+headerTestFrag+"\t0,17,32\tx").ValidateDispatcher(nil); err == nil {
		t.Error("dispatcher rows without an engine catalog accepted")
	}
}

// Synthetic dispatcher header: words at cells 0, 17 and 32 of 38.
var (
	headerTestHeader       = "Aaaaa" + strings.Repeat(" ", 12) + "Bbbbbbb" + strings.Repeat(" ", 8) + "Cccccc"
	headerTestShifted      = "Aaaaa" + strings.Repeat(" ", 11) + "Bbbbbbb" + strings.Repeat(" ", 9) + "Cccccc"
	headerTestFrag         = engineTestKey(headerTestHeader)
	headerTestUntranslated = engineTestKey("Zz  Yy")
)

func engineTestKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("frag.%x", h[:6])
}

func headerTestEngine(t *testing.T) *EngineTextCatalog {
	t.Helper()
	upper := strings.ToUpper(headerTestHeader)
	events := string(engineTSV("frag", headerTestHeader, headerTestShifted, "Zz  Yy"))
	h := sha256.Sum256([]byte(upper))
	events += fmt.Sprintf("%s.uc\t%d\t%x\n", headerTestFrag, len(upper), h)
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: []byte(events),
		FragmentText:   engineZh("frag", headerTestHeader, "醫生 病患 結果", headerTestShifted, "醫生 病患 結果"),
		ItemEvents:     engineTSV("item", "Zqx"),
		ItemText:       engineZh("item", "Zqx", "物"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHeaderColumnsDispatcher(t *testing.T) {
	c := headerTestEngine(t)
	shiftedKey := engineTestKey(headerTestShifted)
	// The shifted row is listed with the wrong anchors on purpose: its
	// original starts (0,16,32) differ, so it keeps the general layout.
	list := headerTestList(t, "dispatcher\t"+headerTestFrag+"\t0,17,32\tx", "dispatcher\t"+shiftedKey+"\t0,17,32\tx")
	caller := CodeKey{Segment: 0x0763, Offset: 0x1282}
	ret := Address{0x0763, 0x1282}
	w := NewEngineDispatchWatcher(c, map[CodeKey]bool{caller: true})
	if err := w.SetHeaderColumns(list); err != nil {
		t.Fatal(err)
	}
	enter := func(row uint16, s string) {
		w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 15, row, 1}, []byte(s))
		w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	}
	enter(4, headerTestHeader)
	enter(5, strings.ToUpper(headerTestHeader))
	enter(6, headerTestShifted)
	want := "醫生" + strings.Repeat("　", 15) + "病患" + strings.Repeat("　", 13) + "結果" + strings.Repeat("　", 4)
	for _, row := range []uint8{4, 5} {
		l, ok := lineAt(w, row)
		if !ok || string(l.Text) != want || l.Width != 38 {
			t.Fatalf("row %d: %q", row, string(l.Text))
		}
		// Each name starts at its original column.
		for i, col := range []int{0, 17, 32} {
			r := []rune("醫病結")[i]
			k := strings.IndexRune(string(l.Text), r)
			if u := textUnits([]rune(string(l.Text)[:k])); u != 2*col {
				t.Fatalf("row %d %c at unit %d, want %d", row, r, u, 2*col)
			}
		}
	}
	l, ok := lineAt(w, 6)
	if !ok || string(l.Text) != pad("醫生 病患 結果", 38) {
		t.Fatalf("mismatch row: %q", string(l.Text))
	}
	if w.HeaderStats.Anchored != 2 || w.HeaderStats.Mismatches != 1 || w.Stats.Hits != 3 {
		t.Fatalf("stats %+v %+v", w.HeaderStats, w.Stats)
	}
	// Without the list the header row keeps the general layout.
	w2 := NewEngineDispatchWatcher(c, map[CodeKey]bool{caller: true})
	w2.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 15, 4, 1}, []byte(headerTestHeader))
	if l, _ := lineAt(w2, 4); string(l.Text) != pad("醫生 病患 結果", 38) {
		t.Fatalf("no list: %q", string(l.Text))
	}
}

// Load check through the live runtime with the formal text directory: the
// formal list loads; a list that fails the check stops the runtime.
func TestHeaderColumnsLiveLoad(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT not set")
	}
	fontPath := filepath.Join(root, "workplace/current-font/buckrogers-eten-top-pad.golemfnt")
	if _, err := os.Stat(fontPath); err != nil {
		t.Skip("本機工作字型缺席")
	}
	src := filepath.Join(root, "text")
	r, err := LoadLiveRuntime(src, fontPath)
	if err != nil {
		t.Fatal(err)
	}
	h := r.menu.HeaderColumns()
	if !equalInts(h.MenuColumns("career.screen.columns.heading"), []int{2, 8, 14}) || !h.HasDispatcher() || r.engDisp.headers != h {
		t.Fatal("formal list not installed")
	}
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(src, "cmudict"), filepath.Join(dir, "cmudict")); err != nil {
		t.Fatal(err)
	}
	formal, _ := os.ReadFile(filepath.Join(src, HeaderColumnsFile))
	for name, bad := range map[string]string{
		"menu width":      strings.Replace(string(formal), "2,8,14", "2,4,14", 1),
		"dispatcher last": strings.Replace(string(formal), "0,17,32", "0,17,37", 1),
		"unknown key":     strings.Replace(string(formal), "frag.0ddcc5022a9c", "frag.000000000000", 1),
	} {
		if err := os.WriteFile(filepath.Join(dir, HeaderColumnsFile), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLiveRuntime(dir, fontPath); err == nil || !strings.Contains(err.Error(), HeaderColumnsFile) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
