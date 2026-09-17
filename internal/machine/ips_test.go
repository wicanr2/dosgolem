package machine

import (
	"math"
	"testing"
)

// `docs/spec/198`：每秒指令數。

func TestSetInstructionsPerSecondSetsIRQ0Interval(t *testing.T) {
	m := New()
	m.PITDiv = 16571
	m.IRQ0Every = DefaultIRQ0Every // 計時器開著
	m.SetInstructionsPerSecond(IPSXT)
	want := uint64(math.Round(float64(IPSXT) / (PITBaseHz / 16571)))
	if d := int64(m.IRQ0Every) - int64(want); d < -1 || d > 1 {
		t.Errorf("XT 速度、分頻 16571 的 IRQ0 間隔 %d，要 %d ±1", m.IRQ0Every, want)
	}
}

func TestInstructionsPerSecondRoundTrip(t *testing.T) {
	m := New()
	if got := m.InstructionsPerSecond(); math.Abs(got-StepsPerSecond()) > 1 {
		t.Errorf("未設定時 %.0f，要等於 StepsPerSecond %.0f", got, StepsPerSecond())
	}
	for _, ips := range []uint64{IPSXT, IPSAT8, IPSAT12} {
		m.SetInstructionsPerSecond(ips)
		if got := m.InstructionsPerSecond(); math.Abs(got-float64(ips))/float64(ips) > 0.001 {
			t.Errorf("設 %d 讀回 %.0f", ips, got)
		}
	}
	m.SetInstructionsPerSecond(0)
	if got := m.InstructionsPerSecond(); math.Abs(got-StepsPerSecond()) > 1 {
		t.Errorf("設 0 後讀回 %.0f，要還原預設", got)
	}
}

func TestInstructionsPerSecondSurvivesSnapshot(t *testing.T) {
	m := New()
	m.SetInstructionsPerSecond(IPSAT8)
	snap := m.Snapshot()
	m.SetInstructionsPerSecond(0)
	m.Restore(snap)
	if got := m.InstructionsPerSecond(); math.Abs(got-IPSAT8)/IPSAT8 > 0.001 {
		t.Errorf("還原後 %.0f，要 %d", got, IPSAT8)
	}
}

// TestTypematicFollowsMachineSpeed：按住「1 秒份的指令數」，XT 速度與預設速度下重複次數相同。
func TestTypematicFollowsMachineSpeed(t *testing.T) {
	count := func(ips uint64) int {
		m := armKeyboard(t)
		m.SetInstructionsPerSecond(ips)
		m.HoldKey(0x39, 0, uint64(2*m.InstructionsPerSecond()), true)
		return m.TimedKeysPending()
	}
	if a, b := count(IPSXT), count(0); a != b {
		t.Errorf("按住 2 秒：XT 速度 %d 個事件、預設速度 %d 個，應相同", a, b)
	}
}
