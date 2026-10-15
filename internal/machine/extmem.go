package machine

import "fmt"

// 延伸實體記憶體、8042 A20 與 EMM 開關（`docs/spec/430-physical-memory-and-xms-unification`）。
//
// 三者都是**選項，預設關閉**：既有收據記的是當時的記憶體與狀態，預設行為一變，
// 舊收據重跑會得到另一個畫面而沒有任何測試報錯。
// `internal/state/baseline_anchor_test.go` 釘住預設值。

const (
	// ExtBase 是延伸記憶體的實體起點（1 MB）。
	ExtBase = 0x100000

	extMinKB = 4096
	extMaxKB = 64512 // 63 MB，XMS 以 KB 回報時不超過 65535（規格 §2.2 AH=08h）
)

// xmsHookEntry 是可掛鉤的 XMS 入口（8 bytes）：`jmp short +3` 越過三個 NOP，
// 落在 `int F5h / retf`。SCCD 讀到第一個 byte 是 EBh 就以 `入口+2+rel8` 為原處理常式，
// 並把入口前 5 bytes 改寫成 `EA` 遠跳（規格 §2.3）。
var xmsHookEntry = []byte{0xEB, 0x03, 0x90, 0x90, 0x90, 0xCD, 0xF5, 0xCB}

// SetExtendedMemoryKB 開啟 1 MB 之上的延伸記憶體。合法值：0（預設），或 4096..64512
// 且為 4 的倍數。檢查順序固定（規格 §2.1）：
//
//  1. programLoaded 為真，錯誤；
//  2. len(m.Mem) != MemSize（LE 路徑會增長 Mem），錯誤；
//  3. 範圍與倍數不合，錯誤；
//  4. 目前已是 kb > 0：任何呼叫（含 0 與同值）都錯誤。kb 仍為 0 時 SetExtendedMemoryKB(0) 是無害空呼叫。
//
// kb > 0 時完成三件事：建立 Ext（全 0，不套 MemPoison）、把現有 hma 內容複製到 Ext[0:HMASize]、
// 在 StubSeg:XMSHookEntryOff 寫入 8 bytes 入口（initVectors 在 New() 內、早於本設定，所以入口不由它寫）。
func (m *Machine) SetExtendedMemoryKB(kb int) error {
	if m.programLoaded {
		return fmt.Errorf("machine: 程式已載入，不能再設定延伸記憶體")
	}
	if len(m.Mem) != MemSize {
		return fmt.Errorf("machine: 記憶體已不是 %d bytes（%d），不能設定延伸記憶體", MemSize, len(m.Mem))
	}
	if kb != 0 && (kb < extMinKB || kb > extMaxKB || kb%4 != 0) {
		return fmt.Errorf("machine: 延伸記憶體 %d KB 超出範圍（0 或 %d..%d 且為 4 的倍數）", kb, extMinKB, extMaxKB)
	}
	if m.Ext != nil {
		return fmt.Errorf("machine: 延伸記憶體已設為 %d KB，不能再改", len(m.Ext)/1024)
	}
	if kb == 0 {
		return nil
	}
	m.Ext = make([]uint8, kb*1024)
	copy(m.Ext[:HMASize], m.hma[:])
	m.WriteBytes(uint32(StubSeg)*16+XMSHookEntryOff, xmsHookEntry)
	return nil
}

// ExtendedMemoryKB 回 SetExtendedMemoryKB 設的值（0 ＝ 關閉）。
func (m *Machine) ExtendedMemoryKB() int { return len(m.Ext) / 1024 }

// XMSEntryOff 回 `int 2Fh AX=4310h` 該交出的 XMS 入口位移（段是 StubSeg）：
// kb == 0 是 XMSTrapOff，kb > 0 是可掛鉤的 XMSHookEntryOff。
func (m *Machine) XMSEntryOff() uint16 {
	if m.Ext != nil {
		return XMSHookEntryOff
	}
	return XMSTrapOff
}

// ReadPhys8 以實體位址讀一個 byte，純記憶體視圖，**不套 A20 遮罩、不做分頁轉換**。
// 走實體層（physical.go，`docs/spec/432-linear-memory-and-paging` §3.1）：
//
//   - a < 1 MB：低記憶體路徑（含 VGA、ROM、監看）；
//   - 1 MB ≤ a < 1 MB + len(Ext)：讀 Ext；
//   - 其他：0xFF。
func (m *Machine) ReadPhys8(a uint32) uint8 { return m.readPhysRaw(a) }

// WritePhys8 以實體位址寫一個 byte；沒有對應記憶體的位址丟棄並累加 UnmappedPhysWrites。
func (m *Machine) WritePhys8(a uint32, v uint8) { m.writePhysRaw(a, v) }

// ReadPhys16／32 與 WritePhys16／32 逐 byte 組成，跨越區段邊界時各 byte 各自歸類。
func (m *Machine) ReadPhys16(a uint32) uint16 {
	return uint16(m.ReadPhys8(a)) | uint16(m.ReadPhys8(a+1))<<8
}

func (m *Machine) ReadPhys32(a uint32) uint32 {
	return uint32(m.ReadPhys16(a)) | uint32(m.ReadPhys16(a+2))<<16
}

func (m *Machine) WritePhys16(a uint32, v uint16) {
	m.WritePhys8(a, uint8(v))
	m.WritePhys8(a+1, uint8(v>>8))
}

func (m *Machine) WritePhys32(a uint32, v uint32) {
	m.WritePhys16(a, uint16(v))
	m.WritePhys16(a+2, uint16(v>>16))
}

// out8042 是 8042 的 A20 控制（規格 §2.4，kb > 0 時由 Out8 呼叫）：
//
//   - 任何 `out 64h,v` 先清除「等待輸出埠資料」旗標，`v == D1h` 再設起；
//   - 旗標為真時的下一個 `out 60h,v`：以 v 的位元 1 呼叫 SetA20，清除旗標。
//     位元 0 為 0（要求重設）只留在 PortLog，不重設（DOSBox-X `keyboard.cpp` 666–678 行）。
//
// 不支援 D0h、port 92h、`int 15h AX=2400h`–`2403h`。
func (m *Machine) out8042(port uint16, v uint8) {
	switch port {
	case 0x64:
		m.kbdAwaitOutport = v == 0xD1
	case 0x60:
		if m.kbdAwaitOutport {
			m.kbdAwaitOutport = false
			m.SetA20(v&2 != 0)
		}
	}
}

// SetEMSPresent 決定要不要呈現內建 EMM（預設呈現）。檢查順序（規格 §2.5）：
// programLoaded 為真一律錯誤，不論引數；其後依目前值：
// 目前呈現而 present == true 不做事；目前呈現而 present == false 執行撤銷；
// 目前不呈現而 present == false 不做事；目前不呈現而 present == true 錯誤。
//
// 撤銷 initVectors 寫入的三項：EMSSeg 的 `CD F6 CF`、`EMMXXXX0` 簽章（都還原為 MemPoison 值），
// int 67h 向量改指 StubSeg:StubOff(0x67)。`internal/dos` 在 emsCall 入口看 EMSPresent 決定要不要服務。
func (m *Machine) SetEMSPresent(present bool) error {
	if m.programLoaded {
		return fmt.Errorf("machine: 程式已載入，不能再設定 EMM")
	}
	if m.emsAbsent {
		if present {
			return fmt.Errorf("machine: EMM 已撤銷，不能再呈現")
		}
		return nil
	}
	if present {
		return nil
	}
	m.emsAbsent = true
	base := uint32(EMSSeg) * 16
	for i := uint32(0); i < 3; i++ {
		m.Mem[base+i] = MemPoison
	}
	for i := uint32(0); i < 8; i++ {
		m.Mem[base+0x0A+i] = MemPoison
	}
	m.Write16(0x67*4, StubOff(0x67))
	m.Write16(0x67*4+2, StubSeg)
	return nil
}

// EMSPresent 回內建 EMM 目前有沒有呈現。
func (m *Machine) EMSPresent() bool { return !m.emsAbsent }
