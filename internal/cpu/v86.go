package cpu

import "fmt"

// V86 規則（docs/spec/436-tss-task-return-v86-entry §4）。
//
// 進入 V86 後由既有核心以 432 的 V86 語意執行（段值×16、經線性層、CPL 3 的使用者分頁權限），segmented=false。
// 這裡只放 IOPL 敏感指令與 I/O 許可點陣圖；違反時記 #GP(0) 事件（435 §2.2），由 Machine 經 IDT 遞送。

const ioplMask = 0x3000

// setFlagsKeepIOPL 寫旗標，但 IOPL 保留舊值（V86 的 popf、iret 不改 IOPL；DOSBox-X CPU_POPF 在 CPL>0
// 拿掉 FLAG_IOPL，IRET 以 FMASK_NORMAL|FLAG_NT 設旗標）。
func (c *CPU) setFlagsKeepIOPL(v uint16) {
	c.SetFlags(v&^ioplMask | c.Flags&ioplMask)
}

// v86Popf 是 V86 的 popf／popfd（pop 由呼叫端給，popfd 只取低 16 位元，高半丟棄而不改 VM、RF）。
// IOPL ≠ 3 時記 #GP(0) 事件，不彈出（DOSBox-X CPU_POPF，cpu.cpp:774-788）。
func (c *CPU) v86Popf(pop func() uint16) {
	if !c.iopl3() {
		c.v86Fail("POPF")
		return
	}
	c.setFlagsKeepIOPL(pop())
}

// v86Iretd 是 V86 的 66 CF：IOPL=3 時從 V86 堆疊彈出 EIP、CS、EFLAGS 各 4 bytes（DOSBox-X cpu.cpp:1548-1561）。
// IOPL<3 記 #GP(0) 事件（435 §2.2）；EIP 高半非 0 仍是一般失敗（刻意從嚴）。三個值都先讀，檢查通過才改 SP、CS:IP 與旗標。
func (c *CPU) v86Iretd() {
	if !c.iopl3() {
		c.v86Fail("IRETD")
		return
	}
	sp := c.R[SP]
	eip := c.read32(SS, uint32(sp))
	cs := c.read32(SS, uint32(sp+4))
	fl := c.read32(SS, uint32(sp+8))
	if eip>>16 != 0 {
		c.failf("V86 IRETD：彈出的 EIP %08X 高 16 位元不為 0，失敗即關閉（刻意從嚴，docs/spec/436 §4）", eip)
		return
	}
	c.charge(3 * cycStack)
	c.R[SP] = sp + 12
	c.IP = uint16(eip)
	c.Seg[CS] = uint16(cs)
	c.setFlagsKeepIOPL(uint16(fl))
}

// ioOK 是指令層的 I/O 檢查（436 §4）：V86 以外恆為 true（分段模式的 CPL ≤ IOPL 仍在 in8／out8）。
// size 是存取的埠數（1 或 2），port 是起始埠。V86 一律查 TR 的 I/O 點陣圖，不論 IOPL。
func (c *CPU) ioOK(port uint16, size int) bool {
	if c.Sys == nil || !c.Sys.VM() {
		return true
	}
	return c.v86IOPermit(port, size)
}

// v86IOPermit 照 DOSBox-X CPU_IO_Exception（cpu.cpp:1093-1111）：TR 無效或快取不是 386 TSS 失敗；
// 讀 TR.base+66h 的位移 ofs，ofs > TR.limit 失敗（只比位移）；讀 TR.base+ofs+port/8 起的 16 位元，
// 與 ((1<<size)-1) << (port&7) 相交非 0 失敗。失敗時記 #GP(0) 事件（435 §2.2），回 false。
func (c *CPU) v86IOPermit(port uint16, size int) bool {
	sel, tr := c.Sys.TR()
	if sel&0xFFFC == 0 || tr.Attr&attrP == 0 || tr.Attr&(attrS|0x0D) != 0x09 {
		c.v86GP0(fmt.Sprintf("V86 I/O 埠 %04X：TR %04X 無效或不是 386 TSS（屬性 %04X），#GP(0)"+
			"（docs/spec/436 §4、435 §2.1）", port, sel, tr.Attr))
		return false
	}
	ofs := c.Sys.ReadSup16(tr.Base + 0x66)
	if uint32(ofs) > tr.Limit {
		c.v86GP0(fmt.Sprintf("V86 I/O 埠 %04X：I/O 點陣圖位移 %04X 超過 TSS 界限 %X，#GP(0)"+
			"（docs/spec/436 §4、435 §2.1）", port, ofs, tr.Limit))
		return false
	}
	bm := c.Sys.ReadSup16(tr.Base + uint32(ofs) + uint32(port/8))
	if mask := uint16((1<<size)-1) << (port & 7); bm&mask != 0 {
		c.v86GP0(fmt.Sprintf("V86 I/O 埠 %04X（%d 個埠）：I/O 點陣圖 %04X 擋下，#GP(0)"+
			"（docs/spec/436 §4、435 §2.1）", port, size, bm))
		return false
	}
	return true
}
