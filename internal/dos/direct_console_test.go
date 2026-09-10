package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"testing"
)

func TestDirectConsoleInputConsumesWithoutEcho(t *testing.T) {
	m, d := newTest(t)
	d.Stdin = []byte{'6', 0, 3}
	m.CPU.R[cpu.DX] = 0xFF
	for i, want := range []byte{'6', 0, 3} {
		call(m, d, 0x21, 0x0600)
		if byte(m.CPU.R[cpu.AX]) != want || m.CPU.Flags&cpu.ZF != 0 {
			t.Fatalf("第 %d 個字元未正確返回：AX=%04X FLAGS=%04X", i, m.CPU.R[cpu.AX], m.CPU.Flags)
		}
	}
	if len(d.Stdin) != 0 || len(d.Console) != 0 || len(d.KeyReads) != 3 {
		t.Fatal("消耗、回顯或輸入軌跡不符合契約")
	}
	call(m, d, 0x21, 0x0699)
	if byte(m.CPU.R[cpu.AX]) != 0 || m.CPU.Flags&cpu.ZF == 0 || d.Blocked {
		t.Fatal("空佇列必須立即返回 AL=0、ZF=1")
	}
}
