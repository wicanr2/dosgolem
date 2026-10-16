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

// cplSys 是 CPL 可設的 SysBus 替身。
type cplSys struct {
	fakeSys
	cpl uint8
}

func (s *cplSys) CPL() uint8 { return s.cpl }

func newSegCPU(t *testing.T, cpl uint8) (*CPU, *testBus) {
	t.Helper()
	b := newTestBus()
	c := New(b)
	c.Model = Model80386
	c.SetFlags(0) // 386 的旗標（IOPL 0）；New 以 8086 的固定位元重置
	c.Sys = &cplSys{cpl: cpl}
	c.Seg[CS], c.IP = 0x1000, 0
	c.SetSegmented(true)
	c.SetSegCache(CS, SegCache{Base: 0x10000, Limit: 0xFFFF, Attr: 0x9B})
	return c, b
}

// 434 §6：CPL > IOPL 的 I/O 失敗即關閉；§5.1：CPL ≠ 0 時任何段載入失敗。
func TestSegmentedCPLChecks(t *testing.T) {
	c, b := newSegCPU(t, 3)
	b.Write8(0x10000, 0xE4) // in al,60h
	b.Write8(0x10001, 0x60)
	if err := c.Step(); err == nil || !strings.Contains(err.Error(), "IOPL") {
		t.Errorf("CPL 3 > IOPL 0 的 in：應失敗，實際 %v", err)
	}
	c, b = newSegCPU(t, 3)
	c.SetFlags(c.Flags | 0x3000) // IOPL 3
	b.Write8(0x10000, 0xE4)
	b.Write8(0x10001, 0x60)
	if err := c.Step(); err != nil {
		t.Errorf("CPL 3 ≤ IOPL 3 的 in：應照常，實際 %v", err)
	}
	c, b = newSegCPU(t, 3)
	for i, v := range []byte{0x8E, 0xD8} { // mov ds,ax
		b.Write8(0x10000+uint32(i), v)
	}
	c.R[AX] = 0x08
	if err := c.Step(); err == nil || !strings.Contains(err.Error(), "CPL 3") {
		t.Errorf("CPL 3 的段載入：應失敗，實際 %v", err)
	}
}
