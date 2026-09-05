package rich2

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// BASIC runtime 在 `RUN_full.EXE` 裡的位置（`rich2/docs/re/050`）。
//
// 演算法與追蹤在 `runtime/basic`——那一層對任何 MS BASIC 編的程式都成立；
// **這四個值是這一支 binary 的**，runtime 連結進不同的程式會落在不同地方。
//
// 那份筆記的輸入檔是 `RUN_unpacked.EXE`，但這幾個位址都在 `0x3AE58` 以下，
// 所以在 `RUN_full.EXE` 上不變（`rich2/CLAUDE.md` §4.1 的遷移規則）；
// DGROUP 的偏移本來就不受換檔影響。
var BASIC = basic.Config{
	RNDEntry:  0x2E3AA, // RND 包裝（1CBE:17CA）
	Randomize: 0x2E423, // RANDOMIZE（1CBE:1843）
	StateLo:   0x22D1,  // 狀態低 16 位
	StateHi:   0x22D3,  // 高 8 位
}

// 以下四個是 `runtime/basic` 的薄包裝，省掉每次都要傳 BASIC。

// RNDState 讀目前的 24 位元狀態。
func RNDState(o *oracle.Oracle) uint32 { return basic.State(o, BASIC) }

// SetRNDState 直接設定 24 位元狀態（實驗用，見 `basic.SetState`）。
func SetRNDState(o *oracle.Oracle, state uint32) { basic.SetState(o, BASIC, state) }

// TraceRND 掛上亂數攔截。**要在 Run／RunUntil 之前叫。**
func TraceRND(o *oracle.Oracle) *basic.Trace { return basic.TraceRND(o, BASIC) }

// LCGNext 推進一步（`basic.LCGNext`）。
func LCGNext(x uint32) uint32 { return basic.LCGNext(x) }

// RNDCall／RNDTrace 是舊名字，指向 `runtime/basic` 的型別。
type (
	RNDCall  = basic.Call
	RNDTrace = basic.Trace
)

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
