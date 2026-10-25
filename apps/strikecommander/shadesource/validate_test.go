package shadesource

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"os"
	"runtime"
	"testing"
	"time"
)

func testRead(start Far, data []byte) *Read {
	return CaptureRead(func(a uint32, b []byte) bool {
		for i := range b {
			off := uint16(a+uint32(i)-uint32(start.Seg)*16) - start.Off
			b[i] = data[int(off)]
		}
		return true
	}, start, uint32(len(data)))
}
func fullTestProof(id Identity) Proof {
	p := Proof{Steps: 1, Base: id.Base, ModelsGeneration: id.ModelsGeneration, ShadeEpoch: id.ShadeEpoch, Reads: []ProofRead{}}
	for i, s := range Signatures() {
		p.Reads = append(p.Reads, ProofRead{uint16(i), *testRead(Far{id.Base + s.Seg, s.Off}, SignatureBytes(s, id.Base))})
	}
	return p
}
func materialFixture(class uint16) (model.Input, Packet) {
	id := emptyID()
	s := emptyInput()
	ref := realspace.FarPtr{Seg: 0x6100, Off: 0x100}
	rec := realspace.FarPtr{Seg: 0x6000, Off: 0x200}
	elem := hd3d.FarPtr{Seg: 0x7000, Off: 0x300}
	f := realspace.FaceObs{Face: ref, FaceVT: class, SI: 5, AL: 1, Returned: true, Elem: realspace.FarPtr{Seg: elem.Seg, Off: elem.Off}}
	src := &realspace.FaceSourceObs{Generation: 1, O1Steps: 5, Element: f.Elem, Proof: &realspace.FaceSourceProof{Rule: realspace.FaceSourceRule, Base: id.Base, Generation: 1}}
	f.Source = src
	for _, sig := range realspace.FaceSourceSignatures() {
		src.Proof.Bytes = append(src.Proof.Bytes, realspace.SourceBytes{Seg: id.Base + sig.Seg, Off: sig.Off, Data: realspace.SourceSignatureBytes(sig, id.Base)})
	}
	var raw [18]byte
	binary.LittleEndian.PutUint16(raw[:], class)
	for k, at := range []int{6, 10, 14} {
		binary.LittleEndian.PutUint16(raw[at:], uint16(0x100+k*18))
		binary.LittleEndian.PutUint16(raw[at+2:], 0x7200)
	}
	for i, role := range []*realspace.FaceSourceRole{&src.F1Call, &src.F1Return, &src.BuildCall, &src.ElementReturn} {
		ctor := uint16(0x1b70)
		if class == 0x33b9 {
			ctor = 0x1664
		}
		*role = realspace.FaceSourceRole{Steps: uint64(10 + i*10), Deliveries: 7, Generation: 1, SS: 0x5000, SP: 0x900, BP: 0x100, SI: 5, Rec: rec, Face: ref, Raw: raw, MethodF1: realspace.FarPtr{Seg: id.Base + 0x20a8, Off: 2}, MethodBuild: realspace.FarPtr{Seg: id.Base + 0x2579, Off: ctor}}
		if i == 0 || i == 2 {
			role.HasFaceArgument = true
			role.FaceArgument = ref
		}
		for k, at := range []int{6, 10, 14} {
			role.FullFar[k] = realspace.FarPtr{Seg: 0x7200, Off: binary.LittleEndian.Uint16(raw[at:])}
			role.FullProj[k][0] = byte(k * 10)
			role.FullProj[k][16] = byte(k + 1)
			role.ConventionProj[k] = role.FullProj[k]
		}
	}
	f.Proj = src.F1Return.FullProj
	s.Objects = []realspace.ObjObs{{Steps: 5, Rec: rec, Faces: []realspace.FaceObs{f}}}
	header := [16]byte{0x5d, 0x33}
	s.Elements = []model.Element{{Pointer: elem, Header: header}}
	call := hd3d.Call{Kind: hd3d.R4, Steps: 50, DS: id.Base + 0x6c4c, SS: 0x5000, SP: 0x900, Element: elem, Verts: make([]byte, 66)}
	call.Params[1] = 3
	s.Raster.Calls = []hd3d.Call{call}
	p := InitialPacket(id)
	p.Proofs = []Proof{fullTestProof(id)}
	w := R4{CallIndex: 0, FaceRef: &FaceRef{0, 0}, ElementIndex: Index(0), Context: Context{50, 7, id.Base + 0x1d5e, 0xea8, call.DS, call.SS, call.SP}, ProofIndex: Index(0)}
	if class == 0x33b9 {
		p.Builds = []Build{{FaceRef: FaceRef{0, 0}, Context: Context{30, 7, id.Base + 0x1ad5, 0xaa5, call.DS, call.SS, call.SP}, ProofIndex: Index(0), Method24: testRead(Far{call.DS, 0x33dd}, []byte{0x33, 0x16, 0xba, 0x27}), Raw5: testRead(Far{ref.Seg, ref.Off + 0x12}, []byte{1, 2, 3, 4, 127})}}
		w.BuildIndex = Index(0)
	}
	p.Calls = []R4{w}
	return s, p
}
func TestSourceRolesTimelineAndRequiredFields(t *testing.T) {
	for _, class := range []uint16{0x33b9, 0x338d} {
		s, p := materialFixture(class)
		p, _, e := Finalize(s, p)
		if e != nil || p.Status != Ready {
			t.Fatalf("class %x baseline %+v %v", class, p.Reason, e)
		}
		for _, mut := range []func(*model.Input, *Packet){func(s *model.Input, p *Packet) { s.Raster.Calls[0].Steps = 35; p.Calls[0].Context.Steps = 35 }, func(s *model.Input, p *Packet) { s.Objects[0].Steps = 15; s.Objects[0].Faces[0].Source.O1Steps = 15 }, func(s *model.Input, p *Packet) { s.Raster.Calls[0].ReadFailed = true }, func(s *model.Input, p *Packet) { s.Raster.Calls[0].Params[1] = 4 }, func(s *model.Input, p *Packet) { s.Raster.Calls[0].Verts = s.Raster.Calls[0].Verts[:65] }, func(s *model.Input, p *Packet) { s.Objects[0].Faces[0].Source.BuildCall.FaceArgument.Off++ }, func(s *model.Input, p *Packet) {
			h := s.Elements[0]
			h.Header[3] = 1
			s.Elements = append(s.Elements, h)
		}} {
			q, v := materialFixture(class)
			mut(&q, &v)
			v, _, e = Finalize(q, v)
			if e != nil || v.Status != H2 {
				t.Fatalf("source negative class %x status=%v reason=%v err=%v", class, v.Status, v.Reason, e)
			}
		}
	}
}
func TestR4FullTableRowAndBudget(t *testing.T) {
	path := os.Getenv("SC_SHADE_TABLE")
	if path == "" {
		t.Skip("SC_SHADE_TABLE frozen original table required")
	}
	table, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if len(table) != 4096 {
		t.Fatal("profile size")
	}
	s, p := materialFixture(0x338d)
	s.Raster.Calls[0].Params[7] = 0x1f04
	s.Raster.Calls[0].Params[8] = 0xc89e
	p.Calls[0].RowOrigin = ExtraRead
	p.Calls[0].Table = testRead(Far{0xc89e, 0x1004}, table)
	p.Calls[0].Row = testRead(Far{0xc89e, 0x1f04}, table[15*256:])
	p, d, e := Finalize(s, p)
	if e != nil || p.Status != Ready {
		t.Fatalf("original table/raw row status %d reason %d %v", p.Status, p.Reason, e)
	}
	q := clonePacket(p)
	q.Calls[0].Row.Spans[0].Data[0] ^= 1
	q, _, e = Finalize(s, q)
	if e != nil || q.Status != H2 || q.Calls[0].Reason != RowMismatch {
		t.Fatal("row mismatch lost H2", q.Reason, e)
	}
	q = clonePacket(p)
	q.Calls[0].Table.Spans[0].Data[0] ^= 1
	q, _, e = Finalize(s, q)
	if e != nil || q.Status != H2 || q.Calls[0].Reason != TableMismatch {
		t.Fatal("table mismatch lost H2", q.Reason, e)
	}
	o, _ := NewOwner(0)
	defer o.Close()
	l, e := o.Accept(s, p, d)
	if e != nil {
		t.Fatal(e)
	}
	l.Release()
	q = clonePacket(p)
	q.Calls[0].Table.State = Budget
	q.Calls[0].Table.Spans = []Span{}
	q, qd, e := Finalize(s, q)
	if e != nil || q.Status != H3 {
		t.Fatal("budget missing H3", q.Reason, e)
	}
	if l, e = o.Accept(s, q, qd); e == nil || l != nil {
		t.Fatal("H3 packet published")
	}
}
func TestRowScalarHandBoundaries(t *testing.T) {
	_, p := materialFixture(0x33b9)
	f := realspace.FaceObs{FaceVT: 0x33b9, Source: &realspace.FaceSourceObs{}}
	for _, v := range []struct {
		s   byte
		row uint16
	}{{0, 15}, {255, 0}, {127, 8}, {128, 7}} {
		b := p.Builds[0]
		b.Raw5 = cloneRead(b.Raw5)
		b.Raw5.Spans[0].Data[4] = v.s
		r, e := RowIndex(&f, &b)
		if e != None || r != v.row {
			t.Fatalf("class7 scalar %d row %d", v.s, r)
		}
	}
	f.FaceVT = 0x338d
	for _, v := range []struct {
		a, b, c byte
		row     uint16
	}{{0, 0, 0, 15}, {255, 255, 255, 0}, {1, 2, 3, 15}, {228, 228, 228, 0}} {
		f.Source.BuildCall.FullProj[0][16] = v.a
		f.Source.BuildCall.FullProj[1][16] = v.b
		f.Source.BuildCall.FullProj[2][16] = v.c
		r, e := RowIndex(&f, nil)
		if e != None || r != v.row {
			t.Fatal("class8 floor or saturation", r)
		}
	}
}
func TestSourceH4LiveLeaseAndRecovery(t *testing.T) {
	o, _ := NewOwner(0)
	s := emptyInput()
	p := InitialPacket(emptyID())
	p.Proofs = []Proof{fullTestProof(p.Identity)}
	p, d, e := Finalize(s, p)
	if e != nil || p.Status != Ready {
		t.Fatal(e, p.Reason)
	}
	g, _ := o.NewGraph(p.Identity)
	if _, e = g.AddProof(p.Proofs[0]); e != nil {
		t.Fatal(e)
	}
	g.Seal()
	raw, _ := g.Acquire()
	old, e := o.Accept(s, p, d)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := leaseJSON(old)
	o.RevokeThrough(1)
	if e = old.Eligible(); e == nil {
		t.Fatal("already accepted lease remains publishable")
	}
	after, _ := leaseJSON(old)
	if string(before) != string(after) {
		t.Fatal("revocation mutated immutable bytes")
	}
	q, qd, e := exportPacket(raw, s)
	if e == nil || q.Status != Ready || qd != d {
		t.Fatal("historical bytes confused with new eligibility")
	}
	old.Release()
	raw.Release()
	g.Retire()
	if l, e := o.Accept(s, p, d); e == nil || l != nil {
		t.Fatal("historical epoch republished after references gone")
	}
	p.Identity.ShadeEpoch = 2
	p.Proofs = []Proof{fullTestProof(p.Identity)}
	p, d, e = Finalize(s, p)
	if e != nil {
		t.Fatal(e)
	}
	old, e = o.Accept(s, p, d)
	if e != nil {
		t.Fatal("new verified epoch failed", e)
	}
	old.Release()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("recovery leaked")
	}
}
func TestSourceH4DuringAccept(t *testing.T) {
	o, _ := NewOwner(0)
	defer o.Close()
	s := emptyInput()
	s.Raster.Buf3D = make([]byte, 8<<20)
	p := InitialPacket(emptyID())
	p.Proofs = []Proof{fullTestProof(p.Identity)}
	p, d, e := Finalize(s, p)
	if e != nil {
		t.Fatal(e)
	}
	baseline := o.OwnedBytes()
	type done struct {
		l *Lease
		e error
	}
	ch := make(chan done, 1)
	go func() { l, e := o.Accept(s, p, d); ch <- done{l, e} }()
	limit := time.Now().Add(5 * time.Second)
	for {
		o.mu.Lock()
		if o.owned > baseline && o.external == 0 {
			o.revokeLocked(1)
			o.mu.Unlock()
			break
		}
		published := o.external != 0
		o.mu.Unlock()
		if published {
			t.Fatal("test missed validation window")
		}
		if time.Now().After(limit) {
			t.Fatal("validation did not start")
		}
		runtime.Gosched()
	}
	got := <-ch
	if got.e == nil || got.l != nil {
		if got.l != nil {
			got.l.Release()
		}
		t.Fatal("in-flight revoked source published")
	}
	if e, ok := got.e.(*Error); !ok || e.Status != H4 {
		t.Fatal("revoked result did not report H4", got.e)
	}
}

func TestProofExactDeadlineRequiresRefresh(t *testing.T) {
	s, p := materialFixture(0x338d)
	s.Raster.Calls[0].Steps = 5000001
	p.Calls[0].Context.Steps = 5000001
	p, _, e := Finalize(s, p)
	if e != nil || p.Status != H2 || p.Reason != ProofUnavailable {
		t.Fatal("old proof accepted exactly at deadline", p.Status, p.Reason, e)
	}
}
