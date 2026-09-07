package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// EMS 頁配置與映射的測試（`docs/spec/008` §7）。

func emsAH(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX] >> 8) }

// TestEMSFrameSegmentAndAlloc 釘住 41h／43h 的三路（spec §2）。
func TestEMSFrameSegmentAndAlloc(t *testing.T) {
	m, d := newTest(t)

	call(m, d, 0x67, 0x4100)
	if m.CPU.R[cpu.BX] != 0xE000 {
		t.Errorf("AH=41h 頁框段 ＝ %04X，預期 E000", m.CPU.R[cpu.BX])
	}
	if emsAH(m.CPU) != 0 {
		t.Error("AH=41h 要回 AH=0")
	}

	m.CPU.R[cpu.BX] = 3 // 配 3 頁
	call(m, d, 0x67, 0x4300)
	h := m.CPU.R[cpu.DX]
	if emsAH(m.CPU) != 0 || h == 0 {
		t.Fatalf("配 3 頁要成功且有 handle，AH=%02X DX=%d",
			emsAH(m.CPU), h)
	}

	m.CPU.R[cpu.BX] = 6 // 只剩 5 頁，要 6 頁 → 88h
	call(m, d, 0x67, 0x4300)
	if got := emsAH(m.CPU); got != 0x88 {
		t.Errorf("超額要回 AH=88h，得到 %02X", got)
	}

	m.CPU.R[cpu.BX] = 0 // 0 頁 → 87h
	call(m, d, 0x67, 0x4300)
	if got := emsAH(m.CPU); got != 0x87 {
		t.Errorf("要 0 頁要回 AH=87h，得到 %02X", got)
	}

	// 可用頁數要反映在 42h。
	call(m, d, 0x67, 0x4200)
	if m.CPU.R[cpu.BX] != 5 || m.CPU.R[cpu.DX] != 8 {
		t.Errorf("42h 要回 BX=5 DX=8，得到 BX=%d DX=%d",
			m.CPU.R[cpu.BX], m.CPU.R[cpu.DX])
	}
}

// TestEMSMapWriteReadAndWriteBack 釘住複製式分頁的核心：
// 映射後讀寫頁框 ＝ 讀寫那一頁；換映射時舊頁**要寫回**，
// 漏寫回的症狀是「存進 EMS 的資料換頁回來變 0」——資料安靜消失。
func TestEMSMapWriteReadAndWriteBack(t *testing.T) {
	m, d := newTest(t)

	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x67, 0x4300) // 配 2 頁
	h := m.CPU.R[cpu.DX]

	// 映射邏輯頁 1 到實體頁 0，寫一個 pattern。
	m.CPU.R[cpu.DX] = h
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x67, 0x4400) // AL=0（實體頁 0）
	if emsAH(m.CPU) != 0 {
		t.Fatal("映射邏輯頁 1 失敗")
	}
	frame := uint32(emsFrameSeg) * 16
	m.Write8(frame, 0x5A)
	m.Write8(frame+0x3FFF, 0xA5)

	// 換映射到邏輯頁 0：頁框要變回 0（新頁），舊內容存進邏輯頁 1。
	m.CPU.R[cpu.DX] = h
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x67, 0x4400)
	if got := m.Read8(frame); got != 0 {
		t.Errorf("換映射後頁框要是新頁（0），得到 %02X——寫回／換入有一邊漏了", got)
	}

	// 映射回邏輯頁 1：剛才寫的要還在。
	m.CPU.R[cpu.DX] = h
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x67, 0x4400)
	if got := m.Read8(frame); got != 0x5A {
		t.Errorf("映射回邏輯頁 1，開頭 ＝ %02X，預期 5A（寫回漏了）", got)
	}
	if got := m.Read8(frame + 0x3FFF); got != 0xA5 {
		t.Errorf("映射回邏輯頁 1，結尾 ＝ %02X，預期 A5", got)
	}

	// 錯誤路徑：無效 handle／邏輯頁超出。
	m.CPU.R[cpu.DX] = 99
	call(m, d, 0x67, 0x4400)
	if got := emsAH(m.CPU); got != 0x83 {
		t.Errorf("無效 handle 要回 83h，得到 %02X", got)
	}
	m.CPU.R[cpu.DX] = h
	m.CPU.R[cpu.BX] = 5
	call(m, d, 0x67, 0x4400)
	if got := emsAH(m.CPU); got != 0x8A {
		t.Errorf("邏輯頁超出要回 8Ah，得到 %02X", got)
	}
}

// TestEMSRelease 釘住 45h：頁池回升、handle 作廢、佔著的實體頁解除。
func TestEMSRelease(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x67, 0x4300)
	h := m.CPU.R[cpu.DX]

	m.CPU.R[cpu.DX] = h
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x67, 0x4400) // 佔住實體頁 0
	m.Write8(uint32(emsFrameSeg)*16, 0x77)

	m.CPU.R[cpu.DX] = h
	call(m, d, 0x67, 0x4500) // 釋放
	if emsAH(m.CPU) != 0 {
		t.Fatal("釋放失敗")
	}
	call(m, d, 0x67, 0x4200)
	if m.CPU.R[cpu.BX] != 8 {
		t.Errorf("釋放後可用頁 ＝ %d，預期 8", m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.DX] = h
	call(m, d, 0x67, 0x4500) // 再釋放同一個 → 83h
	if got := emsAH(m.CPU); got != 0x83 {
		t.Errorf("重複釋放要回 83h，得到 %02X", got)
	}
}

// TestFileAttr 釘住 int 21h AH=43h AL=00 的兩路（spec §4）。
func TestFileAttr(t *testing.T) {
	m, d := newTest(t)
	// 找得到的檔（EMMXXXX0 裝置也算「存在」）。
	m.WriteBytes(0x50000, append([]byte("EMMXXXX0"), 0))
	m.CPU.Seg[cpu.DS] = 0x5000
	m.CPU.R[cpu.DX] = 0
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.CX] != 0x20 {
		t.Errorf("存在的檔要 CF=0、CX=20h，得到 CF=%v CX=%04X",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.CX])
	}
	// 找不到。
	m.WriteBytes(0x50000, append([]byte("nope.dat"), 0))
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Errorf("找不到要 CF=1、AX=2，得到 CF=%v AX=%04X",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}
