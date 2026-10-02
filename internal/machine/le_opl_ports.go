package machine

// LEOPLPorts 沿用既有 OPL／VGA 狀態，並轉接受限 DSP；未知埠明確拒絕。
// 計時器為既有偵測近似，不代表真實時間或音訊波形。
type LEOPLPorts struct {
	PIT0                          LEPIT0
	BIOSClock                     *LEBIOSClock
	dmaAuto                       bool
	dmaStereo, dmaSigned, dmaFIFO bool
	dmaBlockSize                  uint32
	sampleCredit                  uint64
	device                        *Machine
	virtualMicros                 uint64
	dmaActive                     bool
	dma16Active                   bool
	dma16WordsLeft                uint32
	sample16Credit                uint64
	dmaLeft                       uint32
	picPending                    bool
	picInService                  bool
	picReadISR                    [2]bool
	DMACompletions                uint64
	DMA16Completions              uint64
	IRQ7Deliveries                uint64
	PCM                           []byte
	PCM16                         []byte
	dsp                           SoundBlasterDSP
	picMasks                      [2]byte
	dma                           *DMA8237
	secondaryDMA                  *DMA8237
	Log                           []LEOPLPortEvent
	Reads                         map[uint16]uint64
	Writes                        map[uint16]uint64
}
type LEOPLPortEvent struct {
	Port  uint16
	Value uint8
	Write bool
}

func NewLEOPLPorts() *LEOPLPorts {
	m := New()
	m.SetAdLib(true)
	p := &LEOPLPorts{device: m, dma: NewDMA8237(), secondaryDMA: NewDMA8237(), picMasks: [2]byte{0xf8, 0x2c}, Reads: map[uint16]uint64{}, Writes: map[uint16]uint64{}}
	p.dsp.StartDMA = p.startDSPDMA
	p.dsp.Start16DMA = p.startDSP16DMA
	p.dsp.StartAutoDMA = func(n uint32) bool { return p.startDMA(n, true) }
	p.dsp.StartSB16AutoDMA = p.startSB16AutoDMA
	p.dsp.CancelDMA = func() {
		p.dmaActive = false
		p.dmaLeft = 0
		p.dmaStereo, p.dmaSigned, p.dmaFIFO = false, false, false
		p.dma16Active = false
		p.dma16WordsLeft = 0
		p.picPending = false
	}
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

func secondaryDMARegister(port uint16) (uint16, bool) {
	if port >= 0xc0 && port <= 0xce && port&1 == 0 {
		return (port - 0xc0) >> 1, true
	}
	switch port {
	case 0xd4, 0xd6, 0xd8, 0xda, 0xdc, 0xde:
		return (port - 0xc0) >> 1, true
	}
	return 0, false
}

func secondaryDMAPageChannel(port uint16) (int, bool) {
	switch port {
	case 0x8f:
		return 0, true // PC DMA 通道 4
	case 0x8b:
		return 1, true // PC DMA 通道 5
	case 0x89:
		return 2, true // PC DMA 通道 6
	case 0x8a:
		return 3, true // PC DMA 通道 7
	}
	return 0, false
}

func (p *LEOPLPorts) In8(port uint16) (uint8, bool) {
	if port == 0x40 {
		v, ok := p.PIT0.readLatchedMode2()
		if ok {
			p.record(port, v, false)
		}
		return v, ok
	}
	if port == 0x20 || port == 0xa0 {
		v := byte(0)
		if port == 0x20 && (p.picReadISR[0] && p.picInService || !p.picReadISR[0] && p.picPending) {
			v = 0x80
		}
		if port == 0x20 && !p.picReadISR[0] && p.BIOSClock != nil && p.BIOSClock.Pending {
			v |= 1
		}
		if port == 0x20 && p.picReadISR[0] && p.BIOSClock != nil && p.BIOSClock.InService {
			v |= 1
		}
		p.record(port, v, false)
		return v, true
	}

	if ch, ok := secondaryDMAPageChannel(port); ok {
		if !p.secondaryDMA.PageKnown[ch] {
			return 0, false
		}
		v := p.secondaryDMA.Page[ch]
		p.record(port, v, false)
		return v, true
	}
	if reg, ok := secondaryDMARegister(port); ok && reg <= 7 {
		if v, known := p.secondaryDMA.In8(reg); known {
			p.record(port, v, false)
			return v, true
		}
		return 0, false
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
	if port == 0x3da || port == 0x3c6 {
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
	if port == 0x43 && v == 0 {
		if !p.PIT0.latchMode2(p.BIOSClock) {
			return false
		}
		p.record(port, v, true)
		return true
	}
	if p.PIT0.Out8(port, v) {
		p.record(port, v, true)
		return true
	}
	if port == 0x3c6 || port == 0x3c8 || port == 0x3c9 {
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
		if p.BIOSClock != nil && p.BIOSClock.InService {
			p.BIOSClock.InService = false
		} else {
			p.picInService = false
		}
		p.record(port, v, true)
		return true
	}
	if p.dma.Out8(port, v) {
		p.record(port, v, true)
		return true
	}
	if ch, ok := secondaryDMAPageChannel(port); ok {
		p.secondaryDMA.Page[ch] = v
		p.secondaryDMA.PageKnown[ch] = true
		p.record(port, v, true)
		return true
	}
	if reg, ok := secondaryDMARegister(port); ok {
		if p.secondaryDMA.Out8(reg, v) {
			p.record(port, v, true)
			return true
		}
		return false
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
	MixerIndex                            byte
	DSPIRQPending                         bool
	DSPIRQ16Pending                       bool
	PICPending, PICInService              bool
	VirtualMicros                         uint64
	PICMasks                              [2]byte
	DMABase, DMACurrent                   [8]uint16
	DMAPage                               [4]byte
	DMAMode                               [4]byte
	DMAMask                               byte
	SecondaryDMABase, SecondaryDMACurrent [8]uint16
	SecondaryDMAPage                      [4]byte
	SecondaryDMAMode                      [4]byte
	SecondaryDMAMask                      byte
	DMA16Active                           bool
	DSPTimeConstant                       byte
	DSPTimeConstantKnown                  bool
	DSPRateNumerator, DSPRateDenominator  uint64
	DMAKnown                              [8]uint8
	DMAPageKnown                          [4]bool
	DMAActive, DMAAuto                    bool
	DMABytesLeft, DMABlockSize            uint32
	DMACompletions, DMA16Completions      uint64
	PCMBytes                              int
	DMA8Stereo, DMA8Signed, DMA8FIFO      bool
	DSPAuto8Command                       [4]byte
	DSPAuto8Commands                      uint64
	DMA8SampleCredit                      uint64
}

func (p *LEOPLPorts) State() LEDeviceState {
	return LEDeviceState{
		MixerIndex: p.dsp.mixerIndex, DSPIRQPending: p.dsp.IRQPending, DSPIRQ16Pending: p.dsp.IRQ16Pending,
		PICPending: p.picPending, PICInService: p.picInService, VirtualMicros: p.virtualMicros, PICMasks: p.picMasks,
		DMABase: p.dma.Base, DMACurrent: p.dma.Current, DMAPage: p.dma.Page, DMAMode: p.dma.Mode, DMAMask: p.dma.Mask,
		SecondaryDMABase: p.secondaryDMA.Base, SecondaryDMACurrent: p.secondaryDMA.Current,
		SecondaryDMAPage: p.secondaryDMA.Page, SecondaryDMAMode: p.secondaryDMA.Mode, SecondaryDMAMask: p.secondaryDMA.Mask,
		DMA16Active: p.dma16Active, DSPTimeConstant: p.dsp.TimeConstant, DSPTimeConstantKnown: p.dsp.TimeConstantKnown,
		DSPRateNumerator: p.dsp.RateNumerator, DSPRateDenominator: p.dsp.RateDenominator,
		DMAKnown: p.dma.Known, DMAPageKnown: p.dma.PageKnown,
		DMAActive: p.dmaActive, DMAAuto: p.dmaAuto, DMABytesLeft: p.dmaLeft, DMABlockSize: p.dmaBlockSize,
		DMACompletions: p.DMACompletions, DMA16Completions: p.DMA16Completions, PCMBytes: len(p.PCM),
		DMA8Stereo: p.dmaStereo, DMA8Signed: p.dmaSigned, DMA8FIFO: p.dmaFIFO,
		DSPAuto8Command: p.dsp.Auto8Command, DSPAuto8Commands: p.dsp.Auto8Commands,
		DMA8SampleCredit: p.sampleCredit,
	}
}
