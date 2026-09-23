package cpu

import "testing"

// x87 探測語意（`docs/spec/202`）：`FNINIT` 設控制字 `037Fh`，
// `FNSTCW m16` 存回去。Watcom 6.5 的 `WCC`／`WCG` 啟動碼就靠
// `fninit；fnstcw；cmp 高位, 03h` 判有沒有 8087；回不出來就走
// 缺席路，chained compile 以 E142 收場（見該 spec §2）。
//
// 只有探測：算術一律不做。`FNSTCW` 之前沒 `FNINIT` 就存出初值 0——
// 跟以前「只做記憶體讀取」的行為一致（讀出來是零，寫回去的也是零）。
func TestFPUProbeFNINITFNSTCW(t *testing.T) {
	b := newTestBus()
	c := New(b)
	c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x2000, 0x2000, 0x2000
	c.IP, c.R[SP] = 0, 0x1000
	code := []byte{0xDB, 0xE3, 0xD9, 0x3E, 0x00, 0x03} // FNINIT；FNSTCW [0300h]
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	for range code {
		// 逐道跑：第一道是 FNINIT，第二道是 FNSTCW，
		// 後面幾步只是把剩下的位元組走完，不影響結果。
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		// 跑完兩道就夠，後面是垃圾位元組，停在這裡。
		if c.IP >= 6 {
			break
		}
	}
	lo := b.Read8(Addr(c.Seg[DS], 0x0300))
	hi := b.Read8(Addr(c.Seg[DS], 0x0301))
	if lo != 0x7F || hi != 0x03 {
		t.Errorf("FNSTCW 存下 %02X%02X，預期 037F", hi, lo)
	}
	if c.FPUCW != 0x037F {
		t.Errorf("FPUCW＝%04X，預期 037F", c.FPUCW)
	}
}

// 沒 FNINIT 就 FNSTCW：存出初值 0，跟舊行為（讀出來是零）一致。
func TestFPUProbeFNSTCWWithoutINIT(t *testing.T) {
	b := newTestBus()
	c := New(b)
	c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x2000, 0x2000, 0x2000
	c.IP, c.R[SP] = 0, 0x1000
	code := []byte{0xD9, 0x3E, 0x00, 0x03} // FNSTCW [0300h]
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	lo := b.Read8(Addr(c.Seg[DS], 0x0300))
	hi := b.Read8(Addr(c.Seg[DS], 0x0301))
	if lo != 0x00 || hi != 0x00 {
		t.Errorf("未初始化就 FNSTCW 存下 %02X%02X，預期 0000", hi, lo)
	}
}

// 非探測的 ESC 維持舊行為：只做記憶體讀取，不碰控制字。
func TestESCNonProbeUnchanged(t *testing.T) {
	b := newTestBus()
	c := New(b)
	c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x2000, 0x2000, 0x2000
	c.IP, c.R[SP] = 0, 0x1000
	c.Bus.Write8(Addr(c.Seg[DS], 0x0300), 0x34)
	c.Bus.Write8(Addr(c.Seg[DS], 0x0301), 0x12)
	code := []byte{0xD8, 0x06, 0x00, 0x03} // FADD m32（D8 /0，非探測）
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.FPUCW != 0x0000 {
		t.Errorf("非探測 ESC 改了 FPUCW＝%04X，預期 0000", c.FPUCW)
	}
	lo := b.Read8(Addr(c.Seg[DS], 0x0300))
	hi := b.Read8(Addr(c.Seg[DS], 0x0301))
	if lo != 0x34 || hi != 0x12 {
		t.Errorf("非探測 ESC 改了記憶體＝%02X%02X，預期 1234", hi, lo)
	}
}
