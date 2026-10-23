package machine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"strconv"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 一般機器的 SB16 8 位元單次 DMA、ADPCM、靜音區塊與 IRQ7（docs/spec/444-legacy-sb16-dma8-irq7）。
//
// 硬體組態固定為基底 220h、8 位元 DMA 通道 1、IRQ 7（§2.1）。選項預設關閉（m.sb8 == nil）；
// 本檔的程式碼都只在 m.sb8 != nil 時執行。時基與 IRQ0 同一個時鐘（§2.4），不用 LE 的 1 微秒／指令。
//
// DSP D0h／D4h 的暫停與恢復見 docs/spec/445-legacy-sb16-dma8-pause-resume：暫停視為已停止的傳輸，
// 暫停期間可改寫 DMA，新的啟動命令隱式取消舊傳輸。
//
// idle 與靜音中的 D0h／D4h（接受、無狀態變化）與 22Ch 的忙碌週期見
// docs/spec/446-legacy-sb16-idle-control-and-busy-cycle。

const (
	// sbDMA8StateV 是 sbDMA8State 自帶的版本號（§2.9）。
	sbDMA8StateV = 1
	// stateVersionSB8 是選項開啟時 machineState.Version 寫的值（§2.9）；選項關閉照舊寫 stateVersion（3）。
	stateVersionSB8 = 4
	// sbDMALogMax 是 SBDMALog 的上限（§2.8）。
	sbDMALogMax = 4096
	// sbDMA8Ch 是固定的 8 位元 DMA 通道（§2.1）。
	sbDMA8Ch = 1
	// sbBusyHz 是 446 §2.4 的 H：DOSBox-X busy_cycle_hz 的預設 8333333/6/16（C 整數除法，sblaster.cpp:4344）。
	sbBusyHz = 8333333 / 6 / 16
)

// sbDMA8 是 §2.2–§2.7 的狀態：主片 8237 暫存器檔、TC 位元、PIC 的 IRQ7 最小模型與進行中的傳輸。
// 從片 IMR 與 OCW3 讀取選擇由 459 §3.3 搬到 Machine（m.pic），與保護模式剖面共用一份。
type sbDMA8 struct {
	dma  *DMA8237
	tc   uint8 // 08h 狀態暫存器 bit0–3（DMA8237 沒有 TC 欄位）
	irr7 bool
	isr7 bool

	// 進行中（或最近一次）的傳輸。active 為假時其餘欄位只是最近一次的紀錄。
	active  bool
	cmd     uint8
	n       uint32 // DMA byte 數（靜音為取樣數）
	samples uint64 // §2.4 的 N
	start   uint64
	due     uint64
	cycles  bool   // 時鐘單位：true＝CPU.Cycles，false＝Steps
	addr    uint16 // 開始時通道 1 的位址
	count   uint16 // 開始時通道 1 的計數（程式寫入值）

	// 445 §2.2：paused ⇒ active 且 cmd≠80h；!paused ⇒ pausedAt==0 且 !dmaRewritten。
	paused       bool   // 進行中的 DMA 傳輸已被 D0h 暫停
	pausedAt     uint64 // 暫停那一刻的原始時鐘（單位同 cycles）；未暫停時為 0
	dmaRewritten bool   // 暫停期間通道 1 的 DMA 已改寫（§2.6）
	pauses       uint64 // 實際進入暫停的次數（診斷，不進狀態檔，§2.9）
	resumes      uint64 // 實際恢復的次數（診斷，不進狀態檔）

	// 446 §2.4：22Ch 的忙碌週期（busy_cycle_io_hack、busy_cycle_last_check）。進狀態檔（446 §2.6）。
	busyIO         uint8  // busy_cycle_io_hack 對 16 取餘
	busyLast       uint64 // 上一次一般狀態下讀 22Ch 的 t
	busyLastCycles bool   // busyLast 的單位：真為 CPU.Cycles
	// 446 §2.3：idle 與靜音中被接受的 D0h／D4h 次數（診斷，不進狀態檔）。
	idleControls uint64

	dma8Completions    uint64
	silenceCompletions uint64
	irq7Deliveries     uint64
	irq7Defaulted      uint64
}

// SBDMA8Stats 是 probe 報告與測試用的唯讀檢視。
type SBDMA8Stats struct {
	DMA8Completions, SilenceCompletions uint64
	IRQ7Deliveries, IRQ7Defaulted       uint64
	IRR7, ISR7, Active                  bool
	// 445 §2.9：Paused 是目前狀態；Pauses、Resumes 是診斷計數（不進狀態檔，讀檔後從本機的累計繼續）。
	Paused          bool
	Pauses, Resumes uint64
	// 446 §2.3：idle 與靜音中被接受的 D0h／D4h 次數（診斷，不進狀態檔）。
	IdleControls uint64
}

// SoundBlaster 回 SetSoundBlaster 設的值（§2.1，與 AdLib 同類）。
func (m *Machine) SoundBlaster() bool { return m.soundBlasterPresent }

// SBDMA8 回 §2.1 的選項是否開啟。
func (m *Machine) SBDMA8() bool { return m.sb8 != nil }

// SBDMA8Stats 回計數器與 IRR7／ISR7；選項關閉時為零值。
func (m *Machine) SBDMA8Stats() SBDMA8Stats {
	s := m.sb8
	if s == nil {
		return SBDMA8Stats{}
	}
	return SBDMA8Stats{DMA8Completions: s.dma8Completions, SilenceCompletions: s.silenceCompletions,
		IRQ7Deliveries: s.irq7Deliveries, IRQ7Defaulted: s.irq7Defaulted, IRR7: s.irr7, ISR7: s.isr7, Active: s.active,
		Paused: s.paused, Pauses: s.pauses, Resumes: s.resumes, IdleControls: s.idleControls}
}

// SBDMA8Err 回 §2.7 的黏著錯誤（沒有時 nil）。
func (m *Machine) SBDMA8Err() error { return m.sb8Err }

// SetSBDMA8 開啟 §2.1 的選項。條件：SoundBlaster() 為真、尚未載入程式、len(Mem)==MemSize；違反時回錯誤、狀態不變。
// 開啟後不可關閉：SetSBDMA8(false) 回錯誤。已開啟時再開啟不動作。
func (m *Machine) SetSBDMA8(on bool) error {
	if !on {
		if m.sb8 != nil {
			return errors.New("machine: SB16 DMA8／IRQ7 選項開啟後不能關閉（docs/spec/444 §2.1）")
		}
		return nil
	}
	if m.sb8 != nil {
		return nil
	}
	switch {
	case !m.soundBlasterPresent:
		return errors.New("machine: SB16 DMA8／IRQ7 選項需要先 SetSoundBlaster(true)（docs/spec/444 §2.1）")
	case m.programLoaded:
		return errors.New("machine: SB16 DMA8／IRQ7 選項只能在載入程式之前開啟（docs/spec/444 §2.1）")
	case len(m.Mem) != MemSize:
		return fmt.Errorf("machine: SB16 DMA8／IRQ7 選項需要 %d bytes 的記憶體，這台是 %d（docs/spec/444 §2.1）", MemSize, len(m.Mem))
	}
	m.sb8 = &sbDMA8{dma: NewDMA8237()}
	m.pic.slaveIMR = 0xFF // 444 §2.6 的初值；剖面開啟時 LoadEXE／LoadCOM 另清 bit 4（459 §3.7）
	m.wireSB8()
	return nil
}

// wireSB8 把 §2.3 的回呼接到這台機器（SetSBDMA8、Restore、LoadState 都會呼叫）。
// StartAutoDMA、StartSB16AutoDMA、Start16DMA 維持 nil；ControlDMA8 由 445 §2.1 接上。
func (m *Machine) wireSB8() {
	d := &m.soundBlaster
	d.StartDMA = func(n uint32) bool { return m.sb8StartDMA(0x14, n) }
	d.StartADPCMDMA = m.sb8StartDMA
	d.StartSilence = m.sb8StartSilence
	d.CancelDMA = m.sb8Cancel
	d.ControlDMA8 = m.sb8Control
}

// sb8Fail 是 §2.2–§2.6 的失敗即關閉：存進黏著欄位 sb8Err（只留第一個），由 Step 回傳。
func (m *Machine) sb8Fail(format string, a ...any) {
	if m.sb8Err != nil {
		return
	}
	m.sb8Err = fmt.Errorf("machine: %04X:%04X SB16 DMA8／IRQ7 失敗即關閉：%s（step=%d，docs/spec/444）",
		m.insnCS, m.insnIP, fmt.Sprintf(format, a...), m.Steps)
}

// sb8DSPFail 是 §2.3 表末列：SoundBlasterDSP.Out8 對 224h–22Fh 回 false 的寫入。
func (m *Machine) sb8DSPFail(p uint16, v uint8) {
	d := &m.soundBlaster
	m.sb8Fail("DSP 寫入 %03Xh←%02Xh 未被接受（命令或參數未實作；DSP 半狀態 pending=%02Xh lengthHighNext=%t "+
		"lengthLow=%02Xh reset=%t b0Step=%d c6Step=%d）", p, v, d.pending, d.lengthHighNext, d.lengthLow, d.reset, d.b0Step, d.c6Step)
}

// sbFormat 回命令的類型名稱與每個 DMA byte 的取樣數（§2.3、§2.4）。
func sbFormat(cmd uint8) (string, uint64) {
	switch cmd {
	case 0x14:
		return "pcm8", 1
	case 0x74:
		return "adpcm4", 2
	case 0x75:
		return "adpcm4ref", 2
	case 0x76:
		return "adpcm26", 3
	case 0x77:
		return "adpcm26ref", 3
	case 0x16:
		return "adpcm2", 4
	case 0x17:
		return "adpcm2ref", 4
	case 0x80:
		return "silence", 1
	}
	return "", 0
}

// ---- §2.4 傳輸啟動與時間 ----------------------------------------------------

// sb8StartDMA 是 14h 與 ADPCM 的啟動（§2.4 第 1–8 項）。任一不成立回 false 並失敗即關閉。
func (m *Machine) sb8StartDMA(cmd uint8, n uint32) bool {
	s := m.sb8
	d := s.dma
	_, mult := sbFormat(cmd)
	if s.active && !s.paused { // 445 §2.7 第 1 項：未暫停的傳輸照 444 失敗即關閉
		m.sb8Fail("傳輸進行中又收到啟動命令 %02Xh", cmd)
		return false
	}
	if s.active { // 445 §2.7 第 2 項：暫停中先隱式取消舊傳輸，再檢查 444 §2.4 第 2–8 項
		m.sb8CancelBody()
	}
	switch {
	case d.Known[2] != 3 || d.Known[3] != 3 || !d.PageKnown[sbDMA8Ch]:
		m.sb8Fail("DSP %02Xh 啟動時通道 1 的位址、計數或頁暫存器未寫齊（Known=%d/%d、PageKnown=%t）",
			cmd, d.Known[2], d.Known[3], d.PageKnown[sbDMA8Ch])
		return false
	case d.Mode[sbDMA8Ch] != 0x48:
		m.sb8Fail("DSP %02Xh 啟動時通道 1 模式 %02Xh 不是 48h（單次、讀、遞增、不自動初始化）", cmd, d.Mode[sbDMA8Ch])
		return false
	case d.Mask&2 != 0:
		m.sb8Fail("DSP %02Xh 啟動時通道 1 被遮罩（Mask=%X）", cmd, d.Mask)
		return false
	case uint64(n) > uint64(d.Current[3])+1:
		m.sb8Fail("DSP %02Xh 長度 %d 大於通道 1 計數 %d+1", cmd, n, d.Current[3])
		return false
	}
	page := uint32(d.Page[sbDMA8Ch]) << 16
	for i := uint32(0); i < n; i++ {
		if a := page | uint32(d.Current[2]+uint16(i)); a >= 0xA0000 && a < 0xC0000 {
			m.sb8Fail("DSP %02Xh 的 DMA 來源 %06Xh 落在 A0000h–BFFFFh", cmd, a)
			return false
		}
	}
	return m.sb8Begin(cmd, n, uint64(n)*mult)
}

// sb8StartSilence 是 80h 的啟動（§2.4：只檢查第 1、7、8 項）。
func (m *Machine) sb8StartSilence(n uint32) bool {
	if m.sb8.active && !m.sb8.paused {
		m.sb8Fail("傳輸進行中又收到啟動命令 80h")
		return false
	}
	if m.sb8.active { // 445 §2.7：暫停中先隱式取消舊傳輸
		m.sb8CancelBody()
	}
	return m.sb8Begin(0x80, n, uint64(n))
}

// sb8Begin 算 Δ（第 7、8 項）、記下開始與到期時刻，並記 start 觀測。
func (m *Machine) sb8Begin(cmd uint8, n uint32, samples uint64) bool {
	s := m.sb8
	dsp := &m.soundBlaster
	if dsp.RateNumerator == 0 || dsp.RateDenominator == 0 {
		m.sb8Fail("DSP %02Xh 啟動時取樣率未設定（R=%d、D=%d）", cmd, dsp.RateNumerator, dsp.RateDenominator)
		return false
	}
	delta, ok := m.sb8Delta(samples, dsp.RateNumerator, dsp.RateDenominator)
	if !ok {
		m.sb8Fail("DSP %02Xh 的到期間隔 Δ 溢位（N=%d、R=%d、D=%d）", cmd, samples, dsp.RateNumerator, dsp.RateDenominator)
		return false
	}
	start := m.Steps
	if m.CycleClock {
		start = m.CPU.Cycles
	}
	due, carry := bits.Add64(start, delta, 0)
	if carry != 0 {
		m.sb8Fail("DSP %02Xh 的到期時刻溢位（開始 %d、Δ %d）", cmd, start, delta)
		return false
	}
	s.active, s.cmd, s.n, s.samples = true, cmd, n, samples
	s.start, s.due, s.cycles = start, due, m.CycleClock
	s.paused, s.pausedAt, s.dmaRewritten = false, 0, false // 445 §2.8
	s.addr, s.count = 0, 0
	if cmd != 0x80 {
		s.addr, s.count = s.dma.Current[2], s.dma.Current[3]
	}
	e := m.sb8Event("start", true, true)
	m.sb8Log(e)
	return true
}

// sb8Delta 是 §2.4 的表：以 128 位元整數算上整。商超過 2^63、或中間值超過 128 位元時回 false。Δ 至少為 1。
func (m *Machine) sb8Delta(samples, r, d uint64) (uint64, bool) {
	ndHi, nd := bits.Mul64(samples, d)
	if ndHi != 0 {
		return 0, false
	}
	var hi, lo, den uint64
	var ok bool
	switch {
	case m.CycleClock:
		hz := m.CPUHz
		if hz == 0 {
			hz = DefaultCPUHz
		}
		hi, lo = bits.Mul64(nd, hz)
		den = r
	case m.IRQ0Every > 0:
		h, l := bits.Mul64(nd, m.IRQ0Every)
		if hi, lo, ok = mul128x64(h, l, 315_000_000); !ok {
			return 0, false
		}
		if den, ok = mulU64(r, 264, uint64(m.PITDiv)); !ok {
			return 0, false
		}
	default:
		k := uint64(165_000) * 315_000_000 * uint64(m.ClockPercent())
		hi, lo = bits.Mul64(nd, k)
		if den, ok = mulU64(r, 264, 17_000, 100); !ok {
			return 0, false
		}
	}
	if den == 0 || hi >= den {
		return 0, false
	}
	q, rem := bits.Div64(hi, lo, den)
	if rem != 0 {
		q++ // 上整；q 原為 2^64−1 時加一環繞成 0，視為溢位
		if q == 0 {
			return 0, false
		}
	}
	if q > 1<<63 {
		return 0, false
	}
	if q == 0 {
		q = 1
	}
	return q, true
}

// mul128x64 回 (h:l)×k，超過 128 位元時 ok 為假。
func mul128x64(h, l, k uint64) (uint64, uint64, bool) {
	ph, pl := bits.Mul64(l, k)
	hh, hl := bits.Mul64(h, k)
	if hh != 0 {
		return 0, 0, false
	}
	hi, carry := bits.Add64(hl, ph, 0)
	if carry != 0 {
		return 0, 0, false
	}
	return hi, pl, true
}

// mulU64 回各數的乘積，溢位 uint64 時 ok 為假。
func mulU64(vs ...uint64) (uint64, bool) {
	p := uint64(1)
	for _, v := range vs {
		hi, lo := bits.Mul64(p, v)
		if hi != 0 {
			return 0, false
		}
		p = lo
	}
	return p, true
}

// ---- §2.5 傳輸完成與進度 ----------------------------------------------------

// sb8Now 是傳輸所用時鐘的現在值；暫停中回 pausedAt（445 §2.4），進度、即時讀值、取消與事件因此凍結。
func (m *Machine) sb8Now() uint64 {
	if m.sb8.paused {
		return m.sb8.pausedAt
	}
	return m.sb8RawNow()
}

// sb8RawNow 是 445 §2.5 的原始時鐘 now：傳輸的時鐘單位為步數時取 m.Steps，為週期時取 m.CPU.Cycles。
// 不受暫停影響；pausedAt 與恢復的平移量 d 只用這支。
func (m *Machine) sb8RawNow() uint64 {
	if m.sb8.cycles {
		return m.CPU.Cycles
	}
	return m.Steps
}

// sb8Progress 是已傳 k = min(n, ⌊經過·n/Δ⌋)（§2.5）。
func (m *Machine) sb8Progress() uint32 {
	s := m.sb8
	now := m.sb8Now()
	if now <= s.start {
		return 0
	}
	el, delta := now-s.start, s.due-s.start
	if el >= delta {
		return s.n
	}
	hi, lo := bits.Mul64(el, uint64(s.n))
	q, _ := bits.Div64(hi, lo, delta) // el < delta、n ≤ 2^16，所以 hi < delta
	if q > uint64(s.n) {
		return s.n
	}
	return uint32(q)
}

// sb8Poll 在 Step 開頭（tick 之前）判斷進行中的傳輸是否到期，到期就完成（§2.5 第 1–4 步）。
// 比較時 m.Steps 尚未遞增，等於上一道指令的序號。
func (m *Machine) sb8Poll() {
	s := m.sb8
	if s.paused { // 445 §2.4：暫停中不完成（pausedAt 可能 ≥ due）
		return
	}
	if m.sb8Now() < s.due {
		return
	}
	if s.cmd != 0x80 {
		d := s.dma
		d.Current[2] = s.addr + uint16(s.n)
		d.Current[3] = uint16(uint32(s.count) - s.n)
		if uint32(s.count)+1 == s.n { // 到 TC：計數成 FFFFh、設 TC 位元、設遮罩（dma.cpp:513-549）
			s.tc |= 1 << sbDMA8Ch
			d.Mask |= 1 << sbDMA8Ch
		}
	}
	e := m.sb8Event("complete", false, true)
	e.Transferred = s.n
	m.sb8Log(e)
	dsp := &m.soundBlaster
	if !dsp.IRQPending {
		s.irr7 = true
	}
	dsp.IRQPending = true
	s.active = false
	if s.cmd == 0x80 {
		s.silenceCompletions++
	} else {
		s.dma8Completions++
	}
}

// sb8Cancel 是 DSP 重設的 CancelDMA（444 §2.3、445 §2.8）：取消主體之後，IRR7 若尚未被 CPU 接受則清除。
func (m *Machine) sb8Cancel() {
	m.sb8CancelBody()
	m.sb8.irr7 = false
	m.stuckClear(stuckIRQ7) // 459 §3.3a：待處理消失，結束被擋住的一段
}

// sb8CancelBody 是 445 §2.8 的取消主體（隱式取消與 DSP 重設共用）：進行中的傳輸或靜音取消，
// DMA 依進度（暫停中即凍結的進度）寫回暫存器、不設 TC；暫停期間改寫過 DMA 時不寫回。
// 不動 IRR7、ISR7、IRQPending。清旗標在算進度之後。
func (m *Machine) sb8CancelBody() {
	s := m.sb8
	if !s.active {
		return
	}
	k := uint32(0)
	if s.cmd != 0x80 {
		k = m.sb8Progress()
		if !s.dmaRewritten {
			s.dma.Current[2] = s.addr + uint16(k)
			s.dma.Current[3] = uint16(uint32(s.count) - k)
		}
	}
	e := m.sb8Event("cancel", true, true)
	e.Transferred = k
	m.sb8Log(e)
	s.active = false
	s.paused, s.pausedAt, s.dmaRewritten = false, 0, false
}

// ---- 445 §2.3、§2.5 暫停與恢復 ----------------------------------------------------

// sbDMACmd 回 cmd 是否屬 445 §2.3 的「DMA 傳輸」（14h、74h–77h、16h、17h）。
func sbDMACmd(cmd uint8) bool {
	_, mult := sbFormat(cmd)
	return mult != 0 && cmd != 0x80
}

// sb8Control 是 SoundBlasterDSP.ControlDMA8（445 §2.3，446 §2.2 修訂）：paused 為真代表 D0h，否則 D4h。
// 拒絕時先以 sb8Fail 寫入訊息再回 false（sb8Fail 只留第一個錯誤，所以保留的是這裡的訊息）。
func (m *Machine) sb8Control(paused bool) bool {
	s := m.sb8
	op := uint8(0xD4)
	if paused {
		op = 0xD0
	}
	switch {
	case !s.active: // 446 §2.2：idle 接受，無狀態變化
		m.sb8IdleControl(op, false)
		return true
	case !sbDMACmd(s.cmd): // 446 §2.2：80h 靜音中接受，無狀態變化；靜音照原到期時刻完成
		m.sb8IdleControl(op, true)
		return true
	case paused:
		if s.paused { // 冪等：pausedAt 不覆寫，不計數
			return true
		}
		s.pausedAt = m.sb8RawNow()
		s.paused = true
		s.pauses++
		e := m.sb8Event("pause", true, true)
		e.Transferred = m.sb8Progress()
		m.sb8Log(e)
		return true
	case !s.paused: // 未暫停的 DMA 傳輸中的 D4h：接受，不動作
		return true
	case s.dmaRewritten:
		m.sb8Fail("DSP 寫入 22Ch←D4h 未被接受：暫停期間通道 1 的 DMA 已改寫，恢復的語意沒有證據（docs/spec/445 §2.3）")
		return false
	}
	return m.sb8Resume()
}

// sb8IdleControl 是 446 §2.2 前兩列的接受：只計數與記 control-idle（446 §2.3），不動任何狀態。
// silence 為真時帶入靜音的傳輸欄位（Format=silence），再把 Command 覆寫為 D0h 或 D4h。
func (m *Machine) sb8IdleControl(op uint8, silence bool) {
	m.sb8.idleControls++
	e := m.sb8Event("control-idle", true, silence)
	e.Command = op
	m.sb8Log(e)
}

// sb8Resume 是 445 §2.5 的恢復：d = now − pausedAt，start、due 各加 d；due + d 溢位時失敗即關閉、欄位都不改。
func (m *Machine) sb8Resume() bool {
	s := m.sb8
	k := m.sb8Progress() // 凍結的進度；平移後 now − start' = pausedAt − start，所以與恢復後的進度相同
	d := m.sb8RawNow() - s.pausedAt
	due, carry := bits.Add64(s.due, d, 0)
	if carry != 0 {
		m.sb8Fail("DSP D4h 恢復時到期時刻溢位（到期 %d、平移 %d，docs/spec/445 §2.5）", s.due, d)
		return false
	}
	s.start += d
	s.due = due
	s.paused, s.pausedAt, s.dmaRewritten = false, 0, false
	s.resumes++
	e := m.sb8Event("resume", true, true)
	e.Transferred = k
	m.sb8Log(e)
	return true
}

// ---- §2.2、§2.6 埠 ------------------------------------------------------------

// sb8In 處理選項開啟時的讀取：00h–08h、20h、A0h、A1h、頁暫存器、22Ch（446 §2.4）與 22Eh。回 false 表示交回既有路徑。
func (m *Machine) sb8In(port uint16) (uint8, bool) {
	s := m.sb8
	switch port {
	case 0, 1, 2, 3, 4, 5, 6, 7:
		return m.sb8ReadDMAReg(port), true
	case 0x08:
		v := s.tc & 0x0F // bit4–7 為 0（dma.cpp:388-397），讀後清 TC
		s.tc = 0
		return v, true
	case 0x20, 0xA0, 0xA1: // 459 §3.3：共用的 PIC 埠模型（剖面關閉時與 444 相同）
		return m.picIn(port), true
	case 0x87, 0x83, 0x81, 0x82:
		if v, ok := s.dma.In8(port); ok {
			return v, true
		}
		return 0, false // 未寫過的頁暫存器維持現況（FFh）
	case 0x22C:
		return m.sb8BusyRead(), true
	case 0x22E:
		v, _ := m.soundBlaster.In8(port) // 清 IRQPending
		s.irr7 = false                   // 尚未被 CPU 接受的 IRR7 一併清除（sblaster.cpp:3158-3163）
		m.stuckClear(stuckIRQ7)          // 459 §3.3a：待處理消失，結束被擋住的一段
		return v, true
	}
	return 0, false
}

// ---- 446 §2.4 22Ch 的忙碌週期 ------------------------------------------------

// sb8BusyRead 是 446 §2.4 的「每次讀取」。只由 CPU 的 in（含 insb）經 Machine.In8 觸發；主機端工具不得以 In8(22Ch) 窺看。
//
// 時基 A/B 與三個常數先算；溢位或 A==0 時失敗即關閉、回 FFh，三個忙碌欄位不動。
func (m *Machine) sb8BusyRead() uint8 {
	s := m.sb8
	if m.soundBlaster.reset { // 226h 寫 1 之後、寫 0 之前：回 FFh，三欄不動（sblaster.cpp:3216-3218）
		return 0xFF
	}
	a, k, g, ok := m.sb8BusyRate()
	if !ok {
		m.sb8Fail("22Ch 忙碌週期的時基溢位（CycleClock=%t、CPUHz=%d、IRQ0Every=%d、PITDiv=%d、ClockPercent=%d，docs/spec/446 §2.4）",
			m.CycleClock, m.CPUHz, m.IRQ0Every, m.PITDiv, m.ClockPercent())
		return 0xFF
	}
	t := m.Steps
	if m.CycleClock {
		t = m.CPU.Cycles
	}
	s.busyIO = (s.busyIO + 1) & 15 // 第 1 步：在判斷之前（sblaster.cpp:3204）
	rephase := s.busyLastCycles != m.CycleClock || t < s.busyLast
	if !rephase { // 20 µs：(t − busyLast)·50000·B ≥ A（sblaster.cpp:1756 的 >=）
		gh, gl := bits.Mul64(t-s.busyLast, g)
		rephase = gh != 0 || gl >= a
	}
	if rephase { // p(t) = ⌊16·H·B·t / A⌋ mod 16（sblaster.cpp:1757）
		hi, lo := bits.Mul64(k, t)
		q, _ := bits.Div64(hi%a, lo, a) // 2^64 是 16 的倍數，只需商的低 64 位元
		s.busyIO = uint8(q & 15)
	}
	s.busyLast, s.busyLastCycles = t, m.CycleClock // 第 3 步：每次都更新（sblaster.cpp:1759）
	if s.busyIO < 8 {                              // duty 50：(io·100)/16 < 50 ⇔ io < 8
		return 0xFF
	}
	return 0x7F
}

// sb8BusyRate 回 446 §2.4 的 A（每秒時鐘數的分子）、K = 16·H·B、G = 50000·B。任一溢位或 A==0 時 ok 為假。
// 時鐘種類、PIT 分頻、IRQ0Every、時脈比例都取讀取當下的值（不用傳輸的 s.cycles）。
func (m *Machine) sb8BusyRate() (a, k, g uint64, ok bool) {
	var b uint64
	switch {
	case m.CycleClock:
		a = m.CPUHz
		if a == 0 {
			a = DefaultCPUHz
		}
		b, ok = 1, true
	case m.IRQ0Every > 0:
		if a, ok = mulU64(m.IRQ0Every, 315_000_000); !ok {
			return 0, 0, 0, false
		}
		b, ok = mulU64(264, uint64(m.PITDiv))
	default:
		if a, ok = mulU64(165_000, 315_000_000, uint64(m.ClockPercent())); !ok {
			return 0, 0, 0, false
		}
		b, ok = mulU64(264, 17_000, 100)
	}
	if !ok || a == 0 {
		return 0, 0, 0, false
	}
	if k, ok = mulU64(16, sbBusyHz, b); !ok {
		return 0, 0, 0, false
	}
	if g, ok = mulU64(50_000, b); !ok {
		return 0, 0, 0, false
	}
	return a, k, g, true
}

// sb8ReadDMAReg 讀 00h–07h：依 flip-flop 取低／高位元組並切換；Known 位元缺時不切換、回 FFh 並計數。
// 傳輸進行中的通道 1 位址與計數改用 §2.5 的即時值，不寫回暫存器。
func (m *Machine) sb8ReadDMAReg(port uint16) uint8 {
	s := m.sb8
	d := s.dma
	shift, bit := uint(0), uint8(1)
	if d.high {
		shift, bit = 8, 2
	}
	if d.Known[port]&bit == 0 {
		m.SBDMAUnknownReads++
		return 0xFF
	}
	v := d.Current[port]
	if s.active && !s.dmaRewritten && s.cmd != 0x80 && (port == 2 || port == 3) { // 445 §2.6：改寫後回暫存器本身
		k := uint16(m.sb8Progress())
		if port == 2 {
			v = s.addr + k
		} else {
			v = s.count - k
		}
	}
	d.high = !d.high
	return uint8(v >> shift)
}

// sb8Out 處理選項開啟時的寫入：00h–0Fh、20h、A0h、A1h、頁暫存器。回 false 表示交回既有路徑。
func (m *Machine) sb8Out(p uint16, v uint8) bool {
	s := m.sb8
	d := s.dma
	xfer := s.active && !s.paused && s.cmd != 0x80 // 靜音不碰 DMA（§2.5 第 1 步）；暫停中見 445 §2.6
	if s.paused && !s.dmaRewritten && sb8RewritesCh1(p, v) {
		// 445 §2.6：暫停期間第一次改寫之前，先把凍結的進度寫進現值暫存器（Base 不動），再照 444 §2.2 執行寫入。
		k := m.sb8Progress()
		d.Current[2] = s.addr + uint16(k)
		d.Current[3] = uint16(uint32(s.count) - k)
		s.dmaRewritten = true
	}
	switch p {
	case 0x08, 0x09:
		if v != 0 {
			m.sb8Fail("DMA %02Xh 寫入非 0 值 %02Xh（命令／請求暫存器未建模）", p, v)
		}
		return true
	case 0x02, 0x03:
		if xfer {
			m.sb8Fail("傳輸進行中改寫通道 1 的%s暫存器（%02Xh←%02Xh）", map[uint16]string{2: "位址", 3: "計數"}[p], p, v)
			return true
		}
	case 0x0B:
		if xfer && v&3 == sbDMA8Ch {
			m.sb8Fail("傳輸進行中改寫通道 1 的模式（0Bh←%02Xh）", v)
			return true
		}
	case 0x0A, 0x0D, 0x0F:
		if xfer {
			nm := d.Mask
			switch p {
			case 0x0A:
				if bit := uint8(1) << (v & 3); v&4 != 0 {
					nm |= bit
				} else {
					nm &^= bit
				}
			case 0x0D:
				nm = 15
			case 0x0F:
				nm = v & 15
			}
			if d.Mask&2 == 0 && nm&2 != 0 {
				m.sb8Fail("傳輸進行中遮罩通道 1（%02Xh←%02Xh）", p, v)
				return true
			}
		}
	case 0x83:
		if xfer {
			m.sb8Fail("傳輸進行中改寫通道 1 的頁暫存器（83h←%02Xh）", v)
			return true
		}
		d.Out8(p, v)
		return true
	case 0x87, 0x81, 0x82:
		d.Out8(p, v)
		return true
	case 0x20, 0xA0, 0xA1: // 459 §3.3：共用的 PIC 埠模型（剖面關閉時與 444 相同）
		m.picOut(p, v)
		return true
	}
	if p <= 0x0F {
		d.Out8(p, v)
		if p == 0x0D {
			s.tc = 0 // 主清除另清 TC 位元（dma.cpp:336-344）
		}
		return true
	}
	return false
}

// sb8RewritesCh1 是 445 §2.6 的表：暫停期間視為改寫通道 1 的寫入。
// 0Ch（清 flip-flop）、08h、09h、讀取、其他通道的暫存器不算。
func sb8RewritesCh1(p uint16, v uint8) bool {
	switch p {
	case 0x02, 0x03, 0x83, 0x0D, 0x0E, 0x0F:
		return true
	case 0x0A, 0x0B:
		return v&3 == sbDMA8Ch
	}
	return false
}

// 20h、A0h 的寫入見 pic.go 的 picOut（459 §3.3；剖面關閉時即 444 §2.6 的原規則）。

// sb8EOI 清 ISR7（為 1 時）並記 eoi。
func (m *Machine) sb8EOI() {
	if !m.sb8.isr7 {
		return
	}
	m.sb8.isr7 = false
	m.sb8Log(m.sb8Event("eoi", true, false))
}

// ---- §2.7 IRQ7 遞送 -----------------------------------------------------------

// sb8IRQ7Ready 是 §2.7 第 1 步的條件（也是 segWait 的「有待處理 IRQ」的 IRQ7 部分）。
// 459 §3.3、§3.3a：另要 ISR0、ISR1、ISR2 為 0（這三位只在保護模式剖面設起，剖面關閉時恆為 0）。
func (m *Machine) sb8IRQ7Ready() bool {
	s := m.sb8
	return s != nil && s.irr7 && !s.isr7 && m.picMask&0x80 == 0 && m.pic.isr&7 == 0
}

// sb8IRQ7 是 Step 的 IRQ7 分支（鍵盤分支之後、Steps++ 之前），§2.7 第 1–7 步。
//
// 第 4 步（分段模式）的 segWait 在第 1 步成立後、第 2、3 步之前呼叫，與既有鍵盤分支相同：segWait 自己判斷 IF，
// IF=0 的步也要呼叫才會把計數歸零（434 §5.3 的「連續」）。分段模式下 eventThisStep 不會為真、也不遞送，
// 所以遞送結果與規格的條列次序相同。
func (m *Machine) sb8IRQ7() {
	// 459 §3.3a：保護模式剖面在分段模式以外更新 IRQ7 的卡住計數（超過 L 時 irqErr 由 Step 回傳）。
	if m.pmProfile && !m.CPU.Segmented() && m.irq7Stuck() {
		return
	}
	if !m.sb8IRQ7Ready() {
		return
	}
	if m.CPU.Segmented() {
		m.segWait()
		return
	}
	if m.eventThisStep || !m.CPU.Flag(cpu.IF) {
		return
	}
	s := m.sb8
	if m.ReadSup16(0x0F*4+2) == StubSeg {
		s.irr7 = false
		s.irq7Defaulted++
		m.stuckClear(stuckIRQ7)
		m.sb8Log(m.sb8Event("defaulted", false, false))
		return
	}
	if m.ivtGuardIRQ(0x0F) {
		return
	}
	s.irr7, s.isr7 = false, true
	s.irq7Deliveries++
	m.eventThisStep = true
	if m.pmProfile {
		m.stuckClear(stuckIRQ7)
		m.picd.isrSetAt[4] = m.Steps
		if m.pic.isr&4 != 0 || m.pic.isr4 || m.pic.uir {
			m.picd.irq7InIRQ12++
		}
	}
	m.sb8Log(m.sb8Event("irq7", false, false))
	m.raiseIRQ(0x0F)
}

// ---- §2.8 觀測紀錄 -----------------------------------------------------------

// SBDMAEvent 是 §2.8 的一筆觀測。不進狀態檔、不影響執行。
type SBDMAEvent struct {
	Kind               string
	Step, Cycles       uint64
	CPUMode            string
	CS, IP             uint16
	Command            uint8
	Format             string
	Length             uint32
	Samples            uint64
	RateNum, RateDen   uint64
	TimeConstant       uint8
	TCKnown, SpeakerOn bool
	IRQ0Every          uint64
	PITDiv             uint32
	Clock              string
	Channel, DMAMode   uint8
	Page               uint8
	Addr, Count        uint16
	Phys               uint32
	Due                uint64
	Transferred        uint32
	SHA256             string
}

// SBDMAEventFields 是 -sb-dma-log 的欄名，與 §2.8 的表同序。
var SBDMAEventFields = []string{"Kind", "Step", "Cycles", "CPUMode", "CS", "IP", "Command", "Format", "Length", "Samples",
	"RateNum", "RateDen", "TimeConstant", "TCKnown", "SpeakerOn", "IRQ0Every", "PITDiv", "Clock",
	"Channel", "DMAMode", "Page", "Addr", "Count", "Phys", "Due", "Transferred", "SHA256"}

// Fields 回與 SBDMAEventFields 同序的欄值：步數、計數等十進位；段值、位移、命令、模式、頁、位址、計數、實體位址為十六進位；
// 布林值為 0／1。
func (e SBDMAEvent) Fields() []string {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	u := func(v uint64) string { return strconv.FormatUint(v, 10) }
	return []string{e.Kind, u(e.Step), u(e.Cycles), e.CPUMode,
		fmt.Sprintf("%04X", e.CS), fmt.Sprintf("%04X", e.IP), fmt.Sprintf("%02X", e.Command), e.Format,
		u(uint64(e.Length)), u(e.Samples), u(e.RateNum), u(e.RateDen), u(uint64(e.TimeConstant)), b(e.TCKnown), b(e.SpeakerOn),
		u(e.IRQ0Every), u(uint64(e.PITDiv)), e.Clock, u(uint64(e.Channel)), fmt.Sprintf("%02X", e.DMAMode),
		fmt.Sprintf("%02X", e.Page), fmt.Sprintf("%04X", e.Addr), fmt.Sprintf("%04X", e.Count), fmt.Sprintf("%06X", e.Phys),
		u(e.Due), u(uint64(e.Transferred)), e.SHA256}
}

// sb8Mode 回 CPUMode 欄的值。
func (m *Machine) sb8Mode() string {
	switch {
	case m.CPU.Segmented():
		return "seg"
	case m.v86:
		return "v86"
	}
	return "real"
}

// sb8Event 組一筆觀測。inInsn：事件發生在指令執行中（start、cancel、eoi、pause、resume、control-idle），CS:IP 取本道指令起點；
// 否則（complete、irq7、defaulted）取步間的 CS:IP。xfer：帶入傳輸欄位（start、complete、cancel、pause、resume、靜音中的 control-idle）。
func (m *Machine) sb8Event(kind string, inInsn, xfer bool) SBDMAEvent {
	s := m.sb8
	dsp := &m.soundBlaster
	e := SBDMAEvent{Kind: kind, Step: m.Steps, Cycles: m.CPU.Cycles, CPUMode: m.sb8Mode(),
		CS: m.CPU.Seg[cpu.CS], IP: m.CPU.IP,
		RateNum: dsp.RateNumerator, RateDen: dsp.RateDenominator, TimeConstant: dsp.TimeConstant,
		TCKnown: dsp.TimeConstantKnown, SpeakerOn: dsp.SpeakerOn,
		IRQ0Every: m.IRQ0Every, PITDiv: m.PITDiv, Clock: "steps"}
	if inInsn {
		e.CS, e.IP = m.insnCS, m.insnIP
	}
	cyc := m.CycleClock
	if xfer {
		cyc = s.cycles
		e.Command, e.Length, e.Samples, e.Due = s.cmd, s.n, s.samples, s.due
		e.Format, _ = sbFormat(s.cmd)
		if s.cmd != 0x80 && (kind == "start" || kind == "complete") { // 445 §2.9：cancel、pause、resume 為空
			e.SHA256 = m.sb8SHA()
		}
		if s.cmd != 0x80 {
			e.Channel, e.DMAMode, e.Page = sbDMA8Ch, s.dma.Mode[sbDMA8Ch], s.dma.Page[sbDMA8Ch]
			e.Addr, e.Count = s.addr, s.count
			e.Phys = uint32(e.Page)<<16 | uint32(e.Addr)
		}
	}
	if cyc {
		e.Clock = "cycles"
	}
	return e
}

// sb8SHA 以 peekPhysRaw 讀 n 個實體位址 Page<<16 | ((Addr+i)&FFFFh) 算 SHA-256（§2.5 第 2 步）。不寫記憶體。
func (m *Machine) sb8SHA() string {
	s := m.sb8
	page := uint32(s.dma.Page[sbDMA8Ch]) << 16
	buf := make([]byte, s.n)
	for i := range buf {
		buf[i] = m.peekPhysRaw(page | uint32(s.addr+uint16(i)))
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// sb8Log 記一筆觀測：OnSBDMA 非 nil 時只呼叫它（與 onOPL 同類）；否則追加到 SBDMALog，滿 4096 筆丟最舊的。
func (m *Machine) sb8Log(e SBDMAEvent) {
	if m.OnSBDMA != nil {
		m.OnSBDMA(e)
		return
	}
	if len(m.SBDMALog) >= sbDMALogMax {
		n := copy(m.SBDMALog, m.SBDMALog[len(m.SBDMALog)-sbDMALogMax+1:])
		m.SBDMALog = m.SBDMALog[:n]
		m.SBDMALogDropped++
	}
	m.SBDMALog = append(m.SBDMALog, e)
}

// ---- §2.9 狀態檔 -------------------------------------------------------------

// sbDMA8State 是 §2.9 的鏡像結構：全部欄位匯出（gob 忽略函式與未匯出欄位）。
type sbDMA8State struct {
	V int

	// SoundBlasterDSP 除回呼以外的全部欄位。
	SpeakerOn                      bool
	RateNumerator, RateDenominator uint64
	Auto8Command                   [4]byte
	Auto8Commands                  uint64
	BlockSize                      uint32
	BlockSizeKnown                 bool
	Pending                        uint8
	LengthLow                      byte
	LengthHighNext                 bool
	B0Step                         uint8
	C6Step, C6Mode                 uint8
	IRQPending, IRQ16Pending       bool
	TimeConstant                   uint8
	TimeConstantKnown              bool
	Reset                          bool
	MixerIndex                     uint8
	VoiceVolume                    [2]byte
	VoiceVolumeKnown               [2]bool
	Reply                          []byte

	// 主片 DMA8237 全部欄位與 TC 位元。
	DMAPage      [4]uint8
	DMAPageKnown [4]bool
	DMABase      [8]uint16
	DMACurrent   [8]uint16
	DMAKnown     [8]uint8
	DMAMode      [4]uint8
	DMAMask      uint8
	DMAHigh      bool
	DMATC        uint8

	// PIC。
	PICMask  uint8
	SlaveIMR uint8
	IRR7     bool
	ISR7     bool
	ReadISR  [2]bool

	// 傳輸。
	Active      bool
	Cmd         uint8
	N           uint32
	Samples     uint64
	Start, Due  uint64
	CycleUnit   bool
	Addr, Count uint16

	// 445 §2.10：暫停狀態。omitempty 讓未暫停時的 JSON（StateDigest）與 444 版相同。
	Paused                  bool   `json:",omitempty"`
	PausedAt                uint64 `json:",omitempty"`
	DMARewrittenWhilePaused bool   `json:",omitempty"`

	// 446 §2.6：22Ch 忙碌週期。omitempty 讓三欄為零值時的 JSON 與 445 版相同。
	DSPBusyIO         uint8  `json:",omitempty"`
	DSPBusyLast       uint64 `json:",omitempty"`
	DSPBusyLastCycles bool   `json:",omitempty"`

	DMA8Completions, SilenceCompletions, IRQ7Deliveries, IRQ7Defaulted uint64
}

// sb8Save 拍下 sbDMA8State（深拷貝）。
func (m *Machine) sb8Save() *sbDMA8State {
	s, d, dsp := m.sb8, m.sb8.dma, &m.soundBlaster
	return &sbDMA8State{
		V:         sbDMA8StateV,
		SpeakerOn: dsp.SpeakerOn, RateNumerator: dsp.RateNumerator, RateDenominator: dsp.RateDenominator,
		Auto8Command: dsp.Auto8Command, Auto8Commands: dsp.Auto8Commands,
		BlockSize: dsp.BlockSize, BlockSizeKnown: dsp.BlockSizeKnown,
		Pending: dsp.pending, LengthLow: dsp.lengthLow, LengthHighNext: dsp.lengthHighNext,
		B0Step: dsp.b0Step, C6Step: dsp.c6Step, C6Mode: dsp.c6Mode,
		IRQPending: dsp.IRQPending, IRQ16Pending: dsp.IRQ16Pending,
		TimeConstant: dsp.TimeConstant, TimeConstantKnown: dsp.TimeConstantKnown, Reset: dsp.reset,
		MixerIndex: dsp.mixerIndex, VoiceVolume: dsp.voiceVolume, VoiceVolumeKnown: dsp.voiceVolumeKnown,
		Reply: append([]byte(nil), dsp.reply...),

		DMAPage: d.Page, DMAPageKnown: d.PageKnown, DMABase: d.Base, DMACurrent: d.Current, DMAKnown: d.Known,
		DMAMode: d.Mode, DMAMask: d.Mask, DMAHigh: d.high, DMATC: s.tc,

		// 459 §5.2：主片 IMR、從片 IMR、讀取選擇取 Machine 的單一儲存位置（與 PM 版本 4 同值）。
		PICMask: m.picMask, SlaveIMR: m.pic.slaveIMR, IRR7: s.irr7, ISR7: s.isr7, ReadISR: m.pic.readISR,

		Active: s.active, Cmd: s.cmd, N: s.n, Samples: s.samples, Start: s.start, Due: s.due, CycleUnit: s.cycles,
		Addr: s.addr, Count: s.count,
		Paused: s.paused, PausedAt: s.pausedAt, DMARewrittenWhilePaused: s.dmaRewritten,
		DSPBusyIO: s.busyIO, DSPBusyLast: s.busyLast, DSPBusyLastCycles: s.busyLastCycles,

		DMA8Completions: s.dma8Completions, SilenceCompletions: s.silenceCompletions,
		IRQ7Deliveries: s.irq7Deliveries, IRQ7Defaulted: s.irq7Defaulted,
	}
}

// sb8Apply 把 sbDMA8State 寫回（深拷貝），並重新接上回呼。呼叫端保證選項開啟。
func (m *Machine) sb8Apply(st *sbDMA8State) {
	s, d, dsp := m.sb8, m.sb8.dma, &m.soundBlaster
	dsp.SpeakerOn, dsp.RateNumerator, dsp.RateDenominator = st.SpeakerOn, st.RateNumerator, st.RateDenominator
	dsp.Auto8Command, dsp.Auto8Commands = st.Auto8Command, st.Auto8Commands
	dsp.BlockSize, dsp.BlockSizeKnown = st.BlockSize, st.BlockSizeKnown
	dsp.pending, dsp.lengthLow, dsp.lengthHighNext = st.Pending, st.LengthLow, st.LengthHighNext
	dsp.b0Step, dsp.c6Step, dsp.c6Mode = st.B0Step, st.C6Step, st.C6Mode
	dsp.IRQPending, dsp.IRQ16Pending = st.IRQPending, st.IRQ16Pending
	dsp.TimeConstant, dsp.TimeConstantKnown, dsp.reset = st.TimeConstant, st.TimeConstantKnown, st.Reset
	dsp.mixerIndex, dsp.voiceVolume, dsp.voiceVolumeKnown = st.MixerIndex, st.VoiceVolume, st.VoiceVolumeKnown
	dsp.reply = append([]byte(nil), st.Reply...)

	d.Page, d.PageKnown, d.Base, d.Current, d.Known = st.DMAPage, st.DMAPageKnown, st.DMABase, st.DMACurrent, st.DMAKnown
	d.Mode, d.Mask, d.high = st.DMAMode, st.DMAMask, st.DMAHigh
	s.tc = st.DMATC

	m.picMask = st.PICMask
	m.pic.slaveIMR, s.irr7, s.isr7, m.pic.readISR = st.SlaveIMR, st.IRR7, st.ISR7, st.ReadISR

	s.active, s.cmd, s.n, s.samples = st.Active, st.Cmd, st.N, st.Samples
	s.start, s.due, s.cycles = st.Start, st.Due, st.CycleUnit
	s.addr, s.count = st.Addr, st.Count
	s.paused, s.pausedAt, s.dmaRewritten = st.Paused, st.PausedAt, st.DMARewrittenWhilePaused
	s.busyIO, s.busyLast, s.busyLastCycles = st.DSPBusyIO, st.DSPBusyLast, st.DSPBusyLastCycles

	s.dma8Completions, s.silenceCompletions = st.DMA8Completions, st.SilenceCompletions
	s.irq7Deliveries, s.irq7Defaulted = st.IRQ7Deliveries, st.IRQ7Defaulted
	m.wireSB8()
}

// sb8Validate 是 445 §2.10、446 §2.6 的讀檔驗證（LoadState 在 V 的檢查之後、任何寫入之前呼叫）。
func sb8Validate(st *sbDMA8State) error {
	switch {
	case st.Paused && (!st.Active || st.Cmd == 0x80):
		return fmt.Errorf("machine: sbDMA8State 暫停中但沒有進行中的 DMA 傳輸（Active=%t、Cmd=%02Xh，docs/spec/445 §2.10）", st.Active, st.Cmd)
	case st.Paused && st.PausedAt < st.Start:
		return fmt.Errorf("machine: sbDMA8State 的暫停時刻 %d 早於開始時刻 %d（docs/spec/445 §2.10）", st.PausedAt, st.Start)
	case st.Paused && !st.CycleUnit && st.PausedAt > st.Due:
		return fmt.Errorf("machine: sbDMA8State 的暫停時刻 %d 晚於到期時刻 %d（步數時鐘，docs/spec/445 §2.10）", st.PausedAt, st.Due)
	case !st.Paused && (st.PausedAt != 0 || st.DMARewrittenWhilePaused):
		return fmt.Errorf("machine: sbDMA8State 未暫停但暫停欄位不為零（PausedAt=%d、DMARewrittenWhilePaused=%t，docs/spec/445 §2.10）",
			st.PausedAt, st.DMARewrittenWhilePaused)
	case st.DSPBusyIO >= 16:
		return fmt.Errorf("machine: sbDMA8State 的 DSPBusyIO=%d 不小於 16（docs/spec/446 §2.6）", st.DSPBusyIO)
	}
	return nil
}
