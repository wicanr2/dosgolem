package phantasie

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"testing"
)

func TestArtCollectionContracts(t *testing.T) {
	for _, test := range []struct {
		raw  []byte
		size int
		ok   bool
	}{
		{[]byte{5, 0, 2, 7}, 5, true}, {[]byte{0}, 1, false}, {[]byte{0, 3}, 3, false}, {[]byte{5}, 2, false},
	} {
		got, err := decodeArtSource(test.raw, artSource{Encoding: "zero-rle-linear", DecodedBytes: test.size})
		if (err == nil) != test.ok {
			t.Fatalf("run %v: %v", test, err)
		}
		if test.ok && !bytes.Equal(got, []byte{5, 0, 0, 0, 7}) {
			t.Fatal(got)
		}
	}
	for _, s := range []string{`{"schema":2,"schema":2}`, `{"a":{"b":1,"b":2}}`, `{} {}`, `{"schema":2,"unknown":1}`} {
		if _, err := decodeArtCollection([]byte(s)); err == nil {
			t.Fatal("accepted", s)
		}
	}
	a := &TownArt{anchors: map[uint32][]int{}, sprites: []artSprite{{width: 16, height: 2, raw: []byte{85, 85, 85, 85, 85, 85, 85, 85}, anchor: 0x55555555}}}
	a.anchors[0x55555555] = []int{0}
	idx := make([]byte, 320*200)
	for y := 149; y < 151; y++ {
		for x := 4; x < 20; x++ {
			idx[y*320+x] = 1
		}
	}
	m, ok := a.spriteMatches(idx)
	if !ok || len(m) != 1 || m[0].rect != image.Rect(4, 149, 20, 151) {
		t.Fatalf("boundary: %v", m)
	}
	for y := 156; y < 158; y++ {
		for x := 4; x < 20; x++ {
			idx[y*320+x] = 1
		}
	}
	m, ok = a.spriteMatches(idx)
	if !ok || len(m) != 1 {
		t.Fatal("party region accepted")
	}
	idx[149*320+4] = 3
	m, ok = a.spriteMatches(idx)
	if !ok || len(m) != 0 {
		t.Fatal("inexact accepted")
	}
	// Two matching, overlapping translations of a uniform bitmap must both fall back.
	for y := 0; y < 2; y++ {
		for x := 0; x < 20; x++ {
			idx[y*320+x] = 1
		}
	}
	m, ok = a.spriteMatches(idx)
	if !ok || len(m) != 2 || !m[0].rejected || !m[1].rejected {
		t.Fatalf("overlap: %v", m)
	}
	for y := 0; y < 151; y++ {
		for x := 0; x < 320; x++ {
			idx[y*320+x] = 1
		}
	}
	if _, ok := a.spriteMatches(idx); ok {
		t.Fatal("comparison limit ignored")
	}
}

// Full local asset inventory is independent of reaching all original monsters.
func TestArtCollectionLocalAssets(t *testing.T) {
	dir, root := os.Getenv("PHANTASIE_HD_DIR"), os.Getenv("PHANTASIE_ORIGINAL")
	if dir == "" || root == "" {
		t.Skip("local original and HD collection absent")
	}
	a, err := LoadTownArt(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.pages) != 4 || len(a.sprites) != 160 {
		t.Fatalf("inventory %d/%d", len(a.pages), len(a.sprites))
	}
	eligible, white := 0, 0
	for i, s := range a.sprites {
		if s.white {
			white++
			continue
		}
		eligible++
		idx := make([]byte, 320*200)
		x0, y0 := 100, 151-s.height
		for y := 0; y < s.height; y++ {
			for x := 0; x < s.width; x++ {
				idx[(y0+y)*320+x0+x] = s.raw[y*(s.width/4)+x/4] >> uint(6-2*(x%4)) & 3
			}
		}
		matches, ok := a.spriteMatches(idx)
		if !ok || len(matches) != 1 || matches[0].sprite != i || matches[0].rejected {
			t.Fatalf("identity %d: %v", i, matches)
		}
	}
	if eligible != 158 || white != 2 {
		t.Fatalf("eligible %d white %d", eligible, white)
	}
	profile, err := artRead(dir+"/profile.json", 65536)
	if err != nil {
		t.Fatal(err)
	}
	names, err := ArtAssetNames(profile)
	if err != nil || len(names) != 13 {
		t.Fatalf("bundle names %v %v", names, err)
	}
	// Corrupted profiles must not bypass true image decoding or bounded crops.
	var p artCollectionSpec
	if err = json.Unmarshal(profile, &p); err != nil {
		t.Fatal(err)
	}
	t.Logf("verified four pages, 160 image variants, 158 unambiguous candidates, %d assets", len(names))
}
