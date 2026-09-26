package session

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// A000 寫入迴圈：mov ax,A000 / mov es,ax / xor di,di / mov al,5 / stosb / jmp stosb
var observerLoopCOM = []byte{0xB8, 0x00, 0xA0, 0x8E, 0xC0, 0x31, 0xFF, 0xB0, 0x05, 0xAA, 0xEB, 0xFD}

type recordingObserver struct {
	ips       []uint16
	writes    int
	failAt    int
	panicAt   int
	panicVW   int
	keep      *StepView
	readStale bool
}

func (r *recordingObserver) BeforeStep(v StepView) error {
	if r.readStale && r.keep != nil {
		_ = r.keep.IP()
	}
	r.ips = append(r.ips, v.IP())
	n := len(r.ips)
	if r.keep == nil {
		kept := v
		r.keep = &kept
	}
	if r.failAt == n {
		return errors.New("stop here")
	}
	if r.panicAt == n {
		panic("boom")
	}
	return nil
}

func (r *recordingObserver) VideoWrite(machine.VideoWrite) {
	r.writes++
	if r.panicVW == r.writes {
		panic("video boom")
	}
}

func (r *recordingObserver) Frame([]byte, [256][3]uint8) {}

func observedOwner(t *testing.T, obs StepObserver) *Owner {
	t.Helper()
	o := startSyntheticOwner(t, observerLoopCOM)
	o.observer = obs
	return o
}

func runTurn(t *testing.T, o *Owner, budget InstructionBudget) (TickReceipt, error) {
	t.Helper()
	if _, err := o.Deliver(CapturedUpdate{StartedPanel: ownerPanel(t, o), Layout: o.layout, SourceGeneration: o.generation}); err != nil {
		return TickReceipt{}, err
	}
	return o.Advance(budget)
}

func machineDigest(o *Owner) [32]byte {
	h := sha256.New()
	h.Write(o.machine.Mem[:])
	c := o.machine.CPU
	for _, v := range c.R {
		h.Write([]byte{byte(v), byte(v >> 8)})
	}
	for _, v := range c.Seg {
		h.Write([]byte{byte(v), byte(v >> 8)})
	}
	h.Write([]byte{byte(c.IP), byte(c.IP >> 8), byte(c.Flags), byte(c.Flags >> 8)})
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func TestObserverSeesEveryStepAndChangesNothing(t *testing.T) {
	obs := &recordingObserver{}
	withObs := observedOwner(t, obs)
	bare := startSyntheticOwner(t, observerLoopCOM)
	for i := 0; i < 3; i++ {
		a, err := runTurn(t, withObs, 20)
		if err != nil || a.Reason != StopReasonBudgetExhausted {
			t.Fatalf("observed turn: %+v %v", a, err)
		}
		b, err := runTurn(t, bare, 20)
		if err != nil || b.Steps != a.Steps {
			t.Fatalf("bare turn: %+v %v", b, err)
		}
	}
	if machineDigest(withObs) != machineDigest(bare) || withObs.machine.Steps != bare.machine.Steps {
		t.Fatal("觀測器改變了 machine 狀態")
	}
	if uint64(len(obs.ips)) != withObs.machine.Steps {
		t.Fatalf("BeforeStep %d 次，實際 %d 步", len(obs.ips), withObs.machine.Steps)
	}
	// 前四步是 mov/mov/xor/mov，之後在 stosb(0x109)／jmp(0x10A) 間循環。
	want := []uint16{0x100, 0x103, 0x105, 0x107, 0x109, 0x10A, 0x109}
	for i, ip := range want {
		if obs.ips[i] != ip {
			t.Fatalf("第 %d 步 IP=%#x，要 %#x", i, obs.ips[i], ip)
		}
	}
	stosb := 0
	for _, ip := range obs.ips {
		if ip == 0x109 {
			stosb++
		}
	}
	if obs.writes != stosb {
		t.Fatalf("VideoWrite %d 次，stosb %d 次", obs.writes, stosb)
	}
	// 回合外的 Step 不轉發寫入。
	before := obs.writes
	for i := 0; i < 4; i++ {
		if err := withObs.machine.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if obs.writes != before {
		t.Fatal("回合結束後仍轉發 VideoWrite")
	}
}

func assertObserverFault(t *testing.T, o *Owner, r TickReceipt, err error, wantSteps uint64) {
	t.Helper()
	if err == nil || r.Reason != StopReasonObserverFault || r.Steps != wantSteps {
		t.Fatalf("收據 %+v err=%v，要 ObserverFault、%d 步", r, err, wantSteps)
	}
	if o.Status().Phase != PhaseFailed || o.closer.(*countingCloser).calls != 1 {
		t.Fatalf("phase=%v close=%d", o.Status().Phase, o.closer.(*countingCloser).calls)
	}
	steps := o.machine.Steps
	if _, err := o.Advance(10); err == nil || o.machine.Steps != steps {
		t.Fatal("故障後仍前進")
	}
}

func TestObserverErrorStopsBeforeThatStep(t *testing.T) {
	o := observedOwner(t, &recordingObserver{failAt: 5})
	r, err := runTurn(t, o, 20)
	assertObserverFault(t, o, r, err, 4)
}

func TestObserverPanicIsRecovered(t *testing.T) {
	o := observedOwner(t, &recordingObserver{panicAt: 3})
	r, err := runTurn(t, o, 20)
	assertObserverFault(t, o, r, err, 2)
}

func TestVideoWritePanicStopsAfterThatStep(t *testing.T) {
	o := observedOwner(t, &recordingObserver{panicVW: 1})
	r, err := runTurn(t, o, 20)
	// 第 5 步 stosb 觸發 panic；該步完成，第 6 步前停止。
	assertObserverFault(t, o, r, err, 5)
}

func TestRetainedViewIsDetected(t *testing.T) {
	o := observedOwner(t, &recordingObserver{readStale: true})
	r, err := runTurn(t, o, 20)
	// 第 1 步保留視圖，第 2 步讀舊視圖 → 第 3 步前 ObserverFault。
	assertObserverFault(t, o, r, err, 2)
}

func TestNilObserverKeepsBarePath(t *testing.T) {
	o := startSyntheticOwner(t, observerLoopCOM)
	r, err := runTurn(t, o, 7)
	if err != nil || r.Reason != StopReasonBudgetExhausted || r.Steps != 7 {
		t.Fatalf("%+v %v", r, err)
	}
}

// mov ah,4Ch / int 21h：第二步就結束程式。之後不得再觀測或執行任何一步。
var observerExitCOM = []byte{0xB4, 0x4C, 0xCD, 0x21, 0x90, 0x90, 0x90, 0x90}

func TestObserverStopsAtProgramExit(t *testing.T) {
	obs := &recordingObserver{}
	o := startSyntheticOwner(t, observerExitCOM)
	o.observer = obs
	r, err := runTurn(t, o, 50)
	if err != nil || r.Reason != StopReasonProgramStopped || o.Status().Phase != PhaseStopped {
		t.Fatalf("收據 %+v err=%v phase=%v", r, err, o.Status().Phase)
	}
	if len(obs.ips) != 2 || r.Steps != 2 {
		t.Fatalf("結束後仍觀測或執行：BeforeStep %d 次、%d 步", len(obs.ips), r.Steps)
	}
}
