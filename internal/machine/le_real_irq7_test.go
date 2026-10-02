package machine

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// 自製ISR：保存AX／DX，確認DSP，EOI，還原暫存器，IRET。共9條指令。
var realIRQ7Handler = []byte{0x50, 0x52, 0xba, 0x2e, 2, 0xec, 0xb0, 0x20, 0xe6, 0x20, 0x5a, 0x58, 0xcf}

func realIRQ7Fixture(t *testing.T, code []byte) (*MOO2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	s, m, _ := mouseCallbackFixture(t, 0)
	// 自製映像與低位ISR已佔到15000h，明示分離的DOS配置入口。
	m.DOSArenaBase = 0x16000
	s.DPMI.Attach(m)
	p := s.DPMI.RealModeIO.(*LEOPLPorts)
	copy(m.Mem[0x15000:], code)
	binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0x15000000)
	c := m.CPU
	for i := range c.R {
		c.R[i] = 0xa1230000 + uint32(i)*0x123
	}
	c.R[cpu386.ESP] = 0x80000
	c.EFlags = cpu386.IF | cpu386.DF | 0x247
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x123, 2
	c.FPUStack = [8]float64{2, 3}
	for i := 0x7ff80; i < 0x80080; i++ {
		m.Mem[i] = byte(i)
	}
	p.picPending, p.dsp.IRQPending, p.picMasks[0] = true, true, 0x78
	return s, m, p
}

func TestRealIRQ7PassdownFrameCallerClockAndReuse(t *testing.T) {
	s, m, p := realIRQ7Fixture(t, realIRQ7Handler)
	c := m.CPU
	before := irq0Snapshot(c)
	stack := append([]byte(nil), m.Mem[0x7ff80:0x80080]...)
	blocks, last, next := s.DPMI.DOSMemory(), s.DPMI.dosLast, s.DPMI.nextSel
	descriptors := make(map[uint16]cpu386.Descriptor)
	for k, v := range c.Descriptors {
		descriptors[k] = v
	}
	for repeat := uint64(1); repeat <= 3; repeat++ {
		p.picPending, p.dsp.IRQPending = true, true
		if err := p.AdvanceProtectedMode(c, m); err != nil {
			t.Fatal(err)
		}
		tr := p.IRQ7Last
		if tr == nil || !tr.Returned || tr.Steps != 9 || tr.Entry != "1500:0000" || tr.Stop != "FFFF:FFF0" || tr.Error != "" {
			t.Fatalf("真實ISR返回：%+v", tr)
		}
		if p.IRQ7Passdowns != repeat || p.IRQ7Returns != repeat || p.IRQ7Deliveries != repeat || p.picPending || p.picInService || p.dsp.IRQPending {
			t.Fatalf("來源確認／EOI：%+v", p.State())
		}
		if p.virtualMicros != repeat*10 || p.BIOSClock.Micros != repeat*10 {
			t.Fatal("外層一次＋9真實指令，派送不加tick")
		}
		if irq0Snapshot(c) != before || !bytes.Equal(stack, m.Mem[0x7ff80:0x80080]) {
			t.Fatal("完整caller／FPU／堆疊變動")
		}
		if !reflect.DeepEqual(blocks, s.DPMI.DOSMemory()) || last != s.DPMI.dosLast || next != s.DPMI.nextSel || !reflect.DeepEqual(descriptors, c.Descriptors) {
			t.Fatal("私有配置改了client帳本或descriptor")
		}
		ss := p.realIRQ7.stackSegment
		if ss == 0 || s.DPMI.dosBrk != uint32(ss)*16+4096 {
			t.Fatal("私有堆疊未重用")
		}
		frame := m.Mem[uint32(ss)*16+4090 : uint32(ss)*16+4096]
		if !bytes.Equal(frame, []byte{0xf0, 0xff, 0xff, 0xff, 0x47, 0x06}) {
			t.Fatalf("FLAGS／CS／IP硬體框架=%X", frame)
		}
		if tr.InputR != tr.OutputR || tr.InputSeg != tr.OutputSeg || tr.InputFlags != tr.OutputFlags {
			t.Fatal("自製ISR沒有恢復實模式輸入")
		}
		wantIO := []LEOPLPortEvent{{Port: 0x22e, Value: 0}, {Port: 0x20, Value: 0x20, Write: true}}
		if !reflect.DeepEqual(tr.IO, wantIO) {
			t.Fatalf("真實I/O=%+v", tr.IO)
		}
	}
	if err := c.Step(); err != nil || c.EIP != before.eip+1 {
		t.Fatal("原版caller下一NOP", err)
	}
}

func TestRealIRQ7GatesAndValidationPreservePending(t *testing.T) {
	for _, gate := range []string{"if", "mask", "service", "irq0", "pmvector", "rmvector", "ivtzero", "ivtbad", "segment", "allocation", "stack"} {
		t.Run(gate, func(t *testing.T) {
			s, m, p := realIRQ7Fixture(t, realIRQ7Handler)
			blocked := true
			switch gate {
			case "if":
				m.CPU.EFlags &^= cpu386.IF
			case "mask":
				p.picMasks[0] |= 0x80
			case "service":
				p.picInService = true
			case "irq0":
				p.BIOSClock.protectedIRQ0.active = true
			case "pmvector":
				s.dosVectors[0x0f] = 8<<32 | 0x15000
				blocked = false
			case "rmvector":
				s.DPMI.SetRealModeVector(0x0f, 0x1500, 0)
				blocked = false
			case "ivtzero":
				binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0)
				blocked = false
			case "ivtbad":
				m.Mem = m.Mem[:0x18000]
				binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0xfffffff0)
				blocked = false
			case "segment":
				m.CPU.SetDescriptor(0x10, cpu386.Descriptor{Base: 1, Limit: 0xffff, Writable: true})
				blocked = false
			case "allocation":
				s.DPMI.dosBrk = dosMemTop
				blocked = false
			case "stack":
				p.realIRQ7 = &leRealIRQ7{h: s.DPMI, stackSegment: 0xffff}
				blocked = false
			}
			before := irq0Snapshot(m.CPU)
			err := p.AdvanceProtectedMode(m.CPU, m)
			if blocked && err != nil || !blocked && err == nil {
				t.Fatal("遮罩／拒絕", err)
			}
			if irq0Snapshot(m.CPU) != before || !p.picPending || !p.dsp.IRQPending || p.IRQ7Deliveries != 0 || p.IRQ7Passdowns != 0 {
				t.Fatal("派送前驗界改了現場")
			}
		})
	}
}

func TestRealIRQ7FailuresRemainStoppedAndCallerRestored(t *testing.T) {
	cases := []struct {
		name    string
		code    []byte
		message string
	}{
		{"cpu", []byte{0x0f, 0xff}, "停止"},
		{"port", []byte{0xba, 0x34, 0x12, 0xec}, "IN"},
		{"int", []byte{0xcd, 0x77}, "巢狀INT"},
		{"hlt", []byte{0xf4}, "HLT"},
		{"noeoi", []byte{0xcf}, "EOI"},
		{"retf", []byte{0xb0, 0x20, 0xe6, 0x20, 0xcb}, "IRET"},
		{"stack", []byte{0xb0, 0x20, 0xe6, 0x20, 0x5b, 0x5a, 0x59, 0x6a, 0, 0x51, 0x52, 0x53, 0xcf}, "堆疊"},
		{"timeout", []byte{0xeb, 0xfe}, "20000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, m, p := realIRQ7Fixture(t, tc.code)
			before := irq0Snapshot(m.CPU)
			stack := append([]byte(nil), m.Mem[0x7ff80:0x80080]...)
			err := p.AdvanceProtectedMode(m.CPU, m)
			if err == nil || !strings.Contains(err.Error(), tc.message) || p.IRQ7Last == nil || p.IRQ7Last.Error == "" || p.IRQ7Last.Returned || !p.realIRQ7.failed || p.IRQ7Returns != 0 {
				t.Fatalf("%v %+v", err, p.IRQ7Last)
			}
			if irq0Snapshot(m.CPU) != before || !bytes.Equal(stack, m.Mem[0x7ff80:0x80080]) {
				t.Fatal("失敗改了caller")
			}
			micros := p.virtualMicros
			if err = p.AdvanceProtectedMode(m.CPU, m); err == nil || !strings.Contains(err.Error(), "已失敗") || p.virtualMicros != micros {
				t.Fatal("失敗後仍執行", err)
			}
			r := cpu.New(&dpmiRealBus{m: m, io: p})
			if err = p.AdvanceRealMode(r, m); err == nil || p.virtualMicros != micros {
				t.Fatal("另一模式繞過失敗", err)
			}
		})
	}
}
