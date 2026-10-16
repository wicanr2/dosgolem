package cpu

import (
	"strings"
	"testing"
)

// docs/spec/434-legacy-core-segmented-mode §9 第 6 項：pop cs 在分段模式失敗即關閉。
// 分段模式只在 Model80386 由 Machine 進入（0F 是雙位元組前綴，不到 POP CS），
// 這裡以 Model8086 直接造出分段狀態，驗 434 §5.1 的防線。
func TestSegmentedPopCSFails(t *testing.T) {
	b := newTestBus()
	c := New(b)
	c.Model = Model8086
	c.Seg[CS], c.IP = 0x1000, 0
	c.Seg[SS], c.R[SP] = 0x2000, 0x100
	c.SetSegmented(true)
	c.SetSegCache(CS, SegCache{Base: 0x10000, Limit: 0xFFFF, Attr: 0x9B})
	c.SetSegCache(SS, SegCache{Base: 0x20000, Limit: 0xFFFF, Attr: 0x93})
	b.Write8(0x10000, 0x0F)
	b.Write8(0x20100, 0x34)
	b.Write8(0x20101, 0x12)
	err := c.Step()
	if err == nil || !strings.Contains(err.Error(), "POP CS") {
		t.Fatalf("分段模式的 POP CS 應失敗即關閉：%v", err)
	}
	if c.Seg[CS] != 0x1000 || c.R[SP] != 0x100 {
		t.Errorf("POP CS 失敗後 CS、SP 不得改變：CS=%04X SP=%04X", c.Seg[CS], c.R[SP])
	}
}
