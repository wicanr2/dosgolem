package machine

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/454-session-view-block-read-and-digest §3.1 第 1–6 項（machine 側）與 §3.2 第 1 項。
//
// 合成程式（實模式 .COM，PSP:0100）：
//
//	0100  E8 0D 00      call 0110        ; 呼叫子程式 S
//	0103  EB FB         jmp  0100
//	0110  FF 06 00 08   inc word [0800h] ; S 入口：程式自己計數
//	0114  C3            ret
//	0180  CF            iret             ; IRQ0、IRQ1、IRQ7 的處理常式
//	0181  CB            retf             ; 週期遠呼叫的目標
//
// 測試不掛 internal/dos：向量 08h、09h、0Fh 改指 0180h 的 IRET（預設 stub 會轉呼 int 1Ch 而無限遞迴）。
const (
	s454Entry   = 0x0110
	s454Counter = 0x0800
	s454IRET    = 0x0180
	s454RETF    = 0x0181
)

func prog454() []byte {
	code := make([]byte, 0x82)
	copy(code, []byte{0xE8, 0x0D, 0x00, 0xEB, 0xFB})
	copy(code[0x10:], []byte{0xFF, 0x06, 0x00, 0x08, 0xC3})
	code[0x80], code[0x81] = 0xCF, 0xCB
	return code
}

// obs454 是一個觀察點的紀錄：觀察到的 PC 與當下的送達計數。
type obs454 struct {
	cs, ip uint16
	d      uint64
}

// run454 以 RunUntilObserved 跑 budget 步，記下每個觀察點；inject 在第一次觀察到 S 入口時呼叫一次
// （讓送達落在 S 入口的觀察點）。最後補一筆停止後的計數，當最後一個觀察點的「下一個觀察點」。
func run454(t *testing.T, m *Machine, budget uint64, injectAt int, inject func(m *Machine)) []obs454 {
	t.Helper()
	var tr []obs454
	seenS := 0
	_, err := m.RunUntilObserved(nil, budget, func(m *Machine) error {
		tr = append(tr, obs454{m.CPU.Seg[cpu.CS], m.CPU.IP, m.Deliveries()})
		if m.CPU.Seg[cpu.CS] == PSPSeg && m.CPU.IP == s454Entry {
			seenS++
			if inject != nil && seenS == injectAt {
				inject(m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("執行：%v", err)
	}
	return append(tr, obs454{0xFFFF, 0xFFFF, m.Deliveries()})
}

// check454 照 012 §3.2.1 去重：觀察到 S、下一個觀察點計數不變才算一次命中。斷言去重後的命中數等於程式自己的計數；
// 每次落在 S 的送達，下一個觀察點計數加 1 且同一個 PC 之後再被觀察到。回落在 S 的送達次數。
func check454(t *testing.T, what string, m *Machine, tr []obs454) int {
	t.Helper()
	hits, atS := 0, 0
	for i := 0; i+1 < len(tr); i++ {
		o, n := tr[i], tr[i+1]
		if n.d < o.d || n.d-o.d > 1 {
			t.Fatalf("%s：第 %d 個觀察點到下一個，計數 %d → %d", what, i, o.d, n.d)
		}
		if o.cs != PSPSeg || o.ip != s454Entry {
			continue
		}
		if n.d == o.d {
			hits++
			continue
		}
		atS++
		again := false
		for _, x := range tr[i+1:] {
			if x.cs == o.cs && x.ip == o.ip {
				again = true
				break
			}
		}
		if !again {
			t.Errorf("%s：第 %d 個觀察點在 S 有送達，之後沒有再觀察到 S", what, i)
		}
	}
	calls := int(m.Mem[uint32(PSPSeg)*16+s454Counter]) | int(m.Mem[uint32(PSPSeg)*16+s454Counter+1])<<8
	if hits != calls {
		t.Errorf("%s：去重後 S 命中 %d 次，程式實際呼叫 %d 次", what, hits, calls)
	}
	if atS == 0 {
		t.Errorf("%s：沒有任何送達落在 S 入口的觀察點", what)
	}
	return atS
}

func new454(t *testing.T) *Machine {
	t.Helper()
	m := New()
	mustNil(t, "LoadCOM", m.LoadCOM(prog454()))
	prep454(m)
	return m
}

func prep454(m *Machine) {
	for _, v := range []uint8{0x08, 0x09, 0x0F} {
		m.WritePhys16(uint32(v)*4, s454IRET)
		m.WritePhys16(uint32(v)*4+2, PSPSeg)
	}
	m.IRQ0Every = 0
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.picMask = 0
}

// §3.1 第 1 項：IRQ0（實模式）。
func TestDeliveries454IRQ0RealMode(t *testing.T) {
	m := new454(t)
	m.IRQ0Every, m.nextIRQ0 = 7, 7 // 與 4 步一圈的迴圈互質，IRQ0 會輪流落在每個 PC
	tr := run454(t, m, 20000, 0, nil)
	n := check454(t, "IRQ0", m, tr)
	if m.Ticks == 0 || m.Deliveries() != m.Ticks {
		t.Errorf("IRQ0：Deliveries %d，Ticks %d（只有 IRQ0 送達，應相等）", m.Deliveries(), m.Ticks)
	}
	t.Logf("IRQ0：%d 次送達，其中 %d 次落在 S 入口", m.Deliveries(), n)
}

// §3.1 第 2 項：IRQ0（V86，經 IDT 遞送）。
func TestDeliveries454IRQ0V86(t *testing.T) {
	m := deliverProg(t, []byte{0x90, 0x90})
	m.irq0Pending, m.picMask = true, 0
	d0 := m.Deliveries()
	mustStep(t, m, "V86 IRQ0")
	if !m.CPU.Segmented() {
		t.Fatal("前提：IRQ0 應經 IDT 遞送進 ring 0")
	}
	if m.Deliveries() != d0+1 {
		t.Errorf("V86 IRQ0：Deliveries %d → %d，應加 1", d0, m.Deliveries())
	}
}

// §3.1 第 3 項：週期遠呼叫（回呼案在 session，需 internal/dos 的 int F9h 收尾）。
func TestDeliveries454Periodic(t *testing.T) {
	m := new454(t)
	m.SetPeriodicFarCall(PSPSeg, s454RETF, 13)
	tr := run454(t, m, 20000, 0, nil)
	n := check454(t, "週期遠呼叫", m, tr)
	if m.PeriodicCalls() == 0 || m.Deliveries() != m.PeriodicCalls() {
		t.Errorf("週期遠呼叫：Deliveries %d，Calls %d", m.Deliveries(), m.PeriodicCalls())
	}
	t.Logf("週期遠呼叫：%d 次送達，其中 %d 次落在 S 入口", m.Deliveries(), n)
}

// §3.1 第 4 項：IRQ1（佇列與定時鍵）、IRQ7，各觸發一次落在 S 入口觀察點的送達。
func TestDeliveries454IRQ1IRQ7(t *testing.T) {
	cases := []struct {
		name   string
		mk     func(t *testing.T) *Machine
		inject func(m *Machine)
		check  func(m *Machine) bool
	}{
		{"IRQ1 佇列", new454, func(m *Machine) {
			m.QueueScan(0x1E)
			m.KeyEvery, m.nextKey = 1, 0
		}, func(m *Machine) bool { return m.KeyIRQs == 1 }},
		{"IRQ1 定時鍵", new454, func(m *Machine) {
			m.ScheduleKey(m.Steps, 0x1E, false)
		}, func(m *Machine) bool { return m.KeyIRQs == 1 }},
		{"IRQ7", func(t *testing.T) *Machine {
			// 444 的 SB16 選項；請求線直接在觀察點立起（白箱），走 sb8IRQ7 → raiseIRQ(0Fh)。
			m := newSB8(t, prog454(), nil)
			prep454(m)
			return m
		}, func(m *Machine) {
			m.sb8.irr7 = true
		}, func(m *Machine) bool { return m.sb8.irq7Deliveries == 1 }},
	}
	for _, c := range cases {
		m := c.mk(t)
		tr := run454(t, m, 2000, 3, c.inject)
		check454(t, c.name, m, tr)
		if !c.check(m) || m.Deliveries() != 1 {
			t.Errorf("%s：應恰好送達一次：Deliveries %d", c.name, m.Deliveries())
		}
	}
}

// §3.1 第 5 項：V86 下的 CD 15 不計（閘 DPL 3、閘 DPL 0 改送 #GP、IOPL < 3 改送 #GP(0)）；
// 同一個 PC 不再被觀察到。
func TestDeliveries454V86IntNotCounted(t *testing.T) {
	cases := []struct {
		name string
		prep func(m *Machine)
	}{
		{"閘 DPL 3", func(m *Machine) { putGate(m, 0x15, 0x18, uint32(gateOff(0x15)), 0xEE00) }},
		{"閘 DPL 0（#GP）", nil},
		{"IOPL < 3（#GP(0)）", func(m *Machine) {
			putGate(m, 0x15, 0x18, uint32(gateOff(0x15)), 0xEE00)
			m.CPU.SetFlags(cpu.IF)
		}},
	}
	for _, c := range cases {
		m := deliverProg(t, []byte{0xCD, 0x15, 0x90})
		if c.prep != nil {
			c.prep(m)
		}
		cs := m.CPU.Seg[cpu.CS]
		var tr []obs454
		if _, err := m.RunUntilObserved(nil, 6, func(m *Machine) error {
			tr = append(tr, obs454{m.CPU.Seg[cpu.CS], m.CPU.IP, m.Deliveries()})
			return nil
		}); err != nil {
			t.Fatalf("%s：%v", c.name, err)
		}
		if !m.CPU.Segmented() {
			t.Fatalf("%s：前提：應進 ring 0", c.name)
		}
		if tr[0].cs != cs || tr[0].ip != 0x100 {
			t.Fatalf("%s：第一個觀察點應是 CD 15", c.name)
		}
		for i, o := range tr {
			if o.d != tr[0].d || m.Deliveries() != tr[0].d {
				t.Errorf("%s：第 %d 個觀察點 Deliveries %d，應維持 %d", c.name, i, o.d, tr[0].d)
			}
			if i > 0 && o.cs == cs && o.ip == 0x100 {
				t.Errorf("%s：第 %d 個觀察點又看到 CD 15", c.name, i)
			}
		}
	}
}

// §3.1 第 6 項：沒有中斷的程式跑 10 萬步，計數是 0（幀照常產生，幀不是送達）。
func TestDeliveries454NoneIsZero(t *testing.T) {
	m := New()
	m.IRQ0Every = 0
	mustNil(t, "LoadCOM", m.LoadCOM(stepLoopCOM))
	m.VGAFrameEvery, m.nextFrame = 997, 997
	for i := 0; i < 100000; i++ {
		mustStep(t, m, "迴圈")
	}
	if m.Deliveries() != 0 || m.Frames == 0 {
		t.Errorf("Deliveries %d（應 0），Frames %d", m.Deliveries(), m.Frames)
	}
}

// §3.2 第 1 項：deliveries 在 Machine 結構的最後。
func TestDeliveries454FieldLast(t *testing.T) {
	rt := reflect.TypeOf(Machine{})
	last := rt.Field(rt.NumField() - 1)
	if last.Name != "deliveries" {
		t.Errorf("Machine 最後一個欄位是 %s，應為 deliveries", last.Name)
	}
	var m Machine
	if unsafe.Offsetof(m.deliveries)+unsafe.Sizeof(m.deliveries) != unsafe.Sizeof(m) {
		t.Errorf("deliveries 位移 %d + 8 ≠ 結構大小 %d", unsafe.Offsetof(m.deliveries), unsafe.Sizeof(m))
	}
}
