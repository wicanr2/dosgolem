package strikecommander

import (
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"reflect"
	"sync"
)

// legacyArena accounts allocation capacities, not encoded JSON lengths. Each
// pre-reserved typed backing has one record, regardless of its slice aliases.
// Record backing is itself pre-reserved; zero records are reused after free.
// It contains no raw machine view and never lends a mutable graph to callers.
type legacyAllocation struct {
	key   uintptr
	bytes uint64
}
type legacyArena struct {
	mu              sync.Mutex
	pool            *shadesource.ProtectedProducer
	records         []legacyAllocation
	metadata        uint64
	refs            int
	sealed, retired bool
}

func newLegacyArena(p *shadesource.ProtectedProducer) (*legacyArena, error) {
	n := uint64(reflect.TypeFor[legacyArena]().Size())
	if e := p.ReserveLegacy(n); e != nil {
		return nil, e
	}
	return &legacyArena{pool: p, metadata: n, refs: 1}, nil
}
func (a *legacyArena) slotLocked() (int, error) {
	for i, r := range a.records {
		if r.key == 0 {
			return i, nil
		}
	}
	n := max(8, cap(a.records)*2)
	bytes := uint64(n) * uint64(reflect.TypeFor[legacyAllocation]().Size())
	if e := a.pool.ReserveLegacy(bytes); e != nil {
		return 0, e
	}
	oldBytes := uint64(cap(a.records)) * uint64(reflect.TypeFor[legacyAllocation]().Size())
	old := a.records
	v := make([]legacyAllocation, n)
	copy(v, old)
	a.records = v
	old = nil
	a.metadata += bytes - oldBytes
	a.pool.ReleaseLegacy(oldBytes)
	return len(v) / 2, nil
}
func legacyMake[T any](a *legacyArena, n int) ([]T, error) {
	if a == nil {
		return make([]T, n), nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sealed || a.retired {
		return nil, fmt.Errorf("legacy arena no longer mutable")
	}
	unit := uint64(reflect.TypeFor[T]().Size())
	if n < 0 || unit != 0 && uint64(n) > ^uint64(0)/unit {
		return nil, fmt.Errorf("legacy allocation overflow")
	}
	if n == 0 {
		return []T{}, nil
	}
	bytes := uint64(n) * unit
	if e := a.pool.ReserveLegacy(bytes); e != nil {
		return nil, e
	}
	i, e := a.slotLocked()
	if e != nil {
		a.pool.ReleaseLegacy(bytes)
		return nil, e
	}
	v := make([]T, n)
	a.records[i] = legacyAllocation{reflect.ValueOf(v).Pointer(), bytes}
	return v, nil
}
func legacyNew[T any](a *legacyArena) (*T, error) {
	v, e := legacyMake[T](a, 1)
	if e != nil {
		return nil, e
	}
	return &v[0], nil
}
func (a *legacyArena) freeKey(key uintptr) {
	if a == nil || key == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, r := range a.records {
		if r.key == key {
			a.pool.ReleaseLegacy(r.bytes)
			a.records[i] = legacyAllocation{}
			return
		}
	}
	panic("strikecommander: releasing unowned legacy backing")
}
func legacyFree[T any](a *legacyArena, v []T) {
	if a != nil && cap(v) > 0 {
		a.freeKey(reflect.ValueOf(v).Pointer())
	}
}
func legacyFreePointer[T any](a *legacyArena, v *T) {
	if a != nil && v != nil {
		a.freeKey(reflect.ValueOf(v).Pointer())
	}
}

// legacyAppend retains old and replacement backing charges through the copy.
// Pending/raw element fields already have their own backing ownership records.
func legacyAppend[T any](a *legacyArena, p *[]T, v T) error {
	if a == nil {
		*p = append(*p, v)
		return nil
	}
	if len(*p) == cap(*p) {
		n := max(4, cap(*p)*2)
		next, e := legacyMake[T](a, n)
		if e != nil {
			return e
		}
		copy(next, *p)
		old := *p
		next = next[:len(old)]
		*p = next
		legacyFree(a, old)
	}
	*p = append(*p, v)
	return nil
}
func (a *legacyArena) seal() { a.mu.Lock(); defer a.mu.Unlock(); a.sealed = true }
func (a *legacyArena) acquire() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.sealed || a.retired {
		return fmt.Errorf("legacy scene unavailable")
	}
	a.refs++
	return nil
}
func (a *legacyArena) dropLocked() {
	a.refs--
	if a.refs < 0 {
		panic("strikecommander: legacy ref underflow")
	}
	if a.refs == 0 {
		for _, r := range a.records {
			if r.key != 0 {
				a.pool.ReleaseLegacy(r.bytes)
			}
		}
		a.records = nil
		a.pool.ReleaseLegacy(a.metadata)
		a.metadata = 0
	}
}
func (a *legacyArena) retire() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.retired {
		a.retired = true
		a.dropLocked()
	}
}
func (a *legacyArena) release() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dropLocked()
}
