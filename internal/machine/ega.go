package machine

// EGA mode 0Dh 的平面式 VRAM（`docs/spec/007`）。
//
// mode 0Dh 是 320×200、16 色，**四個位元平面疊在 A0000 同一段位址**：
// 一個平面 `320 × 200 ÷ 8 = 8000` bytes，CPU 寫一個 byte 就同時決定八個像素
// 在那個平面上的位元。哪些平面吃得到那次寫入，由序列器的 Map Mask
//（索引 `02`）決定。
//
// **沒有平面式 VRAM 的時候症狀不是壞掉，是「一張正常但錯的圖」**：
// 四個平面的寫入互相蓋掉，只剩最後一個平面，看起來仍是一張 320×200 的圖。
// 這正是要把它做出來的理由。

const (
	// egaPlanes 是平面數。
	egaPlanes = 4
	// egaPlaneBytes 是一個平面在 A0000 段裡的長度。整段留 64 KB，
	// 與 `A0000..AFFFF` 的定址範圍一致。
	egaPlaneBytes = 0x10000
	// egaVRAMBase／egaVRAMEnd 是平面式定址的範圍。
	egaVRAMBase = 0xA0000
	egaVRAMEnd  = 0xB0000
	// sequencerMapMaskIndex 是 Map Mask 的序列器索引。
	sequencerMapMaskIndex = 0x02
	// egaMapMaskAll 是四個平面都打開，也是重置值。
	egaMapMaskAll = 0x0F

	// 圖形控制器（埠 3CE 索引／3CF 資料）的索引。
	gcSetReset       = 0x00
	gcEnableSetReset = 0x01
	gcColorCompare   = 0x02
	gcDataRotate     = 0x03
	gcReadMapSelect  = 0x04
	gcMode           = 0x05
	gcMiscellaneous  = 0x06
	gcColorDontCare  = 0x07
	gcBitMask        = 0x08
)

// ega 是平面式 VRAM 的狀態。
type ega struct {
	// planes[n] 是平面 n。
	planes [egaPlanes][]uint8
	// index 是序列器索引暫存器（埠 `3C4`）的目前值。
	index uint8
	// mapMask 是序列器索引 `02` 的低四位（埠 `3C5`）。位元 n ＝ 平面 n。
	mapMask uint8
	// planarSeen 記「Map Mask 曾被寫成不是 0Fh 的值」。
	//
	// **這是判斷用的訊號，不是模式暫存器**：Pool of Radiance 從來沒呼叫
	// `int 10h AH=00`，BDA 的模式位元組一路是 03h，所以沒有比它更硬的依據
	//（`docs/spec/007` §2.1）。
	planarSeen bool

	// 圖形控制器（`docs/spec/011` §4）。
	gcIndex   uint8
	setReset  uint8 // 索引 00：每個平面要填的顏色位元
	enableSR  uint8 // 索引 01：哪些平面吃 setReset
	dataRot   uint8 // 索引 03：bit0–2 右旋次數、bit3–4 邏輯運算
	readMap   uint8 // 索引 04：read mode 0 讀哪一個平面
	mode      uint8 // 索引 05：bit0–1 寫入模式、bit3 讀取模式
	bitMask   uint8 // 索引 08：這一次寫入動得了哪幾個位元
	dontCare  uint8 // 索引 07：read mode 1 比對時忽略哪些平面
	colorComp uint8 // 索引 02：read mode 1 比對的顏色

	// latch 是 CPU 上一次讀 VRAM 時各平面的內容。
	//
	// ⚠ **latch 是圖形控制器的核心，不是最佳化。** EGA 的一次寫入
	// 只動 Bit Mask 打開的位元，其餘位元**從 latch 補回去**——所以
	// 「讀一次再寫一次」是必要動作，不是多餘的。少了 latch，
	// 遮罩之外的位元會被歸零，畫面上是一條一條的直線雜訊，
	// 看起來像時序問題不像少了一個暫存器。
	latch [egaPlanes]uint8
}

func newEGA() *ega {
	// bitMask 的重置值是 FFh（八個位元都動得了）。**預設 0 的話畫面
	// 一片空白**，而空白看起來像「還沒畫」不像暫存器沒初始化。
	e := &ega{mapMask: egaMapMaskAll, bitMask: 0xFF}
	for plane := range e.planes {
		e.planes[plane] = make([]uint8, egaPlaneBytes)
	}
	return e
}

// outSequencer 收 `3C4`（索引）與 `3C5`（資料）。
func (e *ega) outSequencer(port uint16, value uint8) {
	switch port {
	case 0x3C4:
		e.index = value
	case 0x3C5:
		if e.index != sequencerMapMaskIndex {
			return
		}
		e.mapMask = value & 0x0F
		if e.mapMask != egaMapMaskAll {
			e.planarSeen = true
		}
	}
}

// outGraphics 收 `3CE`（索引）與 `3CF`（資料）。
func (e *ega) outGraphics(port uint16, value uint8) {
	switch port {
	case 0x3CE:
		e.gcIndex = value & 0x0F
	case 0x3CF:
		switch e.gcIndex {
		case gcSetReset:
			e.setReset = value & 0x0F
		case gcEnableSetReset:
			e.enableSR = value & 0x0F
		case gcColorCompare:
			e.colorComp = value & 0x0F
		case gcDataRotate:
			e.dataRot = value & 0x1F
		case gcReadMapSelect:
			e.readMap = value & 0x03
		case gcMode:
			e.mode = value
			e.planarSeen = true
		case gcColorDontCare:
			e.dontCare = value & 0x0F
		case gcBitMask:
			e.bitMask = value
			e.planarSeen = true
		}
	}
}

// read 是 CPU 從 A0000 段讀一個 byte。
//
// ⚠ **讀取有副作用**：四個平面同時被鎖進 latch，之後的寫入靠它補回
// Bit Mask 以外的位元。把讀取當成沒有副作用的話，`read-modify-write`
// 的那個 read 就白做了，而那正是 EGA 畫圖的標準寫法。
func (e *ega) read(address uint32) uint8 {
	offset := address - egaVRAMBase
	for plane := 0; plane < egaPlanes; plane++ {
		e.latch[plane] = e.planes[plane][offset]
	}
	if e.mode&0x08 == 0 { // read mode 0：直接讀選定的平面
		return e.latch[e.readMap]
	}
	// read mode 1：每個位元回報「這八個像素的顏色是否等於 Color Compare」，
	// Color Don't Care 指定哪些平面不參與比較。
	var out uint8
	for bit := 0; bit < 8; bit++ {
		match := true
		for plane := 0; plane < egaPlanes; plane++ {
			if e.dontCare&(1<<uint(plane)) == 0 {
				continue
			}
			want := e.colorComp>>uint(plane)&1 != 0
			got := e.latch[plane]>>uint(bit)&1 != 0
			if want != got {
				match = false
				break
			}
		}
		if match {
			out |= 1 << uint(bit)
		}
	}
	return out
}

// write 把一次 A0000 段的寫入送進圖形控制器的資料路徑。
//
// 四種寫入模式（`docs/spec/011` §4）：
//
//	0  CPU 的值（可右旋）進資料路徑；Enable Set/Reset 打開的平面改用
//	   Set/Reset 的顏色。之後過邏輯運算、Bit Mask，再由 Map Mask 決定寫哪些平面。
//	1  latch 原封不動寫回去。用來做快速搬移（讀一格、寫一格）。
//	2  CPU 值的第 n 位決定平面 n 整個 byte 是 FF 還是 00——**畫單色圖形的模式**。
//	3  EGA 沒有，VGA 才有。這裡不實作。
func (e *ega) write(address uint32, value uint8) {
	offset := address - egaVRAMBase
	mode := e.mode & 0x03

	if mode == 1 {
		for plane := 0; plane < egaPlanes; plane++ {
			if e.mapMask&(1<<uint(plane)) == 0 {
				continue
			}
			e.planes[plane][offset] = e.latch[plane]
		}
		return
	}

	rotated := value
	if n := e.dataRot & 0x07; n != 0 && mode == 0 {
		rotated = value>>n | value<<(8-n)
	}
	op := e.dataRot >> 3 & 0x03

	for plane := 0; plane < egaPlanes; plane++ {
		if e.mapMask&(1<<uint(plane)) == 0 {
			continue
		}
		var data uint8
		switch {
		case mode == 2:
			data = 0
			if value>>uint(plane)&1 != 0 {
				data = 0xFF
			}
		case e.enableSR&(1<<uint(plane)) != 0:
			data = 0
			if e.setReset>>uint(plane)&1 != 0 {
				data = 0xFF
			}
		default:
			data = rotated
		}
		switch op {
		case 1:
			data &= e.latch[plane]
		case 2:
			data |= e.latch[plane]
		case 3:
			data ^= e.latch[plane]
		}
		// Bit Mask 之外的位元從 latch 補回去。
		e.planes[plane][offset] = data&e.bitMask | e.latch[plane]&^e.bitMask
	}
}

// SequencerIndex 回序列器索引暫存器的目前值。
func (m *Machine) SequencerIndex() uint8 { return m.ega.index }

// MapMask 回序列器索引 `02` 的低四位。
func (m *Machine) MapMask() uint8 { return m.ega.mapMask }

// EGAPlanarActive 回報 Map Mask 是否曾被寫成不是 `0Fh` 的值。
//
// 它是**訊號不是事實**：真正的模式暫存器沒被設過（`docs/spec/007` §2.1），
// 呼叫端要自己決定信不信。
func (m *Machine) EGAPlanarActive() bool { return m.ega.planarSeen }

// EGAPlane 回傳一個平面的複本，方便逐平面檢查。
func (m *Machine) EGAPlane(plane int) []uint8 {
	if plane < 0 || plane >= egaPlanes {
		return nil
	}
	out := make([]uint8, egaPlaneBytes)
	copy(out, m.ega.planes[plane])
	return out
}

// IndexedEGA 把四個平面組成色號陣列。
//
// 第 (x, y) 個像素取每個平面同一個位元：位元編號是 `7 - (x mod 8)`，
// 平面 n 貢獻結果的第 n 位。**回的是色號不是 RGB**，與 Indexed 一致
// ——對拍在色號空間做。
//
// ⚠ **尺寸要由呼叫端給。** 平面式 VRAM 本身不記解析度：同一份平面資料
// 在 mode 0Dh 是 320×200、在 mode 10h 是 640×350，猜錯不會報錯，
// 只會得到一張錯位但看起來像圖的東西。
func (m *Machine) IndexedEGASize(w, h int) []uint8 {
	if w <= 0 || h <= 0 || w%8 != 0 {
		return nil
	}
	rowBytes := w / 8
	if rowBytes*h > egaPlaneBytes {
		return nil
	}
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			offset := y*rowBytes + x/8
			bit := uint(7 - x%8)
			var value uint8
			for plane := 0; plane < egaPlanes; plane++ {
				if m.ega.planes[plane][offset]>>bit&1 != 0 {
					value |= 1 << uint(plane)
				}
			}
			out[y*w+x] = value
		}
	}
	return out
}

// IndexedEGA 是 mode 0Dh 的 320×200。
func (m *Machine) IndexedEGA() []uint8 { return m.IndexedEGASize(VideoWidth, VideoHigh) }
