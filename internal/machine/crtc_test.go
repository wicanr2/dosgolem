package machine

import "testing"

// CRTC 的效果（`docs/spec/192`）。
//
// 這幾個參數少了誰，症狀都是「畫面解錯」而不是報錯——斜成平行四邊形、
// 狀態列跟著捲走、整張圖是第一列重複幾百次。

// TestPitchFollowsOffset 是規格 §4 的第 1 條：列距跟著 offset 走。
//
// **捲動的遊戲一定會改它**：邏輯畫面比可視寬，水平捲動就是把顯示起點
// 往右挪幾個位元組。列距不跟著走的話，每一列都會往同一個方向錯開
// 固定的量，整張圖斜成平行四邊形。
func TestPitchFollowsOffset(t *testing.T) {
	m := planar(t)
	// BIOS 給 640 寬的標準值：40 個字組 ＝ 80 bytes。
	if got := m.VGA.Pitch(640); got != 80 {
		t.Errorf("預設列距 %d，預期 80", got)
	}
	crtcReg(m, 0x13, 50) // 邏輯畫面加寬到 100 bytes
	if got := m.VGA.Pitch(640); got != 100 {
		t.Errorf("offset ＝ 50 時列距 %d，預期 100", got)
	}
	// 0 是「還沒被設過」，不是「列距 0」——退回用寬度推。
	crtcReg(m, 0x13, 0)
	if got := m.VGA.Pitch(640); got != 80 {
		t.Errorf("offset ＝ 0 時列距 %d，預期退回 80", got)
	}
}

// TestWiderOffsetShiftsEachRow 釘住列距真的影響解碼。
func TestWiderOffsetShiftsEachRow(t *testing.T) {
	m := planar(t)
	crtcReg(m, 0x13, 50) // 列距 100 bytes
	// 第 1 列（y=1）的第一個位元組在位址 100。
	setPlane(m, 0, 100, 0x80)
	px := m.PlanarPixels(640, 480)
	if px[640] != 1 { // (0,1)
		t.Errorf("列距 100 時 (0,1) ＝ %d，預期 1", px[640])
	}
	// 位址 80（列距 80 時才是第 1 列）現在屬於第 0 列的中段。
	if px[0] != 0 {
		t.Errorf("(0,0) ＝ %d，預期 0", px[0])
	}
}

// TestLineCompareSplitsTheScreen 是規格 §4 的第 2 條：分割線之下從
// 位址 0 起算，不受顯示起點影響。
//
// **遊戲用它固定狀態列**：上半部捲動、下半部不動。少了它，狀態列會
// 跟著地圖一起捲走。
func TestLineCompareSplitsTheScreen(t *testing.T) {
	m := planar(t)
	const pitch = 80
	setLineCompare(m, 100)       // 第 100 列開始是固定區
	setDisplayStart(m, 10*pitch) // 上半部捲到第 10 列

	setPlane(m, 0, 0, 0x80)        // 位址 0：固定區的第一列
	setPlane(m, 0, 10*pitch, 0x40) // 起點所在：可捲區的第一列

	px := m.PlanarPixels(640, 480)
	if px[1] != 1 { // (1,0) ＝ 起點那一列的第 2 個像素
		t.Errorf("可捲區第一列 ＝ %d，預期 1（跟著顯示起點）", px[1])
	}
	// 分割在**下一列**生效（`docs/spec/192` §2.3）：第 100 列還是可捲區
	// 的最後一列，第 101 列才從位址 0 起算。
	if got := px[101*640]; got != 1 {
		t.Errorf("固定區第一列（y=101）＝ %d，預期 1（從位址 0 起算）", got)
	}
	if got := px[100*640]; got != 0 {
		t.Errorf("y=100 ＝ %d，預期 0——它還是可捲區的最後一列", got)
	}
}

// TestLineCompareUsesAllNineBits 是規格 §4 的第 3 條。
//
// 9 位元散在三個暫存器裡，**只讀低 8 位的症狀是「分割線出現在畫面上方
// 某處」**——看起來像分割線設錯了，不像少讀了兩個位元。
func TestLineCompareUsesAllNineBits(t *testing.T) {
	for _, c := range []struct {
		name          string
		r18, r07, r09 uint8
		want          int
	}{
		{"只有低 8 位", 100, 0x00, 0x00, 100},
		{"第 8 位（07 的 bit4）", 100, 0x10, 0x00, 100 | 1<<8},
		{"第 9 位（09 的 bit6）", 100, 0x00, 0x40, 100 | 1<<9},
		{"三個都有", 0xFF, 0x10, 0x40, 0x3FF},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := planar(t)
			crtcReg(m, 0x18, c.r18)
			crtcReg(m, 0x07, c.r07)
			crtcReg(m, 0x09, c.r09)
			if got := m.VGA.LineCompare(); got != c.want {
				t.Errorf("LineCompare ＝ %d，預期 %d", got, c.want)
			}
		})
	}
}

// TestModeSetGivesStandardCRTCValues 是規格 §4 的第 4 條。
//
// **不給預設值的話，只設起點、不設 offset 的程式會拿到列距 0**，
// 整張圖變成第一列重複幾百次。而 mode control 的 bit6 若是 0，
// 顯示起點會被當成 word 模式而多乘一倍。
func TestModeSetGivesStandardCRTCValues(t *testing.T) {
	for _, when := range []string{"New", "SetVideoMode"} {
		t.Run(when, func(t *testing.T) {
			m := New()
			if when == "SetVideoMode" {
				m.SetVideoMode(0x12)
			}
			if got := m.VGA.crtc[0x17] & 0x40; got == 0 {
				t.Error("mode control 的 bit6 是 0——顯示起點會被當成 word 模式多乘一倍")
			}
			if got := m.VGA.Pitch(640); got != 80 {
				t.Errorf("列距 %d，預期 80", got)
			}
			if lc := m.VGA.LineCompare(); lc < 480 {
				t.Errorf("line compare ＝ %d，預期是「掃不到」的大值——"+
					"否則畫面下半部會莫名其妙從位址 0 重數", lc)
			}
		})
	}
}

// setLineCompare 設 9 位元的 line compare（散在三個暫存器）。
func setLineCompare(m *Machine, v int) {
	crtcReg(m, 0x18, uint8(v))
	r07 := m.VGA.crtc[0x07]&^0x10 | uint8(v>>8&1)<<4
	crtcReg(m, 0x07, r07)
	r09 := m.VGA.crtc[0x09]&^0x40 | uint8(v>>9&1)<<6
	crtcReg(m, 0x09, r09)
}

// TestCRTCRegistersReadBack 釘住 CRTC 讀得回自己寫進去的值。
//
// 「讀 → 改一個位元 → 寫回」是設暫存器的標準寫法。讀不回去的話程式會
// 把一個零值寫回去，而那多半是別的欄位——症狀是「設了 A 卻壞了 B」。
func TestCRTCRegistersReadBack(t *testing.T) {
	m := planar(t)
	for _, p := range []struct {
		name     string
		idx, dat uint16
	}{
		{"彩色（3D4／3D5）", 0x3D4, 0x3D5},
		{"單色（3B4／3B5）", 0x3B4, 0x3B5},
	} {
		t.Run(p.name, func(t *testing.T) {
			m.Out8(p.idx, 0x13)
			m.Out8(p.dat, 0x50)
			if got, ok := m.VGA.In(p.idx); !ok || got != 0x13 {
				t.Errorf("索引讀回 %02X（ok=%v），預期 13", got, ok)
			}
			if got, ok := m.VGA.In(p.dat); !ok || got != 0x50 {
				t.Errorf("資料讀回 %02X（ok=%v），預期 50", got, ok)
			}
		})
	}
}

// TestDisplayStartWrapsAtPlaneSize 釘住起點加列距之後在平面大小內環繞。
//
// 一個平面是 64 KB。捲到接近尾端時後面幾列會繞回開頭——真機就是這樣，
// 不繞的話會讀到平面外，而 Go 會 panic 而不是給出錯的畫面。
func TestDisplayStartWrapsAtPlaneSize(t *testing.T) {
	m := planar(t)
	setDisplayStart(m, 0xFFC0) // 離平面尾端只剩 64 bytes
	setPlane(m, 0, 0, 0x80)    // 繞回開頭的第一個位元組
	px := m.PlanarPixels(640, 480)
	// 起點 + 64 bytes 就繞回位址 0；那是第 0 列的第 512 個像素之後。
	if px[512] != 1 {
		t.Errorf("環繞後的像素 ＝ %d，預期 1", px[512])
	}
}

// TestModeSetResetsScrolling 釘住設模式會把捲動狀態清掉。
//
// 上一個模式翻到一半的頁號留著的話，新模式的第一張畫面會從半路開始
// ——而那看起來像「畫面畫錯了」，不像沒有重設暫存器。
func TestModeSetResetsScrolling(t *testing.T) {
	m := planar(t)
	setDisplayStart(m, 1000)
	setLineCompare(m, 100)
	crtcReg(m, 0x13, 60)

	m.SetVideoMode(0x12)
	if got := m.DisplayStart(); got != 0 {
		t.Errorf("設模式之後顯示起點是 %d，預期 0", got)
	}
	if got := m.VGA.Pitch(640); got != 80 {
		t.Errorf("設模式之後列距是 %d，預期 80", got)
	}
	if lc := m.VGA.LineCompare(); lc < 480 {
		t.Errorf("設模式之後 line compare ＝ %d，預期是掃不到的大值", lc)
	}
}

// TestLineCompareZeroRepeatsFirstScanline 是「分割在下一列生效」的判準
// （`docs/spec/192` §2.3）。
//
// `line compare ＝ 0` 時第一條掃描線會畫兩次——這是真機的邊界行為，
// DOSBox 的註解拿它當 `+1` 語意的理由，並附了一個踩到它的案例
// （畫面頂端多一條白線）。
//
// **只有 `+1` 產生得出這個效果**：第 0 列用顯示起點、第 1 列用位址 0，
// 起點也是 0 的話就是同一條線畫兩次。若是「那一列本身就歸零」，
// 第 0 列本來就用起點，不會重複——所以這一條同時擋住寫回 `y >= split`。
func TestLineCompareZeroRepeatsFirstScanline(t *testing.T) {
	m := planar(t)
	setLineCompare(m, 0)
	setPlane(m, 0, 0, 0x80)  // 位址 0：第 0 列與第 1 列都會讀到它
	setPlane(m, 0, 80, 0x80) // 列距 80：一般情況下第 1 列會讀這裡

	px := m.PlanarPixels(640, 480)
	if px[0] != 1 {
		t.Fatalf("第 0 列 ＝ %d，預期 1", px[0])
	}
	if px[640] != 1 {
		t.Errorf("第 1 列 ＝ %d，預期 1（line compare ＝ 0 時第一條線畫兩次）", px[640])
	}
	// 第 2 列才走到位址 80。
	if px[2*640] != 1 {
		t.Errorf("第 2 列 ＝ %d，預期 1（(y−split−1)×pitch ＝ 80）", px[2*640])
	}
}

// TestFreshVGAIsSafeToQuery 是 `newVGA` 的防呆確認。
//
// **一台剛造好、程式還沒設過任何暫存器的機器，每一個查詢都要回得出
// 能用的值。** 這裡的每一項回 0 都會讓畫面解碼安靜地壞掉：
//
//	Pitch  ＝ 0 → 每一列都讀同一個位址，整張圖是第一列重複幾百次
//	起點單位錯 → 顯示起點多乘一倍，畫面跳到別的地方
//	LineCompare 小 → 畫面莫名其妙從某一列開始重數
//
// 沒有一項會報錯——倒出來的圖看起來只是「不對」。
func TestFreshVGAIsSafeToQuery(t *testing.T) {
	v := newVGA()

	// 列距：offset 沒被設過，要退回用呼叫端給的寬度推，不能是 0。
	for _, w := range []int{320, 640} {
		if got := v.Pitch(w); got != w/8 {
			t.Errorf("Pitch(%d) ＝ %d，預期 %d", w, got, w/8)
		}
	}

	// 顯示起點：沒設過是 0，而且單位要是位元組（mode control bit6 ＝ 1）。
	if got := v.DisplayStart(); got != 0 {
		t.Errorf("DisplayStart ＝ %d，預期 0", got)
	}
	if v.crtc[0x17]&0x40 == 0 {
		t.Error("mode control 的 bit6 是 0——起點會被當成 word 而多乘一倍")
	}

	// 分割：要是「掃不到」的大值，否則畫面下半部會從位址 0 重數。
	if lc := v.LineCompare(); lc < 480 {
		t.Errorf("LineCompare ＝ %d，預期 ≥ 480", lc)
	}

	// 尺寸：**這一個要回 0**，因為 CRTC 真的還沒被設過——
	// 回一個猜的尺寸會讓呼叫端拿它當事實，而不是退回模式表。
	if w, h := v.Size(); w != 0 || h != 0 {
		t.Errorf("Size ＝ %d×%d，預期 0×0（還沒被設過）", w, h)
	}

	// 解一張圖不會 panic，而且每一列都不同（列距不是 0）。
	v.Planes[0][0] = 0x80
	v.Planes[0][v.Pitch(640)] = 0x80
	px := v.Pixels(640, 480)
	if px[0] != 1 || px[640] != 1 {
		t.Errorf("解出來的前兩列是 %d／%d，預期 1／1", px[0], px[640])
	}
}

// TestSizeComesFromCRTCNotTheModeNumber 釘住「畫面多大問 CRTC」。
//
// 模式編號只是 BIOS 的一個代號。**直接設暫存器換解析度的程式從來不改
// 它**——那種程式在模式表上會被讀成「還在上一個模式」，而畫面早就
// 不是那個大小了。
func TestSizeComesFromCRTCNotTheModeNumber(t *testing.T) {
	m := planar(t)
	m.SetVideoMode(0x12) // BIOS 說 640×480
	if w, h := m.PlanarSize(); w != 640 || h != 480 {
		t.Fatalf("設模式之後 %d×%d，預期 640×480", w, h)
	}
	// 程式自己把 CRTC 改成 640×350，模式編號不動。
	crtcReg(m, 0x01, 640/8-1)
	crtcReg(m, 0x12, uint8((350-1)&0xFF))
	crtcReg(m, 0x07, m.VGA.crtc[0x07]&^0x42|uint8((350-1)>>8&1)<<1)
	if w, h := m.PlanarSize(); w != 640 || h != 350 {
		t.Errorf("改了 CRTC 之後 %d×%d，預期 640×350——尺寸是問模式表來的", w, h)
	}
	if m.VideoMode() != 0x12 {
		t.Error("模式編號不該被動到（這正是不能拿它當尺寸依據的理由）")
	}
}
