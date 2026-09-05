package wolong

import "testing"

// TestHotzoneBoxes 釘住熱區圖的解讀：每格 8×8 像素、每列 80 個 byte。
//
// ⚠ **編號 0 是「沒有熱區」，不是「第 0 號熱區」。** 把它算進去的話
// 每一張圖都會多出一個覆蓋整個畫面的框，而那看起來很像「到處都能點」。
func TestHotzoneBoxes(t *testing.T) {
	m := make([]byte, HotzoneCols*HotzoneRows)
	// 4 號熱區佔 x 432..463、y 0..31（＝《臥龍傳》的系統開關，
	// 與臥龍傳專案 `docs/playtest/39` 記的值一致）。
	for row := 0; row < 4; row++ {
		for col := 54; col <= 57; col++ {
			m[row*HotzoneCols+col] = 4
		}
	}
	boxes := hotzoneBoxes(m)
	if len(boxes) != 1 {
		t.Fatalf("回了 %d 個熱區，預期 1（編號 0 不算）", len(boxes))
	}
	b := boxes[0]
	if b.ID != 4 || b.X != 432 || b.Y != 0 || b.W != 32 || b.H != 32 {
		t.Errorf("熱區 ＝ #%d x %d y %d %d×%d，預期 #4 x 432 y 0 32×32",
			b.ID, b.X, b.Y, b.W, b.H)
	}
	if b.Cells != 16 {
		t.Errorf("格數 ＝ %d，預期 16", b.Cells)
	}
}

// TestHotzoneBoxesShortMap 釘住「讀不到就回 nil」。
//
// **回一張空的熱區圖會讓「還沒登記」看起來像「這裡不能點」**，
// 而那兩件事的處置完全不同。
func TestHotzoneBoxesShortMap(t *testing.T) {
	if boxes := hotzoneBoxes(make([]byte, 10)); boxes != nil {
		t.Errorf("圖太短卻回了 %d 個熱區", len(boxes))
	}
}
