package dos

import "github.com/wicanr2/dosgolem/internal/machine"

// `int 10h` 文字模式的游標、捲動與讀寫字元（`docs/spec/201`）。
//
// 行為照 DOSBox-X `int10_char.cpp`：游標位置只存在 BIOS 資料區，
// 字元直接落在 `B800` 的「字元＋屬性」兩個 byte 一格裡。

const bdaBase = 0x40 * 16

// bdaCursor 是某一頁游標在 BDA 的位址：先欄、後列。
func bdaCursor(page uint8) uint32 { return bdaBase + 0x50 + uint32(page)*2 }

// textMode 回報目前模式是不是 `B800` 的彩色文字模式（00h–03h）。
func (d *DOS) textMode() bool { return d.M.VideoMode() <= 0x03 }

// cursorOf 取某一頁的游標；`FFh` 表示目前顯示頁（BDA `62h`）。
func (d *DOS) cursorOf(page uint8) (row, col int, p uint8) {
	if page == 0xFF {
		page = d.M.Read8(bdaBase + 0x62)
	}
	at := bdaCursor(page)
	return int(d.M.Read8(at + 1)), int(d.M.Read8(at)), page
}

// textCell 是某一頁第 row 列第 col 欄的線性位址。
func (d *DOS) textCell(page uint8, row, col int) uint32 {
	cols := int(d.M.Read16(bdaBase + 0x4A))
	return uint32(machine.TextSeg)*16 + uint32(page)*uint32(d.M.Read16(bdaBase+0x4C)) + uint32((row*cols+col)*2)
}

// scrollText 捲動目前顯示頁上的一個視窗（含端點）。n > 0 往下捲、n < 0 往上捲、
// n == 0 或超過視窗列數就清整個視窗；空出來的列填「attr 屬性的空白」。
func (d *DOS) scrollText(top, left, bottom, right uint8, n int, attr uint8) {
	if top > bottom || left > right {
		return
	}
	rows := int(d.M.Read8(bdaBase+0x84)) + 1
	cols := int(d.M.Read16(bdaBase + 0x4A))
	t, l, b, r := int(top), int(left), int(bottom), int(right)
	if b >= rows {
		b = rows - 1
	}
	if r >= cols {
		r = cols - 1
	}
	base := uint32(machine.TextSeg)*16 + uint32(d.M.Read16(bdaBase+0x4E))
	cell := func(row, col int) uint32 { return base + uint32((row*cols+col)*2) }
	copyRow := func(dst, src int) {
		for col := l; col <= r; col++ {
			d.M.Write16(cell(dst, col), d.M.Read16(cell(src, col)))
		}
	}
	fillRow := func(row int) {
		for col := l; col <= r; col++ {
			d.M.Write16(cell(row, col), uint16(attr)<<8|0x20)
		}
	}

	height := b - t + 1
	if n == 0 || n > height || -n > height {
		for row := t; row <= b; row++ {
			fillRow(row)
		}
		return
	}
	if n < 0 { // 往上捲：由上而下複製，最下面 |n| 列清空
		n = -n
		for row := t; row+n <= b; row++ {
			copyRow(row, row+n)
		}
		for row := b - n + 1; row <= b; row++ {
			fillRow(row)
		}
		return
	}
	for row := b; row-n >= t; row-- { // 往下捲：由下而上複製，最上面 n 列清空
		copyRow(row, row-n)
	}
	for row := t; row < t+n; row++ {
		fillRow(row)
	}
}
