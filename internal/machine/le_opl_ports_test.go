package machine

import "testing"

func TestLESecondaryDMASingleMask(t *testing.T) {
	p := NewLEOPLPorts()
	if got := p.State().SecondaryDMAMask; got != 0x0f {
		t.Fatalf("第二 DMA 初始遮罩=%02X", got)
	}
	if !p.Out8(0xd4, 1) || p.State().SecondaryDMAMask != 0x0d {
		t.Fatal("D4h 清除相對通道 1 遮罩失敗")
	}
	if !p.Out8(0xd4, 5) || p.State().SecondaryDMAMask != 0x0f {
		t.Fatal("D4h/05h 設置相對通道 1 遮罩失敗")
	}
	if !p.Out8(0xd4, 0x83) || p.State().SecondaryDMAMask != 0x07 {
		t.Fatal("D4h 高位應忽略，相對通道 3 應獨立")
	}
	if p.State().DMAMask != 0x0f || p.Writes[0xd4] != 3 {
		t.Fatal("第二 DMA 遮罩污染第一控制器，或未記錄輸出")
	}
	if _, ok := p.In8(0xd4); ok || p.Out8(0xd2, 5) {
		t.Fatal("未實作的第二 DMA 讀取／其他埠應拒絕")
	}
}

func TestLESecondaryDMARegisterProgramming(t *testing.T) {
	p := NewLEOPLPorts()
	if !p.Out8(0, 0x55) { // 第一控制器現停在高位元組。
		t.Fatal("第一 DMA 位址低位寫入失敗")
	}
	if !p.Out8(0xc4, 0xee) || !p.Out8(0xd8, 0x99) || !p.Out8(0xc4, 0x34) || !p.Out8(0xc4, 0x12) {
		t.Fatal("第二 DMA 位址與指標設定失敗")
	}
	if !p.Out8(0xc6, 0x78) || !p.Out8(0xc6, 0x56) || !p.Out8(0xd6, 0x45) {
		t.Fatal("第二 DMA 計數或 mode 設定失敗")
	}
	s := p.State()
	if s.SecondaryDMABase[2] != 0x1234 || s.SecondaryDMACurrent[2] != 0x1234 || s.SecondaryDMABase[3] != 0x5678 || s.SecondaryDMAMode[1] != 0x44 {
		t.Fatalf("第二 DMA 狀態錯誤：base=%X current=%X mode=%X", s.SecondaryDMABase, s.SecondaryDMACurrent, s.SecondaryDMAMode)
	}
	if !p.Out8(0, 0xaa) || p.State().DMABase[0] != 0xaa55 {
		t.Fatal("第二 DMA 的 D8h 不得清掉第一控制器的高位元組指標")
	}
	if !p.Out8(0xd8, 0) {
		t.Fatal("第二 DMA 位元組指標清除失敗")
	}
	if v, ok := p.In8(0xc4); !ok || v != 0x34 {
		t.Fatalf("第二 DMA 位址低位讀取=%02X ok=%t", v, ok)
	}
	if v, ok := p.In8(0xc4); !ok || v != 0x12 {
		t.Fatalf("第二 DMA 位址高位讀取=%02X ok=%t", v, ok)
	}
	if p.State().DMAMask != 0x0f || p.State().SecondaryDMAMask != 0x0f || p.Writes[0xd8] != 2 {
		t.Fatal("DMA 控制器狀態交叉污染或未記錄 D8h")
	}
	if p.Out8(0xc1, 1) || p.Out8(0xd0, 0) || p.Out8(0xd2, 0) || p.Out8(0x88, 0) {
		t.Fatal("未知第二 DMA 埠被放行")
	}
}

func TestLESecondaryDMAPageRegisters(t *testing.T) {
	p := NewLEOPLPorts()
	for _, port := range []uint16{0x8f, 0x8b, 0x89, 0x8a} {
		if _, ok := p.In8(port); ok {
			t.Fatalf("未設定的第二 DMA 頁埠 %02X 不應有讀值", port)
		}
	}
	for _, tc := range []struct {
		port uint16
		ch   int
		v    byte
	}{{0x8f, 0, 0x12}, {0x8b, 1, 0}, {0x89, 2, 0x34}, {0x8a, 3, 0x56}} {
		if !p.Out8(tc.port, tc.v) {
			t.Fatalf("第二 DMA 頁埠 %02X 寫入失敗", tc.port)
		}
		if v, ok := p.In8(tc.port); !ok || v != tc.v {
			t.Fatalf("第二 DMA 頁埠 %02X 讀回=%02X ok=%t", tc.port, v, ok)
		}
		if p.State().SecondaryDMAPage[tc.ch] != tc.v || p.Writes[tc.port] != 1 || p.Reads[tc.port] != 1 {
			t.Fatalf("第二 DMA 頁埠 %02X 狀態／紀錄錯誤", tc.port)
		}
	}
	if p.State().DMAPage != [4]byte{} || p.State().DMAMask != 0x0f {
		t.Fatal("第二 DMA 頁寫入污染第一控制器")
	}
	if p.Out8(0x88, 1) {
		t.Fatal("未知頁暫存器被放行")
	}
}

func TestLEOPLAliasesAndDetection(t *testing.T) {
	p := NewLEOPLPorts()
	write := func(reg, v uint8) {
		if !p.Out8(0x228, reg) || !p.Out8(0x389, v) {
			t.Fatal("alias拒絕")
		}
	}
	read := func(want uint8) {
		v, ok := p.In8(0x220)
		if !ok || v != want {
			t.Fatalf("狀態=%x want=%x", v, want)
		}
	}
	write(4, 0x60)
	write(4, 0x80)
	read(0)
	write(2, 0xff)
	write(4, 0x21)
	read(0xc0)
	write(4, 0x60)
	write(4, 0x80)
	read(0)
	if p.Out8(0x22c, 0xff) {
		t.Fatal("未知DSP命令不可放行")
	}
	if _, ok := p.In8(0x40); ok {
		t.Fatal("PIT尚未實作不可放行")
	}
}

func TestLEPICMasks(t *testing.T) {
	p := NewLEOPLPorts()
	read := func(port uint16, want byte) {
		t.Helper()
		v, ok := p.In8(port)
		if !ok || v != want {
			t.Fatalf("PIC %x=%x", port, v)
		}
	}
	read(0x21, 0xf8)
	read(0xa1, 0x2c)
	p.Out8(0xa1, 0xff)
	read(0xa1, 0xff)
	read(0x21, 0xf8)
	p.Out8(0x21, 0)
	read(0x21, 0)
	read(0xa1, 0xff)
	if p.Out8(0x20, 0x60) {
		t.Fatal("未支援PIC命令")
	}
}
