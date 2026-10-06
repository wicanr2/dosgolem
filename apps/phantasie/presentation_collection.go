package phantasie

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strings"
)

type artSource struct {
	ID           string `json:"id"`
	File         string `json:"file"`
	Bytes        int64  `json:"bytes"`
	SHA256       string `json:"sha256"`
	Encoding     string `json:"encoding"`
	DecodedBytes int    `json:"decoded_bytes"`
}
type artImage struct {
	File   string `json:"file"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type artPageSpec struct {
	Source     string   `json:"source"`
	SourceRect [4]int   `json:"source_rect"`
	Image      string   `json:"image"`
	ImageRect  [4]int   `json:"image_rect"`
	Preserve   [][4]int `json:"preserve"`
}
type artSpriteSpec struct {
	Kind         string `json:"kind,omitempty"`
	Source       string `json:"source"`
	SourceOffset int    `json:"source_offset"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Index        int    `json:"index"`
	State        int    `json:"state"`
	Image        string `json:"image"`
	ImageRect    [4]int `json:"image_rect"`
}
type artCollectionSpec struct {
	Schema  int             `json:"schema"`
	Sources []artSource     `json:"sources"`
	Images  []artImage      `json:"images"`
	Pages   []artPageSpec   `json:"pages"`
	Sprites []artSpriteSpec `json:"sprites"`
}
type artSprite struct {
	kind                        string
	width, height, index, state int
	raw                         []byte
	paint                       *image.RGBA
	anchorRow, anchorByte       int
	anchor                      uint32
	white                       bool
}

// Check object keys before struct decoding, including nested objects.
func uniqueArtJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return fmt.Errorf("art JSON nesting")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if t == nil {
			return fmt.Errorf("art JSON null")
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate art JSON key")
				}
				seen[key] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("art JSON delimiter")
		}
		end, err := d.Token()
		if err != nil {
			return err
		}
		if (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
			return fmt.Errorf("art JSON end")
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("extra art JSON")
	}
	return nil
}

func decodeArtCollection(data []byte) (artCollectionSpec, error) {
	var p artCollectionSpec
	if err := uniqueArtJSON(data); err != nil {
		return p, err
	}
	if err := artCollectionShape(data); err != nil {
		return p, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	maxSprites := 160
	if p.Schema == 3 {
		maxSprites = 192
	}
	if (p.Schema != 2 && p.Schema != 3) || len(p.Sources) < 1 || len(p.Sources) > 8 || len(p.Images) < 1 || len(p.Images) > 32 || len(p.Pages) > 4 || len(p.Sprites) > maxSprites || len(p.Pages)+len(p.Sprites) == 0 {
		return p, fmt.Errorf("art collection limits")
	}
	return p, nil
}

func artCollectionShape(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	fields := func(row map[string]json.RawMessage, names []string) error {
		if len(row) != len(names) {
			return fmt.Errorf("art required fields")
		}
		for _, name := range names {
			if _, ok := row[name]; !ok {
				return fmt.Errorf("art required field")
			}
		}
		return nil
	}
	if err := fields(root, []string{"schema", "sources", "images", "pages", "sprites"}); err != nil {
		return err
	}
	groups := map[string][]string{
		"sources": {"id", "file", "bytes", "sha256", "encoding", "decoded_bytes"},
		"images":  {"file", "bytes", "sha256"},
		"pages":   {"source", "source_rect", "image", "image_rect", "preserve"},
		"sprites": {"source", "source_offset", "width", "height", "index", "state", "image", "image_rect"},
	}
	var schema int
	if err := json.Unmarshal(root["schema"], &schema); err != nil {
		return err
	}
	if schema == 3 {
		groups["sprites"] = append(groups["sprites"], "kind")
	}
	for group, names := range groups {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(root[group], &rows); err != nil {
			return err
		}
		for _, row := range rows {
			if err := fields(row, names); err != nil {
				return err
			}
			for _, key := range []string{"source_rect", "image_rect"} {
				if raw, ok := row[key]; ok {
					var rect []int
					if err := json.Unmarshal(raw, &rect); err != nil {
						return err
					}
					if len(rect) != 4 {
						return fmt.Errorf("art rectangle length")
					}
				}
			}
			if raw, ok := row["preserve"]; ok {
				var rects [][]int
				if err := json.Unmarshal(raw, &rects); err != nil {
					return err
				}
				for _, rect := range rects {
					if len(rect) != 4 {
						return fmt.Errorf("art preserve length")
					}
				}
			}
		}
	}
	return nil
}

// ArtAssetNames lets bundle consumers verify every declared image identity.
func ArtAssetNames(data []byte) ([]string, error) {
	if err := uniqueArtJSON(data); err != nil {
		return nil, err
	}
	var header struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	names := []string{"profile.json"}
	if header.Schema == 1 {
		var p ArtProfile
		d := json.NewDecoder(bytes.NewReader(data))
		d.DisallowUnknownFields()
		if err := d.Decode(&p); err != nil {
			return nil, err
		}
		if !artBaseName(p.Image) || p.Image == "profile.json" {
			return nil, fmt.Errorf("art image name")
		}
		return append(names, p.Image), nil
	}
	p, err := decodeArtCollection(data)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{"profile.json": true}
	for _, img := range p.Images {
		if !artBaseName(img.File) || seen[strings.ToLower(img.File)] {
			return nil, fmt.Errorf("art image name")
		}
		seen[strings.ToLower(img.File)] = true
		names = append(names, img.File)
	}
	return names, nil
}

func decodeArtSource(raw []byte, s artSource) ([]byte, error) {
	if s.DecodedBytes < 1 || s.DecodedBytes > 65536 {
		return nil, fmt.Errorf("art decoded size")
	}
	if s.Encoding == "cga-page" {
		if s.DecodedBytes != 16384 || len(raw) != 16384 {
			return nil, fmt.Errorf("art CGA size")
		}
		return raw, nil
	}
	if s.Encoding == "raw-linear" {
		if len(raw) != s.DecodedBytes {
			return nil, fmt.Errorf("art raw linear size")
		}
		return raw, nil
	}
	if s.Encoding != "zero-rle-page" && s.Encoding != "zero-rle-linear" {
		return nil, fmt.Errorf("art source encoding")
	}
	if s.Encoding == "zero-rle-page" && s.DecodedBytes != 16384 {
		return nil, fmt.Errorf("art page size")
	}
	out := make([]byte, 0, s.DecodedBytes)
	for i := 0; i < len(raw); i++ {
		n := 1
		v := raw[i]
		if v == 0 {
			i++
			if i >= len(raw) {
				return nil, fmt.Errorf("art truncated run")
			}
			n = int(raw[i]) + 1
		}
		if n > s.DecodedBytes-len(out) {
			return nil, fmt.Errorf("art excessive run")
		}
		for j := 0; j < n; j++ {
			out = append(out, v)
		}
	}
	if len(out) != s.DecodedBytes {
		return nil, fmt.Errorf("art incomplete run")
	}
	return out, nil
}
func artRect(v [4]int, bounds image.Rectangle, min int) (image.Rectangle, error) {
	x, y, w, h := v[0], v[1], v[2], v[3]
	if x < bounds.Min.X || y < bounds.Min.Y || w < min || h < min || w > bounds.Dx() || h > bounds.Dy() || x > bounds.Max.X-w || y > bounds.Max.Y-h {
		return image.Rectangle{}, fmt.Errorf("art crop bounds")
	}
	return image.Rect(x, y, x+w, y+h), nil
}
func artCrop(img image.Image, crop [4]int, w, h int, opaque bool) (*image.RGBA, error) {
	r, err := artRect(crop, img.Bounds(), 1)
	if err != nil {
		return nil, err
	}
	if math.Abs(float64(r.Dx()*h)/float64(r.Dy()*w)-1) > 0.005 {
		return nil, fmt.Errorf("art crop aspect")
	}
	if opaque {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a != 65535 {
					return nil, fmt.Errorf("art page alpha")
				}
			}
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, w*2, h*2))
	for y := 0; y < h*2; y++ {
		for x := 0; x < w*2; x++ {
			out.Set(x, y, img.At(r.Min.X+x*r.Dx()/(w*2), r.Min.Y+y*r.Dy()/(h*2)))
		}
	}
	return out, nil
}

func loadArtCollection(root, dir string, data []byte) (*TownArt, error) {
	p, err := decodeArtCollection(data)
	if err != nil {
		return nil, err
	}
	sources := map[string][]byte{}
	encodings := map[string]string{}
	files := map[string]bool{}
	sourceFiles := map[string]artImage{}
	for _, s := range p.Sources {
		if p.Schema == 2 && s.Encoding == "raw-linear" {
			return nil, fmt.Errorf("art schema 2 source encoding")
		}
		if !artBaseName(s.ID) || !artBaseName(s.File) || sources[s.ID] != nil || files[strings.ToLower(s.File)] {
			return nil, fmt.Errorf("art source name")
		}
		raw, err := artRead(filepath.Join(root, s.File), 65536)
		if err != nil {
			return nil, err
		}
		if !artIdentity(raw, s.Bytes, s.SHA256) {
			return nil, fmt.Errorf("art source identity")
		}
		decoded, err := decodeArtSource(raw, s)
		if err != nil {
			return nil, err
		}
		sources[s.ID] = decoded
		encodings[s.ID] = s.Encoding
		files[strings.ToLower(s.File)] = true
		sourceFiles[strings.ToLower(s.File)] = artImage{File: s.File, Bytes: s.Bytes, SHA256: s.SHA256}
	}
	a := &TownArt{anchors: map[uint32][]int{}}
	// Construct source identities before reading images; no partial result escapes.
	for _, page := range p.Pages {
		source := sources[page.Source]
		if len(source) != 16384 || (encodings[page.Source] != "cga-page" && encodings[page.Source] != "zero-rle-page") || len(page.Preserve) > 16 {
			return nil, fmt.Errorf("art page source")
		}
		r, err := artRect(page.SourceRect, image.Rect(0, 0, 320, 200), 9)
		if err != nil {
			return nil, err
		}
		entry := &TownArt{rect: r, source: make([]byte, 320*200)}
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				entry.source[y*320+x] = source[y%2*8192+y/2*80+x/4] >> uint(6-2*(x%4)) & 3
			}
		}
		for _, v := range page.Preserve {
			pr, err := artRect(v, r, 1)
			if err != nil {
				return nil, err
			}
			entry.preserve = append(entry.preserve, pr)
		}
		a.pages = append(a.pages, entry)
	}
	type spriteIdentity struct {
		kind         string
		index, state int
	}
	identities := map[spriteIdentity]bool{}
	bitmaps := map[string]int{}
	for _, spec := range p.Sprites {
		source := sources[spec.Source]
		w, h := spec.Width, spec.Height
		kind := spec.Kind
		if p.Schema == 2 {
			kind = "monster"
		}
		identity := spriteIdentity{kind, spec.Index, spec.State}
		encoding := "zero-rle-linear"
		if p.Schema == 3 {
			switch kind {
			case "monster":
				if spec.Index < 0 || spec.Index > 79 || (spec.State != 1 && spec.State != 2) {
					return nil, fmt.Errorf("art monster identity")
				}
			case "party":
				encoding = "raw-linear"
				if len(source) != 4096 || w != 32 || h != 32 || spec.Index < 0 || spec.Index > 15 || (spec.State != 1 && spec.State != 2) || spec.SourceOffset != spec.Index*256 {
					return nil, fmt.Errorf("art party geometry or identity")
				}
			default:
				return nil, fmt.Errorf("art sprite kind")
			}
		}
		if encodings[spec.Source] != encoding || w < 16 || w > 80 || w%4 != 0 || h < 1 || h > 80 || spec.SourceOffset < 0 || w/4*h > len(source) || spec.SourceOffset > len(source)-w/4*h || spec.Index < 0 || spec.State < 0 || identities[identity] {
			return nil, fmt.Errorf("art sprite geometry or identity")
		}
		identities[identity] = true
		raw := source[spec.SourceOffset : spec.SourceOffset+w/4*h]
		key := fmt.Sprintf("%s/%d/%d/", kind, w, h) + string(raw)
		if _, exists := bitmaps[key]; exists {
			return nil, fmt.Errorf("duplicate art bitmap")
		}
		bitmaps[key] = len(a.sprites)
		sprite := artSprite{kind: kind, width: w, height: h, index: spec.Index, state: spec.State, raw: raw, white: true}
		nonzero := false
		for _, v := range raw {
			if v != 0 {
				nonzero = true
			}
			for shift := 0; shift < 8; shift += 2 {
				px := v >> uint(shift) & 3
				if px != 0 && px != 3 {
					sprite.white = false
				}
			}
		}
		if !nonzero {
			return nil, fmt.Errorf("empty art bitmap")
		}
		best := -1
		for y := 0; y < h; y++ {
			for xb := 0; xb <= w/4-4; xb++ {
				count := 0
				for _, v := range raw[y*w/4+xb : y*w/4+xb+4] {
					if v != 0 {
						count++
					}
				}
				if count > best {
					best = count
					sprite.anchorRow = y
					sprite.anchorByte = xb
					sprite.anchor = binary.LittleEndian.Uint32(raw[y*w/4+xb:])
				}
			}
		}
		a.sprites = append(a.sprites, sprite)
	}
	imageSeen := map[string]bool{}
	var total int64
	pageReady := make([]bool, len(a.pages))
	spriteReady := make([]bool, len(a.sprites))
	for _, spec := range p.Images {
		if source, ok := sourceFiles[strings.ToLower(spec.File)]; ok && (source.Bytes != spec.Bytes || source.SHA256 != spec.SHA256) {
			return nil, fmt.Errorf("art conflicting basename identity")
		}
		if !artBaseName(spec.File) || strings.EqualFold(spec.File, "profile.json") || imageSeen[strings.ToLower(spec.File)] {
			return nil, fmt.Errorf("art image name")
		}
		imageSeen[strings.ToLower(spec.File)] = true
		b, err := artRead(filepath.Join(dir, spec.File), 16*1024*1024)
		if err != nil {
			return nil, err
		}
		total += int64(len(b))
		if total > 64*1024*1024 || !artIdentity(b, spec.Bytes, spec.SHA256) {
			return nil, fmt.Errorf("art image identity or total")
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 4_000_000 {
			return nil, fmt.Errorf("art image dimensions")
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		for i, page := range p.Pages {
			if page.Image == spec.File {
				a.pages[i].paint, err = artCrop(img, page.ImageRect, a.pages[i].rect.Dx(), a.pages[i].rect.Dy(), true)
				if err != nil {
					return nil, err
				}
				pageReady[i] = true
			}
		}
		for i, sprite := range p.Sprites {
			if sprite.Image == spec.File {
				a.sprites[i].paint, err = artCrop(img, sprite.ImageRect, sprite.Width, sprite.Height, false)
				if err != nil {
					return nil, err
				}
				spriteReady[i] = true
			}
		}
	}
	for _, ready := range append(pageReady, spriteReady...) {
		if !ready {
			return nil, fmt.Errorf("art missing image reference")
		}
	}
	for i, sprite := range a.sprites {
		if !sprite.white {
			a.anchors[sprite.anchor] = append(a.anchors[sprite.anchor], i)
		}
	}
	return a, nil
}

func (a *TownArt) drawPage(dst []byte, protected image.Rectangle) {
	for y := a.rect.Min.Y * 2; y < a.rect.Max.Y*2; y++ {
		for x := a.rect.Min.X * 2; x < a.rect.Max.X*2; x++ {
			pt := image.Pt(x/2, y/2)
			keep := pt.In(protected)
			for _, r := range a.preserve {
				if pt.In(r) {
					keep = true
					break
				}
			}
			if keep {
				continue
			}
			i := (y*640 + x) * 4
			j := (y-a.rect.Min.Y*2)*a.paint.Stride + (x-a.rect.Min.X*2)*4
			copy(dst[i:i+4], a.paint.Pix[j:j+4])
		}
	}
}

type artMatch struct {
	sprite   int
	rect     image.Rectangle
	rejected bool
}

func (a *TownArt) spriteMatches(indexed []byte) ([]artMatch, bool) {
	if len(indexed) != 320*200 {
		return nil, false
	}
	rows := 151
	for _, s := range a.sprites {
		if s.kind == "party" {
			rows = 200
			break
		}
	}
	packed := make([]byte, 80*rows)
	for y := 0; y < rows; y++ {
		for xb := 0; xb < 80; xb++ {
			for j := 0; j < 4; j++ {
				packed[y*80+xb] |= (indexed[y*320+xb*4+j] & 3) << uint(6-2*j)
			}
		}
	}
	matches := []artMatch{}
	seen := map[[3]int]bool{}
	comparisons := 0
	for y := 0; y < rows; y++ {
		for xb := 0; xb <= 76; xb++ {
			anchor := binary.LittleEndian.Uint32(packed[y*80+xb:])
			for _, si := range a.anchors[anchor] {
				s := a.sprites[si]
				ox, oy := xb-s.anchorByte, y-s.anchorRow
				if ox < 0 || oy < 0 || ox > 80-s.width/4 {
					continue
				}
				if s.kind == "party" {
					if oy != 156 || s.width != 32 || s.height != 32 {
						continue
					}
				} else if oy > 151-s.height {
					continue
				}
				comparisons++
				if comparisons > 10000 {
					return nil, false
				}
				equal := true
				for row := 0; row < s.height; row++ {
					if !bytes.Equal(packed[(oy+row)*80+ox:(oy+row)*80+ox+s.width/4], s.raw[row*s.width/4:(row+1)*s.width/4]) {
						equal = false
						break
					}
				}
				key := [3]int{si, ox, oy}
				if equal && !seen[key] {
					seen[key] = true
					matches = append(matches, artMatch{sprite: si, rect: image.Rect(ox*4, oy, ox*4+s.width, oy+s.height)})
				}
			}
		}
	}
	for i := range matches {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].rect.Overlaps(matches[j].rect) {
				matches[i].rejected = true
				matches[j].rejected = true
			}
		}
	}
	return matches, true
}
func (a *TownArt) draw(dst, indexed, rgb []byte) {
	if len(a.pages) == 0 && len(a.sprites) == 0 {
		if protected, ok := a.Match(indexed); ok {
			a.drawPage(dst, protected)
		}
		return
	}
	var page *TownArt
	var protected image.Rectangle
	for _, candidate := range a.pages {
		if pr, ok := candidate.Match(indexed); ok {
			if page != nil {
				return
			}
			page = candidate
			protected = pr
		}
	}
	if page != nil {
		page.drawPage(dst, protected)
		return
	}
	matches, ok := a.spriteMatches(indexed)
	if !ok {
		return
	}
	background := [3]byte{}
	hasBackground := false
	for i, px := range indexed {
		if px == 0 && i*3+2 < len(rgb) {
			copy(background[:], rgb[i*3:i*3+3])
			hasBackground = true
			break
		}
	}
	if !hasBackground {
		return
	}
	for _, m := range matches {
		if m.rejected {
			continue
		}
		s := a.sprites[m.sprite]
		for y := m.rect.Min.Y * 2; y < m.rect.Max.Y*2; y++ {
			for x := m.rect.Min.X * 2; x < m.rect.Max.X*2; x++ {
				i := (y*640 + x) * 4
				j := (y-m.rect.Min.Y*2)*s.paint.Stride + (x-m.rect.Min.X*2)*4
				r, g, b, alpha := s.paint.Pix[j], s.paint.Pix[j+1], s.paint.Pix[j+2], s.paint.Pix[j+3]
				copy(dst[i:i+3], background[:])
				dst[i+3] = 255
				if r == 0 && g == 0 && b == 0 {
					continue
				}
				for c, v := range []byte{r, g, b} {
					dst[i+c] = byte(int(v) + (int(background[c])*(255-int(alpha))+127)/255)
				}
			}
		}
	}
}
