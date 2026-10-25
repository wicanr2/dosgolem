package strikecommander

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"testing"
)

func registryTest(t *testing.T) (*hdSceneRegistry, *shadesource.Owner) {
	t.Helper()
	o, e := shadesource.NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := o.EnableProtectedProducer(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	r, e := newHDSceneRegistry(o, p, nil, hd3d.Tables{})
	if e != nil {
		t.Fatal(e)
	}
	return r, o
}
func registryCapture(t *testing.T, r *hdSceneRegistry, key uint64) *hdSealedCapture {
	t.Helper()
	a, e := newLegacyArena(r.pool)
	if e != nil {
		t.Fatal(e)
	}
	g, e := legacyNew[ModelGroup](a)
	if e != nil {
		t.Fatal(e)
	}
	g.Buf3D, e = legacyMake[byte](a, 64000)
	if e != nil {
		t.Fatal(e)
	}
	g.Key = key
	g.Desc = [6]uint16{1, 2, 0, 0, 319, 199}
	c, e := legacyNew[hdSealedCapture](a)
	if e != nil {
		t.Fatal(e)
	}
	*c = hdSealedCapture{Group: g, Arena: a, Serial: key, CUKey: key, ShadeEpoch: 4, SignatureGeneration: 2, SignatureValid: true}
	a.seal()
	r.onSealed(c)
	a.retire()
	return c
}
func TestHDRegistryTwoSlotsImmutableBindingsAndNoLateScene(t *testing.T) {
	r, o := registryTest(t)
	registryCapture(t, r, 1)
	a := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(a, 1)
	if _, e := r.lookup(a.hdBinding); e != nil {
		t.Fatal(e)
	}
	registryCapture(t, r, 2)
	registryCapture(t, r, 3)
	count := 0
	for _, s := range r.slots {
		if s.capture != nil {
			count++
			if s.capture.CUKey == 2 {
				t.Fatal("superseded candidate retained")
			}
		}
	}
	if count != 2 {
		t.Fatal(count)
	}
	b := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(b, 3)
	if _, e := r.lookup(a.hdBinding); e == nil {
		t.Fatal("old image silently retargeted")
	}
	missing := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(missing, 99)
	registryCapture(t, r, 99)
	if _, e := r.lookup(missing.hdBinding); e == nil {
		t.Fatal("late source filled published NoHD binding")
	}
	r.close()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("registry retained source", o.OwnedBytes())
	}
}
func TestHDRegistryVoidAndP6RevokePublishedQualification(t *testing.T) {
	r, o := registryTest(t)
	c := registryCapture(t, r, 1)
	img := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(img, 1)
	var p [256][3]byte
	frame := make([]byte, 64000)
	r.frame(c.Group, 100, frame, p)
	frame[0] = 1
	r.frame(c.Group, HDStaleSteps+101, frame, p)
	if _, e := r.lookup(img.hdBinding); e == nil {
		t.Fatal("P6 revoked source remained qualified")
	}
	registryCapture(t, r, 2)
	next := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(next, 2)
	if _, e := r.lookup(next.hdBinding); e != nil {
		t.Fatal("new CU did not recover", e)
	}
	r.invalidateCU(2)
	if _, e := r.lookup(next.hdBinding); e == nil {
		t.Fatal("void source remained qualified")
	}
	r.close()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
func TestHDRegistryLowQuotaRejectsBeforeLargeControlBacking(t *testing.T) {
	o, _ := shadesource.NewOwner(0)
	p, _ := o.EnableProtectedProducer(1024, 1024)
	if r, e := newHDSceneRegistry(o, p, nil, hd3d.Tables{}); e == nil || r != nil {
		t.Fatal("large policy control bypassed quota")
	}
	if p.Usage().LegacyOwned != 0 {
		t.Fatal("failed registry retained control")
	}
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
func TestHDBackgroundUsesOnlySameSourceAndActualQualifiedPixels(t *testing.T) {
	indexed := make([]byte, 64000)
	indexed[0] = 9
	var palette [256][3]byte
	palette[9] = [3]byte{90, 90, 90}
	palette[7] = [3]byte{70, 70, 70}
	before, after := make([]byte, 320), make([]byte, 320)
	before[0] = 7
	after[0] = 9
	item := []bgItem{{x0: 0, x1: 1, y0: 0, rows: 1, before: before, after: after}}
	for _, n := range []int{2, 3} {
		size := 320 * 200 * n * n
		out := make([]byte, size*4)
		scaleFrontendInto(out, indexed, palette, n)
		rgb, mask, applied := make([]byte, size*4), make([]byte, size), make([]byte, size)
		for i := range rgb {
			rgb[i] = 17
		}
		mask[0] = 1
		buf := make([]byte, 64000)
		buf[0] = 7
		got := labelBackgroundHD(out, indexed, palette, n, item, &hdFlightBackground{buf: buf, rgb: rgb, qualified: mask, applied: applied})
		if got[0] != 17 || applied[0] != 1 || got[4] != 70 {
			t.Fatal("qualified subpixel expanded to full block", n)
		}
		buf[0] = 8
		applied[0] = 0
		got = labelBackgroundHD(out, indexed, palette, n, item, &hdFlightBackground{buf: buf, rgb: rgb, qualified: mask, applied: applied})
		if got[0] != 70 || applied[0] != 0 {
			t.Fatal("foreign before used HD")
		}
	}
}

func TestHDRegistryExtraH4RevokesOnceAndFreshEpochRecovers(t *testing.T) {
	r, o := registryTest(t)
	registryCapture(t, r, 1)
	img := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(img, 1)
	epoch := r.epoch
	o.RevokeThrough(4)
	r.refreshRevocations()
	if r.epoch != epoch+1 {
		t.Fatal("revocation did not advance display eligibility", r.epoch)
	}
	if _, e := r.lookup(img.hdBinding); e == nil {
		t.Fatal("revoked source retained prepared eligibility")
	}
	r.refreshRevocations()
	if r.epoch != epoch+1 {
		t.Fatal("repeated H4 changed safety identity")
	}
	c := registryCapture(t, r, 2)
	c.ShadeEpoch = 5
	next := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.bind(next, 2)
	r.refreshRevocations()
	if _, e := r.lookup(next.hdBinding); e != nil {
		t.Fatal("fresh epoch did not recover", e)
	}
	r.close()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}

func TestFrontendCloseStatisticsReadActualRetainedOwner(t *testing.T) {
	rt, e := NewFrontendRuntime(runtimeTestConfig(t))
	if e != nil {
		t.Fatal(e)
	}
	reg, o := registryTest(t)
	hold, e := o.ReserveOwned(2 * 320 * 200 * 3 * 3 * 4)
	if e != nil {
		t.Fatal(e)
	}
	rt.hd = &frontendHD{owner: o, pool: reg.pool, registry: reg, models: &ModelRecorder{shade: &shadeCapture{owner: o}}, rgbaReservation: hold, prepared: rt.output.prepared}
	rt.so.Models = rt.hd.models
	rt.output.prepared.onDrained = func() { hold.Release() }
	key := prepareKey(1, 2)
	rt.output.prepared.invalidate(key.Validity)
	l, e := rt.output.prepared.prepare(key, func(dst []byte) (hdPreparedKind, string, error) { return hdReady, "", nil })
	if e != nil {
		t.Fatal(e)
	}
	rt.Close()
	if got := rt.HDTotalOwned(); got != 2*320*200*3*3*4 {
		t.Fatal("Close hid a real live RGBA owner", got)
	}
	if rt.ProducerUsage().LegacyOwned != 0 || rt.ProducerUsage().ExtraOwned != 0 {
		t.Fatal("producer remained owned", rt.ProducerUsage())
	}
	l.Release()
	if got := rt.HDTotalOwned(); got != 0 {
		t.Fatal("last reader retained real owner bytes", got)
	}
	rt.Close()
	if rt.HDTotalOwned() != o.OwnedBytes() {
		t.Fatal("statistics did not retain actual owner")
	}
}
