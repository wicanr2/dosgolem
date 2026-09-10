package oracle

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// 埠寫入序列的觀測（從 san1-draw-speed-and-speech 併回來）。
//
// **顯示模式只有這裡問得到**：直接寫暫存器換模式的程式從不呼叫
// `int 10h AH=00`，BDA 那一格會一路停在開機值 03h。所以這兩支壞掉的
// 症狀是「看起來還在文字模式」，不是報錯。

// portFixture 造一台機器，寫幾個埠。
func portFixture(t *testing.T) *Oracle {
	t.Helper()
	m := machine.New()
	o := &Oracle{m: m}
	// 序列器、CRTC 顯示起點、以及一個不相干的埠。
	m.Out8(0x3C4, 0x02)
	m.Out8(0x3C5, 0x0F)
	m.Out8(0x3D4, 0x0C)
	m.Out8(0x3D5, 0x00)
	m.Out8(0x3D4, 0x0D)
	m.Out8(0x3D5, 0x50)
	m.Out8(0x21, 0xFD) // 8259 的遮罩，不該混進 VGA 的查詢
	return o
}

// TestPortWritesRecordsEverything 釘住「每一次寫入都記著」。
func TestPortWritesRecordsEverything(t *testing.T) {
	o := portFixture(t)
	got := o.PortWrites()
	if len(got) != 7 {
		t.Fatalf("記到 %d 次寫入，預期 7", len(got))
	}
	if got[0].Port != 0x3C4 || got[0].Val != 0x02 {
		t.Errorf("第一筆是 %04X=%02X，預期 3C4=02", got[0].Port, got[0].Val)
	}
	// Step 要跟著走，才能與 trace 對齊。
	for i := 1; i < len(got); i++ {
		if got[i].Step < got[i-1].Step {
			t.Errorf("第 %d 筆的 Step 倒退了", i)
		}
	}
}

// TestPortWritesInFiltersByRange 釘住範圍過濾。
//
// 查「這支程式怎麼設顯示的」時，8259 的遮罩混進來就得自己再濾一次，
// 而漏濾的那一次會把它讀成一個 VGA 暫存器。
func TestPortWritesInFiltersByRange(t *testing.T) {
	o := portFixture(t)
	crtc := o.PortWritesIn(0x3D4, 0x3D5)
	if len(crtc) != 4 {
		t.Fatalf("CRTC 範圍收到 %d 筆，預期 4", len(crtc))
	}
	for _, w := range crtc {
		if w.Port < 0x3D4 || w.Port > 0x3D5 {
			t.Errorf("範圍外的 %04X 混進來了", w.Port)
		}
	}
	if n := len(o.PortWritesIn(0x3C4, 0x3C5)); n != 2 {
		t.Errorf("序列器範圍收到 %d 筆，預期 2", n)
	}
	if n := len(o.PortWritesIn(0x300, 0x310)); n != 0 {
		t.Errorf("沒人寫的範圍收到 %d 筆，預期 0", n)
	}
}

// TestPortWritesCarriesDisplayStart 是這一組的用途本身：從埠序列
// 讀得出程式把顯示起點設到哪，而且與 Machine.DisplayStart 一致。
func TestPortWritesCarriesDisplayStart(t *testing.T) {
	o := portFixture(t)
	var hi, lo uint8
	var idx uint8
	for _, w := range o.PortWritesIn(0x3D4, 0x3D5) {
		switch w.Port {
		case 0x3D4:
			idx = w.Val
		case 0x3D5:
			switch idx {
			case 0x0C:
				hi = w.Val
			case 0x0D:
				lo = w.Val
			}
		}
	}
	fromPorts := uint32(hi)<<8 | uint32(lo)
	if got := o.m.DisplayStart(); got != fromPorts {
		t.Errorf("埠序列讀出起點 %d，Machine.DisplayStart 說 %d", fromPorts, got)
	}
	if fromPorts != 0x50 {
		t.Errorf("起點 %d，預期 0x50", fromPorts)
	}
}
