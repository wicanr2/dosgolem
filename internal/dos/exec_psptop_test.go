package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestChildPSPHasOwnMemoryTop 釘住子行程 PSP:0002 是它自己的記憶體上限。
//
// 真 DOS 配給子行程剛好需要的塊，PSP:0002 是擁有塊尾端；以前這裡填全域
// MemTop，拿它算可用量的程式（Watcom 6.5 啟動碼）會看到大到不真實的數字
// （`retro-runtime-study-private#32` R48）。
func TestChildPSPHasOwnMemoryTop(t *testing.T) {
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
	if top == machine.MemTop {
		t.Errorf("子行程 PSP:0002＝%04X，還是全域 MemTop（應是行程自有上限）", top)
	}
	if top >= machine.MemTop {
		t.Errorf("子行程 PSP:0002＝%04X，不在合理範圍內", top)
	}
}
