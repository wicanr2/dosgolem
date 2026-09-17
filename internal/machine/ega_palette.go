package machine

// 200 線 EGA 模式（0Dh／0Eh）設模式時的屬性暫存器與 DAC 預設值（`docs/spec/194`）。
//
// **EGA 時代的程式只設屬性暫存器、從來不寫 DAC**——EGA 卡沒有 DAC。
// VGA BIOS 設這兩個模式時會先把 DAC 載成 CGA 相容的 RGBI 規則，
// 少了這一步，色號完全正確、畫面卻整片是黑的，而且不會報錯。

// load200LinePalette 套用 `194` §2：屬性暫存器 8–15 帶 bit 4（高亮），DAC 0–63 依 RGBI 規則。
// DAC 64 起不動。
func (m *Machine) load200LinePalette() {
	for i := 0; i < 8; i++ {
		m.VGA.ac[i] = uint8(i)
		m.VGA.ac[i+8] = uint8(i) | 0x10
	}
	m.VGA.ac[0x10] = 0x01 // mode control：圖形
	m.VGA.ac[0x12] = 0x0F // color plane enable
	for i := 0; i < 64; i++ {
		r, g, b := rgbiDAC(uint8(i))
		m.DAC[i*3+0], m.DAC[i*3+1], m.DAC[i*3+2] = r, g, b
	}
}

// rgbiDAC 把 6 位元色值照 200 線 RGBI 螢幕的接法轉成 DAC 的 6 位元 R、G、B。
//
// 只有 bit 0（B）、bit 1（G）、bit 2（R）、bit 4（高亮）接到螢幕；bit 3、bit 5 不接。
// 低亮度的「紅＋綠」是棕色：綠只給一半（`15h`），不是暗黃。
func rgbiDAC(v uint8) (r, g, b uint8) {
	hi := v>>4&1 == 1
	level := func(on bool) uint8 {
		switch {
		case on && hi:
			return 0x3F
		case on:
			return 0x2A
		case hi:
			return 0x15
		default:
			return 0x00
		}
	}
	bOn, gOn, rOn := v&1 != 0, v&2 != 0, v&4 != 0
	r, g, b = level(rOn), level(gOn), level(bOn)
	if !hi && rOn && gOn && !bOn {
		g = 0x15 // 棕色
	}
	return r, g, b
}
