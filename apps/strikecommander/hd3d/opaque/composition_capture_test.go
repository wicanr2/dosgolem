package opaque

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"testing"
)

func TestCompositionBufCURequiresActualCompletionEvidence(t *testing.T) {
	c, in, pal, _ := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	cap := *in.Opaque.Capture
	cap.Kind = CaptureBufCU
	cap.SourceCS = 0x1234
	cap.SourceIP = 0xa5
	in.Opaque.Capture = &cap
	bytes := append([]byte(nil), in.Opaque.Scene.Raster.Buf3D...)
	good := CompletionImageEvidence{Indexed: bytes, Palette: pal.Colors, Steps: cap.Steps, CUKey: in.Opaque.Identity.CUKey, SourceCS: cap.SourceCS, SourceIP: cap.SourceIP}
	in.Completion = &good
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	s.Release()
	for _, kind := range []string{"absent", "pixels", "palette", "steps", "cu", "cs", "ip"} {
		v := good
		v.Indexed = append([]byte(nil), good.Indexed...)
		q := in
		q.Completion = &v
		switch kind {
		case "absent":
			q.Completion = nil
		case "pixels":
			v.Indexed[0] ^= 1
		case "palette":
			v.Palette[0][0] ^= 1
		case "steps":
			v.Steps++
		case "cu":
			v.CUKey++
		case "cs":
			v.SourceCS++
		case "ip":
			v.SourceIP++
		}
		if snap, e := c.Accept(q); e == nil {
			snap.Release()
			t.Fatal("foreign completion accepted", kind)
		}
	}
	r, old := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	bindRevisionCapture(&old, old.Scene.Raster.Buf3D)
	old.Capture.Kind = CaptureBufCU
	if s, e := r.Accept(old); e == nil {
		s.Release()
		t.Fatal("default013a accepted3b-only completion kind")
	}
}
