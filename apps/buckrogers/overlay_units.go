package buckrogers

import (
	"fmt"
	"strings"
)

// Spec 032 (Buck repo): code in GAME.OVR's Turbo Pascal overlay units is
// loaded at a different segment every time. Generic families name such code
// by the unit's file offset plus the offset inside the unit.

// CodeKey is a code address that survives overlay reloads. Unit is the
// unit's FileOfs in GAME.OVR (0 for code in the main program image, where
// Segment is used as is).
type CodeKey struct {
	Unit    uint32
	Segment uint16
	Offset  uint16
}

func (k CodeKey) String() string {
	if k.Unit != 0 {
		return fmt.Sprintf("OVR:%05X:%04X", k.Unit, k.Offset)
	}
	return fmt.Sprintf("%04X:%04X", k.Segment, k.Offset)
}

// ParseCodeKey reads "SSSS:OOOO" or "OVR:FFFFF:OOOO".
func ParseCodeKey(s string) (CodeKey, error) {
	var k CodeKey
	if strings.HasPrefix(s, "OVR:") {
		var unit uint32
		var off uint16
		if n, err := fmt.Sscanf(s, "OVR:%05X:%04X", &unit, &off); err != nil || n != 2 || unit == 0 || len(s) != 14 {
			return k, fmt.Errorf("buckrogers: 程式位址 %q 無效", s)
		}
		return CodeKey{Unit: unit, Offset: off}, nil
	}
	if n, err := fmt.Sscanf(s, "%04X:%04X", &k.Segment, &k.Offset); err != nil || n != 2 || len(s) != 9 {
		return k, fmt.Errorf("buckrogers: 程式位址 %q 無效", s)
	}
	return k, nil
}

// legacyUnitSegments maps the segments at which the older, character-
// creation families recorded overlay code to their units (phase-264 §2,
// identical in three checkpoints).
var legacyUnitSegments = map[uint16]uint32{
	0x37F1: 0x2BA60, 0x2368: 0x21832, 0x2684: 0x24CDF, 0x1C41: 0x1799A, 0x1FEB: 0x27BBE,
	0x2807: 0x081D9, 0x2E13: 0x00904, 0x2A33: 0x07DA5,
}

// LegacyCodeKey interprets an address recorded by an older family.
func LegacyCodeKey(a Address) CodeKey {
	if u, ok := legacyUnitSegments[a.Segment]; ok {
		return CodeKey{Unit: u, Offset: a.Offset}
	}
	return CodeKey{Segment: a.Segment, Offset: a.Offset}
}

// MemReader reads emulated DOS memory by linear address.
type MemReader interface {
	Read8(a uint32) uint8
	Read16(a uint32) uint16
}

const (
	ovrStubScanLimit = 0x4000  // paragraphs: stubs live in the EXE image
	ovrFileSize      = 0x33497 // GAME.OVR length (phase-264)
	ovrLoadSegField  = 0x10
)

type ovrStub struct {
	seg  uint16
	unit uint32
}

// OverlayUnits resolves runtime CS:IP to CodeKey by reading the overlay
// stub headers (spec 032 §3.2). It only reads memory.
type OverlayUnits struct {
	stubs     []ovrStub
	scanned   bool
	Scans     int
	Ambiguous int // lookups where two stubs claimed the same segment
}

func isOvrStub(m MemReader, p uint32) (uint32, bool) {
	a := p << 4
	if m.Read8(a) != 0xCD || m.Read8(a+1) != 0x3F || m.Read8(a+4) == 0xCD ||
		(m.Read8(a+5) == 0xCD && m.Read8(a+6) == 0x3F) {
		return 0, false
	}
	unit := uint32(m.Read16(a+4)) | uint32(m.Read16(a+6))<<16
	if m.Read16(a+8) <= 0x40 || unit == 0 || unit >= ovrFileSize {
		return 0, false
	}
	return unit, true
}

func (o *OverlayUnits) scan(m MemReader) {
	o.stubs = o.stubs[:0]
	for p := uint32(0); p < ovrStubScanLimit; p++ {
		if unit, ok := isOvrStub(m, p); ok {
			o.stubs = append(o.stubs, ovrStub{seg: uint16(p), unit: unit})
		}
	}
	o.scanned = true
	o.Scans++
}

// Invalidate forces a rescan (restore / discontinuity).
func (o *OverlayUnits) Invalidate() { o.scanned = false }

// Key returns the stable identity of a runtime address. When two stubs
// report the same load segment the address is left unresolved (and counted),
// never attributed to one of them by scan order.
func (o *OverlayUnits) Key(m MemReader, a Address) CodeKey {
	if !o.scanned {
		o.scan(m)
	}
	var found CodeKey
	hits := 0
	for i := 0; i < len(o.stubs); i++ {
		s := o.stubs[i]
		if unit, ok := isOvrStub(m, uint32(s.seg)); !ok || unit != s.unit {
			o.scan(m)
			i, hits = -1, 0
			continue
		}
		if ls := m.Read16(uint32(s.seg)<<4 + ovrLoadSegField); ls != 0 && ls == a.Segment {
			found = CodeKey{Unit: s.unit, Offset: a.Offset}
			hits++
		}
	}
	if hits == 1 {
		return found
	}
	if hits > 1 {
		o.Ambiguous++
	}
	return CodeKey{Segment: a.Segment, Offset: a.Offset}
}

// loadedUnit is one unit currently in memory.
type loadedUnit struct {
	unit uint32
	seg  uint16
}

// Loaded lists the units currently loaded (spec 033 §2.1).
func (o *OverlayUnits) Loaded(m MemReader) []loadedUnit {
	if !o.scanned {
		o.scan(m)
	}
	var out []loadedUnit
	for i := 0; i < len(o.stubs); i++ {
		s := o.stubs[i]
		if unit, ok := isOvrStub(m, uint32(s.seg)); !ok || unit != s.unit {
			o.scan(m)
			i, out = -1, out[:0]
			continue
		}
		if ls := m.Read16(uint32(s.seg)<<4 + ovrLoadSegField); ls != 0 {
			out = append(out, loadedUnit{s.unit, ls})
		}
	}
	return out
}

// legacyNoMatch is a segment no older family's address uses.
const legacyNoMatch = 0xFFFF

// LegacyNormaliser maps runtime segments to the segments at which the older
// families recorded the same overlay code (spec 033). It is a plain array
// lookup between rebuilds; only entries set by the last rebuild differ from
// the identity.
type LegacyNormaliser struct {
	tab    [1 << 16]uint16
	set    [1 << 16]bool
	dirty  []uint16
	Builds int
}

var unitLegacySegment = func() map[uint32]uint16 {
	m := map[uint32]uint16{}
	for seg, unit := range legacyUnitSegments {
		m[unit] = seg
	}
	return m
}()

// Rebuild recomputes the table from the current load segments.
func (n *LegacyNormaliser) Rebuild(o *OverlayUnits, m MemReader) {
	for _, s := range n.dirty {
		n.set[s] = false
	}
	n.dirty = n.dirty[:0]
	count := map[uint16]int{}
	units := o.Loaded(m)
	for _, u := range units {
		count[u.seg]++
	}
	for _, u := range units {
		v := uint16(legacyNoMatch)
		if l, ok := unitLegacySegment[u.unit]; ok && count[u.seg] == 1 {
			v = l
		}
		n.tab[u.seg], n.set[u.seg] = v, true
		n.dirty = append(n.dirty, u.seg)
	}
	n.Builds++
}

// Addr normalises one address; segments no unit occupies are unchanged.
func (n *LegacyNormaliser) Addr(a Address) Address {
	if n != nil && n.set[a.Segment] {
		a.Segment = n.tab[a.Segment]
	}
	return a
}
