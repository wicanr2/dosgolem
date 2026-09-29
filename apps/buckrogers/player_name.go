package buckrogers

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Spec 038 (Buck repo): player character names.  The party is read from the
// current party chain (phase-281) only at the two name-related hooks; the
// Chinese comes from the spec-036 glossary (exact English match) or the
// spec-037 transliterator.  Nothing here writes memory.

const (
	partyHeadPtr     = 0x4EC5 // DS: far pointer to the first record
	partySelectedPtr = 0x4EC1 // DS: far pointer to the selected / iterated record
	partyCountBase   = 0x3BCC // DS: far pointer; member count at +0x33E
	partyCountOffset = 0x033E
	partyRecordSize  = 0x103
	partyNextPtr     = 0xFF
	partyGenderOfs   = 0x26
	partyMaxMembers  = 6 // phase-281 only saw members 0–5
	partyNameMax     = 15
	// Records live on the DOS heap in conventional memory; a chain pointing
	// past it is not the party.
	partyMemLimit = 0xA0000
)

// PartyMember is one of the first N records of the party chain.
type PartyMember struct {
	Seg, Off uint16 // the far pointer the chain (and the dispatcher) uses
	Name     string
	Gender   translit.Gender
}

// PartySnapshot is the first N records of the chain, N = [DS:3BCC]+0x33E.
type PartySnapshot struct {
	Members []PartyMember
}

// MemberAt returns the member whose record starts at seg:off.
func (s *PartySnapshot) MemberAt(seg, off uint16) *PartyMember {
	if s == nil {
		return nil
	}
	for i := range s.Members {
		if s.Members[i].Seg == seg && s.Members[i].Off == off {
			return &s.Members[i]
		}
	}
	return nil
}

// FirstNamed returns the first member with exactly this name.
func (s *PartySnapshot) FirstNamed(name string) *PartyMember {
	if s == nil {
		return nil
	}
	for i := range s.Members {
		if s.Members[i].Name == name {
			return &s.Members[i]
		}
	}
	return nil
}

// Names lists the members as NAME/M|F|? (diagnostics, spec 038 §5.3).
func (s *PartySnapshot) Names() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.Members))
	for i, m := range s.Members {
		g := "?"
		switch m.Gender {
		case translit.Male:
			g = "M"
		case translit.Female:
			g = "F"
		}
		out[i] = m.Name + "/" + g
	}
	return out
}

func partyPtrOK(seg, off uint16, size uint32) bool {
	if seg == 0 && off == 0 {
		return false
	}
	return uint32(seg)<<4+uint32(off)+size <= partyMemLimit
}

// ReadPartySnapshot implements spec 038 §3.1 for the data segment ds.  It
// only reads.  Any bad pointer, a count above 6, or a name that is not 1–15
// printable ASCII bytes rejects the whole snapshot.
func ReadPartySnapshot(m MemReader, ds uint16) (*PartySnapshot, error) {
	cOff, cSeg := m.Read16(linear(ds, partyCountBase)), m.Read16(linear(ds, partyCountBase+2))
	if !partyPtrOK(cSeg, cOff, partyCountOffset+1) {
		return nil, fmt.Errorf("隊員數指標 %04X:%04X 無效", cSeg, cOff)
	}
	n := int(m.Read8(linear(cSeg, cOff+partyCountOffset)))
	if n > partyMaxMembers {
		return nil, fmt.Errorf("隊員數 %d 大於 %d", n, partyMaxMembers)
	}
	s := &PartySnapshot{}
	off, seg := m.Read16(linear(ds, partyHeadPtr)), m.Read16(linear(ds, partyHeadPtr+2))
	for i := 0; i < n; i++ {
		if !partyPtrOK(seg, off, partyRecordSize) {
			return nil, fmt.Errorf("第 %d 筆指標 %04X:%04X 無效", i, seg, off)
		}
		base := linear(seg, off)
		l := int(m.Read8(base))
		if l < 1 || l > partyNameMax {
			return nil, fmt.Errorf("第 %d 筆名字長度 %d", i, l)
		}
		b := make([]byte, l)
		for k := range b {
			c := m.Read8(base + 1 + uint32(k))
			if c < 0x20 || c > 0x7E {
				return nil, fmt.Errorf("第 %d 筆名字含非可列印字元", i)
			}
			b[k] = c
		}
		g := translit.GenderUnknown
		switch m.Read8(base + partyGenderOfs) {
		case 0:
			g = translit.Male
		case 1:
			g = translit.Female
		}
		s.Members = append(s.Members, PartyMember{Seg: seg, Off: off, Name: string(b), Gender: g})
		off, seg = m.Read16(base+partyNextPtr), m.Read16(base+partyNextPtr+2)
	}
	return s, nil
}

// partyTracker keeps the last accepted snapshot (spec 038 §3.1: a rejected
// read reuses it; before the first accepted read the party is empty).
type partyTracker struct {
	last     *PartySnapshot
	Taken    int
	Rejected int
	lastErr  string
}

func (t *partyTracker) refresh(m MemReader, ds uint16) *PartySnapshot {
	s, err := ReadPartySnapshot(m, ds)
	if err != nil {
		t.Rejected++
		t.lastErr = err.Error()
	} else {
		t.Taken++
		t.last = s
	}
	if t.last == nil {
		return &PartySnapshot{}
	}
	return t.last
}

func (t *partyTracker) summary() string {
	s := fmt.Sprintf("taken=%d rejected=%d names=[%s]", t.Taken, t.Rejected, strings.Join(t.last.Names(), ","))
	if t.lastErr != "" {
		s += " lastReject=" + t.lastErr
	}
	return s
}

// nameTransliterator is translit.Transliterator (an interface for tests).
type nameTransliterator interface {
	Transliterate(name string, gender translit.Gender) (string, translit.Tier, bool)
}

type playerNameKey struct {
	name   string
	gender translit.Gender
}

type playerNameResult struct {
	zh string
	ok bool
}

// PlayerNames turns a player name into Chinese: an exact spec-036 glossary
// `english` match wins (e.g. BUCK → 巴克), otherwise spec 037.  Results are
// cached by (name, gender).
type PlayerNames struct {
	tr    nameTransliterator
	names *NameGlossary
	cache map[playerNameKey]playerNameResult
}

func NewPlayerNames(tr nameTransliterator, names *NameGlossary) *PlayerNames {
	return &PlayerNames{tr: tr, names: names, cache: map[playerNameKey]playerNameResult{}}
}

// Chinese returns the display Chinese; ok=false keeps the English.
func (p *PlayerNames) Chinese(name string, g translit.Gender) (string, bool) {
	if p == nil || name == "" {
		return "", false
	}
	k := playerNameKey{name, g}
	if r, hit := p.cache[k]; hit {
		return r.zh, r.ok
	}
	var r playerNameResult
	if zh, ok := p.names.ChineseFor(name); ok {
		r = playerNameResult{zh, true}
	} else if p.tr != nil {
		zh, _, ok := p.tr.Transliterate(name, g)
		r = playerNameResult{zh, ok && zh != ""}
	}
	p.cache[k] = r
	return r.zh, r.ok
}

// Spec 038 §3.2: the ECL printer callers where the first operand can be a
// player name, and the dispatcher caller of the combat name column.
var (
	eclNameCallerB79 = CodeKey{Unit: 0x00904, Offset: 0x0B79}
	eclNameCallerB4A = CodeKey{Unit: 0x00904, Offset: 0x0B4A}
	partyNameCaller  = CodeKey{Unit: 0x2BA60, Offset: 0x235A}
)

const (
	eclOperandTypeAddr = 0x6D9A // DS: type of operand 1
	eclOperandHighAddr = 0x6DDA // DS: address of operand 1, high byte
	eclOperandLowAddr  = 0x6E1A // DS: address of operand 1, low byte
	eclOperandLiteral  = 0x80
	eclOperandVariable = 0x81
	eclVarSelected     = 0x7C00 // the record [DS:4EC1] points to
	eclVarName         = 0x7BA4 // a string variable seen holding names
	battleNameColumn   = 23
)

// EclPlayerContext is what LiveRuntime reads at a 056C entry for spec 038:
// the caller's CodeKey, operand 1 of the current ECL instruction, the
// [DS:4EC1] pointer and the party snapshot.
type EclPlayerContext struct {
	Caller         CodeKey
	OpType         uint8
	OpAddr         uint16
	SelSeg, SelOff uint16
	Party          *PartySnapshot
}

// eclPlayerName implements the ECL half of spec 038 §3.2.
func eclPlayerName(c *EclPlayerContext, original []byte) (*PartyMember, bool) {
	if c == nil || c.Party == nil || len(original) == 0 {
		return nil, false
	}
	if c.Caller != eclNameCallerB79 && c.Caller != eclNameCallerB4A {
		return nil, false
	}
	if c.OpType != eclOperandVariable {
		return nil, false // 0x80 literal text and other types are never names
	}
	s := string(original)
	switch c.OpAddr {
	case eclVarSelected:
		if m := c.Party.MemberAt(c.SelSeg, c.SelOff); m != nil && m.Name == s {
			return m, true
		}
	case eclVarName:
		if m := c.Party.FirstNamed(s); m != nil {
			return m, true
		}
	}
	return nil, false
}

// readEclPlayerContext reads the operand table at a 056C entry.
func readEclPlayerContext(m MemReader, ds uint16, caller CodeKey, party *PartySnapshot) *EclPlayerContext {
	return &EclPlayerContext{
		Caller: caller,
		OpType: m.Read8(linear(ds, eclOperandTypeAddr)),
		OpAddr: uint16(m.Read8(linear(ds, eclOperandHighAddr)))<<8 | uint16(m.Read8(linear(ds, eclOperandLowAddr))),
		SelOff: m.Read16(linear(ds, partySelectedPtr)),
		SelSeg: m.Read16(linear(ds, partySelectedPtr+2)),
		Party:  party,
	}
}
