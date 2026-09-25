package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// TestSpawnDoesNotOverlapLiveArena 是 R55 E142 的根因測試：
//
// app 先 `AH=48h` 拿堆再 EXEC，子行程 PSP＋映像要是落在活的堆塊上，
// 父堆就被蓋掉（真 DOS 的 MCB arena 絕不允許；參照側子 DS＝0x6CC0 高位載入）。
// 修法（spec-205）：落子走 arena。以前的 `freeSeg+1` 盲落會讓這個測試紅掉——
// 子 PSP 正好是剛配出去的堆段，PSP 頭幾個位元組就把哨兵蓋了。
func TestSpawnDoesNotOverlapLiveArena(t *testing.T) {
	m, d := newTest(t)
	// 父配 0x100 段。
	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("AH=48h 失敗，Missing=%v", d.Missing)
	}
	heap := m.CPU.R[cpu.AX]
	// 哨兵寫在堆塊開頭（子 PSP 落上來第一個被蓋的就是這裡）。
	for i := uint16(0); i < 16; i++ {
		m.Write8(cpu.Addr(heap, i), 0xA5)
	}

	writeChild(t, d, "CHILD.COM", []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21})
	execChild(m, d, "CHILD.COM")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗，Missing=%v", d.Missing)
	}
	for i := 0; i < 200 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	// 哨兵還在：子行程沒壓到活堆。
	for i := uint16(0); i < 16; i++ {
		if got := m.Read8(cpu.Addr(heap, i)); got != 0xA5 {
			t.Fatalf("堆位元組+%d＝%02X，預期哨兵 A5（子行程蓋掉了活堆）", i, got)
		}
	}
	// 子 PSP 不得等於剛配出去的堆段。
	childPSP := d.ExecLog[len(d.ExecLog)-1].PSP
	if childPSP == heap {
		t.Errorf("子 PSP＝%04X，正好是活堆段（落子壓堆）", childPSP)
	}
}
