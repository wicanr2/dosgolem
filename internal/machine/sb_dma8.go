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

const (
	// sbDMA8StateV 是 sbDMA8State 自帶的版本號（§2.9）。
	sbDMA8StateV = 1
	// stateVersionSB8 是選項開啟時 machineState.Version 寫的值（§2.9）；選項關閉照舊寫 stateVersion（3）。
	stateVersionSB8 = 4
	// sbDMALogMax 是 SBDMALog 的上限（§2.8）。
	sbDMALogMax = 4096
	// sbDMA8Ch 是固定的 8 位元 DMA 通道（§2.1）。
	sbDMA8Ch = 1
)

// sbDMA8 是 §2.2–§2.7 的狀態：主片 8237 暫存器檔、TC 位元、PIC 的 IRQ7 最小模型與進行中的傳輸。
type sbDMA8 struct {
	dma      *DMA8237
	tc       uint8 // 08h 狀態暫存器 bit0–3（DMA8237 沒有 TC 欄位）
	slaveIMR uint8 // A1h，初值 FFh
	irr7     bool
	isr7     bool
	readISR  [2]bool // OCW3 讀取選擇：[0] 主片（20h）、[1] 從片（A0h）；false＝IRR

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
		IRQ7Deliveries: s.irq7Deliveries, IRQ7Defaulted: s.irq7Defaulted, IRR7: s.irr7, ISR7: s.isr7, Active: s.active}
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
	m.sb8 = &sbDMA8{dma: NewDMA8237(), slaveIMR: 0xFF}
	m.wireSB8()
	return nil
}

// wireSB8 把 §2.3 的回呼接到這台機器（SetSBDMA8、Restore、LoadState 都會呼叫）。
// StartAutoDMA、StartSB16AutoDMA、Start16DMA、ControlDMA8 維持 nil。
func (m *Machine) wireSB8() {
	d := &m.soundBlaster
	d.StartDMA = func(n uint32) bool { return m.sb8StartDMA(0x14, n) }
	d.StartADPCMDMA = m.sb8StartDMA
	d.StartSilence = m.sb8StartSilence
	d.CancelDMA = m.sb8Cancel
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
	switch {
	case s.active:
		m.sb8Fail("傳輸進行中又收到啟動命令 %02Xh", cmd)
		return false
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
	if m.sb8.active {
		m.sb8Fail("傳輸進行中又收到啟動命令 80h")
		return false
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
		if den, ok = mulU64(r, 264, uint64(m.PITDivisor())); !ok {
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

// sb8Now 是傳輸所用時鐘的現在值。
func (m *Machine) sb8Now() uint64 {
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

// sb8Cancel 是 DSP 重設的 CancelDMA（§2.3）：進行中的傳輸或靜音取消，DMA 依進度寫回暫存器、不設 TC；
// IRR7 若尚未被 CPU 接受則清除。
func (m *Machine) sb8Cancel() {
	s := m.sb8
	if s.active {
		k := uint32(0)
		if s.cmd != 0x80 {
			k = m.sb8Progress()
			s.dma.Current[2] = s.addr + uint16(k)
			s.dma.Current[3] = uint16(uint32(s.count) - k)
		}
		e := m.sb8Event("cancel", true, true)
		e.Transferred = k
		m.sb8Log(e)
		s.active = false
	}
	s.irr7 = false
}

// ---- §2.2、§2.6 埠 ------------------------------------------------------------

// sb8In 處理選項開啟時的讀取：00h–08h、20h、A0h、A1h、頁暫存器與 22Eh。回 false 表示交回既有路徑。
func (m *Machine) sb8In(port uint16) (uint8, bool) {
	s := m.sb8
	switch port {
	case 0, 1, 2, 3, 4, 5, 6, 7:
		return m.sb8ReadDMAReg(port), true
	case 0x08:
		v := s.tc & 0x0F // bit4–7 為 0（dma.cpp:388-397），讀後清 TC
		s.tc = 0
		return v, true
	case 0x20:
		var v uint8
		if s.readISR[0] {
			if s.isr7 {
				v |= 0x80
			}
		} else {
			if m.irq0Pending {
				v |= 0x01
			}
			if s.irr7 {
				v |= 0x80
			}
		}
		return v, true
	case 0xA0:
		return 0, true
	case 0xA1:
		return s.slaveIMR, true
	case 0x87, 0x83, 0x81, 0x82:
		if v, ok := s.dma.In8(port); ok {
			return v, true
		}
		return 0, false // 未寫過的頁暫存器維持現況（FFh）
	case 0x22E:
		v, _ := m.soundBlaster.In8(port) // 清 IRQPending
		s.irr7 = false                   // 尚未被 CPU 接受的 IRR7 一併清除（sblaster.cpp:3158-3163）
		return v, true
	}
	return 0, false
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
	if s.active && s.cmd != 0x80 && (port == 2 || port == 3) {
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
	xfer := s.active && s.cmd != 0x80 // 靜音不碰 DMA（§2.5 第 1 步）
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
	case 0x20:
		m.sb8PICWrite(0, v)
		return true
	case 0xA0:
		m.sb8PICWrite(1, v)
		return true
	case 0xA1:
		s.slaveIMR = v
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

// sb8PICWrite 是 §2.6 的 20h（i=0）與 A0h（i=1）寫入。
func (m *Machine) sb8PICWrite(i int, v uint8) {
	s := m.sb8
	port := [2]uint16{0x20, 0xA0}[i]
	switch {
	case v&0x10 != 0:
		m.sb8Fail("PIC %02Xh 寫入 ICW1（%02Xh）：不支援重新程式化 PIC", port, v)
	case v&0x18 == 0x08: // OCW3
		if i == 0 && v&0x44 != 0 {
			m.sb8Fail("PIC 20h 寫入 OCW3 %02Xh：poll 或特殊遮罩未建模", v)
			return
		}
		switch v & 3 {
		case 2:
			s.readISR[i] = false
		case 3:
			s.readISR[i] = true
		}
	default: // OCW2
		switch v & 0xE0 {
		case 0x00, 0x40:
		case 0x20: // 非特定 EOI
			if i == 0 {
				m.sb8EOI()
			}
		case 0x60: // 特定 EOI
			if i == 0 && v&7 == 7 {
				m.sb8EOI()
			}
		default:
			m.sb8Fail("PIC %02Xh 寫入 OCW2 %02Xh：旋轉或設定優先序未建模", port, v)
		}
	}
}

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
func (m *Machine) sb8IRQ7Ready() bool {
	s := m.sb8
	return s != nil && s.irr7 && !s.isr7 && m.picMask&0x80 == 0
}

// sb8IRQ7 是 Step 的 IRQ7 分支（鍵盤分支之後、Steps++ 之前），§2.7 第 1–7 步。
//
// 第 4 步（分段模式）的 segWait 在第 1 步成立後、第 2、3 步之前呼叫，與既有鍵盤分支相同：segWait 自己判斷 IF，
// IF=0 的步也要呼叫才會把計數歸零（434 §5.3 的「連續」）。分段模式下 eventThisStep 不會為真、也不遞送，
// 所以遞送結果與規格的條列次序相同。
func (m *Machine) sb8IRQ7() {
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
		m.sb8Log(m.sb8Event("defaulted", false, false))
		return
	}
	if m.ivtGuardIRQ(0x0F) {
		return
	}
	s.irr7, s.isr7 = false, true
	s.irq7Deliveries++
	m.eventThisStep = true
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

// sb8Event 組一筆觀測。inInsn：事件發生在指令執行中（start、cancel、eoi），CS:IP 取本道指令起點；
// 否則（complete、irq7、defaulted）取步間的 CS:IP。xfer：帶入傳輸欄位（start、complete、cancel）。
func (m *Machine) sb8Event(kind string, inInsn, xfer bool) SBDMAEvent {
	s := m.sb8
	dsp := &m.soundBlaster
	e := SBDMAEvent{Kind: kind, Step: m.Steps, Cycles: m.CPU.Cycles, CPUMode: m.sb8Mode(),
		CS: m.CPU.Seg[cpu.CS], IP: m.CPU.IP,
		RateNum: dsp.RateNumerator, RateDen: dsp.RateDenominator, TimeConstant: dsp.TimeConstant,
		TCKnown: dsp.TimeConstantKnown, SpeakerOn: dsp.SpeakerOn,
		IRQ0Every: m.IRQ0Every, PITDiv: m.PITDivisor(), Clock: "steps"}
	if inInsn {
		e.CS, e.IP = m.insnCS, m.insnIP
	}
	cyc := m.CycleClock
	if xfer {
		cyc = s.cycles
		e.Command, e.Length, e.Samples, e.Due = s.cmd, s.n, s.samples, s.due
		e.Format, _ = sbFormat(s.cmd)
		if s.cmd != 0x80 && kind != "cancel" {
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

		PICMask: m.picMask, SlaveIMR: s.slaveIMR, IRR7: s.irr7, ISR7: s.isr7, ReadISR: s.readISR,

		Active: s.active, Cmd: s.cmd, N: s.n, Samples: s.samples, Start: s.start, Due: s.due, CycleUnit: s.cycles,
		Addr: s.addr, Count: s.count,

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
	s.slaveIMR, s.irr7, s.isr7, s.readISR = st.SlaveIMR, st.IRR7, st.ISR7, st.ReadISR

	s.active, s.cmd, s.n, s.samples = st.Active, st.Cmd, st.N, st.Samples
	s.start, s.due, s.cycles = st.Start, st.Due, st.CycleUnit
	s.addr, s.count = st.Addr, st.Count

	s.dma8Completions, s.silenceCompletions = st.DMA8Completions, st.SilenceCompletions
	s.irq7Deliveries, s.irq7Defaulted = st.IRQ7Deliveries, st.IRQ7Defaulted
	m.wireSB8()
}
