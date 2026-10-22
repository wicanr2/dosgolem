package machine

import (
	"math"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 實模式的 SB 單次 8 位元 DMA 與 IRQ7（`docs/spec/445-real-mode-sb-single-dma`）。
//
// 保護模式那一條（`LEOPLPorts`，規格 296／304／305）有完整的 DMA 時鐘，
// 實模式的 `Machine` 原本只有 DSP 埠：DSP `14h` 收到長度時 `StartDMA` 是 nil，
// 命令被拒，音效永遠不會「播完」。驅動等 IRQ7 清「播放中」旗標的程式就停在
// 那裡（智冠中文版《魔法門 V》離開商店，規格 445 §2）。
//
// 只做規格 445 涵蓋的那一段：通道 1、單次、記憶體→裝置。自動初始化、16 位元
// 與暫停照舊不接，收到時 DSP 照舊拒絕，並記在 SBDMARejects。

// sbDMA 是一次單次傳輸的進度。
type sbDMA struct {
	active      bool
	left        uint32 // 還沒送的 byte
	credit      uint64 // 取樣信用：每道指令加 RateNumerator
	threshold   uint64 // 一個取樣要的信用：每秒指令數 × RateDenominator
	irq7Pending bool
}

// SBDMAState 是 DMA 控制器、DSP 時間設定與傳輸進度的存檔格式（gob 要匯出欄位）。
//
// 舊狀態檔沒有它：讀進來是 nil，機器保持「沒有傳輸、沒有掛起的 IRQ7」，
// 與舊執行器讀回來的結果相同。
type SBDMAState struct {
	Page      [4]uint8
	PageKnown [4]bool
	Base      [8]uint16
	Current   [8]uint16
	Known     [8]uint8
	Mode      [4]uint8
	Mask      uint8
	High      bool

	TimeConstant      uint8
	TimeConstantKnown bool
	RateNumerator     uint64
	RateDenominator   uint64
	SpeakerOn         bool
	IRQPending        bool

	Active      bool
	Left        uint32
	Credit      uint64
	Threshold   uint64
	IRQ7Pending bool

	Completions, IRQ7Deliveries, IRQ7Stalls, Rejects uint64
}

// SBPCMCap 是 SBPCM 最多留幾個 byte。觀測用，不是音訊輸出。
const SBPCMCap = 1 << 20

// wireSBDMA 在 SB 存在時建立第一組 DMA 控制器並接上 DSP 的回呼。
func (m *Machine) wireSBDMA() {
	if m.dma8 == nil {
		m.dma8 = NewDMA8237()
	}
	m.soundBlaster.StartDMA = m.startSBDMA
	m.soundBlaster.CancelDMA = m.cancelSBDMA
}

// startSBDMA 是 DSP `14h` 收到完整長度時的回呼。條件與 `LEOPLPorts.startDMA`
// 相同：通道 1 模式 48h（單次、讀）、未遮蔽、位址／計數／頁都寫過、長度不超過計數。
func (m *Machine) startSBDMA(n uint32) bool {
	d, s := m.dma8, &m.soundBlaster
	if m.sb.active || s.RateNumerator == 0 || s.RateDenominator == 0 || d.Mask&2 != 0 || d.Mode[1] != 0x48 ||
		d.Known[2] != 3 || d.Known[3] != 3 || !d.PageKnown[1] || n == 0 || n > uint32(d.Current[3])+1 {
		m.SBDMARejects++
		return false
	}
	sps := uint64(math.Round(m.StepsPerSecondNow()))
	if sps == 0 {
		m.SBDMARejects++
		return false
	}
	m.sb = sbDMA{active: true, left: n, threshold: sps * s.RateDenominator, irq7Pending: m.sb.irq7Pending}
	return true
}

// cancelSBDMA 是 DSP 重設：傳輸與掛起的 IRQ7 一併取消。
func (m *Machine) cancelSBDMA() {
	m.sb.active = false
	m.sb.irq7Pending = false
}

// sbTick 每道指令推進取樣並派送 IRQ7。呼叫端只在 active 或 irq7Pending 時呼叫。
func (m *Machine) sbTick() {
	if m.sb.active && m.dma8.Mask&2 == 0 {
		m.sb.credit += m.soundBlaster.RateNumerator
		if m.sb.credit >= m.sb.threshold {
			m.sb.credit -= m.sb.threshold
			m.sbTransferByte()
		}
	}
	if !m.sb.irq7Pending || !m.CPU.Flag(cpu.IF) || m.picMask&0x80 != 0 {
		return
	}
	// 與 keyTick 同樣的理由：向量還指著 stub 時不送，也不丟掉。
	if m.Read16(0x0f*4+2) == StubSeg {
		m.IRQ7Stalls++
		return
	}
	m.sb.irq7Pending = false
	m.IRQ7Deliveries++
	m.CPU.Interrupt(0x0f)
}

// sbTransferByte 送一個 byte：位址加一、計數減一，計數從 0 減到 FFFFh 是終點，
// 單次模式在終點設遮罩（8237A 資料表；DOSBox-X `src/hardware/dma.cpp` 的 ReachedTC）。
func (m *Machine) sbTransferByte() {
	d := m.dma8
	a := uint32(d.Page[1])<<16 | uint32(d.Current[2])
	v := m.Read8(a)
	if m.onSB != nil {
		m.onSB(SBSample{Step: m.Steps, Value: v})
	} else if len(m.SBPCM) < SBPCMCap {
		m.SBPCM = append(m.SBPCM, v)
	}
	d.Current[2]++
	terminal := d.Current[3] == 0
	d.Current[3]--
	if terminal {
		d.Mask |= 2
	}
	m.sb.left--
	if m.sb.left == 0 {
		m.sb.active = false
		m.SBDMACompletions++
		m.soundBlaster.IRQPending = true
		m.sb.irq7Pending = true
	}
}

// sbState 匯出存檔格式；沒有 SB 時回 nil。
func (m *Machine) sbState() *SBDMAState {
	if m.dma8 == nil {
		return nil
	}
	d, s := m.dma8, &m.soundBlaster
	return &SBDMAState{
		Page: d.Page, PageKnown: d.PageKnown, Base: d.Base, Current: d.Current, Known: d.Known, Mode: d.Mode, Mask: d.Mask, High: d.high,
		TimeConstant: s.TimeConstant, TimeConstantKnown: s.TimeConstantKnown, RateNumerator: s.RateNumerator,
		RateDenominator: s.RateDenominator, SpeakerOn: s.SpeakerOn, IRQPending: s.IRQPending,
		Active: m.sb.active, Left: m.sb.left, Credit: m.sb.credit, Threshold: m.sb.threshold, IRQ7Pending: m.sb.irq7Pending,
		Completions: m.SBDMACompletions, IRQ7Deliveries: m.IRQ7Deliveries, IRQ7Stalls: m.IRQ7Stalls, Rejects: m.SBDMARejects,
	}
}

// setSBState 還原存檔格式。nil（舊檔或存檔時沒有 SB）只清掉傳輸進度。
func (m *Machine) setSBState(st *SBDMAState) {
	m.sb = sbDMA{}
	if st == nil || m.dma8 == nil {
		return
	}
	d, s := m.dma8, &m.soundBlaster
	d.Page, d.PageKnown, d.Base, d.Current, d.Known, d.Mode, d.Mask, d.high =
		st.Page, st.PageKnown, st.Base, st.Current, st.Known, st.Mode, st.Mask, st.High
	s.TimeConstant, s.TimeConstantKnown, s.RateNumerator = st.TimeConstant, st.TimeConstantKnown, st.RateNumerator
	s.RateDenominator, s.SpeakerOn, s.IRQPending = st.RateDenominator, st.SpeakerOn, st.IRQPending
	m.sb = sbDMA{active: st.Active, left: st.Left, credit: st.Credit, threshold: st.Threshold, irq7Pending: st.IRQ7Pending}
	m.SBDMACompletions, m.IRQ7Deliveries, m.IRQ7Stalls, m.SBDMARejects = st.Completions, st.IRQ7Deliveries, st.IRQ7Stalls, st.Rejects
}
