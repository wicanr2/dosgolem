package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"testing"
)

func TestBIOSMotorCountdownBoundaries(t *testing.T) {
	for _, initial := range []byte{0, 1, 2, 255} {
		m := New()
		const scratch = 0x2000
		// Machine 單獨測試未掛 DOS 的 INT1C hook；安裝明確的 IRET 接收端。
		m.WriteBytes(cpu.Addr(scratch, 0x300), []byte{0xCF})
		m.Write16(0x1C*4, 0x300)
		m.Write16(0x1C*4+2, scratch)
		m.WriteBytes(cpu.Addr(scratch, 0x400), []byte{0x90, 0xF4})
		m.CPU.Seg[cpu.CS] = scratch
		m.CPU.IP = 0x400
		m.CPU.Seg[cpu.SS] = scratch
		m.CPU.R[cpu.SP] = 0x200
		m.CPU.Seg[cpu.DS] = 0x1234
		m.CPU.R[cpu.AX] = 0x5678
		m.CPU.R[cpu.DX] = 0x9abc
		m.CPU.SetFlags(m.CPU.Flags | cpu.IF | cpu.CF)
		flags := m.CPU.Flags
		m.Write8(0x440, initial)
		m.CPU.Interrupt(8)
		for i := 0; i < 100 && !m.CPU.Halted; i++ {
			if err := m.Step(); err != nil {
				t.Fatal(err)
			}
		}
		want := initial
		if want > 0 {
			want--
		}
		if m.Read8(0x440) != want || !m.CPU.Halted {
			t.Fatalf("初值 %d，倒數未完成", initial)
		}
		if m.CPU.Seg[cpu.DS] != 0x1234 || m.CPU.R[cpu.AX] != 0x5678 || m.CPU.R[cpu.DX] != 0x9abc || m.CPU.Flags != flags {
			t.Fatal("暫存器或旗標未保留")
		}
	}
}
