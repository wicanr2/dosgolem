package shadesource

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

func legacyFace(s model.Input, ref FaceRef) (*realspace.ObjObs, *realspace.FaceObs) {
	if uint64(ref.Object) >= uint64(len(s.Objects)) {
		return nil, nil
	}
	o := &s.Objects[ref.Object]
	if uint64(ref.Face) >= uint64(len(o.Faces)) {
		return o, nil
	}
	return o, &o.Faces[ref.Face]
}

// CheckRoles retains original operands and checks every common metadata role.
// Only the two explicitly verified constructors are supported by this profile.
func CheckRoles(o *realspace.ObjObs, f *realspace.FaceObs, id Identity) Reason {
	if o == nil || f == nil || f.Source == nil || f.Source.Proof == nil {
		return MissingBinding
	}
	s := f.Source
	p := s.Proof
	if s.Generation != id.ModelsGeneration || p.Generation != s.Generation || p.Base != id.Base {
		return GenerationMismatch
	}
	sigs := realspace.FaceSourceSignatures()
	if p.Rule != realspace.FaceSourceRule || len(p.Bytes) != len(sigs) {
		return ProofUnavailable
	}
	for i, sig := range sigs {
		x := p.Bytes[i]
		want := realspace.SourceSignatureBytes(sig, id.Base)
		if x.Seg != id.Base+sig.Seg || x.Off != sig.Off || len(x.Data) != len(want) {
			return ProofUnavailable
		}
		if !bytes.Equal(x.Data, want) {
			return ProofMismatch
		}
	}
	ctor := uint16(0)
	switch f.FaceVT {
	case 0x33b9:
		ctor = 0x1664
	case 0x338d:
		ctor = 0x1b70
	default:
		return UnsupportedClass
	}
	rs := [4]realspace.FaceSourceRole{s.F1Call, s.F1Return, s.BuildCall, s.ElementReturn}
	if rs[0].Steps <= s.O1Steps {
		return RoleMismatch
	}
	if !f.Returned || f.AL == 0 || s.O1Steps != o.Steps || s.Element != f.Elem || s.O1Steps == 0 {
		return RoleMismatch
	}
	for i, r := range rs {
		if r.ReadFailed || r.Generation != s.Generation || r.Rec != o.Rec || r.Face != f.Face || r.SI != f.SI || r.SS != rs[0].SS || r.BP != rs[0].BP || r.Steps == 0 || i > 0 && r.Steps <= rs[i-1].Steps {
			return RoleMismatch
		}
		if (i == 0 || i == 2) && (!r.HasFaceArgument || r.FaceArgument != f.Face) {
			return RoleMismatch
		}
		if r.Raw != rs[0].Raw || binary.LittleEndian.Uint16(r.Raw[:]) != f.FaceVT {
			return RoleMismatch
		}
		for k, at := range []int{6, 10, 14} {
			far := realspace.FarPtr{Off: binary.LittleEndian.Uint16(r.Raw[at:]), Seg: binary.LittleEndian.Uint16(r.Raw[at+2:])}
			if far != r.FullFar[k] || far.Seg != r.FullFar[0].Seg || r.FullProj[k] != r.ConventionProj[k] {
				return RoleMismatch
			}
		}
		if r.MethodF1 != (realspace.FarPtr{Off: 2, Seg: id.Base + 0x20a8}) || r.MethodBuild != (realspace.FarPtr{Off: ctor, Seg: id.Base + 0x2579}) {
			return RoleMismatch
		}
	}
	if rs[1].FullProj != f.Proj || rs[2].FullProj != rs[3].FullProj {
		return RoleMismatch
	}
	return None
}
func proofRef(p *Packet, i *uint32, steps uint64) Reason {
	if i == nil {
		return MissingProof
	}
	if uint64(*i) >= uint64(len(p.Proofs)) {
		return Bounds
	}
	v := &p.Proofs[*i]
	if steps < v.Steps || steps-v.Steps >= 5000000 {
		return ProofUnavailable
	}
	return CheckProof(v, p.Identity)
}
func buildReason(s model.Input, p *Packet, b *Build) Reason {
	o, f := legacyFace(s, b.FaceRef)
	r := CheckRoles(o, f, p.Identity)
	r = MergeReason(r, proofRef(p, b.ProofIndex, b.Context.Steps))
	if f == nil || f.Source == nil {
		return r
	}
	v := f.Source.BuildCall
	c := b.Context
	if f.FaceVT != 0x33b9 {
		return MergeReason(r, UnsupportedClass)
	}
	if c.Steps != v.Steps || c.Deliveries != v.Deliveries || c.SS != v.SS || c.SP != v.SP || c.CS != p.Identity.Base+0x1ad5 || c.IP != 0xaa5 || c.DS != p.Identity.Base+0x6c4c {
		r = MergeReason(r, RoleMismatch)
	}
	m := CheckRead(b.Method24, Far{c.DS, 0x33dd}, 4)
	r = MergeReason(r, m)
	if m == None {
		a := readBytes(b.Method24)
		if binary.LittleEndian.Uint16(a) != 0x1633 || binary.LittleEndian.Uint16(a[2:]) != p.Identity.Base+0x2579 {
			r = MergeReason(r, RoleMismatch)
		}
	}
	if b.Raw5 == nil {
		return MergeReason(r, MissingRaw5)
	}
	return MergeReason(r, CheckRead(b.Raw5, Far{f.Face.Seg, f.Face.Off + 0x12}, 5))
}
func cFar(p hd3d.FarPtr) Far { return Far{p.Seg, p.Off} }
func RowIndex(f *realspace.FaceObs, b *Build) (uint16, Reason) {
	if f == nil || f.Source == nil {
		return 0, MissingBinding
	}
	switch f.FaceVT {
	case 0x33b9:
		if b == nil || b.Raw5 == nil || b.Raw5.State != Complete {
			return 0, MissingRaw5
		}
		v := readBytes(b.Raw5)
		if len(v) != 5 {
			return 0, MissingRaw5
		}
		return uint16(255-v[4]) >> 4, None
	case 0x338d:
		sum := 0
		for k := 0; k < 3; k++ {
			sum += int(f.Source.BuildCall.FullProj[k][0x10])
		}
		scalar := min(255, 3*sum/8)
		return uint16(255-scalar) >> 4, None
	default:
		return 0, UnsupportedClass
	}
}
func callReason(s model.Input, p *Packet, w *R4) Reason {
	if uint64(w.CallIndex) >= uint64(len(s.Raster.Calls)) {
		return Bounds
	}
	c := s.Raster.Calls[w.CallIndex]
	if c.Kind != hd3d.R4 {
		return RoleMismatch
	}
	r := proofRef(p, w.ProofIndex, w.Context.Steps)
	ctx := w.Context
	if ctx.Steps != c.Steps || ctx.DS != c.DS || ctx.SS != c.SS || ctx.SP != c.SP || ctx.CS != p.Identity.Base+0x1d5e || ctx.IP != 0xea8 {
		r = MergeReason(r, RoleMismatch)
	}
	if w.FaceRef == nil || w.ElementIndex == nil {
		return MergeReason(r, MissingBinding)
	}
	if uint64(*w.ElementIndex) >= uint64(len(s.Elements)) {
		return MergeReason(r, Bounds)
	}
	e := s.Elements[*w.ElementIndex]
	if e.Pointer != c.Element {
		return MergeReason(r, MissingBinding)
	}
	for _, other := range s.Elements {
		if other.Pointer == e.Pointer && other.Header != e.Header {
			r = MergeReason(r, MissingBinding)
		}
	}
	o, f := legacyFace(s, *w.FaceRef)
	r = MergeReason(r, CheckRoles(o, f, p.Identity))
	if f == nil || f.Source == nil {
		return r
	}
	if c.Steps <= f.Source.ElementReturn.Steps || c.ReadFailed || c.Params[1] != 3 || len(c.Verts) != 66 {
		r = MergeReason(r, RoleMismatch)
	}
	if f.Source.Element.Seg != c.Element.Seg || f.Source.Element.Off != c.Element.Off {
		r = MergeReason(r, MissingBinding)
	}
	count := 0
	for _, o := range s.Objects {
		for _, f := range o.Faces {
			if f.Source != nil && f.Source.Element.Seg == c.Element.Seg && f.Source.Element.Off == c.Element.Off {
				count++
			}
		}
	}
	if count != 1 {
		r = MergeReason(r, MissingBinding)
	}
	var b *Build
	if f.FaceVT == 0x33b9 {
		if w.BuildIndex == nil {
			r = MergeReason(r, MissingRaw5)
		} else if uint64(*w.BuildIndex) >= uint64(len(p.Builds)) {
			r = MergeReason(r, Bounds)
		} else {
			b = &p.Builds[*w.BuildIndex]
			if b.FaceRef != *w.FaceRef {
				r = MergeReason(r, MissingBinding)
			}
			r = MergeReason(r, b.Reason)
		}
	} else if w.BuildIndex != nil {
		r = MergeReason(r, RoleMismatch)
	}
	if c.Params[7] == 0 && c.Params[8] == 0 {
		if w.RowOrigin != NoRow || w.Row != nil || w.Table != nil {
			r = MergeReason(r, RoleMismatch)
		}
		return r
	}
	row, rr := RowIndex(f, b)
	r = MergeReason(r, rr)
	if rr != None {
		return r
	}
	if w.RowOrigin == NoRow {
		return MergeReason(r, ReadFailed)
	}
	sf := Far{c.Params[8], c.Params[7]}
	tr := CheckRead(w.Row, sf, 256)
	r = MergeReason(r, tr)
	if w.RowOrigin == OriginalCall {
		if uint32(sf.Off)+256 > 65536 || len(c.Tex.Shade.Spans) != 1 || w.Row == nil || len(w.Row.Spans) != 1 || c.Tex.Shade.Spans[0].Off != sf.Off || !bytes.Equal(c.Tex.Shade.Spans[0].Data, w.Row.Spans[0].Data) {
			r = MergeReason(r, RoleMismatch)
		}
	}
	tf := Far{sf.Seg, sf.Off - row*256}
	fr := CheckRead(w.Table, tf, 4096)
	r = MergeReason(r, fr)
	if fr == None {
		full := readBytes(w.Table)
		if sha256.Sum256(full) != HashLiteral(TableSHAHex) {
			r = MergeReason(r, TableMismatch)
		}
		if tr == None && !bytes.Equal(readBytes(w.Row), full[int(row)*256:int(row+1)*256]) {
			r = MergeReason(r, RowMismatch)
		}
	}
	return r
}
func ordered(p *Packet) error {
	for i := range p.Proofs {
		if i > 0 {
			a, b := p.Proofs[i-1], p.Proofs[i]
			if b.Steps < a.Steps || b.Steps == a.Steps && b.Deliveries <= a.Deliveries {
				return failure(InvalidSchema, "proof order")
			}
		}
	}
	seen := make(map[FaceRef]bool, len(p.Builds))
	for i, b := range p.Builds {
		if seen[b.FaceRef] {
			return failure(InvalidSchema, "duplicate Build face")
		}
		seen[b.FaceRef] = true
		if i > 0 {
			a := p.Builds[i-1]
			if b.Context.Steps < a.Context.Steps || b.Context.Steps == a.Context.Steps && (b.FaceRef.Object < a.FaceRef.Object || b.FaceRef.Object == a.FaceRef.Object && b.FaceRef.Face <= a.FaceRef.Face) {
				return failure(InvalidSchema, "Build order")
			}
		}
	}
	for i := 1; i < len(p.Calls); i++ {
		if p.Calls[i].CallIndex <= p.Calls[i-1].CallIndex {
			return failure(InvalidSchema, "call order")
		}
	}
	return nil
}

// qualify computes finite diagnostic states without changing legacy or raw bytes.
func qualify(s model.Input, p *Packet) error {
	if e := packetShape(p); e != nil {
		return e
	}
	if e := ordered(p); e != nil {
		return e
	}
	if p.Identity.EXESHA != HashLiteral(EXESHAHex) || p.Identity.SignatureSetSHA != HashLiteral(SignatureSetSHAHex) || p.Identity.CUKey != s.Key || s.Key != s.Raster.Key || p.Identity.ResetGeneration == 0 || p.Identity.ShadeEpoch == 0 || p.Identity.GroupSerial == 0 {
		return failure(IdentityMismatch, "source identity")
	}
	r := None
	if len(p.Proofs) == 0 {
		r = MissingProof
	}
	if p.Counters.ResourceRejected != 0 {
		r = MergeReason(r, ResourceLimit)
	}
	for i := range p.Proofs {
		x := &p.Proofs[i]
		x.Reason = CheckProof(x, p.Identity)
		x.Status = statusFor(x.Reason)
		r = MergeReason(r, x.Reason)
	}
	for i := range p.Builds {
		x := &p.Builds[i]
		x.Reason = buildReason(s, p, x)
		x.Status = statusFor(x.Reason)
		r = MergeReason(r, x.Reason)
	}
	for i := range p.Calls {
		x := &p.Calls[i]
		x.Reason = callReason(s, p, x)
		x.Status = statusFor(x.Reason)
		r = MergeReason(r, x.Reason)
	}
	p.Reason = r
	p.Status = statusFor(r)
	return nil
}

// Validate accepts a consistently described H2 diagnostic packet, but never calls
// that source Ready. H4 is visible in Packet.Status; Accept refuses to publish it.
func Validate(s model.Input, p Packet) error {
	if e := packetShape(&p); e != nil {
		return e
	}
	want := p
	want.Proofs = append([]Proof{}, p.Proofs...)
	want.Builds = append([]Build{}, p.Builds...)
	want.Calls = append([]R4{}, p.Calls...)
	if e := qualify(s, &want); e != nil {
		return e
	}
	if p.Status != want.Status || p.Reason != want.Reason {
		return failure(InvalidSchema, "packet diagnostic state differs")
	}
	for i, x := range p.Proofs {
		if x.Status != want.Proofs[i].Status || x.Reason != want.Proofs[i].Reason {
			return failure(InvalidSchema, "proof diagnostic state differs")
		}
	}
	for i, x := range p.Builds {
		if x.Status != want.Builds[i].Status || x.Reason != want.Builds[i].Reason {
			return failure(InvalidSchema, "Build diagnostic state differs")
		}
	}
	for i, x := range p.Calls {
		if x.Status != want.Calls[i].Status || x.Reason != want.Calls[i].Reason {
			return failure(InvalidSchema, "R4 diagnostic state differs")
		}
	}
	return nil
}
