package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// AH=57h（Move／Exchange Memory Region）的契約測試，
// 規格 `docs/spec/429-ems-move-exchange-region` §5。
//
// 期望值照描述子版面與 LIM 4.0 自己列。重點是複製式分頁下的一致性：
// 映著的頁現行內容在 frame，沒映的在備份，兩邊都要讀寫對。

const emsDescAddr = 0x3000 // 描述子放在 0000:3000（DS=0、SI=3000h）

// emsAllocPages 配 n 頁，回傳 handle。
func emsAllocPages(t *testing.T, m *machine.Machine, d *DOS, n uint16) uint16 {
	t.Helper()
	m.CPU.R[cpu.BX] = n
	if ah := emsCallAX(m, d, 0x4300); ah != 0 {
		t.Fatalf("配 %d 頁回狀態 %02X", n, ah)
	}
	return m.CPU.R[cpu.DX]
}

// emsMapPage 把 handle 的邏輯頁映到實體頁 phys。
func emsMapPage(t *testing.T, m *machine.Machine, d *DOS, h, logical uint16, phys uint8) {
	t.Helper()
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = logical, h
	if ah := emsCallAX(m, d, 0x4400|uint16(phys)); ah != 0 {
		t.Fatalf("映射邏輯頁 %d 到實體頁 %d 回狀態 %02X", logical, phys, ah)
	}
}

// emsEnd 是描述子的一端：typ 0 常規（segPage 是段）、1 EMS（segPage 是頁號）。
type emsEnd struct {
	typ     uint8
	handle  uint16
	off     uint16
	segPage uint16
}

// emsMove 寫好描述子後呼叫 AH=57h，回傳 AH。
func emsMove(m *machine.Machine, d *DOS, al uint8, n uint32, src, dst emsEnd) uint8 {
	m.Write16(emsDescAddr, uint16(n))
	m.Write16(emsDescAddr+2, uint16(n>>16))
	for i, e := range []emsEnd{src, dst} {
		at := uint32(emsDescAddr + 4 + i*7)
		m.Write8(at, e.typ)
		m.Write16(at+1, e.handle)
		m.Write16(at+3, e.off)
		m.Write16(at+5, e.segPage)
	}
	m.CPU.Seg[cpu.DS] = 0
	m.CPU.R[cpu.SI] = emsDescAddr
	return emsCallAX(m, d, 0x5700|uint16(al))
}

func TestEMSMoveConventionalToUnmappedPage(t *testing.T) {
	m, d := newTest(t)
	h := emsAllocPages(t, m, d, 2)
	src := uint32(0x4000)
	m.WriteBytes(src, []byte{1, 2, 3, 4})

	// 邏輯頁 1 位移 10。
	if ah := emsMove(m, d, 0, 4, emsEnd{0, 0, 0, 0x400}, emsEnd{1, h, 10, 1}); ah != 0 {
		t.Fatalf("AH=57h 回狀態 %02X", ah)
	}
	emsMapPage(t, m, d, h, 1, 2)
	frame := emsFrameAddr(2)
	for i, want := range []byte{1, 2, 3, 4} {
		if got := m.Read8(frame + 10 + uint32(i)); got != want {
			t.Fatalf("frame+%d 是 %02X，預期 %02X——資料沒有進邏輯頁 1", 10+i, got, want)
		}
	}
}

func TestEMSMoveReadsMappedPageFromFrame(t *testing.T) {
	m, d := newTest(t)
	h := emsAllocPages(t, m, d, 1)
	emsMapPage(t, m, d, h, 0, 0)
	// 寫進 frame、不換頁：備份還是舊的 0。
	m.WriteBytes(emsFrameAddr(0)+100, []byte{0xAA, 0xBB})

	if ah := emsMove(m, d, 0, 2, emsEnd{1, h, 100, 0}, emsEnd{0, 0, 0, 0x500}); ah != 0 {
		t.Fatalf("AH=57h 回狀態 %02X", ah)
	}
	if a, b := m.Read8(0x5000), m.Read8(0x5001); a != 0xAA || b != 0xBB {
		t.Fatalf("讀到 %02X %02X，預期 AA BB——讀的是備份不是 frame", a, b)
	}
}

func TestEMSMoveCrossesLogicalPages(t *testing.T) {
	m, d := newTest(t)
	h := emsAllocPages(t, m, d, 2)
	data := []byte{9, 8, 7, 6, 5, 4, 3, 2}
	m.WriteBytes(0x4000, data)

	if ah := emsMove(m, d, 0, 8, emsEnd{0, 0, 0, 0x400}, emsEnd{1, h, 16380, 0}); ah != 0 {
		t.Fatalf("AH=57h 回狀態 %02X", ah)
	}
	emsMapPage(t, m, d, h, 0, 0)
	emsMapPage(t, m, d, h, 1, 1)
	// 實體頁 0、1 在 frame 裡相鄰，所以跨頁的 8 bytes 在線性位址上連續。
	for i, want := range data {
		if got := m.Read8(emsFrameAddr(0) + 16380 + uint32(i)); got != want {
			t.Fatalf("第 %d byte 是 %02X，預期 %02X", i, got, want)
		}
	}
}

func TestEMSExchangeSwapsBothSides(t *testing.T) {
	m, d := newTest(t)
	h := emsAllocPages(t, m, d, 1)
	emsMapPage(t, m, d, h, 0, 3)
	m.WriteBytes(emsFrameAddr(3), []byte{0x11, 0x22, 0x33})
	m.WriteBytes(0x6000, []byte{0x44, 0x55, 0x66})

	if ah := emsMove(m, d, 1, 3, emsEnd{0, 0, 0, 0x600}, emsEnd{1, h, 0, 0}); ah != 0 {
		t.Fatalf("AH=57h AL=01 回狀態 %02X", ah)
	}
	for i, want := range []byte{0x11, 0x22, 0x33} {
		if got := m.Read8(0x6000 + uint32(i)); got != want {
			t.Fatalf("常規端第 %d byte 是 %02X，預期 %02X", i, got, want)
		}
	}
	for i, want := range []byte{0x44, 0x55, 0x66} {
		if got := m.Read8(emsFrameAddr(3) + uint32(i)); got != want {
			t.Fatalf("EMS 端第 %d byte 是 %02X，預期 %02X", i, got, want)
		}
	}
}

func TestEMSMoveErrorsLeaveMemoryAlone(t *testing.T) {
	m, d := newTest(t)
	h := emsAllocPages(t, m, d, 1)
	m.WriteBytes(0x4000, []byte{0x5A})
	m.Write8(0x7000, 0xC3)

	cases := []struct {
		name string
		al   uint8
		n    uint32
		src  emsEnd
		dst  emsEnd
		want uint8
	}{
		{"handle 不存在", 0, 1, emsEnd{0, 0, 0, 0x400}, emsEnd{1, h + 7, 0, 0}, 0x83},
		{"超出 handle 範圍", 0, 2, emsEnd{0, 0, 0, 0x400}, emsEnd{1, h, 16383, 0}, 0x8A},
		{"子功能 02h", 2, 1, emsEnd{0, 0, 0, 0x400}, emsEnd{0, 0, 0, 0x700}, 0x84},
		{"長度超過 1 MB", 0, 1<<20 + 1, emsEnd{0, 0, 0, 0x400}, emsEnd{0, 0, 0, 0x700}, 0x96},
	}
	for _, tc := range cases {
		if ah := emsMove(m, d, tc.al, tc.n, tc.src, tc.dst); ah != tc.want {
			t.Errorf("%s：回 %02X，預期 %02X", tc.name, ah, tc.want)
		}
		if got := m.Read8(0x7000); got != 0xC3 {
			t.Fatalf("%s：失敗的呼叫改了記憶體（0x7000 變成 %02X）", tc.name, got)
		}
	}
	emsMapPage(t, m, d, h, 0, 0)
	if got := m.Read8(emsFrameAddr(0)); got != 0 {
		t.Errorf("失敗的呼叫寫進了 EMS（邏輯頁 0 第 0 byte 是 %02X）", got)
	}
}
