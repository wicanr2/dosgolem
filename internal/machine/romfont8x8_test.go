package machine

import "testing"

// F000:FA6E 放 8×8 字型（`docs/spec/199-rom-font-8x8`）。
//
// 反面的症狀：BGI 的 DEFAULT_FONT 有 37 個字元讀這裡，讀到 0 就畫成空白，
// 數字與標點消失，而字母照常——看起來像「程式漏印」。
func TestROMFont8x8AtFA6E(t *testing.T) {
	m := New()
	const base = 0xFFA6E
	for ch := 0; ch < 128; ch++ {
		for row := 0; row < 8; row++ {
			if got, want := m.Read8(uint32(base+ch*8+row)), romFont8x8[ch][row]; got != want {
				t.Fatalf("字元 %02X 第 %d 列：%02X，預期 %02X", ch, row, got, want)
			}
		}
	}
	if m.Read8(base+'A'*8) != 0x30 { // font8x8 的 0x0C 反轉位元順序
		t.Fatalf("'A' 第一列是 %02X，預期 30", m.Read8(base+'A'*8))
	}
	if m.Read8(base+'1'*8+1) == 0 {
		t.Fatal("'1' 的字形是空的")
	}
}
