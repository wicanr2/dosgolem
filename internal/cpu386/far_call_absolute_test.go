package cpu386

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

func farCALLFixture() (*CPU, farRETBus) {
	c, b := farRETFixture()
	c.EIP = 0
	c.EFlags = 0x1ffff
	for i, v := range []byte{0xff, 0x1d, 0x40, 0, 0, 0, 0x90} {
		b[uint32(i)] = v
	}
	c.SetDescriptor(8, Descriptor{Limit: 6})
	c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 31, Writable: true})
	c.SetDescriptor(0x28, Descriptor{Base: 512, Limit: 255})
	farRETFrame(b, 576, 64, 0x10, 0, 6)
	b[64] = 0xcb
	for i := uint32(152); i < 160; i++ {
		b[i] = 0xa5
	}
	return c, b
}
func checkFarCALL(t *testing.T, c *CPU, b farRETBus, success bool, target uint32, selector uint16) {
	t.Helper()
	r, seg, flags := c.R, c.Seg, c.EFlags
	control, status, depth, stack := c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack
	memory := make(farRETBus)
	for k, v := range b {
		memory[k] = v
	}
	oldSP, oldCS := r[ESP], seg[SegCS]
	desc := c.Descriptors[seg[SegSS]]
	err := c.Step()
	if success {
		if err != nil || c.EIP != target {
			t.Fatalf("遠CALL目的錯誤：%08X %v", c.EIP, err)
		}
		r[ESP] -= 8
		seg[SegCS] = selector
		var want [8]byte
		binary.LittleEndian.PutUint32(want[:4], 6)
		binary.LittleEndian.PutUint32(want[4:], uint32(oldCS))
		for i, v := range want {
			memory[desc.Base+oldSP-8+uint32(i)] = v
		}
	} else if err == nil {
		t.Fatal("未支援遠CALL沒有拒絕")
	}
	if c.R != r || c.Seg != seg || c.EFlags != flags || c.FPUControl != control || c.FPUStatus != status || c.FPUDepth != depth || c.FPUStack != stack || !reflect.DeepEqual(b, memory) {
		t.Fatalf("遠CALL核心／RAM改變不符：R=%X 段=%X", c.R, c.Seg)
	}
}
func TestFarCALLAbsoluteFullTargetAndCBConsumer(t *testing.T) {
	for rpl := uint16(0); rpl < 4; rpl++ {
		for _, target := range []uint32{0x40, 0x10000, 0x7fffffff, 0x80000000, 0xfffffffe, 0xffffffff} {
			t.Run(fmt.Sprintf("權限%d目的%08X", rpl, target), func(t *testing.T) {
				c, b := farCALLFixture()
				c.Seg[SegCS] = 8 | rpl
				c.SetDescriptor(8|rpl, Descriptor{Limit: 6})
				c.SetDescriptor(16|rpl, Descriptor{Limit: ^uint32(0)})
				farRETFrame(b, 576, target, 16|rpl, 0, 6)
				b[target] = 0xcb
				checkFarCALL(t, c, b, true, target, 16|rpl)
				// 執行真正CB讀取遠CALL框架，不手動改EIP／ESP。
				memory := make(farRETBus)
				for k, v := range b {
					memory[k] = v
				}
				if err := c.Step(); err != nil || c.EIP != 6 || c.R[ESP] != 32 || c.Seg[SegCS] != 8|rpl || !reflect.DeepEqual(b, memory) {
					t.Fatalf("CB返回消費失敗：%v", err)
				}
			})
		}
	}
	c, b := farCALLFixture()
	c.SetDescriptor(0x28, Descriptor{Limit: 255})
	b[2] = 152
	farRETFrame(b, 152, 64, 16, 0, 6)
	checkFarCALL(t, c, b, true, 64, 16)
}
func TestFarCALLAbsoluteRejectsBoundaries(t *testing.T) {
	changes := map[string]func(*CPU, farRETBus){
		"未知目前CS":  func(c *CPU, b farRETBus) { c.Seg[SegCS] = 0x48 },
		"目前CS非平坦": func(c *CPU, b farRETBus) { c.SetDescriptor(8, Descriptor{Base: 1, Limit: 99}) },
		"下一EIP越界": func(c *CPU, b farRETBus) { c.SetDescriptor(8, Descriptor{Limit: 5}) },
		"未知目的CS":  func(c *CPU, b farRETBus) { farRETFrame(b, 576, 64, 0x48, 0, 6) },
		"目的CS非平坦": func(c *CPU, b farRETBus) { c.SetDescriptor(16, Descriptor{Base: 1, Limit: 99}) },
		"不同權限": func(c *CPU, b farRETBus) {
			c.SetDescriptor(17, Descriptor{Limit: 99})
			farRETFrame(b, 576, 64, 17, 0, 6)
		},
		"目的越界":   func(c *CPU, b farRETBus) { c.SetDescriptor(16, Descriptor{Limit: 63}) },
		"目的不可讀":  func(c *CPU, b farRETBus) { delete(b, 64) },
		"虛擬8086": func(c *CPU, b farRETBus) { c.EFlags |= 1 << 17 },
		"未知來源":   func(c *CPU, b farRETBus) { c.Seg[SegDS] = 0x48 },
		"來源段界":   func(c *CPU, b farRETBus) { c.SetDescriptor(0x28, Descriptor{Base: 512, Limit: 68}) },
		"來源線性溢位": func(c *CPU, b farRETBus) { c.SetDescriptor(0x28, Descriptor{Base: ^uint32(0) - 65, Limit: 99}) },
		"ESP下溢":  func(c *CPU, b farRETBus) { c.R[ESP] = 7 },
		"SS未知":   func(c *CPU, b farRETBus) { c.Seg[SegSS] = 0x48 },
		"SS不可寫":  func(c *CPU, b farRETBus) { c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 31}) },
		"SS越界":   func(c *CPU, b farRETBus) { c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 30, Writable: true}) },
		"SS線性溢位": func(c *CPU, b farRETBus) {
			c.SetDescriptor(0x20, Descriptor{Base: ^uint32(0) - 25, Limit: 31, Writable: true})
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) { c, b := farCALLFixture(); change(c, b); checkFarCALL(t, c, b, false, 0, 0) })
	}
	for i := uint32(0); i < 6; i++ {
		c, b := farCALLFixture()
		delete(b, 576+i)
		checkFarCALL(t, c, b, false, 0, 0)
	}
	for i := uint32(0); i < 8; i++ {
		c, b := farCALLFixture()
		delete(b, 152+i)
		checkFarCALL(t, c, b, false, 0, 0)
	}
	for i := uint32(2); i < 6; i++ {
		c, b := farCALLFixture()
		delete(b, i)
		checkFarCALL(t, c, b, false, 0, 0)
	}
	for n := uint16(0); n < 4; n++ {
		for _, current := range []bool{true, false} {
			c, b := farCALLFixture()
			c.SetDescriptor(n, Descriptor{Limit: 99})
			if current {
				c.Seg[SegCS] = n
			} else {
				farRETFrame(b, 576, 64, n, 0, 6)
			}
			checkFarCALL(t, c, b, false, 0, 0)
		}
	}
}
func TestFarCALLAbsoluteRejectsOtherShapesAndPrefixes(t *testing.T) {
	for modrm := uint8(0); ; modrm++ {
		if modrm>>3&7 == 3 && modrm != 0x1d {
			c, b := farCALLFixture()
			b[1] = modrm
			checkFarCALL(t, c, b, false, 0, 0)
		}
		if modrm == 255 {
			break
		}
	}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3} {
		c, b := farCALLFixture()
		for i, v := range []byte{prefix, 0xff, 0x1d, 0x40, 0, 0, 0} {
			b[uint32(i)] = v
		}
		c.SetDescriptor(8, Descriptor{Limit: 7})
		checkFarCALL(t, c, b, false, 0, 0)
	}
}
