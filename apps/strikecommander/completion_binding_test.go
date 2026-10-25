package strikecommander

import (
	"bytes"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/internal/machine"
	"testing"
)

// Real Flight.Observe, CU tracker callbacks, Presenter.Observe/publish and real
// Runtime binder/registry. Only the model/source payload is an owned synthetic
// fixture; this test does not claim model replay, shaders, or an original VM run.
type completionBindingRig struct {
	r            *presRig
	runtime      *FrontendRuntime
	model        *ModelRecorder
	key          uint64
	publications []*presentImage
	expectBound  bool
}

func newCompletionBindingRig(t *testing.T) *completionBindingRig {
	t.Helper()
	r := newPresRig(t, 0)
	r.w.o = NewFlightObserver(nil)
	r.w.o.discoverCompletionCU = true
	r.so.Flight = r.w.o
	r.p.Attach(r.so)
	reg, owner := registryTest(t)
	model := NewModelRecorder(nil)
	model.base, model.baseOK = tpBase, true
	model.tracker.SetBase(tpBase, true)
	model.materialSources, model.sourceGeneration = true, 2
	model.sourceProof = &realspace.FaceSourceProof{Base: tpBase, Generation: 2, Rule: realspace.FaceSourceRule}
	r.so.Models = model
	output, err := NewFrontendPresentation(r.p, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	rt := &FrontendRuntime{so: r.so, present: r.p, output: output, hd: &frontendHD{registry: reg, models: model, owner: owner, pool: reg.pool}}
	rig := &completionBindingRig{r: r, runtime: rt, model: model, expectBound: true}
	r.p.bindHDImage = rt.bindHDImage
	r.p.onCapture = append(r.p.onCapture, func(PresentInfo) {
		image := r.p.cur
		if image.info.Kind == PresCU || image.info.Kind == PresCUReturn {
			if image.sourceCU != rig.key {
				t.Errorf("published image did not already own accepted CU: got=%d want=%d", image.sourceCU, rig.key)
			}
			if rig.expectBound && (image.hdBinding.cu != rig.key || image.hdBinding.serial != rig.key) {
				t.Errorf("published binding not ready at callback: %+v", image.hdBinding)
			}
			if image.info.Kind == PresCUReturn && r.p.cu.entry != 0 {
				t.Error("test did not exercise cleared Presenter scratch")
			}
		}
		rig.publications = append(rig.publications, image)
	})
	t.Cleanup(func() {
		reg.close()
		owner.Close()
		if owner.OwnedBytes() != 0 {
			t.Error("source fixture leaked", owner.OwnedBytes())
		}
	})
	return rig
}
func (b *completionBindingRig) at(cs, ip, cx, sp uint16) {
	r := b.r
	r.v.steps += 100
	r.v.cs, r.v.ip, r.v.cx, r.v.ss, r.v.sp = cs, ip, cx, tSS, sp
	r.so.Labels.Observe(r.v)
	r.so.Flight.Observe(r.v)
	b.model.tracker.Observe(r.v, &cuHooks{OnEntryAccepted: func(_ FlightView, steps uint64, _ uint16) {
		b.key = steps
		registryCapture(r.t, b.runtime.hd.registry, steps)
	}})
	r.p.Observe(r.v)
}
func (b *completionBindingRig) enter()             { b.at(tpBase+segFJemm, ipCUDiff, 1, tCUSP) }
func (b *completionBindingRig) returnCU(sp uint16) { b.at(tpBase+segFJemm, ipCUDiffRet, 1, sp) }
func (b *completionBindingRig) next()              { b.at(0x1234, 0x0010, 1, tCUSP) }
func completionAssertBinding(t *testing.T, b *completionBindingRig, kind PresentKind) {
	t.Helper()
	img := b.r.p.cur
	if img == nil || img.info.Kind != kind || img.sourceCU != b.key || img.hdBinding.cu != b.key || img.hdBinding.serial != b.key {
		t.Fatalf("same-CU completion binding absent: image=%+v key=%d", img, b.key)
	}
	if _, err := b.runtime.hd.registry.lookup(img.hdBinding); err != nil {
		t.Fatal("bound fixture was not retained", err)
	}
}
func TestCompletionBindingNeedReturnOwnsOriginalEntryBeforePublish(t *testing.T) {
	b := newCompletionBindingRig(t)
	b.enter()
	if b.r.p.cur != nil || !b.r.p.cu.needReturn {
		t.Fatal("empty candidates did not choose needReturn")
	}
	key := b.key
	b.r.copyFB()
	b.returnCU(tCUSP + 2)
	if b.r.p.cur != nil || !b.r.w.o.cu.Entry().InProgress {
		t.Fatal("wrong return published or discarded valid entry")
	}
	b.returnCU(tCUSP)
	b.next()
	completionAssertBinding(t, b, PresCUReturn)
	if b.key != key || !bytes.Equal(b.r.p.cur.indexed, b.r.v.vga) || len(b.publications) != 1 {
		t.Fatal("return used later key/pixels or duplicated publication")
	}
	old := b.r.p.cur
	b.r.p.derive(PresPalette, b.r.v.pal, nil, false)
	if old.sourceCU != key || b.r.p.cur.sourceCU != key || b.r.p.cur.hdBinding.cu != key {
		t.Fatal("derived palette mutated/lost immutable entry")
	}
}
func TestCompletionBindingResidueAndUnchangedBufCU(t *testing.T) {
	for _, residue := range []bool{false, true} {
		t.Run(map[bool]string{false: "equal", true: "residue"}[residue], func(t *testing.T) {
			b := newCompletionBindingRig(t)
			b.r.p.fbDesc = presDesc{valid: true, ds: qDS, seg: 0xffff, list: qList, base: tpBase}
			b.enter()
			completionAssertBinding(t, b, PresCU)
			entryImage, key := b.r.p.cur, b.key
			b.r.copyFB()
			if residue {
				b.r.v.vga[1600] ^= 1
			}
			b.returnCU(tCUSP)
			b.next()
			if residue {
				completionAssertBinding(t, b, PresCUReturn)
				if b.r.p.counts.CUResidue != 1 || !bytes.Equal(b.r.p.cur.indexed, b.r.v.vga) {
					t.Fatal("residue did not capture actual VGA")
				}
			} else {
				if b.r.p.cur != entryImage || b.r.p.counts.CUResidue != 0 || b.r.count(PresCUReturn) != 0 {
					t.Fatal("equal VGA created a redundant return image")
				}
			}
			if entryImage.sourceCU != key || entryImage.hdBinding.cu != key {
				t.Fatal("later return mutated earlier completion image")
			}
		})
	}
}
func TestCompletionBindingRejectsWrongIdentityAndRevocation(t *testing.T) {
	b := newCompletionBindingRig(t)
	b.enter()
	b.r.copyFB()
	b.returnCU(tCUSP)
	b.next()
	completionAssertBinding(t, b, PresCUReturn)
	image, key := b.r.p.cur, b.key
	reg := b.runtime.hd.registry
	registryCapture(t, reg, key+1) // an actual other live slot makes the wrong-token negative non-vacuous.
	bad := *image
	bad.sourceCU = key + 1
	bad.hdBinding = hdImageBinding{}
	b.runtime.bindHDImage(&bad)
	if _, err := reg.lookup(bad.hdBinding); err == nil {
		t.Fatal("borrowed adjacent CU despite real tracker mismatch")
	}
	bad = *image
	bad.hdBinding = hdImageBinding{}
	b.model.base = tpBase + 1
	b.runtime.bindHDImage(&bad)
	b.model.base = tpBase
	if _, err := reg.lookup(bad.hdBinding); err == nil {
		t.Fatal("accepted different Flight/model base")
	}
	reg.invalidateCU(key)
	if _, err := reg.lookup(image.hdBinding); err == nil {
		t.Fatal("revoked old source remained qualified")
	}
	bad = *image
	bad.hdBinding = hdImageBinding{}
	b.runtime.bindHDImage(&bad)
	if _, err := reg.lookup(bad.hdBinding); err == nil {
		t.Fatal("revoked source got a new binding")
	}
	if image.sourceCU != key {
		t.Fatal("revocation mutated immutable capture token")
	}
}
func TestCompletionBindingGeneralCaptureAndModeHaveNoBorrowedCU(t *testing.T) {
	b := newCompletionBindingRig(t)
	b.enter()
	b.r.copyFB()
	b.returnCU(tCUSP)
	b.next()
	completionAssertBinding(t, b, PresCUReturn)
	old := b.r.p.cur
	if !b.r.p.capture(b.r.v, PresShape, 0x1234, 0x0010, b.r.v.vga, &b.r.v.pal, nil) {
		t.Fatal("ordinary 2D capture failed")
	}
	if b.r.p.cur.sourceCU != 0 || b.r.p.cur.hdBinding.slot >= 0 {
		t.Fatal("ordinary 2D inherited old CU")
	}
	b.runtime.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: b.r.v.steps})
	if _, err := b.runtime.hd.registry.lookup(old.hdBinding); err == nil {
		t.Fatal("Mode preserved old source binding")
	}
	if old.sourceCU != b.key {
		t.Fatal("Mode mutated previously published image")
	}
}

func TestCompletionBindingRevokesOldQualificationBeforeOnCapture(t *testing.T) {
	for _, failure := range []string{"material-source", "flight-base"} {
		t.Run(failure, func(t *testing.T) {
			b := newCompletionBindingRig(t)
			b.enter()
			b.r.copyFB()
			b.returnCU(tCUSP)
			b.next()
			completionAssertBinding(t, b, PresCUReturn)
			old, key := b.r.p.cur, b.key
			oldBinding := old.hdBinding
			reg := b.runtime.hd.registry
			if _, err := reg.lookup(oldBinding); err != nil {
				t.Fatal("vacuous prior qualification", err)
			}
			b.expectBound = false
			switch failure {
			case "material-source":
				b.model.sourceProof = nil
			case "flight-base":
				b.r.w.o.baseOK = false
			}
			observed := 0
			b.r.p.onCapture = append(b.r.p.onCapture, func(PresentInfo) {
				observed++
				if _, err := reg.lookup(oldBinding); err == nil {
					t.Error("old qualification survived until after publication")
				}
				if _, err := reg.lookup(b.r.p.cur.hdBinding); err == nil {
					t.Error("new image inherited invalid source")
				}
				if old.sourceCU != key || old.hdBinding != oldBinding {
					t.Error("revocation mutated published source bytes/identity")
				}
			})
			// Force a real publish while stale source state is live. There is no outer
			// Runtime.BeforeStep return that could hide missing pre-publication sync.
			if !b.r.p.captureCU(b.r.v, PresCUReturn, tpBase+segFJemm, ipCUDiffRet, b.r.v.vga, &b.r.v.pal, nil, key) {
				t.Fatal("new image was not published")
			}
			if observed != 1 {
				t.Fatal("required OnCapture callback did not execute", observed)
			}
		})
	}
}
