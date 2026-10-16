package cpu

import "math/bits"

// 80386 的 0F 雙位元組指令（`docs/spec/197-cpu386-real-mode-subset` §2.2）。
//
// 只在 Model80386 進來（`execute`）。**沒列在這裡的第二個位元組一律失敗即關閉**，
// 不會退化成 8086 的 POP CS。語意照 DOSBox-X `src/cpu/core_normal/prefix_0f.h`
// （16 位元運算元）與 `prefix_66_0f.h`（32 位元運算元）。
//
// 已實作：
//
//	0F 80–8F  Jcc near（rel16；帶 66 為 rel32）
//	0F 90–9F  SETcc r/m8
//	0F A0／A1／A8／A9  PUSH／POP FS／GS
//	0F A3／AB／B3／BB／BA  BT／BTS／BTR／BTC
//	0F A4／A5／AC／AD  SHLD／SHRD
//	0F AF     IMUL r, r/m
//	0F B2／B4／B5  LSS／LFS／LGS
//	0F B6／B7／BE／BF  MOVZX／MOVSX
//	0F BC／BD  BSF／BSR
//
// `Sys` 非 nil（保護模式 profile 開啟）時，`default` 分支另外交給 sys.go 處理
// `0F 00`–`03`、`06`、`20`–`24`、`26` 的實模式系統指令（`docs/spec/433-real-mode-system-instructions` §4）；
// `Sys` 為 nil 時這些照下列「不實作」處理，錯誤訊息不變。
//
// 刻意**不**實作（沒有證據用到；用到時再補，補之前照樣失敗即關閉）：
// `0F 00／01` 系統指令、MOV CR／DR／TR、486 以上的 BSWAP／XADD／CMPXCHG／
// CPUID。偵測常式回報 386，486 指令在這顆 CPU 上本來就不存在。

func (c *CPU) execute0F() error {
	op2 := c.fetch8()
	c.curOp = op2
	switch {
	case op2 >= 0x80 && op2 <= 0x8F: // Jcc near
		c.jccNear(op2)

	case op2 >= 0x90 && op2 <= 0x9F: // SETcc
		m := c.decodeModRM()
		v := uint8(0)
		if c.cond(op2 & 0x0F) {
			v = 1
		}
		c.set8(m.rm, v)

	case op2 == 0xA0 || op2 == 0xA8: // PUSH FS／GS
		v := c.FSeg
		if op2 == 0xA8 {
			v = c.GSeg
		}
		if c.operand32 {
			c.push32(uint32(v))
		} else {
			c.push(v)
		}
	case op2 == 0xA1 || op2 == 0xA9: // POP FS／GS
		var v uint16
		if c.operand32 {
			v = uint16(c.pop32())
		} else {
			v = c.pop()
		}
		if op2 == 0xA1 {
			c.loadSeg(FS, v)
		} else {
			c.loadSeg(GS, v)
		}

	case op2 == 0xA3, op2 == 0xAB, op2 == 0xB3, op2 == 0xBB:
		return c.bitOp(op2)
	case op2 == 0xBA:
		return c.bitOpImm(op2)

	case op2 == 0xA4 || op2 == 0xA5 || op2 == 0xAC || op2 == 0xAD: // SHLD／SHRD
		m := c.decodeModRM()
		var n uint8
		if op2&1 == 0 {
			n = c.fetch8()
		} else {
			n = uint8(c.R[CX])
		}
		if n &= 31; n == 0 {
			break // 計數遮成 0：運算元與旗標都不動
		}
		left := op2 < 0xA8
		if c.operand32 {
			c.set32(m.rm, c.doubleShift32(left, c.get32(m.rm), c.reg32(m.reg), n))
		} else {
			if n > 16 {
				return c.errf(op2, "0F %02X：16 位元的 SHLD／SHRD 計數 %d 超過 16，Intel 標為未定義（docs/spec/197 §2.2）", op2, n)
			}
			c.set16(m.rm, c.doubleShift16(left, c.get16(m.rm), c.R[m.reg], n))
		}

	case op2 == 0xBC || op2 == 0xBD: // BSF／BSR
		m := c.decodeModRM()
		var v uint32
		width := 16
		if c.operand32 {
			v, width = c.get32(m.rm), 32
		} else {
			v = uint32(c.get16(m.rm))
		}
		if v == 0 {
			c.setFlag(ZF, true) // 目的暫存器不動
			break
		}
		c.setFlag(ZF, false)
		idx := bits.TrailingZeros32(v)
		if op2 == 0xBD {
			idx = width - 1 - bits.LeadingZeros32(v<<(32-width))
		}
		if c.operand32 {
			c.setReg32(m.reg, uint32(idx))
		} else {
			c.R[m.reg] = uint16(idx)
		}

	case op2 == 0xAF: // IMUL r, r/m
		m := c.decodeModRM()
		if c.operand32 {
			c.setReg32(m.reg, c.imul32Two(c.reg32(m.reg), c.get32(m.rm)))
		} else {
			p := int32(int16(c.R[m.reg])) * int32(int16(c.get16(m.rm)))
			over := p != int32(int16(p))
			c.setFlag(CF, over)
			c.setFlag(OF, over)
			c.R[m.reg] = uint16(p)
		}

	case op2 == 0xB2 || op2 == 0xB4 || op2 == 0xB5: // LSS／LFS／LGS
		m := c.decodeModRM()
		if m.rm.isReg {
			return c.errf(op2, "0F %02X：來源必須是記憶體", op2)
		}
		c.charge(cycFar)
		// 遠指標先整個讀完，再載入段暫存器，最後寫通用暫存器（同 LES／LDS，434 §5.1）。
		var off uint32
		var seg uint16
		if c.operand32 {
			off = c.get32(m.rm)
			seg = c.get16(m.rm.at(4))
		} else {
			off = uint32(c.get16(m.rm))
			seg = c.get16(m.rm.at(2))
		}
		dst := GS
		switch op2 {
		case 0xB2:
			dst = SS
		case 0xB4:
			dst = FS
		}
		if !c.loadSeg(dst, seg) {
			break
		}
		if c.operand32 {
			c.setReg32(m.reg, off)
		} else {
			c.R[m.reg] = uint16(off)
		}

	case op2 == 0xB6 || op2 == 0xB7 || op2 == 0xBE || op2 == 0xBF: // MOVZX／MOVSX
		m := c.decodeModRM()
		signed := op2 >= 0xBE
		var v uint32
		if op2&1 == 0 { // 位元組來源
			b := c.get8(m.rm)
			v = uint32(b)
			if signed {
				v = uint32(int32(int8(b)))
			}
		} else { // 字來源
			w := c.get16(m.rm)
			v = uint32(w)
			if signed {
				v = uint32(int32(int16(w)))
			}
		}
		if c.operand32 {
			c.setReg32(m.reg, v)
		} else {
			c.R[m.reg] = uint16(v) // 16 位元目的地：高半不動
		}

	default:
		// 433 §4：系統指令只在 Sys 非 nil（保護模式 profile 開啟）時處理；
		// 未處理的照現況失敗即關閉，Sys == nil 時錯誤訊息與現況逐字相同。
		if c.Sys != nil {
			if handled, err := c.execSys0F(op2); handled {
				return err
			}
		}
		return c.errf(op2, "0F %02X：80386 子集沒有這一道（docs/spec/197 §2.2）", op2)
	}
	return nil
}

// jccNear 是 0F 80–8F：rel16（帶 66 為 rel32）。
//
// 實模式 IP 只有 16 位元：rel32 的目標超過 0xFFFF 在真機是 #GP，這裡記 fault。
func (c *CPU) jccNear(op2 uint8) {
	if c.operand32 {
		rel := c.fetch32()
		if c.cond(op2 & 0x0F) {
			c.charge(cycJump)
			target := uint32(c.IP) + rel
			if target > 0xFFFF {
				if c.Code32() {
					c.failf("Jcc rel32 目標 %08X：EIP 超過 FFFFh，docs/spec/443 §2.3", target)
					return
				}
				c.failf("Jcc rel32 目標 %08X 超出實模式 64 KB 段限（docs/spec/197 §2.2）", target)
				return
			}
			c.IP = uint16(target)
		}
		return
	}
	rel := c.fetch16()
	if c.cond(op2 & 0x0F) {
		c.charge(cycJump)
		c.IP += rel
	}
}

// bitOp 是 BT／BTS／BTR／BTC r/m, r（0F A3／AB／B3／BB）。
//
// 位元位移來自暫存器。暫存器運算元取低 4（16 位元）或 5（32 位元）位元；
// **記憶體運算元的位元位移是有號的完整暫存器值**，要連位址一起調整
// （`prefix_0f.h` 的 `eaa += (int16)rmrw >> 4 * 2`）。
func (c *CPU) bitOp(op2 uint8) error {
	m := c.decodeModRM()
	var bit int32
	width := 16
	if c.operand32 {
		bit, width = int32(c.reg32(m.reg)), 32
	} else {
		bit = int32(int16(c.R[m.reg]))
	}
	o := m.rm
	if !o.isReg {
		shift := uint(4)
		if width == 32 {
			shift = 5
		}
		adj := (bit >> shift) * int32(width/8)
		if c.segmented {
			// 434 §4.2：分段模式依偏移寬度相加（16 位元環繞或 32 位元），界限由存取時檢查。
			o = o.at(uint32(adj))
		} else {
			eff := int32(o.off) + adj
			if eff < 0 || eff > 0xFFFF {
				c.failf("BT 系位址調整後 %d 超出實模式 64 KB 段限（docs/spec/197 §2.2）", eff)
				return nil
			}
			o.off = uint32(eff)
		}
	}
	c.bitApply(op2, o, uint(bit)&uint(width-1), width)
	return nil
}

// bitOpImm 是 0F BA /4–/7 imm8：BT／BTS／BTR／BTC r/m, imm8。
func (c *CPU) bitOpImm(op2 uint8) error {
	m := c.decodeModRM()
	width := 16
	if c.operand32 {
		width = 32
	}
	n := uint(c.fetch8()) & uint(width-1)
	var op uint8
	switch m.reg {
	case 4:
		op = 0xA3
	case 5:
		op = 0xAB
	case 6:
		op = 0xB3
	case 7:
		op = 0xBB
	default:
		return c.errf(op2, "0F BA /%d：不存在", m.reg)
	}
	c.bitApply(op, m.rm, n, width)
	return nil
}

// bitApply 做位元測試：CF ＝ 該位元的舊值；BTS 設 1、BTR 清 0、BTC 反轉。
// 其他旗標未定義，不動。
func (c *CPU) bitApply(op uint8, o operand, n uint, width int) {
	var v uint32
	if width == 32 {
		v = c.get32(o)
	} else {
		v = uint32(c.get16(o))
	}
	mask := uint32(1) << n
	c.setFlag(CF, v&mask != 0)
	switch op {
	case 0xAB: // BTS
		v |= mask
	case 0xB3: // BTR
		v &^= mask
	case 0xBB: // BTC
		v ^= mask
	default: // BT：不寫回
		return
	}
	if width == 32 {
		c.set32(o, v)
	} else {
		c.set16(o, uint16(v))
	}
}
