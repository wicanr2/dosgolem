package machine

import "testing"

func TestPIT0Mode3Configuration(t *testing.T) {
	p := LEPIT0{}
	if p.Out8(0x40, 1) || p.Out8(0x43, 0x34) {
		t.Fatal("未知或未設定")
	}
	if !p.Out8(0x43, 0x36) || !p.Out8(0x40, 0x34) || p.Loaded || !p.Out8(0x40, 0x12) || p.Reload != 0x1234 || !p.Loaded {
		t.Fatal("低高位重載")
	}
	p.Out8(0x40, 0x99)
	p.Out8(0x43, 0x36)
	p.Out8(0x40, 0)
	p.Out8(0x40, 0)
	if p.Reload != 65536 || !p.Loaded {
		t.Fatal("控制字清半筆與零重載")
	}
	if p.Out8(0x41, 0) || p.Out8(0x42, 0) {
		t.Fatal("其他通道")
	}
}
