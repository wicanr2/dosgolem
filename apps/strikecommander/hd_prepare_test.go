package strikecommander

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func prepareKey(seq uint64, n int) hdPrepareKey {
	return hdPrepareKey{Validity: hdValidity{Seq: seq, SafetyVersion: seq, SettingsVersion: 1}, Safety: HDSafetyState{Eligibility: HDEligible, Version: seq}, Scale: n, HD: true}
}
func TestHDPrepareOnlyOutsideCallbacksAndImmutableLease(t *testing.T) {
	p := newHDPreparation()
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	calls := 0
	produce := func(dst []byte) (hdPreparedKind, string, error) { calls++; dst[0] = 17; return hdReady, "", nil }
	p.enterCallback()
	if _, e := p.prepare(k, produce); e == nil || calls != 0 {
		t.Fatal("callback rendered", e)
	}
	p.leaveCallback()
	a, e := p.prepare(k, produce)
	if e != nil {
		t.Fatal(e)
	}
	b, e := p.prepare(k, produce)
	if e != nil || calls != 1 {
		t.Fatal("same key rendered twice", e)
	}
	raw := make([]byte, 640*400*4)
	if e = a.CopyRGBA(raw); e != nil || raw[0] != 17 {
		t.Fatal(e)
	}
	raw[0] = 99
	if e = a.CopyRGBA(raw); e != nil || raw[0] != 17 {
		t.Fatal("RGBA mutable alias", e)
	}
	a.Release()
	b.Release()
	if e = a.CopyRGBA(raw); e == nil {
		t.Fatal("released bytes reacquired")
	}
}
func TestHDPrepareSupersededAndSameSeqSafetyRecovery(t *testing.T) {
	p := newHDPreparation()
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	next := prepareKey(2, 2)
	_, e := p.prepare(k, func(dst []byte) (hdPreparedKind, string, error) {
		// Simulate a source/setting invalidation while the synchronous producer is
		// active. A reentrant Prepare must not deadlock or start another producer.
		if _, e := p.prepare(k, func([]byte) (hdPreparedKind, string, error) {
			t.Fatal("reentrant producer invoked")
			return hdReady, "", nil
		}); e == nil {
			t.Fatal("reentrant Prepare ran")
		}
		p.invalidate(next.Validity)
		return hdReady, "", nil
	})
	if e == nil {
		t.Fatal("superseded result published")
	}
	if _, ok := p.lookup(k); ok {
		t.Fatal("old current key remained eligible")
	}
	k = prepareKey(2, 2)
	k.Safety.Eligibility = HDStale
	k.Safety.Version = 3
	k.Validity.SafetyVersion = 3
	p.invalidate(k.Validity)
	a, e := p.prepare(k, func(b []byte) (hdPreparedKind, string, error) { b[0] = 9; return hdSafetyFallback, "stale", nil })
	if e != nil {
		t.Fatal(e)
	}
	defer a.Release()
	eligible := k
	eligible.Safety.Eligibility = HDEligible
	eligible.Safety.Version++
	eligible.Validity.SafetyVersion++
	p.invalidate(eligible.Validity)
	b, e := p.prepare(eligible, func(b []byte) (hdPreparedKind, string, error) { b[0] = 17; return hdReady, "", nil })
	if e != nil {
		t.Fatal(e)
	}
	defer b.Release()
	ia, _ := a.Info()
	ib, _ := b.Info()
	if ia.Kind != hdSafetyFallback || ib.Kind != hdReady || ia.Key == ib.Key || ia.SHA == ib.SHA {
		t.Fatal("safety fallback aliased HD memo")
	}
	old := make([]byte, 640*400*4)
	a.CopyRGBA(old)
	if old[0] != 9 {
		t.Fatal("published old safety image changed")
	}
}

type hdShortWriter struct{}

func (hdShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestHDPrepareWriterFailureAndTwoRGBAReaders(t *testing.T) {
	p := newHDPreparation()
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	producer := func(b []byte) (hdPreparedKind, string, error) { return hdFixedFallback, "H2", nil }
	a, e := p.prepare(k, producer)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Release()
	if e = a.WriteRGBA(hdShortWriter{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = a.WriteRGBA(&buf); e != nil || buf.Len() != 640*400*4 {
		t.Fatal(e)
	}
	k = prepareKey(2, 2)
	p.invalidate(k.Validity)
	b, e := p.prepare(k, producer)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Release()
	k = prepareKey(3, 2)
	p.invalidate(k.Validity)
	if _, e = p.prepare(k, producer); e == nil {
		t.Fatal("third live RGBA admitted")
	}
	a.Release()
	q, e := p.prepare(k, func(b []byte) (hdPreparedKind, string, error) { return hdReady, "", fmt.Errorf("writer failed") })
	if e == nil || q != nil {
		t.Fatal("producer error swallowed")
	}
}

func TestHDPrepareRetiredReaderCannotBeReusedWhenAnotherSlotIsFree(t *testing.T) {
	p := newHDPreparation()
	defer p.close()
	makeOne := func(seq uint64, value byte) *hdPreparedLease {
		k := prepareKey(seq, 2)
		p.invalidate(k.Validity)
		v, e := p.prepare(k, func(dst []byte) (hdPreparedKind, string, error) { dst[0] = value; return hdReady, "", nil })
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	a := makeOne(1, 17)
	defer a.Release()
	b := makeOne(2, 19)
	b.Release()
	c := makeOne(3, 23)
	defer c.Release()
	info, e := a.Info()
	if e != nil || info.Key.Validity.Seq != 1 {
		t.Fatal("old immutable reader identity overwritten", info, e)
	}
	raw := make([]byte, 640*400*4)
	if e = a.CopyRGBA(raw); e != nil || raw[0] != 17 {
		t.Fatal("live old reader bytes overwritten", e)
	}
}

func TestHDPrepareOneEngineBufferConsumedAndSameSHARebuilt(t *testing.T) {
	p := newHDPreparation()
	p.maxBuffers = 1
	p.consumeOnRelease = true
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	producer := func(b []byte) (hdPreparedKind, string, error) { b[0] = 17; return hdReady, "", nil }
	a, e := p.prepare(k, producer)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := a.Info()
	front := make([]byte, 640*400*4)
	a.CopyRGBA(front)
	a.Release()
	count := 0
	for i := range p.entries {
		if len(p.entries[i].rgba) != 0 {
			count++
		}
	}
	if count != 0 || front[0] != 17 {
		t.Fatal("consumed engine buffer retained", count)
	}
	b, e := p.prepare(k, producer)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := b.Info()
	if before.SHA != after.SHA {
		t.Fatal("same key rebuild changed")
	}
	b.Release()
	if q, e := p.prepare(k, func(b []byte) (hdPreparedKind, string, error) { b[0] = 99; return hdReady, "", nil }); e == nil || q != nil {
		t.Fatal("same key rebuilt with different bytes")
	}
}

func TestHDPrepareCloseKeepsOwnerChargeUntilLastReader(t *testing.T) {
	p := newHDPreparation()
	p.consumeOnRelease = true
	drained := 0
	p.onDrained = func() { drained++ }
	key := prepareKey(1, 2)
	p.invalidate(key.Validity)
	l, e := p.prepare(key, func(dst []byte) (hdPreparedKind, string, error) { return hdReady, "", nil })
	if e != nil {
		t.Fatal(e)
	}
	p.close()
	if drained != 0 {
		t.Fatal("Close released a live ready buffer's owner charge")
	}
	l.Release()
	if drained != 1 {
		t.Fatal("last lease did not release owner charge", drained)
	}
	l.Release()
	p.close()
	if drained != 1 {
		t.Fatal("drain not once")
	}
}

func TestHDPreparePublishesFinalContentAndTableIdentity(t *testing.T) {
	p := newHDPreparation()
	defer p.close()
	k := prepareKey(1, 2)
	p.invalidate(k.Validity)
	p.refineKey = func(k *hdPrepareKey) { k.SourceKey = [32]byte{17}; k.ExtraKey = [32]byte{31}; k.TableEpoch = 2 }
	l, e := p.prepare(k, func(dst []byte) (hdPreparedKind, string, error) { return hdReady, "", nil })
	if e != nil {
		t.Fatal(e)
	}
	v, e := l.Info()
	if e != nil || v.Key.SourceKey != ([32]byte{17}) || v.Key.ExtraKey != ([32]byte{31}) || v.Key.TableEpoch != 2 {
		t.Fatal("prepared key missed validated final identity", v, e)
	}
	l.Release()
}
