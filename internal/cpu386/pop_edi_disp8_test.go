package cpu386

import (
	"encoding/binary"
	"testing"
)

func TestPopDwordToDSEdiDisp8(t *testing.T) {
	newCPU := func(code []byte, dsLimit uint32) (*CPU, testBus) {
		mem := testBus(make([]byte, 256))
		copy(mem, code)
		binary.LittleEndian.PutUint32(mem[0x60:], 0xdeadbeef)
		c := New(mem)
		c.R[ESP], c.R[EDI], c.Seg[SegSS], c.Seg[SegDS], c.EFlags = 0x20, 0x20, 0x28, 0x30, IF|CF
		c.SetDescriptor(0x28, Descriptor{Base: 0x40, Limit: 0x7f, Writable: true})
		c.SetDescriptor(0x30, Descriptor{Base: 0x80, Limit: dsLimit, Writable: true})
		return c, mem
	}

	c, mem := newCPU([]byte{0x8f, 0x47, 0x14}, 0x7f)
	if err := c.Step(); err != nil || c.EIP != 3 || c.R[ESP] != 0x24 || c.R[EDI] != 0x20 ||
		c.EFlags != IF|CF || binary.LittleEndian.Uint32(mem[0xb4:]) != 0xdeadbeef ||
		binary.LittleEndian.Uint32(mem[0x60:]) != 0xdeadbeef {
		t.Fatalf("POP DS:[EDI+14h] EIP=%X ESP=%X flags=%X dest=%X err=%v",
			c.EIP, c.R[ESP], c.EFlags, binary.LittleEndian.Uint32(mem[0xb4:]), err)
	}

	c, mem = newCPU([]byte{0x8f, 0x47, 0xf8}, 0x7f)
	if err := c.Step(); err != nil || binary.LittleEndian.Uint32(mem[0x98:]) != 0xdeadbeef || c.R[ESP] != 0x24 {
		t.Fatalf("負 disp8 未正確寫入：dest=%X ESP=%X err=%v", binary.LittleEndian.Uint32(mem[0x98:]), c.R[ESP], err)
	}

	c, mem = newCPU([]byte{0x8f, 0x47, 0x14}, 0x35)
	if err := c.Step(); err == nil || c.R[ESP] != 0x20 || binary.LittleEndian.Uint32(mem[0xb4:]) != 0 {
		t.Fatal("目的 dword 跨越 DS 界限時，ESP 或目的被修改")
	}

	c, mem = newCPU([]byte{0x8f, 0x47, 0x14}, 0x7f)
	c.R[ESP] = 0x7f
	if err := c.Step(); err == nil || c.R[ESP] != 0x7f || binary.LittleEndian.Uint32(mem[0xb4:]) != 0 {
		t.Fatal("來源 dword 跨越 SS 界限時，ESP 或目的被修改")
	}

	for _, code := range [][]byte{{0x8f, 0x4f, 0x14}, {0x66, 0x8f, 0x47, 0x14}} {
		c, mem = newCPU(code, 0x7f)
		if err := c.Step(); err == nil || c.R[ESP] != 0x20 || binary.LittleEndian.Uint32(mem[0xb4:]) != 0 {
			t.Fatalf("未列形狀 % X 應拒絕且保留狀態", code)
		}
	}
	c = New(testBus{0x8f, 0x47})
	c.R[ESP] = 0x20
	if err := c.Step(); err == nil || c.R[ESP] != 0x20 {
		t.Fatal("截短的 disp8 應拒絕且保留 ESP")
	}
}
