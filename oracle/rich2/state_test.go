package rich2_test

import (
	"testing"

	"github.com/wicanr2/dosgolem/oracle/rich2"
)

// TestReadPlayerMoney 釘住 BASIC 陣列的讀法。
//
// 兩件事一起釘：描述子的前兩個 word 是 `(位移, 段)`，以及**列主序**。
//
// ⚠ **兩者搞錯都不會報錯**，只會讀到別的玩家的別的欄位，
// 而值看起來完全合理（都是四位數的錢）。判準是「開局現金與存款相等」
// ——那在行主序之下會變成「相鄰兩格相等」，機率低但不是零；
// 加上「五、六號槽是 0」與「一號是 25000」才夠。
func TestReadPlayerMoney(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= rich2.MaxPlayers; i++ {
		t.Logf("  玩家 %d：現金 %6d　存款 %6d",
			i, rich2.Cash(o, i), rich2.Deposit(o, i))
	}

	// 一號玩家是畫面上的那個，開局 25000（`rich2/docs/re/013` §4）。
	if got := rich2.Cash(o, 1); got != 25000 {
		t.Errorf("玩家 1 的現金是 %d，預期 25000", got)
	}
	if got := rich2.Deposit(o, 1); got != 25000 {
		t.Errorf("玩家 1 的存款是 %d，預期 25000", got)
	}

	// 新局四個玩家，五、六號槽沒人。
	active := rich2.ActivePlayers(o)
	if len(active) != 4 {
		t.Errorf("有 %v 個槽有錢，預期 4 個（新局四個玩家）", active)
	}
	for _, p := range []int{5, 6} {
		if rich2.Cash(o, p) != 0 {
			t.Errorf("玩家 %d 的槽應該是空的，現金卻是 %d", p, rich2.Cash(o, p))
		}
	}

	// 每個在場玩家的現金與存款開局相等——這正是列主序的判準來源。
	for _, p := range active {
		if c, d := rich2.Cash(o, p), rich2.Deposit(o, p); c != d {
			t.Errorf("玩家 %d 開局現金 %d ≠ 存款 %d——索引順序可能算錯了", p, c, d)
		}
	}
}

// TestArrayBoundsAreSane 釘住陣列的界。
//
// **越界讀不會報錯**，只會讀到鄰居的資料。四個陣列的大小要對得上
// `rich2/docs/re/014` §2 的 DIM 表。
func TestArrayBoundsAreSane(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		arr  interface{ Size() int }
		want int
	}{
		{"11A2h 玩家金錢", rich2.Money(o), 1440},
		{"1146h 玩家狀態", rich2.PlayerState(o), 360},
		{"1174h 土地表", rich2.Land(o), 900},
		{"122Ch 棋盤", rich2.Board(o), 11320},
	} {
		if got := c.arr.Size(); got != c.want {
			t.Errorf("%s 的大小是 %d bytes，DIM 表寫 %d", c.name, got, c.want)
		}
	}

	m := rich2.Money(o)
	if m.InRange(0, 0) {
		t.Error("玩家 0 應該在界外（第一維是 1..6）")
	}
	if !m.InRange(6, 59) {
		t.Error("玩家 6、欄 59 應該在界內")
	}
	if m.InRange(7, 0) {
		t.Error("玩家 7 應該在界外")
	}
}
