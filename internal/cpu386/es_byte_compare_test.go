package cpu386

import "testing"

func TestESByteCompareBaseAndBounds(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x26, 0x80, 0x3b, 0})
	c := New(mem)
	c.Seg[SegES] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[EBX] = 4
	c.EFlags = IF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EFlags != IF|ZF|PF || mem[68] != 0 {
		t.Fatal("CMP ES零值不符")
	}
	c.EIP = 0
	mem[3] = 1
	if err := c.Step(); err != nil || c.EFlags != IF|CF|PF|AF|SF {
		t.Fatalf("負差旗標=%X err=%v", c.EFlags, err)
	}
	c.EIP = 0
	c.R[EBX] = 32
	flags := c.EFlags
	if err := c.Step(); err == nil || c.EFlags != flags || mem[68] != 0 {
		t.Fatal("ES越界未保持狀態")
	}
}

func TestESByteCompareDLWithEAXMemory(t *testing.T) {
	mem := testBus(make([]byte, 96))
	copy(mem, []byte{0x26, 0x3a, 0x10})
	mem[0x10], mem[0x30] = 9, 5
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x160, 0x28
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 95})
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63})
	c.R[EAX], c.R[EDX], c.EFlags = 0x10, 0x12340005, IF|CF
	if err := c.Step(); err != nil || c.EIP != 3 || c.EFlags != IF|ZF|PF || c.R[EDX] != 0x12340005 || mem[0x30] != 5 {
		t.Fatalf("ES:AX byte 比較錯誤：EIP=%X flags=%X EDX=%X err=%v", c.EIP, c.EFlags, c.R[EDX], err)
	}
	c.EIP, c.R[EDX] = 0, 0x12340004
	if err := c.Step(); err != nil || c.EFlags != IF|CF|SF|PF|AF || c.R[EDX] != 0x12340004 {
		t.Fatalf("CMP 方向與借位錯誤：flags=%X EDX=%X err=%v", c.EFlags, c.R[EDX], err)
	}
	c.EIP = 0
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 15})
	flags := c.EFlags
	if err := c.Step(); err == nil || c.EFlags != flags || c.R[EDX] != 0x12340004 {
		t.Fatal("ES 越界應拒絕且保留暫存器與旗標")
	}
}
