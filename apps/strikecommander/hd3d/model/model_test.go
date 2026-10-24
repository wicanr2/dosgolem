package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

var handVertices = [][3]int32{{2688, -2432, 0}, {7808, -2432, 0}, {2688, -7552, 0}}

func handChunk(tag string, b []byte) []byte {
	r := append([]byte(tag), make([]byte, 4)...)
	binary.BigEndian.PutUint32(r[4:], uint32(len(b)))
	r = append(r, b...)
	if len(b)&1 != 0 {
		r = append(r, 0)
	}
	return r
}
func handForm(kind string, b ...[]byte) []byte {
	x := []byte(kind)
	for _, p := range b {
		x = append(x, p...)
	}
	return handChunk("FORM", x)
}
func handVerts(v [][3]int32) []byte {
	var b []byte
	for _, p := range v {
		for _, x := range p {
			b = binary.LittleEndian.AppendUint32(b, uint32(x))
		}
	}
	return b
}
func handLibrary(t *testing.T, appr bool) *realspace.Library {
	t.Helper()
	v, kind, class := handVertices, "POLY", byte(4)
	if appr {
		v, kind, class = [][3]int32{{-10, 0, 0}, {10, 0, 0}, {0, -150, 0}}, "AFTB", 2
	}
	iff := handForm("REAL", handForm("APPR", handForm(kind,
		handChunk("INFO", []byte{0}), handChunk("VERT", handVerts(v)),
		handForm("DETA", handChunk("LVL0", []byte{0, 0, 0, 0, 0, 0})),
		handForm("TRIS", handChunk("VTRI", []byte{class, 0, 1, 2, 17, 0, 0, 0})), handChunk("LNTH", []byte{0xf4, 1, 0, 0}))))
	dir := t.TempDir()
	archive := make([]byte, 82)
	binary.LittleEndian.PutUint32(archive, 1)
	binary.LittleEndian.PutUint32(archive[4:], 82)
	archive[8] = 1
	copy(archive[9:74], `..\..\DATA\OBJECTS\HAND.IFF`)
	binary.LittleEndian.PutUint32(archive[74:], 82)
	binary.LittleEndian.PutUint32(archive[78:], uint32(len(iff)))
	archive = append(archive, iff...)
	for _, name := range []string{"BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		b := archive
		if name != "BIGTRE.TRE" {
			b = []byte{0, 0, 0, 0, 8, 0, 0, 0}
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	l, err := realspace.LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries()) != 1 {
		t.Fatalf("hand library: %+v", l.Diagnostics())
	}
	return l
}

func handCamera() realspace.Camera {
	c := realspace.Camera{CamObj: 0x1234}
	binary.LittleEndian.PutUint32(c.Raw[12:], 256)
	binary.LittleEndian.PutUint32(c.Raw[16:], 256)
	return c
}

// The expected original projections are literal hand values. No Transform,
// Project, geometry renderer, or Replay output creates these observations.
func handObject(mult int32, elem hd3d.FarPtr) realspace.ObjObs {
	o := realspace.ObjObs{CamObj: 0x1234, RecVT: 0x10c2, VertRT: append([][3]int32(nil), handVertices...), VertXYZ: handVerts(handVertices), NFaces: 1, FaceList: []uint16{0}, Flag17BC: 1, Near: 256, Far: 100000}
	o.VertSHA = sha256.Sum256(o.VertXYZ)
	binary.LittleEndian.PutUint16(o.RecRaw[:], o.RecVT)
	o.RecRaw[9] = o.LOD
	binary.LittleEndian.PutUint16(o.ModelHead[2:], 3)
	binary.LittleEndian.PutUint16(o.ModelHead[0x42:], 1)
	m := [12]int32{256 * mult, 0, 0, 0, 0, 0, 0, 256 * mult, 0, 256 * mult, 0, 0}
	for i, n := range m {
		binary.LittleEndian.PutUint32(o.RecRaw[0x23+i*4:], uint32(n))
	}
	f := realspace.FaceObs{FaceVT: 0x3441, Returned: true, AL: 1, VIdx: [3]int16{0, 1, 2}, Elem: realspace.FarPtr{Off: elem.Off, Seg: elem.Seg}}
	for i, p := range [3][5]int32{{10, 10, 2688 * mult, 256 * mult, -2432 * mult}, {30, 10, 7808 * mult, 256 * mult, -2432 * mult}, {10, 30, 2688 * mult, 256 * mult, -7552 * mult}} {
		binary.LittleEndian.PutUint16(f.Proj[i][:], uint16(p[0]))
		binary.LittleEndian.PutUint16(f.Proj[i][2:], uint16(p[1]))
		for j := 0; j < 3; j++ {
			binary.LittleEndian.PutUint32(f.Proj[i][4+j*4:], uint32(p[j+2]))
		}
		f.Proj[i][17] = 6
	}
	o.Faces = []realspace.FaceObs{f}
	return o
}

type handWrite struct {
	element hd3d.FarPtr
	index   int
	value   byte
}

// R6 uses a flat, literal 9-color table and horizontal horizon. Original pixels,
// Writer, and Written are assembled directly from the listed hand writes.
func handInput(t *testing.T, objects []realspace.ObjObs, writes ...handWrite) (Input, hd3d.Tables) {
	t.Helper()
	desc := [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}
	first := hd3d.Call{Seq: 0, Kind: hd3d.R6, Desc: desc, Params: [13]uint16{0, 43, 32, 160, 32}, DS: 0x1000, SS: 0x1000, Aux: hd3d.AuxRef{Color0: 9, Color1: 9, HasFlags: true}, Object: true} // malicious metadata
	in := Input{Key: 77, Cameras: []realspace.Camera{handCamera()}, Objects: objects, Raster: hd3d.Group{Key: 77, Calls: []hd3d.Call{first}, List: make([]uint16, 201), Buf3D: bytes.Repeat([]byte{9}, hd3d.PixelCount)}}
	for i := range in.Raster.List {
		in.Raster.List[i] = uint16(16 + 320*i)
	}
	orig := &hd3d.Result{Valid: true, Checked: true, Repaintable: true, CallElements: map[uint16]hd3d.FarPtr{0: {}}}
	for i := range orig.Pixels {
		orig.Pixels[i], orig.Writer[i], orig.Written[i] = 9, 0, true
	}
	seen := map[hd3d.FarPtr]bool{}
	for i, w := range writes {
		seq := uint16(i + 1)
		in.Raster.Calls = append(in.Raster.Calls, hd3d.Call{Seq: seq, Kind: hd3d.BlackBox, Desc: desc, Element: w.element, Patch: []hd3d.PixelSet{{Index: uint32(w.index), Value: w.value}}})
		orig.Pixels[w.index], orig.Writer[w.index], in.Raster.Buf3D[w.index] = w.value, seq, w.value
		orig.CallElements[seq] = w.element
		if w.element != (hd3d.FarPtr{}) && !seen[w.element] {
			var h [16]byte
			binary.LittleEndian.PutUint16(h[:], 0x17c1)
			in.Elements = append(in.Elements, Element{w.element, h})
			seen[w.element] = true
		}
	}
	in.Original = orig // ObjectWritten is deliberately all false, not trusted.
	tab := bytes.Repeat([]byte{9}, 200)
	for i, off := range []uint16{8, 56, 104, 152} {
		binary.LittleEndian.PutUint16(tab[0xc0+2*i:], off)
	}
	return in, hd3d.Tables{R6Table: tab}
}

func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRenderOriginalLibraryAndImmutableInput(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	o := handObject(1, ep)
	// A forged serialized selection and unrelated POLY VertRT are not geometry.
	o.Verdict = realspace.ObjVerdict{Status: realspace.Passed, Selected: &realspace.ModelEntry{Name: "FORGED", Verts: [][3]int32{{999, 999, 999}}}}
	o.VertRT[0] = [3]int32{999, 999, 999}
	in, tables := handInput(t, []realspace.ObjObs{o}, handWrite{ep, 15*320 + 15, 7})
	before, libraryBefore := encoded(t, in), encoded(t, l.Entries())
	for _, scale := range []int{2, 3} {
		r, err := Render(in, l, tables, scale)
		if err != nil || !r.Passed || !r.B.Passed || !r.C.Passed || r.B.Eligible != 3 || r.B.ZeroX != 3 || r.B.ZeroY != 3 || r.C.C1 != 1 || r.C.C2 != 0 || r.C.C3b == 0 || r.Diagnostics.ClassifiedObjectCalls != 1 || !r.Diagnostics.ReplayPixelsEqual || !r.Diagnostics.ReplayWriterEqual {
			t.Fatalf("scale %d: err=%v result=%+v B=%+v C=%+v", scale, err, r.Objects, r.B, r.C)
		}
		idx := (15*scale+(scale-1)/2)*r.Frame.Width + 15*scale + (scale-1)/2
		if r.Frame.DepthQ48[idx] == 0 || r.Frame.Owner[idx].Object != 0 || r.Objects[0].Name == "FORGED" {
			t.Fatal("trusted library geometry was not rendered")
		}
	}
	if !bytes.Equal(before, encoded(t, in)) || !bytes.Equal(libraryBefore, encoded(t, l.Entries())) {
		t.Fatal("input, original writer, metadata, or trusted model library changed")
	}
}

func TestCIndependentEverWrittenAndElementOrderK10(t *testing.T) {
	l := handLibrary(t, false)
	a, b := hd3d.FarPtr{Off: 16, Seg: 512}, hd3d.FarPtr{Off: 32, Seg: 512}
	pixel := 15*320 + 15
	objects := []realspace.ObjObs{handObject(1, a), handObject(2, b)}
	// Creation order is A,B. R10 drawing order is B,A. The near A wins both.
	in, tables := handInput(t, objects, handWrite{b, pixel, 6}, handWrite{a, pixel, 7})
	r, err := Render(in, l, tables, 3)
	if err != nil || !r.Passed || r.C.C1 != 1 || r.C.C4 != 0 {
		t.Fatalf("K10 baseline: %v %+v", err, r.C)
	}
	// Explicit wrong association has the ordinal-pairing mutant's signature.
	in.Objects[0].Faces[0].Elem, in.Objects[1].Faces[0].Elem = in.Objects[1].Faces[0].Elem, in.Objects[0].Faces[0].Elem
	wrong, err := Render(in, l, tables, 3)
	if err != nil || wrong.C.C1 != 0 || wrong.C.C4 != 1 {
		t.Fatalf("K10 negative insensitive: %v %+v", err, wrong.C)
	}
	in, tables = handInput(t, []realspace.ObjObs{handObject(1, a)}, handWrite{a, pixel, 7}, handWrite{hd3d.FarPtr{}, pixel, 8})
	r, err = Render(in, l, tables, 3)
	if err != nil || !r.C.Available || r.C.C3a != 1 || r.C.C1 != 0 || r.C.C3b == 0 {
		t.Fatalf("C3a must use classified historical writes: %v %+v", err, r.C)
	}
}

func TestUnlinkedAndLinkedOutsideSetK17K25(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	for _, k := range []string{"K17", "K25"} {
		t.Run(k, func(t *testing.T) {
			o := handObject(1, ep)
			if k == "K17" {
				o.Faces[0].Elem = realspace.FarPtr{}
			}
			in, tab := handInput(t, []realspace.ObjObs{o}, handWrite{ep, 15*320 + 15, 7})
			if k == "K25" {
				binary.LittleEndian.PutUint16(in.Elements[0].Header[:], 0x8888)
			}
			r, err := Render(in, l, tab, 3)
			if err != nil || r.Passed || !r.C.Available || r.C.Passed || (k == "K17" && r.C.Unlinked != 1) || (k == "K25" && r.C.LinkedOutsideSet != 1) {
				t.Fatalf("%s: %v %+v", k, err, r.C)
			}
		})
	}
}

func TestUnavailableEvidenceAndFailedObjects(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	for _, k := range []string{"no-writer", "missing-header", "vt-conflict", "e1-conflict", "unknown-object", "read-failed-object", "read-failed-call", "writer-mismatch", "pixel-mismatch"} {
		t.Run(k, func(t *testing.T) {
			in, tab := handInput(t, []realspace.ObjObs{handObject(1, ep)}, handWrite{ep, 15*320 + 15, 7})
			switch k {
			case "no-writer":
				in.Original = nil
			case "missing-header":
				in.Elements = nil
			case "vt-conflict":
				x := in.Elements[0]
				x.Header[0]++
				in.Elements = append(in.Elements, x)
			case "e1-conflict":
				in.Objects = append(in.Objects, handObject(1, ep))
			case "unknown-object":
				in.Objects[0].RecVT = 0xffff
				binary.LittleEndian.PutUint16(in.Objects[0].RecRaw[:2], 0xffff)
			case "read-failed-object":
				in.Objects[0].ReadFailed = true
			case "read-failed-call":
				in.Raster.Calls[1].ReadFailed = true
			case "writer-mismatch":
				in.Original.Writer[15*320+15] = 0
			case "pixel-mismatch":
				in.Raster.Calls[1].Patch[0].Value++
			}
			before := encoded(t, in)
			r, err := Render(in, l, tab, 3)
			if err != nil || r.Passed {
				t.Fatalf("must not report success: %v %+v", err, r)
			}
			failedObject := k == "unknown-object" || k == "read-failed-object"
			if failedObject {
				if r.Objects[0].Status != realspace.Failed || r.C.ObjectNotDrawn != 1 || r.B.SkippedObjects != 1 {
					t.Fatalf("object failure disappeared: %+v %+v", r.Objects, r.C)
				}
			} else if r.C.Available || len(r.Diagnostics.CUnavailable) == 0 {
				t.Fatalf("unavailable C evidence was hidden: %+v", r.C)
			}
			if !bytes.Equal(before, encoded(t, in)) {
				t.Fatal("failure handling mutated original evidence")
			}
		})
	}
}

func TestC5PriorityAndEightNeighborBandK12(t *testing.T) {
	old := make([]pixelOwner, hd3d.PixelCount)
	for y := 10; y <= 20; y++ {
		for x := 10; x <= 20; x++ {
			old[y*320+x] = pixelOwner{faceID{0, 0}, true}
		}
	}
	makeResult := func() Result {
		return Result{Frame: geometry.Frame{Scale: 3, Width: 960, Height: 600, Owner: make([]geometry.FaceRef, 960*600), DepthQ48: make([]uint64, 960*600)}, Objects: []ObjectReport{{Status: realspace.Passed}}}
	}
	for _, kind := range []string{"C5", "C6", "K12"} {
		r := makeResult()
		p := faceProperty{clipped: kind == "C5", clamped: kind != "K12"}
		classifyCoverage(&r, old, make([]bool, hd3d.PixelCount), map[faceID]faceProperty{{0, 0}: p})
		if kind == "C5" && (r.C.C5 != 121 || r.C.C6 != 0 || r.C.C2 != 0) || kind == "C6" && (r.C.C6 != 121 || r.C.C5 != 0 || r.C.C2 != 0) || kind == "K12" && (r.C.C2 != 121 || r.C.C2OutsideBand != 81) || r.C.BoundaryPixels != 88 {
			t.Fatalf("%s hand count: %+v", kind, r.C)
		}
	}
}

func TestAPPRUsesOnlyRuntimeApex(t *testing.T) {
	l := handLibrary(t, true)
	e := l.Entries()[0]
	rt := append([][3]int32(nil), e.Verts...)
	rt[2][1] = -400
	o := handObject(1, hd3d.FarPtr{})
	o.RecVT, o.VertRT, o.VertXYZ = 0x314a, rt, handVerts(rt)
	o.VertSHA = sha256.Sum256(o.VertXYZ)
	binary.LittleEndian.PutUint16(o.RecRaw[:], o.RecVT)
	f := &o.Faces[0]
	f.FaceVT = 0x3495
	for i, p := range [3][5]int32{{19, 20, -10, 256, 0}, {20, 20, 10, 256, 0}, {20, 22, 0, 256, -400}} {
		binary.LittleEndian.PutUint16(f.Proj[i][:], uint16(p[0]))
		binary.LittleEndian.PutUint16(f.Proj[i][2:], uint16(p[1]))
		for j := 0; j < 3; j++ {
			binary.LittleEndian.PutUint32(f.Proj[i][4+j*4:], uint32(p[j+2]))
		}
	}
	cam := handCamera()
	binary.LittleEndian.PutUint16(cam.Raw[8:], 20)
	binary.LittleEndian.PutUint16(cam.Raw[10:], 20)
	binary.LittleEndian.PutUint32(cam.Raw[12:], 4096)
	r, err := Render(Input{Objects: []realspace.ObjObs{o}, Cameras: []realspace.Camera{cam}}, l, hd3d.Tables{}, 3)
	if err != nil || r.Objects[0].Status != realspace.Passed || r.Objects[0].Kind != realspace.APPR || r.Frame.DepthQ48[65*960+60] == 0 {
		t.Fatalf("runtime Y=-400 must extend beyond file Y=-150: %v %+v", err, r.Objects)
	}
	o.VertRT[2][1] = -300                                                      // typed RT and captured raw bytes now disagree.
	binary.LittleEndian.PutUint32(o.Faces[0].Proj[2][12:], uint32(0xfffffed4)) // -300; unchanged original sy=22.
	r, err = Render(Input{Objects: []realspace.ObjObs{o}, Cameras: []realspace.Camera{cam}}, l, hd3d.Tables{}, 3)
	if err != nil || r.Passed || r.Objects[0].Status == realspace.Passed || r.Objects[0].Reason != "runtime vertex bytes mismatch" {
		t.Fatal("APPR RT/raw inconsistency accepted")
	}
}

func TestRenderALZeroAndInvalidNumericInputs(t *testing.T) {
	l := handLibrary(t, false)
	o := handObject(1, hd3d.FarPtr{})
	o.Faces[0].AL = 0
	in := Input{Cameras: []realspace.Camera{handCamera()}, Objects: []realspace.ObjObs{o}}
	r, err := Render(in, l, hd3d.Tables{}, 3)
	if err != nil || r.Objects[0].Status != realspace.Passed || r.Objects[0].VisibleFaces != 0 || r.Objects[0].Triangles != 0 || r.Frame.Stats.WrittenSamples != 0 {
		t.Fatalf("AL=0 was submitted: %v %+v", err, r.Objects)
	}
	if _, err := Render(in, nil, hd3d.Tables{}, 3); err == nil {
		t.Fatal("nil original-file library accepted")
	}
	if _, err := Render(in, l, hd3d.Tables{}, 1); err == nil {
		t.Fatal("unsupported N=1 accepted")
	}
	in.Objects[0].Near = 0
	if _, err := Render(in, l, hd3d.Tables{}, 3); err == nil {
		t.Fatal("Near=0 accepted")
	}
}

func TestDedupeClampExactAndWireColor(t *testing.T) {
	l := handLibrary(t, false)
	o := handObject(1, hd3d.FarPtr{})
	r, err := Render(Input{Cameras: []realspace.Camera{handCamera()}, Objects: []realspace.ObjObs{o, o}}, l, hd3d.Tables{}, 2)
	if err != nil || r.B.Eligible != 3 || r.B.Duplicates != 3 {
		t.Fatalf("per-group tuple dedup: %v %+v", err, r.B)
	}
	binary.LittleEndian.PutUint32(o.RecRaw[0x23+3*4:], 8189312)
	for i, x := range []uint32{8192000, 8197120, 8192000} {
		binary.LittleEndian.PutUint16(o.Faces[0].Proj[i][:], 32000)
		binary.LittleEndian.PutUint32(o.Faces[0].Proj[i][4:], x)
	}
	r, err = Render(Input{Cameras: []realspace.Camera{handCamera()}, Objects: []realspace.ObjObs{o}}, l, hd3d.Tables{}, 2)
	if err != nil || r.Objects[0].Status != realspace.Passed || r.Objects[0].ClampedVertices != 3 || r.Objects[0].ClampExactVertices != 2 || r.B.ExcludedClamped != 3 || r.B.Applicable {
		t.Fatalf("raw ±7D00 and exact unclamped boundary: %v %+v %+v", err, r.Objects, r.B)
	}
	e := &realspace.ModelEntry{Kind: realspace.POLY, GeoKey: [32]byte{0, 191, 192}, AppKey: [32]byte{193, 194, 195}}
	if fmt.Sprint(wireColor(e)) != "{64 255 64 255}" {
		t.Fatal(wireColor(e))
	}
	e.Kind = realspace.APPR
	if fmt.Sprint(wireColor(e)) != "{65 66 67 255}" {
		t.Fatal(wireColor(e))
	}
}

func TestTypedRecordMetadataMustMatchCapturedBytes(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	for _, field := range []string{"RecVT", "LOD"} {
		t.Run(field, func(t *testing.T) {
			in, tables := handInput(t, []realspace.ObjObs{handObject(1, ep)}, handWrite{ep, 15*320 + 15, 7})
			if field == "RecVT" {
				binary.LittleEndian.PutUint16(in.Objects[0].RecRaw[:2], 0x314a)
			} else {
				in.Objects[0].RecRaw[9] = 1
			}
			before := encoded(t, in)
			r, err := Render(in, l, tables, 3)
			if err != nil || r.Passed || r.Objects[0].Status != realspace.Failed || r.Objects[0].Drawn || r.Frame.Stats.WrittenSamples != 0 || r.C.ObjectNotDrawn != 1 {
				t.Fatalf("typed %s overrode raw capture: %v %+v %+v", field, err, r.Objects, r.C)
			}
			if !bytes.Equal(before, encoded(t, in)) {
				t.Fatal("metadata rejection rewrote capture")
			}
		})
	}
}

func TestEmptyBGroupDoesNotImposeStateDenominatorGate(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	for _, kind := range []string{"clamped", "no-divide", "background-only"} {
		t.Run(kind, func(t *testing.T) {
			o := handObject(1, ep)
			if kind == "clamped" {
				binary.LittleEndian.PutUint32(o.RecRaw[0x23+3*4:], 8189312)
				for i, x := range []uint32{8192000, 8197120, 8192000} {
					binary.LittleEndian.PutUint16(o.Faces[0].Proj[i][:], 32000)
					binary.LittleEndian.PutUint32(o.Faces[0].Proj[i][4:], x)
				}
			} else {
				o.Flag17BC, o.Near = 0, 512
			}
			in, tables := handInput(t, []realspace.ObjObs{o}, handWrite{ep, 15*320 + 15, 7})
			if kind == "background-only" {
				in, tables = handInput(t, nil)
			}
			r, err := Render(in, l, tables, 3)
			if err != nil || !r.Passed || r.B.Applicable || !r.B.Passed || r.B.Eligible != 0 || !r.C.Available || !r.C.Passed {
				t.Fatalf("%s: group incorrectly requires state B denominator: %v %+v %+v", kind, err, r.B, r.C)
			}
			if kind == "clamped" && (r.B.ExcludedClamped != 3 || r.C.C6 != 1) || kind == "no-divide" && (r.B.ExcludedNoDivide != 3 || r.C.C5 != 1) {
				t.Fatalf("wrong explicit exclusion: B=%+v C=%+v", r.B, r.C)
			}
		})
	}
}

func frameHash(f geometry.Frame) [32]byte {
	h := sha256.New()
	h.Write(f.Wire.Pix)
	var b [18]byte
	for i, depth := range f.DepthQ48 {
		binary.LittleEndian.PutUint64(b[:8], depth)
		binary.LittleEndian.PutUint32(b[8:12], f.Owner[i].Object)
		binary.LittleEndian.PutUint32(b[12:16], f.Owner[i].Face)
		b[16], b[17] = 0, 0
		if f.Owner[i].Clipped {
			b[16] = 1
		}
		if f.Owner[i].Clamped {
			b[17] = 1
		}
		h.Write(b[:])
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func TestRenderRetainsEarlierFrameBuffers(t *testing.T) {
	l := handLibrary(t, false)
	ep := hd3d.FarPtr{Off: 16, Seg: 512}
	in, tables := handInput(t, []realspace.ObjObs{handObject(1, ep)}, handWrite{ep, 15*320 + 15, 7})
	first, err := Render(in, l, tables, 3)
	if err != nil || !first.Passed {
		t.Fatal(err)
	}
	before := frameHash(first.Frame)
	wire := append([]byte(nil), first.Frame.Wire.Pix...)
	// A second same-sized Render clears all geometry, exposing accidental reuse.
	second := Input{Cameras: []realspace.Camera{handCamera()}, Objects: []realspace.ObjObs{handObject(1, ep)}}
	second.Objects[0].Faces[0].AL = 0
	r, err := Render(second, l, tables, 3)
	if err != nil || r.Frame.Stats.WrittenSamples != 0 {
		t.Fatalf("second render: %v", err)
	}
	if before != frameHash(first.Frame) || !bytes.Equal(wire, first.Frame.Wire.Pix) {
		t.Fatal("later Render changed retained wire/owner/depth buffers")
	}
}
