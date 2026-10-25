package r5asset

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
)

type Model uint8

const (
	F16   Model = 1
	MiG29 Model = 2
)

type Limits struct{ WorkerBytes, ControlBytes uint64 }
type Reader struct {
	instance, generation uint64
	slot                 uint8
}
type readerSlot struct {
	generation uint64
	live       bool
}
type FileIdentity struct {
	Bytes uint64
	SHA   [32]byte
}
type AssetIdentity struct {
	IngestRule, CoordinateRule, MaterialDescriptorRule, StorageLayoutRule uint32
	Model                                                                 Model
	DecoderProfile                                                        [32]byte
	Files                                                                 [5]FileIdentity
	Origin, Provenance, SHA                                               [32]byte
}
type BatchIdentity struct {
	Assets [2]AssetIdentity
	SHA    [32]byte
}
type TextureDescriptor struct {
	Width, Height uint32
	Channels      uint8
	SHA           [32]byte
}
type Stats struct {
	Owned, Peak, GlobalPeak, ControlReservation, WorkerPeak uint64
	Readers, Active                                         uint32
	Loading, Published, Closing, Drained, Disposed          bool
}
type assetData struct {
	bin     [2][]byte
	binHold [2]*allocation
	mesh    [2]meshData
	tex     [6]*texture
}

func (d *assetData) close() {
	if d == nil {
		return
	}
	for i, t := range d.tex {
		if t != nil {
			t.close()
			d.tex[i] = nil
		}
	}
	for i, h := range d.binHold {
		d.bin[i] = nil
		if h != nil {
			h.release()
			d.binHold[i] = nil
		}
	}
}

// Loader is a lower-level app-private factory dependency. It is supported only
// after the SAME owner has its existing HD renderer/worker and protected
// producer configured by the app factory. It does not manufacture that proof,
// create another Owner, or infer the premise from an OwnedBytes threshold.
type Loader struct {
	mu                                           sync.Mutex
	cond                                         sync.Cond
	owner                                        *shadesource.Owner
	root                                         string
	limits                                       Limits
	control                                      *shadesource.OwnedReservation
	instance                                     uint64
	slots                                        [2]readerSlot
	readers, active                              uint32
	loading, closing, drained, disposed, adopted bool
	data                                         *assetData
	identity                                     BatchIdentity
	owned, peak, globalPeak, workerPeak          atomic.Uint64
}

var instances atomic.Uint64

func nextInstance(a *atomic.Uint64) (uint64, error) {
	for {
		n := a.Load()
		if n == ^uint64(0) {
			return 0, reject(Ownership, 0, 200)
		}
		if a.CompareAndSwap(n, n+1) {
			return n + 1, nil
		}
	}
}
func maxAtomic(a *atomic.Uint64, n uint64) {
	for {
		p := a.Load()
		if p >= n || a.CompareAndSwap(p, n) {
			return
		}
	}
}

// NewForConfiguredHD is not a general arbitrary-Owner entry point. The app's
// private startup factory must establish the configuration described above.
func NewForConfiguredHD(o *shadesource.Owner, root string, limits Limits) (*Loader, error) {
	if o == nil || root == "" {
		return nil, reject(Ownership, 0, 201)
	}
	if limits.WorkerBytes == 0 {
		limits.WorkerBytes = 32 << 20
	}
	if limits.ControlBytes == 0 {
		limits.ControlBytes = 64 << 10
	}
	if limits.WorkerBytes > 32<<20 || limits.ControlBytes > 64<<10 {
		return nil, reject(Capacity, 0, 202)
	}
	// Root copy, path assembly, hash/IO controls and all fixed controller values
	// share this reservation. No path-sized string is made before this check.
	base := uint64(unsafe.Sizeof(Loader{})) + uint64(unsafe.Sizeof(assetData{})) + uint64(unsafe.Sizeof(shadesource.OwnedReservation{}))
	if uint64(len(root)) > (^uint64(0)-base-8192)/3 || base+3*uint64(len(root))+8192 > limits.ControlBytes {
		return nil, reject(Capacity, 0, 203)
	}
	h, e := o.ReserveOwned(limits.ControlBytes)
	if e != nil {
		return nil, reject(Capacity, 0, 204)
	}
	id, e := nextInstance(&instances)
	if e != nil {
		h.Release()
		return nil, e
	}
	l := &Loader{owner: o, root: strings.Clone(filepath.Clean(root)), limits: limits, control: h, instance: id}
	l.cond.L = &l.mu
	l.owned.Store(limits.ControlBytes)
	l.peak.Store(limits.ControlBytes)
	l.globalPeak.Store(o.OwnedBytes())
	for m, p := range profiles {
		a := AssetIdentity{IngestRule: 1, CoordinateRule: 1, MaterialDescriptorRule: 1, StorageLayoutRule: 1, Model: Model(p.Model), DecoderProfile: decoderProfile, Origin: p.Origin, Provenance: p.Provenance, SHA: assetHash(p)}
		for i, f := range p.Files {
			a.Files[i] = FileIdentity{f.Bytes, f.SHA}
		}
		l.identity.Assets[m] = a
	}
	l.identity.SHA = batchHash()
	return l, nil
}

type allocation struct {
	loader   *Loader
	hold     *shadesource.OwnedReservation
	n        uint64
	released atomic.Bool
}

const allocationOverhead = uint64(unsafe.Sizeof(allocation{})) + uint64(unsafe.Sizeof(shadesource.OwnedReservation{}))

func (l *Loader) allocate(n uint64) (*allocation, error) {
	l.mu.Lock()
	closed := l.closing || l.disposed
	l.mu.Unlock()
	if closed {
		return nil, reject(Closed, 0, 205)
	}
	if n > ^uint64(0)-allocationOverhead {
		return nil, reject(Capacity, 0, 206)
	}
	n += allocationOverhead
	h, e := l.owner.ReserveOwned(n)
	if e != nil {
		return nil, reject(Capacity, 0, 207)
	}
	a := &allocation{loader: l, hold: h, n: n}
	now := l.owned.Add(n)
	maxAtomic(&l.peak, now)
	maxAtomic(&l.globalPeak, l.owner.OwnedBytes())
	return a, nil
}
func (a *allocation) release() {
	if a == nil || !a.released.CompareAndSwap(false, true) {
		return
	}
	a.hold.Release()
	a.loader.owned.Add(^uint64(a.n - 1))
}
func (l *Loader) stopped() bool { l.mu.Lock(); defer l.mu.Unlock(); return l.closing || l.disposed }
func (l *Loader) finishLocked() {
	if !l.closing || l.loading || l.readers != 0 || l.active != 0 || l.drained {
		return
	}
	l.data.close()
	l.data = nil
	l.drained = true
	l.cond.Broadcast()
}
func (l *Loader) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.closing = true
	l.finishLocked()
	l.mu.Unlock()
}
func (l *Loader) WaitDrained() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for !l.drained {
		l.cond.Wait()
	}
}

// Dispose ends the UNIQUE private factory's controller ownership after Close
// and drain. Until then the still-live controller reservation remains charged.
// It never closes the borrowed shared Owner.
func (l *Loader) Dispose() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.disposed {
		return nil
	}
	if !l.closing || !l.drained || l.loading || l.readers != 0 || l.active != 0 {
		return reject(Ownership, 0, 208)
	}
	l.control.Release()
	l.control = nil
	l.owned.Add(^uint64(l.limits.ControlBytes - 1))
	l.disposed = true
	l.root = ""
	return nil
}
func (l *Loader) Adopt(o *shadesource.Owner, batch [32]byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing || l.disposed {
		return reject(Closed, 0, 209)
	}
	if l.loading || l.data == nil || l.adopted || o != l.owner || batch != l.identity.SHA {
		return reject(Ownership, 0, 210)
	}
	l.adopted = true
	return nil
}
func (l *Loader) Acquire() (Reader, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing || l.disposed {
		return Reader{}, reject(Closed, 0, 211)
	}
	if l.loading || l.data == nil {
		return Reader{}, reject(Ownership, 0, 212)
	}
	for i := range l.slots {
		s := &l.slots[i]
		if !s.live && s.generation != ^uint64(0) {
			s.generation++
			s.live = true
			l.readers++
			return Reader{l.instance, s.generation, uint8(i)}, nil
		}
	}
	return Reader{}, reject(Capacity, 0, 213)
}
func (l *Loader) Release(r Reader) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r.instance != l.instance || int(r.slot) >= len(l.slots) {
		return
	}
	s := &l.slots[r.slot]
	if !s.live || s.generation != r.generation {
		return
	}
	s.live = false
	l.readers--
	l.finishLocked()
}
func (l *Loader) begin(r Reader) (*assetData, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.disposed {
		return nil, reject(Closed, 0, 214)
	}
	if r.instance != l.instance || int(r.slot) >= len(l.slots) {
		return nil, reject(Ownership, 0, 215)
	}
	s := l.slots[r.slot]
	if !s.live || s.generation != r.generation || l.data == nil {
		return nil, reject(Ownership, 0, 216)
	}
	if l.active == 2 {
		return nil, reject(Capacity, 0, 217)
	}
	l.active++
	return l.data, nil
}
func (l *Loader) end() { l.mu.Lock(); l.active--; l.finishLocked(); l.mu.Unlock() }
func modelIndex(m Model) (int, error) {
	if m != F16 && m != MiG29 {
		return 0, reject(Structure, 0, 218)
	}
	return int(m) - 1, nil
}
func (l *Loader) Identity(r Reader) (BatchIdentity, error) {
	_, e := l.begin(r)
	if e != nil {
		return BatchIdentity{}, e
	}
	defer l.end()
	return l.identity, nil
}
func (l *Loader) Count(r Reader, m Model) (int, error) {
	d, e := l.begin(r)
	if e != nil {
		return 0, e
	}
	defer l.end()
	i, e := modelIndex(m)
	if e != nil {
		return 0, e
	}
	return d.mesh[i].Count, nil
}
func (l *Loader) Primitive(r Reader, m Model, p int) (Primitive, error) {
	d, e := l.begin(r)
	if e != nil {
		return Primitive{}, e
	}
	defer l.end()
	i, e := modelIndex(m)
	if e != nil {
		return Primitive{}, e
	}
	if p < 0 || p >= d.mesh[i].Count {
		return Primitive{}, reject(Structure, 0, 219)
	}
	return d.mesh[i].Prim[p], nil
}
func (l *Loader) Vertex(r Reader, m Model, p int, i uint64) (Vertex, error) {
	d, e := l.begin(r)
	if e != nil {
		return Vertex{}, e
	}
	defer l.end()
	mi, e := modelIndex(m)
	if e != nil {
		return Vertex{}, e
	}
	return vertexAt(d.bin[mi], d.mesh[mi], p, i)
}
func (l *Loader) ModelPosition(r Reader, m Model, p int, i uint64) ([3]float64, error) {
	v, e := l.Vertex(r, m, p, i)
	if e != nil {
		return [3]float64{}, e
	}
	return inverse(v.Position), nil
}
func (l *Loader) Index(r Reader, m Model, p int, i uint64) (uint32, error) {
	d, e := l.begin(r)
	if e != nil {
		return 0, e
	}
	defer l.end()
	mi, e := modelIndex(m)
	if e != nil {
		return 0, e
	}
	return indexAt(d.bin[mi], d.mesh[mi], p, i)
}
func (l *Loader) Material(r Reader, m Model, i int) (Material, error) {
	d, e := l.begin(r)
	if e != nil {
		return Material{}, e
	}
	defer l.end()
	mi, e := modelIndex(m)
	if e != nil {
		return Material{}, e
	}
	if i < 0 || i >= d.mesh[mi].Count {
		return Material{}, reject(Structure, 0, 220)
	}
	return d.mesh[mi].Materials[i], nil
}
func (l *Loader) Texture(r Reader, m Model, slot int) (TextureDescriptor, error) {
	_, e := l.begin(r)
	if e != nil {
		return TextureDescriptor{}, e
	}
	defer l.end()
	mi, e := modelIndex(m)
	if e != nil {
		return TextureDescriptor{}, e
	}
	if slot < 0 || slot >= 3 {
		return TextureDescriptor{}, reject(Structure, 0, 221)
	}
	f := profiles[mi].Images[slot]
	c := uint8(3)
	if f.Color == 6 {
		c = 4
	}
	return TextureDescriptor{4096, 4096, c, f.SHA}, nil
}
func (l *Loader) Pixel(r Reader, m Model, slot, x, y int) ([4]byte, uint8, error) {
	d, e := l.begin(r)
	if e != nil {
		return [4]byte{}, 0, e
	}
	defer l.end()
	mi, e := modelIndex(m)
	if e != nil {
		return [4]byte{}, 0, e
	}
	if slot < 0 || slot >= 3 || x < 0 || x >= 4096 || y < 0 || y >= 4096 {
		return [4]byte{}, 0, reject(Structure, 0, 222)
	}
	t := d.tex[mi*3+slot]
	return t.pixel(x, y)
}
func (l *Loader) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	control := uint64(0)
	if l.control != nil {
		control = l.limits.ControlBytes
	}
	return Stats{Owned: l.owned.Load(), Peak: l.peak.Load(), GlobalPeak: l.globalPeak.Load(), ControlReservation: control, WorkerPeak: l.workerPeak.Load(), Readers: l.readers, Active: l.active, Loading: l.loading, Published: l.data != nil, Closing: l.closing, Drained: l.drained, Disposed: l.disposed}
}
