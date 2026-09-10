package machine

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

func realModeFixture(t *testing.T, code []byte) (*LEMachine, *DPMIHost) {
	t.Helper()
	m, h := newDPMITest(t)
	copy(m.Mem[0x300:], code)
	h.SetRealModeVector(0x66, 0x30, 0)
	c := m.CPU
	c.Seg[cpu386.SegES] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xfff, Writable: true})
	c.R[cpu386.EAX] = 0x300
	c.R[cpu386.EBX] = 0x66
	c.R[cpu386.EDI] = 0x100
	c.EFlags = cpu386.CF | cpu386.IF | 2
	binary.LittleEndian.PutUint16(m.Mem[0x124:], 0x20)
	binary.LittleEndian.PutUint16(m.Mem[0x120:], 0x202)
	return m, h
}
func TestDPMIRealModeInterruptExecutesAndReturns(t *testing.T) {
	m, h := realModeFixture(t, []byte{0xbb, 0x34, 0x12, 0xb8, 0xcd, 0xab, 0xc6, 0x06, 0x50, 0, 0x5a, 0xcf})
	before := m.CPU.R
	if !h.Handle(m.CPU) {
		t.Fatalf("執行失敗: %+v", h.RealModeLast)
	}
	if m.Mem[0x250] != 0x5a || binary.LittleEndian.Uint16(m.Mem[0x110:]) != 0x1234 || binary.LittleEndian.Uint16(m.Mem[0x11c:]) != 0xabcd {
		t.Fatal("未執行handler或封包順序錯誤")
	}
	if binary.LittleEndian.Uint16(m.Mem[0x120:]) != 0x202 {
		t.Fatal("386 FLAGS錯誤")
	}
	if m.CPU.R != before || m.CPU.EFlags != cpu386.IF|2 || !h.RealModeLast.Returned {
		t.Fatal("PM呼叫者狀態錯誤")
	}
	stack := h.realStack
	if !h.Handle(m.CPU) || h.realStack != stack {
		t.Fatal("預設stack未重用")
	}
}
func TestDPMIRealModeRejectsUnsupportedWithoutSuccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		code []byte
		cx   uint32
	}{
		{"io", []byte{0xe4, 0x40, 0xcf}, 0},
		{"retf", []byte{0xca, 2, 0}, 0},
		{"cx", []byte{0xcf}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, h := realModeFixture(t, tc.code)
			m.CPU.R[cpu386.ECX] = tc.cx
			before := append([]byte(nil), m.Mem[0x100:0x132]...)
			if h.Handle(m.CPU) || h.RealModeLast.Error == "" || h.RealModeLast.Returned {
				t.Fatal("未知能力回報成功")
			}
			for i, b := range before {
				if m.Mem[0x100+i] != b {
					t.Fatal("失敗寫回成功封包")
				}
			}
		})
	}
}
