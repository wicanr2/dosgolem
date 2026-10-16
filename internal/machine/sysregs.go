package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 433 §2、§3.1：Machine 實作既有核心的 SysBus。只在保護模式 profile 開啟時裝在
// m.CPU.Sys（SetProtectedModeProfile）。所有寫入經這些方法（431 §4）：CR0、CR3 一律
// 轉給 432 的 SetCR0、SetCR3，衍生欄位因此不會過期。

var _ cpu.SysBus = (*Machine)(nil)

// CR 回 CRn 的原始值（CR0、CR2、CR3）。CR1 與 CR4 起回錯誤（386 沒有，431 §2）。
func (m *Machine) CR(n int) (uint32, error) {
	switch n {
	case 0:
		return m.cr0, nil
	case 2:
		return m.sys.cr2, nil
	case 3:
		return m.cr3, nil
	}
	return 0, fmt.Errorf("machine: CR%d 在 386 不存在", n)
}

// SetCR 寫 CRn：CR0 走 SetCR0、CR3 走 SetCR3、CR2 照存；其餘回錯誤。
func (m *Machine) SetCR(n int, v uint32) error {
	switch n {
	case 0:
		return m.SetCR0(v)
	case 2:
		if !m.pmProfile {
			return errNoProfile
		}
		m.sys.cr2 = v
		return nil
	case 3:
		return m.SetCR3(v)
	}
	return fmt.Errorf("machine: CR%d 在 386 不存在", n)
}

// DR 讀 DRn（n 為 0–7）。DR4 是 DR6 的別名、DR5 是 DR7 的別名。
func (m *Machine) DR(n int) uint32 {
	switch n & 7 {
	case 4, 6:
		return m.sys.dr6
	case 5, 7:
		return m.sys.dr7
	}
	return m.sys.dr[n&3]
}

// SetDR 寫 DRn。DR6 寫入為 (v|FFFF0FF0h)&FFFFEFFFh，DR7 寫入為 (v|400h)&FFFF2FFFh
// （DOSBox-X cpu/cpu.cpp:2708、2713）。DR7 只保存，不觸發 #DB（後續 S6）。
func (m *Machine) SetDR(n int, v uint32) {
	switch n & 7 {
	case 4, 6:
		m.sys.dr6 = (v | dr6WriteSet) & dr6WriteMask
	case 5, 7:
		m.sys.dr7 = (v | dr7WriteSet) & dr7WriteMask
	default:
		m.sys.dr[n&3] = v
	}
}

// GDTR／IDTR 回描述表暫存器（基底 32 位元、界限 16 位元）。
func (m *Machine) GDTR() (base uint32, limit uint16) { return m.sys.gdtrBase, m.sys.gdtrLimit }
func (m *Machine) IDTR() (base uint32, limit uint16) { return m.sys.idtrBase, m.sys.idtrLimit }

// SetGDTR／SetIDTR 寫描述表暫存器。24 位元或 32 位元基底的取捨在指令處理函式做。
func (m *Machine) SetGDTR(base uint32, limit uint16) { m.sys.gdtrBase, m.sys.gdtrLimit = base, limit }
func (m *Machine) SetIDTR(base uint32, limit uint16) { m.sys.idtrBase, m.sys.idtrLimit = base, limit }

// User 回目前是否為使用者模式（432 的 cplUser）。實模式恆為 false。
func (m *Machine) User() bool { return m.cplUser }

// IVTMismatch 是 433 §2 的 IDTR 守門條件的唯一定義：profile 開啟、仍在實模式、
// IDTR 基底不為 0。成立時實模式的 INT 與 IRQ 失敗即關閉（dosgolem 一律讀 0000:0000 的 IVT，
// 真機與 DOSBox-X 以 IDTR 基底取向量）。
func (m *Machine) IVTMismatch() bool {
	return m.pmProfile && m.cr0&cr0PE == 0 && m.sys.idtrBase != 0
}

// CPL 回目前的 CPL（434 §5.2）。
func (m *Machine) CPL() uint8 { return m.pmCPL }

// LDTR／SetLDTR 讀寫 LDTR 的選擇子與快取；TRSel 回 TR 的選擇子（434 §5.2）。
func (m *Machine) LDTR() (sel uint16, c SegCache) { return m.sys.ldtr.sel, m.sys.ldtr.cache }
func (m *Machine) SetLDTR(sel uint16, c SegCache) { m.sys.ldtr = sysSeg{sel: sel, cache: c} }
func (m *Machine) TRSel() uint16                  { return m.sys.tr.sel }

// ivtGuardIRQ 是 IDTR 守門的 Machine 側（tick 的 INT 08h、keyTick 與 timedKeyTick 的 INT 09h）。
// CPU.Interrupt 沒有回傳值，所以條件成立時不呼叫它，把錯誤存進 m.irqErr，由 Step 回傳。
func (m *Machine) ivtGuardIRQ(vec uint8) bool {
	if !m.IVTMismatch() {
		return false
	}
	if m.irqErr == nil {
		m.irqErr = fmt.Errorf("machine: IRQ（INT %02Xh）：實模式下 IDTR 基底 %08X（界限 %04X）不為 0，"+
			"dosgolem 只讀 0000:0000 的 IVT，失敗即關閉（docs/spec/433 §2）", vec, m.sys.idtrBase, m.sys.idtrLimit)
	}
	return true
}

// takeIRQErr 取出並清掉 m.irqErr。
func (m *Machine) takeIRQErr() error {
	err := m.irqErr
	m.irqErr = nil
	return err
}
