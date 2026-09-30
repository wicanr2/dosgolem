package cpu386

import "testing"

func TestMoveESWordToDSEBX(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x66, 0x8c, 0x03})
	mem[0x2c], mem[0x2d] = 0xaa, 0xbb
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x28, 0x1234
	c.SetDescriptor(0x28, Descriptor{Base: 0x20, Limit: 0x3f, Writable: true})
	c.R[EBX], c.EFlags = 0x0c, IF|CF|ZF
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if err := c.Step(); err != nil || c.EIP != 3 || mem[0x2c] != 0x34 || mem[0x2d] != 0x12 ||
		c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags {
		t.Fatalf("MOV [EBX],ES EIP=%X bytes=%02X %02X R=%X Seg=%X flags=%X err=%v",
			c.EIP, mem[0x2c], mem[0x2d], c.R, c.Seg, c.EFlags, err)
	}
}

func TestMoveESWordToDSEBXRejectsOutOfBoundsAndOtherShapes(t *testing.T) {
	mem := testBus(make([]byte, 0x80))
	copy(mem, []byte{0x66, 0x8c, 0x03})
	mem[0x2c], mem[0x2d] = 0xaa, 0xbb
	c := New(mem)
	c.Seg[SegDS], c.Seg[SegES] = 0x28, 0x1234
	c.SetDescriptor(0x28, Descriptor{Base: 0x20, Limit: 0x0c, Writable: true})
	c.R[EBX], c.EFlags = 0x0c, IF|CF
	if err := c.Step(); err == nil || mem[0x2c] != 0xaa || mem[0x2d] != 0xbb || c.EFlags != IF|CF {
		t.Fatalf("跨界 word 寫入未拒絕：bytes=%02X %02X flags=%X err=%v", mem[0x2c], mem[0x2d], c.EFlags, err)
	}
	for _, code := range [][]byte{{0x66, 0x8c, 0x0b}, {0x26, 0x66, 0x8c, 0x03}} {
		c = New(testBus(code))
		if err := c.Step(); err == nil {
			t.Fatalf("未列形狀 % X 意外獲准", code)
		}
	}
}
