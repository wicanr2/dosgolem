package opaque

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"reflect"
	"sync"
)

// CompositionInput transfers Source to a successful Accept. On rejection the
// caller retains its lease. NoExtra still checks a supplied revocation token.
type CompletionImageEvidence struct {
	Indexed            []byte
	Palette            [256][3]byte
	Steps, CUKey       uint64
	SourceCS, SourceIP uint16
}
type CompositionInput struct {
	Completion  *CompletionImageEvidence
	SourceEpoch uint64
	Opaque      Input
	Source      ShadeSource
	SourceError *Failure
}
type CompositionRenderer struct {
	mu             sync.Mutex
	worker         sync.Mutex
	active         sync.WaitGroup
	core           *Renderer
	budget         CompositionBudget
	baseline       CompositionReservation
	snapshots      map[[32]byte]*CompositionSnapshot
	evidence       TableEvidence
	results        int
	closed         bool
	closeDone      chan struct{}
	blockedThrough uint64
}
type CompositionSnapshot struct {
	manager      *CompositionRenderer
	core         *Snapshot
	source       ShadeSource
	sourceError  *Failure
	sourceEpoch  uint64
	key          [32]byte
	reservation  CompositionReservation
	refs, active int
	released     bool
	slots        [2]compositionSlot
}
type compositionSlot struct {
	done   bool
	result *CompositionResult
	err    error
	hash   [32]byte
}
type CompositionResult struct {
	hash         [32]byte
	manager      *CompositionRenderer
	snapshot     *CompositionSnapshot
	core         *Result
	source       ShadeSource
	tableVersion uint64
	noExtra      bool
	released     bool
}

func NewCompositionRenderer(lib *realspace.Library, mat *MaterialLibrary, t hd3d.Tables, budget CompositionBudget, limits ...Limits) (*CompositionRenderer, error) {
	if budget == nil {
		return nil, fmt.Errorf("missing shared source ledger")
	}
	if lib == nil || mat == nil || len(limits) > 1 {
		return nil, fmt.Errorf("missing libraries or limits")
	}
	v := Limits{}
	if len(limits) == 1 {
		v = limits[0]
	}
	lim, e := normalizeLimits(v)
	if e != nil {
		return nil, e
	}
	if budget.TotalBytes() > lim.TotalBytes {
		return nil, gateFailure(H3, -1, -1, "shared ledger exceeds caller limit")
	}
	// Reserve construction, decoder cache and all retained source-independent
	// data before NewRenderer allocates its table/epoch ledger.
	reservation, e := budget.ReserveComposition((8 << 20) + sourceLedgerBytes + uint64(len(t.R6Table)) + mat.OwnedBytes() + (16 << 20))
	if e != nil {
		return nil, e
	}
	r, e := NewRenderer(lib, mat, t, limits...)
	if e != nil {
		reservation.Release()
		return nil, e
	}
	r.typedFreeze = true
	return &CompositionRenderer{core: r, budget: budget, baseline: reservation, snapshots: map[[32]byte]*CompositionSnapshot{}, closeDone: make(chan struct{})}, nil
}
func compositionKey(in CompositionInput, limit uint64) ([32]byte, error) {
	var d *ShadeDeclaration
	if in.Source != nil {
		v := in.Source.CompositionDeclaration()
		d = &v
	}
	return boundedJSONDigest(struct {
		Input       Input
		Epoch       uint64
		Extra       *ShadeDeclaration
		SourceError *Failure
		Rule        string
	}{Input: Input{Identity: in.Opaque.Identity, Capture: in.Opaque.Capture, SignatureValid: in.Opaque.SignatureValid, ReadFailed: in.Opaque.ReadFailed}, Epoch: in.SourceEpoch, Extra: d, SourceError: in.SourceError, Rule: CompositionRuleVersion + "/" + GlassProfileVersion}, limit)
}
func (c *CompositionRenderer) Accept(in CompositionInput) (*CompositionSnapshot, error) {
	c.worker.Lock()
	defer c.worker.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, gateFailure(H3, -1, -1, "composition renderer closed")
	}
	if cap := in.Opaque.Capture; cap != nil && (cap.Kind == CaptureBufCU || in.Completion != nil) {
		ev := in.Completion
		if ev == nil || len(ev.Indexed) != hd3d.PixelCount || ev.CUKey != in.Opaque.Identity.CUKey || ev.Steps != cap.Steps || ev.SourceCS != cap.SourceCS || ev.SourceIP != cap.SourceIP || sha256.Sum256(ev.Indexed) != cap.IndexedSHA || PaletteHash(ev.Palette) != cap.OriginalPaletteSHA {
			return nil, gateFailure(H3, -1, -1, "BufCU completion evidence differs")
		}
	}
	if in.Source != nil {
		if !c.budget.OwnsCompositionSource(in.Source) {
			return nil, gateFailure(H3, -1, -1, "source lease belongs to a different ledger")
		}
		d := in.Source.CompositionDeclaration()
		if in.SourceEpoch != 0 && in.SourceEpoch != d.Identity.ShadeEpoch {
			return nil, gateFailure(H3, -1, -1, "source revocation epoch differs")
		}
		in.SourceEpoch = d.Identity.ShadeEpoch
	}
	if in.SourceError != nil && in.SourceError.Gate == H4 {
		c.blockedThrough = max(c.blockedThrough, in.SourceEpoch)
		return nil, *in.SourceError
	}
	if c.blockedThrough != 0 && in.SourceEpoch <= c.blockedThrough {
		return nil, gateFailure(H4, -1, -1, "shade epoch disabled")
	}
	if e := c.budget.CompositionEpochEligible(in.SourceEpoch); e != nil {
		return nil, e
	}
	if in.Source != nil {
		if e := in.Source.CompositionEligible(); e != nil {
			return nil, e
		}
		d := in.Source.CompositionDeclaration()
		if d.LegacyInputDigest != in.Opaque.Identity.Group || d.Identity.EXESHA != in.Opaque.Identity.EXE || d.Identity.CUKey != in.Opaque.Identity.CUKey || d.Identity.ModelsGeneration != in.Opaque.Identity.SignatureGeneration || d.DeclaredSourceDigest == ([32]byte{}) {
			return nil, gateFailure(H3, -1, -1, "adopted extra identity differs from legacy scene")
		}
	}
	var key [32]byte
	e := c.budget.CompositionWorkspace(func() error {
		var err error
		key, err = compositionKey(in, c.core.limits.IntermediateBytes)
		return err
	})
	if e != nil {
		return nil, e
	}
	if s := c.snapshots[key]; s != nil && !s.released {
		s.refs++
		if in.Source != nil && in.Source != s.source {
			in.Source.Release()
		}
		return s, nil
	}
	if len(c.snapshots) >= 2 {
		return nil, gateFailure(H3, -1, -1, "composition snapshot slots full")
	}
	// Reserve the real frozen copy plus013a's two N sample allowances before
	// typed cloning. Retained input and glass metadata are charged separately.
	const buffers = uint64((640*400*4 + 960*600*4) * 24)
	charge, e := frozenInputCharge(in.Opaque)
	if e != nil {
		return nil, e
	}
	res, e := c.budget.ReserveComposition(buffers + charge)
	if e != nil {
		return nil, e
	}
	var core *Snapshot
	e = c.budget.CompositionWorkspace(func() error { var x error; core, x = c.core.Accept(in.Opaque); return x })
	if e != nil {
		res.Release()
		return nil, e
	}
	s := &CompositionSnapshot{manager: c, core: core, source: in.Source, sourceError: copyFailure(in.SourceError), sourceEpoch: in.SourceEpoch, key: key, reservation: res, refs: 1}
	c.snapshots[key] = s
	return s, nil
}
func (s *CompositionSnapshot) Render(n int) (*CompositionResult, error) {
	if n != 2 && n != 3 {
		return nil, gateFailure(H3, -1, -1, "scale must be2/3")
	}
	c := s.manager
	c.mu.Lock()
	if s.released || c.closed {
		c.mu.Unlock()
		return nil, gateFailure(H3, -1, -1, "released composition snapshot")
	}
	s.active++
	c.active.Add(1)
	c.mu.Unlock()
	defer c.active.Done()
	c.worker.Lock()
	defer c.worker.Unlock()
	c.mu.Lock()
	defer func() { s.active--; c.mu.Unlock() }()
	idx := n - 2
	prior := s.slots[idx]
	if prior.done && prior.err != nil {
		return nil, prior.err
	}
	if prior.result != nil && !prior.result.released {
		return prior.result, nil
	}
	recompute := prior.done && prior.err == nil
	fail := func(e error) (*CompositionResult, error) {
		if !recompute {
			s.slots[idx] = compositionSlot{done: true, err: e}
		}
		return nil, e
	}
	if c.closed {
		return fail(gateFailure(H3, -1, -1, "composition renderer closed"))
	}
	if c.results >= 2 {
		return fail(gateFailure(H3, -1, -1, "composition results full"))
	}
	c.core.mu.Lock()
	sourceStatus := c.core.sourceStatusLocked(s.core.input.Identity)
	c.core.mu.Unlock()
	if sourceStatus != nil {
		return fail(sourceStatus)
	}
	if s.source != nil {
		if e := s.source.CompositionEligible(); e != nil {
			return fail(e)
		}
	}
	shade := &shadeContext{source: s.source, sourceError: s.sourceError, evidence: &c.evidence}
	var result *Result
	c.mu.Unlock()
	e := c.budget.CompositionWorkspace(func() error { var x error; result, x = c.core.renderComposition(s.core.input, n, shade); return x })
	c.mu.Lock()
	if e == nil && c.closed {
		e = gateFailure(H3, -1, -1, "closed during calculation")
	}
	if e == nil {
		e = c.budget.CompositionEpochEligible(s.sourceEpoch)
		if e != nil {
			shade.sourceH4 = true
		}
	}
	if e == nil && s.source != nil {
		e = s.source.CompositionEligible()
		if e != nil {
			shade.sourceH4 = true
		}
	}
	if e == nil && shade.adopted && !c.evidence.Eligible(shade.tableVersion) {
		e = gateFailure(H3, -1, -1, "table revoked during calculation")
	}
	if e != nil {
		var f Failure
		if errors.As(e, &f) && f.Gate == H4 && !shade.sourceH4 {
			c.core.mu.Lock()
			_ = c.core.blockSourceLocked(s.core.input.Identity)
			c.core.mu.Unlock()
		}
		return fail(e)
	}
	if recompute && result.Hash() != prior.hash {
		return fail(gateFailure(H3, -1, -1, "successful result recomputed with different SHA"))
	}
	r := &CompositionResult{manager: c, snapshot: s, core: result, source: s.source, tableVersion: shade.tableVersion, noExtra: !shade.adopted}
	hash := result.Hash()
	r.hash = hash
	e = c.budget.PublishComposition(s.sourceEpoch, func() error {
		c.results++
		s.slots[idx] = compositionSlot{done: true, result: r, hash: hash}
		return nil
	})
	if e != nil {
		return fail(e)
	}
	return r, nil
}
func (r *CompositionResult) Eligible() error {
	c := r.manager
	c.mu.Lock()
	defer c.mu.Unlock()
	return r.eligibleLocked()
}
func (r *CompositionResult) eligibleLocked() error {
	if r.released || r.snapshot.released || r.manager.closed {
		return gateFailure(H3, -1, -1, "released composition result")
	}
	r.manager.core.mu.Lock()
	sourceErr := r.manager.core.sourceStatusLocked(r.snapshot.core.input.Identity)
	r.manager.core.mu.Unlock()
	if sourceErr != nil {
		return sourceErr
	}
	if r.manager.blockedThrough != 0 && r.snapshot.sourceEpoch <= r.manager.blockedThrough {
		return gateFailure(H4, -1, -1, "shade epoch revoked")
	}
	if e := r.manager.budget.CompositionEpochEligible(r.snapshot.sourceEpoch); e != nil {
		return e
	}
	if r.source != nil {
		if e := r.source.CompositionEligible(); e != nil {
			return e
		}
	}
	if !r.noExtra && !r.manager.evidence.Eligible(r.tableVersion) {
		return gateFailure(H3, -1, -1, "table epoch revoked")
	}
	return nil
}
func (r *CompositionResult) Hash() [32]byte { return r.hash }

func (r *CompositionResult) NoExtra() bool { return r.noExtra }

// TableVersion is the exact accepted table epoch used by this immutable result.
func (r *CompositionResult) TableVersion() uint64 { return r.tableVersion }
func (r *CompositionResult) Sample(x, y, i, j int) (Sample, uint16, bool) {
	c := r.manager
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.eligibleLocked() != nil {
		return Sample{}, 0, false
	}
	v, ok := r.core.Sample(x, y, i, j)
	if !ok {
		return v, 0, false
	}
	at := (y*r.core.width+x)*4 + j*2 + i
	return v, uint16(r.core.samples[at].flags >> 8), true
}
func (r *CompositionResult) Report() Report {
	c := r.manager
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.released {
		return Report{}
	}
	return r.core.Report()
}
func (s *CompositionSnapshot) ReleaseResult(n int) error {
	if n != 2 && n != 3 {
		return gateFailure(H3, -1, -1, "scale must be2/3")
	}
	c := s.manager
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.active != 0 {
		return fmt.Errorf("render in progress")
	}
	slot := &s.slots[n-2]
	if slot.result != nil && !slot.result.released {
		slot.result.released = true
		slot.result.core = nil
		c.results--
	}
	return nil
}
func (s *CompositionSnapshot) Release() error {
	c := s.manager
	c.worker.Lock()
	defer c.worker.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.released {
		return fmt.Errorf("snapshot released")
	}
	if s.active != 0 {
		return fmt.Errorf("render in progress")
	}
	s.refs--
	if s.refs > 0 {
		return nil
	}
	if e := releaseCompositionCore(s.core); e != nil {
		s.refs++
		return e
	}
	s.released = true
	delete(c.snapshots, s.key)
	for i := range s.slots {
		r := s.slots[i].result
		if r != nil && !r.released {
			r.released = true
			r.core = nil
			c.results--
		}
	}
	if s.source != nil {
		s.source.Release()
		s.source = nil
	}
	s.reservation.Release()
	return nil
}
func copyFailure(f *Failure) *Failure {
	if f == nil {
		return nil
	}
	v := *f
	return &v
}
func (c *CompositionRenderer) Close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	c.active.Wait()
	c.worker.Lock()
	defer c.worker.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, s := range c.snapshots {
		for i := range s.slots {
			r := s.slots[i].result
			if r != nil && !r.released {
				r.released = true
				r.core = nil
				c.results--
			}
		}
		_ = releaseCompositionCore(s.core)
		s.released = true
		if s.source != nil {
			s.source.Release()
			s.source = nil
		}
		s.reservation.Release()
		delete(c.snapshots, key)
	}
	c.baseline.Release()
	close(c.closeDone)
	return nil
}

// Frozen JSON decoding may grow slices geometrically. Reserve twice every
// retained backing plus allocation headers, and conservative per-map buckets.
// The32MiB encoder/decoder worker is already in the shared source-owner ledger.
func retainedSceneCharge(in Input) (uint64, error) {
	if e := inputCounts(in.Scene, DefaultLimits()); e != nil {
		return 0, e
	}
	var n uint64
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				n += uint64(v.Type().Elem().Size()) + 256
				walk(v.Elem())
			}
		case reflect.Slice:
			n += uint64(v.Len())*uint64(v.Type().Elem().Size())*2 + 256
			if v.Type().Elem().Kind() != reflect.Uint8 {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i))
				}
			}
		case reflect.Map:
			n += uint64(v.Len())*256 + 256
		case reflect.Struct:
			if v.Type() == reflect.TypeFor[realspace.ObjVerdict]() {
				return
			}
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.String:
			n += uint64(v.Len()) + 256
		}
	}
	n = uint64(reflect.TypeOf(in).Size()) + 256
	walk(reflect.ValueOf(in))
	if n > 32<<20 {
		return 0, gateFailure(H3, -1, -1, "retained frozen input capacity")
	}
	return n, nil
}

func frozenInputCharge(in Input) (uint64, error) {
	n, e := retainedSceneCharge(in)
	if e != nil {
		return 0, e
	}
	r2 := 0
	for _, call := range in.Scene.Raster.Calls {
		if call.Kind == hd3d.R2 {
			r2++
		}
	}
	// At most two fan triangles per clipped triangle, two N result modes.
	glass := uint64(r2) * (2*4096 + 128) * 2
	if glass > 2*DefaultLimits().IntermediateBytes {
		return 0, gateFailure(H3, -1, -1, "retained glass reservation capacity")
	}
	return n + glass, nil
}

func releaseCompositionCore(s *Snapshot) error {
	if e := s.Release(); e != nil {
		return e
	}
	r := s.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.released {
		s.input = Input{}
		s.slots = [2]resultSlot{}
	}
	return nil
}
