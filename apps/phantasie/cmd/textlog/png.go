package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// cgaPalettes 是 CGA 模式 4 的兩組標準調色盤（背景為黑）：
// 0 = 綠、紅、棕；1 = 青、洋紅、白；後面加 h 是高亮度。
var cgaPalettes = map[string][4]color.RGBA{
	"0":  {{0, 0, 0, 255}, {0, 170, 0, 255}, {170, 0, 0, 255}, {170, 85, 0, 255}},
	"0h": {{0, 0, 0, 255}, {85, 255, 85, 255}, {255, 85, 85, 255}, {255, 255, 85, 255}},
	"1":  {{0, 0, 0, 255}, {0, 170, 170, 255}, {170, 0, 170, 255}, {170, 170, 170, 255}},
	"1h": {{0, 0, 0, 255}, {85, 255, 255, 255}, {255, 85, 255, 255}, {255, 255, 255, 255}},
}

// writePNG 把 320×200 的色號陣列放大 scale 倍存成 PNG。
func writePNG(path string, idx []uint8, pal string, scale int) error {
	p, ok := cgaPalettes[pal]
	if !ok {
		p = cgaPalettes["1h"]
	}
	img := image.NewRGBA(image.Rect(0, 0, 320*scale, 200*scale))
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < 320*scale; x++ {
			img.SetRGBA(x, y, p[idx[(y/scale)*320+x/scale]&3])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
