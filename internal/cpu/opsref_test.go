package cpu

// executeCoreRef 是值 switch 改寫前（a3cfff5 ops.go 第 134–605 行）的 executeCore，逐字保留，
// 只供 TestExecuteCoreDifferential 對照（perf 草稿 3a）。

//gocyclo:ignore
func (c *CPU) executeCoreRef(op uint8) error {
	switch {
	// ---- 00–3F：八個 ALU 運算，每個六種定址 ----------------------------
	case op < 0x40 && op&7 < 6 && op != 0x0F && op != 0x27 && op != 0x2F &&
		op != 0x37 && op != 0x3F:
		return c.aluGroup(op)

	// ---- 段暫存器的 PUSH／POP -------------------------------------------
	case op == 0x06:
		c.push(c.Seg[ES])
	case op == 0x07:
		c.loadSeg(ES, c.pop())
	case op == 0x0E:
		c.push(c.Seg[CS])
	case op == 0x0F:
		// **8086 的 0F 是 POP CS**，不是雙位元組前綴。分段模式只在 Model80386（0F 不到這裡），
		// 仍照 434 §5.1 失敗即關閉當防線。
		if c.segmented {
			return c.errf(op, "POP CS：分段模式失敗即關閉（docs/spec/434 §5.1）")
		}
		c.Seg[CS] = c.pop()
	case op == 0x16:
		c.push(c.Seg[SS])
	case op == 0x17:
		c.loadSeg(SS, c.pop())
	case op == 0x1E:
		c.push(c.Seg[DS])
	case op == 0x1F:
		c.loadSeg(DS, c.pop())

	// ---- BCD 調整 --------------------------------------------------------
	case op == 0x27:
		c.daa()
	case op == 0x2F:
		c.das()
	case op == 0x37:
		c.aaa()
	case op == 0x3F:
		c.aas()

	// ---- 40–5F：INC／DEC／PUSH／POP 通用暫存器 --------------------------
	case op >= 0x40 && op <= 0x47:
		r := int(op - 0x40)
		c.R[r] = c.inc(c.R[r], 16)
	case op >= 0x48 && op <= 0x4F:
		r := int(op - 0x48)
		c.R[r] = c.dec(c.R[r], 16)
	case op >= 0x50 && op <= 0x57:
		c.pushOperand(regOperand(int(op - 0x50)))
	case op >= 0x58 && op <= 0x5F:
		c.R[op-0x58] = c.pop()

	// ---- 60–6F：機型相關（`docs/spec/002` §1.1）--------------------------
	//
	// 8086 把這一段當成 `70`–`7F` 的別名（語料驗證過）；80186 把它給了
	// `PUSHA` 那一族。**兩者的指令長度不同**，選錯不會報錯，只會錯位。
	case op >= 0x60 && op <= 0x6F && c.Model >= Model80186:
		return c.op186_6x(op)

	// ---- 60–7F：條件跳躍（8086 的 60–6F 是 70–7F 的別名）----------------
	case op >= 0x60 && op <= 0x7F:
		off := int16(int8(c.fetch8()))
		if c.cond(op & 0x0F) {
			c.charge(cycJump)
			c.IP = uint16(int16(c.IP) + off)
		}

	// ---- 80–83：ALU r/m, imm --------------------------------------------
	case op >= 0x80 && op <= 0x83:
		return c.aluImm(op)

	// ---- 84–8F ----------------------------------------------------------
	case op == 0x84: // TEST r/m8, r8
		m := c.decodeModRM()
		c.logic(uint16(c.get8(m.rm)&c.reg8(m.reg)), 8)
	case op == 0x85: // TEST r/m16, r16
		m := c.decodeModRM()
		c.logic(c.get16(m.rm)&c.R[m.reg], 16)
	case op == 0x86: // XCHG r/m8, r8
		m := c.decodeModRM()
		a, b := c.get8(m.rm), c.reg8(m.reg)
		c.set8(m.rm, b)
		c.setReg8(m.reg, a)
	case op == 0x87: // XCHG r/m16, r16
		m := c.decodeModRM()
		a, b := c.get16(m.rm), c.R[m.reg]
		c.set16(m.rm, b)
		c.R[m.reg] = a
	case op == 0x88: // MOV r/m8, r8
		m := c.decodeModRM()
		c.set8(m.rm, c.reg8(m.reg))
	case op == 0x89: // MOV r/m16, r16
		m := c.decodeModRM()
		c.set16(m.rm, c.R[m.reg])
	case op == 0x8A: // MOV r8, r/m8
		m := c.decodeModRM()
		c.setReg8(m.reg, c.get8(m.rm))
	case op == 0x8B: // MOV r16, r/m16
		m := c.decodeModRM()
		c.R[m.reg] = c.get16(m.rm)
	case op == 0x8C: // MOV r/m16, Sreg
		m := c.decodeModRM()
		if c.Model >= Model80386 && m.reg >= 4 {
			// 386：/4 ＝ FS、/5 ＝ GS；/6、/7 是 #UD，不是 8086 的別名。
			if m.reg > 5 {
				return c.errf(op, "8C /%d：不存在的段暫存器", m.reg)
			}
			c.set16(m.rm, c.segVal(FS+m.reg-4))
			break
		}
		c.set16(m.rm, c.Seg[m.reg&3])
	case op == 0x8D: // LEA r16, m
		// LEA 不碰記憶體，所以 0x67 下的有效位址不受 64 KB 段限檢查。
		c.eaNoLimit = true
		m := c.decodeModRM()
		c.eaNoLimit = false
		// LEA 取的是位移，不碰記憶體。r/m 是暫存器時 8086 的行為未定義；
		// 這裡回位移欄位 0，與實機一致（測資會遮）。
		if c.addr32 && !m.rm.isReg {
			c.R[m.reg] = uint16(c.ea32)
		} else {
			c.R[m.reg] = uint16(m.rm.off)
		}
	case op == 0x8E: // MOV Sreg, r/m16
		c.charge(cycFar)
		m := c.decodeModRM()
		if c.Model >= Model80386 && m.reg >= 4 {
			switch m.reg {
			case 4:
				c.loadSeg(FS, c.get16(m.rm))
			case 5:
				c.loadSeg(GS, c.get16(m.rm))
			default:
				return c.errf(op, "8E /%d：不存在的段暫存器", m.reg)
			}
			break
		}
		if m.reg&3 == CS {
			// MOV CS：8086 照寫（語料行為）；分段模式失敗即關閉（434 §5.1）。
			if c.segmented {
				return c.errf(op, "MOV CS：分段模式失敗即關閉（docs/spec/434 §5.1）")
			}
			c.Seg[CS] = c.get16(m.rm)
			break
		}
		c.loadSeg(m.reg&3, c.get16(m.rm))
	case op == 0x8F: // POP r/m16
		m := c.decodeModRM()
		if c.addr32 && c.eaSPBase && !m.rm.isReg && c.Code32() {
			// 443 §2.4：D=1 下 66 8F 的 ModRM32 以 ESP 為基底時，Intel 以彈出後的 ESP 計算位址，
			// 這裡先解位址後彈出，順序不同，失敗即關閉。
			return c.errf(op, "66 8F：D=1 碼段下以 ESP 為基底的 ModRM32 不支援，失敗即關閉（docs/spec/443 §2.4）")
		}
		c.set16(m.rm, c.pop())

	// ---- 90–9F ----------------------------------------------------------
	case op >= 0x90 && op <= 0x97: // XCHG AX, r16（90 ＝ NOP）
		r := int(op - 0x90)
		c.R[AX], c.R[r] = c.R[r], c.R[AX]
	case op == 0x98: // CBW
		c.R[AX] = uint16(int16(int8(uint8(c.R[AX]))))
	case op == 0x99: // CWD
		if c.R[AX]&0x8000 != 0 {
			c.R[DX] = 0xFFFF
		} else {
			c.R[DX] = 0
		}
	case op == 0x9A: // CALL far ptr16:16
		off := c.fetch16()
		seg := c.fetch16()
		c.farCall(seg, off)
	case op == 0x9B: // WAIT
	case op == 0x9C: // PUSHF
		if c.v86() && !c.iopl3() { // 436 §4（DOSBox-X CPU_PUSHF）
			c.v86Fail("PUSHF")
			break
		}
		c.push(c.Flags)
	case op == 0x9D: // POPF
		if c.v86() {
			c.v86Popf(c.pop)
			break
		}
		c.SetFlags(c.pop())
	case op == 0x9E: // SAHF
		c.SetFlags(c.Flags&0xFF00 | uint16(c.reg8(4)))
	case op == 0x9F: // LAHF
		c.setReg8(4, uint8(c.Flags))

	// ---- A0–A3：MOV 與絕對位址 ------------------------------------------
	case op == 0xA0:
		c.setReg8(0, c.read8(c.dataSeg(DS), uint32(c.fetch16())))
	case op == 0xA1:
		c.R[AX] = c.read16(c.dataSeg(DS), uint32(c.fetch16()))
	case op == 0xA2:
		c.write8(c.dataSeg(DS), uint32(c.fetch16()), uint8(c.R[AX]))
	case op == 0xA3:
		c.write16(c.dataSeg(DS), uint32(c.fetch16()), c.R[AX])

	// ---- A4–AF：字串指令 -------------------------------------------------
	case op >= 0xA4 && op <= 0xAF && op != 0xA8 && op != 0xA9:
		c.stringOp(op)

	case op == 0xA8: // TEST AL, imm8
		c.logic(uint16(uint8(c.R[AX])&c.fetch8()), 8)
	case op == 0xA9: // TEST AX, imm16
		c.logic(c.R[AX]&c.fetch16(), 16)

	// ---- B0–BF：MOV r, imm ----------------------------------------------
	case op >= 0xB0 && op <= 0xB7:
		c.setReg8(int(op-0xB0), c.fetch8())
	case op >= 0xB8 && op <= 0xBF:
		c.R[op-0xB8] = c.fetch16()

	// ---- C0–CF ----------------------------------------------------------
	//
	// ⚠ **C0／C1／C8／C9 也是機型相關的**（`docs/spec/002` §1.1）：
	// 8086 拿它們當 `RET`／`RETF` 的別名，80186 給了移位 imm8 與
	// `ENTER`／`LEAVE`。長度同樣不同，同樣不會報錯。
	case (op == 0xC0 || op == 0xC1) && c.Model >= Model80186:
		return c.shiftImm(op)
	case op == 0xC8 && c.Model >= Model80186:
		c.enter()
	case op == 0xC9 && c.Model >= Model80186:
		c.leave()

	case op == 0xC0 || op == 0xC2: // RET imm16（C0 是 8086 的別名）
		c.charge(cycRet)
		n := c.fetch16()
		c.IP = c.pop()
		c.R[SP] += n
	case op == 0xC1 || op == 0xC3: // RET
		c.charge(cycRet)
		c.IP = c.pop()
	case op == 0xC4, op == 0xC5: // LES／LDS r16, m
		// 遠指標兩個 word 都先讀（段在存取當下才取值，載入 DS 之後再讀會用到新的 DS），
		// 再載入段暫存器，最後寫通用暫存器（434 §5.1：段載入失敗時段值與快取不寫）。
		c.charge(cycFar)
		m := c.decodeModRM()
		off := c.get16(m.rm)
		seg := c.get16(m.rm.at(2))
		dst := ES
		if op == 0xC5 {
			dst = DS
		}
		if c.loadSeg(dst, seg) {
			c.R[m.reg] = off
		}
	case op == 0xC6: // MOV r/m8, imm8
		m := c.decodeModRM()
		c.set8(m.rm, c.fetch8())
	case op == 0xC7: // MOV r/m16, imm16
		m := c.decodeModRM()
		c.set16(m.rm, c.fetch16())
	case op == 0xC8 || op == 0xCA: // RETF imm16（C8 是別名）
		c.charge(cycFar)
		n := c.fetch16()
		c.farRet()
		c.R[SP] += n
	case op == 0xC9 || op == 0xCB: // RETF
		c.charge(cycFar)
		c.farRet()
	case op == 0xCC: // INT 3
		c.doInt(3)
	case op == 0xCD: // INT imm8
		c.doInt(c.fetch8())
	case op == 0xCE: // INTO
		if c.Flag(OF) {
			c.doInt(4)
		}
	case op == 0xCF: // IRET
		if c.segmented {
			// 442 §2.5：殘留快取實模式走實模式 iret，不看 NT（DOSBox-X CPU_IRET 先判 !cpu.pmode，cpu.cpp:1528-1540）。
			if c.realCache() {
				c.iretRealCache()
				break
			}
			// 436 §3.1：NT=1 是任務返回；NT=0 的保護模式 iret 仍失敗即關閉（16 位元 CF 回 V86 不支援，435 §2.5）。
			if c.Flags&NT != 0 {
				return c.taskReturn()
			}
			return c.errf(op, iretSegFail)
		}
		if c.v86() { // 436 §4（DOSBox-X cpu.cpp:1542-1584）；IOPL<3 記 #GP(0)（435 §2.2）
			if !c.iopl3() {
				c.v86Fail("IRET")
				break
			}
			c.IP = c.pop()
			c.Seg[CS] = c.pop()
			c.setFlagsKeepIOPL(c.pop())
			break
		}
		c.IP = c.pop()
		c.Seg[CS] = c.pop()
		c.SetFlags(c.pop())

	// ---- D0–D3：移位與旋轉 ----------------------------------------------
	case op >= 0xD0 && op <= 0xD3:
		return c.shiftGroup(op)

	case op == 0xD4: // AAM imm8
		imm := c.fetch8()
		al := uint8(c.R[AX])
		if imm == 0 {
			// **除數 0 時 AX 完全不動**，但旗標照樣照「結果 0」設一次
			// 才進 INT 0——那個值會被推上堆疊，所以不是無關緊要的細節。
			// 實機資料：SingleStepTests D4 檔裡 47 筆 `aam 0h`，
			// 全部是 ZF=1 PF=1 SF=0 CF=0 AF=0 OF=0、DF 保留。
			// 435 §2.2：V86 的 #DE 不改旗標（386），記事件時還原進入時的旗標。
			fl := c.Flags
			c.logic(0, 8)
			c.divideError()
			if c.ev != nil {
				c.Flags = fl
			}
			return nil
		}
		c.setReg8(4, al/imm)
		c.setReg8(0, al%imm)
		// CF／AF／OF 在語料裡被遮掉（metadata 的 flags-mask ＝ F7EE），
		// 所以清掉是**我們挑的確定值**，不是實機保證。
		c.logic(uint16(al%imm), 8)
	case op == 0xD5: // AAD imm8
		imm := c.fetch8()
		al := uint8(uint16(c.reg8(4))*uint16(imm) + uint16(uint8(c.R[AX])))
		c.setReg8(0, al)
		c.setReg8(4, 0)
		c.logic(uint16(al), 8) // CF／AF／OF 同上，被遮掉
	case op == 0xD6: // SALC（未公開）
		if c.Flag(CF) {
			c.setReg8(0, 0xFF)
		} else {
			c.setReg8(0, 0x00)
		}
	case op == 0xD7: // XLAT
		c.setReg8(0, c.read8(c.dataSeg(DS), uint32(c.R[BX]+uint16(uint8(c.R[AX])))))
	case op >= 0xD8 && op <= 0xDF: // ESC：探測語意見 spec 202，其餘只做記憶體讀取
		m := c.decodeModRM()
		if c.segmented && !x87Allowed(op, m) {
			// 431 §4、434 §6：保護模式下 FNINIT、FNSTCW、FLDCW（與非 ESC 的 FWAIT）以外失敗即關閉。
			return c.errf(op, "x87 ESC %02X /%d：分段模式只支援 FNINIT、FNSTCW、FLDCW、FWAIT（docs/spec/434 §6）", op, m.reg)
		}
		switch {
		case op == 0xDB && m.mod == 3 && m.reg == 4 && m.rm.isReg && m.rm.reg == 3:
			c.FPUCW = 0x037F // FNINIT（`DB E3`）
		case op == 0xD9 && m.reg == 7 && !m.rm.isReg:
			c.set16(m.rm, c.FPUCW) // FNSTCW m16（`D9 /7`）
		default:
			if !m.rm.isReg {
				c.get16(m.rm)
			}
		}

	// ---- E0–EF ----------------------------------------------------------
	case op >= 0xE0 && op <= 0xE3:
		off := int16(int8(c.fetch8()))
		take := false
		switch op {
		case 0xE0: // LOOPNZ
			c.R[CX]--
			take = c.R[CX] != 0 && !c.Flag(ZF)
		case 0xE1: // LOOPZ
			c.R[CX]--
			take = c.R[CX] != 0 && c.Flag(ZF)
		case 0xE2: // LOOP
			c.R[CX]--
			take = c.R[CX] != 0
		case 0xE3: // JCXZ
			take = c.R[CX] == 0
		}
		if take {
			c.IP = uint16(int16(c.IP) + off)
		}
	case op == 0xE4:
		if p := uint16(c.fetch8()); c.ioOK(p, 1) {
			c.setReg8(0, c.in8(p))
		}
	case op == 0xE5:
		if p := uint16(c.fetch8()); c.ioOK(p, 2) {
			c.R[AX] = uint16(c.in8(p)) | uint16(c.in8(p+1))<<8
		}
	case op == 0xE6:
		if p := uint16(c.fetch8()); c.ioOK(p, 1) {
			c.out8(p, uint8(c.R[AX]))
		}
	case op == 0xE7:
		if p := uint16(c.fetch8()); c.ioOK(p, 2) {
			c.out8(p, uint8(c.R[AX]))
			c.out8(p+1, uint8(c.R[AX]>>8))
		}
	case op == 0xE8: // CALL rel16
		c.charge(cycCall)
		off := int16(c.fetch16())
		c.push(c.IP)
		c.IP = uint16(int16(c.IP) + off)
	case op == 0xE9: // JMP rel16
		c.charge(cycJump)
		off := int16(c.fetch16())
		c.IP = uint16(int16(c.IP) + off)
	case op == 0xEA: // JMP far
		c.charge(cycFar)
		off := c.fetch16()
		seg := c.fetch16()
		if c.loadSeg(CS, seg) {
			c.IP = off
		}
	case op == 0xEB: // JMP rel8
		c.charge(cycJump)
		off := int16(int8(c.fetch8()))
		c.IP = uint16(int16(c.IP) + off)
	case op == 0xEC:
		if c.ioOK(c.R[DX], 1) {
			c.setReg8(0, c.in8(c.R[DX]))
		}
	case op == 0xED:
		if c.ioOK(c.R[DX], 2) {
			c.R[AX] = uint16(c.in8(c.R[DX])) | uint16(c.in8(c.R[DX]+1))<<8
		}
	case op == 0xEE:
		if c.ioOK(c.R[DX], 1) {
			c.out8(c.R[DX], uint8(c.R[AX]))
		}
	case op == 0xEF:
		if c.ioOK(c.R[DX], 2) {
			c.out8(c.R[DX], uint8(c.R[AX]))
			c.out8(c.R[DX]+1, uint8(c.R[AX]>>8))
		}

	// ---- F4–FF ----------------------------------------------------------
	case op == 0xF4: // HLT
		if c.segmented {
			// 434 §5.3、431 §3：分段模式期間不遞送 IRQ，hlt 會永遠等不到中斷。
			return c.errf(op, "HLT：分段模式下不遞送 IRQ（保護模式 IDT 遞送屬 S6），失敗即關閉（docs/spec/434 §5.3）")
		}
		if c.v86() { // 435 §2.1：V86 的 hlt 是 #GP(0)
			c.v86GP0("V86 HLT：#GP(0)（docs/spec/435 §2.1）")
			break
		}
		c.Halted = true
	case op == 0xF5: // CMC
		c.setFlag(CF, !c.Flag(CF))
	case op == 0xF6 || op == 0xF7:
		return c.group3(op)
	case op == 0xF8:
		c.setFlag(CF, false)
	case op == 0xF9:
		c.setFlag(CF, true)
	case op == 0xFA:
		if c.v86() && !c.iopl3() { // 436 §4（DOSBox-X CPU_CLI）
			c.v86Fail("CLI")
			break
		}
		c.setFlag(IF, false)
	case op == 0xFB:
		if c.v86() && !c.iopl3() { // 436 §4（DOSBox-X CPU_STI）
			c.v86Fail("STI")
			break
		}
		c.setFlag(IF, true)
	case op == 0xFC:
		c.setFlag(DF, false)
	case op == 0xFD:
		c.setFlag(DF, true)
	case op == 0xFE || op == 0xFF:
		return c.group45(op)

	default:
		return c.errf(op, "未實作的 opcode")
	}
	return nil
}
