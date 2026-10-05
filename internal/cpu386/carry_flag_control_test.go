package cpu386

import "testing"

func TestCarryFlagControlPreservesOtherState(t *testing.T) {
	for _, flags := range []uint32{0, 0xffffffff} {
		c := New(testBus{0xf8, 0xf9})
		c.EFlags = flags
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		before := c.R
		if err := c.Step(); err != nil || c.EFlags != flags&^CF || c.R != before {
			t.Fatalf("CLC不符：%v", err)
		}
		if err := c.Step(); err != nil || c.EFlags != flags|CF || c.R != before {
			t.Fatalf("STC不符：%v", err)
		}
	}
}

func TestComplementCarryPreservesOtherState(t *testing.T) {
	for _, flags := range []uint32{0, CF, 0xfffffffe, 0xffffffff, OF | ZF | AF | PF | SF} {
		mem := testBus{0xf5, 0xf5, 0xa5, 0x5a}
		c := New(mem)
		c.EFlags = flags
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		before := c.R
		for i := 1; i <= 2; i++ {
			want := flags
			if i == 1 {
				want ^= CF
			}
			if err := c.Step(); err != nil || c.EFlags != want || c.R != before || c.EIP != uint32(i) {
				t.Fatalf("CMC %d 初值 %08X：flags=%08X EIP=%X regs=%v error=%v", i, flags, c.EFlags, c.EIP, c.R, err)
			}
			if string(mem) != string([]byte{0xf5, 0xf5, 0xa5, 0x5a}) {
				t.Fatal("CMC 改寫記憶體")
			}
		}
	}
}

func TestComplementCarryUnsupportedPrefixesDoNotPublishState(t *testing.T) {
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0xf2, 0xf3, 0xf0} {
		mem := testBus{prefix, 0xf5, 0xa5}
		c := New(mem)
		c.EFlags = 0xffffffff
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		before := c.R
		if err := c.Step(); err == nil || c.EFlags != 0xffffffff || c.R != before || mem[2] != 0xa5 {
			t.Fatalf("前綴 %02X 未嚴格拒收：%v", prefix, err)
		}
	}
}
