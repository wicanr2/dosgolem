package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EMS（LIM EMS 4.0，`int 67h`）的獨立契約測試。
//
// 這一份不引用分支的既有測試：期望值照 LIM 規格自己列（回傳慣例是
// **AH ＝ 0 成功、非 0 是狀態碼**，不是 DOS 的 CF），再加一條「資料真的
// 跟著映射走」的端到端檢查——只驗狀態碼的話，一個「回成功但沒搬資料」
// 的實作會全部通過，而程式讀到的是上一頁的內容。

// emsCallAX 走真正的進入點。
//
// ⚠ **不是 `int 67h`。** 那個向量指向 EMSSeg 的驅動 header（簽章在那裡），
// header 開頭的 `CD F6` 才轉進服務層；直接發 `int 67h` 會被 `handle` 的
// 「向量還指著 stub 嗎」檢查擋下來——而那正是「程式自己裝了處理常式」的判準。
func emsCallAX(m *machine.Machine, d *DOS, ax uint16) uint8 {
	m.CPU.R[cpu.AX] = ax
	d.handle(m.CPU, 0xF6)
	return uint8(m.CPU.R[cpu.AX] >> 8)
}

// 驅動存在的兩個訊號：`int 67h` 的向量指到一段有 `EMMXXXX0` 簽章的
// driver header。程式是**開檔 EMMXXXX0 或直接比對簽章**來判斷有沒有 EMS 的，
// 只實作服務不擺簽章的話它一開始就認定沒有 EMS，之後一次也不會呼叫。
func TestEMSDriverHeaderIsDiscoverable(t *testing.T) {
	m, _ := newTest(t)
	seg := m.Read16(0x67*4 + 2)
	if seg != machine.EMSSeg {
		t.Fatalf("int 67h 的向量段是 %04X，預期 %04X", seg, machine.EMSSeg)
	}
	base := uint32(seg) * 16
	got := make([]byte, 8)
	for i := range got {
		got[i] = m.Read8(base + 0x0A + uint32(i))
	}
	if string(got) != "EMMXXXX0" {
		t.Errorf("driver header 的簽章是 %q，預期 EMMXXXX0", got)
	}
}

// 配置／映射／釋放的一輪，外加「資料跟著映射走」。
//
// page frame 是一扇窗：同一個實體頁換映射之後，窗裡看到的必須是新那一頁的
// 內容，而剛才寫進去的東西要留在舊的邏輯頁裡。**沒有這一條，一個把
// page frame 當普通記憶體的實作會全部測過**——症狀是程式換頁之後拿到
// 上一頁的資料，看起來像它自己算錯了位址。
func TestEMSMappingMovesData(t *testing.T) {
	m, d := newTest(t)

	if ah := emsCallAX(m, d, 0x4100); ah != 0 {
		t.Fatalf("AH=41h 回狀態 %02X", ah)
	}
	if got := m.CPU.R[cpu.BX]; got != machine.EMSFrameSeg {
		t.Fatalf("page frame 段是 %04X，預期 %04X", got, machine.EMSFrameSeg)
	}

	m.CPU.R[cpu.BX] = 2
	if ah := emsCallAX(m, d, 0x4300); ah != 0 {
		t.Fatalf("配 2 頁回狀態 %02X", ah)
	}
	h := m.CPU.R[cpu.DX]

	frame := uint32(machine.EMSFrameSeg) * 16
	// 邏輯頁 0 映到實體頁 0，寫一個記號。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	if ah := emsCallAX(m, d, 0x4400); ah != 0 {
		t.Fatalf("映射邏輯頁 0 回狀態 %02X", ah)
	}
	m.Write8(frame, 0xA1)

	// 換成邏輯頁 1：窗裡不該還看得到 A1h。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	if ah := emsCallAX(m, d, 0x4400); ah != 0 {
		t.Fatalf("映射邏輯頁 1 回狀態 %02X", ah)
	}
	if got := m.Read8(frame); got == 0xA1 {
		t.Fatal("換頁之後窗裡還是上一頁的資料——映射沒有真的換")
	}
	m.Write8(frame, 0xB2)

	// 換回邏輯頁 0：記號要回來。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	if ah := emsCallAX(m, d, 0x4400); ah != 0 {
		t.Fatalf("換回邏輯頁 0 回狀態 %02X", ah)
	}
	if got := m.Read8(frame); got != 0xA1 {
		t.Fatalf("換回來看到 %02X，預期 A1——寫進去的內容沒有存回邏輯頁", got)
	}

	// 釋放之後 handle 就不該再認得。
	m.CPU.R[cpu.DX] = h
	if ah := emsCallAX(m, d, 0x4500); ah != 0 {
		t.Fatalf("釋放回狀態 %02X", ah)
	}
	m.CPU.R[cpu.DX] = h
	if ah := emsCallAX(m, d, 0x4C00); ah != 0x83 {
		t.Errorf("釋放後查頁數回狀態 %02X，預期 83（handle 無效）", ah)
	}
}

// 錯誤碼要分得開：handle 無效（83h）、邏輯頁超範圍（8Ah）、
// 實體頁超範圍（8Bh）、要的比整台機器有的多（87h）、現在不夠（88h）。
//
// 全部回同一個碼也「能跑」，但程式的錯誤處理會走進同一條路——
// 而三種原因的處置完全不同。
func TestEMSStatusCodesAreDistinct(t *testing.T) {
	m, d := newTest(t)

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, 0
	m.CPU.R[cpu.AX] = 0x4400
	if ah := emsCallAX(m, d, 0x4400); ah != 0x83 {
		t.Errorf("映射不存在的 handle 回 %02X，預期 83", ah)
	}

	m.CPU.R[cpu.BX] = 1
	if ah := emsCallAX(m, d, 0x4300); ah != 0 {
		t.Fatalf("配 1 頁回狀態 %02X", ah)
	}
	h := m.CPU.R[cpu.DX]

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 5, h // 只有 1 頁
	if ah := emsCallAX(m, d, 0x4400); ah != 0x8A {
		t.Errorf("邏輯頁超範圍回 %02X，預期 8A", ah)
	}
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	if ah := emsCallAX(m, d, 0x4409); ah != 0x8B { // AL=9 ＝ 第 10 個實體頁
		t.Errorf("實體頁超範圍回 %02X，預期 8B", ah)
	}

	m.CPU.R[cpu.BX] = 0xFFFF
	if ah := emsCallAX(m, d, 0x4300); ah != 0x87 {
		t.Errorf("要 65535 頁回 %02X，預期 87", ah)
	}

	// 版本要回 4.0：EMS 4.0 才有的功能（`AH=58h`）程式會先問版本再用。
	if ah := emsCallAX(m, d, 0x4600); ah != 0 {
		t.Fatalf("AH=46h 回狀態 %02X", ah)
	}
	if got := uint8(m.CPU.R[cpu.AX]); got != 0x40 {
		t.Errorf("版本回 %02X，預期 40（4.0）", got)
	}
}

// 存／回復映射表（47h／48h）：回復之後窗裡要是存檔當時那一頁。
//
// 中斷處理常式借用 page frame 的慣用法就是「進來 47h、出去 48h」；
// 48h 只把表抄回去而不搬資料的話，被打斷的那一段程式繼續讀到的是
// 中斷處理常式那一頁的內容——**而且它完全不會察覺**。
func TestEMSSaveRestorePageMapMovesDataBack(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	if ah := emsCallAX(m, d, 0x4300); ah != 0 {
		t.Fatalf("配頁回狀態 %02X", ah)
	}
	h := m.CPU.R[cpu.DX]
	frame := uint32(machine.EMSFrameSeg) * 16

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x11)
	if ah := emsCallAX(m, d, 0x4700); ah != 0 { // 存
		t.Fatalf("AH=47h 回狀態 %02X", ah)
	}

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x22)

	if ah := emsCallAX(m, d, 0x4800); ah != 0 { // 回復
		t.Fatalf("AH=48h 回狀態 %02X", ah)
	}
	if got := m.Read8(frame); got != 0x11 {
		t.Fatalf("回復之後窗裡是 %02X，預期 11", got)
	}
	// 中斷那一段寫的東西要留在它自己的邏輯頁裡。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	emsCallAX(m, d, 0x4400)
	if got := m.Read8(frame); got != 0x22 {
		t.Errorf("邏輯頁 1 是 %02X，預期 22", got)
	}
}

// `AH=4Eh` 的對映表存／取要**連資料一起回到原狀**。
//
// 只把表抄回去而不搬資料的話，被打斷的那一段程式繼續讀到的是中斷處理常式
// 那一頁的內容——而且它完全不會察覺。
func TestEMSPageMapSaveRestoreRoundTrip(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	if ah := emsCallAX(m, d, 0x4300); ah != 0 {
		t.Fatalf("配頁回狀態 %02X", ah)
	}
	h := m.CPU.R[cpu.DX]
	frame := uint32(machine.EMSFrameSeg) * 16

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x71)

	// 問這份表要多大，再存到 ES:DI。
	if ah := emsCallAX(m, d, 0x4E03); ah != 0 {
		t.Fatalf("AH=4Eh AL=03 回狀態 %02X", ah)
	}
	size := uint8(m.CPU.R[cpu.AX])
	if size == 0 {
		t.Fatal("對映表大小回 0")
	}
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI] = 0x2400, 0
	if ah := emsCallAX(m, d, 0x4E00); ah != 0 {
		t.Fatalf("存對映表回狀態 %02X", ah)
	}

	// 換成邏輯頁 1 再寫別的東西。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x82)

	// 取回對映表：窗裡要回到存檔當時那一頁。
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x2400, 0
	if ah := emsCallAX(m, d, 0x4E01); ah != 0 {
		t.Fatalf("取回對映表回狀態 %02X", ah)
	}
	if got := m.Read8(frame); got != 0x71 {
		t.Fatalf("取回之後窗裡是 %02X，預期 71", got)
	}
	// 中斷那一段寫的東西要留在它自己的邏輯頁裡。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	emsCallAX(m, d, 0x4400)
	if got := m.Read8(frame); got != 0x82 {
		t.Errorf("邏輯頁 1 是 %02X，預期 82", got)
	}
}

// `AH=50h` 一次換好幾頁是**原子的**：中間有一項不合法就整批不動。
//
// 換一半的話程式讀到的是兩份資料拼起來的東西，而那看起來像資料檔壞了。
func TestEMSMapMultipleIsAtomic(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	emsCallAX(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	frame := uint32(machine.EMSFrameSeg) * 16

	// 先把實體頁 0 映到邏輯頁 0 並寫一個記號。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x11)

	// 一批兩項：第一項合法（邏輯 1 → 實體 0），第二項邏輯頁超範圍。
	list := cpu.Addr(0x2500, 0)
	m.Write16(list, 1)   // 邏輯頁 1
	m.Write16(list+2, 0) // 實體頁 0
	m.Write16(list+4, 9) // 邏輯頁 9：超範圍
	m.Write16(list+6, 1)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x2500, 0
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 2, h
	if ah := emsCallAX(m, d, 0x5000); ah != 0x8A {
		t.Fatalf("邏輯頁超範圍回 %02X，預期 8A", ah)
	}
	if got := m.Read8(frame); got != 0x11 {
		t.Errorf("失敗的一批動到了實體頁 0（現在是 %02X，預期 11）", got)
	}

	// 全部合法就要全部生效。
	m.Write16(list+4, 1) // 邏輯頁 1 → 實體頁 1
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x2500, 0
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 2, h
	if ah := emsCallAX(m, d, 0x5000); ah != 0 {
		t.Fatalf("合法的一批回 %02X", ah)
	}
}

// `AH=51h` 縮放：放大要成功，縮小要**保留前面那些頁的內容**。
func TestEMSReallocKeepsExistingPages(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	emsCallAX(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	frame := uint32(machine.EMSFrameSeg) * 16

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAX(m, d, 0x4400)
	m.Write8(frame, 0x33)

	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 6, h
	if ah := emsCallAX(m, d, 0x5100); ah != 0 {
		t.Fatalf("放大回 %02X", ah)
	}
	m.CPU.R[cpu.DX] = h
	if ah := emsCallAX(m, d, 0x4C00); ah != 0 || m.CPU.R[cpu.BX] != 6 {
		t.Fatalf("放大之後頁數是 %d（狀態 %02X），預期 6", m.CPU.R[cpu.BX], ah)
	}

	// 縮回 1 頁：邏輯頁 0 的內容要還在。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	if ah := emsCallAX(m, d, 0x5100); ah != 0 {
		t.Fatalf("縮小回 %02X", ah)
	}
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAX(m, d, 0x4400)
	if got := m.Read8(frame); got != 0x33 {
		t.Errorf("縮小之後邏輯頁 0 是 %02X，預期 33", got)
	}

	// 要得比整台機器有的多要失敗，而且回目前的頁數。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0xFFFF, h
	if ah := emsCallAX(m, d, 0x5100); ah != 0x88 {
		t.Errorf("要 65535 頁回 %02X，預期 88", ah)
	}
}
