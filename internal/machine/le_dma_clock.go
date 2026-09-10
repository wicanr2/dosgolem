package machine

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu"
)

func (p *LEOPLPorts) startDSPDMA(n uint32) bool { return p.startDMA(n, false) }
func (p *LEOPLPorts) startDMA(n uint32, auto bool) bool {
	d := p.dma
	mode := byte(0x48)
	if auto {
		mode = 0x58
	}
	if p.dmaActive || p.dsp.RateNumerator == 0 || p.dsp.RateDenominator == 0 || d.Mask&2 != 0 || d.Mode[1] != mode ||
		d.Known[2] != 3 || d.Known[3] != 3 || !d.PageKnown[1] || n == 0 || n > uint32(d.Current[3])+1 {
		return false
	}
	p.dmaActive = true
	p.dmaLeft = n
	p.dmaBlockSize = n
	p.dmaAuto = auto
	p.sampleCredit = 0
	return true
}

// AdvanceRealMode 使用明示的1微秒／指令近似，並依真實IVT派送IRQ7。
func (p *LEOPLPorts) AdvanceRealMode(c *cpu.CPU, m *LEMachine) error {
	p.virtualMicros++
	if p.BIOSClock != nil {
		if err := p.BIOSClock.advance(m, p, c.Flags&cpu.IF != 0); err != nil {
			return err
		}
	}
	if p.dmaActive && p.dma.Mask&2 == 0 {
		p.sampleCredit += p.dsp.RateNumerator
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
	if p.picPending && !p.picInService && p.picMasks[0]&0x80 == 0 && c.Flags&cpu.IF != 0 {
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
