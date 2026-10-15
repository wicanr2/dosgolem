package machine

// Peek8 讀一個位元組，**沒有任何副作用**：不載入平面模式的 VGA latch，
// 也不觸發讀取監看（`docs/spec/238` §2.1）。
//
// 平面模式的 A0000 視窗不在線性記憶體裡，而讀它必然改 latch，所以這裡
// 一律回 0——觀測器要的是堆疊與資料段，不是平面顯示記憶體。
//
// 轉換開啟時（`docs/spec/432-linear-memory-and-paging` §6）以客體線性視角、
// 監督者身分轉換：不因 U/S、R/W 失敗，只有頁不存在時回 0xFF；要分辨這個 0xFF
// 是資料還是頁不存在，用 PeekOK。
func (m *Machine) Peek8(a uint32) uint8 {
	if m.xlat {
		v, _ := m.peekLin(a)
		return v
	}
	if off, ok := m.hmaAddr(a); ok {
		if m.Ext != nil {
			return m.Ext[off]
		}
		return m.hma[off]
	}
	return m.peekPhysRaw(a & 0xFFFFF)
}

// Peek16 是兩次 Peek8，小端。
func (m *Machine) Peek16(a uint32) uint16 {
	return uint16(m.Peek8(a)) | uint16(m.Peek8(a+1))<<8
}

// PeekOK 同 Peek8，另回「這個位址讀得到嗎」：轉換開啟而頁不存在時回 (0xFF, false)，
// 其餘回 (值, true)。轉換關閉時恆為 true。
func (m *Machine) PeekOK(a uint32) (uint8, bool) {
	if m.xlat {
		return m.peekLin(a)
	}
	return m.Peek8(a), true
}

// PeekOK16 是兩次 PeekOK；任一個 byte 讀不到，ok 為 false。
func (m *Machine) PeekOK16(a uint32) (uint16, bool) {
	lo, ok1 := m.PeekOK(a)
	hi, ok2 := m.PeekOK(a + 1)
	return uint16(lo) | uint16(hi)<<8, ok1 && ok2
}

// peekLin 是轉換開啟時的觀測轉換：TLB 條目世代符合就用（看到 CPU 此刻會看到的，
// 含舊轉換）；不符就無副作用地走頁表，不填 TLB、不寫 accessed／dirty、不產生錯誤紀錄
// （peekXlatPage，paging.go）。不檢查權限（監督者身分）。之後套 A20，走 peekPhysRaw。
func (m *Machine) peekLin(a uint32) (uint8, bool) {
	if m.paging {
		var ok bool
		if a, ok = m.peekXlatPage(a); !ok {
			return 0xFF, false
		}
	}
	if !m.a20 {
		a &^= 0x100000
	}
	return m.peekPhysRaw(a), true
}
