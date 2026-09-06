package machine

// VGA 的 planar 模式（`docs/spec/013`）。
//
// mode 13h 是「一個位元組一個像素」；EGA／VGA 的 16 色模式不是——
// **一個位元組管八個像素的同一個 bit plane**，四個 plane 疊起來才是色號。
// 沒有這層模型，四次寫入會落在同一段線性記憶體互相覆蓋，而畫面看起來
// 「有東西」（那是最後一次寫入），記憶體、色盤、指令流全部正常。

// planeSize 是一個 plane 的大小：A0000 的 64 KB 視窗。
const planeSize = 0x10000

// vgaWindow 是 planar 模式接管的位址範圍。
const (
	vgaLo = 0xA0000
	vgaHi = 0xB0000
)

// vga 是 planar 模式的狀態。四個 plane、四個 latch，加上 Sequencer 與
// Graphics Controller 的暫存器。
type vga struct {
	// planes 是四個 plane 接在一起（plane p 的第 off 個位元組是
	// `planes[p*planeSize+off]`）。**接在一起是為了 VideoRaw 能一次切出去**
	// ——變化偵測要比對整個畫面，不能每次配置。
	planes [4 * planeSize]uint8

	// latch 是上一次讀 A0000 載進來的四個位元組。**讀取的副作用**，
	// write mode 1 整個機制就靠它（`013` §3.3）。
	latch [4]uint8

	seqIdx uint8
	seq    [8]uint8 // 只有 index 2（Map Mask）有作用

	gcIdx uint8
	gc    [9]uint8
}

// planarMode 說某個視訊模式是不是 planar（`013` §3.1）。
func planarMode(mode uint8) bool {
	switch mode {
	case 0x0D, 0x0E, 0x10, 0x12:
		return true
	}
	return false
}

// sizeOf 回某個模式的畫面尺寸。
func sizeOf(mode uint8) (int, int) {
	switch mode {
	case 0x0D:
		return 320, 200
	case 0x0E:
		return 640, 200
	case 0x10:
		return 640, 350
	case 0x12:
		return 640, 480
	}
	return VideoWidth, VideoHigh
}

// VideoSize 回目前模式的畫面尺寸。mode 13h 與文字模式回 320×200
// （`oracle.Width`／`Height` 的那一組）。
func (m *Machine) VideoSize() (int, int) { return sizeOf(m.VideoMode()) }

// expand 把一個 bit 攤成整個位元組：1 → FF、0 → 00。
func expand(bit uint8) uint8 {
	if bit != 0 {
		return 0xFF
	}
	return 0
}

// ror8 是 8 位元右旋（Data Rotate，`GC[03]` 低 3 bit）。
func ror8(v, n uint8) uint8 {
	n &= 7
	return v>>n | v<<(8-n)&0xFF
}

// vgaRead 讀 planar 記憶體。**先載 latch 再回值**——載 latch 是讀取的
// 副作用，不是可以省掉的一步（`013` §3.3）。
func (m *Machine) vgaRead(off uint32) uint8 {
	g := m.vga
	for p := 0; p < 4; p++ {
		g.latch[p] = g.planes[p*planeSize+int(off)]
	}
	if g.gc[5]&0x08 == 0 { // read mode 0
		return g.planes[int(g.gc[4]&3)*planeSize+int(off)]
	}
	// read mode 1：color compare。回傳的每個 bit 表示「參與比較的 plane
	// 在這個像素上是否都等於 GC[02]」。GC[07] 的 bit ＝ 1 才參與。
	cc, care := g.gc[2]&0x0F, g.gc[7]&0x0F
	var out uint8
	for b := 0; b < 8; b++ {
		match := true
		for p := 0; p < 4; p++ {
			if care>>p&1 == 0 {
				continue
			}
			if g.planes[p*planeSize+int(off)]>>(7-b)&1 != cc>>p&1 {
				match = false
				break
			}
		}
		if match {
			out |= 1 << (7 - b)
		}
	}
	return out
}

// vgaWrite 寫 planar 記憶體，四種 write mode 見 `013` §3.4。
func (m *Machine) vgaWrite(off uint32, v uint8) {
	g := m.vga
	mapMask := g.seq[2] & 0x0F
	mode := g.gc[5] & 3
	fn := g.gc[3] >> 3 & 3
	bitmask := g.gc[8]

	var val [4]uint8
	switch mode {
	case 0:
		d := ror8(v, g.gc[3])
		for p := 0; p < 4; p++ {
			if g.gc[1]>>p&1 != 0 { // enable set/reset
				val[p] = expand(g.gc[0] >> p & 1)
			} else {
				val[p] = d
			}
		}
	case 1:
		// latch 原封不動寫回去。不套 ALU、不套 bit mask——
		// 「讀一個位址再寫另一個位址」的整段複製就是這樣做的。
		for p := 0; p < 4; p++ {
			if mapMask>>p&1 != 0 {
				g.planes[p*planeSize+int(off)] = g.latch[p]
			}
		}
		return
	case 2:
		for p := 0; p < 4; p++ {
			val[p] = expand(v >> p & 1)
		}
	case 3:
		// 資料本身就是 bit mask，顏色一律來自 Set/Reset。
		bitmask &= ror8(v, g.gc[3])
		for p := 0; p < 4; p++ {
			val[p] = expand(g.gc[0] >> p & 1)
		}
	}

	for p := 0; p < 4; p++ {
		if mapMask>>p&1 == 0 {
			continue
		}
		x := val[p]
		switch fn {
		case 1:
			x &= g.latch[p]
		case 2:
			x |= g.latch[p]
		case 3:
			x ^= g.latch[p]
		}
		g.planes[p*planeSize+int(off)] = x&bitmask | g.latch[p]&^bitmask
	}
}

// vgaOut 收 Sequencer 與 Graphics Controller 的寫入。
func (m *Machine) vgaOut(port uint16, v uint8) {
	g := m.vga
	switch port {
	case 0x3C4:
		g.seqIdx = v & 7
	case 0x3C5:
		g.seq[g.seqIdx] = v
	case 0x3CE:
		g.gcIdx = v & 0x0F
	case 0x3CF:
		if int(g.gcIdx) < len(g.gc) {
			g.gc[g.gcIdx] = v
		}
	}
}

// vgaIn 讓程式讀回設定值（先讀再改是常見寫法）。回 (值, 有沒有這個埠)。
func (m *Machine) vgaIn(port uint16) (uint8, bool) {
	g := m.vga
	switch port {
	case 0x3C4:
		return g.seqIdx, true
	case 0x3C5:
		return g.seq[g.seqIdx], true
	case 0x3CE:
		return g.gcIdx, true
	case 0x3CF:
		if int(g.gcIdx) < len(g.gc) {
			return g.gc[g.gcIdx], true
		}
		return 0, true
	}
	return 0, false
}

// vgaReset 清空四個 plane，並把 Sequencer 與 Graphics Controller 設回
// BIOS 設模式後的狀態。設模式時呼叫。
//
// ⚠ **預設值不是零值。** Map Mask 零 ＝ 一個 plane 都不寫、Bit Mask 零 ＝
// 一個位元都不改——兩個都是「寫進去什麼都沒發生」，而畫面是全黑，
// 看起來像「程式還沒畫」。真機的 BIOS 設模式時把它們設成全開，
// 程式因此可以直接開始畫，不必自己初始化。
func (m *Machine) vgaReset() {
	g := m.vga
	for i := range g.planes {
		g.planes[i] = 0
	}
	g.latch = [4]uint8{}
	g.seq = [8]uint8{}
	g.seq[2] = 0x0F // Map Mask：四個 plane 全開
	g.gc = [9]uint8{}
	g.gc[8] = 0xFF // Bit Mask：整個位元組
	g.seqIdx, g.gcIdx = 0, 0
}

// planarIndexed 把四個 plane 疊成色號（0–15）。
func (m *Machine) planarIndexed() []uint8 {
	w, h := m.VideoSize()
	out := make([]uint8, w*h)
	stride := w / 8
	g := m.vga
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := y*stride + x/8
			bit := uint(7 - x%8)
			var v uint8
			for p := 0; p < 4; p++ {
				v |= g.planes[p*planeSize+int(off)] >> bit & 1 << p
			}
			out[y*w+x] = v
		}
	}
	return out
}

// VideoRaw 是畫面記憶體的**直接切片，不複製**（給變化偵測與非零計數用）。
//
// planar 模式回四個 plane 的全部內容（256 KB）；mode 13h 回 A0000 起的
// 320×200。**語意是「畫面有沒有動」，不是色號**——要色號用 Indexed()。
func (m *Machine) VideoRaw() []uint8 {
	if m.planarOn {
		return m.vga.planes[:]
	}
	base := VideoSeg * 16
	return m.Mem[base : base+VideoWidth*VideoHigh]
}
