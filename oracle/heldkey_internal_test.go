package oracle

import (
	"slices"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/449 §3.1 第 3、4 項的 oracle 部分：KeyDown／KeyUp 委派到 Machine 的按住狀態。
// 每段先 scheduleRepeats(n) 再 Run(n)，與 session 的 Advance 相同；不用 RunCycles。
// CyclesXT 下 delay = 120000、period = 22018。

const (
	heldDelay  = 120000
	heldPeriod = 22018
)

func heldRef() *machine.Machine {
	m := machine.New()
	m.SetDOSBoxCycles(machine.CyclesXT)
	return m
}

// runTo 以 cut 切段，從目前步數推進到 end。
func runTo(t *testing.T, o *Oracle, end uint64, cut func(pos, end uint64) uint64) {
	t.Helper()
	for o.m.Steps < end {
		n := min(cut(o.m.Steps, end), end-o.m.Steps)
		o.scheduleRepeats(n)
		if err := o.Run(n); err != nil {
			t.Fatal(err)
		}
	}
}

func heldOracle(t *testing.T) *Oracle {
	t.Helper()
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesXT)
	return o
}

func TestHeldKeyOracleMatchesHoldKeyExactly(t *testing.T) {
	const k = 0x39
	for _, d := range []uint64{480000, heldDelay + 16*heldPeriod} {
		for name, cut := range map[string]func(pos, end uint64) uint64{
			"Single":    func(pos, end uint64) uint64 { return end - pos },
			"Every7919": func(uint64, uint64) uint64 { return 7919 },
			"AtRepeats": func(pos, end uint64) uint64 {
				a := end - d
				next := a + heldDelay
				for next <= pos {
					next += heldPeriod
				}
				return next - pos
			},
		} {
			o := heldOracle(t)
			a := o.m.Steps
			o.KeyDown(k)
			runTo(t, o, a+d, cut)
			o.KeyUp(k)
			ref := heldRef()
			ref.HoldKey(k, a, d, true)
			if got, want := o.m.ScheduledKeys(), ref.ScheduledKeys(); !slices.Equal(got, want) {
				t.Errorf("d=%d %s：oracle 與 HoldKey 不同\n got  %v\n want %v", d, name, got, want)
			}
		}
	}
}

func TestHeldKeyOracleTapZeroMS(t *testing.T) {
	const k = 0x1C
	o := heldOracle(t)
	s := o.m.Steps
	if err := o.RunActions([]Action{{Kind: ActionTap, Scan: k, MS: 0}}, 0, nil); err != nil {
		t.Fatal(err)
	}
	want := []machine.ScheduledKey{{Step: s, KeyEvent: machine.KeyEvent{Scan: k}}, {Step: s, KeyEvent: machine.KeyEvent{Scan: k, Break: true}}}
	if got := o.m.ScheduledKeys(); !slices.Equal(got, want) {
		t.Fatalf("排程 %v，要 %v", got, want)
	}
}

func TestHeldKeyOracleSaveRestore(t *testing.T) {
	const k = 0x39
	every := func(uint64, uint64) uint64 { return 7919 }
	t.Run("RestoreBeforeDown", func(t *testing.T) {
		o := heldOracle(t)
		s := o.Save()
		o.KeyDown(k)
		o.Restore(s)
		if got := o.HeldKeys(); len(got) != 0 {
			t.Fatalf("HeldKeys = %v", got)
		}
	})
	t.Run("RestoreBeforeUp", func(t *testing.T) {
		const d = 480000
		o := heldOracle(t)
		a := o.m.Steps
		o.KeyDown(k)
		runTo(t, o, a+150000, every)
		s := o.Save()
		o.KeyUp(k)
		runTo(t, o, a+300000, every)
		o.Restore(s)
		if got := o.HeldKeys(); !slices.Equal(got, []uint8{k}) {
			t.Fatalf("HeldKeys = %v，要 [%02X]", got, k)
		}
		runTo(t, o, a+d, every)
		o.KeyUp(k)
		ref := heldRef()
		ref.HoldKey(k, a, d, true)
		if got, want := o.m.ScheduledKeys(), ref.ScheduledKeys(); !slices.Equal(got, want) {
			t.Fatalf("還原後放開與 HoldKey 不同\n got  %v\n want %v", got, want)
		}
	})
}

func TestHeldKeyOracleSpeedChangeKeepsDelayPeriod(t *testing.T) {
	const k, d = 0x39, 480000
	every := func(uint64, uint64) uint64 { return 7919 }
	o := heldOracle(t)
	a := o.m.Steps
	o.KeyDown(k)
	runTo(t, o, a+130000, every)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	runTo(t, o, a+d, every)
	o.KeyUp(k)
	ref := heldRef()
	ref.HoldKey(k, a, d, true)
	if got, want := o.m.ScheduledKeys(), ref.ScheduledKeys(); !slices.Equal(got, want) {
		t.Fatalf("改速度後的重複碼與原速度 HoldKey 不同\n got  %v\n want %v", got, want)
	}
}
