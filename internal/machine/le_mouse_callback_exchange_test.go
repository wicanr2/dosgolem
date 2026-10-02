package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func mouseSnapshot314(c *cpu386.CPU) leMouseCallbackSnapshot {
	return leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
}

func TestLEMouseExchangeReturnsOldTargetAndRestoresIt(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x123, 2
	c.FPUStack = [8]float64{1.25, 2.5, 3, 4, 5, 6, 7, 8}
	c.SetDescriptor(0x18, cpu386.Descriptor{Limit: 0xffffffff})
	m.Mem[0x23001] = 0xcb
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0xabcd0014, 0x98760004, 0x23001, 0x18
	before := mouseSnapshot314(c)
	mem := bytes.Clone(m.Mem)
	if !s.Handle(c, 0x33) {
		t.Fatal("交換被拒絕")
	}
	want := before
	want.r[cpu386.ECX] = 0x98760001
	want.r[cpu386.EDX] = 0x12000
	want.seg[cpu386.SegES] = 8
	if mouseSnapshot314(c) != want || !bytes.Equal(m.Mem, mem) {
		t.Fatal("舊遮罩／完整遠指標或保存欄位錯誤")
	}
	sel, off := s.MouseCallbackTarget()
	if sel != 0x18 || off != 0x23001 || s.mouseCallback.mask != 4 {
		t.Fatal("新註冊錯誤")
	}
	if !s.Handle(c, 0x33) || c.R[cpu386.ECX] != 0x98760004 || c.R[cpu386.EDX] != 0x23001 || c.Seg[cpu386.SegES] != 0x18 || s.mouseCallback.target.offset != 0x12000 || s.mouseCallback.mask != 1 {
		t.Fatal("不能用返回值交換回舊目標")
	}
}

func TestLEMouseExchangeRejectsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		mask uint32
		sel  uint16
		off  uint32
	}{
		{0x80, 8, 0x12000}, {1, 0, 0x12000}, {1, 0x28, 0x12000}, {1, 8, 0xffffffff}, {1, 8, leMouseCallbackReturn}, {1, 0x18, 0x12000},
	} {
		s, m, _ := mouseCallbackFixture(t, 1)
		c := m.CPU
		c.SetDescriptor(0x18, cpu386.Descriptor{Base: 1, Limit: 0xffffffff})
		if err := s.InjectMouseEvent(1, 2, 0, 0, 0); err != nil {
			t.Fatal(err)
		}
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0x14, tc.mask, tc.off, tc.sel
		before := mouseSnapshot314(c)
		mem := bytes.Clone(m.Mem)
		event := s.mouseCallback.queue[0]
		if s.Handle(c, 0x33) || mouseSnapshot314(c) != before || !bytes.Equal(m.Mem, mem) || s.mouseCallback.mask != 1 || s.mouseCallback.target != (leMouseCallbackTarget{8, 0x12000}) || len(s.mouseCallback.queue) != 1 || s.mouseCallback.queue[0] != event {
			t.Fatalf("拒絕發生突變：%+v", tc)
		}
	}
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	c.R[cpu386.EAX] = 0x14
	other := cpu386.New(m)
	other.R = c.R
	other.Seg = c.Seg
	if s.Handle(other, 0x33) || NewFD2StartupDOS(nil).Handle(c, 0x33) || NewMOO2StartupDOS(nil).Handle(c, 0x33) {
		t.Fatal("錯誤CPU或未安裝服務被放行")
	}
}

func TestLEMouseExchangeDisableAndEmptyTarget(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	if err := s.InjectMouseEvent(1, 2, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0x14, 0x98760000, 0xffffffff, 0
	if !s.Handle(c, 0x33) || c.R[cpu386.ECX] != 0x98760001 || c.R[cpu386.EDX] != 0x12000 || c.Seg[cpu386.SegES] != 8 || len(s.mouseCallback.queue) != 0 || s.mouseCallback.mask != 0 {
		t.Fatal("解除沒有返回舊值或清除待派送事件")
	}
	c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0x98760001, 0x12000, 8
	if !s.Handle(c, 0x33) || c.R[cpu386.ECX] != 0x98760000 || c.R[cpu386.EDX] != 0 || c.Seg[cpu386.SegES] != 0 || s.mouseCallback.mask != 1 {
		t.Fatal("空註冊返回值錯誤")
	}
}

func TestLEMouseExchangeKeepsQueuedTargetAndActiveFrame(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	m.Mem[0x23001] = 0xcb
	if err := s.InjectMouseEvent(1, 2, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x14, 1, 0x23001
	if !s.Handle(c, 0x33) {
		t.Fatal("交換失敗")
	}
	if err := s.InjectMouseEvent(3, 4, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Step(); err != nil || c.EIP != 0x12000 {
		t.Fatalf("舊事件目標未保持：%v", err)
	}
	saved := s.mouseCallback.saved
	frame := bytes.Clone(m.Mem[s.mouseCallback.stackDescriptor.Base+leMouseCallbackStackSize-8 : s.mouseCallback.stackDescriptor.Base+leMouseCallbackStackSize])
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0x14, 1, 0x12000, 8
	if !s.Handle(c, 0x33) || s.mouseCallback.saved != saved || !s.mouseCallback.active || !bytes.Equal(frame, m.Mem[s.mouseCallback.stackDescriptor.Base+leMouseCallbackStackSize-8:s.mouseCallback.stackDescriptor.Base+leMouseCallbackStackSize]) {
		t.Fatal("活動框架被交換破壞")
	}
	for i := 0; i < 8; i++ {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if mouseSnapshot314(c) != saved || s.mouseCallback.completed != 1 {
		t.Fatal("活動回呼未恢復原框架")
	}
	if err := c.Step(); err != nil || c.EIP != 0x23001 {
		t.Fatalf("交換後已排事件目標錯誤：%v", err)
	}
	if err := c.Step(); err != nil || s.mouseCallback.completed != 2 {
		t.Fatalf("新目標未正常遠返回：%v", err)
	}
}
