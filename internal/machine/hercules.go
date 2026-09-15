package machine

// Hercules 圖形模式的顯示記憶體。
//
// Hercules（HGC）把畫面放在 `B0000`，不在 `A0000`（EGA／VGA）也不在
// `B8000`（CGA／文字）。1 bpp，四個 bank 交錯：第 r 列在
// `bank(r%4)*0x2000 + (r/4)*stride`。頁 0 是 `B0000`–`B7FFF`，頁 1 是
// `B8000`–`BFFFF`（與 CGA 撞位址），顯示哪一頁由模式埠 `3B8h` 的 bit 7 決定。
//
// **幾何不是常數。** HGC 的標準是 720×348（每列 45 個字 ＝ 90 bytes，
// 87 個字列 × 4 條掃描線），但那只是 BIOS 沒有的預設——程式自己寫 6845
// CRTC（`3B4h` 索引、`3B5h` 資料）就能改。三國演義寫的是 R1＝40、R6＝102、
// R9＝3，也就是 **640×408、每列 80 bytes**，與它的 EGA 畫面同一個幾何
// （softworld_san `docs/spec/006`）。拿 720×348 去解會得到一張有規律的雜訊，
// 而那不會報錯。所以解碼前先問 CRTC：
//
//	寬 ＝ R1 × 16 像素（每字 16 點：兩個 byte）  stride ＝ R1 × 2
//	高 ＝ R6 × (R9 + 1)
//
// R1／R6 沒被寫過（＝ 0）才退回標準的 720×348。
//
// **它是線性記憶體，不走平面路徑**：`planarOn` 只接管 `A0000`–`AFFFF`，
// `B0000` 一直在 `Mem` 裡。所以 `WatchWrites`／`Bytes` 本來就看得到它；
// 缺的只是「把它當畫面解出來」與「probe 報那一塊有沒有東西」——
// 少了這兩件事，選 Hercules 的程式在 probe 眼裡是「一次都沒畫」的假零。
const (
	HerculesBase = 0xB0000
	HerculesPage = 0x8000
	herculesBank = 0x2000

	hgcDefaultCols = 45 // 720 px
	hgcDefaultRows = 87 // × 4 條掃描線 ＝ 348
	hgcDefaultScan = 3  // R9：每字列 4 條掃描線
)

// hgcState 是程式寫進 6845 與模式埠的值。
type hgcState struct {
	idx  uint8
	crtc [18]uint8
	mode uint8 // 3B8h：bit1 圖形、bit3 顯示開、bit7 顯示頁 1
}

func (m *Machine) hgcOut(p uint16, v uint8) {
	switch p {
	case 0x3B4:
		m.hgc.idx = v
	case 0x3B5:
		if int(m.hgc.idx) < len(m.hgc.crtc) {
			m.hgc.crtc[m.hgc.idx] = v
		}
	case 0x3B8:
		m.hgc.mode = v
	}
}

// HerculesGeometry 回目前 CRTC 設定下的畫面寬、高與每列的位元組數。
func (m *Machine) HerculesGeometry() (w, h, stride int) {
	cols, rows, scan := int(m.hgc.crtc[1]), int(m.hgc.crtc[6]), int(m.hgc.crtc[9])
	if cols == 0 || rows == 0 {
		cols, rows, scan = hgcDefaultCols, hgcDefaultRows, hgcDefaultScan
	}
	return cols * 16, rows * (scan + 1), cols * 2
}

// HerculesPageBase 回顯示中那一頁的線性位址（`3B8h` bit 7 選頁 1）。
func (m *Machine) HerculesPageBase() uint32 {
	if m.hgc.mode&0x80 != 0 {
		return HerculesBase + HerculesPage
	}
	return HerculesBase
}

// HerculesNonZero 回 `B0000` 頁 0 裡非零位元組的數量（0..32768）。
//
// probe 用它報「那一塊有沒有東西」。判準是位元組不是像素：
// Hercules 是 1 bpp，一個非零位元組就是至少一個亮點。固定看頁 0——
// 翻頁的程式兩頁都會畫，而「有沒有畫」問頁 0 就夠。
func (m *Machine) HerculesNonZero() int {
	n := 0
	for a := uint32(HerculesBase); a < HerculesBase+HerculesPage; a++ {
		if m.Mem[a] != 0 {
			n++
		}
	}
	return n
}

// Hercules 把顯示中的那一頁解成 w×h 的 0／1 陣列（列優先），
// 寬高照 `HerculesGeometry`。
//
// 只解碼，不看模式埠的圖形位元——程式沒切進圖形模式時這裡解出來的是
// 文字頁的位元圖樣，看起來像雜訊而不像空白；要分辨「沒畫」與「畫了」
// 看 `HerculesNonZero`。
func (m *Machine) Hercules() []uint8 {
	w, h, stride := m.HerculesGeometry()
	base := m.HerculesPageBase()
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		row := base + uint32(y&3)*herculesBank + uint32(y>>2)*uint32(stride)
		for x := 0; x < w; x++ {
			a := row + uint32(x>>3)
			if a >= HerculesBase+2*HerculesPage {
				break
			}
			if m.Mem[a]&(0x80>>uint(x&7)) != 0 {
				out[y*w+x] = 1
			}
		}
	}
	return out
}
