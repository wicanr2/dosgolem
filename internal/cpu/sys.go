package cpu

import "errors"

// SysBus 是既有核心存取系統暫存器的介面（docs/spec/433-real-mode-system-instructions §3.1）。
//
// 由 machine.Machine 實作，只在保護模式 profile 開啟時裝在 CPU.Sys；nil 時行為完全不變。
// 所有寫入都經這裡，由 Machine 的方法重算衍生欄位（431 §4）。
type SysBus interface {
	// CR 回 CRn 的原始值；mov r32,cr0 的 |7FFFFFF0h 與 CR3 的遮罩在指令處理函式做。
	// 不存在的 CRn（CR1、CR4 起）回錯誤。
	CR(n int) (uint32, error)
	SetCR(n int, v uint32) error
	// DR 讀寫 DR0–DR7（DR4、DR5 是 DR6、DR7 的別名）。
	DR(n int) uint32
	SetDR(n int, v uint32)
	GDTR() (base uint32, limit uint16)
	SetGDTR(base uint32, limit uint16)
	IDTR() (base uint32, limit uint16)
	SetIDTR(base uint32, limit uint16)
	// User 回目前是否為使用者模式（432 的 cplUser）。實模式恆為 false。
	User() bool
	// IVTMismatch 是 433 §2 的 IDTR 守門條件：profile 開啟、PE=0 而 IDTR 基底不為 0。
	IVTMismatch() bool

	// 434 §5.2：分段模式的段載入用。
	// CPL 回目前的 CPL（Machine 的 pmCPL）。
	CPL() uint8
	// ReadSup32 以監督者身分經 432 的線性層讀 4 bytes（讀 GDT、LDT 描述元）。
	ReadSup32(addr uint32) uint32
	// LDTR／SetLDTR 讀寫 LDTR 的選擇子與快取；TRSel 回 TR 的選擇子。
	LDTR() (sel uint16, c SegCache)
	SetLDTR(sel uint16, c SegCache)
	TRSel() uint16

	// 436 §2.1：ltr、任務返回與 V86。
	// VM 回 EFLAGS.VM；核心判定 V86 一律用 c.Sys != nil && c.Sys.VM()。
	VM() bool
	// TR／SetTR 讀寫 TR 的選擇子與快取。
	TR() (sel uint16, c SegCache)
	SetTR(sel uint16, c SegCache)
	// ReadSup8、ReadSup16、WriteSup8：監督者身分經 432 的線性層存取（GDT 的忙碌位元、TSS 的 I/O 點陣圖）。
	ReadSup8(addr uint32) uint8
	ReadSup16(addr uint32) uint16
	WriteSup8(addr uint32, v uint8)
}

// ErrModeSwitch 是實模式→保護模式的交棒哨兵（433 §5、431 §5）：`mov cr0` 或 `lmsw` 使 CR0.PE
// 由 0 變 1 時，該道指令照常完成（IP 前進、CR0 更新）後由 Step 回傳。只改 TS、MP 或 PE 不變時不回。
var ErrModeSwitch = errors.New("cpu: CR0.PE 由 0 變 1，交棒給保護模式核心（docs/spec/433 §5）")

// ErrTaskReturn 是任務返回的哨兵（436 §3.1）：分段模式下 iret（CF）或 iretd（66 CF）且 EFLAGS.NT=1 時，
// 核心把 IP 設回本道指令起點、記下下一道的位址（TaskReturnNextIP）後回傳，其他狀態都不改。
// 任務切換由 Machine 做（436 §3.2）。
var ErrTaskReturn = errors.New("cpu: 分段模式 iret 且 NT=1，任務返回交給 Machine（docs/spec/436 §3）")

const (
	cr0PE = 1 << 0
	cr0TS = 1 << 3
)

// ivtGuard 是 433 §2 的 IDTR 守門（CPU 側）：profile 開啟、仍在實模式、IDTR 基底不為 0 時，
// 實模式的 INT 與例外失敗即關閉（dosgolem 一律讀 0000:0000 的 IVT，真機與 DOSBox-X 以 IDTR 基底取向量，
// DOSBox-X cpu/cpu.cpp:1294-1296）。成立時以 failf 記下原因並回 true，呼叫端不得呼叫 Interrupt。
//
// 434 §5.3 擴充：分段模式下同樣失敗即關閉（保護模式 IDT 遞送屬 435／S6）。
func (c *CPU) ivtGuard(n uint8, what string) bool {
	if c.segmented {
		c.failf("%s（INT %02Xh）：分段模式下的中斷與例外經保護模式 IDT 遞送，屬 435／S6，失敗即關閉"+
			"（docs/spec/434 §5.3）", what, n)
		return true
	}
	if c.Sys == nil || !c.Sys.IVTMismatch() {
		return false
	}
	base, limit := c.Sys.IDTR()
	c.failf("%s（INT %02Xh）：實模式下 IDTR 基底 %08X（界限 %04X）不為 0，dosgolem 只讀 0000:0000 的 IVT，"+
		"失敗即關閉（docs/spec/433 §2）", what, n, base, limit)
	return true
}

// execSys0F 處理 433 §4 的系統指令。只在 c.Sys 非 nil 時由 execute0F 的 default 分支呼叫；
// handled 為 false 表示不歸這裡管，呼叫端照現況失敗即關閉。
func (c *CPU) execSys0F(op2 uint8) (handled bool, err error) {
	switch op2 {
	case 0x00, 0x02, 0x03:
		if c.segmented {
			// 434 §6：分段模式的 0F 00 群組在 seg0F00；LAR、LSL 失敗即關閉（JEMM 沒用）。
			if op2 == 0x00 {
				return true, c.seg0F00()
			}
			return true, c.errf(op2, "0F %02X：分段模式的 LAR、LSL 失敗即關閉（JEMM 沒用，docs/spec/434 §6）", op2)
		}
		// 0F 00 整組、LAR、LSL：DOSBox-X 在實模式送 IVT 第 6 項（prefix_0f.h:22、128、141）；刻意從嚴。
		return true, c.errf(op2, "0F %02X：實模式的 0F 00 群組、LAR、LSL 失敗即關閉"+
			"（DOSBox-X 送 IVT 第 6 項；刻意從嚴，docs/spec/433 §4）", op2)
	case 0x01:
		return true, c.sys0F01()
	case 0x06: // CLTS
		if err := c.privileged(op2, "CLTS"); err != nil {
			return true, err
		}
		cr0, err := c.Sys.CR(0)
		if err == nil {
			err = c.Sys.SetCR(0, cr0&^cr0TS)
		}
		if err != nil {
			return true, c.errf(op2, "0F 06 CLTS：%v", err)
		}
		return true, nil
	case 0x20, 0x21, 0x22, 0x23:
		return true, c.sysMovCRDR(op2)
	case 0x24, 0x26:
		// TRx：DOSBox-X 對 TR6、TR7 在實模式照常執行，其餘 #UD（prefix_0f.h:222-250）；刻意從嚴。
		return true, c.errf(op2, "0F %02X：MOV TRx 失敗即關閉（刻意從嚴，docs/spec/433 §4）", op2)
	}
	return false, nil
}

// privileged 在 Sys.User() 為真時回錯誤（V86 的特權指令路徑在 435）。實模式恆可執行。
func (c *CPU) privileged(op2 uint8, name string) error {
	if c.Sys.User() {
		return c.errf(op2, "0F %02X %s：使用者模式下的特權指令失敗即關閉（V86 路徑在 435）", op2, name)
	}
	return nil
}

// sys0F01 是 0F 01 群組：SGDT、SIDT、LGDT、LIDT、SMSW、LMSW（DOSBox-X prefix_0f.h:72-124、
// prefix_66_0f.h:72-118）。
func (c *CPU) sys0F01() error {
	const op2 = 0x01
	m := c.decodeModRM()
	sub := m.reg
	if m.rm.isReg && sub != 4 && sub != 6 {
		return c.errf(op2, "0F 01 /%d：暫存器運算元失敗即關閉（docs/spec/433 §4）", sub)
	}
	switch sub {
	case 0, 1: // SGDT／SIDT：16 位元界限＋完整 32 位元基底（386 不遮成 24 位元）
		var base uint32
		var limit uint16
		if sub == 0 {
			base, limit = c.Sys.GDTR()
		} else {
			base, limit = c.Sys.IDTR()
		}
		c.set16(m.rm, limit)
		c.set32(m.rm.at(2), base)
	case 2, 3: // LGDT／LIDT：16 位元運算元只取 24 位元基底，66 前綴取整個 dword
		name := "LGDT"
		if sub == 3 {
			name = "LIDT"
		}
		if err := c.privileged(op2, name); err != nil {
			return err
		}
		limit := c.get16(m.rm)
		base := c.get32(m.rm.at(2))
		if !c.operand32 {
			base &= 0x00FFFFFF
		}
		if sub == 2 {
			c.Sys.SetGDTR(base, limit)
		} else {
			c.Sys.SetIDTR(base, limit)
		}
	case 4: // SMSW：讀原始 CR0（cpu.cpp:2784）
		cr0, err := c.Sys.CR(0)
		if err != nil {
			return c.errf(op2, "0F 01 /4 SMSW：%v", err)
		}
		switch {
		case !m.rm.isReg:
			c.set16(m.rm, uint16(cr0)) // 記憶體形式一律 2 bytes
		case c.operand32:
			c.setReg32(m.rm.reg, cr0) // 66 前綴的暫存器形式寫整個 CR0
		default:
			c.R[m.rm.reg] = uint16(cr0) // 16 位元暫存器：保留高半
		}
	case 6: // LMSW：只改 CR0 低 4 位元，不能清除 PE（cpu.cpp:2788-2796）
		if err := c.privileged(op2, "LMSW"); err != nil {
			return err
		}
		v := c.get16(m.rm)
		old, err := c.Sys.CR(0)
		if err != nil {
			return c.errf(op2, "0F 01 /6 LMSW：%v", err)
		}
		w := uint32(v)
		if old&cr0PE != 0 {
			w |= cr0PE
		}
		return c.setCR0(op2, "LMSW", old, old&^0xF|w&0xF)
	default: // /5、/7
		return c.errf(op2, "0F 01 /%d：失敗即關閉（DOSBox-X 對記憶體 /5 靜默略過、/7 INVLPG 在 386 為 #UD；"+
			"刻意從嚴，docs/spec/433 §4）", sub)
	}
	return nil
}

// setCR0 經 Sys 寫 CR0，PE 由 0 變 1 時回 ErrModeSwitch（433 §5：寫入前後比較 PE）。
func (c *CPU) setCR0(op2 uint8, name string, old, v uint32) error {
	if err := c.Sys.SetCR(0, v); err != nil {
		return c.errf(op2, "%s：%v", name, err)
	}
	now, err := c.Sys.CR(0)
	if err != nil {
		return c.errf(op2, "%s：%v", name, err)
	}
	if old&cr0PE == 0 && now&cr0PE != 0 {
		return ErrModeSwitch
	}
	return nil
}

// sysMovCRDR 是 MOV r32,CRn（0F 20）、MOV r32,DRn（0F 21）、MOV CRn,r32（0F 22）、
// MOV DRn,r32（0F 23）。只讀一個 ModRM 位元組，mod 欄位一律視為暫存器形式，不解有效位址
// （prefix_0f.h:196-208）；運算元一律 32 位元，與 66 前綴無關（core_normal/helpers.h:201）。
func (c *CPU) sysMovCRDR(op2 uint8) error {
	b := c.fetch8()
	n := int(b>>3) & 7
	r := int(b) & 7
	if err := c.privileged(op2, "MOV CR／DR"); err != nil {
		return err
	}
	switch op2 {
	case 0x20: // MOV r32,CRn（cpu.cpp:2653-2693）
		if n != 0 && n != 2 && n != 3 {
			return c.errf(op2, "0F 20：MOV r32,CR%d 失敗即關閉（386 沒有 CR%d）", n, n)
		}
		v, err := c.Sys.CR(n)
		if err != nil {
			return c.errf(op2, "0F 20：%v", err)
		}
		switch n {
		case 0:
			v |= 0x7FFFFFF0 // 386 的讀回（cpu.cpp:2669）；smsw 讀原始值
		case 3:
			v &= 0xFFFFF000
		}
		c.setReg32(r, v)
	case 0x22: // MOV CRn,r32（cpu.cpp:2580-2636）
		v := c.reg32(r)
		switch n {
		case 0:
			old, err := c.Sys.CR(0)
			if err != nil {
				return c.errf(op2, "0F 22：%v", err)
			}
			return c.setCR0(op2, "0F 22 MOV CR0", old, v)
		case 2, 3:
			if err := c.Sys.SetCR(n, v); err != nil {
				return c.errf(op2, "0F 22：%v", err)
			}
		default:
			return c.errf(op2, "0F 22：MOV CR%d,r32 失敗即關閉（386 沒有 CR%d）", n, n)
		}
	case 0x21: // MOV r32,DRn（cpu.cpp:2695-2749）
		c.setReg32(r, c.Sys.DR(n))
	case 0x23: // MOV DRn,r32
		c.Sys.SetDR(n, c.reg32(r))
	}
	return nil
}
