package strikecommander

// 013b: optional source capture. No hash, material-library lookup, rendering or
// machine view escapes these callbacks; packet export/validation runs afterwards.
import (
	"crypto/sha256"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"github.com/wicanr2/dosgolem/session"
)

type shadeCapture struct {
	exeHash                               [32]byte
	owner                                 *shadesource.Owner
	reset, epoch, serial, modelGeneration uint64
	proof                                 *shadesource.Proof
	valid, closed                         bool
	next                                  uint64
}
type shadeHit struct {
	build       *shadesource.Build
	call        *shadesource.R4
	reservation *shadesource.Reservation
	epoch       uint64
	element     shadesource.Far
}

// EnableShadeSourceCapture is a separate explicit opt-in, before the first
// Observe. It never implicitly enables the legacy material metadata API.
func (r *ModelRecorder) EnableShadeSourceCapture(exe []byte, totalBytes uint64) error {
	if r.configurationLocked || r.shade != nil || !r.materialSources || r.raster == nil {
		return &shadesource.Error{Status: shadesource.H3, Reason: shadesource.InvalidSchema, Detail: "requires unused SceneRecorder + EnableMaterialSources"}
	}
	actual := sha256.Sum256(exe)
	if actual != shadesource.HashLiteral(shadesource.EXESHAHex) {
		return &shadesource.Error{Status: shadesource.H3, Reason: shadesource.IdentityMismatch, Detail: "actual EXE profile SHA differs"}
	}
	owner, e := shadesource.NewOwner(totalBytes)
	if e != nil {
		return e
	}
	r.shade = &shadeCapture{owner: owner, exeHash: actual, reset: 1, epoch: 1}
	return nil
}
func (r *ModelRecorder) ShadeSourceOwner() *shadesource.Owner {
	if r.shade == nil {
		return nil
	}
	return r.shade.owner
}
func (r *ModelRecorder) CloseShadeSources() {
	if r.shade != nil && !r.shade.closed {
		r.shade.closed = true
		if r.pend != nil {
			r.shadeVoided(r.pend)
		}
		r.shade.owner.Close()
	}
}
func shadeContext(v modelView) shadesource.Context {
	return shadesource.Context{Steps: v.Steps(), Deliveries: v.Deliveries(), CS: v.CS(), IP: v.IP(), DS: v.DS(), SS: v.SS(), SP: v.SP()}
}
func (r *ModelRecorder) shadeID(key uint64) shadesource.Identity {
	s := r.shade
	return shadesource.Identity{EXESHA: s.exeHash, SignatureSetSHA: shadesource.HashLiteral(shadesource.SignatureSetSHAHex), Base: r.base, ResetGeneration: s.reset, ModelsGeneration: r.sourceGeneration, ShadeEpoch: s.epoch, GroupSerial: s.serial + 1, CUKey: key}
}
func (r *ModelRecorder) shadeDrop(g *ModelGroup) {
	if g != nil && g.ShadeSource != nil {
		g.ShadeSource.Retire()
	}
}
func (r *ModelRecorder) shadeReset() {
	if r.shade == nil {
		return
	}
	s := r.shade
	if r.pend != nil && r.pend.shade != nil {
		r.pend.shade.reservation.Release()
	}
	r.shadeDrop(r.open)
	r.shadeDrop(r.await)
	r.shadeDrop(r.shown)
	s.reset++
	s.epoch++
	s.proof = nil
	s.valid = false
	s.next = 0
	s.modelGeneration = 0

}
func (r *ModelRecorder) shadeWake() uint64 {
	s := r.shade
	if s == nil || s.closed || !r.baseOK {
		return session.NoWake
	}
	if s.modelGeneration != r.sourceGeneration {
		return 0
	}
	return s.next
}
func (r *ModelRecorder) shadeBefore(v modelView) {
	s := r.shade
	if s == nil || s.closed || !r.baseOK {
		return
	}
	if s.modelGeneration != r.sourceGeneration {
		s.modelGeneration = r.sourceGeneration
		s.epoch++
		s.proof = nil
		s.valid = false
		s.next = 0
	}
	if v.Steps() < s.next {
		return
	}
	id := r.shadeID(0)
	p := &shadesource.Proof{Steps: v.Steps(), Deliveries: v.Deliveries(), Base: r.base, ModelsGeneration: r.sourceGeneration, ShadeEpoch: s.epoch, Reads: []shadesource.ProofRead{}}
	for i, sig := range shadesource.Signatures() {
		rd := shadesource.CaptureRead(v.ReadBlock, shadesource.Far{Seg: r.base + sig.Seg, Off: sig.Off}, uint32(len(sig.Hex)/2))
		p.Reads = append(p.Reads, shadesource.ProofRead{SignatureIndex: uint16(i), Capture: *rd})
		r.shadeCountRead(rd)
	}
	p.Reason = shadesource.CheckProof(p, id)
	switch p.Reason {
	case shadesource.None:
		p.Status = shadesource.Ready
	case shadesource.ProofMismatch:
		p.Status = shadesource.H4
	case shadesource.ResourceLimit:
		p.Status = shadesource.H3
	default:
		p.Status = shadesource.H2
	}
	// A restored profile starts a new extra epoch; previous poisoned group content
	// is never rebound. The original metadata generation is unchanged.
	if p.Status == shadesource.Ready && s.proof != nil && !s.valid {
		s.epoch++
		p.ShadeEpoch = s.epoch
	}
	if p.Status == shadesource.H4 {
		s.owner.RevokeThrough(s.epoch)
	}
	s.proof = p
	s.valid = p.Status == shadesource.Ready
	if s.valid {
		s.next = v.Steps() + SigCheckSteps
	} else {
		s.next = v.Steps() + flightReverifySteps
	}
	if r.open != nil && r.open.ShadeSource != nil {
		g := r.open.ShadeSource
		id := g.ID()
		if id.ShadeEpoch != s.epoch {
			id.ShadeEpoch = s.epoch
			id.ModelsGeneration = r.sourceGeneration
			id.Base = r.base
			_ = g.SetIdentity(id)
		}
		if _, e := g.AddProof(*p); e != nil {
			g.ResourceFailure()
		}
	}
}
func (r *ModelRecorder) shadeCountRead(rd *shadesource.Read) {
	if r.open == nil || r.open.ShadeSource == nil {
		return
	}
	c := shadesource.Counters{}
	for _, s := range rd.Spans {
		c.ReadCalls++
		c.ReadBytes += uint64(s.Requested)
		if !s.OK {
			c.ShortReads++
		}
	}
	r.open.ShadeSource.Count(c)
}
func (r *ModelRecorder) shadeProofIndex(g *shadesource.Graph) *uint32 {
	if r.shade.proof == nil {
		return nil
	}
	i, e := g.AddProof(*r.shade.proof)
	if e != nil {
		g.ResourceFailure()
		return nil
	}
	return &i
}
func (r *ModelRecorder) captureShade(v modelView, h *modelHit) *shadeHit {
	s := r.shade
	if s == nil || s.closed || r.open == nil || r.open.ShadeSource == nil {
		return nil
	}
	g := r.open.ShadeSource
	isBuild := h.kind == 0xaa5 && h.sourceRole != nil && h.sourceRole.Raw[0] == 0xb9 && h.sourceRole.Raw[1] == 0x33
	isR4 := h.raster != nil && h.raster.kind == "call" && h.raster.call.Kind == hd3d.R4
	if !isBuild && !isR4 {
		return nil
	}
	if !r.shadeBounds() {
		g.ResourceFailure()
		return nil
	}
	g.Count(shadesource.Counters{Candidates: 1})
	reserve, e := s.owner.ReservePending()
	if e != nil {
		g.ResourceFailure()
		return nil
	}
	out := &shadeHit{reservation: reserve, epoch: s.epoch}
	ctx := shadeContext(v)
	if isBuild {
		st, ok := r.frames[modelFrameKey{h.ss, h.bp}]
		if !ok || st.face < 0 {
			reserve.Release()
			g.Count(shadesource.Counters{BindingRejected: 1})
			return nil
		}
		b := &shadesource.Build{FaceRef: shadesource.FaceRef{Object: uint32(st.obj), Face: uint32(st.face)}, Context: ctx, ProofIndex: r.shadeProofIndex(g)}
		out.build = b
		if s.valid && !h.sourceRole.ReadFailed && h.sourceRole.HasFaceArgument && h.sourceRole.FaceArgument == h.sourceRole.Face && ctx.DS == r.base+0x6c4c {
			b.Method24 = shadesource.CaptureRead(v.ReadBlock, shadesource.Far{Seg: ctx.DS, Off: 0x33dd}, 4)
			r.shadeCountRead(b.Method24)
			b.Raw5 = shadesource.CaptureRead(v.ReadBlock, shadesource.Far{Seg: h.sourceRole.Face.Seg, Off: h.sourceRole.Face.Off + 0x12}, 5)
			r.shadeCountRead(b.Raw5)
		}
		return out
	}
	c := &shadesource.R4{Context: ctx, ProofIndex: r.shadeProofIndex(g)}
	out.call = c
	if len(r.raster.stack) == 0 {
		return out
	}
	elem := r.raster.stack[len(r.raster.stack)-1].element
	out.element = shadesource.Far{Seg: elem.Seg, Off: elem.Off}
	ref, ok := r.shadeElementRef(out.element)
	if !ok {
		return out
	}
	c.FaceRef = &ref
	if int(ref.Object) >= len(r.open.Objects) || int(ref.Face) >= len(r.open.Objects[ref.Object].Faces) {
		return out
	}
	f := &r.open.Objects[ref.Object].Faces[ref.Face]
	if f.Source == nil || f.Source.ElementReturn.Steps >= ctx.Steps || f.Source.Element != f.Elem || f.Source.BuildCall.ReadFailed || f.Source.ElementReturn.ReadFailed {
		return out
	}
	header := -1
	for i, e := range r.open.Elements {
		if e.Pointer == elem {
			if header >= 0 && r.open.Elements[header].Header != e.Header {
				return out
			}
			if header < 0 {
				header = i
			}
		}
	}
	if header < 0 {
		return out
	}
	c.ElementIndex = shadesource.Index(header)
	if f.FaceVT == 0x33b9 {
		c.BuildIndex = g.BuildIndex(ref)
	} else if f.FaceVT != 0x338d {
		return out
	}
	if !s.valid {
		return out
	}
	params := h.raster.call.Params
	if params[7] == 0 && params[8] == 0 {
		return out
	}
	row, reason := shadesource.RowIndex(f, g.BuildValue(c.BuildIndex))
	if reason != shadesource.None {
		return out
	}
	// Always use same-view ExtraRead. The legacy capture only has aggregate success
	// and a logical wrap span, neither is a replacement for actual request spans.
	c.RowOrigin = shadesource.ExtraRead
	c.Row = shadesource.CaptureRead(v.ReadBlock, shadesource.Far{Seg: params[8], Off: params[7]}, 256)
	r.shadeCountRead(c.Row)
	c.Table = shadesource.CaptureRead(v.ReadBlock, shadesource.Far{Seg: params[8], Off: params[7] - row*256}, 4096)
	r.shadeCountRead(c.Table)
	return out
}
func (r *ModelRecorder) shadeVoided(h *modelHit) {
	if h == nil || h.shade == nil {
		return
	}
	h.shade.reservation.Release()
	if r.open != nil && r.open.ShadeSource != nil {
		r.open.ShadeSource.Count(shadesource.Counters{Voided: 1})
	}
}
func (r *ModelRecorder) applyShade(h *modelHit) {
	s := r.shade
	if s == nil || s.closed {
		if h.shade != nil {
			h.shade.reservation.Release()
		}
		return
	}
	if h.kind == 0x102 {

		if r.open != nil {
			g, e := s.owner.NewGraph(r.shadeID(0))
			if e == nil {
				r.open.ShadeSource = g
				if s.proof != nil {
					if _, e = g.AddProof(*s.proof); e != nil {
						g.ResourceFailure()
					}
				}
			} else {
				r.open.ShadeSourceError = &shadesource.Error{Status: shadesource.H3, Reason: shadesource.ResourceLimit, Detail: "source graph allocation rejected"}
			}
		}
		return
	}

	sh := h.shade
	if sh == nil {
		return
	}
	defer sh.reservation.Release()
	if r.open == nil || r.open.ShadeSource == nil || s.epoch != sh.epoch {
		return
	}
	g := r.open.ShadeSource
	var e error
	if sh.build != nil {
		e = g.AddBuild(*sh.build, sh.reservation)
	} else if sh.call != nil {
		n := len(r.open.Calls)
		if n == 0 {
			return
		}
		c := r.open.Calls[n-1]
		if c.Kind != hd3d.R4 || c.Steps != sh.call.Context.Steps || c.Element.Seg != sh.element.Seg || c.Element.Off != sh.element.Off {
			return
		}
		sh.call.CallIndex = uint32(n - 1)
		e = g.AddCall(*sh.call, sh.reservation)
	}
	if e != nil {
		g.ResourceFailure()
	} else {
		g.Count(shadesource.Counters{Confirmed: 1})
	}
}
func (r *ModelRecorder) shadeSeal(g *ModelGroup, key uint64) {
	if r.shade == nil || g == nil || g.ShadeSource == nil {
		return
	}
	id := g.ShadeSource.ID()
	id.CUKey = key
	// Only the final CU key is late-bound. No hash or published ModelGroup mutation.
	g.ShadeSource.BindCU(key)
	g.ShadeSource.Seal()
	r.shade.serial++
}

// The lookup uses only the already captured Source.ElementReturn/E1 records.
// It does not allocate a second map or run CheckObject in a callback.
func (r *ModelRecorder) shadeBounds() bool {
	if r.open == nil || len(r.open.Objects) > 256 {
		return false
	}
	n := 0
	for _, o := range r.open.Objects {
		n += len(o.Faces)
		if n > 32768 {
			return false
		}
	}
	return true
}
func (r *ModelRecorder) shadeElementRef(element shadesource.Far) (shadesource.FaceRef, bool) {
	var ref shadesource.FaceRef
	n := 0
	if !r.shadeBounds() {
		return ref, false
	}
	for oi := range r.open.Objects {
		for fi, f := range r.open.Objects[oi].Faces {
			if f.Source != nil && f.Source.ElementReturn.Steps != 0 && f.Source.Element.Seg == element.Seg && f.Source.Element.Off == element.Off {
				n++
				ref = shadesource.FaceRef{Object: uint32(oi), Face: uint32(fi)}
			}
		}
	}
	return ref, n == 1
}
