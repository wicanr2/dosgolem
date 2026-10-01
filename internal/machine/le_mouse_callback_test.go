package machine

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func mouseCallbackFixture(t *testing.T, mask uint16) (*MOO2StartupDOS, *LEMachine, *int) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x120000)}
	c := cpu386.New(m)
	m.CPU = c
	c.SetDescriptor(8, cpu386.Descriptor{Limit: 0xffffffff})
	c.SetDescriptor(0x10, cpu386.Descriptor{Limit: 0xffffffff, Writable: true})
	c.Seg = [6]uint16{8, 0x10, 8, 0, 0, 0x10}
	c.EIP, c.R[cpu386.ESP], c.EFlags = 0x600, 0x80000, 0x202
	m.Mem[0x600] = 0x90
	// 自製回呼把六個參數存到可讀資料，再修改 EAX 並遠返回。
	code := []byte{}
	for index, reg := range []byte{0, 3, 1, 2, 6, 7} {
		code = append(code, 0x89, reg<<3|5)
		code = binary.LittleEndian.AppendUint32(code, uint32(0x900+4*index))
	}
	code = append(code, 0xb8, 0xff, 0xff, 0xff, 0xff, 0xcb)
	copy(m.Mem[0x12000:], code)
	calls := new(int)
	c.StepHook = func(*cpu386.CPU) (bool, error) { (*calls)++; return false, nil }
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err != nil {
		t.Fatal(err)
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0xabcd000c, 0x98760000|uint32(mask), 0x12000
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if !s.Handle(c, 0x33) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags {
		t.Fatal("註冊必須保存完整 EDX 與所有輸入欄位")
	}
	return s, m, calls
}

func TestLEMouseCallbackDispatchWritesArgumentsAndRestoresState(t *testing.T) {
	s, m, calls := mouseCallbackFixture(t, 1)
	c := m.CPU
	for i := range c.R {
		c.R[i] = 0x13570000 + uint32(i)
	}
	c.R[cpu386.ESP] = 0x80000
	c.EFlags = 0x246 | cpu386.DF
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x123, 2
	c.FPUStack = [8]float64{2, 3}
	s.SetMouseState(640, 240, 0)
	before := leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	if err := s.InjectMouseEvent(657, 189, 0, -2, 6); err != nil {
		t.Fatal(err)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 0x12000 || c.EFlags&(cpu386.IF|cpu386.DF) != 0 || !s.mouseCallback.active || c.Seg[cpu386.SegSS] == 0x10 {
		t.Fatalf("回呼入口錯誤：EIP=%X flags=%X SS=%X", c.EIP, c.EFlags, c.Seg[cpu386.SegSS])
	}
	// 在活動回呼期間排下一事件；STI 也不可讓回呼重入。
	if err := s.InjectMouseEvent(658, 190, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	c.EFlags |= cpu386.IF
	c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = 0, 0, [8]float64{}, 0
	for i := 0; i < 8; i++ {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	after := leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	if after != before {
		t.Fatalf("被中斷架構狀態未完整恢復：%+v", after)
	}
	want := []uint32{1, 0, 657, 189, 0xfffe, 6}
	for i, value := range want {
		if got := binary.LittleEndian.Uint32(m.Mem[0x900+4*i:]); got != value {
			t.Fatalf("參數 %d=%X，預期 %X", i, got, value)
		}
	}
	mask, pending, active, started, completed := s.MouseCallbackState()
	if mask != 1 || pending != 1 || active || started != 1 || completed != 1 || *calls != 9 {
		t.Fatalf("FIFO／非重入／既有 hook：%d %d %t %d %d calls=%d", mask, pending, active, started, completed, *calls)
	}
	ports := s.DPMI.RealModeIO.(*LEOPLPorts)
	if ports.BIOSClock.Micros != 9 {
		t.Fatal("回呼步進漏掉既有 BIOS 時鐘")
	}
	if err := c.Step(); err != nil || !s.mouseCallback.active || c.R[cpu386.ECX] != 658 {
		t.Fatalf("第二事件未按序派送：%v", err)
	}
}

func TestLEMouseCallbackMasksRangesAndIF(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 2)
	c := m.CPU
	s.mouseRangeX = newMouseCoordinateRange(0, 100)
	s.mouseRangeY = newMouseCoordinateRange(0, 50)
	s.SetMouseState(80, 40, 0)
	if err := s.InjectMouseEvent(200, 90, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(s.mouseCallback.queue) != 0 || s.mouseX != 100 || s.mouseY != 50 {
		t.Fatal("遮罩／座標限制不符")
	}
	if err := s.InjectMouseEvent(100, 50, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	c.EFlags &^= cpu386.IF
	if handled, err := s.mouseCallback.step(c); handled || err != nil || len(s.mouseCallback.queue) != 1 {
		t.Fatal("IF=0 不得派送")
	}
	c.EFlags |= cpu386.IF
	if err := c.Step(); err != nil || c.R[cpu386.EAX] != 2 || c.R[cpu386.EBX] != 1 || c.R[cpu386.ECX] != 100 || c.R[cpu386.EDX] != 50 {
		t.Fatalf("左鍵事件／範圍錯誤：R=%X err=%v", c.R, err)
	}
}

func TestLEMouseCallbackButtonTransitionsAndQueueLimit(t *testing.T) {
	s, _, _ := mouseCallbackFixture(t, 0x7f)
	for _, tc := range []struct{ buttons, flags uint16 }{{1, 2}, {0, 4}, {2, 8}, {0, 16}, {4, 32}, {0, 64}} {
		if err := s.InjectMouseEvent(s.mouseX, s.mouseY, tc.buttons, 0, 0); err != nil {
			t.Fatal(err)
		}
		if got := s.mouseCallback.queue[len(s.mouseCallback.queue)-1].flags; got != tc.flags {
			t.Fatalf("按鍵類型=%X", got)
		}
	}
	for len(s.mouseCallback.queue) < leMouseCallbackQueueLimit {
		if err := s.InjectMouseEvent(s.mouseX, s.mouseY, s.mouseButtons, 1, 0); err != nil {
			t.Fatal(err)
		}
	}
	beforeX, beforeY, beforeButtons := s.mouseX, s.mouseY, s.mouseButtons
	if s.InjectMouseEvent(1, 2, 1, 1, 0) == nil || s.mouseX != beforeX || s.mouseY != beforeY || s.mouseButtons != beforeButtons || len(s.mouseCallback.queue) != leMouseCallbackQueueLimit {
		t.Fatal("滿佇列應拒絕且不改狀態")
	}
	if s.InjectMouseEvent(1, 2, 8, 0, 0) == nil {
		t.Fatal("未知按鍵被放行")
	}
}

func TestLEMouseCallbackResetDisableAndRegistrationRejection(t *testing.T) {
	for _, fn := range []uint32{0, 0x21, 0x0c} {
		s, m, _ := mouseCallbackFixture(t, 1)
		if err := s.InjectMouseEvent(1, 2, 0, 0, 0); err != nil {
			t.Fatal(err)
		}
		m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX] = fn, 0, 0xffffffff
		m.CPU.Seg[cpu386.SegES] = 0
		if !s.Handle(m.CPU, 0x33) || s.mouseCallback.mask != 0 || len(s.mouseCallback.queue) != 0 {
			t.Fatalf("解除／重設 %X 失敗", fn)
		}
	}
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	for _, tc := range []struct {
		mask     uint32
		selector uint16
		offset   uint32
	}{{0x80, 8, 0x12000}, {1, 0, 0x12000}, {1, 0x28, 0x12000}, {1, 8, 0xffffffff}} {
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.Seg[cpu386.SegES] = 0xc, tc.mask, tc.offset, tc.selector
		beforeR, beforeSeg := c.R, c.Seg
		if s.Handle(c, 0x33) || c.R != beforeR || c.Seg != beforeSeg || s.mouseCallback.target.offset != 0x12000 {
			t.Fatal("無效註冊應拒絕且保存舊目標")
		}
	}
	c.SetDescriptor(0x28, cpu386.Descriptor{Base: 1, Limit: 0xffffffff})
	c.Seg[cpu386.SegES], c.R[cpu386.EDX] = 0x28, 0x12000
	if s.Handle(c, 0x33) {
		t.Fatal("非平坦執行段被放行")
	}
	if NewFD2StartupDOS(nil).Handle(c, 0x33) || NewMOO2StartupDOS(nil).Handle(c, 0x33) {
		t.Fatal("未安裝派送器不應接受註冊")
	}
}

func TestLEMouseCallbackBadReturnFailsClosed(t *testing.T) {
	for _, corrupt := range []string{"frame", "stack", "descriptor"} {
		s, m, _ := mouseCallbackFixture(t, 1)
		c := m.CPU
		if err := s.InjectMouseEvent(1, 2, 0, 0, 0); err != nil {
			t.Fatal(err)
		}
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		c.EIP = 0x12000 + 41 // 自製回呼最後的 CB。
		d := s.mouseCallback
		switch corrupt {
		case "frame":
			m.Mem[d.stackDescriptor.Base+leMouseCallbackStackSize-8] = 0
		case "stack":
			c.R[cpu386.ESP]--
		case "descriptor":
			c.SetDescriptor(d.stackSelector, cpu386.Descriptor{})
		}
		beforeR, beforeSeg, beforeEIP := c.R, c.Seg, c.EIP
		if c.Step() == nil || !d.active || d.completed != 0 || c.R != beforeR || c.Seg != beforeSeg || c.EIP != beforeEIP {
			t.Fatalf("錯誤返回 %s 未拒絕", corrupt)
		}
	}
}

func TestLEMousePositionSettingPreservesCallbackAndQueryState(t *testing.T) {
	s, m, _ := mouseCallbackFixture(t, 1)
	c := m.CPU
	s.mouseRangeX = newMouseCoordinateRange(0, 1278)
	s.mouseRangeY = newMouseCoordinateRange(0, 479)
	s.SetMouseState(320, 240, 2)
	for _, tc := range []struct {
		x, y         uint32
		wantX, wantY uint16
	}{{640, 0x003400f0, 640, 240}, {0xffff, 0xffff, 0, 0}, {1278, 479, 1278, 479}, {1300, 500, 1278, 479}} {
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0xabcd0004, tc.x, tc.y
		beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
		if !s.Handle(c, 0x33) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags {
			t.Fatal("位置設定不可改輸入欄位")
		}
		if s.mouseCallback.mask != 1 || len(s.mouseCallback.queue) != 0 || s.mouseButtons != 2 || s.mouseSensitivityX != 50 || s.mouseSensitivityY != 50 || s.mouseDoubleSpeed != 50 {
			t.Fatal("位置設定改動其他滑鼠狀態")
		}
		c.R[cpu386.EAX] = 3
		if !s.Handle(c, 0x33) || uint16(c.R[cpu386.ECX]) != tc.wantX || uint16(c.R[cpu386.EDX]) != tc.wantY || uint16(c.R[cpu386.EBX]) != 2 {
			t.Fatalf("受控位置讀回錯誤：R=%X", c.R)
		}
	}
	c.R[cpu386.EAX] = 4
	beforeR := c.R
	if NewFD2StartupDOS(nil).Handle(c, 0x33) || c.R != beforeR {
		t.Fatal("一般 FD2 不得接受 MOO2 位置設定")
	}
}
