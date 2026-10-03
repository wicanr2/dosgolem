package oracle

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// CGAScroll 寫入的位元組由 CGA4() 讀回，與預期像素相同（版面一致性）：
// 在掃描線 3（奇數線，bank 2000h）與掃描線 4（偶數線）各畫一個已知圖樣，上捲 1 列後出現在上一個字元格。
func TestCGA4ReadsWhatCGAScrollWrote(t *testing.T) {
	m := machine.New()
	m.SetVideoMode(0x04)
	o := &Oracle{m: m}
	// 字元格 (列 1, 欄 2) 的第 3 條掃描線（y = 1*8 + 3 = 11，奇數線）第一個位元組 = 0b11100100：像素 3,2,1,0
	// 位址以字面算術：B8000 + (11/2)*80 + (11%2)*0x2000 + 2*2
	m.Mem[0xB8000+(11/2)*80+(11%2)*0x2000+2*2] = 0xE4
	m.CGAScroll(0, 2, 24, 2, 1, false, 0) // 欄 2 的整列上捲 1 列
	idx := o.CGA4()
	// 上捲後該位元組在 y = 3（0*8 + 3）
	got := [4]uint8{idx[3*Width+2*8], idx[3*Width+2*8+1], idx[3*Width+2*8+2], idx[3*Width+2*8+3]}
	if got != [4]uint8{3, 2, 1, 0} {
		t.Errorf("上捲後 (y=3, x=16..19) 的色號 = %v，要 [3 2 1 0]", got)
	}
	if v := idx[11*Width+2*8]; v != 0 {
		t.Errorf("原位置 (y=11) 應被填成 0，得 %d", v)
	}
}

// CGAPalette 與 CGA4RGB：四個色號各一塊，換算成預設調色盤（調色盤 1、高強度、背景黑）的字面 RGB。
func TestCGA4RGBMatchesPalette(t *testing.T) {
	m := machine.New()
	m.SetVideoMode(0x04)
	o := &Oracle{m: m}
	// 第 0 條掃描線的前 4 個像素設成色號 0、1、2、3（0b00011011 = 1B）
	m.Mem[0xB8000] = 0x1B
	rgb := o.CGA4RGB()
	want := [4][3]uint8{{0, 0, 0}, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}
	for i := 0; i < 4; i++ {
		got := [3]uint8{rgb[3*i], rgb[3*i+1], rgb[3*i+2]}
		if got != want[i] {
			t.Errorf("像素 %d 的 RGB = %v，要 %v", i, got, want[i])
		}
	}
	if p := o.CGAPalette(); p != want {
		t.Errorf("CGAPalette = %v，要 %v", p, want)
	}
	m.SetColorSelect(0x00) // 調色盤 0 無強度：綠、紅、棕
	rgb = o.CGA4RGB()
	want = [4][3]uint8{{0, 0, 0}, {0, 170, 0}, {170, 0, 0}, {170, 85, 0}}
	for i := 0; i < 4; i++ {
		got := [3]uint8{rgb[3*i], rgb[3*i+1], rgb[3*i+2]}
		if got != want[i] {
			t.Errorf("調色盤 0：像素 %d 的 RGB = %v，要 %v", i, got, want[i])
		}
	}
}
