package cpu386

import (
	"bytes"
	"math"
	"testing"
)

func TestFMULSingleOriginalStackAddress(t *testing.T) {
	mem, c := newFPU(t, []byte{0xd8, 0x4c, 0x24, 0x30})
	c.Seg[SegSS] = 0x30
	c.SetDescriptor(0x30, Descriptor{Base: 128, Limit: 127, Writable: true})
	c.R[ESP] = 16
	put32(mem, 192, math.Float32bits(-1.5))
	put32(mem, 64, math.Float32bits(99)) // 同偏移的DS反對照。
	c.FPUStack = [8]float64{8, 7, 6}
	c.FPUDepth = 3
	c.FPUControl = 0x127f
	c.FPUStatus = 0x1234
	c.EFlags = IF | DF | CF | OF | 2
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 4 || c.FPUStack != [8]float64{-12, 7, 6} || c.FPUDepth != 3 {
		t.Fatalf("EIP=%d stack=%v depth=%d", c.EIP, c.FPUStack, c.FPUDepth)
	}
	if c.FPUControl != 0x127f || c.FPUStatus != 0x1234 || c.EFlags != IF|DF|CF|OF|2 || !bytes.Equal(mem, before) {
		t.Fatal("乘法改寫了控制狀態或來源記憶體")
	}
}

func TestFMULSingleAddressForms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		program []byte
		setup   func(*CPU)
		at      int
	}{
		{"DS", []byte{0xd8, 0x0e}, func(c *CPU) { c.R[ESI] = 100 }, 100},
		{"負disp8", []byte{0xd8, 0x4e, 0xfc}, func(c *CPU) { c.R[ESI] = 104 }, 100},
		{"SIB縮放", []byte{0xd8, 0x4c, 0x8b, 0xfc}, func(c *CPU) { c.R[EBX] = 96; c.R[ECX] = 2 }, 100},
		{"disp32無基址", []byte{0xd8, 0x0d, 100, 0, 0, 0}, func(c *CPU) {}, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, tc.program)
			tc.setup(c)
			put32(mem, tc.at, math.Float32bits(1.25))
			c.FPUStack[0] = -2
			c.FPUDepth = 1
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.FPUStack[0] != -2.5 || c.FPUDepth != 1 || c.EIP != uint32(len(tc.program)) {
				t.Fatalf("stack=%v depth=%d EIP=%d", c.FPUStack, c.FPUDepth, c.EIP)
			}
		})
	}
}

func TestFMULSingleNumericValues(t *testing.T) {
	for _, tc := range []struct {
		name        string
		bits        uint32
		start, want float64
	}{
		{"分數", 0x3fc00000, 3, 4.5},
		{"負值", 0xbfa00000, 2, -2.5},
		{"負零", 0x80000000, 2, math.Copysign(0, -1)},
		{"最小非正規", 1, 2, math.Ldexp(1, -148)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, []byte{0xd8, 0x0e})
			put32(mem, 100, tc.bits)
			c.FPUStack[0] = tc.start
			c.FPUDepth = 1
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if math.Float64bits(c.FPUStack[0]) != math.Float64bits(tc.want) {
				t.Fatalf("got %v want %v", c.FPUStack[0], tc.want)
			}
		})
	}
}

func TestFMULSingleRejectsWithoutPublishing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		program []byte
		setup   func(*CPU)
	}{
		{"空堆疊", []byte{0xd8, 0x0e}, func(c *CPU) { c.FPUDepth = 0 }},
		{"其他群組", []byte{0xd8, 0x06}, func(c *CPU) {}},
		{"暫存器", []byte{0xd8, 0xc9}, func(c *CPU) {}},
		{"operand16", []byte{0x66, 0xd8, 0x0e}, func(c *CPU) {}},
		{"段覆寫", []byte{0x26, 0xd8, 0x0e}, func(c *CPU) {}},
		{"repeat", []byte{0xf3, 0xd8, 0x0e}, func(c *CPU) {}},
		{"段尾", []byte{0xd8, 0x0e}, func(c *CPU) { c.SetDescriptor(0x28, Descriptor{Limit: 102}) }},
		{"實體尾", []byte{0xd8, 0x0e}, func(c *CPU) { c.SetDescriptor(0x28, Descriptor{Base: 154, Limit: 255}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, tc.program)
			c.FPUStack = [8]float64{3, 5, 7}
			c.FPUDepth = 3
			c.FPUStatus = 0x1234
			tc.setup(c)
			before := *c
			source := append([]byte(nil), mem...)
			if err := c.Step(); err == nil {
				t.Fatal("未拒絕")
			}
			if c.FPUStack != before.FPUStack || c.FPUDepth != before.FPUDepth || c.FPUControl != before.FPUControl || c.FPUStatus != before.FPUStatus || c.EFlags != before.EFlags || !bytes.Equal(mem, source) {
				t.Fatal("拒絕後發布了狀態")
			}
		})
	}
	for _, program := range [][]byte{{0xd8}, {0xd8, 0x4c}, {0xd8, 0x4c, 0x24}, {0xd8, 0x0d, 100, 0, 0}} {
		mem := testBus(append([]byte(nil), program...))
		c := New(mem)
		c.FPUStack[0] = 3
		c.FPUDepth = 1
		if err := c.Step(); err == nil || c.FPUStack[0] != 3 || c.FPUDepth != 1 {
			t.Fatalf("截斷指令 %x err=%v", program, err)
		}
	}
}
