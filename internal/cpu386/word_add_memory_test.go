package cpu386

import (
	"encoding/binary"
	"testing"
)

func TestWordADDMemoryWidthAndFlags(t *testing.T) {
	for _, stack := range []bool{false, true} {
		for _, v := range []struct {
			a, b, w uint16
			f       uint32
		}{
			{12, 3, 15, PF}, {0xffff, 1, 0, CF | AF | PF | ZF}, {0x7fff, 1, 0x8000, OF | AF | PF | SF},
		} {
			mem := testBus(make([]byte, 128))
			code := []byte{0x66, 0x01, 0x02}
			seg := SegDS
			if stack {
				code = []byte{0x66, 0x01, 0x45, 0}
				seg = SegSS
			}
			copy(mem, code)
			c := New(mem)
			c.Seg[seg] = 0x30
			c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 95, Writable: true})
			c.R[EDX] = 32
			c.R[EBP] = 32
			c.R[EAX] = 0xabcd0000 | uint32(v.b)
			c.EFlags = IF
			binary.LittleEndian.PutUint16(mem[64:], v.a)
			mem[66] = 0xaa
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if binary.LittleEndian.Uint16(mem[64:]) != v.w || mem[66] != 0xaa || c.R[EAX] != 0xabcd0000|uint32(v.b) || c.EFlags != IF|v.f {
				t.Fatalf("兩位元組目的或旗標不符：%x flags=%x", mem[64:67], c.EFlags)
			}
		}
	}
}
func TestWordADDMemoryRejectDoesNotPublish(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, 0x01, 0x02})
		c := New(mem)
		c.Seg[SegDS] = 0x30
		limit := uint32(32)
		if readonly {
			limit = 95
		}
		c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: limit, Writable: !readonly})
		c.R[EDX] = 32
		c.R[EAX] = 1
		c.EFlags = IF | CF
		binary.LittleEndian.PutUint16(mem[64:], 0xffff)
		if c.Step() == nil {
			t.Fatal("未拒絕無效寫入")
		}
		if binary.LittleEndian.Uint16(mem[64:]) != 0xffff || c.EFlags != IF|CF || c.R[EAX] != 1 {
			t.Fatal("拒絕仍發布狀態")
		}
	}
}
func TestWordADDRegisterPreservesUpper(t *testing.T) {
	c := New(testBus([]byte{0x66, 0x01, 0xc0}))
	c.R[EAX] = 0xbeef8000
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0xbeef0000 || c.EFlags&(CF|OF|ZF) != (CF|OF|ZF) {
		t.Fatalf("別名結果不符：%x", c.R[EAX])
	}
}

func TestWordADD83MemorySignedAndAddresses(t *testing.T) {
	for _, form := range []struct {
		name    string
		code    []byte
		segment int
		setup   func(*CPU)
	}{
		{"actual-4C", []byte{0x66, 0x83, 0x46, 0x4c}, SegDS, func(c *CPU) { c.R[ESI] = 4 }},
		{"actual-4E", []byte{0x66, 0x83, 0x46, 0x4e}, SegDS, func(c *CPU) { c.R[ESI] = 2 }},
		{"SS-negative-disp", []byte{0x66, 0x83, 0x45, 0xf0}, SegSS, func(c *CPU) { c.R[EBP] = 96 }},
		{"SIB-stack", []byte{0x66, 0x83, 0x44, 0x8d, 0x04}, SegSS, func(c *CPU) { c.R[EBP] = 64; c.R[ECX] = 3 }},
		{"absolute-disp32", []byte{0x66, 0x83, 0x05, 0x50, 0, 0, 0}, SegDS, func(c *CPU) {}},
	} {
		for _, v := range []struct {
			name   string
			before uint16
			imm    byte
			want   uint16
			flags  uint32
		}{
			{"actual-positive", 5605, 15, 5620, AF},
			{"negative-imm8", 0, 0xff, 0xffff, SF | PF},
			{"carry-and-zero", 0xffff, 1, 0, CF | AF | ZF | PF},
			{"signed-overflow", 0x7fff, 1, 0x8000, OF | AF | SF | PF},
			{"negative-overflow", 0x8000, 0xff, 0x7fff, CF | OF | PF},
			{"aux-carry", 0x000f, 1, 0x0010, AF},
			{"zero-no-carry", 0, 0, 0, ZF | PF},
		} {
			t.Run(form.name+"/"+v.name, func(t *testing.T) {
				mem := testBus(make([]byte, 192))
				code := append(append([]byte(nil), form.code...), v.imm)
				copy(mem, code)
				c := New(mem)
				c.Seg[form.segment] = 0x30
				c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 159, Writable: true})
				form.setup(c)
				c.EFlags = IF | 0x100 | CF | PF | AF | ZF | SF | OF
				binary.LittleEndian.PutUint16(mem[112:], v.before)
				mem[111], mem[114] = 0xaa, 0xbb
				regs, segs := c.R, c.Seg
				wantMem := append([]byte(nil), mem...)
				binary.LittleEndian.PutUint16(wantMem[112:], v.want)
				if err := c.Step(); err != nil {
					t.Fatal(err)
				}
				if c.EFlags != IF|0x100|v.flags || c.EIP != uint32(len(code)) || c.R != regs || c.Seg != segs {
					t.Fatalf("flags/EIP/GPR/segment 不符：flags=%x EIP=%x", c.EFlags, c.EIP)
				}
				for i := range mem {
					if mem[i] != wantMem[i] {
						t.Fatalf("memory byte %d=%x want=%x", i, mem[i], wantMem[i])
					}
				}
			})
		}
	}
}

func TestWordADD83MemoryRejectDoesNotPublish(t *testing.T) {
	for _, name := range []string{"readonly", "short-limit", "unknown-selector", "short-bus", "truncated-imm", "truncated-disp", "repeat", "repne", "segment", "address", "lock", "double-operand"} {
		t.Run(name, func(t *testing.T) {
			mem := testBus(make([]byte, 192))
			code := []byte{0x66, 0x83, 0x46, 0x4c, 15}
			c := New(mem)
			c.R[ESI] = 4
			c.Seg[SegDS] = 0x30
			c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 159, Writable: true})
			c.EFlags = IF | 0x100 | CF | OF | AF | SF
			switch name {
			case "readonly":
				c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 159, Writable: false})
			case "short-limit":
				c.SetDescriptor(0x30, Descriptor{Base: 32, Limit: 80, Writable: true})
			case "unknown-selector":
				c.Seg[SegDS] = 0x38
			case "short-bus":
				c.R[ESI] = 83
			case "truncated-imm":
				c.EIP = 188
				code = code[:4]
			case "truncated-disp":
				c.EIP = 187
				code = []byte{0x66, 0x83, 0x86, 0x4c, 0}
			case "repeat":
				code = append([]byte{0xf3}, code...)
			case "repne":
				code = append([]byte{0xf2}, code...)
			case "segment":
				code = append([]byte{0x26}, code...)
			case "address":
				code = append([]byte{0x67}, code...)
			case "lock":
				code = append([]byte{0xf0}, code...)
			case "double-operand":
				code = append([]byte{0x66}, code...)
			}
			copy(mem[c.EIP:], code)
			binary.LittleEndian.PutUint16(mem[112:], 0xffff)
			before := append([]byte(nil), mem...)
			regs, segs, flags := c.R, c.Seg, c.EFlags
			if c.Step() == nil {
				t.Fatal("未拒絕不支援／越界形狀")
			}
			if c.R != regs || c.Seg != segs || c.EFlags != flags {
				t.Fatal("失敗後發布 GPR／segment／flags")
			}
			for i := range mem {
				if mem[i] != before[i] {
					t.Fatalf("失敗後改寫 byte %d", i)
				}
			}
		})
	}
}
