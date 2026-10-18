package mixer

import "testing"

// `docs/spec/442-state-opl-registers` §4 的契約測試。

// voice 是第 0 聲部發音中的暫存器檔：兩個運算子乘數 1、載波最大音量、快 attack，key-on 於 block 4。
func voice() (b0 [256]uint8) {
	for r, v := range map[int]uint8{
		0x01: 0x20, 0x20: 0x01, 0x23: 0x01, 0x40: 0x10, 0x43: 0x00,
		0x60: 0xF0, 0x63: 0xF0, 0x80: 0x77, 0x83: 0x77, 0xA0: 0x98, 0xB0: 0x31,
	} {
		b0[r] = v
	}
	return b0
}

func peak(s []float32) float32 {
	var p float32
	for _, v := range s {
		if v < 0 {
			v = -v
		}
		if v > p {
			p = v
		}
	}
	return p
}

func TestPrimeOPLRestoresVoice(t *testing.T) {
	m := New(cdTestSPS)
	if n := m.PrimeOPL(voice(), [256]uint8{}); n != 10 { // 0x43 為 0 不寫
		t.Fatalf("寫了 %d 筆，要 10", n)
	}
	if p := peak(m.Render(nil, 0, 4800)); p < 0.01 {
		t.Fatalf("prime 之後峰值 %v，應有聲音", p)
	}
	if p := peak(New(cdTestSPS).Render(nil, 0, 4800)); p != 0 {
		t.Fatalf("沒 prime 的峰值 %v，應靜音", p)
	}
}

func TestPrimeOPLSkipsZeroAndTimers(t *testing.T) {
	if n := New(cdTestSPS).PrimeOPL([256]uint8{}, [256]uint8{}); n != 0 {
		t.Fatalf("全 0 寫了 %d 筆", n)
	}
	var timers [256]uint8
	timers[0x02], timers[0x03], timers[0x04] = 0xFF, 0xFF, 0x21
	if n := New(cdTestSPS).PrimeOPL(timers, [256]uint8{}); n != 0 {
		t.Fatalf("計時器暫存器寫了 %d 筆", n)
	}
	var b1 [256]uint8
	b1[0x20], b1[0xB0] = 0x01, 0x31
	if n := New(cdTestSPS).PrimeOPL([256]uint8{}, b1); n != 0 {
		t.Fatalf("NEW 位元沒設，第二組寫了 %d 筆", n)
	}
	b1[0x05] = 0x01
	if n := New(cdTestSPS).PrimeOPL([256]uint8{}, b1); n != 3 {
		t.Fatalf("NEW 位元設了，第二組寫 %d 筆，要 3", n)
	}
}
