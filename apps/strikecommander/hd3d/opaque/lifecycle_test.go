package opaque

import (
	"errors"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"testing"
)

func managerFixture(t *testing.T, limits ...Limits) (*Renderer, Input) {
	t.Helper()
	rang := []byte{1, 0, 5, 0, 99, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	m, e := NewMaterialLibrary([]Asset{{TRE: "A", Name: "X", IFF: handMaterial(0)}}, iffForm("PALT", iffChunk("RANG", rang)))
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRenderer(&realspace.Library{}, m, hd3d.Tables{}, limits...)
	if e != nil {
		t.Fatal(e)
	}
	in := Input{Identity: literalIdentity(), Scene: model.Input{Key: 7, Raster: hd3d.Group{Key: 7}}, SignatureValid: true}
	in.Identity.Assets = m.Key()
	in.Identity.Group, e = InputDigest(in.Scene)
	if e != nil {
		t.Fatal(e)
	}
	return r, in
}
func requireGate(t *testing.T, e error, g FailureGate) {
	t.Helper()
	var f Failure
	if !errors.As(e, &f) || f.Gate != g {
		t.Fatalf("want%s got%v", g, e)
	}
}
func TestSnapshotFixedFailureCapacityAndEpoch(t *testing.T) {
	r, in := managerFixture(t, Limits{Snapshots: 1})
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	_, first := s.Render(2)
	requireGate(t, first, H3)
	in.Scene.Original = &hd3d.Result{Valid: true, Checked: true, Repaintable: true}
	_, again := s.Render(2)
	if again != first {
		t.Fatal("same slot retried or changed failure")
	}
	other := in
	other.Scene.Key = 8
	other.Scene.Raster.Key = 8
	other.Identity.CUKey = 8
	other.Identity.Group, _ = InputDigest(other.Scene)
	_, e = r.Accept(other)
	requireGate(t, e, H3)
	if e = s.Release(); e != nil {
		t.Fatal(e)
	}
	s, e = r.Accept(other)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Release(); e != nil {
		t.Fatal(e)
	}
	bad := in
	bad.Identity.Group, _ = InputDigest(bad.Scene)
	bad.SignatureValid = false
	_, e = r.Accept(bad)
	requireGate(t, e, H4)
	bad.SignatureValid = true
	_, e = r.Accept(bad)
	requireGate(t, e, H4)
	bad.Identity.SignatureGeneration++
	s, e = r.Accept(bad)
	if e != nil {
		t.Fatal("new verified epoch not accepted", e)
	}
	s.Release()
}
func TestCapacityReservationAndMixedIdentity(t *testing.T) {
	r, in := managerFixture(t, Limits{TotalBytes: 16 << 20})
	_, e := r.Accept(in)
	requireGate(t, e, H3)
	r, in = managerFixture(t)
	in.Identity.Group[0] ^= 1
	_, e = r.Accept(in)
	requireGate(t, e, H3)
}

// T09: freeing another snapshot's result does not retry an already failed slot.
func TestResultCapacityFailureRemainsFixedAfterResultReleased(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	r.limits.Results = 1
	a, _ := drawOracle(t, r, in, 2)
	other := in
	other.Scene.Key = 8
	other.Scene.Raster.Key = 8
	other.Identity.CUKey = 8
	other.Identity.Group, _ = InputDigest(other.Scene)
	b, e := r.Accept(other)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Release()
	_, first := b.Render(2)
	requireGate(t, first, H3)
	if e = a.Release(); e != nil {
		t.Fatal(e)
	}
	_, again := b.Render(2)
	if again != first {
		t.Fatalf("failed slot retried after capacity became free: first=%v again=%v", first, again)
	}
	if r.results != 0 {
		t.Fatal("failed slot recomputed", r.results)
	}
}
