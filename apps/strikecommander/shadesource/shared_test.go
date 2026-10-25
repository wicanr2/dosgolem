package shadesource

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"reflect"
	"testing"
)

func sharedRawFixture(t *testing.T) (*Owner, *ProtectedProducer, *Graph, *RawLease, model.Input) {
	t.Helper()
	legacy, p := materialFixture(0x338d)
	// Literal synthetic rows isolate sharing/qualification mechanics. Their SHA
	// deliberately differs from the original profile, so the packet is legal H2.
	table := make([]byte, 4096)
	for i := range table {
		table[i] = byte(i / 256)
	}
	legacy.Raster.Calls[0].Params[7] = 0x1f00
	legacy.Raster.Calls[0].Params[8] = 0xc89e
	p.Calls[0].RowOrigin = ExtraRead
	p.Calls[0].Table = testRead(Far{0xc89e, 0x1000}, table)
	p.Calls[0].Row = testRead(Far{0xc89e, 0x1f00}, table[15*256:])
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	pool, e := o.EnableProtectedProducer(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	g, e := o.NewGraph(p.Identity)
	if e != nil {
		t.Fatal(e)
	}
	for _, proof := range p.Proofs {
		if _, e = g.AddProof(proof); e != nil {
			t.Fatal(e)
		}
	}
	for _, c := range p.Calls {
		res, e := o.ReservePending()
		if e != nil {
			t.Fatal(e)
		}
		if e = g.AddCall(c, res); e != nil {
			res.Release()
			t.Fatal(e)
		}
	}
	g.Seal()
	raw, e := g.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	return o, pool, g, raw, legacy
}
func TestProtectedSharedValidationTransfersOneSlotWithoutRawMutation(t *testing.T) {
	o, pool, g, raw, legacy := sharedRawFixture(t)
	beforeExtra := pool.Usage().ExtraOwned
	rawBefore := g.data
	l, e := raw.ShareValidated(legacy)
	if e != nil {
		t.Fatal(e)
	}
	if !raw.released || raw.graph != nil || o.external != 1 || o.accepted != 0 || l.shared != g {
		t.Fatal("did not transfer the existing external slot")
	}
	if pool.Usage().ExtraOwned != beforeExtra {
		t.Fatal("shared producer byte allocation duplicated")
	}
	if g.data.LegacyInputDigest != rawBefore.LegacyInputDigest || g.data.Status != rawBefore.Status || g.data.Calls[0].Status != rawBefore.Calls[0].Status {
		t.Fatal("raw diagnostics mutated")
	}
	if &l.data.Calls[0] == &g.data.Calls[0] || &l.data.Proofs[0] == &g.data.Proofs[0] {
		t.Fatal("diagnostic arrays alias raw")
	}
	if reflect.ValueOf(l.data.Calls[0].Table.Spans[0].Data).Pointer() != reflect.ValueOf(g.data.Calls[0].Table.Spans[0].Data).Pointer() {
		t.Fatal("immutable table was copied rather than shared")
	}
	if e = Validate(legacy, l.data); e != nil {
		t.Fatal(e)
	}
	d := l.Declaration()
	hash, e := SourceDigest(l.data)
	if e != nil || hash != d.DeclaredSourceDigest {
		t.Fatal("shared declaration failed", e)
	}
	raw.Release()
	g.Retire()
	o.Close()
	if o.OwnedBytes() == 0 {
		t.Fatal("live shared lease freed")
	}
	l.Release()
	if o.OwnedBytes() != 0 {
		t.Fatal("shared total leaked", o.OwnedBytes(), pool.Usage())
	}
}
func TestProtectedSharedScopedSourceCannotOutliveScopeOrPinRetirement(t *testing.T) {
	o, _, g, raw, legacy := sharedRawFixture(t)
	l, e := raw.ShareValidated(legacy)
	if e != nil {
		t.Fatal(e)
	}
	var saved opaque.ShadeSource
	before := o.OwnedBytes()
	e = l.WithCompositionSource(func(s opaque.ShadeSource) error {
		saved = s
		if !o.OwnsCompositionSource(s) {
			t.Fatal("same owner scoped port rejected")
		}
		var tb [4096]byte
		var rb [256]byte
		_, e := s.ReadShadeCall(0, &tb, &rb)
		if e != nil {
			t.Fatal(e)
		}
		tb[0] ^= 255
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if o.OwnedBytes() != before || o.external != 1 {
		t.Fatal("scope leaked bytes or an external slot")
	}
	if saved.CompositionEligible() == nil || o.OwnsCompositionSource(saved) {
		t.Fatal("expired scope retained qualification")
	}
	if _, e = saved.ReadShadeCall(0, nil, nil); e == nil {
		t.Fatal("expired scope can still read")
	}
	g.Retire()
	l.Release()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("expired consumer pinned producer graph", o.OwnedBytes())
	}
}
func TestProtectedSharedValidationH4CannotPublishAndRevokesOld(t *testing.T) {
	o, _, g, raw, legacy := sharedRawFixture(t)
	// Mutation stands for a complete original anchor mismatch before Seal. Only
	// this synthetic fixture is edited; no application callback edits sealed raw.
	g.data.Proofs[0].Reads[0].Capture.Spans[0].Data[0] ^= 1
	before := o.OwnedBytes()
	if l, e := raw.ShareValidated(legacy); e == nil || l != nil {
		t.Fatal("H4 shared packet published")
	}
	if o.OwnedBytes() != before || raw.released || o.blockedThrough < g.ID().ShadeEpoch {
		t.Fatal("failed shared admission changed slot/account or failed revoke")
	}
	raw.Release()
	g.Retire()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}

func TestProtectedSharedScopeReleaseCannotPinProducerDuringNewCapture(t *testing.T) {
	o, pool, g, raw, legacy := sharedRawFixture(t)
	l, e := raw.ShareValidated(legacy)
	if e != nil {
		t.Fatal(e)
	}
	entered, finish, done := make(chan opaque.ShadeSource, 1), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- l.WithCompositionSource(func(s opaque.ShadeSource) error { entered <- s; <-finish; return nil })
	}()
	scoped := <-entered
	g.Retire()
	l.Release()
	if pool.Usage().ExtraOwned != 0 || o.external != 0 || len(o.graphs) != 0 {
		t.Fatal("consumer scope pinned producer graph", pool.Usage())
	}
	if scoped.CompositionEligible() == nil {
		t.Fatal("retired parent remained qualified")
	}
	if _, e = scoped.ReadShadeCall(0, nil, nil); e == nil {
		t.Fatal("retired parent remained readable")
	}
	next, e := o.NewGraph(Identity{ShadeEpoch: 2})
	if e != nil {
		t.Fatal("producer slot was not reusable", e)
	}
	next.Retire()
	close(finish)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
