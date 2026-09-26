package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func engineTSV(prefix string, texts ...string) []byte {
	var b strings.Builder
	b.WriteString("event_key\toriginal_length\toriginal_sha256\n")
	for _, t := range texts {
		h := sha256.Sum256([]byte(t))
		fmt.Fprintf(&b, "%s.%x\t%d\t%x\n", prefix, h[:6], len(t), h)
	}
	return []byte(b.String())
}

func engineZh(prefix string, pairs ...string) []byte {
	var b strings.Builder
	b.WriteString("key\ttranslation\tsource\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		h := sha256.Sum256([]byte(pairs[i]))
		fmt.Fprintf(&b, "%s.%x\t%s\tecl-batch-editorial\n", prefix, h[:6], pairs[i+1])
	}
	return []byte(b.String())
}

func engineFixture(t *testing.T) *EngineTextCatalog {
	t.Helper()
	frags := []string{" makes ", "his", " tactics roll.", "Hitting for ", " points ", "of damage", "AC", "Attacks", " leads ", "From"}
	items := []string{"Bolt", "Gun", "Laser", "Pistol", "Heavy Body", "Armor", "Good", "Space"}
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: engineTSV("frag", frags...),
		FragmentText: engineZh("frag", " makes ", "進行", "his", "他的", " tactics roll.", "戰術判定。", "Hitting for ", "造成",
			" points ", "點", "of damage", "傷害", "AC", "防禦", "Attacks", "攻擊"),
		ItemEvents:    engineTSV("item", items...),
		ItemText:      engineZh("item", "Bolt", "爆能", "Gun", "槍", "Laser", "雷射", "Pistol", "手槍", "Heavy Body", "重型", "Armor", "護甲"),
		MonsterEvents: engineTSV("monster", "TERRINE WARRIOR", "NEO WARRIOR"),
		MonsterText:   engineZh("monster", "TERRINE WARRIOR", "特林戰士", "NEO WARRIOR", "NEO 戰士"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEngineDecomposeAndTranslate(t *testing.T) {
	c := engineFixture(t)
	cases := []struct{ in, sig, zh string }{
		{"FLAVIUS makes his tactics roll.", "_|F|F|F", "FLAVIUS 進行他的戰術判定。"},
		{"Hitting for 2 points of damage", "F|_|F|F", "造成 2 點傷害"},
		{"Bolt Gun (100)", "I|_", "爆能槍 (100)"},
		{"Heavy Body Armor", "I", "重型護甲"},
		{"AC", "F", "防禦"},
		{"TERRINE WARRIOR makes his tactics roll.", "_|F|F|F", "特林戰士進行他的戰術判定。"},
		// The slot already carries the space after the fragment: no second space.
		{"Hitting for  2 points of damage", "F|_|F|F", "造成 2 點傷害"},
	}
	for _, tc := range cases {
		parts := c.Decompose(tc.in)
		var kinds []string
		for _, p := range parts {
			kinds = append(kinds, string(p.Kind))
		}
		if got := strings.Join(kinds, "|"); got != tc.sig {
			t.Errorf("%q: kinds %s want %s", tc.in, got, tc.sig)
		}
		if zh, ok := c.Translate(tc.in); !ok || zh != tc.zh {
			t.Errorf("%q: %q %v want %q", tc.in, zh, ok, tc.zh)
		}
	}
	// Untranslated fragment, all-caps names, and a lone common item word.
	if zh, ok := c.Translate("TERRINE WARRIOR"); !ok || zh != "特林戰士" {
		t.Errorf("monster name: %q %v", zh, ok)
	}
	for _, in := range []string{"CELESTE leads NEO WARRIOR.", "FLAVIUS", "Good", "ACE PILOT"} {
		if zh, ok := c.Translate(in); ok {
			t.Errorf("%q translated to %q", in, zh)
		}
	}
}

func TestEngineDispatchAllowListAndLifecycle(t *testing.T) {
	c := engineFixture(t)
	if _, err := LoadEngineDispatchCallers([]byte("caller\tnote\n37F1:101E\tx\n"), map[Address]bool{{0x37F1, 0x101E}: true}); err == nil {
		t.Fatal("owned caller accepted")
	}
	allow, err := LoadEngineDispatchCallers([]byte("caller\tnote\n1FEB:0560\tAC\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	w := NewEngineDispatchWatcher(c, allow)
	ret := Address{0x1FEB, 0x0560}
	args := [6]uint16{0, 0, 0, 5, 3, 30}
	w.ObserveEntry(Address{0x1FEB, 0x0999}, 1, 0x100, Address{0x1FEB, 0x0999}, args, []byte("AC"))
	if w.Page() != nil {
		t.Fatal("caller outside allow list drew")
	}
	w.ObserveEntry(ret, 1, 0x100, ret, args, []byte("AC"))
	p := w.Page()
	if p == nil || p.Rows[0].Row != 3 || p.Rows[0].Col != 30 || string(p.Rows[0].Cells[0].Rune) != "防" {
		t.Fatalf("%+v", p)
	}
	w.SetNameCallers(map[Address]bool{{0x37F1, 0x235A}: true})
	nameCaller := Address{0x37F1, 0x235A}
	w.ObserveEntry(nameCaller, 1, 0x100, nameCaller, [6]uint16{0, 0, 0, 5, 7, 23}, []byte("FLAVIUS"))
	w.ObserveInstruction(nameCaller, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveEntry(nameCaller, 1, 0x100, nameCaller, [6]uint16{0, 0, 0, 5, 8, 23}, []byte("AC"))
	w.ObserveInstruction(nameCaller, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveEntry(nameCaller, 1, 0x100, nameCaller, [6]uint16{0, 0, 0, 5, 9, 23}, []byte("TERRINE WARRIOR"))
	w.ObserveInstruction(nameCaller, 1, 0x100+engineDispatchReturnDelta)
	if p := w.Page(); p == nil || len(p.Rows) != 2 || p.Rows[1].Row != 9 {
		t.Fatalf("name-only caller: %+v", p)
	}
	w.ObserveEntry(ret, 1, 0x100, ret, args, []byte("AC"))
	w.ObserveVideoWrite(3*8*320 + 30*8)
	if w.Page() == nil {
		t.Fatal("in-call write dropped line")
	}
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveVideoWrite(3*8*320 + 31*8)
	for _, r := range w.Page().Rows {
		if r.Row == 3 {
			t.Fatal("post-call write kept line")
		}
	}
}

func TestEngineTemplateWildcard(t *testing.T) {
	h := func(s string) string { x := sha256.Sum256([]byte(s)); return fmt.Sprintf("frag.%x", x[:6]) }
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: engineTSV("frag", "Drop ", " forever? ", "Knife", "Use antidote on "),
		FragmentText:   engineZh("frag", "Drop ", "丟棄", " forever? ", "永久嗎？", "Knife", "小刀", "Use antidote on ", "使用解毒劑於"),
		ItemEvents:     engineTSV("item", "Bolt", "Gun"),
		ItemText:       engineZh("item", "Bolt", "爆能", "Gun", "槍"),
		TemplateEvents: []byte("event_key\tsignature\ntpl.drop\t" + h("Drop ") + "|*|" + h(" forever? ") + "\ntpl.antidote\t" + h("Use antidote on ") + "|_\n"),
		TemplateText:   []byte("key\ttranslation\tsource\ntpl.drop\t永久丟棄{0}嗎？\tecl-batch-editorial\ntpl.antidote\t對{0}使用解毒劑\tecl-batch-editorial\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"Drop Knife forever? ":    "永久丟棄小刀嗎？",
		"Drop Bolt Gun forever? ": "永久丟棄爆能槍嗎？",
		"Use antidote on FLAVIUS": "對FLAVIUS使用解毒劑",
	} {
		if got, ok := c.Translate(in); !ok || got != want {
			t.Errorf("%q -> %q %v, want %q", in, got, ok, want)
		}
	}
}
