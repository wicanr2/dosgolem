package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"testing"
)

func dmaIRQFixture(t *testing.T) (*LEOPLPorts, *LEMachine, *cpu.CPU) {
	t.Helper()
	p := NewLEOPLPorts()
	m := &LEMachine{Mem: make([]byte, 1024)}
	m.Mem[0x3c] = 0
	m.Mem[0x3d] = 1
	m.Mem[0x100] = 0xcf
	copy(m.Mem[0x200:], []byte{10, 20, 30, 40})
	c := cpu.New(&Machine{Mem: m.Mem})
	c.Model = cpu.Model80386
	c.IP = 0x10
	c.R[cpu.SP] = 0x300
	c.SetFlags(cpu.IF | 2)
	for _, w := range [][2]uint16{{0x21, 0x78}, {0xa, 5}, {0xc, 0}, {2, 0}, {2, 2}, {3, 3}, {3, 0}, {0x83, 0}, {0xb, 0x49}, {0xa, 1}, {0x22c, 0x40}, {0x22c, 211}, {0x22c, 0x14}, {0x22c, 3}, {0x22c, 0}} {
		if !p.Out8(w[0], byte(w[1])) {
			t.Fatalf("設定埠%x拒絕", w[0])
		}
	}
	return p, m, c
}

func dma16IRQFixture(t *testing.T) (*LEOPLPorts, *LEMachine, *cpu.CPU) {
	t.Helper()
	p := NewLEOPLPorts()
	m := &LEMachine{Mem: make([]byte, 1024)}
	m.Mem[0x3d] = 1
	m.Mem[0x100] = 0xcf
	m.Mem[0x200], m.Mem[0x201] = 0x34, 0x12
	c := cpu.New(&Machine{Mem: m.Mem})
	c.Model = cpu.Model80386
	c.IP = 0x10
	c.R[cpu.SP] = 0x300
	c.SetFlags(cpu.IF | 2)
	for _, w := range [][2]uint16{
		{0x226, 1}, {0x226, 0}, {0x21, 0x78}, {0xd4, 5}, {0xd8, 0},
		{0xc4, 0}, {0xc4, 1}, {0xc6, 1}, {0xc6, 0}, {0x8b, 0}, {0xd6, 0x49}, {0xd4, 1},
		{0x22c, 0xb0}, {0x22c, 0x30}, {0x22c, 0}, {0x22c, 0},
	} {
		if !p.Out8(w[0], byte(w[1])) {
			t.Fatalf("16位元DMA設定埠 %X 值 %X 拒絕", w[0], w[1])
		}
	}
	return p, m, c
}

func TestSB16B0SingleWordCompletionAndIRQ(t *testing.T) {
	p, m, c := dma16IRQFixture(t)
	for i := 0; i < 22; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.DMA16Completions != 0 || p.IRQ7Deliveries != 0 || len(p.PCM16) != 0 {
		t.Fatal("16位元DMA過早完成")
	}
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	if p.DMA16Completions != 1 || p.IRQ7Deliveries != 1 || c.IP != 0x100 ||
		p.secondaryDMA.Current[2] != 0x101 || p.secondaryDMA.Current[3] != 0 || p.secondaryDMA.Mask&2 != 0 ||
		len(p.PCM16) != 2 || p.PCM16[0] != 0x34 || p.PCM16[1] != 0x12 || !p.dsp.IRQ16Pending {
		t.Fatalf("B0完成狀態錯誤：state=%+v pcm=% X", p.State(), p.PCM16)
	}
	p.In8(0x22f)
	p.Out8(0x20, 0x20)
	if err := c.Step(); err != nil || c.IP != 0x10 || p.dsp.IRQ16Pending {
		t.Fatalf("16位元IRQ確認／IRET失敗：err=%v ip=%X", err, c.IP)
	}
}

func TestSB16B0RejectsUnconfiguredAndResetCancels(t *testing.T) {
	p := NewLEOPLPorts()
	p.Out8(0x226, 1)
	p.Out8(0x226, 0)
	for _, v := range []byte{0xb0, 0x30, 0} {
		if !p.Out8(0x22c, v) {
			t.Fatal("B0 前三筆遭拒絕")
		}
	}
	if p.Out8(0x22c, 0) || p.dma16Active || p.dsp.IRQ16Pending {
		t.Fatal("未設定的16位元DMA被放行")
	}
	p, m, c := dma16IRQFixture(t)
	p.Out8(0x226, 1)
	for i := 0; i < 100; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.dma16Active || p.DMA16Completions != 0 || p.IRQ7Deliveries != 0 || p.dsp.IRQ16Pending {
		t.Fatal("DSP reset未取消16位元DMA或IRQ")
	}
}
func TestDMACompletionUsesOriginalIVT(t *testing.T) {
	p, m, c := dmaIRQFixture(t)
	for i := 0; i < 179; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.DMACompletions != 0 || p.IRQ7Deliveries != 0 || len(p.PCM) != 3 || p.dma.Current[3] != 0 {
		t.Fatal("DMA過早完成")
	}
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	if p.DMACompletions != 1 || p.IRQ7Deliveries != 1 || c.IP != 0x100 || c.R[cpu.SP] != 0x2fa || p.dma.Current[2] != 0x204 || p.dma.Current[3] != 0xffff || p.dma.Mask&2 == 0 {
		t.Fatal("DMA完成／IRQ狀態")
	}
	for i, b := range []byte{10, 20, 30, 40} {
		if p.PCM[i] != b {
			t.Fatal("DMA讀錯資料")
		}
	}
	p.In8(0x22e)
	p.Out8(0x20, 0x20)
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.IP != 0x10 || c.R[cpu.SP] != 0x300 || c.Flags&cpu.IF == 0 {
		t.Fatal("IRQ未經IRET還原")
	}
	for i := 0; i < 200; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.IRQ7Deliveries != 1 || p.dsp.IRQPending {
		t.Fatal("重複中斷")
	}
}
func TestDMAIRQWaitsForMaskAndIF(t *testing.T) {
	for _, mask := range []bool{true, false} {
		p, m, c := dmaIRQFixture(t)
		if mask {
			p.Out8(0x21, 0xf8)
		} else {
			c.SetFlags(2)
		}
		for i := 0; i < 180; i++ {
			if err := p.AdvanceRealMode(c, m); err != nil {
				t.Fatal(err)
			}
		}
		if p.DMACompletions != 1 || p.IRQ7Deliveries != 0 || !p.picPending {
			t.Fatal("待處理IRQ遺失")
		}
		p.Out8(0x21, 0x78)
		c.SetFlags(cpu.IF | 2)
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
		if p.IRQ7Deliveries != 1 {
			t.Fatal("IRQ未解除等待")
		}
	}
}
func TestDMARejectsUnsupportedAndResetCancels(t *testing.T) {
	p := NewLEOPLPorts()
	if p.startDSPDMA(4) {
		t.Fatal("未設定DMA被接受")
	}
	p, m, c := dmaIRQFixture(t)
	p.Out8(0x226, 1)
	for i := 0; i < 200; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.DMACompletions != 0 || p.IRQ7Deliveries != 0 || p.dmaActive {
		t.Fatal("reset未取消DMA")
	}
	p.dsp.TimeConstantKnown = true
	p.dma.Mode[1] = 0x58
	if p.startDSPDMA(4) {
		t.Fatal("auto-init尚未支援")
	}
}

func TestPICRequestAndInServiceReadback(t *testing.T) {
	p := NewLEOPLPorts()
	p.picPending = true
	read := func(w byte) {
		t.Helper()
		v, ok := p.In8(0x20)
		if !ok || v != w {
			t.Fatalf("PIC=%x", v)
		}
	}
	read(0x80)
	p.Out8(0x20, 0xb)
	read(0)
	p.picPending = false
	p.picInService = true
	read(0x80)
	read(0x80)
	p.Out8(0xa0, 0xa)
	read(0x80)
	p.Out8(0x20, 0x20)
	read(0)
	p.Out8(0x20, 0xa)
	read(0)
}

func TestDMAAutoInitReloadsAndFractionalRate(t *testing.T) {
	p, m, c := dmaIRQFixture(t)
	p.Out8(0x226, 1)
	p.Out8(0x226, 0)
	p.Out8(0xb, 0x59)
	for _, v := range []byte{0x48, 3, 0, 0x1c} {
		if !p.Out8(0x22c, v) {
			t.Fatal("auto-init設定")
		}
	}
	c.SetFlags(2)
	for i := 0; i < 10000; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.PCM) != 220 || p.DMACompletions != 55 || p.dma.Current[2] != 0x200 || p.dma.Current[3] != 3 || p.dma.Mask&2 != 0 {
		t.Fatalf("auto-init samples=%d blocks=%d", len(p.PCM), p.DMACompletions)
	}
	for i, b := range p.PCM {
		if b != []byte{10, 20, 30, 40}[i%4] {
			t.Fatal("auto-init讀取未重載")
		}
	}
	c.SetFlags(cpu.IF | 2)
	if err := p.AdvanceRealMode(c, m); err != nil {
		t.Fatal(err)
	}
	p.In8(0x22e)
	p.Out8(0x20, 0x20)
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 183; i++ {
		if err := p.AdvanceRealMode(c, m); err != nil {
			t.Fatal(err)
		}
	}
	if p.IRQ7Deliveries != 2 {
		t.Fatal("確認後未再派送IRQ")
	}
}
