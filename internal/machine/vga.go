package machine

// VGA 16 色平面模式（`docs/spec/007`）。
//
// mode 0Dh–12h 的 `A0000` 不是一塊線性緩衝區，而是**四個平面**：
// 同一個位移在四個平面各有一個 byte，湊出 8 個像素的 4 bit 色號。
// CPU 寫進去的 byte 要先過 Set/Reset、位元遮罩、ALU 與 latch，
// 最後由 Map Mask 決定哪幾個平面真的吃這次寫入。
//
// **模型少了 latch 就會壞得很安靜**：畫字時沒被字型位元覆蓋的平面
// 會被清成 0，畫面上看起來只是「顏色不對」。

// PlaneSize 是一個平面的大小（`A0000`–`AFFFF`）。
const PlaneSize = 0x10000

// VGA 是繪圖控制器、序列器與屬性控制器的狀態，加上四個平面。
//
// 零值不可用（平面沒配置），用 newVGA。
type VGA struct {
	// Planes 是四個平面。**逐點對拍讀的是這裡**，不是 Mem。
	Planes [4][]uint8

	// latch 是四個平面的鎖存器。讀一次 VRAM 就整組更新。
	latch [4]uint8

	// seq 是序列器（`3C4`/`3C5`），只有 index 02h（Map Mask）有用。
	seq    [8]uint8
	seqIdx uint8

	// gc 是繪圖控制器（`3CE`/`3CF`）。
	gc    [16]uint8
	gcIdx uint8

	// ac 是屬性控制器（`3C0`）：0–0Fh 是調色盤暫存器。
	//
	// ⚠ **同一個埠先寫索引再寫資料**，由 acFlip 決定這次是哪一種。
	// 讀 `3DA` 會把 acFlip 重設回「下一次寫的是索引」——
	// 少了這一條，程式重設 flip-flop 之後我們的相位就與它相反，
	// 於是索引被當成資料、資料被當成索引，整份調色盤錯位。
	ac     [32]uint8
	acIdx  uint8
	acFlip bool
}

func newVGA() *VGA {
	v := &VGA{}
	for i := range v.Planes {
		v.Planes[i] = make([]uint8, PlaneSize)
	}
	// Map Mask 預設四個平面全開、位元遮罩全開。BIOS 設模式時會自己寫，
	// 但程式也可能只改自己在意的那幾個，預設值錯了會少畫平面。
	v.seq[2] = 0x0F
	v.gc[8] = 0xFF
	return v
}

// planarMode 判斷某個 BIOS 視訊模式是不是 16 色平面。
//
// **用模式判斷，不要用「程式寫過 3CE 沒有」**——那會讓同一支程式在
// 切模式前後落進不同的行為，切回去時也不會恢復。
func planarMode(mode uint8) bool {
	switch mode {
	case 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12:
		return true
	}
	return false
}

// resetMode 做 BIOS 設模式時做的事：清四個平面、暫存器回預設。
//
// **調色盤暫存器設成 identity**（0–15 → DAC 0–15）。真機的 EGA 預設值
// 不是 identity，但 `KI.EXE` 開機自己寫了一輪 identity 進去，
// 兩者一致；寫成別的值會讓「程式沒寫調色盤」的情況顏色全錯。
func (v *VGA) resetMode() {
	for p := range v.Planes {
		for i := range v.Planes[p] {
			v.Planes[p][i] = 0
		}
	}
	v.latch = [4]uint8{}
	v.seq, v.gc, v.ac = [8]uint8{}, [16]uint8{}, [32]uint8{}
	v.seq[2], v.gc[8] = 0x0F, 0xFF
	for i := 0; i < 16; i++ {
		v.ac[i] = uint8(i)
	}
	v.seqIdx, v.gcIdx, v.acIdx, v.acFlip = 0, 0, 0, false
}

// ---- 埠 ------------------------------------------------------------------

// Out 處理寫到 VGA 暫存器的埠。回 true 表示這個埠是它的。
func (v *VGA) Out(p uint16, val uint8) bool {
	switch p {
	case 0x3C4:
		v.seqIdx = val & 0x07
	case 0x3C5:
		v.seq[v.seqIdx] = val
	case 0x3CE:
		v.gcIdx = val & 0x0F
	case 0x3CF:
		v.gc[v.gcIdx] = val
	case 0x3C0:
		if v.acFlip {
			v.ac[v.acIdx] = val
			v.acFlip = false
		} else {
			// bit 5 是「調色盤位址來源」，不是索引的一部分。
			v.acIdx = val & 0x1F
			v.acFlip = true
		}
	default:
		return false
	}
	return true
}

// ResetACFlip 是讀 `3DA` 的副作用：下一次寫 `3C0` 是索引。
func (v *VGA) ResetACFlip() { v.acFlip = false }

// ---- 記憶體 --------------------------------------------------------------

// Read 讀一個位移，順便把四個平面載進 latch。
//
// **latch 一定要在這裡載**，包括程式只是為了載 latch 而做的 dummy read
// （`docs/spec/007` §2）。
func (v *VGA) Read(off uint16) uint8 {
	for p := 0; p < 4; p++ {
		v.latch[p] = v.Planes[p][off]
	}
	if v.gc[5]&0x08 != 0 { // 讀取模式 1：色彩比較
		cmp, care := v.gc[2]&0x0F, v.gc[7]&0x0F
		var out uint8
		for bit := 0; bit < 8; bit++ {
			mask := uint8(0x80 >> bit)
			var px uint8
			for p := 0; p < 4; p++ {
				if v.latch[p]&mask != 0 {
					px |= 1 << p
				}
			}
			if (px^cmp)&care == 0 {
				out |= mask
			}
		}
		return out
	}
	return v.latch[v.gc[4]&0x03]
}

// Write 把一個 byte 寫進四個平面（`docs/spec/007` §3.3）。
func (v *VGA) Write(off uint16, data uint8) {
	mode := v.gc[5] & 0x03
	bm := v.gc[8]
	rot := v.gc[3] & 0x07
	fn := (v.gc[3] >> 3) & 0x03
	esr, sr := v.gc[1]&0x0F, v.gc[0]&0x0F

	var out [4]uint8
	switch mode {
	case 0:
		rd := ror8(data, rot)
		for p := 0; p < 4; p++ {
			src := rd
			if esr&(1<<p) != 0 {
				src = expand(sr&(1<<p) != 0)
			}
			out[p] = merge(alu(fn, src, v.latch[p]), v.latch[p], bm)
		}
	case 1:
		// latch 原封不動寫回去。**位元遮罩與 ALU 都不參與**，
		// 這是「搬一整塊」的快路徑。
		out = v.latch
	case 2:
		for p := 0; p < 4; p++ {
			src := expand(data&(1<<p) != 0)
			out[p] = merge(alu(fn, src, v.latch[p]), v.latch[p], bm)
		}
	case 3:
		// ⚠ **模式 3 不看 Enable Set/Reset**：四個平面一律用 Set/Reset，
		// 而 CPU 送出去的 byte 變成位元遮罩的一部分。
		m := bm & ror8(data, rot)
		for p := 0; p < 4; p++ {
			out[p] = merge(expand(sr&(1<<p) != 0), v.latch[p], m)
		}
	}

	mm := v.seq[2] & 0x0F
	for p := 0; p < 4; p++ {
		if mm&(1<<p) != 0 {
			v.Planes[p][off] = out[p]
		}
	}
}

func ror8(v, n uint8) uint8 {
	n &= 7
	return v>>n | v<<(8-n)
}

func expand(b bool) uint8 {
	if b {
		return 0xFF
	}
	return 0x00
}

// alu 是功能選擇：00 取代、01 AND、10 OR、11 XOR。
func alu(fn, src, latch uint8) uint8 {
	switch fn {
	case 1:
		return src & latch
	case 2:
		return src | latch
	case 3:
		return src ^ latch
	}
	return src
}

// merge 用位元遮罩挑：遮罩開著的位元取 v，關著的取 latch。
func merge(v, latch, mask uint8) uint8 {
	return v&mask | latch&^mask
}

// ---- 取畫面 --------------------------------------------------------------

// PlanarHeight 回某個平面模式的高度。寬固定 640（0Dh／0Eh 是 320／640）。
func planarSize(mode uint8) (w, h int) {
	switch mode {
	case 0x0D:
		return 320, 200
	case 0x0E:
		return 640, 200
	case 0x0F, 0x10:
		return 640, 350
	case 0x11, 0x12:
		return 640, 480
	}
	return 0, 0
}

// Pixels 把四個平面攤成每點一個 4 bit 色號。
//
// 列距是 `w/8` bytes。**不讀 CRTC**——目標程式不改它
// （實測 `KI.EXE` 一次都沒寫過 `3D4`）。
func (v *VGA) Pixels(w, h int) []uint8 {
	out := make([]uint8, w*h)
	pitch := w / 8
	for y := 0; y < h; y++ {
		for bx := 0; bx < pitch; bx++ {
			off := y*pitch + bx
			b0, b1 := v.Planes[0][off], v.Planes[1][off]
			b2, b3 := v.Planes[2][off], v.Planes[3][off]
			base := y*w + bx*8
			for bit := 0; bit < 8; bit++ {
				m := uint8(0x80 >> bit)
				var px uint8
				if b0&m != 0 {
					px |= 1
				}
				if b1&m != 0 {
					px |= 2
				}
				if b2&m != 0 {
					px |= 4
				}
				if b3&m != 0 {
					px |= 8
				}
				out[base+bit] = px
			}
		}
	}
	return out
}

// DACIndex 把 4 bit 像素值翻成 DAC 索引（`docs/spec/007` §3.5）。
//
// 屬性控制器的調色盤暫存器給低 6 位；模式控制（index 10h）的 bit 7 為 1 時，
// 高兩位改由色彩選擇（index 14h）的 bit 0–1 供應，否則取調色盤暫存器自己的
// bit 4–5。色彩選擇的 bit 2–3 永遠是 DAC 索引的 bit 6–7。
//
// **不要寫死成 identity。** `KI.EXE` 把它設成 identity，
// 所以寫死也「會動」——換一支程式才會發現顏色全錯，而且查不到來源。
func (v *VGA) DACIndex(px uint8) uint8 {
	p := v.ac[px&0x0F]
	idx := p & 0x0F
	if v.ac[0x10]&0x80 != 0 {
		idx |= (v.ac[0x14] & 0x03) << 4
	} else {
		idx |= p & 0x30
	}
	idx |= (v.ac[0x14] & 0x0C) << 4
	return idx
}

// ---- 機器層的接線 --------------------------------------------------------

// Planar 回目前平面模式的畫面：寬、高、每點一個 4 bit 色號。
//
// 不是平面模式就回 `0, 0, nil`——**回一片全 0 的畫面會讓
// 「模式不對」看起來像「畫面是黑的」**。
func (m *Machine) Planar() (w, h int, px []uint8) {
	mode := m.VideoMode()
	if !planarMode(mode) {
		return 0, 0, nil
	}
	w, h = planarSize(mode)
	return w, h, m.VGA.Pixels(w, h)
}

// PlanarRGB 回目前平面模式的畫面，每點三個 byte。
func (m *Machine) PlanarRGB() (w, h int, rgb []uint8) {
	w, h, px := m.Planar()
	if px == nil {
		return 0, 0, nil
	}
	pal := m.Palette()
	rgb = make([]uint8, len(px)*3)
	for i, p := range px {
		c := pal[m.VGA.DACIndex(p)]
		rgb[i*3], rgb[i*3+1], rgb[i*3+2] = c[0], c[1], c[2]
	}
	return w, h, rgb
}

// clone 複製一份 VGA 狀態（快照用）。**四個平面要真的複製**——
// 共用底層陣列的話還原之後兩份會互相汙染。
func (v *VGA) clone() VGA {
	out := *v
	for p := range v.Planes {
		out.Planes[p] = append([]uint8(nil), v.Planes[p]...)
	}
	return out
}

// restore 把狀態倒回去，沿用現有的平面陣列不重新配置。
func (v *VGA) restore(s *VGA) {
	planes := v.Planes
	*v = *s
	v.Planes = planes
	for p := range planes {
		copy(planes[p], s.Planes[p])
	}
}
