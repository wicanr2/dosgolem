package machine

import "testing"

// 這一組測試逐項對照 `docs/spec/007` §3.3 的算式。
//
// ⚠ **單元測試綠不代表接對了。** 這一層真正的接線點是 `Read8`／`Write8`
// 的分支，直接呼叫 `vga.Write()` 的測試把那個分支拿掉照樣全綠——
// 所以最後一支測試走機器層。

// setGC 是「先寫索引再寫資料」的縮寫。
func setGC(v *VGA, idx, val uint8) { v.Out(0x3CE, idx); v.Out(0x3CF, val) }
func setSeq(v *VGA, idx, val uint8) { v.Out(0x3C4, idx); v.Out(0x3C5, val) }

// TestWriteMode0Latch 釘住「dummy read 載 latch」這一步。
//
// 文字繪製走的就是這條（`docs/re/28` §1，臥龍傳專案）：
// 先讀一次 VRAM 把四個平面載進 latch，再寫字型 byte。
// **漏掉 latch 的實作會把沒被字型位元覆蓋的平面清成 0**，
// 畫面上看起來只是「顏色不對」，不是「字沒出來」。
func TestWriteMode0Latch(t *testing.T) {
	v := newVGA()
	// 底下先鋪一個 15 號色（四個平面都是 FF）。
	for p := 0; p < 4; p++ {
		v.Planes[p][0] = 0xFF
	}
	setGC(v, 5, 0)    // 寫入模式 0
	setGC(v, 1, 0x0F) // 四個平面都用 Set/Reset
	setGC(v, 0, 0x01) // Set/Reset ＝ 1 號色
	setGC(v, 8, 0xF0) // 只動高 4 個 bit

	v.Read(0) // ← dummy read，載 latch
	v.Write(0, 0xFF)

	// 高 4 bit 變成 1 號色（平面 0 ＝ 1、其餘 0），低 4 bit 保持 15 號色。
	want := [4]uint8{0xFF, 0x0F, 0x0F, 0x0F}
	for p := 0; p < 4; p++ {
		if got := v.Planes[p][0]; got != want[p] {
			t.Errorf("平面 %d ＝ %02X，預期 %02X", p, got, want[p])
		}
	}
}

// TestWriteMode0NoLatchLosesPlanes 是上一支的反面：**沒有 dummy read
// 就會掉平面**。這一支證明測試真的在量 latch，不是在量別的東西。
func TestWriteMode0NoLatchLosesPlanes(t *testing.T) {
	v := newVGA()
	for p := 0; p < 4; p++ {
		v.Planes[p][0] = 0xFF
	}
	setGC(v, 5, 0)
	setGC(v, 1, 0x0F)
	setGC(v, 0, 0x01)
	setGC(v, 8, 0xF0)

	v.Write(0, 0xFF) // 沒有 dummy read

	if v.Planes[1][0] != 0x00 {
		t.Fatalf("平面 1 ＝ %02X，預期 00——沒載 latch 的話低 4 bit 應該被清掉，"+
			"這一支測不到 latch 就沒有意義", v.Planes[1][0])
	}
}

// TestWriteMode3 釘住清畫面走的那條路（`sub_1ED93`）。
//
// 模式 3：CPU 的資料與位元遮罩取交集，顏色一律來自 Set/Reset，
// **不看 Enable Set/Reset**。
func TestWriteMode3(t *testing.T) {
	v := newVGA()
	setGC(v, 5, 3)
	setGC(v, 0, 0x0A) // Set/Reset ＝ 10 號色（平面 1 與 3）
	setGC(v, 1, 0x00) // Enable Set/Reset 全關——模式 3 應該無視它
	setGC(v, 3, 0x00)
	setGC(v, 8, 0xFF)

	v.Write(0, 0xCC)

	want := [4]uint8{0x00, 0xCC, 0x00, 0xCC}
	for p := 0; p < 4; p++ {
		if got := v.Planes[p][0]; got != want[p] {
			t.Errorf("平面 %d ＝ %02X，預期 %02X（模式 3 不看 Enable Set/Reset）",
				p, got, want[p])
		}
	}
}

// TestWriteMode1CopiesLatch 釘住搬區塊的快路徑：latch 原封不動寫回去，
// 位元遮罩與 ALU 都不參與。
func TestWriteMode1CopiesLatch(t *testing.T) {
	v := newVGA()
	for p := 0; p < 4; p++ {
		v.Planes[p][10] = uint8(0x11 * (p + 1))
	}
	setGC(v, 5, 1)
	setGC(v, 8, 0x00) // 位元遮罩全關：模式 1 應該照樣整個搬過去

	v.Read(10)
	v.Write(20, 0x00)

	for p := 0; p < 4; p++ {
		if got, want := v.Planes[p][20], uint8(0x11*(p+1)); got != want {
			t.Errorf("平面 %d ＝ %02X，預期 %02X", p, got, want)
		}
	}
}

// TestWriteMode2 釘住「CPU 資料的每個 bit 選一個平面」。
func TestWriteMode2(t *testing.T) {
	v := newVGA()
	setGC(v, 5, 2)
	setGC(v, 8, 0xFF)
	setGC(v, 3, 0)

	v.Read(0)
	v.Write(0, 0x05) // 平面 0 與 2 全開

	want := [4]uint8{0xFF, 0x00, 0xFF, 0x00}
	for p := 0; p < 4; p++ {
		if got := v.Planes[p][0]; got != want[p] {
			t.Errorf("平面 %d ＝ %02X，預期 %02X", p, got, want[p])
		}
	}
}

// TestMapMaskBlocksPlanes 釘住序列器的 Map Mask。
func TestMapMaskBlocksPlanes(t *testing.T) {
	v := newVGA()
	setGC(v, 5, 2)
	setGC(v, 8, 0xFF)
	setSeq(v, 2, 0x05) // 只有平面 0 與 2 吃這次寫入

	v.Read(0)
	v.Write(0, 0x0F) // 四個平面都想全開

	want := [4]uint8{0xFF, 0x00, 0xFF, 0x00}
	for p := 0; p < 4; p++ {
		if got := v.Planes[p][0]; got != want[p] {
			t.Errorf("平面 %d ＝ %02X，預期 %02X", p, got, want[p])
		}
	}
}

// TestReadMode1ColorCompare 釘住「哪些像素等於某個顏色」。
func TestReadMode1ColorCompare(t *testing.T) {
	v := newVGA()
	// 高 4 個像素是 5 號色（平面 0 與 2），低 4 個是 0。
	v.Planes[0][0], v.Planes[2][0] = 0xF0, 0xF0

	setGC(v, 5, 0x08) // 讀取模式 1
	setGC(v, 2, 0x05) // 比較值 ＝ 5
	setGC(v, 7, 0x0F) // 四個平面都要比

	if got := v.Read(0); got != 0xF0 {
		t.Errorf("色彩比較回 %02X，預期 F0", got)
	}
	setGC(v, 7, 0x00) // 都不比 → 每個像素都算命中
	if got := v.Read(0); got != 0xFF {
		t.Errorf("Don't Care 全開時回 %02X，預期 FF", got)
	}
}

// TestACFlipFlopResetsOn3DA 釘住屬性控制器的索引／資料相位。
//
// 少了「讀 3DA 重設 flip-flop」這一條，程式重設之後我們的相位與它相反，
// 於是索引被當成資料、資料被當成索引，**整份調色盤錯位**。
func TestACFlipFlopResetsOn3DA(t *testing.T) {
	v := newVGA()
	v.resetMode() // 調色盤是 identity，動到哪一格看得出來
	v.Out(0x3C0, 0x02) // 索引
	v.ResetACFlip()    // ← 程式在中途讀了 3DA
	v.Out(0x3C0, 0x03) // 這一次仍然是索引
	v.Out(0x3C0, 0x09) // 這一次才是資料
	if v.ac[3] != 0x09 {
		t.Errorf("調色盤暫存器 3 ＝ %02X，預期 09", v.ac[3])
	}
	if v.ac[2] != 0x02 {
		t.Errorf("調色盤暫存器 2 ＝ %02X，預期還是 identity 的 02——"+
			"重設 flip-flop 之後那一次寫的是索引不是資料", v.ac[2])
	}
}

// TestMachineRoutesPlanarMemory 是**接線測試**。
//
// 上面每一支都直接呼叫 VGA，把 `Read8`／`Write8` 的分支整段拿掉照樣全綠。
// 這一支從機器層進去：設 mode 12h、用 `Write8` 寫、用 `Planar()` 讀。
func TestMachineRoutesPlanarMemory(t *testing.T) {
	m := New()
	m.SetVideoMode(0x12)
	if w, h, px := m.Planar(); w != 640 || h != 480 || px == nil {
		t.Fatalf("mode 12h 的畫面是 %d×%d，預期 640×480", w, h)
	}
	// 模式 2 ＋ 位元遮罩：把最左邊那個像素設成 12 號色。
	m.Out8(0x3CE, 5)
	m.Out8(0x3CF, 2)
	m.Out8(0x3CE, 8)
	m.Out8(0x3CF, 0x80)
	m.Read8(VideoSeg * 16)
	m.Write8(VideoSeg*16, 0x0C)

	_, _, px := m.Planar()
	if px[0] != 12 {
		t.Errorf("(0,0) ＝ %d，預期 12——Write8 有沒有走進 VGA？", px[0])
	}
	if px[1] != 0 {
		t.Errorf("(1,0) ＝ %d，預期 0（位元遮罩只開了最高位）", px[1])
	}
	// **不是平面模式就要回 nil，不是回一片全 0 的畫面。**
	m.SetVideoMode(0x13)
	if _, _, px := m.Planar(); px != nil {
		t.Error("mode 13h 也回了平面畫面——那會讓「模式不對」看起來像「畫面全黑」")
	}
}

// TestPlanarSnapshotRoundTrip 釘住快照有沒有帶到平面。
//
// 漏掉的話：還原之後記憶體與 CPU 全對，畫面卻是還原之前那一張——
// 看起來像「遊戲沒重畫」，不像「快照少存東西」。
func TestPlanarSnapshotRoundTrip(t *testing.T) {
	m := New()
	m.SetVideoMode(0x12)
	m.Out8(0x3CE, 5)
	m.Out8(0x3CF, 2)
	m.Read8(VideoSeg * 16)
	m.Write8(VideoSeg*16, 0x0F)

	snap := m.Snapshot()
	m.Write8(VideoSeg*16, 0x00) // 塗掉
	if _, _, px := m.Planar(); px[0] != 0 {
		t.Fatal("塗掉之後 (0,0) 不是 0，這一支測不到還原")
	}
	m.Restore(snap)
	if _, _, px := m.Planar(); px[0] != 15 {
		t.Errorf("還原之後 (0,0) ＝ %d，預期 15", px[0])
	}
}

// TestWriteMode3AppliesALU 釘住「模式 3 的功能選擇照常作用」。
//
// 這一條漏掉的症狀是**一次出現兩個看起來無關的錯**：
// 用 XOR 反白一列的程式會把那一列的字整個蓋掉，反白條的顏色也跟著錯。
// 單看任何一個都不會指向寫入模式（實例：《臥龍傳》的君主清單，
// 反白列變成純黃色空白條，同一張圖上捲軸滑塊也不見了）。
func TestWriteMode3AppliesALU(t *testing.T) {
	v := newVGA()
	v.Planes[0][0] = 0xF0 // 底下已經有東西
	setGC(v, 5, 3)
	setGC(v, 0, 0x01)         // Set/Reset ＝ 平面 0
	setGC(v, 3, 0x03<<3)      // 功能選擇 ＝ XOR
	setGC(v, 8, 0xFF)

	v.Read(0)
	v.Write(0, 0xFF) // 位元遮罩全開

	if got := v.Planes[0][0]; got != 0x0F {
		t.Errorf("平面 0 ＝ %02X，預期 0F（FF XOR F0）——模式 3 沒套功能選擇？", got)
	}
}

// TestSnapshotKeepsClockAndCallbacks 釘住快照有沒有帶到時鐘與回呼。
//
// 漏掉的症狀與 `nextIRQ0` 那個坑同一個形狀：從快照展開的機器記憶體與 CPU
// 全對，但**時鐘不會走**，或者卡在一個永遠回不來的回呼裡——
// 而畫面看起來完全正常。
func TestSnapshotKeepsClockAndCallbacks(t *testing.T) {
	m := New()
	m.Write8(0x3000*16, 0xCB) // retf
	m.SetPeriodicFarCall(0x3000, 0, 1000)
	m.QueueCallback(QueuedCall{Seg: 0x3000, Off: 0})

	snap := m.Snapshot()
	m.ClearPeriodicFarCall()
	m.cbQueue = nil

	m.Restore(snap)
	if !m.periodic.on {
		t.Error("還原之後週期回呼是關的——時鐘不會走")
	}
	if m.CallbackPending() != 1 {
		t.Errorf("還原之後佇列有 %d 筆，預期 1", m.CallbackPending())
	}
}
