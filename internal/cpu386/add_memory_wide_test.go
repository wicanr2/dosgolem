package cpu386

import (
	"encoding/binary"
	"testing"
)

func TestADDMemoryWideStackAndFlags(t *testing.T) {
	for _, v := range []struct{ a, b, w, f uint32 }{{100, 2280, 2380, 0}, {0xffffffff, 1, 0, CF | AF | PF | ZF}, {0x7fffffff, 1, 0x80000000, OF | AF | PF | SF}} {
		mem := testBus(make([]byte, 128))
		code := []byte{0x81, 0x44, 0x24, 0x0c}
		code = binary.LittleEndian.AppendUint32(code, v.b)
		copy(mem, code)
		c := New(mem)
		c.Seg[SegSS] = 0x30
		c.SetDescriptor(0x30, Descriptor{Base: 0, Limit: 127, Writable: true})
		c.R[ESP] = 64
		c.EFlags = IF
		binary.LittleEndian.PutUint32(mem[76:], v.a)
		if e := c.Step(); e != nil {
			t.Fatal(e)
		}
		if binary.LittleEndian.Uint32(mem[76:]) != v.w || c.EFlags != IF|v.f {
			t.Fatalf("結果／旗標不符 %x %x", binary.LittleEndian.Uint32(mem[76:]), c.EFlags)
		}
	}
}
