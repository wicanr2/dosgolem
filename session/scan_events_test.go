package session

import (
	"slices"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/449 §3.1 的 session 單元測試：IRQ1 掃描碼送鍵通道。
//
// 合成程式是 `jmp $`，不裝 int 09h。KbdIntercept 關閉時事件留在 timedKeys 可以數。
// CyclesXT 下 InstructionsPerSecond 為 240000，delay = 120000、period = 22018。

var scanLoopCOM = []byte{0xEB, 0xFE}

const (
	scanDelay  = 120000
	scanPeriod = 22018
)

// scanOwner 建一個 Running 的合成 owner，開機前設成 CyclesXT。
func scanOwner(t *testing.T, kbd bool) *Owner {
	t.Helper()
	layout := sessionLayout(1, host.OutputScale2, false)
	o, err := New(Config{InitialScale: layout.Scale, InitialLayout: layout, KbdIntercept: kbd})
	if err != nil {
		t.Fatal(err)
	}
	o.closer = &countingCloser{}
	if err := o.machine.LoadCOM(scanLoopCOM); err != nil {
		t.Fatal(err)
	}
	o.machine.SetDOSBoxCycles(machine.CyclesXT)
	if got := o.machine.InstructionsPerSecond(); got != 240000 {
		t.Fatalf("InstructionsPerSecond = %v，要 240000", got)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	return o
}

// scanRef 是只呼叫 HoldKey 的參考機器。
func scanRef() *machine.Machine {
	m := machine.New()
	m.SetDOSBoxCycles(machine.CyclesXT)
	return m
}

func scanUpdate(t *testing.T, o *Owner, evs ...ScanEvent) CapturedUpdate {
	t.Helper()
	v := ownerView(t, o)
	return CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration, ScanEvents: evs}
}

func deliverScan(t *testing.T, o *Owner, evs ...ScanEvent) InputReceipt {
	t.Helper()
	r, err := o.Deliver(scanUpdate(t, o, evs...))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func advanceScan(t *testing.T, o *Owner, n InstructionBudget) TickReceipt {
	t.Helper()
	r, err := o.Advance(n)
	if err != nil || r.Steps != uint64(n) {
		t.Fatalf("Advance(%d) = (%+v, %v)", n, r, err)
	}
	return r
}

func down(s uint8) ScanEvent { return ScanEvent{Scan: s} }
func up(s uint8) ScanEvent   { return ScanEvent{Scan: s, Break: true} }

func maskOf(scans ...uint8) [2]uint64 {
	var m [2]uint64
	for _, s := range scans {
		m[s>>6] |= 1 << (s & 63)
	}
	return m
}

// 第 1 項：拒絕。
func TestScanEventsRejectedBeforeCommit(t *testing.T) {
	enter := host.PanelEvent{Kind: host.PanelEventOpen}
	cases := []struct {
		name  string
		build func(t *testing.T, o *Owner) CapturedUpdate
	}{
		{"WithBIOSKeys", func(t *testing.T, o *Owner) CapturedUpdate {
			u := scanUpdate(t, o, down(0x1E))
			u.BIOSKeys = []dos.Key{{Scan: 0x1C, ASCII: 0x0D}}
			return u
		}},
		{"Scan00", func(t *testing.T, o *Owner) CapturedUpdate { return scanUpdate(t, o, down(0x1E), down(0)) }},
		{"Scan80", func(t *testing.T, o *Owner) CapturedUpdate { return scanUpdate(t, o, up(0x80)) }},
		{"ScanE0", func(t *testing.T, o *Owner) CapturedUpdate { return scanUpdate(t, o, down(0xE0)) }},
		{"PausedScan00", func(t *testing.T, o *Owner) CapturedUpdate {
			u := scanUpdate(t, o, down(0))
			u.PanelEvents = []host.PanelEvent{enter}
			return u
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := scanOwner(t, false)
			u := tc.build(t, o)
			timed, bda := o.machine.TimedKeysPending(), o.dos.KeysPending()
			if _, err := o.Deliver(u); err == nil {
				t.Fatal("Deliver 沒有回錯誤")
			}
			if o.Status().Phase != PhaseFailed {
				t.Fatalf("Phase = %v，要 PhaseFailed", o.Status().Phase)
			}
			if o.machine.TimedKeysPending() != timed || o.dos.KeysPending() != bda || o.machine.HeldMask() != ([2]uint64{}) {
				t.Fatalf("拒絕後 timedKeys %d→%d、BDA %d→%d、held %v", timed, o.machine.TimedKeysPending(), bda, o.dos.KeysPending(), o.machine.HeldMask())
			}
		})
	}
}

// holdSession 在 Steps 0 按下 k、在 d 放開，cut 決定每回合的預算；pendingRef 非 nil 時
// 逐回合斷言 ScanPendingBefore（期望值取自參考機器，放開碼在放開之前不計）。
func holdSession(t *testing.T, k uint8, d uint64, cut func(pos, end uint64) uint64, pendingRef []machine.ScheduledKey) []machine.ScheduledKey {
	t.Helper()
	o := scanOwner(t, false)
	a := o.machine.Steps
	deliverScan(t, o, down(k))
	pos, end := a, a+d
	for pos < end {
		n := min(cut(pos, end), end-pos)
		r := advanceScan(t, o, InstructionBudget(n))
		if pendingRef != nil {
			want := 0
			for _, e := range pendingRef {
				if e.Step <= pos+n && !e.Break {
					want++
				}
			}
			if r.ScanPendingBefore != want {
				t.Fatalf("回合 [%d,%d] ScanPendingBefore = %d，要 %d", pos, pos+n, r.ScanPendingBefore, want)
			}
		}
		pos += n
		if pos < end {
			deliverScan(t, o)
		}
	}
	deliverScan(t, o, up(k))
	advanceScan(t, o, 1)
	return o.machine.ScheduledKeys()
}

// cutAtRepeats 讓每個切點都落在下一個重複碼的步數上（a = 0）。
func cutAtRepeats(pos, end uint64) uint64 {
	next := uint64(scanDelay)
	for next <= pos {
		next += scanPeriod
	}
	return next - pos
}

// 第 2 項：與 HoldKey 逐步等價。
func TestScanEventsHoldMatchesHoldKey(t *testing.T) {
	const k = 0x39
	for _, d := range []uint64{480000, scanDelay + 16*scanPeriod} {
		ref := scanRef()
		ref.HoldKey(k, 0, d, true)
		want := ref.ScheduledKeys()
		cuts := []struct {
			name string
			cut  func(pos, end uint64) uint64
		}{
			{"Single", func(pos, end uint64) uint64 { return end - pos }},
			{"Every7919", func(uint64, uint64) uint64 { return 7919 }},
			{"AtRepeats", cutAtRepeats},
		}
		for _, c := range cuts {
			var pendingRef []machine.ScheduledKey
			// 逐回合計數只在 d 不落在重複碼上時成立：d 落在重複碼上時，最後一回合 session 端
			// 已排入 end 那一步的重複碼（放開時才取消），參考機器沒有。
			if c.name == "AtRepeats" && d == 480000 {
				pendingRef = want
			}
			got := holdSession(t, k, d, c.cut, pendingRef)
			if !slices.Equal(got, want) {
				t.Errorf("d=%d %s：排程 %d 個，HoldKey %d 個\n got  %v\n want %v", d, c.name, len(got), len(want), got, want)
			}
		}
	}
}

// 第 3 項：同一批按下與放開。
func TestScanEventsTapSameBatch(t *testing.T) {
	const k = 0x1C
	o := scanOwner(t, false)
	deliverScan(t, o)
	advanceScan(t, o, 1000)
	s := o.machine.Steps
	r := deliverScan(t, o, down(k), up(k))
	if r.ScanEventsCommitted != 2 || r.HeldScans != ([2]uint64{}) {
		t.Fatalf("receipt %+v", r)
	}
	want := []machine.ScheduledKey{{Step: s, KeyEvent: machine.KeyEvent{Scan: k}}, {Step: s, KeyEvent: machine.KeyEvent{Scan: k, Break: true}}}
	if got := o.machine.ScheduledKeys(); !slices.Equal(got, want) {
		t.Fatalf("排程 %v，要 %v", got, want)
	}
}

// 第 5 項：面板、失焦與收據值。
func TestScanEventsPanelFocusAndReceipts(t *testing.T) {
	const k, m, j = 0x1E, 0x30, 0x48
	t.Run("DownRepeatAndUnheldUp", func(t *testing.T) {
		o := scanOwner(t, false)
		r := deliverScan(t, o, down(k))
		if r.ScanEventsCommitted != 1 || r.ScanReleasesForced != 0 || r.HeldScans != maskOf(k) {
			t.Fatalf("按下 receipt %+v", r)
		}
		advanceScan(t, o, 1000)
		before := o.machine.ScheduledKeys()
		r = deliverScan(t, o, down(k))
		if r.ScanEventsCommitted != 0 || !slices.Equal(o.machine.ScheduledKeys(), before) {
			t.Fatalf("重按 receipt %+v，排程 %v", r, o.machine.ScheduledKeys())
		}
		advanceScan(t, o, 1000)
		before = o.machine.ScheduledKeys()
		r = deliverScan(t, o, up(m))
		if r.ScanEventsCommitted != 0 || !slices.Equal(o.machine.ScheduledKeys(), before) || r.HeldScans != maskOf(k) {
			t.Fatalf("放開沒按著的鍵 receipt %+v，排程 %v", r, o.machine.ScheduledKeys())
		}
	})
	t.Run("PanelOpenForcesRelease", func(t *testing.T) {
		o := scanOwner(t, false)
		deliverScan(t, o, down(k))
		advanceScan(t, o, 1000)
		u := scanUpdate(t, o, down(j))
		u.PanelEvents = []host.PanelEvent{{Kind: host.PanelEventOpen}}
		r, err := o.Deliver(u)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Paused || r.ScanEventsCommitted != 0 || r.ScanReleasesForced != 1 || r.HeldScans != ([2]uint64{}) {
			t.Fatalf("開面板 receipt %+v", r)
		}
		ks := o.machine.ScheduledKeys()
		if last := ks[len(ks)-1]; last.Scan != k || !last.Break {
			t.Fatalf("最後一個事件 %+v，要 %02X 的放開碼", last, k)
		}
		for _, e := range ks {
			if e.Scan == j {
				t.Fatalf("paused 批次的按下碼被排進去：%+v", e)
			}
		}
		entry := o.machine.TimedKeysPending()
		tr, err := o.Advance(0)
		if err != nil || tr.Reason != StopReasonPanelPaused {
			t.Fatalf("paused Advance = (%+v, %v)", tr, err)
		}
		if tr.ScanPendingBefore != entry || tr.ScanPendingAfter != entry || tr.KeyIRQs != 0 || tr.KeyStalls != 0 ||
			o.machine.TimedKeysPending() != entry {
			t.Fatalf("paused 回合 %+v，進入時 %d、之後 %d", tr, entry, o.machine.TimedKeysPending())
		}
	})
	t.Run("FocusLostReleasesOnly", func(t *testing.T) {
		o := scanOwner(t, false)
		deliverScan(t, o, down(k), down(m))
		advanceScan(t, o, 1000)
		u := scanUpdate(t, o, down(j), up(k))
		u.FocusLost = true
		r, err := o.Deliver(u)
		if err != nil {
			t.Fatal(err)
		}
		if r.Paused || r.ScanEventsCommitted != 1 || r.ScanReleasesForced != 1 || r.HeldScans != ([2]uint64{}) {
			t.Fatalf("FocusLost receipt %+v", r)
		}
		var brk []uint8
		for _, e := range o.machine.ScheduledKeys() {
			if e.Scan == j {
				t.Fatalf("FocusLost 批次的按下碼被排進去：%+v", e)
			}
			if e.Break {
				brk = append(brk, e.Scan)
			}
		}
		if !slices.Equal(brk, []uint8{k, m}) {
			t.Fatalf("放開碼 %v，要 [%02X %02X]", brk, k, m)
		}
	})
	t.Run("PlanCopiesCallerSlice", func(t *testing.T) {
		// Deliver 在同一次呼叫內 commit，Deliver 回來後再改 slice 影響不到任何東西；
		// 所以在 package 內拆成 prepare → 改呼叫端 slice → commit 驗同一條規則。
		o := scanOwner(t, false)
		evs := []ScanEvent{down(k)}
		plan, err := o.prepare(scanUpdate(t, o, evs...))
		if err != nil {
			t.Fatal(err)
		}
		evs[0] = down(j)
		if _, err := o.commit(plan); err != nil {
			t.Fatal(err)
		}
		if got := o.machine.ScheduledKeys(); len(got) != 1 || got[0].Scan != k {
			t.Fatalf("排程 %v，要只有 %02X 的按下碼", got, k)
		}
	})
}

// 第 6 項：計數。
func TestScanEventsTickCounters(t *testing.T) {
	t.Run("InterceptOff", func(t *testing.T) {
		o := scanOwner(t, false)
		deliverScan(t, o, down(0x1E))
		r := advanceScan(t, o, 5000)
		if r.KeyStalls == 0 || r.KeyIRQs != 0 || r.ScanPendingAfter != r.ScanPendingBefore || r.ScanPendingBefore != 1 {
			t.Fatalf("receipt %+v", r)
		}
	})
	t.Run("InterceptOn", func(t *testing.T) {
		o := scanOwner(t, true)
		deliverScan(t, o, down(0x1E), up(0x1E))
		r := advanceScan(t, o, 50000)
		if r.KeyIRQs == 0 || r.KeyStalls != 0 || r.ScanPendingBefore != 2 ||
			r.KeyIRQs != uint64(r.ScanPendingBefore-r.ScanPendingAfter) {
			t.Fatalf("receipt %+v", r)
		}
	})
	t.Run("Paused", func(t *testing.T) {
		o := scanOwner(t, false)
		deliverScan(t, o, down(0x1E))
		advanceScan(t, o, 5000)
		u := scanUpdate(t, o)
		u.PanelEvents = []host.PanelEvent{{Kind: host.PanelEventOpen}}
		if _, err := o.Deliver(u); err != nil {
			t.Fatal(err)
		}
		entry := o.machine.TimedKeysPending()
		r, err := o.Advance(0)
		if err != nil || r.Reason != StopReasonPanelPaused || r.ScanPendingBefore != entry || r.ScanPendingAfter != entry || r.KeyIRQs != 0 || r.KeyStalls != 0 {
			t.Fatalf("paused = (%+v, %v)，進入時 %d", r, err, entry)
		}
	})
}

// 第 7 項：兩鍵次序。
func TestScanEventsTwoKeyOrder(t *testing.T) {
	const j, k = 0x48, 0x1E // j > k
	t.Run("SyncRepeatsByScan", func(t *testing.T) {
		o := scanOwner(t, false)
		deliverScan(t, o, down(j), down(k))
		for i := 0; i < 50; i++ {
			advanceScan(t, o, 7919)
			deliverScan(t, o)
		}
		advanceScan(t, o, 1)
		ks := o.machine.ScheduledKeys()
		pairs := 0
		for i := 1; i < len(ks); i++ {
			if ks[i].Step != ks[i-1].Step || ks[i].Step == 0 {
				continue
			}
			pairs++
			if ks[i-1].Scan != k || ks[i].Scan != j {
				t.Fatalf("第 %d 步重複碼次序 %02X、%02X，要 %02X 在 %02X 之前", ks[i].Step, ks[i-1].Scan, ks[i].Scan, k, j)
			}
		}
		if pairs == 0 {
			t.Fatal("沒有同步的重複碼")
		}
	})
	t.Run("RepeatOnBoundary", func(t *testing.T) {
		const b = scanDelay + 3*scanPeriod
		const e = b + 400000
		o := scanOwner(t, false)
		deliverScan(t, o, down(k))
		advanceScan(t, o, b)
		deliverScan(t, o, down(j))
		for pos := uint64(b); pos < e; {
			n := min(uint64(7919), e-pos)
			advanceScan(t, o, InstructionBudget(n))
			pos += n
			if pos < e {
				deliverScan(t, o)
			}
		}
		deliverScan(t, o, up(k), up(j))
		advanceScan(t, o, 1)
		ref := scanRef()
		ref.HoldKey(k, 0, e, true)
		ref.HoldKey(j, b, e-b, true)
		if got, want := o.machine.ScheduledKeys(), ref.ScheduledKeys(); !slices.Equal(got, want) {
			t.Fatalf("排程與 HoldKey 不同\n got  %v\n want %v", got, want)
		}
	})
}

// 第 9 項的補充：不給 ScanEvents 時收據的新欄位為零（680 步錨點本身由
// TestLegacyConfigZeroValueAnchor 檢查）。
func TestScanEventsZeroValueReceipts(t *testing.T) {
	o := startSyntheticOwner(t, audioLoopCOM)
	for i := 0; i < 4; i++ {
		in := deliverScan(t, o)
		if in.ScanEventsCommitted != 0 || in.ScanReleasesForced != 0 || in.HeldScans != ([2]uint64{}) {
			t.Fatalf("InputReceipt %+v", in)
		}
		r := advanceScan(t, o, 10*audioLoopSteps)
		if r.KeyIRQs != 0 || r.KeyStalls != 0 || r.ScanPendingBefore != 0 || r.ScanPendingAfter != 0 {
			t.Fatalf("TickReceipt %+v", r)
		}
	}
}
