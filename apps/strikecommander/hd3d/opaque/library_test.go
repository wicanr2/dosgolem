package opaque

import (
	"bytes"
	"encoding/binary"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"os"
	"reflect"
	"testing"
)

func iffChunk(tag string, b []byte) []byte {
	out := make([]byte, 8+len(b)+(len(b)&1))
	copy(out, tag)
	binary.BigEndian.PutUint32(out[4:], uint32(len(b)))
	copy(out[8:], b)
	return out
}
func iffForm(tag string, cs ...[]byte) []byte {
	return iffChunk("FORM", append([]byte(tag), bytes.Join(cs, nil)...))
}
func handMaterial(uv byte) []byte {
	vt := []byte{0, 0, 1, 2, 0, 0, 0, 0}
	verts := make([]byte, 36)
	lod := []byte{0, 0, 0, 0, 0, 0}
	u := []byte{0, 0, uv, 0, 1, 0, 0, 1}
	tex := append([]byte("TEST\x00\x00\x00\x00\x03\x00\x01\x00LZ"), []byte{0, 15, 32, 72, 16, 16}...)
	return iffForm("REAL", iffForm("APPR", iffForm("POLY", iffChunk("VERT", verts), iffForm("DETA", iffChunk("LVL0", lod)), iffForm("TRIS", iffChunk("VTRI", vt), iffChunk("UVXY", u), iffForm("TXMS", iffChunk("TXMP", tex))))))
}
func TestMaterialSourceAndLiteralLZW(t *testing.T) {
	got, e := decodeTexture(append([]byte("TEST\x00\x00\x00\x00\x03\x00\x01\x00LZ"), []byte{0, 15, 32, 72, 16, 16}...))
	if e != nil || !bytes.Equal(got.pixels, []byte{7, 8, 9}) || got.w != 3 || got.h != 1 {
		t.Fatal(got, e)
	}
	rang := []byte{1, 0, 5, 0, 99, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	pal := iffForm("PALT", iffChunk("RANG", rang))
	l, e := NewMaterialLibrary([]Asset{{TRE: "A", Name: "X", IFF: handMaterial(0)}, {TRE: "B", Name: "Y", IFF: handMaterial(1)}}, pal)
	if e != nil {
		t.Fatal(e)
	}
	a, e := l.entry("A", "X", realspace.POLY)
	if e != nil || a.uv[0][2] != 0 {
		t.Fatal(a, e)
	}
	b, e := l.entry("B", "Y", realspace.POLY)
	if e != nil || b.uv[0][2] != 1 {
		t.Fatal(b, e)
	}
	if a.key == b.key {
		t.Fatal("different original UV collapsed")
	}
	// Caller mutation cannot change a retained original source.
	raw := handMaterial(0)
	l, e = NewMaterialLibrary([]Asset{{TRE: "A", Name: "X", IFF: raw}}, pal)
	if e != nil {
		t.Fatal(e)
	}
	clear(raw)
	a, e = l.entry("A", "X", realspace.POLY)
	if e != nil || a.uv[0][2] != 0 {
		t.Fatal("input alias", e)
	}
}

func TestMaterialLibraryOriginalSources(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("SC_MODEL_ROOT original input required")
	}
	m, e := LoadMaterialLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	g, e := realspace.LoadLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, entry := range g.Entries() {
		a, e := m.entry(entry.TRE, entry.Name, entry.Kind)
		if e != nil {
			t.Fatal(e)
		}
		if a.vert != entry.VertSHA || !reflect.DeepEqual(a.vtri, entry.VTRI) || !reflect.DeepEqual(a.lod, entry.LOD) {
			t.Fatal("independent original material source geometry differs", entry.Name)
		}
		n++
	}
	if n == 0 {
		t.Fatal("no original material sources")
	}
	t.Logf("original model sources=%d materialOwned=%d key=%x", n, m.OwnedBytes(), m.Key())
}
