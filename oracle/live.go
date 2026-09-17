package oracle

import (
	"fmt"
	"math"
	"sort"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/opl2"
	"github.com/wicanr2/dosgolem/internal/state"
)

// 即時執行介面（`docs/spec/199`）：給人玩的前端用。
//
// 對拍的時間線是「第幾道指令」；前端的時間線是「這一幀份的機器時間」。
// 兩者在這裡接起來：速度用 DOSBox 相容 cycles（`198`），按鍵是「現在按下／現在放開」，
// 音訊一段一段取。

// SetDOSBoxCycles 以 DOSBox 的扣法與速度跑（`198` §3.2）；0 還原指令數時鐘。
func (o *Oracle) SetDOSBoxCycles(perMs uint64) { o.m.SetDOSBoxCycles(perMs) }

// DOSBoxCycles 回每毫秒 cycles；沒開時回 0。
func (o *Oracle) DOSBoxCycles() uint64 { return o.m.DOSBoxCycles() }

// Cycles 回目前累計的 cycles。
func (o *Oracle) Cycles() uint64 { return o.m.CPU.Cycles }

// SetAdLib 設定 388h 上有沒有 OPL2。
func (o *Oracle) SetAdLib(present bool) { o.m.SetAdLib(present) }

// RunCycles 跑到累計 cycles 增加 n 為止（`199` §3.2）。
func (o *Oracle) RunCycles(n uint64) error {
	if o.m.DOSBoxCycles() == 0 {
		return fmt.Errorf("RunCycles 需要先 SetDOSBoxCycles：指令數時鐘沒有機器時間可以對齊")
	}
	o.scheduleRepeats(n)
	target := o.m.CPU.Cycles + n
	cond := Cond{
		name:  fmt.Sprintf("再跑 %d 個 cycle", n),
		ready: func(o *Oracle) bool { return o.m.CPU.Cycles >= target },
	}
	// 每道指令至少一個 cycle，所以 n＋1 道一定夠。
	return o.RunUntil(cond, Budget(n+1))
}

// heldKey 是一個按著的鍵：下一個 typematic 按下碼排在第幾步。
type heldKey struct{ next uint64 }

// KeyDown 在目前步數按下 scan（`199` §3.3）；已經按著就不動。
func (o *Oracle) KeyDown(scan uint8) {
	if o.held == nil {
		o.held = map[uint8]*heldKey{}
	}
	if _, ok := o.held[scan]; ok {
		return
	}
	now := o.m.Steps
	o.m.ScheduleKey(now, scan, false)
	delay := uint64(machine.TypematicDelayMS / 1000 * o.m.InstructionsPerSecond())
	o.held[scan] = &heldKey{next: now + delay}
}

// KeyUp 在目前步數放開 scan；沒按著就不動。之後還沒送出的重複按下碼一併取消。
func (o *Oracle) KeyUp(scan uint8) {
	if _, ok := o.held[scan]; !ok {
		return
	}
	now := o.m.Steps
	o.m.CancelTimedKeys(scan, now)
	o.m.ScheduleKey(now, scan, true)
	delete(o.held, scan)
}

// HeldKeys 回目前按著的鍵，依掃描碼排序。
func (o *Oracle) HeldKeys() []uint8 {
	out := make([]uint8, 0, len(o.held))
	for k := range o.held {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// scheduleRepeats 把落在接下來 n 道指令之內的 typematic 重複排進去。
func (o *Oracle) scheduleRepeats(n uint64) {
	if len(o.held) == 0 {
		return
	}
	period := uint64(o.m.InstructionsPerSecond() / machine.TypematicRate)
	if period == 0 {
		period = 1
	}
	end := o.m.Steps + n
	for _, scan := range o.HeldKeys() {
		h := o.held[scan]
		for ; h.next <= end; h.next += period {
			o.m.ScheduleKey(h.next, scan, false)
		}
	}
}

// LoadStateFile 讀 probe `-save-state` 格式的狀態檔（`199` §3.4）。
// ⚠ 狀態檔含原版的整份記憶體，呼叫端負責不散布。
func (o *Oracle) LoadStateFile(path string) error {
	if err := state.Load(path, o.m, o.d); err != nil {
		return err
	}
	o.held = nil
	return nil
}

// SaveStateFile 寫 probe `-load-state` 讀得懂的狀態檔。⚠ 同上，不要散布。
func (o *Oracle) SaveStateFile(path string) error { return state.Save(path, o.m, o.d) }

// Audio 是串流音訊（`199` §3.5）：每次 Render 回上次到現在這段機器時間的取樣。
type Audio struct {
	o        *Oracle
	rate     int
	tone     *machine.ToneDecoder
	synth    *opl2.Synth
	phase    float64
	lastCyc  uint64
	lastStep uint64
	frac     float64
}

// NewAudio 建立串流音訊；rate ≤ 0 用 44,100。一台機器同時只能有一個。
func (o *Oracle) NewAudio(rate int) *Audio {
	if rate <= 0 {
		rate = 44100
	}
	return &Audio{o: o, rate: rate, tone: machine.NewToneDecoder(), synth: opl2.New(rate),
		lastCyc: o.m.CPU.Cycles, lastStep: o.m.Steps}
}

// Rate 回取樣率。
func (a *Audio) Rate() int { return a.rate }

// audioEvent 是依步數合併後的喇叭或 OPL2 寫入。
type audioEvent struct {
	step     uint64
	port     *machine.PortWrite
	reg, val uint8
}

// Render 回從上次 Render 到現在的單聲道 16 位元取樣，並清掉消化完的埠紀錄與 OPL 紀錄。
func (a *Audio) Render() []int16 {
	m := a.o.m
	nowCyc, nowStep := m.CPU.Cycles, m.Steps
	var secs float64
	if per := m.DOSBoxCycles(); per > 0 {
		secs = float64(nowCyc-a.lastCyc) / float64(per*1000)
	} else {
		secs = float64(nowStep-a.lastStep) / m.InstructionsPerSecond()
	}
	exact := secs*float64(a.rate) + a.frac
	n := int(exact)
	a.frac = exact - float64(n)

	evs := make([]audioEvent, 0, len(m.PortLog)+len(m.OPL))
	for i := range m.PortLog {
		switch m.PortLog[i].Port {
		case 0x42, 0x43, 0x61:
			evs = append(evs, audioEvent{step: m.PortLog[i].Step, port: &m.PortLog[i]})
		}
	}
	adlib := m.AdLib()
	if adlib {
		for _, w := range m.OPL {
			if w.Bank == 0 {
				evs = append(evs, audioEvent{step: w.Step, reg: w.Reg, val: w.Val})
			}
		}
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].step < evs[j].step })
	apply := func(e audioEvent) {
		if e.port != nil {
			a.tone.Feed(*e.port)
		} else {
			a.synth.Write(e.reg, e.val)
		}
	}

	pcm := make([]int16, n)
	j := 0
	span := nowStep - a.lastStep
	for i := range pcm {
		at := a.lastStep
		if n > 0 {
			at += uint64(float64(span) * float64(i) / float64(n))
		}
		for ; j < len(evs) && evs[j].step <= at; j++ {
			apply(evs[j])
		}
		v := 0.0
		if hz := a.tone.Hz(); hz > 0 {
			if a.phase < 0.5 {
				v = 0.25
			} else {
				v = -0.25
			}
			a.phase += hz / float64(a.rate)
			a.phase -= math.Floor(a.phase)
		}
		if adlib {
			v += a.synth.Sample()
		}
		pcm[i] = int16(math.Round(math.Max(-1, math.Min(1, v)) * 32767))
	}
	for ; j < len(evs); j++ {
		apply(evs[j])
	}
	m.PortLog = m.PortLog[:0]
	m.ClearOPL()
	a.lastCyc, a.lastStep = nowCyc, nowStep
	return pcm
}
