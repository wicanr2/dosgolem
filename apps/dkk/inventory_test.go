package dkk

import "testing"

// TestInventoryClassifies 檢查盤點分類：A 呼叫記字串、呼叫期間的逐字不另記、
// 選單呼叫端記 C、其他逐字依同列連續欄併成 G。
func TestInventoryClassifies(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantC(m)
	const aEntry = 0x6200
	inv := &Inventory{a: aEntry, g: gEntry, run: -1, seen: map[string]int{}}
	s := linear(cSS, cSP)
	glyph := func(site int, ch byte, row, col int) {
		retOff := uint16(0x0100)
		if site >= 0 {
			retOff = uint16(retBase + 0x40*site)
		}
		m.put16(s, retOff)
		m.put16(s+2, menuSeg)
		for i, v := range []int{1, int(ch), 1, 0, 10, row, col} {
			m.put16(s+4+uint32(2*i), uint16(v))
		}
		inv.before(1, 0x05F4, 0x01F5, cSS, cSP, m)
	}
	// 路徑 A：字串 "Hi" 放在 2000:0000
	m[0x20000], m[0x20001], m[0x20002] = 2, 'H', 'i'
	m.put16(s, 0x0500)
	m.put16(s+2, 0x3000)
	for i, v := range []uint16{0x0000, 0x2000, 0, 10, 3, 4} {
		m.put16(s+4+uint32(2*i), v)
	}
	inv.before(1, aEntry>>4, aEntry&0xF, cSS, cSP, m)
	glyph(-1, 'H', 3, 4)                                  // A 呼叫期間：不記
	inv.before(1, 0x3000, 0x0500, cSS, cSP+4+argBytes, m) // A 返回
	glyph(-1, 'O', 5, 0)
	glyph(-1, 'K', 5, 1)
	glyph(2, 'X', 24, 0)
	want := []struct{ path, text string }{{"A", "Hi"}, {"G", "OK"}, {"C", "X"}}
	if len(inv.Events) != len(want) {
		t.Fatalf("事件 %d 筆：%+v", len(inv.Events), inv.Events)
	}
	for i, w := range want {
		if e := inv.Events[i]; e.Path != w.path || e.Text != w.text {
			t.Errorf("事件 %d：%s %q，要 %s %q", i, e.Path, e.Text, w.path, w.text)
		}
	}
	if e := inv.Events[0]; e.Row != 3 || e.Col != 4 || e.Caller != "3000:0500" {
		t.Errorf("A 事件位置：%+v", e)
	}
}
