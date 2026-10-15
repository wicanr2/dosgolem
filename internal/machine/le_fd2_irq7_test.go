package machine

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func fd2IRQ7Fixture(t *testing.T) (*FD2StartupDOS, *LEMachine, *LEOPLPorts) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x120000), DOSArenaBase: 0x16000}
	c := cpu386.New(m)
	m.CPU = c
	c.SetDescriptor(8, cpu386.Descriptor{Limit: 0xffffffff})
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xffffffff, Writable: true})
	c.Seg = [6]uint16{8, 0x160, 0x160, 0, 0x20, 0x160}
	c.EIP, c.R[cpu386.ESP], c.EFlags = 0x600, 0x80000, 0x646
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x123, 2
	c.FPUStack = [8]float64{2, 3}
	m.Mem[0x600] = 0x90
	copy(m.Mem[0x15000:], realIRQ7Handler)
	binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0x15000000)
	s := NewFD2StartupDOS(nil)
	s.AttachMachine(m)
	p := NewLEOPLPorts()
	s.DPMI.RealModeIO = p
	if !InstallLEBIOSClock(m, p) {
		t.Fatal("BIOS時鐘")
	}
	p.picPending, p.dsp.IRQPending, p.picMasks[0] = true, true, 0x78
	return s, m, p
}

func TestFD2RealIRQ7ExplicitPassdownAndCallerReturn(t *testing.T) {
	s, m, p := fd2IRQ7Fixture(t)
	if err := InstallLERealIRQ7(m, s, p); err != nil {
		t.Fatal(err)
	}
	if p.BIOSClock.protectedIRQ0 != nil || p.keyboardIRQ1 != nil {
		t.Fatal("不能借用MOO2的IRQ0／鍵盤橋接")
	}
	before := irq0Snapshot(m.CPU)
	stack := append([]byte(nil), m.Mem[0x7ff80:0x80080]...)
	blocks, last, next := s.DPMI.DOSMemory(), s.DPMI.dosLast, s.DPMI.nextSel
	for repeat := uint64(1); repeat <= 3; repeat++ {
		p.picPending, p.dsp.IRQPending = true, true
		if err := p.AdvanceProtectedMode(m.CPU, m); err != nil {
			t.Fatal(err)
		}
		tr := p.IRQ7Last
		if tr == nil || !tr.Returned || tr.Entry != "1500:0000" || tr.Stop != "FFFF:FFF0" || tr.Steps != 9 || tr.Error != "" {
			t.Fatalf("真實ISR邊界：%+v", tr)
		}
		if p.IRQ7Passdowns != repeat || p.IRQ7Returns != repeat || p.picPending || p.picInService || p.dsp.IRQPending {
			t.Fatalf("DSP確認／EOI未完成：%+v", p.State())
		}
		if irq0Snapshot(m.CPU) != before || !bytes.Equal(stack, m.Mem[0x7ff80:0x80080]) {
			t.Fatal("caller／FPU／保護模式堆疊未保存")
		}
		if !reflect.DeepEqual(blocks, s.DPMI.DOSMemory()) || last != s.DPMI.dosLast || next != s.DPMI.nextSel {
			t.Fatal("私有堆疊改變client配置帳本")
		}
		if p.virtualMicros != repeat*10 || p.BIOSClock.Micros != repeat*10 {
			t.Fatal("時間應為外層一次加九條ISR指令")
		}
	}
	if err := m.CPU.Step(); err != nil || m.CPU.EIP != before.eip+1 {
		t.Fatal("未續行真正caller", err)
	}
}

func TestFD2RealIRQ7GatesAndUnknownVectors(t *testing.T) {
	for _, gate := range []string{"uninstalled", "if", "mask", "pmvector", "rmvector", "invalidivt"} {
		t.Run(gate, func(t *testing.T) {
			s, m, p := fd2IRQ7Fixture(t)
			if gate != "uninstalled" {
				if err := InstallLERealIRQ7(m, s, p); err != nil {
					t.Fatal(err)
				}
			}
			blocked := gate == "if" || gate == "mask"
			switch gate {
			case "if":
				m.CPU.EFlags &^= cpu386.IF
			case "mask":
				p.picMasks[0] |= 0x80
			case "pmvector":
				s.dosVectors[0x0f] = 8<<32 | 0x15000
			case "rmvector":
				s.DPMI.SetRealModeVector(0x0f, 0x1500, 0)
			case "invalidivt":
				binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0)
			}
			before := irq0Snapshot(m.CPU)
			err := p.AdvanceProtectedMode(m.CPU, m)
			if blocked && err != nil || !blocked && err == nil || irq0Snapshot(m.CPU) != before || !p.picPending || p.IRQ7Passdowns != 0 {
				t.Fatalf("閘門／未知向量改變現場：%v", err)
			}
		})
	}
}

func TestFD2RealIRQ7InstallRejectsMissingAndDuplicateContext(t *testing.T) {
	s, m, p := fd2IRQ7Fixture(t)
	for _, call := range []func() error{
		func() error { return InstallLERealIRQ7(nil, s, p) },
		func() error { return InstallLERealIRQ7(m, nil, p) },
		func() error { return InstallLERealIRQ7(m, s, nil) },
		func() error { return InstallLERealIRQ7(&LEMachine{}, s, p) },
	} {
		if call() == nil || p.realIRQ7 != nil {
			t.Fatal("無效安裝未拒收")
		}
	}
	if err := InstallLERealIRQ7(m, s, p); err != nil {
		t.Fatal(err)
	}
	bridge := p.realIRQ7
	if InstallLERealIRQ7(m, s, p) == nil || p.realIRQ7 != bridge {
		t.Fatal("重複安裝改寫現場")
	}
}
