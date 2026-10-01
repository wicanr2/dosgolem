package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func newMOO2VideoTest(t *testing.T) (*LEMachine, *MOO2StartupDOS) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x120000), DOSArenaBase: 0x10000, Ports: map[uint16]uint8{}}
	m.CPU = cpu386.New(m)
	m.CPU.SetDescriptor(0x10, cpu386.Descriptor{Limit: 0x11ffff, Writable: true})
	m.CPU.Seg[cpu386.SegDS] = 0x10
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err != nil {
		t.Fatal(err)
	}
	return m, s
}

func setMOO2VideoMode(t *testing.T, m *LEMachine, s *MOO2StartupDOS) {
	t.Helper()
	m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX] = 0x4f02, 0x101, 0, 0
	if !s.Handle(m.CPU, 0x10) || m.CPU.R[cpu386.EAX] != 0x4f {
		t.Fatal("模式設定失敗")
	}
}

func setMOO2Bank(t *testing.T, m *LEMachine, s *MOO2StartupDOS, bank uint32) {
	t.Helper()
	m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX] = 0x4f05, 0, bank
	if !s.Handle(m.CPU, 0x10) {
		t.Fatal("區段設定失敗")
	}
}

func TestMOO2VBEWindowOriginalReturnAndRejections(t *testing.T) {
	m, s := newMOO2VideoTest(t)
	if m.VBEState().Active || m.VBEIndexed() != nil {
		t.Fatal("未設定模式就有顯存消費端")
	}
	m.CPU.R[cpu386.EAX] = 0x4f05
	if s.Handle(m.CPU, 0x10) {
		t.Fatal("未設定模式卻接受切換")
	}
	setMOO2VideoMode(t, m, s)
	// DOSBox-X 0180:0035CC54 的同次輸入；只比較本服務返回。
	m.CPU.R = [8]uint32{0x4f05, 0, 5, 0, 0x3ebb5c, 0x3ebb88, 0, 0x3a2090}
	m.CPU.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	m.CPU.EFlags = 0x206
	want, segments := m.CPU.R, m.CPU.Seg
	want[cpu386.EAX] = 0x4f
	if !s.Handle(m.CPU, 0x10) || m.CPU.R != want || m.CPU.Seg != segments || m.CPU.EFlags != 0x206 || m.VBEState().Bank != 5 {
		t.Fatal("原版樣本服務返回不符")
	}
	for _, bank := range []uint32{0, 5, 31} {
		setMOO2Bank(t, m, s, bank)
		m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX] = 0x4f05, 0x100, 0xabcd1234
		before := m.CPU.R
		before[cpu386.EAX], before[cpu386.EDX] = 0x4f, 0xabcd0000|bank
		if !s.Handle(m.CPU, 0x10) || m.CPU.R != before || m.CPU.EFlags != 0x206 || m.CPU.Seg != segments {
			t.Fatal("讀回未保留非目的狀態")
		}
	}
	for _, tc := range []struct{ ax, bx, dx uint32 }{
		{0x4f05, 1, 0}, {0x4f05, 0x101, 0}, {0x4f05, 0x200, 0}, {0x4f05, 0x10000, 0},
		{0x4f05, 0, 32}, {0x4f05, 0, 0x10000}, {0x10004f05, 0, 0},
	} {
		m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX] = tc.ax, tc.bx, tc.dx
		before, state := m.CPU.R, m.VBEState()
		if s.Handle(m.CPU, 0x10) || m.CPU.R != before || m.VBEState() != state || m.CPU.EFlags != 0x206 || m.CPU.Seg != segments {
			t.Fatalf("未知形狀未原樣拒絕：%+v", tc)
		}
	}
	m.CPU.R[cpu386.EAX] = 0x4f05
	if NewMOO2StartupDOS(nil).Handle(m.CPU, 0x10) || NewFD2StartupDOS(nil).Handle(m.CPU, 0x10) {
		t.Fatal("未附掛或其他遊戲接受顯存服務")
	}
}

func TestMOO2VBEWindowCPUAndDPMIMapping(t *testing.T) {
	m, s := newMOO2VideoTest(t)
	setMOO2VideoMode(t, m, s)
	m.Mem[0xa0000] = 0xee
	for _, bank := range []uint32{0, 5, 31} {
		setMOO2Bank(t, m, s, bank)
		for i := uint32(0); i < 4; i++ {
			if err := m.Write8(0xa0000+i, byte(bank+i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	setMOO2Bank(t, m, s, 5)
	if v, err := m.Read32(0xa0000); err != nil || v != 0x08070605 {
		t.Fatalf("dword=%X err=%v", v, err)
	}
	if v, err := m.Read16(0xa0001); err != nil || v != 0x0706 {
		t.Fatalf("word=%X err=%v", v, err)
	}
	// 由 CPU 執行 MOV byte [000A0000h],7Fh，不以直寫 framebuffer 代替。
	copy(m.Mem[0x110000:], []byte{0xc6, 0x05, 0, 0, 0x0a, 0, 0x7f})
	m.CPU.EIP = 0x110000
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	b := &dpmiRealBus{m: m}
	if b.Read8(0xa0000) != 0x7f || m.Mem[0xa0000] != 0xee {
		t.Fatal("CPU 映射或 RAM 隔離錯誤")
	}
	b.Write8(0xa0001, 0x9a)
	if v, ok := m.CPU.ReadSegment8(0x10, 0xa0001); !ok || v != 0x9a {
		t.Fatal("DPMI 寫入未回到 CPU 消費端")
	}
	if err := m.Write8(0xaffff, 0x21); err != nil {
		t.Fatal(err)
	}
	m.Mem[0x9ffff], m.Mem[0xb0000], m.Mem[0xb0001], m.Mem[0xb0002] = 0x42, 0x43, 0x44, 0x45
	if v, err := m.Read32(0xaffff); err != nil || v != 0x45444321 {
		t.Fatalf("尾端跨界=%X err=%v", v, err)
	}
	if v, err := m.Read16(0x9ffff); err != nil || v != 0x7f42 {
		t.Fatalf("起點跨界=%X err=%v", v, err)
	}
	for _, bank := range []uint32{0, 31} {
		setMOO2Bank(t, m, s, bank)
		if v, err := m.Read32(0xa0000); err != nil || v != (bank+3)<<24|(bank+2)<<16|(bank+1)<<8|bank {
			t.Fatal("切換覆蓋其他區段")
		}
	}
	b.Read8(uint32(len(m.Mem)))
	first := b.err
	b.Write8(0xa0000, 0xff)
	if first == nil || b.err != first || b.Read8(0xa0000) != 31 {
		t.Fatal("DPMI 首個錯誤或拒絕後寫入契約改變")
	}
	if m.VBEState().Writes != 15 {
		t.Fatalf("寫入計數=%d", m.VBEState().Writes)
	}
}

func TestMOO2VBEConsumersAndModeReset(t *testing.T) {
	m, s := newMOO2VideoTest(t)
	setMOO2VideoMode(t, m, s)
	if len(m.vbeVideo.framebuffer) != 2*1024*1024 || m.vbeVideo.pages != 6 {
		t.Fatal("固定容量或頁數不符")
	}
	if err := m.Write8(0xa0000, 0x13); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		port  uint16
		value byte
	}{{0x3c8, 3}, {0x3c9, 63}, {0x3c9, 32}, {0x3c9, 0}, {0x3c6, 0x0f}} {
		if !m.CPU.PortOut(p.port, p.value) {
			t.Fatal("調色盤埠拒絕")
		}
	}
	indexed, rgb := m.VBEIndexed(), m.VBERGB()
	if len(indexed) != 640*480 || indexed[0] != 0x13 || len(rgb) != 640*480*3 || !bytes.Equal(rgb[:3], []byte{255, 130, 0}) {
		t.Fatal("索引／RGB 消費端不符")
	}
	indexed[0], rgb[0] = 0xff, 0
	if m.VBEIndexed()[0] != 0x13 || m.VBERGB()[0] != 255 {
		t.Fatal("外部快照修改內部狀態")
	}
	setMOO2Bank(t, m, s, 28)
	if err := m.Write8(0xa0000, 0x55); err != nil {
		t.Fatal(err)
	}
	setMOO2Bank(t, m, s, 31)
	if err := m.Write8(0xa0000, 0x66); err != nil {
		t.Fatal(err)
	}
	m.Mem[0xa0000] = 0x77
	m.CPU.R[cpu386.EAX] = 3
	if !s.Handle(m.CPU, 0x10) || s.vbeModeSet || m.VBEState().Active || m.VBEIndexed() != nil || m.VBERGB() != nil {
		t.Fatal("文字模式未停用視窗")
	}
	if v, err := m.Read8(0xa0000); err != nil || v != 0x77 {
		t.Fatal("停用後 RAM 路徑不符")
	}
	setMOO2VideoMode(t, m, s)
	if m.VBEState().Bank != 0 || !bytes.Equal(m.VBEIndexed(), make([]byte, 640*480)) {
		t.Fatal("重設未清頁或歸零區段")
	}
	setMOO2Bank(t, m, s, 28)
	if v, err := m.Read8(0xa0000); err != nil || v != 0 {
		t.Fatal("額外影像頁未清除")
	}
	setMOO2Bank(t, m, s, 31)
	if v, err := m.Read8(0xa0000); err != nil || v != 0x66 {
		t.Fatal("已報告頁外顯存被清除")
	}
	if v, ok := m.CPU.PortIn(0x3c6); !ok || v != 0xff {
		t.Fatal("重設未恢復 DAC 遮罩")
	}
}
