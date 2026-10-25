package shadesource

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"testing"
	"time"
)

func TestCompositionControlledReadHasNoAliasAndSharesTotal(t *testing.T) {
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	table := make([]byte, 4096)
	table[3] = 17
	row := make([]byte, 256)
	row[3] = 17
	capture := func(data []byte) *Read {
		return CaptureRead(func(_ uint32, dst []byte) bool { copy(dst, data); return true }, Far{Seg: 4}, uint32(len(data)))
	}
	// Internal owned accepted fixture isolates the value-view contract. Packet
	// acceptance and canonical proof validation retain their independent tests.
	l := &Lease{owner: o, data: Packet{Identity: Identity{ShadeEpoch: 1}, Calls: []R4{{CallIndex: 5, FaceRef: &FaceRef{Object: 2, Face: 3}, Table: capture(table), Row: capture(row)}}}}
	var tb [4096]byte
	var rb [256]byte
	w, e := l.ReadShadeCall(5, &tb, &rb)
	if e != nil || !w.TableComplete || !w.RowComplete || w.Object != 2 || tb[3] != 17 {
		t.Fatal(w, e)
	}
	tb[3] = 99
	_, e = l.ReadShadeCall(5, &tb, &rb)
	if e != nil || tb[3] != 17 {
		t.Fatal("mutable raw alias", e)
	}
	before := o.OwnedBytes()
	r, e := o.ReserveComposition(1024)
	if e != nil {
		t.Fatal(e)
	}
	if o.OwnedBytes() != before+1024 {
		t.Fatal("not in source total")
	}
	r.Release()
	r.Release()
	if o.OwnedBytes() != before {
		t.Fatal("reservation leak")
	}
	o.RevokeThrough(1)
	if _, e = l.ReadShadeCall(5, &tb, &rb); e == nil {
		t.Fatal("revoked witness still read")
	}
	if e = l.CompositionEligible(); e == nil {
		t.Fatal("revocation ignored")
	} else if f, ok := e.(opaque.Failure); !ok || f.Gate != opaque.H4 {
		t.Fatal(e)
	}
	o.Close()
	if _, e = o.ReserveComposition(1); e == nil {
		t.Fatal("closed owner accepted new bytes")
	}
}

func TestCompositionPublicationSerializedWithEpochRevocation(t *testing.T) {
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	defer o.Close()
	entered := make(chan struct{})
	finish := make(chan struct{})
	published := make(chan error, 1)
	go func() { published <- o.PublishComposition(1, func() error { close(entered); <-finish; return nil }) }()
	<-entered
	revoked := make(chan struct{})
	go func() { o.RevokeThrough(1); close(revoked) }()
	select {
	case <-revoked:
		t.Fatal("revocation passed an unfinished publication transaction")
	case <-time.After(10 * time.Millisecond):
	}
	close(finish)
	if e = <-published; e != nil {
		t.Fatal(e)
	}
	<-revoked
	called := false
	if e = o.PublishComposition(1, func() error { called = true; return nil }); e == nil || called {
		t.Fatal("revoked epoch published")
	}
	foreign, _ := NewOwner(0)
	defer foreign.Close()
	l := &Lease{owner: foreign}
	if o.OwnsCompositionSource(l) {
		t.Fatal("foreign owner escaped shared ledger")
	}
	if !foreign.OwnsCompositionSource(l) {
		t.Fatal("same owner rejected")
	}
}
