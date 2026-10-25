package shadesource

import "github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"

// CallWitness contains values only. ReadCall copies bytes into fixed caller
// storage; no returned slice or pointer aliases the accepted source packet.
type CallWitness struct {
	CallIndex                  uint32
	Face                       FaceRef
	HasFace                    bool
	Context                    Context
	Status                     Status
	Reason                     Reason
	TableStart, RowStart       Far
	TableComplete, RowComplete bool
	Raw5                       [5]byte
	HasRaw5                    bool
}

func copyReadInto(dst []byte, r *Read) bool {
	if r == nil || uint32(len(dst)) != r.Requested || CheckRead(r, r.Start, r.Requested) != None {
		return false
	}
	at := 0
	for _, s := range r.Spans {
		at += copy(dst[at:], s.Data)
	}
	return at == len(dst)
}

// ReadCall serializes against Release and checks current revocation. The caller
// must reserve its 4352-byte scratch before invoking this method.
func (l *Lease) ReadCall(index uint32, table *[4096]byte, row *[256]byte) (CallWitness, error) {
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if l.released {
		return CallWitness{}, failure(ResourceLimit, "released lease")
	}
	if l.data.Identity.ShadeEpoch <= o.blockedThrough {
		return CallWitness{}, failure(SourceEpochPoisoned, "source revoked")
	}
	for _, c := range l.data.Calls {
		if c.CallIndex != index {
			continue
		}
		w := CallWitness{CallIndex: index, Context: c.Context, Status: c.Status, Reason: c.Reason}
		if c.FaceRef != nil {
			w.Face = *c.FaceRef
			w.HasFace = true
		}
		if c.Table != nil {
			w.TableStart = c.Table.Start
			if table != nil {
				w.TableComplete = copyReadInto(table[:], c.Table)
			}
		}
		if c.Row != nil {
			w.RowStart = c.Row.Start
			if row != nil {
				w.RowComplete = copyReadInto(row[:], c.Row)
			}
		}
		if c.BuildIndex != nil && uint64(*c.BuildIndex) < uint64(len(l.data.Builds)) {
			b := l.data.Builds[*c.BuildIndex]
			if b.Status == Ready {
				w.HasRaw5 = copyReadInto(w.Raw5[:], b.Raw5)
			}
		}
		return w, nil
	}
	return CallWitness{}, failure(MissingBinding, "no same-call witness")
}

// OwnedReservation extends the same Owner total for3b retained data and work.
// It is released only after the final real user, including producer Close.
type OwnedReservation struct {
	owner    *Owner
	bytes    uint64
	released bool
}

func (o *Owner) ReserveOwned(n uint64) (*OwnedReservation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, failure(ResourceLimit, "owner closed")
	}
	if e := o.reserve(n); e != nil {
		return nil, e
	}
	return &OwnedReservation{owner: o, bytes: n}, nil
}
func (r *OwnedReservation) Release() {
	if r == nil {
		return
	}
	o := r.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !r.released {
		r.released = true
		o.owned -= r.bytes
	}
}

func (l *Lease) CompositionDeclaration() opaque.ShadeDeclaration {
	d := l.Declaration()
	i := d.Identity
	return opaque.ShadeDeclaration{LegacyInputDigest: d.LegacyInputDigest, DeclaredSourceDigest: d.DeclaredSourceDigest, Identity: opaque.ShadeIdentity{EXESHA: i.EXESHA, SignatureSetSHA: i.SignatureSetSHA, Base: i.Base, ResetGeneration: i.ResetGeneration, ModelsGeneration: i.ModelsGeneration, ShadeEpoch: i.ShadeEpoch, GroupSerial: i.GroupSerial, CUKey: i.CUKey}}
}
func (l *Lease) CompositionEligible() error {
	e := l.Eligible()
	if e == nil {
		return nil
	}
	g := opaque.H3
	if x, ok := e.(*Error); ok && x.Status == H4 {
		g = opaque.H4
	}
	return opaque.Failure{Gate: g, Object: -1, Face: -1, Pass: -1, Reason: e.Error()}
}
func (l *Lease) ReadShadeCall(index uint32, t *[4096]byte, r *[256]byte) (opaque.ShadeWitness, error) {
	w, e := l.ReadCall(index, t, r)
	if e != nil {
		if q := l.CompositionEligible(); q != nil {
			return opaque.ShadeWitness{}, q
		}
		return opaque.ShadeWitness{}, e
	}
	return opaque.ShadeWitness{CallIndex: w.CallIndex, Object: w.Face.Object, Face: w.Face.Face, HasFace: w.HasFace, Status: uint64(w.Status), Reason: uint64(w.Reason), TableSeg: w.TableStart.Seg, TableOff: w.TableStart.Off, RowSeg: w.RowStart.Seg, RowOff: w.RowStart.Off, TableComplete: w.TableComplete, RowComplete: w.RowComplete, Raw5: w.Raw5, HasRaw5: w.HasRaw5}, nil
}
func (o *Owner) ReserveComposition(n uint64) (opaque.CompositionReservation, error) {
	return o.ReserveOwned(n)
}

// The32MiB source worker reservation is shared, never added as another256MiB
// budget. A closed owner cannot start new3b work.
func (o *Owner) CompositionWorkspace(fn func() error) error {
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	closed := o.closed
	o.mu.Unlock()
	if closed {
		return failure(ResourceLimit, "owner closed")
	}
	return fn()
}

func (o *Owner) CompositionEpochEligible(epoch uint64) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.blockedThrough != 0 && epoch <= o.blockedThrough {
		return opaque.Failure{Gate: opaque.H4, Object: -1, Face: -1, Pass: -1, Reason: "shade epoch revoked"}
	}
	if o.closed {
		return opaque.Failure{Gate: opaque.H3, Object: -1, Face: -1, Pass: -1, Reason: "source producer closed"}
	}
	return nil
}

func (o *Owner) OwnsCompositionSource(source opaque.ShadeSource) bool {
	var l *Lease
	switch v := source.(type) {
	case *Lease:
		l = v
	case *scopedCompositionSource:
		if !v.live() {
			return false
		}
		l = v.parent
	default:
		return false
	}
	if l.owner != o {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return !l.released
}

// PublishComposition performs only the caller's already-prepared pointer/state
// swap under the same lock as RevokeThrough. No hashing, decode or rendering
// may run in fn, and fn must not reenter this Owner.
func (o *Owner) PublishComposition(epoch uint64, fn func() error) error {
	if fn == nil {
		return failure(InvalidSchema, "missing publication")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return failure(ResourceLimit, "owner closed")
	}
	if o.blockedThrough != 0 && epoch <= o.blockedThrough {
		return opaque.Failure{Gate: opaque.H4, Object: -1, Face: -1, Pass: -1, Reason: "shade epoch revoked before publication"}
	}
	return fn()
}
