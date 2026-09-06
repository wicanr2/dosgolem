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

// 岔路方向的覆寫。
//
// 原版的方向是抽的，不是誰選的（`rich2/docs/spec/006` R11）：
//
//	11A49  fistp ds:1C0h                       ; 候選方向 = INT(RND×4)+1
//	11A50  IF 122Ch[目前格][1C0h+3] == 0 THEN 重抽   ; 那個方向沒有出口
//	11A6E  IF |ds:10DEh − ds:1C0h| == 2 THEN 重抽    ; 回頭
//	11AA6  ds:10DEh = ds:1C0h                  ; 採用
//
// **`0x11A50` 是候選已經寫好、檢查還沒開始的那一刻**，所以在那裡覆寫
// `ds:1C0h` 就等於指定這一次岔路要往哪走。
const (
	// IDADirPicked 是候選方向寫好之後的第一個位址。
	IDADirPicked = 0x11A50
	// VarDirCandidate 是候選方向（1..4）。目前方向是 `state.go` 的
	// `VarDirection`（`ds:10DEh`），這裡不重複宣告。
	VarDirCandidate = 0x01C0
)

// DirOverride 是一次岔路方向的覆寫。
type DirOverride struct {
	pick  func(o *oracle.Oracle, drawn int) int
	Drawn []int // 原版本來抽到的（覆寫前）
	Used  []int // 實際採用的
}

// Pick 掛上「這一次岔路往哪走」的決定函式。
//
// 回傳 1..4 表示覆寫，回傳 0 或超出範圍表示不動（用原版抽到的）。
func (d *DirOverride) Pick(f func(o *oracle.Oracle, drawn int) int) { d.pick = f }

// ForceDirection 掛上岔路方向的覆寫。**要在 Run／Click 之前叫。**
//
// ⚠⚠ **這會改變亂數序列，和 ForceSteps 一樣是導航工具不是 parity 工具。**
// 原版抽到「沒有出口」或「回頭」時會**重抽**，每一次重抽都多消耗一次亂數；
// 覆寫成一個合法方向就把那些重抽跳過了。所以**用了它的那一步不能拿來
// 對拍亂數消耗**，只能拿來「把棋子開到我要的地方」。
//
// 覆寫點是 `0x11A50`——候選方向已經寫進 `ds:1C0h`、出口與回頭檢查
// 都還沒跑的那一刻。寫一個**合法**的方向進去，兩道檢查都會通過。
// 寫一個沒有出口的方向進去，原版會照它自己的規則重抽，等於沒覆寫。
func ForceDirection(o *oracle.Oracle) *DirOverride {
	d := &DirOverride{}
	o.OnCall(o.IDA(IDADirPicked), func(o *oracle.Oracle) {
		drawn := int(int16(o.Word(o.DS(VarDirCandidate))))
		d.Drawn = append(d.Drawn, drawn)
		use := drawn
		if d.pick != nil {
			if v := d.pick(o, drawn); v >= 1 && v <= 4 {
				use = v
				o.WriteU16(o.DS(VarDirCandidate), uint16(v))
			}
		}
		d.Used = append(d.Used, use)
	})
	return d
}

