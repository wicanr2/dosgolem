package machine

import (
	"encoding/binary"
	"fmt"
)

// PM 欄位的內容（docs/spec/432-linear-memory-and-paging §7、§10 第 6 項）。
//
// machineState.PM 是不透明的 []byte，自帶版本號。版本 1 共 16 bytes，小端序：
//
//	| 位移 | 大小 | 欄位 |
//	|---|---|---|
//	| 0 | 2 | 版本號，1 |
//	| 2 | 1 | PMProfile（0 或 1） |
//	| 3 | 1 | pmCPL |
//	| 4 | 4 | CR0 |
//	| 8 | 4 | CR3 |
//	| 12 | 2 | EFlagsHi |
//	| 14 | 2 | 保留，寫 0 |
//
// 433–435 以提升版本號在尾端增加內容，讀取時支援所有舊版本（舊版本缺的欄位取初值）。
// profile 關閉時 PM 為 nil。

const (
	pmVersion1 = 1
	pmV1Size   = 16
)

// pmFields 是 PM 解碼後的內容。
type pmFields struct {
	cpl      uint8
	cr0, cr3 uint32
	eflagsHi uint16
}

// encodePM 編碼目前的保護模式狀態；profile 關閉回 nil。
func (m *Machine) encodePM() []byte {
	if !m.pmProfile {
		return nil
	}
	b := make([]byte, pmV1Size)
	le := binary.LittleEndian
	le.PutUint16(b[0:], pmVersion1)
	b[2] = 1
	b[3] = m.pmCPL
	le.PutUint32(b[4:], m.cr0)
	le.PutUint32(b[8:], m.cr3)
	le.PutUint16(b[12:], m.eflagsHi)
	return b
}

// decodePM 解碼並驗證 PM（全部驗證通過才回傳，呼叫端在任何寫入之前呼叫）。
// 錯誤條件：未知版本號、長度小於該版本的大小、PM[2]（PMProfile）不為 1、pmCPL > 3、
// CR0 含 386 保留位元、EFlagsHi 含 RF 與 VM 以外的位元、VM=1 而 PE=0。
func decodePM(b []byte) (pmFields, error) {
	var f pmFields
	if len(b) < 2 {
		return f, fmt.Errorf("machine: PM 長度 %d，連版本號都放不下", len(b))
	}
	le := binary.LittleEndian
	ver := le.Uint16(b)
	switch ver {
	case pmVersion1:
		if len(b) < pmV1Size {
			return f, fmt.Errorf("machine: PM 版本 1 需要 %d bytes，只有 %d", pmV1Size, len(b))
		}
	default:
		return f, fmt.Errorf("machine: PM 版本 %d 不認得", ver)
	}
	if b[2] != 1 {
		return f, fmt.Errorf("machine: PM 的 PMProfile 欄位是 %d，應為 1", b[2])
	}
	f.cpl = b[3]
	f.cr0 = le.Uint32(b[4:])
	f.cr3 = le.Uint32(b[8:])
	f.eflagsHi = le.Uint16(b[12:])
	if f.cpl > 3 {
		return f, fmt.Errorf("machine: PM 的 pmCPL %d 超出 0..3", f.cpl)
	}
	if f.cr0&^uint32(cr0Mask) != 0 {
		return f, fmt.Errorf("machine: PM 的 CR0 %08X 含 386 保留位元", f.cr0)
	}
	if f.eflagsHi&^uint16(eflagsHiRF|eflagsHiVM) != 0 {
		return f, fmt.Errorf("machine: PM 的 EFlagsHi %04X 含 RF、VM 以外的位元", f.eflagsHi)
	}
	if f.eflagsHi&eflagsHiVM != 0 && f.cr0&cr0PE == 0 {
		return f, fmt.Errorf("machine: PM 的 VM=1 而 CR0.PE=0")
	}
	return f, nil
}

// applyPM 套用解碼後的保護模式狀態（或在 profile 關閉時還原初值），
// 作廢 TLB 並重算衍生欄位。LoadState、Restore 共用。
func (m *Machine) applyPM(f pmFields) {
	m.cr0 = f.cr0 | pmCR0Init
	m.cr3, m.eflagsHi, m.pmCPL = f.cr3, f.eflagsHi, f.cpl
	m.invalidateTLB()
	m.recalcPM()
}
