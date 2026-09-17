package machine

// 機器速度：DOSBox 相容的 cycles（`docs/spec/198`）。
//
// 指令數時鐘的預設速度（StepsPerSecond，約每秒 11.58 M 道）是對拍的標定，不是給人玩的速度。
// 進度綁在 CPU 上的遊戲，在預設速度下會快到不能玩；說明書寫 XT／AT 的遊戲要回到 DOSBox-X 同機型的 cycles。
//
// ⚠ DOSBox 的 cycle 不是指令數：字串指令每迭代一次多扣一個。繪圖密集的程式 `rep movsb` 一次上千 byte，
// 照指令數設速度會比 DOSBox-X 快好幾倍，而且畫面每一格都對。

// DOSBox-X「Emulate CPU speed」的機型預設（每毫秒 cycles）。
const (
	CyclesXT   = 240  // 8088 XT 4.77 MHz
	CyclesAT8  = 750  // 286 8 MHz
	CyclesAT12 = 1510 // 286 12 MHz
)

// SetDOSBoxCycles 以 DOSBox 的扣法與速度跑：週期時鐘、DOSBox 計費、CPUHz ＝ perMs × 1000。
// perMs ＝ 0 還原指令數時鐘。
func (m *Machine) SetDOSBoxCycles(perMs uint64) {
	if perMs == 0 {
		m.CycleClock = false
		m.CPU.DOSBoxCost = false
		m.CPUHz = DefaultCPUHz
	} else {
		m.CycleClock = true
		m.CPU.DOSBoxCost = true
		m.CPUHz = perMs * 1000
	}
	m.recalcIRQ0()
}

// DOSBoxCycles 回目前設定的每毫秒 cycles；沒有開 DOSBox 計費時回 0。
func (m *Machine) DOSBoxCycles() uint64 {
	if !m.CycleClock || !m.CPU.DOSBoxCost {
		return 0
	}
	return m.CPUHz / 1000
}

// InstructionsPerSecond 回機器目前的速度（每秒指令數），給按鍵重複與音訊時間軸換算用。
// DOSBox 計費開著時回每秒 cycles（字串指令讓 cycles 多於指令數，所以這是上限近似）；
// 否則是指令數時鐘的速度，未設定時等於 StepsPerSecond()。
func (m *Machine) InstructionsPerSecond() float64 {
	if m.CycleClock && m.CPU.DOSBoxCost {
		return float64(m.CPUHz)
	}
	base := m.IRQ0Base
	if base == 0 {
		base = DefaultIRQ0Every
	}
	return float64(base) * PITBaseHz / calibrationDivisor
}
