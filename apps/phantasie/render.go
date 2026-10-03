package phantasie

import (
	"image"
	"image/png"
	"os"
	"sort"

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

// VisibleHash 是疊字的可見格集合的雜湊（docs/spec/004 §7 第 3 項：語言切換前後可見格集合不變）：
// 每筆疊字的非透明格換算成像素範圍 (Y, X0, X1)，同一 Y 相鄰範圍合併後雜湊。與 Key、字面、顏色無關。
func (o *Overlay) VisibleHash() uint64 {
	type seg struct{ y, x0, x1 int }
	var rs []seg
	for _, s := range o.Layer.Stamps {
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			rs = append(rs, seg{s.Y, s.X + i*s.CellW, s.X + (i+1)*s.CellW})
		}
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].y != rs[j].y {
			return rs[i].y < rs[j].y
		}
		return rs[i].x0 < rs[j].x0
	})
	var merged []seg
	for _, r := range rs {
		if n := len(merged); n > 0 && merged[n-1].y == r.y && r.x0 <= merged[n-1].x1 {
			if r.x1 > merged[n-1].x1 {
				merged[n-1].x1 = r.x1
			}
			continue
		}
		merged = append(merged, r)
	}
	var b []byte
	for _, r := range merged {
		b = append(b, byte(r.y), byte(r.y>>8), byte(r.x0), byte(r.x0>>8), byte(r.x1), byte(r.x1>>8))
	}
	return fnv64(b)
}
