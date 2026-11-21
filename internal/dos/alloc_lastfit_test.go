package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// last fit 從自由區塊尾端切（`docs/spec/433-last-fit-allocates-at-top`）。
func TestLastFitAllocatesAtTopOfBlock(t *testing.T) {
	m, d := newTest(t)
	d.initArena()
	last := d.arena[len(d.arena)-1]
	end := last.seg + 1 + last.size // 最高自由區塊資料區的尾端（不含）

	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x21, 0x5801)
	m.CPU.R[cpu.BX] = 0x10
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("last fit 配置失敗")
	}
	if got, want := m.CPU.R[cpu.AX], end-0x10; got != want {
		t.Fatalf("last fit 拿到 %04X，預期尾端 %04X", got, want)
	}

	// 前段仍是自由的：改回 first fit 配得到最前面。
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x21, 0x5801)
	m.CPU.R[cpu.BX] = 0x10
	call(m, d, 0x21, 0x4800)
	if got := m.CPU.R[cpu.AX]; got != last.seg+1 {
		t.Errorf("first fit 拿到 %04X，預期前段開頭 %04X", got, last.seg+1)
	}
}

func TestLastFitExactSizeTakesWholeBlock(t *testing.T) {
	m, d := newTest(t)
	d.initArena()
	last := d.arena[len(d.arena)-1]
	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x21, 0x5801)
	m.CPU.R[cpu.BX] = last.size
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("剛好大小的 last fit 配置失敗")
	}
	if got := m.CPU.R[cpu.AX]; got != last.seg+1 {
		t.Errorf("拿到 %04X，預期整塊 %04X", got, last.seg+1)
	}
}
