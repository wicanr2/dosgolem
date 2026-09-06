package cpu

import "fmt"

// 80386 最小子集（`docs/spec/012`）：0x66 operand-size 前綴。
//
// **指令清單是掃出來的，不是照手冊列的**——只有 DOSJP.COM 實際用到的
// 六道（CWDE／SHL EAX,imm8／MOV EAX,m16／MOV m16,EAX／MOV [m16],imm32，
// 加上段前綴的組合）。其他 opcode 帶 0x66 一樣報「未實作」——
// 靜靜放行才會讓錯誤看不見。
//
// EAX 的表示：`R[AX]` 是低半、`EAXHi` 是高半。**16 位元寫 AX 不清高半**
// （386 的行為）。

func (c *CPU) getEAX() uint32 { return uint32(c.R[AX]) | uint32(c.EAXHi)<<16 }

func (c *CPU) setEAX(v uint32) {
	c.R[AX] = uint16(v)
	c.EAXHi = uint16(v >> 16)
}

// execute32 是 0x66 之後的分派（`docs/spec/012` §3）。
func (c *CPU) execute32(op uint8) error {
	switch op {
	case 0x98: // CWDE：AX 符號擴展到 EAX
		c.setEAX(uint32(int32(int16(c.R[AX]))))

	case 0xA1: // MOV EAX, [moffs16]
		off := c.fetch16()
		c.setEAX(c.read32(c.dataSeg(DS), off))

	case 0xA3: // MOV [moffs16], EAX
		off := c.fetch16()
		c.write32(c.dataSeg(DS), off, c.getEAX())

	case 0xC1: // SHL EAX, imm8（只支援 modrm E0——DOSJP 的唯一用法）
		m := c.decodeModRM()
		n := c.fetch8()
		if m.reg != 4 || !m.rm.isReg || m.rm.reg != AX {
			return fmt.Errorf("cpu: %04X:%04X 66 C1 /%d：386 子集只有 SHL EAX,imm8",
				c.opCS, c.opIP, m.reg)
		}
		v := c.getEAX()
		res := v << (n & 31)
		if n > 0 && n <= 31 {
			// CF ＝ 最後移出的位元。
			if v>>(32-n)&1 != 0 {
				c.SetFlags(c.Flags | CF)
			} else {
				c.SetFlags(c.Flags &^ CF)
			}
		}
		c.setEAX(res)
		c.setSZP32(res)

	case 0xC7: // MOV dword [m16], imm32（modrm 06 是絕對位址）
		m := c.decodeModRM()
		if m.reg != 0 {
			return fmt.Errorf("cpu: %04X:%04X 66 C7 /%d：386 子集只有 /0",
				c.opCS, c.opIP, m.reg)
		}
		lo, hi := c.fetch16(), c.fetch16()
		c.setOperand32(m.rm, uint32(lo)|uint32(hi)<<16)

	default:
		return fmt.Errorf("cpu: %04X:%04X op 66 %02X：386 子集沒有這一道（docs/spec/012）",
			c.opCS, c.opIP, op)
	}
	return nil
}

func (c *CPU) read32(seg, off uint16) uint32 {
	return uint32(c.read16(seg, off)) | uint32(c.read16(seg, off+2))<<16
}

func (c *CPU) write32(seg, off uint16, v uint32) {
	c.write16(seg, off, uint16(v))
	c.write16(seg, off+2, uint16(v>>16))
}

func (c *CPU) setOperand32(o operand, v uint32) {
	if o.isReg {
		if o.reg != AX {
			panic("386 子集的暫存器運算元只有 EAX")
		}
		c.setEAX(v)
		return
	}
	c.write32(o.seg, o.off, v)
}

// setSZP32 是 32 位元版的 setSZP（SF 看 bit31、ZF 看整個 32 位元）。
func (c *CPU) setSZP32(v uint32) {
	f := c.Flags &^ (SF | ZF | PF)
	if v == 0 {
		f |= ZF
	}
	if v&0x80000000 != 0 {
		f |= SF
	}
	if parity(uint16(v)) {
		f |= PF
	}
	c.SetFlags(f)
}
