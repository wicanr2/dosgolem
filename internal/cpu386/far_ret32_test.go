package cpu386

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

// 稀疏匯流排可驗證完整 32 位目標與溢位邊界，不配置 4 GiB 記憶體。
type farRETBus map[uint32]byte

func (b farRETBus) Read8(addr uint32) (byte, error) {
	if v, ok := b[addr]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("未映射位址 %08X", addr)
}

func (b farRETBus) Write8(addr uint32, value byte) error {
	b[addr] = value
	return nil
}

func farRETFrame(b farRETBus, address, target uint32, selector uint16, high uint16, count int) {
	var frame [8]byte
	binary.LittleEndian.PutUint32(frame[:4], target)
	binary.LittleEndian.PutUint16(frame[4:6], selector)
	binary.LittleEndian.PutUint16(frame[6:8], high)
	for i := 0; i < count; i++ {
		b[address+uint32(i)] = frame[i]
	}
}

func farRETFixture() (*CPU, farRETBus) {
	b := farRETBus{0: 0xcb, 64: 0x90}
	c := New(b)
	c.R = [8]uint32{0x11111111, 0x22222222, 0x33333333, 0x44444444, 32, 0x55555555, 0x66666666, 0x77777777}
	c.Seg = [6]uint16{8, 0x28, 0x30, 0x38, 0x40, 0x20}
	c.EFlags = 0x1ffff // 含未被 RET 修改的旗標，VM=0。
	c.FPUControl, c.FPUStatus, c.FPUDepth = 0x37f, 0x1234, 3
	c.FPUStack = [8]float64{1, 2, 3, 4, 5, 6, 7, 8}
	c.SetDescriptor(8, Descriptor{Limit: ^uint32(0)})
	c.SetDescriptor(0x10, Descriptor{Limit: ^uint32(0)})
	c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 39}) // 唯讀、非平坦 SS。
	c.SetDescriptor(0x28, Descriptor{Base: 512, Limit: 255})
	farRETFrame(b, 160, 64, 0x10, 0xdead, 8)
	return c, b
}

func checkFarRET(t *testing.T, c *CPU, b farRETBus, success bool, target uint32, selector uint16) {
	t.Helper()
	r, seg, flags := c.R, c.Seg, c.EFlags
	control, status, depth, stack := c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack
	before := make(farRETBus, len(b))
	for k, v := range b {
		before[k] = v
	}
	err := c.Step()
	if success {
		r[ESP] += 8
		seg[SegCS] = selector
		if err != nil || c.EIP != target {
			t.Fatalf("CB 返回 EIP=%08X，預期 %08X，錯誤=%v", c.EIP, target, err)
		}
	} else if err == nil {
		t.Fatal("未核准的 CB 應拒絕")
	}
	if c.R != r || c.Seg != seg || c.EFlags != flags || c.FPUControl != control || c.FPUStatus != status || c.FPUDepth != depth || c.FPUStack != stack || !reflect.DeepEqual(b, before) {
		t.Fatalf("CB 成功／失敗均不得改寫其他狀態：R=%X 段=%X 旗標=%X", c.R, c.Seg, c.EFlags)
	}
}

func TestFarRET32SamePrivilegeAndIgnoredHighWord(t *testing.T) {
	for rpl := uint16(0); rpl < 4; rpl++ {
		for _, high := range []uint16{0, 1, 0xffff, 0xdead} {
			t.Run(fmt.Sprintf("權限%d高word%04X", rpl, high), func(t *testing.T) {
				c, b := farRETFixture()
				c.Seg[SegCS] = 8 | rpl
				c.SetDescriptor(8|rpl, Descriptor{Limit: 0}) // 只需目前段已知平坦。
				c.SetDescriptor(16|rpl, Descriptor{Limit: 64})
				farRETFrame(b, 160, 64, 16|rpl, high, 8)
				checkFarRET(t, c, b, true, 64, 16|rpl)
			})
		}
	}
}

func TestFarRET32NecessarySixBytesOnly(t *testing.T) {
	for count := 0; count <= 8; count++ {
		t.Run(fmt.Sprintf("可讀%dbytes", count), func(t *testing.T) {
			c, b := farRETFixture()
			for i := uint32(0); i < 8; i++ {
				delete(b, 160+i)
			}
			farRETFrame(b, 160, 64, 16, 0xffff, count)
			c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 37}) // SP+5 可讀即可。
			checkFarRET(t, c, b, count >= 6, 64, 16)
		})
	}
	for missing := uint32(0); missing < 6; missing++ {
		c, b := farRETFixture()
		delete(b, 160+missing)
		checkFarRET(t, c, b, false, 0, 0)
	}
}

func TestFarRET32FullTargetAndStackBoundary(t *testing.T) {
	for _, target := range []uint32{0, 1, 0x7fff, 0x8000, 0xffff, 0x10000, 0x7fffffff, 0x80000000, 0xfffffffe, 0xffffffff} {
		t.Run(fmt.Sprintf("目標%08X", target), func(t *testing.T) {
			c, b := farRETFixture()
			if target != 0 {
				b[target] = 0x90
			}
			farRETFrame(b, 160, target, 16, 0xdead, 8)
			checkFarRET(t, c, b, true, target, 16)
		})
	}
	for delta := uint32(0); delta <= 8; delta++ {
		c, b := farRETFixture()
		c.R[ESP] = ^uint32(0) - delta
		c.SetDescriptor(0x20, Descriptor{Limit: ^uint32(0)})
		if delta == 8 {
			farRETFrame(b, c.R[ESP], 64, 16, 0, 6)
		}
		checkFarRET(t, c, b, delta == 8, 64, 16)
	}
}

func TestFarRET32RejectsUnknownAndUnmodeledStateAtomically(t *testing.T) {
	for name, change := range map[string]func(*CPU, farRETBus){
		"目前CS未知":  func(c *CPU, _ farRETBus) { c.Seg[SegCS] = 0x48 },
		"目前CS非平坦": func(c *CPU, _ farRETBus) { c.SetDescriptor(8, Descriptor{Base: 1, Limit: 255}) },
		"返回CS未知":  func(_ *CPU, b farRETBus) { farRETFrame(b, 160, 64, 0x48, 0, 8) },
		"返回CS非平坦": func(c *CPU, _ farRETBus) { c.SetDescriptor(16, Descriptor{Base: 1, Limit: 255}) },
		"返回權限不同": func(c *CPU, b farRETBus) {
			c.SetDescriptor(17, Descriptor{Limit: 255})
			farRETFrame(b, 160, 64, 17, 0, 8)
		},
		"返回目標越界":  func(c *CPU, _ farRETBus) { c.SetDescriptor(16, Descriptor{Limit: 63}) },
		"返回目標不可讀": func(_ *CPU, b farRETBus) { delete(b, 64) },
		"SS未知":    func(c *CPU, _ farRETBus) { c.Seg[SegSS] = 0x48 },
		"SS範圍不足":  func(c *CPU, _ farRETBus) { c.SetDescriptor(0x20, Descriptor{Base: 128, Limit: 36}) },
		"SS線性溢位":  func(c *CPU, _ farRETBus) { c.SetDescriptor(0x20, Descriptor{Base: ^uint32(0) - 35, Limit: 255}) },
		"虛擬8086":  func(c *CPU, _ farRETBus) { c.EFlags |= 1 << 17 },
	} {
		t.Run(name, func(t *testing.T) {
			c, b := farRETFixture()
			change(c, b)
			checkFarRET(t, c, b, false, 0, 0)
		})
	}
	for null := uint16(0); null < 4; null++ {
		for _, source := range []bool{true, false} {
			c, b := farRETFixture()
			c.SetDescriptor(null, Descriptor{Limit: 255})
			if source {
				c.Seg[SegCS] = null
			} else {
				farRETFrame(b, 160, 64, null, 0, 8)
			}
			checkFarRET(t, c, b, false, 0, 0)
		}
	}
}

func TestFarRET32DoesNotUseSegmentCallbacks(t *testing.T) {
	for _, reject := range []bool{false, true} {
		c, b := farRETFixture()
		c.SegmentRead8 = func(uint16, uint32) (uint8, bool) { t.Fatal("CB 不得呼叫 SegmentRead8"); return 0, false }
		c.SegmentRead16 = func(uint16, uint32) (uint16, bool) { t.Fatal("CB 不得呼叫 SegmentRead16"); return 0, false }
		c.SegmentLoadOK = func(uint16, int) bool { t.Fatal("CB 不得呼叫 SegmentLoadOK"); return true }
		if reject {
			farRETFrame(b, 160, 64, 0x48, 0, 8)
		}
		checkFarRET(t, c, b, !reject, 64, 16)
	}
}

func TestFarRET32RejectsPrefixesAndImmediateForm(t *testing.T) {
	for _, code := range [][]byte{
		{0x66, 0xcb}, {0xf0, 0xcb}, {0xf2, 0xcb}, {0xf3, 0xcb}, {0x67, 0xcb},
		{0x26, 0xcb}, {0x2e, 0xcb}, {0x36, 0xcb}, {0x3e, 0xcb}, {0x64, 0xcb}, {0x65, 0xcb},
		{0x66, 0x66, 0xcb}, {0x2e, 0x2e, 0xcb}, {0xca, 0, 0},
	} {
		t.Run(fmt.Sprintf("編碼%X", code), func(t *testing.T) {
			c, b := farRETFixture()
			for i, value := range code {
				b[uint32(i)] = value
			}
			checkFarRET(t, c, b, false, 0, 0)
		})
	}
}

func TestFarRET32OriginalFiniteSample(t *testing.T) {
	// 規格 280 的 DOSBox-X CS:EIP 空間；目標只放可讀佔位，不執行核心入口。
	b := farRETBus{0x378e9c: 0xcb, 0xd49: 0x90}
	c := New(b)
	c.EIP, c.EFlags = 0x378e9c, 0x246
	c.R = [8]uint32{0x1a988, 0, 0x3ebc74, 0x1a988, 0x6830, 0x3ebbc0, 0x470, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0xd0}
	for _, selector := range []uint16{0x180, 0x80, 0xd0} {
		c.SetDescriptor(selector, Descriptor{Limit: ^uint32(0)})
	}
	farRETFrame(b, 0x6830, 0xd49, 0x80, 0, 8)
	checkFarRET(t, c, b, true, 0xd49, 0x80)
}
