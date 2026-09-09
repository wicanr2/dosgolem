package oracle

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestAtComparesLinearAddress 釘住「`At` 比線性位址，不比 `段:偏移` 這一對數字」。
//
// ⚠ **這個錯誤是靜默的。** 真實模式下同一個 byte 有無數種 `段:偏移` 寫法，
// 程式走到哪一種由呼叫端當時的 CS 決定。比結構的話條件永遠不成立，
// 而 `RunUntil` 只會在跑滿預算之後回錯——形狀與「那段程式碼真的沒被執行」
// 一模一樣。實際踩到的樣子是：同一次執行裡 `OnCall`（比線性位址）攔到了，
// 而 `RunUntil(At(...))`（當時比結構）跑滿六億道指令還說沒到。
func TestAtComparesLinearAddress(t *testing.T) {
	o := &Oracle{m: machine.New()}
	// 同一個線性位址 0x2C5A 的兩種寫法。
	want := Addr{0x02C5, 0x000A}
	if got := (Addr{0x0200, 0x0C5A}).Linear(); got != want.Linear() {
		t.Fatalf("測試前提壞了：兩種寫法的線性位址不同（%05X）", got)
	}
	o.m.CPU.Seg[cpu.CS], o.m.CPU.IP = 0x0200, 0x0C5A
	if !At(want).ready(o) {
		t.Fatal("CS:IP 在同一個線性位址上，At 卻不成立")
	}
	o.m.CPU.Seg[cpu.CS], o.m.CPU.IP = 0x0200, 0x0C5B
	if At(want).ready(o) {
		t.Fatal("差一個 byte 也算走到了")
	}
}
