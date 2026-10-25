package strikecommander

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
)

type hdValidity struct{ Seq, SafetyVersion, SourceSerial, SourceEpoch, SettingsVersion uint64 }
type hdPrepareKey struct {
	Validity                                                                          hdValidity
	Safety                                                                            HDSafetyState
	Scale                                                                             int
	SourceKey, ExtraKey, IndexedSHA, PaletteSHA, FontCatalogSHA, SubtitleSHA          [32]byte
	RevocationVersion, TableEpoch                                                     uint64
	NoExtra, Subtitles, HD                                                            bool
	Language                                                                          string
	SourceProfile, PolicyVersion, VisibilityVersion, CompositionVersion, GlassVersion string
	LegacyLimit, ExtraLimit                                                           uint64
}
type hdPreparedKind uint8

const (
	hdNotPrepared hdPreparedKind = iota
	hdReady
	hdFixedFallback
	hdSafetyFallback
)

type hdPreparedInfo struct {
	Kind   hdPreparedKind
	Key    hdPrepareKey
	SHA    [32]byte
	Reason string
}
type hdPreparedEntry struct {
	info           hdPreparedInfo
	rgba           []byte
	readers        int
	current, ready bool
}

// hdPreparation owns ready RGBA independently of core samples. Runtime charges
// its one engine buffer and the frontend buffer to the same global ledger.
// Leases copy to caller-preallocated frontend storage; no RGBA alias escapes.
// It starts no goroutine and retains no queue of intermediate captures.
type hdPreparation struct {
	onDrained         func()
	refineKey         func(*hdPrepareKey)
	maxBuffers        int
	consumeOnRelease  bool
	mu                sync.Mutex
	active            sync.WaitGroup
	current           hdValidity
	callbackDepth     int
	readers           int
	preparing, closed bool
	entries           [2]hdPreparedEntry
}
type hdPreparedLease struct {
	owner    *hdPreparation
	entry    *hdPreparedEntry
	released bool
}

func newHDPreparation() *hdPreparation { return &hdPreparation{maxBuffers: 2} }
func (p *hdPreparation) invalidate(v hdValidity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == v {
		return
	}
	p.current = v
	for i := range p.entries {
		e := &p.entries[i]
		if e.info.Key.Validity != v {
			e.current = false
			if e.readers == 0 {
				e.rgba = nil
				e.ready = false
			}
		}
	}
}
func (p *hdPreparation) enterCallback() { p.mu.Lock(); p.callbackDepth++; p.mu.Unlock() }
func (p *hdPreparation) leaveCallback() { p.mu.Lock(); p.callbackDepth--; p.mu.Unlock() }
func (p *hdPreparation) lookup(k hdPrepareKey) (*hdPreparedLease, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.readers >= 2 || k.Validity != p.current {
		return nil, false
	}
	for i := range p.entries {
		e := &p.entries[i]
		if e.ready && len(e.rgba) != 0 && e.current && e.info.Key == k {
			e.readers++
			p.readers++
			return &hdPreparedLease{owner: p, entry: e}, true
		}
	}
	return nil, false
}

// prepare is called only after Advance and all callbacks return. Both success
// and HD fallback use a fully composed RGBA. A callback can only lookup a ready
// value. Cancellation is checked immediately before publishing the entry.
func (p *hdPreparation) prepare(k hdPrepareKey, produce func([]byte) (hdPreparedKind, string, error)) (*hdPreparedLease, error) {
	return p.prepareWithFixedFallback(k, produce, nil)
}

func (p *hdPreparation) prepareWithFixedFallback(k hdPrepareKey, produce func([]byte) (hdPreparedKind, string, error), rebuild func([]byte, string) (hdPreparedKind, string, error)) (*hdPreparedLease, error) {
	if k.Scale != 2 && k.Scale != 3 {
		return nil, fmt.Errorf("HD scale")
	}
	if produce == nil {
		return nil, fmt.Errorf("NotPrepared: missing producer")
	}
	if len(k.Language) > 64 || k.Safety.Version != k.Validity.SafetyVersion {
		return nil, fmt.Errorf("invalid prepare identity")
	}
	p.mu.Lock()
	if p.closed || p.readers >= 2 || p.callbackDepth != 0 || p.preparing || k.Validity != p.current {
		p.mu.Unlock()
		return nil, fmt.Errorf("NotPrepared")
	}
	for i := range p.entries {
		e := &p.entries[i]
		if e.ready && len(e.rgba) != 0 && e.current && e.info.Key == k {
			e.readers++
			p.readers++
			p.mu.Unlock()
			return &hdPreparedLease{owner: p, entry: e}, nil
		}
	}
	var saved *hdPreparedInfo
	var savedSlot *hdPreparedEntry
	for i := range p.entries {
		e := &p.entries[i]
		if e.ready && e.current && e.info.Key == k {
			v := e.info
			saved = &v
			savedSlot = e
		}
	}
	buffers := 0
	for i := range p.entries {
		if len(p.entries[i].rgba) != 0 {
			buffers++
		}
	}
	if buffers >= p.maxBuffers {
		p.mu.Unlock()
		return nil, fmt.Errorf("H3 independent RGBA allowance full")
	}
	// Empty RGBA does not retire a still-valid outcome. Rebuild a known key
	// in its own slot; a new key may use only retired/unpublished metadata.
	slot := savedSlot
	if slot == nil {
		for i := range p.entries {
			e := &p.entries[i]
			if e.readers == 0 && (!e.current || !e.ready) {
				slot = e
				break
			}
		}
	}
	if slot != nil && slot.readers != 0 {
		slot = nil
	}
	if slot == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("H3 independent RGBA slots occupied")
	}
	p.preparing = true
	p.active.Add(1)
	size := 320 * k.Scale * 200 * k.Scale * 4
	// This is one of the two explicit app RGBA allowances. No core/source data
	// may be stored in these entries or in the separate8MiB small-state budget.
	slot.rgba = make([]byte, size)
	slot.ready = false
	p.mu.Unlock()
	var kind hdPreparedKind
	var reason string
	var err error
	rebuildingFixed := saved != nil && saved.Kind == hdFixedFallback && rebuild != nil
	if rebuildingFixed {
		kind, reason, err = rebuild(slot.rgba, saved.Reason)
	} else {
		kind, reason, err = produce(slot.rgba)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.active.Done()
	p.preparing = false
	restoreMetadata := func() {
		slot.rgba = nil
		slot.ready, slot.current = false, false
		if saved != nil && !p.closed && saved.Key.Validity == p.current {
			slot.info = *saved
			slot.ready, slot.current = true, true
		}
	}
	if p.closed || k.Validity != p.current {
		restoreMetadata()
		return nil, fmt.Errorf("NotPrepared: superseded")
	}
	if err != nil {
		restoreMetadata()
		return nil, err
	}
	if p.refineKey != nil {
		p.refineKey(&k)
	}
	if len(reason) > 4096 {
		restoreMetadata()
		return nil, fmt.Errorf("H3 reason capacity")
	}
	if kind != hdReady && kind != hdFixedFallback && kind != hdSafetyFallback {
		restoreMetadata()
		return nil, fmt.Errorf("invalid prepared outcome")
	}
	if k.Safety.Eligibility != HDEligible && kind != hdSafetyFallback {
		restoreMetadata()
		return nil, fmt.Errorf("safety qualification mismatch")
	}
	hash := sha256.Sum256(slot.rgba)
	if rebuildingFixed && k != saved.Key {
		restoreMetadata()
		return nil, fmt.Errorf("NotPrepared: superseded fixed fallback")
	}
	if saved != nil && (saved.SHA != hash || saved.Kind != kind || saved.Reason != reason) {
		restoreMetadata()
		return nil, fmt.Errorf("H3 prepared key recomputed differently")
	}
	slot.info = hdPreparedInfo{Kind: kind, Key: k, SHA: hash, Reason: reason}
	slot.ready = true
	slot.current = true
	slot.readers = 1
	p.readers++
	// Only invalidated metadata is retired. Different requests/N within the
	// same Validity keep their two bounded outcomes, independently of buffers.
	for i := range p.entries {
		e := &p.entries[i]
		if e != slot && e.info.Key.Validity != p.current {
			e.current = false
			if e.readers == 0 {
				e.rgba = nil
				e.ready = false
			}
		}
	}
	return &hdPreparedLease{owner: p, entry: slot}, nil
}
func (l *hdPreparedLease) Info() (hdPreparedInfo, error) {
	p := l.owner
	p.mu.Lock()
	defer p.mu.Unlock()
	if l.released {
		return hdPreparedInfo{}, fmt.Errorf("released composite lease")
	}
	return l.entry.info, nil
}
func (l *hdPreparedLease) CopyRGBA(dst []byte) error {
	p := l.owner
	p.mu.Lock()
	defer p.mu.Unlock()
	if l.released || len(dst) != len(l.entry.rgba) {
		return fmt.Errorf("released composite or dimensions")
	}
	copy(dst, l.entry.rgba)
	return nil
}
func (l *hdPreparedLease) WriteRGBA(w io.Writer) error {
	p := l.owner
	p.mu.Lock()
	defer p.mu.Unlock()
	if l.released {
		return fmt.Errorf("released composite lease")
	}
	b := l.entry.rgba
	n, e := w.Write(b)
	if e != nil {
		return e
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func (l *hdPreparedLease) Release() {
	if l == nil {
		return
	}
	p := l.owner
	p.mu.Lock()
	defer p.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	e := l.entry
	e.readers--
	p.readers--
	if e.readers == 0 && p.consumeOnRelease {
		e.rgba = nil
	}
	if e.readers == 0 && !e.current {
		e.rgba = nil
		e.ready = false
	}
	p.drainLocked()
}
func (p *hdPreparation) close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.active.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.entries {
		e := &p.entries[i]
		e.current = false
		if e.readers == 0 {
			e.rgba = nil
			e.ready = false
		}
	}
	p.drainLocked()
}

func (p *hdPreparation) drainLocked() {
	if p.closed && p.readers == 0 && p.onDrained != nil {
		fn := p.onDrained
		p.onDrained = nil
		fn()
	}
}
