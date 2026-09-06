package cpu

import "testing"

// `PUSH SP` 是 8086 與 186 以上唯一一道結果不同的 push：
// 8086 推「已經減 2 之後」的 SP，186 以上推舊值（`docs/spec/002` §4 第 1 點）。
//
// 為什麼值得一支專門的測試：編譯器用 `sub sp, n` ＋ `push sp` 在堆疊上開
// 暫時物件，再拿推上去的那個值當它的位址。差 2 的指標讓物件整個錯開一個
// word，**不會當掉，只會讓第二個欄位讀到堆疊殘值**——症狀離成因很遠。
func TestPushSPByModel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model Model
		want  uint16 // 推進去的那個 word
	}{
		{"8086 推減 2 之後的值", Model8086, 0x0FFE},
		{"186 推舊值", Model80186, 0x1000},
		{"386 推舊值", Model80386, 0x1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, code := range [][]byte{{0x54}, {0xFF, 0xF4}} { // PUSH SP／PUSH r/m16 /6
				b := newTestBus()
				c := New(b)
				c.Model = tc.model
				c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x2000, 0x2000, 0x2000
				c.IP, c.R[SP] = 0, 0x1000
				for i, v := range code {
					c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
				}
				if err := c.Step(); err != nil {
					t.Fatal(err)
				}
				if c.R[SP] != 0x0FFE {
					t.Fatalf("%X：SP=%04X，預期 0FFE", code, c.R[SP])
				}
				got := uint16(b.Read8(Addr(c.Seg[SS], 0x0FFE))) |
					uint16(b.Read8(Addr(c.Seg[SS], 0x0FFF)))<<8
				if got != tc.want {
					t.Errorf("%X：推進去 %04X，預期 %04X", code, got, tc.want)
				}
			}
		})
	}
}

// 編譯器配暫時物件的完整形狀：`sub sp,4` ＋ `push sp` 之後，推上去的那個
// 值必須指到那四個位元組的開頭。8086 會少 2，186 以上剛好。
func TestSubSPThenPushSPPointsAtTheBlock(t *testing.T) {
	b := newTestBus()
	c := New(b)
	c.Model = Model80386
	c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x3000, 0x3000, 0x3000
	c.IP, c.R[SP] = 0, 0x0100
	for i, v := range []byte{0x83, 0xEC, 0x04, 0x54} { // sub sp,4 / push sp
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	if err := c.Step(); err != nil { // sub sp,4
		t.Fatal(err)
	}
	block := c.R[SP] // 0x00FC，四個位元組的開頭
	if err := c.Step(); err != nil { // push sp
		t.Fatal(err)
	}
	got := uint16(b.Read8(Addr(c.Seg[SS], c.R[SP]))) |
		uint16(b.Read8(Addr(c.Seg[SS], c.R[SP]+1)))<<8
	if got != block {
		t.Errorf("推上去的指標是 %04X，暫時物件在 %04X——差 %d 個 byte，"+
			"物件的第二個欄位會讀到殘值", got, block, int(block)-int(got))
	}
}
