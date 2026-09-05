package rich2_test

import (
	"testing"

	"github.com/wicanr2/dosgolem/apps/rich2"
)

// TestRollAndBuy 釘住一整步：擲骰 → 落在無主地 → 買下來。
//
// 這是對拍的最小完整單位。任何一段斷掉，遊戲內的 parity 全部做不了。
func TestRollAndBuy(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	player := rich2.Turn(o)
	cashBefore := rich2.Cash(o, player)

	from, to, err := rich2.Roll(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("玩家 %d 從格號 %d 走到 %d（%d 道指令）", player, from, to, o.Steps())
	if from == to {
		t.Fatal("格號沒變——擲骰沒生效")
	}

	// 落在無主地會跳買地對話框，Enter 買下。
	if err := rich2.Answer(o, true, 0); err != nil {
		t.Fatal(err)
	}
	cashAfter := rich2.Cash(o, player)
	t.Logf("玩家 %d 的現金 %d → %d（差 %d）",
		player, cashBefore, cashAfter, cashBefore-cashAfter)

	if cashAfter >= cashBefore {
		t.Errorf("買了地現金卻沒少（%d → %d）", cashBefore, cashAfter)
	}
	// 回合應該推進到下一個玩家。
	if rich2.Turn(o) == player {
		t.Errorf("買完地還是玩家 %d 的回合", player)
	}
}

// TestDeclinePurchase 釘住 ESC 這一條。
//
// **Yes 通了不代表 No 也通。** 兩條路徑走的是不同的分支，
// 而「沒買成」與「根本沒收到鍵」的畫面表現一樣。
func TestDeclinePurchase(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	player := rich2.Turn(o)
	cashBefore := rich2.Cash(o, player)

	if _, _, err := rich2.Roll(o); err != nil {
		t.Fatal(err)
	}
	if err := rich2.Answer(o, false, 0); err != nil {
		t.Fatal(err)
	}

	cashAfter := rich2.Cash(o, player)
	t.Logf("玩家 %d 的現金 %d → %d", player, cashBefore, cashAfter)
	if cashAfter != cashBefore {
		t.Errorf("拒買卻扣了錢（%d → %d）", cashBefore, cashAfter)
	}
	// 回合仍然要推進——拒買不是「什麼都沒發生」。
	if rich2.Turn(o) == player {
		t.Errorf("拒買之後還是玩家 %d 的回合，ESC 可能沒被收到", player)
	}
}
