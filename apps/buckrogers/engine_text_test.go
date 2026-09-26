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
		ItemEvents: engineTSV("item", items...),
		ItemText:   engineZh("item", "Bolt", "爆能", "Gun", "槍", "Laser", "雷射", "Pistol", "手槍", "Heavy Body", "重型", "Armor", "護甲"),
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
	for _, in := range []string{"CELESTE leads NEO WARRIOR.", "TERRINE WARRIOR", "Good", "ACE PILOT"} {
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
	w.ObserveVideoWrite(3*8*320 + 30*8)
	if w.Page() == nil {
		t.Fatal("in-call write dropped line")
	}
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveVideoWrite(3*8*320 + 31*8)
	if w.Page() != nil {
		t.Fatal("post-call write kept line")
	}
}
