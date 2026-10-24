package opaque

import (
	"crypto/sha256"
	"testing"
)

func literalIdentity() SceneIdentity {
	return SceneIdentity{EXE: sha256.Sum256([]byte("exe")), Assets: sha256.Sum256([]byte("assets")), State: sha256.Sum256([]byte("state")), Group: sha256.Sum256([]byte("group")), AppGeneration: 1, SignatureGeneration: 1, CUKey: 7}
}
func TestResolvePartialCoverageBackgroundAndOwnership(t *testing.T) {
	id := literalIdentity()
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 9
	}
	var colors [256][3]uint8
	colors[5] = [3]uint8{20, 40, 60}
	colors[6] = [3]uint8{100, 120, 140}
	colors[9] = [3]uint8{4, 8, 12}
	cap := CaptureIdentity{Scene: id, Kind: CaptureBuf3D, Steps: 8, IndexedSHA: sha256.Sum256(idx), OriginalPaletteSHA: PaletteHash(colors)}
	pal, e := NewPaletteSnapshot(id, cap, colors)
	if e != nil {
		t.Fatal(e)
	}
	bg, e := NewBackgroundComponents(id, cap, 2, idx, pal)
	if e != nil {
		t.Fatal(e)
	}
	r := &Result{identity: id, capture: &cap, scale: 2, width: 640, height: 400, samples: make([]sample, 640*400*4), report: Report{Valid: true}}
	r.samples[0] = sample{color: SampleColor{5, 6, 1 << 22}, depth: 1}
	r.samples[1] = sample{color: SampleColor{5, 6, 3 << 22}, depth: 1}
	out, e := ResolveRGB(r, bg, pal)
	if e != nil {
		t.Fatal(e)
	}
	if got := out.RGBAAt(0, 0); got.R != 32 || got.G != 44 || got.B != 56 || got.A != 255 {
		t.Fatal(got)
	}
	first := r.Hash()
	idx[0] = 7
	colors[9] = [3]uint8{255, 255, 255}
	out.Pix[0] = 0
	again, e := ResolveRGB(r, bg, pal)
	if e != nil || again.Pix[0] != 32 || r.Hash() != first {
		t.Fatal("alias", e)
	}
	if _, e = ResolveRGB(r, nil, pal); e == nil {
		t.Fatal("missing background accepted")
	}
	wrong := pal
	wrong.Capture.Steps++
	if _, e = ResolveRGB(r, bg, wrong); e == nil {
		t.Fatal("mixed capture accepted")
	}
	wrong = pal
	wrong.Colors[5][0]++
	if _, e = ResolveRGB(r, bg, wrong); e == nil {
		t.Fatal("changed palette with old hash accepted")
	}
	noCap := *r
	noCap.capture = nil
	if _, e = ResolveRGB(&noCap, bg, pal); e == nil {
		t.Fatal("unbound components became RGB")
	}
	deferred := *r
	deferred.report.Needs3b = true
	if _, e = ResolveRGB(&deferred, bg, pal); e == nil {
		t.Fatal("R2 silently rendered")
	}
}
