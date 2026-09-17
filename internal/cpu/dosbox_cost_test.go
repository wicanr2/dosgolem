package cpu

import "testing"

// `docs/spec/198` §4 第 1 項：DOSBox 計費。每道指令 1 個 cycle，字串指令每次迭代再 1 個，其他不計。
//
// 反向對照（第 7 項）：把 stringOnce 裡的 DOSBox 計費拿掉，movsb 與 rep movsb 兩列失敗。
func TestDOSBoxCost(t *testing.T) {
	for _, tc := range []struct {
		name string
		code []byte
		cx   uint16
		want uint64
	}{
		{"mov ax,bx", []byte{0x89, 0xD8}, 0, 1},
		{"movsb", []byte{0xA4}, 0, 2},
		{"rep movsb（CX=5）", []byte{0xF3, 0xA4}, 5, 6},
		{"rep movsb（CX=0）", []byte{0xF3, 0xA4}, 0, 1},
		{"out dx,al", []byte{0xEE}, 0, 1},
		{"es: mov ax,[0]", []byte{0x26, 0xA1, 0x00, 0x00}, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(newTestBus())
			c.DOSBoxCost = true
			c.Seg[CS], c.Seg[DS], c.Seg[ES], c.Seg[SS] = 0x2000, 0x3000, 0x4000, 0x2000
			c.IP, c.R[SP], c.R[CX] = 0, 0x1000, tc.cx
			for i, v := range tc.code {
				c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
			}
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.Cycles != tc.want {
				t.Errorf("%X：%d 個 cycle，要 %d", tc.code, c.Cycles, tc.want)
			}
		})
	}
}

// 關著時照舊按類別計費（不是 DOSBox 的 1）。
func TestDOSBoxCostOffKeepsCategoryCost(t *testing.T) {
	c := New(newTestBus())
	c.Seg[CS] = 0x2000
	c.Bus.Write8(Addr(0x2000, 0), 0xA4) // movsb
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Cycles != cycBase+cycString {
		t.Errorf("movsb %d 個週期，要 %d", c.Cycles, cycBase+cycString)
	}
}
