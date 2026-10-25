package shadesource

// ProtectedProducerVersion is part of every bounded capture/scene identity.
const ProtectedProducerVersion = "013c-producer-48-16-v1"
const MaxLegacyProducer uint64 = 48 << 20
const MaxExtraProducer uint64 = 16 << 20

type ProducerUsage struct {
	LegacyLimit, ExtraLimit, LegacyOwned, ExtraOwned, LegacyPeak, ExtraPeak uint64
}

// ProtectedProducer is a suballocation ledger within its Owner's total. It
// reserves the complete profile once; consumer allocations cannot borrow it.
// The recorder owns all legacy reservations through final scene retirement.
type ProtectedProducer struct {
	owner                                                                 *Owner
	legacyLimit, extraLimit, legacyUsed, extraUsed, legacyPeak, extraPeak uint64
	freed                                                                 bool
}

func (o *Owner) EnableProtectedProducer(legacy, extra uint64) (*ProtectedProducer, error) {
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.producer != nil || len(o.graphs) != 0 || o.pending || o.accepted != 0 || o.external != 0 {
		return nil, failure(ResourceLimit, "producer must be configured before capture")
	}
	if legacy == 0 {
		legacy = MaxLegacyProducer
	}
	if extra == 0 {
		extra = MaxExtraProducer
	}
	if legacy > MaxLegacyProducer || extra > MaxExtraProducer {
		return nil, failure(ResourceLimit, "producer profile exceeds fixed maxima")
	}
	if e := o.reserve(legacy + extra); e != nil {
		return nil, e
	}
	p := &ProtectedProducer{owner: o, legacyLimit: legacy, extraLimit: extra}
	o.producer = p
	return p, nil
}
func (p *ProtectedProducer) Usage() ProducerUsage {
	o := p.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	return ProducerUsage{p.legacyLimit, p.extraLimit, p.legacyUsed, p.extraUsed, p.legacyPeak, p.extraPeak}
}
func (p *ProtectedProducer) ReserveLegacy(n uint64) error {
	o := p.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || p.freed || n > p.legacyLimit-p.legacyUsed {
		return failure(ResourceLimit, "protected legacy producer limit")
	}
	p.legacyUsed += n
	p.legacyPeak = max(p.legacyPeak, p.legacyUsed)
	return nil
}
func (p *ProtectedProducer) ReleaseLegacy(n uint64) {
	o := p.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if n > p.legacyUsed {
		panic("shadesource: legacy ledger underflow")
	}
	p.legacyUsed -= n
	o.freeFixedIfDone()
}

// reserveProducer/releaseProducer require Owner.mu. They cover extra graphs,
// the one optional pending slot and their controlled external lease records.
func (o *Owner) reserveProducer(n uint64) error {
	p := o.producer
	if p == nil {
		return o.reserve(n)
	}
	if p.freed || n > p.extraLimit-p.extraUsed {
		return failure(ResourceLimit, "protected extra producer limit")
	}
	p.extraUsed += n
	p.extraPeak = max(p.extraPeak, p.extraUsed)
	return nil
}
func (o *Owner) releaseProducer(n uint64) {
	p := o.producer
	if p == nil {
		o.owned -= n
		return
	}
	if n > p.extraUsed {
		panic("shadesource: extra ledger underflow")
	}
	p.extraUsed -= n
}
func (o *Owner) releaseReservation(n uint64, producer bool) {
	if producer {
		o.releaseProducer(n)
	} else {
		o.owned -= n
	}
}

// RollbackUnused cancels only an empty, live configuration. The recorder calls
// this after releasing partial control allocations when its Enable fails.
func (p *ProtectedProducer) RollbackUnused() error {
	o := p.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || p.freed || o.producer != p || p.legacyUsed != 0 || p.extraUsed != 0 || len(o.graphs) != 0 || o.pending || o.external != 0 || o.accepted != 0 {
		return failure(ResourceLimit, "protected producer still has owners")
	}
	o.owned -= p.legacyLimit + p.extraLimit
	o.producer = nil
	p.freed = true
	return nil
}
