// Package cpu386 是與既有 8086 核心隔離的 32-bit 平坦執行核心。
// 目前只涵蓋 docs/spec/008 的 FD2 entry 第一個執行閘門。
package cpu386

import "fmt"

type Bus interface {
	Read8(addr uint32) (uint8, error)
	Write8(addr uint32, value uint8) error
}

const (
	EAX = iota
	ECX
	EDX
	EBX
	ESP
	EBP
	ESI
	EDI
)

const (
	CF uint32 = 1 << 0
	PF uint32 = 1 << 2
	AF uint32 = 1 << 4
	ZF uint32 = 1 << 6
	SF uint32 = 1 << 7
	IF uint32 = 1 << 9
	OF uint32 = 1 << 11
)

type CPU struct {
	R       [8]uint32
	EIP     uint32
	EFlags  uint32
	Bus     Bus
	IntHook func(*CPU, uint8) bool
}

type Error struct {
	EIP    uint32
	Opcode uint8
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("cpu386: EIP=%08X opcode=%02X：%s", e.EIP, e.Opcode, e.Reason)
}

func New(bus Bus) *CPU { return &CPU{Bus: bus, EFlags: 2} }

func (c *CPU) fetch8() (uint8, error) {
	v, err := c.Bus.Read8(c.EIP)
	if err == nil {
		c.EIP++
	}
	return v, err
}

func (c *CPU) fetch16() (uint16, error) {
	a, err := c.fetch8()
	if err != nil {
		return 0, err
	}
	b, err := c.fetch8()
	if err != nil {
		return 0, err
	}
	return uint16(a) | uint16(b)<<8, nil
}

func (c *CPU) fetch32() (uint32, error) {
	a, err := c.fetch16()
	if err != nil {
		return 0, err
	}
	b, err := c.fetch16()
	if err != nil {
		return 0, err
	}
	return uint32(a) | uint32(b)<<16, nil
}

func (c *CPU) write16(addr uint32, value uint16) error {
	if err := c.Bus.Write8(addr, uint8(value)); err != nil {
		return err
	}
	return c.Bus.Write8(addr+1, uint8(value>>8))
}

func (c *CPU) write32(addr uint32, value uint32) error {
	if err := c.write16(addr, uint16(value)); err != nil {
		return err
	}
	return c.write16(addr+2, uint16(value>>16))
}

func (c *CPU) setReg8(index int, value uint8) {
	if index < 4 {
		c.R[index] = c.R[index]&0xffffff00 | uint32(value)
	} else {
		r := index - 4
		c.R[r] = c.R[r]&0xffff00ff | uint32(value)<<8
	}
}

func (c *CPU) reg8(index int) uint8 {
	if index < 4 {
		return uint8(c.R[index])
	}
	return uint8(c.R[index-4] >> 8)
}

func parity(value uint8) bool {
	value ^= value >> 4
	value &= 0xf
	return (0x6996>>value)&1 == 0
}

func (c *CPU) setLogicFlags(value uint32) {
	c.EFlags &^= CF | PF | AF | ZF | SF | OF
	if value == 0 {
		c.EFlags |= ZF
	}
	if value&0x80000000 != 0 {
		c.EFlags |= SF
	}
	if parity(uint8(value)) {
		c.EFlags |= PF
	}
}

func (c *CPU) sub32(left, right uint32) uint32 {
	result := left - right
	c.EFlags &^= CF | PF | AF | ZF | SF | OF
	if left < right {
		c.EFlags |= CF
	}
	if (left^right^result)&0x10 != 0 {
		c.EFlags |= AF
	}
	if result == 0 {
		c.EFlags |= ZF
	}
	if result&0x80000000 != 0 {
		c.EFlags |= SF
	}
	if parity(uint8(result)) {
		c.EFlags |= PF
	}
	if ((left^right)&(left^result))&0x80000000 != 0 {
		c.EFlags |= OF
	}
	return result
}

func (c *CPU) sub16(left, right uint16) uint16 {
	result := left - right
	c.EFlags &^= CF | PF | AF | ZF | SF | OF
	if left < right {
		c.EFlags |= CF
	}
	if (left^right^result)&0x10 != 0 {
		c.EFlags |= AF
	}
	if result == 0 {
		c.EFlags |= ZF
	}
	if result&0x8000 != 0 {
		c.EFlags |= SF
	}
	if parity(uint8(result)) {
		c.EFlags |= PF
	}
	if ((left^right)&(left^result))&0x8000 != 0 {
		c.EFlags |= OF
	}
	return result
}

func (c *CPU) Step() error {
	start := c.EIP
	op, err := c.fetch8()
	if err != nil {
		return &Error{start, 0, err.Error()}
	}
	operand16 := false
	if op == 0x66 {
		operand16 = true
		op, err = c.fetch8()
		if err != nil {
			return &Error{start, 0x66, err.Error()}
		}
	}
	fail := func(reason string) error { return &Error{start, op, reason} }
	switch {
	case op == 0xeb:
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
	case op == 0xfb:
		c.EFlags |= IF
	case op == 0x83:
		if operand16 {
			return fail("16-bit 83 尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 || (modrm>>3)&7 != 4 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		imm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		rm := int(modrm & 7)
		c.R[rm] &= uint32(int32(int8(imm)))
		c.setLogicFlags(c.R[rm])
	case op == 0xc1:
		if operand16 {
			return fail("16-bit C1 尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 || (modrm>>3)&7 != 5 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		countByte, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		count := uint(countByte & 0x1f)
		rm := int(modrm & 7)
		if count != 0 {
			value := c.R[rm]
			result := value >> count
			c.setLogicFlags(result)
			if value>>(count-1)&1 != 0 {
				c.EFlags |= CF
			}
			c.R[rm] = result
		}
	case op == 0x8b:
		if operand16 {
			return fail("16-bit 8B 尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		c.R[(modrm>>3)&7] = c.R[modrm&7]
	case op == 0x89:
		if operand16 {
			return fail("16-bit 89 尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		source := c.R[(modrm>>3)&7]
		if modrm>>6 == 3 {
			c.R[modrm&7] = source
		} else if modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if e = c.write32(addr, source); e != nil {
				return fail(e.Error())
			}
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op == 0x88:
		if operand16 {
			return fail("88 不接受 operand-size override")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		value := c.reg8(int((modrm >> 3) & 7))
		if modrm>>6 == 3 {
			c.setReg8(int(modrm&7), value)
		} else if modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if e = c.Bus.Write8(addr, value); e != nil {
				return fail(e.Error())
			}
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op >= 0xb8 && op <= 0xbf:
		reg := int(op - 0xb8)
		if operand16 {
			value, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(value)
		} else {
			value, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			c.R[reg] = value
		}
	case op == 0xa3:
		addr, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if operand16 {
			e = c.write16(addr, uint16(c.R[EAX]))
		} else {
			e = c.write32(addr, c.R[EAX])
		}
		if e != nil {
			return fail(e.Error())
		}
	case op == 0xa2:
		if operand16 {
			return fail("A2 不接受 operand-size override")
		}
		addr, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if e = c.Bus.Write8(addr, c.reg8(0)); e != nil {
			return fail(e.Error())
		}
	case op >= 0xb0 && op <= 0xb7:
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.setReg8(int(op-0xb0), value)
	case op == 0x2b:
		if operand16 {
			return fail("16-bit 2B 尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		reg, rm := (modrm>>3)&7, modrm&7
		c.R[reg] = c.sub32(c.R[reg], c.R[rm])
	case op == 0x3d:
		if !operand16 {
			return fail("32-bit CMP EAX,imm32 尚未支援")
		}
		value, e := c.fetch16()
		if e != nil {
			return fail(e.Error())
		}
		c.sub16(uint16(c.R[EAX]), value)
	case op == 0x0f:
		extended, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if extended != 0x85 || operand16 {
			return fail(fmt.Sprintf("0F %02X 尚未支援", extended))
		}
		delta, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&ZF == 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int32(delta)))
		}
	case op == 0xcd:
		number, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.IntHook == nil || !c.IntHook(c, number) {
			return fail(fmt.Sprintf("INT %02X 未處理", number))
		}
	default:
		return fail("opcode 尚未支援")
	}
	return nil
}
