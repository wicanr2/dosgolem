package session

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// 規格 460 §5.5：session 單元測試（合成觀察器）。每條都以同一個觀察器型別在逐步模式（o.sparse 為假）跑一次當對照。

// spRec 是實作 SparseObserver 與 ModeObserver 的合成觀察器。
type spRec struct {
	ips    IPSet
	wakeAt uint64 // NextWake：wakeAt 大於最近一次觀察的步數時回 wakeAt
	zeroN  int    // NextWake 先回 0 這麼多次

	seen      []uint64 // 每次 BeforeStep 的 Steps()
	seenIP    []uint16
	acted     []uint64 // Steps() >= wakeAt 那一次（期限型工作，做一次）
	actedOnce bool
	last      uint64
	frameAt   []uint64 // FrameAt 的引數（只有稀疏模式）
	frameLast []uint64 // Frame 時最近一次 BeforeStep 的步數
	pendMode  bool
	afterMode []uint64

	vw       int
	panicVW  int
	keep     *StepView
	misuseVW bool
}

func (r *spRec) BeforeStep(v StepView) error {
	s := v.Steps()
	r.seen, r.seenIP, r.last = append(r.seen, s), append(r.seenIP, v.IP()), s
	if r.wakeAt != 0 && s >= r.wakeAt && !r.actedOnce {
		r.acted, r.actedOnce = append(r.acted, s), true
	}
	if r.pendMode {
		r.afterMode, r.pendMode = append(r.afterMode, s), false
	}
	if r.keep == nil {
		k := v
		r.keep = &k
	}
	return nil
}

func (r *spRec) VideoWrite(machine.VideoWrite) {
	r.vw++
	if r.panicVW != 0 && r.vw == r.panicVW {
		panic("sparse video boom")
	}
	if r.misuseVW && r.vw == 3 && r.keep != nil {
		_ = r.keep.IP()
	}
}

func (r *spRec) Frame([]byte, [256][3]uint8) { r.frameLast = append(r.frameLast, r.last) }
func (r *spRec) TriggerIPs() IPSet           { return r.ips }
func (r *spRec) FrameAt(steps uint64)        { r.frameAt = append(r.frameAt, steps); r.last = steps }
func (r *spRec) VideoModeChange(machine.ModeChange) {
	r.pendMode = true
}

func (r *spRec) NextWake() uint64 {
	if r.zeroN > 0 {
		r.zeroN--
		return 0
	}
	if r.wakeAt != 0 && r.wakeAt > r.last {
		return r.wakeAt
	}
	return NoWake
}

var (
	_ SparseObserver = (*spRec)(nil)
	_ ModeObserver   = (*spRec)(nil)
)

func sparseOwner(t *testing.T, code []byte, obs StepObserver, sparse bool) *Owner {
	t.Helper()
	o := startSyntheticOwner(t, code)
	o.observer, o.sparse = obs, sparse
	return o
}

// 觸發 IP：稀疏模式只在回合第一步與 IP 在集合內的步觀察。
func TestSparseTriggerIP(t *testing.T) {
	dense := &spRec{}
	d := sparseOwner(t, observerLoopCOM, dense, false)
	sp := &spRec{}
	sp.ips.Add(0x109)
	s := sparseOwner(t, observerLoopCOM, sp, true)
	for i := 0; i < 3; i++ {
		ra, errA := runTurn(t, d, 1000)
		rb, errB := runTurn(t, s, 1000)
		if errA != nil || errB != nil || ra.MachineStepsAfter != rb.MachineStepsAfter {
			t.Fatalf("回合 %d：%+v %v｜%+v %v", i, ra, errA, rb, errB)
		}
	}
	if uint64(len(dense.seen)) != d.machine.Steps {
		t.Fatalf("逐步模式 BeforeStep %d 次，%d 步", len(dense.seen), d.machine.Steps)
	}
	var want []uint64
	for i, st := range dense.seen {
		if st%1000 == 0 || dense.seenIP[i] == 0x109 { // 回合第一步（每回合 1000 步）
			want = append(want, st)
		}
	}
	if !reflect.DeepEqual(sp.seen, want) {
		t.Fatalf("稀疏模式觀察 %d 步，預期 %d 步", len(sp.seen), len(want))
	}
	if machineDigest(d) != machineDigest(s) {
		t.Fatal("兩種模式的機器狀態不同")
	}
	obs, forced := s.SparseStats()
	if obs != uint64(len(sp.seen)) || forced != 3 {
		t.Errorf("SparseStats=(%d,%d)，觀察 %d 次、Force 3 次", obs, forced, len(sp.seen))
	}
}

// 步數期限與 NextWake 回 0。
func TestSparseDeadlineAndZero(t *testing.T) {
	sp := &spRec{wakeAt: 37}
	s := sparseOwner(t, observerLoopCOM, sp, true)
	if _, err := runTurn(t, s, 100); err != nil {
		t.Fatal(err)
	}
	if want := []uint64{0, 37}; !reflect.DeepEqual(sp.seen, want) {
		t.Fatalf("期限：觀察 %v，要 %v", sp.seen, want)
	}
	z := &spRec{zeroN: 3}
	s = sparseOwner(t, observerLoopCOM, z, true)
	if _, err := runTurn(t, s, 100); err != nil {
		t.Fatal(err)
	}
	if want := []uint64{0, 1, 2, 3}; !reflect.DeepEqual(z.seen, want) {
		t.Fatalf("NextWake 0：觀察 %v，要 %v", z.seen, want)
	}
}

// Frame 之後的 Force 與 FrameAt 的步數。
func TestSparseFrameForceAndFrameAt(t *testing.T) {
	dense := &spRec{}
	d := sparseOwner(t, observerLoopCOM, dense, false)
	sp := &spRec{}
	s := sparseOwner(t, observerLoopCOM, sp, true)
	const n = 3*machine.DefaultVGAFrameEvery + 1000
	if _, err := runTurn(t, d, n); err != nil {
		t.Fatal(err)
	}
	if _, err := runTurn(t, s, n); err != nil {
		t.Fatal(err)
	}
	if len(sp.frameAt) < 3 || !reflect.DeepEqual(sp.frameAt, dense.frameLast) {
		t.Fatalf("FrameAt %v，逐步模式 Frame 時的步數 %v", sp.frameAt, dense.frameLast)
	}
	want := []uint64{0}
	for _, f := range sp.frameAt {
		want = append(want, f+1)
	}
	if !reflect.DeepEqual(sp.seen, want) {
		t.Fatalf("Frame 之後的觀察 %v，要 %v", sp.seen, want)
	}
}

// 模式設定之後的 Force：mov ax,0013h／int 10h／jmp $。
func TestSparseModeChangeForce(t *testing.T) {
	code := []byte{0xB8, 0x13, 0x00, 0xCD, 0x10, 0xEB, 0xFE}
	dense := &spRec{}
	d := sparseOwner(t, code, dense, false)
	sp := &spRec{}
	s := sparseOwner(t, code, sp, true)
	if _, err := runTurn(t, d, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := runTurn(t, s, 200); err != nil {
		t.Fatal(err)
	}
	if len(dense.afterMode) == 0 || !reflect.DeepEqual(sp.afterMode, dense.afterMode) {
		t.Fatalf("模式設定之後的觀察：稀疏 %v，逐步 %v", sp.afterMode, dense.afterMode)
	}
}

// SBDMA 之後的 Force：每筆事件之後第一次 BeforeStep 的步數與逐步模式相同。
type spSB struct {
	sbRec
	spRec
}

func (r *spSB) BeforeStep(v StepView) error     { r.spRec.BeforeStep(v); return r.sbRec.BeforeStep(v) }
func (r *spSB) VideoWrite(w machine.VideoWrite) {}
func (r *spSB) Frame(i []byte, p [256][3]uint8) { r.spRec.Frame(i, p) }

func TestSparseSBDMAForce(t *testing.T) {
	dense := &sbRec{}
	d := startSBOwner(t, true, dense)
	sp := &spSB{}
	s := startSBOwner(t, true, sp)
	s.sparse = true
	if _, err := runTurn(t, d, sbtBudget); err != nil {
		t.Fatal(err)
	}
	if _, err := runTurn(t, s, sbtBudget); err != nil {
		t.Fatal(err)
	}
	if len(dense.next) == 0 || !reflect.DeepEqual(sp.next, dense.next) || !reflect.DeepEqual(sp.readSHA, dense.readSHA) {
		t.Fatalf("SBDMA 之後：稀疏 next %d 筆，逐步 %d 筆，或內容不同", len(sp.next), len(dense.next))
	}
	if uint64(len(sp.spRec.seen)) >= s.machine.Steps/2 {
		t.Fatalf("稀疏模式觀察 %d 步／%d 步，不像稀疏", len(sp.spRec.seen), s.machine.Steps)
	}
}

// S14：程式結束之後第一步即停（Exited 條件）：mov cx,10／loop $／mov ax,4C00h／int 21h／jmp $。
func TestSparseExitedStopsAtSameStep(t *testing.T) {
	code := []byte{0xB9, 0x0A, 0x00, 0xE2, 0xFE, 0xB8, 0x00, 0x4C, 0xCD, 0x21, 0xEB, 0xFE}
	d := sparseOwner(t, code, &spRec{}, false)
	s := sparseOwner(t, code, &spRec{}, true)
	ra, errA := runTurn(t, d, 1000)
	rb, errB := runTurn(t, s, 1000)
	if errA != nil || errB != nil {
		t.Fatalf("%v %v", errA, errB)
	}
	if ra.Reason != StopReasonProgramStopped || rb.Reason != ra.Reason || rb.MachineStepsAfter != ra.MachineStepsAfter {
		t.Fatalf("逐步 %v 停在 %d；稀疏 %v 停在 %d", ra.Reason, ra.MachineStepsAfter, rb.Reason, rb.MachineStepsAfter)
	}
}

// S15：兩次 Advance 之間外部改變觀察器使期限變近，下一次期限觀察的步數與逐步模式相同。
func TestSparseTurnStartForce(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		r := &spRec{}
		o := sparseOwner(t, observerLoopCOM, r, sparse)
		if _, err := runTurn(t, o, 100); err != nil {
			t.Fatal(err)
		}
		r.wakeAt = o.machine.Steps + 5 // 回合之間由外部改變
		if _, err := runTurn(t, o, 100); err != nil {
			t.Fatal(err)
		}
		if want := []uint64{105}; !reflect.DeepEqual(r.acted, want) {
			t.Fatalf("sparse=%t：期限工作在 %v，要 %v", sparse, r.acted, want)
		}
	}
}

// S16、S17：VideoWrite 內 panic、保留的 StepView 在 VideoWrite 內使用：回報觀察器錯誤時的步數與逐步模式相同。
func TestSparsePanicAndMisuseReportStep(t *testing.T) {
	for _, c := range []struct {
		name string
		mk   func() *spRec
	}{
		{"panic", func() *spRec { return &spRec{panicVW: 3} }},
		{"misuse", func() *spRec { return &spRec{misuseVW: true} }},
	} {
		d := sparseOwner(t, observerLoopCOM, c.mk(), false)
		s := sparseOwner(t, observerLoopCOM, c.mk(), true)
		ra, errA := runTurn(t, d, 500)
		rb, errB := runTurn(t, s, 500)
		if errA == nil || errB == nil {
			t.Fatalf("%s：應回觀察器錯誤：%v｜%v", c.name, errA, errB)
		}
		if ra.MachineStepsAfter != rb.MachineStepsAfter || ra.Reason != rb.Reason {
			t.Fatalf("%s：逐步停在 %d（%v），稀疏停在 %d（%v）", c.name, ra.MachineStepsAfter, ra.Reason, rb.MachineStepsAfter, rb.Reason)
		}
	}
}

// 不實作 SparseObserver 或 SparseObserve 為假：照每步呼叫。
func TestSparseFallsBackToEveryStep(t *testing.T) {
	plain := &recordingObserver{}
	o := sparseOwner(t, observerLoopCOM, plain, true)
	if _, err := runTurn(t, o, 300); err != nil {
		t.Fatal(err)
	}
	if len(plain.ips) != 300 {
		t.Fatalf("不實作 SparseObserver：BeforeStep %d 次，要 300", len(plain.ips))
	}
	r := &spRec{}
	o = sparseOwner(t, observerLoopCOM, r, false)
	if _, err := runTurn(t, o, 300); err != nil {
		t.Fatal(err)
	}
	if len(r.seen) != 300 {
		t.Fatalf("SparseObserve 為假：BeforeStep %d 次，要 300", len(r.seen))
	}
	if obs, forced := o.SparseStats(); obs != 0 || forced != 0 {
		t.Fatalf("逐步模式 SparseStats=(%d,%d)", obs, forced)
	}
}

// ShouldObserve 與展開的運算式在隨機輸入下相同（460 §3.1 第 3 點的保險）；IPSet 的 Add／Has／Union。
func TestSparseShouldObserveRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(460))
	for i := 0; i < 200000; i++ {
		var f machine.StepFilter
		var set IPSet
		for k := 0; k < 4; k++ {
			ip := uint16(rng.Intn(65536))
			set.Add(ip)
			if !set.Has(ip) {
				t.Fatal("IPSet.Has")
			}
		}
		f.IPs = set
		f.Wake = rng.Uint64() >> uint(rng.Intn(64))
		f.Force = rng.Intn(8) == 0
		ex := rng.Intn(16) == 0
		if rng.Intn(2) == 0 {
			f.Exited = &ex
		}
		ip := uint16(rng.Intn(65536))
		if rng.Intn(4) == 0 {
			ip = uint16(rng.Intn(4))
			f.IPs[0] |= 1 << ip
		}
		steps := rng.Uint64() >> uint(rng.Intn(64))
		want := f.Force || steps >= f.Wake || f.IPs[ip>>6]&(1<<(ip&63)) != 0 || (f.Exited != nil && *f.Exited)
		if got := f.ShouldObserve(ip, steps); got != want {
			t.Fatalf("ShouldObserve(%#x, %d) = %t，展開式 %t", ip, steps, got, want)
		}
	}
	var a, b IPSet
	a.Add(1)
	b.Add(65535)
	a.Union(&b)
	if !a.Has(1) || !a.Has(65535) || a.Has(2) {
		t.Fatal("IPSet.Union")
	}
}
