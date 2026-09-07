package machine

import "testing"

// planar 寫讀路徑的測試（`docs/spec/009` §5）。
//
// 釘的全是「畫面安靜地錯」的點：map mask、位元遮罩、latch、write mode 2。
// 每一條的反面都不會報錯，只會讓畫面缺一個 plane 或 XOR 變成蓋寫。

// newPlanar 造一台切到 mode 12h 的機器。
func newPlanar() *Machine {
	m := New()
	m.SetVideoMode(0x12)
	return m
}

// TestPlanarMapMaskSelectsPlanes：map mask 沒生效的話，每個 plane
// 都拿到同一份資料——顏色永遠是 15（四 plane 全有），看起來像「畫面太白」。
func TestPlanarMapMaskSelectsPlanes(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x01 // 只開 plane 0
	m.Write8(0xA0000, 0xFF)
	if m.vram[0][0] != 0xFF || m.vram[1][0] != 0 || m.vram[2][0] != 0 || m.vram[3][0] != 0 {
		t.Fatalf("map mask=1 時只有 plane 0 該寫到：%02X %02X %02X %02X",
			m.vram[0][0], m.vram[1][0], m.vram[2][0], m.vram[3][0])
	}
	// 平坦 Mem 也照寫（偵錯工具依賴它）。
	if m.Mem[0xA0000] != 0xFF {
		t.Error("平坦 Mem 沒寫到——-dump-linear 與 -watch 會看不到")
	}
}

// TestPlanarBitmaskMixesLatch：位元遮罩關掉的位元要保留 latch 值
// （＝上一次讀進來的 plane 內容）。漏掉的話 XOR 游標之類的
// read-modify-write 會把周圍像素清掉。
func TestPlanarBitmaskMixesLatch(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x0F
	// 先在四個 plane 放 0xF0（模擬既有畫面），再裝進 latch。
	for p := 0; p < 4; p++ {
		m.vram[p][0] = 0xF0
	}
	m.Read8(0xA0000) // 裝 latch
	m.gc[8] = 0x0F   // 只寫低 4 位
	m.Write8(0xA0000, 0x05)
	for p := 0; p < 4; p++ {
		if got := m.vram[p][0]; got != 0xF5 {
			t.Errorf("plane %d ＝ %02X，預期 F5（高 4 位保留 latch）", p, got)
		}
	}
}

// TestPlanarWriteMode2ExpandsBits：mode 2 把 CPU 資料的 bit p
// 展開成 plane p 的 0x00/0xFF——實心色塊填色走這條。
func TestPlanarWriteMode2ExpandsBits(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x0F
	m.gc[5] = 2             // write mode 2
	m.Write8(0xA0000, 0x05) // bit0＋bit2
	want := [4]uint8{0xFF, 0x00, 0xFF, 0x00}
	for p := 0; p < 4; p++ {
		if got := m.vram[p][0]; got != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X", p, got, want[p])
		}
	}
}

// TestPlanarReadLoadsLatchAndSelectsPlane：讀取要裝 latch（RMW 的前提）
// 且回 gc[4] 選的 plane。回錯 plane 的話 XOR 繪圖讀到的永遠是 plane 0。
func TestPlanarReadLoadsLatchAndSelectsPlane(t *testing.T) {
	m := newPlanar()
	m.vram[2][7] = 0x3C
	m.gc[4] = 2
	if got := m.Read8(0xA0007); got != 0x3C {
		t.Errorf("讀 plane 2 ＝ %02X，預期 3C", got)
	}
	for p := 0; p < 4; p++ {
		if m.latch[p] != m.vram[p][7] {
			t.Errorf("latch[%d] ＝ %02X，預期 %02X（讀取沒裝 latch）",
				p, m.latch[p], m.vram[p][7])
		}
	}
}

// TestPlanarRotateAndXOR：gc[3] 的旋轉與 XOR 函數（游標繪製慣用）。
func TestPlanarRotateAndXOR(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x01
	m.vram[0][0] = 0xFF
	m.Read8(0xA0000)   // latch ＝ FF
	m.gc[3] = 1 | 3<<3 // 右旋 1、XOR
	m.Write8(0xA0000, 0x01)
	// rot(01,1) ＝ 80；80 XOR FF ＝ 7F
	if got := m.vram[0][0]; got != 0x7F {
		t.Errorf("旋轉＋XOR ＝ %02X，預期 7F", got)
	}
}

// TestPlanarPixelsDecodes：四個 plane 各放一個 bit，第 0 像素要出色號 5。
func TestPlanarPixelsDecodes(t *testing.T) {
	m := newPlanar()
	m.vram[0][0] = 0x80 // 第 0 像素 bit
	m.vram[2][0] = 0x80
	px := m.PlanarPixels(640, 480)
	if px[0] != 0x05 {
		t.Errorf("pixel 0 ＝ %02X，預期 05", px[0])
	}
	if px[1] != 0 {
		t.Errorf("pixel 1 ＝ %02X，預期 00", px[1])
	}
}

// TestMode13Unaffected：mode 13h 的線性語意不能被 planar 機制動到。
func TestMode13Unaffected(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	m.Write8(0xA0000, 0x42)
	if got := m.Indexed()[0]; got != 0x42 {
		t.Errorf("mode 13h Indexed()[0] ＝ %02X，預期 42", got)
	}
	if m.vram[0][0] != 0 {
		t.Error("mode 13h 不該動到 plane")
	}
}

// TestPlanarWriteMode3MasksWithCPUByte：write mode 3 的 CPU 位元組是
// **遮罩**，顏色來自 set/reset（`docs/spec/010` §2）。
//
// 當成 mode 0 處理的話字形只會剩零星像素——而且不會報錯，只是字變醜，
// 很容易被當成「字型解碼還沒做完」而往錯的方向查。
func TestPlanarWriteMode3MasksWithCPUByte(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x0F
	m.gc[5] = 3    // write mode 3
	m.gc[0] = 0x09 // set/reset：plane 0 與 3 給 1
	m.gc[8] = 0xFF // 位元遮罩全開
	m.Write8(0xA0000, 0xC3)
	want := [4]uint8{0xC3, 0x00, 0x00, 0xC3}
	for p := 0; p < 4; p++ {
		if m.vram[p][0] != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X（CPU byte 是遮罩，顏色來自 gc[0]）",
				p, m.vram[p][0], want[p])
		}
	}
}

// TestPlanarWriteMode3AndsBitmaskRegister：有效遮罩是「旋轉後的 CPU
// 位元組 AND 位元遮罩暫存器」，兩者缺一都會多畫或少畫。
func TestPlanarWriteMode3AndsBitmaskRegister(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x01
	m.gc[5] = 3
	m.gc[0] = 0x01 // plane 0 給 1
	m.gc[8] = 0x0F // 只准動低 4 位
	m.Write8(0xA0000, 0xFF)
	if m.vram[0][0] != 0x0F {
		t.Errorf("plane 0 ＝ %02X，預期 0F（gc[8] 沒有被 AND 進去）", m.vram[0][0])
	}
}

// TestPlanarMode0EnableSetReset：mode 0 下 gc[1] 選中的 plane 資料
// 來自 gc[0]，不是 CPU 位元組（`docs/spec/010` §3）。
//
// 沒實作等於把 gc[1] 當永遠是 0：大面積填色會整片填成錯的色號，
// 而每一個像素本身都「有畫到」，所以看起來只是顏色怪，不像 bug。
func TestPlanarMode0EnableSetReset(t *testing.T) {
	m := newPlanar()
	m.seq[2] = 0x0F
	m.gc[5] = 0
	m.gc[1] = 0x03 // plane 0/1 走 set/reset
	m.gc[0] = 0x01 // plane 0 給 1、plane 1 給 0
	m.Write8(0xA0000, 0xFF)
	want := [4]uint8{0xFF, 0x00, 0xFF, 0xFF}
	for p := 0; p < 4; p++ {
		if m.vram[p][0] != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X", p, m.vram[p][0], want[p])
		}
	}
}
