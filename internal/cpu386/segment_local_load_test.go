package cpu386

import "testing"

func TestSegmentLoadEBPLocalUsesSSWord(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		code := []byte{0x8e, 0x45, 0xfc}
		if prefix {
			code = append([]byte{0x66}, code...)
		}
		copy(mem, code)
		c := New(mem)
		c.R[EBP] = 16
		c.Seg[SegSS] = 0x28
		c.Seg[SegES] = 0x30
		c.EFlags = IF | CF
		c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
		c.SetDescriptor(0x38, Descriptor{Base: 96, Limit: 31, Writable: true})
		mem[76] = 0x38
		mem[78] = 0xff
		if err := c.Step(); err != nil || c.Seg[SegES] != 0x38 || c.EFlags != IF|CF {
			t.Fatalf("word載入失敗：%v", err)
		}
		c.EIP = 0
		mem[76] = 0x40
		if err := c.Step(); err == nil || c.Seg[SegES] != 0x38 {
			t.Fatal("無效selector未拒絕")
		}
		c.EIP = 0
		c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 12, Writable: true})
		if err := c.Step(); err == nil || c.Seg[SegES] != 0x38 {
			t.Fatal("SS越界未拒絕")
		}
	}
}

func TestSegmentLoadAbsoluteWordUsesESOverride(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x26, 0x66, 0x8e, 0x1d, 0x20, 0, 0, 0})
	mem[0x20], mem[0x40] = 0x99, 0x38
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x160, 0x28
	c.EFlags = IF | CF
	c.SetDescriptor(0x160, Descriptor{Base: 0, Limit: 127, Writable: true})
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
	c.SetDescriptor(0x38, Descriptor{Base: 96, Limit: 31, Writable: true})
	if err := c.Step(); err != nil || c.Seg[SegDS] != 0x38 || c.EFlags != IF|CF || c.EIP != 8 {
		t.Fatalf("ES 絕對位址段載入錯誤：DS=%X flags=%X EIP=%X err=%v", c.Seg[SegDS], c.EFlags, c.EIP, err)
	}
	c.EIP, c.Seg[SegDS] = 0, 0x160
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 0x20, Writable: true})
	if err := c.Step(); err == nil || c.Seg[SegDS] != 0x160 {
		t.Fatal("ES 描述子越界應拒絕且保留 DS")
	}
	c.EIP = 0
	c.SetDescriptor(0x28, Descriptor{Base: 32, Limit: 63, Writable: true})
	mem[0x40] = 0x40
	if err := c.Step(); err == nil || c.Seg[SegDS] != 0x160 {
		t.Fatal("無效 selector 應拒絕且保留 DS")
	}
}
