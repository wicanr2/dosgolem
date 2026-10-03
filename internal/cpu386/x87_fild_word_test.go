package cpu386

import (
	"bytes"
	"testing"
)

func TestFILD16OriginalStackAddressAndWidth(t *testing.T) {
	mem, c := newFPU(t, []byte{0xdf, 0x44, 0x24, 0x34})
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: 69})
	c.R[ESP] = 16
	mem[196] = 0xf9
	mem[197] = 0xff // -7。SS只允許這兩bytes。
	mem[68] = 99
	mem[69] = 0 // DS反對照。
	mem[198] = 0
	mem[199] = 0x80
	c.FPUStack = [8]float64{0.5, 7, 9}
	c.FPUDepth = 3
	c.FPUControl = 0x127f
	c.FPUStatus = 0x1234
	c.EFlags = IF | DF | CF | 2
	source := append([]byte(nil), mem...)
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 4 || c.FPUStack != [8]float64{-7, 0.5, 7, 9} || c.FPUDepth != 4 || c.FPUControl != 0x127f || c.FPUStatus != 0x1234 || c.EFlags != IF|DF|CF|2 || !bytes.Equal(mem, source) {
		t.Fatalf("EIP=%d stack=%v depth=%d", c.EIP, c.FPUStack, c.FPUDepth)
	}
}

func TestFILD16SignedValuesAndEmptyStack(t *testing.T) {
	for _, tc := range []struct {
		raw  uint16
		want float64
	}{{0x8000, -32768}, {0xffff, -1}, {0, 0}, {0x7fff, 32767}} {
		mem, c := newFPU(t, []byte{0xdf, 0x06})
		mem[100] = byte(tc.raw)
		mem[101] = byte(tc.raw >> 8)
		mem[102] = 0xab
		mem[103] = 0xcd
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.FPUDepth != 1 || c.FPUStack[0] != tc.want || c.EIP != 2 {
			t.Fatalf("raw=%x value=%v depth=%d", tc.raw, c.FPUStack[0], c.FPUDepth)
		}
	}
}

func TestFILD16MultiplyAddConsumerChain(t *testing.T) {
	mem, c := newFPU(t, []byte{0xdf, 0x44, 0x24, 0x34, 0xde, 0xc9, 0x66, 0x0f, 0xb6, 0x44, 0x24, 0x38, 0x89, 0x44, 0x24, 0x34, 0xdf, 0x44, 0x24, 0x34, 0xde, 0xc1})
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: 127, Writable: true})
	c.R[ESP] = 16
	mem[196] = 0xf9
	mem[197] = 0xff
	mem[200] = 8
	c.R[EAX] = 0xdead0000
	c.FPUStack = [8]float64{0.5, 7}
	c.FPUDepth = 2
	for i := 0; i < 6; i++ {
		if err := c.Step(); err != nil {
			t.Fatalf("step%d: %v", i, err)
		}
	}
	if c.EIP != 22 || c.FPUDepth != 2 || c.FPUStack[0] != 4.5 || c.FPUStack[1] != 7 || c.R[EAX] != 0xdead0008 {
		t.Fatalf("EIP=%d stack=%v depth=%d eax=%x", c.EIP, c.FPUStack, c.FPUDepth, c.R[EAX])
	}
}

func TestFILD16RejectsWithoutPublishing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		program []byte
		setup   func(*CPU)
	}{
		{"滿堆疊", []byte{0xdf, 0x06}, func(c *CPU) { c.FPUDepth = 8 }},
		{"段尾", []byte{0xdf, 0x06}, func(c *CPU) { c.SetDescriptor(0x28, Descriptor{Limit: 100}) }},
		{"實體尾", []byte{0xdf, 0x06}, func(c *CPU) { c.SetDescriptor(0x28, Descriptor{Base: 155, Limit: 255}) }},
		{"未知DF", []byte{0xdf, 0x2e}, func(c *CPU) {}},
		{"暫存器", []byte{0xdf, 0xc0}, func(c *CPU) {}},
		{"operand16", []byte{0x66, 0xdf, 0x06}, func(c *CPU) {}},
		{"段覆寫", []byte{0x26, 0xdf, 0x06}, func(c *CPU) {}},
		{"REP", []byte{0xf3, 0xdf, 0x06}, func(c *CPU) {}},
		{"REPNE", []byte{0xf2, 0xdf, 0x06}, func(c *CPU) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, tc.program)
			c.FPUStack = [8]float64{1, 2, 3, 4, 5, 6, 7, 8}
			c.FPUDepth = 2
			c.FPUStatus = 0x1234
			tc.setup(c)
			before := *c
			source := append([]byte(nil), mem...)
			if err := c.Step(); err == nil {
				t.Fatal("未拒絕")
			}
			if c.FPUStack != before.FPUStack || c.FPUDepth != before.FPUDepth || c.FPUStatus != before.FPUStatus || c.FPUControl != before.FPUControl || c.EFlags != before.EFlags || c.R != before.R || !bytes.Equal(mem, source) {
				t.Fatal("拒絕後發布狀態")
			}
		})
	}
	for _, program := range [][]byte{{0xdf}, {0xdf, 0x44}, {0xdf, 0x44, 0x24}, {0xdf, 0x05, 100, 0, 0}} {
		c := New(testBus(append([]byte(nil), program...)))
		c.FPUStack[0] = 7
		c.FPUDepth = 1
		if err := c.Step(); err == nil || c.FPUStack[0] != 7 || c.FPUDepth != 1 {
			t.Fatalf("截斷 %x err=%v", program, err)
		}
	}
}
