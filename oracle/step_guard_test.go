package oracle

import (
	"errors"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"reflect"
	"testing"
)

func guardMachine(t *testing.T) *Oracle {
	t.Helper()
	m := machine.New()
	// 合成 COM：nop; jmp 回 nop。沒有任何原版輸入。
	if err := m.LoadCOM([]byte{0x90, 0xEB, 0xFD}); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, t.TempDir())
	d.Install()
	t.Cleanup(d.Close)
	return &Oracle{m: m, d: d, onCall: map[uint32][]func(*Oracle){}}
}

func TestStepGuardStopsBeforeHooksStubAndClock(t *testing.T) {
	o := guardMachine(t)
	hooks, stubs := 0, 0
	o.OnCall(o.IP(), func(*Oracle) { hooks++ })
	o.Stub(o.IP(), func(*Oracle) uint32 { stubs++; return 0 })
	wait := &InputWaitError{Stopped: o.IP()}
	o.SetStepGuard(func(*Oracle) error { return wait })
	before := o.m.Snapshot()
	for i := 0; i < 3; i++ {
		err := o.Run(10)
		var w *InputWaitError
		if !errors.As(err, &w) || w != wait {
			t.Fatalf("Run=%v", err)
		}
		if !reflect.DeepEqual(before, o.m.Snapshot()) {
			t.Fatal("等待改變機器快照")
		}
	}
	if hooks != 0 || stubs != 0 {
		t.Fatalf("hooks=%d stubs=%d", hooks, stubs)
	}
	// 等待不能被誤認為預算耗盡或已跑完。
	if o.Steps() != 0 {
		t.Fatalf("steps=%d", o.Steps())
	}
}

func TestStepGuardConditionBudgetAndErrorOrder(t *testing.T) {
	o := guardMachine(t)
	calls := 0
	sentinel := errors.New("guard fault")
	o.SetStepGuard(func(*Oracle) error { calls++; return sentinel })
	if err := o.RunUntil(At(o.IP()), Budget(1)); err != nil || calls != 0 {
		t.Fatalf("At=%v calls=%d", err, calls)
	}
	var be *BudgetError
	if err := o.RunUntil(NewCond("never", func(*Oracle) bool { return false }), Budget(0)); !errors.As(err, &be) || calls != 0 {
		t.Fatalf("budget=%v calls=%d", err, calls)
	}
	if err := o.Run(1); err != sentinel || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
	o.d.Exited = true
	var ee *ExitError
	if err := o.Run(1); !errors.As(err, &ee) || calls != 1 {
		t.Fatalf("exit=%v calls=%d", err, calls)
	}
}

func TestStepGuardResumeReplacementAndRemoval(t *testing.T) {
	o := guardMachine(t)
	if err := o.Run(2); err != nil || o.Steps() != 2 {
		t.Fatalf("default=%v steps=%d", err, o.Steps())
	}
	blocked := true
	calls := 0
	o.SetStepGuard(func(o *Oracle) error {
		calls++
		if blocked {
			return &InputWaitError{Stopped: o.IP()}
		}
		return nil
	})
	if err := o.Run(1); err == nil {
		t.Fatal("未阻塞")
	}
	blocked = false
	if err := o.Run(2); err != nil || o.Steps() != 4 {
		t.Fatalf("resume=%v steps=%d", err, o.Steps())
	}
	o.SetStepGuard(func(*Oracle) error { return errors.New("replacement") })
	if err := o.Run(1); err == nil || err.Error() != "replacement" {
		t.Fatalf("replace=%v", err)
	}
	old := calls
	o.SetStepGuard(nil)
	if err := o.Run(2); err != nil || o.Steps() != 6 || calls != old {
		t.Fatalf("remove=%v steps=%d calls=%d", err, o.Steps(), calls)
	}
}
