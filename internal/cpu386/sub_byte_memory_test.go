package cpu386

import "testing"

func TestSUBByteMemorySIBAndBounds(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x2a, 0x0c, 0x06})
	mem[70] = 5
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 4
	c.R[EAX] = 2
	c.R[ECX] = 0x12340003
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[ECX] != 0x123400fe || c.EFlags != IF|CF|AF|SF || mem[70] != 5 {
		t.Fatalf("SIB SUB不符：%v flags=%X", err, c.EFlags)
	}
	c.EIP = 0
	c.R[ESI] = 32
	flags := c.EFlags
	if err := c.Step(); err == nil || c.R[ECX] != 0x123400fe || c.EFlags != flags {
		t.Fatal("SUB越界未保持目的與旗標")
	}
}
