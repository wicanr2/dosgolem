package cpu386

import "testing"

func TestSegmentWordStoreEBPLocalUsesSSAndTwoBytes(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		for i := range mem {
			mem[i] = 0xcc
		}
		code := []byte{0x8c, 0x5d, 0xfc}
		if prefix {
			code = append([]byte{0x66}, code...)
		}
		copy(mem, code)
		c := New(mem)
		c.R[EBP] = 16
		c.Seg[SegDS] = 0x160
		c.Seg[SegSS] = 0x28
		c.EFlags = IF | OF | CF
		c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if mem[76] != 0x60 || mem[77] != 1 || mem[75] != 0xcc || mem[78] != 0xcc || c.EFlags != IF|OF|CF || c.EIP != uint32(len(code)) {
			t.Fatal("段word寫入寬度／位置／旗標錯誤")
		}
		c.EIP = 0
		c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 12, Writable: true})
		mem[76] = 0xcc
		if err := c.Step(); err == nil || mem[76] != 0xcc {
			t.Fatal("未原子拒絕跨越SS界限的word")
		}
	}
}
