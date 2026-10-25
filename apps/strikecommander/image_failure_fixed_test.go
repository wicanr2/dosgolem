package strikecommander

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixedMemoRig(t *testing.T, badPalette bool) (*rgbMemoRig, *hdRegistrySlot) {
	t.Helper()
	f := newRGBMemoRig(t)
	s := f.install(filepath.Join(os.Getenv("SC_RGB_MEMO_CASES"), "runtime-corpus-003/run-s800f2-n3-001/C-group-001"), 0)
	// Extra simultaneously retained test comparisons are explicitly charged;
	// they are not hidden engine/caller RGBA slots in a player path.
	hold, e := f.rt.hd.owner.ReserveOwned(8 << 20)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(hold.Release)
	if badPalette {
		pal := f.rt.present.cur.palette
		pal[20] = [3]byte{135, 135, 135}
		f.rt.present.derive(PresPalette, pal, nil, false)
	}
	return f, s
}

func fixedMemoRound(t *testing.T, f *rgbMemoRig, req FrontendPrepareRequest) (FrontendCompositeInfo, []byte) {
	t.Helper()
	i, b, e := imageMemoRound(f.rt, req)
	if e != nil {
		t.Fatal(e)
	}
	p := f.rt.output.prepared
	p.mu.Lock()
	readers := p.readers
	for x := range p.entries {
		if len(p.entries[x].rgba) != 0 {
			p.mu.Unlock()
			t.Fatal("engine RGBA remained after both lease releases")
		}
	}
	p.mu.Unlock()
	if readers != 0 {
		t.Fatal("leases did not end", readers)
	}
	return i, b
}

func fixedMemoMetadata(p *hdPreparation) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for i := range p.entries {
		e := &p.entries[i]
		if e.current && e.ready {
			n++
		}
	}
	return n
}

func TestImageFixedMemoBothScalesAndDiagnostics(t *testing.T) {
	f, slot := fixedMemoRig(t, true)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	a, ab := fixedMemoRound(t, f, req)
	req.Scale = 3
	b, bb := fixedMemoRound(t, f, req)
	if a.Kind != FrontendFixedFallback || b.Kind != FrontendFixedFallback || slot.memo[0].failed || slot.memo[1].failed || fixedMemoMetadata(f.rt.output.prepared) != 2 {
		t.Fatal("two real RGB image failures not retained independently")
	}
	if e := f.rt.hd.renderer.Close(); e != nil {
		t.Fatal(e)
	}
	for _, scale := range []int{2, 3, 2} {
		req.Scale = scale
		got, pixels := fixedMemoRound(t, f, req)
		want, raw := a, ab
		if scale == 3 {
			want, raw = b, bb
		}
		if !reflect.DeepEqual(got, want) || !bytes.Equal(pixels, raw) || slot.memo[0].failed || slot.memo[1].failed {
			t.Fatal("N change washed fixed key or retried closed core", scale, got.Reason)
		}
		d := f.rt.HDDiagnostics()
		if d.Scale != scale || d.Reason != want.Reason {
			t.Fatal("fixed fallback diagnostics retained previous N", d)
		}
	}
}

func TestImageFixedMemoTwoRequestsProtectThirdAndValidityRetires(t *testing.T) {
	f, slot := fixedMemoRig(t, true)
	aReq := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	a, ab := fixedMemoRound(t, f, aReq)
	bReq := aReq
	bReq.Subtitles = true // Keep SettingsVersion unchanged on purpose.
	b, bb := fixedMemoRound(t, f, bReq)
	if fixedMemoMetadata(f.rt.output.prepared) != 2 {
		t.Fatal("second same-N request retired first legal metadata")
	}
	third := aReq
	third.HD = false
	lease, e := f.rt.Prepare(third)
	if e == nil || lease != nil || !strings.Contains(e.Error(), "slots occupied") || fixedMemoMetadata(f.rt.output.prepared) != 2 {
		t.Fatal("third request washed legal metadata or published an unrecorded fallback", lease, e)
	}
	if e := f.rt.hd.renderer.Close(); e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		req   FrontendPrepareRequest
		info  FrontendCompositeInfo
		bytes []byte
	}{{aReq, a, ab}, {bReq, b, bb}} {
		got, pixels := fixedMemoRound(t, f, x.req)
		if !reflect.DeepEqual(got, x.info) || !bytes.Equal(pixels, x.bytes) || slot.memo[0].failed {
			t.Fatal("third rejection poisoned prior image/request failure", got.Reason)
		}
	}
	third.SettingsVersion++ // A real validity change retires both old keys.
	got, _ := fixedMemoRound(t, f, third)
	if got.Kind != FrontendReady || fixedMemoMetadata(f.rt.output.prepared) != 1 {
		t.Fatal("new validity could not retire obsolete metadata", got)
	}
}

func TestImageFixedMemoQuotaNewRequestAndSafety(t *testing.T) {
	f, slot := fixedMemoRig(t, false)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	warm, _ := fixedMemoRound(t, f, req)
	if warm.Kind != FrontendReady {
		t.Fatal("real baseline not Ready", warm)
	}
	owner := f.rt.hd.owner
	free := owner.TotalBytes() - owner.OwnedBytes()
	hold, e := owner.ReserveOwned(free - (1 << 20))
	if e != nil {
		t.Fatal(e)
	}
	defer hold.Release()
	req.SettingsVersion++
	first, firstBytes := fixedMemoRound(t, f, req)
	if first.Kind != FrontendFixedFallback || !strings.Contains(first.Reason, "owner total limit") || slot.memo[0].failed {
		t.Fatal("did not hit image/request quota fallback", first)
	}
	hold.Release()
	for i := 0; i < 2; i++ {
		got, b := fixedMemoRound(t, f, req)
		if !reflect.DeepEqual(got, first) || !bytes.Equal(b, firstBytes) || slot.memo[0].failed {
			t.Fatal("released quota changed same-key fallback", got)
		}
	}
	f.rt.output.safety.set(HDStale)
	f.rt.output.invalidate()
	safe, _ := fixedMemoRound(t, f, req)
	if safe.Kind != FrontendSafetyFallback || slot.memo[0].failed {
		t.Fatal("Safety poisoned scene or became HD failure", safe)
	}
	f.rt.output.safety.set(HDEligible)
	f.rt.output.invalidate()
	eligible, _ := fixedMemoRound(t, f, req)
	if eligible.Kind != FrontendReady || slot.memo[0].failed || eligible.key == first.key {
		t.Fatal("new Safety key inherited image-local quota failure", eligible)
	}
	req.SettingsVersion++
	newRequest, _ := fixedMemoRound(t, f, req)
	if newRequest.Kind != FrontendReady || newRequest.key == first.key {
		t.Fatal("new request could not recover", newRequest)
	}
}

func TestImageFixedMemoCloseKeepsHeldLease(t *testing.T) {
	f, _ := fixedMemoRig(t, true)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	first, expected := fixedMemoRound(t, f, req)
	lease, e := f.rt.Prepare(req)
	if e != nil {
		t.Fatal(e)
	}
	h := f.rt.hd
	f.rt.Close()
	if h.rgbaReservation == nil {
		t.Fatal("Close refunded live reader backing")
	}
	got, e := lease.Info()
	if e != nil || !reflect.DeepEqual(got, first) {
		t.Fatal("Close changed immutable held info", got, e)
	}
	actual := make([]byte, len(expected))
	if e = lease.CopyRGBA(actual); e != nil || !bytes.Equal(actual, expected) {
		t.Fatal("Close changed held fallback pixels", e)
	}
	lease.Release()
	if h.rgbaReservation != nil {
		t.Fatal("last reader did not end engine reservation")
	}
	if l, e := f.rt.Prepare(req); e == nil || l != nil {
		t.Fatal("closed runtime rebuilt failure", l, e)
	}
}

func TestImageFixedMemoCancelledRebuild(t *testing.T) {
	p := newHDPreparation()
	p.consumeOnRelease = true
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	produced := 0
	produce := func(dst []byte) (hdPreparedKind, string, error) {
		produced++
		dst[0] = 17
		return hdFixedFallback, "fixed", nil
	}
	a, e := p.prepareWithFixedFallback(k, produce, nil)
	if e != nil {
		t.Fatal(e)
	}
	a.Release()
	next := prepareKey(2, 2)
	l, e := p.prepareWithFixedFallback(k, produce, func(dst []byte, reason string) (hdPreparedKind, string, error) {
		p.invalidate(next.Validity)
		dst[0] = 17
		return hdFixedFallback, reason, nil
	})
	if e == nil || l != nil || produced != 1 || fixedMemoMetadata(p) != 0 {
		t.Fatal("cancelled failure rebuild published or retried producer", l, e, produced)
	}
	b, e := p.prepare(next, func(dst []byte) (hdPreparedKind, string, error) { return hdReady, "", nil })
	if e != nil {
		t.Fatal(e)
	}
	b.Release()
}

func TestImageFixedMemoRebuildErrorKeepsMetadata(t *testing.T) {
	p := newHDPreparation()
	p.consumeOnRelease = true
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	produced := 0
	produce := func(dst []byte) (hdPreparedKind, string, error) {
		produced++
		dst[0] = 31
		return hdFixedFallback, "fixed", nil
	}
	a, e := p.prepareWithFixedFallback(k, produce, nil)
	if e != nil {
		t.Fatal(e)
	}
	info, _ := a.Info()
	a.Release()
	l, e := p.prepareWithFixedFallback(k, produce, func([]byte, string) (hdPreparedKind, string, error) {
		return hdNotPrepared, "", fmt.Errorf("test rebuild failure")
	})
	if e == nil || l != nil || fixedMemoMetadata(p) != 1 {
		t.Fatal("failed rebuild erased already-delivered metadata", l, e)
	}
	b, e := p.prepareWithFixedFallback(k, produce, func(dst []byte, reason string) (hdPreparedKind, string, error) {
		dst[0] = 31
		return hdFixedFallback, reason, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	got, _ := b.Info()
	b.Release()
	if got != info || produced != 1 {
		t.Fatal("fixed rebuild failure allowed HD retry or changed result", got, produced)
	}
}
