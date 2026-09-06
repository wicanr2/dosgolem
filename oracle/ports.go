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
