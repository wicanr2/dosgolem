// Package mixer 把 machine 的 OPL、喇叭與 PIT 通道 2 事件依步數合成成
// 48 kHz 立體聲（`docs/spec/240` §3.4–3.5）。
//
// 時間基準是模擬步數：樣本序號 = 絕對步數 × 取樣率 ÷ 每秒步數，
// 以整數換算，不累積誤差。事件只收集、不在 Step 內合成；Render 在
// 回合結束後依本回合的步數區間產生樣本。
package mixer

import (
	"github.com/wicanr2/dosgolem/audio/nukedopl"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// SampleRate 是輸出取樣率。
const SampleRate = 48000

// pitHz 是 PIT 的輸入時脈。
const pitHz = 1193182

// speakerAmp 是喇叭相對滿刻度的振幅。
const speakerAmp = 0.2

// speakerLowpass 是一階低通的係數：1-exp(-2π·6000/48000)，截止約 6 kHz。
// 純方波直接輸出高頻刺耳；實體喇叭本身就會衰減高頻。
const speakerLowpass = 0.5447

type eventKind uint8

const (
	evOPL eventKind = iota
	evSpeaker
	evPIT2
)

type event struct {
	step uint64
	kind eventKind
	opl  machine.OPLWrite
	spk  machine.SpeakerSample
	pit  machine.PIT2Change
}

// Mixer 實作 session.AudioObserver，並在 Render 時產生樣本。
// 只能在同一個 goroutine 使用。
type Mixer struct {
	stepsPerSecond uint64
	chip           *nukedopl.Chip
	newm           bool
	events         []event
	next           uint64 // 下一個要產生的絕對樣本序號
	started        bool

	gate, data uint8
	divisor    uint32
	phase      float64 // 方波相位，單位為週期
	dcX, dcY   float64 // 喇叭的隔直濾波狀態
	lp         float64 // 喇叭的一階低通狀態（近似喇叭紙盆的高頻衰減）

	opl [2]int16
}

// New 建一個混音器。stepsPerSecond 是前端推進速率（每秒步數）。
func New(stepsPerSecond uint64) *Mixer {
	return &Mixer{stepsPerSecond: stepsPerSecond, chip: nukedopl.New(SampleRate)}
}

// OPLWrite 收一筆 OPL 寫入。
func (m *Mixer) OPLWrite(w machine.OPLWrite) {
	m.events = append(m.events, event{step: w.Step, kind: evOPL, opl: w})
}

// SpeakerSample 收一次喇叭資料線變化。
func (m *Mixer) SpeakerSample(s machine.SpeakerSample) {
	m.events = append(m.events, event{step: s.Step, kind: evSpeaker, spk: s})
}

// PITChannel2 收一次通道 2 重載。
func (m *Mixer) PITChannel2(c machine.PIT2Change) {
	m.events = append(m.events, event{step: c.Step, kind: evPIT2, pit: c})
}

// sampleOf 是步數對應的絕對樣本序號（無條件捨去）。
func (m *Mixer) sampleOf(step uint64) uint64 {
	// 步數 × 48000 在 uint64 內可到約 3.8×10^14 步，遠超過實際執行量。
	return step * SampleRate / m.stepsPerSecond
}

// oplAddr 依 DOSBox-X adlib.cpp:379–386 把 machine 的寫入換成 Nuked 位址；
// ok 為 false 表示不送合成器（計時器暫存器）。
func (m *Mixer) oplAddr(w machine.OPLWrite) (addr uint16, ok bool) {
	addr = uint16(w.Reg)
	if w.Bank == 1 && (w.Reg == 0x05 || m.newm) {
		addr |= 0x100
	}
	switch addr {
	case 0x02, 0x03, 0x04:
		return 0, false
	case 0x105:
		m.newm = w.Val&1 != 0
	}
	return addr, true
}

func (m *Mixer) apply(e event) {
	switch e.kind {
	case evOPL:
		if addr, ok := m.oplAddr(e.opl); ok {
			m.chip.WriteRegBuffered(addr, e.opl.Val)
		}
	case evSpeaker:
		m.gate, m.data = e.spk.Gate, e.spk.Level
	case evPIT2:
		m.divisor = e.pit.Divisor
	}
}

// speaker 回下一個喇叭樣本（隔直後），並推進方波相位。
func (m *Mixer) speaker() float64 {
	var level float64
	if m.gate != 0 {
		if m.divisor != 0 {
			freq := float64(pitHz) / float64(m.divisor)
			if freq <= SampleRate/2 {
				if m.data != 0 && m.phase < 0.5 {
					level = 1
				}
				m.phase += freq / SampleRate
				m.phase -= float64(int(m.phase))
			}
		}
	} else if m.data != 0 {
		level = 1
	}
	m.lp += speakerLowpass * (level*speakerAmp - m.lp)
	x := m.lp
	y := x - m.dcX + 0.995*m.dcY
	m.dcX, m.dcY = x, y
	return y
}

// Render 產生步數區間 [before, after) 的樣本，附加到 dst 後回傳
// （交錯立體聲 float32）。before 與上次的 after 不連續時（例如第一次），
// 樣本序號從 before 重新起算，不補中間的樣本。
func (m *Mixer) Render(dst []float32, before, after uint64) []float32 {
	start := m.sampleOf(before)
	if !m.started || start > m.next {
		m.next, m.started = start, true
	}
	end := m.sampleOf(after)
	ev := m.events
	for ; m.next < end; m.next++ {
		for len(ev) > 0 && m.sampleOf(ev[0].step) <= m.next {
			m.apply(ev[0])
			ev = ev[1:]
		}
		m.chip.GenerateStream(m.opl[:])
		s := m.speaker()
		dst = append(dst, clamp(float64(m.opl[0])/32768+s), clamp(float64(m.opl[1])/32768+s))
	}
	// 這一段尚未輪到的事件（理論上不會有：事件步數都小於 after）照樣套用，
	// 讓暫存器狀態不落後。
	for _, e := range ev {
		m.apply(e)
	}
	m.events = m.events[:0]
	return dst
}

func clamp(v float64) float32 {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return float32(v)
}
