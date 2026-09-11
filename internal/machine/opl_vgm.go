package machine

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/vgm"
)

// StepsPerSecondNow 是**這台機器現在**的名目速度：每秒幾道指令。
//
//	IRQ0Every × PITHz
//
// 一句話：每次計時器中斷走 `IRQ0Every` 道指令，而程式認定兩次中斷
// 相差 `1/PITHz` 秒。兩邊都是機器自己的設定，不是外部標定。
//
// **它不是另一套模型，是把 StepsPerSecond() 寫成它成立的那個形式。**
// 分頻沒被釘死時 `recalcIRQ0` 讓 `IRQ0Every = IRQ0Base × d / 65536`，
// 而 `PITHz = PITBaseHz / d`——相乘之後 d 約掉，得到的就是那個常數。
//
// ⚠ **差別出在 `IRQ0Pinned`。** 呼叫端用 `-tick N` 釘死間隔之後
// `recalcIRQ0` 不再重算，d 就不會被約掉；這時 `StepsPerSecond()`
// 回的仍是 3.00 M，而機器實際上跑的可能是 11.65 M（logh3：分頻 2048、
// `-tick 20000`）。拿錯的那一個去換算時間，症狀是**曲子播得出來、
// 只是慢了將近四倍**，而且沒有任何一個環節會報錯。
//
// 計時器關著（`IRQ0Every == 0`）時沒有別的時間來源，退回 StepsPerSecond()。
// 見 `docs/spec/190-opl-vgm-dump` §3。
func (m *Machine) StepsPerSecondNow() float64 {
	if m.IRQ0Every == 0 {
		return StepsPerSecond()
	}
	return float64(m.IRQ0Every) * m.PITHz()
}

// OPLVGM 把 OPL 暫存器寫入序列倒成 VGM（`docs/spec/190-opl-vgm-dump`）。
//
// stepsPerSecond 傳 0 ＝ 用 StepsPerSecondNow()。
//
// ⚠ **序列是空的會回錯誤，不會寫出一份沒有音符的 VGM。**
// 空序列最常見的原因是 AdLib 沒開（`SetAdLib` 預設 false，
// 偵測失敗的程式整段跳過音樂路徑），而那與「這個程式沒有音樂」
// 長得一模一樣。
func (m *Machine) OPLVGM(w io.Writer, stepsPerSecond float64) (vgm.Stats, error) {
	if stepsPerSecond <= 0 {
		stepsPerSecond = m.StepsPerSecondNow()
	}
	ws := make([]vgm.Write, len(m.OPL))
	for i, o := range m.OPL {
		ws[i] = vgm.Write{Bank: o.Bank, Reg: o.Reg, Val: o.Val, Step: o.Step}
	}
	return vgm.Encode(w, ws, stepsPerSecond)
}
