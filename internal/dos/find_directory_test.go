package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestFindFirstDirectoryRequiresAttributeAndWritesDirectoryDTA(t *testing.T) {
	m, d := newTest(t)
	if err := os.Mkdir(filepath.Join(d.Root, "SAVE"), 0o755); err != nil {
		t.Fatal(err)
	}

	putFileName(m, `C:\BUCK\SAVE`)
	m.CPU.R[cpu.CX] = 0
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 18 {
		t.Fatalf("CX=0 搜尋目錄回 CF=%t AX=%d，預期 CF=1 AX=18",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}

	putFileName(m, `C:\BUCK\SAVE`)
	m.CPU.R[cpu.CX] = 0x10
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("CX=10h 搜尋目錄失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	dta := cpu.Addr(machine.PSPSeg, 0x80)
	if got := m.Read8(dta + 0x15); got != 0x10 {
		t.Errorf("DTA 屬性 = %02X，預期 10", got)
	}
	if lo, hi := m.Read16(dta+0x1A), m.Read16(dta+0x1C); lo != 0 || hi != 0 {
		t.Errorf("目錄大小 = %04X:%04X，預期 0", hi, lo)
	}
	name := make([]byte, 4)
	for i := range name {
		name[i] = m.Read8(dta + 0x1E + uint32(i))
	}
	if string(name) != "SAVE" {
		t.Errorf("DTA 名稱 = %q，預期 SAVE", name)
	}
}

func TestFindFirstSeesDirectoryCreatedInScratch(t *testing.T) {
	m, d := newTest(t)
	d.Scratch = t.TempDir()

	putFileName(m, `C:\BUCK\SAVE`)
	call(m, d, 0x21, 0x3900)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("建立 Scratch 目錄失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	putFileName(m, `C:\BUCK\SAVE`)
	m.CPU.R[cpu.CX] = 0x10
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("找不到剛建立的 Scratch 目錄：AX=%d", m.CPU.R[cpu.AX])
	}
	if got := m.Read8(cpu.Addr(machine.PSPSeg, 0x80) + 0x15); got != 0x10 {
		t.Errorf("Scratch 目錄 DTA 屬性 = %02X，預期 10", got)
	}
}
