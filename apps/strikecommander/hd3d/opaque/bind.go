package opaque

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"math/big"
	"reflect"
	"strings"
)

type faceRef struct{ object, face int }
type sceneLinks struct {
	owners     map[hd3d.FarPtr][]faceRef
	headers    map[hd3d.FarPtr][16]byte
	badHeaders map[hd3d.FarPtr]bool
	calls      map[hd3d.FarPtr][]hd3d.Call
}

func objectVT(v uint16) bool {
	switch v {
	case 0x17c1, 0x17c5, 0x17c9, 0x17cd, 0x17d1, 0x17d5, 0x17d9:
		return true
	}
	return false
}
func qualifyOriginal(in Input, tables hd3d.Tables) (sceneLinks, error) {
	bad := func(s string) (sceneLinks, error) {
		return sceneLinks{}, Failure{Gate: H3, Object: -1, Face: -1, Pass: -1, Reason: s}
	}
	orig := in.Scene.Original
	g := in.Scene.Raster
	if in.ReadFailed || orig == nil || !orig.Valid || !orig.Checked || !orig.Repaintable || g.Key != in.Identity.CUKey || len(g.Buf3D) != hd3d.PixelCount {
		return bad("missing/invalid original group and writer")
	}
	l := sceneLinks{owners: map[hd3d.FarPtr][]faceRef{}, headers: map[hd3d.FarPtr][16]byte{}, badHeaders: map[hd3d.FarPtr]bool{}, calls: map[hd3d.FarPtr][]hd3d.Call{}}
	for oi, o := range in.Scene.Objects {
		for fi, f := range o.Faces {
			if f.AL != 0 {
				p := hd3d.FarPtr{Off: f.Elem.Off, Seg: f.Elem.Seg}
				if p != (hd3d.FarPtr{}) {
					l.owners[p] = append(l.owners[p], faceRef{oi, fi})
				}
			}
		}
	}
	for _, e := range in.Scene.Elements {
		if h, ok := l.headers[e.Pointer]; ok && h != e.Header {
			l.badHeaders[e.Pointer] = true
		}
		l.headers[e.Pointer] = e.Header
	}
	calls := append([]hd3d.Call(nil), g.Calls...)
	if len(calls) > 65535 || len(orig.CallElements) != len(calls) {
		return bad("invalid original sequence/element map")
	}
	for i := range calls {
		c := &calls[i]
		if c.Seq == 0xffff || int(c.Seq) != i || c.ReadFailed {
			return bad("invalid sequence or failed original read")
		}
		if p, ok := orig.CallElements[c.Seq]; !ok || p != c.Element {
			return bad("writer element identity differs")
		}
		h, known := l.headers[c.Element]
		if c.Element != (hd3d.FarPtr{}) && (!known || l.badHeaders[c.Element]) && len(l.owners[c.Element]) == 0 {
			return bad("unlinked element has no unique raw header")
		}
		c.Object = known && !l.badHeaders[c.Element] && objectVT(binary.LittleEndian.Uint16(h[:]))
		if c.Object && len(l.owners[c.Element]) == 0 {
			return bad("fixed-set object call has no E1 owner")
		}
		l.calls[c.Element] = append(l.calls[c.Element], *c)
	}
	for i, seq := range orig.Writer {
		if !orig.Written[i] || seq == 0xffff || int(seq) >= len(calls) || orig.Pixels[i] != g.Buf3D[i] {
			return bad("original pixel/writer not fully qualified")
		}
	}
	g.Calls = calls
	replay := hd3d.Replay(g, tables)
	if !replay.Valid || !replay.Checked || !replay.Repaintable || replay.Pixels != orig.Pixels || replay.Writer != orig.Writer {
		return bad(fmt.Sprintf("classified replay differs valid=%t checked=%t repaintable=%t pixelsEqual=%t writerEqual=%t faults=%v captureShort=%d firstMismatch=%d", replay.Valid, replay.Checked, replay.Repaintable, replay.Pixels == orig.Pixels, replay.Writer == orig.Writer, replay.Diagnostics.Faults, replay.Diagnostics.CaptureShort, replay.Diagnostics.FirstMismatch))
	}
	return l, nil
}

type boundPass struct {
	kind       hd3d.CallKind
	call       hd3d.Call
	constant   SampleColor
	word, u, v [3]uint16
	band       Band
	tex        texture
}
type triPass struct {
	bound                  *boundPass
	word, u, v             [3]*big.Rat
	wordInt, uInt, vInt    [3]uint16
	integerWord, integerUV bool
}
type boundTriangle struct {
	tri    geometry.MeshTriangle
	passes []triPass
}
type boundFace struct {
	index               int
	triangles           []boundTriangle
	clipped, allClipped bool
	deferred            []DeferredOp
}

func supported(faceVT uint16, k string) bool {
	switch faceVT {
	case 0x3441:
		return k == "R1"
	case 0x3415:
		return k == "R1" || k == "R3"
	case 0x338d:
		return k == "R1/R4" || k == "R3/R4"
	case 0x33b9:
		return k == "R1/R4"
	case 0x33e9, 0x335d:
		return k == "R4"
	case 0x3495:
		return k == "R2"
	}
	return false
}
func permuteXY(c hd3d.Call, f realspace.FaceObs) [][3]int {
	var out [][3]int
	for _, p := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		ok := true
		for i, j := range p {
			ok = ok && bytes.Equal(c.Verts[i*22:i*22+4], f.Proj[j][:4])
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}
func sourceAttribute(v [3]uint16, s geometry.CornerSource) (*big.Rat, error) {
	if s.Kind == geometry.SourceOriginal {
		return clipAttribute(v, 0, s.Original, 0, 0, 0)
	}
	if s.Kind == geometry.SourceNearEdge {
		return clipAttribute(v, 1, s.In, s.Out, s.N, s.D)
	}
	return nil, fmt.Errorf("unknown geometric corner source")
}
func (r *Renderer) bindR4(c hd3d.Call, f realspace.FaceObs, o realspace.ObjObs, fi int, candidates []*realspace.ModelEntry) (boundPass, error) {
	b := boundPass{kind: hd3d.R4, call: c}
	perms := permuteXY(c, f)
	if len(perms) == 0 {
		return b, fmt.Errorf("R4 XY has no original corner bijection")
	}
	for _, q := range f.Proj {
		if q[17]&4 == 0 {
			return b, fmt.Errorf("R4 original XY not projected")
		}
	}
	var expected [3][2]uint16
	var chosen decodedTexture
	for ci, e := range candidates {
		m, err := r.materials.entry(e.TRE, e.Name, e.Kind)
		if err != nil {
			return b, err
		}
		if m.vert != e.VertSHA || !reflect.DeepEqual(m.vtri, e.VTRI) || !reflect.DeepEqual(m.lod, e.LOD) {
			return b, fmt.Errorf("material source geometry/LOD differs")
		}
		if int(o.LOD) >= len(m.lod) || fi >= len(m.lod[o.LOD]) {
			return b, fmt.Errorf("original material face out of range")
		}
		tri := m.lod[o.LOD][fi]
		if tri > 255 {
			return b, fmt.Errorf("UVXY byte triangle index cannot identify face")
		}
		var uv [8]byte
		found := 0
		for _, q := range m.uv {
			if uint16(q[0]) == tri {
				uv = q
				found++
			}
		}
		if found != 1 || int(uv[1]) >= len(m.textures) {
			return b, fmt.Errorf("UVXY/texture slot not unique")
		}
		var values [3][2]uint16
		for j := range values {
			values[j] = [2]uint16{uint16(uv[2+2*j]) * 256, uint16(uv[3+2*j]) * 256}
		}
		match := false
		for _, perm := range perms {
			ok := true
			for i, j := range perm {
				ok = ok && binary.LittleEndian.Uint16(c.Verts[22*i+6:]) == values[j][0] && binary.LittleEndian.Uint16(c.Verts[22*i+8:]) == values[j][1]
			}
			match = match || ok
		}
		if !match {
			return b, fmt.Errorf("captured XY/UV does not match every compatible original")
		}
		tx, err := r.materials.texture(m.textures[uv[1]])
		if err != nil {
			return b, err
		}
		if c.Params[6] != tx.w {
			return b, fmt.Errorf("R4 pitch differs from original texture")
		}
		if ci == 0 {
			expected, chosen = values, tx
		} else if values != expected || tx.w != chosen.w || tx.h != chosen.h || !bytes.Equal(tx.pixels, chosen.pixels) {
			return b, fmt.Errorf("compatible originals have distinct UV/texture content")
		}
	}
	if len(candidates) == 0 {
		return b, fmt.Errorf("no compatible original material")
	}
	// Content never chooses a candidate. Check all already bound source bytes;
	// saved addresses outside W*H remain captured memory, not guessed file wrap.
	for _, s := range c.Tex.Pixels.Spans {
		for i, v := range s.Data {
			a := uint16(uint32(s.Off) + uint32(i) - uint32(c.Params[4]))
			if int(a) < len(chosen.pixels) && v != chosen.pixels[a] {
				return b, fmt.Errorf("bound original texture byte differs at%04x", a)
			}
		}
	}
	for j := range expected {
		b.u[j], b.v[j] = expected[j][0], expected[j][1]
	}
	var err error
	b.tex, err = compileTexture(c)
	return b, err
}

// R1 has no per-corner material attributes, so every XY bijection is legal.
// Matching consumes each original corner once, including repeated coordinates.
func sameXYMultiset(raw []byte, proj [3][18]byte) bool {
	if len(raw) != 66 {
		return false
	}
	var used [3]bool
	for i := 0; i < 3; i++ {
		found := false
		for j := 0; j < 3; j++ {
			if !used[j] && bytes.Equal(raw[i*22:i*22+4], proj[j][:4]) {
				used[j] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *Renderer) bindFace(in Input, l sceneLinks, o realspace.ObjObs, oi, fi, n int, points []geometry.Point3, e *realspace.ModelEntry, candidates []*realspace.ModelEntry) (boundFace, error) {
	f := o.Faces[fi]
	out := boundFace{index: fi}
	ptr := hd3d.FarPtr{Off: f.Elem.Off, Seg: f.Elem.Seg}
	owners := l.owners[ptr]
	if ptr == (hd3d.FarPtr{}) || len(owners) != 1 || owners[0] != (faceRef{oi, fi}) {
		return out, fmt.Errorf("E1 ownership not unique")
	}
	h, ok := l.headers[ptr]
	if !ok || l.badHeaders[ptr] || !objectVT(binary.LittleEndian.Uint16(h[:])) {
		return out, fmt.Errorf("missing/conflicting/outside-set raw element header")
	}
	calls := l.calls[ptr]
	kinds := make([]string, len(calls))
	for i, c := range calls {
		kinds[i] = string(c.Kind)
		if c.ReadFailed || c.Params[1] != 3 || len(c.Verts) != 66 {
			return out, fmt.Errorf("material requires original n3 raw66")
		}
	}
	if !supported(f.FaceVT, strings.Join(kinds, "/")) {
		return out, fmt.Errorf("unsupported original class/pass grammar")
	}
	if len(calls) == 1 && calls[0].Kind == hd3d.R2 {
		out.deferred = []DeferredOp{{Object: oi, Face: fi, Call: calls[0]}}
		return out, nil
	}
	passes := make([]boundPass, 0, len(calls))
	for _, c := range calls {
		p := boundPass{kind: c.Kind, call: c}
		switch c.Kind {
		case hd3d.R1:
			if !sameXYMultiset(c.Verts, f.Proj) {
				return out, fmt.Errorf("R1 raw XY differs from original face geometry")
			}
			var err error
			p.constant, err = r1Color(c)
			if err != nil {
				return out, err
			}
		case hd3d.R3:
			if err := realspace.ValidateFaceSource(&o, &f); err != nil {
				var sig *realspace.SourceSignatureError
				if errors.As(err, &sig) {
					return out, Failure{Gate: H4, Object: oi, Face: fi, Pass: int(c.Seq), Reason: err.Error()}
				}
				return out, err
			}
			if c.Steps <= f.Source.ElementReturn.Steps {
				return out, fmt.Errorf("R3 call predates its E1 source")
			}
			if f.Source.Generation != in.Identity.SignatureGeneration {
				return out, fmt.Errorf("R3 source epoch differs from scene")
			}
			if c.Params[2] != f.Elem.Off+6 || c.Params[3] != f.Elem.Seg {
				return out, fmt.Errorf("R3 vertex far is not active element+6")
			}
			for i := 0; i < 3; i++ {
				if !bytes.Equal(c.Verts[i*22:i*22+4], f.Source.BuildCall.FullProj[i][:4]) {
					return out, fmt.Errorf("R3 call differs from constructor slot order")
				}
				p.word[i] = binary.LittleEndian.Uint16(c.Verts[i*22+4:])
			}
			found := 0
			for _, band := range r.materials.bands {
				ok := true
				for _, v := range p.word {
					ok = ok && uint64(v) >= uint64(band.Base)*256 && uint64(v) <= uint64(band.Base+band.Range)*256
				}
				if ok {
					p.band = band
					found++
				}
			}
			if found != 1 {
				return out, fmt.Errorf("R3 original band not unique")
			}
		case hd3d.R4:
			var err error
			p, err = r.bindR4(c, f, o, fi, candidates)
			if err != nil {
				return out, err
			}
		default:
			return out, fmt.Errorf("unknown material pass")
		}
		passes = append(passes, p)
	}
	indices := e.VTRI[e.LOD[o.LOD][fi]]
	var ps [3]geometry.Point3
	for i := range ps {
		ps[i] = points[indices[i+1]]
	}
	cam := in.Scene.Cameras[o.CamIdx]
	gc := geometry.Camera{CX: int16(binary.LittleEndian.Uint16(cam.Raw[8:])), CY: int16(binary.LittleEndian.Uint16(cam.Raw[10:])), FX: int32(binary.LittleEndian.Uint32(cam.Raw[12:])), FY: int32(binary.LittleEndian.Uint32(cam.Raw[16:]))}
	clamped := false
	for _, q := range f.Proj {
		sx, sy := int16(binary.LittleEndian.Uint16(q[:])), int16(binary.LittleEndian.Uint16(q[2:]))
		clamped = clamped || (q[17]&4 != 0 && (sx == 0x7d00 || sx == -0x7d00 || sy == 0x7d00 || sy == -0x7d00))
	}
	mesh, err := geometry.ProjectFaceSamples(geometry.FaceInput{Vertices: ps, Camera: gc, Flag17BC: o.Flag17BC, Near: o.Near, Ref: geometry.FaceRef{Object: uint32(oi), Face: uint32(fi), Clamped: clamped}}, n)
	if err != nil {
		return out, err
	}
	out.clipped, out.allClipped = mesh.Info.Clipped, mesh.Info.AllClipped
	for _, tri := range mesh.Triangles {
		bt := boundTriangle{tri: tri}
		for pi := range passes {
			p := &passes[pi]
			tp := triPass{bound: p}
			for j, v := range tri.Vertices {
				switch p.kind {
				case hd3d.R3:
					tp.word[j], err = sourceAttribute(p.word, v.Source)
				case hd3d.R4:
					tp.u[j], err = sourceAttribute(p.u, v.Source)
					if err == nil {
						tp.v[j], err = sourceAttribute(p.v, v.Source)
					}
				}
				if err != nil {
					return out, err
				}
			}
			tp.wordInt, tp.integerWord = integerAttrs(tp.word)
			tp.uInt, tp.integerUV = integerAttrs(tp.u)
			var vi bool
			tp.vInt, vi = integerAttrs(tp.v)
			tp.integerUV = tp.integerUV && vi
			bt.passes = append(bt.passes, tp)
		}
		out.triangles = append(out.triangles, bt)
	}
	return out, nil
}
func materialObjectKey(fs []boundFace, id SceneIdentity) [32]byte {
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(id)
	seen := map[uint16]bool{}
	for _, f := range fs {
		for _, op := range f.deferred {
			if !seen[op.Call.Seq] {
				_ = enc.Encode(op.Call)
				seen[op.Call.Seq] = true
			}
		}
		for _, t := range f.triangles {
			for _, p := range t.passes {
				if !seen[p.bound.call.Seq] {
					_ = enc.Encode(p.bound.call)
					seen[p.bound.call.Seq] = true
				}
			}
		}
	}
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}
