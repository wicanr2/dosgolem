package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 規格 446：讀檔過濾掛點。程式讀到的是過濾後的 bytes，檔案本身不變。
func TestReadFilterRewritesGuestBytes(t *testing.T) {
	m, d := newTest(t)
	body := make([]byte, 64)
	for i := range body {
		body[i] = byte(i)
	}
	path := filepath.Join(d.Root, "FONT.FNT")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	var seen []int64
	d.ReadFilter = func(name string, pos int64, data []byte) {
		if name != "font.fnt" {
			t.Errorf("過濾器拿到的檔名 %q", name)
		}
		seen = append(seen, pos)
		for i := range data {
			data[i] ^= 0xFF
		}
	}
	m.WriteBytes(0x50000, append([]byte("font.fnt"), 0))
	c := m.CPU
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x5000, 0
	call(m, d, 0x21, 0x3D00)
	if c.Flags&cpu.CF != 0 {
		t.Fatalf("開檔失敗 AX=%04X", c.R[cpu.AX])
	}
	h := c.R[cpu.AX]
	c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = h, 0, 30
	call(m, d, 0x21, 0x4200)
	c.R[cpu.BX], c.R[cpu.CX] = h, 4
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x6000, 0
	call(m, d, 0x21, 0x3F00)
	if c.R[cpu.AX] != 4 {
		t.Fatalf("讀到 %d bytes", c.R[cpu.AX])
	}
	for i := 0; i < 4; i++ {
		if got, want := m.Read8(0x60000+uint32(i)), byte(30+i)^0xFF; got != want {
			t.Fatalf("第 %d 個 byte = %02X，該是過濾後的 %02X", i, got, want)
		}
	}
	if len(seen) != 1 || seen[0] != 30 {
		t.Fatalf("過濾器收到的起點 %v，該是 [30]", seen)
	}
	if after, _ := os.ReadFile(path); after[30] != 30 {
		t.Fatal("過濾不能改到主機上的檔案")
	}
	// 讀到檔尾：0 bytes 不呼叫過濾器。
	c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = h, 0, 64
	call(m, d, 0x21, 0x4200)
	c.R[cpu.BX], c.R[cpu.CX] = h, 4
	call(m, d, 0x21, 0x3F00)
	if c.R[cpu.AX] != 0 || len(seen) != 1 {
		t.Fatalf("檔尾讀到 %d bytes、過濾器被叫 %d 次", c.R[cpu.AX], len(seen))
	}
}
