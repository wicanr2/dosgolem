package main

import (
	"image/color"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// 畫面轉換與 cmd/webplay 相同（`docs/spec/200-webplay` §3、`docs/spec/441-play-frontend` §2.3）。

// screenRGB 回目前畫面：平面模式、mode 13h，其餘當文字模式畫（`docs/spec/200-webplay` §3）。
func screenRGB(m *machine.Machine) (int, int, []uint8) {
	if w, h, rgb := m.PlanarRGB(); rgb != nil {
		return w, h, rgb
	}
	if m.VideoMode() == 0x13 {
		w, h := 320, 200
		raw, pal := m.VideoRaw(), m.Palette()
		rgb := make([]uint8, w*h*3)
		for i := 0; i < w*h && i < len(raw); i++ {
			c := pal[raw[i]]
			rgb[i*3], rgb[i*3+1], rgb[i*3+2] = c[0], c[1], c[2]
		}
		return w, h, rgb
	}
	return textRGB(m)
}

// cga16 是文字模式用的固定 16 色（簡化：不走屬性調色盤與 DAC）。
var cga16 = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 170, 255}, {0, 170, 0, 255}, {0, 170, 170, 255},
	{170, 0, 0, 255}, {170, 0, 170, 255}, {170, 85, 0, 255}, {170, 170, 170, 255},
	{85, 85, 85, 255}, {85, 85, 255, 255}, {85, 255, 85, 255}, {85, 255, 255, 255},
	{255, 85, 85, 255}, {255, 85, 255, 255}, {255, 255, 85, 255}, {255, 255, 255, 255},
}

// textRGB 把 B800h 的 80×25 字元以 F000:FA6E 的 8×8 字型畫成 640×200。
func textRGB(m *machine.Machine) (int, int, []uint8) {
	const cols, rows, font = 80, 25, 0xFFA6E
	w, h := cols*8, rows*8
	rgb := make([]uint8, w*h*3)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			a := uint32(0xB8000 + (r*cols+c)*2)
			ch, attr := m.Read8(a), m.Read8(a+1)
			fg, bg := cga16[attr&15], cga16[attr>>4&7]
			for y := 0; y < 8; y++ {
				bits := uint8(0)
				if ch < 128 {
					bits = m.Read8(uint32(font + int(ch)*8 + y))
				}
				for x := 0; x < 8; x++ {
					p := bg
					if bits&(0x80>>x) != 0 {
						p = fg
					}
					i := ((r*8+y)*w + c*8 + x) * 3
					rgb[i], rgb[i+1], rgb[i+2] = p.R, p.G, p.B
				}
			}
		}
	}
	return w, h, rgb
}
