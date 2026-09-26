package machine

// Peek8 讀一個位元組，**沒有任何副作用**：不載入平面模式的 VGA latch，
// 也不觸發讀取監看（`docs/spec/238` §2.1）。
//
// 平面模式的 A0000 視窗不在線性記憶體裡，而讀它必然改 latch，所以這裡
// 一律回 0——觀測器要的是堆疊與資料段，不是平面顯示記憶體。
func (m *Machine) Peek8(a uint32) uint8 {
	if off, ok := m.hmaAddr(a); ok {
		return m.hma[off]
	}
	a &= 0xFFFFF
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return 0
	}
	return m.Mem[a]
}

// Peek16 是兩次 Peek8，小端。
func (m *Machine) Peek16(a uint32) uint16 {
	return uint16(m.Peek8(a)) | uint16(m.Peek8(a+1))<<8
}
