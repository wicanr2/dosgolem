package cpu386

import "testing"

func TestPushGSUsesDwordStackAndPreservesFlags(t *testing.T) {
	mem := testBus(make([]byte, 96))
	copy(mem, []byte{0x0f, 0xa8})
	c := New(mem)
	c.Seg[SegSS], c.Seg[SegGS] = 0x28, 0x1234
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
	c.R[ESP], c.EFlags = 16, IF|CF|ZF
	if err := c.Step(); err != nil || c.R[ESP] != 12 || c.EIP != 2 || c.EFlags != IF|CF|ZF {
		t.Fatalf("PUSH GS 狀態錯誤：ESP=%X EIP=%X flags=%X err=%v", c.R[ESP], c.EIP, c.EFlags, err)
	}
	if mem[44] != 0x34 || mem[45] != 0x12 || mem[46] != 0 || mem[47] != 0 {
		t.Fatalf("PUSH GS 須寫入零延伸 dword：% X", mem[44:48])
	}
	c.EIP, c.R[ESP] = 0, 3
	if err := c.Step(); err == nil || c.R[ESP] != 3 {
		t.Fatal("ESP 下溢應拒絕")
	}
	c.EIP, c.R[ESP] = 0, 16
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 14, Writable: true})
	mem[44] = 0
	if err := c.Step(); err == nil || c.R[ESP] != 16 || mem[44] != 0 {
		t.Fatal("SS 描述子越界應拒絕且不寫入")
	}
}
