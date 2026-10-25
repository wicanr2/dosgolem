package shadesource

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"reflect"
	"sync"
)

// ShareValidated consumes a raw lease's existing external slot only on success.
// It retains immutable captured backing and owns separate diagnostic arrays;
// callback capture is never modified. Hash/qualification run outside callbacks.
func (l *RawLease) ShareValidated(legacy model.Input) (*Lease, error) {
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	if l.released || o.closed || o.producer == nil {
		o.mu.Unlock()
		return nil, failure(ResourceLimit, "shared validation requires live protected raw source")
	}
	raw, g := l.graph.data, l.graph
	if raw.Identity.ShadeEpoch <= o.blockedThrough {
		o.mu.Unlock()
		return nil, failure(SourceEpochPoisoned, "revoked shade epoch")
	}
	o.mu.Unlock()
	if e := packetShape(&raw); e != nil {
		return nil, e
	}
	// Only these three arrays are modified by qualify. Read/Span/raw bytes and
	// binding pointers remain immutable and retained by the source Graph.
	n := uint64(reflect.TypeFor[Lease]().Size()) + uint64(len(raw.Proofs))*uint64(reflect.TypeFor[Proof]().Size()) + uint64(len(raw.Builds))*uint64(reflect.TypeFor[Build]().Size()) + uint64(len(raw.Calls))*uint64(reflect.TypeFor[R4]().Size())
	o.mu.Lock()
	e := o.reserve(n)
	o.mu.Unlock()
	if e != nil {
		return nil, e
	}
	success := false
	defer func() {
		if !success {
			o.mu.Lock()
			o.owned -= n
			o.mu.Unlock()
		}
	}()
	p := raw
	p.Proofs = make([]Proof, len(raw.Proofs))
	copy(p.Proofs, raw.Proofs)
	p.Builds = make([]Build, len(raw.Builds))
	copy(p.Builds, raw.Builds)
	p.Calls = make([]R4, len(raw.Calls))
	copy(p.Calls, raw.Calls)
	h, e := o.profileLegacyDigest(legacy)
	if e != nil {
		return nil, e
	}
	p.LegacyInputDigest = h
	if e = qualify(legacy, &p); e != nil {
		return nil, e
	}
	x, e := SourceDigest(p)
	if e != nil {
		return nil, e
	}
	d := Declaration{h, p.Identity, x}
	o.mu.Lock()
	defer o.mu.Unlock()
	if p.Status == H4 {
		o.revokeLocked(p.Identity.ShadeEpoch)
		return nil, failure(p.Reason, "verified source H4 revoked epoch")
	}
	if p.Status == H3 {
		return nil, failure(p.Reason, "source H3 packet rejected")
	}
	if l.released || o.closed || p.Identity.ShadeEpoch <= o.blockedThrough {
		return nil, failure(SourceEpochPoisoned, "source invalidated during shared validation")
	}
	// Transfer the raw ref/slot rather than acquire a third external source.
	l.released = true
	l.graph = nil
	success = true
	return &Lease{owner: o, data: p, declared: d, bytes: n, shared: g}, nil
}

// scopedCompositionSource is valid only during WithCompositionSource. It is a
// value-copy read port, not a retained third source lease or a backing alias.
// Retiring the parent does not wait on this port or pin producer capacity.
type scopedCompositionSource struct {
	mu     sync.Mutex
	parent *Lease
	active bool
}

func (l *Lease) WithCompositionSource(fn func(opaque.ShadeSource) error) error {
	if fn == nil {
		return failure(InvalidSchema, "missing scoped consumer")
	}
	o := l.owner
	o.mu.Lock()
	if l.released || o.closed || l.data.Identity.ShadeEpoch <= o.blockedThrough {
		o.mu.Unlock()
		return failure(SourceEpochPoisoned, "scoped source unavailable")
	}
	n := uint64(reflect.TypeFor[scopedCompositionSource]().Size())
	if e := o.reserve(n); e != nil {
		o.mu.Unlock()
		return e
	}
	s := &scopedCompositionSource{parent: l, active: true}
	o.mu.Unlock()
	defer func() { s.Release(); o.mu.Lock(); o.owned -= n; o.mu.Unlock() }()
	return fn(s)
}
func (s *scopedCompositionSource) live() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.active }
func (s *scopedCompositionSource) CompositionDeclaration() opaque.ShadeDeclaration {
	return s.parent.CompositionDeclaration()
}
func (s *scopedCompositionSource) CompositionEligible() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return opaque.Failure{Gate: opaque.H3, Object: -1, Face: -1, Pass: -1, Reason: "composition source scope ended"}
	}
	return s.parent.CompositionEligible()
}
func (s *scopedCompositionSource) ReadShadeCall(i uint32, t *[4096]byte, r *[256]byte) (opaque.ShadeWitness, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return opaque.ShadeWitness{}, opaque.Failure{Gate: opaque.H3, Object: -1, Face: -1, Pass: -1, Reason: "composition source scope ended"}
	}
	return s.parent.ReadShadeCall(i, t, r)
}
func (s *scopedCompositionSource) Release() { s.mu.Lock(); s.active = false; s.mu.Unlock() }
