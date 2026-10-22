package machine

import "fmt"

// SetInstructionClockPercent changes the declared instruction-clock CPU rate
// between instructions. PIT, VGA and keyboard periods scale with that rate;
// remaining device phases are preserved to integer-instruction precision.
// This does not execute additional instructions or speed up the audio chip.
// Frontends must pace the declared rate and may cap it to host capacity.
// Cycle-clock and explicitly pinned IRQ configurations are intentionally refused.
func (m *Machine) SetInstructionClockPercent(percent int) error {
	if percent < 1 || percent > 3200 || m.CycleClock || m.IRQ0Pinned {
		return fmt.Errorf("machine: unsupported live instruction-clock setting %d", percent)
	}
	old := m.ClockPercent()
	if old == percent {
		return nil
	}
	base := m.IRQ0Base
	if base == 0 {
		base = DefaultIRQ0Every
	}
	if base != ScaleSteps(DefaultIRQ0Every, old) {
		return fmt.Errorf("machine: custom instruction-clock calibration")
	}
	irqEvery, irqNext := m.IRQ0Every, m.nextIRQ0
	vgaEvery, vgaNext := m.VGAFrameEvery, m.nextFrame
	keyEvery, keyNext := m.KeyEvery, m.nextKey
	m.clockPct = percent
	m.IRQ0Base = ScaleSteps(DefaultIRQ0Every, percent)
	m.recalcIRQ0()
	phase := func(next, before, after uint64) uint64 {
		if before == 0 || next <= m.Steps {
			return next
		}
		return m.Steps + ((next-m.Steps)*after+before/2)/before
	}
	m.nextIRQ0 = phase(irqNext, irqEvery, m.IRQ0Every)
	if vgaEvery != 0 {
		m.VGAFrameEvery = ScaleSteps(DefaultVGAFrameEvery, percent)
	}
	if keyEvery != 0 {
		m.KeyEvery = ScaleSteps(DefaultKeyIRQEvery, percent)
	}
	m.nextFrame = phase(vgaNext, vgaEvery, m.VGAFrameEvery)
	m.nextKey = phase(keyNext, keyEvery, m.KeyEvery)
	// 進行中的 SB 單次 DMA（`docs/spec/445-real-mode-sb-single-dma`）：每個取樣的指令數跟著時脈縮放，
	// 已累積的信用按同一比例換算，取樣率在真實時間上不變（`docs/spec/449-live-instruction-clock`）。
	if m.sb.active && old > 0 {
		m.sb.threshold = m.sb.threshold * uint64(percent) / uint64(old)
		m.sb.credit = m.sb.credit * uint64(percent) / uint64(old)
	}
	return nil
}
