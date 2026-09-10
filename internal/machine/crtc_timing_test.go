package machine

import (
	"math"
	"testing"
)

// 時序暫存器（`docs/spec/193`）。**預設不啟用**——既有的對拍收據都
// 建立在 3DA 的行為模型上，換掉會讓每一份都失效。

// TestTimingMatchesVGAStandardModes 是規格 §5 的第 1 條。
//
// 這兩個模式的 refresh rate 是 VGA 的標準值，公式算錯會立刻偏掉——
// 所以它同時驗了像素時鐘、字元寬度與 htotal／vtotal 的修正項。
func TestTimingMatchesVGAStandardModes(t *testing.T) {
	for _, c := range []struct {
		mode     uint8
		wantHKHz float64
		wantVHz  float64
	}{
		{0x12, 31.47, 59.94}, // 640×480（與 NTSC 同源，不是整數 60）
		{0x13, 31.47, 70.09}, // 320×200
	} {
		m := New()
		m.SetVideoMode(c.mode)
		tm := m.VGA.Timing()
		if got := tm.LineHz / 1000; math.Abs(got-c.wantHKHz) > 0.05 {
			t.Errorf("模式 %02Xh 水平 %.2f kHz，預期 %.2f", c.mode, got, c.wantHKHz)
		}
		if got := tm.FrameHz; math.Abs(got-c.wantVHz) > 0.05 {
			t.Errorf("模式 %02Xh 垂直 %.2f Hz，預期 %.2f", c.mode, got, c.wantVHz)
		}
	}
}

// TestVerticalPositionsUseOverflowBits 是規格 §5 的第 2 條。
//
// 三個垂直位置都是 10 位元，高位散在 overflow 裡——**只讀低 8 位的
// 症狀是「回掃在畫面中間就開始了」**，看起來像時序算錯，
// 不像少讀了兩個位元。
func TestVerticalPositionsUseOverflowBits(t *testing.T) {
	m := planar(t)
	// 全部設成需要第 8 位與第 9 位才表達得出來的值。
	crtcReg(m, 0x06, 0x0B)      // vertical total 低 8 位
	crtcReg(m, 0x10, 0xEA)      // retrace start 低 8 位
	crtcReg(m, 0x15, 0xE7)      // blank start 低 8 位
	crtcReg(m, 0x07, 0x3E)      // overflow：total bit0/5、retrace bit2/7、blank bit3
	crtcReg(m, 0x09, 0x40|0x20) // max scan line bit5 ＝ blank 的第 9 位

	tm := m.VGA.Timing()
	if tm.VTotal <= 0xFF+2 {
		t.Errorf("VTotal ＝ %d，看起來只讀了低 8 位", tm.VTotal)
	}
	if tm.VRetraceStart <= 0xFF {
		t.Errorf("VRetraceStart ＝ %d，看起來只讀了低 8 位", tm.VRetraceStart)
	}
	if tm.VBlankStart <= 0xFF {
		t.Errorf("VBlankStart ＝ %d，看起來只讀了低 8 位", tm.VBlankStart)
	}
}

// TestStatusPortUnchangedWhenTimingOff 是規格 §5 的第 3 條，也是這份
// 規格最重要的一條：**預設行為一個位元都不能變**。
func TestStatusPortUnchangedWhenTimingOff(t *testing.T) {
	m := New()
	if m.CRTCTiming {
		t.Fatal("CRTCTiming 預設就是開的——既有的對拍收據會全部失效")
	}
	// 綁行為不綁相位：行為模型只有兩種值，而且每 16 次讀翻轉一次。
	// 相位（第一次讀落在哪一段）是實作細節，程式的等待迴圈不在乎。
	var got []uint8
	for i := 0; i < 64; i++ {
		got = append(got, m.In8(0x3DA))
	}
	seen := map[uint8]int{}
	flips := 0
	for i, v := range got {
		seen[v]++
		if i > 0 && v != got[i-1] {
			flips++
		}
	}
	if len(seen) != 2 || seen[0x00] == 0 || seen[0x09] == 0 {
		t.Fatalf("讀到的值是 %v，行為模型應該只有 00 與 09 兩種", seen)
	}
	// 64 次讀、每 16 次翻一次 ＝ 三或四次邊緣，看第一次讀落在段內哪裡。
	if flips < 3 || flips > 4 {
		t.Errorf("64 次讀翻轉 %d 次，預期 3 或 4（每 16 次一次）", flips)
	}
}

// TestStatusPortFollowsCRTCWhenTimingOn 是規格 §5 的第 4 條。
//
// 兩件事都要成立：**兩種等待迴圈都轉得出來**（少一種程式就死在迴圈裡），
// 而且回掃的頻率與 CRTC 算出來的一致（那是這個模型存在的理由）。
func TestStatusPortFollowsCRTCWhenTimingOn(t *testing.T) {
	m := New()
	m.SetVideoMode(0x12)
	m.CRTCTiming = true

	tm := m.VGA.Timing()
	stepsPerFrame := StepsPerSecond() / tm.FrameHz

	// 掃過整整一幀，數回掃期間佔幾成。
	var inRetrace, total int
	for s := 0; s < int(stepsPerFrame); s += 64 {
		m.Steps = uint64(s)
		total++
		if m.In8(0x3DA)&0x08 != 0 {
			inRetrace++
		}
	}
	if inRetrace == 0 {
		t.Fatal("整整一幀都沒有回掃——等回掃開始的迴圈會死在裡面")
	}
	if inRetrace == total {
		t.Fatal("整整一幀都在回掃——等回掃結束的迴圈會死在裡面")
	}
	// 回掃佔一幀的比例 ＝ (vtotal − retrace start) / vtotal。
	want := float64(tm.VTotal-tm.VRetraceStart) / float64(tm.VTotal)
	if got := float64(inRetrace) / float64(total); math.Abs(got-want) > 0.05 {
		t.Errorf("回掃佔 %.1f%%，CRTC 說應該是 %.1f%%", got*100, want*100)
	}
}

// TestPixelClockFollowsMiscOutput 釘住像素時鐘的來源。
//
// Misc Output（`3C2`）的 bit2–3 選時鐘。少了它，所有模式都會被算成
// 25.175 MHz——而 28.322 MHz 那一組（720 寬的文字模式）會差 12%。
func TestPixelClockFollowsMiscOutput(t *testing.T) {
	m := planar(t)
	m.Out8(0x3C2, 0x00) // bit2-3 ＝ 0
	if got := m.VGA.Timing().PixelClock; math.Abs(got-25_175_000) > 1 {
		t.Errorf("時鐘選擇 0 ＝ %.0f Hz，預期 25,175,000", got)
	}
	m.Out8(0x3C2, 0x04) // bit2-3 ＝ 1
	if got := m.VGA.Timing().PixelClock; math.Abs(got-28_322_000) > 1 {
		t.Errorf("時鐘選擇 1 ＝ %.0f Hz，預期 28,322,000", got)
	}
}

// TestSizeDividesByScanLinesPerRow 釘住「CRTC 說的是掃描線，不是像素列」。
//
// 200 列的模式每個字元列掃兩次（max scan line ＝ 1），CRTC 的
// vertical display end 是 400。不除的話 `Size()` 回兩倍的高度，
// 而畫面會被解成上下各一半、每一列重複——**看起來像圖被拉長了**，
// 不像少除了一個數。
func TestSizeDividesByScanLinesPerRow(t *testing.T) {
	for _, c := range []struct {
		mode uint8
		w, h int
	}{
		{0x12, 640, 480}, // 每列一條掃描線
		{0x0D, 320, 200}, // 每列兩條
		{0x0E, 640, 200}, // 每列兩條
		{0x10, 640, 350}, // 每列一條
	} {
		m := New()
		m.SetVideoMode(c.mode)
		w, h := m.VGA.Size()
		if w != c.w || h != c.h {
			t.Errorf("模式 %02Xh：Size ＝ %d×%d，預期 %d×%d", c.mode, w, h, c.w, c.h)
		}
		// 時序那一邊看的是掃描線，兩者要對得起來。
		tm := m.VGA.Timing()
		rows := int(m.VGA.crtc[0x09]&0x1F) + 1
		if tm.VDisplayEnd != c.h*rows {
			t.Errorf("模式 %02Xh：時序的 VDisplayEnd ＝ %d，預期 %d 條掃描線",
				c.mode, tm.VDisplayEnd, c.h*rows)
		}
	}
}
