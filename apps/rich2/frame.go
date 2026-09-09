package rich2

import "github.com/wicanr2/dosgolem/oracle"

// FrameWait 是原版「一個像素幀畫完、開始等待」的 IDA 線性位址。
//
// 出處：`rich2/docs/spec/059`「每個像素幀的等待」（confirmed）——
// 每個像素幀畫完後固定等 4 個 PIT tick，等待入口是 `0x1DD76`
// （→ `0x406B5`／`0x406AE`，`rich2/docs/re/148` §6）。
//
// **這是幀的準確定義**：它是原版自己認為「這一幀畫完了」的那一點。
const FrameWait = 0x1DD76

// FrameTicks 是一個像素幀等待的計時器中斷數（同一份規格）。
//
// PIT 頻率 `1,193,182 / 17,000 = 70.187 Hz`（`rich2/docs/re/154`），
// 所以一個像素幀約 **57.0 ms**。這個數字用來換算節奏，
// **不要拿它當幀的判準**——理由見 `EachFrame`。
const FrameTicks = 4

// EachFrame 在移動動畫的每一個像素幀畫完時呼叫 f，回傳取消函式。
//
// ⚠ **不要用「每 4 個 tick」代替。** 計時器在非動畫期間照樣走
// （等輸入、選單、其他動畫），拿 tick 數除以 4 會產生一堆**沒有畫面更新的
// 假幀**，而假幀不會報錯——它只會讓「第 N 幀」整段錯位，
// 症狀是「畫面看起來都對，只是比對永遠差一點」。
//
// ⚠ **回呼時的畫面是這一幀畫完的結果**：`FrameWait` 在繪製之後、
// 等待之前，正是要取畫面的時刻。
func EachFrame(o *oracle.Oracle, f func(*oracle.Oracle)) func() {
	live := true
	o.OnCall(o.IDA(FrameWait), func(o *oracle.Oracle) {
		if live {
			f(o)
		}
	})
	// `OnCall` 只能加不能移除，所以取消是把回呼變成 no-op。
	return func() { live = false }
}
