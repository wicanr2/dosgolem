package cpu386

import "testing"

func TestCSByteLoadUsesDescriptorAndPreservesFlags(t *testing.T) {
	mem := testBus(make([]byte, 256))
	code := []byte{0x2e, 0x8a, 0x82, 0x20, 0, 0, 0}
	copy(mem, code)
	mem[100] = 0xab
	c := New(mem)
	c.Seg[SegCS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 127})
	c.R[EAX] = 0x12345678
	c.R[EDX] = 4
	c.EFlags = IF | OF | CF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x123456ab || c.EFlags != IF|OF|CF || c.EIP != 7 {
		t.Fatal("CS byte讀取錯誤")
	}
	c.EIP = 0
	c.R[EDX] = 128
	if err := c.Step(); err == nil || c.R[EAX] != 0x123456ab {
		t.Fatal("CS越界未拒絕")
	}
}
