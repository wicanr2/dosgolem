package buckrogers

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Spec 038 §5.1 synthetic cases.  Memory layout follows phase-281:
// DS=0EC0, [DS:3BCC]=3A81:0000 with the member count at +0x33E, the chain
// head at DS:4EC5, 259-byte records linked through +0xFF.

const testDS = 0x0EC0

type partyRec struct {
	seg, off uint16
	name     string
	gender   byte
	nameLen  int // overrides len(name) when non-zero (bad lengths)
}

func putFar(m fakeMem, a uint32, seg, off uint16) {
	binary.LittleEndian.PutUint16(m[a:], off)
	binary.LittleEndian.PutUint16(m[a+2:], seg)
}

// partyMem builds the chain of recs (all linked, last one 0000:0000) with
// the member count n.
func partyMem(n int, recs ...partyRec) fakeMem {
	m := make(fakeMem, 0x100000)
	putFar(m, linear(testDS, partyCountBase), 0x3A81, 0)
	m[linear(0x3A81, partyCountOffset)] = byte(n)
	if len(recs) > 0 {
		putFar(m, linear(testDS, partyHeadPtr), recs[0].seg, recs[0].off)
	}
	for i, r := range recs {
		base := linear(r.seg, r.off)
		l := len(r.name)
		if r.nameLen != 0 {
			l = r.nameLen
		}
		m[base] = byte(l)
		copy(m[base+1:], r.name)
		m[base+partyGenderOfs] = r.gender
		if i+1 < len(recs) {
			putFar(m, base+partyNextPtr, recs[i+1].seg, recs[i+1].off)
		}
	}
	return m
}

var (
	recFlavius = partyRec{seg: 0x5747, off: 0x0002, name: "FLAVIUS", gender: 0}
	recCeleste = partyRec{seg: 0x5757, off: 0x0005, name: "CELESTE", gender: 1}
	recBuck    = partyRec{seg: 0x5767, off: 0x0008, name: "BUCK", gender: 0}
	recMon1    = partyRec{seg: 0x5800, off: 0x0004, name: "TERRINE WARRIOR", gender: 0}
	recMon2    = partyRec{seg: 0x5810, off: 0x0007, name: "TERRINE WARRIOR", gender: 0}
)

func TestPartySnapshotRead(t *testing.T) {
	// Normal: three members, genders read from +0x26.
	s, err := ReadPartySnapshot(partyMem(3, recFlavius, recCeleste, partyRec{seg: 0x5777, off: 0, name: "ZED", gender: 2}), testDS)
	if err != nil || !reflect.DeepEqual(s.Names(), []string{"FLAVIUS/M", "CELESTE/F", "ZED/?"}) {
		t.Fatalf("normal: %v %v", s.Names(), err)
	}
	if m := s.MemberAt(0x5757, 0x0005); m == nil || m.Name != "CELESTE" || m.Gender != translit.Female {
		t.Fatalf("MemberAt: %+v", m)
	}
	// Combat: monsters follow the members on the same chain; only N taken.
	s, err = ReadPartySnapshot(partyMem(2, recFlavius, recCeleste, recMon1, recMon2), testDS)
	if err != nil || len(s.Members) != 2 || s.MemberAt(recMon1.seg, recMon1.off) != nil {
		t.Fatalf("combat chain: %v %v", s.Names(), err)
	}
	// N = 0: empty party (character creation).
	if s, err := ReadPartySnapshot(partyMem(0), testDS); err != nil || len(s.Members) != 0 {
		t.Fatalf("empty: %v %v", s, err)
	}
	for name, m := range map[string]fakeMem{
		"count above 6":      partyMem(7, recFlavius),
		"count beyond chain": partyMem(3, recFlavius, recCeleste), // third pointer is 0000:0000
		"head pointer 0":     func() fakeMem { m := partyMem(1, recFlavius); putFar(m, linear(testDS, partyHeadPtr), 0, 0); return m }(),
		"count pointer 0": func() fakeMem {
			m := partyMem(1, recFlavius)
			putFar(m, linear(testDS, partyCountBase), 0, 0)
			return m
		}(),
		"pointer out of range": func() fakeMem {
			m := partyMem(1, recFlavius)
			putFar(m, linear(testDS, partyHeadPtr), 0xA000, 0)
			return m
		}(),
		"name length 0":  partyMem(1, partyRec{seg: 0x5747, off: 2, name: "", nameLen: 0}),
		"name length 16": partyMem(1, partyRec{seg: 0x5747, off: 2, name: "ABCDEFGHIJKLMNOP"}),
		"non-printable":  partyMem(1, partyRec{seg: 0x5747, off: 2, name: "AB\x07C"}),
	} {
		if s, err := ReadPartySnapshot(m, testDS); err == nil {
			t.Errorf("%s accepted: %v", name, s.Names())
		}
	}
}

func TestPartyTrackerKeepsLastGood(t *testing.T) {
	var tr partyTracker
	// Before any accepted snapshot a rejected read is an empty party.
	if s := tr.refresh(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "A\x01"}), testDS); s == nil || len(s.Members) != 0 {
		t.Fatalf("initial reject: %+v", s)
	}
	good := tr.refresh(partyMem(2, recFlavius, recCeleste), testDS)
	if len(good.Members) != 2 {
		t.Fatal("good snapshot")
	}
	// A read whose DS is not the data segment (garbage chain) keeps the old one.
	if s := tr.refresh(partyMem(2, recFlavius, partyRec{seg: 0x5757, off: 5, name: "CEL\xFFSTE"}), testDS); !reflect.DeepEqual(s.Names(), good.Names()) {
		t.Fatalf("reject must reuse last: %v", s.Names())
	}
	if tr.Taken != 1 || tr.Rejected != 2 {
		t.Fatalf("counts %+v", tr)
	}
	// An accepted change (member removed) replaces it.
	if s := tr.refresh(partyMem(1, recCeleste), testDS); !reflect.DeepEqual(s.Names(), []string{"CELESTE/F"}) {
		t.Fatalf("removal: %v", s.Names())
	}
}

type fakeTranslit map[string]string

func (f fakeTranslit) Transliterate(name string, g translit.Gender) (string, translit.Tier, bool) {
	if g == translit.Female {
		if zh, ok := f[name+"/F"]; ok {
			return zh, translit.TierDict, true
		}
	}
	zh, ok := f[name]
	return zh, translit.TierDict, ok
}

func testPlayers(t *testing.T) *PlayerNames {
	return NewPlayerNames(fakeTranslit{
		"FLAVIUS": "弗拉維烏斯", "CELESTE": "塞萊斯特", "CELESTE/F": "塞萊絲特",
		"PORT": "波特", "HIM": "希姆", "TERRINE WARRIOR": "特林•沃里爾", "BUCK": "錯誤",
		"NICOLE STEELE": "妮可•斯蒂爾",
	}, testNames(t))
}

func TestPlayerNamesGlossaryFirst(t *testing.T) {
	p := testPlayers(t)
	if zh, ok := p.Chinese("BUCK", translit.Male); !ok || zh != "巴克" {
		t.Fatalf("BUCK: %q %v", zh, ok)
	}
	if zh, _ := p.Chinese("CELESTE", translit.Female); zh != "塞萊絲特" {
		t.Fatalf("gender: %q", zh)
	}
	if zh, _ := p.Chinese("CELESTE", translit.Male); zh != "塞萊斯特" {
		t.Fatalf("gender male: %q", zh)
	}
	if _, ok := p.Chinese("R2D2", translit.Male); ok {
		t.Fatal("transliteration failure must keep English")
	}
	if _, ok := p.Chinese("", translit.Male); ok {
		t.Fatal("empty name")
	}
}

// Real spec-037 data (the translit package's test copy of text/).
func TestPlayerNamesRealTransliterator(t *testing.T) {
	dir := filepath.Join("..", "..", "xlate", "translit", "testdata", "text")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no translit testdata")
	}
	tr, err := translit.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPlayerNames(tr, testNames(t))
	if zh, ok := p.Chinese("BUCK", translit.Male); !ok || zh != "巴克" {
		t.Fatalf("BUCK via glossary: %q", zh)
	}
	want, _, wok := tr.Transliterate("FLAVIUS", translit.Male)
	if zh, ok := p.Chinese("FLAVIUS", translit.Male); ok != wok || zh != want {
		t.Fatalf("FLAVIUS: %q want %q", zh, want)
	}
}

// --- dispatcher (combat right column, OVR:2BA60:235A) ---

func battleWatcher(t *testing.T) *EngineDispatchWatcher {
	w := NewEngineDispatchWatcher(engineFixture(t), map[CodeKey]bool{})
	w.SetNameCallers(map[CodeKey]bool{partyNameCaller: true})
	w.SetPlayerNames(testPlayers(t))
	return w
}

func nameArgs(r partyRec, row, col uint8) [6]uint16 {
	return [6]uint16{r.off, r.seg, 0, 5, uint16(row), uint16(col)}
}

// drawn returns the text drawn on row (trailing padding kept).
func drawn(w *EngineDispatchWatcher, row uint8) (string, bool) {
	for _, l := range w.Lines() {
		if l.Row == row {
			return string(l.Text), true
		}
	}
	return "", false
}

func battleEntry(w *EngineDispatchWatcher, rec partyRec, row, col uint8, s string, party *PartySnapshot) {
	ret := Address{0x37F1, 0x235A}
	w.ObserveEntryParty(partyNameCaller, 1, 0x100, ret, nameArgs(rec, row, col), []byte(s), party, nil)
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
}

func TestBattleColumnPlayerNames(t *testing.T) {
	party, err := ReadPartySnapshot(partyMem(3, recFlavius, recCeleste, recBuck, recMon1, recMon2), testDS)
	if err != nil {
		t.Fatal(err)
	}
	w := battleWatcher(t)
	battleEntry(w, recFlavius, 1, 23, "FLAVIUS", party)
	battleEntry(w, recCeleste, 10, 23, "CELESTE", party)
	battleEntry(w, recBuck, 12, 23, "BUCK", party)
	battleEntry(w, recMon1, 15, 23, "TERRINE WARRIOR", party)
	battleEntry(w, recMon2, 16, 23, "TERRINE WARRIOR", party)
	// Spec 039 §3.1: the Chinese is padded to 2×(original length) half units.
	pad := func(s string, n int) string { return string(padUnits([]rune(s), 2*n)) }
	for row, want := range map[uint8]string{
		1: pad("弗拉維烏斯", 7), 10: pad("塞萊絲特", 7), 12: pad("巴克", 4), // member: Chinese only, in the original cells
		15: pad("特林戰士", 15), 16: pad("特林戰士", 15), // monsters (same name twice): monster table
	} {
		if got, ok := drawn(w, row); !ok || got != want {
			t.Errorf("row %d: %q want %q", row, got, want)
		}
	}
	// Pointer not on the chain (e.g. a stack copy): the current monster lookup.
	w = battleWatcher(t)
	battleEntry(w, partyRec{seg: 0x3F00, off: 0x10}, 1, 23, "TERRINE WARRIOR", party)
	battleEntry(w, partyRec{seg: 0x3F00, off: 0x10}, 2, 23, "FLAVIUS", party)
	if got, _ := drawn(w, 1); got[:len("特林戰士")] != "特林戰士" {
		t.Errorf("off-chain monster: %q", got)
	}
	if _, ok := drawn(w, 2); ok {
		t.Error("off-chain player name drawn")
	}
	// Same pointer, different content (record space reused): not a hit.
	w = battleWatcher(t)
	battleEntry(w, recFlavius, 1, 23, "PIERRE", party)
	if _, ok := drawn(w, 1); ok {
		t.Error("record reuse drawn")
	}
	// A member pointer whose name is a monster name: never the monster table.
	named := partyRec{seg: 0x5747, off: 2, name: "TERRINE WARRIOR"}
	p2, _ := ReadPartySnapshot(partyMem(1, named), testDS)
	w = battleWatcher(t)
	battleEntry(w, named, 1, 23, "TERRINE WARRIOR", p2)
	if got, _ := drawn(w, 1); got != pad("特林•沃里爾", 15) {
		t.Errorf("member named like a monster at col 23: %q", got)
	}
	// Chinese longer than the name (spec 039 half units: • is one): English.
	w = battleWatcher(t)
	nicole := partyRec{seg: 0x5747, off: 2, name: "HIM"}
	p3, _ := ReadPartySnapshot(partyMem(1, nicole), testDS)
	battleEntry(w, nicole, 1, 23, "HIM", p3) // 希姆 = 4 ≤ 6 units: drawn
	if got, _ := drawn(w, 1); got != pad("希姆", 3) {
		t.Errorf("fits: %q", got)
	}
	short := partyRec{seg: 0x5747, off: 2, name: "PORT"}
	p4, _ := ReadPartySnapshot(partyMem(1, short), testDS)
	w = battleWatcher(t)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"PORT": "波特蘭德"}, nil))
	battleEntry(w, short, 1, 23, "PORT", p4) // 8 ≤ 8 units
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"PORT": "波特•蘭德"}, nil))
	battleEntry(w, short, 2, 23, "PORT", p4) // 9 > 8 units
	if _, ok := drawn(w, 1); !ok {
		t.Error("4 cells in 4 not drawn")
	}
	if _, ok := drawn(w, 2); ok {
		t.Error("5 cells in 4 drawn")
	}
	// Without the transliterator a member stays English (no monster lookup).
	w = battleWatcher(t)
	w.SetPlayerNames(nil)
	battleEntry(w, named, 1, 23, "TERRINE WARRIOR", p2)
	if _, ok := drawn(w, 1); ok {
		t.Error("member drawn without resolver")
	}
	// No snapshot (nil): the pre-038 behaviour.
	w = battleWatcher(t)
	battleEntry(w, named, 1, 23, "TERRINE WARRIOR", nil)
	if got, _ := drawn(w, 1); got[:len("特林戰士")] != "特林戰士" {
		t.Errorf("nil snapshot: %q", got)
	}
}

func TestBattleColumnCondition(t *testing.T) {
	party, _ := ReadPartySnapshot(partyMem(2, recFlavius, recBuck), testDS)
	// The same member string: col 23 draws; cols 1/8/17 on a row outside
	// the spec 038 §3.4 table (row 2 here) keep English.
	for _, col := range []uint8{23, 1, 8, 17} {
		w := battleWatcher(t)
		battleEntry(w, recBuck, 2, col, "BUCK", party)
		_, ok := drawn(w, 2)
		if ok != (col == 23) {
			t.Errorf("col %d drawn=%v", col, ok)
		}
	}
	// Col 17 with a player named like a monster: never the monster (§3.4
	// draws the member's own Chinese there).
	named := partyRec{seg: 0x5747, off: 2, name: "NEO WARRIOR"}
	p2, _ := ReadPartySnapshot(partyMem(1, named), testDS)
	w := battleWatcher(t)
	battleEntry(w, named, 5, 17, "NEO WARRIOR", p2)
	if got, ok := drawn(w, 5); ok && got[:len("NEO 戰士")] == "NEO 戰士" {
		t.Error("col 17 member drawn as monster")
	}
	// Col 17 with a monster record (not a member): monster lookup as before.
	mon := partyRec{seg: 0x5800, off: 4, name: "NEO WARRIOR"}
	p3, _ := ReadPartySnapshot(partyMem(1, recFlavius, mon), testDS)
	battleEntry(w, mon, 6, 17, "NEO WARRIOR", p3)
	if got, _ := drawn(w, 6); got[:len("NEO 戰士")] != "NEO 戰士" {
		t.Errorf("col 17 monster: %q", got)
	}
	// 21DE never draws in the combat column (not in the §3.4 table).
	w = battleWatcher(t)
	w.ObserveEntryParty(partySelectedCaller, 1, 0x100, Address{0x37F1, 0x21DE}, nameArgs(recBuck, 4, 23), []byte("BUCK"), party, nil)
	if _, ok := drawn(w, 4); ok {
		t.Error("21DE drew a player name at col 23")
	}
	if !w.NeedsParty(partySelectedCaller) || !w.NeedsParty(partyCursorCaller) || !battleWatcher(t).NeedsParty(partyNameCaller) {
		t.Error("NeedsParty")
	}
	if w.NeedsParty(CodeKey{Unit: 0x2BA60, Offset: 0x2391}) {
		t.Error("NeedsParty other caller")
	}
	// Empty string is never a player name.
	w = battleWatcher(t)
	battleEntry(w, recBuck, 4, 23, "", party)
	if len(w.Lines()) != 0 {
		t.Error("empty string drawn")
	}
}

// --- ECL 056C ---

func eclPlayerEntry(s string, clear bool, col, row uint8, ctx *EclPlayerContext) EclTextEntry {
	e := eclEntry(s, clear, col, row)
	e.Player = ctx
	return e
}

func eclCtx(party *PartySnapshot, caller CodeKey, typ uint8, addr uint16, sel partyRec) *EclPlayerContext {
	return &EclPlayerContext{Caller: caller, OpType: typ, OpAddr: addr, SelSeg: sel.seg, SelOff: sel.off, Party: party}
}

func eclPlayerWatcher(t *testing.T, pairs ...string) *EclTextWatcher {
	w := NewEclTextWatcher(eclFixture(t, append([]string{"UNUSED FIXTURE", "未用"}, pairs...)...))
	w.SetNames(testNames(t))
	w.SetPlayerNames(testPlayers(t))
	return w
}

// rowText joins the lines one row shows (continuations are separate lines).
func rowText(p *EclTextPage, row uint8) string {
	s := ""
	for _, l := range p.Lines {
		if l.Row == row {
			s += string(l.Text)
		}
	}
	return s
}

func lastLine(p *EclTextPage) string {
	if p == nil || len(p.Lines) == 0 {
		return ""
	}
	return string(p.Lines[len(p.Lines)-1].Text)
}

func TestEclPlayerNameOperandTypes(t *testing.T) {
	party, _ := ReadPartySnapshot(partyMem(3, recFlavius, recCeleste, recBuck), testDS)
	// 0x81@7BA4 equal to a member: 中文(英文); BUCK uses the 036 name.
	w := eclPlayerWatcher(t, "BUCK", "雄鹿", "PORT", "港口")
	w.ObserveEntry(eclPlayerEntry("BUCK", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if got := lastLine(w.Page()); got != "巴克(BUCK)" || w.Stats.PlayerNames != 1 {
		t.Fatalf("7BA4 BUCK: %q %+v", got, w.Stats)
	}
	// 0x81@7C00: the [DS:4EC1] record must be a member and match.
	w = eclPlayerWatcher(t)
	w.ObserveEntry(eclPlayerEntry("CELESTE", true, 1, 17, eclCtx(party, eclNameCallerB4A, 0x81, eclVarSelected, recCeleste)))
	if got := lastLine(w.Page()); got != "塞萊絲特(CELESTE)" {
		t.Fatalf("7C00: %q", got)
	}
	// 7C00 pointing to a monster record (not in the snapshot): unchanged.
	w = eclPlayerWatcher(t)
	w.ObserveEntry(eclPlayerEntry("CELESTE", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarSelected, recMon1)))
	if w.Page() != nil || w.Stats.PlayerNames != 0 {
		t.Fatal("7C00 on a monster record hit")
	}
	// 0x80 literal text shaped like a player name (PORT): the 027 catalog.
	portParty, _ := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "PORT"}), testDS)
	w = eclPlayerWatcher(t, "PORT", "港口")
	w.ObserveEntry(eclPlayerEntry("PORT", true, 1, 17, eclCtx(portParty, eclNameCallerB79, 0x80, 0, partyRec{})))
	if got := lastLine(w.Page()); got != "港口" || w.Stats.PlayerNames != 0 {
		t.Fatalf("0x80 PORT: %q", got)
	}
	// ... while the same PORT through the whitelisted variable is the player.
	w = eclPlayerWatcher(t, "PORT", "港口")
	w.ObserveEntry(eclPlayerEntry("PORT", true, 1, 17, eclCtx(portParty, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if got := lastLine(w.Page()); got != "波特(PORT)" {
		t.Fatalf("0x81 PORT: %q", got)
	}
	// 0x81 whose content is not a member: the catalog as before.
	w = eclPlayerWatcher(t, "PORT", "港口")
	w.ObserveEntry(eclPlayerEntry("PORT", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if got := lastLine(w.Page()); got != "港口" {
		t.Fatalf("0x81 not equal: %q", got)
	}
	// 0x81@7BB8 (pronouns) with a player named HIM: not a hit.
	himParty, _ := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: "HIM"}), testDS)
	w = eclPlayerWatcher(t, "HIM", "他")
	w.ObserveEntry(eclPlayerEntry("HIM", true, 1, 17, eclCtx(himParty, eclNameCallerB79, 0x81, 0x7BB8, partyRec{})))
	if got := lastLine(w.Page()); got != "他" {
		t.Fatalf("7BB8 HIM: %q", got)
	}
	// A non-ECL caller of 056C with a stale 0x81@7BA4 operand: not a hit.
	w = eclPlayerWatcher(t)
	w.ObserveEntry(eclPlayerEntry("BUCK", true, 1, 17, eclCtx(party, CodeKey{Unit: 0x27BBE, Offset: 0x0AE5}, 0x81, eclVarName, partyRec{})))
	if w.Page() != nil || w.Stats.PlayerNames != 0 {
		t.Fatal("engine caller hit")
	}
	// Empty string never.
	w = eclPlayerWatcher(t)
	w.ObserveEntry(eclPlayerEntry("", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if w.Stats.PlayerNames != 0 {
		t.Fatal("empty string hit")
	}
	// No resolver: the pre-038 behaviour.
	w = NewEclTextWatcher(eclFixture(t, "BUCK", "雄鹿"))
	w.ObserveEntry(eclPlayerEntry("BUCK", true, 1, 17, eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if got := lastLine(w.Page()); got != "雄鹿" {
		t.Fatalf("no resolver: %q", got)
	}
}

func TestEclPlayerNameContinuationAndFallback(t *testing.T) {
	noShrink(t) // spec 056 §5.8: this test asserts the step-down order of spec 036/038/045 without shrinking
	party, _ := ReadPartySnapshot(partyMem(2, recFlavius, recCeleste), testDS)
	ctx := eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{})
	finish := func(w *EclTextWatcher, e EclTextEntry) { w.ObserveInstruction(e.Return, e.SS, e.SP+eclTextReturnDelta) }

	// Single name (flag 1) then " ATTACKS." (flag 0): 塞萊絲特(CELESTE)發動攻擊。
	w := eclPlayerWatcher(t, " ATTACKS.", "發動攻擊。")
	e := eclPlayerEntry("CELESTE", true, 1, 17, ctx)
	w.ObserveEntry(e)
	finish(w, e)
	after := eclEntry(" ATTACKS.", false, 8, 17)
	after.Return = Address{0x2E13, 0x0B4A}
	w.ObserveEntry(after)
	p := w.Page()
	if p == nil || rowText(p, 17) != "塞萊絲特(CELESTE)發動攻擊。" {
		t.Fatalf("name + continuation: %+v", p)
	}
	// Name inside a sentence (flag 0 continuation after a Chinese page).
	w = eclPlayerWatcher(t, "YOU SEE ", "你看到")
	e = eclEntry("YOU SEE ", true, 1, 17)
	w.ObserveEntry(e)
	finish(w, e)
	w.ObserveEntry(eclPlayerEntry("FLAVIUS", false, 9, 17, ctx))
	if p := w.Page(); p == nil || rowText(p, 17) != "你看到弗拉維烏斯(FLAVIUS)" {
		t.Fatalf("in sentence: %+v", p)
	}

	// Three tiers in a narrow window (9 columns × 1 row = 18 half units,
	// spec 039).
	narrow := func(s string, clear bool, col uint8) EclTextEntry {
		e := eclPlayerEntry(s, clear, col, 17, ctx)
		e.Left, e.Right, e.Top, e.Bottom = 1, 9, 17, 17
		return e
	}
	// 弗拉維烏斯(FLAVIUS) = 19 units > 18 → 弗拉維烏斯 (10).
	w = eclPlayerWatcher(t)
	w.ObserveEntry(narrow("FLAVIUS", true, 1))
	if got := lastLine(w.Page()); got != "弗拉維烏斯" || w.Stats.PlayerNameChineseOnly != 1 || w.Stats.PlayerNames != 1 {
		t.Fatalf("chinese only: %q %+v", got, w.Stats)
	}
	// Continuation after 一二三四五六 (12 units) on a page: Chinese (10) does not
	// fit in the 6 units left either → the English passthrough (7, which does
	// not fit → overflow).
	w = eclPlayerWatcher(t, "ABCDEF", "一二三四五六")
	e = narrow("ABCDEF", true, 1)
	e.Player = nil
	w.ObserveEntry(e)
	finish(w, e)
	w.ObserveEntry(narrow("FLAVIUS", false, 7))
	if w.Stats.PlayerNameEnglish != 1 || w.Stats.Passthrough != 1 || w.Stats.Overflows != 1 || w.Page() != nil {
		t.Fatalf("english fallback overflow: %+v", w.Stats)
	}
	// English fits where Chinese does not (short Latin name, long Chinese):
	// after 一二 there are 14 units, 塞… needs 18, CELESTE 7.
	w = eclPlayerWatcher(t, "AB", "一二")
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "塞萊絲特塞萊絲特塞"}, nil))
	e = narrow("AB", true, 1)
	e.Player = nil
	w.ObserveEntry(e)
	finish(w, e)
	w.ObserveEntry(narrow("CELESTE", false, 3))
	p = w.Page()
	if w.Stats.PlayerNameEnglish != 1 || w.Stats.PlayerNames != 0 || p == nil || rowText(p, 17) != "一二CELESTE" ||
		p.Keys[len(p.Keys)-1] != "passthrough" {
		t.Fatalf("english fallback: %+v %+v", w.Stats, p)
	}
	// Fresh page, nothing fits: the original English shows (no page).
	w = eclPlayerWatcher(t)
	w.SetPlayerNames(NewPlayerNames(fakeTranslit{"CELESTE": "塞萊絲特塞萊絲特塞萊絲"}, nil)) // 22 units
	w.ObserveEntry(narrow("CELESTE", true, 1))
	if w.Page() != nil || w.Stats.PlayerNameEnglish != 1 {
		t.Fatalf("fresh english: %+v", w.Stats)
	}
	// Transliteration failure: English, never the catalog.
	w = eclPlayerWatcher(t, "R2D2", "機器人")
	r2 := partyRec{seg: 0x5747, off: 2, name: "R2D2"}
	p5, _ := ReadPartySnapshot(partyMem(1, r2), testDS)
	w.ObserveEntry(eclPlayerEntry("R2D2", true, 1, 17, eclCtx(p5, eclNameCallerB79, 0x81, eclVarName, partyRec{})))
	if w.Page() != nil {
		t.Fatal("untransliterable name drew catalog text")
	}
	// The 036 counters stay apart.
	if w.Stats.NameFirstOnly+w.Stats.NameUnannotated != 0 {
		t.Fatal("036 counters touched")
	}
}

func TestReadEclPlayerContext(t *testing.T) {
	m := partyMem(1, recFlavius)
	m[linear(testDS, eclOperandTypeAddr)] = 0x81
	m[linear(testDS, eclOperandHighAddr)] = 0x7C
	m[linear(testDS, eclOperandLowAddr)] = 0x00
	putFar(m, linear(testDS, partySelectedPtr), recFlavius.seg, recFlavius.off)
	party, _ := ReadPartySnapshot(m, testDS)
	c := readEclPlayerContext(m, testDS, eclNameCallerB79, party)
	if c.OpType != 0x81 || c.OpAddr != 0x7C00 || c.SelSeg != recFlavius.seg || c.SelOff != recFlavius.off {
		t.Fatalf("%+v", c)
	}
	if mem, ok := eclPlayerName(c, []byte("FLAVIUS")); !ok || mem.Gender != translit.Male {
		t.Fatal("7C00 from memory")
	}
}
