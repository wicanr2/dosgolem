package machine

import "testing"

func TestDSPResetHandshake(t *testing.T) {
	var s SoundBlasterDSP
	read := func(p uint16, w uint8) {
		t.Helper()
		v, ok := s.In8(p)
		if !ok || v != w {
			t.Fatalf("port %x=%x want %x", p, v, w)
		}
	}
	if !s.Out8(0x226, 0) {
		t.Fatal("reset low")
	}
	read(0x22e, 0)
	s.Out8(0x226, 1)
	read(0x22c, 0x80)
	read(0x22e, 0)
	s.Out8(0x226, 0)
	read(0x226, 0xff)
	read(0x22e, 0x80)
	read(0x22a, 0xaa)
	read(0x22e, 0)
	s.Out8(0x226, 0)
	read(0x22e, 0)
	s.Out8(0x226, 1)
	s.Out8(0x226, 0)
	s.Out8(0x226, 1)
	read(0x22e, 0)
	if s.Out8(0x22c, 0xff) || s.Out8(0x226, 2) {
		t.Fatal("未知命令被接受")
	}
}

func TestDSPB0ExactCommandAndIRQSource(t *testing.T) {
	var s SoundBlasterDSP
	started := 0
	s.Start16DMA = func(words uint32) bool {
		if words != 1 {
			t.Fatalf("B0 長度=%d", words)
		}
		started++
		return true
	}
	for _, v := range []byte{0xb0, 0x30, 0, 0} {
		if !s.Out8(0x22c, v) {
			t.Fatalf("B0 序列在 %02X 被拒絕", v)
		}
	}
	if started != 1 || s.pending != 0 {
		t.Fatal("B0 完成前啟動 DMA 或未清命令狀態")
	}
	s.IRQ16Pending = true
	s.Out8(0x224, 0x82)
	if v, _ := s.In8(0x225); v != 2 {
		t.Fatalf("16位元IRQ來源=%02X", v)
	}
	s.In8(0x22e)
	if v, _ := s.In8(0x225); v != 2 {
		t.Fatal("8位元確認埠誤清16位元IRQ")
	}
	s.In8(0x22f)
	if v, _ := s.In8(0x225); v != 0 {
		t.Fatal("16位元確認埠未清IRQ")
	}
	for _, invalid := range [][]byte{{0xb0, 0x20}, {0xb0, 0x30, 1}, {0xb0, 0x30, 0, 1}} {
		s.Out8(0x226, 1)
		s.Out8(0x226, 0)
		for i, v := range invalid {
			ok := s.Out8(0x22c, v)
			if ok != (i != len(invalid)-1) {
				t.Fatalf("未知 B0 序列 % X 在第%d筆 ok=%t", invalid, i, ok)
			}
		}
	}
}
func TestLEVGAStatusPolling(t *testing.T) {
	p := NewLEOPLPorts()
	seen := map[byte]bool{}
	for i := 0; i < 64; i++ {
		v, ok := p.In8(0x3da)
		if !ok {
			t.Fatal("VGA拒絕")
		}
		seen[v&8] = true
	}
	if !seen[0] || !seen[8] {
		t.Fatal("回掃狀態未交替")
	}
}

func TestDSPVersionQueue(t *testing.T) {
	var s SoundBlasterDSP
	s.Out8(0x226, 1)
	if s.Out8(0x22c, 0xe1) {
		t.Fatal("reset 期间命令被接受")
	}
	s.Out8(0x226, 0)
	if !s.Out8(0x22c, 0xe1) {
		t.Fatal("E1拒絕")
	}
	for _, want := range []byte{4, 5} {
		v, ok := s.In8(0x22a)
		if !ok || v != want {
			t.Fatalf("version %x", v)
		}
	}
	v, _ := s.In8(0x22e)
	if v&0x80 != 0 {
		t.Fatal("佇列未清")
	}
}

func TestSBMixerHardwareConfig(t *testing.T) {
	var s SoundBlasterDSP
	for _, tc := range []struct{ index, want byte }{{0x80, 4}, {0x81, 0x22}} {
		if !s.Out8(0x224, tc.index) {
			t.Fatal("mixer索引拒絕")
		}
		idx, _ := s.In8(0x224)
		v, ok := s.In8(0x225)
		if idx != tc.index || !ok || v != tc.want {
			t.Fatal("mixer配置")
		}
	}
	s.Out8(0x224, 0xff)
	if _, ok := s.In8(0x225); ok {
		t.Fatal("未知索引放行")
	}
	if s.Out8(0x225, 0) {
		t.Fatal("未支援寫入放行")
	}
}

func TestSB16VoiceVolumeMixerRegisters(t *testing.T) {
	var s SoundBlasterDSP
	read := func(index, want byte) {
		t.Helper()
		if !s.Out8(0x224, index) {
			t.Fatal("混音器索引寫入遭拒")
		}
		if got, ok := s.In8(0x225); !ok || got != want {
			t.Fatalf("索引 %02X 讀值 %02X ok=%t，預期 %02X", index, got, ok, want)
		}
	}
	read(0x32, 0xc0)
	read(0x33, 0xc0)
	s.Out8(0x224, 0x32)
	if !s.Out8(0x225, 0xa7) {
		t.Fatal("左聲道音量寫入遭拒")
	}
	read(0x32, 0xa0)
	read(0x33, 0xc0)
	s.Out8(0x224, 0x33)
	if !s.Out8(0x225, 0x1f) {
		t.Fatal("右聲道音量寫入遭拒")
	}
	s.Out8(0x226, 1)
	s.Out8(0x226, 0)
	read(0x32, 0xa0)
	read(0x33, 0x18)
	s.Out8(0x224, 0x31)
	if _, ok := s.In8(0x225); ok || s.Out8(0x225, 0xff) {
		t.Fatal("未知混音器索引被放行")
	}
}

func TestDSPTimeConstantParameter(t *testing.T) {
	var s SoundBlasterDSP
	s.Out8(0x22c, 0x40)
	s.Out8(0x22c, 0xe1)
	if !s.TimeConstantKnown || s.TimeConstant != 0xe1 || len(s.reply) != 0 {
		t.Fatal("參數誤當命令")
	}
	s.Out8(0x22c, 0x40)
	s.Out8(0x226, 1)
	s.Out8(0x226, 0)
	s.Out8(0x22c, 0xe1)
	if s.TimeConstantKnown || len(s.reply) != 2 || s.reply[0] != 4 {
		t.Fatal("reset未清命令狀態")
	}
}

func TestSBMixerIRQSourceAndAcknowledge(t *testing.T) {
	var s SoundBlasterDSP
	s.Out8(0x224, 0x82)
	v, _ := s.In8(0x225)
	if v != 0 {
		t.Fatal("虛構IRQ")
	}
	s.IRQPending = true
	for i := 0; i < 2; i++ {
		v, ok := s.In8(0x225)
		if !ok || v != 1 {
			t.Fatal("中斷來源")
		}
	}
	s.In8(0x22e)
	v, _ = s.In8(0x225)
	if v != 0 || s.IRQPending {
		t.Fatal("IRQ未確認")
	}
}

func TestDSPBlockSizeDoesNotStartDMA(t *testing.T) {
	var s SoundBlasterDSP
	s.StartDMA = func(uint32) bool { t.Fatal("48不應啟動DMA"); return false }
	for _, tc := range []struct {
		lo, hi byte
		n      uint32
	}{{0, 0, 1}, {255, 255, 65536}, {0x14, 0x40, 0x4015}} {
		for _, b := range []byte{0x48, tc.lo, tc.hi} {
			if !s.Out8(0x22c, b) {
				t.Fatal("48参数拒絕")
			}
		}
		if !s.BlockSizeKnown || s.BlockSize != tc.n {
			t.Fatal("48長度")
		}
	}
}

func TestDSP4SpeakerFlag(t *testing.T) {
	s := SoundBlasterDSP{}
	s.CancelDMA = func() { t.Fatal("speaker command cancelled DMA") }
	for _, v := range []byte{0xd1, 0xd3, 0xd1} {
		if !s.Out8(0x22c, v) || !s.Out8(0x22c, 0xd8) {
			t.Fatal("speaker command")
		}
		got, ok := s.In8(0x22a)
		want := byte(0)
		if v == 0xd1 {
			want = 0xff
		}
		if !ok || got != want || s.SpeakerOn != (v == 0xd1) {
			t.Fatal("speaker status")
		}
	}
	s.CancelDMA = nil
	s.Out8(0x226, 1)
	s.Out8(0x226, 0)
	s.In8(0x22a)
	s.Out8(0x22c, 0xd8)
	if got, _ := s.In8(0x22a); got != 0 {
		t.Fatal("reset flag")
	}
	s.Out8(0x22c, 0x40)
	s.Out8(0x22c, 0xd1)
	if s.TimeConstant != 0xd1 || s.SpeakerOn {
		t.Fatal("parameter mistaken for speaker command")
	}
}

func TestDSPExplicitOutputRate(t *testing.T) {
	for _, rate := range []uint16{5000, 11025, 22050, 44100, 45000} {
		s := SoundBlasterDSP{RateNumerator: 12345, RateDenominator: 1, TimeConstantKnown: true}
		if !s.Out8(0x22c, 0x41) || !s.Out8(0x22c, byte(rate>>8)) || s.RateNumerator != 12345 {
			t.Fatal("41半筆发布")
		}
		if !s.Out8(0x22c, byte(rate)) || s.RateNumerator != uint64(rate) || s.RateDenominator != 1 || s.TimeConstantKnown {
			t.Fatal("41順序")
		}
	}
	for _, rate := range []uint16{0, 4999, 45001, 65535} {
		s := SoundBlasterDSP{RateNumerator: 22050, RateDenominator: 1}
		s.Out8(0x22c, 0x41)
		s.Out8(0x22c, byte(rate>>8))
		if s.Out8(0x22c, byte(rate)) || s.RateNumerator != 22050 {
			t.Fatal("41無效率")
		}
	}
	s := SoundBlasterDSP{}
	s.Out8(0x22c, 0x41)
	s.Out8(0x22c, 0xac)
	s.Out8(0x226, 1)
	s.Out8(0x226, 0)
	if s.pending != 0 || s.lengthHighNext || s.RateNumerator != 22050 {
		t.Fatal("41 reset")
	}
}
