package machine

import "testing"

func TestDMAProgrammingAndSharedFlipFlop(t *testing.T) {
	d := NewDMA8237()
	if _, ok := d.In8(2); ok {
		t.Fatal("未初始化讀取被接受")
	}
	d.Out8(0xc, 0)
	d.Out8(2, 0x34)
	d.Out8(2, 0x12)
	d.Out8(3, 0xff)
	d.Out8(3, 0x01)
	if d.Base[2] != 0x1234 || d.Base[3] != 0x1ff {
		t.Fatal("低高位設定")
	}
	for _, tc := range []struct {
		p uint16
		v byte
	}{{2, 0x34}, {3, 1}} {
		v, ok := d.In8(tc.p)
		if !ok || v != tc.v {
			t.Fatal("共享位元組phase")
		}
	}
	d.Out8(0xc, 0)
	v, _ := d.In8(3)
	if v != 0xff {
		t.Fatal("清phase")
	}
	d.Out8(0xe, 0)
	d.Out8(0xa, 5)
	if d.Mask != 2 {
		t.Fatal("單通道遮罩")
	}
	d.Out8(0xb, 0x49)
	if d.Mode[1] != 0x48 {
		t.Fatal("通道模式")
	}
	d.Out8(0xd, 0)
	if d.Mask != 15 || d.high || d.Current[2] != 0x1234 {
		t.Fatal("主清除")
	}
	if d.Out8(9, 5) {
		t.Fatal("DMA請求尚未實作")
	}
}

func TestDMAPagesPreservePhase(t *testing.T) {
	d := NewDMA8237()
	d.Out8(2, 0x34)
	for ch, p := range []uint16{0x87, 0x83, 0x81, 0x82} {
		if _, ok := d.In8(p); ok {
			t.Fatal("未知page被讀取")
		}
		d.Out8(p, byte(ch+7))
		v, ok := d.In8(p)
		if !ok || v != byte(ch+7) || d.Page[ch] != v || !d.high {
			t.Fatal("page或phase")
		}
	}
	d.Out8(2, 0x12)
	if d.Current[2] != 0x1234 {
		t.Fatal("page污染位址phase")
	}
}
