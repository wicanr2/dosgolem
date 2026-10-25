package strikecommander

import (
	"bytes"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"testing"
)

func TestShadeRealCUFrameExportLifecycle(t *testing.T) {
	r, v := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	r.Observe(v)
	v.steps = 31
	v.ip = 0
	r.Observe(v)
	r.open.Buf3D = make([]byte, 64000)
	r.tracker.SetBase(r.base, true)
	var raw *shadesource.RawLease
	var group *ModelGroup
	seen := 0
	r.OnGroup = func(g *ModelGroup) {
		seen++
		group = g
		var e error
		raw, e = g.ShadeSource.Acquire()
		if e != nil {
			t.Fatalf("real CU Frame Acquire: %v", e)
		}
	}
	v.cs, v.ip, v.steps = r.base+segFJemm, ipCUDiff, 100
	for i, b := range sigJemm {
		v.mem[lin(v.cs, 0x90+uint16(i))] = byte(b)
	}
	r.Observe(v)
	v.steps, v.ip = 120, ipCUDiffRet
	r.Observe(v)
	r.Frame(121)
	if seen != 1 || raw == nil || group != r.shown {
		t.Fatal("CU -> await -> shown did not transfer")
	}
	defer raw.Release()
	var dst bytes.Buffer
	d, e := raw.Export(group.SceneInput(), &dst)
	if e != nil {
		t.Fatal("outside Frame Export", e)
	}
	p, e := shadesource.UnmarshalPacket(dst.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	h, e := shadesource.SourceDigest(p)
	if e != nil || h != d.DeclaredSourceDigest {
		t.Fatal("sealed source digest/Writer")
	}
	if p.Identity.CUKey != group.Key {
		t.Fatal("CU identity differs from actual Frame key")
	}
}
func TestShadeR7ReplacesProducerAndFailureIsVisible(t *testing.T) {
	r, v := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	old := r.open.ShadeSource
	for i := 0; i < 8; i++ {
		r.apply(&modelHit{kind: 0x102, steps: uint64(100 + i), valid: true, base: r.base})
		if old.Valid() {
			t.Fatal("R7 replacement retained old producer")
		}
		if r.open == nil || r.open.ShadeSource == nil || r.open.ShadeSourceError != nil {
			t.Fatal("R7 source silently unavailable")
		}
		old = r.open.ShadeSource
	}
	// Closed source owner forces a reproducible configuration failure without
	// changing the original model recorder's allocation or ReadFailed flags.
	r.shade.owner.Close()
	r.apply(&modelHit{kind: 0x102, steps: 200, valid: true, base: r.base})
	if r.open == nil || r.open.ShadeSource != nil || r.open.ShadeSourceError == nil || r.open.ShadeSourceError.Status != shadesource.H3 || r.open.ReadFailed {
		t.Fatal("graph allocation H3 was silently dropped")
	}
	_ = v
}
func TestShadeAwaitReplacementMUModeKeepLease(t *testing.T) {
	r, _ := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	r.open.Buf3D = make([]byte, 64000)
	r.hooks.OnEntryAccepted(nil, 100, 0)
	first := r.await
	raw, e := first.ShadeSource.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Release()
	r.apply(&modelHit{kind: 0x102, steps: 110, valid: true, base: r.base})
	r.open.Buf3D = make([]byte, 64000)
	r.hooks.OnEntryAccepted(nil, 120, 0)
	if r.await == first || first.ShadeSource.Valid() {
		t.Fatal("replaced await did not retire producer")
	}
	var b bytes.Buffer
	if _, e = raw.Export(first.SceneInput(), &b); e != nil {
		t.Fatal("held lease invalidated by await replacement", e)
	}
	r.hooks.OnMU(nil, 0)
	if r.open != nil {
		t.Fatal("MU kept source open")
	}
	if r.await.ShadeSource.Valid() {
		t.Fatal("MU kept obsolete source eligibility")
	}
	before := r.shade.owner.OwnedBytes()
	r.VideoModeChange()
	if before == 0 || r.shade.owner.OwnedBytes() == 0 {
		t.Fatal("Mode freed externally held source")
	}
	b.Reset()
	if _, e = raw.Export(first.SceneInput(), &b); e != nil {
		t.Fatal("Mode mutated retained immutable bytes", e)
	}
}
