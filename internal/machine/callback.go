package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// 從外面插進去的遠呼叫（`docs/spec/009` §3）。
//
// 真機上這種東西是別人的 ISR 在呼叫程式登記的回呼：滑鼠驅動的事件常式、
// 音效驅動的時鐘回呼都是。**ISR 會把暫存器全部存起來**，我們插進去的時候
// 一樣要做——否則被打斷的那段程式會莫名其妙拿到別的 `AX`，
// 而症狀出現在很後面，完全不指向這裡。

// CallbackRetOff 是返回哨兵在 StubSeg 裡的位移，內容是 `CD F3`。
// **不能與字型 stub 或 mouseStubOff 撞。**
const CallbackRetOff = 0x30

// IntCallbackReturn 是返回哨兵用的中斷號。真機沒有這一支。
const IntCallbackReturn = 0xF3

// QueuedCall 是一次要插進去的遠呼叫，含要先擺好的暫存器。
type QueuedCall struct {
	Seg, Off               uint16
	AX, BX, CX, DX, SI, DI uint16
}

// callbackFrame 是被打斷的那一刻的完整 CPU 狀態。
type callbackFrame struct {
	regs  [8]uint16
	segs  [4]uint16
	ip    uint16
	flags uint16
}

// QueueCallback 排一次遠呼叫。**排隊不丟**——丟掉的話快速移動的軌跡
// 會少幾格，而畫面看起來完全正常。
func (m *Machine) QueueCallback(q QueuedCall) {
	m.cbQueue = append(m.cbQueue, q)
}

// CallbackPending 回還有幾次沒送出去。
func (m *Machine) CallbackPending() int { return len(m.cbQueue) }

// CallbacksMade 是已經送出去幾次。**收工前看一眼**：0 次與「遊戲不看事件」
// 長得一模一樣。
func (m *Machine) CallbacksMade() uint64 { return m.cbMade }

// installCallbackStub 在 StubSeg 種返回哨兵。New 會叫它。
func (m *Machine) installCallbackStub() {
	base := uint32(StubSeg) * 16
	m.Mem[base+CallbackRetOff] = 0xCD
	m.Mem[base+CallbackRetOff+1] = IntCallbackReturn
}

// startCallback 把佇列最前面那一筆送出去。已經有一個在跑就不動。
func (m *Machine) startCallback() bool {
	if m.cbActive || len(m.cbQueue) == 0 {
		return false
	}
	q := m.cbQueue[0]
	m.cbQueue = m.cbQueue[1:]

	c := m.CPU
	m.cbSaved = callbackFrame{regs: c.R, segs: c.Seg, ip: c.IP, flags: c.Flags}
	m.cbActive = true
	m.cbMade++

	// 返回位址指到哨兵，不是指回原處——**暫存器要一起還原**，
	// 所以回程一定要經過我們手上。
	c.FarCallWithReturn(q.Seg, q.Off, StubSeg, CallbackRetOff)
	c.R[cpu.AX], c.R[cpu.BX] = q.AX, q.BX
	c.R[cpu.CX], c.R[cpu.DX] = q.CX, q.DX
	c.R[cpu.SI], c.R[cpu.DI] = q.SI, q.DI
	return true
}

// FinishCallback 把狀態整份還原。由返回哨兵的 `int F3h` 呼叫。
//
// 回 false 表示「根本沒有回呼在跑」——那是個 bug 的訊號，不要吞掉。
func (m *Machine) FinishCallback() bool {
	if !m.cbActive {
		return false
	}
	c := m.CPU
	c.R, c.Seg, c.IP = m.cbSaved.regs, m.cbSaved.segs, m.cbSaved.ip
	c.SetFlags(m.cbSaved.flags)
	m.cbActive = false
	return true
}
