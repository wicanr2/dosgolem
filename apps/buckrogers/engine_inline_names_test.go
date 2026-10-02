package buckrogers

import (
	"encoding/binary"
	"strings"
	"testing"
)

// Buck repo spec 054 in the dispatcher (the sentence caller 0763:1282) and
// the snapshots the live runtime reads for it and for the ECL sentence callers.

func inlineDispatcher(t *testing.T, lang string, eng *EngineTextCatalog, tr fakeTranslit) *EngineDispatchWatcher {
	t.Helper()
	other := CodeKey{Unit: 0x27BBE, Offset: 0x051D}
	w := NewEngineDispatchWatcher(eng, map[CodeKey]bool{inlineNameCaller: true, other: true})
	w.SetPlayerNames(NewPlayerNames(tr, nil))
	return w
}

func inlineLine(w *EngineDispatchWatcher) string {
	if len(w.Lines()) == 0 {
		return "<none>"
	}
	return strings.TrimRight(string(w.Lines()[0].Text), " \u3000")
}

func dispatchSentence(w *EngineDispatchWatcher, caller CodeKey, s string, party *PartySnapshot) {
	w.ObserveEntryParty(caller, 0x1841, 0x3D00, Address{0x0763, 0x0100}, [6]uint16{0, 0, 0, 15, 12, 1}, []byte(s), party, nil)
}

func TestDispatchInlineNameKo(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	party := inlineParty(t, recFlavius, recRoarke)
	cases := []struct {
		s, with, without string
		inline           int
	}{
		{"FLAVIUS makes his tactics roll.", "플라비우스는 자신의 전술 판정을 한다.", "FLAVIUS은(는) 자신의 전술 판정을 한다.", 1},
		{"ROARKE is hit FOR 3 points of Damage.", "로앙은 공격을 받아 3 점의 피해.", "ROARKE은(는) 공격을 받아 3 점의 피해.", 1},
		{"MARCUS makes his tactics roll.", "MARCUS은(는) 자신의 전술 판정을 한다.", "MARCUS은(는) 자신의 전술 판정을 한다.", 0},
	}
	for _, c := range cases {
		w := inlineDispatcher(t, LangKo, eng, inlineReadings)
		dispatchSentence(w, inlineNameCaller, c.s, party)
		if got := inlineLine(w); got != c.with || w.Stats.InlineNames != c.inline || w.Stats.InlineNameFallback != 0 || w.Stats.Hits != 1 {
			t.Errorf("%q 有快照：%q %+v，應為 %q", c.s, got, w.Stats, c.with)
		}
		w = inlineDispatcher(t, LangKo, eng, inlineReadings)
		dispatchSentence(w, inlineNameCaller, c.s, nil)
		if got := inlineLine(w); got != c.without || w.Stats.InlineNames != 0 {
			t.Errorf("%q 無快照：%q %+v，應為 %q", c.s, got, w.Stats, c.without)
		}
		// Another allowed caller never looks the name up, snapshot or not.
		w = inlineDispatcher(t, LangKo, eng, inlineReadings)
		dispatchSentence(w, CodeKey{Unit: 0x27BBE, Offset: 0x051D}, c.s, party)
		if got := inlineLine(w); got != c.without || w.Stats.InlineNames != 0 {
			t.Errorf("%q 其他呼叫端：%q %+v，應為 %q", c.s, got, w.Stats, c.without)
		}
	}
}

func TestDispatchInlineNameJa(t *testing.T) {
	eng := inlineCatalog(t, LangJa, inlineJaFrags, map[string]string{"NEO WARRIOR": "NEO 戦士"}, map[string]string{"Bolt": "ボルト"})
	w := inlineDispatcher(t, LangJa, eng, fakeTranslit{"FLAVIUS": "フラビウス"})
	dispatchSentence(w, inlineNameCaller, "FLAVIUS makes his tactics roll.", inlineParty(t, recFlavius))
	if got := inlineLine(w); got != "フラビウスは自分の戦術判定を行う。" || w.Stats.InlineNames != 1 {
		t.Errorf("ja：%q %+v", got, w.Stats)
	}
}

// The reading is wider than the original cells (2 units each): the current
// text is drawn; when it does not fit either the old miss is recorded.
func TestDispatchInlineNameWidthFallback(t *testing.T) {
	eng := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	party := inlineParty(t, recAb)
	const sentence = "AB makes his tactics roll."
	w := inlineDispatcher(t, LangKo, eng, inlineReadings)
	dispatchSentence(w, inlineNameCaller, sentence, party)
	plain, _ := eng.Translate(sentence)
	if with, _, _ := eng.TranslateParty(sentence, party.NameFunc(NewPlayerNames(inlineReadings, nil))); stringUnits(with) <= 2*len(sentence) || stringUnits(plain) > 2*len(sentence) {
		t.Fatalf("fixture：讀音版 %d 單位應超過 %d，現行 %d 單位應放得下", stringUnits(with), 2*len(sentence), stringUnits(plain))
	}
	if got := inlineLine(w); got != plain || w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 1 || w.Stats.Hits != 1 || w.Stats.Misses != 0 {
		t.Errorf("退回現行輸出：%q %+v，應為 %q", got, w.Stats, plain)
	}
	// A translation that is wider than the cells with the English name too:
	// the reading is tried first, the current text second, then the miss of
	// before.
	long := map[string]string{}
	for k, v := range koFragments {
		long[k] = v
	}
	long[" tactics roll."] = "아주아주아주아주아주아주아주 긴 번역문입니다 정말로 매우 길어요"
	wide := inlineCatalog(t, LangKo, long, inlineKoMonsters, inlineKoItems)
	w = inlineDispatcher(t, LangKo, wide, inlineReadings)
	dispatchSentence(w, inlineNameCaller, sentence, party)
	if len(w.Lines()) != 0 || w.Stats.Hits != 0 || w.Stats.Misses != 1 || w.Stats.InlineNames != 0 || w.Stats.InlineNameFallback != 1 {
		t.Errorf("兩個版本都放不下：%q %+v", inlineLine(w), w.Stats)
	}
}

// zh watchers hold player names too; the snapshot changes nothing for them.
func TestDispatchInlineNameZhUnchanged(t *testing.T) {
	party := inlineParty(t, recFlavius, recRoarke)
	zhReadings := fakeTranslit{"FLAVIUS": "弗拉維烏斯", "ROARKE": "羅克"}
	for _, lang := range []string{LangZhTW, LangZhCN, LangTest, ""} {
		eng := engineFreezeCatalog(t, lang)
		for _, s := range []string{"FLAVIUS makes his tactics roll.", "ROARKE", "Bolt Gun", "(from behind) Hitting for 3 points of damage"} {
			a := inlineDispatcher(t, lang, eng, zhReadings)
			dispatchSentence(a, inlineNameCaller, s, party)
			b := inlineDispatcher(t, lang, eng, zhReadings)
			dispatchSentence(b, inlineNameCaller, s, nil)
			if inlineLine(a) != inlineLine(b) || a.Stats != b.Stats {
				t.Errorf("lang %q %q：%q %+v 對 %q %+v", lang, s, inlineLine(a), a.Stats, inlineLine(b), b.Stats)
			}
		}
	}
}

// --- what the live runtime reads ----------------------------------------------

// inlineView is a StepReader over synthetic memory that counts the reads of
// the party count pointer (every snapshot starts with it).
type inlineView struct {
	mem        fakeMem
	ss, sp, ds uint16
	partyReads *int
}

func (v inlineView) Steps() uint64 { return 1 }
func (v inlineView) CS() uint16    { return 0 }
func (v inlineView) IP() uint16    { return 0 }
func (v inlineView) SS() uint16    { return v.ss }
func (v inlineView) SP() uint16    { return v.sp }
func (v inlineView) DS() uint16    { return v.ds }
func (v inlineView) ES() uint16    { return 0 }
func (v inlineView) DI() uint16    { return 0 }
func (v inlineView) CX() uint16    { return 0 }
func (v inlineView) Read8(a uint32) uint8 {
	return v.mem[a]
}
func (v inlineView) Read16(a uint32) uint16 {
	if v.partyReads != nil && a == linear(v.ds, partyCountBase) {
		*v.partyReads++
	}
	return binary.LittleEndian.Uint16(v.mem[a:])
}
func (v inlineView) Palette() [256][3]uint8 { return [256][3]uint8{} }

// inlineRuntime is a runtime with one lane per language (zh-TW first), each
// with an ECL watcher, a dispatcher and the player names of the readings.
func inlineRuntime(t *testing.T, langs ...string) *LiveRuntime {
	t.Helper()
	r := &LiveRuntime{}
	for _, lang := range langs {
		var eng *EngineTextCatalog
		var tr fakeTranslit
		switch lang {
		case LangKo:
			eng, tr = inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems), inlineReadings
		default:
			eng, tr = engineFreezeCatalog(t, lang), fakeTranslit{"FLAVIUS": "弗拉維烏斯"}
		}
		players := NewPlayerNames(tr, nil)
		l := &liveLane{lang: lang, players: players}
		l.ecl = inlineEclWatcher(t, lang, LayoutFor(lang), eng)
		l.ecl.SetPlayerNames(players)
		l.engDisp = NewEngineDispatchWatcher(eng, map[CodeKey]bool{inlineNameCaller: true})
		l.engDisp.SetPlayerNames(players)
		r.lanes = append(r.lanes, l)
	}
	r.setInlineNames()
	return r
}

const (
	inlineSS, inlineSP = 0x1000, 0x0100
)

// inlineCall writes the 056C frame of a call into mem: the original string at
// 1100:0000, the return address ret, a window of one row.
func inlineCall(mem fakeMem, ret Address, s string) {
	frame := linear(inlineSS, inlineSP)
	put := func(off uint32, v uint16) { binary.LittleEndian.PutUint16(mem[frame+off:], v) }
	put(0, ret.Offset)
	put(2, ret.Segment)
	put(4, 0)
	put(6, 0x1100)
	put(8, 1)  // clear
	put(10, 0) // background
	put(12, 10)
	put(14, 17) // bottom
	put(16, 38) // right
	put(18, 17) // top
	put(20, 1)  // left
	base := linear(0x1100, 0)
	mem[base] = byte(len(s))
	copy(mem[base+1:], s)
}

func inlineMemory(party ...partyRec) fakeMem {
	mem := partyMem(len(party), party...)
	putStub(mem, 0x206, 0x27BBE, 0x37CE, 0x3000)
	putStub(mem, 0x300, 0x1CA15, 0x2000, 0x3100)
	putStub(mem, 0x320, 0x00904, 0x2000, 0x3200)
	return mem
}

func lanePage(r *LiveRuntime, lang string) string {
	for _, l := range r.lanes {
		if l.lang == lang {
			if p := l.ecl.Page(); p != nil {
				return eclRow(p, 17)
			}
			return "<none>"
		}
	}
	return "<no lane>"
}

func TestRuntimeEclSentenceCallersReadTheirOwnSnapshot(t *testing.T) {
	const sentence = "FLAVIUS makes his tactics roll."
	const withName = "플라비우스는 자신의 전술 판정을 한다."
	const english = "FLAVIUS은(는) 자신의 전술 판정을 한다."
	for _, tc := range []struct {
		caller Address
		want   string
		taken  int // spec 038 callers refresh the shared tracker
	}{
		{Address{0x3000, 0x0AE5}, withName, 0},
		{Address{0x3100, 0x1E42}, withName, 0},
		{Address{0x3200, 0x2813}, withName, 0},
		{Address{0x3200, 0x0B79}, english, 1},
		{Address{0x3200, 0x0B4A}, english, 1},
		{Address{0x3000, 0x0B79}, english, 0},
		{Address{0x4000, 0x0AE5}, english, 0},
	} {
		r := inlineRuntime(t, LangZhTW, LangKo)
		mem := inlineMemory(recFlavius, recRoarke)
		inlineCall(mem, tc.caller, sentence)
		v := inlineView{mem: mem, ss: inlineSS, sp: inlineSP, ds: testDS}
		r.observeEclText(v, eclTextPrinter)
		if got := lanePage(r, LangKo); got != tc.want {
			t.Errorf("%v：%q，應為 %q", tc.caller, got, tc.want)
		}
		if r.party.Taken != tc.taken || r.party.Rejected != 0 || (tc.taken == 0) != (r.party.last == nil) {
			t.Errorf("%v：共用追蹤 %+v（應取 %d 次）", tc.caller, r.party, tc.taken)
		}
		// The zh lane never sees the reading.
		if got := lanePage(r, LangZhTW); strings.Contains(got, "플") {
			t.Errorf("%v：zh-TW 的畫面含韓文：%q", tc.caller, got)
		}
	}
}

// A refused read (a count the chain cannot satisfy) gives no names and leaves
// the tracker alone.
func TestRuntimeEclSentenceCallerRefusedSnapshot(t *testing.T) {
	r := inlineRuntime(t, LangZhTW, LangKo)
	mem := inlineMemory(recFlavius)
	mem[linear(0x3A81, partyCountOffset)] = 3 // three members, a chain of one
	inlineCall(mem, Address{0x3000, 0x0AE5}, "FLAVIUS makes his tactics roll.")
	r.observeEclText(inlineView{mem: mem, ss: inlineSS, sp: inlineSP, ds: testDS}, eclTextPrinter)
	if got := lanePage(r, LangKo); got != "FLAVIUS은(는) 자신의 전술 판정을 한다." {
		t.Errorf("讀取被拒：%q", got)
	}
	if r.party.Taken != 0 || r.party.Rejected != 0 || r.party.last != nil {
		t.Errorf("共用追蹤被動到：%+v", r.party)
	}
}

// Without a Korean or Japanese lane the sentence callers do not read the
// party at all.
func TestRuntimeNoInlineSnapshotWithoutKoJaLane(t *testing.T) {
	r := inlineRuntime(t, LangZhTW, LangZhCN)
	if r.inlineNames {
		t.Fatal("inlineNames 不應成立")
	}
	reads := 0
	mem := inlineMemory(recFlavius)
	inlineCall(mem, Address{0x3000, 0x0AE5}, "FLAVIUS makes his tactics roll.")
	r.observeEclText(inlineView{mem: mem, ss: inlineSS, sp: inlineSP, ds: testDS, partyReads: &reads}, eclTextPrinter)
	if reads != 0 {
		t.Errorf("讀了隊伍 %d 次", reads)
	}
	v := inlineView{mem: mem, ss: inlineSS, sp: inlineSP, ds: testDS, partyReads: &reads}
	if p := r.dispatchParty(v, inlineNameCaller); p != nil || reads != 0 {
		t.Errorf("1282 不應讀隊伍：%v %d", p, reads)
	}
}

func TestRuntimeDispatchParty(t *testing.T) {
	r := inlineRuntime(t, LangZhTW, LangKo)
	r.lanes[0].engDisp.SetNameCallers(map[CodeKey]bool{partyNameCaller: true})
	mem := inlineMemory(recFlavius, recRoarke)
	reads := 0
	v := inlineView{mem: mem, ss: inlineSS, sp: inlineSP, ds: testDS, partyReads: &reads}

	// The sentence caller: its own snapshot, the tracker untouched.
	p := r.dispatchParty(v, inlineNameCaller)
	if p == nil || len(p.Members) != 2 || reads != 1 || r.party.Taken != 0 || r.party.last != nil {
		t.Errorf("1282：%v 讀取 %d 次 %+v", p, reads, r.party)
	}
	// The spec 038 name caller still refreshes the tracker.
	p = r.dispatchParty(v, partyNameCaller)
	if p == nil || r.party.Taken != 1 {
		t.Errorf("235A：%v %+v", p, r.party)
	}
	// Any other caller takes none.
	reads = 0
	if p := r.dispatchParty(v, CodeKey{Unit: 0x27BBE, Offset: 0x051D}); p != nil || reads != 0 {
		t.Errorf("其他呼叫端：%v 讀取 %d 次", p, reads)
	}
	// A refused read gives nil, not the tracker's last snapshot.
	bad := inlineMemory(recFlavius)
	bad[linear(0x3A81, partyCountOffset)] = 3
	if p := r.dispatchParty(inlineView{mem: bad, ss: inlineSS, sp: inlineSP, ds: testDS}, inlineNameCaller); p != nil {
		t.Errorf("讀取被拒應為 nil：%v", p)
	}
	if r.party.Taken != 1 || r.party.Rejected != 0 {
		t.Errorf("共用追蹤被動到：%+v", r.party)
	}
}
