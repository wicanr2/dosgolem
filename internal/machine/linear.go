package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 線性層與保護模式 profile（docs/spec/432-linear-memory-and-paging §2、§3.2、§3.3、§4）。
//
// 「轉換開啟」（衍生欄位 m.xlat）＝ profile 開啟且 CR0.PE=1。其餘情形 Read8／Write8
// 與 432 之前逐位元組相同。
//
// 這一份是 432 實作的前半（C1）：兩層存取、setter、服務層位址、PM 編碼。分頁（§5）
// 尚未實作，所以 m.paging 恆為 false，轉換開啟時實體＝線性，再套 A20 遮罩。

const (
	// pmCR0Init 是 CR0 的初值。386 的 ET（bit 4）恆為 1，寫入時一律補上。
	pmCR0Init = 0x00000010
	cr0PE     = 1 << 0
	cr0PG     = 1 << 31
	// cr0Mask 是 386 有意義的位元：bit 0–4 與 bit 31。其餘（含 bit 16 WP）寫入時清為 0。
	cr0Mask = 0x8000001F

	eflagsHiRF = 1 << 0 // EFLAGS bit 16
	eflagsHiVM = 1 << 1 // EFLAGS bit 17
)

// recalcPM 重算衍生欄位（xlat、paging、cplUser），並同步取指令快路徑。
// 只在 setter、Restore、LoadState 呼叫；Read8／Write8 熱路徑只讀衍生欄位。
// 431 §4 規定 CR0、EFlagsHi、pmCPL 的所有寫入都經這些方法，所以衍生欄位不會過期。
func (m *Machine) recalcPM() {
	pe := m.cr0&cr0PE != 0
	m.xlat = m.pmProfile && pe
	// C2 起改為 m.xlat && m.cr0&cr0PG != 0。
	m.paging = false
	m.cplUser = false
	if pe {
		m.cplUser = m.eflagsHi&eflagsHiVM != 0 || m.pmCPL == 3
	}
	m.syncCodeFastPath()
}

// invalidateTLB 把世代號加一（uint32，溢位時整份清除）。TLB 本身在分頁實作（C2）加入。
func (m *Machine) invalidateTLB() {
	m.tlbGen++
	if m.tlbGen == 0 {
		m.tlbClear()
	}
}

// tlbClear 整份清除 TLB（世代號溢位時）。C2 填入。
func (m *Machine) tlbClear() { m.tlbGen = 1 }

var errNoProfile = fmt.Errorf("machine: 保護模式 profile 未開啟（先 SetProtectedModeProfile(true)）")

// SetProtectedModeProfile 開關保護模式 profile（預設關閉）。檢查順序：
//
//  1. programLoaded 為真，錯誤；
//  2. 開啟時 ExtendedMemoryKB() == 0，錯誤（否則 PE 由 0 變 1 時 HMA 內容會消失）；
//  3. len(m.Mem) != MemSize，錯誤；
//  4. 開啟時 CPU 型號不是 80386，錯誤；
//  5. 與現值不同時，CR0 必須是初值（432 §10 第 2 項）。
//
// 切換成功後 CR3、EFlagsHi、pmCPL 回到初值（它們只在 profile 開啟時才有意義）。
func (m *Machine) SetProtectedModeProfile(on bool) error {
	if m.programLoaded {
		return fmt.Errorf("machine: 程式已載入，不能再切換保護模式 profile")
	}
	if on && m.ExtendedMemoryKB() == 0 {
		return fmt.Errorf("machine: 保護模式 profile 需要延伸記憶體（先 SetExtendedMemoryKB），否則 PE 由 0 變 1 時 HMA 內容會消失")
	}
	if len(m.Mem) != MemSize {
		return fmt.Errorf("machine: 記憶體已不是 %d bytes（%d），不能開保護模式 profile", MemSize, len(m.Mem))
	}
	if on && m.CPU.Model != cpu.Model80386 {
		return fmt.Errorf("machine: 保護模式 profile 要求 CPU 型號為 80386")
	}
	if on == m.pmProfile {
		return nil
	}
	if m.cr0 != pmCR0Init {
		return fmt.Errorf("machine: CR0 不是初值（%08X），不能切換保護模式 profile", m.cr0)
	}
	m.pmProfile = on
	m.cr3, m.eflagsHi, m.pmCPL = 0, 0, 0
	m.invalidateTLB()
	m.recalcPM()
	return nil
}

// ProtectedModeProfile 回 profile 目前是否開啟。
func (m *Machine) ProtectedModeProfile() bool { return m.pmProfile }

// SetCR0 寫 CR0。一律補 00000010h，只保留 bit 0–4 與 bit 31（含 WP 在內的其餘位元清為 0）。
// PE 或 PG 改變時作廢 TLB。profile 關閉回錯誤；PE 由 1 寫成 0 而 VM=1 也回錯誤
// （431 §5：保護模式→實模式不支援）。
func (m *Machine) SetCR0(v uint32) error {
	if !m.pmProfile {
		return errNoProfile
	}
	v = v&cr0Mask | pmCR0Init
	if m.cr0&cr0PE != 0 && v&cr0PE == 0 && m.eflagsHi&eflagsHiVM != 0 {
		return fmt.Errorf("machine: VM=1 時不能把 CR0.PE 清成 0（保護模式→實模式不支援）")
	}
	if (m.cr0^v)&(cr0PE|cr0PG) != 0 {
		m.invalidateTLB()
	}
	m.cr0 = v
	m.recalcPM()
	return nil
}

// SetCR3 寫 CR3 並作廢 TLB。profile 關閉回錯誤。
func (m *Machine) SetCR3(v uint32) error {
	if !m.pmProfile {
		return errNoProfile
	}
	m.cr3 = v
	m.invalidateTLB()
	return nil
}

// SetEFlagsHi 寫 EFLAGS 的高 16 位元（EFlagsHi ＝ EFLAGS >> 16）。只接受 RF（bit 0）
// 與 VM（bit 1）；386 的 AC、ID 恆為 0。VM=1 需要 PE=1。
func (m *Machine) SetEFlagsHi(hi uint16) error {
	if !m.pmProfile {
		return errNoProfile
	}
	if hi&^(eflagsHiRF|eflagsHiVM) != 0 {
		return fmt.Errorf("machine: EFlagsHi %04X 含 RF、VM 以外的位元", hi)
	}
	if hi&eflagsHiVM != 0 && m.cr0&cr0PE == 0 {
		return fmt.Errorf("machine: CR0.PE=0 時不能設 VM")
	}
	m.eflagsHi = hi
	m.recalcPM()
	return nil
}

// SetVM 是 SetEFlagsHi 的包裝：只動 VM 位元，保留 RF。真正的進出 V86 由 435 與 S4 處理；
// 本規格的測試以它注入 V86 狀態，屬合成測試。
func (m *Machine) SetVM(on bool) error {
	hi := m.eflagsHi &^ eflagsHiVM
	if on {
		hi |= eflagsHiVM
	}
	return m.SetEFlagsHi(hi)
}

// SetPMCPL 寫保護模式的 CPL（由 434 的 cpu386 寫入；本規格內只供測試，預設 0）。
func (m *Machine) SetPMCPL(cpl uint8) error {
	if !m.pmProfile {
		return errNoProfile
	}
	if cpl > 3 {
		return fmt.Errorf("machine: CPL %d 超出 0..3", cpl)
	}
	m.pmCPL = cpl
	m.recalcPM()
	return nil
}

// CR0／CR3／PMCPL／EFlagsHi 回目前的值；Translating 回「轉換是否開啟」（m.xlat），
// 供 internal/dos 的診斷回溯選擇 Peek 或 Read8。
func (m *Machine) CR0() uint32       { return m.cr0 }
func (m *Machine) CR3() uint32       { return m.cr3 }
func (m *Machine) PMCPL() uint8      { return m.pmCPL }
func (m *Machine) EFlagsHi() uint16  { return m.eflagsHi }
func (m *Machine) Translating() bool { return m.xlat }

// readLin／writeLin 是轉換開啟時的線性層（xlat 為真才呼叫）。
// 線性位址 a → 分頁（m.paging，C2）→ 實體位址 → 套 A20：phys = a20 ? phys : phys &^ 0x100000
// → 實體層。分頁時 A20 套在轉換之後的實體位址上。
// user 是存取者身分（Read8／Write8 傳 m.cplUser，監督者存取傳 false）。
func (m *Machine) readLin(a uint32, user bool) uint8 {
	_ = user // C2 起供權限檢查使用
	if !m.a20 {
		a &^= 0x100000
	}
	return m.readPhysRaw(a)
}

func (m *Machine) writeLin(a uint32, v uint8, user bool) {
	_ = user
	if !m.a20 {
		a &^= 0x100000
	}
	m.writePhysRaw(a, v)
}

// 監督者存取（432 §3.3）：與線性層相同，但一律以監督者身分檢查權限（不論目前 CPL）。
// 供 435 經 IDT 遞送 V86 事件時讀 IDT、TSS、寫 ring 0 堆疊，434 的 cpu386 在 CPL=3
// 載入段描述元時讀 GDT、LDT，以及 435 的 I/O 權限點陣圖（§10 第 7 項）。
// 不採全域旗標，避免設定後漏還原。轉換關閉時與 Read8／Write8 相同。
//
// 多 byte 存取逐 byte 轉換，小端。

func (m *Machine) ReadSup8(a uint32) uint8 {
	if m.xlat {
		return m.readLin(a, false)
	}
	return m.Read8(a)
}

func (m *Machine) ReadSup16(a uint32) uint16 {
	return uint16(m.ReadSup8(a)) | uint16(m.ReadSup8(a+1))<<8
}

func (m *Machine) ReadSup32(a uint32) uint32 {
	return uint32(m.ReadSup16(a)) | uint32(m.ReadSup16(a+2))<<16
}

func (m *Machine) WriteSup8(a uint32, v uint8) {
	if m.xlat {
		m.writeLin(a, v, false)
		return
	}
	m.Write8(a, v)
}

func (m *Machine) WriteSup16(a uint32, v uint16) {
	m.WriteSup8(a, uint8(v))
	m.WriteSup8(a+1, uint8(v>>8))
}

func (m *Machine) WriteSup32(a uint32, v uint32) {
	m.WriteSup16(a, uint16(v))
	m.WriteSup16(a+2, uint16(v>>16))
}

// SvcAddr 是服務層（internal/dos）算客體指標的位址：轉換關閉時回 cpu.Addr(seg, off)
// （20 位元遮罩，行為不變）；轉換開啟時回 seg*16 + off（不遮罩，同 cpu.Linear）。
// 客體的 FFFF:0010 起在轉換開啟且 A20 開時是線性 0x100000 起，服務層不能折回 0。
// internal/dos 的非測試檔不得直接呼叫 cpu.Addr（internal/repolint 檢查）。
func (m *Machine) SvcAddr(seg, off uint16) uint32 {
	if m.xlat {
		return cpu.Linear(seg, off)
	}
	return cpu.Addr(seg, off)
}
