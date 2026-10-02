package machine

import (
	"reflect"
	"testing"
)

func TestDSPDMA8ControlCommandParsingAndRefusal(t *testing.T) {
	for _, command := range []byte{0xd0, 0xd4} {
		for _, available := range []bool{false, true} {
			for _, accept := range []bool{false, true} {
				s := SoundBlasterDSP{SpeakerOn: true, IRQPending: true, IRQ16Pending: true}
				calls := 0
				if available {
					s.ControlDMA8 = func(paused bool) bool {
						calls++
						if paused != (command == 0xd0) {
							t.Fatal("控制方向錯誤")
						}
						return accept
					}
				}
				if s.Out8(0x22c, command) != (available && accept) || calls != map[bool]int{false: 0, true: 1}[available] || !s.SpeakerOn || !s.IRQPending || !s.IRQ16Pending || s.pending != 0 {
					t.Fatal("控制命令／拒絕破壞DSP狀態")
				}
				s.Out8(0x226, 1)
				before := calls
				if s.Out8(0x22c, command) || calls != before {
					t.Fatal("reset期間控制DMA")
				}
			}
		}
	}
	// 控制碼在參數位置仍是資料，不能跳過尚未完成的命令。
	for _, prefix := range [][]byte{{0x40}, {0x41}, {0x14}, {0x48}, {0xc6, 0x20}, {0xc6}, {0xb0}} {
		var s SoundBlasterDSP
		s.ControlDMA8 = func(bool) bool { t.Fatal("參數誤當控制命令"); return true }
		s.StartDMA = func(uint32) bool { return true }
		s.StartSB16AutoDMA = func(uint32, byte) bool { return true }
		for _, b := range prefix {
			if !s.Out8(0x22c, b) {
				t.Fatal("前綴拒絕")
			}
		}
		s.Out8(0x22c, 0xd0)
		if prefix[0] != 0x40 {
			s.Out8(0x22c, 0xd4)
		}
	}
}

func TestDMA8PauseResumeBothModesIndependentSamples(t *testing.T) {
	for _, mode := range []byte{0, 0x10, 0x20, 0x30} {
		for _, schedule := range []int{0, 1, 2} {
			p, m, r, _ := sharedDMAFixture(t, mode, 22050)
			step := func(n int) { sharedStep(t, p, m, r, schedule == 1 || schedule == 2 && n%3 == 0) }
			channels := uint64(1)
			if mode&0x20 != 0 {
				channels = 2
			}
			for tick := 1; tick <= 53; tick++ {
				step(tick)
			}
			wantSamples := uint64(53) * 22050 * channels / 1000000
			wantCredit := uint64(53) * 22050 * channels % 1000000
			if uint64(len(p.PCM)) != wantSamples || p.sampleCredit != wantCredit {
				t.Fatal("獨立初態取樣數")
			}
			before := p.State()
			for repeat := 0; repeat < 2; repeat++ {
				if !p.Out8(0x22c, 0xd0) {
					t.Fatal("暫停拒絕")
				}
			}
			want := before
			want.DMA8Paused = true
			if p.State() != want || p.DMA8PauseCommands != 2 || p.DMA8ResumeCommands != 0 || p.DMA8ControlLast.Command != 0xd0 || p.DMA8ControlLast.After != want {
				t.Fatal("暫停改變來源／IRQ／信用")
			}
			for tick := 1; tick <= 10000; tick++ {
				step(tick)
			}
			want.VirtualMicros += 10000
			if p.State() != want || p.BIOSClock.Micros != want.VirtualMicros || !p.dmaActive || uint64(len(p.PCM)) != wantSamples {
				t.Fatalf("暫停狀態或時鐘：%+v", p.State())
			}
			for repeat := 0; repeat < 2; repeat++ {
				if !p.Out8(0x22c, 0xd4) {
					t.Fatal("恢復拒絕")
				}
			}
			want.DMA8Paused = false
			if p.State() != want || p.DMA8ResumeCommands != 2 || p.DMA8ControlLast.Command != 0xd4 {
				t.Fatal("恢復重載或丟信用")
			}
			for tick := 1; tick <= 1000; tick++ {
				step(tick)
				activeTicks := uint64(53 + tick)
				samples := activeTicks * 22050 * channels / 1000000
				if uint64(len(p.PCM)) != samples || p.sampleCredit != activeTicks*22050*channels%1000000 || p.DMACompletions != samples/2 || p.dma.Current[2] != 0x200+uint16(samples%4) || p.dma.Current[3] != 3-uint16(samples%4) || p.dmaLeft != 2-uint32(samples%2) {
					t.Fatalf("恢復後tick%d：%+v", tick, p.State())
				}
			}
			for i, b := range p.PCM {
				if b != []byte{0, 0x7f, 0x80, 0xff}[i%4] {
					t.Fatal("來源byte／重載錯誤")
				}
			}
		}
	}
}

func TestDMA8PauseSingleCycleCompletionAndNoSourceAccess(t *testing.T) {
	p, m, r := dmaIRQFixture(t)
	r.SetFlags(2)
	for i := 0; i < 70; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 1 || p.sampleCredit != 25000000 {
		t.Fatal("單次DMA初態")
	}
	if !p.Out8(0x22c, 0xd0) {
		t.Fatal("single暫停")
	}
	saved := m.Mem
	m.Mem = nil
	for i := 0; i < 1000; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal("暫停仍讀來源", err)
		}
	}
	if len(p.PCM) != 1 || p.sampleCredit != 25000000 || p.dmaLeft != 3 {
		t.Fatal("暫停丟資料／剩餘")
	}
	m.Mem = saved
	if !p.Out8(0x22c, 0xd4) {
		t.Fatal("single恢復")
	}
	for i := 0; i < 110; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(p.PCM, []byte{10, 20, 30, 40}) || p.DMACompletions != 1 || p.dmaActive || !p.dsp.IRQPending || p.dma.Mask&2 == 0 {
		t.Fatalf("single完成：%+v", p.State())
	}
	before := p.State()
	if p.Out8(0x22c, 0xd0) || p.Out8(0x22c, 0xd4) || p.State() != before {
		t.Fatal("未知idle控制被接受")
	}
}

func TestDMA8PauseIRQMaskResetAndWidthIsolation(t *testing.T) {
	p, m, r := c6DMAFixture(t, 0x20, 2, 22050)
	p.dsp.IRQPending, p.dsp.IRQ16Pending = true, true
	p.picPending, p.picInService = true, true
	before := p.State()
	if !p.Out8(0x22c, 0xd0) {
		t.Fatal("IRQ pending暫停")
	}
	want := before
	want.DMA8Paused = true
	if p.State() != want {
		t.Fatal("暫停清pending IRQ")
	}
	p.In8(0x22e)
	if p.dsp.IRQPending || !p.dsp.IRQ16Pending || !p.picPending || !p.picInService {
		t.Fatal("8位確認隔離")
	}
	p.Out8(0x20, 0x20)
	if p.picInService {
		t.Fatal("暫停阻止EOI")
	}
	p.In8(0x22f)
	p.picPending = false
	// 控制埠D4h是DMA8237，不是寫DSP的D4命令。
	if !p.Out8(0xd4, 5) || !p.dma8Paused {
		t.Fatal("secondary mask混同DSP")
	}
	for _, command := range []byte{0xd5, 0xd6, 0xda, 0xd9} {
		before = p.State()
		if p.Out8(0x22c, command) || p.State() != before {
			t.Fatal("未建模命令接受")
		}
	}
	if !p.Out8(0xa, 5) || !p.Out8(0x22c, 0xd4) {
		t.Fatal("masked恢復")
	}
	for i := 0; i < 200; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 0 || p.sampleCredit != 0 {
		t.Fatal("遮罩仍取樣")
	}
	if !p.Out8(0xa, 1) {
		t.Fatal("解除遮罩")
	}
	for i := 0; i < 23; i++ {
		if err := p.AdvanceRealMode(r, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 1 {
		t.Fatal("解除遮罩不能續行")
	}
	if !p.Out8(0x22c, 0xd0) || !p.Out8(0x226, 1) || p.dma8Paused || p.dmaActive || p.dsp.IRQPending || p.dsp.IRQ16Pending {
		t.Fatal("reset沒有取消pause／IRQ")
	}
	p.Out8(0x226, 0)
	if p.Out8(0x22c, 0xd4) {
		t.Fatal("reset後虛構active")
	}
	if !p.startSB16AutoDMA(2, 0) || p.dma8Paused || p.sampleCredit != 0 {
		t.Fatal("新啟動保留舊pause／信用")
	}
	p16, m16, r16 := dma16IRQFixture(t)
	r16.SetFlags(2)
	before = p16.State()
	if p16.Out8(0x22c, 0xd0) || p16.Out8(0x22c, 0xd4) || p16.State() != before || p16.DMA8ControlLast != nil {
		t.Fatal("D0/D4影響16位傳輸")
	}
	for i := 0; i < 23; i++ {
		if err := p16.AdvanceRealMode(r16, m16); err != nil {
			t.Fatal(err)
		}
	}
	if p16.DMA16Completions != 1 || !p16.dsp.IRQ16Pending || !reflect.DeepEqual(p16.PCM16, []byte{0x34, 0x12}) {
		t.Fatal("16位傳輸停止")
	}
}
