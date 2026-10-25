package opaque

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"reflect"
	"testing"
)

func TestCompositionTypedFreezePreservesDigestAndNoMutableAliases(t *testing.T) {
	_, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	f := &in.Scene.Objects[0].Faces[0]
	f.Source = &realspace.FaceSourceObs{Proof: &realspace.FaceSourceProof{Bytes: []realspace.SourceBytes{{Data: []byte{1, 2, 3}}}}}
	in.Scene.Raster.Calls[2].Tex.Pixels.Spans = []hd3d.Span{{Off: 7, Data: []byte{11, 13}}}
	want, e := InputDigest(in.Scene)
	if e != nil {
		t.Fatal(e)
	}
	out, n, e := typedFreezeInput(in, 32<<20)
	if e != nil || n == 0 {
		t.Fatal(n, e)
	}
	got, e := InputDigest(out.Scene)
	if e != nil || got != want {
		t.Fatal("typed clone changed legacy digest", e)
	}
	in.Scene.Objects[0].VertXYZ[0] ^= 1
	in.Scene.Objects[0].Faces[0].Source.Proof.Bytes[0].Data[0] = 99
	in.Scene.Raster.Calls[2].Verts[4] = 81
	in.Scene.Raster.Calls[2].Tex.Pixels.Spans[0].Data[0] = 97
	in.Scene.Original.Writer[0] = 71
	in.Scene.Original.CallElements[0] = hd3d.FarPtr{Off: 71}
	if out.Scene.Objects[0].Faces[0].Source.Proof.Bytes[0].Data[0] != 1 || out.Scene.Raster.Calls[2].Verts[4] != 17 || out.Scene.Raster.Calls[2].Tex.Pixels.Spans[0].Data[0] != 11 || out.Scene.Original.Writer[0] == 71 || out.Scene.Original.CallElements[0].Off == 71 {
		t.Fatal("typed input retained mutable caller alias")
	}
	after, e := InputDigest(out.Scene)
	if e != nil || after != want {
		t.Fatal("later caller mutation changed frozen digest", e)
	}
	if !reflect.DeepEqual(out.Scene.Objects[0].Verdict, realspace.ObjVerdict{}) {
		t.Fatal("untrusted selected geometry retained")
	}
	if _, _, e = typedFreezeInput(in, n-1); e == nil {
		t.Fatal("preflight budget ignored")
	}
}
func TestCompositionTypedFreezeActuallyFeedsRendererAndRelease(t *testing.T) {
	c, in, _, budget := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	before := budget.OwnedBytes()
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	in.Opaque.Scene.Raster.Calls[2].Verts[4] = 81
	r, e := s.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	sample, _, ok := r.Sample(20, 41, 0, 0)
	if !ok || !sample.Coverage || sample.Color.Lo != 17 {
		t.Fatal("real Render used caller mutation", sample)
	}
	if e = s.Release(); e != nil {
		t.Fatal(e)
	}
	if budget.OwnedBytes() != before || s.core.input.Scene.Objects != nil {
		t.Fatal("released typed scene kept backing or charge")
	}
	if _, _, ok = r.Sample(20, 41, 0, 0); ok {
		t.Fatal("released output retained qualification")
	}
}
