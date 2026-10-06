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

func TestPartyArtContracts(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	bitmap := make([]byte, 256)
	for i := range bitmap {
		bitmap[i] = byte(i%251 + 1)
	}
	party := make([]byte, 4096)
	copy(party[2304:2560], bitmap)
	monster := make([]byte, 4096)
	copy(monster[:256], bitmap)
	encoded := []byte{}
	for i := 0; i < len(monster); {
		if monster[i] != 0 {
			encoded = append(encoded, monster[i])
			i++
			continue
		}
		n := 1
		for n < 256 && i+n < len(monster) && monster[i+n] == 0 {
			n++
		}
		encoded = append(encoded, 0, byte(n-1))
		i += n
	}
	identity := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	if err := os.WriteFile(filepath.Join(root, "party.raw"), party, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "monster.rle"), encoded, 0644); err != nil {
		t.Fatal(err)
	}
	paint := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			paint.SetRGBA(x, y, color.RGBA{R: 200, G: 70, B: 20, A: 255})
		}
	}
	var pb bytes.Buffer
	if err := png.Encode(&pb, paint); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "paint.png"), pb.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	p := artCollectionSpec{Schema: 3, Sources: []artSource{{ID: "p", File: "party.raw", Bytes: 4096, SHA256: identity(party), Encoding: "raw-linear", DecodedBytes: 4096}, {ID: "m", File: "monster.rle", Bytes: int64(len(encoded)), SHA256: identity(encoded), Encoding: "zero-rle-linear", DecodedBytes: 4096}}, Images: []artImage{{File: "paint.png", Bytes: int64(pb.Len()), SHA256: identity(pb.Bytes())}}, Pages: []artPageSpec{}, Sprites: []artSpriteSpec{{Kind: "party", Source: "p", SourceOffset: 2304, Width: 32, Height: 32, Index: 9, State: 1, Image: "paint.png", ImageRect: [4]int{0, 0, 32, 32}}, {Kind: "monster", Source: "m", Width: 32, Height: 32, Index: 56, State: 1, Image: "paint.png", ImageRect: [4]int{0, 0, 32, 32}}}}
	load := func(p artCollectionSpec) (*TownArt, error) {
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return loadArtCollection(root, dir, b)
	}
	a, err := load(p)
	if err != nil {
		t.Fatal(err)
	}
	stamp := func(idx []byte, x, y int) {
		for sy := 0; sy < 32; sy++ {
			for sx := 0; sx < 32; sx++ {
				idx[(y+sy)*320+x+sx] = bitmap[sy*8+sx/4] >> uint(6-2*(sx%4)) & 3
			}
		}
	}
	idx := make([]byte, 320*200)
	stamp(idx, 84, 156)
	stamp(idx, 100, 119)
	m, ok := a.spriteMatches(idx)
	if !ok || len(m) != 2 {
		t.Fatalf("cross-kind identical bitmap %v", m)
	}
	for _, r := range m {
		if r.rejected {
			t.Fatal("disjoint matches rejected")
		}
		if a.sprites[r.sprite].kind == "party" && r.rect != image.Rect(84, 156, 116, 188) {
			t.Fatal("party geometry", r)
		}
		if a.sprites[r.sprite].kind == "monster" && r.rect != image.Rect(100, 119, 132, 151) {
			t.Fatal("monster geometry", r)
		}
	}
	for _, test := range []struct {
		name   string
		x, y   int
		change func([]byte)
	}{{"wrong-y", 84, 155, nil}, {"wrong-alignment", 85, 156, nil}, {"occluded", 84, 156, func(p []byte) { p[156*320+84] ^= 1 }}, {"white", 84, 156, func(p []byte) {
		for i, v := range p {
			if v != 0 {
				p[i] = 3
			}
		}
	}}} {
		t.Run(test.name, func(t *testing.T) {
			idx := make([]byte, 320*200)
			stamp(idx, test.x, test.y)
			if test.change != nil {
				test.change(idx)
			}
			m, ok := a.spriteMatches(idx)
			if !ok || len(m) != 0 {
				t.Fatal("inexact party accepted", m)
			}
		})
	}
	for _, test := range []struct {
		name   string
		change func(*artCollectionSpec)
	}{
		{"kind", func(p *artCollectionSpec) { p.Sprites[0].Kind = "portrait" }},
		{"party-index", func(p *artCollectionSpec) { p.Sprites[0].Index = 16 }},
		{"monster-index", func(p *artCollectionSpec) { p.Sprites[1].Index = 80 }},
		{"party-state", func(p *artCollectionSpec) { p.Sprites[0].State = 3 }},
		{"monster-state", func(p *artCollectionSpec) { p.Sprites[1].State = 0 }},
		{"offset", func(p *artCollectionSpec) { p.Sprites[0].SourceOffset = 2048 }},
		{"width", func(p *artCollectionSpec) { p.Sprites[0].Width = 48 }},
		{"height", func(p *artCollectionSpec) { p.Sprites[0].Height = 31 }},
		{"source-encoding", func(p *artCollectionSpec) { p.Sprites[0].Source = "m" }},
		{"source-length", func(p *artCollectionSpec) { p.Sources[0].DecodedBytes = 4095 }},
		{"duplicate-identity", func(p *artCollectionSpec) { p.Sprites = append(p.Sprites, p.Sprites[0]) }},
		{"duplicate-bitmap", func(p *artCollectionSpec) { r := p.Sprites[1]; r.Index = 57; p.Sprites = append(p.Sprites, r) }},
		{"sprite-limit", func(p *artCollectionSpec) {
			for len(p.Sprites) < 193 {
				p.Sprites = append(p.Sprites, p.Sprites[0])
			}
		}},
		{"source-limit", func(p *artCollectionSpec) {
			for len(p.Sources) < 9 {
				p.Sources = append(p.Sources, p.Sources[0])
			}
		}},
		{"wrong-sha", func(p *artCollectionSpec) { p.Images[0].SHA256 = string(bytes.Repeat([]byte("0"), 64)) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, _ := json.Marshal(p)
			var q artCollectionSpec
			json.Unmarshal(b, &q)
			test.change(&q)
			if _, err := load(q); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	base, _ := json.Marshal(p)
	var generic map[string]any
	json.Unmarshal(base, &generic)
	for _, version := range []int{2, 3} {
		q := map[string]any{}
		for k, v := range generic {
			q[k] = v
		}
		q["schema"] = version
		if version == 3 {
			var rows []map[string]any
			b, _ := json.Marshal(q["sprites"])
			json.Unmarshal(b, &rows)
			delete(rows[0], "kind")
			q["sprites"] = rows
		}
		b, _ := json.Marshal(q)
		if _, err := decodeArtCollection(b); err == nil {
			t.Fatal("schema shape accepted", version)
		}
	}
	// raw-linear is new in schema 3 and must remain invalid in old schema 2.
	q := p
	q.Schema = 2
	q.Sprites = append([]artSpriteSpec(nil), p.Sprites...)
	for i := range q.Sprites {
		q.Sprites[i].Kind = ""
	}
	if _, err := load(q); err == nil {
		t.Fatal("schema2 raw-linear accepted")
	}
}

func TestPartyArtLocalAssets(t *testing.T) {
	root, dir := os.Getenv("PHANTASIE_ORIGINAL"), os.Getenv("PHANTASIE_PARTY_HD_DIR")
	if root == "" || dir == "" {
		t.Skip("local party HD input absent")
	}
	a, err := LoadTownArt(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.pages) != 4 || len(a.sprites) != 192 {
		t.Fatal("inventory", len(a.pages), len(a.sprites))
	}
	eligible, white, party := 0, 0, 0
	for i, s := range a.sprites {
		if s.white {
			white++
			continue
		}
		eligible++
		x, y := 100, 151-s.height
		if s.kind == "party" {
			party++
			y = 156
		}
		idx := make([]byte, 320*200)
		for sy := 0; sy < s.height; sy++ {
			for sx := 0; sx < s.width; sx++ {
				idx[(y+sy)*320+x+sx] = s.raw[sy*(s.width/4)+sx/4] >> uint(6-2*(sx%4)) & 3
			}
		}
		m, ok := a.spriteMatches(idx)
		if !ok || len(m) != 1 || m[0].sprite != i || m[0].rejected {
			t.Fatal("identity", i, m)
		}
	}
	if eligible != 190 || white != 2 || party != 32 {
		t.Fatal("eligible/white/party", eligible, white, party)
	}
	b, err := os.ReadFile(filepath.Join(dir, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := ArtAssetNames(b)
	if err != nil || len(names) != 14 {
		t.Fatal("asset names", names, err)
	}
	t.Log("four pages, 160 monster variants, 32 party variants; 190 candidates, 2 white fallbacks, 14 assets")
}
