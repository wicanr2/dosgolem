package cpu386

import "testing"

func TestMoveESFromDSEBXWord(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x8e, 0x03}) // mov es,[ebx]
	mem[44], mem[45], mem[46], mem[47] = 0x38, 0x00, 0xff, 0xff
	c := New(mem)
	c.R[EBX], c.Seg[SegDS], c.Seg[SegES], c.EFlags = 12, 0x28, 0x30, IF|CF
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
	c.SetDescriptor(0x38, Descriptor{Base: 64, Limit: 63, Writable: true})
	if err := c.Step(); err != nil || c.EIP != 2 || c.Seg[SegES] != 0x38 || c.R[EBX] != 12 || c.EFlags != IF|CF || mem[44] != 0x38 {
		t.Fatalf("MOV ES,DS:[EBX] EIP=%X ES=%X EBX=%X flags=%X err=%v", c.EIP, c.Seg[SegES], c.R[EBX], c.EFlags, err)
	}
	c.EIP, c.Seg[SegES] = 0, 0x30
	mem[44] = 0x40
	if err := c.Step(); err == nil || c.Seg[SegES] != 0x30 {
		t.Fatal("無效 selector 應拒絕且保留 ES")
	}
	c.EIP = 0
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 12, Writable: true})
	if err := c.Step(); err == nil || c.Seg[SegES] != 0x30 {
		t.Fatal("DS word 跨越界限應拒絕且保留 ES")
	}
	for _, code := range [][]byte{{0x66, 0x8e, 0x03}, {0x26, 0x8e, 0x03}, {0x8e, 0x13}} {
		copy(mem, code)
		c = New(mem)
		c.R[EBX], c.Seg[SegDS] = 12, 0x28
		c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
		if err := c.Step(); err == nil {
			t.Fatalf("未列形狀 % X 應拒絕", code)
		}
	}
}
