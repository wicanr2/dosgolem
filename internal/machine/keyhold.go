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
	return n
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
	if !m.timedKeyDue() {
		return false
	}
	if !m.CPU.Flag(cpu.IF) || m.picMask&0x02 != 0 {
		return false
	}
	if m.Read16(0x09*4+2) == StubSeg {
		m.keyStalls++
		return false
	}
	ev := m.timedKeys[0].Ev
	m.timedKeys = m.timedKeys[1:]
	m.kbdData = ev.Code()
	m.KeyIRQs++
	m.CPU.Interrupt(0x09)
	return true
}
