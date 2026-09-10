package cpu386

import (
	"encoding/binary"
	"testing"
)

func TestWordADDMemoryWidthAndFlags(t *testing.T) {
	for _, stack := range []bool{false, true} {
		for _, v := range []struct {
			a, b, w uint16
			f       uint32
		}{
			{12, 3, 15, PF}, {0xffff, 1, 0, CF | AF | PF | ZF}, {0x7fff, 1, 0x8000, OF | AF | PF | SF},
		} {
			mem := testBus(make([]byte, 128))
			code := []byte{0x66, 0x01, 0x02}
			seg := SegDS
			if stack {
				code = []byte{0x66, 0x01, 0x45, 0}
				seg = SegSS
			}
			copy(mem, code)
			c := New(mem)
			c.Seg[seg] = 0x30
			c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: true})
			c.R[EDX] = 32
			c.R[EBP] = 32
			c.R[EAX] = 0xabcd0000 | uint32(v.b)
			c.EFlags = IF
			binary.LittleEndian.PutUint16(mem[64:], v.a)
			mem[66] = 0xaa
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if binary.LittleEndian.Uint16(mem[64:]) != v.w || mem[66] != 0xaa || c.R[EAX] != 0xabcd0000|uint32(v.b) || c.EFlags != IF|v.f {
				t.Fatalf("兩位元組目的或旗標不符：%x flags=%x", mem[64:67], c.EFlags)
			}
		}
	}
}
func TestWordADDMemoryRejectDoesNotPublish(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, 0x01, 0x02})
		c := New(mem)
		c.Seg[SegDS] = 0x30
		limit := uint32(32)
		if readonly {
			limit = 95
		}
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: limit, Writable: !readonly})
		c.R[EDX] = 32
		c.R[EAX] = 1
		c.EFlags = IF | CF
		binary.LittleEndian.PutUint16(mem[64:], 0xffff)
		if c.Step() == nil {
			t.Fatal("未拒絕無效寫入")
		}
		if binary.LittleEndian.Uint16(mem[64:]) != 0xffff || c.EFlags != IF|CF || c.R[EAX] != 1 {
			t.Fatal("拒絕仍發布狀態")
		}
	}
}
func TestWordADDRegisterPreservesUpper(t *testing.T) {
	c := New(testBus([]byte{0x66, 0x01, 0xc0}))
	c.R[EAX] = 0xbeef8000
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0xbeef0000 || c.EFlags&(CF|OF|ZF) != (CF|OF|ZF) {
		t.Fatalf("別名結果不符：%x", c.R[EAX])
	}
}
