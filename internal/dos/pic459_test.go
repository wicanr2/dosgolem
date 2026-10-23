package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/459-mouse-irq12-v86-delivery §6.2 第 29 項（Go 端 biosInt9）：保護模式剖面下，程式掛 int 09h 再 chain 回
// 舊向量（StubSeg 的 per-vector stub）時，biosInt9 結尾送一次 out 20h,20h，ISR1 清 0；實模式剖面不送。

// newPMProg 造一台（保護模式剖面或實模式剖面）載入 code 的機器並裝上服務層。code 在 PSP:0100。
func newPMProg(t *testing.T, pm bool, code []byte) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	if pm {
		if err := m.SetExtendedMemoryKB(4096); err != nil {
			t.Fatal(err)
		}
		if err := m.SetProtectedModeProfile(true); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	m.IRQ0Every = 0
	d := New(m, t.TempDir())
	d.Install()
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	return m, d
}

func TestPIC459BiosInt9EOI(t *testing.T) {
	// 0100: jmp $；0110: int 09h 包裝常式 jmp far StubSeg:StubOff(9)（chain 回 BIOS）。
	code := make([]byte, 0x20)
	code[0], code[1] = 0xEB, 0xFE
	copy(code[0x10:], farJmp(machine.StubSeg, machine.StubOff(0x09)))
	for _, pm := range []bool{true, false} {
		m, _ := newPMProg(t, pm, code)
		m.Write16(0x09*4, 0x0110)
		m.Write16(0x09*4+2, machine.PSPSeg)
		m.KeyEvery = 1
		m.QueueScan(0x1E)
		n := len(m.PortLog)
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
		if pm && !m.PICStats().ISR1 {
			t.Fatal("保護模式剖面：IRQ1 送達應設 ISR1")
		}
		for i := 0; i < 20 && !(m.CPU.Seg[cpu.CS] == machine.PSPSeg && m.CPU.IP == 0x100); i++ {
			if err := m.Step(); err != nil {
				t.Fatal(err)
			}
		}
		var eoi int
		for _, w := range m.PortLog[n:] {
			if w.Port == 0x20 && w.Val == 0x20 {
				eoi++
			}
		}
		want := 0
		if pm {
			want = 1
		}
		if eoi != want || m.PICStats().ISR1 {
			t.Errorf("pm=%t：biosInt9 送 EOI %d 次（應 %d）、ISR1=%t", pm, eoi, want, m.PICStats().ISR1)
		}
		if m.BIOSKeyCount() != 1 {
			t.Errorf("pm=%t：biosInt9 應把鍵放進緩衝：%d", pm, m.BIOSKeyCount())
		}
	}
}
