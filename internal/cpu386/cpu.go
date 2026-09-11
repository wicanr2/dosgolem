// Package cpu386 是與既有 8086 核心隔離的 32-bit 平坦執行核心。
// 目前依文件化切片擴充 DOS/4GW 啟動路徑；未列形狀一律失敗即關閉。
package cpu386

import (
	"fmt"
	"math"
)

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
	R       [8]uint32
	Seg     [6]uint16
	EIP     uint32
	EFlags  uint32
	Bus     Bus
	IntHook func(*CPU, uint8) bool
	// PortOut 回 true 表示 byte 輸出已被平台層接受；未安裝或拒絕時失敗即關閉。
	PortOut func(port uint16, value uint8) bool
	// PortIn 僅接受平台明確支援的 byte 輸入。
	PortIn func(port uint16) (uint8, bool)
	// StepHook 可在已登錄的執行期函式入口取代一次指令步進。
	// 回傳 handled=false 時仍由 CPU 正常解碼，維持失敗即關閉。
	StepHook      func(*CPU) (handled bool, err error)
	SegmentRead16 func(selector uint16, offset uint32) (uint16, bool)
	SegmentRead8  func(selector uint16, offset uint32) (uint8, bool)
	SegmentLoadOK func(selector uint16, destination int) bool
	Descriptors   map[uint16]Descriptor

	// descCache 是 Descriptors 的唯讀快取。保護模式下每一次記憶體存取都要把
	// selector 解析成 base／limit，`map[uint16]` 的雜湊在剖析中佔了整體執行
	// 時間約四分之一（見 SetDescriptor 的說明）。這裡只放正向結果，
	// SetDescriptor 一律整份失效，因此可見行為與直接查 map 完全相同。
	descCache  [descCacheEntries]descCacheEntry
	FPUControl uint16
	FPUStatus  uint16
	FPUStack   [8]float64
	FPUDepth   uint8
}

type Descriptor struct {
	Base     uint32
	Limit    uint32
	Writable bool
}

// descCacheEntries 是 selector 快取的組數，必須是 2 的冪。索引取
// `selector>>3`（descriptor index）的低位；DOS/4GW 下同時活著的 selector
// 只有個位數，撞號時退回 map，不影響正確性。
const descCacheEntries = 16

type descCacheEntry struct {
	selector uint16
	valid    bool
	d        Descriptor
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

// SetDescriptor 是 Descriptors 的唯一寫入端（全庫只有這裡寫，其餘都是讀），
// 因此在這裡整份清掉 descCache 就足以維持一致。descriptor 的變更相對於
// 記憶體存取是極罕見事件，整份失效比逐項維護簡單且不易出錯。
func (c *CPU) SetDescriptor(selector uint16, descriptor Descriptor) {
	c.Descriptors[selector] = descriptor
	c.descCache = [descCacheEntries]descCacheEntry{}
}

func (c *CPU) ReadSegment8(selector uint16, offset uint32) (uint8, bool) {
	return c.readSegment8(selector, offset)
}

// WriteSegmentBytes 先驗證完整 descriptor 範圍，再依序寫入資料。
func (c *CPU) WriteSegmentBytes(selector uint16, offset uint32, data []byte) bool {
	if len(data) == 0 {
		return true
	}
	linear, ok := c.segmentLinear(selector, offset, uint32(len(data)), true)
	if !ok {
		return false
	}
	for index, value := range data {
		if c.Bus.Write8(linear+uint32(index), value) != nil {
			return false
		}
	}
	return true
}

func (c *CPU) canLoadSegment(selector uint16, destination int) bool {
	if selector == 0 && destination != SegCS && destination != SegSS {
		return true
	}
	if _, ok := c.Descriptors[selector]; ok {
		return true
	}
	return c.SegmentLoadOK != nil && c.SegmentLoadOK(selector, destination)
}

func (c *CPU) segmentLinear(selector uint16, offset uint32, size uint32, write bool) (uint32, bool) {
	entry := &c.descCache[(selector>>3)&(descCacheEntries-1)]
	if !entry.valid || entry.selector != selector {
		if !c.fillDescCache(entry, selector) {
			return 0, false
		}
	}
	d := entry.d
	if size == 0 || (write && !d.Writable) || offset > d.Limit || size-1 > d.Limit-offset {
		return 0, false
	}
	linear := uint64(d.Base) + uint64(offset)
	if linear+uint64(size) > uint64(^uint32(0))+1 {
		return 0, false
	}
	return uint32(linear), true
}

// fillDescCache 是 selector 解析的慢路徑：查 map 並填入快取。只快取查得到的
// selector；查不到不留任何紀錄，之後 SetDescriptor 補上時才不會被一筆過期的
// 否定結果擋住。獨立成函式只是為了讓 segmentLinear 的快路徑好讀；編譯器是否
// 把它 inline 回去對產出的行為沒有差別。
func (c *CPU) fillDescCache(entry *descCacheEntry, selector uint16) bool {
	d, ok := c.Descriptors[selector]
	if !ok {
		return false
	}
	entry.selector, entry.d, entry.valid = selector, d, true
	return true
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

func (c *CPU) add16(a, value uint16) uint16 {
	result := a + value
	c.setLogicFlags16(result)
	if uint32(a)+uint32(value) > 0xffff {
		c.EFlags |= CF
	}
	if (^(a ^ value) & (a ^ result) & 0x8000) != 0 {
		c.EFlags |= OF
	}
	if (a^value^result)&0x10 != 0 {
		c.EFlags |= AF
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

func (c *CPU) add8(left, right uint8) uint8 {
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
	if result&0x80 != 0 {
		c.EFlags |= SF
	}
	if parity(result) {
		c.EFlags |= PF
	}
	if (^(left ^ right) & (left ^ result) & 0x80) != 0 {
		c.EFlags |= OF
	}
	return result
}

func (c *CPU) Step() error {
	if c.StepHook != nil {
		handled, err := c.StepHook(c)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	start := c.EIP
	op, err := c.fetch8()
	if err != nil {
		return &Error{start, 0, err.Error()}
	}
	operand16 := false
	segmentOverride := -1
	repe := false
	repne := false
	for op == 0x66 || op == 0x26 || op == 0x2e || op == 0x36 || op == 0xf2 || op == 0xf3 {
		switch op {
		case 0x66:
			if operand16 {
				return &Error{start, op, "重複 operand-size prefix"}
			}
			operand16 = true
		case 0x26, 0x2e, 0x36:
			if segmentOverride >= 0 {
				return &Error{start, op, "重複 segment prefix"}
			}
			segmentOverride = SegES
			if op == 0x2e {
				segmentOverride = SegCS
			}
			if op == 0x36 {
				segmentOverride = SegSS
			}
		case 0xf3:
			if repe || repne {
				return &Error{start, op, "重複或衝突的 repeat prefix"}
			}
			repe = true
		case 0xf2:
			if repe || repne {
				return &Error{start, op, "重複或衝突的 repeat prefix"}
			}
			repne = true
		}
		op, err = c.fetch8()
		if err != nil {
			return &Error{start, 0, err.Error()}
		}
	}
	fail := func(reason string) error { return &Error{start, op, reason} }
	if repe && op != 0xaa && op != 0xab && op != 0xae && op != 0xa5 && op != 0xa4 {
		return fail("REP／REPE prefix 只支援 STOSB／STOSD／SCASB／MOVSD／MOVSB")
	}
	if repne && op != 0xae && op != 0xa4 && op != 0xa5 {
		return fail("F2 prefix 只支援 SCASB／MOVSB／MOVSD")
	}
	if segmentOverride == SegSS && (op != 0x89 || !operand16 || repe || repne) {
		return fail("SS override 只支援 16-bit MOV memory store")
	}
	if segmentOverride == SegCS && op != 0xff && op != 0x8a {
		return fail("CS override 只支援間接 JMP／MOV byte load")
	}
	if segmentOverride >= 0 && !(segmentOverride == SegSS && op == 0x89) && !(segmentOverride == SegCS && op == 0xff) && !(segmentOverride == SegES && op == 0x0f) && op != 0x80 && op != 0x8a && op != 0x8b && op != 0x8c && op != 0x8e {
		return fail("segment override 只支援 8A／8B／8C／8E")
	}
	switch {
	case op == 0xf8 || op == 0xf9:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("CLC／STC前綴未支援")
		}
		if op == 0xf8 {
			c.EFlags &^= CF
		} else {
			c.EFlags |= CF
		}

	case op == 0x86:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("XCHG byte prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail("XCHG byte僅支援暫存器")
		}
		a, b := int((modrm>>3)&7), int(modrm&7)
		av, bv := c.reg8(a), c.reg8(b)
		c.setReg8(a, bv)
		c.setReg8(b, av)
	case op == 0x32:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("XOR byte prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail("XOR byte僅支援暫存器")
		}
		dst, src := int((modrm>>3)&7), int(modrm&7)
		v := c.reg8(dst) ^ c.reg8(src)
		c.setReg8(dst, v)
		c.setLogicFlags8(v)
	case op == 0x33:
		if operand16 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 3 {
				return fail("word XOR僅支援暫存器")
			}
			dst, src := (modrm>>3)&7, modrm&7
			v := uint16(c.R[dst]) ^ uint16(c.R[src])
			c.R[dst] = c.R[dst]&0xffff0000 | uint32(v)
			c.setLogicFlags16(v)
			break
		}
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("XOR33 prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		value := c.R[modrm&7]
		if modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			var ok bool
			value, ok = c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("XOR來源越界")
			}
		}
		dst := (modrm >> 3) & 7
		c.R[dst] ^= value
		c.setLogicFlags(c.R[dst])

	case op == 0x35:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("XOR EAX prefix尚未支援")
		}
		imm, err := c.fetch32()
		if err != nil {
			return fail(err.Error())
		}
		c.R[EAX] ^= imm
		c.setLogicFlags(c.R[EAX])
	case op == 0x98:
		if segmentOverride >= 0 || repe || repne {
			return fail("98 prefix未支援")
		}
		if operand16 {
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(uint16(int16(int8(c.R[EAX]))))
		} else {
			c.R[EAX] = uint32(int32(int16(c.R[EAX])))
		}
	case op == 0x99:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("CDQ prefix尚未支援")
		}
		c.R[EDX] = uint32(int32(c.R[EAX]) >> 31)
	case op == 0xe2:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("LOOP prefix 尚未支援")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.R[ECX]--
		if c.R[ECX] != 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0xe3:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("JECXZ prefix 尚未支援")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.R[ECX] == 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0xec:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("IN prefix 尚未支援")
		}
		if c.PortIn == nil {
			return fail("IN 未安裝平台輸入")
		}
		port := uint16(c.R[EDX])
		value, ok := c.PortIn(port)
		if !ok {
			return fail(fmt.Sprintf("IN port %04X 未處理", port))
		}
		c.setReg8(EAX, value)
	case op == 0xa9:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("TEST EAX prefix 尚未支援")
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.setLogicFlags(c.R[EAX] & value)
	case op == 0x1b:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("SBB prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail("SBB記憶體形狀尚未支援")
		}
		dst := modrm >> 3 & 7
		left, right := c.R[dst], c.R[modrm&7]
		carry := c.EFlags & CF
		result := c.sub32(left, right+carry)
		c.EFlags &^= CF | AF | OF
		if uint64(left) < uint64(right)+uint64(carry) {
			c.EFlags |= CF
		}
		if (left^right^result)&0x10 != 0 {
			c.EFlags |= AF
		}
		if ((left^right)&(left^result))&0x80000000 != 0 {
			c.EFlags |= OF
		}
		c.R[dst] = result
	case op == 0xa8:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("AL immediate 不接受 prefix")
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		result := uint8(c.R[EAX]) & value
		c.setLogicFlags8(result)

	case op == 0x60 || op == 0x61:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("PUSHAD／POPAD prefix未支援")
		}
		oldESP := c.R[ESP]
		if op == 0x60 {
			if oldESP < 32 {
				return fail("PUSHAD ESP下溢")
			}
			next := oldESP - 32
			if _, ok := c.segmentLinear(c.Seg[SegSS], next, 32, true); !ok {
				return fail("PUSHAD堆疊拒絕")
			}
			values := [8]uint32{c.R[EDI], c.R[ESI], c.R[EBP], oldESP, c.R[EBX], c.R[EDX], c.R[ECX], c.R[EAX]}
			for i, v := range values {
				if !c.writeSegment32(c.Seg[SegSS], next+uint32(i)*4, v) {
					return fail("PUSHAD寫入失敗")
				}
			}
			c.R[ESP] = next
		} else {
			if oldESP > ^uint32(0)-32 {
				return fail("POPAD ESP溢位")
			}
			if _, ok := c.segmentLinear(c.Seg[SegSS], oldESP, 32, false); !ok {
				return fail("POPAD堆疊拒絕")
			}
			next := c.R
			for i, r := range []int{EDI, ESI, EBP, ESP, EBX, EDX, ECX, EAX} {
				if r == ESP {
					continue
				}
				v, ok := c.readSegment32(c.Seg[SegSS], oldESP+uint32(i)*4)
				if !ok {
					return fail("POPAD讀取失敗")
				}
				next[r] = v
			}
			next[ESP] = oldESP + 32
			c.R = next
		}
	case op == 0xa0:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("A0 prefix未支援")
		}
		addr, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		v, ok := c.readSegment8(c.Seg[SegDS], addr)
		if !ok {
			return fail("A0來源越界")
		}
		c.setReg8(0, v)
	case op == 0x68:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("68 不接受目前的 prefix")
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, value) {
			return fail(fmt.Sprintf("PUSH immediate dword stack write %04X:%08X 尚未支援", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0x6a:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("6A 不接受目前的 prefix")
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, uint32(int32(int8(value)))) {
			return fail(fmt.Sprintf("PUSH immediate stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0xf7:
		if segmentOverride >= 0 || repe || repne {
			return fail("F7 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if operand16 && modrm>>6 == 3 && (modrm>>3)&7 == 4 {
			product := uint32(uint16(c.R[EAX])) * uint32(uint16(c.R[modrm&7]))
			c.R[EAX] = c.R[EAX]&0xffff0000 | product&0xffff
			c.R[EDX] = c.R[EDX]&0xffff0000 | product>>16
			c.EFlags &^= CF | OF
			if product>>16 != 0 {
				c.EFlags |= CF | OF
			}
			break
		}
		if !operand16 && modrm>>6 == 3 && (modrm>>3)&7 == 4 {
			product := uint64(c.R[EAX]) * uint64(c.R[modrm&7])
			c.R[EAX] = uint32(product)
			c.R[EDX] = uint32(product >> 32)
			c.EFlags &^= CF | OF
			if c.R[EDX] != 0 {
				c.EFlags |= CF | OF
			}
			break
		}

		if !operand16 && (modrm>>3)&7 == 7 {
			source := c.R[modrm&7]
			if modrm>>6 != 3 {
				seg, addr, e := c.decodeAddress32(modrm)
				if e != nil {
					return fail(e.Error())
				}
				var ok bool
				source, ok = c.readSegment32(c.Seg[seg], addr)
				if !ok {
					return fail("IDIV來源越界")
				}
			}
			divisor := int64(int32(source))
			dividend := int64(uint64(c.R[EDX])<<32 | uint64(c.R[EAX]))
			if divisor == 0 {
				return fail("IDIV 除以零")
			}
			if dividend == (-1<<63) && divisor == -1 {
				return fail("IDIV 商溢位")
			}
			q, r := dividend/divisor, dividend%divisor
			if q < -2147483648 || q > 2147483647 {
				return fail("IDIV 商溢位")
			}
			c.R[EAX], c.R[EDX] = uint32(q), uint32(r)
			break
		}
		if !operand16 && (modrm>>3)&7 == 6 {
			source := c.R[modrm&7]
			if modrm>>6 != 3 {
				seg, addr, err := c.decodeAddress32(modrm)
				if err != nil {
					return fail(err.Error())
				}
				var ok bool
				source, ok = c.readSegment32(c.Seg[seg], addr)
				if !ok {
					return fail("DIV來源越界")
				}
			}
			divisor := uint64(source)
			dividend := uint64(c.R[EDX])<<32 | uint64(c.R[EAX])
			if divisor == 0 {
				return fail("DIV 除以零")
			}
			q := dividend / divisor
			if q > 0xffffffff {
				return fail("DIV 商溢位")
			}
			c.R[EAX] = uint32(q)
			c.R[EDX] = uint32(dividend % divisor)
			break
		}

		if operand16 {
			if modrm>>6 != 3 || (modrm>>3)&7 != 0 {
				return fail("F7 word形狀尚未支援")
			}
			imm, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			c.setLogicFlags16(uint16(c.R[modrm&7]) & imm)
			break
		}
		if modrm>>6 != 3 && (modrm>>3)&7 == 0 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			mask, err := c.fetch32()
			if err != nil {
				return fail(err.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("TEST dword來源越界")
			}
			c.setLogicFlags(value & mask)
			break
		}

		if modrm == 0x5c {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("stack NEG SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[ESP]) + int64(int8(delta)))
			value, ok := c.readSegment32(c.Seg[SegSS], addr)
			if !ok {
				return fail(fmt.Sprintf("stack NEG read %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
			result := uint32(0) - value
			if !c.writeSegment32(c.Seg[SegSS], addr, result) {
				return fail(fmt.Sprintf("stack NEG write %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
			c.sub32(0, value)
			break
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("F7 ModRM %02X 尚未支援", modrm))
		}
		reg := modrm & 7
		switch (modrm >> 3) & 7 {
		case 2:
			c.R[reg] = ^c.R[reg]
		case 3:
			c.R[reg] = c.sub32(0, c.R[reg])

		default:
			return fail(fmt.Sprintf("F7 ModRM %02X 尚未支援", modrm))
		}

	case op == 0x01 || op == 0x29:
		if (operand16 && op != 0x01) || segmentOverride >= 0 || repe || repne {
			return fail("ADD/SUB prefix尚未支援")
		}
		modrm, err := c.fetch8()
		if err != nil {
			return fail(err.Error())
		}
		if operand16 {
			src := uint16(c.R[modrm>>3&7])
			if modrm>>6 == 3 {
				dst := modrm & 7
				result := c.add16(uint16(c.R[dst]), src)
				c.R[dst] = c.R[dst]&0xffff0000 | uint32(result)
				break
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("word ADD 來源越界")
			}
			if !c.writeSegment16(c.Seg[seg], addr, value+src) {
				return fail("word ADD 目的拒絕")
			}
			c.add16(value, src)
			break
		}
		src := c.R[modrm>>3&7]
		if modrm>>6 == 3 {
			dst := modrm & 7
			if op == 0x01 {
				c.R[dst] = c.add32(c.R[dst], src)
			} else {
				c.R[dst] = c.sub32(c.R[dst], src)
			}
			break
		}
		seg, addr, err := c.decodeAddress32(modrm)
		if err != nil {
			return fail(err.Error())
		}
		value, ok := c.readSegment32(c.Seg[seg], addr)
		if !ok {
			return fail("ADD/SUB來源越界")
		}
		result := value + src
		if op == 0x29 {
			result = value - src
		}
		if !c.writeSegment32(c.Seg[seg], addr, result) {
			return fail("ADD/SUB目的拒絕")
		}
		if op == 0x01 {
			c.add32(value, src)
		} else {
			c.sub32(value, src)
		}
	case op == 0x03:
		if operand16 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 3 {
				return fail("word ADD僅支援暫存器")
			}
			dst, src := (modrm>>3)&7, modrm&7
			result := c.add16(uint16(c.R[dst]), uint16(c.R[src]))
			c.R[dst] = c.R[dst]&0xffff0000 | uint32(result)
			break
		}
		if operand16 || segmentOverride >= 0 || repe {
			return fail("03 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 == 3 {
			dst, src := (modrm>>3)&7, modrm&7
			c.R[dst] = c.add32(c.R[dst], c.R[src])
			break
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		value, ok := c.readSegment32(c.Seg[seg], addr)
		if !ok {
			return fail("ADD來源越界")
		}
		dst := (modrm >> 3) & 7
		c.R[dst] = c.add32(c.R[dst], value)
	case op == 0x39:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("39 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("CMP dword ModRM %02X 尚未支援", modrm))
		}
		dst, src := modrm&7, (modrm>>3)&7
		c.sub32(c.R[dst], c.R[src])
	case op == 0x09:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("09 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 == 3 {
			dst, src := modrm&7, (modrm>>3)&7
			c.R[dst] |= c.R[src]
			c.setLogicFlags(c.R[dst])
			break
		}
		if modrm>>6 != 1 || modrm&7 == ESP {
			return fail(fmt.Sprintf("09 ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		base := modrm & 7
		addr := uint32(int64(c.R[base]) + int64(int8(delta)))
		segment := SegDS
		if base == EBP {
			segment = SegSS
		}
		value, ok := c.readSegment32(c.Seg[segment], addr)
		if !ok {
			return fail(fmt.Sprintf("OR dword read %04X:%08X 未處理", c.Seg[segment], addr))
		}
		result := value | c.R[(modrm>>3)&7]
		if !c.writeSegment32(c.Seg[segment], addr, result) {
			return fail(fmt.Sprintf("OR dword write %04X:%08X 未處理", c.Seg[segment], addr))
		}
		c.setLogicFlags(result)
	case op == 0x22 || op == 0x02:
		// 02 = ADD r8, r/m8；22 = AND r8, r/m8。兩個都是「目的是暫存器、來源可以是
		// 記憶體」，記憶體形式由 decodeAddress32 解位址；旗標沿用同一組 add8／
		// setLogicFlags8，暫存器形式與記憶體形式只差在來源怎麼取。
		if operand16 || repe || repne {
			return fail("byte暫存器運算prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		dst := int((modrm >> 3) & 7)
		var b uint8
		if modrm>>6 == 3 {
			if segmentOverride >= 0 {
				return fail("byte暫存器運算prefix尚未支援")
			}
			b = c.reg8(int(modrm & 7))
		} else {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if segmentOverride >= 0 {
				seg = segmentOverride
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail(fmt.Sprintf("byte運算讀取 %04X:%08X 未處理", c.Seg[seg], addr))
			}
			b = value
		}
		a := c.reg8(dst)
		if op == 0x02 {
			c.setReg8(dst, c.add8(a, b))
		} else {
			v := a & b
			c.setReg8(dst, v)
			c.setLogicFlags8(v)
		}
	case op == 0x0a:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("0A 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 == 3 {
			dst, src := int((modrm>>3)&7), int(modrm&7)
			v := c.reg8(dst) | c.reg8(src)
			c.setReg8(dst, v)
			c.setLogicFlags8(v)
			break
		}
		if modrm>>6 != 1 || modrm&7 == ESP {
			return fail(fmt.Sprintf("0A ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		base := modrm & 7
		addr := uint32(int64(c.R[base]) + int64(int8(delta)))
		segment := SegDS
		if base == EBP {
			segment = SegSS
		}
		value, ok := c.readSegment8(c.Seg[segment], addr)
		if !ok {
			return fail(fmt.Sprintf("OR byte read %04X:%08X 未處理", c.Seg[segment], addr))
		}
		destination := int((modrm >> 3) & 7)
		result := c.reg8(destination) | value
		c.setReg8(destination, result)
		c.setLogicFlags8(result)
	case op == 0x85:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("85 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("TEST dword ModRM %02X 尚未支援", modrm))
		}
		left, right := modrm&7, (modrm>>3)&7
		c.setLogicFlags(c.R[left] & c.R[right])
	case op == 0x84:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("84 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("TEST byte ModRM %02X 尚未支援", modrm))
		}
		left, right := int(modrm&7), int((modrm>>3)&7)
		c.setLogicFlags8(c.reg8(left) & c.reg8(right))
	case op == 0x31:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("31 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("XOR dword ModRM %02X 尚未支援", modrm))
		}
		dst, src := modrm&7, (modrm>>3)&7
		c.R[dst] ^= c.R[src]
		c.setLogicFlags(c.R[dst])
	case op == 0x00 || op == 0x08 || op == 0x20 || op == 0x28 || op == 0x30:
		// 目的是 r/m8 的那一半 byte 運算：00 ADD、08 OR、20 AND、28 SUB、30 XOR。
		// 對應的 r8, r/m8 方向（02／0A／22／2A／32）本來就在，兩邊差別只有結果寫
		// 回哪裡。算術用 add8／sub8（它們自己設進位與溢位），邏輯運算用
		// setLogicFlags8。
		if operand16 || repe {
			return fail(fmt.Sprintf("%02X 不接受目前的 prefix", op))
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		src := c.reg8(int((modrm >> 3) & 7))
		apply := func(left, right uint8) uint8 {
			switch op {
			case 0x00:
				return c.add8(left, right)
			case 0x28:
				return c.sub8(left, right)
			}
			var value uint8
			switch op {
			case 0x08:
				value = left | right
			case 0x20:
				value = left & right
			default:
				value = left ^ right
			}
			c.setLogicFlags8(value)
			return value
		}
		if modrm>>6 == 3 {
			if segmentOverride >= 0 {
				return fail(fmt.Sprintf("%02X 不接受 segment override", op))
			}
			dst := int(modrm & 7)
			c.setReg8(dst, apply(c.reg8(dst), src))
			break
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		if segmentOverride >= 0 {
			seg = segmentOverride
		}
		value, ok := c.readSegment8(c.Seg[seg], addr)
		if !ok {
			return fail(fmt.Sprintf("byte運算讀取 %04X:%08X 未處理", c.Seg[seg], addr))
		}
		if !c.writeSegment8(c.Seg[seg], addr, apply(value, src)) {
			return fail(fmt.Sprintf("byte運算寫入 %04X:%08X 未處理", c.Seg[seg], addr))
		}
	case op == 0x06:
		if operand16 {
			return fail("16-bit PUSH ES 尚未支援")
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, uint32(c.Seg[SegES])) {
			return fail(fmt.Sprintf("PUSH ES stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0x1e:
		if operand16 {
			return fail("16-bit PUSH DS 尚未支援")
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, uint32(c.Seg[SegDS])) {
			return fail(fmt.Sprintf("PUSH DS stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0x07:
		if operand16 {
			return fail("16-bit POP ES 尚未支援")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		selector := uint16(value)
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		if !c.canLoadSegment(selector, SegES) {
			return fail(fmt.Sprintf("ES selector %04X 未登錄", selector))
		}
		c.Seg[SegES] = selector
		c.R[ESP] += 4
	case op == 0x1f:
		if operand16 {
			return fail("16-bit POP DS 尚未支援")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		selector := uint16(value)
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		if !c.canLoadSegment(selector, SegDS) {
			return fail(fmt.Sprintf("DS selector %04X 未登錄", selector))
		}
		c.Seg[SegDS] = selector
		c.R[ESP] += 4
	case op == 0xdb:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("DB x87 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		// DB /0 是 FILD m32int：讀一個 32 位元**有號**整數推上 x87 堆疊。記憶體形式
		// 的 reg 欄位是指令延伸碼不是暫存器，所以只認 /0；其他延伸碼（/1 FISTTP、
		// /2 FIST、/3 FISTP、/5 FLD m80、/7 FSTP m80）行為不同，猜錯會安靜地算出
		// 錯的數字。
		if modrm>>6 != 3 {
			if (modrm>>3)&7 != 0 {
				return fail(fmt.Sprintf("x87 DB /%d 記憶體形式尚未支援", (modrm>>3)&7))
			}
			if c.FPUDepth >= 8 {
				return fail("FILD x87 stack overflow")
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail(fmt.Sprintf("FILD dword read %04X:%08X 未處理", c.Seg[seg], addr))
			}
			for i := int(c.FPUDepth); i > 0; i-- {
				c.FPUStack[i] = c.FPUStack[i-1]
			}
			c.FPUStack[0] = float64(int32(value))
			c.FPUDepth++
			break
		}
		if modrm != 0xe3 {
			return fail(fmt.Sprintf("x87 DB ModRM %02X 尚未支援", modrm))
		}
		c.FPUControl = 0x037f
		c.FPUStatus = 0
		c.FPUStack = [8]float64{}
		c.FPUDepth = 0
	case op == 0xde:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("DE x87 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		switch modrm {
		case 0xf9:
			if c.FPUDepth < 2 {
				return fail("FDIVP x87 stack underflow")
			}
			dividend, divisor := c.FPUStack[1], c.FPUStack[0]
			if divisor == 0 {
				c.FPUStatus |= 1 << 2
				c.FPUStack[1] = math.Copysign(math.Inf(1), dividend*divisor)
			} else {
				c.FPUStack[1] = dividend / divisor
			}
			copy(c.FPUStack[0:], c.FPUStack[1:c.FPUDepth])
			c.FPUDepth--
		case 0xd9:
			if c.FPUDepth < 2 {
				return fail("FCOMPP x87 stack underflow")
			}
			left, right := c.FPUStack[0], c.FPUStack[1]
			c.FPUStatus &^= 0x4500
			switch {
			case math.IsNaN(left) || math.IsNaN(right):
				c.FPUStatus |= 0x4500
			case left < right:
				c.FPUStatus |= 0x0100
			case left == right:
				c.FPUStatus |= 0x4000
			}
			copy(c.FPUStack[0:], c.FPUStack[2:c.FPUDepth])
			c.FPUDepth -= 2
		default:
			return fail(fmt.Sprintf("x87 DE ModRM %02X 尚未支援", modrm))
		}
	case op == 0xdf:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("DF x87 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm != 0xe0 {
			return fail(fmt.Sprintf("x87 DF ModRM %02X 尚未支援", modrm))
		}
		c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(c.FPUStatus)
	case op == 0xd9:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("D9 x87 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		switch modrm {
		case 0xe8:
			if c.FPUDepth >= 8 {
				return fail("x87 stack overflow")
			}
			for i := int(c.FPUDepth); i > 0; i-- {
				c.FPUStack[i] = c.FPUStack[i-1]
			}
			c.FPUStack[0] = 1
			c.FPUDepth++
		case 0x3c:
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 || !c.writeSegment16(c.Seg[SegSS], c.R[ESP], c.FPUControl) {
				return fail(fmt.Sprintf("FNSTCW stack write %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
			}
		case 0x2d:
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("FLDCW read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.FPUControl = value
		case 0x2c:
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("FLDCW SIB %02X 尚未支援", sib))
			}
			value, ok := c.readSegment16(c.Seg[SegSS], c.R[ESP])
			if !ok {
				return fail(fmt.Sprintf("FLDCW stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
			}
			c.FPUControl = value
		case 0xee:
			if c.FPUDepth >= 8 {
				return fail("x87 stack overflow")
			}
			for i := int(c.FPUDepth); i > 0; i-- {
				c.FPUStack[i] = c.FPUStack[i-1]
			}
			c.FPUStack[0] = 0
			c.FPUDepth++
		case 0xc0:
			if c.FPUDepth == 0 || c.FPUDepth >= 8 {
				return fail("FLD ST(0) x87 stack unavailable")
			}
			value := c.FPUStack[0]
			for i := int(c.FPUDepth); i > 0; i-- {
				c.FPUStack[i] = c.FPUStack[i-1]
			}
			c.FPUStack[0] = value
			c.FPUDepth++
		case 0xe0:
			if c.FPUDepth == 0 {
				return fail("FCHS x87 stack underflow")
			}
			c.FPUStack[0] = -c.FPUStack[0]
		default:
			return fail(fmt.Sprintf("x87 D9 ModRM %02X 尚未支援", modrm))
		}
	case op == 0x9b:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("WAIT 不接受目前的 prefix")
		}
	case op == 0x9e:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("SAHF 不接受目前的 prefix")
		}
		ah := uint8(c.R[EAX] >> 8)
		c.EFlags &^= SF | ZF | AF | PF | CF
		c.EFlags |= uint32(ah) & (SF | ZF | AF | PF | CF)
	case op >= 0x40 && op <= 0x47:
		if operand16 {
			if segmentOverride >= 0 || repe || repne {
				return fail("word INC prefix未支援")
			}
			reg := op - 0x40
			carry := c.EFlags & CF
			result := c.add16(uint16(c.R[reg]), 1)
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(result)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		reg := int(op - 0x40)
		carry := c.EFlags & CF
		c.R[reg] = c.add32(c.R[reg], 1)
		c.EFlags = c.EFlags&^CF | carry
	case op >= 0x48 && op <= 0x4f:
		reg := int(op - 0x48)
		carry := c.EFlags & CF
		if operand16 {
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(c.sub16(uint16(c.R[reg]), 1))
		} else {
			c.R[reg] = c.sub32(c.R[reg], 1)
		}
		c.EFlags = c.EFlags&^CF | carry
	case op == 0xfe:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("FE 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 && (modrm>>3)&7 <= 1 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("INC／DEC byte來源越界")
			}
			result := value + 1
			if (modrm>>3)&7 == 1 {
				result = value - 1
			}
			if !c.writeSegment8(c.Seg[seg], addr, result) {
				return fail("INC／DEC byte寫入失敗")
			}
			carry := c.EFlags & CF
			if (modrm>>3)&7 == 1 {
				c.sub8(value, 1)
			} else {
				c.add8(value, 1)
			}
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if modrm>>6 != 3 || (modrm>>3)&7 > 1 {
			return fail(fmt.Sprintf("FE ModRM %02X 尚未支援", modrm))
		}
		reg := int(modrm & 7)
		carry := c.EFlags & CF
		if (modrm>>3)&7 == 1 {
			c.setReg8(reg, c.sub8(c.reg8(reg), 1))
		} else {
			c.setReg8(reg, c.add8(c.reg8(reg), 1))
		}
		c.EFlags = c.EFlags&^CF | carry
	case op >= 0x58 && op <= 0x5f:
		if operand16 {
			value, ok := c.readSegment16(c.Seg[SegSS], c.R[ESP])
			if !ok || c.R[ESP] > ^uint32(0)-2 {
				return fail(fmt.Sprintf("16-bit stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
			}
			reg := op - 0x58
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(value)
			c.R[ESP] += 2
			break
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		c.R[op-0x58] = value
		c.R[ESP] += 4
	case op == 0xaa:
		if operand16 || segmentOverride >= 0 {
			return fail("STOSB 不接受目前的 prefix")
		}
		count := uint32(1)
		if repe || repne {
			count = c.R[ECX]
		}
		for count > 0 {
			if !c.writeSegment8(c.Seg[SegES], c.R[EDI], c.reg8(0)) {
				return fail(fmt.Sprintf("STOSB write %04X:%08X 未處理", c.Seg[SegES], c.R[EDI]))
			}
			if c.EFlags&DF != 0 {
				c.R[EDI]--
			} else {
				c.R[EDI]++
			}
			if repe {
				c.R[ECX]--
				count = c.R[ECX]
			} else {
				break
			}
		}
	case op == 0xa5:
		if segmentOverride >= 0 {
			return fail("MOVS段覆寫未支援")
		}
		width := uint32(4)
		if operand16 {
			width = 2
		}
		count := uint32(1)
		if repe || repne {
			count = c.R[ECX]
		}
		for count > 0 {
			if operand16 {
				value, ok := c.readSegment16(c.Seg[SegDS], c.R[ESI])
				if !ok {
					return fail("MOVSW來源越界")
				}
				if !c.writeSegment16(c.Seg[SegES], c.R[EDI], value) {
					return fail("MOVSW目的越界")
				}
			} else {
				value, ok := c.readSegment32(c.Seg[SegDS], c.R[ESI])
				if !ok {
					return fail("MOVSD來源越界")
				}
				if !c.writeSegment32(c.Seg[SegES], c.R[EDI], value) {
					return fail("MOVSD目的越界")
				}
			}
			if c.EFlags&DF != 0 {
				c.R[ESI] -= width
				c.R[EDI] -= width
			} else {
				c.R[ESI] += width
				c.R[EDI] += width
			}
			count--
			if repe || repne {
				c.R[ECX]--
			}
		}
	case op == 0xab:
		if segmentOverride >= 0 || repne {
			return fail("STOS prefix未支援")
		}
		width := uint32(4)
		if operand16 {
			width = 2
		}
		count := uint32(1)
		if repe {
			count = c.R[ECX]
		}
		for count > 0 {
			ok := false
			if operand16 {
				ok = c.writeSegment16(c.Seg[SegES], c.R[EDI], uint16(c.R[EAX]))
			} else {
				ok = c.writeSegment32(c.Seg[SegES], c.R[EDI], c.R[EAX])
			}
			if !ok {
				return fail("STOS目的越界")
			}
			if c.EFlags&DF != 0 {
				c.R[EDI] -= width
			} else {
				c.R[EDI] += width
			}
			count--
			if repe {
				c.R[ECX]--
			}
		}
	case op == 0xa4:
		if operand16 || segmentOverride >= 0 {
			return fail("MOVSB 不接受目前的 prefix")
		}
		count := uint32(1)
		if repe || repne {
			count = c.R[ECX]
		}
		for count > 0 {
			value, ok := c.readSegment8(c.Seg[SegDS], c.R[ESI])
			if !ok {
				return fail("MOVSB source 讀取失敗")
			}
			if !c.writeSegment8(c.Seg[SegES], c.R[EDI], value) {
				return fail("MOVSB destination 寫入失敗")
			}
			delta := uint32(1)
			if c.EFlags&DF != 0 {
				delta = ^uint32(0)
			}
			c.R[ESI] += delta
			c.R[EDI] += delta
			count--
			if repe || repne {
				c.R[ECX]--
			}
		}
	case op == 0xac:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("LODSB 不接受目前的 prefix")
		}
		value, ok := c.readSegment8(c.Seg[SegDS], c.R[ESI])
		if !ok {
			return fail(fmt.Sprintf("LODSB read %04X:%08X 未處理", c.Seg[SegDS], c.R[ESI]))
		}
		c.setReg8(0, value)
		if c.EFlags&DF != 0 {
			c.R[ESI]--
		} else {
			c.R[ESI]++
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
			if repe || repne {
				c.R[ECX]--
				count = c.R[ECX]
				if repe && c.EFlags&ZF == 0 || repne && c.EFlags&ZF != 0 {
					break
				}
			} else {
				break
			}
		}
	case op >= 0x50 && op <= 0x57:
		if operand16 {
			if c.R[ESP] < 2 {
				return fail("ESP underflow")
			}
			nextESP := c.R[ESP] - 2
			if !c.writeSegment16(c.Seg[SegSS], nextESP, uint16(c.R[op-0x50])) {
				return fail(fmt.Sprintf("16-bit stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
			}
			c.R[ESP] = nextESP
			break
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, c.R[op-0x50]) {
			return fail(fmt.Sprintf("stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0x9c:
		if segmentOverride >= 0 || repe || repne {
			return fail("9C 不接受目前的 prefix")
		}
		if operand16 {
			if c.R[ESP] < 2 {
				return fail("ESP underflow")
			}
			next := c.R[ESP] - 2
			if !c.writeSegment16(c.Seg[SegSS], next, uint16(c.EFlags)) {
				return fail("PUSHF word 寫入失敗")
			}
			c.R[ESP] = next
			break
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, c.EFlags) {
			return fail(fmt.Sprintf("PUSHFD stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
	case op == 0x9d:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("9D 不接受目前的 prefix")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("POPFD stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		c.R[ESP] += 4
		c.EFlags = value
	case op == 0xe8:
		if operand16 {
			return fail("16-bit near CALL 尚未支援")
		}
		delta, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, c.EIP) {
			return fail(fmt.Sprintf("CALL stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
		c.EIP = uint32(int64(c.EIP) + int64(int32(delta)))
	case op == 0xff:
		if operand16 {
			if segmentOverride >= 0 || repe || repne {
				return fail("word FF prefix未支援")
			}
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if (modrm>>3)&7 != 1 || modrm>>6 == 3 {
				return fail("word FF僅支援記憶體DEC")
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("word DEC來源越界")
			}
			if !c.writeSegment16(c.Seg[seg], addr, value-1) {
				return fail("word DEC寫入失敗")
			}
			carry := c.EFlags & CF
			c.sub16(value, 1)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if segmentOverride == SegCS && (group != 4 || modrm>>6 == 3) {
			return fail("CS override 只支援間接記憶體 JMP")
		}
		if group == 4 {
			target := c.R[modrm&7]
			if modrm>>6 != 3 {
				seg, addr, err := c.decodeAddress32(modrm)
				if err != nil {
					return fail(err.Error())
				}
				if segmentOverride >= 0 {
					seg = segmentOverride
				}
				var ok bool
				target, ok = c.readSegment32(c.Seg[seg], addr)
				if !ok {
					return fail("間接 JMP 目標讀取失敗")
				}
			}
			c.EIP = target
			break
		}

		if (group == 0 || group == 1) && modrm>>6 != 3 && segmentOverride < 0 && !repe && !repne {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("INC/DEC來源讀取失敗")
			}
			result := value + 1
			if group == 1 {
				result = value - 1
			}
			if !c.writeSegment32(c.Seg[seg], addr, result) {
				return fail("INC/DEC寫入失敗")
			}
			carry := c.EFlags & CF
			if group == 0 {
				c.add32(value, 1)
			} else {
				c.sub32(value, 1)
			}
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if group == 2 && modrm>>6 != 3 && segmentOverride < 0 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			target, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("間接CALL來源越界")
			}
			if c.R[ESP] < 4 {
				return fail("ESP underflow")
			}
			next := c.R[ESP] - 4
			if !c.writeSegment32(c.Seg[SegSS], next, c.EIP) {
				return fail("間接CALL堆疊越界")
			}
			c.R[ESP] = next
			c.EIP = target
			break
		}

		if modrm>>6 == 0 && modrm&7 == EBP && group == 0 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("INC dword read %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			result := value + 1
			if !c.writeSegment32(c.Seg[SegDS], addr, result) {
				return fail(fmt.Sprintf("INC dword write %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			carry := c.EFlags & CF
			c.add32(value, 1)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if modrm>>6 == 0 && modrm&7 != ESP && modrm&7 != EBP && group == 0 {
			addr := c.R[modrm&7]
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("INC dword read %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			result := value + 1
			if !c.writeSegment32(c.Seg[SegDS], addr, result) {
				return fail(fmt.Sprintf("INC dword write %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			carry := c.EFlags & CF
			c.add32(value, 1)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if modrm>>6 == 0 && modrm&7 == EBP && group == 1 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("DEC dword read %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			result := value - 1
			if !c.writeSegment32(c.Seg[SegDS], addr, result) {
				return fail(fmt.Sprintf("DEC dword write %04X:%08X 尚未支援", c.Seg[SegDS], addr))
			}
			carry := c.EFlags & CF
			c.sub32(value, 1)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if modrm>>6 == 1 && modrm&7 != ESP && group == 1 {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int8(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment32(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("DEC dword read %04X:%08X 尚未支援", c.Seg[segment], addr))
			}
			result := value - 1
			if !c.writeSegment32(c.Seg[segment], addr, result) {
				return fail(fmt.Sprintf("DEC dword write %04X:%08X 尚未支援", c.Seg[segment], addr))
			}
			carry := c.EFlags & CF
			c.sub32(value, 1)
			c.EFlags = c.EFlags&^CF | carry
			break
		}
		if modrm>>6 != 3 && group == 6 && segmentOverride < 0 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("PUSH來源越界")
			}
			if c.R[ESP] < 4 {
				return fail("ESP underflow")
			}
			next := c.R[ESP] - 4
			if !c.writeSegment32(c.Seg[SegSS], next, value) {
				return fail("PUSH堆疊越界")
			}
			c.R[ESP] = next
			break
		}

		if modrm>>6 != 3 || group != 2 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		if c.R[ESP] < 4 {
			return fail("ESP underflow")
		}
		nextESP := c.R[ESP] - 4
		if !c.writeSegment32(c.Seg[SegSS], nextESP, c.EIP) {
			return fail(fmt.Sprintf("indirect CALL stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
		}
		c.R[ESP] = nextESP
		c.EIP = c.R[modrm&7]
	case op == 0xeb:
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
	case op == 0xe9:
		if operand16 {
			return fail("16-bit near JMP 尚未支援")
		}
		delta, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.EIP = uint32(int64(c.EIP) + int64(int32(delta)))
	case op == 0xee:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("EE prefix未支援")
		}
		port := uint16(c.R[EDX])
		if c.PortOut == nil || !c.PortOut(port, uint8(c.R[EAX])) {
			return fail(fmt.Sprintf("OUT port %04X 未處理", port))
		}
	case op == 0xe6:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("E6 不接受目前的 prefix")
		}
		port, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.PortOut == nil || !c.PortOut(uint16(port), uint8(c.R[EAX])) {
			return fail(fmt.Sprintf("OUT port %02X 未處理", port))
		}
	case op == 0x75:
		if operand16 {
			return fail("75 不接受 operand-size override")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&ZF == 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x73:
		if operand16 {
			return fail("73 不接受 operand-size override")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&CF == 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x72:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("72 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&CF != 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x77:
		if operand16 {
			return fail("77 不接受 operand-size override")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&(CF|ZF) == 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x7c:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("7C 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (c.EFlags&SF != 0) != (c.EFlags&OF != 0) {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x76:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("76 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&(CF|ZF) != 0 {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x7f:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("7F 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&ZF == 0 && (c.EFlags&SF != 0) == (c.EFlags&OF != 0) {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x7e:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("7E 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if c.EFlags&ZF != 0 || (c.EFlags&SF != 0) != (c.EFlags&OF != 0) {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0x7d:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("7D 不接受目前的 prefix")
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (c.EFlags&SF != 0) == (c.EFlags&OF != 0) {
			c.EIP = uint32(int64(c.EIP) + int64(int8(delta)))
		}
	case op == 0xfb:
		c.EFlags |= IF
	case op == 0xfa:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("FA 不接受目前的 prefix")
		}
		c.EFlags &^= IF
	case op == 0x81:
		if segmentOverride >= 0 || repe || repne {
			return fail("81 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if operand16 {
			if modrm>>6 != 3 || (group != 0 && group != 1 && group != 4) {
				return fail("81 word形狀尚未支援")
			}
			imm, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			reg := modrm & 7
			if group == 0 {
				v := c.add16(uint16(c.R[reg]), imm)
				c.R[reg] = c.R[reg]&0xffff0000 | uint32(v)
				break
			}
			v := uint16(c.R[reg]) | imm
			if group == 4 {
				v = uint16(c.R[reg]) & imm
			}
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(v)
			c.setLogicFlags16(v)
			break
		}

		if modrm>>6 != 3 && (group == 5 || group == 0) {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("SUB dword來源越界")
			}
			flags := c.EFlags
			result := c.sub32(value, imm)
			if group == 0 {
				result = c.add32(value, imm)
			}
			if !c.writeSegment32(c.Seg[seg], addr, result) {
				c.EFlags = flags
				return fail("SUB dword寫入越界")
			}
			break
		}
		if modrm>>6 != 3 && group == 7 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			imm, err := c.fetch32()
			if err != nil {
				return fail(err.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("CMP dword 來源越界")
			}
			c.sub32(value, imm)
			break
		}

		if modrm>>6 == 3 && group == 0 {
			value, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			reg := modrm & 7
			c.R[reg] = c.add32(c.R[reg], value)
			break
		}
		if modrm>>6 == 3 && group == 5 {
			value, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			reg := modrm & 7
			c.R[reg] = c.sub32(c.R[reg], value)
			break
		}
		if modrm>>6 == 3 && group == 7 {
			value, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			c.sub32(c.R[modrm&7], value)
			break
		}
		if modrm>>6 != 3 || group != 4 {
			return fail(fmt.Sprintf("81 ModRM %02X 尚未支援", modrm))
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		reg := modrm & 7
		c.R[reg] &= value
		c.setLogicFlags(c.R[reg])
	case op == 0x69 || op == 0x6b:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("IMUL立即值prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		source := c.R[modrm&7]
		if modrm>>6 != 3 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			var ok bool
			source, ok = c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("IMUL立即值來源越界")
			}
		}
		var imm uint32
		if op == 0x6b {
			v, err := c.fetch8()
			if err != nil {
				return fail(err.Error())
			}
			imm = uint32(int32(int8(v)))
		} else {
			imm, e = c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
		}
		product := int64(int32(source)) * int64(int32(imm))
		result := uint32(product)
		c.EFlags &^= CF | OF
		if product != int64(int32(result)) {
			c.EFlags |= CF | OF
		}
		c.R[(modrm>>3)&7] = result
	case op == 0x83:
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7

		if modrm>>6 != 3 && group == 5 && !operand16 && segmentOverride < 0 && !repe && !repne {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("SUB dword來源越界")
			}
			flags := c.EFlags
			result := c.sub32(value, uint32(int32(int8(imm))))
			if !c.writeSegment32(c.Seg[seg], addr, result) {
				c.EFlags = flags
				return fail("SUB dword寫入越界")
			}
			break
		}
		if group == 0 && modrm>>6 != 3 && !operand16 && segmentOverride < 0 && !repe && !repne {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("ADD dword來源越界")
			}
			flags := c.EFlags
			result := c.add32(value, uint32(int32(int8(imm))))
			if !c.writeSegment32(c.Seg[seg], addr, result) {
				c.EFlags = flags
				return fail("ADD dword寫入越界")
			}
			break
		}

		if operand16 && modrm>>6 == 3 && group == 5 && segmentOverride < 0 && !repe && !repne {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			reg := modrm & 7
			v := c.sub16(uint16(c.R[reg]), uint16(int16(int8(imm))))
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(v)
			break
		}

		if operand16 && modrm>>6 == 3 && group == 7 && segmentOverride < 0 && !repe && !repne {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			c.sub16(uint16(c.R[modrm&7]), uint16(int16(int8(imm))))
			break
		}

		if group == 7 && modrm>>6 != 3 && segmentOverride < 0 && !repe && !repne {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if operand16 {
				v, ok := c.readSegment16(c.Seg[seg], addr)
				if !ok {
					return fail("CMP word來源讀取失敗")
				}
				c.sub16(v, uint16(int16(int8(imm))))
			} else {
				v, ok := c.readSegment32(c.Seg[seg], addr)
				if !ok {
					return fail("CMP dword來源讀取失敗")
				}
				c.sub32(v, uint32(int32(int8(imm))))
			}
			break
		}
		if operand16 && modrm>>6 == 3 && group == 1 && segmentOverride < 0 && !repe && !repne {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			reg := modrm & 7
			v := uint16(c.R[reg]) | uint16(int16(int8(imm)))
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(v)
			c.setLogicFlags16(v)
			break
		}

		if operand16 {
			if segmentOverride >= 0 || repe || repne || group != 7 || modrm>>6 != 1 || modrm&7 == ESP {
				return fail("16-bit 83 形狀尚未支援")
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			seg := SegDS
			if base == EBP {
				seg = SegSS
			}
			value, ok := c.readSegment16(c.Seg[seg], c.R[base]+uint32(int32(int8(delta))))
			if !ok {
				return fail("CMP word 讀取失敗")
			}
			c.sub16(value, uint16(int16(int8(imm))))
			break
		}
		if modrm>>6 == 0 && modrm&7 == 5 && group == 7 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP dword read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.sub32(value, uint32(int32(int8(imm))))
			break
		}
		if modrm>>6 == 1 && modrm&7 != ESP && group == 7 {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP dword read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.sub32(value, uint32(int32(int8(imm))))
			break
		}
		if modrm>>6 == 1 && modrm&7 == ESP && group == 7 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("CMP stack SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[ESP]) + int64(int8(delta)))
			value, ok := c.readSegment32(c.Seg[SegSS], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP stack dword read %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
			c.sub32(value, uint32(int32(int8(imm))))
			break
		}
		if modrm>>6 == 2 && modrm&7 != ESP && group == 7 && segmentOverride < 0 {
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int32(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment32(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP base+disp32 dword read %04X:%08X 未處理", c.Seg[segment], addr))
			}
			c.sub32(value, uint32(int32(int8(imm))))
			break
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		imm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		rm := int(modrm & 7)
		switch group {
		case 0:
			c.R[rm] = c.add32(c.R[rm], uint32(int32(int8(imm))))
		case 4:
			c.R[rm] &= uint32(int32(int8(imm)))
			c.setLogicFlags(c.R[rm])
		case 5:
			c.R[rm] = c.sub32(c.R[rm], uint32(int32(int8(imm))))
		case 7:
			c.sub32(c.R[rm], uint32(int32(int8(imm))))
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
		group := (modrm >> 3) & 7
		if (group == 1 || group == 4 || group == 6) && modrm>>6 != 3 && segmentOverride < 0 && !repe && !repne {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("OR byte來源越界")
			}
			result := value | imm
			if group == 4 {
				result = value & imm
			}
			if group == 6 {
				result = value ^ imm
			}
			if !c.writeSegment8(c.Seg[seg], addr, result) {
				return fail("OR byte寫入失敗")
			}
			c.setLogicFlags8(result)
			break
		}
		if group == 7 && modrm>>6 != 3 && (segmentOverride < 0 || segmentOverride == SegES) {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			imm, err := c.fetch8()
			if err != nil {
				return fail(err.Error())
			}
			if segmentOverride == SegES {
				seg = SegES
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("CMP byte 來源越界")
			}
			c.sub8(value, imm)
			break
		}

		if segmentOverride == SegES && group == 7 && modrm == 0x38 {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[SegES], c.R[EAX])
			if !ok {
				return fail(fmt.Sprintf("CMP byte read %04X:%08X 未處理", c.Seg[SegES], c.R[EAX]))
			}
			c.sub8(value, imm)
		} else if segmentOverride < 0 && modrm == 0x4c {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x03 {
				return fail(fmt.Sprintf("80 OR SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[EBX]) + int64(c.R[EAX]) + int64(int8(delta)))
			value, ok := c.readSegment8(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("OR byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			result := value | imm
			if !c.writeSegment8(c.Seg[SegDS], addr, result) {
				return fail(fmt.Sprintf("OR byte write %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.setLogicFlags8(result)
		} else if modrm>>6 == 3 && (group == 0 || group == 1 || group == 4 || group == 5 || group == 7) {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			rm := int(modrm & 7)
			if group == 0 {
				c.setReg8(rm, c.add8(c.reg8(rm), imm))
			} else if group == 1 {
				result := c.reg8(rm) | imm
				c.setReg8(rm, result)
				c.setLogicFlags8(result)
			} else if group == 4 {
				result := c.reg8(rm) & imm
				c.setReg8(rm, result)
				c.setLogicFlags8(result)
			} else if group == 5 {
				c.setReg8(rm, c.sub8(c.reg8(rm), imm))
			} else {
				c.sub8(c.reg8(rm), imm)
			}
		} else if segmentOverride < 0 && !repe && !repne && modrm>>6 == 0 && modrm&7 != ESP && modrm&7 != EBP && group == 7 {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			value, ok := c.readSegment8(c.Seg[SegDS], c.R[base])
			if !ok {
				return fail(fmt.Sprintf("CMP byte read %04X:%08X 未處理", c.Seg[SegDS], c.R[base]))
			}
			c.sub8(value, imm)
		} else if segmentOverride < 0 && modrm>>6 == 1 && modrm&7 != ESP && (group == 1 || group == 4) {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int8(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment8(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("logical byte read %04X:%08X 未處理", c.Seg[segment], addr))
			}
			result := value & imm
			operation := "AND"
			if group == 1 {
				result = value | imm
				operation = "OR"
			}
			if !c.writeSegment8(c.Seg[segment], addr, result) {
				return fail(fmt.Sprintf("%s byte write %04X:%08X 未處理", operation, c.Seg[segment], addr))
			}
			c.setLogicFlags8(result)
		} else if group == 7 && modrm == 0x3d {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.sub8(value, imm)
		} else if modrm>>6 == 0 && modrm&7 == 5 && (group == 1 || group == 4) {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("logical byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			result := value & imm
			if group == 1 {
				result = value | imm
			}
			if !c.writeSegment8(c.Seg[SegDS], addr, result) {
				return fail(fmt.Sprintf("logical byte write %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.setLogicFlags8(result)
		} else if group == 7 && (modrm == 0x3e || modrm == 0x7e) {
			addr := c.R[ESI]
			if modrm == 0x7e {
				delta, e := c.fetch8()
				if e != nil {
					return fail(e.Error())
				}
				addr = uint32(int64(addr) + int64(int8(delta)))
			}
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("CMP byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.sub8(value, imm)
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op == 0xd0 || op == 0xc0:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("byte shift prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if modrm>>6 != 3 || (group != 4 && group != 5) {
			return fail("byte shift形狀未支援")
		}
		count := byte(1)
		if op == 0xc0 {
			count, e = c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			count &= 31
		}
		if count == 0 {
			break
		}
		rm := int(modrm & 7)
		value := c.reg8(rm)
		result := value
		carry := byte(0)
		oldOF := c.EFlags & OF
		for n := byte(0); n < count; n++ {
			if group == 4 {
				carry = result >> 7
				result <<= 1
			} else {
				carry = result & 1
				result >>= 1
			}
		}
		c.setLogicFlags8(result)
		c.EFlags |= uint32(carry)
		if count == 1 {
			overflow := value >> 7
			if group == 4 {
				overflow = result>>7 ^ carry
			}
			if overflow != 0 {
				c.EFlags |= OF
			}
		} else {
			c.EFlags |= oldOF
		}
		c.setReg8(rm, result)
	case op == 0xd3:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("D3 prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if modrm>>6 != 3 || (group != 4 && group != 5 && group != 7) {
			return fail("D3形狀尚未支援")
		}
		count := uint(c.R[ECX] & 31)
		if count == 0 {
			break
		}
		rm := modrm & 7
		value := c.R[rm]
		result := value >> count
		carry := value >> (count - 1) & 1
		if group == 4 {
			result = value << count
			carry = value >> (32 - count) & 1
		}
		if group == 7 {
			result = uint32(int32(value) >> count)
		}
		oldOF := c.EFlags & OF
		c.setLogicFlags(result)
		c.EFlags |= carry
		if count == 1 {
			if group == 4 && (result>>31^carry) != 0 || group == 5 && value>>31 != 0 {
				c.EFlags |= OF
			}
		} else {
			c.EFlags |= oldOF
		}
		c.R[rm] = result
	case op == 0xd1 && !operand16:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("D1 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if modrm>>6 == 3 && group == 5 {
			rm := modrm & 7
			value := c.R[rm]
			result := value >> 1
			c.setLogicFlags(result)
			c.EFlags |= value & 1
			if value&0x80000000 != 0 {
				c.EFlags |= OF
			}
			c.R[rm] = result
			break
		}
		if modrm>>6 == 3 && group == 7 {
			rm := modrm & 7
			value := c.R[rm]
			result := uint32(int32(value) >> 1)
			c.setLogicFlags(result)
			c.EFlags |= value & 1
			c.R[rm] = result
			break
		}

		if modrm>>6 != 3 || (group != 1 && group != 2) {
			return fail(fmt.Sprintf("D1 ModRM %02X 尚未支援", modrm))
		}
		rm := int(modrm & 7)
		value := c.R[rm]
		var result, carry, overflow uint32
		if group == 2 {
			carryIn := uint32(0)
			if c.EFlags&CF != 0 {
				carryIn = 1
			}
			result = value<<1 | carryIn
			carry = value >> 31
			overflow = result>>31 ^ carry
		} else {
			result = value>>1 | value<<31
			carry = value & 1
			overflow = result>>31 ^ (result >> 30 & 1)
		}
		c.EFlags &^= CF | OF
		if carry != 0 {
			c.EFlags |= CF
		}
		if overflow&1 != 0 {
			c.EFlags |= OF
		}
		c.R[rm] = result
	case op == 0xc1 || op == 0xd1:
		if operand16 {
			if segmentOverride >= 0 || repe || repne {
				return fail("word shift prefix未支援")
			}
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			group := (modrm >> 3) & 7
			if modrm>>6 != 3 || (group != 0 && group != 4 && group != 5 && group != 7) {
				return fail("word shift形狀未支援")
			}
			count := byte(1)
			if op == 0xc1 {
				count, e = c.fetch8()
				if e != nil {
					return fail(e.Error())
				}
			}
			count &= 31
			if group == 0 {
				if count >= 16 {
					return fail("word ROL count子集未支援")
				}
				if count == 0 {
					break
				}
				rm := modrm & 7
				value := uint16(c.R[rm])
				result := value<<count | value>>(16-count)
				c.EFlags &^= CF
				c.EFlags |= uint32(result & 1)
				if count == 1 {
					c.EFlags &^= OF
					if result>>15^(result&1) != 0 {
						c.EFlags |= OF
					}
				}
				c.R[rm] = c.R[rm]&0xffff0000 | uint32(result)
				break
			}
			if count == 0 {
				break
			}
			rm := modrm & 7
			value := uint16(c.R[rm])
			result := value
			carry := uint16(0)
			oldOF := c.EFlags & OF
			for n := byte(0); n < count; n++ {
				if group == 4 {
					carry = result >> 15
					result <<= 1
				} else {
					carry = result & 1
					if group == 7 {
						result = uint16(int16(result) >> 1)
					} else {
						result >>= 1
					}
				}
			}
			c.setLogicFlags16(result)
			c.EFlags |= uint32(carry)
			if count == 1 {
				overflow := uint16(0)
				if group == 4 {
					overflow = result>>15 ^ carry
				} else if group == 5 {
					overflow = value >> 15
				}
				if overflow != 0 {
					c.EFlags |= OF
				}
			} else {
				c.EFlags |= oldOF
			}
			c.R[rm] = c.R[rm]&0xffff0000 | uint32(result)
			break
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		group := (modrm >> 3) & 7
		if modrm>>6 != 3 || (group != 4 && group != 5 && group != 7) {
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
			if group == 7 {
				result = uint32(int32(value) >> count)
			}
			carry := value >> (count - 1) & 1
			if group == 4 {
				result = value << count
				carry = value >> (32 - count) & 1
			}
			c.setLogicFlags(result)
			if carry != 0 {
				c.EFlags |= CF
			}
			c.R[rm] = result
		}
	case op == 0xf6:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("F6 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 == 3 && (modrm>>3)&7 == 4 {
			result := uint16(c.reg8(0)) * uint16(c.reg8(int(modrm&7)))
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(result)
			c.EFlags &^= CF | OF
			if result > 255 {
				c.EFlags |= CF | OF
			}
			break
		}
		if modrm>>6 == 3 && (modrm>>3)&7 == 0 {
			imm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			c.setLogicFlags8(c.reg8(int(modrm&7)) & imm)
			break
		}
		if modrm>>6 == 3 || (modrm>>3)&7 != 0 {
			return fail("F6記憶體形狀未支援")
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		imm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		value, ok := c.readSegment8(c.Seg[seg], addr)
		if !ok {
			return fail("TEST byte來源越界")
		}
		c.setLogicFlags8(value & imm)
	case op == 0xc6:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("C6 prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (modrm>>3)&7 != 0 || modrm>>6 == 3 {
			return fail("C6僅支援/0 memory")
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if !c.writeSegment8(c.Seg[seg], addr, value) {
			return fail("C6 byte寫入失敗")
		}
	case op == 0xc7:
		if segmentOverride >= 0 || repe || repne {
			return fail("C7 prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (modrm>>3)&7 != 0 || modrm>>6 == 3 {
			return fail("C7僅支援/0 memory")
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		if operand16 {
			v, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment16(c.Seg[seg], addr, v) {
				return fail("C7 word寫入失敗")
			}
		} else {
			v, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment32(c.Seg[seg], addr, v) {
				return fail("C7 dword寫入失敗")
			}
		}
	case op == 0xc9:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("C9 不接受目前的 prefix")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[EBP])
		if !ok {
			return fail(fmt.Sprintf("LEAVE stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[EBP]))
		}
		c.R[ESP] = c.R[EBP] + 4
		c.R[EBP] = value
	case op == 0x8b:
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if operand16 && segmentOverride < 0 && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("MOV word 讀取失敗")
			}
			dst := (modrm >> 3) & 7
			c.R[dst] = c.R[dst]&0xffff0000 | uint32(value)
		} else if operand16 && segmentOverride == SegES && modrm>>6 == 0 && modrm&7 == 5 {
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
		} else if operand16 && segmentOverride < 0 && modrm>>6 == 3 {
			reg := (modrm >> 3) & 7
			c.R[reg] = c.R[reg]&0xffff0000 | c.R[modrm&7]&0xffff
		} else if operand16 {
			return fail(fmt.Sprintf("16-bit ModRM %02X 尚未支援", modrm))
		} else if segmentOverride < 0 && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("MOV dword讀取失敗")
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && !repe && !repne && modrm>>6 == 0 && modrm&7 == 4 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm == 0x04 && sib == 0x24 {
				value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
				if !ok {
					return fail(fmt.Sprintf("stack dword read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
				}
				c.R[EAX] = value
				break
			}
			if sib == 0xb0 {
				addr := c.R[EAX] + c.R[ESI]<<2
				value, ok := c.readSegment32(c.Seg[SegDS], addr)
				if !ok {
					return fail(fmt.Sprintf("scaled indexed dword read %04X:%08X 未處理", c.Seg[SegDS], addr))
				}
				c.R[EAX] = value
				break
			}
			scale, index, base := sib>>6, (sib>>3)&7, sib&7
			if index == ESP || base != EBP {
				return fail(fmt.Sprintf("absolute indexed dword SIB %02X 尚未支援", sib))
			}
			displacement, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := displacement + c.R[index]<<scale
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("absolute indexed dword read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment32(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("absolute dword read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 0 && modrm&7 != ESP && modrm&7 != EBP {
			base := modrm & 7
			value, ok := c.readSegment32(c.Seg[SegDS], c.R[base])
			if !ok {
				return fail(fmt.Sprintf("segment dword read %04X:%08X 未處理", c.Seg[SegDS], c.R[base]))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 1 && modrm&7 != 4 {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
			segment := SegDS
			if modrm&7 == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment32(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("segment dword read %04X:%08X 未處理", c.Seg[segment], addr))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 1 && modrm&7 == 4 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("stack dword SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[ESP]) + int64(int8(delta)))
			value, ok := c.readSegment32(c.Seg[SegSS], addr)
			if !ok {
				return fail(fmt.Sprintf("stack dword read %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 2 && modrm&7 == ESP {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("stack disp32 dword SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[ESP]) + int64(int32(delta)))
			value, ok := c.readSegment32(c.Seg[SegSS], addr)
			if !ok {
				return fail(fmt.Sprintf("stack disp32 dword read %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
			c.R[(modrm>>3)&7] = value
		} else if segmentOverride < 0 && modrm>>6 == 2 && modrm&7 != ESP {
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int32(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment32(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("base+disp32 dword read %04X:%08X 未處理", c.Seg[segment], addr))
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
		if operand16 && segmentOverride == SegSS {
			// Spec 185: SS word store，僅無 SIB 的基底／disp8 形式。
			mode, base := modrm>>6, modrm&7
			if base == ESP || (mode == 0 && base == EBP) || mode > 1 {
				return fail(fmt.Sprintf("SS word store ModRM %02X 尚未支援", modrm))
			}
			addr := c.R[base]
			if mode == 1 {
				delta, e := c.fetch8()
				if e != nil {
					return fail(e.Error())
				}
				addr += uint32(int32(int8(delta)))
			}
			if !c.writeSegment16(c.Seg[SegSS], addr, uint16(source)) {
				return fail(fmt.Sprintf("SS word write %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
		} else if operand16 && segmentOverride < 0 && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment16(c.Seg[seg], addr, uint16(source)) {
				return fail("MOV word 寫入失敗")
			}
		} else if operand16 && segmentOverride < 0 && modrm>>6 == 3 {
			destination := modrm & 7
			c.R[destination] = c.R[destination]&0xffff0000 | source&0xffff
		} else if operand16 {
			return fail(fmt.Sprintf("16-bit ModRM %02X 尚未支援", modrm))
		} else if segmentOverride < 0 && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment32(c.Seg[seg], addr, source) {
				return fail("MOV dword 寫入失敗")
			}
		} else if modrm>>6 == 3 {
			c.R[modrm&7] = source
		} else if modrm>>6 == 0 && modrm&7 == ESP && segmentOverride < 0 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib == 0x24 && !repe && !repne {
				if !c.writeSegment32(c.Seg[SegSS], c.R[ESP], source) {
					return fail(fmt.Sprintf("stack base dword write %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
				}
				break
			}
			if modrm == 0x14 && sib == 0x98 {
				addr := c.R[EAX] + c.R[EBX]<<2
				if !c.writeSegment32(c.Seg[SegDS], addr, source) {
					return fail(fmt.Sprintf("scaled indexed dword write %04X:%08X 未處理", c.Seg[SegDS], addr))
				}
				break
			}
			scale, index, base := sib>>6, (sib>>3)&7, sib&7
			if index == ESP || base != EBP {
				return fail(fmt.Sprintf("absolute indexed dword SIB %02X 尚未支援", sib))
			}
			displacement, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := displacement + c.R[index]<<scale
			if !c.writeSegment32(c.Seg[SegDS], addr, source) {
				return fail(fmt.Sprintf("absolute indexed dword write %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
		} else if modrm>>6 == 1 && modrm&7 != ESP && segmentOverride < 0 {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int8(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			if !c.writeSegment32(c.Seg[segment], addr, source) {
				return fail(fmt.Sprintf("base dword write %04X:%08X 未處理", c.Seg[segment], addr))
			}
		} else if modrm>>6 == 1 && modrm&7 == ESP && segmentOverride < 0 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("stack dword SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr := uint32(int64(c.R[ESP]) + int64(int8(delta)))
			if !c.writeSegment32(c.Seg[SegSS], addr, source) {
				return fail(fmt.Sprintf("stack dword write %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
		} else if modrm>>6 == 2 && modrm&7 == ESP && segmentOverride < 0 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x24 {
				return fail(fmt.Sprintf("stack disp32 dword SIB %02X 尚未支援", sib))
			}
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := c.R[ESP] + uint32(int32(delta))
			if !c.writeSegment32(c.Seg[SegSS], addr, source) {
				return fail(fmt.Sprintf("stack disp32 dword write %04X:%08X 未處理", c.Seg[SegSS], addr))
			}
		} else if modrm>>6 == 2 && modrm&7 != ESP && segmentOverride < 0 {
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int32(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			if !c.writeSegment32(c.Seg[segment], addr, source) {
				return fail(fmt.Sprintf("base+disp32 dword write %04X:%08X 未處理", c.Seg[segment], addr))
			}
		} else if modrm>>6 == 0 && modrm&7 == 5 {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if e = c.write32(addr, source); e != nil {
				return fail(e.Error())
			}
		} else if modrm>>6 == 0 && modrm&7 != ESP {
			addr := c.R[modrm&7]
			if !c.writeSegment32(c.Seg[SegDS], addr, source) {
				return fail(fmt.Sprintf("indirect dword write %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
		} else {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
	case op == 0x8c:
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
		if segmentOverride < 0 && modrm>>6 == 1 && modrm&7 == EBP {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment16(c.Seg[seg], addr, value) {
				return fail("段暫存器區域word寫入越界")
			}
			break
		}
		if operand16 && segmentOverride < 0 && modrm>>6 == 0 && modrm&7 == EBP {
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment16(c.Seg[SegDS], addr, value) {
				return fail(fmt.Sprintf("segment word write %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
		} else if operand16 && segmentOverride < 0 && modrm>>6 == 3 {
			reg := modrm & 7
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(value)
		} else if operand16 {
			return fail(fmt.Sprintf("16-bit segment ModRM %02X 尚未支援", modrm))
		} else if modrm>>6 == 3 {
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
		if operand16 {
			return fail("8A 不接受 operand-size override")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if (segmentOverride < 0 || segmentOverride == SegCS) && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if segmentOverride == SegCS {
				seg = SegCS
			}
			v, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("MOV byte讀取失敗")
			}
			c.setReg8(int((modrm>>3)&7), v)
			break
		}
		if segmentOverride < 0 && modrm>>6 == 3 {
			c.setReg8(int((modrm>>3)&7), c.reg8(int(modrm&7)))
			break
		}
		if segmentOverride < 0 && !repe && !repne && modrm == 0x04 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if sib != 0x2f {
				return fail(fmt.Sprintf("MOV AL SIB %02X 尚未支援", sib))
			}
			addr := c.R[EDI] + c.R[EBP]
			value, ok := c.readSegment8(c.Seg[SegDS], addr)
			if !ok {
				return fail(fmt.Sprintf("MOV AL SIB byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			c.setReg8(0, value)
			break
		}
		if segmentOverride < 0 && !repe && !repne && modrm>>6 == 0 && modrm&7 != ESP && modrm&7 != EBP {
			base := modrm & 7
			value, ok := c.readSegment8(c.Seg[SegDS], c.R[base])
			if !ok {
				return fail(fmt.Sprintf("segment byte read %04X:%08X 未處理", c.Seg[SegDS], c.R[base]))
			}
			c.setReg8(int((modrm>>3)&7), value)
			break
		}
		if segmentOverride < 0 && !repe && !repne && modrm>>6 == 2 && modrm&7 == ESP {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			scale, index, base := sib>>6, (sib>>3)&7, sib&7
			if index == ESP {
				return fail(fmt.Sprintf("byte SIB %02X 無 index 尚未支援", sib))
			}
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := c.R[base] + c.R[index]<<scale + uint32(int32(delta))
			segment := SegDS
			if base == ESP || base == EBP {
				segment = SegSS
			}
			value, ok := c.readSegment8(c.Seg[segment], addr)
			if !ok {
				return fail(fmt.Sprintf("SIB byte read %04X:%08X 未處理", c.Seg[segment], addr))
			}
			c.setReg8(int((modrm>>3)&7), value)
			break
		}
		if modrm>>6 != 1 || modrm&7 == 4 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		addr := uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
		segment := SegDS
		if segmentOverride >= 0 {
			segment = segmentOverride
		}
		value, ok := c.readSegment8(c.Seg[segment], addr)
		if !ok {
			return fail(fmt.Sprintf("segment byte read %04X:%08X 未處理", c.Seg[segment], addr))
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
		mod, rm := modrm>>6, modrm&7
		if mod == 3 {
			return fail("LEA 不接受暫存器來源")
		}
		var addr uint32
		noBase := mod == 0 && rm == 5
		if rm == 4 {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			scale, index, base := sib>>6, (sib>>3)&7, sib&7
			noBase = mod == 0 && base == 5
			if !noBase {
				addr = c.R[base]
			}
			if index != 4 {
				addr += c.R[index] << scale
			}
		} else if !noBase {
			addr = c.R[rm]
		}
		if mod == 1 {
			d, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			addr += uint32(int32(int8(d)))
		} else if mod == 2 || noBase {
			d, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr += d
		}
		c.R[(modrm>>3)&7] = addr

	case op == 0x8e:
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
		if segmentOverride < 0 && modrm>>6 == 1 && modrm&7 == EBP {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			v, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("段暫存器區域word讀取越界")
			}
			if !c.canLoadSegment(v, destination) {
				return fail("區域word selector不可載入")
			}
			c.Seg[destination] = v
			break
		}

		if operand16 && (segmentOverride >= 0 || modrm>>6 != 3 && (modrm>>6 != 0 || modrm&7 != EBP)) {
			return fail(fmt.Sprintf("16-bit segment ModRM %02X 尚未支援", modrm))
		} else if modrm>>6 == 3 {
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
			} else if operand16 {
				var ok bool
				value, ok = c.readSegment16(c.Seg[SegDS], addr)
				if !ok {
					return fail(fmt.Sprintf("segment word read %04X:%08X 未處理", c.Seg[SegDS], addr))
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
		} else if segmentOverride < 0 && modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			if !c.writeSegment8(c.Seg[seg], addr, value) {
				return fail("MOV byte寫入失敗")
			}
		} else if segmentOverride < 0 && !repe && !repne && modrm>>6 == 0 && modrm&7 != ESP && modrm&7 != EBP {
			base := modrm & 7
			if !c.writeSegment8(c.Seg[SegDS], c.R[base], value) {
				return fail(fmt.Sprintf("MOV byte write %04X:%08X 未處理", c.Seg[SegDS], c.R[base]))
			}
		} else if segmentOverride < 0 && !repe && !repne && modrm>>6 == 2 && modrm&7 == ESP {
			sib, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			scale, index, base := sib>>6, (sib>>3)&7, sib&7
			if index == ESP {
				return fail(fmt.Sprintf("byte SIB %02X 無 index 尚未支援", sib))
			}
			delta, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			addr := c.R[base] + c.R[index]<<scale + uint32(int32(delta))
			segment := SegDS
			if base == ESP || base == EBP {
				segment = SegSS
			}
			if !c.writeSegment8(c.Seg[segment], addr, value) {
				return fail(fmt.Sprintf("SIB byte write %04X:%08X 未處理", c.Seg[segment], addr))
			}
		} else if segmentOverride < 0 && !repe && !repne && modrm>>6 == 1 && modrm&7 != ESP {
			delta, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			base := modrm & 7
			addr := uint32(int64(c.R[base]) + int64(int8(delta)))
			segment := SegDS
			if base == EBP {
				segment = SegSS
			}
			if !c.writeSegment8(c.Seg[segment], addr, value) {
				return fail(fmt.Sprintf("MOV byte write %04X:%08X 未處理", c.Seg[segment], addr))
			}
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
	case op == 0x90:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("NOP prefix尚未支援")
		}
	case op == 0x87:
		if segmentOverride >= 0 || repe || repne {
			return fail("XCHG prefix尚未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		seg, addr, e := c.decodeAddress32(modrm)
		if e != nil {
			return fail(e.Error())
		}
		reg := (modrm >> 3) & 7
		if operand16 {
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok || !c.writeSegment16(c.Seg[seg], addr, uint16(c.R[reg])) {
				return fail("XCHG word 存取失敗")
			}
			c.R[reg] = c.R[reg]&0xffff0000 | uint32(value)
		} else {
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok || !c.writeSegment32(c.Seg[seg], addr, c.R[reg]) {
				return fail("XCHG dword 存取失敗")
			}
			c.R[reg] = value
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
	case op == 0xad:
		if segmentOverride >= 0 || repe || repne {
			return fail("LODS prefix未支援")
		}
		width := uint32(4)
		if operand16 {
			value, ok := c.readSegment16(c.Seg[SegDS], c.R[ESI])
			if !ok {
				return fail("LODSW來源越界")
			}
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(value)
			width = 2
		} else {
			value, ok := c.readSegment32(c.Seg[SegDS], c.R[ESI])
			if !ok {
				return fail("LODSD來源越界")
			}
			c.R[EAX] = value
		}
		if c.EFlags&DF != 0 {
			c.R[ESI] -= width
		} else {
			c.R[ESI] += width
		}
	case op == 0xa1:
		if segmentOverride >= 0 || repe || repne {
			return fail("A1 不接受目前的 prefix")
		}
		addr, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		if operand16 {
			value, ok := c.readSegment16(c.Seg[SegDS], addr)
			if !ok {
				return fail("MOV AX來源越界")
			}
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(value)
			break
		}
		value, ok := c.readSegment32(c.Seg[SegDS], addr)
		if !ok {
			return fail(fmt.Sprintf("MOV EAX read %04X:%08X 未處理", c.Seg[SegDS], addr))
		}
		c.R[EAX] = value
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
			if segmentOverride >= 0 || repe || repne {
				return fail("word SUB prefix未支援")
			}
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			dst := (modrm >> 3) & 7
			value := uint16(c.R[modrm&7])
			if modrm>>6 != 3 {
				seg, addr, e := c.decodeAddress32(modrm)
				if e != nil {
					return fail(e.Error())
				}
				var ok bool
				value, ok = c.readSegment16(c.Seg[seg], addr)
				if !ok {
					return fail("word SUB來源越界")
				}
			}
			result := c.sub16(uint16(c.R[dst]), value)
			c.R[dst] = c.R[dst]&0xffff0000 | uint32(result)
			break
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		reg, rm := (modrm>>3)&7, modrm&7
		if modrm>>6 == 3 {
			c.R[reg] = c.sub32(c.R[reg], c.R[rm])
		} else {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("SUB來源越界")
			}
			c.R[reg] = c.sub32(c.R[reg], value)
		}
	case op == 0x2a:
		if operand16 || segmentOverride >= 0 {
			return fail("2A 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("SUB byte來源越界")
			}
			reg := int((modrm >> 3) & 7)
			c.setReg8(reg, c.sub8(c.reg8(reg), value))
			break
		}

		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		reg, rm := int((modrm>>3)&7), int(modrm&7)
		c.setReg8(reg, c.sub8(c.reg8(reg), c.reg8(rm)))
	case op == 0x3d:
		if operand16 {
			value, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			c.sub16(uint16(c.R[EAX]), value)
		} else {
			value, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			c.sub32(c.R[EAX], value)
		}

	case op == 0x3b:
		if operand16 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 3 {
				return fail("word CMP僅支援暫存器")
			}
			c.sub16(uint16(c.R[(modrm>>3)&7]), uint16(c.R[modrm&7]))
			break
		}
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("CMP prefix尚未支援")
		}
		modrm, err := c.fetch8()
		if err != nil {
			return fail(err.Error())
		}
		src := c.R[modrm&7]
		if modrm>>6 != 3 {
			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			var ok bool
			src, ok = c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("CMP來源越界")
			}
		}
		c.sub32(c.R[modrm>>3&7], src)

	case op == 0x38:
		if operand16 || segmentOverride >= 0 {
			return fail("38 不接受目前的 prefix")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 == 3 {
			c.sub8(c.reg8(int(modrm&7)), c.reg8(int((modrm>>3)&7)))
			break
		}
		if modrm>>6 != 1 || modrm&7 == 4 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		delta, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		addr := uint32(int64(c.R[modrm&7]) + int64(int8(delta)))
		value, ok := c.readSegment8(c.Seg[SegDS], addr)
		if !ok {
			return fail(fmt.Sprintf("CMP byte read %04X:%08X 未處理", c.Seg[SegDS], addr))
		}
		c.sub8(value, c.reg8(int((modrm>>3)&7)))
	case op == 0x0d:
		if operand16 {
			return fail("16-bit OR accumulator 尚未支援")
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.R[EAX] |= value
		c.setLogicFlags(c.R[EAX])
	case op == 0x0c:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("0C 不接受目前的 prefix")
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		result := uint8(c.R[EAX]) | value
		c.R[EAX] = c.R[EAX]&0xffffff00 | uint32(result)
		c.setLogicFlags8(result)
	case op == 0x25:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("25 不接受目前的 prefix")
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.R[EAX] &= value
		c.setLogicFlags(c.R[EAX])
	case op == 0x0b:
		if operand16 {
			if segmentOverride >= 0 || repe || repne {
				return fail("word OR prefix未支援")
			}
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 3 {
				return fail("word OR僅支援暫存器")
			}
			dst, src := (modrm>>3)&7, modrm&7
			result := uint16(c.R[dst]) | uint16(c.R[src])
			c.R[dst] = c.R[dst]&0xffff0000 | uint32(result)
			c.setLogicFlags16(result)
			break
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		if modrm>>6 != 3 {
			return fail(fmt.Sprintf("ModRM %02X 尚未支援", modrm))
		}
		reg, rm := (modrm>>3)&7, modrm&7
		c.R[reg] |= c.R[rm]
		c.setLogicFlags(c.R[reg])
	case op == 0x2d:
		if segmentOverride >= 0 || repe || repne {
			return fail("SUB accumulator prefix未支援")
		}
		if operand16 {
			value, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			result := c.sub16(uint16(c.R[EAX]), value)
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(result)
			break
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.R[EAX] = c.sub32(c.R[EAX], value)
	case op == 0x05:
		if segmentOverride >= 0 || repe || repne {
			return fail("ADD accumulator prefix未支援")
		}
		if operand16 {
			value, e := c.fetch16()
			if e != nil {
				return fail(e.Error())
			}
			result := c.add16(uint16(c.R[EAX]), value)
			c.R[EAX] = c.R[EAX]&0xffff0000 | uint32(result)
			break
		}
		value, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		c.R[EAX] = c.add32(c.R[EAX], value)
	case op == 0x24:
		if operand16 {
			return fail("24 不接受 operand-size override")
		}
		value, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		result := c.reg8(0) & value
		c.setReg8(0, result)
		c.setLogicFlags8(result)
	case op == 0x3a:
		if operand16 || segmentOverride >= 0 || repe || repne {
			return fail("CMP byte prefix未支援")
		}
		modrm, e := c.fetch8()
		if e != nil {
			return fail(e.Error())
		}
		var value uint8
		if modrm>>6 == 3 {
			value = c.reg8(int(modrm & 7))
		} else {
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			var ok bool
			value, ok = c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("CMP byte來源越界")
			}
		}
		c.sub8(c.reg8(int((modrm>>3)&7)), value)
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
		if segmentOverride == SegES {
			if extended != 0xb6 || operand16 || repe || repne {
				return fail("ES extended僅支援MOVZX byte")
			}
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 == 3 {
				return fail("ES MOVZX需要記憶體来源")
			}
			_, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[SegES], addr)
			if !ok {
				return fail("ES MOVZX來源越界")
			}
			c.R[(modrm>>3)&7] = uint32(value)
			break
		}

		if extended == 0xbe && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			var value byte
			if modrm>>6 == 3 {
				value = c.reg8(int(modrm & 7))
			} else {
				seg, addr, e := c.decodeAddress32(modrm)
				if e != nil {
					return fail(e.Error())
				}
				var ok bool
				value, ok = c.readSegment8(c.Seg[seg], addr)
				if !ok {
					return fail("MOVSX byte來源越界")
				}
			}
			dst := (modrm >> 3) & 7
			if operand16 {
				c.R[dst] = c.R[dst]&0xffff0000 | uint32(uint16(int16(int8(value))))
			} else {
				c.R[dst] = uint32(int32(int8(value)))
			}
			break
		}
		if extended == 0xbf && !operand16 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 == 3 {
				c.R[(modrm>>3)&7] = uint32(int32(int16(c.R[modrm&7])))
				break
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("MOVSX word 讀取失敗")
			}
			c.R[(modrm>>3)&7] = uint32(int32(int16(value)))
			break
		}

		if extended == 0xaf && !operand16 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 == 3 {
				dst := modrm >> 3 & 7
				product := int64(int32(c.R[dst])) * int64(int32(c.R[modrm&7]))
				result := uint32(product)
				c.EFlags &^= CF | OF
				if product != int64(int32(result)) {
					c.EFlags |= CF | OF
				}
				c.R[dst] = result
				break
			}

			seg, addr, err := c.decodeAddress32(modrm)
			if err != nil {
				return fail(err.Error())
			}
			value, ok := c.readSegment32(c.Seg[seg], addr)
			if !ok {
				return fail("IMUL來源越界")
			}
			dst := modrm >> 3 & 7

			product := int64(int32(c.R[dst])) * int64(int32(value))
			result := uint32(product)
			c.EFlags &^= CF | OF
			if product != int64(int32(result)) {
				c.EFlags |= CF | OF
			}
			c.R[dst] = result
			break
		}
		if (extended == 0xa0 || extended == 0xa1) && !operand16 && segmentOverride < 0 && !repe {
			if extended == 0xa0 {
				if c.R[ESP] < 4 {
					return fail("ESP underflow")
				}
				nextESP := c.R[ESP] - 4
				if !c.writeSegment32(c.Seg[SegSS], nextESP, uint32(c.Seg[SegFS])) {
					return fail(fmt.Sprintf("PUSH FS stack write %04X:%08X 未處理", c.Seg[SegSS], nextESP))
				}
				c.R[ESP] = nextESP
			} else {
				value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
				selector := uint16(value)
				if !ok || c.R[ESP] > ^uint32(0)-4 {
					return fail(fmt.Sprintf("POP FS stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
				}
				if !c.canLoadSegment(selector, SegFS) {
					return fail(fmt.Sprintf("FS selector %04X 未登錄", selector))
				}
				c.Seg[SegFS] = selector
				c.R[ESP] += 4
			}
			break
		}
		if extended == 0xb4 && !operand16 && segmentOverride < 0 && !repe {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 0 || modrm&7 != 5 {
				return fail(fmt.Sprintf("0F B4 ModRM %02X 尚未支援", modrm))
			}
			addr, e := c.fetch32()
			if e != nil {
				return fail(e.Error())
			}
			if addr > ^uint32(0)-4 {
				return fail(fmt.Sprintf("LFS pointer address %08X 溢位", addr))
			}
			offset, okOffset := c.readSegment32(c.Seg[SegDS], addr)
			selector, okSelector := c.readSegment16(c.Seg[SegDS], addr+4)
			if !okOffset || !okSelector {
				return fail(fmt.Sprintf("LFS pointer read %04X:%08X 未處理", c.Seg[SegDS], addr))
			}
			if !c.canLoadSegment(selector, SegFS) {
				return fail(fmt.Sprintf("LFS FS selector %04X 未登錄", selector))
			}
			c.R[(modrm>>3)&7] = offset
			c.Seg[SegFS] = selector
			break
		}
		if extended == 0xb7 && !operand16 && segmentOverride < 0 && !repe {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 == 3 {
				c.R[(modrm>>3)&7] = uint32(uint16(c.R[modrm&7]))
				break
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment16(c.Seg[seg], addr)
			if !ok {
				return fail("MOVZX word來源越界")
			}
			c.R[(modrm>>3)&7] = uint32(value)
			break
		}
		if extended == 0xb6 && segmentOverride < 0 && !repe && !repne {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 == 3 {
				value := c.reg8(int(modrm & 7))
				dst := (modrm >> 3) & 7
				if operand16 {
					c.R[dst] = c.R[dst]&0xffff0000 | uint32(value)
				} else {
					c.R[dst] = uint32(value)
				}
				break
			}
			seg, addr, e := c.decodeAddress32(modrm)
			if e != nil {
				return fail(e.Error())
			}
			value, ok := c.readSegment8(c.Seg[seg], addr)
			if !ok {
				return fail("MOVZX byte來源越界")
			}
			dst := (modrm >> 3) & 7
			if operand16 {
				c.R[dst] = c.R[dst]&0xffff0000 | uint32(value)
			} else {
				c.R[dst] = uint32(value)
			}
			break
		}
		if (extended == 0x94 || extended == 0x95) && !operand16 && segmentOverride < 0 && !repe {
			modrm, e := c.fetch8()
			if e != nil {
				return fail(e.Error())
			}
			if modrm>>6 != 3 {
				return fail(fmt.Sprintf("0F %02X ModRM %02X 尚未支援", extended, modrm))
			}
			value := uint8(0)
			if extended == 0x94 && c.EFlags&ZF != 0 || extended == 0x95 && c.EFlags&ZF == 0 {
				value = 1
			}
			c.setReg8(int(modrm&7), value)
			break
		}
		if extended < 0x80 || extended > 0x8f || operand16 || segmentOverride >= 0 || repe || repne {
			return fail(fmt.Sprintf("0F %02X 尚未支援", extended))
		}
		delta, e := c.fetch32()
		if e != nil {
			return fail(e.Error())
		}
		cf, zf, sf, of, pf := c.EFlags&CF != 0, c.EFlags&ZF != 0, c.EFlags&SF != 0, c.EFlags&OF != 0, c.EFlags&PF != 0
		conditions := [16]bool{of, !of, cf, !cf, zf, !zf, cf || zf, !cf && !zf, sf, !sf, pf, !pf, sf != of, sf == of, zf || sf != of, !zf && sf == of}
		if conditions[extended&15] {
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
	case op == 0xc3:
		if operand16 {
			return fail("16-bit near RET 尚未支援")
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		if !ok || c.R[ESP] > ^uint32(0)-4 {
			return fail(fmt.Sprintf("RET stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		c.R[ESP] += 4
		c.EIP = value
	case op == 0xc2:
		if operand16 || segmentOverride >= 0 || repe {
			return fail("C2 不接受目前的 prefix")
		}
		cleanup, e := c.fetch16()
		if e != nil {
			return fail(e.Error())
		}
		value, ok := c.readSegment32(c.Seg[SegSS], c.R[ESP])
		increase := uint32(cleanup) + 4
		if !ok || c.R[ESP] > ^uint32(0)-increase {
			return fail(fmt.Sprintf("RET immediate stack read %04X:%08X 未處理", c.Seg[SegSS], c.R[ESP]))
		}
		c.R[ESP] += increase
		c.EIP = value
	default:
		return fail("opcode 尚未支援")
	}
	return nil
}

func (c *CPU) setLogicFlags16(v uint16) {
	c.setLogicFlags(uint32(v))
	if v&0x8000 != 0 {
		c.EFlags |= SF
	}
}
