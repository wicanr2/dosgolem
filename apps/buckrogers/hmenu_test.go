package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func hmenuFixture(t *testing.T, pairs ...string) *HMenuCatalog {
	t.Helper()
	ev := "event_key\toriginal_length\toriginal_sha256\n"
	tr := "key\ttranslation\tsource\n"
	for i := 0; i+1 < len(pairs); i += 2 {
		k := fmt.Sprintf("hmenu.t%02d", i/2)
		ev += fmt.Sprintf("%s\t%d\t%x\n", k, len(pairs[i]), sha256.Sum256([]byte(pairs[i])))
		if pairs[i+1] != "" {
			tr += fmt.Sprintf("%s\t%s\tecl-batch-editorial\n", k, pairs[i+1])
		}
	}
	c, err := LoadHMenuCatalog([]byte(ev), []byte(tr))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func hmenuEntry(text string, sel uint8, items ...[2]uint8) HMenuEntry {
	return HMenuEntry{SS: 0x1841, SP: 0x3000, Return: Address{0x37F1, 0x0ACF}, Text: []byte(text), Row: 24, Col: 0,
		Items: items, Selected: sel, Normal: 10, Hot: 15}
}

func TestHMenuBuildColorsAndLifecycle(t *testing.T) {
	w := NewHMenuWatcher(hmenuFixture(t, "Move", "移動(M)", "Area", "區域(A)", "Look", ""))
	w.ObserveEntry(hmenuEntry("Move Area", 2, [2]uint8{1, 4}, [2]uint8{6, 9}))
	pg := w.Page()
	if pg == nil || len(pg.Rows) != 1 || len(pg.Rows[0].Cells) != 40 {
		t.Fatalf("page: %+v", pg)
	}
	p := pg.Rows[0]
	// 移動(M): M is hot (fg 15); item 2 selected: inverse (bg 15, fg 0).
	if c := p.Cells[3]; c.Rune != 'M' || c.FG != 15 || c.BG != 0 {
		t.Fatalf("hotkey cell %+v", c)
	}
	if c := p.Cells[0]; c.FG != 10 {
		t.Fatalf("normal cell %+v", c)
	}
	if c := p.Cells[6]; c.Rune != '區' || c.BG != 15 || c.FG != 0 {
		t.Fatalf("selected cell %+v", c)
	}
	w.ObserveVideoWrite(24*8*320 + 5)
	if w.Page() == nil {
		t.Fatal("in-call write dropped")
	}
	w.ObserveEntry(hmenuEntry("Move", 1, [2]uint8{1, 4}))
	if w.Page() != nil || w.Stats.Reentries != 1 {
		t.Fatal("re-entry kept page")
	}
	w.ObserveEntry(hmenuEntry("Move", 1, [2]uint8{1, 4}))
	w.ObserveInstruction(Address{0x37F1, 0x0ACF}, 0x1841, 0x3000+hmenuReturnDelta)
	if w.InCall() || w.Page() == nil {
		t.Fatal("return not verified")
	}
	w.ObserveVideoWrite(23*8*320 + 5)
	if w.Page() == nil {
		t.Fatal("other row dropped")
	}
	w.ObserveVideoWrite(24*8*320 + 5)
	if w.Page() != nil {
		t.Fatal("post-call write kept page")
	}
	// Untranslated item, '@', overflow.
	w.ObserveEntry(hmenuEntry("Move Look", 1, [2]uint8{1, 4}, [2]uint8{6, 9}))
	if w.Page() != nil {
		t.Fatal("untranslated item drew")
	}
	w.ObserveInstruction(Address{0x37F1, 0x0ACF}, 0x1841, 0x3000+hmenuReturnDelta)
	w.ObserveEntry(hmenuEntry("Mo@e", 1, [2]uint8{1, 4}))
	if w.Page() != nil {
		t.Fatal("item spanning @ drew")
	}
	two := hmenuEntry("Move@Area", 1, [2]uint8{1, 4}, [2]uint8{6, 9})
	two.Row = 23
	w.ObserveEntry(two)
	if pg := w.Page(); pg == nil || len(pg.Rows) != 2 || pg.Rows[1].Row != 24 || pg.Rows[1].Cells[0].Rune != '區' {
		t.Fatalf("two-row menu: %+v", w.Page())
	}
	w.ObserveInstruction(Address{0x37F1, 0x0ACF}, 0x1841, 0x3000+hmenuReturnDelta)
	e := hmenuEntry("Move", 1, [2]uint8{1, 4})
	e.Col = 36
	w.ObserveEntry(e)
	if w.Page() != nil || w.Stats.Overflows != 1 {
		t.Fatal("overflow drew")
	}
}

func TestHMenuStartsAtFirstItemColumn(t *testing.T) {
	w := NewHMenuWatcher(hmenuFixture(t, "Yes", "是(Y)", "No", "否(N)"))
	e := hmenuEntry("   Yes No", 1, [2]uint8{4, 6}, [2]uint8{8, 9})
	e.Col = 10
	w.ObserveEntry(e)
	p := w.Page()
	if p == nil || p.Rows[0].Col != 13 || p.Rows[0].Cells[0].Rune != '是' || len(p.Rows[0].Cells) != 27 {
		t.Fatalf("%+v", p)
	}
}

func TestHMenuLookupFallsBackToCapitals(t *testing.T) {
	c := hmenuFixture(t, "ORDER FOOD", "點餐(O)", "Wait", "等待(W)")
	if z, ok := c.lookup("Order food"); !ok || z != "點餐(O)" {
		t.Fatalf("mixed-case ECL item: %q %v", z, ok)
	}
	if _, ok := c.lookup("wAIT"); ok {
		t.Fatal("capitals fallback must not lower-case catalog keys")
	}
	if _, ok := c.lookup("TALK"); ok {
		t.Fatal("missing item hit")
	}
}
