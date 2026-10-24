package session

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// StepObserver watches a sealed session one instruction at a time
// (docs/spec/238).  It only ever receives a read-only StepView and value-type
// video writes; it can neither mutate nor retain the owned machine.
type StepObserver interface {
	BeforeStep(v StepView) error
	VideoWrite(w machine.VideoWrite)
	// Frame is called on each vertical retrace with copies of the indexed
	// frame and palette; mutating them does not touch the machine.
	Frame(indexed []byte, palette [256][3]uint8)
}

// AudioObserver is an optional extension of StepObserver (docs/spec/240
// §3.3).  It is only called inside a turn; callbacks collect value-type
// writes with their step and must not synthesise inside Step.
type AudioObserver interface {
	OPLWrite(w machine.OPLWrite)
	SpeakerSample(s machine.SpeakerSample)
	// PITChannel2 carries the speaker square-wave divisor on the same step
	// timeline as SpeakerSample (§3.2).
	PITChannel2(c machine.PIT2Change)
}

// ModeObserver is an optional extension of StepObserver (Buck spec 052): it is
// told every video mode set (int 10h AH=00), which no A000 write observer can
// see.  The callback runs inside a Step; it must not change execution state
// and must not read the palette (the BIOS loads it after the call).
type ModeObserver interface {
	VideoModeChange(c machine.ModeChange)
}

// SBDMAObserver 是 StepObserver 的選配擴充（docs/spec/452）：轉發 SB16 8 位元 DMA 的每一筆觀測事件
// （machine.SBDMAEvent，444 §2.8、445 §2.9），全部 Kind 照原值、不過濾。回呼在步驟嘗試之內呼叫，
// 沒有開著的 StepView；需要讀記憶體的消費端在下一次 BeforeStep 讀（452 §2.4）。
type SBDMAObserver interface {
	SBDMA(e machine.SBDMAEvent)
}

// IPSet 是 65536 位元的 IP 集合（docs/spec/460 §3.1）：bit ip 為 1 表示 IP == ip 的步驟要觀察。
type IPSet [1024]uint64

// Add 把 ip 加進集合。
func (s *IPSet) Add(ip uint16) { s[ip>>6] |= 1 << (ip & 63) }

// Has 回 ip 是否在集合內。
func (s *IPSet) Has(ip uint16) bool { return s[ip>>6]&(1<<(ip&63)) != 0 }

// Union 把 o 併進 s。
func (s *IPSet) Union(o *IPSet) {
	for i := range s {
		s[i] |= o[i]
	}
}

// NoWake 是 SparseObserver.NextWake 的「沒有期限」。
const NoWake = ^uint64(0)

// SparseObserver 是 StepObserver 的選配擴充（docs/spec/460）。Config.SparseObserve 為真且觀察器實作它時，
// session 只在 460 §3.3 的步驟呼叫 BeforeStep；否則照 238 每步呼叫。
type SparseObserver interface {
	StepObserver
	// TriggerIPs 回觸發 IP 集合。session 只在每次 runObserved 開始時讀一次並複製；回合內集合不變。
	TriggerIPs() IPSet
	// NextWake 回下一個無論 IP 都要觀察的步數（BeforeStep 時的 Steps() 值）。0 表示下一步；沒有期限回 NoWake。
	// session 只在 BeforeStep 正常返回之後讀。
	NextWake() uint64
	// FrameAt 在每次 Frame 之前呼叫，steps 是回掃當下的 machine.Steps（460 §2.4）。
	FrameAt(steps uint64)
}

// muteAudio keeps m.OPL/m.Speaker from growing while AdLib is on and no
// AudioObserver is forwarding (§3.1): the empty collectors drop every write.
func (o *Owner) muteAudio() {
	o.machine.ObserveOPLWrites(func(machine.OPLWrite) {})
	o.machine.ObserveSpeaker(func(machine.SpeakerSample) {})
	o.machine.ObservePITChannel2(nil)
}

// StepView is valid only inside the BeforeStep call it was passed to.  A
// retained view reads zero and makes the next BeforeStep an ObserverFault.
type StepView struct {
	o     *Owner
	token uint64
}

func (v StepView) live() bool {
	if v.o == nil {
		return false
	}
	if !v.o.viewOpen || v.o.viewToken != v.token {
		v.o.viewMisuse = true
		v.o.filter.Force = true // 460 §3.3：稀疏模式下一步必觀察，錯誤照逐步模式在下一步回報
		return false
	}
	return true
}

func (v StepView) reg(r int) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.R[r]
}

func (v StepView) seg(s int) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.Seg[s]
}

func (v StepView) Steps() uint64 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Steps
}
func (v StepView) CS() uint16 { return v.seg(cpu.CS) }
func (v StepView) DS() uint16 { return v.seg(cpu.DS) }
func (v StepView) ES() uint16 { return v.seg(cpu.ES) }
func (v StepView) SS() uint16 { return v.seg(cpu.SS) }
func (v StepView) IP() uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.IP
}
func (v StepView) AX() uint16 { return v.reg(cpu.AX) }
func (v StepView) BX() uint16 { return v.reg(cpu.BX) }
func (v StepView) CX() uint16 { return v.reg(cpu.CX) }
func (v StepView) DX() uint16 { return v.reg(cpu.DX) }
func (v StepView) SP() uint16 { return v.reg(cpu.SP) }
func (v StepView) BP() uint16 { return v.reg(cpu.BP) }
func (v StepView) SI() uint16 { return v.reg(cpu.SI) }
func (v StepView) DI() uint16 { return v.reg(cpu.DI) }
func (v StepView) Flags() uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.CPU.Flags
}

// Palette returns a copy of the current DAC palette.
func (v StepView) Palette() [256][3]uint8 {
	if !v.live() {
		return [256][3]uint8{}
	}
	return v.o.machine.Palette()
}

// Read8 and Read16 never have side effects (machine.Peek8).
func (v StepView) Read8(a uint32) uint8 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Peek8(a)
}
func (v StepView) Read16(a uint32) uint16 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Peek16(a)
}

// Deliveries 回 machine.Deliveries()（docs/spec/454 §2.1）。保留的視圖回 0 並設 viewMisuse。
func (v StepView) Deliveries() uint64 {
	if !v.live() {
		return 0
	}
	return v.o.machine.Deliveries()
}

// ReadBlock 只做一次 live() 檢查，之後呼叫 machine.PeekBlock（docs/spec/454 §2.2）：
// 線性位址 a 起 len(dst) bytes，與逐位元組 Read8 相同、沒有副作用；ok 為假表示範圍內有頁不存在。
// 保留的視圖把 dst 清成全 0、回 false 並設 viewMisuse。
func (v StepView) ReadBlock(a uint32, dst []byte) (ok bool) {
	if !v.live() {
		clear(dst)
		return false
	}
	return v.o.machine.PeekBlock(a, dst)
}

// IndexedLayout 回 machine.IndexedLayout()（docs/spec/458 §2.2）；保留的視圖回零值並設 viewMisuse。
func (v StepView) IndexedLayout() machine.IndexedLayout {
	if !v.live() {
		return machine.IndexedLayout{}
	}
	return v.o.machine.IndexedLayout()
}

// IndexedInto 只做一次 live() 檢查，之後呼叫 machine.IndexedInto（docs/spec/458 §2.2）：把實體 VGA 的畫面色號
// 寫進 dst[:W*H]，不經線性層與頁表。保留的視圖不寫 dst、回零值與 ok=false 並設 viewMisuse。
// 消費端以 *StepView 呼叫，以 IndexedLayout() 配緩衝一次、之後重用。
func (v StepView) IndexedInto(dst []byte) (machine.IndexedLayout, bool) {
	if !v.live() {
		return machine.IndexedLayout{}, false
	}
	return v.o.machine.IndexedInto(dst)
}

// errProgramExited stops the observed loop at the first instruction after the
// program has exited.  It is not an error of the program or the observer.
var errProgramExited = errors.New("session: 程式已結束")

// runObserved is Advance's observer path.  Faults from the observer are
// returned as observerErr so Advance can classify them apart from original
// program faults.
func (o *Owner) runObserved(budget uint64) (stop machine.Stop, rawErr, observerErr error) {
	// 460：稀疏模式只在 Config.SparseObserve 為真且觀察器實作 SparseObserver 時啟用；否則以下照 238 每步呼叫。
	// filter 的 Force 在稀疏模式之外不被讀取，所以各包裝無條件設定也不改變逐步模式的行為。
	sp, sparse := o.observer.(SparseObserver)
	sparse = sparse && o.sparse
	f := &o.filter
	// videoPanic latches a panic raised inside a Step (video write or
	// frame callback); the Step completes and the next attempt stops.
	var videoPanic error
	o.machine.ObserveVideoWrites(func(w machine.VideoWrite) {
		if videoPanic != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				videoPanic = fmt.Errorf("session: 觀測器 VideoWrite panic：%v", r)
				f.Force = true // 460 §3.3：錯誤在下一步回報
			}
		}()
		o.observer.VideoWrite(w)
	})
	o.machine.SetOnFrame(func() {
		if videoPanic != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				videoPanic = fmt.Errorf("session: 觀測器 Frame panic：%v", r)
				f.Force = true
			}
		}()
		if sparse {
			sp.FrameAt(o.machine.Steps) // 460 §3.3：FrameAt → Frame → Force
		}
		o.observer.Frame(o.machine.Indexed(), o.machine.Palette())
		f.Force = true
	})
	// guard latches a panic raised inside a Step by an optional observer
	// extension (audio, video mode); shared by all of them.
	guard := func(name string, fn func()) {
		if videoPanic != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				videoPanic = fmt.Errorf("session: 觀測器 %s panic：%v", name, r)
				f.Force = true
			}
		}()
		fn()
	}
	if mo, ok := o.observer.(ModeObserver); ok {
		o.machine.ObserveModeChanges(func(c machine.ModeChange) {
			guard("VideoModeChange", func() { mo.VideoModeChange(c) })
			f.Force = true // 460 §3.3
		})
	}
	if ao, ok := o.observer.(AudioObserver); ok {
		o.machine.ObserveOPLWrites(func(w machine.OPLWrite) { guard("OPLWrite", func() { ao.OPLWrite(w) }) })
		o.machine.ObserveSpeaker(func(s machine.SpeakerSample) { guard("SpeakerSample", func() { ao.SpeakerSample(s) }) })
		o.machine.ObservePITChannel2(func(c machine.PIT2Change) { guard("PITChannel2", func() { ao.PITChannel2(c) }) })
	}
	// 452 §2.3：只在觀察器實作 SBDMAObserver 時安裝；否則不碰 OnSBDMA，事件照現況進 SBDMALog。
	so, sbdma := o.observer.(SBDMAObserver)
	if sbdma {
		o.machine.OnSBDMA = func(e machine.SBDMAEvent) {
			guard("SBDMA", func() { so.SBDMA(e) })
			f.Force = true // 460 §3.3：消費端在下一次 BeforeStep 讀（452 §2.4）
		}
	}
	defer func() {
		o.machine.ObserveVideoWrites(nil)
		o.machine.ObserveModeChanges(nil)
		if sbdma {
			o.machine.OnSBDMA = nil
		}
		o.machine.SetOnFrame(nil)
		if o.machine.AdLibPresent() {
			o.muteAudio()
		} else {
			o.machine.ObserveOPLWrites(nil)
			o.machine.ObserveSpeaker(nil)
			o.machine.ObservePITChannel2(nil)
		}
	}()
	inObserver := false
	observe := func(m *machine.Machine) error {
		// The receipt runner stops as soon as the program exits; without this
		// check the CPU would keep stepping undefined memory after exit.
		if o.dos.Exited {
			return errProgramExited
		}
		if videoPanic != nil {
			observerErr = videoPanic
			return videoPanic
		}
		if o.viewMisuse {
			observerErr = errors.New("session: 觀測器在回呼外使用了 StepView")
			return observerErr
		}
		o.viewToken++
		o.viewOpen = true
		inObserver = true
		e := o.observer.BeforeStep(StepView{o: o, token: o.viewToken})
		inObserver = false
		o.viewOpen = false
		if e != nil {
			observerErr = fmt.Errorf("session: 觀測器 BeforeStep：%w", e)
			return observerErr
		}
		if sparse {
			f.Wake = sp.NextWake() // 460 §3.1：只在 BeforeStep 正常返回之後讀
		}
		return nil
	}
	if sparse {
		// 460 §3.3：回合開始時複製集合、期限歸零、第一步必觀察（回合之間呼叫端可能改過觀察器）。
		f.IPs = sp.TriggerIPs()
		f.Wake, f.Force, f.Exited = NoWake, true, &o.dos.Exited
	}
	// A BeforeStep panic happens before that instruction's Step, so unwinding
	// the whole loop here leaves the machine exactly where it was.  Recovering
	// once around the loop instead of once per step keeps the hot path cheap.
	func() {
		defer func() {
			if r := recover(); r != nil {
				if !inObserver {
					panic(r) // not the observer's panic: never reclassify it
				}
				o.viewOpen = false
				observerErr = fmt.Errorf("session: 觀測器 BeforeStep panic：%v", r)
				stop, rawErr = machine.StopBudget, observerErr
			}
		}()
		if sparse {
			stop, rawErr = o.machine.RunUntilSparse(nil, budget, f, observe)
			return
		}
		stop, rawErr = o.machine.RunUntilObserved(nil, budget, observe)
	}()
	if errors.Is(rawErr, errProgramExited) {
		// Advance classifies the exited DOS as ProgramStopped.
		rawErr = nil
	}
	if observerErr == nil && videoPanic != nil {
		observerErr = videoPanic
	}
	if observerErr == nil && o.viewMisuse {
		observerErr = errors.New("session: 觀測器在回呼外使用了 StepView")
	}
	if observerErr != nil && rawErr == observerErr {
		rawErr = nil
	}
	return stop, rawErr, observerErr
}

// SparseStats 回稀疏模式累計的 BeforeStep 次數與其中因 Force 而呼叫的次數（docs/spec/460 §5.1 第 3 條的觀察呼叫率；
// 逐步模式兩者為 0）。只讀計數，不碰機器。
func (o *Owner) SparseStats() (observed, forced uint64) {
	if o == nil {
		return 0, 0
	}
	return o.filter.Observed, o.filter.Forced
}
