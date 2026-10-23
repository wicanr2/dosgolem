package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/436-tss-task-return-v86-entry §6 第 4 項的 Machine 側，依 435 §2.4、§4 改寫：V86 下 IRQ0 在遞送條件成立時
// 經 IDT 遞送（不再失敗即關閉）、IF=0 或 picMask 遮蔽時延後；回呼與週期遠呼叫照常執行。環境見 v86deliver_test.go。

func TestV86IRQ0Gate(t *testing.T) {
	m := deliverProg(t, []byte{0x90, 0x90, 0x90, 0x90})
	m.irq0Pending, m.picMask = true, 0
	ticks := m.Ticks
	mustStep(t, m, "可遞送的 IRQ0")
	if m.irq0Pending || m.Ticks != ticks+1 || !m.CPU.Segmented() || m.CPU.Seg[cpu.CS] != 0x18 {
		t.Errorf("V86 下可遞送的 IRQ0 應經 IDT 遞送：pending=%v Ticks %d→%d CS=%04X", m.irq0Pending, ticks, m.Ticks, m.CPU.Seg[cpu.CS])
	}

	m = deliverProg(t, []byte{0x90, 0x90})
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	m.irq0Pending, m.picMask = true, 0
	mustStep(t, m, "IF=0：延後")
	if !m.irq0Pending || m.CPU.Segmented() {
		t.Error("IF=0 時 irq0Pending 應保留、不遞送")
	}

	m = deliverProg(t, []byte{0x90, 0x90})
	m.picMask = 0x01
	m.irq0Pending = true
	mustStep(t, m, "picMask 遮蔽：延後")
	if !m.irq0Pending || m.CPU.Segmented() {
		t.Error("picMask 遮蔽時 irq0Pending 應保留、不遞送")
	}
}

func TestV86KeyIRQGate(t *testing.T) {
	m := deliverProg(t, []byte{0x90, 0x90})
	m.picMask = 0
	m.WritePhys16(0x09*4+2, 0x1234) // 向量不指向 stub
	m.keyQueue = append(m.keyQueue, KeyEvent{})
	m.KeyEvery, m.nextKey = 1, 0
	mustStep(t, m, "可遞送的 IRQ1")
	if len(m.keyQueue) != 0 || m.KeyIRQs != 1 || !m.CPU.Segmented() || m.CPU.IP != gateOff(9)+1 {
		t.Errorf("V86 下可遞送的 IRQ1 應經 IDT 遞送：佇列 %d KeyIRQs %d IP=%04X", len(m.keyQueue), m.KeyIRQs, m.CPU.IP)
	}
}

func TestV86CallbacksDeferred(t *testing.T) {
	// 435 §4：V86 下不再延後（436 §4 的延後取消）；分段模式的延後見 TestV86CallbacksRun。
	// 459 §2.3「剖面開，預期改變」：保護模式剖面的回呼改經 IRQ12，這個夾具沒有服務層，Step 回 §3.4 條件 1a 的
	// 「IRQ12 樁需要服務層」錯誤，不直接遠呼叫（IDT 74h 的送達與回呼執行改在 mouse459_test.go 有服務層的夾具驗）。
	m := deliverProg(t, []byte{0x90, 0x90, 0x90, 0x90})
	m.SetPeriodicFarCall(0x2000, 0x10, 1)
	m.QueueCallback(QueuedCall{Seg: 0x2000, Off: 0x20})
	wantStepErr(t, m, "V86 回呼（沒有服務層）", "服務層")
	if m.CallbacksMade() != 0 || m.CallbackPending() != 1 {
		t.Errorf("沒有服務層：不直接遠呼叫、佇列不動：已發 %d 佇列 %d", m.CallbacksMade(), m.CallbackPending())
	}
}
