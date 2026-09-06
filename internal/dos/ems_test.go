package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EMS（`docs/spec/014`）的釘死測試。

// emsCallAH 發一次 EMS 服務，回 AH（0 ＝ 成功）。
func emsCallAH(m *machine.Machine, d *DOS, ax uint16) uint8 {
	m.CPU.R[cpu.AX] = ax
	d.handle(m.CPU, 0xF6)
	return uint8(m.CPU.R[cpu.AX] >> 8)
}

// TestEMSSignature 釘住偵測路徑：int 67h 向量指的段，+0Ah 是 EMMXXXX0。
//
// 少了它程式判定沒有 EMS——**然後什麼都不說**，只是不去讀那些放在 EMS 裡
// 的資料檔（源平合戰的 OPEN.EXE 就是這樣停在開場 logo）。
func TestEMSSignature(t *testing.T) {
	m := machine.New()
	seg := m.Read16(0x67*4 + 2)
	sig := make([]byte, 8)
	for i := range sig {
		sig[i] = m.Read8(uint32(seg)*16 + 0x0A + uint32(i))
	}
	got := string(sig)
	if got != "EMMXXXX0" {
		t.Errorf("int 67h 向量段 %04X 的 +0Ah 是 %q，要 EMMXXXX0", seg, got)
	}
}

// TestEMSAllocMapFlush 釘住配置、映射與**換頁時的寫回**。
func TestEMSAllocMapFlush(t *testing.T) {
	m, d := newTest(t)

	if ah := emsCallAH(m, d, 0x4000); ah != 0 {
		t.Fatalf("AH=40h 狀態 %02X", ah)
	}
	if ah := emsCallAH(m, d, 0x4600); ah != 0 || uint8(m.CPU.R[cpu.AX]) != 0x40 {
		t.Errorf("版本回 AX=%04X，要 AL=40h", m.CPU.R[cpu.AX])
	}
	if ah := emsCallAH(m, d, 0x4100); ah != 0 || m.CPU.R[cpu.BX] != machine.EMSFrameSeg {
		t.Errorf("page frame 回 %04X，要 %04X", m.CPU.R[cpu.BX], machine.EMSFrameSeg)
	}

	// 配兩頁。
	m.CPU.R[cpu.BX] = 2
	if ah := emsCallAH(m, d, 0x4300); ah != 0 {
		t.Fatalf("配置失敗 %02X", ah)
	}
	h := m.CPU.R[cpu.DX]

	base := uint32(machine.EMSFrameSeg) * 16
	// 邏輯頁 0 → 實體頁 0，寫一個記號。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	if ah := emsCallAH(m, d, 0x4400); ah != 0 {
		t.Fatalf("映射失敗 %02X", ah)
	}
	m.Write8(base, 0xAA)

	// 換成邏輯頁 1，寫另一個記號。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 1, h
	if ah := emsCallAH(m, d, 0x4400); ah != 0 {
		t.Fatalf("換頁失敗 %02X", ah)
	}
	if got := m.Read8(base); got == 0xAA {
		t.Error("換頁之後還看得到前一頁的內容")
	}
	m.Write8(base, 0xBB)

	// 換回邏輯頁 0：要看到 AA，不是 BB——這一步就是寫回的驗收。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	if ah := emsCallAH(m, d, 0x4400); ah != 0 {
		t.Fatalf("換回失敗 %02X", ah)
	}
	if got := m.Read8(base); got != 0xAA {
		t.Errorf("換回邏輯頁 0 讀到 %02X，要 AA——換頁時沒有寫回", got)
	}
}

// TestEMSErrors 釘住錯誤碼走 AH，不是 CF。
func TestEMSErrors(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 0x9999
	if ah := emsCallAH(m, d, 0x4C00); ah != 0x83 {
		t.Errorf("無效 handle 回 AH=%02X，要 83h", ah)
	}
	m.CPU.R[cpu.BX] = 0xFFFF
	if ah := emsCallAH(m, d, 0x4300); ah != 0x87 {
		t.Errorf("要 65535 頁回 AH=%02X，要 87h", ah)
	}
	if ah := emsCallAH(m, d, 0x5A00); ah != 0x84 {
		t.Errorf("沒實作的功能回 AH=%02X，要 84h", ah)
	}
}

// TestEMSReleaseFlushes 釋放 handle 之前要把還映著的頁寫回。
func TestEMSReleaseFlushes(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 1
	emsCallAH(m, d, 0x4300)
	h := m.CPU.R[cpu.DX]
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = 0, h
	emsCallAH(m, d, 0x4400)
	m.Write8(uint32(machine.EMSFrameSeg)*16, 0x5A)

	m.CPU.R[cpu.DX] = h
	if ah := emsCallAH(m, d, 0x4500); ah != 0 {
		t.Fatalf("釋放失敗 %02X", ah)
	}
	if ah := emsCallAH(m, d, 0x4200); ah != 0 || m.CPU.R[cpu.DX] != emsTotalPages {
		t.Errorf("釋放後未配置頁數沒回到總數（BX=%d DX=%d）", m.CPU.R[cpu.BX], m.CPU.R[cpu.DX])
	}
}
