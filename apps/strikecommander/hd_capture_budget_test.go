package strikecommander

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"reflect"
	"testing"
)

func TestLegacyArenaPreflightTypedGrowthAndRetiredLease(t *testing.T) {
	o, e := shadesource.NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := o.EnableProtectedProducer(4096, 1024)
	if e != nil {
		t.Fatal(e)
	}
	a, e := newLegacyArena(p)
	if e != nil {
		t.Fatal(e)
	}
	var values []uint64
	for i := uint64(0); i < 9; i++ {
		if e = legacyAppend(a, &values, i); e != nil {
			t.Fatal(e)
		}
	}
	if cap(values) != 16 || values[8] != 8 {
		t.Fatal(values, cap(values))
	}
	want := uint64(reflect.TypeFor[legacyArena]().Size()) + uint64(cap(a.records))*uint64(reflect.TypeFor[legacyAllocation]().Size()) + uint64(cap(values))*8
	if got := p.Usage().LegacyOwned; got != want {
		t.Fatal("capacity not accounted", got, want)
	}
	if p.Usage().LegacyPeak <= want {
		t.Fatal("growth failed to account old/new overlap")
	}
	a.seal()
	if e = a.acquire(); e != nil {
		t.Fatal(e)
	}
	a.retire()
	o.Close()
	if p.Usage().LegacyOwned != want || o.OwnedBytes() == 0 {
		t.Fatal("external lease lost retired charge")
	}
	if _, e = legacyMake[byte](a, 1); e == nil {
		t.Fatal("retired graph accepted write")
	}
	a.release()
	if o.OwnedBytes() != 0 {
		t.Fatal("final release not zero", o.OwnedBytes())
	}
}
func TestLegacyArenaFailureBeforeBackingAndRollback(t *testing.T) {
	o, _ := shadesource.NewOwner(0)
	p, _ := o.EnableProtectedProducer(1024, 1024)
	a, e := newLegacyArena(p)
	if e != nil {
		t.Fatal(e)
	}
	before := p.Usage().LegacyOwned
	if v, e := legacyMake[byte](a, 2048); e == nil || v != nil {
		t.Fatal("oversized backing allocated", len(v), e)
	}
	if p.Usage().LegacyOwned != before || len(a.records) != 0 {
		t.Fatal("preflight failure changed owner")
	}
	// Fit payload but not its first allocation record backing.
	n := 1024 - int(before) - 1
	if v, e := legacyMake[byte](a, n); e == nil || v != nil {
		t.Fatal("unaccounted metadata allowed")
	}
	if p.Usage().LegacyOwned != before {
		t.Fatal("record failure leaked payload reservation")
	}
	a.retire()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
func TestLegacyArenaIndependentProfilesAndTemporaryFree(t *testing.T) {
	o, _ := shadesource.NewOwner(0)
	p, _ := o.EnableProtectedProducer(4096, 1024)
	a, _ := newLegacyArena(p)
	b, e := legacyMake[byte](a, 512)
	if e != nil {
		t.Fatal(e)
	}
	before := p.Usage().LegacyOwned
	legacyFree(a, b)
	b = nil
	if p.Usage().LegacyOwned != before-512 {
		t.Fatal("scratch lifetime not released")
	}
	for i := 0; i < 100; i++ {
		v, e := legacyMake[uint32](a, 7)
		if e != nil {
			t.Fatal(e)
		}
		legacyFree(a, v)
	}
	if cap(a.records) != 8 {
		t.Fatal("record metadata grew despite released slots")
	}
	a.retire()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal(o.OwnedBytes())
	}
}
