package machine

import "testing"

func TestPITChannel2Decode(t *testing.T) {
	m := New()
	var got []PIT2Change
	m.ObservePITChannel2(func(c PIT2Change) { got = append(got, c) })
	// 通道 0 的命令不影響通道 2。
	m.Out8(0x43, 0x36)
	m.Out8(0x42, 0x99)
	if len(got) != 0 {
		t.Fatalf("未選通道 2 卻有事件 %+v", got)
	}
	m.Out8(0x43, 0xB6) // 通道 2、低高位元組、模式 3
	m.Out8(0x42, 0x34)
	if len(got) != 0 {
		t.Fatal("只寫低位元組就發事件")
	}
	m.Out8(0x42, 0x12)
	m.Out8(0x43, 0x96) // 只寫低位元組、模式 3
	m.Out8(0x42, 0x40)
	m.Out8(0x43, 0xA6) // 只寫高位元組
	m.Out8(0x42, 0x00) // 0 ＝ 65536
	if len(got) != 3 || got[0].Divisor != 0x1234 || got[0].Mode != 3 || got[1].Divisor != 0x40 || got[2].Divisor != PITDefaultDivisor {
		t.Fatalf("事件 %+v", got)
	}
	m.Out8(0x43, 0x80) // 鎖存命令：不改設定
	if d, mode := m.PITChannel2(); d != PITDefaultDivisor || mode != 3 {
		t.Fatalf("鎖存後 %#x %d", d, mode)
	}
}

func TestSpeakerObserverDedupeWithoutSeries(t *testing.T) {
	m := New()
	var got []SpeakerSample
	m.ObserveSpeaker(func(s SpeakerSample) { got = append(got, s) })
	for _, v := range []uint8{0, 3, 3, 1, 1, 0, 0x4C} { // 0x4C：高位元不算，等於 0
		m.Out8(0x61, v)
	}
	if len(m.Speaker) != 0 {
		t.Fatal("觀測器掛上時仍追加")
	}
	if len(got) != 3 || got[0].Level != 1 || got[1].Level != 0 || got[1].Gate != 1 || got[2].Gate != 0 {
		t.Fatalf("去重錯誤 %+v", got)
	}
	m.ObserveSpeaker(nil)
	m.Out8(0x61, 0) // 與上一筆相同：不記
	m.Out8(0x61, 2)
	if len(m.Speaker) != 1 || m.Speaker[0].Level != 1 {
		t.Fatalf("解除後序列 %+v", m.Speaker)
	}
}

func TestOPLObserverStopsSeries(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	var got []OPLWrite
	m.ObserveOPLWrites(func(w OPLWrite) { got = append(got, w) })
	m.Out8(0x388, 0xB0)
	m.Out8(0x389, 0x2A)
	if len(got) != 1 || got[0].Reg != 0xB0 || got[0].Val != 0x2A || len(m.OPL) != 0 {
		t.Fatalf("%+v %d", got, len(m.OPL))
	}
	if m.OPLRegs(0)[0xB0] != 0x2A {
		t.Fatal("觀測器掛上時暫存器沒更新")
	}
}
