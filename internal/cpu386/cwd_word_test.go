package cpu386

import (
	"bytes"
	"testing"
)

// 規格360用signed數值範圍，低word替換沿獨立little-endian視圖。
func cwdWordDX360(ax uint16) uint16 {
	signed := int(ax)
	if signed >= 32768 {
		signed -= 65536
	}
	if signed < 0 {
		return 65535
	}
	return 0
}
func TestCWDWordAllValuesFlagsAndHighWords(t *testing.T) {
	code := []byte{0x66, 0x99}
	c, mem := xchgByteFixture354(code)
	for i := range c.R {
		c.R[i] = 0xa5b60000 + uint32(i)*0x10000 + 0x1357
	}
	initial := snapshotInImmediate(c)
	for high, axHigh := range []uint32{0, 0xffff0000, 0x80000000, 0x7fff0000} {
		dxOld := []uint32{0x00008001, 0x80001357, 0xffff7fff, 0x1357ffff}[high]
		for pattern := 0; pattern < 64; pattern++ {
			flags := uint32(2 | IF | DF | 0x200000)
			for bit, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if pattern/(1<<bit)%2 != 0 {
					flags |= flag
				}
			}
			for value := 0; value < 65536; value++ {
				ax := uint16(value)
				c.R = initial.r
				c.R[EAX] = axHigh + uint32(ax)
				c.R[EDX] = dxOld
				c.EIP, c.EFlags = 0, flags
				want := initial
				want.r = imulWordReplace357(c.R, EDX, cwdWordDX360(ax))
				want.flags = flags
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want {
					t.Fatalf("AX=%04X EAXhigh=%X oldEDX=%X flags=%X err=%v", ax, axHigh, dxOld, flags, err)
				}
			}
		}
	}
	expected := make([]byte, len(mem))
	copy(expected, code)
	if !bytes.Equal(mem, expected) {
		t.Fatal("CWD修改RAM")
	}
}
func TestCWDWordTruncationAndPrefixes(t *testing.T) {
	code := []byte{0x66, 0x99}
	for cut := 0; cut < len(code); cut++ {
		c, mem := xchgByteFixture354(code)
		c.R[EAX], c.R[EDX] = 0x8001, 0xa5b61357
		bus := &xchgByteBus354{testBus: mem, readFail: uint32(cut), writeFail: ^uint32(0)}
		c.Bus = bus
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
			t.Fatalf("cut=%d err=%v", cut, err)
		}
	}
	for _, prefix := range []byte{0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0x67, 0xf0, 0xf2, 0xf3} {
		for _, suffix := range [][]byte{{0x66, 0x99}, {0x99}} {
			c, mem := xchgByteFixture354(append([]byte{prefix}, suffix...))
			c.R[EAX], c.R[EDX] = 0x8001, 0xa5b61357
			for _, seg := range c.Seg {
				c.SetDescriptor(seg, Descriptor{Base: 512, Limit: 511, Writable: true})
			}
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
				t.Fatalf("prefix=%02X suffix=%X err=%v", prefix, suffix, err)
			}
		}
	}
}
func TestCWDWordExistingCDQAndConsumerPath(t *testing.T) {
	for _, eax := range []uint32{0, 1, 0x7fff, 0x8000, 0xffff, 0x7fffffff, 0x80000000, 0xffffffff} {
		c, mem := xchgByteFixture354([]byte{0x99})
		c.R[EAX], c.R[EDX] = eax, 0xa5b61357
		want := snapshotInImmediate(c)
		want.r[EDX] = 0
		if eax >= 2147483648 {
			want.r[EDX] = 0xffffffff
		}
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != 1 || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("CDQ eax=%X err=%v", eax, err)
		}
	}
	code := []byte{0x66, 0x99, 0x66, 0x2b, 0xc2, 0x66, 0xd1, 0xf8}
	for _, ax := range []uint16{0, 1, 2, 3, 0x7fff, 0x8000, 0x8001, 0xfffd, 0xfffe, 0xffff} {
		c, mem := xchgByteFixture354(code)
		c.R[EAX], c.R[EDX] = 0xa5b60000|uint32(ax), 0x13570001
		want := snapshotInImmediate(c)
		want.r = imulWordReplace357(c.R, EDX, cwdWordDX360(ax))
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("CWD AX=%X err=%v", ax, err)
		}
		dx := cwdWordDX360(ax)
		sub := subWordValue359(ax, dx)
		want.r = imulWordReplace357(want.r, EAX, sub)
		want.flags = subWordFlags359(ax, dx, want.flags)
		if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("SUB AX=%X err=%v", ax, err)
		}
		signed := int(sub)
		if signed >= 32768 {
			signed -= 65536
		}
		half := signed / 2
		if signed < 0 && signed%2 != 0 {
			half--
		}
		result := uint16((half%65536 + 65536) % 65536)
		flags := want.flags &^ (CF | PF | AF | ZF | SF | OF)
		if sub%2 != 0 {
			flags |= CF
		}
		if result == 0 {
			flags |= ZF
		}
		if half < 0 {
			flags |= SF
		}
		ones := 0
		for n := int(result) % 256; n > 0; n /= 2 {
			ones += n % 2
		}
		if ones%2 == 0 {
			flags |= PF
		}
		want.r = imulWordReplace357(want.r, EAX, result)
		want.flags = flags
		if err := c.Step(); err != nil || c.EIP != 8 || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("SAR AX=%X err=%v", ax, err)
		}
		// AF=0只核對工具模型，ISA數值驗收取其餘五定義旗標。
	}
}

func TestCWDWordAllFlagBitsBoundaries(t *testing.T) {
	for bit := 0; bit < 32; bit++ {
		for _, ax := range subWordBounds359 {
			c, mem := xchgByteFixture354([]byte{0x66, 0x99})
			c.R[EAX], c.R[EDX] = 0x80000000|uint32(ax), 0xffff1357
			c.EFlags = 1 << uint(bit)
			want := snapshotInImmediate(c)
			want.r = imulWordReplace357(c.R, EDX, cwdWordDX360(ax))
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
				t.Fatalf("bit=%d AX=%X err=%v", bit, ax, err)
			}
		}
	}
}
