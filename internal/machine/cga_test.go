package machine

import (
	"bytes"
	"testing"
)

// 版面位移（字面值：第 y 條掃描線在 (y/2)×80 + (y%2)×2000h）。
func TestCGAScanlineOffsetLiterals(t *testing.T) {
	for _, c := range []struct {
		y    int
		want uint32
	}{{0, 0}, {1, 0x2000}, {2, 80}, {3, 0x2050}, {198, 0x1EF0}, {199, 0x2000 + 99*80}} {
		if got := CGAScanlineOffset(c.y); got != c.want {
			t.Errorf("CGAScanlineOffset(%d) = %#x，要 %#x", c.y, got, c.want)
		}
	}
	if CGAScanlineOffset(199) != 0x3EF0 {
		t.Errorf("最後一條掃描線要在 3EF0h")
	}
}

// 設模式 04h 重設色彩選擇為 30h，而且在通知 onModeChange 之前完成。
func TestSetVideoModeResetsColorSelectBeforeObserver(t *testing.T) {
	m := New()
	m.SetColorSelect(0x2A)
	seen := -1
	m.ObserveModeChanges(func(ModeChange) { seen = int(m.ColorSelect()) })
	m.SetVideoMode(0x04)
	if m.ColorSelect() != 0x30 {
		t.Errorf("設模式 04h 後色彩選擇 = %#x，要 30h", m.ColorSelect())
	}
	if seen != 0x30 {
		t.Errorf("觀察者看到的色彩選擇 = %#x，重設要在通知之前完成（30h）", seen)
	}
	m.SetColorSelect(0x2A)
	m.SetVideoMode(0x05)
	if m.ColorSelect() != 0x30 {
		t.Errorf("設模式 05h 後色彩選擇 = %#x，要 30h", m.ColorSelect())
	}
	// 其他模式不動它
	m.SetColorSelect(0x2A)
	m.SetVideoMode(0x13)
	if m.ColorSelect() != 0x2A {
		t.Errorf("設模式 13h 不應改色彩選擇，得 %#x", m.ColorSelect())
	}
}

// CGAPalette：預設值、調色盤 0／1、有無強度、背景色變更，共 8 組字面值。
func TestCGAPaletteLiterals(t *testing.T) {
	black := [3]uint8{0, 0, 0}
	for _, c := range []struct {
		name string
		mode uint8
		cs   uint8
		want [4][3]uint8
	}{
		{"非 CGA 模式回預設", 0x13, 0x00, [4][3]uint8{black, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}},
		{"預設 30h（調色盤 1 高強度，背景黑）", 0x04, 0x30, [4][3]uint8{black, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}},
		{"調色盤 1 無強度 20h", 0x04, 0x20, [4][3]uint8{black, {0, 170, 170}, {170, 0, 170}, {170, 170, 170}}},
		{"調色盤 0 無強度 00h", 0x04, 0x00, [4][3]uint8{black, {0, 170, 0}, {170, 0, 0}, {170, 85, 0}}},
		{"調色盤 0 高強度 10h", 0x04, 0x10, [4][3]uint8{black, {85, 255, 85}, {255, 85, 85}, {255, 255, 85}}},
		{"背景藍 01h、調色盤 0", 0x04, 0x01, [4][3]uint8{{0, 0, 170}, {0, 170, 0}, {170, 0, 0}, {170, 85, 0}}},
		{"背景淺灰 07h、調色盤 1 高強度 37h", 0x04, 0x37, [4][3]uint8{{170, 170, 170}, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}},
		{"模式 05h 暫用同一色表 30h", 0x05, 0x30, [4][3]uint8{black, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}},
	} {
		m := New()
		m.SetVideoMode(c.mode)
		if isCGAGraphics(c.mode) {
			m.SetColorSelect(c.cs)
		}
		if got := m.CGAPalette(); got != c.want {
			t.Errorf("%s: 得 %v，要 %v", c.name, got, c.want)
		}
	}
}

// 存態往返：色彩選擇在 BDA（Mem 內），SaveState 後 LoadState 不變。
// 負對照：若把色彩選擇改存在新的 Machine 結構欄位而沒補存態，這個測試會失敗。
func TestColorSelectSurvivesSaveState(t *testing.T) {
	m := New()
	m.SetVideoMode(0x04)
	m.SetColorSelect(0x15)
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	m2 := New()
	if err := m2.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if m2.ColorSelect() != 0x15 {
		t.Errorf("存態往返後色彩選擇 = %#x，要 15h", m2.ColorSelect())
	}
	want := m.CGAPalette()
	if got := m2.CGAPalette(); got != want {
		t.Errorf("存態往返後 CGAPalette = %v，要 %v", got, want)
	}
}
