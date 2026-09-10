package machine

import "testing"

func TestBIOSLowResolutionPlanarPalette(t *testing.T) {
	for _, mode := range []uint8{0x0D, 0x0E} {
		m := New()
		m.SetVideoMode(mode)
		for x, c := range []uint8{6, 8, 15} {
			m.PlanarPutPixel(0, 0x80>>x, c)
		}
		_, _, rgb := m.PlanarRGB()
		want := []uint8{170, 85, 0, 85, 85, 85, 255, 255, 255}
		for i, c := range want {
			if rgb[i] != c {
				t.Fatalf("模式 %02X 的 RGB[%d]=%d，要 %d", mode, i, rgb[i], c)
			}
		}
		// 真正的 DAC 埠改寫仍優先，不以固定 RGB 查表掩蓋。
		m.Out8(0x3C8, 6)
		m.Out8(0x3C9, 63)
		m.Out8(0x3C9, 0)
		m.Out8(0x3C9, 0)
		_, _, rgb = m.PlanarRGB()
		if rgb[0] != 255 || rgb[1] != 0 || rgb[2] != 0 {
			t.Fatal("DAC 改寫未反映到 RGB")
		}
		m.SetVideoMode(mode)
		if m.DAC[18] != 42 || m.DAC[19] != 21 {
			t.Fatal("重新設模式未恢復色盤")
		}
	}
}

func TestBIOSPaletteLeavesOtherModesAlone(t *testing.T) {
	m := New()
	m.DAC[0] = 17
	m.SetVideoMode(0x13)
	if m.DAC[0] != 17 {
		t.Fatal("本切片不應改寫 mode 13h 色盤")
	}
}
