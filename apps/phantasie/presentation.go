package phantasie

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ArtProfile describes an optional, read-only original page and replacement.
// The calling project supplies identities; this renderer has no asset names.
type ArtProfile struct {
	Schema       int    `json:"schema"`
	SourceFile   string `json:"source_file"`
	SourceBytes  int64  `json:"source_bytes"`
	SourceSHA256 string `json:"source_sha256"`
	SourceRect   [4]int `json:"source_rect"`
	Image        string `json:"image"`
	ImageBytes   int64  `json:"image_bytes"`
	ImageSHA256  string `json:"image_sha256"`
}

type TownArt struct {
	rect     image.Rectangle
	source   []byte
	paint    *image.RGBA
	pages    []*TownArt
	preserve []image.Rectangle
	sprites  []artSprite
	anchors  map[uint32][]int
}

func artBaseName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\:\x00") && filepath.Base(name) == name
}

func artRead(path string, max int64) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	for p := abs; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("art path contains symlink")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > max {
		return nil, fmt.Errorf("art file size or type")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) != info.Size() {
		return nil, fmt.Errorf("art file changed")
	}
	return b, nil
}

func artIdentity(data []byte, size int64, hash string) bool {
	if len(hash) != 64 || hash != strings.ToLower(hash) || int64(len(data)) != size {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == hash
}

// LoadTownArt verifies the complete group before exposing any replacement.
func LoadTownArt(root, dir string) (*TownArt, error) {
	b, err := artRead(filepath.Join(dir, "profile.json"), 64*1024)
	if err != nil {
		return nil, err
	}
	if err := uniqueArtJSON(b); err != nil {
		return nil, err
	}
	var header struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(b, &header); err != nil {
		return nil, err
	}
	if header.Schema == 2 {
		return loadArtCollection(root, dir, b)
	}
	var p ArtProfile
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return nil, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("extra art JSON")
	}
	if p.Schema != 1 || !artBaseName(p.SourceFile) || !artBaseName(p.Image) || p.Image == "profile.json" || p.SourceBytes != 16384 {
		return nil, fmt.Errorf("art profile identity")
	}
	x, y, w, h := p.SourceRect[0], p.SourceRect[1], p.SourceRect[2], p.SourceRect[3]
	if x < 0 || y < 0 || w < 9 || h < 9 || w > 320 || h > 200 || x > 320-w || y > 200-h {
		return nil, fmt.Errorf("art rectangle")
	}
	raw, err := artRead(filepath.Join(root, p.SourceFile), 16384)
	if err != nil {
		return nil, err
	}
	if !artIdentity(raw, p.SourceBytes, p.SourceSHA256) {
		return nil, fmt.Errorf("art source identity")
	}
	b, err = artRead(filepath.Join(dir, p.Image), 16*1024*1024)
	if err != nil {
		return nil, err
	}
	if !artIdentity(b, p.ImageBytes, p.ImageSHA256) {
		return nil, fmt.Errorf("art image identity")
	}
	cfg, err := png.DecodeConfig(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 4_000_000 || math.Abs(float64(cfg.Width*h)/float64(cfg.Height*w)-1) > 0.005 {
		return nil, fmt.Errorf("art image dimensions")
	}
	img, err := png.Decode(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	a := &TownArt{rect: image.Rect(x, y, x+w, y+h), source: make([]byte, 320*200), paint: image.NewRGBA(image.Rect(0, 0, w*2, h*2))}
	for sy := 0; sy < 200; sy++ {
		for sx := 0; sx < 320; sx++ {
			a.source[sy*320+sx] = raw[(sy%2)*8192+(sy/2)*80+sx/4] >> uint(6-2*(sx%4)) & 3
		}
	}
	for py := 0; py < h*2; py++ {
		for px := 0; px < w*2; px++ {
			a.paint.Set(px, py, img.At(px*cfg.Width/(w*2), py*cfg.Height/(h*2)))
		}
	}
	return a, nil
}

// Match returns the full rectangle that must remain original, including blank
// window pixels. A single changed border pixel rejects the entire replacement.
func (a *TownArt) Match(indexed []byte) (image.Rectangle, bool) {
	if a == nil || len(indexed) != 320*200 {
		return image.Rectangle{}, false
	}
	r := a.rect
	protected := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := y*320 + x
			if indexed[i] == a.source[i] {
				continue
			}
			if x < r.Min.X+4 || x >= r.Max.X-4 || y < r.Min.Y+4 || y >= r.Max.Y-4 {
				return image.Rectangle{}, false
			}
			protected = protected.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return protected, true
}

// ComposePresentationInto only changes the caller's display buffer. Original
// frames, overlay invalidation, guest memory and guest input remain untouched.
func ComposePresentationInto(dst []byte, ov *Overlay, indexed, rgb []byte, theme string, art *TownArt) {
	if theme != "hd" || art == nil {
		ComposeInto(dst, ov, indexed, rgb, 2, nil)
	} else {
		ComposeInto(dst, nil, indexed, rgb, 2, nil)
		art.draw(dst, indexed, rgb)
		if ov != nil && ov.Drawing() {
			ov.Layer.Draw(dst, 2, nil)
		}
	}
	if theme == "amber" {
		for i := 0; i < len(dst); i += 4 {
			l := (77*int(dst[i]) + 150*int(dst[i+1]) + 29*int(dst[i+2])) >> 8
			dst[i], dst[i+1], dst[i+2] = byte(l), byte(3*l/4), byte(l/4)
		}
	}
}

func NextTheme(current string, art *TownArt) string {
	if current == "original" {
		return "amber"
	}
	if current == "amber" && art != nil {
		return "hd"
	}
	return "original"
}
