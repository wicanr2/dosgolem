package main

import (
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
)

func words(ks []dos.Key) []uint16 {
	out := make([]uint16, len(ks))
	for i, k := range ks {
		out[i] = k.Word()
	}
	return out
}

func eq(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRepeatTiming(t *testing.T) {
	var fired []int
	for d := 1; d <= 50; d++ {
		if fires(d) {
			fired = append(fired, d-1)
		}
	}
	want := []int{0, 30, 36, 42, 48}
	if len(fired) != len(want) {
		t.Fatalf("%v", fired)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Fatalf("%v", fired)
		}
	}
}

func TestNumpadDropsDuplicateDigit(t *testing.T) {
	keys, _ := mapKeys(frameInput{Chars: []rune{'8'}, Held: map[string]int{"KP8": 1}}, false, nil)
	if !eq(words(keys), []uint16{0x4800}) {
		t.Fatalf("%04X", words(keys))
	}
	// 上排數字照送。
	keys, _ = mapKeys(frameInput{Chars: []rune{'8'}, Held: map[string]int{}}, false, nil)
	if !eq(words(keys), []uint16{0x0938}) {
		t.Fatalf("%04X", words(keys))
	}
}

func TestCtrlAltLetters(t *testing.T) {
	keys, _ := mapKeys(frameInput{Chars: []rune{'c'}, Held: map[string]int{"C": 1}, Ctrl: true}, false, nil)
	if !eq(words(keys), []uint16{0x2E03}) {
		t.Fatalf("Ctrl+C %04X", words(keys))
	}
	keys, _ = mapKeys(frameInput{Held: map[string]int{"X": 1}, Alt: true}, false, nil)
	if !eq(words(keys), []uint16{0x2D00}) {
		t.Fatalf("Alt+X %04X", words(keys))
	}
	// 沒有修飾鍵時字母只走字元路徑。
	keys, _ = mapKeys(frameInput{Chars: []rune{'x'}, Held: map[string]int{"X": 1}}, false, nil)
	if !eq(words(keys), []uint16{0x2D78}) {
		t.Fatalf("x %04X", words(keys))
	}
}

func TestReservedAndHelp(t *testing.T) {
	keys, acts := mapKeys(frameInput{Held: map[string]int{"F1": 1, "F4": 1, "F12": 5}}, false, nil)
	if !eq(words(keys), []uint16{0x3E00}) || len(acts) != 1 || acts[0] != actHelp {
		t.Fatalf("%04X %v", words(keys), acts)
	}
	keys, acts = mapKeys(frameInput{Chars: []rune{'a'}, Held: map[string]int{"F1": 1, "Enter": 1}}, true, nil)
	if len(keys) != 0 || len(acts) != 1 {
		t.Fatalf("說明頁開啟仍送鍵 %v %v", keys, acts)
	}
}

func TestLogbookConsumesPaging(t *testing.T) {
	keys, _ := mapKeys(frameInput{Held: map[string]int{"PgDn": 1}}, false, func(int) bool { return true })
	if len(keys) != 0 {
		t.Fatal("手札開啟仍送 PgDn")
	}
	keys, _ = mapKeys(frameInput{Held: map[string]int{"PgDn": 1}}, false, func(int) bool { return false })
	if !eq(words(keys), []uint16{0x5100}) {
		t.Fatalf("%04X", words(keys))
	}
}

func TestMouseMapping(t *testing.T) {
	cases := []struct {
		cx, cy, scale int
		x, y          int
		target        host.MouseTarget
	}{
		{0, 0, 2, 0, 36, host.MouseTargetCanvas},
		{639, 399, 2, 638, 36 + 398, host.MouseTargetCanvas},
		{959, 599, 3, 638, 36 + 398, host.MouseTargetCanvas},
		{640, 10, 2, 640, 36 + 10, host.MouseTargetOutside},
		{-1, 10, 2, -2, 36 + 10, host.MouseTargetOutside},
		{10, 600, 3, 6, 36 + 400, host.MouseTargetOutside},
	}
	for _, c := range cases {
		dx, dy := dosPoint(c.cx, c.cy, c.scale)
		ev := mouseEvent(host.MouseEventDown, dx, dy)
		if ev.X != c.x || ev.Y != c.y || ev.Target != c.target {
			t.Fatalf("%+v → %+v", c, ev)
		}
	}
}

func TestParseScript(t *testing.T) {
	s, err := parseScript("5:F4,6:Ctrl+C,7:Alt+x,10:click@100;50,30:press@1;2,31:release@400;2,32:blur,40:help,41:Enter,42:scale")
	if err != nil {
		t.Fatal(err)
	}
	if s[10][0].mouse != host.MouseEventDown || s[16][0].mouse != host.MouseEventUp || s[16][0].x != 100 {
		t.Fatalf("click %+v %+v", s[10], s[16])
	}
	if s[31][0].mouse != host.MouseEventUp || s[31][0].x != 400 || !s[32][0].blur || !s[40][0].help {
		t.Fatal("press/release/blur/help")
	}
	if !s[42][0].scale {
		t.Fatal("scale")
	}
	if s[5][0].key.Word() != 0x3E00 || s[6][0].key.Word() != 0x2E03 || s[7][0].key.Word() != 0x2D00 {
		t.Fatal("鍵")
	}
	for _, bad := range []string{"1:click@400;1", "1:click@1,2", "1:click@1;1,3:click@2;2", "1:Ctrl+1", "0:Enter", "1:Nope"} {
		if _, err := parseScript(bad); err == nil {
			t.Fatalf("%q 應失敗", bad)
		}
	}
}
