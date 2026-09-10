package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// EGA／VGA 的 16 色平面模式（`docs/spec/009`／`013`）。
//
// mode 13h 是「一個位元組一個像素」；EGA／VGA 的 16 色模式不是——
// **一個位元組管八個像素的同一個 bit plane**，四個 plane 疊起來才是色號。
// 沒有這層模型，四次寫入會落在同一段線性記憶體互相覆蓋，而畫面看起來
// 「有東西」（那是最後一次寫入），記憶體、色盤、指令流全部正常。
//
// CPU 寫進去的 byte 要先過 Set/Reset、位元遮罩、ALU 與 latch，
// 最後由 Map Mask 決定哪幾個平面真的吃這次寫入。
// **模型少了 latch 就會壞得很安靜**：畫字時沒被字型位元覆蓋的平面
// 會被清成 0，畫面上看起來只是「顏色不對」。

// PlaneSize 是一個平面的大小（`A0000`–`AFFFF`）。
const PlaneSize = 0x10000

// vgaLo／vgaHi 是平面模式接管的位址範圍。
const (
	vgaLo = 0xA0000
	vgaHi = 0xB0000
)

// VGA 是繪圖控制器、序列器與屬性控制器的狀態，加上四個平面。
//
// 零值不可用（平面沒配置），用 newVGA。
type VGA struct {
	// mem 是四個平面接在一起（平面 p 的第 off 個位元組是
	// `mem[p*PlaneSize+off]`）。**接在一起是為了 Raw 能一次切出去**
	// ——變化偵測要比對整個畫面，不能每次配置。
	mem []uint8

	// Planes 是指進 mem 的四個視窗。**逐點對拍讀的是這裡**，不是 Mem。
	Planes [4][]uint8

	// latch 是四個平面的鎖存器。讀一次 VRAM 就整組更新。
	latch [4]uint8

	// seq 是序列器（`3C4`/`3C5`），只有 index 02h（Map Mask）有用。
	seq    [8]uint8
	seqIdx uint8

	// gc 是繪圖控制器（`3CE`/`3CF`）。
	gc    [16]uint8
	gcIdx uint8

	// ac 是屬性控制器（`3C0`）：0–0Fh 是調色盤暫存器、10h 模式控制、
	// 11h 邊框色、14h 色彩選擇。
	//
	// ⚠ **同一個埠先寫索引再寫資料**，由 acFlip 決定這次是哪一種。
	// 讀 `3DA` 會把 acFlip 重設回「下一次寫的是索引」——
	// 少了這一條，程式重設 flip-flop 之後我們的相位就與它相反，
	// 於是索引被當成資料、資料被當成索引，整份調色盤錯位。
	ac     [32]uint8
	acIdx  uint8
	acFlip bool

	// crtc 是 CRT 控制器（`3D4`／`3D5`，單色是 `3B4`／`3B5`）。
	// 目前只用到顯示起點 index `0C`／`0D` 與 mode control `17`。
	crtc    [32]uint8
	crtcIdx uint8

	// planarSeen 記「Map Mask 曾被寫成不是 0Fh 的值」。
	//
	// **這是判斷用的訊號，不是模式暫存器。** 有些程式從來不呼叫
	// `int 10h AH=00`（Pool of Radiance 就是，BDA 的模式位元組一路是
	// 03h），直接自己設暫存器——只認 BDA 的話那種程式永遠走不到平面
	// 路徑，四次寫入疊在同一段線性記憶體上，而畫面看起來仍是一張圖。
	// 證據等級：**假說**（`docs/spec/007` §2.1）。
	planarSeen bool
}

func newVGA() *VGA {
	v := &VGA{mem: make([]uint8, 4*PlaneSize)}
	for p := range v.Planes {
		v.Planes[p] = v.mem[p*PlaneSize : (p+1)*PlaneSize]
	}
	v.resetMode(0)
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

// planarSize 回某個平面模式的畫面大小。
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

// resetMode 做 BIOS 設模式時做的事：清四個平面、暫存器回預設。
//
// ⚠ **預設值不是零值。** Map Mask 零 ＝ 一個平面都不寫、Bit Mask 零 ＝
// 一個位元都不改——兩個都是「寫進去什麼都沒發生」，而畫面是全黑，
// 看起來像「程式還沒畫」。真機的 BIOS 設模式時把它們設成全開，
// 程式因此可以直接開始畫，不必自己初始化。
//
// **調色盤暫存器設成 identity**（0–15 → DAC 0–15）。真機的 EGA 預設值
// 不是 identity，但被觀測的程式開機自己寫了一輪 identity 進去，
// 兩者一致；寫成別的值會讓「程式沒寫調色盤」的情況顏色全錯。
// resetMode 把暫存器設回 mode 的開機狀態並清畫面。
//
// mode ＝ 0 表示「還不知道是哪一個」（`newVGA` 的初始化），這時不設
// offset：`Pitch` 會退回用呼叫端給的寬度推。
func (v *VGA) resetMode(mode uint8) {
	for i := range v.mem {
		v.mem[i] = 0
	}
	v.latch = [4]uint8{}
	v.seq, v.gc, v.ac = [8]uint8{}, [16]uint8{}, [32]uint8{}
	v.seq[2], v.gc[8] = 0x0F, 0xFF
	for i := 0; i < 16; i++ {
		v.ac[i] = uint8(i)
	}
	v.seqIdx, v.gcIdx, v.acIdx, v.acFlip = 0, 0, 0, false
	// CRTC：顯示起點歸零，其餘給該模式的標準值（`docs/spec/192`）。
	//
	// **不給預設值的話，只設起點、不設 offset 的程式會拿到列距 0**，
	// 整張圖變成第一列重複幾百次；而 mode control 的 bit6 若是 0，
	// 顯示起點會被當成 word 模式而多乘一倍。
	v.crtc, v.crtcIdx = [32]uint8{}, 0
	v.crtc[0x17] = 0xE3 // bit6 ＝ 1：起點以位元組計
	// offset 只在**知道是哪個模式**的時候給：真機開機是文字模式，
	// CRTC 的值由 BIOS 依模式設。不知道就留 0，`Pitch` 會退回用寬度推
	// ——猜一個寬度的話，320 寬的模式會拿到 640 的列距而每一列錯開一倍。
	if w, _ := planarSize(mode); w > 0 {
		v.crtc[0x13] = uint8(w / 16) // offset 的單位是字組
	}
	// line compare 全 1 ＝ 永遠掃不到 ＝ 不分割。
	v.crtc[0x18] = 0xFF
	v.crtc[0x07] |= 0x10
	v.crtc[0x09] |= 0x40
	v.planarSeen = false
}

// ---- 埠 ------------------------------------------------------------------

// Out 處理寫到 VGA 暫存器的埠。回 true 表示這個埠是它的。
func (v *VGA) Out(p uint16, val uint8) bool {
	switch p {
	case 0x3C4:
		v.seqIdx = val & 0x07
	case 0x3C5:
		v.seq[v.seqIdx] = val
		if v.seqIdx == 2 && val&0x0F != 0x0F {
			v.planarSeen = true
		}
	case 0x3CE:
		v.gcIdx = val & 0x0F
	case 0x3CF:
		v.gc[v.gcIdx] = val
	case 0x3D4, 0x3B4:
		v.crtcIdx = val & 0x1F
	case 0x3D5, 0x3B5:
		v.crtc[v.crtcIdx] = val
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

// In 讓程式讀回設定值（先讀再改是常見寫法）。回 (值, 這個埠是不是它的)。
func (v *VGA) In(p uint16) (uint8, bool) {
	switch p {
	case 0x3C4:
		return v.seqIdx, true
	case 0x3C5:
		return v.seq[v.seqIdx], true
	case 0x3CE:
		return v.gcIdx, true
	case 0x3CF:
		return v.gc[v.gcIdx], true
	case 0x3C1:
		return v.ac[v.acIdx], true
	case 0x3D4, 0x3B4:
		return v.crtcIdx, true
	case 0x3D5, 0x3B5:
		return v.crtc[v.crtcIdx], true
	}
	return 0, false
}

// ResetACFlip 是讀 `3DA` 的副作用：下一次寫 `3C0` 是索引。
func (v *VGA) ResetACFlip() { v.acFlip = false }

// ---- 暫存器查詢（診斷用）--------------------------------------------------

// SeqIndex 是序列器目前選中的索引，MapMask 是 index 02h。
func (v *VGA) SeqIndex() uint8 { return v.seqIdx }

// MapMask 是序列器 index 02h：哪幾個平面吃寫入。
func (v *VGA) MapMask() uint8 { return v.seq[2] }

// PlanarSeen 回報 Map Mask 曾被寫成不是 0Fh 的值。
func (v *VGA) PlanarSeen() bool { return v.planarSeen }

// WriteMode 是繪圖控制器 index 05h 的低兩位。
func (v *VGA) WriteMode() uint8 { return v.gc[5] & 0x03 }

// Regs 回目前的繪圖控制器、序列器與 latch。
//
// 畫面上的位元不一定等於 CPU 寫進去的位元組：write mode、bit mask、
// set/reset 與 latch 會先改一次。查「為什麼寫 09 出來是 C3」的時候，
// 光看寫入指令沒有用，要看那一刻硬體的狀態。
func (v *VGA) Regs() (gc [16]uint8, seq [8]uint8, latch [4]uint8) {
	return v.gc, v.seq, v.latch
}

// Pal 是屬性控制器的第 i 個調色盤暫存器（i ＝ 0–15）。
func (v *VGA) Pal(i int) uint8 { return v.ac[i&0x0F] }

// SetPal 設第 i 個調色盤暫存器。
func (v *VGA) SetPal(i int, c uint8) { v.ac[i&0x0F] = c }

// Overscan 是邊框色（屬性控制器暫存器 11h）。
func (v *VGA) Overscan() uint8 { return v.ac[0x11] }

// SetOverscan 設邊框色。
func (v *VGA) SetOverscan(c uint8) { v.ac[0x11] = c }

// ---- 記憶體 --------------------------------------------------------------

// Read 讀一個位移，順便把四個平面載進 latch。
//
// **latch 一定要在這裡載**，包括程式只是為了載 latch 而做的 dummy read。
func (v *VGA) Read(off uint32) uint8 {
	off &= PlaneSize - 1
	for p := 0; p < 4; p++ {
		v.latch[p] = v.Planes[p][off]
	}
	if v.gc[5]&0x08 == 0 { // 讀取模式 0：回 gc[4] 選的平面
		return v.Planes[v.gc[4]&0x03][off]
	}
	// 讀取模式 1：色彩比較。回傳的每個 bit 表示「參與比較的平面在這個
	// 像素上是否都等於 GC[02]」；GC[07] 的 bit ＝ 1 才參與。
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

// Write 把一個 byte 寫進四個平面。四種 write mode 的差別只在
// 「每個平面拿到什麼資料」與「遮罩從哪來」，之後的 ALU、位元遮罩與
// map mask 是共通的。
func (v *VGA) Write(off uint32, data uint8) {
	off &= PlaneSize - 1
	mode := v.gc[5] & 0x03
	bm := v.gc[8]
	rot := v.gc[3] & 0x07
	fn := (v.gc[3] >> 3) & 0x03
	esr, sr := v.gc[1]&0x0F, v.gc[0]&0x0F
	mm := v.seq[2] & 0x0F

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
		// 這是「讀一個位址再寫另一個位址」的整段複製快路徑。
		out = v.latch
	case 2:
		for p := 0; p < 4; p++ {
			src := expand(data&(1<<p) != 0)
			out[p] = merge(alu(fn, src, v.latch[p]), v.latch[p], bm)
		}
	case 3:
		// ⚠ **模式 3 不看 Enable Set/Reset**：四個平面一律用 Set/Reset，
		// 而 CPU 送出去的 byte 變成位元遮罩的一部分。
		//
		// ⚠ **但功能選擇（AND／OR／XOR）照常作用。**
		// 少了這一段，用 XOR 反白一列的程式會把那一列的字**整個蓋掉**：
		// 反白條的顏色也跟著錯（XOR 出來的綠色變成 Set/Reset 的黃色）。
		// 兩個症狀來自同一行，而單看任何一個都像別的問題。
		m := bm & ror8(data, rot)
		for p := 0; p < 4; p++ {
			src := expand(sr&(1<<p) != 0)
			out[p] = merge(alu(fn, src, v.latch[p]), v.latch[p], m)
		}
	}

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

// Raw 是四個平面的**直接切片，不複製**（給變化偵測與非零計數用）。
func (v *VGA) Raw() []uint8 { return v.mem }

// ---- 取畫面 --------------------------------------------------------------

// DisplayStart 回 CRTC 的顯示起點，換算成平面內的**位元組**位移。
//
// index `0C`／`0D` 是起點的高／低位元組；單位由 mode control（index
// `17`）的 bit6 決定：1 ＝ byte 模式（直接用），0 ＝ word 模式（乘 2）。
// BIOS 把 16 色 planar 模式設成 `0xE3`，所以預設是 byte 模式。
//
// ⚠ **沒有這個換算，畫面在翻頁的程式倒出來的永遠是第 0 頁**——
// 而那看起來只是「畫面對不上」，不像少了一個暫存器。
func (v *VGA) DisplayStart() uint32 { return displayStart(v.crtc) }

func displayStart(crtc [32]uint8) uint32 {
	start := uint32(crtc[0x0C])<<8 | uint32(crtc[0x0D])
	if crtc[0x17]&0x40 == 0 {
		start *= 2
	}
	return start & 0xFFFF
}

// Pitch 回一列的位元組數（CRTC offset，index `13`）。
//
// 單位是**字組**，所以位元組數是 `offset × 2`。BIOS 給 640 寬的模式
// 設 40，正好是 `640/8`。
//
// **捲動的遊戲一定會改它**：邏輯畫面比可視畫面寬，水平捲動就是把顯示
// 起點往右挪幾個位元組。列距不跟著走的話每一列都往同一個方向錯開固定
// 的量，整張圖斜成平行四邊形——那看起來像解碼壞了，不像少讀了一個
// 暫存器。
//
// `0` 是「還沒被設過」，不是「列距 0」：退回用模式的寬度推。
func (v *VGA) Pitch(w int) int {
	if v.crtc[0x13] == 0 {
		return w / 8
	}
	return int(v.crtc[0x13]) * 2
}

// LineCompare 回分割畫面的掃描列（9 位元，散在三個暫存器）。
//
// 掃到那一列時位址計數器歸零，所以**分割線之下的位址從 0 起算、
// 與顯示起點無關**——遊戲用它固定狀態列：上半部捲動、下半部不動。
//
// ⚠ **三個來源都要算**：index `18` 是低 8 位、`07` 的 bit4 是第 8 位、
// `09` 的 bit6 是第 9 位。只讀低 8 位的症狀是「分割線出現在畫面上方
// 某處」，看起來像分割線設錯了，不像少讀了兩個位元。
//
// 沒有分割時 BIOS 設全 1（`0x3FF`），也就是永遠掃不到。
func (v *VGA) LineCompare() int {
	lc := int(v.crtc[0x18])
	lc |= int(v.crtc[0x07]>>4&1) << 8
	lc |= int(v.crtc[0x09]>>6&1) << 9
	return lc
}

// Pixels 把四個平面攤成每點一個 4 bit 色號。
//
// 列距是 `w/8` bytes，**起點跟著 CRTC 的顯示起點走**（`DisplayStart`）
// ——畫面在翻頁的程式不讀它就永遠倒出第 0 頁。
func (v *VGA) Pixels(w, h int) []uint8 {
	out := make([]uint8, w*h)
	pitch := v.Pitch(w)
	start := v.DisplayStart()
	split := v.LineCompare()
	visible := w / 8 // 一列可視多少位元組；邏輯列可能更寬
	for y := 0; y < h; y++ {
		// 掃到分割線那一列時位址計數器歸零，所以**那一列本身**就從 0
		// 起算（`docs/spec/192` §2.2）。
		row := int(start) + y*pitch
		if y >= split {
			row = (y - split) * pitch
		}
		for bx := 0; bx < visible; bx++ {
			off := (row + bx) & (PlaneSize - 1)
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

// DACIndex 把 4 bit 像素值翻成 DAC 索引。
//
// 屬性控制器的調色盤暫存器給低 6 位；模式控制（index 10h）的 bit 7 為 1 時，
// 高兩位改由色彩選擇（index 14h）的 bit 0–1 供應，否則取調色盤暫存器自己的
// bit 4–5。色彩選擇的 bit 2–3 永遠是 DAC 索引的 bit 6–7。
//
// **不要寫死成 identity。** 被觀測的程式把它設成 identity，
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

// clone 複製一份 VGA 狀態（快照用）。**四個平面要真的複製**——
// 共用底層陣列的話還原之後兩份會互相汙染。
func (v *VGA) clone() *VGA {
	out := &VGA{mem: append([]uint8(nil), v.mem...)}
	out.latch, out.seq, out.gc, out.ac = v.latch, v.seq, v.gc, v.ac
	out.seqIdx, out.gcIdx, out.acIdx, out.acFlip = v.seqIdx, v.gcIdx, v.acIdx, v.acFlip
	for p := range out.Planes {
		out.Planes[p] = out.mem[p*PlaneSize : (p+1)*PlaneSize]
	}
	return out
}

// restore 把狀態倒回去，沿用現有的平面陣列不重新配置。
func (v *VGA) restore(s *VGA) {
	copy(v.mem, s.mem)
	v.latch, v.seq, v.gc, v.ac = s.latch, s.seq, s.gc, s.ac
	v.seqIdx, v.gcIdx, v.acIdx, v.acFlip = s.seqIdx, s.gcIdx, s.acIdx, s.acFlip
}

// ---- 機器層的接線 --------------------------------------------------------

// VideoSize 回目前模式的畫面尺寸。mode 13h 與文字模式回 320×200。
func (m *Machine) VideoSize() (int, int) {
	if w, h := planarSize(m.VideoMode()); w != 0 {
		return w, h
	}
	return VideoWidth, VideoHigh
}

// PlanarSize 回目前平面模式的畫面大小。不是平面模式就回 `0, 0`。
func (m *Machine) PlanarSize() (w, h int) {
	if !m.planarOn {
		return 0, 0
	}
	return planarSize(m.VideoMode())
}

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

// PlanarPutPixel 在平面模式下畫一個點：off 是位元組位移、mask 是那一格的
// 位元、color 是 0–15 的色號。
//
// **不走 VGA 的寫入路徑**：`int 10h AH=0Ch` 是 BIOS 服務，BIOS 自己會設
// 好 Set/Reset 與位元遮罩再寫；我們直接改平面，程式先前設的繪圖控制器
// 狀態因此不受影響——真 BIOS 也會把它設回去。走 VGA.Write 的話，
// 程式留在暫存器裡的 write mode 或 Map Mask 會把這一點畫錯或畫不出來。
func (m *Machine) PlanarPutPixel(off uint32, mask, color uint8) {
	off &= PlaneSize - 1
	for p := 0; p < 4; p++ {
		if color&(1<<p) != 0 {
			m.VGA.Planes[p][off] |= mask
		} else {
			m.VGA.Planes[p][off] &^= mask
		}
	}
}

// PlanarPixels 把四個平面解成色號陣列。w/h 由呼叫端依模式給。
func (m *Machine) PlanarPixels(w, h int) []uint8 { return m.VGA.Pixels(w, h) }

// DisplayStart 是 CRTC 的顯示起點（`VGA.DisplayStart`）。
//
// 倒畫面之前問一次：非零就表示程式在翻頁，而第 0 頁多半是上一幕。
func (m *Machine) DisplayStart() uint32 { return m.VGA.DisplayStart() }

// planarIndexed 把四個平面疊成目前模式尺寸的色號（0–15）。
func (m *Machine) planarIndexed() []uint8 {
	w, h := m.VideoSize()
	return m.VGA.Pixels(w, h)
}

// VideoRaw 是畫面記憶體的**直接切片，不複製**（給變化偵測與非零計數用）。
//
// 平面模式回四個平面的全部內容（256 KB）；mode 13h 回 A0000 起的
// 320×200。**語意是「畫面有沒有動」，不是色號**——要色號用 Indexed()。
func (m *Machine) VideoRaw() []uint8 {
	if m.planarOn {
		return m.VGA.Raw()
	}
	base := uint32(VideoSeg) * 16
	return m.Mem[base : base+VideoWidth*VideoHigh]
}

// vgaSnap 記下一次寫入與當下的顯示卡狀態。
func (m *Machine) vgaSnap(off uint32, v uint8, row int) VGAWrite {
	g := m.VGA
	return VGAWrite{
		Step: m.Steps, CS: m.CPU.Seg[cpu.CS], IP: m.CPU.IP, Off: off, Row: row, Val: v,
		Mode: g.gc[5] & 3, MapMask: g.seq[2], BitMask: g.gc[8],
		SetReset: g.gc[0], EnableSR: g.gc[1], Rotate: g.gc[3],
		Latch: g.latch,
	}
}

// ---- EGA 的診斷介面 ------------------------------------------------------
//
// EGA 與 VGA 的 16 色模式是同一套硬體（序列器 Map Mask、繪圖控制器、
// latch、write mode），所以下面這幾支就是上面那份狀態的另一個名字。
// 保留是因為 `apps/` 與 `cmd/probe` 用這組名字問問題。

// SequencerIndex 是序列器目前選中的索引。
func (m *Machine) SequencerIndex() uint8 { return m.VGA.SeqIndex() }

// MapMask 是序列器 index 02h。
func (m *Machine) MapMask() uint8 { return m.VGA.MapMask() }

// EGAPlanarActive 回目前是不是平面模式。見 Machine.planarActive。
func (m *Machine) EGAPlanarActive() bool { return m.planarOn }

// EGAPlane 回第 plane 個平面的內容（直接切片，不複製）。
func (m *Machine) EGAPlane(plane int) []uint8 { return m.VGA.Planes[plane&3] }

// IndexedEGASize 把四個平面解成 w×h 的色號陣列。
func (m *Machine) IndexedEGASize(w, h int) []uint8 { return m.VGA.Pixels(w, h) }

// IndexedEGA 用目前模式的尺寸解畫面。
func (m *Machine) IndexedEGA() []uint8 { return m.planarIndexed() }

// planarActive 決定 `A0000` 走不走平面路徑。
//
// 兩條分支各自帶著一個判準，而兩個都對——對它們自己那支程式而言：
//
//   - **視訊模式**（wolong）：切模式前後行為要一致，切回去也要恢復。
//     用「程式寫過 3CE 沒有」當開關會讓同一支程式在不同時間落進不同行為。
//   - **Map Mask 訊號**（san1／pool-of-radiance）：有些程式從來不呼叫
//     `int 10h AH=00`，直接自己設暫存器，BDA 的模式位元組一路是 03h。
//     只認模式的話那種程式永遠走不到平面路徑。
//
// 合起來的判準：**模式是平面模式就是平面**；模式是 13h（程式明講要線性）
// 就不是；其餘（含文字模式）交給 Map Mask 訊號決定。文字模式的畫面在
// `B8000`，與 `A0000` 不衝突，所以在文字模式下承認訊號是安全的。
//
// ⚠ 訊號那一半是**假說**，不是量到的事實。
func (m *Machine) planarActive() bool {
	mode := m.VideoMode()
	if planarMode(mode) {
		return true
	}
	if mode == 0x13 {
		return false
	}
	return m.VGA.planarSeen
}
