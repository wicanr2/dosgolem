package cpu386

import (
	"encoding/binary"
	"testing"
)

func TestADDMemoryImmediateSignedAndSegments(t *testing.T) {
	for _, v := range []struct {
		before      uint32
		imm         byte
		want, flags uint32
	}{
		{100, 4, 104, 0}, {0, 0xff, 0xffffffff, SF | PF}, {0xffffffff, 1, 0, CF | PF | AF | ZF}, {0x7fffffff, 1, 0x80000000, OF | SF | AF | PF},
	} {
		for _, stack := range []bool{false, true} {
			mem := testBus(make([]byte, 128))
			copy(mem, []byte{0x83, 0x00, v.imm})
			c := New(mem)
			c.R[EAX] = 12
			c.Seg[SegDS] = 0x28
			c.Seg[SegSS] = 0x30
			c.EFlags = IF
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
			c.SetDescriptor(0x30, Descriptor{Base: 64, Limit: 31, Writable: true})
			if stack {
				copy(mem, []byte{0x83, 0x45, 0xfc, v.imm})
				c.R[EBP] = 16
			}
			binary.LittleEndian.PutUint32(mem[76:80], v.before)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if binary.LittleEndian.Uint32(mem[76:80]) != v.want || c.EFlags != IF|v.flags {
				t.Fatalf("結果或旗標不符：%x %x", mem[76:80], c.EFlags)
			}
			c.EIP = 0
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 13, Writable: true})
			c.SetDescriptor(0x30, Descriptor{Base: 64, Limit: 13, Writable: true})
			before := append([]byte(nil), mem...)
			flags := c.EFlags
			if err := c.Step(); err == nil {
				t.Fatal("未拒絕越界")
			}
			for i := range mem {
				if mem[i] != before[i] {
					t.Fatal("失敗後改寫記憶體")
				}
			}
			if c.EFlags != flags {
				t.Fatal("失敗後改寫旗標")
			}
		}
	}
}
