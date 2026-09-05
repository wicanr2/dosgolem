package rich2

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 股市每日更新的觀測。
//
// 更新的入口是 `0x2C510`，一整輪跑完由 `0x1334D` 的 far call 進去
// （`rich2/docs/re/032` §8.1）。**這一支是「一天」的邊界**——
// 玩家指標繞回第一位時才呼叫一次。
//
// 框住它要兩個攔截點：進入點拿更新前的狀態，返回點拿更新後的狀態。
// 中間所有的 `RND` 都屬於這一次更新，照順序留著就能拿去重播。
const (
	// IDAStockDay 是股市更新副程式的進入點（`0CED:F640`）。
	IDAStockDay = 0x2C510
	// IDAStockDayEnd 是「一輪結束」那條路上，**股市與衝擊都算完之後**
	// 的第一個位址（`0x133C0 mov ds:10AAh, 1`，玩家指標歸 1）。
	//
	// ⚠ **不要拿 far call 的返回位址當出口。** `0x1334D` 那一帶
	// IDA 沒認成程式碼，反組譯出來是錯位的（`in ax, dx` 那種），
	// 照著算出來的返回位址攔不到任何東西——而症狀是
	// 「After 一直沒填」，看起來像攔截機制壞掉。
	// `0x133C0` 是兩條分支的匯合點，有交叉參考佐證對齊。
	//
	// 中間只有衝擊值自己的演化，不碰股票陣列，所以在這裡取的
	// After 與副程式剛返回時相同。
	//
	// ⚠ 第二個呼叫端 `0x192D7`（事件帶來的衝擊，`rich2/docs/re/032` §8.1）
	// 不走這條路，那一種更新的 `Done` 會是 false。刻意的——
	// 事件那一路還沒對上事件表，混進來只會讓樣本不知道是哪一種。
	IDAStockDayEnd = 0x133C0
)

// 更新用到的兩個全域（`rich2/docs/re/032` §8.1、§8.4）。
const (
	// VarDayCount 是累計天數（`ds:1038h`）。全檔只有一處 `inc`，不歸零。
	VarDayCount = 0x1038
	// VarDayOfYear 是年內日序（`ds:103Ah`），副程式一進來就自己算。
	VarDayOfYear = 0x103A
	// VarImpact 是外來衝擊值（`ds:103Ch`）。**更新用的是上一天留下來的值**，
	// 演化在更新之後才發生，所以在進入點讀到的就是這一次用的。
	VarImpact = 0x103C
)

// 衝擊值演化的兩個亂數呼叫端（`0x13353`–`0x133BD`）。
//
//	衝擊 = 四捨六入五成雙(衝擊 + RND × 201 − 100)   ← ImpactCallerStep
//	衝擊夾在 −400 … +600
//	IF RND > 0.9 THEN 衝擊 = −衝擊 ÷ 2              ← ImpactCallerFlip
const (
	ImpactCallerStep = 0x1335C
	ImpactCallerFlip = 0x133A1
)

// StockDay 是一次股市更新的完整觀測：前後狀態、期間抽到的每一個亂數。
//
// 有了這三樣就能離線重播：拿 Before 餵重製版的更新常式，
// 亂數用 Rnd 逐個回放，結果應該逐格等於 After。
// **這比「跑很多局看分布」強得多**——分布只驗得到形狀，逐格重播驗的是算式。
type StockDay struct {
	Step      uint64      // 進入更新的那一道指令
	DayCount  int         // `ds:1038h`，進入時的值
	DayOfYear int         // `ds:103Ah`，副程式算完之後的值（見 Note）
	Impact    int         // `ds:103Ch`，進入時的值
	Before    [][]float32 // 20 × 10，更新前
	After     [][]float32 // 20 × 10，更新後
	Rnd       []float64   // 這一次更新期間抽到的值，照順序
	Callers   []uint32    // 與 Rnd 一一對應的呼叫端（IDA 線性位址）

	// 衝擊值自己的演化（`0x13353`–`0x133BD`）緊接在股市更新後面。
	// 收在這裡是因為只有在這個時間點才觀測得到，
	// 而它決定的是**明天**的股價。
	ImpactRnd   []float64
	ImpactAfter int

	Done bool // 有沒有走到 0x133C0（事件觸發的那一種不會）
}

// StockDayLog 收集整場的股市更新。
type StockDayLog struct {
	Days []StockDay
	cur  *StockDay
}

// WatchStockDays 掛上三個攔截，回傳會自己長大的紀錄。**要在 Run 之前叫。**
//
// 三個攔截分別是：更新的進入點、更新的返回點、以及 `RND`。
// `RND` 那一個只在「兩點之間」記錄，所以不會混進擲骰或抽方向。
func WatchStockDays(o *oracle.Oracle) *StockDayLog {
	log := &StockDayLog{}

	o.OnCall(o.IDA(IDAStockDay), func(o *oracle.Oracle) {
		d := StockDay{
			Step:     o.Steps(),
			DayCount: int(int16(o.Word(o.DS(VarDayCount)))),
			Impact:   int(int16(o.Word(o.DS(VarImpact)))),
			Before:   snapshotStocks(o),
		}
		log.Days = append(log.Days, d)
		log.cur = &log.Days[len(log.Days)-1]
	})

	o.OnCall(o.IDA(IDAStockDayEnd), func(o *oracle.Oracle) {
		if log.cur == nil {
			return
		}
		log.cur.After = snapshotStocks(o)
		log.cur.DayOfYear = int(int16(o.Word(o.DS(VarDayOfYear))))
		log.cur.ImpactAfter = int(int16(o.Word(o.DS(VarImpact))))
		log.cur.Done = true
		log.cur = nil
	})

	o.OnCall(o.IDA(BASIC.RNDEntry), func(o *oracle.Oracle) {
		if log.cur == nil {
			return
		}
		// **只收副程式自己抽的。** 更新返回之後、`0x133C0` 之前還有
		// 衝擊值演化的兩次抽取（`0x1335C`、`0x133A1`），
		// 那兩次屬於下一段流程，混進來會讓重播多兩個值。
		caller := o.ToIDA(o.Caller())
		if caller < StockDayLo || caller >= StockDayHi {
			if caller == ImpactCallerStep || caller == ImpactCallerFlip {
				log.cur.ImpactRnd = append(log.cur.ImpactRnd,
					basic.Call{State: basic.State(o, BASIC)}.Value())
			}
			return
		}
		log.cur.Rnd = append(log.cur.Rnd,
			basic.Call{State: basic.State(o, BASIC)}.Value())
		log.cur.Callers = append(log.cur.Callers, caller)
	})

	return log
}

func snapshotStocks(o *oracle.Oracle) [][]float32 {
	out := make([][]float32, StockCount)
	a := Stocks(o)
	for n := range out {
		out[n] = make([]float32, StockCols)
		for c := range out[n] {
			out[n][c] = a.Float32(n, c)
		}
	}
	return out
}
