package machine

import "testing"

// MemPoison 是除錯開關：開機記憶體填非零 pattern，抓「讀到未初始化記憶體」。
//
// 觸發案例：Watcom 6.5 的 chained compile 在全零記憶體下 E142，
// 填 CC 之後變成 E143＋空轉——行為與初值有關（`retro-runtime-study-private#32`
// R49）。沒有這個開關，那種相依只能用猜的。
func TestMemPoisonFillsUntouchedAreas(t *testing.T) {
	old := MemPoison
	MemPoison = 0xCC
	defer func() { MemPoison = old }()
	m := New()
	// 沒人寫過的自由區留著 pattern。
	if got := m.Mem[0x50000]; got != 0xCC {
		t.Errorf("自由區位元組＝%02X，預期 CC（poison 沒填進去）", got)
	}
	// 初始化照常覆蓋自己的區域：向量表不是 CC。
	if m.Mem[0] == 0xCC && m.Mem[1] == 0xCC {
		t.Error("向量表還是 CC——initVectors 沒覆蓋，poison 蓋掉了初始化")
	}
	// 預設（0）行為不變：關掉就是全零。
	MemPoison = 0
	m2 := New()
	if got := m2.Mem[0x50000]; got != 0x00 {
		t.Errorf("預設開機位元組＝%02X，預期 00（poison 預設值改了既有行為）", got)
	}
}
