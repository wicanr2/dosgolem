package cpu386

import "testing"

func TestESMOVZXByteAndBoundaries(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x26, 0x0f, 0xb6, 0x06})
	mem[68] = 0x80
	c := New(mem)
	c.Seg[SegES] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 4
	c.R[EAX] = 0xffffffff
	c.EFlags = IF | CF
	if err := c.Step(); err != nil || c.R[EAX] != 128 || c.EFlags != IF|CF {
		t.Fatalf("MOVZX錯誤：%v", err)
	}
	c.EIP = 0
	c.R[ESI] = 32
	if err := c.Step(); err == nil || c.R[EAX] != 128 {
		t.Fatal("越界未拒絕")
	}
	c.EIP = 0
	mem[2] = 0xaf
	if err := c.Step(); err == nil || c.R[EAX] != 128 {
		t.Fatal("未核准的ES extended未拒絕")
	}
}
