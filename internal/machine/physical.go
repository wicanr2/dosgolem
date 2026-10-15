package machine

// 實體層（docs/spec/432-linear-memory-and-paging §3.1）。
//
// 輸入是**實體位址**，不做 A20 遮罩、不做分頁轉換。`Read8`／`Write8`（線性層）與
// `ReadPhys*`／`WritePhys*`（430）都走這一層，所以 VGA 平面視窗、ROM 保護、讀寫監看、
// 影片寫入紀錄與 `Ext` 只有一份本體。
//
//   - a < 1 MB：低記憶體路徑；
//   - 1 MB ≤ a < 1 MB + len(Ext)：Ext；
//   - 其餘：讀回 0xFF；寫入丟棄並計 UnmappedPhysWrites。

// readPhysRaw 讀一個實體位址。
func (m *Machine) readPhysRaw(a uint32) uint8 {
	if a >= MemSize {
		if off := a - ExtBase; off < uint32(len(m.Ext)) {
			return m.Ext[off]
		}
		return 0xFF
	}
	// 平面模式的 A0000 視窗不在線性記憶體裡（`docs/spec/013` §3.1）。
	// ⚠ **讀它有副作用**：四個 latch 會被載入，而 latch 決定之後那次
	// 寫入在 Bit Mask 之外的位元（`docs/spec/011` §4）。
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return m.VGA.Read(a - vgaLo)
	}
	if m.rWatchLo <= a && a <= m.rWatchHi {
		m.onRead(a, m.Mem[a])
	}
	return m.Mem[a]
}

// peekPhysRaw 與 readPhysRaw 相同，但不載 VGA latch、不觸發讀取監看
// （對應 `Peek8` 的實體部分，`docs/spec/238` §2.1）。平面視窗一律回 0。
func (m *Machine) peekPhysRaw(a uint32) uint8 {
	if a >= MemSize {
		if off := a - ExtBase; off < uint32(len(m.Ext)) {
			return m.Ext[off]
		}
		return 0xFF
	}
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return 0
	}
	return m.Mem[a]
}

// writePhysRaw 寫一個實體位址。
func (m *Machine) writePhysRaw(a uint32, v uint8) {
	if a >= MemSize {
		if off := a - ExtBase; off < uint32(len(m.Ext)) {
			m.Ext[off] = v
			return
		}
		m.UnmappedPhysWrites++
		return
	}
	if a >= ROMBase {
		m.noteROMWrite(a, v)
		return
	}
	if m.onVideoWrite != nil && a >= vgaLo && a < vgaHi {
		cs, ip := m.CPU.OpAddr()
		m.onVideoWrite(VideoWrite{Step: m.Steps, CS: cs, IP: ip, Offset: a - vgaLo, Value: v, WriteMode: m.VGA.WriteMode()})
	}
	if m.planarOn && a >= vgaLo && a < vgaHi {
		// planar 的位元組不在 Mem[] 裡，WatchWrites 看不到它們。
		// VideoRowWrites 補這個洞：**要分辨「程式沒畫」與「畫了但沒生效」**，
		// 除了看結果還得看它到底有沒有寫進去。
		stride := m.planarStride()
		if m.RowWritesFrom > 0 && m.Steps >= m.RowWritesFrom {
			if r := int(a-vgaLo) / stride; r < len(m.VideoRowWrites) {
				m.VideoRowWrites[r]++
			}
		}
		// 用過哪些 write mode、誰在寫視訊記憶體——**畫面上看得到的東西，
		// 一定有人把它寫進 plane**，這是沒有原始碼時找繪圖常式最短的路。
		m.WriteModeUse[m.VGA.WriteMode()]++
		if m.VRAMSites != nil && (m.VRAMAt < 0 || uint32(m.VRAMAt) == a-vgaLo) {
			cs, ip := m.CPU.OpAddr()
			m.VRAMSites[uint32(cs)<<16|uint32(ip)]++
		}
		// **留最後 40 筆，不是前 40 筆**：要知道畫面上最後是誰寫的，
		// 前面那幾筆通常是被蓋掉的那一批。
		if m.VGATraceRow1 > m.VGATraceRow0 && m.Steps >= m.RowWritesFrom {
			off := int(a - vgaLo)
			r, c := off/stride, off%stride
			inCol := m.VGATraceCol1 == 0 || (c >= m.VGATraceCol0 && c < m.VGATraceCol1)
			if r >= m.VGATraceRow0 && r < m.VGATraceRow1 && inCol {
				m.VGATrace = append(m.VGATrace, m.vgaSnap(a-vgaLo, v, r))
				if len(m.VGATrace) > 40 {
					m.VGATrace = m.VGATrace[1:]
				}
			}
		}
		m.VGA.Write(a-vgaLo, v)
		return
	}
	if m.watchLo <= a && a <= m.watchHi && m.Mem[a] != v {
		m.onWrite(a, m.Mem[a], v)
	}
	if len(m.writeWatches) > 0 {
		old := m.Mem[a]
		m.Mem[a] = v
		// ⚠ **值沒變也算一次寫入**（與 WatchWrites 不同）：
		// 「誰在寫這個位址」與「這個值變了沒」是兩個問題，後者用 WatchWord。
		m.noteWrite(a, old, v)
		return
	}
	m.Mem[a] = v
}

// pageByte／pagePut 是頁表專用的單 byte 實體存取：直接讀寫 Mem／Ext，
// 不經 VGA、ROM 保護與監看。範圍外讀 0xFF、寫入丟棄（同 DOSBox-X `mem.h:263`
// 讀到 FFFFFFFFh 的行為）。
func (m *Machine) pageByte(a uint32) uint8 {
	if a < MemSize {
		return m.Mem[a]
	}
	if off := a - ExtBase; off < uint32(len(m.Ext)) {
		return m.Ext[off]
	}
	return 0xFF
}

func (m *Machine) pagePut(a uint32, v uint8) {
	if a < MemSize {
		m.Mem[a] = v
		return
	}
	if off := a - ExtBase; off < uint32(len(m.Ext)) {
		m.Ext[off] = v
	}
}

// pageRead32 讀頁目錄或頁表項（小端，逐 byte，各 byte 各自歸類）。
// 呼叫端負責 A20 遮罩（432 §5）。
func (m *Machine) pageRead32(a uint32) uint32 {
	return uint32(m.pageByte(a)) | uint32(m.pageByte(a+1))<<8 |
		uint32(m.pageByte(a+2))<<16 | uint32(m.pageByte(a+3))<<24
}

// pageWrite32 寫頁目錄或頁表項，並累加 PageTableWrites（診斷計數，不存檔）。
func (m *Machine) pageWrite32(a uint32, v uint32) {
	m.PageTableWrites++
	m.pagePut(a, uint8(v))
	m.pagePut(a+1, uint8(v>>8))
	m.pagePut(a+2, uint8(v>>16))
	m.pagePut(a+3, uint8(v>>24))
}
