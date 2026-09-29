package buckrogers

import (
	"strings"
	"testing"
)

// Spec 038 §3.4 (第 2 期) acceptance 1: the party column, the full-width
// party table / Pick Character and the character sheet title.

var partyPanelCallers = []CodeKey{partyNameCaller, partySelectedCaller, partyCursorCaller}

func panelWatcher(t *testing.T, tr fakeTranslit) *EngineDispatchWatcher {
	w := NewEngineDispatchWatcher(engineFixture(t), map[CodeKey]bool{})
	w.SetNameCallers(map[CodeKey]bool{partyNameCaller: true})
	if tr == nil {
		w.SetPlayerNames(testPlayers(t))
	} else {
		w.SetPlayerNames(NewPlayerNames(tr, nil))
	}
	return w
}

// panelEntry runs one dispatcher call (entry and return) from caller.
func panelEntry(w *EngineDispatchWatcher, caller CodeKey, rec partyRec, row, col, fg uint8, s string, party *PartySnapshot, video MemReader) {
	ret := Address{0x1C41, caller.Offset}
	args := [6]uint16{rec.off, rec.seg, 0, uint16(fg), uint16(row), uint16(col)}
	w.ObserveEntryParty(caller, 1, 0x100, ret, args, []byte(s), party, video)
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
}

func lineAt(w *EngineDispatchWatcher, row uint8) (EngineDispatchLine, bool) {
	for _, l := range w.Lines() {
		if l.Row == row {
			return l, true
		}
	}
	return EngineDispatchLine{}, false
}

func pad(s string, n int) string {
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

func TestPartyPanelTable(t *testing.T) {
	mem := partyMem(2, recFlavius, recCeleste)
	party, err := ReadPartySnapshot(mem, testDS)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		row, col uint8
		want     string // "" = not drawn
	}{
		{4, 17, pad("弗拉維烏斯", 7)},          // party column: Chinese only
		{9, 17, pad("弗拉維烏斯", 7)},          // last table row
		{4, 1, "弗拉維烏斯(FLAVIUS)"},         // full-width table
		{9, 1, "弗拉維烏斯(FLAVIUS)"},         //
		{1, 8, "弗拉維烏斯(FLAVIUS)"},         // character sheet title
		{3, 17, ""}, {10, 17, ""}, {3, 1, ""}, // rows outside the table
		{1, 1, ""},                            // item page title (then 2391 's)
		{2, 8, ""},                            // sheet column on another row
		{4, 4, ""},                            // training confirm page column 4
		{4, 2, ""},                            //
	}
	for _, caller := range partyPanelCallers {
		for _, c := range cases {
			w := panelWatcher(t, nil)
			panelEntry(w, caller, recFlavius, c.row, c.col, 15, "FLAVIUS", party, mem)
			l, ok := lineAt(w, c.row)
			if c.want == "" {
				if ok {
					t.Errorf("%v r%d c%d drawn %q", caller, c.row, c.col, string(l.Text))
				}
				continue
			}
			if !ok || string(l.Text) != c.want || int(l.Width) != len([]rune(c.want)) || l.Col != c.col {
				t.Errorf("%v r%d c%d: %+v want %q", caller, c.row, c.col, l, c.want)
			}
		}
	}
	// Pointer in the snapshot but different content (record reused): no hit
	// for any caller.
	for _, caller := range partyPanelCallers {
		w := panelWatcher(t, nil)
		panelEntry(w, caller, recFlavius, 4, 17, 15, "PIERRE", party, mem)
		if len(w.Lines()) != 0 {
			t.Errorf("%v: content mismatch drawn", caller)
		}
	}
}

func TestPartyPanelNonMember(t *testing.T) {
	mon := partyRec{seg: 0x5800, off: 4, name: "NEO WARRIOR"}
	mem := partyMem(1, recFlavius, mon)
	party, _ := ReadPartySnapshot(mem, testDS)
	// 21DE / 0388 with a non-member pointer: no overlay, no monster lookup,
	// no counters, and the call's own writes still invalidate.
	for _, caller := range []CodeKey{partySelectedCaller, partyCursorCaller} {
		w := panelWatcher(t, nil)
		panelEntry(w, caller, mon, 5, 17, 15, "NEO WARRIOR", party, mem)
		panelEntry(w, caller, partyRec{seg: 0x3F00, off: 0x10}, 6, 17, 15, "NEO WARRIOR", party, mem)
		if len(w.Lines()) != 0 || w.Stats.Hits != 0 || w.Stats.Misses != 0 {
			t.Errorf("%v: %+v %+v", caller, w.Lines(), w.Stats)
		}
		// nil snapshot: same.
		panelEntry(w, caller, recFlavius, 4, 17, 15, "FLAVIUS", nil, mem)
		if len(w.Lines()) != 0 {
			t.Errorf("%v: nil snapshot drawn", caller)
		}
	}
	// 235A keeps the monster lookup for a non-member.
	w := panelWatcher(t, nil)
	panelEntry(w, partyNameCaller, mon, 5, 17, 11, "NEO WARRIOR", party, mem)
	if l, ok := lineAt(w, 5); !ok || !strings.HasPrefix(string(l.Text), "NEO 戰士") {
		t.Errorf("235A monster: %+v", l)
	}
	// 235A: a member named like a monster is never the monster, in any
	// column and row (training page column 4, item title column 1 row 1),
	// also without spec-037 data.
	named := partyRec{seg: 0x5747, off: 2, name: "NEO WARRIOR"}
	mem2 := partyMem(1, named)
	p2, _ := ReadPartySnapshot(mem2, testDS)
	for _, rc := range [][2]uint8{{4, 4}, {1, 1}, {4, 17}, {4, 1}, {1, 8}, {20, 2}} {
		for _, players := range []bool{true, false} {
			w := panelWatcher(t, nil)
			if !players {
				w.SetPlayerNames(nil)
			}
			panelEntry(w, partyNameCaller, named, rc[0], rc[1], 13, "NEO WARRIOR", p2, mem2)
			if l, ok := lineAt(w, rc[0]); ok && strings.HasPrefix(string(l.Text), "NEO 戰士") {
				t.Errorf("r%d c%d players=%v: member drawn as monster", rc[0], rc[1], players)
			}
		}
	}
}

func TestPartyPanelShortPadsOriginalCells(t *testing.T) {
	long := partyRec{seg: 0x5747, off: 2, name: "NICOLE STEELE", gender: 1}
	mem := partyMem(1, long)
	party, _ := ReadPartySnapshot(mem, testDS)
	w := panelWatcher(t, fakeTranslit{"NICOLE STEELE": "妮可"})
	panelEntry(w, partyNameCaller, long, 6, 17, 12, "NICOLE STEELE", party, mem)
	l, ok := lineAt(w, 6)
	if !ok || l.Width != 13 || string(l.Text) != pad("妮可", 13) || l.BG != 0 || l.FG != 12 {
		t.Fatalf("short: %+v", l)
	}
	// The padding cells are drawn with the call's background.
	p := w.Page()
	if len(p.Rows[0].Cells) != 13 || p.Rows[0].Cells[12].BG != 0 || p.Rows[0].Cells[12].Rune != ' ' {
		t.Fatalf("cells: %+v", p.Rows[0].Cells)
	}
}

func TestPartyPanelBoundaries(t *testing.T) {
	type tc struct {
		name, zh string
		row, col uint8
		want     string // "" = English
	}
	n15 := "ABCDEFGHIJKLMNO"
	cases := []tc{
		// Party column (17 cells, col 17–33): Chinese only.
		{"AB", strings.Repeat("甲", 17), 4, 17, strings.Repeat("甲", 17)},
		{"AB", strings.Repeat("甲", 18), 4, 17, ""},
		// Full-width table (33 cells, col 1–33).
		{n15, strings.Repeat("甲", 16), 5, 1, strings.Repeat("甲", 16) + "(" + n15 + ")"}, // 33
		{n15, strings.Repeat("甲", 17), 5, 1, strings.Repeat("甲", 17)},                  // 34 → Chinese only
		{"AB", strings.Repeat("甲", 34), 5, 1, ""},                                       // neither fits
		// Character sheet title (20 cells, col 8–27).
		{"ABCDEFGHIJKL", strings.Repeat("甲", 6), 1, 8, strings.Repeat("甲", 6) + "(ABCDEFGHIJKL)"}, // 20
		{"ABCDEFGHIJKL", strings.Repeat("甲", 7), 1, 8, strings.Repeat("甲", 7) + "     "},          // 21 → Chinese only, padded to 12
		{"AB", strings.Repeat("甲", 20), 1, 8, strings.Repeat("甲", 20)},                            // Chinese only = 20
		{"AB", strings.Repeat("甲", 21), 1, 8, ""},
	}
	for _, c := range cases {
		rec := partyRec{seg: 0x5747, off: 2, name: c.name}
		mem := partyMem(1, rec)
		party, _ := ReadPartySnapshot(mem, testDS)
		w := panelWatcher(t, fakeTranslit{c.name: c.zh})
		panelEntry(w, partyNameCaller, rec, c.row, c.col, 13, c.name, party, mem)
		l, ok := lineAt(w, c.row)
		if c.want == "" {
			if ok {
				t.Errorf("%s/%d r%d c%d: drawn %q", c.name, len([]rune(c.zh)), c.row, c.col, string(l.Text))
			}
			continue
		}
		if !ok || string(l.Text) != c.want || int(l.Width) != len([]rune(c.want)) {
			t.Errorf("%s/%d r%d c%d: %q want %q", c.name, len([]rune(c.zh)), c.row, c.col, string(l.Text), c.want)
		}
	}
}

func TestPartyPanelExtensionEntryCheck(t *testing.T) {
	mem := partyMem(1, recFlavius)
	party, _ := ReadPartySnapshot(mem, testDS)
	dirty := func(row, col int) fakeMem {
		m := append(fakeMem(nil), mem...)
		m[0xA0000+uint32((row*8+3)*320+col*8+5)] = 7
		return m
	}
	// Sheet: full needs cells 15–21 background; one dirty pixel at col 18
	// steps down to Chinese only (5 ≤ 7 original cells).
	w := panelWatcher(t, nil)
	panelEntry(w, partyNameCaller, recFlavius, 1, 8, 11, "FLAVIUS", party, dirty(1, 18))
	if l, _ := lineAt(w, 1); string(l.Text) != pad("弗拉維烏斯", 7) || l.Width != 7 {
		t.Errorf("dirty extension: %+v", l)
	}
	if w.PartyStats.ChineseOnly != 1 {
		t.Errorf("stats %+v", w.PartyStats)
	}
	// A dirty pixel outside the cells the display needs does not matter.
	w = panelWatcher(t, nil)
	panelEntry(w, partyNameCaller, recFlavius, 1, 8, 11, "FLAVIUS", party, dirty(1, 22))
	if l, _ := lineAt(w, 1); string(l.Text) != "弗拉維烏斯(FLAVIUS)" {
		t.Errorf("clean needed cells: %+v", l)
	}
	// No A000 reader: the check fails.
	w = panelWatcher(t, nil)
	panelEntry(w, partyNameCaller, recFlavius, 1, 8, 11, "FLAVIUS", party, nil)
	if l, _ := lineAt(w, 1); string(l.Text) != pad("弗拉維烏斯", 7) {
		t.Errorf("nil video: %+v", l)
	}
	// Chinese only also wider than the original: English.
	short := partyRec{seg: 0x5747, off: 2, name: "AB"}
	m2 := partyMem(1, short)
	p2, _ := ReadPartySnapshot(m2, testDS)
	m2[0xA0000+uint32((4*8)*320+19*8)] = 1 // col 19 on row 4 (party column)
	w = panelWatcher(t, fakeTranslit{"AB": "甲乙丙"})
	panelEntry(w, partyNameCaller, short, 4, 17, 11, "AB", p2, m2)
	panelEntry(w, partyNameCaller, short, 4, 1, 13, "AB", p2, m2) // full needs 1–7, col 3 clean? row 4 col 19 only
	if l, ok := lineAt(w, 4); !ok || l.Col != 1 || string(l.Text) != "甲乙丙(AB)" {
		t.Errorf("full-width with far dirty cell: %+v", l)
	}
	w = panelWatcher(t, fakeTranslit{"AB": "甲乙丙"})
	panelEntry(w, partyNameCaller, short, 4, 17, 11, "AB", p2, m2)
	if _, ok := lineAt(w, 4); ok {
		t.Error("party column: dirty extension, Chinese only wider than original: must be English")
	}
	m2[0xA0000+uint32((4*8)*320+19*8)] = 0
	m2[0xA0000+uint32((4*8+7)*320+4*8+7)] = 1 // col 4 row 4
	w = panelWatcher(t, fakeTranslit{"AB": "甲乙丙"})
	panelEntry(w, partyNameCaller, short, 4, 1, 13, "AB", p2, m2)
	if _, ok := lineAt(w, 4); ok {
		t.Error("full-width: dirty extension, Chinese only wider than original: must be English")
	}
}

func TestPartyPanelSameTextAcrossCallers(t *testing.T) {
	mem := partyMem(2, recFlavius, recCeleste)
	party, _ := ReadPartySnapshot(mem, testDS)
	for _, sc := range [][2]uint8{{5, 17}, {5, 1}, {1, 8}} {
		var text string
		for i, c := range []struct {
			caller CodeKey
			fg     uint8
		}{{partyNameCaller, 13}, {partySelectedCaller, 15}, {partyCursorCaller, 15}, {partyNameCaller, 11}} {
			w := panelWatcher(t, nil)
			panelEntry(w, c.caller, recCeleste, sc[0], sc[1], c.fg, "CELESTE", party, mem)
			l, ok := lineAt(w, sc[0])
			if !ok || l.FG != c.fg {
				t.Fatalf("%v r%d c%d: %+v", c.caller, sc[0], sc[1], l)
			}
			if i == 0 {
				text = string(l.Text)
			} else if string(l.Text) != text {
				t.Errorf("%v r%d c%d: %q want %q", c.caller, sc[0], sc[1], string(l.Text), text)
			}
		}
		if !strings.HasPrefix(text, "塞萊絲特") {
			t.Errorf("gender: %q", text)
		}
	}
	// Cursor move on one watcher: 235A repaints the old row in the normal
	// color, 0388 the new row in 15; each call replaces its row's line.
	w := panelWatcher(t, nil)
	panelEntry(w, partySelectedCaller, recFlavius, 4, 17, 15, "FLAVIUS", party, mem)
	panelEntry(w, partyNameCaller, recCeleste, 5, 17, 11, "CELESTE", party, mem)
	panelEntry(w, partyNameCaller, recFlavius, 4, 17, 11, "FLAVIUS", party, mem)
	panelEntry(w, partyCursorCaller, recCeleste, 5, 17, 15, "CELESTE", party, mem)
	if len(w.Lines()) != 2 {
		t.Fatalf("lines %+v", w.Lines())
	}
	if l, _ := lineAt(w, 4); l.FG != 11 {
		t.Errorf("old row fg %d", l.FG)
	}
	if l, _ := lineAt(w, 5); l.FG != 15 {
		t.Errorf("new row fg %d", l.FG)
	}
}

func TestPartyPanelExtensionInvalidation(t *testing.T) {
	mem := partyMem(1, recFlavius)
	party, _ := ReadPartySnapshot(mem, testDS)
	at := func(row, col int) uint32 { return uint32((row*8+2)*320 + col*8 + 1) }
	w := panelWatcher(t, nil)
	panelEntry(w, partyNameCaller, recFlavius, 1, 8, 11, "FLAVIUS", party, mem) // cols 8–21
	// Writes during the call do not count; after the call a write in the
	// extension cells (col 21) removes the line, a write at col 22 does not.
	w.ObserveVideoWrite(at(1, 22))
	if len(w.Lines()) != 1 {
		t.Fatal("write outside the line removed it")
	}
	w.ObserveVideoWrite(at(1, 21))
	if len(w.Lines()) != 0 {
		t.Fatal("write in the extension cells kept the line")
	}
	// A later call at the same row replaces the line; a new entry whose own
	// cells only overlap the extension also drops it.
	panelEntry(w, partyNameCaller, recFlavius, 1, 8, 11, "FLAVIUS", party, mem)
	w.ObserveEntryParty(CodeKey{Unit: 0x2BA60, Offset: 0x235A}, 1, 0x100, Address{0x1C41, 0x235A},
		[6]uint16{0x10, 0x3F00, 0, 11, 1, 20}, []byte("X"), party, mem)
	if _, ok := lineAt(w, 1); ok {
		t.Fatal("entry over the extension kept the line")
	}
}
