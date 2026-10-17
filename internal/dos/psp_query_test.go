package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// `AH=51h／62h` 回目前行程的 PSP（`docs/spec/432-current-psp-query`）。
// 以前回常數 PSPSeg，EXEC 的子行程拿到的是第一支程式的 PSP。
func TestPSPQueryReturnsCurrentProcess(t *testing.T) {
	m, d := newTest(t)
	writeChild(t, d, "KID.COM", exitCOM)
	execChild(m, d, "KID.COM")
	child := d.curPSP
	if child == machine.PSPSeg {
		t.Fatalf("子行程 PSP 與第一支程式相同（%04X），測不出差別", child)
	}
	for _, ax := range []uint16{0x5100, 0x6200} {
		m.CPU.R[cpu.BX] = 0
		call(m, d, 0x21, ax)
		if got := m.CPU.R[cpu.BX]; got != child {
			t.Errorf("AX=%04X 回 BX=%04X，預期子行程 %04X", ax, got, child)
		}
	}
	runToParent(t, m, d)
	call(m, d, 0x21, 0x6200)
	if got := m.CPU.R[cpu.BX]; got != machine.PSPSeg {
		t.Errorf("子行程結束後 AH=62h 回 %04X，預期父行程 %04X", got, machine.PSPSeg)
	}
}
