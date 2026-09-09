package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// newScratchDOS 造一個帶唯讀 Root 與可寫 Scratch 的服務層。
func newScratchDOS(t *testing.T, files map[string]string) (*DOS, *machine.Machine) {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	m := machine.New()
	d := New(m, root)
	d.Scratch = t.TempDir()
	d.Install()
	return d, m
}

// putName 把一個 NUL 結尾的檔名放進 DS:DX 指到的地方。
func putName(m *machine.Machine, c *cpu.CPU, name string) {
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x1000, 0x0100
	m.WriteBytes(cpu.Addr(0x1000, 0x0100), append([]byte(name), 0))
}

func int21(m *machine.Machine, d *DOS, ax uint16) *cpu.CPU {
	c := m.CPU
	c.R[cpu.AX] = ax
	d.int21(c)
	return c
}

// 建檔 → 寫 → 讀回來，內容要是寫進去的那一份。
func TestCreateAndWriteLandInTheScratchDirectory(t *testing.T) {
	d, m := newScratchDOS(t, nil)
	c := m.CPU
	putName(m, c, `A:\JUNK\HERO.CHA`)
	int21(m, d, 0x3C00)
	if c.Flags&cpu.CF != 0 {
		t.Fatal("建檔失敗")
	}
	h := c.R[cpu.AX]

	body := []byte("PARTY")
	m.WriteBytes(cpu.Addr(0x1000, 0x0200), body)
	c.Seg[cpu.DS], c.R[cpu.DX], c.R[cpu.BX], c.R[cpu.CX] = 0x1000, 0x0200, h, uint16(len(body))
	int21(m, d, 0x4000)
	if c.Flags&cpu.CF != 0 || c.R[cpu.AX] != uint16(len(body)) {
		t.Fatalf("寫入回 AX=%d CF=%v", c.R[cpu.AX], c.Flags&cpu.CF != 0)
	}
	c.R[cpu.BX] = h
	int21(m, d, 0x3E00)

	got, err := os.ReadFile(filepath.Join(d.Scratch, "HERO.CHA"))
	if err != nil || string(got) != "PARTY" {
		t.Fatalf("暫存層的檔案是 %q（err=%v）", got, err)
	}
	if len(d.Wrote) != 1 {
		t.Fatalf("Wrote 記了 %d 筆，寫成功也要記", len(d.Wrote))
	}
}

// 以寫入模式開一個只存在於原版目錄的檔：要先複製到暫存層，
// **原版那一份一個位元組都不能變**。
func TestOpeningForWriteCopiesIntoScratchAndLeavesRootAlone(t *testing.T) {
	d, m := newScratchDOS(t, map[string]string{"CHARLIST.TXT": "OLD\r\n"})
	c := m.CPU
	putName(m, c, "CHARLIST.TXT")
	int21(m, d, 0x3D02) // 讀寫
	if c.Flags&cpu.CF != 0 {
		t.Fatal("開檔失敗")
	}
	h := c.R[cpu.AX]

	body := []byte("NEW")
	m.WriteBytes(cpu.Addr(0x1000, 0x0200), body)
	c.Seg[cpu.DS], c.R[cpu.DX], c.R[cpu.BX], c.R[cpu.CX] = 0x1000, 0x0200, h, uint16(len(body))
	int21(m, d, 0x4000)
	c.R[cpu.BX] = h
	int21(m, d, 0x3E00)

	root, _ := os.ReadFile(filepath.Join(d.Root, "CHARLIST.TXT"))
	if string(root) != "OLD\r\n" {
		t.Fatalf("原版目錄被改成 %q", root)
	}
	scratch, err := os.ReadFile(filepath.Join(d.Scratch, "CHARLIST.TXT"))
	if err != nil || string(scratch) != "NEW\r\n" {
		t.Fatalf("暫存層是 %q（err=%v）", scratch, err)
	}
}

// 暫存層蓋過原版目錄：存過之後再讀，要讀到自己寫的那一份。
func TestScratchShadowsRootOnRead(t *testing.T) {
	d, m := newScratchDOS(t, map[string]string{"POOL.CFG": "root"})
	if err := os.WriteFile(filepath.Join(d.Scratch, "POOL.CFG"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	putName(m, c, "POOL.CFG")
	int21(m, d, 0x3D00)
	h := c.R[cpu.AX]
	c.Seg[cpu.DS], c.R[cpu.DX], c.R[cpu.BX], c.R[cpu.CX] = 0x1000, 0x0300, h, 16
	int21(m, d, 0x3F00)
	got := make([]byte, c.R[cpu.AX])
	for i := range got {
		got[i] = m.Read8(cpu.Addr(0x1000, 0x0300) + uint32(i))
	}
	if string(got) != "scratch" {
		t.Fatalf("讀到 %q，暫存層應該蓋過原版目錄", got)
	}
}

// 沒有 Scratch 時維持舊行為：不落地、仍回報成功。
// 第一個案例（rich2）走的就是這條，改壞了它不會有任何症狀。
func TestWithoutScratchWritesAreOnlyRecorded(t *testing.T) {
	root := t.TempDir()
	m := machine.New()
	d := New(m, root)
	d.Install()
	c := m.CPU
	putName(m, c, "HERO.CHA")
	int21(m, d, 0x3C00)
	if c.Flags&cpu.CF != 0 {
		t.Fatal("沒有暫存層時建檔也要回一個合法 handle")
	}
	h := c.R[cpu.AX]
	m.WriteBytes(cpu.Addr(0x1000, 0x0200), []byte("XY"))
	c.Seg[cpu.DS], c.R[cpu.DX], c.R[cpu.BX], c.R[cpu.CX] = 0x1000, 0x0200, h, 2
	int21(m, d, 0x4000)
	if c.Flags&cpu.CF != 0 || c.R[cpu.AX] != 2 {
		t.Fatalf("回 AX=%d CF=%v", c.R[cpu.AX], c.Flags&cpu.CF != 0)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("原版目錄多了 %d 個檔", len(entries))
	}
	if len(d.Wrote) != 1 {
		t.Fatalf("Wrote 記了 %d 筆", len(d.Wrote))
	}
}
