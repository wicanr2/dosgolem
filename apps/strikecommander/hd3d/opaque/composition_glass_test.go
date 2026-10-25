package opaque

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"os"
	"testing"
)

// Source binding tests read the original IFFs and the closed normative tuples.
// No glTF mesh is rendered and no synthetic asset is assigned an original SHA.
func TestGlassClosedBindingsOriginalIFFs(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("original model root unavailable")
	}
	lib, e := realspace.LoadLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	mat, e := LoadMaterialLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	r := &Renderer{lib: lib, materials: mat}
	found := map[[32]byte]bool{}
	count := 0
	for _, entry := range lib.Entries() {
		if entry.Kind != realspace.POLY {
			continue
		}
		m, e := mat.entry(entry.TRE, entry.Name, entry.Kind)
		if e != nil {
			t.Fatal(e)
		}
		if found[m.key] {
			continue
		}
		var cases []glassBinding
		switch m.key {
		case hashHex("9c30f74711a85021af0e4ad4afce8611427fb6cac426df8d17029f9b36af9a83"):
			cases = f16GlassBindings
		case hashHex("e49980538cf6b993d6696c0c4a165157c8ff6c23394913b831ecaca36dc79176"):
			for _, fi := range []int{6, 7, 29, 30, 31, 32, 33, 34, 35} {
				cases = append(cases, glassBinding{lod: 0, face: fi, tri: fi})
			}
		default:
			continue
		}
		found[m.key] = true
		for _, b := range cases {
			o := realspace.ObjObs{RecVT: 0x10c2, LOD: uint8(b.lod), Faces: make([]realspace.FaceObs, b.face+1)}
			for i := range o.Faces {
				o.Faces[i].FaceVT = 0x3495
			}
			if e = r.glassMaterial(o, b.face, []*realspace.ModelEntry{entry}); e != nil {
				t.Fatalf("original IFF%X LOD%d face%d: %v", m.key, b.lod, b.face, e)
			}
			count++
			o.RecVT = 0x314a
			if e = r.glassMaterial(o, b.face, []*realspace.ModelEntry{entry}); e == nil {
				t.Fatal("APPR record got tint134")
			}
			o.RecVT = 0x10c2
			if e = r.glassMaterial(o, 0, []*realspace.ModelEntry{entry}); e == nil {
				t.Fatal("outside allowlist face0 accepted")
			}
			changed := *entry
			changed.VTRI = append([][8]byte(nil), entry.VTRI...)
			changed.VTRI[b.tri][1] ^= 1
			if e = r.glassMaterial(o, b.face, []*realspace.ModelEntry{entry, &changed}); e == nil {
				t.Fatal("conflicting compatible geometry accepted")
			}
		}
	}
	if len(found) != 2 || count != 33 {
		t.Fatalf("original material coverage: IFFs%d tuples%d want2/33", len(found), count)
	}
}
