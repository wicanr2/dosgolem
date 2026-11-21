package dkk

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// writeTheme 在 dir 寫一項 16×8、位於 (8,16) 的 3× 主題；HD 圖每像素同色 (200,10,20)。
func writeTheme(t *testing.T, dir string, ref []byte, pngW int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, pngW, 24))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 200, 10, 20, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	sum := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "a.png"), buf.Bytes(), 0o644))
	must(os.WriteFile(filepath.Join(dir, "a.idx"), ref, 0o644))
	j := fmt.Sprintf(`{"scale":3,"items":[{"key":"a","screen_rect":[8,16,16,8],"png":"a.png","png_sha256":%q,"indexed":"a.idx","indexed_sha256":%q}]}`,
		sum(buf.Bytes()), sum(ref))
	must(os.WriteFile(filepath.Join(dir, "theme.json"), []byte(j), 0o644))
}

func TestHDTheme(t *testing.T) {
	ref := make([]byte, 16*8)
	for i := range ref {
		ref[i] = byte(i % 16)
	}
	dir := t.TempDir()
	writeTheme(t, dir, ref, 48)
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(i), uint8(i)}
	}
	screen := make([]byte, 320*200)
	for y := 0; y < 8; y++ {
		copy(screen[(16+y)*320+8:], ref[y*16:(y+1)*16])
	}
	live := NewLive(&xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, nil, nil)
	if err := live.AddHD(dir); err != nil {
		t.Fatal(err)
	}
	// 3×：矩形內換成 HD，矩形外不動
	at := func(out []byte, scale, x, y int) [3]byte {
		o := (y*320*scale + x) * 4
		return [3]byte{out[o], out[o+1], out[o+2]}
	}
	live.Frame(screen, pal)
	out := live.Compose(3, true, nil)
	if got := at(out, 3, 8*3, 16*3); got != [3]byte{200, 10, 20} {
		t.Fatalf("3× 矩形內 %v", got)
	}
	if got := at(out, 3, 8*3-1, 16*3); got != [3]byte{0, 0, 0} {
		t.Fatalf("3× 矩形外 %v", got)
	}
	// 2×：不畫 HD
	if got := at(live.Compose(2, true, nil), 2, 8*2+2, 16*2); got != [3]byte{1, 1, 1} {
		t.Fatalf("2× 應為原版 %v", got)
	}
	// 原版檢視：不畫 HD
	if got := at(live.Compose(3, false, nil), 3, 8*3+3, 16*3); got != [3]byte{1, 1, 1} {
		t.Fatalf("原版檢視 %v", got)
	}
	// 整張不吻合：整組停用
	screen[16*320+8] = 15
	live.Frame(screen, pal)
	if got := at(live.Compose(3, true, nil), 3, 23*3, 23*3); got == [3]byte{200, 10, 20} {
		t.Fatal("基準不吻合仍畫 HD")
	}
}

func TestHDThemeRejects(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, make([]byte, 16*8), 47) // 寬應為 48
	if _, err := LoadHDTheme(dir); err == nil {
		t.Fatal("HD 圖尺寸不合應回錯")
	}
	dir = t.TempDir()
	writeTheme(t, dir, make([]byte, 16*8), 48)
	if err := os.WriteFile(filepath.Join(dir, "a.idx"), make([]byte, 16*8+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadHDTheme(dir); err == nil {
		t.Fatal("基準雜湊不合應回錯")
	}
}
