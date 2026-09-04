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
	f, t, _, e := RollDice(o, opts...)
	return f, t, e
}

// RollDice 同 Roll，但多回傳這一次擲出的步數。
//
// ⚠ **步數要在棋子開始走的那一刻讀，不能等到回合結束。**
// `ds:1B0h` 是全域的——回合一推進，AI 擲的骰子就把它蓋掉了。
// 實測：在 `Answer` 之後讀，六步裡有四步的點數對不上實際走的距離。
func RollDice(o *oracle.Oracle, opts ...RollOpt) (from, to, dice int, err error) {
	f, t, d, _, e := RollPath(o, opts...)
	return f, t, d, e
}

// RollPath 同 RollDice，但多回傳**逐格路徑**。
//
// 路徑是走路期間對玩家位置取樣來的（條件函式每道指令都會被呼叫）。
// 比從方向序列反推可靠——反推要先假設「哪些格會抽方向」，而那正是要驗的。
func RollPath(o *oracle.Oracle, opts ...RollOpt) (from, to, dice int, path []int, err error) {
	cfg := rollCfg{budget: 150_000_000, idle: 20_000_000}
	for _, f := range opts {
		f(&cfg)
	}
	player := Turn(o)
	from = Position(o, player)
	if err = o.Click(BtnMoveX, BtnY); err != nil {
		return from, from, 0, path, fmt.Errorf("點「前進」：%w", err)
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
		return from, Position(o, player), Steps(o), path,
			fmt.Errorf("擲骰之後等棋子動：%w", err)
	}
	// **就是這裡。** 棋子剛開始走，點數已經定了，而回合還沒推進。
	dice = Steps(o)

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
	// 記錄逐格路徑——條件函式每道指令都會被呼叫，順手把位置變化收下來。
	// **這是「原版到底怎麼走」的直接答案**，比從方向序列反推可靠。
	lastPathPos := from
	watched := oracle.NewCond(stopped.String(), func(o *oracle.Oracle) bool {
		if p := Position(o, player); p != lastPathPos && p != 0 {
			path = append(path, p)
			lastPathPos = p
		}
		return stopped.Ready(o)
	})
	if err = o.RunUntil(watched, oracle.Budget(cfg.budget)); err != nil {
		return from, Position(o, player), dice, path, fmt.Errorf("等棋子停：%w", err)
	}
	return from, Position(o, player), dice, path, nil
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
	Dirs     []int // 這一步抽到的方向序列（1..4，含被拒絕的重抽）
	Path     []int // 逐格路徑（不含起點）
	OwnerTo  int   // 終點格走之前的地主（0 ＝ 無主）
	StreetTo int   // 終點格的街道編號（0 ＝ 不是土地）
	Levels   []int // 走之前，同街同主的每一格建物等級（算租金要用）
	DirFrom  int   // 走之前的目前方向（ds:10DEh）
	Cash     int32 // 走完之後的現金
	Paid     int32 // 這一步花掉的錢（負數表示收入）
	RND      int   // 這一步消耗的亂數次數
	Dialog   bool  // 有沒有跳對話框（回合沒自己推進）
}



// ClearDialog 送一個 Enter，清掉擋在前面的對話框。
//
// ⚠ **這是恢復手段，不是常規流程。** 亂送 Enter 會在不該確認的地方確認。
// 只在「點了前進卻等不到棋子動」的時候用——那表示遊戲在等別的輸入
// （卡片、事件、或上一步沒回答完的框），而不是在等擲骰。
func ClearDialog(o *oracle.Oracle) error {
	o.Type("\r")
	return o.Run(40_000_000)
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
	r.DirFrom = PlayerDir(o, player)
	rndBefore := 0
	if tr != nil {
		rndBefore = len(tr.Calls)
	}

	from, to, dice, path, err := RollPath(o)
	if err != nil && from == to {
		// 點了前進卻等不到棋子動：多半是有對話框擋著（卡片、事件、
		// 或上一步沒回答完的框）。清掉再試一次。
		if e := ClearDialog(o); e == nil {
			from, to, dice, path, err = RollPath(o)
		}
	}
	r.From, r.To, r.Dice, r.Path = from, to, dice, path
	r.PosFrom = from
	// ⚠ **方向序列要在這裡收窄。**
	//
	// `Answer` 之後回合就推進了，AI 走路時也會抽方向——把整段都算進來
	// 的話序列裡混著別人的。實測第一步因此拿到 [1 2 3]，
	// 而原版那一格選的是 4（根本不在序列裡）。
	if tr != nil {
		r.Dirs = DirectionPicks(o, tr.Calls[rndBefore:])
	}
	if err != nil {
		return r, err
	}

	ownerBefore := Owner(o, to)
	streetBefore, levelsBefore := StreetLevels(o, to)

	// 回合沒推進 ＝ 遊戲在等回答。
	if Turn(o) == player {
		r.Dialog = true
		if err := Answer(o, buy, 0); err != nil {
			return r, err
		}
	}
	r.Cash = Cash(o, player)
	r.Paid = cashBefore - r.Cash
	// **地主要在結算之前讀。** 買下來之後那一欄就變成自己了。
	// 這裡讀的是走完之後、回答對話框之前的狀態——買地的情況下仍是 0。
	r.OwnerTo = ownerBefore
	r.StreetTo, r.Levels = streetBefore, levelsBefore
	r.PosTo = Position(o, player)
	r.RowTo, r.ColTo = MapCoord(o, player)
	if tr != nil {
		r.RND = len(tr.Calls) - rndBefore
	}
	return r, nil
}
