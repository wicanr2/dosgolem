package machine

import "testing"

func oplOut(m *Machine, reg, val uint8) {
	m.Out8(0x388, reg)
	m.Out8(0x389, val)
}

// TestOPLDetectionSequence 走一次標準的 AdLib 偵測序列。
//
// 這是 `SetAdLib(true)` 唯一真正要通過的東西（`rich2/docs/re/011` §4）：
// 先遮罩兩個計時器並重置 IRQ、讀到 0；啟動計時器 1、延遲、讀到 0xC0。
// **兩個定值都過不了**——回 0 的話第二次失敗，回 0xC0 的話第一次失敗。
func TestOPLDetectionSequence(t *testing.T) {
	m := New()
	m.SetAdLib(true)

	oplOut(m, 0x04, 0x60) // 遮罩兩個計時器
	oplOut(m, 0x04, 0x80) // 重置 IRQ
	if got := m.In8(0x388); got != 0x00 {
		t.Fatalf("第一次讀狀態 %02X，偵測序列要 00", got)
	}
	oplOut(m, 0x02, 0xFF) // 計時器 1 預設值
	oplOut(m, 0x04, 0x21) // 啟動計時器 1（順便遮罩計時器 2）
	if got := m.In8(0x388); got != 0xC0 {
		t.Fatalf("啟動計時器之後讀狀態 %02X，偵測序列要 C0", got)
	}
	oplOut(m, 0x04, 0x60)
	oplOut(m, 0x04, 0x80)
	if got := m.In8(0x388); got != 0x00 {
		t.Fatalf("重置之後讀狀態 %02X，要 00", got)
	}
}

// TestOPLAdLibAbsentAlwaysZero 沒有 AdLib 時狀態永遠是 0。
func TestOPLAdLibAbsentAlwaysZero(t *testing.T) {
	m := New()
	oplOut(m, 0x04, 0x21)
	if got := m.In8(0x388); got != 0x00 {
		t.Fatalf("沒有 AdLib 卻讀到 %02X", got)
	}
}

// TestOPLTimer2AndMasking 計時器 2 與遮罩各自成立。
func TestOPLTimer2AndMasking(t *testing.T) {
	m := New()
	m.SetAdLib(true)

	oplOut(m, 0x04, 0x02) // 只啟動計時器 2，兩個都不遮
	if got := m.In8(0x388); got != 0xA0 {
		t.Fatalf("只有計時器 2 時狀態 %02X，要 A0（bit7｜bit5）", got)
	}
	oplOut(m, 0x04, 0x22) // 遮罩計時器 2
	if got := m.In8(0x388); got != 0x00 {
		t.Fatalf("遮罩計時器 2 之後狀態 %02X，要 00", got)
	}
	oplOut(m, 0x04, 0x03) // 兩個都啟動、都不遮
	if got := m.In8(0x388); got != 0xE0 {
		t.Fatalf("兩個計時器都跑時狀態 %02X，要 E0", got)
	}
}

// TestOPLIRQResetIgnoresOtherBits `04h` 的 bit7 一設，其他位元就不看。
//
// 寫成「先套遮罩再看 bit7」的話，`04h←80h` 會順便把遮罩清成 0——
// 而那個錯誤只會在「重置之後又啟動計時器」的序列上顯現，
// 偵測序列本身照樣過。
func TestOPLIRQResetIgnoresOtherBits(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	oplOut(m, 0x04, 0x60) // 兩個都遮
	oplOut(m, 0x04, 0x80) // 重置：不應該動到遮罩
	oplOut(m, 0x04, 0x01|0x60)
	if got := m.In8(0x388); got != 0x00 {
		t.Fatalf("遮罩應該還在，狀態卻是 %02X", got)
	}
}

// TestOPLRegsAndClear 暫存器檔記得住值，ClearOPL 只清序列。
func TestOPLRegsAndClear(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	oplOut(m, 0x20, 0x31)
	oplOut(m, 0xBD, 0x20)
	if got := m.OPLRegs(0)[0x20]; got != 0x31 {
		t.Fatalf("暫存器 20h ＝ %02X，要 31", got)
	}
	if n := len(m.OPL); n != 2 {
		t.Fatalf("寫入序列 %d 筆，要 2", n)
	}
	m.ClearOPL()
	if n := len(m.OPL); n != 0 {
		t.Fatalf("ClearOPL 之後序列還有 %d 筆", n)
	}
	if got := m.OPLRegs(0)[0xBD]; got != 0x20 {
		t.Fatalf("ClearOPL 動到了暫存器檔：BDh ＝ %02X，要 20", got)
	}
}

// TestOPLSecondBankStatusPortUnchanged 0x38A **不是**狀態埠。
//
// 這台機器是共用的：把 0x38A 也接成狀態埠會讓「以前讀到 0xFF 的程式」
// 改讀到 0x00。OPL3 的偵測序列讀的是基底埠 0x388，沒有序列讀 0x38A。
func TestOPLSecondBankStatusPortUnchanged(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	oplOut(m, 0x04, 0x21)
	if got := m.In8(0x388); got != 0xC0 {
		t.Fatalf("0x388 回 %02X，要 C0", got)
	}
	if got := m.In8(0x38A); got != 0xFF {
		t.Fatalf("0x38A 回 %02X，要維持預設的 FF", got)
	}
}

// TestOPLSecondBank OPL3 的第二組走 0x38A/0x38B，與第一組分開記。
func TestOPLSecondBank(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	oplOut(m, 0x20, 0x11)
	m.Out8(0x38A, 0x20)
	m.Out8(0x38B, 0x22)
	if got := m.OPLRegs(0)[0x20]; got != 0x11 {
		t.Fatalf("第一組的 20h 被第二組蓋掉了：%02X", got)
	}
	if got := m.OPLRegs(1)[0x20]; got != 0x22 {
		t.Fatalf("第二組的 20h ＝ %02X，要 22", got)
	}
	if n := len(m.OPL); n != 2 || m.OPL[0].Bank != 0 || m.OPL[1].Bank != 1 {
		t.Fatalf("序列的 bank 標記不對：%+v", m.OPL)
	}
}
