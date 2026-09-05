package oracle

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// WritePNG 把目前的畫面存成 PNG（色號配上目前的調色盤）。
//
// **mode 13h 的 PNG 是給人看的，不是驗收用的**：逐點比對一律在色號空間做
// （`docs/spec/005` §3.3）——PNG 經過調色盤，而調色盤有循環動畫。
//
// ⚠ **平面模式相反。** 那邊的色號只有 4 bit，還要經過屬性控制器才變成
// DAC 索引；PNG 是把整條鏈算完的結果，反而是可以直接跟原版截圖比的東西。
func (o *Oracle) WritePNG(path string) error {
	return o.WritePNGCrop(path, 0, 0)
}

// WritePNGCrop 存 PNG，並從上面裁掉 top 列、只留 height 列（0 ＝ 到底）。
//
// ⚠ **裁切是呼叫端的事。** 《臥龍傳》的內容是 640×400，但它跑在 640×480 的
// mode 12h 上、y 原點在第 40 列（`docs/spec/007` §2）。把裁切寫進取畫面的
// 路徑會讓「畫面其實是 480 高」這個事實消失，之後想問「上面那 40 列有沒有
// 東西」就沒得問了。
func (o *Oracle) WritePNGCrop(path string, top, height int) error {
	w, h, px := o.Screen()
	if height == 0 {
		height = h - top
	}
	if top < 0 || height <= 0 || top+height > h {
		return fmt.Errorf("裁切範圍 %d..%d 超出畫面高 %d", top, top+height, h)
	}
	pal := o.Palette()
	p := make(color.Palette, 256)
	for i := range pal {
		p[i] = color.RGBA{pal[i][0], pal[i][1], pal[i][2], 255}
	}
	// 平面模式的像素值要經屬性控制器才是 DAC 索引；mode 13h 直接就是。
	toDAC := func(v uint8) uint8 { return v }
	if pw, _ := o.m.PlanarSize(); pw != 0 {
		toDAC = o.m.VGA.DACIndex
	}
	img := image.NewPaletted(image.Rect(0, 0, w, height), p)
	for y := 0; y < height; y++ {
		for x := 0; x < w; x++ {
			img.Pix[y*img.Stride+x] = toDAC(px[(y+top)*w+x])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
