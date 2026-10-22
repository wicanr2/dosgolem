package machine

import (
	"bytes"
	"math"
	"testing"
)

func TestLiveClockScalesDevicePeriodsAndPreservesPhase(t *testing.T) {
	m := New()
	m.recalcIRQ0()
	m.Steps = 12345
	m.nextKey = m.Steps + 100
	before := append([]byte(nil), m.Mem...)
	oldRate := m.InstructionsPerSecond()
	oldEvery, oldNext := m.IRQ0Every, m.nextIRQ0
	oldVGA, oldVGANext := m.VGAFrameEvery, m.nextFrame
	oldKey, oldKeyNext := m.KeyEvery, m.nextKey
	for _, multiplier := range []int{2, 4, 8, 16, 32} {
		if err := m.SetInstructionClockPercent(multiplier * 100); err != nil {
			t.Fatal(err)
		}
		if math.Abs(m.InstructionsPerSecond()/oldRate-float64(multiplier)) > 0.001 {
			t.Fatal("CPU rate")
		}
		if math.Abs(float64(m.IRQ0Every)-float64(oldEvery)*float64(multiplier)) > float64(multiplier) || m.VGAFrameEvery != oldVGA*uint64(multiplier) || m.KeyEvery != oldKey*uint64(multiplier) {
			t.Fatal("device period ratio")
		}
		for _, v := range [][4]uint64{{m.nextIRQ0, oldNext, m.IRQ0Every, oldEvery}, {m.nextFrame, oldVGANext, m.VGAFrameEvery, oldVGA}, {m.nextKey, oldKeyNext, m.KeyEvery, oldKey}} {
			want := m.Steps + ((v[1]-m.Steps)*v[2]+v[3]/2)/v[3]
			if v[0] != want {
				t.Fatal("device phase reset")
			}
		}
		if !bytes.Equal(before, m.Mem) {
			t.Fatal("guest memory changed")
		}
		if err := m.SetInstructionClockPercent(100); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SetInstructionClockPercent(50); err != nil {
		t.Fatal("host capacity limit", err)
	}
	if err := m.SetInstructionClockPercent(100); err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(m.nextIRQ0)-float64(oldNext)) > 2 || math.Abs(float64(m.nextFrame)-float64(oldVGANext)) > 2 || math.Abs(float64(m.nextKey)-float64(oldKeyNext)) > 2 {
		t.Fatal("clock round trip")
	}
}

func TestLiveClockRejectsOtherTimingContracts(t *testing.T) {
	if New().SetInstructionClockPercent(1) != nil {
		t.Fatal("capacity floor rejected")
	}
	for _, p := range []int{0, -1, 3201} {
		if New().SetInstructionClockPercent(p) == nil {
			t.Fatal("invalid rate", p)
		}
	}
	m := New()
	m.CycleClock = true
	if m.SetInstructionClockPercent(200) == nil {
		t.Fatal("cycle clock accepted")
	}
	m = New()
	m.IRQ0Pinned = true
	if m.SetInstructionClockPercent(200) == nil {
		t.Fatal("pinned IRQ accepted")
	}
}

// 規格 449：播放中改時脈，SB 取樣在真實時間上的速率不變（每個取樣的指令數跟著倍率）。
func TestLiveClockKeepsSBSampleRate(t *testing.T) {
	m := armSB(t)
	startVoice(t, m)
	before := m.sb.threshold
	if err := m.SetInstructionClockPercent(400); err != nil {
		t.Fatal(err)
	}
	if m.sb.threshold != before*4 {
		t.Fatalf("400%% 時每個取樣的信用門檻 %d，該是 %d", m.sb.threshold, before*4)
	}
	if err := m.SetInstructionClockPercent(100); err != nil {
		t.Fatal(err)
	}
	if m.sb.threshold != before {
		t.Fatalf("回到 100%% 時門檻 %d，該是 %d", m.sb.threshold, before)
	}
}
