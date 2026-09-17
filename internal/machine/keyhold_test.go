package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// `docs/spec/197`：定時掃描碼事件與按住。

// runKeys 從第 from 步逐步推進到 to，每一步模擬「處理常式已 iret」（IF 重新打開），
// 照指令迴圈的順序先試定時事件、再試 FIFO 佇列，記下每次送出的步數與掃描碼。
func runKeys(m *Machine, from, to uint64) (steps []uint64, codes []uint8) {
	for s := from; s <= to; s++ {
		m.Steps = s
		m.CPU.SetFlags(cpu.IF)
		before := m.KeyIRQs
		if len(m.timedKeys) == 0 || !m.timedKeyTick() {
			if len(m.keyQueue) > 0 {
				m.keyTick()
			}
		}
		if m.KeyIRQs != before {
			steps = append(steps, s)
			codes = append(codes, m.kbdData)
		}
	}
	return steps, codes
}

func TestScheduledKeysIgnoreKeyEvery(t *testing.T) {
	m := armKeyboard(t)
	m.ScheduleKey(100, 0x39, false)
	m.ScheduleKey(110, 0x39, true)
	steps, codes := runKeys(m, 0, 200)
	if len(steps) != 2 || steps[0] != 100 || steps[1] != 110 {
		t.Fatalf("送出步數 %v，要 [100 110]（KeyEvery=%d 不該影響）", steps, m.KeyEvery)
	}
	if codes[0] != 0x39 || codes[1] != 0xB9 {
		t.Errorf("掃描碼 % X，要 39 B9", codes)
	}
}

func TestScheduledKeyWaitsForIF(t *testing.T) {
	m := armKeyboard(t)
	m.ScheduleKey(10, 0x1C, false)
	m.Steps = 10
	m.CPU.SetFlags(0) // IF 關閉（SetFlags 設的是整個旗標字組）
	if m.timedKeyTick() {
		t.Fatal("IF 關著卻送出了")
	}
	if m.TimedKeysPending() != 1 {
		t.Fatalf("IF 關著時事件被丟掉，剩 %d", m.TimedKeysPending())
	}
	steps, _ := runKeys(m, 20, 30)
	if len(steps) != 1 || steps[0] != 20 {
		t.Fatalf("開 IF 後送出步數 %v，要 [20]", steps)
	}
}

func TestHoldKeyWithoutTypematic(t *testing.T) {
	m := armKeyboard(t)
	m.HoldKey(0x39, 1000, 5_000_000, false)
	steps, codes := runKeys(m, 0, 5_001_100)
	if len(steps) != 2 || steps[0] != 1000 || steps[1] != 5_001_000 {
		t.Fatalf("步數 %v，要 [1000 5001000]", steps)
	}
	if codes[0] != 0x39 || codes[1] != 0xB9 {
		t.Errorf("掃描碼 % X", codes)
	}
}

func TestHoldKeyTypematicRepeats(t *testing.T) {
	m := armKeyboard(t)
	sps := StepsPerSecond()
	dur := uint64(2 * sps)
	m.HoldKey(0x39, 0, dur, true)
	var makes, breaks int
	for _, e := range m.timedKeys {
		if e.Ev.Break {
			breaks++
		} else {
			makes++
		}
	}
	// 1 次按下 ＋ ⌊(2000 − 500) ÷ (1000 ÷ 10.9)⌋ ＋ 1 次重複（延遲那一刻本身也送）
	repeats := (2000.0 - TypematicDelayMS) / (1000.0 / TypematicRate)
	want := 1 + int(repeats) + 1
	if makes < want-1 || makes > want+1 || breaks != 1 {
		t.Errorf("按下 %d 次（要 %d ±1）、放開 %d 次（要 1）", makes, want, breaks)
	}
	if last := m.timedKeys[len(m.timedKeys)-1]; !last.Ev.Break || last.Step != dur {
		t.Errorf("最後一個事件 %+v，要第 %d 步放開", last, dur)
	}
}

func TestTimedKeyBeforeFIFO(t *testing.T) {
	m := armKeyboard(t)
	m.PushKey(0x1E) // A
	m.ScheduleKey(0, 0x30, false)
	m.nextKey = 0
	steps, codes := runKeys(m, 0, 0)
	if len(steps) != 1 || codes[0] != 0x30 {
		t.Fatalf("同一步先送了 % X，要先送定時事件 30", codes)
	}
}

func TestPort64SeesDueTimedKey(t *testing.T) {
	m := armKeyboard(t)
	m.ScheduleKey(50, 0x39, false)
	m.Steps = 10
	if v := m.In8(0x64); v&1 != 0 {
		t.Errorf("還沒到期就回報有資料：%02X", v)
	}
	m.Steps = 50
	if v := m.In8(0x64); v&1 == 0 {
		t.Errorf("到期了卻沒回報有資料：%02X", v)
	}
}

// TestTimedKeysSurviveSnapshotRestore：按住中還原快照，放開碼不能不見（否則鍵會一直卡著）。
func TestTimedKeysSurviveSnapshotRestore(t *testing.T) {
	m := armKeyboard(t)
	m.HoldKey(0x39, 100, 1000, false)
	runKeys(m, 0, 500) // 按下已送出，放開還沒
	snap := m.Snapshot()
	runKeys(m, 501, 2000)
	if m.TimedKeysPending() != 0 {
		t.Fatalf("第一次跑完還剩 %d 個事件", m.TimedKeysPending())
	}
	m.Restore(snap)
	if m.TimedKeysPending() != 1 {
		t.Fatalf("還原後剩 %d 個事件，要 1 個（放開碼）", m.TimedKeysPending())
	}
	steps, codes := runKeys(m, 501, 2000)
	if len(steps) != 1 || steps[0] != 1100 || codes[0] != 0xB9 {
		t.Fatalf("還原後送出 %v % X，要第 1100 步 B9", steps, codes)
	}
}
