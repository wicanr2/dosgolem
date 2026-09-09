package cpu386

import "testing"

func TestAddress32SIBSegmentAndWrap(t *testing.T) {
	for _, tc := range []struct {
		m    byte
		tail []byte
		seg  int
		addr uint32
	}{
		{0x44, []byte{0x24, 0xfc}, SegSS, 28},
		{0x04, []byte{0x8d, 0xf0, 0xff, 0xff, 0xff}, SegDS, 24},
		{0x84, []byte{0x4d, 0xfc, 0xff, 0xff, 0xff}, SegSS, 46},
		{0x04, []byte{0x24}, SegSS, 32},
		{0x05, []byte{0xff, 0xff, 0xff, 0xff}, SegDS, 0xffffffff},
		{0x41, []byte{0xfc}, SegDS, 6},
	} {
		c := New(testBus(tc.tail))
		c.R[ESP] = 32
		c.R[EBP] = 30
		c.R[ECX] = 10
		seg, addr, err := c.decodeAddress32(tc.m)
		if err != nil || seg != tc.seg || addr != tc.addr {
			t.Fatalf("modrm=%x seg=%d addr=%x err=%v", tc.m, seg, addr, err)
		}
	}
	c := New(testBus{4})
	c.R[EAX] = 0xfffffffe
	_, addr, err := c.decodeAddress32(0x40)
	if err != nil || addr != 2 {
		t.Fatal("32-bit wrap")
	}
}
func TestWordSIBReadWrite(t *testing.T) {
	for _, op := range []byte{0x8b, 0x89} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, op, 0x44, 0x24, 0xfc})
		mem[64] = 0x34
		mem[65] = 0x12
		c := New(mem)
		c.Seg[SegSS] = 2
		c.SetDescriptor(2, Descriptor{Base: 32, Limit: 95, Writable: true})
		c.R[ESP] = 36
		c.R[EAX] = 0xabcd5678
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if op == 0x8b && c.R[EAX] != 0xabcd1234 {
			t.Fatal("word SIB讀")
		}
		if op == 0x89 && (mem[64] != 0x78 || mem[65] != 0x56 || mem[66] != 0) {
			t.Fatal("word SIB寫")
		}
		if c.EFlags != 0xed7 || c.R[ESP] != 36 {
			t.Fatal("非目的狀態變動")
		}
	}
}

func TestDwordSIBReadWriteCompare(t *testing.T) {
	mem := testBus(make([]byte, 160))
	// DS:[EBX+EAX+8]讀EDX，再写SS:[ESP+EAX+8]，比較SS來源。
	copy(mem, []byte{0x8b, 0x54, 0x03, 8, 0x89, 0x54, 0x04, 8, 0x83, 0x7c, 0x04, 8, 1})
	mem[48] = 1
	c := New(mem)
	c.Seg[SegDS] = 1
	c.Seg[SegSS] = 2
	c.SetDescriptor(1, Descriptor{Base: 32, Limit: 63})
	c.SetDescriptor(2, Descriptor{Base: 96, Limit: 63, Writable: true})
	c.R[EAX] = 4
	c.R[EBX] = 4
	c.R[ESP] = 4
	for i := 0; i < 3; i++ {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.R[EDX] != 1 || mem[112] != 1 || mem[113] != 0 || c.EFlags&ZF == 0 {
		t.Fatal("dword SIB或SS選擇")
	}
}

func TestC7IndexedImmediateWidths(t *testing.T) {
	for _, word := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		code := []byte{0xc7, 0x44, 0x07, 4, 0x78, 0x56, 0x34, 0x12}
		if word {
			code = append([]byte{0x66}, code[:6]...)
		}
		copy(mem, code)
		c := New(mem)
		c.Seg[SegDS] = 1
		c.SetDescriptor(1, Descriptor{Base: 32, Limit: 95, Writable: true})
		c.R[EAX] = 4
		c.R[EDI] = 24
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if mem[64] != 0x78 || mem[65] != 0x56 || c.EFlags != 0xed7 || c.EIP != uint32(len(code)) {
			t.Fatal("C7寬度／位址")
		}
		if word && mem[66] != 0 || !word && mem[67] != 0x12 {
			t.Fatal("C7寬度越界")
		}
	}
}

func TestXCHGDisp32Memory(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x87, 0x83, 0xfc, 0xff, 0xff, 0xff})
	mem[64] = 7
	c := New(mem)
	c.Seg[SegDS] = 1
	c.SetDescriptor(1, Descriptor{Base: 32, Limit: 95, Writable: true})
	c.R[EBX] = 36
	c.R[EAX] = 0x12345678
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 7 || mem[64] != 0x78 || mem[67] != 0x12 || c.EFlags != 0xed7 {
		t.Fatal("XCHG交換錯誤")
	}
	c.EIP = 0
	c.SetDescriptor(1, Descriptor{Base: 32, Limit: 95})
	if err := c.Step(); err == nil || c.R[EAX] != 7 || mem[64] != 0x78 {
		t.Fatal("readonly交換")
	}
}

func TestByteSIBHighRegisterAndBoundary(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x8a, 0x7c, 0x24, 0xfc, 0x88, 0x7c, 0x24, 0xfd})
	mem[64] = 0x56
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 33, Writable: true})
	c.R[ESP] = 36
	c.R[EBX] = 0xabcd1234
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EBX] != 0xabcd5634 || mem[65] != 0x56 || mem[66] != 0 || c.EFlags != 0xed7 {
		t.Fatal("BH／byte邊界")
	}
}

func TestMemoryINCDECFlagsAndFailure(t *testing.T) {
	for _, dec := range []bool{false, true} {
		mem := testBus(make([]byte, 64))
		mod := byte(0x40)
		if dec {
			mod = 0x48
		}
		copy(mem, []byte{0xff, mod, 0})
		if !dec {
			mem[32] = 0xff
			mem[33] = 0xff
			mem[34] = 0xff
			mem[35] = 0x7f
		}
		c := New(mem)
		c.Seg[SegDS] = 1
		c.SetDescriptor(1, Descriptor{Limit: 63, Writable: true})
		c.R[EAX] = 32
		c.EFlags = CF | IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.EFlags&CF == 0 || c.EFlags&SF == 0 || (!dec && c.EFlags&OF == 0) {
			t.Fatal("INC/DEC旗標")
		}
		before := append([]byte(nil), mem...)
		flags := c.EFlags
		c.EIP = 0
		c.SetDescriptor(1, Descriptor{Limit: 63})
		if err := c.Step(); err == nil || c.EFlags != flags {
			t.Fatal("readonly INC/DEC")
		}
		for i, b := range before {
			if mem[i] != b {
				t.Fatal("失敗仍寫入")
			}
		}
	}
}
