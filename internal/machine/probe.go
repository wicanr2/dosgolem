package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 觀測用的三樣東西：寫入監看點、執行中斷點、跑到條件成立。
//
// 這一層是給「拿原版當對照」的工作用的。沒有它，使用端只能自己寫
// 逐條 Step 的迴圈再輪詢狀態——而輪詢會漏掉「變過又變回來」，
// 那種漏法不會報錯，只會讓某些事件看起來從沒發生過。

// WriteHook 是一次被監看到的位元組寫入。addr 是線性位址。
//
// 在寫入**當下**呼叫。這時 `m.CPU.IP` 已經被解碼推過這條指令了，
// 所以「誰寫的」要問 Machine.Insn，不是問 IP。
type WriteHook func(m *Machine, addr uint32, old, new uint8)

// WordHook 是一個被監看的 word 真的變了。
type WordHook func(m *Machine, addr uint32, old, new uint16)

type writeWatch struct {
	id     int
	lo, hi uint32
	on     WriteHook
}

type wordWatch struct {
	id   int
	addr uint32
	last uint16
	on   WordHook
}

// WatchWrite 監看 [lo, hi] 這段位址的每一次位元組寫入。回傳給 Unwatch 用的 id。
//
// **寫入相同的值也算一次寫入**——「誰在寫這個位址」與「這個值變了沒」
// 是兩個不同的問題，要後者用 WatchWord。
func (m *Machine) WatchWrite(lo, hi uint32, on WriteHook) int {
	if on == nil || hi < lo {
		return 0
	}
	m.nextProbe++
	m.writeWatches = append(m.writeWatches, writeWatch{m.nextProbe, lo, hi, on})
	return m.nextProbe
}

// WatchWord 監看一個 word，**值真的變了**才通知，而且是在一條指令做完之後。
//
// 不在寫入當下通知，是因為 16 位元的存放在這台機器上是兩次位元組寫入——
// 當下看會先看到半個新值，那不是任何程式看得到的狀態。
func (m *Machine) WatchWord(addr uint32, on WordHook) int {
	if on == nil {
		return 0
	}
	m.syncWordXlat()
	m.nextProbe++
	m.wordWatches = append(m.wordWatches, wordWatch{m.nextProbe, addr, m.sampleWord(addr), on})
	return m.nextProbe
}

// sampleWord 讀被監看的 word（`docs/spec/432-linear-memory-and-paging` §6、§10 第 8 項）。
// 轉換開啟時用 Peek16，位址是線性視角，與讀寫監看（實體位址）不同；
// 轉換關閉時維持 Read16（Peek 對平面 VGA 視窗與讀取監看的行為不同，改了會讓轉換關閉的路徑變動）。
func (m *Machine) sampleWord(addr uint32) uint16 {
	if m.xlat {
		return m.Peek16(addr)
	}
	return m.Read16(addr)
}

// syncWordXlat 在 m.xlat 與上次取樣時不同時，重新取樣所有被監看的 word 並記下新的 xlat，
// 不發變化通知（轉換開啟前後位址空間的視角不同，那不是客體寫入造成的變化）。
// 回傳是否重新取樣過。
func (m *Machine) syncWordXlat() bool {
	if m.xlat == m.wordXlat {
		return false
	}
	m.wordXlat = m.xlat
	for i := range m.wordWatches {
		m.wordWatches[i].last = m.sampleWord(m.wordWatches[i].addr)
	}
	return true
}

// Unwatch 拿掉一個監看點。id 不存在就什麼都不做。
func (m *Machine) Unwatch(id int) {
	for i := range m.writeWatches {
		if m.writeWatches[i].id == id {
			m.writeWatches = append(m.writeWatches[:i], m.writeWatches[i+1:]...)
			return
		}
	}
	for i := range m.wordWatches {
		if m.wordWatches[i].id == id {
			m.wordWatches = append(m.wordWatches[:i], m.wordWatches[i+1:]...)
			return
		}
	}
}

// BreakAt 在 seg:off 設一個執行中斷點，回傳給 ClearBreak 用的 id。
//
// 位址是**線性**比對的，所以 `1234:0005` 與 `1230:0045` 是同一個點——
// 8086 上那本來就是同一個位元組。
func (m *Machine) BreakAt(seg, off uint16) int {
	m.nextProbe++
	if m.breaks == nil {
		m.breaks = map[int]uint32{}
	}
	m.breaks[m.nextProbe] = uint32(seg)*16 + uint32(off)
	return m.nextProbe
}

// AddLinearBreak 以線性位址設一個執行中斷點（434 §7）：分段模式的位址（可大於 1 MB）只能經這裡設定。
// 比對見 atBreak：分段模式比 PCLinear()，否則比 cpu.Addr(CS, IP)。
func (m *Machine) AddLinearBreak(addr uint32) int {
	m.nextProbe++
	if m.breaks == nil {
		m.breaks = map[int]uint32{}
	}
	m.breaks[m.nextProbe] = addr
	return m.nextProbe
}

// ClearBreak 拿掉一個中斷點。
func (m *Machine) ClearBreak(id int) { delete(m.breaks, id) }

// Stop 說明 RunUntil 為什麼停下來。
type Stop uint8

const (
	// StopBudget ＝ 指令預算用完了。**這通常表示條件根本不會成立**，
	// 不是「還差一點」——把預算調大之前先想想為什麼。
	StopBudget Stop = iota
	// StopPredicate ＝ 條件成立。
	StopPredicate
	// StopBreakpoint ＝ 走到中斷點，**還沒執行那條指令**。
	StopBreakpoint
)

func (s Stop) String() string {
	switch s {
	case StopBudget:
		return "預算用完"
	case StopPredicate:
		return "條件成立"
	case StopBreakpoint:
		return "碰到中斷點"
	}
	return fmt.Sprintf("Stop(%d)", uint8(s))
}

// PreStepObserver is called at the current CS:IP immediately before a Step
// attempt.  It is a DRAFT generic observation seam: callers may inspect the
// machine but must not assume an oracle, a game-specific address map, or a
// frontend-owned lifetime.  Returning an error prevents that Step attempt and
// is returned with StopBudget, matching RunUntil's existing error shape.
type PreStepObserver func(*Machine) error

// RunUntil 一直跑，直到條件成立、走到中斷點、或用完 budget 條指令。
//
// stop 可以是 nil（只看中斷點）。條件與中斷點都在**執行每一條之前**檢查，
// 但**跳過第一條**——否則停在中斷點上之後再呼叫一次會當場又停下來，
// 永遠走不動。
func (m *Machine) RunUntil(stop func(*Machine) bool, budget uint64) (Stop, error) {
	for i := uint64(0); i < budget; i++ {
		if i > 0 || len(m.breaks) == 0 {
			if m.atBreak() {
				return StopBreakpoint, nil
			}
		}
		if i > 0 && stop != nil && stop(m) {
			return StopPredicate, nil
		}
		if err := m.Step(); err != nil {
			return StopBudget, err
		}
	}
	if stop != nil && stop(m) {
		return StopPredicate, nil
	}
	return StopBudget, nil
}

// RunUntilObserved is RunUntil with a DRAFT pre-step observation seam.
//
// The observer is called after the same breakpoint/predicate checks that can
// stop this iteration and immediately before each Step attempt.  This gives a
// sealed future owner the same pre-instruction observation point as
// oracle.OnCall without importing oracle or changing RunUntil's visible
// behavior.  A nil observer is equivalent to RunUntil for stop/error/step
// behavior.  It does not install an observer, route input, or expose any
// owner resource to a frontend.
func (m *Machine) RunUntilObserved(stop func(*Machine) bool, budget uint64, observe PreStepObserver) (Stop, error) {
	for i := uint64(0); i < budget; i++ {
		if i > 0 || len(m.breaks) == 0 {
			if m.atBreak() {
				return StopBreakpoint, nil
			}
		}
		if i > 0 && stop != nil && stop(m) {
			return StopPredicate, nil
		}
		if observe != nil {
			if err := observe(m); err != nil {
				return StopBudget, err
			}
		}
		if err := m.Step(); err != nil {
			return StopBudget, err
		}
	}
	if stop != nil && stop(m) {
		return StopPredicate, nil
	}
	return StopBudget, nil
}

func (m *Machine) atBreak() bool {
	if len(m.breaks) == 0 {
		return false
	}
	// 434 §7：分段模式用 PCLinear()；否則維持 cpu.Addr（20 位元遮罩）不變。
	at := cpu.Addr(m.CPU.Seg[cpu.CS], m.CPU.IP)
	if m.CPU.Segmented() {
		at = m.PCLinear()
	}
	for _, a := range m.breaks {
		if a == at {
			return true
		}
	}
	return false
}

// Insn 是目前這一條指令的起點（`Step` 進來時的 CS:IP）。分段模式下照舊回「選擇子:偏移」；
// 需要線性位址者用 InsnLinear()（434 §7）。
//
// 監看點觸發時 `CPU.IP` 已經被解碼推過去了，所以「是誰寫的」要問這裡。
// 差別在有運算元的指令上會到好幾個位元組——而那正是要回去對碼的位址。
func (m *Machine) Insn() (seg, off uint16) { return m.insnCS, m.insnIP }

// noteWrite 是 Write8 的監看鉤子。呼叫端已經確認有監看點。
func (m *Machine) noteWrite(a uint32, old, v uint8) {
	for _, w := range m.writeWatches {
		if a >= w.lo && a <= w.hi {
			w.on(m, a, old, v)
		}
	}
}

// pollWords 在每一條指令做完之後檢查被監看的 word。
func (m *Machine) pollWords() {
	if m.syncWordXlat() {
		return
	}
	for i := range m.wordWatches {
		w := &m.wordWatches[i]
		if now := m.sampleWord(w.addr); now != w.last {
			old := w.last
			w.last = now
			w.on(m, w.addr, old, now)
		}
	}
}
