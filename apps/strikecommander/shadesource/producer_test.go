package shadesource

import "testing"

func TestProtectedProducerCannotBeBorrowedByConsumer(t *testing.T) {
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := o.EnableProtectedProducer(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	want := fixedReservation + 64<<20
	if got := o.OwnedBytes(); got != want {
		t.Fatalf("reserved total %d want%d", got, want)
	}
	o.mu.Lock()
	remaining := o.limit - o.owned
	e = o.reserve(remaining)
	o.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	// Global consumer space is full; the same producer still captures.
	if e = p.ReserveLegacy(17); e != nil {
		t.Fatal(e)
	}
	g, e := o.NewGraph(Identity{ShadeEpoch: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := o.ReservePending()
	if e != nil {
		t.Fatal(e)
	}
	r.Release()
	if o.OwnedBytes() != o.TotalBytes() {
		t.Fatal("producer reservation counted twice")
	}
	if p.Usage().LegacyOwned != 17 || p.Usage().ExtraOwned != g.bytes {
		t.Fatal(p.Usage(), g.bytes)
	}
	o.mu.Lock()
	o.owned -= remaining
	o.mu.Unlock()
	g.Retire()
	o.Close()
	if got := o.OwnedBytes(); got != want {
		t.Fatalf("Close freed live legacy reservation: %d", got)
	}
	p.ReleaseLegacy(17)
	if o.OwnedBytes() != 0 {
		t.Fatal("post-Close legacy lifetime leaked", o.OwnedBytes())
	}
}
func TestProtectedExtraExhaustionCannotConsumeLegacy(t *testing.T) {
	o, _ := NewOwner(0)
	p, e := o.EnableProtectedProducer(1024, 1024)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = o.ReservePending(); e == nil {
		t.Fatal("8192 pending exceeded extra quota")
	}
	if p.Usage().ExtraOwned != 0 {
		t.Fatal("failed reservation changed extra ledger")
	}
	if e = p.ReserveLegacy(1024); e != nil {
		t.Fatal(e)
	}
	if e = p.ReserveLegacy(1); e == nil {
		t.Fatal("legacy quota ignored")
	}
	if p.Usage().LegacyOwned != 1024 {
		t.Fatal(p.Usage())
	}
	p.ReleaseLegacy(1024)
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
func TestProtectedEnableAtomicAndBoundedProfiles(t *testing.T) {
	o, _ := NewOwner(fixedReservation + 32<<20)
	before := o.OwnedBytes()
	if _, e := o.EnableProtectedProducer(0, 0); e == nil {
		t.Fatal("global reservation ignored")
	}
	if o.OwnedBytes() != before || o.producer != nil {
		t.Fatal("partial enable")
	}
	if _, e := o.EnableProtectedProducer(MaxLegacyProducer+1, 1); e == nil {
		t.Fatal("raised legacy maximum")
	}
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
func TestProtectedRawLeaseAfterCloseKeepsPoolAndFixed(t *testing.T) {
	o, _ := NewOwner(0)
	p, e := o.EnableProtectedProducer(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	g, e := o.NewGraph(Identity{ShadeEpoch: 1})
	if e != nil {
		t.Fatal(e)
	}
	g.Seal()
	l, e := g.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	want := o.OwnedBytes()
	o.Close()
	if o.OwnedBytes() != want || p.Usage().ExtraOwned != g.bytes+128 {
		t.Fatal("live raw lease freed early", o.OwnedBytes(), p.Usage())
	}
	l.Release()
	if o.OwnedBytes() != 0 {
		t.Fatal("final raw release leak", o.OwnedBytes())
	}
}

func TestProtectedConfigurationRollbackIsAtomicAndRejectsLiveOwners(t *testing.T) {
	o, _ := NewOwner(0)
	before := o.OwnedBytes()
	p, e := o.EnableProtectedProducer(1024, 1024)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.ReserveLegacy(17); e != nil {
		t.Fatal(e)
	}
	if e = p.RollbackUnused(); e == nil {
		t.Fatal("rollback discarded a live allocation")
	}
	p.ReleaseLegacy(17)
	if e = p.RollbackUnused(); e != nil {
		t.Fatal(e)
	}
	if o.OwnedBytes() != before || o.producer != nil {
		t.Fatal("partial configuration remained")
	}
	if e = p.RollbackUnused(); e == nil {
		t.Fatal("double rollback succeeded")
	}
	if _, e = o.EnableProtectedProducer(0, 0); e != nil {
		t.Fatal("rollback poisoned old Owner", e)
	}
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
