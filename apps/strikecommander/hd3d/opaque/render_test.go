package opaque

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"os"
	"path/filepath"
	"testing"
)

type faceSpec struct {
	uvShift       byte
	textureAlt    bool
	vt            uint16
	kinds         []hd3d.CallKind
	color         uint16
	missingSource bool
}
type objectSpec struct{ faces []faceSpec }

func packLiteralTexture(pixels []byte) []byte {
	codes := append([]int{256}, make([]int, len(pixels))...)
	for i, v := range pixels {
		codes[i+1] = int(v)
	}
	codes = append(codes, 257)
	bits := make([]byte, (9*len(codes)+7)/8)
	for i, c := range codes {
		for j := 0; j < 9; j++ {
			if c&(1<<j) != 0 {
				bits[(9*i+j)/8] |= 1 << uint((9*i+j)%8)
			}
		}
	}
	return append([]byte("TEST\x00\x00\x00\x00\x03\x00\x01\x00LZ"), bits...)
}
func fixtureArchive(name string, raw []byte) []byte {
	b := make([]byte, 82)
	binary.LittleEndian.PutUint32(b, 1)
	binary.LittleEndian.PutUint32(b[4:], 82)
	b[8] = 1
	copy(b[9:74], name)
	binary.LittleEndian.PutUint32(b[74:], 82)
	binary.LittleEndian.PutUint32(b[78:], uint32(len(raw)))
	return append(b, raw...)
}
func fixtureSource(o realspace.ObjObs, f realspace.FaceObs) *realspace.FaceSourceObs {
	base := uint16(0x241)
	proof := &realspace.FaceSourceProof{Rule: realspace.FaceSourceRule, Base: base, Generation: 1}
	for _, s := range realspace.FaceSourceSignatures() {
		proof.Bytes = append(proof.Bytes, realspace.SourceBytes{Seg: base + s.Seg, Off: s.Off, Data: realspace.SourceSignatureBytes(s, base)})
	}
	var raw [18]byte
	binary.LittleEndian.PutUint16(raw[:], f.FaceVT)
	var fars [3]realspace.FarPtr
	for i, at := range []int{6, 10, 14} {
		fars[i] = realspace.FarPtr{Seg: 0x5000, Off: uint16(0x100 + 18*i)}
		binary.LittleEndian.PutUint16(raw[at:], fars[i].Off)
		binary.LittleEndian.PutUint16(raw[at+2:], fars[i].Seg)
	}
	ctor := uint16(0x10ec)
	if f.FaceVT == 0x338d {
		ctor = 0x1b70
	}
	r := realspace.FaceSourceRole{FaceArgument: f.Face, HasFaceArgument: true, Generation: 1, SS: 0x9000, SP: 0x100, BP: 0x200, SI: f.SI, Rec: o.Rec, Face: f.Face, Raw: raw, FullFar: fars, FullProj: f.Proj, ConventionProj: f.Proj, MethodF1: realspace.FarPtr{Seg: base + 0x20a8, Off: 2}, MethodBuild: realspace.FarPtr{Seg: base + 0x2579, Off: ctor}}
	out := &realspace.FaceSourceObs{Generation: 1, O1Steps: o.Steps, Element: f.Elem, Proof: proof}
	r.Steps = o.Steps + uint64(f.SI)*4 + 1
	out.F1Call = r
	r.Steps++
	out.F1Return = r
	r.Steps++
	out.BuildCall = r
	r.Steps++
	out.ElementReturn = r
	return out
}
func sceneOracle(t *testing.T, specs ...objectSpec) (*Renderer, Input) {
	t.Helper()
	root := t.TempDir()
	empty := []byte{0, 0, 0, 0, 8, 0, 0, 0}
	var assets []Asset
	var archive []byte
	verts := make([]byte, 36)
	binary.LittleEndian.PutUint32(verts[12:], 256)
	binary.LittleEndian.PutUint32(verts[28:], 256)
	for oi, o := range specs {
		var vt, uv, lod []byte
		texturePixels := []byte{255, 255, 255}
		lod = make([]byte, 4)
		for fi, f := range o.faces {
			classes := map[uint16]byte{0x3441: 4, 0x3415: 5, 0x33e9: 6, 0x33b9: 7, 0x338d: 8, 0x346d: 1, 0x3495: 2}
			vt = append(vt, classes[f.vt], 0, 1, 2, 0, 0, 0, 0)
			uv = append(uv, byte(fi), 0, f.uvShift, 0, 1, 0, 0, 1)
			if f.textureAlt {
				texturePixels[0] = 7
			}
			lod = binary.LittleEndian.AppendUint16(lod, uint16(fi))
		}
		raw := iffForm("REAL", iffForm("APPR", iffForm("POLY", iffChunk("VERT", verts), iffForm("DETA", iffChunk("LVL0", lod)), iffForm("TRIS", iffChunk("VTRI", vt), iffChunk("UVXY", uv), iffForm("TXMS", iffChunk("TXMP", packLiteralTexture(texturePixels)))))))
		name := fmt.Sprintf(`..\..\DATA\OBJECTS\X%d.IFF`, oi)
		assets = append(assets, Asset{TRE: "BIGTRE.TRE", Name: name, IFF: raw})
		if oi == 0 {
			archive = fixtureArchive(name, raw)
		} else { // Build one directory with independently prescribed asset bodies.
			all := assets
			archive = make([]byte, 8+74*len(all))
			binary.LittleEndian.PutUint32(archive, uint32(len(all)))
			binary.LittleEndian.PutUint32(archive[4:], uint32(len(archive)))
			for i, a := range all {
				at := 8 + 74*i
				archive[at] = 1
				copy(archive[at+1:at+66], a.Name)
				binary.LittleEndian.PutUint32(archive[at+66:], uint32(len(archive)))
				binary.LittleEndian.PutUint32(archive[at+70:], uint32(len(a.IFF)))
				archive = append(archive, a.IFF...)
			}
		}
	}
	for name, b := range map[string][]byte{"BIGTRE.TRE": archive, "LILTRE.TRE": empty, "TOBIGTRE.TRE": empty} {
		if e := os.WriteFile(filepath.Join(root, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	lib, e := realspace.LoadLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	rang := []byte{1, 0, 1, 0, 99, 0, 30, 0, 0, 0, 0, 0, 0, 0, 0}
	mat, e := NewMaterialLibrary(assets, iffForm("PALT", iffChunk("RANG", rang)))
	if e != nil {
		t.Fatal(e)
	}
	var cam realspace.Camera
	cam.CamObj = 0x1234
	binary.LittleEndian.PutUint16(cam.Raw[8:], 10)
	binary.LittleEndian.PutUint16(cam.Raw[10:], 20)
	binary.LittleEndian.PutUint32(cam.Raw[12:], 256)
	binary.LittleEndian.PutUint32(cam.Raw[16:], 256)
	scene := model.Input{Key: 7, Cameras: []realspace.Camera{cam}, Raster: hd3d.Group{Key: 7, Buf3D: make([]byte, 64000), List: make([]uint16, 201)}, Original: &hd3d.Result{Valid: true, Checked: true, Repaintable: true, CallElements: map[uint16]hd3d.FarPtr{}}}
	for y := range scene.Raster.List {
		scene.Raster.List[y] = uint16(16 + y*320)
	}
	desc := [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}
	scene.Raster.Calls = append(scene.Raster.Calls, hd3d.Call{Seq: 0, Kind: hd3d.R6, Desc: desc, Params: [13]uint16{0, 43, 32, 160, 32}, Aux: hd3d.AuxRef{HasFlags: true, Color0: 9, Color1: 9}}, hd3d.Call{Seq: 1, Kind: hd3d.BlackBox, Desc: desc, Patch: make([]hd3d.PixelSet, 64000)})
	for i := 0; i < 64000; i++ {
		scene.Raster.Calls[1].Patch[i] = hd3d.PixelSet{Index: uint32(i)}
		scene.Original.Writer[i] = 1
		scene.Original.Written[i] = true
	}
	scene.Original.CallElements[0] = hd3d.FarPtr{}
	scene.Original.CallElements[1] = hd3d.FarPtr{}
	for oi, spec := range specs {
		o := realspace.ObjObs{Steps: uint64(1 + oi*100), Rec: realspace.FarPtr{Seg: 0x4000, Off: uint16(oi * 0x60)}, RecVT: 0x10c2, CamObj: cam.CamObj, CamIdx: 0, VertRT: [][3]int32{{0, 0, 0}, {256, 0, 0}, {0, 256, 0}}, VertXYZ: append([]byte(nil), verts...), VertSHA: sha256.Sum256(verts), NFaces: int16(len(spec.faces)), Near: 256, FaceList: make([]uint16, len(spec.faces))}
		binary.LittleEndian.PutUint16(o.RecRaw[:], 0x10c2)
		binary.LittleEndian.PutUint16(o.ModelHead[2:], 3)
		binary.LittleEndian.PutUint16(o.ModelHead[0x42:], uint16(o.NFaces))
		for i, n := range [12]int32{256, 0, 0, 0, 0, 0, 256, 256, 0, 256, 0, 0} {
			binary.LittleEndian.PutUint32(o.RecRaw[0x23+i*4:], uint32(n))
		}
		for fi, fs := range spec.faces {
			o.FaceList[fi] = uint16(fi)
			f := realspace.FaceObs{SI: uint16(fi), Face: realspace.FarPtr{Seg: 0x4100, Off: uint16((oi*20 + fi) * 18)}, FaceVT: fs.vt, Returned: true, AL: 1, VIdx: [3]int16{0, 1, 2}, Elem: realspace.FarPtr{Seg: 0x7000, Off: uint16(0x100 + oi*0x800 + fi*0x100)}}
			xy := [3][2]int16{{10, 20}, {11, 20}, {10, 19}}
			xyz := [3][3]int32{{0, 256, 0}, {256, 256, 0}, {0, 256, 256}}
			for j := 0; j < 3; j++ {
				binary.LittleEndian.PutUint16(f.Proj[j][:], uint16(xy[j][0]))
				binary.LittleEndian.PutUint16(f.Proj[j][2:], uint16(xy[j][1]))
				for k, n := range xyz[j] {
					binary.LittleEndian.PutUint32(f.Proj[j][4+k*4:], uint32(n))
				}
				f.Proj[j][17] = 6
			}
			if !fs.missingSource && (fs.vt == 0x3415 || fs.vt == 0x338d) {
				f.Source = fixtureSource(o, f)
			}
			o.Faces = append(o.Faces, f)
			ptr := hd3d.FarPtr{Off: f.Elem.Off, Seg: f.Elem.Seg}
			var header [16]byte
			binary.LittleEndian.PutUint16(header[:], 0x17d1)
			scene.Elements = append(scene.Elements, model.Element{Pointer: ptr, Header: header})
			for _, kind := range fs.kinds {
				seq := uint16(len(scene.Raster.Calls))
				c := hd3d.Call{Seq: seq, Steps: uint64(1000 + seq), Kind: kind, Element: ptr, Desc: desc, Verts: make([]byte, 66)}
				c.Params[1], c.Params[2], c.Params[3] = 3, f.Elem.Off+6, f.Elem.Seg
				for j := 0; j < 3; j++ {
					binary.LittleEndian.PutUint16(c.Verts[j*22:], uint16(xy[j][0]))
					binary.LittleEndian.PutUint16(c.Verts[j*22+2:], uint16(xy[j][1]))
					binary.LittleEndian.PutUint16(c.Verts[j*22+4:], fs.color)
					if j == 1 {
						binary.LittleEndian.PutUint16(c.Verts[j*22+6:], 256)
					}
					if j == 2 {
						binary.LittleEndian.PutUint16(c.Verts[j*22+8:], 256)
					}
				}
				if kind == hd3d.R4 {
					c.Params[6] = 3
					c.Tex.Pixels.Spans = []hd3d.Span{{Off: 0, Data: []byte{255, 255, 255, 255, 255, 255}}}
				} else if kind == hd3d.R2 {
					c.Table = make([]byte, 256)
					for j := range c.Table {
						c.Table[j] = byte(j)
					}
					// R-L<=1: originalR2 does not write this thin triangle.
				} else {
					value := byte(fs.color)
					if kind == hd3d.R3 {
						value = byte(fs.color >> 8)
					}
					for _, i := range []int{19*320 + 10, 20*320 + 10, 20*320 + 11} {
						scene.Raster.Buf3D[i] = value
						scene.Original.Pixels[i] = value
						scene.Original.Writer[i] = seq
						scene.Original.ObjectWritten[i] = true
					}
				}
				scene.Raster.Calls = append(scene.Raster.Calls, c)
				scene.Original.CallElements[seq] = ptr
			}
		}
		scene.Objects = append(scene.Objects, o)
	}
	// Constant9 horizon initializes all pixels; the explicit full BlackBox patch
	// then clears them to0. Expected object bytes above remain literal.
	table := make([]byte, 200)
	for i := range table {
		table[i] = 9
	}
	for i, v := range []uint16{8, 56, 104, 152} {
		binary.LittleEndian.PutUint16(table[192+2*i:], v)
	}
	r, e := NewRenderer(lib, mat, hd3d.Tables{R6Table: table})
	if e != nil {
		t.Fatal(e)
	}
	in := Input{Identity: literalIdentity(), Scene: scene, SignatureValid: true}
	in.Identity.Assets = mat.Key()
	in.Identity.Group, e = InputDigest(scene)
	if e != nil {
		t.Fatal(e)
	}
	return r, in
}
func drawOracle(t *testing.T, r *Renderer, in Input, n int) (*Snapshot, *Result) {
	t.Helper()
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Render(n)
	if e != nil {
		t.Fatal(e)
	}
	return s, out
}
func TestFullRendererR1AndR3LiteralSamples(t *testing.T) {
	for _, tc := range []struct {
		vt    uint16
		kind  hd3d.CallKind
		color uint16
		want  SampleColor
	}{{0x3441, hd3d.R1, 0xa511, SampleColor{17, 17, 0}}, {0x3415, hd3d.R3, 1344, SampleColor{5, 6, 1 << 22}}} {
		r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: tc.vt, kinds: []hd3d.CallKind{tc.kind}, color: tc.color}}})
		s, out := drawOracle(t, r, in, 2)
		sample, ok := out.Sample(20, 41, 0, 0)
		if !ok || !sample.Coverage || sample.Color != tc.want || sample.DepthQ48 != 1<<40 {
			t.Fatal(tc, sample, out.Report())
		}
		if out.Report().Diagnostics.H2Objects != 0 {
			t.Fatal(out.Report())
		}
		first := out.Hash()
		_, e := s.Render(3)
		if e != nil || first != out.Hash() {
			t.Fatal("first result changed", e)
		}
		s.Release()
	}
}
func TestMultiPassFFAndEqualDepthOrdering(t *testing.T) {
	for _, tc := range []struct {
		vt    uint16
		kinds []hd3d.CallKind
		color uint16
		want  SampleColor
	}{{0x33b9, []hd3d.CallKind{hd3d.R1, hd3d.R4}, 17, SampleColor{17, 17, 0}}, {0x338d, []hd3d.CallKind{hd3d.R3, hd3d.R4}, 1344, SampleColor{5, 6, 1 << 22}}} {
		r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: tc.vt, kinds: tc.kinds, color: tc.color}}})
		s, out := drawOracle(t, r, in, 2)
		p, _ := out.Sample(20, 41, 0, 0)
		if !p.Coverage || p.Color != tc.want {
			t.Fatal("FF lost bottom", p, out.Report())
		}
		s.Release()
	}
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}}, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	s, out := drawOracle(t, r, in, 2)
	p, _ := out.Sample(20, 41, 0, 0)
	if !p.Coverage || p.Owner.Object != 1 || p.Color.Lo != 17 {
		t.Fatal("transparent object retained ghost depth", p, out.Report())
	}
	s.Release()
	r, in = sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}}, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 23}}})
	s, out = drawOracle(t, r, in, 2)
	p, _ = out.Sample(20, 41, 0, 0)
	if p.Owner.Object != 0 || p.Color.Lo != 17 {
		t.Fatal("equal depth did not keep first submission", p, out.Report())
	}
	s.Release()
}
func TestH2ObjectAtomicityAndReadOnlyInput(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}, {vt: 0x3415, kinds: []hd3d.CallKind{hd3d.R3}, color: 1344, missingSource: true}}})
	before, _ := InputDigest(in.Scene)
	s, out := drawOracle(t, r, in, 2)
	if out.Report().Diagnostics.H2Objects != 1 {
		t.Fatal(out.Report())
	}
	p, _ := out.Sample(20, 41, 0, 0)
	if p.Coverage {
		t.Fatal("H2 left partial depth/color", p)
	}
	after, _ := InputDigest(in.Scene)
	if before != after {
		t.Fatal("source group mutated")
	}
	s.Release()
}

func TestMissingHeaderUnknownClassAndNoWriterFailClosed(t *testing.T) {
	for _, which := range []string{"header", "outside-set", "unrecognized-class", "writer"} {
		t.Run(which, func(t *testing.T) {
			vt := uint16(0x3441)
			if which == "unrecognized-class" {
				vt = 0x346d
			}
			r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: vt, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
			switch which {
			case "header":
				in.Scene.Elements = nil
			case "outside-set":
				binary.LittleEndian.PutUint16(in.Scene.Elements[0].Header[:], 0x184a)
			case "writer":
				in.Scene.Original = nil
			}
			in.Identity.Group, _ = InputDigest(in.Scene)
			s, e := r.Accept(in)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Release()
			out, e := s.Render(2)
			if which == "writer" {
				requireGate(t, e, H3)
				return
			}
			if e != nil || out.Report().Diagnostics.H2Objects != 1 {
				t.Fatal(out, e)
			}
			p, _ := out.Sample(20, 41, 0, 0)
			if p.Coverage {
				t.Fatal("failed source drew", p)
			}
		})
	}
}
func TestH1ReadFailureAndCallObjectIgnored(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	s, out := drawOracle(t, r, in, 2)
	first := out.Hash()
	s.Release()
	for i := range in.Scene.Raster.Calls {
		in.Scene.Raster.Calls[i].Object = !in.Scene.Raster.Calls[i].Object
	}
	in.Identity.Group, _ = InputDigest(in.Scene)
	s, out = drawOracle(t, r, in, 2)
	if out.Hash() != first {
		t.Fatal("trusted Call.Object")
	}
	s.Release()
	in.Scene.Objects[0].ReadFailed = true
	in.Identity.Group, _ = InputDigest(in.Scene)
	s, out = drawOracle(t, r, in, 2)
	if out.Report().Diagnostics.H1Objects != 1 || out.Report().Diagnostics.WrittenSamples != 0 {
		t.Fatal(out.Report())
	}
	s.Release()
}
func TestR2RawDeferredAndH4ByteMismatch(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3495, kinds: []hd3d.CallKind{hd3d.R2}, color: 17}}})
	s, out := drawOracle(t, r, in, 2)
	if !out.Report().Needs3b || len(out.DeferredOps()) != 1 {
		t.Fatal(out.Report())
	}
	ops := out.DeferredOps()
	ops[0].Call.Table[0] = 99
	if out.DeferredOps()[0].Call.Table[0] != 0 {
		t.Fatal("deferred table alias")
	}
	s.Release()
	r, in = sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3415, kinds: []hd3d.CallKind{hd3d.R3}, color: 1344}}})
	in.Scene.Objects[0].Faces[0].Source.Proof.Bytes[0].Data[0] ^= 1
	in.Identity.Group, _ = InputDigest(in.Scene)
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Render(2)
	requireGate(t, e, H4)
	_, e = s.Render(3)
	requireGate(t, e, H4)
	s.Release()
}

func TestEveryCompatibleOriginalMustAgree(t *testing.T) {
	for _, kind := range []string{"UV", "texture"} {
		t.Run(kind, func(t *testing.T) {
			b := faceSpec{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}
			if kind == "UV" {
				b.uvShift = 1
			} else {
				b.textureAlt = true
			}
			r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}}, objectSpec{[]faceSpec{b}})
			s, out := drawOracle(t, r, in, 2)
			defer s.Release()
			if out.Report().Diagnostics.H2Objects != 2 || out.Report().Diagnostics.SuccessfulObjects != 0 {
				t.Fatal("selected/filter-first compatible source", out.Report())
			}
		})
	}
}
func TestFFShortCircuitWithMissingUnusedShadeAddress(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}})
	c := &in.Scene.Raster.Calls[2]
	c.Params[7], c.Params[8] = 0x100, 0x9000
	c.Tex.Shade.Spans = []hd3d.Span{{Off: 0x100, Data: []byte{0}}}
	in.Identity.Group, _ = InputDigest(in.Scene)
	s, out := drawOracle(t, r, in, 2)
	defer s.Release()
	p, _ := out.Sample(20, 41, 0, 0)
	if p.Coverage || out.Report().Diagnostics.H2Objects != 0 || out.Report().Diagnostics.TransparentSamples == 0 {
		t.Fatal(out.Report(), p)
	}
}
func TestInputFreezeAndSerializedSelectedIgnored(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	in.Scene.Objects[0].Verdict = realspace.ObjVerdict{Status: realspace.Passed, Selected: &realspace.ModelEntry{Verts: [][3]int32{{999, 999, 999}}}}
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Release()
	in.Scene.Raster.Calls[2].Verts[4] = 23
	in.Scene.Objects[0].VertRT[0][0] = 999
	out, e := s.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := out.Sample(20, 41, 0, 0)
	if !p.Coverage || p.Color.Lo != 17 {
		t.Fatal("caller changed frozen input or trusted Selected", p, out.Report())
	}
}
