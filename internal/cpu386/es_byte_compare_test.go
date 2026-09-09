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
