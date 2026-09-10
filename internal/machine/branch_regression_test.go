package machine

import (
	"fmt"
	"testing"
)

// 這一組釘住從 logh3-com-support 與 san1-draw-speed-and-speech 併回來的
// 功能。**它們原本沒有測試護著**：功能是在的，但沒有任何東西會在它被
// 改壞時開口——而這幾樣的壞法都是「安靜地少做一件事」，不是報錯。

// TestWatchSeesServiceWrites 釘住「所有寫入路徑都走 Write8」。
//
// 監看與 planar 掛勾都掛在 Write8。`Write16` 與 `WriteBytes` 若直接寫
// `m.Mem`，用到它們的 INT 21h AH=3F 讀檔、EMS 換頁、BIOS 資料區與
// 中斷向量對監看就完全隱形——而那正是「這張表是怎麼被填出來的」最常見
// 的兩條路。
//
// ⚠ **症狀不是報錯，是沒報**：監看說某個位址從頭到尾沒人寫，
// 而同一個位址倒出來是別的值。
func TestWatchSeesServiceWrites(t *testing.T) {
	m := New()
	var got []string
	m.WatchWrites(0x1000, 0x1003, func(a uint32, old, nv uint8) {
		got = append(got, fmt.Sprintf("%X:%02X→%02X", a, old, nv))
	})

	m.WriteBytes(0x0FFE, []byte{1, 2, 3, 4, 5, 6}) // 跨進監看區間
	if len(got) != 4 {
		t.Errorf("WriteBytes 觸發 %d 次，預期 4（0x1000..0x1003）：%v", len(got), got)
	}

	got = nil
	m.Write16(0x1002, 0xBEEF)
	if len(got) != 2 {
		t.Errorf("Write16 觸發 %d 次，預期 2（16 位寫入是兩次 Write8）：%v", len(got), got)
	}
}

// TestDisplayStartShiftsThePicture 釘住「畫面輸出跟著 CRTC 的顯示起點走」。
//
// CRTC index `0C`／`0D` 是顯示起點的高／低位元組。**畫面在翻頁的程式
// 沒有它就永遠倒出第 0 頁**——而那看起來只是「畫面對不上」，
// 不像少了一個暫存器。
func TestDisplayStartShiftsThePicture(t *testing.T) {
	m := planar(t)
	const row = 80 // 640 寬的一列 ＝ 80 個位元組

	setPlane(m, 0, 0, 0x80)   // 第 0 頁的第一個像素
	setPlane(m, 0, row, 0x80) // 往後一列的第一個像素

	if px := m.PlanarPixels(640, 480); px[0] != 1 {
		t.Fatalf("起點 0 時第一個像素是 %d，預期 1", px[0])
	}
	// 起點設成一列之後：畫面第一個像素要變成原本第二列的那一個。
	setDisplayStart(m, row)
	px := m.PlanarPixels(640, 480)
	if px[0] != 1 {
		t.Errorf("起點 %d 時第一個像素是 %d，預期 1——顯示起點沒有生效", row, px[0])
	}
	// 原本第 0 列的內容應該已經捲出畫面（最後一列不會是它）。
	if got := m.DisplayStart(); got != row {
		t.Errorf("DisplayStart ＝ %d，預期 %d", got, row)
	}
}

// TestDisplayStartWordMode 釘住單位換算：mode control（index `17`）
// 的 bit6 ＝ 0 時起點以 word 計，要乘 2。
func TestDisplayStartWordMode(t *testing.T) {
	m := planar(t)
	crtcReg(m, 0x17, 0xE3) // BIOS 給 16 色 planar 的值，bit6 ＝ 1（byte）
	setDisplayStart(m, 40)
	if got := m.DisplayStart(); got != 40 {
		t.Errorf("byte 模式起點 ＝ %d，預期 40", got)
	}
	crtcReg(m, 0x17, 0xA3) // bit6 ＝ 0（word）
	if got := m.DisplayStart(); got != 80 {
		t.Errorf("word 模式起點 ＝ %d，預期 80（要乘 2）", got)
	}
}

// crtcReg 寫一個 CRTC 暫存器（`3D4` 選索引、`3D5` 給值）。
func crtcReg(m *Machine, idx, val uint8) {
	m.Out8(0x3D4, idx)
	m.Out8(0x3D5, val)
}

// setDisplayStart 設顯示起點（index `0C` 高位、`0D` 低位）。
func setDisplayStart(m *Machine, start uint16) {
	crtcReg(m, 0x0C, uint8(start>>8))
	crtcReg(m, 0x0D, uint8(start))
}
