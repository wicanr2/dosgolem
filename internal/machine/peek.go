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

// PeekBlock 把線性位址 a 起 len(dst) bytes 讀進 dst，與逐位元組 PeekOK 的結果相同（docs/spec/454 §2.2）。
// ok 為假表示範圍內有頁不存在，該處位元組為 FFh。沒有副作用。len(dst) = 0 時回 true。
//
// a + i 以 uint32 環繞。轉換開啟時每個 4 KiB 線性頁只轉換一次（peekXlatPage：TLB 世代符合就用，
// 否則無副作用地走頁表；不填 TLB、utlb，不寫 accessed／dirty），套 A20 後整段複製；轉換關閉時
// 依 hmaAddr 與 peekPhysRaw 的分段（低記憶體、HMA、Ext）整段複製。平面模式的 A0000 視窗照 Peek8 回 0。
func (m *Machine) PeekBlock(a uint32, dst []byte) (ok bool) {
	ok = true
	for len(dst) > 0 {
		n, good := m.peekRun(a, dst)
		if !good {
			ok = false
		}
		dst = dst[n:]
		a += uint32(n)
	}
	return ok
}

// peekRun 填 dst 開頭一段讀法相同的連續範圍，回填了幾個 byte（至少 1）與該段是否讀得到。
func (m *Machine) peekRun(a uint32, dst []byte) (int, bool) {
	if m.xlat {
		// 不跨 4 KiB 線性頁：一頁一次轉換，A20 遮罩在頁內不變（bit 20 高於頁內位移）。
		n := 0x1000 - int(a&0xFFF)
		if n > len(dst) {
			n = len(dst)
		}
		p := a
		if m.paging {
			var ok bool
			if p, ok = m.peekXlatPage(a); !ok {
				for i := range dst[:n] {
					dst[i] = 0xFF
				}
				return n, false
			}
		}
		if !m.a20 {
			p &^= 0x100000
		}
		m.peekPhysCopy(p, dst[:n])
		return n, true
	}
	if off, ok := m.hmaAddr(a); ok {
		n := int(HMASize - off)
		if n > len(dst) {
			n = len(dst)
		}
		src := m.hma[:]
		if m.Ext != nil {
			src = m.Ext
		}
		if int(off)+n > len(src) { // Ext 比 HMA 短（Peek8 此時會越界；這裡不越界）
			n = len(src) - int(off)
			if n <= 0 {
				panic("machine: PeekBlock 讀 HMA 超出 Ext")
			}
		}
		copy(dst[:n], src[off:])
		return n, true
	}
	// 其餘照 Peek8：a & 0xFFFFF 走 peekPhysRaw。一段不跨 1 MB 邊界（遮罩環繞），A20 開時
	// 低於 1 MB 的一段也不跨進 HMA。
	q := a & 0xFFFFF
	n := int(0x100000 - q)
	if m.a20 && a < MemSize && int(MemSize-a) < n {
		n = int(MemSize - a)
	}
	if n > len(dst) {
		n = len(dst)
	}
	m.peekPhysCopy(q, dst[:n])
	return n, true
}

// peekPhysCopy 是 peekPhysRaw 的整段版：實體位址 p 起 len(dst) bytes（呼叫端保證 p + len 不超出 32 位元）。
func (m *Machine) peekPhysCopy(p uint32, dst []byte) {
	for len(dst) > 0 {
		if p >= MemSize {
			if off := p - ExtBase; off < uint32(len(m.Ext)) {
				k := copy(dst, m.Ext[off:])
				dst = dst[k:]
				p += uint32(k)
				continue
			}
			for i := range dst {
				dst[i] = 0xFF
			}
			return
		}
		end := uint32(MemSize)
		if m.planarOn {
			if p >= vgaLo && p < vgaHi {
				k := int(vgaHi - p)
				if k > len(dst) {
					k = len(dst)
				}
				clear(dst[:k])
				dst = dst[k:]
				p += uint32(k)
				continue
			}
			if p < vgaLo {
				end = vgaLo
			}
		}
		k := int(end - p)
		if k > len(dst) {
			k = len(dst)
		}
		copy(dst[:k], m.Mem[p:])
		dst = dst[k:]
		p += uint32(k)
	}
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
