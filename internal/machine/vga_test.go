package machine

import "testing"

// EGA／VGA 平面模式（`docs/spec/009`／`013`）的釘死測試。
//
// 這些行為錯了不會有任何錯誤訊息——畫面照樣有東西，只是內容是別的。
// 釘的全是「畫面安靜地錯」的點：map mask、位元遮罩、latch、write mode、
// set/reset、旋轉、讀取模式。每一條的反面都不會報錯，只會讓畫面缺一個
// 平面、或 XOR 變成蓋寫。

func planar(t *testing.T) *Machine {
	t.Helper()
	m := New()
	m.SetVideoMode(0x12)
	if !m.planarOn {
		t.Fatal("mode 12h 沒有進 planar")
	}
	return m
}

// seqReg／gcReg 走真正的埠，不直接動內部欄位——**測試要走程式走的路**，
// 直接塞欄位會讓「索引／資料相位」這類 bug 測不出來。
func seqReg(m *Machine, idx, v uint8) { m.Out8(0x3C4, idx); m.Out8(0x3C5, v) }
func gcReg(m *Machine, idx, v uint8)  { m.Out8(0x3CE, idx); m.Out8(0x3CF, v) }

// planeAt 讀某個平面的位元組（繞過 read mode，測試專用）。
func planeAt(m *Machine, p int, off uint32) uint8 { return m.VGA.Planes[p][off] }

// setPlane 直接擺一個位元組進平面（模擬「畫面上已經有東西」）。
func setPlane(m *Machine, p int, off uint32, v uint8) { m.VGA.Planes[p][off] = v }

// TestWriteMode0MapMask 釘住「Map Mask 選哪幾個平面」。
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

// TestPlanarMapMaskSelectsPlanes：map mask 沒生效的話，每個平面都拿到
// 同一份資料——顏色永遠是 15（四個平面全有），看起來像「畫面太白」。
func TestPlanarMapMaskSelectsPlanes(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01) // 只開 plane 0
	m.Write8(0xA0000, 0xFF)
	if planeAt(m, 0, 0) != 0xFF || planeAt(m, 1, 0) != 0 ||
		planeAt(m, 2, 0) != 0 || planeAt(m, 3, 0) != 0 {
		t.Fatalf("map mask=1 時只有 plane 0 該寫到：%02X %02X %02X %02X",
			planeAt(m, 0, 0), planeAt(m, 1, 0), planeAt(m, 2, 0), planeAt(m, 3, 0))
	}
}

// TestPlanarWritesStayOutOfMem 釘住合併之後的模型：**平面不在 Mem[] 裡**。
//
// 早期有一版讓 planar 寫入同時落進線性 Mem，好讓偵錯工具用 -dump-linear
// 直接挖。那份「影子副本」與平面會在 write mode 1／latch 那條路上分岔，
// 而分岔之後兩份都看起來合理。工具改成讀 VGA.Planes，這裡釘住它不再回來。
func TestPlanarWritesStayOutOfMem(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	m.Write8(0xA0000, 0xFF)
	if got := m.Mem[0xA0000]; got != 0 {
		t.Errorf("planar 的寫入落進 Mem[A0000]（%02X）——影子副本會與平面分岔", got)
	}
	if got := planeAt(m, 0, 0); got != 0xFF {
		t.Errorf("plane0 ＝ %02X，要 FF", got)
	}
}

// TestWriteMode3 釘住 write mode 3：資料當 bit mask，顏色來自 Set/Reset。
// 源平合戰的 OPEN.EXE 用的就是這一種（`docs/spec/013` §2）。
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

// TestPlanarWriteMode3MasksWithCPUByte：write mode 3 的 CPU 位元組是
// **遮罩**，顏色來自 set/reset（`docs/spec/010` §2）。
//
// 當成 mode 0 處理的話字形只會剩零星像素——而且不會報錯，只是字變醜，
// 很容易被當成「字型解碼還沒做完」而往錯的方向查。
func TestPlanarWriteMode3MasksWithCPUByte(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 5, 3)    // write mode 3
	gcReg(m, 0, 0x09) // set/reset：plane 0 與 3 給 1
	gcReg(m, 8, 0xFF) // 位元遮罩全開
	m.Write8(0xA0000, 0xC3)
	want := [4]uint8{0xC3, 0x00, 0x00, 0xC3}
	for p := 0; p < 4; p++ {
		if got := planeAt(m, p, 0); got != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X（CPU byte 是遮罩，顏色來自 gc[0]）",
				p, got, want[p])
		}
	}
}

// TestPlanarWriteMode3AndsBitmaskRegister：有效遮罩是「旋轉後的 CPU
// 位元組 AND 位元遮罩暫存器」，兩者缺一都會多畫或少畫。
func TestPlanarWriteMode3AndsBitmaskRegister(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01)
	gcReg(m, 5, 3)
	gcReg(m, 0, 0x01) // plane 0 給 1
	gcReg(m, 8, 0x0F) // 只准動低 4 位
	m.Write8(0xA0000, 0xFF)
	if got := planeAt(m, 0, 0); got != 0x0F {
		t.Errorf("plane 0 ＝ %02X，預期 0F（gc[8] 沒有被 AND 進去）", got)
	}
}

// TestWriteMode3KeepsALU 釘住「模式 3 不看 Enable Set/Reset，但功能選擇
// （AND／OR／XOR）照常作用」。
//
// 少了這一段，用 XOR 反白一列的程式會把那一列的字**整個蓋掉**，
// 而反白條的顏色也跟著錯——兩個症狀來自同一行，單看任何一個都像別的問題。
func TestWriteMode3KeepsALU(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	m.Write8(0xA0000, 0xF0) // 先鋪一層 F0
	m.Read8(0xA0000)        // 載 latch

	gcReg(m, 5, 0x03) // write mode 3
	gcReg(m, 0, 0x01) // set/reset：plane 0 給 1
	gcReg(m, 3, 3<<3) // ALU ＝ XOR
	m.Write8(0xA0000, 0xFF)
	// 遮罩 ＝ FF，src ＝ FF，latch ＝ F0 → FF^F0 ＝ 0F
	if got := planeAt(m, 0, 0); got != 0x0F {
		t.Errorf("write mode 3 ＋ XOR ＝ %02X，要 0F（ALU 沒有作用）", got)
	}
}

// TestWriteMode2 釘住 write mode 2：資料的低 4 bit 直接當四個平面的值。
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

// TestPlanarWriteMode2ExpandsBits：mode 2 把 CPU 資料的 bit p
// 展開成 plane p 的 0x00/0xFF——實心色塊填色走這條。
func TestPlanarWriteMode2ExpandsBits(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 2)          // write mode 2
	m.Write8(0xA0000, 0x05) // bit0＋bit2
	want := [4]uint8{0xFF, 0x00, 0xFF, 0x00}
	for p := 0; p < 4; p++ {
		if got := planeAt(m, p, 0); got != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X", p, got, want[p])
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
	gcReg(m, 1, 0x0F) // enable set/reset：四個平面都用 Set/Reset
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

// TestPlanarBitmaskMixesLatch：位元遮罩關掉的位元要保留 latch 值
// （＝上一次讀進來的平面內容）。漏掉的話 XOR 游標之類的
// read-modify-write 會把周圍像素清掉。
func TestPlanarBitmaskMixesLatch(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	// 先在四個平面放 0xF0（模擬既有畫面），再裝進 latch。
	for p := 0; p < 4; p++ {
		setPlane(m, p, 0, 0xF0)
	}
	m.Read8(0xA0000)  // 裝 latch
	gcReg(m, 8, 0x0F) // 只寫低 4 位
	m.Write8(0xA0000, 0x05)
	for p := 0; p < 4; p++ {
		if got := planeAt(m, p, 0); got != 0xF5 {
			t.Errorf("plane %d ＝ %02X，預期 F5（高 4 位保留 latch）", p, got)
		}
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

// TestPlanarRotateAndXOR：旋轉與 XOR 一起用（游標繪製慣用）。
func TestPlanarRotateAndXOR(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x01)
	setPlane(m, 0, 0, 0xFF)
	m.Read8(0xA0000)    // latch ＝ FF
	gcReg(m, 3, 1|3<<3) // 右旋 1、XOR
	m.Write8(0xA0000, 0x01)
	// rot(01,1) ＝ 80；80 XOR FF ＝ 7F
	if got := planeAt(m, 0, 0); got != 0x7F {
		t.Errorf("旋轉＋XOR ＝ %02X，預期 7F", got)
	}
}

// TestPlanarMode0EnableSetReset：mode 0 下 gc[1] 選中的平面資料
// 來自 gc[0]，不是 CPU 位元組（`docs/spec/010` §3）。
//
// 沒實作等於把 gc[1] 當永遠是 0：大面積填色會整片填成錯的色號，
// 而每一個像素本身都「有畫到」，所以看起來只是顏色怪，不像 bug。
func TestPlanarMode0EnableSetReset(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 5, 0)
	gcReg(m, 1, 0x03) // plane 0/1 走 set/reset
	gcReg(m, 0, 0x01) // plane 0 給 1、plane 1 給 0
	m.Write8(0xA0000, 0xFF)
	want := [4]uint8{0xFF, 0x00, 0xFF, 0xFF}
	for p := 0; p < 4; p++ {
		if got := planeAt(m, p, 0); got != want[p] {
			t.Errorf("plane %d ＝ %02X，預期 %02X", p, got, want[p])
		}
	}
}

// TestReadModes 釘住 read mode 0（讀選定的平面）與 read mode 1（color compare）。
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
	gcReg(m, 7, 0x0F) // 四個平面都參與比較
	gcReg(m, 2, 0x09)
	if got := m.Read8(0xA0000); got != 0xFF {
		t.Errorf("color compare 9 ＝ %02X，要 FF（八個像素都是 9）", got)
	}
	gcReg(m, 2, 0x01)
	if got := m.Read8(0xA0000); got != 0x00 {
		t.Errorf("color compare 1 ＝ %02X，要 00", got)
	}
}

// TestPlanarReadLoadsLatchAndSelectsPlane：讀取要裝 latch（RMW 的前提）
// 且回 gc[4] 選的平面。回錯平面的話 XOR 繪圖讀到的永遠是 plane 0。
func TestPlanarReadLoadsLatchAndSelectsPlane(t *testing.T) {
	m := planar(t)
	setPlane(m, 2, 7, 0x3C)
	gcReg(m, 4, 2)
	if got := m.Read8(0xA0007); got != 0x3C {
		t.Errorf("讀 plane 2 ＝ %02X，預期 3C", got)
	}
	_, _, latch := m.VGAState()
	for p := 0; p < 4; p++ {
		if latch[p] != planeAt(m, p, 7) {
			t.Errorf("latch[%d] ＝ %02X，預期 %02X（讀取沒裝 latch）",
				p, latch[p], planeAt(m, p, 7))
		}
	}
}

// TestPlanarPixelsDecodes：四個平面各放一個 bit，第 0 像素要出色號 5。
func TestPlanarPixelsDecodes(t *testing.T) {
	m := planar(t)
	setPlane(m, 0, 0, 0x80) // 第 0 像素 bit
	setPlane(m, 2, 0, 0x80)
	px := m.PlanarPixels(640, 480)
	if px[0] != 0x05 {
		t.Errorf("pixel 0 ＝ %02X，預期 05", px[0])
	}
	if px[1] != 0 {
		t.Errorf("pixel 1 ＝ %02X，預期 00", px[1])
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

// TestAttrPaletteFlipFlop 釘住屬性控制器的「先索引再資料」相位，
// 以及讀 3DA 會把它重設。
//
// 相位錯掉的話索引被當成資料、資料被當成索引，整份調色盤錯位——
// 畫面的形狀完全正確，只有顏色是別的。
func TestAttrPaletteFlipFlop(t *testing.T) {
	m := planar(t)
	m.In8(0x3DA)        // 重設相位：下一次寫 3C0 是索引
	m.Out8(0x3C0, 3)    // 索引 3
	m.Out8(0x3C0, 0x2A) // 資料
	if got := m.VGA.Pal(3); got != 0x2A {
		t.Errorf("調色盤暫存器 3 ＝ %02X，要 2A", got)
	}
	// 再讀一次 3DA 之後，下一個位元組又是索引而不是資料。
	m.In8(0x3DA)
	m.Out8(0x3C0, 5)
	m.Out8(0x3C0, 0x11)
	if got := m.VGA.Pal(5); got != 0x11 {
		t.Errorf("調色盤暫存器 5 ＝ %02X，要 11（3DA 沒有重設相位）", got)
	}
	if got := m.VGA.Pal(3); got != 0x2A {
		t.Errorf("調色盤暫存器 3 被蓋成 %02X——相位錯位了", got)
	}
}

// TestDACIndexUsesAttrPalette 釘住色彩鏈「4 位元色號 → 屬性調色盤 → DAC」。
//
// **不要寫死成 identity。** 被觀測的程式剛好把它設成 identity，
// 所以寫死也「會動」——換一支程式才會發現顏色全錯，而且查不到來源。
func TestDACIndexUsesAttrPalette(t *testing.T) {
	m := planar(t)
	if got := m.VGA.DACIndex(7); got != 7 {
		t.Errorf("預設是 identity，色號 7 → %d", got)
	}
	m.VGA.SetPal(7, 0x1E)
	if got := m.VGA.DACIndex(7); got != 0x1E {
		t.Errorf("色號 7 → DAC %02X，要 1E", got)
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
	if got := m.Indexed()[7]; got != 0x2A {
		t.Errorf("mode 13h Indexed()[7] ＝ %02X，預期 2A", got)
	}
	if planeAt(m, 0, 0) != 0 {
		t.Error("mode 13h 不該動到平面")
	}
	if px := m.Indexed(); len(px) != VideoWidth*VideoHigh {
		t.Errorf("mode 13h 的 Indexed 長度 %d，要 %d", len(px), VideoWidth*VideoHigh)
	}
	if w, h := m.VideoSize(); w != 320 || h != 200 {
		t.Errorf("mode 13h 的尺寸 %d×%d", w, h)
	}
}

// TestPlanarScreenSize 釘住各平面模式的尺寸與 Indexed 長度。
func TestPlanarScreenSize(t *testing.T) {
	for _, c := range []struct {
		mode uint8
		w, h int
	}{{0x0D, 320, 200}, {0x0E, 640, 200}, {0x0F, 640, 350},
		{0x10, 640, 350}, {0x11, 640, 480}, {0x12, 640, 480}} {
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

// TestSnapshotKeepsPlanes 釘住快照要帶四個平面——它們不在 Mem[] 裡。
func TestSnapshotKeepsPlanes(t *testing.T) {
	m := planar(t)
	seqReg(m, 2, 0x0F)
	gcReg(m, 8, 0xFF)
	gcReg(m, 5, 0x00)
	m.Write8(0xA0000, 0x5A)
	m.VGA.SetPal(4, 0x31) // 屬性調色盤也要跟著存
	snap := m.Snapshot()

	m.Write8(0xA0000, 0xFF)
	m.VGA.SetPal(4, 0x00)
	m.Restore(snap)
	if got := planeAt(m, 0, 0); got != 0x5A {
		t.Errorf("還原之後 plane0 ＝ %02X，要 5A", got)
	}
	if got := m.VGA.Pal(4); got != 0x31 {
		t.Errorf("還原之後屬性調色盤 4 ＝ %02X，要 31", got)
	}
}

// ---- 以下來自臥龍傳分支 ----------------------------------------------
//
// ⚠ **單元測試綠不代表接對了。** 這一層真正的接線點是 `Read8`／`Write8`
// 的分支，直接呼叫 `VGA.Write()` 的測試把那個分支拿掉照樣全綠——
// 所以其中一支特意走機器層。

// setGC／setSeq 是「先寫索引再寫資料」的縮寫（直接對 VGA，不經機器層）。
func setGC(v *VGA, idx, val uint8)  { v.Out(0x3CE, idx); v.Out(0x3CF, val) }
func setSeq(v *VGA, idx, val uint8) { v.Out(0x3C4, idx); v.Out(0x3C5, val) }

var _ = setSeq

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

// 反白列變成純黃色空白條，同一張圖上捲軸滑塊也不見了）。
func TestWriteMode3AppliesALU(t *testing.T) {
	v := newVGA()
	v.Planes[0][0] = 0xF0 // 底下已經有東西
	setGC(v, 5, 3)
	setGC(v, 0, 0x01)    // Set/Reset ＝ 平面 0
	setGC(v, 3, 0x03<<3) // 功能選擇 ＝ XOR
	setGC(v, 8, 0xFF)

	v.Read(0)
	v.Write(0, 0xFF) // 位元遮罩全開

	if got := v.Planes[0][0]; got != 0x0F {
		t.Errorf("平面 0 ＝ %02X，預期 0F（FF XOR F0）——模式 3 沒套功能選擇？", got)
	}
}

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
