package machine

// BIOS 資料區（`0040:0000`–`0040:00FF`）。
//
// **這不是可有可無的擺設。** BASIC runtime 的 `SCREEN` 直接讀
// `0040:0010`（裝置旗標）判斷顯示卡；沒建的話那裡是 0，判斷結果錯了
// 也不會有任何錯誤訊息——症狀是很後面一個 `Illegal function call`。
//
// 溯源鏈（`rich2/docs/re/005`「缺的是 BIOS 資料區」）：
//
//	錯誤訊息的呼叫端 → 0x10934 的 `push 1 / push 0Dh / push 2`（0Dh ＝ mode 13h）
//	→ BASIC runtime 0x306A0 → `mov al, ds:410h`
//
// 取值沿用 `rich2/tools/dosemu.py` 的 `_init_bda`——**它是實際把 `RUN.EXE`
// 跑到資產全部載完的那一份**（`DATA.PAK`／`PART1.PAK`／`SAVE_7.DSK`／
// `RICHA.RIX` 都開了），不是照手冊重新挑的。

const bdaSeg = 0x0040

func (m *Machine) initBDA() {
	base := uint32(bdaSeg * 16)
	w := func(off uint32, v uint16) { m.Write16(base+off, v) }
	b := func(off uint32, v uint8) { m.Mem[base+off] = v }

	w(0x00, 0x03F8) // COM1
	w(0x08, 0x0378) // LPT1

	// 裝置旗標。**這一格是決定性的那一個。**
	//   bit 0    ＝ 1：有軟碟
	//   bit 1    ＝ 0：**沒有數學共處理器**——本程式的浮點走自己內建的
	//                  Microsoft 模擬器（876 個 `INT 34h–3Dh`），
	//                  報告有 x87 反而可能讓它走別條路
	//   bit 4–5  ＝ 00：初始視訊模式 ＝ EGA 或更新
	//   bit 6–7  ＝ 00：一台軟碟
	w(0x10, 0x0001)

	w(0x13, 640)    // 常規記憶體 KB
	w(0x1A, 0x001E) // 鍵盤緩衝區頭
	w(0x1C, 0x001E) // 鍵盤緩衝區尾

	b(0x49, 0x03)   // 目前視訊模式（開機是文字模式 3；SCREEN 13 之後由 int 10h 改）
	w(0x4A, 80)     // 欄數
	w(0x4C, 4096)   // 視訊分頁大小
	w(0x63, 0x03D4) // CRTC 基底埠（彩色）
	b(0x65, 0x29)   // CRT 模式暫存器
	b(0x66, 0x30)   // 調色盤暫存器

	w(0x80, 0x001E) // 鍵盤緩衝區起點
	w(0x82, 0x003E) // 鍵盤緩衝區終點

	b(0x84, 24) // 列數 − 1
	w(0x85, 16) // 字元高度
	b(0x87, 0x60) // EGA 資訊
	b(0x88, 0xF9) // EGA 其他
	b(0x89, 0x51) // VGA 旗標：400 掃描線
	b(0x8A, 0x08) // 顯示卡組合碼：VGA 彩色
}

// SetVideoMode 把目前模式記進 BDA（`0040:0049`）。
// `int 10h AH=00h` 與 `AH=0Fh` 兩邊都讀它，所以只留這一份。
func (m *Machine) SetVideoMode(mode uint8) {
	m.Mem[bdaSeg*16+0x49] = mode
	// mode 13h 是 320×200；欄數要跟著改，`AH=0Fh` 會回它。
	if mode == 0x13 {
		m.Write16(bdaSeg*16+0x4A, 40)
	}
}

// VideoMode 讀回目前模式。
func (m *Machine) VideoMode() uint8 { return m.Mem[bdaSeg*16+0x49] }

// 鍵盤緩衝區（BDA `0040:001E`–`0040:003D`，16 筆 word）。
//
// 為什麼要真的做這個環形緩衝，而不是讓 `int 16h` 直接回一個鍵：
// **很多程式不走 `int 16h`，直接讀 BDA 的頭尾指標判斷「有沒有按鍵」。**
// 三國志（DOS 版）就是——`MAIN.EXE` 在 `0110:1AF6` 一帶把 `DS` 設成 `0x40`
// 之後比對 `001A`／`001C`，兩者相等就繼續等。只補 `int 16h` 的話它一道都不會動，
// 而且表面上完全正常（沒有未實作服務、沒有錯誤、CPU 一直在跑）。
//
// 頭 ＝ 下一個要讀的位置，尾 ＝ 下一個要寫的位置；頭 == 尾 表示空。
// 尾追上頭表示滿——**留一格不用**，否則滿和空分不出來。

const (
	kbHead  = 0x1A
	kbTail  = 0x1C
	kbStart = 0x1E
	kbEnd   = 0x3E
)

func (m *Machine) kbAdvance(p uint16) uint16 {
	p += 2
	if p >= kbEnd {
		p = kbStart
	}
	return p
}

// PushKey 把一筆「掃描碼 ＋ ASCII」放進鍵盤緩衝區；滿了回 false。
func (m *Machine) PushKey(scan, ascii uint8) bool {
	base := uint32(bdaSeg * 16)
	tail := m.Read16(base + kbTail)
	next := m.kbAdvance(tail)
	if next == m.Read16(base+kbHead) {
		return false
	}
	m.Write16(base+uint32(tail), uint16(scan)<<8|uint16(ascii))
	m.Write16(base+kbTail, next)
	return true
}

// PeekKey 回傳緩衝區最前面那一筆，不取走。
func (m *Machine) PeekKey() (uint16, bool) {
	base := uint32(bdaSeg * 16)
	head := m.Read16(base + kbHead)
	if head == m.Read16(base+kbTail) {
		return 0, false
	}
	return m.Read16(base + uint32(head)), true
}

// PopKey 取走最前面那一筆。
func (m *Machine) PopKey() (uint16, bool) {
	v, ok := m.PeekKey()
	if !ok {
		return 0, false
	}
	base := uint32(bdaSeg * 16)
	m.Write16(base+kbHead, m.kbAdvance(m.Read16(base+kbHead)))
	return v, true
}
