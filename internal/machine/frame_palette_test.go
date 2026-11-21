package machine

import "testing"

// TestFramePalettePlanarMapsAttributes 釘住 FramePalette 在平面模式把 attribute 色號
// 經屬性控制器對到 DAC：Indexed() 的值直接查這張表就是畫面顏色。
// 期望值用 EGA 預設色（docs/spec/194 §2.2）直接列出，不重用 DACIndex。
func TestFramePalettePlanarMapsAttributes(t *testing.T) {
	m := New()
	m.SetVideoMode(0x0D)
	pal := m.FramePalette()
	want := map[int][3]uint8{
		7:  {0xAA, 0xAA, 0xAA},
		8:  {0x55, 0x55, 0x55},
		10: {0x55, 0xFF, 0x55},
		14: {0xFF, 0xFF, 0x55},
		15: {0xFF, 0xFF, 0xFF},
	}
	for i, w := range want {
		if pal[i] != w {
			t.Errorf("mode 0Dh FramePalette[%d] = %v，要 %v", i, pal[i], w)
		}
	}
	raw := m.Palette()
	for i := 16; i < 256; i++ {
		if pal[i] != raw[i] {
			t.Fatalf("FramePalette[%d] 不該動：%v／%v", i, pal[i], raw[i])
		}
	}
}

// TestFramePaletteMode13IsPalette 確認非平面模式原樣回 Palette()。
func TestFramePaletteMode13IsPalette(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	m.DAC[15*3] = 0x10
	if m.FramePalette() != m.Palette() {
		t.Fatal("mode 13h 的 FramePalette 應等於 Palette")
	}
}
