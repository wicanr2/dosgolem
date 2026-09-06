package oracle

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 強制呼叫（`docs/spec/005` §3.6）。
//
// # 為什麼需要這一層
//
// 對拍一支純函式（亂數、傷害公式、命中判定）時，「等遊戲自己走到那裡」
// 是最貴的一條路：要先過開機、選單、建隊、進戰鬥，每一步都得先解出正確的
// 按鍵序列，而**中間任何一步錯了，看到的都是「一次都沒抽」**——與
// 「這支常式根本沒被呼叫」長得一模一樣（`~/diagnosis-notes` 02）。
//
// KOL 的實測：跑滿 2.6 億道指令、送 12 批鍵，`rand()` 一次都沒進去。
// 花在「找出怎麼走到那裡」的時間，全部不是花在對拍上。
//
// 直接呼叫走的是同一段機器碼、同一份全域狀態，所以驗到的就是原版的實作；
// 差別只在「誰決定什麼時候呼叫」。**這一層驗常式的實作，不驗呼叫時機**——
// 時機仍然只能靠實際跑過遊戲來確認。
const (
	// callSentinelSeg／callSentinelOff 是安排給被呼叫者的假返回位址。
	// 選在傳統記憶體頂端，那裡不會有程式碼；停止條件比 (Seg,Off) 結構相等，
	// 所以就算它落在無效區也不會被執行到。
	callSentinelSeg = 0xFFF0
	callSentinelOff = 0x0000
)

// DefaultCallBudget 是 Call 的預設指令數上限。
//
// **刻意比 DefaultBudget 小兩個數量級。** 被呼叫的是一支常式不是一段流程；
// 跑掉一百萬道還沒回來，多半是位址指錯或那支其實是 near 常式
// （堆疊上的返回位址就少一個 word，`retf` 會跳到垃圾）。
// 早點失敗比跑滿五秒有用。
const DefaultCallBudget = 1_000_000

// Call 直接呼叫原版的一支 **far** 常式，把原版當成函式庫用。
//
//	seed := o.IDA(0x2F4D0)
//	o.WriteU16(seed, 23)
//	v, err := o.Call(o.IDA(0x2D538)) // rand()
//
// 參數照 C 的 far 呼叫慣例由右到左推上堆疊，版面與 Arg 讀到的一致。
// 回傳 `DX:AX`——取 16 位元的回傳值就 `uint16(v)`。
//
// # 邊界
//
//   - **只還原暫存器，不還原記憶體。** 常式改掉的全域（亂數種子就是）會留著，
//     那正是連續呼叫要的。要整份倒回去用 Save／Restore。
//   - **要先跑到資料段設定好之後**（一般是 `_main` 的進入點）。冷啟動當下
//     `DS` 還不是 DGROUP，讀到的全域會是別的東西**而且不會報錯**。
//   - **只支援 far 常式**（`retf` 結尾）。near 的請自己安排堆疊後用 RunUntil。
//   - 期間 OnCall 的 hook 照常觸發，計時器中斷也照常送。
func (o *Oracle) Call(a Addr, args ...uint16) (uint32, error) {
	return o.CallBudget(DefaultCallBudget, a, args...)
}

// CallBudget 同 Call，但指定這一次的指令數上限。
func (o *Oracle) CallBudget(budget uint64, a Addr, args ...uint16) (uint32, error) {
	saved := *o.m.CPU
	c := o.m.CPU

	ss, sp := c.Seg[cpu.SS], c.R[cpu.SP]
	push := func(v uint16) {
		sp -= 2
		o.m.Write16(cpu.Addr(ss, sp), v)
	}
	for i := len(args) - 1; i >= 0; i-- {
		push(args[i])
	}
	push(callSentinelSeg)
	push(callSentinelOff)

	c.R[cpu.SP] = sp
	c.Seg[cpu.CS], c.IP = a.Seg, a.Off

	err := o.RunUntil(callReturned(), Budget(budget))
	ax, dx := c.R[cpu.AX], c.R[cpu.DX]
	*o.m.CPU = saved

	if err != nil {
		return 0, fmt.Errorf("呼叫 %s（IDA %#x）：%w", a, o.ToIDA(a), err)
	}
	return uint32(dx)<<16 | uint32(ax), nil
}

// callReturned 是「被呼叫的常式 retf 回到了假返回位址」。
//
// 比 (Seg,Off) 結構相等而不是線性位址：假返回位址是我們自己造的，
// 不會有別的東西剛好用同一組 Seg:Off 表示它。
func callReturned() Cond {
	return Cond{
		name: fmt.Sprintf("回到假返回位址 %04X:%04X", callSentinelSeg, callSentinelOff),
		ready: func(o *Oracle) bool {
			return o.m.CPU.Seg[cpu.CS] == callSentinelSeg &&
				o.m.CPU.IP == callSentinelOff
		},
	}
}
