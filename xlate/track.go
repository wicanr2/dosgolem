package xlate

// LineTracker 判斷印字常式逐字命中時，目前這一擊是不是新的一行（spec 202 §2.5）。
//
// 印字常式常以迴圈頭逐字命中：位址（lin）等於上一次 ＋1（useRemain 時另外要求
// remain 等於上一次 −1）就當成同一行的延續，否則是新的一行。第一次呼叫沒有
// 「上一次」可比，一律當新的一行。
type LineTracker struct {
	hasPrev    bool
	prevLin    uint32
	prevRemain int
}

// Hit 記一次命中，回傳這一擊是不是新的一行。
func (t *LineTracker) Hit(lin uint32, remain int, useRemain bool) bool {
	newLine := true
	if t.hasPrev {
		sameLine := lin == t.prevLin+1
		if useRemain {
			sameLine = sameLine && remain == t.prevRemain-1
		}
		newLine = !sameLine
	}
	t.hasPrev = true
	t.prevLin = lin
	t.prevRemain = remain
	return newLine
}
