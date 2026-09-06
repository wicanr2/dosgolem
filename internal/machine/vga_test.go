package machine

import "testing"

// VGA planar（`docs/spec/013`）的釘死測試。
//
// 這些行為錯了不會有任何錯誤訊息——畫面照樣有東西，只是內容是別的。

func planar(t *testing.T) *Machine {
	t.Helper()
	m := New()
	m.SetVideoMode(0x12)
	if !m.planarOn {
		t.Fatal("mode 12h 沒有進 planar")
	}
	return m
}

func seqReg(m *Machine, idx, v uint8) { m.Out8(0x3C4, idx); m.Out8(0x3C5, v) }
func gcReg(m *Machine, idx, v uint8)  { m.Out8(0x3CE, idx); m.Out8(0x3CF, v) }

// planeAt 讀某個 plane 的位元組（繞過 read mode，測試專用）。
func planeAt(m *Machine, p int, off uint32) uint8 {
	return m.vga.planes[p*planeSize+int(off)]
}

// TestWriteMode0MapMask 釘住「Map Mask 選哪幾個 plane」。
//
// 沒有它的話四次寫入疊在同一段記憶體上，最後一次贏——畫面看起來
// 有東西，內容卻是錯的。
func TestWriteMode0MapMask(t *testing.T) {
	m := planar(t)
	gcReg(m, 8, 0xFF) // bit mask：整個位元組
	gcReg(m, 5, 0x00) // write mode 0
	seqReg(m, 2, 0x0A)
	m.Write8(0xA0000, 0xF0)

	for p, want := range []uint8{0x00, 0xF0, 0x00, 0xF0} {
		if got := planeAt(m, p, 0); got != want {
			t.Errorf("plane%d ＝ %02X，要 %02X", p, got, want)
		}
	}
	// 色號：前四個像素 bit1 ＋ bit3 都立著 ＝ 10。
	px := m.Indexed()
	if px[0] != 0x0A || px[4] != 0x00 {
		t.Errorf("色號 [0]=%d [4]=%d，要 10 與 0", px[0], px[4])
	}
}

// TestWriteMode3 釘住 write mode 3：資料當 bit mask，顏色來自 Set/Reset。
// 源平合戰的 OPEN.EXE 用的就是這一種（`013` §2）。
func TestWriteMode3(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 0, 0x09) // Set/Reset：plane 0 與 3
	gcReg(m, 5, 0x03) // write mode 3
	m.Write8(0xA0000, 0x0F)

	for p, want := range []uint8{0x0F, 0x00, 0x00, 0x0F} {
		if got := planeAt(m, p, 0); got != want {
			t.Errorf("plane%d ＝ %02X，要 %02X", p, got, want)
		}
	}
	px := m.Indexed()
	if px[0] != 0 || px[4] != 9 {
		t.Errorf("色號 [0]=%d [4]=%d，要 0 與 9", px[0], px[4])
	}
}

// TestWriteMode2 釘住 write mode 2：資料的低 4 bit 直接當四個 plane 的值。
func TestWriteMode2(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xF0) // 只寫高半個位元組
	gcReg(m, 5, 0x02)
	m.Write8(0xA0000, 0x05) // plane 0 與 2

	for p, want := range []uint8{0xF0, 0x00, 0xF0, 0x00} {
		if got := planeAt(m, p, 0); got != want {
			t.Errorf("plane%d ＝ %02X，要 %02X", p, got, want)
		}
	}
}

// TestWriteMode1CopiesLatch 釘住 write mode 1 ＋ 讀取載 latch。
//
// **讀一次 A0000 會載四個 latch，那是副作用不是最佳化餘地**——
// 整段複製（讀來源、寫目的）就是靠它，漏掉的話複製出來是空白。
func TestWriteMode1CopiesLatch(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	gcReg(m, 0, 0x00)
	gcReg(m, 1, 0x0F) // enable set/reset：四個 plane 都用 Set/Reset
	gcReg(m, 0, 0x06) // 色號 6
	m.Write8(0xA0000, 0xFF)

	gcReg(m, 1, 0x00)
	m.Read8(0xA0000)  // 載 latch
	gcReg(m, 5, 0x01) // write mode 1
	m.Write8(0xA0100, 0x00)

	for p := 0; p < 4; p++ {
		if got, want := planeAt(m, p, 0x100), planeAt(m, p, 0); got != want {
			t.Errorf("plane%d 複製後 ＝ %02X，來源是 %02X", p, got, want)
		}
	}
	px := m.Indexed()
	if px[0] != 6 || px[0x100*8] != 6 {
		t.Errorf("色號 %d／%d，兩邊都要 6", px[0], px[0x100*8])
	}
}

// TestReadModes 釘住 read mode 0（讀選定的 plane）與 read mode 1（color compare）。
func TestReadModes(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	gcReg(m, 1, 0x0F)
	gcReg(m, 0, 0x09) // 色號 9：plane 0 與 3
	m.Write8(0xA0000, 0xFF)
	gcReg(m, 1, 0x00)

	gcReg(m, 4, 0x00) // read map select ＝ plane 0
	if got := m.Read8(0xA0000); got != 0xFF {
		t.Errorf("read mode 0 讀 plane0 ＝ %02X，要 FF", got)
	}
	gcReg(m, 4, 0x01)
	if got := m.Read8(0xA0000); got != 0x00 {
		t.Errorf("read mode 0 讀 plane1 ＝ %02X，要 00", got)
	}

	gcReg(m, 5, 0x08) // read mode 1
	gcReg(m, 7, 0x0F) // 四個 plane 都參與比較
	gcReg(m, 2, 0x09)
	if got := m.Read8(0xA0000); got != 0xFF {
		t.Errorf("color compare 9 ＝ %02X，要 FF（八個像素都是 9）", got)
	}
	gcReg(m, 2, 0x01)
	if got := m.Read8(0xA0000); got != 0x00 {
		t.Errorf("color compare 1 ＝ %02X，要 00", got)
	}
}

// TestBitMaskAndALU 釘住 bit mask 與 ALU 都是**對 latch** 做的。
func TestBitMaskAndALU(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01) // 只動 plane 0
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	m.Write8(0xA0000, 0xF0)

	m.Read8(0xA0000)  // latch ＝ F0
	gcReg(m, 8, 0x3C) // 只改中間四個 bit
	m.Write8(0xA0000, 0xFF)
	if got := planeAt(m, 0, 0); got != 0xFC {
		t.Errorf("bit mask 之後 ＝ %02X，要 FC（F0 的位元只在遮罩內被改）", got)
	}

	m.Read8(0xA0000)
	gcReg(m, 8, 0xFF)
	gcReg(m, 3, 3<<3) // ALU ＝ XOR
	m.Write8(0xA0000, 0xFF)
	if got := planeAt(m, 0, 0); got != 0x03 {
		t.Errorf("XOR 之後 ＝ %02X，要 03（FC ^ FF）", got)
	}
}

// TestDataRotate 釘住 Data Rotate（`GC[03]` 低 3 bit 是右旋位數）。
func TestDataRotate(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	gcReg(m, 3, 0x02) // 右旋 2
	m.Write8(0xA0000, 0x03)
	if got := planeAt(m, 0, 0); got != 0xC0 {
		t.Errorf("右旋 2 之後 ＝ %02X，要 C0", got)
	}
}

// TestSetModeResetsRegisters 釘住「設模式會清畫面，而且把 Map Mask 與
// Bit Mask 設成全開」。
//
// 兩個遮罩留在零值的話，程式設完模式直接畫——寫進去什麼都沒發生，
// 畫面全黑，看起來像「程式還沒畫到」。
func TestSetModeResetsRegisters(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	m.Write8(0xA0000, 0xFF)
	m.SetVideoMode(0x12)
	if got := planeAt(m, 0, 0); got != 0 {
		t.Errorf("設模式之後 plane0 ＝ %02X，要 00", got)
	}
	// 不碰任何暫存器，直接畫。
	m.Write8(0xA0000, 0xFF)
	for p := 0; p < 4; p++ {
		if got := planeAt(m, p, 0); got != 0xFF {
			t.Fatalf("設完模式直接畫，plane%d ＝ %02X，要 FF", p, got)
		}
	}
}

// TestMode13StaysLinear 釘住「mode 13h 不走 planar」——回歸用。
func TestMode13StaysLinear(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	if m.planarOn {
		t.Fatal("mode 13h 進了 planar")
	}
	m.Write8(0xA0000+7, 0x2A)
	if got := m.Read8(0xA0000 + 7); got != 0x2A {
		t.Errorf("mode 13h 讀回 %02X，要 2A", got)
	}
	if got := m.Mem[0xA0000+7]; got != 0x2A {
		t.Errorf("mode 13h 的寫入沒進線性記憶體（%02X）", got)
	}
	if px := m.Indexed(); len(px) != VideoWidth*VideoHigh {
		t.Errorf("mode 13h 的 Indexed 長度 %d，要 %d", len(px), VideoWidth*VideoHigh)
	}
	if w, h := m.VideoSize(); w != 320 || h != 200 {
		t.Errorf("mode 13h 的尺寸 %d×%d", w, h)
	}
}

// TestPlanarScreenSize 釘住各 planar 模式的尺寸與 Indexed 長度。
func TestPlanarScreenSize(t *testing.T) {
	for _, c := range []struct {
		mode uint8
		w, h int
	}{{0x0D, 320, 200}, {0x0E, 640, 200}, {0x10, 640, 350}, {0x12, 640, 480}} {
		m := New()
		m.SetVideoMode(c.mode)
		w, h := m.VideoSize()
		if w != c.w || h != c.h {
			t.Errorf("mode %02Xh 尺寸 %d×%d，要 %d×%d", c.mode, w, h, c.w, c.h)
		}
		if px := m.Indexed(); len(px) != c.w*c.h {
			t.Errorf("mode %02Xh 的 Indexed 長度 %d，要 %d", c.mode, len(px), c.w*c.h)
		}
	}
}

// TestSnapshotKeepsPlanes 釘住快照要帶四個 plane——它們不在 Mem[] 裡。
func TestSnapshotKeepsPlanes(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	m.Write8(0xA0000, 0x5A)
	snap := m.Snapshot()

	m.Write8(0xA0000, 0xFF)
	m.Restore(snap)
	if got := planeAt(m, 0, 0); got != 0x5A {
		t.Errorf("還原之後 plane0 ＝ %02X，要 5A", got)
	}
}
