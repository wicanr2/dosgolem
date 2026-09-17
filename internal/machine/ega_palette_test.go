package machine

import "testing"

// `docs/spec/194`：200 線 EGA 模式設模式時的屬性暫存器與 DAC 預設值。

// TestMode0DAttributeDefaults 釘住 §2.1：8–15 帶 bit 4，不是 identity。
// 寫成 identity 的話 8–15 的 bit 3 在 RGBI 螢幕上不接，高亮色全部變成低亮度。
func TestMode0DAttributeDefaults(t *testing.T) {
	for _, mode := range []uint8{0x0D, 0x0E} {
		m := New()
		m.SetVideoMode(mode)
		for i := 0; i < 16; i++ {
			want := uint8(i)
			if i >= 8 {
				want = uint8(i-8) | 0x10
			}
			if got := m.VGA.Pal(i); got != want {
				t.Errorf("mode %02Xh 屬性暫存器 %d = %02X，要 %02X", mode, i, got, want)
			}
		}
		if m.VGA.ac[0x10] != 0x01 || m.VGA.ac[0x12] != 0x0F {
			t.Errorf("mode %02Xh mode control／plane enable = %02X／%02X", mode, m.VGA.ac[0x10], m.VGA.ac[0x12])
		}
	}
}

// TestMode0DDACAllEntries 對 §2.2 逐筆驗 DAC 0–63，並確認 64 起沒動。
// 期望值直接照表列出，不重用 rgbiDAC，避免拿實作驗實作。
func TestMode0DDACAllEntries(t *testing.T) {
	low := [8][3]uint8{
		{0x00, 0x00, 0x00}, {0x00, 0x00, 0x2A}, {0x00, 0x2A, 0x00}, {0x00, 0x2A, 0x2A},
		{0x2A, 0x00, 0x00}, {0x2A, 0x00, 0x2A}, {0x2A, 0x15, 0x00}, {0x2A, 0x2A, 0x2A},
	}
	high := [8][3]uint8{
		{0x15, 0x15, 0x15}, {0x15, 0x15, 0x3F}, {0x15, 0x3F, 0x15}, {0x15, 0x3F, 0x3F},
		{0x3F, 0x15, 0x15}, {0x3F, 0x15, 0x3F}, {0x3F, 0x3F, 0x15}, {0x3F, 0x3F, 0x3F},
	}
	m := New()
	m.DAC[64*3] = 0x11 // 64 起不能被動到
	m.SetVideoMode(0x0D)
	for i := 0; i < 64; i++ {
		want := low[i&7]
		if i&0x10 != 0 {
			want = high[i&7]
		}
		got := [3]uint8{m.DAC[i*3], m.DAC[i*3+1], m.DAC[i*3+2]}
		if got != want {
			t.Errorf("DAC[%02X] = %02X，要 %02X", i, got, want)
		}
	}
	if m.DAC[64*3] != 0x11 {
		t.Errorf("DAC[40] 被改成 %02X", m.DAC[64*3])
	}
}

// TestOtherPlanarModesKeepDAC 是回歸：350 線 EGA 與 VGA 平面模式不載入 RGBI 表（§3）。
func TestOtherPlanarModesKeepDAC(t *testing.T) {
	for _, mode := range []uint8{0x10, 0x12} {
		m := New()
		m.SetVideoMode(mode)
		for i := 0; i < 64*3; i++ {
			if m.DAC[i] != 0 {
				t.Fatalf("mode %02Xh 設模式後 DAC byte %d = %02X，應該不動", mode, i, m.DAC[i])
			}
		}
		if m.VGA.Pal(9) != 9 {
			t.Errorf("mode %02Xh 屬性暫存器 9 = %02X，應維持 identity", mode, m.VGA.Pal(9))
		}
	}
}

// TestMode0DColorChainEndToEnd 是 §4 第 3 項：只設屬性暫存器（EGA 程式的做法）就要得到正確顏色。
// 數值取自《銀河超能力戰記》淡入完成時寫入的暫存器值（`BH=77h`、`76h`）。
func TestMode0DColorChainEndToEnd(t *testing.T) {
	m := New()
	m.SetVideoMode(0x0D)
	m.VGA.SetPal(15, 0x77&0x3F)
	m.VGA.SetPal(10, 0x76&0x3F)
	pal := m.Palette()
	cases := []struct {
		px   uint8
		want [3]uint8
		name string
	}{
		{15, [3]uint8{0xFF, 0xFF, 0xFF}, "白"},
		{10, [3]uint8{0xFF, 0xFF, 0x55}, "黃"},
		{8, [3]uint8{0x55, 0x55, 0x55}, "深灰（預設）"},
		{6, [3]uint8{0xAA, 0x55, 0x00}, "棕（預設）"},
	}
	for _, c := range cases {
		if got := pal[m.VGA.DACIndex(c.px)]; got != c.want {
			t.Errorf("色號 %d（%s）→ %02X，要 %02X", c.px, c.name, got, c.want)
		}
	}
}
