package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// `docs/spec/441-play-frontend` §4 的契約測試。

func TestFrameTargetsNoDrift(t *testing.T) {
	const sps = 11_580_882
	prev := uint64(0)
	for f := 0; f < 600; f++ {
		target := frameTarget(f, sps)
		if n := target - prev; n != sps/tps && n != sps/tps+1 {
			t.Fatalf("第 %d 格 %d 步，要 %d 或 %d", f, n, sps/tps, sps/tps+1)
		}
		prev = target
	}
	if want := uint64(600) * sps / tps; prev != want {
		t.Fatalf("600 格共 %d 步，要 %d", prev, want)
	}
}

func TestKeyTable(t *testing.T) {
	cases := []struct {
		k           ebiten.Key
		mod         modifiers
		scan, ascii uint8
	}{
		{ebiten.KeyA, modifiers{}, 0x1E, 'a'},
		{ebiten.KeyA, modifiers{shift: true}, 0x1E, 'A'},
		{ebiten.KeyA, modifiers{caps: true}, 0x1E, 'A'},
		{ebiten.KeyA, modifiers{shift: true, caps: true}, 0x1E, 'a'},
		{ebiten.KeyC, modifiers{ctrl: true}, 0x2E, 0x03},
		{ebiten.KeyDigit1, modifiers{shift: true}, 0x02, '!'},
		{ebiten.KeyEnter, modifiers{}, 0x1C, 0x0D},
		{ebiten.KeyEscape, modifiers{}, 0x01, 0x1B},
		{ebiten.KeySpace, modifiers{}, 0x39, ' '},
		{ebiten.KeyArrowUp, modifiers{}, 0x48, 0},
		{ebiten.KeyF10, modifiers{}, 0x44, 0},
	}
	for _, c := range cases {
		k, ok := hostKey(keyCode(c.k), c.mod)
		if !ok || k.Scan != c.scan || k.ASCII != c.ascii {
			t.Errorf("%s %+v：(%02X, %02X, %v)，要 (%02X, %02X)", c.k, c.mod, k.Scan, k.ASCII, ok, c.scan, c.ascii)
		}
	}
	for _, k := range []ebiten.Key{ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyControlLeft, ebiten.KeyAltLeft} {
		key, ok := hostKey(keyCode(k), modifiers{})
		if !ok || !modifierOnly(key) {
			t.Errorf("%s 應是單獨的修飾鍵（不進 BIOS 佇列）", k)
		}
	}
	if key, _ := hostKey(keyCode(ebiten.KeyA), modifiers{}); modifierOnly(key) {
		t.Error("A 不是修飾鍵")
	}
	if _, ok := hostKey(keyCode(ebiten.KeyNumpad5), modifiers{}); ok {
		t.Error("表外的鍵不該送")
	}
}

func TestMouseInside(t *testing.T) {
	if !mouseInside(639, 479, 640, 480) {
		t.Error("(639, 479) 應在畫面內")
	}
	for _, p := range [][2]int{{640, 0}, {0, 480}, {-1, 0}} {
		if mouseInside(p[0], p[1], 640, 480) {
			t.Errorf("%v 應在畫面外", p)
		}
	}
}
