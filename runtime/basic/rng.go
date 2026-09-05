// Package basic 是**編譯後 Microsoft BASIC 程式**的共用支援。
//
// 這一層的判準（`docs/spec/006` §2）：依賴的是**編譯器與執行期的慣例**，
// 換一支同樣用 MS BASIC 編的 binary 仍然成立；但**位址不在這裡**——
// runtime 連結進不同的 binary 會落在不同的地方，所以位址由呼叫端給。
package basic

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// LCG 的三個常數。
//
// **在大富翁2 的 DGROUP 上讀出來驗過**（`ds:279A`／`ds:279E`／`ds:27A2`）。
// 這是 MS BASIC runtime 自己的常數，不是那個遊戲的——但只在一支 binary 上
// 驗過，換一支要重驗一次（`Trace.Verify` 就是在做這件事）。
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

// Config 是 BASIC runtime 在**某一支 binary** 裡的位置。
//
// ⚠ **這四個值是 per-binary 的。** runtime 是連結進去的，位址隨程式而異；
// DGROUP 偏移也一樣。拿別的程式的值套過來不會報錯，只會讓 `TraceRND`
// 一次都攔不到（或攔到別的東西）。
type Config struct {
	RNDEntry  uint32 // `RND` 包裝的進入點（IDA 線性位址）
	Randomize uint32 // `RANDOMIZE` 的進入點
	StateLo   uint16 // 24 位元狀態的低 16 位（DGROUP 偏移）
	StateHi   uint16 // 高 8 位
}

// State 讀目前的 24 位元狀態。
func State(o *oracle.Oracle, c Config) uint32 {
	return uint32(o.Word(o.DS(c.StateLo))) | uint32(o.Byte(o.DS(c.StateHi)))<<16
}

// SetState 直接設定 24 位元狀態。
//
// ⚠ **這是實驗工具，不是「重播」。** 改了狀態，程式接下來的行為就與
// 那一次的自然發展不同了。用它做對照實驗（同一個快照展開幾個不同的種子，
// 看結果怎麼變），用完要 Restore。
func SetState(o *oracle.Oracle, c Config, state uint32) {
	o.WriteU16(o.DS(c.StateLo), uint16(state))
	o.WriteU8(o.DS(c.StateHi), uint8(state>>16))
}

// Call 是一次 `RND` 呼叫。
type Call struct {
	Step   uint64      // 第幾道指令
	State  uint32      // **進入 RND 之前**的狀態
	Caller oracle.Addr // 呼叫端的返回位址
}

// Next 是這一次呼叫產生的新狀態。
func (c Call) Next() uint32 { return LCGNext(c.State) }

// Value 是這一次回傳的 RND 值。
func (c Call) Value() float64 { return float64(c.Next()) / LCGMod }

// Trace 記錄一段執行期間的每一次亂數。
//
// **要重播一個程式的序列，就得在同樣的地方消耗同樣多次。**
// 序列本身是決定性的（LCG），會岔開的永遠是「誰在什麼時候抽了幾次」
// ——例如大富翁2 的擲骰動畫每一幀都真的擲（`rich2/docs/re/155`）。
type Trace struct {
	Calls        []Call
	RandomizeAt  []uint64 // RANDOMIZE 被呼叫的時間點
	InitialState uint32   // 第一次 RND 之前的狀態
	cfg          Config
	initSeen     bool
}

// TraceRND 掛上攔截。**要在 Run／RunUntil 之前叫。**
func TraceRND(o *oracle.Oracle, c Config) *Trace {
	t := &Trace{cfg: c}
	o.OnCall(o.IDA(c.RNDEntry), func(o *oracle.Oracle) {
		s := State(o, c)
		if !t.initSeen {
			t.InitialState, t.initSeen = s, true
		}
		t.Calls = append(t.Calls, Call{
			Step: o.Steps(), State: s, Caller: o.Caller(),
		})
	})
	o.OnCall(o.IDA(c.Randomize), func(o *oracle.Oracle) {
		t.RandomizeAt = append(t.RandomizeAt, o.Steps())
	})
	return t
}

// Verify 檢查每一次呼叫的狀態都是前一次推一步，最後再對一次現況。
//
// **靜態讀出來的公式與實際跑出來的序列是兩件事。** 公式抄錯一個位元不會
// 報錯，只會讓重製版的骰子分布悄悄偏掉。接一支新的 BASIC 程式時，
// 這一道同時驗了「LCG 常數對不對」與「`Config` 的四個位址對不對」。
func (t *Trace) Verify(o *oracle.Oracle) error {
	if len(t.Calls) == 0 {
		return fmt.Errorf("一次都沒呼叫 RND——Config 的位址可能不對")
	}
	for i := 1; i < len(t.Calls); i++ {
		if want, got := t.Calls[i-1].Next(), t.Calls[i].State; want != got {
			return fmt.Errorf("第 %d 次呼叫的狀態是 %06X，預期 %06X（第 %d 道指令）",
				i, got, want, t.Calls[i].Step)
		}
	}
	if want, got := t.Calls[len(t.Calls)-1].Next(), State(o, t.cfg); want != got {
		return fmt.Errorf("最後一次呼叫之後的狀態是 %06X，預期 %06X", got, want)
	}
	return nil
}

// ByCaller 統計每個呼叫端抽了幾次。
func (t *Trace) ByCaller(o *oracle.Oracle) map[uint32]int {
	out := map[uint32]int{}
	for _, c := range t.Calls {
		out[o.ToIDA(c.Caller)]++
	}
	return out
}
