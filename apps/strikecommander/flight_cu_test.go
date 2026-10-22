package strikecommander

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// strike_commander 規格 012 §3.2.2 的抽出驗收：差分測試（第 5 項）與 cuTracker 單元測試（第 6 項）。
// 期望來源是 flight_cu_ref_test.go 的抽出前複本，不拿被測的 FlightObserver 自己的輸出當期望值。

// cuDual 是同一個合成世界餵給新舊兩個觀察器。
type cuDual struct {
	*world
	ref               *refFlightObserver
	commits, refComms []FlightCommit
}

func newCUDual(t *testing.T) *cuDual {
	w := newWorld(t, 0x0241)
	cats := flightTestCats()
	ref := newRefFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
	d := &cuDual{world: w, ref: ref}
	w.shadow = ref
	w.o.OnCommit = func(c FlightCommit) { d.commits = append(d.commits, c) }
	ref.OnCommit = func(c FlightCommit) { d.refComms = append(d.refComms, c) }
	return d
}

// hudAt 先清 band 再寫字，讓每次 puts 都有墨跡、進待提交。
func (d *cuDual) hudAt(s string, y int) {
	d.fillBand(y+1, y+5, 0)
	d.hud(s, 190, y)
}

// check 比較新舊觀察器全部可觀察的狀態。
func (d *cuDual) check(t *testing.T, where string) {
	t.Helper()
	o, r := d.o, d.ref
	diff := func(name string, a, b any) {
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s：%s 不同\n新 %#v\n舊 %#v", where, name, a, b)
		}
	}
	diff("Counts", o.Counts(), r.Counts())
	lo, ldo := o.Log()
	lr, ldr := r.Log()
	diff("Log", lo, lr)
	diff("Log 捨棄", ldo, ldr)
	diff("Visible", o.Visible(), r.Visible())
	diff("Pending", o.Pending(), r.Pending())
	diff("Candidates", o.Candidates(), r.Candidates())
	io, ao := o.CUState()
	ir, ar := r.CUState()
	diff("CUState", [2]bool{io, ao}, [2]bool{ir, ar})
	diff("VGAWindow", o.VGAWindow(), r.VGAWindow())
	diff("Generation", o.Generation(), r.Generation())
	bo, oko := o.Base()
	br, okr := r.Base()
	diff("Base", fmt.Sprint(bo, oko), fmt.Sprint(br, okr))
	diff("OnCommit", d.commits, d.refComms)
}

// TestFlightCUDiffScripted 是第 5 項：固定腳本，涵蓋重複入口、作廢（SS:SP 不同與超過 20 萬步）、逾時（Frame、入口、返回）、
// SS:SP 與 IP 不配對、CH ≠ 0（含進行中逾時後的 CH ≠ 0 入口）、MU CX = 0 與 ≠ 0、base 改變、停用、重驗與簽章層清空、jemm 不符。
// 測試名不以 TestFlightCU 開頭：規格 012 §3.2.2 第 4 項以 -run '^TestFlightCU' 列出的是那 12 個既有測試。
func TestCUTrackerDiffScripted(t *testing.T) {
	d := newCUDual(t)
	steps := []struct {
		name string
		f    func()
	}{
		{"GEAR", func() { d.hudAt("GEAR", 10) }},
		{"CU 入口", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"CU 期間 Frame", func() { d.frame() }},
		{"返回", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"升格", func() { d.frame() }},
		{"空 Frame", func() { d.frame() }},
		{"FLAPS", func() { d.hudAt("FLAPS", 15) }},
		{"入口 2", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"重複入口", func() { d.v.steps += 1000; d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"BRAKE", func() { d.hudAt("BRAKE", 20) }},
		{"作廢 SS:SP 不同", func() { d.cuIn(ipCUDiff, 1, tCUSP-0x20) }},
		{"作廢 25 萬步", func() { d.v.steps += 250_000; d.cuIn(ipCUDiff, 1, tCUSP-0x20) }},
		{"返回 SS:SP 不配對", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"返回 IP 不配對", func() { d.cuRet(ipCUPlainRet, tCUSP-0x20) }},
		{"返回", func() { d.cuRet(ipCUDiffRet, tCUSP-0x20) }},
		{"重複返回", func() { d.cuRet(ipCUDiffRet, tCUSP-0x20) }},
		{"升格", func() { d.frame() }},
		{"DGFT", func() { d.hudAt("DGFT", 25) }},
		{"入口 3", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"Frame 逾時", func() { d.v.steps += cuTimeoutSteps; d.frame() }},
		{"返回被忽略", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"入口 4", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"入口逾時後接受", func() { d.v.steps += cuTimeoutSteps; d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"返回逾時", func() { d.v.steps += cuTimeoutSteps; d.cuRet(ipCUDiffRet, tCUSP) }},
		{"入口 5", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"進行中超過逾時後 CH≠0 入口", func() { d.v.steps += cuTimeoutSteps; d.cuIn(ipCUPlain, 0x0200, 0xFE56) }},
		{"CH≠0 返回", func() { d.cuRet(ipCUPlainRet, 0xFE56) }},
		{"CH≠0 後 Frame", func() { d.frame() }},
		{"一般 CU 入口", func() { d.cuIn(ipCUPlain, 1, 0xFDD8) }},
		{"一般 CU 返回", func() { d.cuRet(ipCUPlainRet, 0xFDD8) }},
		{"差分返回被忽略", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"升格", func() { d.frame() }},
		{"STRF", func() { d.hudAt("STRF", 30) }},
		{"MU CX=0", func() { d.mu(0) }},
		{"VGA 期間 puts", func() { d.hudAt("SRM", 35) }},
		{"VGA 期間橫幅", func() { d.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5}) }},
		{"MU CX≠0", func() { d.mu(1) }},
		{"橫幅", func() { d.fillBand(2, 11, 0); d.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5}) }},
		{"入口 6", func() { d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"返回", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"返回後被取代", func() { d.hudAt("GEAR", 10); d.cuIn(ipCUDiff, 1, tCUSP) }},
		{"返回", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"升格", func() { d.frame() }},
		{"過期", func() {
			d.hudAt("NONE", 40)
			d.v.steps += flightStaleSteps
			d.hudAt("FLAPS", 15)
			d.cuIn(ipCUDiff, 1, tCUSP)
		}},
		{"jemm 不符", func() {
			d.v.mem[lin(d.base+segFJemm, 0x00C0)] ^= 0xFF
			d.cuRet(ipCUDiffRet, tCUSP)
			d.cuIn(ipCUDiff, 1, tCUSP-0x40)
			d.mu(0)
			d.v.mem[lin(d.base+segFJemm, 0x00C0)] ^= 0xFF
		}},
		{"返回", func() { d.cuRet(ipCUDiffRet, tCUSP) }},
		{"升格", func() { d.frame() }},
		{"base 改變", func() {
			d.hudAt("GEAR", 10)
			d.cuIn(ipCUDiff, 1, tCUSP)
			installFlightSigs(d.v, 0x0300)
			d.base = 0x0300
			d.hudAt("BRAKE", 20)
		}},
		{"新 base 的 CU", func() { d.cuIn(ipCUDiff, 1, tCUSP); d.cuRet(ipCUDiffRet, tCUSP); d.frame() }},
		{"base 停用", func() {
			d.cuIn(ipCUDiff, 1, tCUSP)
			installFlightSigs(d.v, 0x0400)
			d.v.mem[lin(0x0400+segFGlyph, 0x0184)] = 0x90
			d.base = 0x0400
			d.hudAt("BRAKE", 20)
		}},
		{"停用期間 CU", func() { d.cuIn(ipCUDiff, 1, tCUSP); d.cuRet(ipCUDiffRet, tCUSP); d.frame() }},
		{"重驗成功", func() {
			d.v.wsig(0x0400+segFGlyph, 0x0184, sigFGlyph, 0)
			d.v.steps += flightReverifySteps
			d.hudAt("BRAKE", 20)
			d.cuIn(ipCUDiff, 1, tCUSP)
			d.cuRet(ipCUDiffRet, tCUSP)
			d.frame()
		}},
		{"簽章層清空", func() {
			d.cuIn(ipCUDiff, 1, tCUSP)
			d.v.mem[lin(d.base+segFText, ipFPuts)] = 0x90
			d.v.steps += SigCheckSteps
			d.at(0x1234, 1)
			d.cuRet(ipCUDiffRet, tCUSP)
			d.frame()
		}},
	}
	for i, s := range steps {
		s.f()
		d.check(t, fmt.Sprintf("第 %d 步 %s", i, s.name))
	}
	// 腳本要真的走過各分支（只檢查新舊相同不足以證明有覆蓋）。
	c := d.ref.Counts()
	for name, n := range map[string]int{"EntriesDuplicate": c.EntriesDuplicate, "EntriesVoided": c.EntriesVoided,
		"Timeouts": c.Timeouts, "ReturnsSSMismatch": c.ReturnsSSMismatch, "CUCHNonZero": c.CUCHNonZero,
		"ReturnsIgnored": c.ReturnsIgnored, "CandidatesReplacedAwaiting": c.CandidatesReplacedAwaiting,
		"CUFrames": c.CUFrames, "SkipStale": c.SkipStale, "SkipVGA": c.SkipVGA, "Promotions": c.Promotions} {
		if n == 0 {
			t.Errorf("腳本沒有走到 %s", name)
		}
	}
	if c.Timeouts < 3 || c.EntriesVoided < 2 || c.ReturnsSSMismatch < 2 {
		t.Errorf("逾時 %d（要 ≥3）、作廢 %d（要 ≥2）、不配對 %d（要 ≥2）", c.Timeouts, c.EntriesVoided, c.ReturnsSSMismatch)
	}
}

// TestCUTrackerDiffRandom 是第 5 項的補充：固定種子的隨機序列，每個動作後比一次。
func TestCUTrackerDiffRandom(t *testing.T) {
	hudKeys := []string{"GEAR", "FLAPS", "BRAKE", "DGFT", "STRF", "SRM", "NONE"}
	sps := []uint16{tCUSP, tCUSP - 0x20, 0xFE56}
	jumps := []uint64{0, 1000, 150_000, 250_000, cuTimeoutSteps, flightStaleSteps}
	for seed := int64(1); seed <= 8; seed++ {
		d := newCUDual(t)
		rng := rand.New(rand.NewSource(seed))
		for i := 0; i < 3000; i++ {
			op := rng.Intn(14)
			switch op {
			case 0, 1:
				k := rng.Intn(len(hudKeys))
				d.hudAt(hudKeys[k], 10+5*k)
			case 2:
				d.fillBand(2, 11, 0)
				d.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5})
			case 3, 4:
				ip := []uint16{ipCUDiff, ipCUPlain}[rng.Intn(2)]
				cx := []uint16{1, 1, 1, 0x0200}[rng.Intn(4)]
				d.cuIn(ip, cx, sps[rng.Intn(len(sps))])
			case 5, 6:
				ip := []uint16{ipCUDiffRet, ipCUPlainRet}[rng.Intn(2)]
				d.cuRet(ip, sps[rng.Intn(len(sps))])
			case 7:
				d.mu(uint16(rng.Intn(2)))
			case 8, 9:
				d.frame()
			case 10:
				d.v.steps += jumps[rng.Intn(len(jumps))]
			case 11:
				if rng.Intn(20) == 0 {
					d.v.mem[lin(d.base+segFJemm, 0x00C0)] ^= 0xFF
				}
			case 12:
				if rng.Intn(40) == 0 {
					nb := []uint16{0x0241, 0x0300}[rng.Intn(2)]
					installFlightSigs(d.v, nb)
					d.base = nb
				}
			case 13:
				if rng.Intn(40) == 0 {
					d.v.mem[lin(d.base+segFText, ipFPuts)] ^= 0xFF
				}
			}
			d.check(t, fmt.Sprintf("種子 %d 第 %d 步（動作 %d）", seed, i, op))
		}
		c := d.ref.Counts()
		t.Logf("種子 %d：升格 %d、重複 %d、作廢 %d、逾時 %d、不配對 %d、CH≠0 %d、CU 期間回掃 %d、返回後被取代 %d",
			seed, c.Promotions, c.EntriesDuplicate, c.EntriesVoided, c.Timeouts, c.ReturnsSSMismatch, c.CUCHNonZero, c.CUFrames, c.CandidatesReplacedAwaiting)
	}
}

// TestCUTrackerDupThenReturn 是第 6 項（分期 (d) 的入口條件）：直接對 cuTracker.Observe 餵
// 「CU 入口 00D6（SS:SP = S）→ 同一入口、同一 SS:SP 再命中 → 返回 00D8（SS:SP = S）」，再呼叫 Frame。
func TestCUTrackerDupThenReturn(t *testing.T) {
	w := newWorld(t, 0x0241)
	v := w.v
	var tr cuTracker
	tr.SetBase(w.base, true)
	dup, acc, ret := 0, 0, 0
	h := &cuHooks{
		OnDuplicate:     func(FlightView, uint16, cuEntry) { dup++ },
		OnEntryAccepted: func(FlightView, uint64, uint16) { acc++ },
		OnReturn: func(_ FlightView, _ uint16, _ cuEntry, out cuReturnOutcome) {
			if out == cuReturnAccepted {
				ret++
			}
		},
	}
	feed := func(ip uint16) {
		v.steps += 100
		v.cs, v.ip = w.base+segFJemm, ip
		tr.Observe(v, h)
	}
	v.cx, v.ss, v.sp = 0x0001, tSS, tCUSP
	feed(ipCUDiff)
	first := v.steps
	feed(ipCUDiff)
	feed(ipCUDiffRet)
	promotes := 0
	var key uint64
	for i := 0; i < 3; i++ {
		v.steps += 100
		if p, k := tr.Frame(v.steps, h); p {
			promotes++
			key = k
		}
	}
	if promotes != 1 || key != first || dup != 1 || acc != 1 || ret != 1 {
		t.Fatalf("升格 %d 次（要 1）、鑰匙 %d（要第一次入口 %d）、OnDuplicate %d（要 1）、接受入口 %d、接受返回 %d",
			promotes, key, first, dup, acc, ret)
	}
}
