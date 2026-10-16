package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// PM 欄位的內容（docs/spec/432-linear-memory-and-paging §7、§10 第 6 項；
// docs/spec/433-real-mode-system-instructions §2.1；docs/spec/434-legacy-core-segmented-mode §2.2）。
//
// machineState.PM 是不透明的 []byte，自帶版本號。encodePM 一律輸出版本 3（142 bytes，小端序）；
// decodePM 支援版本 1（16 bytes，缺的欄位取 sysRegsInit 的初值）、版本 2（80 bytes，
// segmented=false、快取全 0）與版本 3：
//
//	| 位移 | 大小 | 欄位 |
//	|---|---|---|
//	| 0 | 2 | 版本號（1、2 或 3） |
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
//	| 80 | 1 | segmented（0 或 1；以下為版本 3） |
//	| 81 | 1 | 保留，0 |
//	| 82 | 60 | cache[0..5]（ES、CS、SS、DS、FS、GS），每項 Base 4、Limit 4、Attr 2 |
//
// 435 之後以提升版本號在尾端增加內容，讀取時支援所有舊版本（舊版本缺的欄位取初值）。
// 尾端多出的 bytes 不報錯（432 實作收據第 2 項）。profile 關閉時 PM 為 nil。

const (
	pmVersion1 = 1
	pmV1Size   = 16
	pmVersion2 = 2
	pmV2Size   = 80
	pmVersion3 = 3
	pmV3Size   = 142
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

// SegCache 是段描述元快取（433 §3.2 的型別與編碼）。434 §2 移到 internal/cpu，這裡以型別別名沿用。
type SegCache = cpu.SegCache

// segAttrDB 是快取屬性的 D/B 位元（bit 14）。
const segAttrDB = 1 << 14

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

// pmFields 是 PM 解碼後的內容。segmented 與 cache 是既有核心的分段模式狀態（434 §2.2），
// Snapshot 經 CPU 的 Segmented／SegCache 取得，applyPM 以 SetSegmented／SetSegCache 還原。
type pmFields struct {
	cpl      uint8
	cr0, cr3 uint32
	eflagsHi uint16
	sysRegs
	segmented bool
	cache     [6]SegCache
}

// cpuPMFields 取既有核心的分段模式狀態（Snapshot、encodePM 共用）。
func (m *Machine) cpuPMFields() (seg bool, cache [6]SegCache) {
	for i := range cache {
		cache[i] = m.CPU.SegCache(i)
	}
	return m.CPU.Segmented(), cache
}

// encodePM 編碼目前的保護模式狀態（版本 3）；profile 關閉回 nil。
func (m *Machine) encodePM() []byte {
	if !m.pmProfile {
		return nil
	}
	b := make([]byte, pmV3Size)
	le := binary.LittleEndian
	le.PutUint16(b[0:], pmVersion3)
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
	seg, cache := m.cpuPMFields()
	if seg {
		b[80] = 1
	}
	for i, c := range cache {
		putSegCache(b[82+10*i:], c)
	}
	return b
}

// putSegCache／getSegCache 是分段模式快取的 10 bytes：基底 4、界限 4、屬性 2。
func putSegCache(b []byte, c SegCache) {
	le := binary.LittleEndian
	le.PutUint32(b[0:], c.Base)
	le.PutUint32(b[4:], c.Limit)
	le.PutUint16(b[8:], c.Attr)
}

func getSegCache(b []byte) SegCache {
	le := binary.LittleEndian
	return SegCache{Base: le.Uint32(b[0:]), Limit: le.Uint32(b[4:]), Attr: le.Uint16(b[8:])}
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
// 版本 3 另加（434 §2.2）：segmented 只能是 0 或 1；保留 byte 為 0；segmented=0 時六個快取全 0；
// segmented=1 時 PE=1、VM=0、pmCPL=0；各快取 Attr 的 bit 8–11、13 為 0；cache[CS]、cache[SS] 的 D/B 為 0。
// 版本 1 沒有的欄位取 sysRegsInit 的初值；版本 1、2 沒有的分段模式狀態為 segmented=false、快取全 0。
//
// 「版本 1、2 帶 PE=1、VM=0」的拒絕不在這裡，在 LoadState（434 §2.1），直接呼叫 decodePM 的測試不受影響。
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
	case pmVersion3:
		if len(b) < pmV3Size {
			return f, fmt.Errorf("machine: PM 版本 3 需要 %d bytes，只有 %d", pmV3Size, len(b))
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
	if ver == pmVersion2 {
		return f, nil
	}
	return f, decodePMV3(b, &f)
}

// decodePMV3 解碼並驗證版本 3 增加的分段模式狀態（434 §2.2）。
func decodePMV3(b []byte, f *pmFields) error {
	switch b[80] {
	case 0, 1:
	default:
		return fmt.Errorf("machine: PM 的 segmented 欄位是 %d，應為 0 或 1", b[80])
	}
	if b[81] != 0 {
		return fmt.Errorf("machine: PM 的位移 81 保留 byte 是 %d，應為 0", b[81])
	}
	f.segmented = b[80] == 1
	for i := range f.cache {
		f.cache[i] = getSegCache(b[82+10*i:])
	}
	if !f.segmented {
		for i, c := range f.cache {
			if c != (SegCache{}) {
				return fmt.Errorf("machine: PM 的 segmented=0，但快取 %d 不為 0（%+v）", i, c)
			}
		}
		return nil
	}
	if f.cr0&cr0PE == 0 || f.eflagsHi&eflagsHiVM != 0 || f.cpl != 0 {
		return fmt.Errorf("machine: PM 的 segmented=1 要求 PE=1、VM=0、pmCPL=0（CR0=%08X EFlagsHi=%04X CPL=%d）",
			f.cr0, f.eflagsHi, f.cpl)
	}
	for i, c := range f.cache {
		if c.Attr&segAttrZero != 0 {
			return fmt.Errorf("machine: PM 的快取 %d 屬性 %04X 的 bit 8–11 或 bit 13 不為 0", i, c.Attr)
		}
	}
	for _, i := range []int{cpu.CS, cpu.SS} {
		if f.cache[i].Attr&segAttrDB != 0 {
			return fmt.Errorf("machine: PM 的快取 %d（CS 或 SS）屬性 %04X 的 D/B 為 1", i, f.cache[i].Attr)
		}
	}
	return nil
}

// applyPM 套用解碼後的保護模式狀態（或在 profile 關閉時還原初值），清空分頁錯誤紀錄，
// 作廢 TLB 並重算衍生欄位。LoadState、Restore 共用。分段模式狀態依 PM 的 segmented 欄位
// 還原，不重新推導（434 §3.2）。
func (m *Machine) applyPM(f pmFields) {
	m.cr0 = f.cr0 | pmCR0Init
	m.cr3, m.eflagsHi, m.pmCPL = f.cr3, f.eflagsHi, f.cpl
	m.sys = f.sysRegs // 433 §2.1：整份複製
	m.CPU.SetSegmented(f.segmented)
	for i, c := range f.cache {
		m.CPU.SetSegCache(i, c)
	}
	m.resetSegWait()
	m.invalidateTLB()
	m.clearPageFaults()
	m.recalcPM()
}
