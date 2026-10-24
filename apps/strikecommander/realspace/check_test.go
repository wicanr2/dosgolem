package realspace

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"strings"
	"testing"
)

func testCamera() Camera {
	c := Camera{CamObj: 0x1234}
	binary.LittleEndian.PutUint16(c.Raw[8:], 10)
	binary.LittleEndian.PutUint16(c.Raw[10:], 20)
	binary.LittleEndian.PutUint32(c.Raw[12:], 256)
	binary.LittleEndian.PutUint32(c.Raw[16:], 256)
	return c
}
func testObject(e *ModelEntry, rt [][3]int32, lod int) ObjObs {
	if rt == nil {
		rt = append([][3]int32(nil), e.Verts...)
	}
	o := ObjObs{CamObj: 0x1234, CamIdx: 0, RecVT: 0x10c2, VertRT: rt, VertXYZ: vertBytes(rt), LOD: uint8(lod), NFaces: int16(len(e.LOD[lod])), Near: 256, FaceList: make([]uint16, len(e.LOD[lod]))}
	o.VertSHA = sha256.Sum256(o.VertXYZ)
	if e.Kind == APPR {
		o.RecVT = 0x314a
	}
	binary.LittleEndian.PutUint16(o.RecRaw[:], o.RecVT)
	for i, x := range [12]int32{256, 0, 0, 0, 0, 0, 256, 256, 0, 256, 0, 0} {
		binary.LittleEndian.PutUint32(o.RecRaw[0x23+i*4:], uint32(x))
	}
	binary.LittleEndian.PutUint16(o.ModelHead[2:], uint16(len(rt)))
	binary.LittleEndian.PutUint16(o.ModelHead[0x42+lod*2:], uint16(o.NFaces))
	xyz := [][3]int32{{0, 256, 0}, {256, 256, 0}, {0, 256, 256}}
	xy := [][2]int16{{10, 20}, {11, 20}, {10, 19}}
	if e.Kind == APPR {
		xyz = [][3]int32{{-10, 256, 0}, {10, 256, 0}, {0, 256, -400}}
		xy = [][2]int16{{9, 20}, {10, 20}, {10, 22}}
	}
	for i, ti := range e.LOD[lod] {
		r := e.VTRI[ti]
		f := FaceObs{SI: uint16(i), Returned: true, AL: 1, FaceVT: 0x3441, VIdx: [3]int16{int16(r[1]), int16(r[2]), int16(r[3])}}
		if e.Kind == APPR {
			f.FaceVT = 0x3495
		}
		for k, idx := range f.VIdx {
			p := &f.Proj[k]
			binary.LittleEndian.PutUint16(p[:], uint16(xy[idx][0]))
			binary.LittleEndian.PutUint16(p[2:], uint16(xy[idx][1]))
			for j, x := range xyz[idx] {
				binary.LittleEndian.PutUint32(p[4+j*4:], uint32(x))
			}
			p[17] = 6
		}
		o.Faces = append(o.Faces, f)
	}
	return o
}
func checkFixture(t *testing.T) (*Library, ObjObs, []Camera) {
	t.Helper()
	l := syntheticLibrary(t, archiveEntry{objectPath + "X.IFF", modelIFF(POLY, triangleVerts, triangleFaces, []uint16{0})})
	return l, testObject(l.Entries()[0], nil, 0), []Camera{testCamera()}
}

func TestCheckObjectHandComputed(t *testing.T) {
	l, o, c := checkFixture(t)
	v := CheckObject(l, c, &o)
	if v.Status != Passed || v.XYZCompared != 3 || v.ScreenCompared != 3 || v.XYZMismatch != 0 || v.ScreenMismatch != 0 {
		t.Fatalf("hand values: %+v", v)
	}
	o.Faces[0].Proj[0][4] = 1
	v = CheckObject(l, c, &o)
	if v.Status != Failed || v.XYZMismatch != 1 || v.First == nil || v.First.Field != "X" || v.First.Face != 0 || v.First.Vertex != 0 || v.First.Expected != 0 || v.First.Actual != 1 {
		t.Fatalf("first mismatch: %+v", v)
	}
}

func TestUncheckedFieldsAndFaults(t *testing.T) {
	for _, flags := range []byte{0, 2} {
		l, o, c := checkFixture(t)
		for k := range o.Faces[0].Proj {
			o.Faces[0].Proj[k][17] = flags
		}
		o.Faces[0].AL = 0
		v := CheckObject(l, c, &o)
		if v.Status != Unchecked {
			t.Fatalf("flags=%d: %+v", flags, v)
		}
	}
	l, o, c := checkFixture(t)
	o.Faces[0].Proj[1][17] = 4
	if v := CheckObject(l, c, &o); v.Status != Unchecked || v.XYZCompared != 2 || v.ScreenCompared != 3 {
		t.Fatalf("visible face missing bit1: %+v", v)
	}
	for _, fault := range []string{"read", "bounds", "table", "return", "unpaired", "frame", "count", "order", "vertex", "missingcam", "camera"} {
		l, o, c := checkFixture(t)
		switch fault {
		case "read":
			o.ReadFailed = true
		case "bounds":
			o.OutOfBounds = true
		case "table":
			o.FaceTableMismatch = true
		case "return":
			o.FaceRetMismatch = 1
		case "unpaired":
			o.F1RUnpaired = 1
		case "frame":
			o.UnmatchedFrame = 1
		case "count":
			o.NFaces = 2
		case "order":
			o.Faces[0].SI = 1
		case "vertex":
			o.Faces[0].VIdx[0] = -1
		case "missingcam":
			o.CamIdx = -1
		case "camera":
			o.CamObj++
		}
		if v := CheckObject(l, c, &o); v.Status != Failed {
			t.Fatalf("fault %s: %+v", fault, v)
		}
	}
}

func TestCameraPerObjectAndLOD(t *testing.T) {
	l, o, c := checkFixture(t)
	second := testCamera()
	second.CamObj++
	binary.LittleEndian.PutUint16(second.Raw[8:], 25)
	c = append(c, second)
	o2 := o
	o2.Faces = append([]FaceObs(nil), o.Faces...)
	o2.CamIdx = 1
	o2.CamObj = second.CamObj
	for i := range o2.Faces[0].Proj {
		p := &o2.Faces[0].Proj[i]
		binary.LittleEndian.PutUint16(p[:], uint16(readI16(p[:])+15))
	}
	if a, b := CheckObject(l, c, &o), CheckObject(l, c, &o2); a.Status != Passed || b.Status != Passed {
		t.Fatalf("separate cameras: %+v %+v", a, b)
	}
	if v := CheckObject(l, c[1:], &o); v.Status != Failed || v.Reason != "camera mismatch" {
		t.Fatalf("last camera overwrite: %+v", v)
	}
	tris := append(append([][8]byte(nil), triangleFaces...), [8]byte{4, 2, 1, 0})
	l = syntheticLibrary(t, archiveEntry{objectPath + "LOD.IFF", modelIFF(POLY, triangleVerts, tris, []uint16{0}, []uint16{1})})
	o = testObject(l.Entries()[0], nil, 1)
	if v := CheckObject(l, []Camera{testCamera()}, &o); v.Status != Passed {
		t.Fatalf("LOD1: %+v", v)
	}
}

func TestAPPRObjectAndRange(t *testing.T) {
	l := syntheticLibrary(t, archiveEntry{objectPath + "F.IFF", modelIFF(APPR, flameVerts, flameFaces, []uint16{0})})
	rt := append([][3]int32(nil), flameVerts...)
	rt[2][1] = -400
	o := testObject(l.Entries()[0], rt, 0)
	if v := CheckObject(l, []Camera{testCamera()}, &o); v.Status != Passed {
		t.Fatalf("hybrid vertex: %+v", v)
	}
	o.VertRT[2][1] = -100000
	o.VertXYZ = vertBytes(o.VertRT)
	o.VertSHA = sha256.Sum256(o.VertXYZ)
	// Keep the out-of-range observation internally coherent: without the bound
	// it would pass projection, so this specifically detects K22.
	binary.LittleEndian.PutUint32(o.Faces[0].Proj[2][12:], uint32(o.VertRT[2][1]))
	binary.LittleEndian.PutUint16(o.Faces[0].Proj[2][2:], 411)
	if v := CheckObject(l, []Camera{testCamera()}, &o); v.Status != Failed || !v.Unidentified || v.Reason != "apex out of range" {
		t.Fatalf("apex bound: %+v", v)
	}
}

func TestIntegerTransformWrapping(t *testing.T) {
	if v := Transform([3]int32{-1, 0, 0}, [12]int32{1}); v[0] != -1 {
		t.Fatal("negative SHRD must floor", v)
	}
	if v := Transform([3]int32{256, 0, 0}, [12]int32{1, 0, 0, math.MaxInt32}); v[0] != math.MinInt32 {
		t.Fatal("translation must wrap", v)
	}
	v := [3]int32{math.MaxInt32, math.MinInt32, math.MaxInt32}
	m := [12]int32{math.MaxInt32, math.MaxInt32, math.MinInt32, math.MaxInt32}
	// Independent arbitrary precision oracle: wrap the accumulator to signed64,
	// arithmetic shift, take low32, then wrap the translation.
	sum := new(big.Int)
	for j := 0; j < 3; j++ {
		sum.Add(sum, new(big.Int).Mul(big.NewInt(int64(v[j])), big.NewInt(int64(m[j]))))
	}
	mod64 := new(big.Int).Lsh(big.NewInt(1), 64)
	sum.Mod(sum, mod64)
	if sum.Bit(63) != 0 {
		sum.Sub(sum, mod64)
	}
	sum.Rsh(sum, 8)
	sum.Add(sum, big.NewInt(int64(m[3])))
	sum.Mod(sum, new(big.Int).Lsh(big.NewInt(1), 32))
	if got := Transform(v, m)[0]; uint32(got) != uint32(sum.Uint64()) {
		t.Fatalf("64-bit wrap: %d want low32 %d", got, sum.Uint64())
	}
}

func TestIntegerProjectionBoundaries(t *testing.T) {
	c := testCamera()
	x, y, near, _, err := Project(c, 0, 256, [3]int32{256, 128, 0})
	if err != nil || !near || x != 11 || y != 20 {
		t.Fatalf("no division: %d %d %t %v", x, y, near, err)
	}
	x, y, near, _, err = Project(c, 1, 256, [3]int32{-257, 512, -257})
	if err != nil || near || x != 9 || y != 21 {
		t.Fatalf("truncate quotient: %d %d %t %v", x, y, near, err)
	}
	if _, _, near, _, err = Project(c, 0, 256, [3]int32{0, 0, 0}); err != nil || !near {
		t.Fatal("Z=0 takes no division", err)
	}
	if _, _, _, _, err = Project(c, 1, 256, [3]int32{0, 0, 0}); err == nil {
		t.Fatal("flag!=0 Z<=0 must fail")
	}
	binary.LittleEndian.PutUint32(c.Raw[12:], math.MaxInt32)
	if _, _, _, _, err = Project(c, 1, 256, [3]int32{math.MaxInt32, 1, 0}); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatal("idiv overflow", err)
	}
	binary.LittleEndian.PutUint32(c.Raw[12:], 65536)
	x, _, _, clamped, err := Project(c, 0, 256, [3]int32{math.MaxInt32, 0, 0})
	if err != nil || !clamped || x != -0x7d00 {
		t.Fatalf("wrap before clamp: %d %t %v", x, clamped, err)
	}
}
