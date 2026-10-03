package phantasie

import (
	"image"
	"image/png"
	"os"

	"github.com/wicanr2/dosgolem/oracle"
)

// 畫面合成（docs/spec/005 §3 第 3 步）：原版色號畫面逐點放大，再畫疊字。

// FrameBuffers 取目前原版畫面的色號與 RGB（RGB 依 oracle.CGAPalette()，docs/spec/002 §6）。
func FrameBuffers(o *oracle.Oracle) (indexed, rgb []uint8) {
	return o.CGA4(), o.CGA4RGB()
}

// ComposeImage 組出 scale 倍的 RGBA 畫面。顯示語言為 en 時略過 Layer.Draw。
func ComposeImage(ov *Overlay, indexed, rgb []uint8, scale int, missing func(rune)) *image.RGBA {
	w, h := screenW*scale, screenH*scale
	dst := make([]uint8, w*h*4)
	ComposeInto(dst, ov, indexed, rgb, scale, missing)
	return &image.RGBA{Pix: dst, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
}

// ComposeInto 同 ComposeImage，但寫進呼叫端的緩衝區（長度要有 (320×scale)×(200×scale)×4），供前端每幀重用。
func ComposeInto(dst []uint8, ov *Overlay, indexed, rgb []uint8, scale int, missing func(rune)) {
	w, h := screenW*scale, screenH*scale
	for y := 0; y < h; y++ {
		sy := y / scale
		for x := 0; x < w; x++ {
			i := 3 * (sy*screenW + x/scale)
			o := (y*w + x) * 4
			dst[o], dst[o+1], dst[o+2], dst[o+3] = rgb[i], rgb[i+1], rgb[i+2], 255
		}
	}
	if ov != nil && ov.Drawing() {
		ov.Layer.Draw(dst, scale, missing)
	}
}

// WritePNG 把影像存成 PNG。
func WritePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// MemHash 是映像段起至 DGROUP 末端（dg:FFFF）的 FNV-1a 64 雜湊（docs/spec/005 §5 的 mem_hash）。
func MemHash(o *oracle.Oracle, img, dg uint16) uint64 {
	n := int(dg-img)*16 + 0x10000
	return fnv64(o.Bytes(oracle.Far(img, 0), n))
}

// VramHash 是 B800:0000 起 4000h bytes 的 FNV-1a 64 雜湊（vram_hash）。
func VramHash(o *oracle.Oracle) uint64 {
	return fnv64(o.Bytes(oracle.Far(0xB800, 0), pageBytes))
}

// LayerHash 是疊字集合的雜湊（005 §5 的 layer_hash）：Key、位置、Text、State、FG、BG、Transparent 全部納入，
// 使整列反色與透明格遺失會改變雜湊。
func (o *Overlay) LayerHash() uint64 {
	var b []byte
	put := func(s string) { b = append(b, s...); b = append(b, 0) }
	for _, s := range o.Layer.Stamps {
		put(s.Key)
		put(string(s.Text))
		b = append(b, byte(s.X), byte(s.X>>8), byte(s.Y), byte(s.Y>>8), byte(s.Cells), byte(s.CellW), byte(s.State))
		b = append(b, s.FG[:]...)
		b = append(b, s.BG[:]...)
		for i := 0; i < s.Cells; i++ {
			t := byte(0)
			if i < len(s.Transparent) && s.Transparent[i] {
				t = 1
			}
			b = append(b, t)
		}
	}
	return fnv64(b)
}
