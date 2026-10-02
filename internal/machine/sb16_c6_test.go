package machine

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

func TestSB16C6AllModesAndWordLengths(t *testing.T) {
	for _, mode := range []byte{0, 0x10, 0x20, 0x30} {
		for length := 0; length < 65536; length++ {
			s := SoundBlasterDSP{BlockSize: 0x1234, BlockSizeKnown: true}
			calls := 0
			s.StartSB16AutoDMA = func(n uint32, m byte) bool {
				calls++
				if n != uint32(length+1) || m != mode {
					t.Fatalf("mode=%X length=%X：n=%d m=%X", mode, length, n, m)
				}
				return true
			}
			command := [4]byte{0xc6, mode, byte(length), byte(length >> 8)}
			for i, v := range command {
				if !s.Out8(0x22c, v) {
					t.Fatalf("命令 %X 在%d拒絕", command, i)
				}
				if i < 3 && (calls != 0 || s.BlockSize != 0x1234 || !s.BlockSizeKnown || s.Auto8Commands != 0) {
					t.Fatal("完整命令前發布資料")
				}
			}
			if calls != 1 || s.pending != 0 || s.BlockSize != uint32(length+1) || !s.BlockSizeKnown || s.Auto8Command != command || s.Auto8Commands != 1 || s.IRQPending || s.IRQ16Pending {
				t.Fatalf("完成資料錯誤：%X", command)
			}
		}
	}
}

func TestSB16C6RejectsInvalidModesAndIncompleteOrFailedStart(t *testing.T) {
	for value := 0; value < 256; value++ {
		if value == 0 || value == 0x10 || value == 0x20 || value == 0x30 {
			continue
		}
		s := SoundBlasterDSP{BlockSize: 91, BlockSizeKnown: true}
		s.StartSB16AutoDMA = func(uint32, byte) bool { t.Fatal("未知mode啟動DMA"); return true }
		if !s.Out8(0x22c, 0xc6) || s.Out8(0x22c, byte(value)) || s.pending != 0xc6 || s.c6Step != 0 || s.BlockSize != 91 || s.Auto8Commands != 0 {
			t.Fatalf("未知mode=%X遭接受或發布", value)
		}
	}
	for _, callback := range []bool{false, true} {
		s := SoundBlasterDSP{BlockSize: 91, BlockSizeKnown: true, Auto8Commands: 4, Auto8Command: [4]byte{0xc6, 0, 3, 0}}
		calls := 0
		if callback {
			s.StartSB16AutoDMA = func(uint32, byte) bool { calls++; return false }
		}
		for _, v := range []byte{0xc6, 0x20, 0xff} {
			if !s.Out8(0x22c, v) {
				t.Fatal("合法前綴拒絕")
			}
		}
		if s.Out8(0x22c, 7) || s.BlockSize != 91 || !s.BlockSizeKnown || s.Auto8Commands != 4 || s.Auto8Command != [4]byte{0xc6, 0, 3, 0} || s.pending != 0xc6 || s.c6Step != 2 || s.c6Mode != 0x20 || s.lengthLow != 0xff {
			t.Fatal("啟動拒絕後發布結果或破壞解析狀態")
		}
		if calls != map[bool]int{false: 0, true: 1}[callback] {
			t.Fatal("callback次數錯誤")
		}
	}
	for n := 1; n < 4; n++ {
		s := SoundBlasterDSP{}
		calls := 0
		s.StartSB16AutoDMA = func(uint32, byte) bool { calls++; return true }
		for _, v := range []byte{0xc6, 0x20, 0xff}[:n] {
			if !s.Out8(0x22c, v) {
				t.Fatal("合法部分命令拒絕")
			}
		}
		if calls != 0 || s.Auto8Commands != 0 || s.BlockSizeKnown {
			t.Fatal("截短命令啟動")
		}
		if !s.Out8(0x226, 1) || s.pending != 0 || s.c6Step != 0 || s.c6Mode != 0 || !s.Out8(0x226, 0) || !s.Out8(0x22c, 0xd1) || !s.SpeakerOn {
			t.Fatal("reset未取消部分命令")
		}
	}
	for _, command := range []byte{0xc0, 0xc2, 0xc4, 0xc8, 0xca, 0xcc, 0xce} {
		var s SoundBlasterDSP
		if s.Out8(0x22c, command) {
			t.Fatalf("未知Cx命令=%X", command)
		}
	}
}

func c6DMAFixture(t *testing.T, mode byte, samples uint32, rate uint16) (*LEOPLPorts, *LEMachine, *cpu.CPU) {
	t.Helper()
	p, m, c := dmaIRQFixture(t)
	if !p.Out8(0x226, 1) || !p.Out8(0x226, 0) || !p.Out8(0xb, 0x59) {
		t.Fatal("C6 DMA初態")
	}
	copy(m.Mem[0x200:], []byte{0, 0x7f, 0x80, 0xff})
	for _, v := range []byte{0x41, byte(rate >> 8), byte(rate), 0xc6, mode, byte(samples - 1), byte((samples - 1) >> 8)} {
		if !p.Out8(0x22c, v) {
			t.Fatalf("C6設定byte=%X", v)
		}
	}
	c.SetFlags(2)
	return p, m, c
}

func TestSB16C6IndependentSampleClockAllFormatsAndRates(t *testing.T) {
	for _, mode := range []byte{0, 0x10, 0x20, 0x30} {
		for _, rate := range []uint16{5000, 11025, 22050, 44100, 45000} {
			p, m, c := c6DMAFixture(t, mode, 2, rate)
			channels := uint64(1)
			if mode >= 0x20 {
				channels = 2
			}
			for tick := uint64(1); tick <= 10000; tick++ {
				if err := p.AdvanceRealMode(c, m); err != nil {
					t.Fatal(err)
				}
				// 每channel Hz推導，與實作的累積credit分開。
				samples := tick * uint64(rate) * channels / 1000000
				if uint64(len(p.PCM)) != samples || p.DMACompletions != samples/2 || p.dma.Current[2] != 0x200+uint16(samples%4) || p.dma.Current[3] != 3-uint16(samples%4) || !p.dmaActive || !p.dmaAuto || p.dmaStereo != (mode >= 0x20) || p.dmaSigned != (mode == 0x10 || mode == 0x30) || !p.dmaFIFO || p.IRQ7Deliveries != 0 || c.IP != 0x10 || c.Flags != 2 {
					t.Fatalf("mode=%X rate=%d tick=%d samples=%d state=%+v", mode, rate, tick, samples, p.State())
				}
			}
			for i, b := range p.PCM {
				if b != []byte{0, 0x7f, 0x80, 0xff}[i%4] {
					t.Fatal("有號／無號原始byte轉碼或重載錯誤")
				}
			}
			p.In8(0x22e)
			p.dsp.IRQ16Pending = true
			p.Out8(0x224, 0x82)
			if v, _ := p.In8(0x225); v != 2 {
				t.Fatal("8位確認誤清16位來源")
			}
			p.dsp.IRQPending = true
			p.In8(0x22f)
			if v, _ := p.In8(0x225); v != 1 {
				t.Fatal("16位確認誤清8位來源")
			}
		}
	}
}

func TestSB16C6TimeConstantIncludesChannels(t *testing.T) {
	for _, mode := range []byte{0, 0x20} {
		p, m, c := dmaIRQFixture(t)
		p.Out8(0x226, 1)
		p.Out8(0x226, 0)
		p.Out8(0xb, 0x59)
		for _, v := range []byte{0x40, 211, 0xc6, mode, 1, 0} {
			if !p.Out8(0x22c, v) {
				t.Fatal("TC設定")
			}
		}
		c.SetFlags(2)
		for tick := 1; tick <= 10000; tick++ {
			if err := p.AdvanceRealMode(c, m); err != nil {
				t.Fatal(err)
			}
			// 40h的byte吞吐量為1000000／45，mono／stereo都不能再乘2。
			want := tick / 45
			if len(p.PCM) != want || p.DMACompletions != uint64(want/2) {
				t.Fatalf("TC重複channels：mode=%X tick=%d PCM=%d", mode, tick, len(p.PCM))
			}
		}
	}
}

func TestSB16C6OriginalBufferBlockDurationAndIRQ(t *testing.T) {
	p := NewLEOPLPorts()
	m := &LEMachine{Mem: make([]byte, 0x15000)}
	binary.LittleEndian.PutUint32(m.Mem[0x3c:], 0x100)
	m.Mem[0x100] = 0xcf
	for i := 0; i < 4096; i++ {
		m.Mem[0x14000+i] = byte(i)
	}
	c := cpu.New(&Machine{Mem: m.Mem})
	c.Model = cpu.Model80386
	c.IP = 0x10
	c.R[cpu.SP] = 0x300
	c.SetFlags(2)
	for _, w := range [][2]uint16{{0x226, 1}, {0x226, 0}, {0x21, 0x78}, {0xa, 5}, {0xc, 0}, {2, 0}, {2, 0x40}, {3, 0xff}, {3, 0xf}, {0x83, 1}, {0xb, 0x59}, {0xa, 1}, {0x22c, 0xc6}, {0x22c, 0x20}, {0x22c, 0xff}, {0x22c, 7}} {
		if !p.Out8(w[0], byte(w[1])) {
			t.Fatalf("原版形狀設定 %X", w)
		}
	}
	for i := 0; i < 46439; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 2047 || p.DMACompletions != 0 || p.dsp.IRQPending {
		t.Fatal("46439us過早完成2048byte block")
	}
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	if len(p.PCM) != 2048 || p.DMACompletions != 1 || p.dma.Current[2] != 0x4800 || p.dma.Current[3] != 0x7ff || !p.dmaActive || !p.dsp.IRQPending || !p.picPending || p.IRQ7Deliveries != 0 || !bytes.Equal(p.PCM, m.Mem[0x14000:0x14800]) {
		t.Fatalf("第一block誤重載／IRQ／source：%+v", p.State())
	}
	c.SetFlags(cpu.IF | 2)
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	if p.IRQ7Deliveries != 1 || c.IP != 0x100 || c.R[cpu.SP] != 0x2fa {
		t.Fatal("8位IRQ未按IVT派送")
	}
	p.In8(0x22e)
	p.Out8(0x20, 0x20)
	if err := c.Step(); err != nil || c.IP != 0x10 || c.R[cpu.SP] != 0x300 {
		t.Fatalf("原版型IRET：%v", err)
	}
	// 已推進46441us；再到92880us，才是4096byte ring terminal count。
	for i := 46441; i < 92880; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 4096 || p.DMACompletions != 2 || p.dma.Current[2] != 0x4000 || p.dma.Current[3] != 0xfff || p.dma.Mask&2 != 0 || p.IRQ7Deliveries != 2 || !bytes.Equal(p.PCM, m.Mem[0x14000:0x15000]) {
		t.Fatalf("第二block／ring重載錯誤：%+v", p.State())
	}
}

func TestSB16C6ValidationMaskResetAndSourceBounds(t *testing.T) {
	for _, kind := range []string{"8位已執行", "16位已執行", "遮罩", "未知地址", "未知count", "未知page", "非auto模式", "零長度", "超過DMAcount", "未知mode", "無rate", "無denominator", "rate太低", "rate太高", "TC立體聲太低"} {
		p, m, c := dmaIRQFixture(t)
		p.Out8(0x226, 1)
		p.Out8(0x226, 0)
		p.Out8(0xb, 0x59)
		n, mode := uint32(2), byte(0x20)
		switch kind {
		case "8位已執行":
			p.dmaActive = true
		case "16位已執行":
			p.dma16Active = true
		case "遮罩":
			p.dma.Mask |= 2
		case "未知地址":
			p.dma.Known[2] = 1
		case "未知count":
			p.dma.Known[3] = 2
		case "未知page":
			p.dma.PageKnown[1] = false
		case "非auto模式":
			p.dma.Mode[1] = 0x48
		case "零長度":
			n = 0
		case "超過DMAcount":
			n = 5
		case "未知mode":
			mode = 1
		case "無rate":
			p.dsp.RateNumerator = 0
		case "無denominator":
			p.dsp.RateDenominator = 0
		case "rate太低":
			p.dsp.RateNumerator = 4999
		case "rate太高":
			p.dsp.RateNumerator = 45001
		case "TC立體聲太低":
			p.dsp.TimeConstantKnown = true
			p.dsp.RateNumerator = 9999
		}
		before := p.State()
		memory := append([]byte(nil), m.Mem...)
		ip, registers, flags := c.IP, c.R, c.Flags
		if p.startSB16AutoDMA(n, mode) || p.State() != before || !bytes.Equal(m.Mem, memory) || c.IP != ip || c.R != registers || c.Flags != flags {
			t.Fatalf("%s拒絕破壞前態", kind)
		}
	}
	p, m, c := c6DMAFixture(t, 0x30, 2, 22050)
	for i := 0; i < 137; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	p.Out8(0xa, 5)
	before := p.State()
	credit := p.sampleCredit
	for i := 0; i < 1000; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.dma.Current != before.DMACurrent || len(p.PCM) != before.PCMBytes || p.sampleCredit != credit {
		t.Fatal("DMA遮罩仍讀取或累加sample credit")
	}
	p.Out8(0xa, 1)
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	p.Out8(0x226, 1)
	if p.dmaActive || p.dmaStereo || p.dmaSigned || p.dmaFIFO || p.dsp.IRQPending || p.picPending {
		t.Fatal("reset未取消C6與格式")
	}
	before = p.State()
	for i := 0; i < 200; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.DMACompletions != before.DMACompletions || len(p.PCM) != before.PCMBytes {
		t.Fatal("reset後繼續傳輸")
	}
	p, m, c = c6DMAFixture(t, 0x20, 2, 22050)
	p.dma.Page[1] = 1
	address, count := p.dma.Current[2], p.dma.Current[3]
	var failure error
	for i := 0; i < 23; i++ {
		failure = p.AdvanceRealMode(c, m)
		if failure != nil {
			break
		}
	}
	if failure == nil || len(p.PCM) != 0 || p.dma.Current[2] != address || p.dma.Current[3] != count || p.DMACompletions != 0 || p.dsp.IRQPending {
		t.Fatal("source越界未拒絕或發布資料")
	}
}
