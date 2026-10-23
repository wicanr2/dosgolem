package machine

import (
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 定時掃描碼事件與按住按鍵（`docs/spec/197`）。
//
// FIFO 佇列（PushKey）只能送「按下＋放開」，而且被 KeyEvery 節流：按不住，
// 排得密時送出時間還會一路往後拖。會看「按鍵狀態表」的遊戲（按下設旗標、
// 放開清旗標）在戰鬥與移動上需要真的按住——症狀是「連按也打不贏」，
// 看起來像玩法問題，不像輸入沒送到。

// timedKey 是一個定時事件；seq 讓同一步的事件保持加入順序。
type timedKey struct {
	Step uint64
	Ev   KeyEvent
	seq  uint64
}

// TypematicDelayMS 與 TypematicRate 是 BIOS 開機預設（500 ms、10.9 次／秒）。
const (
	TypematicDelayMS = 500.0
	TypematicRate    = 10.9
)

// ScheduleKey 在第 step 道指令（或之後第一個能送的時機）送出一個掃描碼事件。
func (m *Machine) ScheduleKey(step uint64, scan uint8, brk bool) {
	m.timedKeySeq++
	m.timedKeys = append(m.timedKeys, timedKey{Step: step, Ev: KeyEvent{Scan: scan, Break: brk}, seq: m.timedKeySeq})
	sort.SliceStable(m.timedKeys, func(i, j int) bool {
		if m.timedKeys[i].Step != m.timedKeys[j].Step {
			return m.timedKeys[i].Step < m.timedKeys[j].Step
		}
		return m.timedKeys[i].seq < m.timedKeys[j].seq
	})
}

// HoldKey 從第 from 步按住 scan，duration 道指令後放開。typematic 為真時照 BIOS 預設重複送按下碼。
func (m *Machine) HoldKey(scan uint8, from, duration uint64, typematic bool) {
	m.ScheduleKey(from, scan, false)
	end := from + duration
	if typematic {
		sps := m.InstructionsPerSecond() // 跟著機器速度走（`198`）
		delay := uint64(TypematicDelayMS / 1000 * sps)
		period := uint64(sps / TypematicRate)
		for t := from + delay; period > 0 && t < end; t += period {
			m.ScheduleKey(t, scan, false)
		}
	}
	m.ScheduleKey(end, scan, true)
}

// CancelTimedKeys 移除 scan 在 afterStep 之後還沒送出的按下事件，回移除數（`199` §3.3）。
// 即時放開按鍵時用：已經排好的 typematic 重複不能晚於放開碼送出。
func (m *Machine) CancelTimedKeys(scan uint8, afterStep uint64) int {
	kept := m.timedKeys[:0]
	n := 0
	for _, k := range m.timedKeys {
		if k.Ev.Scan == scan && !k.Ev.Break && k.Step > afterStep {
			n++
			continue
		}
		kept = append(kept, k)
	}
	m.timedKeys = kept
	if n > 0 {
		m.stuckClear(stuckIRQ1) // 459 §3.3a：待處理可能消失，重新起算
	}
	return n
}

// CancelTimedKeysFrom 移除 scan 在 fromStep（含）之後還沒送出的按下事件，回移除數（`docs/spec/449` §2.3）。
// 與 CancelTimedKeys 的差別只在下限含 fromStep。
func (m *Machine) CancelTimedKeysFrom(scan uint8, fromStep uint64) int {
	kept := m.timedKeys[:0]
	n := 0
	for _, k := range m.timedKeys {
		if k.Ev.Scan == scan && !k.Ev.Break && k.Step >= fromStep {
			n++
			continue
		}
		kept = append(kept, k)
	}
	m.timedKeys = kept
	if n > 0 {
		m.stuckClear(stuckIRQ1) // 459 §3.3a：待處理可能消失，重新起算
	}
	return n
}

// heldScan 是一個按著的鍵（`docs/spec/449` §2.2）：按下的步數、按下時算好的 typematic
// 延遲與週期，以及下一個重複碼的步數。
type heldScan struct {
	downStep, delay, period, next uint64
}

func (m *Machine) heldBit(scan uint8) (int, uint64) { return int(scan >> 6), 1 << (scan & 63) }

func (m *Machine) isHeld(scan uint8) bool {
	i, b := m.heldBit(scan)
	return m.heldMask[i]&b != 0
}

// HeldKeyDown 在目前步數按下 scan（`docs/spec/449` §2.2）。scan 為 0 或 >= 80h、或已經按著時
// 不排程並回 false；否則排按下碼、記下按住紀錄，回 true。delay、period 在這裡算一次，
// 之後機器速度改變不影響這個鍵，與 HoldKey 相同。
func (m *Machine) HeldKeyDown(scan uint8) bool {
	if scan == 0 || scan >= 0x80 || m.isHeld(scan) {
		return false
	}
	sps := m.InstructionsPerSecond()
	delay := uint64(TypematicDelayMS / 1000 * sps)
	period := uint64(sps / TypematicRate)
	now := m.Steps
	m.ScheduleKey(now, scan, false)
	m.heldScans[scan] = heldScan{downStep: now, delay: delay, period: period, next: now + delay}
	i, b := m.heldBit(scan)
	m.heldMask[i] |= b
	return true
}

// HeldKeyUp 在目前步數放開 scan。scan 為 0 或 >= 80h、或沒按著時不排程並回 false。
// 否則取消該鍵 Step >= max(now, downStep+1) 的按下事件（與 HoldKey 的 t < end 等價，
// 又不刪同一步的按下碼，§2.3），排放開碼，清除按住紀錄，回 true。
func (m *Machine) HeldKeyUp(scan uint8) bool {
	if scan == 0 || scan >= 0x80 || !m.isHeld(scan) {
		return false
	}
	now := m.Steps
	m.CancelTimedKeysFrom(scan, max(now, m.heldScans[scan].downStep+1))
	m.ScheduleKey(now, scan, true)
	m.heldScans[scan] = heldScan{}
	i, b := m.heldBit(scan)
	m.heldMask[i] &^= b
	return true
}

// ScheduleHeldRepeats 依掃描碼由小到大，把每個按著的鍵 next <= Steps+n 的重複碼排進去（`docs/spec/449` §2.3）。
// period 為 0 的鍵不排重複碼。
func (m *Machine) ScheduleHeldRepeats(n uint64) {
	if m.heldMask == [2]uint64{} {
		return
	}
	end := m.Steps + n
	if end < m.Steps {
		end = ^uint64(0)
	}
	for scan := 1; scan < 0x80; scan++ {
		if !m.isHeld(uint8(scan)) {
			continue
		}
		h := &m.heldScans[scan]
		if h.period == 0 {
			continue
		}
		for ; h.next <= end; h.next += h.period {
			m.ScheduleKey(h.next, uint8(scan), false)
		}
	}
}

// ReleaseHeldKeys 依掃描碼由小到大放開每個按著的鍵，回放開的個數（`docs/spec/449` §2.4）。
func (m *Machine) ReleaseHeldKeys() int {
	n := 0
	for scan := 1; scan < 0x80; scan++ {
		if m.HeldKeyUp(uint8(scan)) {
			n++
		}
	}
	return n
}

// HeldKeys 回按著的鍵，依掃描碼排序。
func (m *Machine) HeldKeys() []uint8 {
	out := []uint8{}
	for scan := 1; scan < 0x80; scan++ {
		if m.isHeld(uint8(scan)) {
			out = append(out, uint8(scan))
		}
	}
	return out
}

// HeldMask 回按著的鍵的位元遮罩：bit scan 立起表示按著（給收據用）。
func (m *Machine) HeldMask() [2]uint64 { return m.heldMask }

// clearHeldKeys 清空按住狀態，不動 timedKeys（LoadState 用，`docs/spec/449` §2.2）。
func (m *Machine) clearHeldKeys() {
	m.heldScans = [128]heldScan{}
	m.heldMask = [2]uint64{}
}

// ScheduledKey 是一個還沒送出的定時事件（唯讀檢視）。
type ScheduledKey struct {
	Step uint64
	KeyEvent
}

// ScheduledKeys 回還沒送出的定時事件，依送出順序。
func (m *Machine) ScheduledKeys() []ScheduledKey {
	out := make([]ScheduledKey, len(m.timedKeys))
	for i, k := range m.timedKeys {
		out[i] = ScheduledKey{Step: k.Step, KeyEvent: k.Ev}
	}
	return out
}

// TimedKeysPending 是還沒送出的定時事件數。
func (m *Machine) TimedKeysPending() int { return len(m.timedKeys) }

// timedKeyDue 回報是否有已到期的定時事件（埠 64h 的狀態用）。
func (m *Machine) timedKeyDue() bool {
	return len(m.timedKeys) > 0 && m.Steps >= m.timedKeys[0].Step
}

// timedKeyTick 送出一個到期的定時事件；送出回 true。能送的條件與 keyTick 相同。
func (m *Machine) timedKeyTick() bool {
	// 459 §3.3a、§3.4：保護模式剖面更新 IRQ1 的卡住計數，ISR0、ISR1 為 1 時不送。卡住錯誤已存進 irqErr 時
	// 回 true（這一步不再試 keyTick，Step 回錯誤）。
	if m.pmProfile && m.irq1Gate() {
		return m.irqErr != nil
	}
	if !m.timedKeyDue() {
		return false
	}
	if !m.CPU.Flag(cpu.IF) || m.picMask&0x02 != 0 {
		return false
	}
	if !m.KbdIntercept && m.ReadSup16(0x09*4+2) == StubSeg { // 435 §4：宿主存取；447 §2.5：攔截開啟時不擋
		m.keyStalls++
		return false
	}
	// 433 §2：IDTR 守門在取出 timedKeys[0] 之前。回 true：這一步不再試 keyTick（Step 會回錯誤）。
	if m.ivtGuardIRQ(0x09) {
		return true
	}
	ev := m.timedKeys[0].Ev
	m.timedKeys = m.timedKeys[1:]
	m.kbdData = ev.Code()
	m.KeyIRQs++
	m.eventThisStep = true // 444 §2.7
	if m.pmProfile {       // 459 §3.3a：IRQ1 送達設 ISR1
		m.irq1Accepted()
	}
	m.raiseIRQ(0x09)
	return true
}
