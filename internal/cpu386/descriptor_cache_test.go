package cpu386

import "testing"

type nopBus struct{ mem []byte }

func (b *nopBus) Read8(a uint32) (uint8, error) {
	if int(a) >= len(b.mem) {
		return 0, nil
	}
	return b.mem[a], nil
}
func (b *nopBus) Write8(a uint32, v uint8) error {
	if int(a) < len(b.mem) {
		b.mem[a] = v
	}
	return nil
}
func (b *nopBus) In8(uint16) (uint8, error)   { return 0, nil }
func (b *nopBus) Out8(uint16, uint8) error    { return nil }
func (b *nopBus) In16(uint16) (uint16, error) { return 0, nil }
func (b *nopBus) Out16(uint16, uint16) error  { return nil }

// selector 解析走一層快取，快取只在 SetDescriptor 整份失效。這支測試釘住三件
// 會安靜出錯的事：改寫後不得讀到舊 base、移除後不得仍然解析成功、以及查不到
// 的 selector 不得被記成否定結果擋住之後的註冊。
func TestDescriptorCacheFollowsSetDescriptor(t *testing.T) {
	c := New(&nopBus{mem: make([]byte, 0x2000)})
	const sel = 0x10

	// 尚未註冊：解析必須失敗，且不得留下否定快取。
	if _, ok := c.segmentLinear(sel, 0, 1, false); ok {
		t.Fatal("未註冊的 selector 不應解析成功")
	}

	c.SetDescriptor(sel, Descriptor{Base: 0x100, Limit: 0xff, Writable: true})
	linear, ok := c.segmentLinear(sel, 4, 1, false)
	if !ok || linear != 0x104 {
		t.Fatalf("首次解析 linear=%#x ok=%v，預期 0x104", linear, ok)
	}
	// 命中快取的第二次必須給出同一個結果。
	if linear, ok = c.segmentLinear(sel, 4, 1, false); !ok || linear != 0x104 {
		t.Fatalf("快取命中 linear=%#x ok=%v", linear, ok)
	}

	// 改寫 base：不得讀到舊值。
	c.SetDescriptor(sel, Descriptor{Base: 0x900, Limit: 0xff, Writable: true})
	if linear, ok = c.segmentLinear(sel, 4, 1, false); !ok || linear != 0x904 {
		t.Fatalf("改寫 base 後 linear=%#x ok=%v，預期 0x904", linear, ok)
	}

	// 收掉可寫旗標：寫入必須被拒絕。
	c.SetDescriptor(sel, Descriptor{Base: 0x900, Limit: 0xff})
	if _, ok = c.segmentLinear(sel, 4, 1, true); ok {
		t.Fatal("唯讀 descriptor 仍允許寫入")
	}

	// 縮小 limit：超界必須被拒絕。
	c.SetDescriptor(sel, Descriptor{Base: 0x900, Limit: 1, Writable: true})
	if _, ok = c.segmentLinear(sel, 4, 1, false); ok {
		t.Fatal("縮小 limit 後仍允許超界存取")
	}

	// 撞號的兩個 selector 必須各自解析，不可互相污染。
	other := uint16(sel + descCacheEntries*8)
	c.SetDescriptor(other, Descriptor{Base: 0x2000, Limit: 0xff, Writable: true})
	c.SetDescriptor(sel, Descriptor{Base: 0x900, Limit: 0xff, Writable: true})
	if linear, ok = c.segmentLinear(other, 0, 1, false); !ok || linear != 0x2000 {
		t.Fatalf("撞號 selector %#x linear=%#x ok=%v，預期 0x2000", other, linear, ok)
	}
	if linear, ok = c.segmentLinear(sel, 0, 1, false); !ok || linear != 0x900 {
		t.Fatalf("撞號後原 selector linear=%#x ok=%v，預期 0x900", linear, ok)
	}
}
