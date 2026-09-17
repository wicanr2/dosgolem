package machine

import "math"

// 機器速度：每秒指令數（`docs/spec/198`）。
//
// 指令數時鐘的預設速度（StepsPerSecond，約每秒 11.58 M 道）是對拍的標定，不是給人玩的速度。
// 進度綁在 CPU 上的遊戲，在預設速度下會快到不能玩；說明書寫 XT／AT 的遊戲要回到每秒幾十萬道。

// DOSBox-X「Emulate CPU speed」的機型預設（cycles ≈ 每毫秒指令數）。
const (
	IPSXT   = 240_000   // 8088 XT 4.77 MHz
	IPSAT8  = 750_000   // 286 8 MHz
	IPSAT12 = 1_510_000 // 286 12 MHz
)

// SetInstructionsPerSecond 設定機器每秒執行幾道指令，並重算 IRQ0 間隔；0 還原預設。
func (m *Machine) SetInstructionsPerSecond(ips uint64) {
	if ips == 0 {
		m.IRQ0Base = DefaultIRQ0Every
	} else {
		base := uint64(math.Round(float64(ips) * calibrationDivisor / PITBaseHz))
		if base == 0 {
			base = 1
		}
		m.IRQ0Base = base
	}
	m.recalcIRQ0()
}

// InstructionsPerSecond 回機器目前的速度（每秒指令數）。未設定時等於 StepsPerSecond()。
func (m *Machine) InstructionsPerSecond() float64 {
	base := m.IRQ0Base
	if base == 0 {
		base = DefaultIRQ0Every
	}
	return float64(base) * PITBaseHz / calibrationDivisor
}
