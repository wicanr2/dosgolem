package machine

import (
	"encoding/binary"
	"fmt"
)

// PM 欄位的內容（docs/spec/432-linear-memory-and-paging §7、§10 第 6 項；
// docs/spec/433-real-mode-system-instructions §2.1）。
//
// machineState.PM 是不透明的 []byte，自帶版本號。encodePM 一律輸出版本 2（80 bytes，小端序）；
// decodePM 支援版本 1（16 bytes，缺的欄位取 sysRegsInit 的初值）與版本 2：
//
//	| 位移 | 大小 | 欄位 |
//	|---|---|---|
//	| 0 | 2 | 版本號（1 或 2） |
//	| 2 | 1 | PMProfile（0 或 1） |
//	| 3 | 1 | pmCPL |
//	| 4 | 4 | CR0 |
//	| 8 | 4 | CR3 |
//	| 12 | 2 | EFlagsHi |
//	| 14 | 2 | 保留，寫 0 |
//	| 16 | 4 | CR2（以下為版本 2） |
//	| 20、24、28、32 | 各 4 | DR0–DR3 |
//	| 36 | 4 | DR6 |
//	| 40 | 4 | DR7 |
//	| 44 | 4 | GDTR 基底 |
//	| 48 | 2 | GDTR 界限 |
//	| 50 | 4 | IDTR 基底 |
//	| 54 | 2 | IDTR 界限 |
//	| 56 | 2 | LDTR 選擇子 |
//	| 58 | 4 | LDTR 基底 |
//	| 62 | 4 | LDTR 界限 |
//	| 66 | 2 | LDTR 屬性 |
//	| 68 | 2 | TR 選擇子 |
//	| 70 | 4 | TR 基底 |
//	| 74 | 4 | TR 界限 |
//	| 78 | 2 | TR 屬性 |
//
// 435 之後以提升版本號在尾端增加內容，讀取時支援所有舊版本（舊版本缺的欄位取初值）。
// 尾端多出的 bytes 不報錯（432 實作收據第 2 項）。profile 關閉時 PM 為 nil。

const (
	pmVersion1 = 1
	pmV1Size   = 16
	pmVersion2 = 2
	pmV2Size   = 80
)

// SysRegs 的初值與寫入規則（433 §2）。
const (
	dr6Init      = 0xFFFF1FF0 // cpu.cpp:3651
	dr6WriteSet  = 0xFFFF0FF0 // 寫入為 (v|FFFF0FF0h)&FFFFEFFFh（cpu.cpp:2708）
	dr6WriteMask = 0xFFFFEFFF
	dr7Init      = 0x00000400 // cpu.cpp:3653
	dr7WriteSet  = 0x00000400 // 寫入為 (v|400h)&FFFF2FFFh（cpu.cpp:2713）
	dr7WriteMask = 0xFFFF2FFF
	idtrInitLim  = 0x03FF // cpu.cpp:3641-3642
	// segAttrZero 是段快取屬性中恆為 0 的位元：bits 8–11 與 bit 13（433 §3.2 的 Attr 佈局）。
	segAttrZero = 0x2F00
)

// SegCache 是段描述元快取（433 §3.2）。Attr：bits 0–7 為 386 描述元的 access byte；
// bits 8–11 恆 0；bit 12 AVL；bit 13 恆 0；bit 14 D/B；bit 15 G。Limit 是已依 G 展開的
// 有效界限。LDTR、TR 的屬性用同一編碼。
type SegCache struct {
	Base, Limit uint32
	Attr        uint16
}

// sysSeg 是 LDTR、TR：選擇子＋快取。433 只保存，載入在 434 與後續 S4。
type sysSeg struct {
	sel   uint16
	cache SegCache
}

// sysRegs 收齊 433 §2 新增的系統暫存器（CR0、CR3、EFlagsHi、pmCPL 在 432 已有，不在這裡）。
// Machine 持有一份；pmFields 內嵌它，Snapshot 與 applyPM 整份複製，不逐欄賦值。
type sysRegs struct {
	cr2       uint32
	dr        [4]uint32 // DR0–DR3
	dr6, dr7  uint32
	gdtrBase  uint32
	gdtrLimit uint16
	idtrBase  uint32
	idtrLimit uint16
	ldtr, tr  sysSeg
}

// sysRegsInit 回 433 §2 的初值：DR6 FFFF1FF0h、DR7 400h、IDTR 界限 3FFh，其餘 0。
func sysRegsInit() sysRegs {
	return sysRegs{dr6: dr6Init, dr7: dr7Init, idtrLimit: idtrInitLim}
}

// pmFields 是 PM 解碼後的內容。
type pmFields struct {
	cpl      uint8
	cr0, cr3 uint32
	eflagsHi uint16
	sysRegs
}

// encodePM 編碼目前的保護模式狀態（版本 2）；profile 關閉回 nil。
func (m *Machine) encodePM() []byte {
	if !m.pmProfile {
		return nil
	}
	b := make([]byte, pmV2Size)
	le := binary.LittleEndian
	le.PutUint16(b[0:], pmVersion2)
	b[2] = 1
	b[3] = m.pmCPL
	le.PutUint32(b[4:], m.cr0)
	le.PutUint32(b[8:], m.cr3)
	le.PutUint16(b[12:], m.eflagsHi)
	r := &m.sys
	le.PutUint32(b[16:], r.cr2)
	for i, v := range r.dr {
		le.PutUint32(b[20+4*i:], v)
	}
	le.PutUint32(b[36:], r.dr6)
	le.PutUint32(b[40:], r.dr7)
	le.PutUint32(b[44:], r.gdtrBase)
	le.PutUint16(b[48:], r.gdtrLimit)
	le.PutUint32(b[50:], r.idtrBase)
	le.PutUint16(b[54:], r.idtrLimit)
	putSysSeg(b[56:], r.ldtr)
	putSysSeg(b[68:], r.tr)
	return b
}

// putSysSeg／getSysSeg 是 LDTR、TR 的 12 bytes：選擇子 2、基底 4、界限 4、屬性 2。
func putSysSeg(b []byte, s sysSeg) {
	le := binary.LittleEndian
	le.PutUint16(b[0:], s.sel)
	le.PutUint32(b[2:], s.cache.Base)
	le.PutUint32(b[6:], s.cache.Limit)
	le.PutUint16(b[10:], s.cache.Attr)
}

func getSysSeg(b []byte) sysSeg {
	le := binary.LittleEndian
	return sysSeg{sel: le.Uint16(b[0:]), cache: SegCache{
		Base: le.Uint32(b[2:]), Limit: le.Uint32(b[6:]), Attr: le.Uint16(b[10:]),
	}}
}

// decodePM 解碼並驗證 PM（全部驗證通過才回傳，呼叫端在任何寫入之前呼叫）。
// 錯誤條件（432 §10 第 6 項）：未知版本號、長度小於該版本的大小、PM[2]（PMProfile）不為 1、
// pmCPL > 3、CR0 含 386 保留位元、EFlagsHi 含 RF 與 VM 以外的位元、VM=1 而 PE=0。
// 版本 2 另加（433 §2.1）：DR6&FFFF0FF0h != FFFF0FF0h（bit 12 不驗）；DR7 的 bit 10 不為 1
// 或 bit 12、14、15 不為 0；LDTR、TR 屬性的 bit 8–11 或 bit 13 不為 0。
// 版本 1 沒有的欄位取 sysRegsInit 的初值。
func decodePM(b []byte) (pmFields, error) {
	f := pmFields{sysRegs: sysRegsInit()}
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
	case pmVersion2:
		if len(b) < pmV2Size {
			return f, fmt.Errorf("machine: PM 版本 2 需要 %d bytes，只有 %d", pmV2Size, len(b))
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
	if ver == pmVersion1 {
		return f, nil
	}
	r := &f.sysRegs
	r.cr2 = le.Uint32(b[16:])
	for i := range r.dr {
		r.dr[i] = le.Uint32(b[20+4*i:])
	}
	r.dr6 = le.Uint32(b[36:])
	r.dr7 = le.Uint32(b[40:])
	r.gdtrBase = le.Uint32(b[44:])
	r.gdtrLimit = le.Uint16(b[48:])
	r.idtrBase = le.Uint32(b[50:])
	r.idtrLimit = le.Uint16(b[54:])
	r.ldtr = getSysSeg(b[56:])
	r.tr = getSysSeg(b[68:])
	if r.dr6&dr6WriteSet != dr6WriteSet {
		return f, fmt.Errorf("machine: PM 的 DR6 %08X 缺固定為 1 的位元（FFFF0FF0h）", r.dr6)
	}
	if r.dr7&0x400 == 0 || r.dr7&0xD000 != 0 {
		return f, fmt.Errorf("machine: PM 的 DR7 %08X 的 bit 10 應為 1、bit 12、14、15 應為 0", r.dr7)
	}
	if r.ldtr.cache.Attr&segAttrZero != 0 {
		return f, fmt.Errorf("machine: PM 的 LDTR 屬性 %04X 的 bit 8–11 或 bit 13 不為 0", r.ldtr.cache.Attr)
	}
	if r.tr.cache.Attr&segAttrZero != 0 {
		return f, fmt.Errorf("machine: PM 的 TR 屬性 %04X 的 bit 8–11 或 bit 13 不為 0", r.tr.cache.Attr)
	}
	return f, nil
}

// applyPM 套用解碼後的保護模式狀態（或在 profile 關閉時還原初值），清空分頁錯誤紀錄，
// 作廢 TLB 並重算衍生欄位。LoadState、Restore 共用。
func (m *Machine) applyPM(f pmFields) {
	m.cr0 = f.cr0 | pmCR0Init
	m.cr3, m.eflagsHi, m.pmCPL = f.cr3, f.eflagsHi, f.cpl
	m.sys = f.sysRegs // 433 §2.1：整份複製
	m.invalidateTLB()
	m.clearPageFaults()
	m.recalcPM()
}
