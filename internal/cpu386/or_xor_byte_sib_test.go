package cpu386

import "testing"

// FD2 0x1D7FF `0A 1C 14 or bl, byte ptr [esp+edx]`：升級學指令時把 record+0x1a+idx 的
// 位元設起來。ModRM 1C 帶 SIB（base ESP、index EDX），舊的 mod=01 手寫分支解不到，
// 第七章 r5 第 8 回合升級就停在這裡。0A／32 現在和 02／22／2A／3A 走同一條 decodeAddress32。
func TestOrByteRegisterFromStackSIB(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x0a, 0x1c, 0x14})
	mem[0x22] = 0x40
	c := New(mem)
	c.R[ESP], c.R[EDX], c.R[EBX], c.EFlags = 0x20, 2, 0x11223303, CF|OF|AF
	c.Seg[SegSS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f})
	if err := c.Step(); err != nil || c.R[EBX] != 0x11223343 || c.EIP != 3 || c.EFlags&(CF|OF|AF|ZF|SF) != 0 {
		t.Fatalf("OR BL,[ESP+EDX] EBX=%X EIP=%X flags=%X err=%v", c.R[EBX], c.EIP, c.EFlags, err)
	}
}

func TestXorByteRegisterFromMemoryAndRegister(t *testing.T) {
	mem := testBus(make([]byte, 0x40))
	copy(mem, []byte{0x32, 0x0c, 0x30, 0x32, 0xc1})
	mem[0x24] = 0xf0
	c := New(mem)
	c.R[EAX], c.R[ESI], c.R[ECX] = 0x20, 4, 0x0f
	c.Seg[SegDS] = 0x38
	c.SetDescriptor(0x38, Descriptor{Limit: 0x3f})
	if err := c.Step(); err != nil || c.R[ECX] != 0xff || c.EFlags&SF == 0 || c.EFlags&ZF != 0 {
		t.Fatalf("XOR CL,[EAX+ESI] ECX=%X flags=%X err=%v", c.R[ECX], c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.R[EAX] != 0xdf || c.EIP != 5 {
		t.Fatalf("XOR AL,CL EAX=%X EIP=%X err=%v", c.R[EAX], c.EIP, err)
	}
}
