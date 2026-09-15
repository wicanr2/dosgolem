package machine

import "testing"

// TestHerculesSeesB0000 是「有畫面不得回假零」的正對照：往 `B0000` 寫
// 東西之後，非零計數與解碼後的像素都要看得到它。沒寫 CRTC 時是標準的
// 720×348、每列 90 bytes。
func TestHerculesSeesB0000(t *testing.T) {
	m := New()
	if got := m.HerculesNonZero(); got != 0 {
		t.Fatalf("空機器的 B0000 非零位元組應為 0，得到 %d", got)
	}
	w, h, stride := m.HerculesGeometry()
	if w != 720 || h != 348 || stride != 90 {
		t.Fatalf("預設幾何應為 720×348／90，得到 %d×%d／%d", w, h, stride)
	}
	// 第 5 列（bank 1、第 1 組）最左邊的位元組：第 5 列第 0 個像素。
	m.Mem[HerculesBase+1*herculesBank+1*90] = 0x80
	// 第 347 列（bank 3、第 86 組）最右邊的位元組：第 347 列第 719 個像素。
	m.Mem[HerculesBase+3*herculesBank+86*90+89] = 0x01
	if got := m.HerculesNonZero(); got != 2 {
		t.Fatalf("B0000 非零位元組應為 2，得到 %d", got)
	}
	px := m.Hercules()
	if len(px) != 720*348 {
		t.Fatalf("解出 %d 個像素，應為 %d", len(px), 720*348)
	}
	lit := 0
	for _, p := range px {
		lit += int(p)
	}
	if lit != 2 {
		t.Fatalf("亮點應為 2，得到 %d", lit)
	}
	if px[5*720+0] != 1 || px[347*720+719] != 1 {
		t.Fatalf("亮點落錯位置：(0,5)=%d (719,347)=%d", px[5*720+0], px[347*720+719])
	}
}

// TestHerculesGeometryFollowsCRTC：程式自己寫 6845 就換幾何——三國演義
// 寫 R1＝40、R6＝102、R9＝3（640×408、每列 80 bytes），模式埠 bit 7 選頁 1。
func TestHerculesGeometryFollowsCRTC(t *testing.T) {
	m := New()
	for idx, v := range map[uint8]uint8{1: 40, 6: 102, 9: 3} {
		m.Out8(0x3B4, idx)
		m.Out8(0x3B5, v)
	}
	w, h, stride := m.HerculesGeometry()
	if w != 640 || h != 408 || stride != 80 {
		t.Fatalf("CRTC 給的幾何應為 640×408／80，得到 %d×%d／%d", w, h, stride)
	}
	// 第 407 列（bank 3、第 101 組）最右邊的位元組：第 407 列第 639 個像素。
	m.Mem[HerculesBase+3*herculesBank+101*80+79] = 0x01
	px := m.Hercules()
	if len(px) != 640*408 || px[407*640+639] != 1 {
		t.Fatalf("640×408 的最後一個像素沒解到（len=%d）", len(px))
	}
	// 翻到頁 1：同一個位置要從 B8000 那一頁讀。
	m.Out8(0x3B8, 0x8a)
	if m.HerculesPageBase() != HerculesBase+HerculesPage {
		t.Fatalf("3B8h bit 7 應選頁 1")
	}
	if got := m.Hercules()[407*640+639]; got != 0 {
		t.Fatalf("頁 1 是空的，卻解到頁 0 的亮點")
	}
	m.Mem[HerculesBase+HerculesPage+3*herculesBank+101*80+79] = 0x01
	if got := m.Hercules()[407*640+639]; got != 1 {
		t.Fatalf("頁 1 的亮點沒解到")
	}
	// 非零計數固定看頁 0，不隨翻頁變。
	if got := m.HerculesNonZero(); got != 1 {
		t.Fatalf("頁 0 非零位元組應為 1，得到 %d", got)
	}
}

// TestHerculesIgnoresEGAAndCGA 是不退步的那一面：寫進 `A0000`（EGA／VGA）
// 與 `B8000`（CGA／文字）都不會被算成 Hercules 頁 0 的畫面。
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
