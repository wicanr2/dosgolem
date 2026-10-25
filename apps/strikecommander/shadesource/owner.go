package shadesource

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
)

const fixedReservation uint64 = 40<<20 | 64<<10

// Owner accounts recorder graphs, two external leases and a single synchronous
// validation/export worker. It never retains a machine or starts a goroutine.
type Owner struct {
	producer       *ProtectedProducer
	blockedThrough uint64
	mu             sync.Mutex
	worker         sync.Mutex
	limit, owned   uint64
	closed         bool
	external       int
	pending, fixed bool
	accepted       int
	graphs         map[*Graph]struct{}
}
type Graph struct {
	owner           *Owner
	data            Packet
	bytes           uint64
	frozen, retired bool
	refs            int
	token           Identity
}
type RawLease struct {
	owner    *Owner
	graph    *Graph
	released bool
}
type Lease struct {
	shared   *Graph
	owner    *Owner
	data     Packet
	declared Declaration
	bytes    uint64
	released bool
}
type Reservation struct {
	producer bool
	owner    *Owner
	bytes    uint64
	released bool
}

func NewOwner(total uint64) (*Owner, error) {
	if total == 0 {
		total = MaxTotalBytes
	}
	if total > MaxTotalBytes || total < fixedReservation {
		return nil, failure(ResourceLimit, "owner fixed work/proof reservation")
	}
	return &Owner{limit: total, owned: fixedReservation, fixed: true, graphs: make(map[*Graph]struct{}, 5)}, nil
}
func (o *Owner) OwnedBytes() uint64 { o.mu.Lock(); defer o.mu.Unlock(); return o.owned }
func (o *Owner) TotalBytes() uint64 { return o.limit }
func (o *Owner) reserve(n uint64) error {
	if n > o.limit-o.owned {
		return failure(ResourceLimit, "owner total limit")
	}
	o.owned += n
	return nil
}
func (o *Owner) ReservePending() (*Reservation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.pending {
		return nil, failure(ResourceLimit, "owner closed/pending occupied")
	}
	if e := o.reserveProducer(8192); e != nil {
		return nil, e
	}
	o.pending = true
	return &Reservation{owner: o, bytes: 8192, producer: o.producer != nil}, nil
}
func (r *Reservation) Release() {
	if r == nil {
		return
	}
	o := r.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !r.released {
		o.releaseReservation(r.bytes, r.producer)
		if r.bytes == 8192 {
			o.pending = false
		}
		r.released = true
		o.freeFixedIfDone()
	}
}
func (o *Owner) NewGraph(id Identity) (*Graph, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	active := 0
	for g := range o.graphs {
		if !g.retired {
			active++
		}
	}
	if o.closed || active >= 3 || len(o.graphs)+o.accepted >= 5 {
		return nil, failure(ResourceLimit, "live graph limit")
	}
	data := InitialPacket(id)
	n := footprint(data) + 256
	if e := o.reserveProducer(n); e != nil {
		return nil, e
	}
	g := &Graph{owner: o, data: data, bytes: n, refs: 1, token: id}
	o.graphs[g] = struct{}{}
	return g, nil
}
func (g *Graph) ID() Identity { o := g.owner; o.mu.Lock(); defer o.mu.Unlock(); return g.token }
func (g *Graph) SetIdentity(id Identity) error {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if g.frozen || g.retired || len(g.data.Builds) != 0 || len(g.data.Calls) != 0 {
		return failure(GenerationMismatch, "nonempty graph identity")
	}
	g.data.Identity = id
	g.token = id
	return nil
}
func (g *Graph) Valid() bool {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	return !g.retired && !o.closed
}
func (g *Graph) ResourceFailure() {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !g.frozen && !g.retired {
		g.data.Counters.ResourceRejected++
	}
}
func (g *Graph) Count(c Counters) {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !g.frozen && !g.retired {
		a := &g.data.Counters
		a.ReadCalls += c.ReadCalls
		a.ReadBytes += c.ReadBytes
		a.Candidates += c.Candidates
		a.Confirmed += c.Confirmed
		a.Voided += c.Voided
		a.ShortReads += c.ShortReads
		a.ResourceRejected += c.ResourceRejected
		a.BindingRejected += c.BindingRejected
		a.RowMismatch += c.RowMismatch
		a.TableMismatch += c.TableMismatch
	}
}
func grow[T any](g *Graph, p *[]T) error {
	if len(*p) < cap(*p) {
		return nil
	}
	newCap := max(4, cap(*p)*2)
	unit := uint64(reflect.TypeFor[T]().Size())
	n := uint64(newCap) * unit
	if g.bytes+n > MaxGraphBytes {
		return failure(ResourceLimit, "graph capacity")
	}
	if e := g.owner.reserveProducer(n); e != nil {
		return e
	}
	v := make([]T, len(*p), newCap)
	copy(v, *p)
	old := uint64(cap(*p)) * unit
	*p = v
	g.bytes += n - old
	g.owner.releaseProducer(old)
	return nil
}
func (g *Graph) reconcile(res *Reservation) error {
	actual := footprint(g.data) + 256
	old := g.bytes
	if actual > MaxGraphBytes {
		return failure(ResourceLimit, "graph bytes")
	}
	// Pending was reserved before any new raw allocation; ownership moves here.
	credit := uint64(0)
	if res != nil && !res.released {
		credit = res.bytes
	}
	if actual > old+credit {
		if e := g.owner.reserveProducer(actual - old - credit); e != nil {
			return e
		}
	} else {
		g.owner.releaseProducer(old + credit - actual)
	}
	g.bytes = actual
	if res != nil {
		res.released = true
		if res.bytes == 8192 {
			g.owner.pending = false
		}
	}
	return nil
}
func (g *Graph) AddProof(p Proof) (uint32, error) {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if g.frozen || g.retired || o.closed {
		return 0, failure(ResourceLimit, "inactive graph")
	}
	for i, x := range g.data.Proofs {
		if x.Steps == p.Steps && x.Deliveries == p.Deliveries {
			return uint32(i), nil
		}
	}
	// Current proof bytes are covered by the fixed 64KiB reservation. Reserve the
	// graph's own reference/copy allowance before appending.
	if e := grow(g, &g.data.Proofs); e != nil {
		return 0, e
	}
	if e := o.reserveProducer(64 << 10); e != nil {
		return 0, e
	}
	r := &Reservation{owner: o, bytes: 64 << 10, producer: o.producer != nil}
	g.data.Proofs = append(g.data.Proofs, p)
	if e := g.reconcile(r); e != nil {
		g.data.Proofs[len(g.data.Proofs)-1] = Proof{}
		g.data.Proofs = g.data.Proofs[:len(g.data.Proofs)-1]
		o.releaseReservation(r.bytes, r.producer)
		return 0, e
	}
	return uint32(len(g.data.Proofs) - 1), nil
}
func (g *Graph) AddBuild(b Build, res *Reservation) error {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if g.frozen || g.retired || o.closed || len(g.data.Builds) >= 32768 {
		return failure(ResourceLimit, "inactive/full graph")
	}
	if e := grow(g, &g.data.Builds); e != nil {
		return e
	}
	g.data.Builds = append(g.data.Builds, b)
	if e := g.reconcile(res); e != nil {
		g.data.Builds[len(g.data.Builds)-1] = Build{}
		g.data.Builds = g.data.Builds[:len(g.data.Builds)-1]
		return e
	}
	return nil
}
func tableView(r *Read) []byte {
	if r == nil || r.State != Complete || len(r.Spans) == 0 || uint32(cap(r.Spans[0].Data)) < r.Requested {
		return nil
	}
	return r.Spans[0].Data[:r.Requested]
}
func sameRead(a, b *Read) bool {
	av, bv := tableView(a), tableView(b)
	return av != nil && bv != nil && bytes.Equal(av, bv)
}
func (g *Graph) AddCall(c R4, res *Reservation) error {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if g.frozen || g.retired || o.closed || len(g.data.Calls) >= 65536 {
		return failure(ResourceLimit, "inactive/full graph")
	}
	if e := grow(g, &g.data.Calls); e != nil {
		return e
	}
	// Content sharing is a byte comparison, never a callback hash. Each witness
	// retains its own address/spans and call origin even when byte backing is shared.
	for i := range g.data.Calls {
		a := g.data.Calls[i].Table
		if sameRead(a, c.Table) {
			pool := tableView(a)
			pos := 0
			for j := range c.Table.Spans {
				n := len(c.Table.Spans[j].Data)
				c.Table.Spans[j].Data = pool[pos : pos+n]
				pos += n
			}
			break
		}
	}
	g.data.Calls = append(g.data.Calls, c)
	if e := g.reconcile(res); e != nil {
		g.data.Calls[len(g.data.Calls)-1] = R4{}
		g.data.Calls = g.data.Calls[:len(g.data.Calls)-1]
		return e
	}
	return nil
}
func (g *Graph) BuildIndex(ref FaceRef) *uint32 {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, b := range g.data.Builds {
		if b.FaceRef == ref {
			return Index(i)
		}
	}
	return nil
}
func (g *Graph) BuildValue(i *uint32) *Build {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if g.frozen || g.retired || i == nil || uint64(*i) >= uint64(len(g.data.Builds)) {
		return nil
	}
	b := g.data.Builds[*i]
	b.Raw5 = cloneRead(b.Raw5)
	b.Method24 = cloneRead(b.Method24)
	b.ProofIndex = cloneIndex(b.ProofIndex)
	return &b
}
func (g *Graph) Seal() { o := g.owner; o.mu.Lock(); defer o.mu.Unlock(); g.frozen = true }
func (g *Graph) drop() {
	g.refs--
	if g.refs == 0 {
		o := g.owner
		o.releaseProducer(g.bytes)
		delete(o.graphs, g)
		g.data = Packet{}
		g.bytes = 0
		g.retired = true
	}
}
func (g *Graph) Retire() {
	if g == nil {
		return
	}
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !g.retired {
		g.retired = true
		g.drop()
	}
}
func (g *Graph) Acquire() (*RawLease, error) {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !g.frozen || g.retired || o.closed || o.external >= 2 {
		return nil, failure(ResourceLimit, "raw lease unavailable")
	}
	if e := o.reserveProducer(128); e != nil {
		return nil, e
	}
	o.external++
	g.refs++
	return &RawLease{owner: o, graph: g}, nil
}
func (g *Graph) MarshalJSON() ([]byte, error) {
	id := g.ID()
	return json.Marshal(struct{ GroupSerial, ShadeEpoch, CUKey uint64 }{id.GroupSerial, id.ShadeEpoch, id.CUKey})
}
func (l *RawLease) Release() {
	if l == nil {
		return
	}
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	defer o.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	o.external--
	o.releaseProducer(128)
	l.graph.drop()
	o.freeFixedIfDone()
}
func clonePacket(p Packet) Packet {
	p.RuleVersion = strings.Clone(RuleVersion)
	proofs := make([]Proof, len(p.Proofs))
	copy(proofs, p.Proofs)
	p.Proofs = proofs
	for i := range p.Proofs {
		reads := make([]ProofRead, len(p.Proofs[i].Reads))
		copy(reads, p.Proofs[i].Reads)
		p.Proofs[i].Reads = reads
		for j := range p.Proofs[i].Reads {
			p.Proofs[i].Reads[j].Capture = *cloneRead(&p.Proofs[i].Reads[j].Capture)
		}
	}
	builds := make([]Build, len(p.Builds))
	copy(builds, p.Builds)
	p.Builds = builds
	for i := range p.Builds {
		b := &p.Builds[i]
		b.ProofIndex = cloneIndex(b.ProofIndex)
		b.Method24 = cloneRead(b.Method24)
		b.Raw5 = cloneRead(b.Raw5)
	}
	pool := make(map[[32]byte][]byte)
	calls := make([]R4, len(p.Calls))
	copy(calls, p.Calls)
	p.Calls = calls
	for i := range p.Calls {
		c := &p.Calls[i]
		if c.FaceRef != nil {
			v := *c.FaceRef
			c.FaceRef = &v
		}
		c.ElementIndex = cloneIndex(c.ElementIndex)
		c.BuildIndex = cloneIndex(c.BuildIndex)
		c.ProofIndex = cloneIndex(c.ProofIndex)
		c.Row = cloneRead(c.Row)
		c.Table = cloneRead(c.Table)
		if c.Table != nil && c.Table.State == Complete {
			data := readBytes(c.Table)
			key := sha256.Sum256(data)
			if old, ok := pool[key]; ok && bytes.Equal(data, old) {
				data = old
			} else {
				pool[key] = data
			}
			pos := 0
			for j := range c.Table.Spans {
				n := len(c.Table.Spans[j].Data)
				c.Table.Spans[j].Data = data[pos : pos+n]
				pos += n
			}
		}
	}
	return p
}
func cloneIndex(p *uint32) *uint32 {
	if p == nil {
		return nil
	}
	n := *p
	return &n
}
func cloneRead(p *Read) *Read {
	if p == nil {
		return nil
	}
	r := *p
	r.Spans = make([]Span, len(p.Spans))
	copy(r.Spans, p.Spans)
	for i := range r.Spans {
		data := make([]byte, len(r.Spans[i].Data))
		copy(data, r.Spans[i].Data)
		r.Spans[i].Data = data
	}
	return &r
}

// Export streams receipt bytes only. Writer must obey io.Writer's borrowing
// contract and must not reenter this Owner. No raw Packet/byte alias is returned.
func (l *RawLease) Export(legacy model.Input, dst io.Writer) (Declaration, error) {
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	if l.released {
		o.mu.Unlock()
		return Declaration{}, failure(ResourceLimit, "released raw lease")
	}
	raw := l.graph.data
	o.mu.Unlock()
	// Hash legacy before creating the private intermediate clone, so each working
	// stage fits the single reserved 32MiB intermediate arena.
	h, e := o.profileLegacyDigest(legacy)
	if e != nil {
		return Declaration{}, e
	}
	if _, e = clonePlan(raw); e != nil {
		return Declaration{}, e
	}
	p := clonePacket(raw)
	p.LegacyInputDigest = h
	if e = qualify(legacy, &p); e != nil {
		return Declaration{}, e
	}
	x, e := SourceDigest(p)
	if e != nil {
		return Declaration{}, e
	}
	d := Declaration{h, p.Identity, x}
	if e = o.profileWritePacket(dst, p); e != nil {
		return Declaration{}, e
	}
	o.mu.Lock()
	revoked := p.Identity.ShadeEpoch <= o.blockedThrough
	o.mu.Unlock()
	if revoked {
		return d, failure(SourceEpochPoisoned, "historical raw packet is not newly eligible")
	}
	return d, nil
}

// Accept is the direct controlled raw -> accepted route. The independent
// declaration is generated from this trusted sealed capture, not foreign input.
func (l *RawLease) Accept(legacy model.Input) (*Lease, error) {
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	if l.released {
		o.mu.Unlock()
		return nil, failure(ResourceLimit, "released raw lease")
	}
	p := l.graph.data
	o.mu.Unlock()
	return o.acceptLocked(legacy, p, nil)
}
func Finalize(legacy model.Input, p Packet) (Packet, Declaration, error) {
	var d Declaration
	h, e := opaque.InputDigest(legacy)
	if e != nil {
		return Packet{}, d, e
	}
	p.LegacyInputDigest = h
	if e = qualify(legacy, &p); e != nil {
		return Packet{}, d, e
	}
	x, e := SourceDigest(p)
	if e != nil {
		return Packet{}, d, e
	}
	return p, Declaration{h, p.Identity, x}, nil
}
func (o *Owner) Accept(legacy model.Input, p Packet, d Declaration) (*Lease, error) {
	o.worker.Lock()
	defer o.worker.Unlock()
	return o.acceptLocked(legacy, p, &d)
}
func (o *Owner) acceptLocked(legacy model.Input, input Packet, external *Declaration) (*Lease, error) {
	o.mu.Lock()
	if input.Identity.ShadeEpoch <= o.blockedThrough {
		o.mu.Unlock()
		return nil, failure(SourceEpochPoisoned, "revoked shade epoch")
	}
	if o.closed || o.external >= 2 || len(o.graphs)+o.accepted >= 5 {
		o.mu.Unlock()
		return nil, failure(ResourceLimit, "external lease limit/closed")
	}
	o.mu.Unlock()
	planned, e := clonePlan(input)
	if e != nil {
		return nil, e
	}
	o.mu.Lock()
	if e = o.reserve(planned); e != nil {
		o.mu.Unlock()
		return nil, e
	}
	o.mu.Unlock()
	success := false
	defer func() {
		if !success {
			o.mu.Lock()
			o.owned -= planned
			o.mu.Unlock()
		}
	}()
	h, e := o.profileLegacyDigest(legacy)
	if e != nil {
		return nil, e
	}
	p := clonePacket(input)
	actual := footprint(p) + 256
	if actual > planned {
		return nil, failure(ResourceLimit, "clone exceeded preflight allocation")
	}
	var d Declaration
	if external == nil {
		p.LegacyInputDigest = h
		if e = qualify(legacy, &p); e != nil {
			return nil, e
		}
		x, e := SourceDigest(p)
		if e != nil {
			return nil, e
		}
		d = Declaration{h, p.Identity, x}
	} else {
		d = *external
		if h != p.LegacyInputDigest || h != d.LegacyInputDigest || p.Identity != d.Identity {
			return nil, failure(IdentityMismatch, "declared legacy/identity mismatch")
		}
		x, e := SourceDigest(p)
		if e != nil {
			return nil, e
		}
		if x != d.DeclaredSourceDigest {
			return nil, failure(IdentityMismatch, "external DeclaredSourceDigest mismatch")
		}
		if e = Validate(legacy, p); e != nil {
			return nil, e
		}
	}
	// No malformed/undeclared input can reach the revocation side effect.
	if p.Status == H4 {
		o.mu.Lock()
		o.revokeLocked(p.Identity.ShadeEpoch)
		o.mu.Unlock()
		return nil, failure(p.Reason, "verified source H4 revoked epoch")
	}
	if p.Status == H3 {
		return nil, failure(p.Reason, "source H3 packet rejected")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if p.Identity.ShadeEpoch <= o.blockedThrough {
		return nil, failure(SourceEpochPoisoned, "revoked during validation")
	}
	if o.closed || o.external >= 2 || len(o.graphs)+o.accepted >= 5 {
		return nil, failure(ResourceLimit, "owner changed during validation")
	}
	o.owned -= planned - actual
	o.external++
	o.accepted++
	success = true
	return &Lease{owner: o, data: p, declared: d, bytes: actual}, nil
}

// clonePlan counts the exact-len output allocations before any output clone.
// Complete equal tables retain one shared backing. The preflight models output
// allocations, then reconciles against actual cloned backing capacities.
func clonePlan(p Packet) (uint64, error) {
	if e := packetShape(&p); e != nil {
		return 0, e
	}
	n := uint64(reflect.TypeOf(p).Size()) + 256
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				n += uint64(v.Type().Elem().Size())
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.Slice:
			n += uint64(v.Len()) * uint64(v.Type().Elem().Size())
			if v.Type().Elem().Kind() != reflect.Uint8 {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i))
				}
			}
		case reflect.String:
			n += uint64(v.Len())
		}
	}
	walk(reflect.ValueOf(p))
	// The output owns one backing for each equal complete table, plus separate
	// per-call Read/Span records. Compute that exact retained-data reduction
	// before allocating any clone, not from input capacities or pointer aliases.
	tables := make(map[[32]byte][]*Read)
	for _, c := range p.Calls {
		r := c.Table
		if r == nil || r.Requested != 4096 || CheckRead(r, r.Start, 4096) != None {
			continue
		}
		h := sha256.New()
		for _, span := range r.Spans {
			_, _ = h.Write(span.Data)
		}
		var key [32]byte
		copy(key[:], h.Sum(nil))
		equal := false
		for _, old := range tables[key] {
			if equalReadContent(old, r) {
				equal = true
				break
			}
		}
		if equal {
			n -= 4096
		} else {
			tables[key] = append(tables[key], r)
		}
	}
	if n > MaxGraphBytes {
		return 0, failure(ResourceLimit, "private clone output bound")
	}
	return n, nil
}
func writePacket(dst io.Writer, p Packet) error {
	if dst == nil {
		return failure(InvalidSchema, "nil receipt writer")
	}
	b, e := MarshalPacket(p)
	if e != nil {
		return e
	}
	n, e := dst.Write(b)
	if e != nil {
		return e
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func (l *Lease) Declaration() Declaration { return l.declared }
func (l *Lease) WriteJSON(dst io.Writer) error {
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	if l.released {
		o.mu.Unlock()
		return failure(ResourceLimit, "released lease")
	}
	p := l.data
	o.mu.Unlock()
	return o.profileWritePacket(dst, p)
}
func (l *Lease) Release() {
	if l == nil {
		return
	}
	o := l.owner
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	defer o.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	o.external--
	o.owned -= l.bytes
	if l.shared != nil {
		o.releaseProducer(128)
		l.shared.drop()
		l.shared = nil
	} else {
		o.accepted--
	}
	l.data = Packet{}
	o.freeFixedIfDone()
}
func (o *Owner) Close() {
	o.worker.Lock()
	defer o.worker.Unlock()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.closed = true
	for g := range o.graphs {
		if !g.retired {
			g.retired = true
			g.drop()
		}
	}
	o.freeFixedIfDone()
}

// footprint accounts struct backing capacities and the union of byte backing
// intervals (including split/subslice views) without dereferencing raw pointers.
func footprint(p Packet) uint64 {
	n := uint64(reflect.TypeOf(p).Size())
	type interval struct{ a, b uintptr }
	var spans []interval
	var visit func(reflect.Value)
	visit = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				n += uint64(v.Type().Elem().Size())
				visit(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				visit(v.Field(i))
			}
		case reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				if v.Cap() > 0 {
					a := v.Pointer()
					spans = append(spans, interval{a, a + uintptr(v.Cap())})
				}
			} else {
				n += uint64(v.Cap()) * uint64(v.Type().Elem().Size())
				for i := 0; i < v.Len(); i++ {
					visit(v.Index(i))
				}
			}
		case reflect.String:
			n += uint64(v.Len())
		}
	}
	visit(reflect.ValueOf(p))
	sort.Slice(spans, func(i, j int) bool { return spans[i].a < spans[j].a })
	var end uintptr
	for _, s := range spans {
		if s.b > end {
			n += uint64(s.b - max(end, s.a))
			end = s.b
		}
	}
	return n
}

// BindCU only changes the not-yet-published graph header, before Seal.
func (g *Graph) BindCU(key uint64) {
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !g.frozen && !g.retired {
		g.data.Identity.CUKey = key
		g.token.CUKey = key
	}
}

// Keep work/proof reservations while a post-Close consumer can still export.
func (o *Owner) freeFixedIfDone() {
	if o.closed && o.external == 0 && len(o.graphs) == 0 && !o.pending && (o.producer == nil || o.producer.legacyUsed == 0 && o.producer.extraUsed == 0) && o.fixed {
		o.owned -= fixedReservation
		o.fixed = false
		if o.producer != nil && !o.producer.freed {
			o.owned -= o.producer.legacyLimit + o.producer.extraLimit
			o.producer.freed = true
		}
	}
}

// RevokeThrough is called only after a complete, correctly located runtime code
// mismatch. Epoch is recorder-local monotonic; one high-water value bounds all
// historical revocations without retaining an unbounded map.
func (o *Owner) RevokeThrough(epoch uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.revokeLocked(epoch)
}
func (l *Lease) Eligible() error {
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if l.released {
		return failure(ResourceLimit, "released lease")
	}
	if l.data.Identity.ShadeEpoch <= o.blockedThrough {
		return failure(SourceEpochPoisoned, "source revoked")
	}
	return nil
}

func (o *Owner) revokeLocked(epoch uint64) { o.blockedThrough = max(o.blockedThrough, epoch) }

func equalReadContent(a, b *Read) bool {
	if a == nil || b == nil || a.Requested != b.Requested {
		return false
	}
	i, j, ai, bj := 0, 0, 0, 0
	for i < len(a.Spans) && j < len(b.Spans) {
		x, y := a.Spans[i].Data[ai:], b.Spans[j].Data[bj:]
		k := min(len(x), len(y))
		if k == 0 {
			return false
		}
		if !bytes.Equal(x[:k], y[:k]) {
			return false
		}
		ai += k
		bj += k
		if ai == len(a.Spans[i].Data) {
			i++
			ai = 0
		}
		if bj == len(b.Spans[j].Data) {
			j++
			bj = 0
		}
	}
	return i == len(a.Spans) && j == len(b.Spans)
}
