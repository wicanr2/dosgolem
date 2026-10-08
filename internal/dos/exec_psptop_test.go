package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 這一檔釘住子行程 PSP:0002 的規則。規則演進史：
//
//   - 一開始填全域 MemTop：洞裡的子行程會拿到大到不真實的數字
//     （Watcom 6.5 啟動碼，`retro-runtime-study-private#32` R48）。
//   - c673226 改填映像尾端（prog.EndSeg）：這是錯的——真 DOS 配給子行程
//     的是整塊自由區，TLINK 拿映像尾端當記憶體上限直接失敗退出（exit 02，
//     BCC 編譯鏈斷掉；回退兩行即復活，實證）。
//   - 正確規則：PSP:0002＝配給子行程那塊自由區的尾端。arena 還沒建時
//     整段 [freeSeg, MemTop) 都是它的，填 MemTop；落在洞裡時填洞頂。

// TestChildPSPHasFullBlockTop 釘住 arena 還沒建時，子行程 PSP:0002＝MemTop。
//
// 這是 BCC／TLINK 鏈的場景：父行程沒配過 48h，子行程擁有 freeSeg 以上全部。
func TestChildPSPHasFullBlockTop(t *testing.T) {
	m, d := newTest(t)
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
	childPSP := d.ExecLog[len(d.ExecLog)-1].PSP
	top := m.Read16(uint32(childPSP)*16 + 2)
	if top != machine.MemTop {
		t.Errorf("子行程 PSP:0002＝%04X，預期 MemTop %04X（整塊自由區尾端）",
			top, uint16(machine.MemTop))
	}
}

// TestChildPSPHasHoleTop 釘住落在洞裡的子行程，PSP:0002＝洞頂（不是 MemTop）。
//
// 這是 Watcom WCC／WCG 的場景：父堆活塊在洞的上方，子行程看不到它們。
func TestChildPSPHasHoleTop(t *testing.T) {
	m, d := newTest(t)
	// 配 A、配 B、還 A：A 的位置變成下面有洞、上面有活塊 B。
	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("配 A 失敗，Missing=%v", d.Missing)
	}
	heapA := m.CPU.R[cpu.AX]
	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("配 B 失敗，Missing=%v", d.Missing)
	}
	heapB := m.CPU.R[cpu.AX]
	m.CPU.Seg[cpu.ES] = heapA
	call(m, d, 0x21, 0x4900)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("還 A 失敗，Missing=%v", d.Missing)
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
	childPSP := d.ExecLog[len(d.ExecLog)-1].PSP
	top := m.Read16(uint32(childPSP)*16 + 2)
	// 洞頂＝活塊 B 的 MCB 段（A 的 MCB 段＋0x100 資料＋1）。
	want := heapB - 1
	if top != want {
		t.Errorf("子行程 PSP:0002＝%04X，預期洞頂 %04X（子 PSP＝%04X）",
			top, want, childPSP)
	}
	if top >= machine.MemTop {
		t.Errorf("子行程 PSP:0002＝%04X，還是全域 MemTop（應是洞頂）", top)
	}
}
