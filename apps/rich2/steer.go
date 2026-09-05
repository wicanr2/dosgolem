package rich2

import "github.com/wicanr2/dosgolem/oracle"

// 把棋子開到指定的格子。
//
// 對拍規則層的瓶頸不是讀不到資料，是**走不到那一格**：十一種落地格
// 走十九步只踩到十種，而每一步要 40–60 秒。靠擲骰碰運氣不是辦法。
//
// 出路是攔住移動迴圈的準備段：
//
//	119A0  call far 2F04:166E   ; 擲骰／動畫，回來就是下一行
//	119A5  mov ax, ds:1B0h      ; 步數（兩顆骰子的和）
//	119A8  mov ds:1B6h, ax      ; ★ 複製進迴圈計數
//	119AB  mov ax, 1            ; 迴圈索引從 1 起算
//	119AE  jmp 11C80            ; 進迴圈
//
// 在 `119A5` 覆寫 `ds:1B0h`，下一道指令就把我們的值搬進迴圈計數。
// **一次移動只經過這裡一次**，所以不必擔心中途被改回去。
//
// ⚠ **這一段 IDA 沒認成程式碼**（落在一大塊 `db` 裡），所以
// `ida_dump.idc` 印出來的位址是錯的——它給的是 `119C0`，差了 0x1B，
// 掛上去一次都攔不到。正確的位址是在檔案裡搜 byte pattern
// `A1 B0 01 A3 B6 01 B8 01 00 E9` 找出來的（全檔唯一），再用
// `tools/dis16.sh` 確認 `119A0` 的 far call 剛好結束在這裡。
//
// 所以 `StepOverride` 自己會驗——攔到的時候先確認 `ds:1B0h` 等於剛擲出來
// 的點數，對不上就表示位址錯了，而不是安靜地不生效。
const IDAMoveSetup = 0x119A5

// StepOverride 覆寫接下來每一次移動的步數。
//
// ⚠ **畫面上的骰子仍然是原版真的擲出來的那一組**——被換掉的只有
// 「走幾格」。要做畫面對拍就不能同時開這個。
type StepOverride struct {
	n    int
	Seen []int // 每一次攔截時，原版原本要走的步數
}

// ForceSteps 掛上覆寫。**要在 Run／Click 之前叫。** 回傳的物件用 Set 控制：
// 設 0 就是不干預，讓原版照自己的點數走。
func ForceSteps(o *oracle.Oracle) *StepOverride {
	s := &StepOverride{}
	o.OnCall(o.IDA(IDAMoveSetup), func(o *oracle.Oracle) {
		s.Seen = append(s.Seen, Steps(o))
		if s.n > 0 {
			o.WriteU16(o.DS(VarSteps), uint16(s.n))
		}
	})
	return s
}

// Set 設定接下來要走幾步；0 表示不干預。
func (s *StepOverride) Set(n int) { s.n = n }

// Reached 回報攔到過幾次，用來驗「這個位址真的是每次移動經過一次」。
func (s *StepOverride) Reached() int { return len(s.Seen) }
