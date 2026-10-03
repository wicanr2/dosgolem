package machine

// CGA 圖形模式（04h、05h：320×200，每像素 2 位元，40×25 個 8×8 字元格）的版面、視窗捲動與色彩選擇
// （docs/spec/250-cga-int10-scroll-and-palette）。

const (
	cgaBase    = 0xB8000 // B800:0000
	cgaOddBank = 0x2000  // 奇數掃描線的 bank 位移
	cgaStride  = 80      // 320 像素 × 2 位元 ÷ 8
	cgaCols    = 40
	cgaRows    = 25
	cgaCellH   = 8

	// bdaColorSelect 是 BDA `0040:0066`，色彩選擇暫存器（位元定義同 3D9 埠）：
	// bit 0–3 背景色（16 色之一），bit 4 前景高強度，bit 5 調色盤（0 綠紅棕，1 青洋紅淺灰）。
	// 存在 BDA 內，所以 Snapshot 與 SaveState 自動涵蓋，不需要新的 Machine 欄位。
	bdaColorSelect = 0x66
	// cgaDefaultColorSelect 是設定模式 04h、05h 之後的初值：調色盤 1、高強度、背景黑。
	cgaDefaultColorSelect = 0x30
)

// CGAScanlineOffset 回第 y 條掃描線（0 至 199）在 B800 段內的位移：偶數線在 y/2 × 80，奇數線另加 2000h。
// oracle.CGA4 與視窗捲動共用這一份（不複製常數）。
func CGAScanlineOffset(y int) uint32 {
	return uint32(y/2)*cgaStride + uint32(y%2)*cgaOddBank
}

// CGARGB16 是 CGA 的 16 色 RGB（索引 0 黑、1 藍、2 綠、3 青、4 紅、5 洋紅、6 棕、7 淺灰、
// 8 深灰、9 淺藍、10 淺綠、11 淺青、12 淺紅、13 淺洋紅、14 黃、15 白）。
var CGARGB16 = [16][3]uint8{
	{0, 0, 0}, {0, 0, 170}, {0, 170, 0}, {0, 170, 170},
	{170, 0, 0}, {170, 0, 170}, {170, 85, 0}, {170, 170, 170},
	{85, 85, 85}, {85, 85, 255}, {85, 255, 85}, {85, 255, 255},
	{255, 85, 85}, {255, 85, 255}, {255, 255, 85}, {255, 255, 255},
}

func isCGAGraphics(mode uint8) bool { return mode == 0x04 || mode == 0x05 }

// IsCGAGraphics 回目前視訊模式是不是 CGA 圖形模式（04h 或 05h）。
func (m *Machine) IsCGAGraphics() bool { return isCGAGraphics(m.VideoMode()) }

// ColorSelect 讀 BDA 的色彩選擇暫存器。
func (m *Machine) ColorSelect() uint8 { return m.Mem[bdaSeg*16+bdaColorSelect] }

// SetColorSelect 寫 BDA 的色彩選擇暫存器。
func (m *Machine) SetColorSelect(v uint8) { m.Mem[bdaSeg*16+bdaColorSelect] = v }

// CGAPalette 回目前模式四個色號的 RGB：色號 0 是背景色（色彩選擇的 bit 0–3），色號 1 至 3 依 bit 5 與
// bit 4 取：調色盤 0 為綠、紅、棕，調色盤 1 為青、洋紅、淺灰；高強度時各加 8（淺綠、淺紅、黃；淺青、淺洋紅、白）。
// 模式不是 04h、05h 時回預設值（調色盤 1、高強度、背景黑）。模式 05h 暫用與 04h 相同的色表（未驗證）。
func (m *Machine) CGAPalette() [4][3]uint8 {
	cs := uint8(cgaDefaultColorSelect)
	if m.IsCGAGraphics() {
		cs = m.ColorSelect()
	}
	first := uint8(2) // 調色盤 0：綠 2、紅 4、棕 6
	if cs&0x20 != 0 {
		first = 3 // 調色盤 1：青 3、洋紅 5、淺灰 7
	}
	hi := uint8(0)
	if cs&0x10 != 0 {
		hi = 8
	}
	return [4][3]uint8{
		CGARGB16[cs&0x0F],
		CGARGB16[first+hi],
		CGARGB16[first+2+hi],
		CGARGB16[first+4+hi],
	}
}

// CGAScroll 是 BIOS INT 10h AH=06h（down=false）與 AH=07h（down=true）在 CGA 圖形模式的實作。
// top、left、bottom、right 是視窗的字元格座標（CH、CL、DH、DL），lines 是 AL，fill 是 BH。
//
// 語意依 DOSBox-X `INT10_ScrollWindow`（`src/ints/int10_char.cpp`）：先檢查上緣大於下緣或左緣大於右緣，
// 成立就不動作（先比較，後夾邊）；再夾到螢幕（列 0–24、欄 0–39）。填色取 fill 的低 2 位元複製到一個位元組的
// 4 個像素。lines 為 0，或不小於視窗列數：整個視窗填色。其他：視窗內容逐條掃描線複製（只動視窗橫向範圍的位元組），
// 空出的 lines 列填色；上捲由上往下複製、下捲由下往上複製，避免來源被覆蓋。
// 寫入走 Write8（保留寫入觀察者的通知）。不改暫存器與游標。
func (m *Machine) CGAScroll(top, left, bottom, right, lines int, down bool, fill byte) {
	if top > bottom || left > right {
		return
	}
	if bottom >= cgaRows {
		bottom = cgaRows - 1
	}
	if right >= cgaCols {
		right = cgaCols - 1
	}
	if top > bottom || left > right { // 夾邊後視窗已不存在（例如左緣本來就在螢幕外）
		return
	}
	rows := bottom - top + 1
	x0, x1 := left*2, (right+1)*2
	attr := (fill & 3) * 0x55

	fillRow := func(r int) {
		for y := r * cgaCellH; y < (r+1)*cgaCellH; y++ {
			base := cgaBase + CGAScanlineOffset(y)
			for x := x0; x < x1; x++ {
				m.Write8(base+uint32(x), attr)
			}
		}
	}
	copyRow := func(src, dst int) {
		for i := 0; i < cgaCellH; i++ {
			s := cgaBase + CGAScanlineOffset(src*cgaCellH+i)
			d := cgaBase + CGAScanlineOffset(dst*cgaCellH+i)
			for x := x0; x < x1; x++ {
				m.Write8(d+uint32(x), m.Read8(s+uint32(x)))
			}
		}
	}

	if lines == 0 || lines >= rows {
		for r := top; r <= bottom; r++ {
			fillRow(r)
		}
		return
	}
	if !down {
		for r := top + lines; r <= bottom; r++ {
			copyRow(r, r-lines)
		}
		for r := bottom - lines + 1; r <= bottom; r++ {
			fillRow(r)
		}
		return
	}
	for r := bottom - lines; r >= top; r-- {
		copyRow(r, r+lines)
	}
	for r := top; r < top+lines; r++ {
		fillRow(r)
	}
}
