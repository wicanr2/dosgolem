package machine

import "testing"

// EGA／VGA 繪圖控制器的獨立契約測試。
//
// 這一份**不引用任何分支的既有測試**：期望值是照 IBM EGA／VGA 的暫存器
// 定義自己算出來的（write mode、set/reset、位元遮罩、ALU、latch、map mask
// 的作用順序），用來驗合併後的實作，而不是驗某一支分支當初怎麼寫。
//
// 為什麼要獨立算一次：四個平面的模型錯了**不會報錯**，畫面上只是「顏色不對」
// 或「字被蓋掉一塊」——而那兩個症狀看起來都像素材壞了。

// 每個測試自己組一台乾淨的 VGA，暫存器用 Out 走真正的埠路徑。
func newTestVGA() *VGA { return newVGA() }

func (v *VGA) seqOut(idx, val uint8) {
	v.Out(0x3C4, idx)
	v.Out(0x3C5, val)
}

func (v *VGA) gcOut(idx, val uint8) {
	v.Out(0x3CE, idx)
	v.Out(0x3CF, val)
}

// pixelAt 把四個平面的同一個位元組拆成八個色號。
func pixelAt(v *VGA, off uint32) [8]uint8 {
	var px [8]uint8
	for bit := 0; bit < 8; bit++ {
		mask := uint8(0x80 >> bit)
		var c uint8
		for p := 0; p < 4; p++ {
			if v.Planes[p][off]&mask != 0 {
				c |= 1 << p
			}
		}
		px[bit] = c
	}
	return px
}

// 設模式之後的重置值：Map Mask 全開、Bit Mask 全開。
//
// 兩個只要有一個是 0，寫進去就什麼都不會發生——而畫面是全黑，
// 看起來像「程式還沒開始畫」。
func TestVGAResetValuesLetWritesThrough(t *testing.T) {
	v := newTestVGA()
	if got := v.MapMask(); got != 0x0F {
		t.Errorf("Map Mask 重置值 %#x，預期 0F", got)
	}
	gc, _, _ := v.Regs()
	if gc[8] != 0xFF {
		t.Errorf("Bit Mask 重置值 %#x，預期 FF", gc[8])
	}
	if v.WriteMode() != 0 {
		t.Errorf("write mode 重置值 %d，預期 0", v.WriteMode())
	}
}

// write mode 2：**資料位元組的低四位是色號**，每個平面拿到的是
// 「這個平面的位元有沒有被選中」展開成的 FFh／00h，再過位元遮罩。
func TestVGAWriteMode2UsesDataAsColorAndHonorsBitMask(t *testing.T) {
	v := newTestVGA()
	v.gcOut(5, 0x02) // write mode 2
	v.Write(0, 0x05) // 色號 5 ＝ 平面 0 與 2
	for i, c := range pixelAt(v, 0) {
		if c != 0x05 {
			t.Fatalf("第 %d 個像素是 %#x，預期 05", i, c)
		}
	}

	// 位元遮罩只放行高四位；低四位保留 latch（這裡是 0）。
	v2 := newTestVGA()
	v2.gcOut(5, 0x02)
	v2.gcOut(8, 0xF0)
	v2.Write(0, 0x05)
	px := pixelAt(v2, 0)
	for i := 0; i < 4; i++ {
		if px[i] != 0x05 {
			t.Errorf("遮罩放行的第 %d 個像素是 %#x，預期 05", i, px[i])
		}
	}
	for i := 4; i < 8; i++ {
		if px[i] != 0 {
			t.Errorf("遮罩擋住的第 %d 個像素是 %#x，預期 0", i, px[i])
		}
	}
}

// write mode 0：Enable Set/Reset 決定「這個平面用 Set/Reset 的顏色」還是
// 「用 CPU 送出去的位元組」，Data Rotate 先把位元組右旋。
func TestVGAWriteMode0SetResetAndRotate(t *testing.T) {
	v := newTestVGA()
	v.gcOut(1, 0x0F) // 四個平面都用 Set/Reset
	v.gcOut(0, 0x09) // 色號 9 ＝ 平面 0 與 3
	v.Write(0, 0x00) // 資料被忽略
	for i, c := range pixelAt(v, 0) {
		if c != 0x09 {
			t.Fatalf("Set/Reset 的第 %d 個像素是 %#x，預期 09", i, c)
		}
	}

	// 不開 Set/Reset：四個平面都吃同一個（旋轉過的）位元組。
	// 01h 右旋一位是 80h ＝ 只有最左邊那個像素。
	v2 := newTestVGA()
	v2.gcOut(3, 0x01) // 右旋 1
	v2.Write(0, 0x01)
	px := pixelAt(v2, 0)
	if px[0] != 0x0F {
		t.Errorf("旋轉後第 0 個像素是 %#x，預期 0F", px[0])
	}
	for i := 1; i < 8; i++ {
		if px[i] != 0 {
			t.Errorf("第 %d 個像素是 %#x，旋轉後只有位元 7 該亮", i, px[i])
		}
	}
}

// ALU（功能選擇）作用在「新資料」與 **latch** 之間，不是與記憶體現值之間。
// 沒有先讀一次就沒有 latch，XOR 出來的結果會與畫面上的東西無關。
func TestVGAFunctionSelectWorksAgainstLatches(t *testing.T) {
	v := newTestVGA()
	// 先把整個位元組畫成色號 0Fh。
	v.gcOut(5, 0x02)
	v.Write(0, 0x0F)

	// 讀一次載入 latch，再用 XOR 寫色號 0Fh：預期整個位元組被清成 0。
	v.Read(0)
	v.gcOut(3, 0x18) // 功能選擇 ＝ XOR（bit4-3 ＝ 11），旋轉 0
	v.Write(0, 0x0F)
	for i, c := range pixelAt(v, 0) {
		if c != 0 {
			t.Fatalf("XOR 同一個顏色之後第 %d 個像素是 %#x，預期 0", i, c)
		}
	}
}

// write mode 1：latch 原封不動寫回去，**位元遮罩與 ALU 都不參與**。
// 這是「讀一個位址、寫另一個位址」的整段複製快路徑。
func TestVGAWriteMode1CopiesLatchesVerbatim(t *testing.T) {
	v := newTestVGA()
	v.gcOut(5, 0x02)
	v.Write(0x100, 0x0A) // 來源畫成色號 A

	v.Read(0x100)    // 載 latch
	v.gcOut(5, 0x01) // write mode 1
	v.gcOut(8, 0x00) // 位元遮罩全關——mode 1 不該理它
	v.Write(0x200, 0x00)

	for p := 0; p < 4; p++ {
		if got, want := v.Planes[p][0x200], v.Planes[p][0x100]; got != want {
			t.Fatalf("平面 %d 複製出來是 %02X，來源是 %02X", p, got, want)
		}
	}
}

// write mode 3：四個平面一律用 Set/Reset（**不看 Enable Set/Reset**），
// CPU 送出去的位元組旋轉之後與 Bit Mask 相 AND 當作遮罩。
func TestVGAWriteMode3AndsDataIntoTheBitMask(t *testing.T) {
	v := newTestVGA()
	v.gcOut(5, 0x03) // write mode 3
	v.gcOut(0, 0x0C) // Set/Reset ＝ 色號 C
	v.gcOut(1, 0x00) // Enable Set/Reset 全關，mode 3 照樣用 Set/Reset
	v.gcOut(8, 0xF0) // Bit Mask
	v.Write(0, 0xCC) // 實際遮罩 ＝ F0h & CCh ＝ C0h

	px := pixelAt(v, 0)
	for i := 0; i < 2; i++ {
		if px[i] != 0x0C {
			t.Errorf("遮罩放行的第 %d 個像素是 %#x，預期 0C", i, px[i])
		}
	}
	for i := 2; i < 8; i++ {
		if px[i] != 0 {
			t.Errorf("遮罩擋住的第 %d 個像素是 %#x，預期 0", i, px[i])
		}
	}
}

// Map Mask 決定哪幾個平面真的吃這一次寫入，它在最後一關。
func TestVGAMapMaskGatesPlanesLast(t *testing.T) {
	v := newTestVGA()
	v.seqOut(2, 0x02) // 只開平面 1
	v.gcOut(5, 0x02)
	v.Write(0, 0x0F) // 色號 F，但只有平面 1 寫得進去
	for p := 0; p < 4; p++ {
		want := uint8(0x00)
		if p == 1 {
			want = 0xFF
		}
		if got := v.Planes[p][0]; got != want {
			t.Errorf("平面 %d 是 %02X，預期 %02X", p, got, want)
		}
	}
}

// 讀取模式 0 回 GC[04] 選中的那個平面；讀哪一個都會更新 latch。
func TestVGAReadMode0SelectsPlane(t *testing.T) {
	v := newTestVGA()
	v.seqOut(2, 0x04) // 只寫平面 2
	v.gcOut(5, 0x02)
	v.Write(0, 0x0F)

	v.gcOut(4, 0x02) // 讀平面 2
	if got := v.Read(0); got != 0xFF {
		t.Errorf("讀平面 2 得到 %02X，預期 FF", got)
	}
	v.gcOut(4, 0x01) // 讀平面 1（沒寫過）
	if got := v.Read(0); got != 0x00 {
		t.Errorf("讀平面 1 得到 %02X，預期 00", got)
	}
	_, _, latch := v.Regs()
	if latch[2] != 0xFF || latch[1] != 0x00 {
		t.Errorf("latch = %v，四個平面都該在讀取時更新", latch)
	}
}

// 讀取模式 1 是**色彩比較**：回傳的每個位元表示「這個像素等於 GC[02]」，
// 而 GC[07] 標出哪幾個平面參與比較。
func TestVGAReadMode1ComparesColors(t *testing.T) {
	v := newTestVGA()
	v.gcOut(5, 0x02)
	v.gcOut(8, 0xF0)
	v.Write(0, 0x05) // 左四個像素色號 5，右四個 0
	v.gcOut(8, 0xFF)

	v.gcOut(5, 0x0A) // write mode 2 ＋ 讀取模式 1
	v.gcOut(2, 0x05) // 比較色號 5
	v.gcOut(7, 0x0F) // 四個平面都參與
	if got := v.Read(0); got != 0xF0 {
		t.Errorf("色彩比較回 %02X，預期 F0", got)
	}

	// 全部「不在意」時每個像素都算相等。
	v.gcOut(7, 0x00)
	if got := v.Read(0); got != 0xFF {
		t.Errorf("全部不在意時回 %02X，預期 FF", got)
	}
}

// 屬性控制器的 3C0h 是**索引與資料交替**的同一個埠，讀 3DAh 把它扳回索引。
//
// 扳不回去的話下一次寫索引會被當成資料寫進上一個暫存器——調色盤錯一格，
// 畫面顏色整片不對，而暫存器本身看起來都是合法值。
func TestVGAAttributeControllerFlipFlop(t *testing.T) {
	v := newTestVGA()
	v.Out(0x3C0, 0x03) // 索引 3
	v.Out(0x3C0, 0x2A) // 資料
	if got := v.Pal(3); got != 0x2A {
		t.Fatalf("調色盤 3 是 %02X，預期 2A", got)
	}
	// 現在該回到「下一次是索引」。中途重設一次也要能對齊。
	v.Out(0x3C0, 0x05)
	v.ResetACFlip() // 讀 3DAh 的副作用
	v.Out(0x3C0, 0x07)
	v.Out(0x3C0, 0x11)
	if got := v.Pal(7); got != 0x11 {
		t.Errorf("重設 flip-flop 之後調色盤 7 是 %02X，預期 11", got)
	}
	if got := v.Pal(5); got != 5 {
		t.Errorf("被打斷的那一次不該寫進調色盤 5（現在是 %02X）", got)
	}
}
