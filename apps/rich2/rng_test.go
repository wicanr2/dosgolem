package rich2_test

import (
	"testing"

	"github.com/wicanr2/dosgolem/apps/rich2"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// TestRNDFollowsLCG 釘住原版 BASIC runtime 的亂數。
//
// `rich2/docs/re/050` 用 unicorn 靜態＋動態解出這個公式並標 confirmed。
// 這裡在同一個 Go 行程裡重驗一次，涵蓋從冷啟動到棋盤的**每一次**呼叫。
func TestRNDFollowsLCG(t *testing.T) {
	o := load(t)
	tr := rich2.TraceRND(o)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	if err := tr.Verify(o); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d 次呼叫全部符合 x' = (x × 43FD43FD + C39EC3) mod 2²⁴", len(tr.Calls))
}

// TestRandomizeWithZeroTimerGivesZeroState 釘住 remake 的 seed 對應
// （`rich2/WORKLIST.md` P1.1）。
//
// 原版用 `RANDOMIZE TIMER` 取種子。dosgolem 讓 `int 21h AH=2Ch` 回 CX=DX=0，
// 等同 rich2 的固定種子版（patch_seed.py 把 TIMER 內部換成 xor cx,cx/xor dx,dx）。
// 在那個條件下**初始狀態是 0**——所以 remake 的 `seed = 0` 精確對應
// 原版固定種子版的序列，兩邊可以逐次比對。
func TestRandomizeWithZeroTimerGivesZeroState(t *testing.T) {
	o := load(t)
	tr := rich2.TraceRND(o)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	if len(tr.RandomizeAt) != 1 {
		t.Errorf("RANDOMIZE 被呼叫 %d 次，預期 1 次", len(tr.RandomizeAt))
	}
	if tr.InitialState != 0 {
		t.Errorf("初始狀態是 %06X，預期 000000", tr.InitialState)
	}
	// 第一個抽出來的值因此一定是加數本身。
	if got := tr.Calls[0].Next(); got != basic.LCGAdd {
		t.Errorf("第一次抽到 %06X，預期 %06X（＝ LCG 的加數）", got, basic.LCGAdd)
	}
}

// TestRNDConsumptionMap 記錄誰在消耗亂數。
//
// **remake 要重播原版的序列，就得在同樣的地方消耗同樣多次。**
// 這個測試不釘死次數（那會隨走到哪裡而變），只釘住三件已知的事。
func TestRNDConsumptionMap(t *testing.T) {
	o := load(t)
	tr := rich2.TraceRND(o)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	by := tr.ByCaller(o)
	for k, n := range by {
		t.Logf("  IDA %05X ×%d", k, n)
	}

	// 新局初始化的迴圈：索引 0–49，每一圈抽三次
	// （1B61F → INT(RND×6)、1B649 → INT(RND×260+20)、1B678 → INT(RND×-900-40)）。
	for _, a := range []uint32{0x1B61F, 0x1B649, 0x1B678} {
		if by[a] != 50 {
			t.Errorf("IDA %05X 抽了 %d 次，預期 50（新局初始化的迴圈跑 50 圈）", a, by[a])
		}
	}
	// 防拷出題：1BD51 一次、1BDBB 每題一次。
	if by[0x1BD51] != 1 {
		t.Errorf("IDA 1BD51 抽了 %d 次，預期 1", by[0x1BD51])
	}
	if by[0x1BDBB] != 3 {
		t.Errorf("IDA 1BDBB 抽了 %d 次，預期 3（防拷三題）", by[0x1BDBB])
	}
}
