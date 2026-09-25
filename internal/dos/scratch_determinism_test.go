package dos

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 暫存層新建的檔，DOS 看到的日期時間要是虛擬時刻，不是主機建檔時間
// （`docs/spec/237` §2.1）。
func TestScratchFileTimeIsVirtual(t *testing.T) {
	d, m := newScratchDOS(t, nil)
	c := m.CPU
	putName(m, c, "HERO.CHA")
	int21(m, d, 0x3C00)
	if c.Flags&cpu.CF != 0 {
		t.Fatal("建檔失敗")
	}
	h := c.R[cpu.AX]
	m.WriteBytes(cpu.Addr(0x1000, 0x0200), []byte("X"))
	c.Seg[cpu.DS], c.R[cpu.DX], c.R[cpu.BX], c.R[cpu.CX] = 0x1000, 0x0200, h, 1
	int21(m, d, 0x4000)
	c.R[cpu.BX] = h
	int21(m, d, 0x3E00)

	info, err := os.Stat(filepath.Join(d.Scratch, "HERO.CHA"))
	if err != nil {
		t.Fatal(err)
	}
	gotT, gotD := dosDateTime(info)
	v := d.clock()
	wantT := uint16(v.Hour)<<11 | uint16(v.Min)<<5 | uint16(v.Sec/2)
	wantD := uint16(1993-1980)<<9 | 1<<5 | 1
	if gotT != wantT || gotD != wantD {
		t.Fatalf("DOS 時間=%04x 日期=%04x，要 %04x／%04x", gotT, gotD, wantT, wantD)
	}
	if info.ModTime().Year() != 1993 || time.Since(info.ModTime()) < 24*time.Hour {
		t.Fatalf("mtime 仍是主機時間：%v", info.ModTime())
	}
}

// 寫時複製與建檔遇到大小寫不同的既有暫存檔，要用那一份，不另建
// （`docs/spec/237` §2.3）。
func TestScratchReusesExistingNameIgnoringCase(t *testing.T) {
	d, m := newScratchDOS(t, map[string]string{"CHARS.DAX": "ROOT"})
	if err := os.WriteFile(filepath.Join(d.Scratch, "CHARS.DAX"), []byte("SCRATCH"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	putName(m, c, "chars.dax")
	int21(m, d, 0x3D02)
	if c.Flags&cpu.CF != 0 {
		t.Fatal("開檔失敗")
	}
	c.R[cpu.BX] = c.R[cpu.AX]
	int21(m, d, 0x3E00)

	putName(m, c, "hero.who")
	int21(m, d, 0x3C00)
	c.R[cpu.BX] = c.R[cpu.AX]
	int21(m, d, 0x3E00)
	putName(m, c, "HERO.WHO")
	int21(m, d, 0x3C00)
	c.R[cpu.BX] = c.R[cpu.AX]
	int21(m, d, 0x3E00)

	entries, err := os.ReadDir(d.Scratch)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 {
		t.Fatalf("暫存層有 %v，大小寫不同的同名檔只能有一份", names)
	}
	got, _ := os.ReadFile(filepath.Join(d.Scratch, "CHARS.DAX"))
	if string(got) != "SCRATCH" {
		t.Fatalf("既有暫存檔被覆蓋成 %q", got)
	}
}
