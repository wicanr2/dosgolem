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

func TestCMCComplementsOnlyCarry(t *testing.T) {
	for _, flags := range []uint32{0x202, 0x203, 0xffffffff} {
		mem := testBus{0xf5, 0xf5}
		c := New(mem)
		c.EFlags = flags
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		before := c.R
		if err := c.Step(); err != nil || c.EFlags != flags^CF || c.EIP != 1 || c.R != before {
			t.Fatalf("CMC第一次 flags=%X EIP=%X err=%v", c.EFlags, c.EIP, err)
		}
		if err := c.Step(); err != nil || c.EFlags != flags || c.EIP != 2 || c.R != before {
			t.Fatalf("CMC第二次 flags=%X EIP=%X err=%v", c.EFlags, c.EIP, err)
		}
		if mem[0] != 0xf5 || mem[1] != 0xf5 {
			t.Fatal("CMC改變記憶體")
		}
	}
}

func TestCMCPrefixFailsClosed(t *testing.T) {
	c := New(testBus{0x66, 0xf5})
	c.EFlags = 0x202
	if err := c.Step(); err == nil || c.EFlags != 0x202 {
		t.Fatalf("未支援 CMC 前綴未拒絕或改變旗標: %v", err)
	}
}
