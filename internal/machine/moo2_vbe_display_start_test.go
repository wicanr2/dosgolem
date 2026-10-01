package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func callMOO2DisplayStart(m *LEMachine, s *MOO2StartupDOS, bx, cx, dx uint32) bool {
	m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX] = 0x4f07, bx, cx, dx
	return s.Handle(m.CPU, 0x10)
}

func TestMOO2VBEDisplayStartOriginalReturnAndBounds(t *testing.T) {
	m, s := newMOO2VideoTest(t)
	if !callMOO2DisplayStart(m, s, 0, 0, 0) || callMOO2DisplayStart(m, s, 0, 0, 512) || callMOO2DisplayStart(m, s, 1, 0, 0) {
		t.Fatal("啟用前的歷史零起點邊界改變")
	}
	setMOO2VideoMode(t, m, s)
	// DOSBox-X 0180:0035CCA7 的具名同次架構輸入。
	m.CPU.R = [8]uint32{0x4f07, 0, 0x200, 0, 0x3ebb80, 0x3ebba8, 0, 0x3a2090}
	m.CPU.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	m.CPU.EFlags = 0x246
	want, segments := m.CPU.R, m.CPU.Seg
	want[cpu386.EAX] = 0x4f
	if !s.Handle(m.CPU, 0x10) || m.CPU.R != want || m.CPU.Seg != segments || m.CPU.EFlags != 0x246 || m.VBEState().StartY != 512 || !s.vbeStartSet || s.vbeStartY != 512 {
		t.Fatal("原版同次返回不符")
	}
	for _, y := range []uint32{0, 512, 2796} {
		if !callMOO2DisplayStart(m, s, 0, 0, y) {
			t.Fatal("合法起點拒絕")
		}
		if len(m.VBEIndexed()) != 640*480 {
			t.Fatal("完整頁不可讀")
		}
		m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX] = 0x4f07, 1, 0xabcd1234, 0x56780009
		want = m.CPU.R
		want[cpu386.EAX], want[cpu386.ECX], want[cpu386.EDX] = 0x4f, 0xabcd0000, 0x56780000|y
		before := m.VBEState()
		if !s.Handle(m.CPU, 0x10) || m.CPU.R != want || m.CPU.Seg != segments || m.CPU.EFlags != 0x246 || m.VBEState() != before {
			t.Fatal("讀回或非目的欄位不符")
		}
	}
	for _, tc := range []struct{ ax, bx, cx, dx uint32 }{
		{0x4f07, 0, 0, 2797}, {0x4f07, 0, 0, 65535}, {0x4f07, 0, 0, 0x10000},
		{0x4f07, 0, 1, 0}, {0x4f07, 0, 0x10000, 0}, {0x4f07, 2, 0, 0},
		{0x4f07, 0x80, 0, 0}, {0x4f07, 0x100, 0, 0}, {0x4f07, 0x10000, 0, 0}, {0x10004f07, 0, 0, 0},
	} {
		m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX] = tc.ax, tc.bx, tc.cx, tc.dx
		beforeR, state, y := m.CPU.R, m.VBEState(), s.vbeStartY
		if s.Handle(m.CPU, 0x10) || m.CPU.R != beforeR || m.VBEState() != state || s.vbeStartY != y || m.CPU.Seg != segments || m.CPU.EFlags != 0x246 {
			t.Fatalf("未知形狀未原樣拒絕：%+v", tc)
		}
	}
}

func TestMOO2VBEDisplayStartBothConsumersAndReset(t *testing.T) {
	m, s := newMOO2VideoTest(t)
	setMOO2VideoMode(t, m, s)
	bridge := &dpmiRealBus{m: m}
	writePage := func(y uint32, value byte, real bool) {
		var lastBank uint32 = 0xffffffff
		for i := uint32(0); i < 640*480; i++ {
			offset := y*640 + i
			bank := offset / 65536
			if bank != lastBank {
				setMOO2Bank(t, m, s, bank)
				lastBank = bank
			}
			addr := 0xa0000 + offset%65536
			if real {
				bridge.Write8(addr, value)
			} else if err := m.Write8(addr, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	writePage(0, 3, false)
	writePage(512, 7, true)
	if bridge.err != nil {
		t.Fatal(bridge.err)
	}
	setMOO2Bank(t, m, s, 5)
	copy(m.Mem[0x110000:], []byte{0xc6, 0x05, 0, 0, 0x0a, 0, 0x17})
	m.CPU.EIP = 0x110000
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	setMOO2Bank(t, m, s, 9)
	bankState := m.VBEState()
	if bankState.Bank != 9 || bankState.Writes != 2*640*480+1 {
		t.Fatal("寫入不在實際視窗映射")
	}
	for _, p := range []struct {
		port  uint16
		value byte
	}{{0x3c8, 3}, {0x3c9, 0}, {0x3c9, 0}, {0x3c9, 0}, {0x3c8, 7}, {0x3c9, 63}, {0x3c9, 32}, {0x3c9, 1}, {0x3c6, 0x0f}} {
		if !m.CPU.PortOut(p.port, p.value) {
			t.Fatal("調色盤拒絕")
		}
	}
	for _, y := range []uint32{512, 0, 512} {
		if !callMOO2DisplayStart(m, s, 0, 0, y) {
			t.Fatal("起點設定拒絕")
		}
		state := m.VBEState()
		if state.Bank != bankState.Bank || state.BankSets != bankState.BankSets || state.Writes != bankState.Writes {
			t.Fatal("顯示起點改了寫入區段或顯存")
		}
		index, color := byte(7), []byte{255, 130, 4}
		if y == 0 {
			index, color = 3, []byte{0, 0, 0}
		}
		pixels, rgb := m.VBEIndexed(), m.VBERGB()
		want := bytes.Repeat([]byte{index}, 640*480)
		if y == 512 {
			want[0] = 0x17
		}
		if !bytes.Equal(pixels, want) {
			t.Fatal("索引仍取錯頁或漏像素")
		}
		if !bytes.Equal(rgb, bytes.Repeat(color, 640*480)) {
			t.Fatal("RGB 未使用同一起點與調色盤")
		}
		pixels[0], rgb[0] = 0x55, 0x55
		if m.VBEIndexed()[0] != want[0] || m.VBERGB()[0] != color[0] {
			t.Fatal("快照外洩內部資料")
		}
	}
	setMOO2VideoMode(t, m, s)
	if m.VBEState().StartY != 0 || s.vbeStartY != 0 || !bytes.Equal(m.VBEIndexed(), make([]byte, 640*480)) {
		t.Fatal("模式重設未歸零起點／清頁")
	}
	if !callMOO2DisplayStart(m, s, 0, 0, 512) {
		t.Fatal("清頁後起點拒絕")
	}
	m.CPU.R[cpu386.EAX] = 3
	if !s.Handle(m.CPU, 0x10) || s.vbeStartSet || m.VBEIndexed() != nil || callMOO2DisplayStart(m, s, 1, 0, 0) || callMOO2DisplayStart(m, s, 0, 0, 512) {
		t.Fatal("停用後仍取圖或接受非零服務")
	}
	if !callMOO2DisplayStart(m, s, 0, 0, 0) {
		t.Fatal("停用後歷史零輸入被拒絕")
	}
}
