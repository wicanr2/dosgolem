package machine

import (
	"errors"
	"fmt"
)

// 機器時脈比例（`docs/spec/242`）：宣告一顆較慢的虛擬 CPU。計時器、VGA 回掃、
// 鍵盤中斷的間隔按比例縮小，主機每秒跑得出的指令數不夠名目速度時，遊戲時間仍能
// 追上實際時間。p=100 時不改任何欄位。

// ScaleSteps 回 round(v × percent / 100)；v=0 回 0，v>0 時至少 1。
// 機器與前端共用這一條公式。
func ScaleSteps(v uint64, percent int) uint64 {
	if v == 0 {
		return 0
	}
	n := (v*uint64(percent) + 50) / 100
	if n < 1 {
		n = 1
	}
	return n
}

// SetClockPercent 設定時脈比例（10–100）。只能在第一道指令之前、且未開週期時鐘時呼叫。
func (m *Machine) SetClockPercent(p int) error {
	if p < 10 || p > 100 {
		return fmt.Errorf("machine: 時脈比例須在 10–100，得 %d", p)
	}
	if m.Steps != 0 {
		return errors.New("machine: 時脈比例只能在第一道指令之前設定")
	}
	if m.CycleClock {
		return errors.New("machine: 週期時鐘開啟時不能設定時脈比例")
	}
	m.clockPct = p
	if p == 100 {
		return nil
	}
	m.IRQ0Base = ScaleSteps(DefaultIRQ0Every, p)
	m.nextIRQ0 = ScaleSteps(m.nextIRQ0, p)
	m.recalcIRQ0()
	m.VGAFrameEvery = ScaleSteps(DefaultVGAFrameEvery, p)
	m.nextFrame = ScaleSteps(m.nextFrame, p)
	m.KeyEvery = ScaleSteps(DefaultKeyIRQEvery, p)
	m.nextKey = ScaleSteps(m.nextKey, p)
	return nil
}

// ClockPercent 回時脈比例；未設定為 100。
func (m *Machine) ClockPercent() int {
	if m.clockPct == 0 {
		return 100
	}
	return m.clockPct
}

// StepsPerSecondScaled 是縮放後的名目速度（指令／秒）。
func (m *Machine) StepsPerSecondScaled() float64 {
	return StepsPerSecond() * float64(m.ClockPercent()) / 100
}
