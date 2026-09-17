package main

import "testing"

func TestParseDamageHookReadsEveryField(t *testing.T) {
	h, err := parseDamageHook("stub=010A:00AC,hp=11B,side=10E,combat=4954:5,party=0,lock,kill")
	if err != nil {
		t.Fatal(err)
	}
	if h.seg != 0x010A || h.off != 0x00AC || h.hpField != 0x11B || h.sideField != 0x10E ||
		h.combatAt != 0x4954 || h.combatValue != 5 || h.partySide != 0 || !h.partySideKnown || !h.lock || !h.kill {
		t.Fatalf("parsed %+v", h)
	}
}

func TestParseDamageHookRejectsBadSpecs(t *testing.T) {
	for _, spec := range []string{
		"hp=11B",                            // 缺 stub
		"stub=010A",                         // stub 缺 offset
		"stub=010A:00AC,lock",               // 開 lock 但沒給我方陣營
		"stub=010A:00AC,party=0,frobnicate", // 不認得的鍵
	} {
		if _, err := parseDamageHook(spec); err == nil {
			t.Fatalf("accepted %q", spec)
		}
	}
	if _, err := parseDamageHook("stub=010A:00AC,hp=11B,side=10E"); err != nil {
		t.Fatalf("只記錄的設定不該被擋：%v", err)
	}
}

func TestDamageHookDecide(t *testing.T) {
	h := &damageHook{lock: true, kill: true}
	cases := []struct {
		party              bool
		hp, damage, wanted uint8
	}{
		{true, 8, 5, 0},    // 我方：鎖 HP
		{false, 30, 4, 30}, // 敵方：一擊斃命
		{false, 30, 0, 0},  // 敵方沒受傷（豁免成功）：不改
		{false, 3, 9, 9},   // 敵方本來就會死：不改
	}
	for _, c := range cases {
		if got := h.decide(c.party, c.hp, c.damage); got != c.wanted {
			t.Fatalf("decide(%t, %d, %d) = %d, want %d", c.party, c.hp, c.damage, got, c.wanted)
		}
	}
	if got := (&damageHook{}).decide(true, 8, 5); got != 5 {
		t.Fatalf("沒開 lock 也改了傷害：%d", got)
	}
}
