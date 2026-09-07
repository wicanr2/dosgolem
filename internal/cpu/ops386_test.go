package cpu

import "testing"

// 386 子集（`docs/spec/012`）的釘死測試。每一道的語意都從
// DOSJP.COM 的實際用法來；語料路徑（Model8086）不受影響。

// new386 造一顆 80386 model 的 CPU，接一塊平坦記憶體。
func new386() (*CPU, *testBus) {
	b := newTestBus()
	c := New(b)
	c.Model = Model80386
	c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x1000, 0x1000, 0x1000
	c.IP = 0
	return c, b
}

func run386(t *testing.T, c *CPU, code []byte) {
	t.Helper()
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
}

func TestCWDE(t *testing.T) {
	c, _ := new386()
	c.R[AX] = 0x8001
	run386(t, c, []byte{0x66, 0x98})
	if got := c.getEAX(); got != 0xFFFF8001 {
		t.Errorf("CWDE 後 EAX=%08X，預期 FFFF8001", got)
	}
	c.R[AX] = 0x0007
	c.EAXHi = 0x1234
	run386(t, c, []byte{0x66, 0x98})
	if got := c.getEAX(); got != 0x00000007 {
		t.Errorf("CWDE 後 EAX=%08X，預期 00000007", got)
	}
}

func TestSHLEAX(t *testing.T) {
	c, _ := new386()
	// DOSJP 的用法：JIS 碼左移 4 變 XMS 位移。
	c.R[AX] = 0x253F
	c.EAXHi = 0
	run386(t, c, []byte{0x66, 0xC1, 0xE0, 0x04})
	if got := c.getEAX(); got != 0x000253F0 {
		t.Errorf("SHL EAX,4 後 EAX=%08X，預期 000253F0", got)
	}
	if c.Flags&CF != 0 {
		t.Error("移出的位元是 0，CF 不該立")
	}
	c.setEAX(0xF0000000)
	run386(t, c, []byte{0x66, 0xC1, 0xE0, 0x04})
	if c.Flags&CF == 0 {
		t.Error("F0000000 << 4 移出 1，CF 要立")
	}
}

func TestMoveAXMoffs32(t *testing.T) {
	c, _ := new386()
	// MOV CS:[0200h], EAX 然後 MOV EAX, CS:[0200h]。
	c.setEAX(0x12345678)
	run386(t, c, []byte{0x66, 0x2E, 0xA3, 0x00, 0x02})
	c.setEAX(0)
	run386(t, c, []byte{0x66, 0x2E, 0xA1, 0x00, 0x02})
	if got := c.getEAX(); got != 0x12345678 {
		t.Errorf("讀回 EAX=%08X，預期 12345678", got)
	}
}

func TestMovDwordImm32(t *testing.T) {
	c, _ := new386()
	// MOV dword [011Ah], 00000020h——DOSJP 填 XMS 描述子的那一道。
	run386(t, c, []byte{0x66, 0xC7, 0x06, 0x1A, 0x01, 0x20, 0x00, 0x00, 0x00})
	at := Addr(0x1000, 0x11A)
	if got := c.Bus.Read8(at); got != 0x20 {
		t.Errorf("[011A]=%02X", got)
	}
	if got := c.Bus.Read8(at + 3); got != 0x00 {
		t.Errorf("[011D]=%02X", got)
	}
}

// TestWriteAXKeepsEAXHi 釘住「16 位元寫 AX 不清 EAX 高半」。
// 清了的話 DOSJP 算好的 XMS 位移會在下一個 mov ax 之後歸零，
// 字型全部搬到位元組 0——不會報錯，只會顯示垃圾。
func TestWriteAXKeepsEAXHi(t *testing.T) {
	c, _ := new386()
	c.EAXHi = 0xBEEF
	run386(t, c, []byte{0xB8, 0x34, 0x12}) // mov ax,1234h
	if got := c.getEAX(); got != 0xBEEF1234 {
		t.Errorf("mov ax 之後 EAX=%08X，高半被清了", got)
	}
}

// Test66GateKeeps8086CorpusPath 釘住 model gate：
// 8086 上 0x66 是 76h（JBE）的別名，不是前綴。
func Test66GateKeeps8086CorpusPath(t *testing.T) {
	b := newTestBus()
	c := New(b) // 預設 Model8086
	c.Seg[CS], c.IP = 0x1000, 0
	// 66 02 = JBE +2；ZF 立起來讓它跳。
	c.SetFlags(c.Flags | ZF)
	if err := func() error {
		c.Bus.Write8(Addr(0x1000, 0), 0x66)
		c.Bus.Write8(Addr(0x1000, 1), 0x02)
		return c.Step()
	}(); err != nil {
		t.Fatal(err)
	}
	if c.IP != 4 {
		t.Errorf("8086 上 66 02 應該是 JBE +2（IP=4），得到 IP=%d", c.IP)
	}
}
