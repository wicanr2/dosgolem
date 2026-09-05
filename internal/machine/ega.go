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
}

func newEGA() *ega {
	e := &ega{mapMask: egaMapMaskAll}
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

// write 把一次 A0000 段的寫入分到 Map Mask 打開的平面上。
func (e *ega) write(address uint32, value uint8) {
	offset := address - egaVRAMBase
	for plane := 0; plane < egaPlanes; plane++ {
		if e.mapMask&(1<<uint(plane)) == 0 {
			continue
		}
		e.planes[plane][offset] = value
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

// IndexedEGA 把四個平面組成 mode 0Dh 的 320×200 色號陣列。
//
// 第 (x, y) 個像素取每個平面同一個位元：位元編號是 `7 - (x mod 8)`，
// 平面 n 貢獻結果的第 n 位。**回的是色號不是 RGB**，與 Indexed 一致
// ——對拍在色號空間做。
func (m *Machine) IndexedEGA() []uint8 {
	out := make([]uint8, VideoWidth*VideoHigh)
	const rowBytes = VideoWidth / 8
	for y := 0; y < VideoHigh; y++ {
		for x := 0; x < VideoWidth; x++ {
			offset := y*rowBytes + x/8
			bit := uint(7 - x%8)
			var value uint8
			for plane := 0; plane < egaPlanes; plane++ {
				if m.ega.planes[plane][offset]>>bit&1 != 0 {
					value |= 1 << uint(plane)
				}
			}
			out[y*VideoWidth+x] = value
		}
	}
	return out
}
