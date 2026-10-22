package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 規格 447：唯讀疊層。讀檔順序 Scratch、Layers、Root；疊層永遠不寫。
func TestReadLayersOrderAndNeverWritten(t *testing.T) {
	m, d := newTest(t)
	layer, scratch := t.TempDir(), t.TempDir()
	d.Layers, d.Scratch = []string{layer}, scratch
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(d.Root, "A.DAT", "root")
	write(layer, "A.DAT", "layr")
	write(d.Root, "B.DAT", "root")
	write(layer, "C.DAT", "layr")
	write(scratch, "C.DAT", "scra")

	c := m.CPU
	readFirst4 := func(name string, mode uint16) string {
		t.Helper()
		m.WriteBytes(0x50000, append([]byte(name), 0))
		c.Seg[cpu.DS], c.R[cpu.DX] = 0x5000, 0
		call(m, d, 0x21, 0x3D00|mode)
		if c.Flags&cpu.CF != 0 {
			t.Fatalf("開 %s 失敗 AX=%04X", name, c.R[cpu.AX])
		}
		h := c.R[cpu.AX]
		c.R[cpu.BX], c.R[cpu.CX] = h, 4
		c.Seg[cpu.DS], c.R[cpu.DX] = 0x6000, 0
		call(m, d, 0x21, 0x3F00)
		got := string([]byte{m.Read8(0x60000), m.Read8(0x60001), m.Read8(0x60002), m.Read8(0x60003)})
		c.R[cpu.BX] = h
		call(m, d, 0x21, 0x3E00)
		return got
	}
	if got := readFirst4("a.dat", 0); got != "layr" {
		t.Fatalf("疊層要蓋過 Root：讀到 %q", got)
	}
	if got := readFirst4("b.dat", 0); got != "root" {
		t.Fatalf("疊層沒有的檔讀 Root：讀到 %q", got)
	}
	if got := readFirst4("c.dat", 0); got != "scra" {
		t.Fatalf("暫存層要蓋過疊層：讀到 %q", got)
	}
	// 以讀寫模式開疊層裡的檔：複製到暫存層，疊層不動。
	if got := readFirst4("a.dat", 2); got != "layr" {
		t.Fatalf("讀寫模式開疊層檔讀到 %q", got)
	}
	copied := lookupDOS(scratch, "A.DAT") // 複製檔沿用程式給的大小寫
	if b, err := os.ReadFile(copied); copied == "" || err != nil || string(b) != "layr" {
		t.Fatalf("寫入模式要先複製到暫存層：%q %q %v", copied, b, err)
	}
	if b, _ := os.ReadFile(filepath.Join(layer, "A.DAT")); string(b) != "layr" {
		t.Fatal("疊層被改了")
	}
}
