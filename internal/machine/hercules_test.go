package machine

import "testing"

// TestHerculesSeesB0000 是「有畫面不得回假零」的正對照：往 `B0000` 寫
// 東西之後，非零計數與解碼後的像素都要看得到它。
func TestHerculesSeesB0000(t *testing.T) {
	m := New()
	if got := m.HerculesNonZero(); got != 0 {
		t.Fatalf("空機器的 B0000 非零位元組應為 0，得到 %d", got)
	}
	// 第 5 列（bank 1、第 1 組）最左邊的位元組：第 5 列第 0 個像素。
	m.Mem[HerculesBase+1*herculesBank+1*herculesStride] = 0x80
	// 第 347 列（bank 3、第 86 組）最右邊的位元組：第 347 列第 719 個像素。
	m.Mem[HerculesBase+3*herculesBank+86*herculesStride+89] = 0x01
	if got := m.HerculesNonZero(); got != 2 {
		t.Fatalf("B0000 非零位元組應為 2，得到 %d", got)
	}
	px := m.Hercules()
	if len(px) != HerculesWidth*HerculesHeight {
		t.Fatalf("解出 %d 個像素，應為 %d", len(px), HerculesWidth*HerculesHeight)
	}
	lit := 0
	for _, p := range px {
		lit += int(p)
	}
	if lit != 2 {
		t.Fatalf("亮點應為 2，得到 %d", lit)
	}
	if px[5*HerculesWidth+0] != 1 || px[347*HerculesWidth+719] != 1 {
		t.Fatalf("亮點落錯位置：(0,5)=%d (719,347)=%d",
			px[5*HerculesWidth+0], px[347*HerculesWidth+719])
	}
}

// TestHerculesIgnoresEGAAndCGA 是不退步的那一面：寫進 `A0000`（EGA／VGA）
// 與 `B8000`（CGA／文字）都不會被算成 Hercules 的畫面。
func TestHerculesIgnoresEGAAndCGA(t *testing.T) {
	m := New()
	m.Mem[0xA0000] = 0xFF
	m.Mem[0xAFFFF] = 0xFF
	m.Mem[0xB8000] = 0xFF
	m.Mem[0xBFFFF] = 0xFF
	if got := m.HerculesNonZero(); got != 0 {
		t.Fatalf("A0000／B8000 的寫入不該算進 B0000：得到 %d", got)
	}
	for i, p := range m.Hercules() {
		if p != 0 {
			t.Fatalf("像素 %d 亮了，B0000 明明是空的", i)
		}
	}
}
