package machine

// Hercules 圖形模式的顯示記憶體。
//
// Hercules（HGC）把畫面放在 `B0000`，不在 `A0000`（EGA／VGA）也不在
// `B8000`（CGA／文字）。720×348、每列 90 個位元組、1 bpp，四個 bank
// 交錯：第 r 列在 `bank(r%4)*0x2000 + (r/4)*90`。頁 0 是 `B0000`–`B7FFF`，
// 頁 1 是 `B8000`–`BFFFF`（與 CGA 撞位址，程式選了哪一頁由 `3B8h` 埠決定，
// 這裡固定取頁 0——原版的 DOS 遊戲幾乎都只用頁 0）。
//
// **它是線性記憶體，不走平面路徑**：`planarOn` 只接管 `A0000`–`AFFFF`，
// `B0000` 一直在 `Mem` 裡。所以 `WatchWrites`／`Bytes` 本來就看得到它；
// 缺的只是「把它當畫面解出來」與「probe 報那一塊有沒有東西」——
// 少了這兩件事，選 Hercules 的程式在 probe 眼裡是「一次都沒畫」的假零。
const (
	HerculesBase   = 0xB0000
	HerculesPage   = 0x8000
	HerculesWidth  = 720
	HerculesHeight = 348
	herculesStride = 90
	herculesBank   = 0x2000
)

// HerculesNonZero 回 `B0000` 頁 0 裡非零位元組的數量（0..32768）。
//
// probe 用它報「那一塊有沒有東西」。判準是位元組不是像素：
// Hercules 是 1 bpp，一個非零位元組就是至少一個亮點。
func (m *Machine) HerculesNonZero() int {
	n := 0
	for a := uint32(HerculesBase); a < HerculesBase+HerculesPage; a++ {
		if m.Mem[a] != 0 {
			n++
		}
	}
	return n
}

// Hercules 把 `B0000` 頁 0 解成 720×348 的 0／1 陣列（列優先）。
//
// 只解碼，不看 `3B8h`／`3BFh` 的模式位元——程式沒切進圖形模式時
// 這裡解出來的是文字頁的位元圖樣，看起來像雜訊而不像空白；
// 要分辨「沒畫」與「畫了」看 `HerculesNonZero`。
func (m *Machine) Hercules() []uint8 {
	out := make([]uint8, HerculesWidth*HerculesHeight)
	for y := 0; y < HerculesHeight; y++ {
		row := uint32(HerculesBase) + uint32(y&3)*herculesBank + uint32(y>>2)*herculesStride
		for x := 0; x < HerculesWidth; x++ {
			if m.Mem[row+uint32(x>>3)]&(0x80>>uint(x&7)) != 0 {
				out[y*HerculesWidth+x] = 1
			}
		}
	}
	return out
}
