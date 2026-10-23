package dos

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/459-mouse-irq12-v86-delivery §6.2 第 1、6、10、18、22 項（服務層：int FBh／FCh 的接手、int 33h、RunMouseEvent）。

const (
	m459CB    = 0x0300 // 回呼：inc word cs:[0380]; mov cs:[0382],ax; retf
	m459Count = 0x0380
)

// mouseProg 組 COM：main 在 0100h，回呼在 0300h。
func mouseProg(main []byte) []byte {
	code := make([]byte, 0x290)
	for i := range code {
		code[i] = 0x90
	}
	copy(code, main)
	copy(code[m459CB-0x100:], []byte{0x2E, 0xFF, 0x06, 0x80, 0x03, 0x2E, 0xA3, 0x82, 0x03, 0xCB})
	for i := m459Count - 0x100; i < m459Count-0x100+0x10; i++ {
		code[i] = 0 // 計數與 AX 的存放區
	}
	return code
}

func newMouse459(t *testing.T, pm bool, main []byte) (*machine.Machine, *DOS) {
	t.Helper()
	if main == nil {
		main = []byte{0xEB, 0xFE}
	}
	m, d := newPMProg(t, pm, mouseProg(main))
	d.Mouse.Handler.Seg, d.Mouse.Handler.Off, d.Mouse.Handler.Mask, d.Mouse.Handler.Set = machine.PSPSeg, m459CB, 0xFFFF, true
	return m, d
}

func cbCount(m *machine.Machine) uint16 { return m.Read16(uint32(machine.PSPSeg)*16 + m459Count) }

// 第 1、10 項：RunMouseEvent 在剖面開啟時回呼返回回 (true, nil) 且 UIR 為 0；經 IRQ12（送達 1 次、回呼 1 次）。
func TestMouse459RunMouseEvent(t *testing.T) {
	m, d := newMouse459(t, true, nil)
	d0 := m.Deliveries()
	ok, err := d.RunMouseEvent(EvMove, 3, -2)
	_, isr4, isr2, uir, queued, _ := m.MouseIRQ12()
	if !ok || err != nil || uir || isr4 || isr2 || queued != 0 || cbCount(m) != 1 || m.Deliveries() != d0+1 {
		t.Errorf("RunMouseEvent：ok=%t err=%v UIR=%t ISR4=%t ISR2=%t 佇列 %d 回呼 %d 送達 %d", ok, err, uir, isr4, isr2, queued, cbCount(m), m.Deliveries()-d0)
	}
	if st := m.PICStats(); st.IRQ12 != 1 || st.CallbacksMade != 1 {
		t.Errorf("應經 IRQ12：送達 %d 回呼 %d", st.IRQ12, st.CallbacksMade)
	}

	// 事件被丟棄：程式在 IF=0 時以 AX=000Ch、CX=0 停用，再 sti。回 (false, nil)，步數遠小於 callbackBudget。
	prog := []byte{0xB8, 0x0C, 0x00, 0x31, 0xC9, 0xCD, 0x33, 0xFB, 0xEB, 0xFE}
	m, d = newMouse459(t, true, prog)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	s0 := m.Steps
	ok, err = d.RunMouseEvent(EvMove, 1, 1)
	if ok || err != nil || m.Steps-s0 > 1000 || m.MouseIRQ12Discards() != 1 {
		t.Errorf("丟棄：ok=%t err=%v 步數 %d 丟棄 %d", ok, err, m.Steps-s0, m.MouseIRQ12Discards())
	}

	// IRQ12 被遮：逾時錯誤，訊息含 ISR4、ISR2、UIR、從片 IMR 與佇列筆數。
	m, d = newMouse459(t, true, nil)
	m.Out8(0xA1, 0xFF)
	_, err = d.RunMouseEvent(EvMove, 1, 1)
	for _, want := range []string{"ISR4", "ISR2", "UIR", "從片 IMR=FFh", "佇列 1 筆"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("被遮時應逾時、訊息含 %s：%v", want, err)
		}
	}
}

// 第 18 項：int FBh／FCh 只在樁內的那一道、且剖面開啟時接手。
func TestMouse459HandleTrapAddress(t *testing.T) {
	m, d := newMouse459(t, true, []byte{0xCD, 0xFB, 0xCD, 0xFC, 0xEB, 0xFE})
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	d.MouseEvent(EvMove)
	for i := 0; i < 2; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	_, _, _, uir, queued, _ := m.MouseIRQ12()
	if uir || queued != 1 || m.CallbacksMade() != 0 || cbCount(m) != 0 {
		t.Errorf("程式自己的 CD FB 不應被服務層接手：UIR=%t 佇列 %d 回呼 %d", uir, queued, m.CallbacksMade())
	}
	if unimpl(d, 0xFB) != 1 || unimpl(d, 0xFC) != 1 {
		t.Errorf("程式自己的 CD FB／CD FC 照一般軟體中斷（IVT 仍指 StubSeg：記未實作）：%v", d.Unimplemented)
	}
	// 剖面關閉：Mouse74Off+5 的 CD FB 也不接手。
	r, rd := newMouse459(t, false, nil)
	copy(r.Mem[machine.StubSeg*16+machine.Mouse74Off:], []byte{0xFB, 0x1E, 0x06, 0x66, 0x60, 0xCD, 0xFB})
	r.CPU.Seg[cpu.CS], r.CPU.IP = machine.StubSeg, machine.Mouse74TrapIP
	r.CPU.SetFlags(r.CPU.Flags &^ cpu.IF)
	if err := r.Step(); err != nil {
		t.Fatal(err)
	}
	if unimpl(rd, 0xFB) != 1 || r.CallbacksMade() != 0 {
		t.Errorf("剖面關閉時 Mouse74Off+5 的 CD FB 不應接手：%v", rd.Unimplemented)
	}
	// UIR 本來為 0 時的 int FCh（樁內那一道）：記一筆未實作，不吞掉。
	m, d = newMouse459(t, true, nil)
	m.CPU.Seg[cpu.CS], m.CPU.IP = machine.StubSeg, machine.MouseUIROff
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	for i := 0; i < 2; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if unimpl(d, 0xFC) != 1 {
		t.Errorf("UIR 為 0 時的 int FCh 應記一筆未實作：%v", d.Unimplemented)
	}
}

// 第 6、22 項：int 33h 的 AX=0000h、000Ch、0014h。
func TestMouse459Int33(t *testing.T) {
	// AX=0000h：佇列清空、丟棄計數加上清掉的筆數、從片 IMR bit 4 清 0、之後不再遞送。
	m, d := newMouse459(t, true, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	m.Out8(0xA1, 0xFF)
	d.MouseEvent(EvMove)
	d.MouseEvent(EvMove)
	call(m, d, 0x33, 0x0000)
	pending, _, _, uir, queued, discards := m.MouseIRQ12()
	if pending || uir || queued != 0 || discards != 2 || m.PICStats().SlaveIMR != 0xEF {
		t.Errorf("AX=0000h：待處理=%t UIR=%t 佇列 %d 丟棄 %d 從片 IMR %02X", pending, uir, queued, discards, m.PICStats().SlaveIMR)
	}
	// 剖面關閉時 AX=0000h 不動佇列（現況：實模式剖面的直接遠呼叫佇列由 startCallback 消耗）。
	r, rd := newMouse459(t, false, nil)
	r.CPU.SetFlags(r.CPU.Flags &^ cpu.IF)
	rd.MouseEvent(EvMove)
	call(r, rd, 0x33, 0x0000)
	if r.CallbackPending() != 1 {
		t.Errorf("剖面關閉：AX=0000h 不應清佇列：%d", r.CallbackPending())
	}

	// AX=000Ch CX=0：排入後停用，遞送時丟棄（返回樁 +0），不呼叫回呼。
	m, d = newMouse459(t, true, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	d.MouseEvent(EvMove)
	m.CPU.R[cpu.CX] = 0
	call(m, d, 0x33, 0x000C)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	runUntil(t, m, 200, "丟棄", func() bool { return m.MouseIRQ12Discards() == 1 && atMouseMain(m) })
	if cbCount(m) != 0 || m.PICStats().ISR4 || m.PICStats().ISR2 {
		t.Errorf("停用後丟棄：回呼 %d ISR4=%t ISR2=%t", cbCount(m), m.PICStats().ISR4, m.PICStats().ISR2)
	}

	// AX=0014h 交換常式：已排入的事件以新常式呼叫（遞送當下的登記）；新常式遮罩不含該事件時丟棄。
	m, d = newMouse459(t, true, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	d.MouseEvent(EvMove)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX] = machine.PSPSeg, m459CB+5, EvMove // 新常式：mov cs:[0382],ax; retf
	call(m, d, 0x33, 0x0014)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	runUntil(t, m, 200, "交換", func() bool { return m.CallbacksMade() == 1 && atMouseMain(m) })
	if cbCount(m) != 0 || m.Read16(uint32(machine.PSPSeg)*16+0x382) != EvMove {
		t.Errorf("交換後應以新常式呼叫：舊常式計數 %d、新常式 AX=%04X", cbCount(m), m.Read16(uint32(machine.PSPSeg)*16+0x382))
	}
	d.MouseEvent(EvMove) // 以目前登記（遮罩含移動）排入
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX] = machine.PSPSeg, m459CB, EvLeftDown
	call(m, d, 0x33, 0x0014) // 遞送之前換成遮罩不含移動的常式
	runUntil(t, m, 200, "遮罩不含", func() bool { return m.MouseIRQ12Discards() == 1 && atMouseMain(m) })
	if cbCount(m) != 0 {
		t.Errorf("新常式遮罩不含事件時應丟棄：回呼 %d", cbCount(m))
	}
}

// unimpl 回 Unimplemented 中中斷號為 n 的總次數（不分 AH、AL）。
func unimpl(d *DOS, n uint8) int {
	s := 0
	for k, v := range d.Unimplemented {
		if k.Int == n {
			s += v
		}
	}
	return s
}

func atMouseMain(m *machine.Machine) bool {
	return m.CPU.Seg[cpu.CS] == machine.PSPSeg && m.CPU.IP == 0x100
}
