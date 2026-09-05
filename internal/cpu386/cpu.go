// Package cpu386 是與既有 8086 核心隔離的 32-bit 平坦執行核心。
// 目前依文件化切片擴充 DOS/4GW 啟動路徑；未列形狀一律失敗即關閉。
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
	SegCS = iota
	SegDS
	SegES
	SegFS
	SegGS
	SegSS
)

const (
	CF uint32 = 1 << 0
	PF uint32 = 1 << 2
	AF uint32 = 1 << 4
	ZF uint32 = 1 << 6
	SF uint32 = 1 << 7
	IF uint32 = 1 << 9
	DF uint32 = 1 << 10
	OF uint32 = 1 << 11
)

type CPU struct {
	R             [8]uint32
	Seg           [6]uint16
	EIP           uint32
	EFlags        uint32
	Bus           Bus
	IntHook       func(*CPU, uint8) bool
	SegmentRead16 func(selector uint16, offset uint32) (uint16, bool)
	SegmentRead8  func(selector uint16, offset uint32) (uint8, bool)
	SegmentLoadOK func(selector uint16, destination int) bool
	Descriptors   map[uint16]Descriptor
}

type Descriptor struct {
	Base     uint32
	Limit    uint32
	Writable bool
}

type Error struct {
	EIP    uint32
	Opcode uint8
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("cpu386: EIP=%08X opcode=%02X：%s", e.EIP, e.Opcode, e.Reason)
}

func New(bus Bus) *CPU { return &CPU{Bus: bus, EFlags: 2, Descriptors: make(map[uint16]Descriptor)} }

func (c *CPU) SetDescriptor(selector uint16, descriptor Descriptor) {
	c.Descriptors[selector] = descriptor
}

func (c *CPU) canLoadSegment(selector uint16, destination int) bool {
	if _, ok := c.Descriptors[selector]; ok {
		return true
	}
	return c.SegmentLoadOK != nil && c.SegmentLoadOK(selector, destination)
}

func (c *CPU) segmentLinear(selector uint16, offset uint32, size uint32, write bool) (uint32, bool) {
	d, ok := c.Descriptors[selector]
	if !ok || size == 0 || (write && !d.Writable) || offset > d.Limit || size-1 > d.Limit-offset {
		return 0, false
	}
	linear := uint64(d.Base) + uint64(offset)
	if linear+uint64(size) > uint64(^uint32(0))+1 {
		return 0, false
	}
	return uint32(linear), true
}

func (c *CPU) writeSegment16(selector uint16, offset uint32, value uint16) bool {
	linear, ok := c.segmentLinear(selector, offset, 2, true)
	if !ok || c.write16(linear, value) != nil {
		return false
	}
	return true
}

func (c *CPU) readSegment8(selector uint16, offset uint32) (uint8, bool) {
	if c.SegmentRead8 != nil {
		if value, ok := c.SegmentRead8(selector, offset); ok {
			return value, true
		}
	}
	linear, ok := c.segmentLinear(selector, offset, 1, false)
	if !ok {
		return 0, false
	}
	value, err := c.Bus.Read8(linear)
	return value, err == nil
}

func (c *CPU) readSegment16(selector uint16, offset uint32) (uint16, bool) {
	if c.SegmentRead16 != nil {
		if value, ok := c.SegmentRead16(selector, offset); ok {
			return value, true
		}
	}
	linear, ok := c.segmentLinear(selector, offset, 2, false)
	if !ok {
		return 0, false
	}
	value, err := c.read16(linear)
	return value, err == nil
}

func (c *CPU) writeSegment8(selector uint16, offset uint32, value uint8) bool {
	linear, ok := c.segmentLinear(selector, offset, 1, true)
	return ok && c.Bus.Write8(linear, value) == nil
}

func (c *CPU) readSegment32(selector uint16, offset uint32) (uint32, bool) {
	if c.SegmentRead8 != nil {
		var value uint32
		for i := uint32(0); i < 4; i++ {
			part, ok := c.SegmentRead8(selector, offset+i)
			if !ok {
				value = 0
				break
			}
			value |= uint32(part) << (8 * i)
			if i == 3 {
				return value, true
			}
		}
	}
	linear, ok := c.segmentLinear(selector, offset, 4, false)
	if !ok {
		return 0, false
	}
	a, err := c.read16(linear)
	if err != nil {
		return 0, false
	}
	b, err := c.read16(linear + 2)
	return uint32(a) | uint32(b)<<16, err == nil
}

func (c *CPU) writeSegment32(selector uint16, offset uint32, value uint32) bool {
	linear, ok := c.segmentLinear(selector, offset, 4, true)
	if !ok || c.write32(linear, value) != nil {
		return false
	}
	return true
}

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

func (c *CPU) read16(addr uint32) (uint16, error) {
	a, err := c.Bus.Read8(addr)
	if err != nil {
		return 0, err
	}
	b, err := c.Bus.Read8(addr + 1)
	if err != nil {
		return 0, err
	}
	return uint16(a) | uint16(b)<<8, nil
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

func (c *CPU) setLogicFlags8(value uint8) {
	c.EFlags &^= CF | PF | AF | ZF | SF | OF
	if value == 0 {
		c.EFlags |= ZF
	}
	if value&0x80 != 0 {
		c.EFlags |= SF
	}
	if parity(value) {
		c.EFlags |= PF
	}
}

func (c *CPU) add32(left, right uint32) uint32 {
	result := left + right
	c.EFlags &^= CF | PF | AF | ZF | SF | OF
	if result < left {
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
	if (^(left ^ right) & (left ^ result) & 0x80000000) != 0 {
		c.EFlags |= OF
	}
	return result
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

func (c *CPU) sub8(left, right uint8) uint8 {
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
	if result&0x80 != 0 {
		c.EFlags |= SF
	}
	if parity(result) {
		c.EFlags |= PF
	}
	if ((left^right)&(left^result))&0x80 != 0 {
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
	segmentOverride := -1
	repe := false
	for op == 0x66 || op == 0x26 || op == 0xf3 {
		switch op {
		case 0x66:
			if operand16 {
				return &Error{start, op, "重複 operand-size prefix"}
			}
			operand16 = true
		case 0x26:
			if segmentOverride >= 0 {
				return &Error{start, op, "重複 segment prefix"}
			}
			segmentOverride = SegES
		case 0xf3:
			if repe {
				return &Error{start, op, "重複 REPE prefix"}
			}
			repe = true
		}
		op, err = c.fetch8()
		if err != nil {
			return &Error{start, 0, err.Error()}
		}
	}
	fail := func(reason string) error { return &Error{start, op, reason} }
	if repe && op != 0xae {
		return fail("REPE prefix 只支援 SCASB")
	}
	if segmentOverride >= 0 && op != 0x8a && op != 0x8b && op != 0x8c && op != 0x8e {
		return fail("segment override 只支援 8A／8B／8C／8E")
	}
	switch {
	case op >= 0x48 && op <= 0x4f:
		if operand16 {
			return fail("16-bit DEC 尚未支援")
		}
		reg := int(op - 0x48)
		carry := c.EFlags & CF
		c.R[reg] = c.sub32(c.R[reg], 1)
		c.EFlags = c.EFlags&^CF | carry
	case op >= 0x58 && op <= 0x5f:
		if operand16 {
			return fail("16-bit POP 尚未支援")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		c.R[op-0x58] = value
		c.R[ESP] += 4
	case op == 0xaa:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("STOSB 不接受目前的 prefix")
		}
		if !c.writeSegment8(c.Seg[SegES], c.R[EDI], c.reg8(0)) {
			return fail(fmt.Sprintf("STOSB write %04X:%08X 未處理", c.Seg[SegES], c.R[EDI]))
		}
		if c.EFlags&DF != 0 {
			c.R[EDI]--
		} else {
			c.R[EDI]++
		}
	case op == 0xfc:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("CLD 不接受目前的 prefix")
		}
		c.EFlags &^= DF
	case op == 0xae:
		if operand16 || segmentOverride >= 0 {
			return fail("SCASB 不接受目前的 prefix")
		}
		count := uint32(1)
		if repe {
			count = c.R[ECX]
		}
		for count > 0 {
			value, ok := c.readSegment8(c.Seg[SegES], c.R[EDI])
			if !ok {
				return fail(fmt.Sprintf("SCASB read %04X:%08X 未處理", c.Seg[SegES], c.R[EDI]))
			}
			c.sub8(c.reg8(0), value)
			if c.EFlags&DF != 0 {
				c.R[EDI]--
			} else {
				c.R[EDI]++
			}
			if repe {
				c.R[ECX]--
				count = c.R[ECX]
				if c.EFlags&ZF == 0 {
					break
				}
			} else {
				break
			}
		}
	case op >= 0x50 && op <= 0x57:
		if operand16 {
			return fail("16-bit PUSH 尚未支援")
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, c.R[op-0x50]) {
			return fail(fmt.Sprintf("stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
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
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		imm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		rm := int(modrm & 7)
		switch (modrm >> 3) & 7 {
		case 0:
			c.R[rm] = c.add32(c.R[rm], uint32(int32(int8(imm))))
		case 4:
			c.R[rm] &= uint32(int32(int8(imm)))
			c.setLogicFlags(c.R[rm])
		default:
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op == 0x80:
		if operand16 {
			return fail("80 不接受 operand-size override")
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
		result := c.reg8(rm) & imm
		c.setReg8(rm, result)
		c.setLogicFlags8(result)
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
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if operand16 && segmentOverride == SegES && modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[segmentOverride], addr)
			if !ok {
				return fail(fmt.Sprintf("segment word read %04X:%08X 未處理", c.Seg[segmentOverride], addr))
			}
			reg := (modrm >> 3) & 7
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(value)
		} else if operand16 {
			return fail(fmt.Sprintf("16-bit ModRM %02X 尚未支援", modrm))
		} else if segmentOverride < 0 && modrm>>6 == 0 && modrm&7 == 6 {
			value, ok := c.readSegment32(c.Seg[SegDS], c.R[ESI])
			if !ok {
				return fail(fmt.Sprintf("segment dword read %04X:%08X 未處理", c.Seg[SegDS], c.R[ESI]))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride >= 0 {
			return fail(fmt.Sprintf("segment ModRM %02X 尚未支援", modrm))
		} else if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		} else {
			c.R[(modrm>>3)&7] = c.R[modrm&7]
		}
	case op == 0x89:
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		source := c.R[(modrm>>3)&7]
		if operand16 && segmentOverride < 0 && modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if e = c.write16(addr, uint16(source)); e != nil {
				return fail(e.Error())
			}
		} else if operand16 {
			return fail(fmt.Sprintf("16-bit ModRM %02X 尚未支援", modrm))
		} else if modrm>>6 == 3 {
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
	case op == 0x8c:
		if operand16 {
			return fail("8C 不接受 operand-size override")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		encoding := int((modrm >> 3) & 7)
		segmentByEncoding := [...]int{SegES, SegCS, SegSS, SegDS, SegFS, SegGS}
		if encoding >= len(segmentByEncoding) {
			return fail(fmt.Sprintf("segment 編碼 %d 無效", encoding))
		}
		value := c.Seg[segmentByEncoding[encoding]]
		if modrm>>6 == 3 {
			c.R[modrm&7] = uint32(value)
		} else if modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if segmentOverride >= 0 {
				if !c.writeSegment16(c.Seg[segmentOverride], addr, value) {
					return fail(fmt.Sprintf("segment word write %04X:%08X 未處理", c.Seg[segmentOverride], addr))
				}
			} else if e = c.write16(addr, value); e != nil {
				return fail(e.Error())
			}
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op == 0x8a:
		if operand16 || segmentOverride < 0 {
			return fail("8A 目前只支援 segment byte read")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 1 || modrm&7 == 4 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		addr := uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
		value, ok := c.readSegment8(c.Seg[segmentOverride], addr)
		if !ok {
			return fail(fmt.Sprintf("segment byte read %04X:%08X 未處理", c.Seg[segmentOverride], addr))
		}
		c.setReg8(int((modrm>>3)&7), value)
	case op == 0x8d:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("LEA 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 1 || modrm&7 == 4 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.R[(modrm>>3)&7] = uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
	case op == 0x8e:
		if operand16 {
			return fail("8E 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		encoding := int((modrm >> 3) & 7)
		segmentByEncoding := [...]int{SegES, -1, SegSS, SegDS, SegFS, SegGS}
		if encoding >= len(segmentByEncoding) || segmentByEncoding[encoding] < 0 {
			return fail(fmt.Sprintf("segment 編碼 %d 無效", encoding))
		}
		destination := segmentByEncoding[encoding]
		var value uint16
		if modrm>>6 == 3 {
			value = uint16(c.R[modrm&7])
		} else if modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if segmentOverride >= 0 {
				var ok bool
				value, ok = c.readSegment16(c.Seg[segmentOverride], addr)
				if !ok {
					return fail(fmt.Sprintf("segment word read %04X:%08X 未處理", c.Seg[segmentOverride], addr))
				}
			} else {
				value, e = c.read16(addr)
				if e != nil {
					return fail(e.Error())
				}
			}
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		if !c.canLoadSegment(value, destination) {
			return fail(fmt.Sprintf("selector %04X 不可載入", value))
		}
		c.Seg[destination] = value
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
	case op == 0x2a:
		if operand16 || segmentOverride >= 0 {
			return fail("2A 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		reg, rm := int((modrm>>3)&7), int(modrm&7)
		c.setReg8(reg, c.sub8(c.reg8(reg), c.reg8(rm)))
	case op == 0x3d:
		if !operand16 {
			return fail("32-bit CMP EAX,imm32 尚未支援")
		}
		value, e := c.fetch16()
		if e != nil {
			return fail(e.Error())
		}
		c.sub16(uint16(c.R[EAX]), value)
	case op == 0x3c:
		if operand16 {
			return fail("3C 不接受 operand-size override")
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.sub8(uint8(c.R[EAX]), value)
	case op == 0x74:
		if operand16 {
			return fail("74 不接受 operand-size override")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&ZF != 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x0f:
		extended, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (extended != 0x84 && extended != 0x85) || operand16 {
			return fail(fmt.Sprintf("0F %02X 尚未支援", extended))
		}
		delta, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		shouldJump := extended == 0x84 && c.EFlags&ZF != 0 || extended == 0x85 && c.EFlags&ZF == 0
		if shouldJump {
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
