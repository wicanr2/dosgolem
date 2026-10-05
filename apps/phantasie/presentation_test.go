package phantasie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func artFixture(t *testing.T) (string, string, ArtProfile) {
	t.Helper()
	root, dir := t.TempDir(), t.TempDir()
	raw := make([]byte, 16384)
	paint := image.NewRGBA(image.Rect(0, 0, 320, 184))
	for y := 0; y < 184; y++ {
		for x := 0; x < 320; x++ {
			paint.SetRGBA(x, y, color.RGBA{80, 90, 100, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, paint); err != nil {
		t.Fatal(err)
	}
	hash := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	p := ArtProfile{Schema: 1, SourceFile: "source.bin", SourceBytes: 16384, SourceSHA256: hash(raw), SourceRect: [4]int{0, 8, 320, 184}, Image: "paint.png", ImageBytes: int64(b.Len()), ImageSHA256: hash(b.Bytes())}
	for name, data := range map[string][]byte{filepath.Join(root, p.SourceFile): raw, filepath.Join(dir, p.Image): b.Bytes()} {
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeArtProfile(t, dir, p)
	return root, dir, p
}

func writeArtProfile(t *testing.T, dir string, p ArtProfile) {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "profile.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPresentationWindowAndBorder(t *testing.T) {
	root, dir, _ := artFixture(t)
	a, err := LoadTownArt(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	idx, rgb := make([]byte, 320*200), make([]byte, 320*200*3)
	idx[50*320+20] = 1
	idx[100*320+299] = 1
	rgb[(50*320+20)*3] = 255
	rgb[(100*320+299)*3] = 255
	protected, ok := a.Match(idx)
	if !ok || protected != image.Rect(20, 50, 300, 101) {
		t.Fatalf("window: %v %v", protected, ok)
	}
	dst := make([]byte, 640*400*4)
	ComposePresentationInto(dst, nil, idx, rgb, "hd", a)
	for _, point := range []image.Point{{20, 50}, {160, 75}, {299, 100}} {
		i := (point.Y*2*640 + point.X*2) * 4
		j := (point.Y*320 + point.X) * 3
		if !bytes.Equal(dst[i:i+3], rgb[j:j+3]) {
			t.Fatalf("protected blank/window changed: %v", point)
		}
	}
	if !bytes.Equal(dst[(20*640+2)*4:(20*640+2)*4+4], []byte{80, 90, 100, 255}) {
		t.Fatal("no painted background")
	}
	idx[8*320+150] = 1
	if _, ok := a.Match(idx); ok {
		t.Fatal("changed border accepted")
	}
	original := make([]byte, len(dst))
	ComposeInto(original, nil, idx, rgb, 2, nil)
	ComposePresentationInto(dst, nil, idx, rgb, "hd", a)
	if !bytes.Equal(dst, original) {
		t.Fatal("fallback changed original")
	}
	ComposePresentationInto(dst, nil, idx, rgb, "original", a)
	if !bytes.Equal(dst, original) {
		t.Fatal("original differs")
	}
	ComposePresentationInto(dst, nil, idx, rgb, "amber", a)
	i := (100*640 + 40) * 4
	if !bytes.Equal(dst[i:i+4], []byte{76, 57, 19, 255}) {
		t.Fatalf("amber red: %v", dst[i:i+4])
	}
	if NextTheme("original", nil) != "amber" || NextTheme("amber", nil) != "original" || NextTheme("amber", a) != "hd" || NextTheme("hd", a) != "original" {
		t.Fatal("theme cycle")
	}
}

func TestPresentationRejectsInvalidAssets(t *testing.T) {
	for _, kind := range []string{"path", "hash", "source", "rectangle", "unknown", "extra", "symlink", "image"} {
		t.Run(kind, func(t *testing.T) {
			root, dir, p := artFixture(t)
			switch kind {
			case "path":
				p.Image = "../paint.png"
				writeArtProfile(t, dir, p)
			case "hash":
				p.ImageSHA256 = ""
				writeArtProfile(t, dir, p)
			case "source":
				os.WriteFile(filepath.Join(root, p.SourceFile), []byte{1}, 0600)
			case "rectangle":
				p.SourceRect = [4]int{0, 8, 320, 200}
				writeArtProfile(t, dir, p)
			case "unknown":
				b, _ := json.Marshal(p)
				b = append(b[:len(b)-1], []byte(",\"unknown\":true}")...)
				os.WriteFile(filepath.Join(dir, "profile.json"), b, 0600)
			case "extra":
				b, _ := json.Marshal(p)
				os.WriteFile(filepath.Join(dir, "profile.json"), append(b, []byte("{}")...), 0600)
			case "symlink":
				os.Rename(filepath.Join(dir, p.Image), filepath.Join(dir, "target"))
				os.Symlink("target", filepath.Join(dir, p.Image))
			case "image":
				os.WriteFile(filepath.Join(dir, p.Image), []byte("not PNG"), 0600)
			}
			if _, err := LoadTownArt(root, dir); err == nil {
				t.Fatal("accepted invalid asset")
			}
		})
	}
}
