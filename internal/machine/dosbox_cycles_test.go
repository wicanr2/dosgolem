package machine

import (
	"bytes"
	"math"
	"testing"
)

// `docs/spec/198`：DOSBox 相容的 cycles。

func TestSetDOSBoxCyclesSetsIRQ0Interval(t *testing.T) {
	m := New()
	m.PITDiv = 16571
	m.SetDOSBoxCycles(CyclesXT)
	if !m.CycleClock || !m.CPU.DOSBoxCost {
		t.Fatalf("CycleClock=%v DOSBoxCost=%v，都要開", m.CycleClock, m.CPU.DOSBoxCost)
	}
	if want := uint64(240000 * 16571 * 264 / 315_000_000); m.CycPerIRQ0() != want {
		t.Errorf("XT、分頻 16571 的 IRQ0 間隔 %d 個 cycle，要 %d", m.CycPerIRQ0(), want)
	}
	if m.DOSBoxCycles() != CyclesXT {
		t.Errorf("DOSBoxCycles() = %d，要 %d", m.DOSBoxCycles(), CyclesXT)
	}
}

func TestSetDOSBoxCyclesZeroRestores(t *testing.T) {
	m := New()
	m.SetDOSBoxCycles(CyclesAT8)
	if got := m.InstructionsPerSecond(); got != 750000 {
		t.Errorf("at8 的 InstructionsPerSecond %.0f，要 750000", got)
	}
	m.SetDOSBoxCycles(0)
	if m.CycleClock || m.CPU.DOSBoxCost || m.CPUHz != DefaultCPUHz || m.DOSBoxCycles() != 0 {
		t.Errorf("還原後 CycleClock=%v DOSBoxCost=%v CPUHz=%d", m.CycleClock, m.CPU.DOSBoxCost, m.CPUHz)
	}
	if got := m.InstructionsPerSecond(); math.Abs(got-StepsPerSecond()) > 1 {
		t.Errorf("還原後 %.0f，要等於 StepsPerSecond %.0f", got, StepsPerSecond())
	}
}

func TestDOSBoxCyclesSurvivesSnapshotAndState(t *testing.T) {
	m := New()
	m.SetDOSBoxCycles(CyclesAT8)
	snap := m.Snapshot()
	m.SetDOSBoxCycles(0)
	m.Restore(snap)
	if m.DOSBoxCycles() != CyclesAT8 {
		t.Errorf("快照還原後 %d，要 %d", m.DOSBoxCycles(), CyclesAT8)
	}

	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	n := New()
	if err := n.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if n.DOSBoxCycles() != CyclesAT8 {
		t.Errorf("狀態檔還原後 %d，要 %d", n.DOSBoxCycles(), CyclesAT8)
	}
}

// TestTypematicFollowsMachineSpeed：按住「2 秒份的指令數」，XT 速度與預設速度下重複次數相同。
func TestTypematicFollowsMachineSpeed(t *testing.T) {
	count := func(perMs uint64) int {
		m := armKeyboard(t)
		m.SetDOSBoxCycles(perMs)
		m.HoldKey(0x39, 0, uint64(2*m.InstructionsPerSecond()), true)
		return m.TimedKeysPending()
	}
	if a, b := count(CyclesXT), count(0); a != b {
		t.Errorf("按住 2 秒：XT 速度 %d 個事件、預設速度 %d 個，應相同", a, b)
	}
}
