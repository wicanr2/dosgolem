package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EMS 的釘死測試（`docs/spec/008` §7、`docs/spec/014`）。
//
// ⚠ **進入點是 `int F6h` 不是 `int 67h`。** `int 67h` 的向量指向 EMSSeg 的
// 驅動 header（那裡才有 `EMMXXXX0` 簽章），header 開頭的 `CD F6` 才轉進
// 服務層。直接發 `int 67h` 會被 handle 的向量檢查擋掉——**那是對的**，
// 因為那正是「程式自己裝了處理常式」的判準。

func emsAH(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX] >> 8) }

// emsCallAH 發一次 EMS 服務，回 AH（0 ＝ 成功）。
func emsCallAH(m *machine.Machine, d *DOS, ax uint16) uint8 {
	m.CPU.R[cpu.AX] = ax
	d.handle(m.CPU, 0xF6)
	return uint8(m.CPU.R[cpu.AX] >> 8)
}

// TestEMSSignature 釘住偵測路徑：int 67h 向量指的段，+0Ah 是 EMMXXXX0。
//
// 少了它程式判定沒有 EMS——**然後什麼都不說**，只是不去讀那些放在 EMS 裡
// 的資料檔（源平合戰的 OPEN.EXE 就是這樣停在開場 logo）。
func TestEMSSignature(t *testing.T) {
	m := machine.New()
	seg := m.Read16(0x67*4 + 2)
	sig := make([]byte, 8)
	for i := range sig {
		sig[i] = m.Read8(uint32(seg)*16 + 0x0A + uint32(i))
	}
	got := string(sig)
	if got != "EMMXXXX0" {
		t.Errorf("int 67h 向量段 %04X 的 +0Ah 是 %q，要 EMMXXXX0", seg, got)
	}
	// 簽章前面要真的是 trampoline，不是別的東西被寫到這裡。
	if a := uint32(seg) * 16; m.Read8(a) != 0xCD || m.Read8(a+1) != 0xF6 {
		t.Errorf("驅動 header 開頭 ＝ %02X %02X，要 CD F6（int F6h）",
			m.Read8(a), m.Read8(a+1))
	}
}

// TestEMSFrameSegmentAndAlloc 釘住 41h／42h／43h 的四路。
//
// 三個錯誤碼分得開才有診斷價值：87h ＝ 機器根本沒那麼多頁、
// 88h ＝ 有但現在沒空、0 頁 ＝ 87h（見 emsAlloc 的說明，與手冊不同）。
func TestEMSFrameSegmentAndAlloc(t *testing.T) {
	m, d := newTest(t)

	emsCallAH(m, d, 0x4100)
	if m.CPU.R[cpu.BX] != machine.EMSFrameSeg {
		t.Errorf("AH=41h 頁框段 ＝ %04X，預期 %04X",
			m.CPU.R[cpu.BX], uint16(machine.EMSFrameSeg))
	}
	if emsAH(m.CPU) != 0 {
		t.Error("AH=41h 要回 AH=0")
	}

	m.CPU.R[cpu.BX] = 3 // 配 3 頁
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	if emsAH(m.CPU) != 0 || h == 0 {
		t.Fatalf("配 3 頁要成功且有 handle，AH=%02X DX=%d", emsAH(m.CPU), h)
	}

	// 只剩 emsTotalPages-3 頁，多要一頁 → 88h（有那麼多，但沒空）。
	m.CPU.R[cpu.BX] = uint16(emsTotalPages - 3 + 1)
	emsCallAH(m, d, 0x4300)
	if got := emsAH(m.CPU); got != 0x88 {
		t.Errorf("超額（機器有但沒空）要回 AH=88h，得到 %02X", got)
	}

	// 比機器總量還多 → 87h。
	m.CPU.R[cpu.BX] = 0xFFFF
	emsCallAH(m, d, 0x4300)
	if got := emsAH(m.CPU); got != 0x87 {
		t.Errorf("超過機器總量要回 AH=87h，得到 %02X", got)
	}

	m.CPU.R[cpu.BX] = 0 // 0 頁 → 87h
	emsCallAH(m, d, 0x4300)
	if got := emsAH(m.CPU); got != 0x87 {
		t.Errorf("要 0 頁要回 AH=87h，得到 %02X", got)
	}

	// 可用頁數要反映在 42h。
	emsCallAH(m, d, 0x4200)
	if m.CPU.R[cpu.BX] != emsTotalPages-3 || m.CPU.R[cpu.DX] != emsTotalPages {
		t.Errorf("42h 要回 BX=%d DX=%d，得到 BX=%d DX=%d",
			emsTotalPages-3, emsTotalPages, m.CPU.R[cpu.BX], m.CPU.R[cpu.DX])
	}
}

// TestEMSVersionAndMapFlush 釘住 46h 與複製式分頁的核心：
// 映射後讀寫頁框 ＝ 讀寫那一頁；換映射時舊頁**要寫回**，
// 漏寫回的症狀是「存進 EMS 的資料換頁回來變 0」——資料安靜消失。
func TestEMSVersionAndMapFlush(t *testing.T) {
	m, d := newTest(t)

	if ah := emsCallAH(m, d, 0x4000); ah != 0 {
		t.Fatalf("AH=40h 狀態 %02X", ah)
	}
	if ah := emsCallAH(m, d, 0x4600); ah != 0 || uint8(m.CPU.R[cpu.AX]) != 0x40 {
		t.Errorf("版本回 AX=%04X，要 AL=40h", m.CPU.R[cpu.AX])
	}

	m.CPU.R[cpu.BX] = 2
	emsCallAH(m, d, 0x4300) // 配 2 頁
	h := m.CPU.R[cpu.DX]

	base := uint32(machine.EMSFrameSeg) * 16

	// 映射邏輯頁 1 到實體頁 0，頭尾各寫一個 pattern。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 1
	if ah := emsCallAH(m, d, 0x4400); ah != 0 { // AL=0（實體頁 0）
		t.Fatalf("映射邏輯頁 1 失敗 %02X", ah)
	}
	m.Write8(base, 0x5A)
	m.Write8(base+emsPageSize-1, 0xA5)

	// 換映射到邏輯頁 0：頁框要變回 0（新頁），舊內容存進邏輯頁 1。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400)
	if got := m.Read8(base); got != 0 {
		t.Errorf("換映射後頁框要是新頁（0），得到 %02X——寫回／換入有一邊漏了", got)
	}
	m.Write8(base, 0xBB)

	// 映射回邏輯頁 1：剛才寫的要還在。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 1
	emsCallAH(m, d, 0x4400)
	if got := m.Read8(base); got != 0x5A {
		t.Errorf("映射回邏輯頁 1，開頭 ＝ %02X，預期 5A（寫回漏了）", got)
	}
	if got := m.Read8(base + emsPageSize - 1); got != 0xA5 {
		t.Errorf("映射回邏輯頁 1，結尾 ＝ %02X，預期 A5", got)
	}

	// 再換回邏輯頁 0：BB 也要還在。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400)
	if got := m.Read8(base); got != 0xBB {
		t.Errorf("換回邏輯頁 0 讀到 %02X，要 BB——換頁時沒有寫回", got)
	}

	// 錯誤路徑：無效 handle／邏輯頁超出／實體頁超出。
	m.CPU.R[cpu.DX] = 99
	if got := emsCallAH(m, d, 0x4400); got != 0x83 {
		t.Errorf("無效 handle 要回 83h，得到 %02X", got)
	}
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 5
	if got := emsCallAH(m, d, 0x4400); got != 0x8A {
		t.Errorf("邏輯頁超出要回 8Ah，得到 %02X", got)
	}
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	if got := emsCallAH(m, d, 0x4400+uint16(emsPhysPages)); got != 0x8B {
		t.Errorf("實體頁超出要回 8Bh，得到 %02X", got)
	}
}

// TestEMSUnmapAndSaveRestore 釘住 44h 的 BX=FFFFh（解除映射）與 47h/48h。
func TestEMSUnmapAndSaveRestore(t *testing.T) {
	m, d := newTest(t)
	base := uint32(machine.EMSFrameSeg) * 16

	m.CPU.R[cpu.BX] = 2
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]

	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400)
	m.Write8(base, 0x11)

	if ah := emsCallAH(m, d, 0x4700); ah != 0 { // 存下映射
		t.Fatalf("47h 失敗 %02X", ah)
	}

	// 解除映射之後換上邏輯頁 1。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0xFFFF
	if ah := emsCallAH(m, d, 0x4400); ah != 0 {
		t.Fatalf("解除映射失敗 %02X", ah)
	}
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 1
	emsCallAH(m, d, 0x4400)
	m.Write8(base, 0x22)

	// 還原映射：實體頁 0 要變回邏輯頁 0，內容是 11h。
	if ah := emsCallAH(m, d, 0x4800); ah != 0 {
		t.Fatalf("48h 失敗 %02X", ah)
	}
	if got := m.Read8(base); got != 0x11 {
		t.Errorf("還原映射後讀到 %02X，要 11h", got)
	}

	// 而且邏輯頁 1 的 22h 沒被吃掉。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 1
	emsCallAH(m, d, 0x4400)
	if got := m.Read8(base); got != 0x22 {
		t.Errorf("邏輯頁 1 讀到 %02X，要 22h——48h 之前沒有寫回", got)
	}
}

// TestEMSHandleQueries 釘住 4Bh／4Ch／58h。
func TestEMSHandleQueries(t *testing.T) {
	m, d := newTest(t)

	if ah := emsCallAH(m, d, 0x4B00); ah != 0 || m.CPU.R[cpu.BX] != 0 {
		t.Errorf("一開始 handle 數要 0，得到 AH=%02X BX=%d", ah, m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.BX] = 3
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	if ah := emsCallAH(m, d, 0x4B00); ah != 0 || m.CPU.R[cpu.BX] != 1 {
		t.Errorf("配過一次之後 handle 數要 1，得到 AH=%02X BX=%d", ah, m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.DX] = h
	if ah := emsCallAH(m, d, 0x4C00); ah != 0 || m.CPU.R[cpu.BX] != 3 {
		t.Errorf("4Ch 要回 BX=3，得到 AH=%02X BX=%d", ah, m.CPU.R[cpu.BX])
	}

	// 58h AL=00：把可映射位址陣列寫到 ES:DI。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI] = 0x5000, 0
	if ah := emsCallAH(m, d, 0x5800); ah != 0 || m.CPU.R[cpu.CX] != emsPhysPages {
		t.Fatalf("58h 回 AH=%02X CX=%d，要 CX=%d", ah, m.CPU.R[cpu.CX], emsPhysPages)
	}
	for p := 0; p < emsPhysPages; p++ {
		seg := m.Read16(0x50000 + uint32(p)*4)
		idx := m.Read16(0x50000 + uint32(p)*4 + 2)
		want := uint16(machine.EMSFrameSeg) + uint16(p)*(emsPageSize/16)
		if seg != want || idx != uint16(p) {
			t.Errorf("58h 第 %d 項 ＝ (%04X,%d)，要 (%04X,%d)", p, seg, idx, want, p)
		}
	}
}

// TestEMSErrors 釘住錯誤碼走 AH，不是 CF。
func TestEMSErrors(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 0x9999
	if ah := emsCallAH(m, d, 0x4C00); ah != 0x83 {
		t.Errorf("無效 handle 回 AH=%02X，要 83h", ah)
	}
	m.CPU.R[cpu.BX] = 0xFFFF
	if ah := emsCallAH(m, d, 0x4300); ah != 0x87 {
		t.Errorf("要 65535 頁回 AH=%02X，要 87h", ah)
	}
	if ah := emsCallAH(m, d, 0x5A00); ah != 0x84 {
		t.Errorf("沒實作的功能回 AH=%02X，要 84h", ah)
	}
	// CF 不是 EMS 的回傳管道——錯誤時也不該被動到。
	if m.CPU.Flags&cpu.CF != 0 {
		t.Error("EMS 的錯誤不該設 CF")
	}
}

// TestEMSRelease 釘住 45h：頁池回升、handle 作廢、佔著的實體頁寫回並解除。
func TestEMSRelease(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]

	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400) // 佔住實體頁 0
	m.Write8(uint32(machine.EMSFrameSeg)*16, 0x77)

	m.CPU.R[cpu.DX] = h
	if ah := emsCallAH(m, d, 0x4500); ah != 0 { // 釋放
		t.Fatalf("釋放失敗 %02X", ah)
	}
	emsCallAH(m, d, 0x4200)
	if m.CPU.R[cpu.BX] != emsTotalPages || m.CPU.R[cpu.DX] != emsTotalPages {
		t.Errorf("釋放後可用頁 ＝ %d，預期 %d", m.CPU.R[cpu.BX], emsTotalPages)
	}
	m.CPU.R[cpu.DX] = h
	if got := emsCallAH(m, d, 0x4500); got != 0x83 { // 再釋放同一個
		t.Errorf("重複釋放要回 83h，得到 %02X", got)
	}
}

// TestEMSPagesSeesUnmapped 釘住 EMSPages：**沒有映射進頁框的頁也要看得到**。
//
// 遊戲把字型與圖庫整批塞進 EMS，任何時候只有 4 頁在 1 MB 空間裡。
// 只掃主記憶體的話，「找不到」與「不存在」分不開。
func TestEMSPagesSeesUnmapped(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 3
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]

	// 邏輯頁 2 寫一個記號之後換掉映射——它就不在 1 MB 空間裡了。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 2
	emsCallAH(m, d, 0x4400)
	m.Write8(uint32(machine.EMSFrameSeg)*16+7, 0x3C)
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400)

	pages := d.EMSPages()
	if len(pages) != 3 {
		t.Fatalf("EMSPages 回 %d 頁，要 3", len(pages))
	}
	if pages[2].Handle != h || pages[2].Page != 2 {
		t.Fatalf("第三筆是 handle=%d page=%d，要 handle=%d page=2",
			pages[2].Handle, pages[2].Page, h)
	}
	if got := pages[2].Data[7]; got != 0x3C {
		t.Errorf("換走的那一頁讀到 %02X，要 3C——EMSPages 看不到沒映射的頁", got)
	}
}

// TestEMSFlushAllBeforeRead 釘住「**還映著**的那一頁要先 flush 才讀得到現況」。
func TestEMSFlushAllBeforeRead(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 1
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 0
	emsCallAH(m, d, 0x4400)
	m.Write8(uint32(machine.EMSFrameSeg)*16, 0x9E)

	if got := d.EMSPages()[0].Data[0]; got != 0 {
		t.Errorf("沒 flush 就讀到 %02X，這個測試的前提（延遲寫回）已經不成立", got)
	}
	d.EMSFlushAll()
	if got := d.EMSPages()[0].Data[0]; got != 0x9E {
		t.Errorf("EMSFlushAll 之後讀到 %02X，要 9E", got)
	}
}

// TestFileAttr 釘住 int 21h AH=43h AL=00 的兩路（`docs/spec/007` §4）。
func TestFileAttr(t *testing.T) {
	m, d := newTest(t)
	// 找得到的檔（EMMXXXX0 裝置也算「存在」）。
	m.WriteBytes(0x50000, append([]byte("EMMXXXX0"), 0))
	m.CPU.Seg[cpu.DS] = 0x5000
	m.CPU.R[cpu.DX] = 0
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.CX] != 0x20 {
		t.Errorf("存在的檔要 CF=0、CX=20h，得到 CF=%v CX=%04X",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.CX])
	}
	// 找不到。
	m.WriteBytes(0x50000, append([]byte("nope.dat"), 0))
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Errorf("找不到要 CF=1、AX=2，得到 CF=%v AX=%04X",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}
