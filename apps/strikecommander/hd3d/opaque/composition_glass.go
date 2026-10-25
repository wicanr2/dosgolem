package opaque

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"image"
	"reflect"
)

const GlassProfileVersion = "glass-r5-tint134-half-v1"

type glassSurface struct {
	object, face int
	seq          uint16
	triangles    []geometry.MeshTriangle
	bounds       [4]image.Rectangle
}
type glassBinding struct {
	lod, face, tri int
	raw            [8]byte
}

// 013c5.3 fixed F16 LOD0 and explicitly approved LOD1工程 mapping.
var f16GlassBindings = []glassBinding{
	{0, 40, 165, [8]byte{0x02, 0x71, 0x91, 0x75, 0x14, 0x00, 0x01, 0x00}},
	{0, 41, 166, [8]byte{0x02, 0x74, 0x73, 0x8e, 0x14, 0x00, 0x01, 0x00}},
	{0, 42, 167, [8]byte{0x02, 0x74, 0x92, 0x90, 0x14, 0x00, 0x01, 0x00}},
	{0, 43, 168, [8]byte{0x02, 0x90, 0x71, 0x74, 0x14, 0x00, 0x01, 0x00}},
	{0, 44, 169, [8]byte{0x02, 0x74, 0x8e, 0x92, 0x14, 0x00, 0x01, 0x00}},
	{0, 45, 170, [8]byte{0x02, 0x71, 0x90, 0x91, 0x14, 0x00, 0x01, 0x00}},
	{0, 46, 171, [8]byte{0x02, 0x91, 0x8d, 0x75, 0x14, 0x00, 0x01, 0x00}},
	{0, 47, 172, [8]byte{0x02, 0x75, 0x8d, 0x5c, 0x14, 0x00, 0x01, 0x00}},
	{0, 48, 173, [8]byte{0x02, 0x73, 0x8f, 0x8e, 0x14, 0x00, 0x01, 0x00}},
	{0, 49, 174, [8]byte{0x02, 0x73, 0x72, 0x8f, 0x14, 0x00, 0x01, 0x00}},
	{0, 50, 175, [8]byte{0x02, 0x72, 0x6e, 0x8f, 0x14, 0x00, 0x01, 0x00}},
	{0, 51, 176, [8]byte{0x02, 0x8d, 0x6e, 0x5c, 0x14, 0x00, 0x01, 0x00}},
	{0, 116, 241, [8]byte{0x02, 0x92, 0x6a, 0x90, 0x14, 0x00, 0x01, 0x00}},
	{0, 117, 242, [8]byte{0x02, 0x90, 0x8d, 0x91, 0x14, 0x00, 0x01, 0x00}},
	{0, 118, 243, [8]byte{0x02, 0x90, 0x6a, 0x8d, 0x14, 0x00, 0x01, 0x00}},
	{0, 119, 244, [8]byte{0x02, 0x6a, 0x6e, 0x8d, 0x14, 0x00, 0x01, 0x00}},
	{0, 120, 245, [8]byte{0x02, 0x6a, 0x8f, 0x6e, 0x14, 0x00, 0x01, 0x00}},
	{0, 121, 246, [8]byte{0x02, 0x6a, 0x92, 0x8f, 0x14, 0x00, 0x01, 0x00}},
	{0, 122, 247, [8]byte{0x02, 0x8f, 0x92, 0x8e, 0x14, 0x00, 0x01, 0x00}},
	{1, 65, 94, [8]byte{0x02, 0x57, 0x33, 0x21, 0x07, 0x00, 0x02, 0x00}},
	{1, 66, 95, [8]byte{0x02, 0x39, 0x57, 0x36, 0x07, 0x00, 0x02, 0x00}},
	{1, 67, 96, [8]byte{0x02, 0x57, 0x37, 0x33, 0x07, 0x00, 0x02, 0x00}},
	{1, 71, 100, [8]byte{0x02, 0x36, 0x57, 0x21, 0x07, 0x00, 0x02, 0x00}},
	{1, 72, 101, [8]byte{0x02, 0x39, 0x37, 0x57, 0x07, 0x00, 0x02, 0x00}},
}

func (r *Renderer) glassMaterial(o realspace.ObjObs, fi int, candidates []*realspace.ModelEntry) error {
	if o.RecVT != 0x10c2 || o.Faces[fi].FaceVT != 0x3495 || len(candidates) == 0 {
		return fmt.Errorf("unresolved R2 model/class")
	}
	for _, e := range candidates {
		if e.Kind != realspace.POLY {
			return fmt.Errorf("APPR is original R2 fallback")
		}
		m, err := r.materials.entry(e.TRE, e.Name, e.Kind)
		if err != nil {
			return err
		}
		if m.vert != e.VertSHA || !reflect.DeepEqual(m.vtri, e.VTRI) || !reflect.DeepEqual(m.lod, e.LOD) {
			return fmt.Errorf("glass material source geometry/LOD differs")
		}
		if int(o.LOD) >= len(m.lod) || fi >= len(m.lod[o.LOD]) {
			return fmt.Errorf("glass LOD/face")
		}
		tri := int(m.lod[o.LOD][fi])
		if tri >= len(m.vtri) {
			return fmt.Errorf("glass VTRI")
		}
		raw := m.vtri[tri]
		if raw[0] != 2 {
			return fmt.Errorf("glass original class is not2")
		}
		switch m.key {
		case hashHex("9c30f74711a85021af0e4ad4afce8611427fb6cac426df8d17029f9b36af9a83"):
			found := false
			for _, b := range f16GlassBindings {
				if b.lod == int(o.LOD) && b.face == fi && b.tri == tri && b.raw == raw {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("F16 face outside closed material mapping")
			}
		case hashHex("e49980538cf6b993d6696c0c4a165157c8ff6c23394913b831ecaca36dc79176"):
			if o.LOD != 0 || tri != fi || !(fi == 6 || fi == 7 || fi >= 29 && fi <= 35) {
				return fmt.Errorf("MiG29 face outside closed material mapping")
			}
		default:
			return fmt.Errorf("R2 original IFF content not in glass allowlist")
		}
	}
	return nil
}
func (r *Renderer) bindCompositionFace(in Input, l sceneLinks, o realspace.ObjObs, oi, fi, n int, points []geometry.Point3, e *realspace.ModelEntry, candidates []*realspace.ModelEntry) (boundFace, error) {
	bf, err := r.bindFace(in, l, o, oi, fi, n, points, e, candidates)
	if err != nil {
		return bf, err
	}
	if len(bf.deferred) == 0 {
		return bf, nil
	}
	// An APPR remains the original R2 footprint. It is never tint134.
	if e.Kind == realspace.APPR {
		return bf, nil
	}
	if err = r.glassMaterial(o, fi, candidates); err != nil {
		return bf, err
	}
	call := bf.deferred[0].Call
	if !sameXYMultiset(call.Verts, o.Faces[fi].Proj) {
		return bf, fmt.Errorf("R2 original XY geometry differs")
	}
	indices := e.VTRI[e.LOD[o.LOD][fi]]
	var ps [3]geometry.Point3
	for k := range ps {
		ps[k] = points[indices[k+1]]
	}
	cam := in.Scene.Cameras[o.CamIdx]
	gc := geometry.Camera{CX: int16(binary.LittleEndian.Uint16(cam.Raw[8:])), CY: int16(binary.LittleEndian.Uint16(cam.Raw[10:])), FX: int32(binary.LittleEndian.Uint32(cam.Raw[12:])), FY: int32(binary.LittleEndian.Uint32(cam.Raw[16:]))}
	mesh, err := geometry.ProjectFaceSamples(geometry.FaceInput{Vertices: ps, Camera: gc, Flag17BC: o.Flag17BC, Near: o.Near, Ref: geometry.FaceRef{Object: uint32(oi), Face: uint32(fi)}}, n)
	if err != nil {
		return bf, err
	}
	bf.clipped, bf.allClipped = mesh.Info.Clipped, mesh.Info.AllClipped
	bf.glass = &glassSurface{object: oi, face: fi, seq: call.Seq, triangles: mesh.Triangles}
	for phase := 0; phase < 4; phase++ {
		for _, tri := range bf.glass.triangles {
			q, e := geometry.SampleBounds(tri, phase&1, phase>>1)
			if e != nil {
				return bf, e
			}
			bf.glass.bounds[phase] = bf.glass.bounds[phase].Union(q)
		}
	}
	return bf, nil
}
func glassDepth(g glassSurface, x, y, i, j int) (uint64, bool, error) {
	return glassDepthRead(&g, x, y, i, j)
}

// The caller holds the immutable snapshot active during RGB resolution.
func glassDepthRead(g *glassSurface, x, y, i, j int) (uint64, bool, error) {
	if !image.Pt(x, y).In(g.bounds[j*2+i]) {
		return 0, false, nil
	}
	for _, tri := range g.triangles {
		w, ok, e := sampleWeights(tri, x, y, i, j)
		if e != nil {
			return 0, false, e
		}
		if ok {
			return w.depth(), true, nil
		}
	}
	return 0, false, nil
}

var _ hd3d.CallKind
