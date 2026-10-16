package machine

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/436-tss-task-return-v86-entry §6 第 4 項的 Machine 側：V86 下 IRQ0 在遞送條件成立時失敗、
// IF=0 或 picMask 遮蔽時延後；回呼與週期遠呼叫延後、不消耗。V86 以 SetVM 注入（432 的合成測試做法）。

func v86Machine(t *testing.T) *Machine {
	t.Helper()
	m := newSysProg(t, []byte{0x90, 0x90, 0x90, 0x90})
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustNil(t, "SetVM", m.SetVM(true))
	if !m.v86 {
		t.Fatal("前提：m.v86 應為真")
	}
	return m
}

func TestV86IRQ0Gate(t *testing.T) {
	m := v86Machine(t)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.irq0Pending = true
	ip, ticks := m.CPU.IP, m.Ticks
	err := m.Step()
	if err == nil || !strings.Contains(err.Error(), "V86") || !strings.Contains(err.Error(), "INT 08h") {
		t.Fatalf("V86 下可遞送的 IRQ0 應失敗即關閉：%v", err)
	}
	if !m.irq0Pending || m.Ticks != ticks || m.CPU.IP != ip {
		t.Errorf("失敗時不得消耗狀態：pending=%v Ticks %d→%d IP %04X→%04X", m.irq0Pending, ticks, m.Ticks, ip, m.CPU.IP)
	}

	m = v86Machine(t)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	m.irq0Pending = true
	mustStep(t, m, "IF=0：延後")
	if !m.irq0Pending {
		t.Error("IF=0 時 irq0Pending 應保留")
	}

	m = v86Machine(t)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.picMask = 0x01
	m.irq0Pending = true
	mustStep(t, m, "picMask 遮蔽：延後")
	if !m.irq0Pending {
		t.Error("picMask 遮蔽時 irq0Pending 應保留")
	}
}

func TestV86KeyIRQGate(t *testing.T) {
	m := v86Machine(t)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.WritePhys16(0x09*4+2, 0x1234) // 向量不指向 stub
	m.keyQueue = append(m.keyQueue, KeyEvent{})
	m.KeyEvery, m.nextKey = 1, 0
	err := m.Step()
	if err == nil || !strings.Contains(err.Error(), "INT 09h") {
		t.Fatalf("V86 下可遞送的 IRQ1 應失敗即關閉：%v", err)
	}
	if len(m.keyQueue) != 1 {
		t.Error("失敗時不得取出鍵盤佇列")
	}
}

func TestV86CallbacksDeferred(t *testing.T) {
	m := v86Machine(t)
	m.SetPeriodicFarCall(0x2000, 0x10, 1)
	m.QueueCallback(QueuedCall{Seg: 0x2000, Off: 0x20})
	sp, next := m.CPU.R[cpu.SP], m.periodic.next
	for i := 0; i < 3; i++ {
		mustStep(t, m, "V86 nop")
	}
	if m.PeriodicCalls() != 0 || m.periodic.next != next || m.CallbackPending() != 1 || m.CallbacksMade() != 0 {
		t.Errorf("回呼應延後不消耗：periodic %d 次 next %d→%d 佇列 %d 已發 %d",
			m.PeriodicCalls(), next, m.periodic.next, m.CallbackPending(), m.CallbacksMade())
	}
	if m.CPU.R[cpu.SP] != sp || m.CPU.IP != 0x103 {
		t.Errorf("回呼不得動堆疊或轉移：SP %04X→%04X IP=%04X", sp, m.CPU.R[cpu.SP], m.CPU.IP)
	}
	// 離開 V86（合成）：延後的回呼照常發出。
	mustNil(t, "SetVM(false)", m.SetVM(false))
	mustNil(t, "SetCR0", m.SetCR0(0x10))
	mustStep(t, m, "實模式")
	if m.CallbacksMade() != 1 {
		t.Errorf("離開 V86 後回呼應發出：%d", m.CallbacksMade())
	}
}
