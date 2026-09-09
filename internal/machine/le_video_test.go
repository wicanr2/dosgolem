package machine

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

func TestLEMode13AndDAC(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x110000)}
	m.CPU = cpu386.New(m)
	p := NewLEOPLPorts()
	if !InstallLEVideo(m, p) || InstallLEVideo(m, p) {
		t.Fatal("裝置安裝")
	}
	m.Mem[0x100000] = 0x55
	m.Mem[0xaffff] = 0xaa
	m.CPU.R[cpu386.EAX] = 0x13
	if !m.Video.Handle(m.CPU) || m.Mem[0x449] != 0x13 || m.Mem[0x44a] != 40 || m.Mem[0xaffff] != 0 || m.Mem[0x100000] != 0x55 {
		t.Fatal("模式／VRAM清除")
	}
	m.Mem[0xa0000] = 7
	m.Mem[0xaf9ff] = 9
	pixels := m.Video.Indexed()
	if len(pixels) != 64000 || pixels[0] != 7 || pixels[63999] != 9 {
		t.Fatal("索引畫面")
	}
	pixels[0] = 0
	if m.Mem[0xa0000] != 7 {
		t.Fatal("快照未複製")
	}
	p.Out8(0x3c8, 7)
	for _, v := range []byte{0xff, 0x20, 0, 1, 2, 3} {
		p.Out8(0x3c9, v)
	}
	pal := m.Video.Palette()
	if pal[7] != [3]byte{255, 130, 0} || pal[8] != [3]byte{4, 8, 12} {
		t.Fatal("DAC遮罩、展開、遞增")
	}
	m.CPU.R[cpu386.EAX] = 0x12
	if m.Video.Handle(m.CPU) || m.Video.ModeSets != 1 || m.Mem[0xa0000] != 7 {
		t.Fatal("未知模式未拒絕")
	}
}
func TestWatcomVideoInPlaceREGS(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	m.Mem = append(m.Mem, make([]byte, 0x100000-len(m.Mem))...)
	p := NewLEOPLPorts()
	InstallLEVideo(m, p)
	svc := &WatcomInt386DPMI{machine: m, entry: 0x2000}
	m.CPU.StepHook = svc.Handle
	m.CPU.EIP = 0x2000
	m.CPU.R[cpu386.ESP] = 0x20
	for i, v := range []uint32{0x3000, 0x10, 0x60, 0x60} {
		binary.LittleEndian.PutUint32(m.Mem[0x20+i*4:], v)
	}
	for i, v := range []uint32{0x13, 1, 2, 3, 4, 5, 1} {
		binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], v)
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	if m.Video.Mode != 0x13 || m.CPU.EIP != 0x3000 || m.CPU.R[cpu386.ESP] != 0x24 || binary.LittleEndian.Uint32(m.Mem[0x78:]) != 0 || binary.LittleEndian.Uint32(m.Mem[0x64:]) != 1 {
		t.Fatal("INT10轉接")
	}
}
