package rich2

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// BASIC runtime 的 `RND`（`rich2/docs/re/050`，IDA 線性位址）。
//
// 那份筆記的輸入檔是 `RUN_unpacked.EXE`，但這幾個位址都在 `0x3AE58` 以下，
// 所以在 `RUN_full.EXE` 上不變（`rich2/CLAUDE.md` §4.1 的遷移規則）；
// DGROUP 的偏移本來就不受換檔影響。
const (
	RNDEntry  = 0x2E3AA // RND 包裝（1CBE:17CA）
	Randomize = 0x2E423 // RANDOMIZE（1CBE:1843）

	rndStateLo = 0x22D1 // 狀態低 16 位
	rndStateHi = 0x22D3 // 狀態高 8 位
)

// LCG 的三個常數。**讀原版的 DGROUP 驗過**（`ds:279A`／`ds:279E`／`ds:27A2`）。
const (
	LCGMul = 0x43FD43FD
	LCGAdd = 0x00C39EC3
	LCGMod = 1 << 24
)

// LCGNext 推進一步。
//
//	xₙ₊₁ = (xₙ × 43FD43FD + C39EC3) mod 2²⁴
//	RND  = xₙ₊₁ / 2²⁴
func LCGNext(x uint32) uint32 {
	return uint32((uint64(x)*LCGMul + LCGAdd) % LCGMod)
}

// RNDState 讀目前的 24 位元狀態。
func RNDState(o *oracle.Oracle) uint32 {
	return uint32(o.Word(o.DS(rndStateLo))) | uint32(o.Byte(o.DS(rndStateHi)))<<16
}

// RNDCall 是一次 `RND` 呼叫。
type RNDCall struct {
	Step   uint64      // 第幾道指令
	State  uint32      // **進入 RND 之前**的狀態
	Caller oracle.Addr // 呼叫端的返回位址
}

// Next 是這一次呼叫產生的新狀態。
func (c RNDCall) Next() uint32 { return LCGNext(c.State) }

// Value 是這一次回傳的 RND 值。
func (c RNDCall) Value() float64 { return float64(c.Next()) / LCGMod }

// RNDTrace 記錄一段執行期間的每一次亂數。
//
// **remake 要重播原版的序列，就得在同樣的地方消耗同樣多次。**
// 序列本身是決定性的（LCG），會岔開的永遠是「誰在什麼時候抽了幾次」
// ——例如擲骰動畫的每一幀都真的擲（`rich2/docs/re/155`）。
type RNDTrace struct {
	Calls        []RNDCall
	RandomizeAt  []uint64 // RANDOMIZE 被呼叫的時間點
	InitialState uint32   // 第一次 RND 之前的狀態
	initSeen     bool
}

// TraceRND 掛上攔截。**要在 Run／RunUntil 之前叫。**
func TraceRND(o *oracle.Oracle) *RNDTrace {
	t := &RNDTrace{}
	o.OnCall(o.IDA(RNDEntry), func(o *oracle.Oracle) {
		s := RNDState(o)
		if !t.initSeen {
			t.InitialState, t.initSeen = s, true
		}
		t.Calls = append(t.Calls, RNDCall{
			Step: o.Steps(), State: s, Caller: o.Caller(),
		})
	})
	o.OnCall(o.IDA(Randomize), func(o *oracle.Oracle) {
		t.RandomizeAt = append(t.RandomizeAt, o.Steps())
	})
	return t
}

// Verify 檢查每一次呼叫的狀態都是前一次推一步，最後再對一次現況。
//
// **靜態讀出來的公式與實際跑出來的序列是兩件事。** 公式抄錯一個位元不會
// 報錯，只會讓 remake 的骰子分布悄悄偏掉。
func (t *RNDTrace) Verify(o *oracle.Oracle) error {
	if len(t.Calls) == 0 {
		return fmt.Errorf("一次都沒呼叫 RND——位址可能不對")
	}
	for i := 1; i < len(t.Calls); i++ {
		if want, got := t.Calls[i-1].Next(), t.Calls[i].State; want != got {
			return fmt.Errorf("第 %d 次呼叫的狀態是 %06X，預期 %06X（第 %d 道指令）",
				i, got, want, t.Calls[i].Step)
		}
	}
	if want, got := t.Calls[len(t.Calls)-1].Next(), RNDState(o); want != got {
		return fmt.Errorf("最後一次呼叫之後的狀態是 %06X，預期 %06X", got, want)
	}
	return nil
}

// ByCaller 統計每個呼叫端抽了幾次。
func (t *RNDTrace) ByCaller(o *oracle.Oracle) map[uint32]int {
	out := map[uint32]int{}
	for _, c := range t.Calls {
		out[o.ToIDA(c.Caller)]++
	}
	return out
}

// SetRNDState 直接設定 24 位元亂數狀態。
//
// ⚠ **這是實驗工具，不是「重播」。** 改了狀態，原版接下來的行為就與
// 那一局的自然發展不同了。用它做對照實驗（同一個快照展開幾個不同的種子，
// 看結果怎麼變），用完要 Restore。
func SetRNDState(o *oracle.Oracle, state uint32) {
	o.WriteU16(o.DS(rndStateLo), uint16(state))
	o.WriteU8(o.DS(rndStateHi), uint8(state>>16))
}

// 選方向的迴圈 `sub_11A1E` 裡的 RND 呼叫端（`rich2/docs/re/014` §4c）。
//
//	11A32:  call RND ／ fmul ds:1B56h(×4) ／ fadd ds:1B5Ah(+1)
//	        fistp ds:1C0h   ; 候選方向 ＝ INT(RND×4)+1 → 1..4
//	11A50:  棋盤[格][候選方向+3] == 0 ？ → 重抽
//	11A7A:  cmp ax,2        ; |目前方向 − 候選| == 2（回頭）？
//	11A82:  call RND        ; ← **是的話從這裡重抽**，同樣 ×4
//
// ⚠ **兩個呼叫端都要算。** 只算 11A32 的話序列會缺一截——實測第一步
// 拿到 [1 2]（兩個都不合格），而原版那一格選的是 4，就在 11A87 那一次。
var DirPickCallers = []uint32{0x11A32, 0x11A87}

// DirPickCaller 留給只要第一支的呼叫端。
const DirPickCaller = 0x11A32

// DirectionPicks 從一段 trace 裡篩出「抽方向」的那些，算成 1..4。
//
// **含被拒絕的重抽。** 原版抽到沒出口或回頭就重抽，序列裡看得到那些。
// 要重現原版走的路徑，兩邊的拒絕規則也得一樣。
func DirectionPicks(o *oracle.Oracle, calls []RNDCall) []int {
	var out []int
	for _, c := range calls {
		at := o.ToIDA(c.Caller)
		hit := false
		for _, k := range DirPickCallers {
			if at == k {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		out = append(out, int(uint64(c.Next())*4/(1<<24))+1)
	}
	return out
}
