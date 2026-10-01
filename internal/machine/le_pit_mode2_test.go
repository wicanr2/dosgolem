package machine

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// Intel 8254 圖 22：模式 2／3 最小值為 2，二進位 0 編碼最大值 65536。
func TestPIT0Mode2And3DivisorDomain(t *testing.T) {
	for _, control := range []byte{0x34, 0x36} {
		for encoded := 0; encoded <= 65535; encoded++ {
			p := LEPIT0{Reload: 99, Generation: 7}
			if !p.Out8(0x43, control) || p.Mode != (control>>1)&7 || p.Loaded {
				t.Fatal("模式選擇")
			}
			if !p.Out8(0x40, byte(encoded)) || p.Loaded || p.Reload != 99 || p.Generation != 7 {
				t.Fatal("半筆不可發布")
			}
			before := p
			ok := p.Out8(0x40, byte(encoded>>8))
			if encoded == 1 {
				if ok || p != before {
					t.Fatal("非法最小除數必須拒絕且保留半筆狀態")
				}
				continue
			}
			want := uint32(encoded)
			if encoded == 0 {
				want = 65536
			}
			if !ok || !p.Loaded || p.Reload != want || p.Generation != 8 {
				t.Fatalf("模式 %d 編碼 %d：%+v", p.Mode, encoded, p)
			}
		}
	}
}

func TestPIT0Mode2ReconfigureAndReject(t *testing.T) {
	p := LEPIT0{}
	if p.Out8(0x40, 0) {
		t.Fatal("未設定資料埠")
	}
	for _, control := range []byte{0x36, 0x34, 0x36, 0x34} {
		generation := p.Generation
		if !p.Out8(0x43, control) || !p.Out8(0x40, 0xaa) {
			t.Fatal("設定半筆")
		}
		for v := 0; v < 256; v++ {
			if byte(v) == 0x34 || byte(v) == 0x36 {
				continue
			}
			before := p
			if p.Out8(0x43, byte(v)) || p != before {
				t.Fatalf("未知控制字 %X 改變狀態", v)
			}
		}
		before := p
		for _, port := range []uint16{0x41, 0x42, 0x44, 0xffff} {
			if p.Out8(port, 0) || p != before {
				t.Fatal("其他埠改變狀態")
			}
		}
		if !p.Out8(0x43, control) || p.highNext || p.Loaded || p.Generation != generation {
			t.Fatal("控制字未清半筆")
		}
		if !p.Out8(0x40, 0x4e) || !p.Out8(0x40, 0x17) || p.Reload != 5966 || p.Generation != generation+1 {
			t.Fatal("原版合法除數")
		}
		// 已載入值在下一個低位元組後仍有效；高位元組完成後才換世代。
		if !p.Out8(0x40, 2) || !p.Loaded || p.Reload != 5966 || p.Generation != generation+1 {
			t.Fatal("重寫半筆破壞已載入值")
		}
		if !p.Out8(0x40, 0) || p.Reload != 2 || p.Generation != generation+2 {
			t.Fatal("完整重寫")
		}
	}
}

func TestPIT0Mode2SharedPortsAndOriginalConsumer(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x120000)}
	c := cpu386.New(m)
	m.CPU = c
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: uint32(len(m.Mem) - 1), Writable: true})
	c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: 0x188, cpu386.SegGS: 0x20, cpu386.SegSS: 0x188}
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err != nil {
		t.Fatal(err)
	}
	p := s.DPMI.RealModeIO.(*LEOPLPorts)
	// 真正的實模式 CPU 經 DPMI 實模式匯流排寫入同一份模式 3 狀態。
	bus := &dpmiRealBus{m: m, io: s.DPMI.RealModeIO}
	r := cpu.New(bus)
	r.Model = cpu.Model80386
	r.Seg[cpu.CS], r.IP = 0x100, 0
	rmCode := []byte{0xb0, 0x36, 0xe6, 0x43, 0xb0, 0, 0xe6, 0x40, 0xb0, 0, 0xe6, 0x40}
	copy(m.Mem[0x1000:], rmCode)
	for i := 0; i < 6; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
		if err := r.Step(); err != nil || bus.err != nil {
			t.Fatalf("實模式 %v／%v", err, bus.err)
		}
	}
	if p.PIT0.Mode != 3 || p.PIT0.Reload != 65536 || p.BIOSClock.Micros != 6 {
		t.Fatal("實模式共享設定")
	}
	// 原版七條設定指令與下一 A1；只重定位程式及讀取 offset。
	code := []byte{0xe6, 0x43, 0xeb, 0, 0x88, 0xd8, 0xe6, 0x40, 0xeb, 0, 0x88, 0xf8, 0xe6, 0x40, 0xa1, 0, 2, 0, 0}
	copy(m.Mem[0x600:], code)
	binary.LittleEndian.PutUint32(m.Mem[0x200:], 0)
	c.EIP, c.EFlags = 0x600, 0x246
	c.R = [8]uint32{0x1734, 0, 0x174e, 0x174e, 0x3ebad4, 8, 0xc8, 8}
	wantR, seg, flags := c.R, c.Seg, c.EFlags
	for i, ip := range []uint32{0x602, 0x604, 0x606, 0x608, 0x60a, 0x60c, 0x60e, 0x613} {
		if err := c.Step(); err != nil {
			t.Fatalf("原版第 %d 指令：%v", i, err)
		}
		if i == 2 {
			wantR[cpu386.EAX] = 0x174e
		}
		if i == 5 {
			wantR[cpu386.EAX] = 0x1717
		}
		if i == 7 {
			wantR[cpu386.EAX] = 0
		}
		if c.EIP != ip || c.R != wantR || c.Seg != seg || c.EFlags != flags {
			t.Fatalf("原版第 %d 狀態不符：R=%X", i, c.R)
		}
	}
	if p.PIT0.Mode != 2 || !p.PIT0.Loaded || p.PIT0.Reload != 5966 || p.PIT0.Generation != 2 || p.BIOSClock.Micros != 14 {
		t.Fatalf("共享模式切換：%+v", p.PIT0)
	}
	wantLog := []LEOPLPortEvent{{0x43, 0x36, true}, {0x40, 0, true}, {0x40, 0, true}, {0x43, 0x34, true}, {0x40, 0x4e, true}, {0x40, 0x17, true}}
	if !reflect.DeepEqual(p.Log, wantLog) {
		t.Fatalf("共享 OUT 紀錄：%+v", p.Log)
	}
	beforeLog := append([]LEOPLPortEvent(nil), p.Log...)
	beforePIT := p.PIT0
	for _, port := range []uint16{0x40, 0x43} {
		if _, ok := p.In8(port); ok {
			t.Fatal("未支援讀回")
		}
	}
	if p.Out8(0x43, 0x35) || p.PIT0 != beforePIT || !reflect.DeepEqual(p.Log, beforeLog) {
		t.Fatal("拒絕不可登錄成功寫入")
	}
	if !bytes.Equal(m.Mem[0x600:0x613], code) {
		t.Fatal("設定指令被改寫")
	}
}

func TestPIT0Mode2ClockPeriodAndBoundaries(t *testing.T) {
	m, p := clockFixture(t)
	advance := func(n int, on bool) {
		t.Helper()
		for i := 0; i < n; i++ {
			if err := p.BIOSClock.advance(m, p, on); err != nil {
				t.Fatal(err)
			}
		}
	}
	p.Out8(0x43, 0x34)
	p.Out8(0x40, 0x4e)
	advance(10000, true)
	if p.BIOSClock.Deliveries != 0 {
		t.Fatal("未完成重載期間的明示暫停近似")
	}
	p.Out8(0x40, 0x17)
	advance(5000, true)
	if p.BIOSClock.Deliveries != 0 {
		t.Fatal("5966 週期過早")
	}
	advance(1, true)
	if p.BIOSClock.Deliveries != 1 {
		t.Fatal("5966 第一個完整週期")
	}
	advance(45000, true)
	// 50,001 微秒約十個週期，與 Intel 的 N/inputHz 對照。
	if p.BIOSClock.Deliveries != 10 || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 10 {
		t.Fatal("分數時基累計")
	}
	p.Out8(0x21, 0xf9)
	advance(10001, true)
	if !p.BIOSClock.Pending || p.BIOSClock.Deliveries != 10 {
		t.Fatal("遮罩")
	}
	p.Out8(0x21, 0xf8)
	advance(1, false)
	if p.BIOSClock.Deliveries != 10 {
		t.Fatal("IF=0")
	}
	advance(1, true)
	if p.BIOSClock.Deliveries != 11 || p.BIOSClock.Pending {
		t.Fatal("合併後派送")
	}
	p.Out8(0x43, 0x34)
	p.Out8(0x40, 0)
	p.Out8(0x40, 0)
	advance(54925, true)
	if p.BIOSClock.Deliveries != 11 {
		t.Fatal("零編碼最大除數的相位重設")
	}
	advance(1, true)
	if p.BIOSClock.Deliveries != 12 {
		t.Fatal("65536 週期")
	}
	binary.LittleEndian.PutUint32(m.Mem[8*4:], 0x1234)
	p.BIOSClock.Pending = true
	if p.BIOSClock.advance(m, p, true) == nil {
		t.Fatal("客製 IRQ0 不可默默近似")
	}
}
