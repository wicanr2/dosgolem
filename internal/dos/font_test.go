package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestFullIndex 釘住 Big5 → 字模格號（`docs/spec/008` §3.2）。
//
// 驗收數字來自臥龍傳專案 `docs/re/29` §5：`END_S13.DAT` 是
// 「408 格符號區 ＋ 倚天 `stdfont.15`」，所以漢字的格號**一律是
// `stdfont.15` 的序號 ＋ 408**。兩個獨立來源給同一個 408，
// 這是這條式子最硬的那一段。
func TestFullIndex(t *testing.T) {
	cases := []struct {
		name     string
		ch, cl   uint8
		want     int
		why      string
	}{
		{"一 A440", 0xA4, 0x40, 408, "漢字區第一個字 ＝ 符號區 408 格之後"},
		{"國 B0EA", 0xB0, 0xEA, 2428, "低位元組 > 7Eh 要先減 22h 接成連續"},
		{"龍 C073", 0xC0, 0x73, 4855, ""},
		{"符號區 A140", 0xA1, 0x40, 0, "第一段：ch < A4h"},
		{"第三段 C940", 0xC9, 0x40, 0x16B1, "16B1h ＝ 5809"},
		{"越界 FA40", 0xFA, 0x40, 0x56, "越界固定回第 56h 格，**不加低位元組**"},
		{"越界 FAFE", 0xFA, 0xFE, 0x56, "越界那一支是 jmp 到乘法之前"},
	}
	for _, c := range cases {
		if got := fullIndex(c.ch, c.cl); got != c.want {
			t.Errorf("%s → %d，預期 %d %s", c.name, got, c.want, c.why)
		}
	}
}

// TestFontServiceHandsOutCallableStubs 釘住 `INT 15h AH=50h` 交出去的東西
// 真的是一段**可以 `call far` 的程式碼**。
//
// 回一個位址而那裡不是 `int/retf` 的話，遊戲的第一次畫字就會飛掉，
// 而症狀是「記憶體不足」——離開碼與根因差了 200 萬道指令。
func TestFontServiceHandsOutCallableStubs(t *testing.T) {
	m, d := newTest(t)
	for _, tc := range []struct {
		bh   uint16
		want uint16
	}{{0x00, fontHalfOff}, {0x01, fontFullOff}} {
		m.CPU.R[cpu.BX] = tc.bh << 8
		call(m, d, 0x15, 0x5000)
		if m.CPU.Seg[cpu.ES] != machine.StubSeg || m.CPU.R[cpu.BX] != tc.want {
			t.Fatalf("BH=%d 回 %04X:%04X，預期 %04X:%04X",
				tc.bh, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX], machine.StubSeg, tc.want)
		}
		if m.CPU.Flags&cpu.CF != 0 {
			t.Errorf("BH=%d 回了 CF=1——呼叫端會直接放棄，向量停在 0000:0000", tc.bh)
		}
		base := uint32(machine.StubSeg)*16 + uint32(tc.want)
		if m.Read8(base) != 0xCD || m.Read8(base+2) != 0xCB {
			t.Errorf("BH=%d 交出去的位址上不是 `int n; retf`（%02X %02X %02X）",
				tc.bh, m.Read8(base), m.Read8(base+1), m.Read8(base+2))
		}
	}
}

// TestGlyphCopiesAndPadsRow16 釘住取字模常式的兩件事：
// 讀對位移，以及**第 16 列固定補 0**（宣告 16 列、實存 15 列）。
func TestGlyphCopiesAndPadsRow16(t *testing.T) {
	dir := t.TempDir()
	// 造一份假的全形字型：第 n 格的 30 個 byte 全部是 n。
	full := make([]byte, 30*5000)
	for n := 0; n < 5000; n++ {
		for i := 0; i < 30; i++ {
			full[n*30+i] = uint8(n)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "END_S13.DAT"), full, 0o644); err != nil {
		t.Fatal(err)
	}
	half := make([]byte, 15*256)
	for n := 0; n < 256; n++ {
		for i := 0; i < 15; i++ {
			half[n*15+i] = uint8(255 - n)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "END_S14.DAT"), half, 0o644); err != nil {
		t.Fatal(err)
	}

	m, d := newTest(t)
	d.Root = dir
	const buf = 0x2000
	// 先把緩衝區塗成 FF，才分得出「寫進去的 0」與「本來就是 0」。
	for i := 0; i < 40; i++ {
		m.Write8(buf+uint32(i), 0xFF)
	}
	m.CPU.Seg[cpu.ES] = 0x0200
	m.CPU.R[cpu.SI] = 0
	m.CPU.R[cpu.CX] = 0xA440 // 「一」＝ 第 408 格
	call(m, d, intFontFull, 0)

	for i := 0; i < 30; i++ {
		if got := m.Read8(buf + uint32(i)); got != uint8(408 & 0xFF) {
			t.Fatalf("字模第 %d 個 byte ＝ %02X，預期 %02X", i, got, uint8(408 & 0xFF))
		}
	}
	if m.Read8(buf+30) != 0 || m.Read8(buf+31) != 0 {
		t.Errorf("第 16 列沒補 0（%02X %02X）", m.Read8(buf+30), m.Read8(buf+31))
	}
	if m.Read8(buf+32) != 0xFF {
		t.Error("寫超過 32 byte——呼叫端只在堆疊上開了 32 byte")
	}

	// 半形：'A' ＝ 第 65 格，15 byte ＋ 一個 0。
	m.CPU.R[cpu.CX] = 0x0041
	call(m, d, intFontHalf, 0)
	if got := m.Read8(buf); got != uint8(255-65) {
		t.Errorf("半形字模 ＝ %02X，預期 %02X", got, uint8(255-65))
	}
	if m.Read8(buf+15) != 0 {
		t.Error("半形第 16 列沒補 0")
	}
	if d.Font.Calls[0] != 1 || d.Font.Calls[1] != 1 {
		t.Errorf("常式呼叫統計 ＝ %v，預期各 1 次", d.Font.Calls)
	}
	if d.Font.Missing != 0 {
		t.Errorf("有 %d 次讀不到字模", d.Font.Missing)
	}
}
