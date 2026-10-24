package realspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func chunk(tag string, b []byte) []byte {
	r := append([]byte(tag), make([]byte, 4)...)
	binary.BigEndian.PutUint32(r[4:], uint32(len(b)))
	r = append(r, b...)
	if len(b)%2 != 0 {
		r = append(r, 0)
	}
	return r
}
func form(kind string, parts ...[]byte) []byte {
	b := []byte(kind)
	for _, p := range parts {
		b = append(b, p...)
	}
	return chunk("FORM", b)
}
func vertBytes(v [][3]int32) []byte {
	var b []byte
	for _, v := range v {
		for _, x := range v {
			b = binary.LittleEndian.AppendUint32(b, uint32(x))
		}
	}
	return b
}
func modelIFF(kind ModelKind, verts [][3]int32, tris [][8]byte, lods ...[]uint16) []byte {
	var vt []byte
	for _, r := range tris {
		vt = append(vt, r[:]...)
	}
	var levels [][]byte
	for i, lod := range lods {
		b := make([]byte, 4)
		for _, idx := range lod {
			b = binary.LittleEndian.AppendUint16(b, idx)
		}
		levels = append(levels, chunk(fmt.Sprintf("LVL%d", i), b))
	}
	name := "POLY"
	if kind == APPR {
		name = "AFTB"
	}
	return form("REAL", form("APPR", form(name, chunk("INFO", []byte{0}), chunk("VERT", vertBytes(verts)), form("DETA", levels...), form("TRIS", chunk("VTRI", vt)), chunk("LNTH", []byte{0xf4, 1, 0, 0}))))
}

type archiveEntry struct {
	name string
	data []byte
}

func archive(entries ...archiveEntry) []byte {
	b := make([]byte, 8+74*len(entries))
	binary.LittleEndian.PutUint32(b, uint32(len(entries)))
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)))
	for i, e := range entries {
		off := 8 + 74*i
		b[off] = 1
		copy(b[off+1:off+66], e.name)
		binary.LittleEndian.PutUint32(b[off+66:], uint32(len(b)))
		binary.LittleEndian.PutUint32(b[off+70:], uint32(len(e.data)))
		b = append(b, e.data...)
	}
	return b
}
func syntheticLibrary(t *testing.T, entries ...archiveEntry) *Library {
	t.Helper()
	l := &Library{}
	if err := l.addTRE("TEST.TRE", archive(entries...)); err != nil {
		t.Fatal(err)
	}
	return l
}

var triangleVerts = [][3]int32{{0, 0, 0}, {256, 0, 0}, {0, 256, 0}}
var triangleFaces = [][8]byte{{4, 0, 1, 2, 17, 0, 0, 0}}

const objectPath = `..\..\DATA\OBJECTS\`

func TestLibraryIdentificationAndPath(t *testing.T) {
	a := modelIFF(POLY, triangleVerts, triangleFaces, []uint16{0})
	bFaces := [][8]byte{{4, 2, 1, 0, 17, 0, 0, 0}}
	b := modelIFF(POLY, triangleVerts, bFaces, []uint16{0})
	other := [][3]int32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	l := syntheticLibrary(t, archiveEntry{objectPath + "A.IFF", a}, archiveEntry{objectPath + "B.IFF", b}, archiveEntry{`..\..\DATA\OTHER\X.IFF`, modelIFF(POLY, other, triangleFaces, []uint16{0})})
	if l.Stats().Poly != 2 {
		t.Fatalf("path: %+v", l.Stats())
	}
	got := l.Identify(sha256.Sum256(vertBytes(triangleVerts)), 0, [][3]int16{{0, 1, 2}}, []uint16{0x3441})
	if len(got) != 1 || got[0].Name != objectPath+"A.IFF" {
		t.Fatalf("VERT hash alone is insufficient: %+v", got)
	}
	if got := l.Identify(sha256.Sum256(vertBytes(other)), 0, [][3]int16{{0, 1, 2}}, []uint16{0x3441}); len(got) != 0 {
		t.Fatal("OTHER must not be loaded")
	}
}

func TestFaceClassAndDeterministicSelection(t *testing.T) {
	a := modelIFF(POLY, triangleVerts, triangleFaces, []uint16{0})
	two := append(append([][8]byte(nil), triangleFaces...), [8]byte{4, 2, 1, 0, 99})
	b := modelIFF(POLY, triangleVerts, two, []uint16{0}, []uint16{1})
	c := modelIFF(POLY, triangleVerts, [][8]byte{{5, 0, 1, 2}}, []uint16{0})
	x, y := archiveEntry{objectPath + "Z.IFF", a}, archiveEntry{objectPath + "A.IFF", b}
	var selected [2][32]byte
	for i, es := range [][]archiveEntry{{x, y, {objectPath + "C.IFF", c}}, {y, x, {objectPath + "C.IFF", c}}} {
		l := syntheticLibrary(t, es...)
		got := l.Identify(sha256.Sum256(vertBytes(triangleVerts)), 0, [][3]int16{{0, 1, 2}}, []uint16{0x3441})
		if len(got) != 2 || got[0].GeoKey == got[1].GeoKey {
			t.Fatalf("class filter / two GeoKeys: %+v", got)
		}
		selected[i] = got[0].GeoKey
		all := l.Identify(sha256.Sum256(vertBytes(triangleVerts)), 0, [][3]int16{{0, 1, 2}}, []uint16{0xffff})
		if len(all) != 3 {
			t.Fatal("unknown FaceVT must not filter")
		}
	}
	if selected[0] != selected[1] {
		t.Fatal("selection depends on archive order")
	}
}

var flameVerts = [][3]int32{{-10, 0, 0}, {10, 0, 0}, {0, -150, 0}}
var flameFaces = [][8]byte{{2, 0, 1, 2}}

func TestAPPRCompatibilityApexAndSeparation(t *testing.T) {
	l := syntheticLibrary(t, archiveEntry{objectPath + "FLAME.IFF", modelIFF(APPR, flameVerts, flameFaces, []uint16{0})})
	if l.Stats().Appr != 1 {
		t.Fatal(l.Diagnostics())
	}
	rt := append([][3]int32(nil), flameVerts...)
	rt[2][1] = -400
	cand := l.IdentifyAppr(rt, 0, [][3]int16{{0, 1, 2}}, []uint16{0x3495})
	if len(cand) != 1 || cand[0].VertSHA == sha256.Sum256(vertBytes(rt)) || !ApexInRange(cand[0], rt) {
		t.Fatal("dynamic apex must identify")
	}
	rt[2][0]++
	if len(l.IdentifyAppr(rt, 0, [][3]int16{{0, 1, 2}}, []uint16{0x3495})) != 0 {
		t.Fatal("apex X must match")
	}
	rt[2][0]--
	rt[2][2]++
	if len(l.IdentifyAppr(rt, 0, [][3]int16{{0, 1, 2}}, []uint16{0x3495})) != 0 {
		t.Fatal("apex Z must match")
	}
	bad := append([][3]int32(nil), flameVerts...)
	bad[2][1] = -10
	l = syntheticLibrary(t, archiveEntry{objectPath + "BAD.IFF", modelIFF(APPR, bad, flameFaces, []uint16{0})})
	if l.Stats().Appr != 0 || l.Stats().Rejected != 1 {
		t.Fatal("apex separation must reject")
	}
}

func TestLibraryMalformedAndKeys(t *testing.T) {
	valid := modelIFF(POLY, triangleVerts, triangleFaces, []uint16{0})
	bad := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(bad[4:], uint32(len(bad)+1))
	l := syntheticLibrary(t, archiveEntry{objectPath + "BAD.IFF", bad}, archiveEntry{objectPath + "OK.IFF", valid}, archiveEntry{objectPath + "NO.IFF", []byte("no FORM")}, archiveEntry{objectPath + "NONE.IFF", form("REAL", chunk("XTRA", []byte{1}))})
	if l.Stats().Poly != 1 || l.Stats().Rejected != 1 || l.Stats().NotFORM != 1 || l.Stats().NoGeometry != 1 {
		t.Fatalf("stats %+v diag %+v", l.Stats(), l.Diagnostics())
	}
	e := l.Entries()[0]
	// Independent byte construction for the LP key; excludes thresholds and colors.
	var key bytes.Buffer
	for _, b := range [][]byte{vertBytes(triangleVerts), {0, 0}, {4, 0, 1, 2}} {
		binary.Write(&key, binary.LittleEndian, uint32(len(b)))
		key.Write(b)
	}
	if e.GeoKey != sha256.Sum256(key.Bytes()) {
		t.Fatal("GeoKey length prefixes")
	}
	l2 := syntheticLibrary(t, archiveEntry{objectPath + "IDX.IFF", modelIFF(POLY, triangleVerts, triangleFaces, []uint16{5})})
	if l2.Stats().Rejected != 1 || len(l2.Entries()) != 0 {
		t.Fatal("out of bounds LOD")
	}
}

func TestRealLibraryCounts(t *testing.T) {
	root := os.Getenv("SC_REALSPACE_ROOT")
	if root == "" {
		t.Skip("SC_REALSPACE_ROOT not set; no substitute for original TRE")
	}
	for _, name := range []string{"BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		if _, err := os.Stat(filepath.Join(root, name)); os.IsNotExist(err) {
			t.Skip("missing original " + name)
		} else if err != nil {
			t.Fatal(err)
		}
	}
	l, err := LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	s := l.Stats()
	t.Logf("archives=%+v poly=%d appr=%d rejected=%d noGeometry=%d notFORM=%d", s.Archives, s.Poly, s.Appr, s.Rejected, s.NoGeometry, s.NotFORM)
	if s.Poly != 440 || s.Appr != 18 {
		t.Fatalf("real archive count %+v", s)
	}
	keys := map[[32]byte]bool{}
	for _, e := range l.Entries() {
		if e.Kind == APPR {
			keys[e.AppKey] = true
		}
	}
	if len(keys) != 9 {
		t.Fatalf("APPR AppKeys %d", len(keys))
	}
	statsBefore := l.Stats()
	_ = l.Identify([32]byte{}, 0, nil, nil)
	if !reflect.DeepEqual(l.Stats(), statsBefore) {
		t.Fatal("identification mutates library stats")
	}
}
