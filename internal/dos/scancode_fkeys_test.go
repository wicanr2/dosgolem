package dos

import "testing"

func TestFunctionAndComboKeys(t *testing.T) {
	for i, want := range []uint16{0x3B00, 0x3C00, 0x3D00, 0x3E00, 0x3F00, 0x4000, 0x4100, 0x4200, 0x4300, 0x4400} {
		k, ok := KeyNamed("F" + string(rune('0'+i+1)))
		if i == 9 {
			k, ok = KeyNamed("F10")
		}
		if !ok || k.Word() != want {
			t.Fatalf("F%d = %04X %v", i+1, k.Word(), ok)
		}
	}
	if k, ok := KeyCtrl('c'); !ok || k.Word() != 0x2E03 {
		t.Fatalf("Ctrl+C %04X", k.Word())
	}
	if k, ok := KeyAlt('X'); !ok || k.Word() != 0x2D00 {
		t.Fatalf("Alt+X %04X", k.Word())
	}
	if _, ok := KeyCtrl('1'); ok {
		t.Fatal("Ctrl+1 不應接受")
	}
	if _, ok := KeyAlt('@'); ok {
		t.Fatal("Alt+@ 不應接受")
	}
}
