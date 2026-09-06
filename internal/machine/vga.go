package machine

// VGA/EGA planar 寫讀路徑（`docs/spec/009` §1）。
//
// 平坦 Mem 的 A0000 段照樣寫（偵錯工具用），但 planar 模式下畫面的
// 真相在四個 plane。CPU 的讀寫都走 Write8／Read8，所以在這裡攔就夠了
// （Write16／WriteBytes 是載入器與 DOS 層用的，不碰 VRAM 語意）。

// planarVideo 回 planar 模式是否生效。
func (m *Machine) planarVideo() bool {
	switch m.VideoMode() {
	case 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12:
		return true
	}
	return false
}

// planarWrite 實作 VGA GC 的寫路徑（`docs/spec/010` §1）。
//
// 四種 write mode 的差別只在「每個 plane 拿到什麼資料」與「遮罩從哪來」，
// 之後的 ALU、位元遮罩與 map mask 是共通的。
func (m *Machine) planarWrite(off uint32, v uint8) {
	off &= 0xFFFF
	gc := &m.gc
	wm := gc[5] & 3
	m.WriteModeUse[wm]++
	rot := func(b uint8) uint8 { // gc[3] 低 3 位：右旋
		n := gc[3] & 7
		return b>>n | b<<(8-n)
	}
	// gc[3] bit3-4：00 replace、01 AND、10 OR、11 XOR（對 latch）
	alu := func(d, l uint8) uint8 {
		switch (gc[3] >> 3) & 3 {
		case 1:
			return d & l
		case 2:
			return d | l
		case 3:
			return d ^ l
		}
		return d
	}
	expand := func(b uint8, p int) uint8 { // 取 bit p 展成 0x00/0xFF
		if b>>uint(p)&1 != 0 {
			return 0xFF
		}
		return 0
	}
	// write mode 3 的位元遮罩是「旋轉後的 CPU 資料 AND 位元遮罩暫存器」，
	// 資料則一律來自 set/reset；CPU 的 byte 在這個模式裡是遮罩不是顏色。
	bitmask := gc[8]
	if wm == 3 {
		bitmask &= rot(v)
	}
	mapMask := m.seq[2] & 0x0F
	for p := 0; p < 4; p++ {
		if mapMask>>uint(p)&1 == 0 {
			continue
		}
		var d uint8
		switch wm {
		case 1: // latch → plane，位元遮罩與 ALU 都不參與
			m.vram[p][off] = m.latch[p]
			continue
		case 2: // CPU 資料的 bit p 展開成 0x00/0xFF
			d = expand(v, p)
		case 3: // 資料來自 set/reset（gc[0]），與 enable 無關
			d = expand(gc[0], p)
		default: // mode 0：enable set/reset（gc[1]）挑的 plane 用 gc[0]
			if gc[1]>>uint(p)&1 != 0 {
				d = expand(gc[0], p)
			} else {
				d = rot(v)
			}
		}
		d = alu(d, m.latch[p])
		m.vram[p][off] = d&bitmask | m.latch[p]&^bitmask
	}
}

// planarRead 實作讀路徑：任何讀都先裝 latch，再依 read mode 回值。
func (m *Machine) planarRead(off uint32) uint8 {
	off &= 0xFFFF
	for p := 0; p < 4; p++ {
		m.latch[p] = m.vram[p][off]
	}
	// read mode 0（gc[5] bit3 ＝ 0）：回 gc[4] 選的 plane。
	// read mode 1（color compare）不實作，回 plane 值（spec §4）。
	return m.vram[m.gc[4]&3][off]
}

// PlanarPixels 把四個 plane 解成 8 位元色號陣列（每像素 4 位元）。
// w/h 由呼叫端依模式給（12h ＝ 640×480、10h ＝ 640×350）。
// 假設線性 plane、無 CRTC 位移（spec §4）。
func (m *Machine) PlanarPixels(w, h int) []uint8 {
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := uint32(y*(w/8) + x/8)
			bit := uint(7 - x%8)
			var px uint8
			for p := 0; p < 4; p++ {
				px |= (m.vram[p][off] >> bit & 1) << uint(p)
			}
			out[y*w+x] = px
		}
	}
	return out
}
