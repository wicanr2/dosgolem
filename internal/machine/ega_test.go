package machine

import "testing"

// EGA 的平面式 VRAM（`docs/spec/007`／`011`）。
//
// 這一組釘的是**沒有呼叫 `int 10h AH=00` 的程式**那條路：Pool of Radiance
// 直接設暫存器，BDA 的模式位元組一路是 03h，只認模式的話它永遠走不到
// 平面路徑——而四次寫入疊在同一段線性記憶體上，畫面看起來仍是一張圖。

const (
	testPlanes     = 4
	testMapMaskIdx = 0x02
	testMapMaskAll = 0x0F
)

// 序列器的 Map Mask 決定一次寫入落在哪些平面上（`docs/spec/007` §3）。
// 四個平面各寫一次同一個位址，組出來的色號要是四個位元的疊加。
func TestMapMaskRoutesWritesToPlanes(t *testing.T) {
	m := New()
	// 一次只開一個平面，四次都寫同一個位址、同一個位元。
	// 位元 7（最左邊那個像素）在四個平面都是 1 → 色號 0Fh。
	for plane := 0; plane < testPlanes; plane++ {
		m.Out8(0x3C4, testMapMaskIdx)
		m.Out8(0x3C5, uint8(1<<uint(plane)))
		m.Write8(vgaLo, 0x80)
	}
	pixels := m.IndexedEGA()
	if pixels[0] != 0x0F {
		t.Errorf("四個平面都寫了，第一個像素是 %#x，預期 0F", pixels[0])
	}
	for x := 1; x < 8; x++ {
		if pixels[x] != 0 {
			t.Errorf("第 %d 個像素是 %#x，只有位元 7 被寫，其餘應該是 0", x, pixels[x])
		}
	}

	// 只開平面 0 與 2 → 色號 5。
	m2 := New()
	m2.Out8(0x3C4, testMapMaskIdx)
	m2.Out8(0x3C5, 0x05)
	m2.Write8(vgaLo, 0xFF)
	pixels = m2.IndexedEGA()
	for x := 0; x < 8; x++ {
		if pixels[x] != 0x05 {
			t.Fatalf("遮罩 05h 寫 FFh，第 %d 個像素是 %#x，預期 05", x, pixels[x])
		}
	}
}

// 位元順序：位元 7 是最左邊那個像素。方向弄反的話畫面會左右鏡射，
// 而那看起來仍然像一張正常的圖。
func TestEGABitOrderIsLeftmostFirst(t *testing.T) {
	m := New()
	m.Out8(0x3C4, testMapMaskIdx)
	m.Out8(0x3C5, 0x01)
	m.Write8(vgaLo, 0x81) // 位元 7 與位元 0
	pixels := m.IndexedEGA()
	if pixels[0] != 1 || pixels[7] != 1 {
		t.Errorf("81h 應該點亮第 0 與第 7 個像素，得到 %v", pixels[:8])
	}
	for x := 1; x < 7; x++ {
		if pixels[x] != 0 {
			t.Errorf("第 %d 個像素不該亮", x)
		}
	}
	// 第二列從 320÷8 ＝ 40 bytes 之後開始。
	m.Write8(vgaLo+VideoWidth/8, 0x80)
	pixels = m.IndexedEGA()
	if pixels[VideoWidth] != 1 {
		t.Errorf("第二列第一個像素是 %d，預期 1", pixels[VideoWidth])
	}
}

// EGAPlanarActive 在「模式是平面模式」或「Map Mask 被寫成不是 0Fh 的值」
// 之後成立。重置值 0Fh 與 mode 13h 的線性寫入相容，所以它本身不算訊號。
func TestPlanarActiveNeedsANonDefaultMask(t *testing.T) {
	m := New()
	if m.EGAPlanarActive() {
		t.Error("什麼都還沒寫就說是平面式")
	}
	if m.MapMask() != testMapMaskAll {
		t.Errorf("Map Mask 的重置值是 %#x，預期 0F", m.MapMask())
	}
	m.Out8(0x3C4, testMapMaskIdx)
	m.Out8(0x3C5, testMapMaskAll)
	if m.EGAPlanarActive() {
		t.Error("寫 0Fh 不該算訊號——它就是重置值")
	}
	m.Out8(0x3C5, 0x02)
	if !m.EGAPlanarActive() {
		t.Error("寫 02h 之後應該算平面式")
	}
	// 索引不是 02 的時候寫 3C5 不該動到 Map Mask。
	m.Out8(0x3C4, 0x01)
	m.Out8(0x3C5, 0x00)
	if m.MapMask() != 0x02 {
		t.Errorf("索引 01 的寫入動到了 Map Mask：%#x", m.MapMask())
	}
}

// TestPlanarModeAloneIsEnough：明講的平面模式不必等 Map Mask 訊號。
func TestPlanarModeAloneIsEnough(t *testing.T) {
	m := New()
	m.SetVideoMode(0x10)
	if !m.EGAPlanarActive() {
		t.Fatal("mode 10h 應該直接就是平面式")
	}
	if m.VGA.PlanarSeen() {
		t.Error("只是設模式，不該記成 Map Mask 訊號")
	}
}

// **mode 13h 不能被這一條動到。** 程式明講要線性緩衝區時，
// Map Mask 訊號不得推翻它。
func TestModeThirteenIndexedIsUnaffected(t *testing.T) {
	m := New()
	m.Out8(0x3C4, testMapMaskIdx)
	m.Out8(0x3C5, 0x04) // 只開平面 2 → 訊號成立
	m.SetVideoMode(0x13)
	if m.EGAPlanarActive() {
		t.Fatal("mode 13h 之後不該還是平面式——程式明講要線性")
	}
	m.Write8(vgaLo+10, 0x2A)
	if got := m.Indexed()[10]; got != 0x2A {
		t.Errorf("mode 13h 的第 10 個色號是 %#x，遮罩不該影響它（預期 2A）", got)
	}
	if got := m.Mem[vgaLo+10]; got != 0x2A {
		t.Errorf("mode 13h 的寫入沒進線性記憶體（%02X）", got)
	}
}
