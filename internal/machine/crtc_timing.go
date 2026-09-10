package machine

// CRTC 的時序（`docs/spec/193`）。
//
// `0x3DA` 的預設實作是**行為模型**：拿讀取次數的 bit4 翻轉，讓兩種等待
// 迴圈都轉得出來。那讓程式跑得下去，代價是回掃的頻率由程式輪詢的快慢
// 決定，不由 CRTC 決定——程式拿回掃當時鐘量速度時，量到的是自己。
//
// 這裡從時序暫存器算出一幀的形狀，再從指令數換算掃到哪一條線。
// **預設不啟用**（`Machine.CRTCTiming`）：既有的對拍收據都建立在行為
// 模型的 `in` 回傳序列上，換掉會讓每一份都失效。
//
// ⚠ **這一層只回答「現在掃到哪裡」，不記錄暫存器變更的時間軸。**
// 所以程式在掃描中途改暫存器（改調色盤做漸層天空、改顯示起點做分割
// 捲動）時，畫面仍然整張用最後一組值解——上下半部應該不同的地方會
// 變成同一種。要重現那個得逐掃描線回放，見 `docs/worklist.json` 的
// `raster-per-scanline-replay`。

// 像素時鐘的兩個選擇（Misc Output 的 bit2–3）。
const (
	pixelClock25MHz = 25_175_000.0
	pixelClock28MHz = 28_322_000.0
)

// Timing 是從 CRTC 算出來的掃描時序。單位：時鐘是 Hz，其餘是字元或掃描列。
type Timing struct {
	PixelClock float64 // 像素時鐘
	CharClock  float64 // 字元時鐘（像素時鐘 ÷ 一個字元幾個像素）
	CharWidth  int     // 一個字元幾個像素（8 或 9）

	HTotal      int // 一條線幾個字元時鐘（含空白期）
	HDisplayEnd int // 顯示區到第幾個字元
	VTotal      int // 一幀幾條掃描線（含空白期）
	VDisplayEnd int // 顯示區到第幾條線

	VRetraceStart int // 垂直回掃從第幾條線開始
	VBlankStart   int // 垂直空白從第幾條線開始

	LineHz  float64 // 水平頻率
	FrameHz float64 // 垂直頻率（refresh rate）
}

// Timing 從時序暫存器算出這一幀的形狀。
//
// 修正項照 DOSBox 的 `VGA_SetupDrawing`：`htotal` 是 CRTC 的值加 3
// （VGA 專屬）再加 2（共通），`vtotal` 加 2。驗算過 VGA 的兩個標準模式
// ——`12h` 得 31.47 kHz／60.06 Hz、`13h` 得 31.47 kHz／70.09 Hz，
// 都落在標準值上（`docs/spec/193` §2）。
//
// ⚠ **垂直方向的三個位置都是 10 位元**，高位散在 overflow（`07`）與
// max scan line（`09`）裡。只讀低 8 位的症狀是「回掃在畫面中間就開始
// 了」，看起來像時序算錯，不像少讀了兩個位元。
func (v *VGA) Timing() Timing {
	t := Timing{PixelClock: pixelClock25MHz, CharWidth: 8}
	if (v.misc>>2)&3 != 0 {
		t.PixelClock = pixelClock28MHz
	}
	if v.seq[1]&1 == 0 {
		t.CharWidth = 9
	}
	t.CharClock = t.PixelClock / float64(t.CharWidth)

	ov := v.crtc[0x07]
	t.HTotal = int(v.crtc[0x00]) + 5
	t.HDisplayEnd = int(v.crtc[0x01]) + 1
	t.VTotal = int(v.crtc[0x06]) | int(ov>>0&1)<<8 | int(ov>>5&1)<<9
	t.VTotal += 2
	t.VDisplayEnd = int(v.crtc[0x12]) | int(ov>>1&1)<<8 | int(ov>>6&1)<<9 + 1
	t.VRetraceStart = int(v.crtc[0x10]) | int(ov>>2&1)<<8 | int(ov>>7&1)<<9
	t.VBlankStart = int(v.crtc[0x15]) | int(ov>>3&1)<<8 | int(v.crtc[0x09]>>5&1)<<9

	if t.HTotal > 0 {
		t.LineHz = t.CharClock / float64(t.HTotal)
	}
	if t.VTotal > 0 {
		t.FrameHz = t.LineHz / float64(t.VTotal)
	}
	return t
}

// ScanLine 回現在掃到第幾條線（0 到 VTotal−1），以及時序是否算得出來。
//
// 時間由指令數驅動，機器速度是 `StepsPerSecond()`
// （`docs/spec/190`）。所以一條線幾道指令 ＝ 速度 ÷ 水平頻率。
//
// ⚠ **這不是週期精確的**：它把機器速度當成常數，而真機上一道指令花多久
// 隨指令而異（`docs/spec/191`）。回掃的**頻率**對得上 CRTC，
// 單一次讀取落在哪個位置則是近似。
func (m *Machine) ScanLine() (line int, ok bool) {
	t := m.VGA.Timing()
	if t.LineHz <= 0 || t.VTotal <= 0 {
		return 0, false
	}
	perLine := StepsPerSecond() / t.LineHz
	if perLine < 1 {
		return 0, false
	}
	return int(float64(m.Steps)/perLine) % t.VTotal, true
}

// statusFromTiming 依掃描位置算 `0x3DA` 的值。
//
// bit3 ＝ 在垂直回掃期間、bit0 ＝ 不在顯示區。**兩個都要會變**：
// 程式等的可能是任一種邊緣，少一種就死在迴圈裡。
func (m *Machine) statusFromTiming() (uint8, bool) {
	line, ok := m.ScanLine()
	if !ok {
		return 0, false
	}
	t := m.VGA.Timing()
	var v uint8
	if t.VRetraceStart > 0 && line >= t.VRetraceStart {
		v |= 0x08
	}
	if line >= t.VDisplayEnd {
		v |= 0x01 // 顯示區之外
	}
	return v, true
}

// setModeTiming 寫入 BIOS 設模式時會給的時序暫存器。
//
// 值取自標準 VGA BIOS 的模式表。驗算（25.175 MHz、8 像素字元）：
// 640×480 得 31.47 kHz／59.94 Hz，320×200 得 31.47 kHz／70.09 Hz，
// 都落在 VGA 的標準值上。
//
// **不在表上的模式不寫。** 那時 `Timing()` 算出來的頻率會是 0，
// 而 `ScanLine` 回 `ok ＝ false`，`0x3DA` 退回行為模型——
// 寫一組猜的值會讓呼叫端拿它當事實。
func setModeTiming(crtc *[32]uint8, mode uint8) {
	// 依掃描線數分組：480 線（`11h`／`12h`）、400 線（`13h`／`0Dh`／`0Eh`，
	// 200 列是 double scan）、350 線（`0Fh`／`10h`）。
	type timing struct{ hTotal, hEnd, vTotal, overflow, vRetrace, vEnd, vBlank, maxScan uint8 }
	var t timing
	switch mode {
	case 0x11, 0x12: // 640×480
		t = timing{0x5F, 0x4F, 0x0B, 0x3E, 0xEA, 0xDF, 0xE7, 0x40}
	case 0x0D, 0x13: // 320×200（400 掃描線）
		t = timing{0x5F, 0x27, 0xBF, 0x1F, 0x9C, 0x8F, 0x96, 0x41}
	case 0x0E: // 640×200（400 掃描線）
		t = timing{0x5F, 0x4F, 0xBF, 0x1F, 0x9C, 0x8F, 0x96, 0x41}
	case 0x0F, 0x10: // 640×350
		t = timing{0x5F, 0x4F, 0xBF, 0x1F, 0x83, 0x5D, 0x63, 0x40}
	default:
		return // 不在表上：讓 Timing 算出 0，呼叫端據此退回行為模型
	}
	crtc[0x00], crtc[0x01] = t.hTotal, t.hEnd
	crtc[0x06], crtc[0x07] = t.vTotal, t.overflow
	crtc[0x10], crtc[0x12], crtc[0x15] = t.vRetrace, t.vEnd, t.vBlank
	// max scan line 的 bit0–4 ＝ 每個字元列幾條掃描線 − 1。
	// **200 列的模式每列掃兩次**，所以 CRTC 說的 400 條掃描線是 200 列
	// ——不除的話 `Size()` 會回兩倍的高度。
	crtc[0x09] = crtc[0x09]&0xE0 | t.maxScan&0x1F
}
