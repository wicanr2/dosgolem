package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// XMS 3.0 的契約測試。期望值照 XMS 3.0 規格自己列。
//
// ⚠ **XMS 的失敗慣例不是 CF，是 `AX=0` ＋ `BL` ＝ 錯誤碼。** 用 CF 的話
// 呼叫端不會看——它檢查的是 AX，於是失敗被讀成成功，然後拿一個沒配到的
// handle 去搬資料。

// xmsCallAH 走 driver entry 的 trampoline（`int F5h`），回 AX。
func xmsCallAH(m *machine.Machine, d *DOS, ax uint16) uint16 {
	m.CPU.R[cpu.AX] = ax
	d.handle(m.CPU, 0xF5)
	return m.CPU.R[cpu.AX]
}

// 偵測要一路通：`int 2Fh AX=4300h` 回 80h、`AX=4310h` 回 driver entry，
// 而 entry 那一段要是**能執行而且會回來**的碼。
func TestXMSDetectionPathIsComplete(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.AX] = 0x4300
	d.handle(m.CPU, 0x2F)
	if got := uint8(m.CPU.R[cpu.AX]); got != 0x80 {
		t.Fatalf("AX=4300h 回 AL=%02X，預期 80", got)
	}
	m.CPU.R[cpu.AX] = 0x4310
	d.handle(m.CPU, 0x2F)
	seg, off := m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX]
	if b := m.Read8(cpu.Addr(seg, off)); b != 0xCD {
		t.Errorf("driver entry %04X:%04X 的第一個位元組是 %02X，預期 CD（trampoline）",
			seg, off, b)
	}
}

// 版本要回「有 HMA」。DX=0 的話程式連 `AH=01h` 都不會問，
// 直接把資料放進傳統記憶體——而那正是我們想觀測的那條路的反面。
func TestXMSVersionReportsHMA(t *testing.T) {
	m, d := newTest(t)
	if ax := xmsCallAH(m, d, 0x0000); ax>>8 < 2 {
		t.Errorf("版本回 %04X，預期至少 2.00", ax)
	}
	if m.CPU.R[cpu.DX] != 1 {
		t.Errorf("DX=%d，預期 1（有 HMA）", m.CPU.R[cpu.DX])
	}
}

// A20 關著的時候位址在 1 MB 環繞——**程式靠這個偵測 HMA**。
//
// 寫 `0000:0000` 再讀 `FFFF:0010`：值一樣就是沒有 A20。
// 預設就開著的話，沒要求過 HMA 的程式會判定它可用然後把資料搬進去。
func TestA20GateControlsWraparound(t *testing.T) {
	m, d := newTest(t)
	if m.A20Enabled() {
		t.Fatal("A20 預設就開著——真機開機後是關的")
	}
	m.Write8(0, 0x11)
	if got := m.Read8(machine.MemSize + 0x10); got != m.Read8(0x10) {
		t.Fatalf("A20 關著時 1 MB 之上沒有環繞（讀到 %02X）", got)
	}

	// 打開之後那一段是獨立的記憶體。
	if ax := xmsCallAH(m, d, 0x0300); ax != 1 { // Global enable A20
		t.Fatalf("AH=03h 回 %d", ax)
	}
	if !m.A20Enabled() {
		t.Fatal("AH=03h 沒有把 A20 打開")
	}
	m.Write8(machine.MemSize+0x10, 0x99)
	if got := m.Read8(0x10); got == 0x99 {
		t.Error("A20 開著卻還是環繞——寫 HMA 蓋掉了中斷向量表")
	}
	if got := m.Read8(machine.MemSize + 0x10); got != 0x99 {
		t.Errorf("HMA 讀回 %02X，預期 99", got)
	}

	// 查詢要回實際狀態。
	if ax := xmsCallAH(m, d, 0x0700); ax != 1 {
		t.Errorf("AH=07h 回 %d，預期 1（A20 開著）", ax)
	}
}

// Local A20 的開關是**成對計數**的：巢狀開兩次要關兩次才真的關。
//
// 直接關的話，外層那一段程式以為 A20 還開著，接著寫進 HMA 的東西會環繞
// 回低位記憶體——蓋掉的多半是中斷向量表。
func TestLocalA20IsNested(t *testing.T) {
	m, d := newTest(t)
	xmsCallAH(m, d, 0x0500) // local enable
	xmsCallAH(m, d, 0x0500)
	xmsCallAH(m, d, 0x0600) // local disable
	if !m.A20Enabled() {
		t.Fatal("開兩次關一次就關掉了")
	}
	xmsCallAH(m, d, 0x0600)
	if m.A20Enabled() {
		t.Error("開兩次關兩次之後應該關掉")
	}
}

// HMA 一次只有一個擁有者，而且拿到就要能定址。
func TestHMAHasOneOwnerAndEnablesA20(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 0xFFFF
	if ax := xmsCallAH(m, d, 0x0100); ax != 1 {
		t.Fatalf("要 HMA 回 %d（BL=%02X）", ax, uint8(m.CPU.R[cpu.BX]))
	}
	if !m.A20Enabled() {
		t.Error("拿到 HMA 卻沒開 A20——寫進去會環繞回中斷向量表")
	}
	m.CPU.R[cpu.DX] = 0xFFFF
	if ax := xmsCallAH(m, d, 0x0100); ax != 0 {
		t.Error("第二個擁有者也拿到了 HMA")
	} else if bl := uint8(m.CPU.R[cpu.BX]); bl != 0x91 {
		t.Errorf("錯誤碼 %02X，預期 91（HMA 已被佔用）", bl)
	}
	if ax := xmsCallAH(m, d, 0x0200); ax != 1 { // release
		t.Error("釋放 HMA 失敗")
	}
	if ax := xmsCallAH(m, d, 0x0200); ax != 0 {
		t.Error("重複釋放竟然成功")
	}
}

// EMB 的鎖定要回一個**每次都一樣、彼此不重疊**的位址，
// 而且鎖著的區塊不能被釋放或縮放。
func TestEMBLockContract(t *testing.T) {
	m, d := newTest(t)
	alloc := func(kb uint16) uint16 {
		m.CPU.R[cpu.DX] = kb
		if ax := xmsCallAH(m, d, 0x0900); ax != 1 {
			t.Fatalf("配 %d KB 失敗（BL=%02X）", kb, uint8(m.CPU.R[cpu.BX]))
		}
		return m.CPU.R[cpu.DX]
	}
	h1, h2 := alloc(64), alloc(64)

	lock := func(h uint16) uint32 {
		m.CPU.R[cpu.DX] = h
		if ax := xmsCallAH(m, d, 0x0C00); ax != 1 {
			t.Fatalf("鎖 handle %d 失敗", h)
		}
		return uint32(m.CPU.R[cpu.DX])<<16 | uint32(m.CPU.R[cpu.BX])
	}
	a1, a2 := lock(h1), lock(h2)
	if a1 == a2 {
		t.Fatal("兩個 handle 鎖出同一個位址")
	}
	if again := lock(h1); again != a1 {
		t.Errorf("同一個 handle 兩次鎖出不同位址：%08X 與 %08X", a1, again)
	}

	// 鎖著就不能放。
	m.CPU.R[cpu.DX] = h1
	if ax := xmsCallAH(m, d, 0x0A00); ax != 0 {
		t.Error("鎖著的區塊竟然釋放得掉")
	} else if bl := uint8(m.CPU.R[cpu.BX]); bl != 0xAB {
		t.Errorf("錯誤碼 %02X，預期 AB（區塊鎖著）", bl)
	}

	// 解鎖兩次（剛才鎖了兩次）之後才放得掉。
	for i := 0; i < 2; i++ {
		m.CPU.R[cpu.DX] = h1
		if ax := xmsCallAH(m, d, 0x0D00); ax != 1 {
			t.Fatalf("第 %d 次解鎖失敗", i)
		}
	}
	m.CPU.R[cpu.DX] = h1
	if ax := xmsCallAH(m, d, 0x0D00); ax != 0 {
		t.Error("多解鎖一次竟然成功")
	}
	m.CPU.R[cpu.DX] = h1
	if ax := xmsCallAH(m, d, 0x0A00); ax != 1 {
		t.Error("解鎖之後仍然放不掉")
	}
}

// `AH=0Eh` 回鎖定次數與大小；`AH=0Fh` 縮放**要保留內容**。
func TestEMBInfoAndReallocKeepContents(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 4
	xmsCallAH(m, d, 0x0900)
	h := m.CPU.R[cpu.DX]

	// 塞一個記號進去（handle 0 是常規記憶體那一端）。
	m.Write8(0x5000, 0x5A)
	desc := cpu.Addr(0x2000, 0)
	m.Write16(desc, 1) // 長度 1
	m.Write16(desc+2, 0)
	m.Write16(desc+4, 0) // 來源 handle 0
	m.Write16(desc+6, 0x5000&0xFFFF)
	m.Write16(desc+8, 0)
	m.Write16(desc+0x0A, h) // 目的 handle
	m.Write16(desc+0x0C, 0)
	m.Write16(desc+0x0E, 0)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x2000, 0
	if ax := xmsCallAH(m, d, 0x0B00); ax != 1 {
		t.Fatal("搬移失敗")
	}

	m.CPU.R[cpu.DX] = h
	if ax := xmsCallAH(m, d, 0x0E00); ax != 1 {
		t.Fatal("AH=0Eh 失敗")
	}
	if kb := m.CPU.R[cpu.DX]; kb != 4 {
		t.Errorf("大小回 %d KB，預期 4", kb)
	}
	if lockCount := m.CPU.R[cpu.BX] >> 8; lockCount != 0 {
		t.Errorf("鎖定次數回 %d，預期 0", lockCount)
	}

	// 放大到 8 KB，內容要留著。
	m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = h, 8
	if ax := xmsCallAH(m, d, 0x0F00); ax != 1 {
		t.Fatal("縮放失敗")
	}
	if got := d.emb[h][0]; got != 0x5A {
		t.Errorf("縮放之後第一個位元組是 %02X，預期 5A——內容沒留住", got)
	}
	if got := len(d.emb[h]); got != 8*1024 {
		t.Errorf("縮放之後大小是 %d，預期 8192", got)
	}
}

// UMB 配置要回真的段，而且**不能與 EMS 的 page frame 重疊**。
//
// 重疊的話 EMS 換頁會把 UMB 裡的東西換掉，而那看起來像
// 「常駐程式突然壞了」。
func TestUMBAllocationAvoidsThePageFrame(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 0x100 // 要 100h 段（4 KB）
	if ax := xmsCallAH(m, d, 0x1000); ax != 1 {
		t.Fatalf("要 UMB 失敗（BL=%02X，可用 %04X 段）",
			uint8(m.CPU.R[cpu.BX]), m.CPU.R[cpu.DX])
	}
	seg, size := m.CPU.R[cpu.BX], m.CPU.R[cpu.DX]
	if size != 0x100 {
		t.Errorf("配到 %04X 段，要的是 100", size)
	}
	if seg < umbStart || seg+size > machine.EMSFrameSeg {
		t.Errorf("UMB 落在 %04X–%04X，與 EMS page frame（%04X）重疊或超界",
			seg, seg+size, machine.EMSFrameSeg)
	}

	// 要得比整塊還大：回實際可用的量，讓呼叫端照它再要一次。
	m.CPU.R[cpu.DX] = 0xFFFF
	if ax := xmsCallAH(m, d, 0x1000); ax != 0 {
		t.Fatal("要 FFFFh 段竟然成功")
	}
	if avail := m.CPU.R[cpu.DX]; avail == 0 || avail == 0xFFFF {
		t.Errorf("可用量回 %04X，要回實際的段數", avail)
	}
	if bl := uint8(m.CPU.R[cpu.BX]); bl != 0xB0 {
		t.Errorf("錯誤碼 %02X，預期 B0（只有比較小的 UMB）", bl)
	}

	// 放掉不認識的段要報錯。
	m.CPU.R[cpu.DX] = 0x1234
	if ax := xmsCallAH(m, d, 0x1100); ax != 0 {
		t.Error("放掉沒配過的 UMB 竟然成功")
	}
}

// 沒實作的功能要照 XMS 的慣例失敗（AX=0、BL=80h）並記一筆，
// **不能安靜地回成功**。
func TestUnknownXMSFunctionFailsWithCode(t *testing.T) {
	m, d := newTest(t)
	if ax := xmsCallAH(m, d, 0x8800); ax != 0 {
		t.Fatal("沒實作的功能竟然回成功")
	}
	if bl := uint8(m.CPU.R[cpu.BX]); bl != 0x80 {
		t.Errorf("錯誤碼 %02X，預期 80（沒有這個功能）", bl)
	}
	if d.Unimplemented[Call{Int: 0xF5, AH: 0x88}] == 0 {
		t.Error("沒實作的功能沒有記一筆")
	}
}
