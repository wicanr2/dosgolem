package machine

// LEOPLPorts 沿用既有 OPL／VGA 狀態，並轉接受限 DSP；未知埠明確拒絕。
// 計時器為既有偵測近似，不代表真實時間或音訊波形。
type LEOPLPorts struct {
	PIT0           LEPIT0
	BIOSClock      *LEBIOSClock
	dmaAuto        bool
	dmaBlockSize   uint32
	sampleCredit   uint64
	device         *Machine
	virtualMicros  uint64
	dmaActive      bool
	dmaLeft        uint32
	picPending     bool
	picInService   bool
	picReadISR     [2]bool
	DMACompletions uint64
	IRQ7Deliveries uint64
	PCM            []byte
	dsp            SoundBlasterDSP
	picMasks       [2]byte
	dma            *DMA8237
	Log            []LEOPLPortEvent
	Reads          map[uint16]uint64
	Writes         map[uint16]uint64
}
type LEOPLPortEvent struct {
	Port  uint16
	Value uint8
	Write bool
}

func NewLEOPLPorts() *LEOPLPorts {
	m := New()
	m.SetAdLib(true)
	p := &LEOPLPorts{device: m, dma: NewDMA8237(), picMasks: [2]byte{0xf8, 0x2c}, Reads: map[uint16]uint64{}, Writes: map[uint16]uint64{}}
	p.dsp.StartDMA = p.startDSPDMA
	p.dsp.StartAutoDMA = func(n uint32) bool { return p.startDMA(n, true) }
	p.dsp.CancelDMA = func() { p.dmaActive = false; p.dmaLeft = 0; p.picPending = false }
	return p
}
func oplAlias(p uint16) (uint16, bool) {
	switch {
	case p >= 0x388 && p <= 0x38b:
		return p, true
	case p >= 0x220 && p <= 0x223:
		return p - 0x220 + 0x388, true
	case p == 0x228 || p == 0x229:
		return p - 0x228 + 0x388, true
	}
	return 0, false
}
func (p *LEOPLPorts) record(port uint16, v uint8, write bool) {
	if write {
		p.Writes[port]++
	} else {
		p.Reads[port]++
	}
	if len(p.Log) < 4096 {
		p.Log = append(p.Log, LEOPLPortEvent{port, v, write})
	}
}
func (p *LEOPLPorts) In8(port uint16) (uint8, bool) {
	if port == 0x20 || port == 0xa0 {
		v := byte(0)
		if port == 0x20 && (p.picReadISR[0] && p.picInService || !p.picReadISR[0] && p.picPending) {
			v = 0x80
		}
		if port == 0x20 && !p.picReadISR[0] && p.BIOSClock != nil && p.BIOSClock.Pending {
			v |= 1
		}
		p.record(port, v, false)
		return v, true
	}

	if v, ok := p.dma.In8(port); ok {
		p.record(port, v, false)
		return v, true
	}
	if port == 0x21 || port == 0xa1 {
		i := 0
		if port == 0xa1 {
			i = 1
		}
		v := p.picMasks[i]
		p.record(port, v, false)
		return v, true
	}
	if v, ok := p.dsp.In8(port); ok {
		p.record(port, v, false)
		return v, true
	}
	if port == 0x3da {
		v := p.device.In8(port)
		p.record(port, v, false)
		return v, true
	}
	alias, ok := oplAlias(port)
	if !ok {
		return 0, false
	}
	v := p.device.In8(alias)
	p.record(port, v, false)
	return v, true
}
func (p *LEOPLPorts) Out8(port uint16, v uint8) bool {
	if p.PIT0.Out8(port, v) {
		p.record(port, v, true)
		return true
	}
	if port == 0x3c8 || port == 0x3c9 {
		p.device.Out8(port, v)
		p.record(port, v, true)
		return true
	}

	if (port == 0x20 || port == 0xa0) && (v == 0x0a || v == 0x0b) {
		i := 0
		if port == 0xa0 {
			i = 1
		}
		p.picReadISR[i] = v == 0x0b
		p.record(port, v, true)
		return true
	}

	if port == 0x20 && v == 0x20 {
		p.picInService = false
		p.record(port, v, true)
		return true
	}
	if p.dma.Out8(port, v) {
		p.record(port, v, true)
		return true
	}
	if port == 0x21 || port == 0xa1 {
		i := 0
		if port == 0xa1 {
			i = 1
		}
		p.picMasks[i] = v
		p.record(port, v, true)
		return true
	}
	if p.dsp.Out8(port, v) {
		p.record(port, v, true)
		return true
	}
	alias, ok := oplAlias(port)
	if !ok {
		return false
	}
	p.device.Steps++
	p.device.Out8(alias, v)
	p.record(port, v, true)
	return true
}

// LEDeviceState 是初始化與傳輸除錯的唯值快照，不暴露內部可寫指標。
type LEDeviceState struct {
	MixerIndex               byte
	DSPIRQPending            bool
	PICPending, PICInService bool
	VirtualMicros            uint64
	PICMasks                 [2]byte
	DMABase, DMACurrent      [8]uint16
	DMAPage                  [4]byte
	DMAMode                  [4]byte
	DMAMask                  byte
	DSPTimeConstant          byte
	DSPTimeConstantKnown     bool
}

func (p *LEOPLPorts) State() LEDeviceState {
	return LEDeviceState{p.dsp.mixerIndex, p.dsp.IRQPending, p.picPending, p.picInService, p.virtualMicros, p.picMasks, p.dma.Base, p.dma.Current, p.dma.Page, p.dma.Mode, p.dma.Mask, p.dsp.TimeConstant, p.dsp.TimeConstantKnown}
}
