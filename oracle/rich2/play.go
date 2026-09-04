package rich2

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 在棋盤上操作：走頂端按鈕列、擲骰、回答對話框。

// 頂端按鈕列的座標（`rich2/tools/dosbox_query.py` 的實測值，
// 見 `rich2/docs/spec/063` §111）。
const (
	BtnMoveX  = 125 // 前進
	BtnQueryX = 167 // 查詢
	BtnY      = 9
)

// WaitReady 等遊戲真的開始讀滑鼠。
//
// ⚠ **「畫面畫完」不等於「準備好收輸入」。** 進棋盤之後遊戲還在跑收尾，
// 那段期間一次都不輪詢滑鼠——這時點下去等於沒點，而按鈕**還是會反白**
// （反白是遊戲自己畫的），所以看起來像「點到了但遊戲不理」。
//
// `ToBoard` 已經等過一次；換頁之後要再等。
func WaitReady(o *oracle.Oracle) error {
	return o.RunUntil(oracle.MousePolled(50), oracle.Budget(400_000_000))
}

// RollOpt 調整一次擲骰。
type RollOpt func(*rollCfg)

type rollCfg struct{ budget, idle uint64 }

// RollBudget 改等待上限，RollIdle 改「格號幾道指令沒變才算走完」。
func RollBudget(n uint64) RollOpt { return func(c *rollCfg) { c.budget = n } }
func RollIdle(n uint64) RollOpt   { return func(c *rollCfg) { c.idle = n } }

// Roll 點「前進」擲骰，等棋子走完，回傳**這個玩家自己的**起點與終點格號。
//
// ⚠ 內部用 `ds:1BE`（目前正在處理的格號）判斷「棋子有沒有動」——那是對的，
// 它一定會變；但**回傳值用 Position**，因為 `ds:1BE` 在回合推進之後
// 指的是別人。
//
// 兩段等待都是必要的：
//
//  1. **等格號改變** ── 骰子在轉的時候格號還沒動。
//  2. **等格號穩定** ── 棋子在走的路上格號一直跳，這時送鍵會被走路那段吃掉。
//
// 拿指令數猜「走完了沒」在這裡特別容易錯：步數不同、路徑不同，
// 走完的時間差好幾倍。
func Roll(o *oracle.Oracle, opts ...RollOpt) (from, to int, err error) {
	cfg := rollCfg{budget: 150_000_000, idle: 20_000_000}
	for _, f := range opts {
		f(&cfg)
	}
	player := Turn(o)
	from = Position(o, player)
	if err = o.Click(BtnMoveX, BtnY); err != nil {
		return from, from, fmt.Errorf("點「前進」：%w", err)
	}
	// ⚠ **等「玩家自己的位置」變，不要等 `ds:1BE`。**
	//
	// `ds:1BE` 在擲骰動畫期間就會動（動畫本身在改它），這時棋子還沒走。
	// 拿它當「動了」的判準會提早回傳，收據上就出現「5 → 5」這種
	// 走了一步卻沒動的紀錄——而亂數確實消耗了幾十次，看起來一切正常。
	moved := oracle.NewCond("玩家位置改變", func(o *oracle.Oracle) bool {
		return Position(o, player) != from || Turn(o) != player
	})
	if err = o.RunUntil(moved, oracle.Budget(cfg.budget)); err != nil {
		return from, Position(o, player), fmt.Errorf("擲骰之後等棋子動：%w", err)
	}

	// ⚠ **`ds:1BE` 是「目前玩家」的格號，不是自己的。**
	//
	// 回合一推進，AI 的棋子開始走，這個變數就跟著跳——「等格號穩定」
	// 於是永遠等不到（實測跑滿一億五千萬道指令仍未達成）。
	// 所以第二個出口是「回合推進了」：那表示這一步已經結算完、
	// 沒有對話框要回答。
	// 玩家位置存成座標，所以「停下來」要看那兩個 word 都不動
	// （`docs/re/184` §3）。**兩個各要一份 WordIdle**——
	// 有狀態的條件不能共用實例。
	ps := PlayerState(o)
	rowAddr := oracle.Phys(ps.Base + uint32((ColRow*6+player-1)*2))
	colAddr := oracle.Phys(ps.Base + uint32((ColCol*6+player-1)*2))
	rIdle := oracle.WordIdle(rowAddr, cfg.idle)
	cIdle := oracle.WordIdle(colAddr, cfg.idle)
	stopped := oracle.NewCond("棋子停下來或回合推進",
		func(o *oracle.Oracle) bool {
			if Turn(o) != player {
				return true
			}
			return rIdle.Ready(o) && cIdle.Ready(o)
		})
	if err = o.RunUntil(stopped, oracle.Budget(cfg.budget)); err != nil {
		return from, Position(o, player), fmt.Errorf("等棋子停：%w", err)
	}
	return from, Position(o, player), nil
}

// Answer 回答 Yes／No 對話框（買地那種）。
//
// 原版的選擇器只認 Enter／ESC／上下鍵（`rich2/docs/re/100`）：
// **Enter 是選目前項，ESC 是取消**。預設停在 Yes。
//
// ⚠ **要先等棋子停**（`Roll` 已經等過）。走路那一段送鍵會被吃掉。
func Answer(o *oracle.Oracle, yes bool, settle uint64) error {
	if settle == 0 {
		settle = 60_000_000
	}
	if yes {
		o.Type("\r")
	} else {
		o.Type("\x1b")
	}
	return o.Run(settle)
}

// ClickButton 點頂端按鈕列的某一顆。
func ClickButton(o *oracle.Oracle, x int) error {
	return o.Click(x, BtnY)
}

// WaitTurn 等輪到某個玩家。
//
// 原版是輪流制，人類是玩家 1，其餘是 AI——AI 的回合會自己跑完，
// 所以要走下一步得先等回到自己。
func WaitTurn(o *oracle.Oracle, player int, budget uint64) error {
	if budget == 0 {
		budget = 600_000_000
	}
	return o.RunUntil(oracle.NewCond(
		fmt.Sprintf("輪到玩家 %d", player),
		func(o *oracle.Oracle) bool { return Turn(o) == player }),
		oracle.Budget(budget))
}

// Turn 結果：走一步之後的觀察。
type TurnResult struct {
	Player   int
	From, To int // 這個玩家自己的格號（座標查 11FEh），走之前與走之後
	PosFrom  int // 同 From，留著讓呼叫端讀起來清楚
	PosTo    int // 走之後的格號
	RowTo    int // 走之後的地圖座標
	ColTo    int
	Dice     int   // 這一步擲出的步數（兩顆骰子的和，ds:1B0h）
	Cash     int32 // 走完之後的現金
	Paid     int32 // 這一步花掉的錢（負數表示收入）
	RND      int   // 這一步消耗的亂數次數
	Dialog   bool  // 有沒有跳對話框（回合沒自己推進）
}



// PlayTurn 走一步：等輪到 player、擲骰、必要時回答對話框。
//
// **對話框不是每一步都有**（只有落在無主地、機會格那些才跳）。
// 判準是「回合有沒有自己推進」——沒推進就表示遊戲在等回答。
// 拿「畫面上有沒有框」當判準要做影像比對，而且框的樣式不只一種。
func PlayTurn(o *oracle.Oracle, player int, buy bool, tr *RNDTrace) (TurnResult, error) {
	r := TurnResult{Player: player}
	if err := WaitTurn(o, player, 0); err != nil {
		return r, err
	}
	if err := WaitReady(o); err != nil {
		return r, err
	}
	cashBefore := Cash(o, player)
	rndBefore := 0
	if tr != nil {
		rndBefore = len(tr.Calls)
	}

	from, to, err := Roll(o)
	r.From, r.To = from, to
	r.PosFrom = from
	if err != nil {
		return r, err
	}

	// 回合沒推進 ＝ 遊戲在等回答。
	if Turn(o) == player {
		r.Dialog = true
		if err := Answer(o, buy, 0); err != nil {
			return r, err
		}
	}
	r.Dice = Steps(o)
	r.Cash = Cash(o, player)
	r.Paid = cashBefore - r.Cash
	r.PosTo = Position(o, player)
	r.RowTo, r.ColTo = MapCoord(o, player)
	if tr != nil {
		r.RND = len(tr.Calls) - rndBefore
	}
	return r, nil
}
