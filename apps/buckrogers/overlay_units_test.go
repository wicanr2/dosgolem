package buckrogers

import (
	"encoding/binary"
	"testing"
)

type fakeMem []byte

func (m fakeMem) Read8(a uint32) uint8   { return m[a] }
func (m fakeMem) Read16(a uint32) uint16 { return binary.LittleEndian.Uint16(m[a:]) }

// putStub writes a synthetic overlay stub header (layout of phase-264 §2).
func putStub(m fakeMem, seg uint16, unit uint32, size, load uint16) {
	a := uint32(seg) << 4
	m[a], m[a+1] = 0xCD, 0x3F
	binary.LittleEndian.PutUint32(m[a+4:], unit)
	binary.LittleEndian.PutUint16(m[a+8:], size)
	binary.LittleEndian.PutUint16(m[a+ovrLoadSegField:], load)
}

func TestOverlayUnitsResolveLoadSegment(t *testing.T) {
	m := make(fakeMem, 1<<20)
	putStub(m, 0x21F, 0x2BA60, 0x24F1, 0x37F1)
	putStub(m, 0x206, 0x27BBE, 0x37CE, 0)
	// A jump-table entry (CD 3F followed by another CD 3F) is not a header.
	m[0x2400], m[0x2401], m[0x2405], m[0x2406] = 0xCD, 0x3F, 0xCD, 0x3F
	var o OverlayUnits
	if k := o.Key(m, Address{0x37F1, 0x0243}); k != (CodeKey{Unit: 0x2BA60, Offset: 0x0243}) {
		t.Fatalf("loaded unit: %v", k)
	}
	if len(o.stubs) != 2 || o.Scans != 1 {
		t.Fatalf("stubs %+v scans %d", o.stubs, o.Scans)
	}
	// The unit moves: the same code now answers at the new segment.
	binary.LittleEndian.PutUint16(m[0x21F0+ovrLoadSegField:], 0x2A65)
	if k := o.Key(m, Address{0x2A65, 0x2163}); k.String() != "OVR:2BA60:2163" {
		t.Fatalf("moved unit: %v", k)
	}
	if k := o.Key(m, Address{0x37F1, 0x0243}); k != (CodeKey{Segment: 0x37F1, Offset: 0x0243}) {
		t.Fatalf("stale segment must not resolve: %v", k)
	}
	// Not loaded (LoadSeg 0) never matches segment 0.
	if k := o.Key(m, Address{0, 0x10}); k.Unit != 0 {
		t.Fatalf("unloaded unit matched: %v", k)
	}
	// Two units claiming one segment: unresolved, counted.
	binary.LittleEndian.PutUint16(m[0x2060+ovrLoadSegField:], 0x2A65)
	if k := o.Key(m, Address{0x2A65, 0x2163}); k.Unit != 0 || o.Ambiguous != 1 {
		t.Fatalf("ambiguous segment: %v %d", k, o.Ambiguous)
	}
	binary.LittleEndian.PutUint16(m[0x2060+ovrLoadSegField:], 0)
	// A stub that stops looking like a header triggers a rescan.
	m[0x2060] = 0
	o.Key(m, Address{0x0763, 0x0424})
	if o.Scans != 2 || len(o.stubs) != 1 {
		t.Fatalf("rescan: scans %d stubs %+v", o.Scans, o.stubs)
	}
}

func TestCodeKeyParseAndLegacy(t *testing.T) {
	for _, s := range []string{"0763:0424", "OVR:2BA60:0243"} {
		k, err := ParseCodeKey(s)
		if err != nil || k.String() != s {
			t.Errorf("%s: %v %v", s, k, err)
		}
	}
	for _, s := range []string{"763:0424", "OVR:2BA6:0243", "OVR:00000:0243", "OVR:2BA60:243", "x"} {
		if _, err := ParseCodeKey(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
	if k := LegacyCodeKey(Address{0x37F1, 0x101E}); k.String() != "OVR:2BA60:101E" {
		t.Errorf("legacy 37F1: %v", k)
	}
	if k := LegacyCodeKey(Address{0x0763, 0x1282}); k.String() != "0763:1282" {
		t.Errorf("main image: %v", k)
	}
	owned := OwnedCallers(map[string][]byte{"skill-exit-events.tsv": []byte("x\t37F1:101E\n")})
	if _, err := LoadEngineDispatchCallers([]byte("caller\tnote\nOVR:2BA60:101E\tx\n"), owned); err == nil {
		t.Error("owned overlay caller accepted")
	}
}

func TestLegacyNormaliser(t *testing.T) {
	m := make(fakeMem, 1<<20)
	putStub(m, 0x21F, 0x2BA60, 0x24F1, 0x37F1) // character-creation segment
	putStub(m, 0x206, 0x27BBE, 0x37CE, 0x1C41) // not where it was recorded
	putStub(m, 0x1C2, 0x1B6E8, 0x1227, 0x3881) // unit without a legacy segment
	var o OverlayUnits
	var n LegacyNormaliser
	n.Rebuild(&o, m)
	if a := n.Addr(Address{0x37F1, 0x15BD}); a != (Address{0x37F1, 0x15BD}) {
		t.Fatalf("identity in character creation: %v", a)
	}
	if a := n.Addr(Address{0x1C41, 0x0388}); a != (Address{0x1FEB, 0x0388}) {
		t.Fatalf("27BBE at 1C41 must read as its recorded 1FEB: %v", a)
	}
	if a := n.Addr(Address{0x3881, 0x007C}); a.Segment != legacyNoMatch {
		t.Fatalf("unit without legacy segment: %v", a)
	}
	if a := n.Addr(Address{0x0763, 0x0424}); a != (Address{0x0763, 0x0424}) {
		t.Fatalf("main image changed: %v", a)
	}
	// The unit moves; the old segment returns to identity.
	binary.LittleEndian.PutUint16(m[0x21F0+ovrLoadSegField:], 0x216E)
	n.Rebuild(&o, m)
	if a := n.Addr(Address{0x216E, 0x15BD}); a != (Address{0x37F1, 0x15BD}) {
		t.Fatalf("moved unit: %v", a)
	}
	if a := n.Addr(Address{0x37F1, 0x15BD}); a != (Address{0x37F1, 0x15BD}) {
		t.Fatalf("vacated segment: %v", a)
	}
	// Two units on one segment: no match.
	binary.LittleEndian.PutUint16(m[0x2060+ovrLoadSegField:], 0x216E)
	n.Rebuild(&o, m)
	if a := n.Addr(Address{0x216E, 0x15BD}); a.Segment != legacyNoMatch {
		t.Fatalf("ambiguous segment: %v", a)
	}
}
