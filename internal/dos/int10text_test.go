package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// `int 10h` 文字模式游標、捲動與讀寫字元（`docs/spec/201`）。

func textAt(m *machine.Machine, row, col int) uint16 {
	return m.Read16(uint32(machine.TextSeg)*16 + uint32((row*80+col)*2))
}

func putText(m *machine.Machine, row, col int, v uint16) {
	m.Write16(uint32(machine.TextSeg)*16+uint32((row*80+col)*2), v)
}

// 游標寫進 BDA、讀得回來，而且各頁獨立。反面的症狀是 Borland conio
// 把每一行都疊在第 1 列（§2）。
func TestInt10CursorLivesInBDA(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0x0000, 0x0305 // 頁 0：列 3、欄 5
	call(m, d, 0x10, 0x0200)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0x0100, 0x0A14 // 頁 1：列 10、欄 20
	call(m, d, 0x10, 0x0200)
	if got := m.Read16(0x450); got != 0x0305 {
		t.Fatalf("BDA 0040:0050 是 %04X，預期 0305（先欄後列）", got)
	}
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX] = 0x0000, 0, 0
	call(m, d, 0x10, 0x0300)
	if m.CPU.R[cpu.DX] != 0x0305 || m.CPU.R[cpu.CX] != 0x0607 {
		t.Fatalf("AH=03h 回 DX=%04X CX=%04X，預期 0305／0607", m.CPU.R[cpu.DX], m.CPU.R[cpu.CX])
	}
	m.CPU.R[cpu.BX] = 0x0100
	call(m, d, 0x10, 0x0300)
	if m.CPU.R[cpu.DX] != 0x0A14 {
		t.Fatalf("頁 1 的游標是 %04X，預期 0A14", m.CPU.R[cpu.DX])
	}
}

// 4×20 的視窗捲 1 列：上移、最下列清成 BH 屬性的空白、視窗外不動。
func TestInt10ScrollWindow(t *testing.T) {
	fill := func(m *machine.Machine) {
		for row := 0; row < 6; row++ {
			for col := 0; col < 22; col++ {
				putText(m, row, col, 0x0700|uint16('A'+row))
			}
		}
	}
	window := func(m *machine.Machine) { m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0x0000, 0x0313 }

	t.Run("往上", func(t *testing.T) {
		m, d := newTest(t)
		fill(m)
		window(m)
		m.CPU.R[cpu.BX] = 0x1E00
		call(m, d, 0x10, 0x0601)
		for row, want := range []uint16{0x0742, 0x0743, 0x0744, 0x1E20} {
			if got := textAt(m, row, 0); got != want {
				t.Errorf("第 %d 列是 %04X，預期 %04X", row, got, want)
			}
		}
		if got := textAt(m, 0, 20); got != 0x0741 {
			t.Errorf("視窗右邊外面被動到：%04X", got)
		}
		if got := textAt(m, 4, 0); got != 0x0745 {
			t.Errorf("視窗下面外面被動到：%04X", got)
		}
	})

	t.Run("往下", func(t *testing.T) {
		m, d := newTest(t)
		fill(m)
		window(m)
		m.CPU.R[cpu.BX] = 0x1E00
		call(m, d, 0x10, 0x0701)
		for row, want := range []uint16{0x1E20, 0x0741, 0x0742, 0x0743} {
			if got := textAt(m, row, 19); got != want {
				t.Errorf("第 %d 列是 %04X，預期 %04X", row, got, want)
			}
		}
	})

	t.Run("AL=0 清整個視窗", func(t *testing.T) {
		m, d := newTest(t)
		fill(m)
		window(m)
		m.CPU.R[cpu.BX] = 0x0700
		call(m, d, 0x10, 0x0600)
		for row := 0; row < 4; row++ {
			if got := textAt(m, row, 7); got != 0x0720 {
				t.Errorf("第 %d 列是 %04X，預期 0720", row, got)
			}
		}
	})

	t.Run("行數超過視窗高度也清視窗", func(t *testing.T) {
		m, d := newTest(t)
		fill(m)
		window(m)
		m.CPU.R[cpu.BX] = 0x0700
		call(m, d, 0x10, 0x0609)
		if got, out := textAt(m, 3, 0), textAt(m, 4, 0); got != 0x0720 || out != 0x0745 {
			t.Errorf("視窗內 %04X、視窗外 %04X，預期 0720／0745", got, out)
		}
	})

	t.Run("右下角超出畫面被夾住", func(t *testing.T) {
		m, d := newTest(t)
		putText(m, 24, 79, 0x0758)
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = 0x1800, 0xFFFF, 0x0700
		call(m, d, 0x10, 0x0600)
		if got := textAt(m, 24, 79); got != 0x0720 {
			t.Errorf("最後一格是 %04X，預期 0720", got)
		}
		if got := m.Read16(uint32(machine.TextSeg)*16 + 25*80*2); got != 0 {
			t.Errorf("畫面外被寫入 %04X", got)
		}
	})
}

// AH=09h 寫 CX 格、跨列、游標不動；0Ah 不改屬性；08h 讀回。
func TestInt10WriteAndReadChar(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0x0000, 0x024E // 列 2、欄 78
	call(m, d, 0x10, 0x0200)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = 0x001F, 3
	call(m, d, 0x10, 0x0941)
	for _, p := range [][2]int{{2, 78}, {2, 79}, {3, 0}} {
		if got := textAt(m, p[0], p[1]); got != 0x1F41 {
			t.Errorf("(%d,%d) 是 %04X，預期 1F41", p[0], p[1], got)
		}
	}
	if got := m.Read16(0x450); got != 0x024E {
		t.Fatalf("游標跑到 %04X，AH=09h 不應移動游標", got)
	}
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = 0x0070, 1
	call(m, d, 0x10, 0x0A42)
	if got := textAt(m, 2, 78); got != 0x1F42 {
		t.Errorf("AH=0Ah 之後是 %04X，預期 1F42（屬性不變）", got)
	}
	m.CPU.R[cpu.BX] = 0xFF00
	call(m, d, 0x10, 0x0800)
	if m.CPU.R[cpu.AX] != 0x1F42 {
		t.Errorf("AH=08h 讀回 %04X，預期 1F42", m.CPU.R[cpu.AX])
	}
}

// 圖形模式不動 B800（§1.3、§1.5）。
func TestInt10TextServicesIgnoredInGraphicsMode(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x13)
	putText(m, 0, 0, 0x0741)
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX], m.CPU.R[cpu.BX] = 0, 0x184F, 0x0700
	call(m, d, 0x10, 0x0600)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = 0x001F, 1
	call(m, d, 0x10, 0x095A)
	if got := textAt(m, 0, 0); got != 0x0741 {
		t.Errorf("mode 13h 下 B800 被改成 %04X", got)
	}
}
