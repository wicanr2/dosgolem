package machine

import "testing"

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
