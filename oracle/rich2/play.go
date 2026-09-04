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

// Roll 點「前進」擲骰，等棋子走完，回傳起點與終點格號。
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
	from = Tile(o)
	if err = o.Click(BtnMoveX, BtnY); err != nil {
		return from, from, fmt.Errorf("點「前進」：%w", err)
	}
	moved := oracle.NewCond("格號改變", func(o *oracle.Oracle) bool {
		return Tile(o) != from
	})
	if err = o.RunUntil(moved, oracle.Budget(cfg.budget)); err != nil {
		return from, Tile(o), fmt.Errorf("擲骰之後等棋子動：%w", err)
	}
	if err = o.RunUntil(oracle.WordIdle(o.DS(VarTile), cfg.idle),
		oracle.Budget(cfg.budget)); err != nil {
		return from, Tile(o), fmt.Errorf("等棋子停：%w", err)
	}
	return from, Tile(o), nil
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
