package machine

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func (p *LEOPLPorts) startDSPDMA(n uint32) bool { return p.startDMA(n, false) }

func (p *LEOPLPorts) startSB16AutoDMA(samples uint32, mode byte) bool {
	if mode&^0x30 != 0 || p.dsp.RateDenominator == 0 {
		return false
	}
	denominator := p.dsp.RateDenominator
	if mode&0x20 != 0 && p.dsp.TimeConstantKnown {
		denominator *= 2
	}
	if p.dsp.RateNumerator < 5000*denominator || p.dsp.RateNumerator > 45000*denominator || !p.startDMA(samples, true) {
		return false
	}
	p.dmaStereo, p.dmaSigned, p.dmaFIFO = mode&0x20 != 0, mode&0x10 != 0, true
	return true
}

func (p *LEOPLPorts) startDSP16DMA(words uint32) bool {
	d := p.secondaryDMA
	if words != 1 || p.dmaActive || p.dma16Active || p.dsp.RateNumerator == 0 || p.dsp.RateDenominator == 0 ||
		d.Mask&2 != 0 || d.Mode[1] != 0x48 || d.Known[2] != 3 || d.Known[3] != 3 ||
		!d.PageKnown[1] || d.Page[1] != 0 || words > uint32(d.Current[3])+1 {
		return false
	}
	p.dma16Active = true
	p.dma16WordsLeft = words
	p.sample16Credit = 0
	return true
}

func (p *LEOPLPorts) startDMA(n uint32, auto bool) bool {
	d := p.dma
	mode := byte(0x48)
	if auto {
		mode = 0x58
	}
	if p.dmaActive || p.dma16Active || p.dsp.RateNumerator == 0 || p.dsp.RateDenominator == 0 || d.Mask&2 != 0 || d.Mode[1] != mode ||
		d.Known[2] != 3 || d.Known[3] != 3 || !d.PageKnown[1] || n == 0 || n > uint32(d.Current[3])+1 {
		return false
	}
	p.dmaActive = true
	p.dmaLeft = n
	p.dmaBlockSize = n
	p.dmaAuto = auto
	p.dmaStereo, p.dmaSigned, p.dmaFIFO = false, false, false
	p.sampleCredit = 0
	return true
}

// AdvanceRealMode 使用明示的1微秒／指令近似，並依真實IVT派送IRQ7。
func (p *LEOPLPorts) AdvanceRealMode(c *cpu.CPU, m *LEMachine) error {
	if p.realIRQ7 != nil && p.realIRQ7.failed {
		return fmt.Errorf("IRQ7 轉送已失敗，不可繼續計時")
	}
	p.virtualMicros++
	if p.BIOSClock != nil {
		if err := p.BIOSClock.advance(m, p, c.Flags&cpu.IF != 0); err != nil {
			return err
		}
	}
	if err := p.advanceDMA(m); err != nil {
		return err
	}
	return p.deliverIRQ7Real(c, m)
}

// 派送不再加時間，兩種模式共用真實IVT與硬體框架。
func (p *LEOPLPorts) deliverIRQ7Real(c *cpu.CPU, m *LEMachine) error {
	if p.picPending && !p.picInService && (p.BIOSClock == nil || !p.BIOSClock.InService) && p.picMasks[0]&0x80 == 0 && c.Flags&cpu.IF != 0 {
		if len(m.Mem) < 64 {
			return fmt.Errorf("IRQ7 IVT不可讀")
		}
		v, _ := m.Read32(0x0f * 4)
		a := uint32(uint16(v>>16))*16 + uint32(uint16(v))
		if v == 0 || uint64(a) >= uint64(len(m.Mem)) {
			return fmt.Errorf("IRQ7尚未安裝有效IVT")
		}
		if c.R[cpu.SP] < 6 || uint64(c.Seg[cpu.SS])*16+uint64(c.R[cpu.SP]) > uint64(len(m.Mem)) {
			return fmt.Errorf("IRQ7 stack不可寫")
		}
		p.picPending = false
		p.picInService = true
		p.IRQ7Deliveries++
		c.Interrupt(0x0f)
	}
	return nil
}

// advanceDMA 只推進取樣與來源狀態，時鐘由兩模式入口各加一次。
func (p *LEOPLPorts) advanceDMA(m *LEMachine) error {
	if p.dmaActive && p.dma.Mask&2 == 0 {
		multiplier := uint64(1)
		// 41h的rate是每channel；40h TimeConstant已包含channels，勿重複乘2。
		if p.dmaStereo && !p.dsp.TimeConstantKnown {
			multiplier = 2
		}
		p.sampleCredit += multiplier * p.dsp.RateNumerator
	}
	if p.dmaActive && p.sampleCredit >= 1000000*p.dsp.RateDenominator && p.dma.Mask&2 == 0 {
		p.sampleCredit -= 1000000 * p.dsp.RateDenominator
		d := p.dma
		a := uint32(d.Page[1])<<16 | uint32(d.Current[2])
		if uint64(a) >= uint64(len(m.Mem)) {
			return fmt.Errorf("DMA讀取超界 %06X", a)
		}
		if len(p.PCM) < 65536 {
			p.PCM = append(p.PCM, m.Mem[a])
		}
		d.Current[2]++
		terminal := d.Current[3] == 0
		d.Current[3]--
		if terminal {
			if p.dmaAuto {
				d.Current[2] = d.Base[2]
				d.Current[3] = d.Base[3]
			} else {
				d.Mask |= 2
			}
		}
		p.dmaLeft--
		if p.dmaLeft == 0 {
			p.dmaActive = p.dmaAuto
			if p.dmaAuto {
				p.dmaLeft = p.dmaBlockSize
			}
			p.DMACompletions++
			if !p.dsp.IRQPending {
				p.picPending = true
			}
			p.dsp.IRQPending = true
		}
	}
	if p.dma16Active && p.secondaryDMA.Mask&2 == 0 {
		// 立體聲每個 16 位元 word 約佔半個取樣週期；僅為規格近似。
		p.sample16Credit += 2 * p.dsp.RateNumerator
		if p.sample16Credit >= 1000000*p.dsp.RateDenominator {
			p.sample16Credit -= 1000000 * p.dsp.RateDenominator
			d := p.secondaryDMA
			a := uint32(d.Page[1]&0xfe)<<16 | uint32(d.Current[2])<<1
			if uint64(a)+1 >= uint64(len(m.Mem)) {
				return fmt.Errorf("16位元DMA讀取超界 %06X", a)
			}
			if len(p.PCM16) < 65536 {
				p.PCM16 = append(p.PCM16, m.Mem[a], m.Mem[a+1])
			}
			d.Current[2]++
			terminal := d.Current[3] == 0
			d.Current[3]--
			if terminal {
				d.Mask |= 2
			}
			p.dma16WordsLeft--
			if p.dma16WordsLeft == 0 {
				p.dma16Active = false
				p.DMA16Completions++
				if !p.dsp.IRQ16Pending {
					p.picPending = true
				}
				p.dsp.IRQ16Pending = true
			}
		}
	}
	return nil
}

// AdvanceProtectedMode 沿規格304共用1微秒近似；規格305轉送有限的實模式IRQ7。
func (p *LEOPLPorts) AdvanceProtectedMode(c *cpu386.CPU, m *LEMachine) error {
	if p.realIRQ7 != nil && p.realIRQ7.failed {
		return fmt.Errorf("IRQ7 轉送已失敗，不可繼續計時")
	}
	p.virtualMicros++
	if err := p.BIOSClock.advance(m, p, c.EFlags&cpu386.IF != 0); err != nil {
		return err
	}
	if err := p.advanceDMA(m); err != nil {
		return err
	}
	clock := p.BIOSClock
	if !p.picPending || p.picInService || clock.InService || p.picMasks[0]&0x80 != 0 || c.EFlags&cpu386.IF == 0 ||
		clock.protectedIRQ0 != nil && clock.protectedIRQ0.active || p.realIRQ7 != nil && p.realIRQ7.active {
		return nil
	}
	ivt, err := m.Read32(0x0f * 4)
	if err != nil {
		return fmt.Errorf("IRQ7 IVT不可讀")
	}
	var vector uint64
	var seg, off uint16
	if d := clock.protectedIRQ0; d != nil {
		vector = d.s.dosVectors[0x0f]
		seg, off = d.s.DPMI.RealModeVector(0x0f)
		if d.s.moo2Profile && vector == 0 && seg == 0 && off == 0 {
			if p.realIRQ7 == nil {
				p.realIRQ7 = &leRealIRQ7{h: d.s.DPMI}
			}
			return p.realIRQ7.dispatch(c, m, p)
		}
	}
	return fmt.Errorf("IRQ7 保護模式派送尚未建模：absolute_ivt0f=%08X dpmi_rm0f=%04X:%04X dos_pm0f=%04X:%08X", ivt, seg, off, uint16(vector>>32), uint32(vector))
}
