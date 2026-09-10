package oracle

// I/O 埠寫入的完整序列。
//
// **顯示模式與 planar 設定只留在埠上，不在記憶體裡。** VGA 的
// Sequencer（3C4/3C5）與 Graphics Controller（3CE/3CF）決定一次
// A0000 的寫入落到哪幾個 plane、走哪種 write mode——只看記憶體
// 會看到四份互相覆蓋的資料，看不出程式其實選過 plane。

// PortWrite 是一次埠寫入。Step 是當時的指令數，可以和 trace 對齊。
type PortWrite struct {
	Port uint16
	Val  uint8
	Step uint64
}

// PortWrites 回傳目前為止的所有埠寫入（複本，依序）。
func (o *Oracle) PortWrites() []PortWrite {
	src := o.m.PortLog
	out := make([]PortWrite, len(src))
	for i, w := range src {
		out[i] = PortWrite{Port: w.Port, Val: w.Val, Step: w.Step}
	}
	return out
}

// PortWritesIn 回 lo–hi（含端點）這段埠的寫入序列。
//
// **顯示模式只有這裡問得到。** BDA 的模式位元組是 `int 10h AH=00` 才會
// 動的，而直接寫暫存器換模式的程式一次都不呼叫它——那一格會一路停在
// 開機值 03h，看起來像「還在文字模式」。畫面幾列高、從哪個位址開始掃描，
// 答案在 CRTC（`3D4`／`3D5`）與序列器的寫入序列裡。
//
// 顯示起點那一項已經解出來了（`Machine.DisplayStart`），問「現在從哪裡
// 開始掃」用它；這一支給的是**過程**——程式一路設了什麼、設了幾次。
func (o *Oracle) PortWritesIn(lo, hi uint16) []PortWrite {
	var out []PortWrite
	for _, w := range o.m.PortLog {
		if w.Port >= lo && w.Port <= hi {
			out = append(out, PortWrite{Port: w.Port, Val: w.Val, Step: w.Step})
		}
	}
	return out
}
