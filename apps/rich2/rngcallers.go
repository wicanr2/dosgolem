package rich2

import (
	"sort"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 已知的 `RND` 呼叫端。
//
// **這張表是從執行期量出來的**，不是從反組譯猜的：掛上 `TraceRND`、走幾步、
// 把 `Caller` 做直方圖，次數大的自己會浮出來（`rich2/docs/re/185`）。
//
// 位址是**呼叫 `RND` 那一行的返回位址**（`o.Caller()`），所以會落在真正那道
// `call` 的下一道指令上——比對反組譯時要記得減回去。
type RNDCaller struct {
	IDA  uint32
	Name string
	Note string
}

// RNDCallers 是目前解出語意的呼叫端。
//
// ⚠ **沒收進來的不是「不存在」，是「還沒解」。** `SplitCalls` 會把它們印成
// 未知，那正是下一輪要查的東西。
var RNDCallers = []RNDCaller{
	{0x215F9, "擲骰", "落在 docs/re/015 §2 的擲骰常式 0x2156E 裡（+0x8B）。" +
		"一步 38–62 次——擲骰動畫每一幀都真的擲（docs/re/155）"},
	{0x11A32, "抽方向", "選方向迴圈 sub_11A1E（docs/re/014 §4c）"},
	{0x11A87, "抽方向（回頭後重抽）", "同上，|目前方向 − 候選| == 2 時從這裡重抽"},
	{StockCallerStep, "股市：動量步", "每檔必抽。(RND − 0.495) ÷ 20"},
	{StockCallerVolEarly, "股市：利多期成交量", "相位 < 30 才走這一支"},
	{StockCallerNudgeUp, "股市：利多期加碼", "再加一個條件：欄 0 < 欄 7"},
	{StockCallerVolLate, "股市：利空期成交量", "相位 > 335 才走這一支"},
	{StockCallerNudgeDn, "股市：利空期減碼", "再加一個條件：欄 0 > 欄 7"},
	{StockCallerVolMid, "股市：平常期成交量", "多數的檔走這一支"},
	{StockCallerFlip, "股市：動量反轉", "每檔必抽。RND > 0.85 才反轉並減半"},
	{StockCallerFloor, "股市：撞下限推動量", "價格跌破 10 才抽"},
	{StockCallerCeil, "股市：撞上限推動量", "價格超過欄 7 ＋ 欄 6 才抽"},
	// 發卡那一段用區間認（見 CardDrawLo／CardDrawHi），不列單一位址。
	{0x137B6, "新聞 INT(RND×16)+100", "docs/re/022 §6"},
}

// 賭場那一段的範圍（`rich2/docs/re/020` §2：賭場 0x156D8、遊樂場 0x16C96）。
//
// 賭場整段有 9 次 `RND`；實測在一步之內看到六個不同的呼叫端各抽一次，
// 全部落在這個區間裡。**`docs/re/014` §4d 把其中三個歸給 `sub_1695B`，
// 那是 IDA 呼叫圖誤判**（同一份筆記的 §4e 就在講呼叫圖不可信）。
const (
	CasinoLo = 0x156D8
	CasinoHi = 0x16C96
)

// CallerName 回呼叫端的語意名稱；沒解出來的回空字串。
func CallerName(ida uint32) string {
	for _, c := range RNDCallers {
		if c.IDA == ida {
			return c.Name
		}
	}
	if ida >= CardDrawLo && ida < CardDrawHi {
		return "發卡 INT(RND×100)"
	}
	if ida >= CasinoLo && ida < CasinoHi {
		return "賭場（段內，未逐項解）"
	}
	return ""
}

// CallerCount 是一個呼叫端在某一段時間窗裡抽了幾次。
type CallerCount struct {
	IDA  uint32
	N    int
	Name string // 沒解出來的是空字串
}

// SplitCalls 把一段抽取按呼叫端分組，次數多的在前。
//
// **這是對齊亂數消耗次數的工作面**：序列本身是決定性的 LCG，兩邊不會岔在
// 數值上，只會岔在「誰在什麼時候抽了幾次」。要重播一局，remake 得在同樣的
// 地方消耗同樣多次——先知道原版在哪裡消耗，才談得上對齊。
func SplitCalls(o *oracle.Oracle, calls []basic.Call) []CallerCount {
	hist := map[uint32]int{}
	for _, c := range calls {
		hist[o.ToIDA(c.Caller)]++
	}
	out := make([]CallerCount, 0, len(hist))
	for at, n := range hist {
		out = append(out, CallerCount{IDA: at, N: n, Name: CallerName(at)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].IDA < out[j].IDA
	})
	return out
}

// CountFrom 回某個呼叫端在這一段裡抽了幾次。
func CountFrom(o *oracle.Oracle, calls []basic.Call, ida uint32) int {
	n := 0
	for _, c := range calls {
		if o.ToIDA(c.Caller) == ida {
			n++
		}
	}
	return n
}

// InWindow 篩出時間窗 [lo, hi) 之內的抽取。
//
// 配 `TurnResult` 的三個切點用：
//
//	[StepClick, StepMoved)    **這個玩家自己的一回合**（擲骰動畫 ＋ 走路）
//	[StepMoved, StepStopped)  幾乎是空的，見 MoveTrace 的說明
//	[StepStopped, 下一步)      落地結算 ＋ **之後 AI 的回合**
//
// ⚠ **第三段含別人的回合。** `PlayTurn` 等到回合推進才回來，所以那一段裡
// 混著 AI 的擲骰與移動。**要單人的數字只能看第一段。**
//
// 實測（`rich2/docs/re/185` §2a）：第一段裡擲骰固定 26 次、
// 與骰子點數無關，方向的次數則與 `TurnResult.Dirs` 的長度一致。
func InWindow(calls []basic.Call, lo, hi uint64) []basic.Call {
	var out []basic.Call
	for _, c := range calls {
		if c.Step >= lo && c.Step < hi {
			out = append(out, c)
		}
	}
	return out
}

// 發卡那一段的範圍（`rich2/docs/re/048`：`0x13681`–`0x137B5`，
// 尾端接著新聞格的進入點 `0x137B6`）。
//
// ⚠ **用區間不是用單一位址。** `13681` 是那一段的**起點**，不是
// `RND` 的返回位址——第一版拿它去等值比對，`DeckSlots` 回空陣列，
// 而「回空」看起來就像「這一步沒發卡」。
const (
	CardDrawLo = 0x13681
	CardDrawHi = 0x137B6
)

// DeckSlots 從一段抽取裡篩出發卡的那幾次，算成牌堆索引 `INT(RND×100)`。
//
// **這一項不需要對齊整體的消耗次數**（那件事本身不成立，見
// `rich2/docs/re/185` §2b）——只要定位發卡自己那一次抽取就夠了。
// 驗的是「算出來的索引 ＝ 牌堆實際變動的那一格」。
func DeckSlots(o *oracle.Oracle, calls []basic.Call) []int {
	var out []int
	for _, c := range calls {
		if at := o.ToIDA(c.Caller); at < CardDrawLo || at >= CardDrawHi {
			continue
		}
		out = append(out, int(uint64(c.Next())*uint64(DeckSize)/basic.LCGMod))
	}
	return out
}
