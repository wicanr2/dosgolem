package cpu

// 80386 實模式子集（`docs/spec/012`、`docs/spec/197-cpu386-real-mode-subset`）。
//
// 三件事：
//
//   - 0x66 operand-size 前綴：把「運算元大小敏感」的指令改成 32 位元；
//   - 0x67 address-size 前綴：ModRM32／SIB（見 `decodeModRM32`）；
//   - 0F 雙位元組（`ops0f.go`）。
//
// **範圍規則：沒實作的形狀一律失敗即關閉。** 一道帶 0x66 的指令只有兩種
// 結局：在 `sizeSensitive` 之外（本來就不看運算元大小，例如位元組運算）
// 走原本的 16 位元路徑；在名單之內就必須由這裡實作，沒實作的回
// 「未實作」——**不能因為 0x66 被吃掉就用 16 位元語意悄悄跑完**，
// 那會讓 32 位元運算的高半默默消失，而且不會報錯。
//
// 32 位元暫存器：`R[i]` 是低半、`Hi[i]` 是高半。16 位元寫入不清高半。

func (c *CPU) getEAX() uint32 { return c.reg32(AX) }

func (c *CPU) setEAX(v uint32) { c.setReg32(AX, v) }

// sizeSensitive 是「帶 0x66 時語意與 16 位元不同」的主 opcode 表。
// 表之外的 opcode 不看運算元大小（位元組運算、跳躍的短位移、I/O 位元組、
// 旗標運算、BCD、浮點跳脫…），帶 0x66 也照 16 位元路徑跑。
var sizeSensitive = func() (t [256]bool) {
	for op := 0; op < 0x40; op++ {
		// ALU：form 1、3、5 是 16／32 位元；form 0、2、4 是位元組。
		// 0x0F、0x26、0x27、0x2E、0x2F、0x36、0x37、0x3E、0x3F 的低 3 位元
		// 是 6 或 7，不在 form 1／3／5 裡，自然排除。
		switch op & 7 {
		case 1, 3, 5:
			t[op] = true
		}
	}
	for _, op := range []int{0x06, 0x07, 0x0E, 0x16, 0x17, 0x1E, 0x1F} {
		t[op] = true // 段暫存器的 push／pop
	}
	for op := 0x40; op <= 0x5F; op++ {
		t[op] = true // inc／dec／push／pop 通用暫存器
	}
	for _, op := range []int{0x60, 0x61, 0x62, 0x68, 0x69, 0x6A, 0x6B, 0x6D, 0x6F} {
		t[op] = true
	}
	for op := 0x70; op <= 0x7F; op++ {
		t[op] = true // 66 ＋ 短條件跳躍：位移與 EIP 寬度的語意，不實作
	}
	for _, op := range []int{0x81, 0x83, 0x85, 0x87, 0x89, 0x8B, 0x8C, 0x8D, 0x8F} {
		t[op] = true
	}
	for op := 0x90; op <= 0x9D; op++ {
		t[op] = true // xchg／cwde／cdq／call far／pushfd／popfd（9B、9E、9F 在下面消掉）
	}
	t[0x9B], t[0x9E], t[0x9F] = false, false, false
	for _, op := range []int{0xA1, 0xA3, 0xA5, 0xA7, 0xA9, 0xAB, 0xAD, 0xAF} {
		t[op] = true
	}
	for op := 0xB8; op <= 0xBF; op++ {
		t[op] = true
	}
	for _, op := range []int{0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCF,
		0xD1, 0xD3, 0xE5, 0xE7, 0xE8, 0xE9, 0xEA, 0xED, 0xEF, 0xF7, 0xFF} {
		t[op] = true
	}
	return t
}()

// implicitAddr 是靠隱含位址暫存器（SI／DI／BX／CX）運作的 opcode。
// 0x67 會把它們改成 ESI／EDI／EBX／ECX，**這裡全部沒實作**，帶 0x67 就報錯
// （moffs 形式 A0–A3 另外實作）。
func implicitAddr(op uint8) bool {
	switch {
	case op >= 0x6C && op <= 0x6F: // INS／OUTS
		return true
	case op >= 0xA4 && op <= 0xA7, op >= 0xAA && op <= 0xAF: // 字串指令
		return true
	case op == 0xD7: // XLAT
		return true
	case op >= 0xE0 && op <= 0xE3: // LOOPcc／JCXZ
		return true
	}
	return false
}

// execute386 處理 80386 擴充。handled 為 false 表示「不歸我管」，
// 呼叫端接著走 executeCore。
func (c *CPU) execute386(op uint8) (handled bool, err error) {
	if op == 0x0F {
		return true, c.execute0F()
	}
	if c.segmented && c.cache[CS].Attr&attrDB != 0 {
		// 443 §2.4：D=1 碼段的相對轉移與計數迴圈，不論 operand32、addr32，先於隱含位址分派。
		switch {
		case op >= 0x70 && op <= 0x7F, op == 0xEB, op >= 0xE0 && op <= 0xE3:
			c.relJump32(op)
			return true, nil
		}
	}
	if c.addr32 {
		switch {
		case op >= 0xA0 && op <= 0xA3: // MOV AL/AX/EAX ↔ moffs（32 位元位移）
			return true, c.moffs32(op)
		case implicitAddr(op):
			if c.Code32() && (op >= 0xA4 && op <= 0xA7 || op >= 0xAA && op <= 0xAF) {
				c.stringOpA32(op) // 443 §2.5
				return true, nil
			}
			return true, c.errf(op, "0x67 前綴下的隱含位址指令未實作（docs/spec/197 §3）")
		}
	}
	if c.operand32 && sizeSensitive[op] {
		return true, c.execute32(op)
	}
	return false, nil
}

// relJump32 是 D=1 碼段的 70–7F、EB、E0–E3（443 §2.3、§2.4）：
//
//   - 目標寬度看 operand32：32 時以 uint32(IP)＋符號延伸的位移計算，大於 FFFFh 失敗即關閉、IP 不設為越界目標
//     （DOSBox-X JumpCond32_b，core_normal/support.h:76-88；EB 見 prefix_66.h:652-657）；16 時 16 位元環繞。
//   - E0–E3 的計數暫存器看 addr32：ECX（32 位元遞減、判斷含高半）或 CX（只動低 16 位元）。
//   - 週期照 executeCore 的同一道指令：70–7F 成立時、EB 一律 charge(cycJump)；E0–E3 不另計。
func (c *CPU) relJump32(op uint8) {
	disp := uint32(int32(int8(c.fetch8())))
	take := false
	switch {
	case op <= 0x7F:
		take = c.cond(op & 0x0F)
		if take {
			c.charge(cycJump)
		}
	case op == 0xEB:
		take = true
		c.charge(cycJump)
	default: // E0–E3：計數暫存器 ECX（addr32）或 CX
		var cnt uint32
		if c.addr32 {
			cnt = c.reg32(CX)
		} else {
			cnt = uint32(c.R[CX])
		}
		if op != 0xE3 {
			if c.addr32 {
				cnt--
				c.setReg32(CX, cnt)
			} else {
				cnt = uint32(uint16(cnt - 1))
				c.R[CX] = uint16(cnt)
			}
		}
		switch op {
		case 0xE0: // LOOPNZ
			take = cnt != 0 && !c.Flag(ZF)
		case 0xE1: // LOOPZ
			take = cnt != 0 && c.Flag(ZF)
		case 0xE2: // LOOP
			take = cnt != 0
		default: // JECXZ／JCXZ
			take = cnt == 0
		}
	}
	if !take {
		return
	}
	if !c.operand32 {
		c.IP += uint16(disp) // 運算元大小 16：16 位元環繞（443 §2.3）
		return
	}
	target := uint32(c.IP) + disp
	if target > 0xFFFF {
		c.failf("相對轉移目標 %08X：EIP 超過 FFFFh，docs/spec/443 §2.3", target)
		return
	}
	c.IP = uint16(target)
}

// moffs32 是 0x67 下的 A0–A3：位移是 32 位元，實模式超過 64 KB 失敗；
// 分段模式不截斷，界限由段快取檢查（434 §4.2）。
func (c *CPU) moffs32(op uint8) error {
	off := c.fetch32()
	o := operand{seg: uint8(c.dataSeg(DS)), off: off, w32: true}
	if !c.segmented {
		if off > 0xFFFF {
			return c.errf(op, "moffs 位移 %08X 超出實模式 64 KB 段限（docs/spec/197 §3）", off)
		}
		o.w32 = false
	}
	switch op {
	case 0xA0:
		c.setReg8(0, c.get8(o))
	case 0xA1:
		if c.operand32 {
			c.setReg32(AX, c.get32(o))
		} else {
			c.R[AX] = c.get16(o)
		}
	case 0xA2:
		c.set8(o, uint8(c.R[AX]))
	case 0xA3:
		if c.operand32 {
			c.set32(o, c.reg32(AX))
		} else {
			c.set16(o, c.R[AX])
		}
	}
	return nil
}

// execute32 是 0x66 之後、運算元大小敏感的 opcode（`sizeSensitive` 內）。
// 語意照 DOSBox-X `src/cpu/core_normal/prefix_66.h`，旗標見 `wide.go`。
//
//gocyclo:ignore
func (c *CPU) execute32(op uint8) error {
	switch {
	// ---- 00–3D：ALU，form 1／3／5 --------------------------------------
	case op < 0x40:
		switch kind, form := int(op>>3), op&7; form {
		case 1, 3: // r/m32, r32 ／ r32, r/m32
			m := c.decodeModRM()
			dst, src := m.rm, regOperand(m.reg)
			if form == 3 {
				dst, src = src, dst
			}
			r, store := c.alu32(kind, c.get32(dst), c.get32(src))
			if store {
				c.set32(dst, r)
			}
		case 5: // EAX, imm32
			r, store := c.alu32(kind, c.reg32(AX), c.fetch32())
			if store {
				c.setReg32(AX, r)
			}
		default: // 06／07／0E／16／17／1E／1F：段暫存器 push／pop
			switch op {
			case 0x06:
				c.push32(uint32(c.Seg[ES]))
			case 0x0E:
				c.push32(uint32(c.Seg[CS]))
			case 0x16:
				c.push32(uint32(c.Seg[SS]))
			case 0x1E:
				c.push32(uint32(c.Seg[DS]))
			case 0x07:
				c.loadSeg(ES, uint16(c.pop32()))
			case 0x17:
				c.loadSeg(SS, uint16(c.pop32()))
			case 0x1F:
				c.loadSeg(DS, uint16(c.pop32()))
			}
		}

	// ---- 40–5F：inc／dec／push／pop r32 ---------------------------------
	case op >= 0x40 && op <= 0x47:
		r := int(op - 0x40)
		c.setReg32(r, c.inc32(c.reg32(r)))
	case op >= 0x48 && op <= 0x4F:
		r := int(op - 0x48)
		c.setReg32(r, c.dec32(c.reg32(r)))
	case op >= 0x50 && op <= 0x57:
		// PUSH ESP 推的是**減之前**的值（286 以上；DOSBox-X 同）。
		v := c.reg32(int(op - 0x50))
		c.push32(v)
	case op >= 0x58 && op <= 0x5F:
		v := c.pop32()
		c.setReg32(int(op-0x58), v)

	case op == 0x60: // PUSHAD：推的是進來時的 ESP
		sp := c.reg32(SP)
		for _, r := range [...]int{AX, CX, DX, BX} {
			c.push32(c.reg32(r))
		}
		c.push32(sp)
		for _, r := range [...]int{BP, SI, DI} {
			c.push32(c.reg32(r))
		}
	case op == 0x61: // POPAD：ESP 那一格丟掉
		for _, r := range [...]int{DI, SI, BP} {
			c.setReg32(r, c.pop32())
		}
		c.pop32()
		for _, r := range [...]int{BX, DX, CX, AX} {
			c.setReg32(r, c.pop32())
		}

	case op == 0x68: // PUSH imm32
		c.push32(c.fetch32())
	case op == 0x6A: // PUSH imm8（符號延伸成 32 位元）
		c.push32(uint32(int32(int8(c.fetch8()))))
	case op == 0x69, op == 0x6B: // IMUL r32, r/m32, imm
		m := c.decodeModRM()
		src := c.get32(m.rm)
		var imm uint32
		if op == 0x69 {
			imm = c.fetch32()
		} else {
			imm = uint32(int32(int8(c.fetch8())))
		}
		c.setReg32(m.reg, c.imul32Two(src, imm))

	// ---- 81／83：ALU r/m32, imm ----------------------------------------
	case op == 0x81, op == 0x83:
		m := c.decodeModRM()
		var imm uint32
		if op == 0x81 {
			imm = c.fetch32()
		} else {
			imm = uint32(int32(int8(c.fetch8())))
		}
		r, store := c.alu32(m.reg, c.get32(m.rm), imm)
		if store {
			c.set32(m.rm, r)
		}

	// ---- 85–8F ----------------------------------------------------------
	case op == 0x85: // TEST r/m32, r32
		m := c.decodeModRM()
		c.logic32(c.get32(m.rm) & c.reg32(m.reg))
	case op == 0x87: // XCHG r/m32, r32
		m := c.decodeModRM()
		a, b := c.get32(m.rm), c.reg32(m.reg)
		c.set32(m.rm, b)
		c.setReg32(m.reg, a)
	case op == 0x89: // MOV r/m32, r32
		m := c.decodeModRM()
		c.set32(m.rm, c.reg32(m.reg))
	case op == 0x8B: // MOV r32, r/m32
		m := c.decodeModRM()
		c.setReg32(m.reg, c.get32(m.rm))
	case op == 0x8C: // MOV r/m32, Sreg：暫存器目的地零延伸，記憶體只寫 16 位元
		m := c.decodeModRM()
		if m.reg > 5 {
			return c.errf(op, "66 8C /%d：不存在的段暫存器", m.reg)
		}
		v := c.segVal(m.reg)
		if c.Model >= Model80386 && m.rm.isReg {
			c.setReg32(m.rm.reg, uint32(v))
		} else {
			c.set16(m.rm, v)
		}
	case op == 0x8D: // LEA r32, m
		c.eaNoLimit = true
		m := c.decodeModRM()
		c.eaNoLimit = false
		if m.rm.isReg {
			return c.errf(op, "66 8D：LEA 的來源不能是暫存器")
		}
		if c.addr32 {
			c.setReg32(m.reg, c.ea32)
		} else {
			c.setReg32(m.reg, uint32(uint16(m.rm.off)))
		}
	case op == 0x8F: // POP r/m32：先彈再解位址（ESP 相對的位址看彈出後的 ESP）
		v := c.pop32()
		m := c.decodeModRM()
		c.set32(m.rm, v)

	// ---- 90–9D ----------------------------------------------------------
	case op == 0x90: // XCHG EAX,EAX ＝ NOP
	case op >= 0x91 && op <= 0x97:
		r := int(op - 0x90)
		a, b := c.reg32(AX), c.reg32(r)
		c.setReg32(AX, b)
		c.setReg32(r, a)
	case op == 0x98: // CWDE
		c.setReg32(AX, uint32(int32(int16(c.R[AX]))))
	case op == 0x99: // CDQ
		if c.reg32(AX)&0x80000000 != 0 {
			c.setReg32(DX, 0xFFFFFFFF)
		} else {
			c.setReg32(DX, 0)
		}
	case op == 0x9C: // PUSHFD：高 16 位元恆 0（VM、RF 在映像裡本來就清掉；AC 寫不進去）
		if c.v86() && !c.iopl3() { // 436 §4
			c.v86Fail("PUSHFD")
			break
		}
		c.push32(uint32(c.Flags))
	case op == 0x9D: // POPFD：只收低 16 位元，EFLAGS 高半（含 AC）丟掉
		if c.v86() { // 436 §4：高半丟棄（不改 VM、RF）
			c.v86Popf(func() uint16 { return uint16(c.pop32()) })
			break
		}
		c.SetFlags(uint16(c.pop32()))

	// ---- A1–AF -----------------------------------------------------------
	case op == 0xA1: // MOV EAX, moffs16
		c.setReg32(AX, c.read32(c.dataSeg(DS), uint32(c.fetch16())))
	case op == 0xA3: // MOV moffs16, EAX
		c.write32(c.dataSeg(DS), uint32(c.fetch16()), c.reg32(AX))
	case op == 0xA5, op == 0xA7, op == 0xAB, op == 0xAD, op == 0xAF:
		c.stringOp32(op)
	case op == 0xA9: // TEST EAX, imm32
		c.logic32(c.reg32(AX) & c.fetch32())

	// ---- B8–BF -----------------------------------------------------------
	case op >= 0xB8 && op <= 0xBF:
		c.setReg32(int(op-0xB8), c.fetch32())

	// ---- C1／D1／D3：移位與旋轉 -------------------------------------------
	case op == 0xC1, op == 0xD1, op == 0xD3:
		m := c.decodeModRM()
		var n uint8
		switch op {
		case 0xC1:
			n = c.fetch8()
		case 0xD1:
			n = 1
		default:
			n = uint8(c.R[CX])
		}
		// 386 把計數遮成 5 位元；遮完是 0 就完全不動旗標與運算元。
		if n &= 31; n != 0 {
			c.set32(m.rm, c.shift32(m.reg, c.get32(m.rm), n))
		}

	case op == 0xC7: // MOV r/m32, imm32
		m := c.decodeModRM()
		if m.reg != 0 {
			return c.errf(op, "66 C7 /%d：不存在", m.reg)
		}
		c.set32(m.rm, c.fetch32())

	// ---- CF：IRETD（436 §3.1、435 §2.5）---------------------------------------------
	case op == 0xCF:
		if c.segmented {
			if c.realCache() { // 442 §2.5：不得走 taskReturn 或 v86ReturnRead
				return c.errf(op, "66 CF IRETD：殘留快取實模式下失敗即關閉（docs/spec/442 §2.5）")
			}
			if c.Flags&NT != 0 {
				return c.taskReturn() // DOSBox-X 的任務返回不看運算元大小（cpu.cpp:1586-1597）
			}
			if c.Sys != nil && c.Sys.CPL() == 0 {
				return c.v86ReturnRead(op) // 435 §2.5：ring 0 iretd 回 V86
			}
			return c.errf(op, iretSegFail)
		}
		if c.v86() {
			c.v86Iretd()
			return nil
		}
		return c.errf(op, "66 %02X：80386 子集沒有這一道（docs/spec/197 §2）", op)

	// ---- F7：group 3 ------------------------------------------------------
	case op == 0xF7:
		return c.group3_32(op)

	// ---- FF：group 5（inc／dec／push；分段模式另有 /3、/5 遠轉移）-------------------
	case op == 0xFF:
		m := c.decodeModRM()
		switch m.reg {
		case 0:
			c.set32(m.rm, c.inc32(c.get32(m.rm)))
		case 1:
			c.set32(m.rm, c.dec32(c.get32(m.rm)))
		case 6:
			c.push32(c.get32(m.rm))
		case 3, 5: // CALL／JMP FAR m16:32（437 §2：只在分段模式）
			if !c.segmented {
				return c.errf(op, "66 FF /%d：32 位元遠轉移只在分段模式實作（docs/spec/197 §2、437 §2）", m.reg)
			}
			if m.rm.isReg { // DOSBox-X prefix_66.h:732、752 為非法指令
				return c.errf(op, "66 FF /%d：遠指標運算元不能是暫存器（mod=3），失敗即關閉（docs/spec/437 §2）", m.reg)
			}
			off := c.get32(m.rm)
			sel := c.get16(m.rm.at(4))
			if m.reg == 3 {
				c.farCall32(off, sel)
			} else {
				c.farJmp32(off, sel)
			}
		default: // 2、4
			return c.errf(op, "66 FF /%d：32 位元的近 call／jmp 未實作（docs/spec/197 §2、437 §2）", m.reg)
		}

	// ---- 9A／EA／CA／CB：分段模式的 16:32 遠轉移（437 §2）---------------------
	case op == 0xEA && c.segmented:
		c.charge(cycFar)
		off := c.fetch32()
		c.farJmp32(off, c.fetch16())
	case op == 0x9A && c.segmented:
		off := c.fetch32()
		c.farCall32(off, c.fetch16())
	case op == 0xCA && c.segmented:
		c.charge(cycFar)
		c.farRet32(c.fetch16())
	case op == 0xCB && c.segmented:
		c.charge(cycFar)
		c.farRet32(0)

	default:
		return c.errf(op, "66 %02X：80386 子集沒有這一道（docs/spec/197 §2）", op)
	}
	return nil
}

// farOffOK：目標偏移高 16 位元須為 0（D=0 碼段的 IP 只有 16 位元；刻意從嚴，437 §2）。
func (c *CPU) farOffOK(off uint32, sel uint16) bool {
	if off>>16 != 0 {
		c.failf("分段模式 32 位元遠轉移：目標 %04X:%08X 的偏移高 16 位元非 0（D=0 碼段，失敗即關閉，docs/spec/437 §2）", sel, off)
		return false
	}
	return true
}

// far32RealCache：殘留快取實模式下 437 的 16:32 遠轉移失敗即關閉（442 §2.5），狀態不改。
func (c *CPU) far32RealCache(what string) bool {
	if !c.realCache() {
		return false
	}
	c.failf("%s：殘留快取實模式下的 32 位元遠轉移失敗即關閉（docs/spec/442 §2.5）", what)
	return true
}

// farJmp32 是分段模式的 66 EA、66 FF /5（437 §2）。順序：指標已讀完、看 fault、驗偏移高半、
// loadSeg(CS)、設 IP。失敗時 CS、CS 快取不變。
func (c *CPU) farJmp32(off uint32, sel uint16) {
	if c.far32RealCache("66 EA／66 FF /5") {
		return
	}
	if c.fault != nil || !c.farOffOK(off, sel) {
		return
	}
	if c.loadSeg(CS, sel) {
		c.IP = uint16(off)
	}
}

// farCall32 是分段模式的 66 9A、66 FF /3（437 §2）：推 32 位元 CS（高 16 位元 0）、32 位元 EIP。
// 順序：指標已讀完、看 fault、驗偏移高半、堆疊預檢（stackCheck，SP-8–SP-1 以 16 位元環繞做寫入的
// segCheck）、loadSeg(CS)、推入、設 IP。loadSeg 成功後的推入不會再失敗（已預檢）。
func (c *CPU) farCall32(off uint32, sel uint16) {
	if c.far32RealCache("66 9A／66 FF /3") {
		return
	}
	if c.fault != nil || !c.farOffOK(off, sel) {
		return
	}
	c.stackCheck()
	if c.fault != nil {
		return
	}
	sp := c.R[SP]
	for k := uint16(1); k <= 8; k++ {
		if !c.segCheck(SS, uint32(sp-k), accWrite) {
			return
		}
	}
	oldCS, oldIP := c.Seg[CS], c.IP
	if !c.loadSeg(CS, sel) {
		return
	}
	c.push32(uint32(oldCS))
	c.push32(uint32(oldIP))
	c.IP = uint16(off)
}

// farRet32 是分段模式的 66 CB、66 CA iw（437 §2）：彈出 32 位元 EIP、32 位元 CS（取低 16 位元，
// 高半不檢查），SP += 8 + iw。順序：stackCheck、讀 SP 與 SP+4 的兩個 dword 而不動 SP、看 fault、
// 驗偏移高半、retRPLOK、loadSeg(CS)、改 SP、設 IP。失敗時 CS、CS 快取、SP 都不變。
func (c *CPU) farRet32(n uint16) {
	if c.far32RealCache("66 CA／66 CB") {
		return
	}
	c.charge(2 * cycStack)
	c.stackCheck()
	sp := c.R[SP]
	ip := c.read32(SS, uint32(sp))
	cs := uint16(c.read32(SS, uint32(sp+4)))
	if c.fault != nil || !c.farOffOK(ip, cs) || !c.retRPLOK(cs) || !c.loadSeg(CS, cs) {
		return
	}
	c.R[SP] = sp + 8 + n
	c.IP = uint16(ip)
}

// group3_32 是 66 F7：TEST／NOT／NEG／MUL／IMUL／DIV／IDIV r/m32。
func (c *CPU) group3_32(op uint8) error {
	m := c.decodeModRM()
	switch m.reg {
	case 0, 1: // TEST r/m32, imm32
		c.logic32(c.get32(m.rm) & c.fetch32())
	case 2: // NOT：不動旗標
		c.set32(m.rm, ^c.get32(m.rm))
	case 3:
		c.set32(m.rm, c.neg32(c.get32(m.rm)))
	case 4:
		c.charge(cycMul)
		c.mul32(c.get32(m.rm))
	case 5:
		c.charge(cycMul)
		c.imul32One(c.get32(m.rm))
	case 6:
		c.charge(cycDiv)
		if !c.div32(c.get32(m.rm)) {
			c.divideFault()
		}
	case 7:
		c.charge(cycDiv)
		if !c.idiv32(c.get32(m.rm)) {
			c.divideFault()
		}
	}
	return nil
}

// stringOp32 是 66 之後的 A5／A7／AB／AD／AF（MOVSD／CMPSD／STOSD／LODSD／SCASD）。
// 位址暫存器仍是 SI／DI、計數仍是 CX（0x67 的組合在 execute386 已拒絕）。
// 規則同 `string.go`：來源段可被前綴覆寫、目的永遠是 ES:DI、REP 先看 CX。
func (c *CPU) stringOp32(op uint8) {
	d := c.delta(32)
	if c.repPrefix == 0 {
		c.stringOnce32(op, d)
		return
	}
	checkZF := op == 0xA7 || op == 0xAF
	wantZF := c.repPrefix == 0xF3
	for c.R[CX] != 0 {
		c.stringOnce32(op, d)
		c.R[CX]--
		if checkZF && c.Flag(ZF) != wantZF {
			break
		}
	}
}

func (c *CPU) stringOnce32(op uint8, d uint16) {
	c.charge(cycString)
	src := c.dataSeg(DS)
	const dst = ES
	si, di := uint32(c.R[SI]), uint32(c.R[DI])
	switch op {
	case 0xA5: // MOVSD
		c.write32(dst, di, c.read32(src, si))
		c.R[SI] += d
		c.R[DI] += d
	case 0xA7: // CMPSD：[SI] − [DI]
		c.sub32(c.read32(src, si), c.read32(dst, di), 0)
		c.R[SI] += d
		c.R[DI] += d
	case 0xAB: // STOSD
		c.write32(dst, di, c.reg32(AX))
		c.R[DI] += d
	case 0xAD: // LODSD
		c.setReg32(AX, c.read32(src, si))
		c.R[SI] += d
	case 0xAF: // SCASD：EAX − [DI]
		c.sub32(c.reg32(AX), c.read32(dst, di), 0)
		c.R[DI] += d
	}
}

// read32／write32：段索引、偏移各 byte 16 位元環繞（實模式與 434 之前逐位元組相同）。
func (c *CPU) read32(i int, off uint32) uint32 {
	if !c.segmented {
		b := uint32(c.segVal(i)) << 4
		o := uint16(off)
		return uint32(c.Bus.Read8(b+uint32(o))) | uint32(c.Bus.Read8(b+uint32(o+1)))<<8 |
			uint32(c.Bus.Read8(b+uint32(o+2)))<<16 | uint32(c.Bus.Read8(b+uint32(o+3)))<<24
	}
	return c.segRead(i, off, 4, false)
}

func (c *CPU) write32(i int, off uint32, v uint32) {
	if !c.segmented {
		b := uint32(c.segVal(i)) << 4
		o := uint16(off)
		c.Bus.Write8(b+uint32(o), uint8(v))
		c.Bus.Write8(b+uint32(o+1), uint8(v>>8))
		c.Bus.Write8(b+uint32(o+2), uint8(v>>16))
		c.Bus.Write8(b+uint32(o+3), uint8(v>>24))
		return
	}
	c.segWrite(i, off, 4, false, v)
}

// fetch32 從 CS:IP 取 4 個位元組（小端）。
func (c *CPU) fetch32() uint32 {
	lo := uint32(c.fetch16())
	hi := uint32(c.fetch16())
	return lo | hi<<16
}

func (c *CPU) reg32(i int) uint32 { return uint32(c.R[i]) | uint32(c.Hi[i])<<16 }

func (c *CPU) setReg32(i int, v uint32) {
	c.R[i] = uint16(v)
	c.Hi[i] = uint16(v >> 16)
}

func (c *CPU) get32(o operand) uint32 {
	if o.isReg {
		return c.reg32(o.reg)
	}
	if o.w32 {
		return c.segRead(int(o.seg), o.off, 4, true)
	}
	return c.read32(int(o.seg), o.off)
}

func (c *CPU) set32(o operand, v uint32) {
	if o.isReg {
		c.setReg32(o.reg, v)
		return
	}
	if o.w32 {
		c.segWrite(int(o.seg), o.off, 4, true, v)
		return
	}
	c.write32(int(o.seg), o.off, v)
}

// push32／pop32：堆疊指標仍是 16 位元的 SP（實模式與分段模式的 SS 都是 B=0；B=1 失敗即關閉），
// ESP 高半不動。每個 byte 的位移各自 wrap，與 16 位元的 push／pop 一致。
func (c *CPU) push32(v uint32) {
	c.charge(cycStack)
	if c.segmented {
		c.stackCheck()
	}
	c.R[SP] -= 4
	c.write32(SS, uint32(c.R[SP]), v)
}

func (c *CPU) pop32() uint32 {
	c.charge(cycStack)
	if c.segmented {
		c.stackCheck()
	}
	v := c.read32(SS, uint32(c.R[SP]))
	c.R[SP] += 4
	return v
}
