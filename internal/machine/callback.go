package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// 從外面插進去的遠呼叫（`docs/spec/009` §3）。
//
// 真機上這種東西是別人的 ISR 在呼叫程式登記的回呼：滑鼠驅動的事件常式、
// 音效驅動的時鐘回呼都是。**ISR 會把暫存器全部存起來**，我們插進去的時候
// 一樣要做——否則被打斷的那段程式會莫名其妙拿到別的 `AX`，
// 而症狀出現在很後面，完全不指向這裡。

// IntCallbackReturn 是返回哨兵用的中斷號。真機沒有這一支。
//
// ⚠ **F2–F6 已經被服務型中斷的 trampoline 佔走**（`machine.initVectors`）：
// F2 ＝ int 21h、F3 ＝ int 10h、F4 ＝ int 15h、F5 ＝ XMS、F6 ＝ EMS。
// 撞號的話回呼結束會被當成一次視訊服務，**而且不會報錯**——
// 回呼永遠不收尾，機器卡在一個回不來的遠呼叫裡。
const IntCallbackReturn = 0xF9

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
	// 80386 暫存器高半與 FS／GS（`docs/spec/197`）：回呼用 32 位元暫存器時，
	// 沒存的話被打斷的程式高半會被改掉，而 16 位元暫存器全對。
	hi         [8]uint16
	fseg, gseg uint16
}

// QueueCallback 排一次遠呼叫。**排隊不丟**——丟掉的話快速移動的軌跡
// 會少幾格，而畫面看起來完全正常。
//
// 459 §3.2：保護模式剖面下佇列上限 cbQueueMax（4096，與 PM 版本 4 的編碼上限相同）。已滿時不排入，
// 設黏著的溢位旗標並記下被拒的筆數；tick 每次進入都回溢位錯誤（失敗即關閉）。實模式剖面不設上限。
func (m *Machine) QueueCallback(q QueuedCall) {
	if m.pmProfile && len(m.cbQueue) >= cbQueueMax {
		m.pic.overflow = true
		m.picd.rejected++
		return
	}
	m.cbQueue = append(m.cbQueue, q)
}

// CallbackPending 回還有幾次沒送出去。
func (m *Machine) CallbackPending() int { return len(m.cbQueue) }

// CallbackActive 回「現在正有一次回呼在跑」。
//
// 同步派送（`dos.RunMouseEvent`）要靠它判斷回呼收工了沒：只看
// CallbacksMade 增加的話，會在回呼**才剛進去**的那一步就以為結束了。
//
// 459 §3.8：滑鼠事件走 IRQ12 時（保護模式剖面）回 UIR（使用者回呼執行中）。
func (m *Machine) CallbackActive() bool {
	if m.MouseIRQ12Active() {
		return m.pic.uir
	}
	return m.cbActive
}

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
	m.cbSaved = callbackFrame{regs: c.R, segs: c.Seg, ip: c.IP, flags: c.Flags,
		hi: c.Hi, fseg: c.FSeg, gseg: c.GSeg}
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

// FinishCallback 把狀態整份還原。由返回哨兵的 `int F9h` 呼叫。
//
// 回 false 表示「根本沒有回呼在跑」——那是個 bug 的訊號，不要吞掉。
func (m *Machine) FinishCallback() bool {
	if !m.cbActive {
		return false
	}
	c := m.CPU
	c.R, c.Seg, c.IP = m.cbSaved.regs, m.cbSaved.segs, m.cbSaved.ip
	c.Hi, c.FSeg, c.GSeg = m.cbSaved.hi, m.cbSaved.fseg, m.cbSaved.gseg
	c.SetFlags(m.cbSaved.flags)
	m.cbActive = false
	return true
}
