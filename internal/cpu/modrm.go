package cpu

// operand 是一個 ModRM 解出來的目的地／來源：不是暫存器就是記憶體。
//
// 拆成一個值而不是兩支函式，是為了讓「讀出來、算一算、寫回去」那一類指令
// （`ADD r/m, r`、`INC r/m`…）只解一次位址。**8086 的 read-modify-write
// 就是同一個位址讀一次寫一次**，解兩次在有段前綴時會漂。
//
// 記憶體運算元帶**段索引**與 uint32 偏移（434 §4.2）：段在存取當下才取值或快取。
// 實模式偏移恆 ≤ FFFFh；w32 只在分段模式的 67 定址（ModRM32、moffs32）為真，
// 表示多 byte 存取與 +2、+4 以 32 位元相加（434 §4.1）。
type operand struct {
	isReg bool
	w32   bool   // 偏移寬度 32 位元（只在分段模式的 67 定址）
	reg   int    // isReg 時的暫存器索引
	seg   int    // 記憶體時的段索引（已經套過段前綴）
	off   uint32 // 記憶體時的位移
}

// at 回「同一運算元往後 k bytes」：偏移依寬度相加（16 位元環繞或 32 位元），用於遠指標、
// 描述表暫存器等多欄位運算元（434 §4.2 的 off+2、off+4）。
func (o operand) at(k uint32) operand {
	if o.w32 {
		o.off += k
	} else {
		o.off = uint32(uint16(o.off + k))
	}
	return o
}

// modrm 是解出來的 ModRM 位元組：reg 欄位與 r/m 運算元。
type modrm struct {
	reg int
	rm  operand
	mod uint8
}

// defaultSeg 是每一種 r/m 定址預設走哪個段。
//
// **只有用到 BP 的那幾種預設是 SS**（[BP+SI]、[BP+DI]、[BP]、[BP+disp]），
// 其餘都是 DS。這一條寫錯不會報錯——它會讓堆疊上的區域變數讀到資料段的內容，
// 而那看起來像「資料解錯了」。
func (c *CPU) decodeModRM() modrm {
	if c.addr32 {
		return c.decodeModRM32()
	}
	b := c.fetch8()
	mod := b >> 6
	reg := int(b>>3) & 7
	rm := int(b) & 7

	if mod == 3 {
		return modrm{reg: reg, mod: mod, rm: operand{isReg: true, reg: rm}}
	}
	// 碰記憶體的那一種比較貴（cycles.go）。
	c.charge(cycMem)

	var off uint16
	def := DS
	switch rm {
	case 0:
		off = c.R[BX] + c.R[SI]
	case 1:
		off = c.R[BX] + c.R[DI]
	case 2:
		off = c.R[BP] + c.R[SI]
		def = SS
	case 3:
		off = c.R[BP] + c.R[DI]
		def = SS
	case 4:
		off = c.R[SI]
	case 5:
		off = c.R[DI]
	case 6:
		if mod == 0 {
			// mod=0 rm=6 是唯一的例外：不是 [BP]，是 16 位元絕對位址，
			// 而且**段回到 DS**。
			off = c.fetch16()
		} else {
			off = c.R[BP]
			def = SS
		}
	case 7:
		off = c.R[BX]
	}

	switch mod {
	case 1:
		// 8 位元位移是**有號**的。當成無號加會讓 [BP-2] 變成 [BP+254]。
		off += uint16(int16(int8(c.fetch8())))
	case 2:
		off += c.fetch16()
	}

	return modrm{reg: reg, mod: mod, rm: operand{seg: c.dataSeg(def), off: uint32(off)}}
}

// decodeModRM32 是 0x67 位址大小前綴下的 ModRM（含 SIB），`docs/spec/197` §3。
//
// 語意照 DOSBox-X 的 `src/cpu/core_normal/table_ea.h`：
//   - r/m＝4 帶一個 SIB 位元組；index＝4 表示沒有索引；
//   - base＝5 且 mod＝0 是沒有基底的 disp32（SIB 與非 SIB 的 r/m＝5 都一樣）；
//   - 預設段：基底是 ESP 或 EBP 用 SS，其餘 DS。**看的是基底暫存器，
//     不是索引暫存器**（`[eax+ebp*2]` 仍是 DS）。
//
// ⚠ 實模式的段限是 64 KB：有效位址超過 `0xFFFF` 在真機是 #GP。這裡
// **不截斷了事**，改記一個 fault 讓 `Step` 回報（LEA 例外，見 `eaNoLimit`）。
// DOSBox-X 預設不查段限（`do_seg_limits` 關），所以那邊會悄悄讀到 64 KB 之外。
//
// 分段模式（434 §4.2）：不截斷、不記 64 KB fault，偏移是完整 32 位元有效位址（w32），
// 界限由存取時以段快取檢查。
func (c *CPU) decodeModRM32() modrm {
	b := c.fetch8()
	mod := b >> 6
	reg := int(b>>3) & 7
	rm := int(b) & 7
	if mod == 3 {
		return modrm{reg: reg, mod: mod, rm: operand{isReg: true, reg: rm}}
	}
	c.charge(cycMem)

	def := DS
	var ea uint32
	switch {
	case rm == 4:
		sib := c.fetch8()
		scale := sib >> 6
		idx := int(sib>>3) & 7
		base := int(sib) & 7
		switch {
		case base == 5 && mod == 0:
			ea = c.fetch32()
		case base == 4:
			ea, def = c.reg32(SP), SS
		case base == 5:
			ea, def = c.reg32(BP), SS
		default:
			ea = c.reg32(base)
		}
		if idx != 4 {
			ea += c.reg32(idx) << scale
		}
	case rm == 5 && mod == 0:
		ea = c.fetch32()
	default:
		ea = c.reg32(rm)
		if rm == 5 {
			def = SS
		}
	}
	switch mod {
	case 1:
		ea += uint32(int32(int8(c.fetch8())))
	case 2:
		ea += c.fetch32()
	}

	c.ea32 = ea
	if c.segmented {
		return modrm{reg: reg, mod: mod, rm: operand{seg: c.dataSeg(def), off: ea, w32: true}}
	}
	if ea > 0xFFFF && !c.eaNoLimit {
		c.failf("32 位元有效位址 %08X 超出實模式 64 KB 段限（docs/spec/197 §3）", ea)
	}
	return modrm{reg: reg, mod: mod, rm: operand{seg: c.dataSeg(def), off: uint32(uint16(ea))}}
}

func (c *CPU) get8(o operand) uint8 {
	if o.isReg {
		return c.reg8(o.reg)
	}
	return c.read8(o.seg, o.off)
}

func (c *CPU) set8(o operand, v uint8) {
	if o.isReg {
		c.setReg8(o.reg, v)
		return
	}
	c.write8(o.seg, o.off, v)
}

func (c *CPU) get16(o operand) uint16 {
	if o.isReg {
		return c.R[o.reg]
	}
	if o.w32 {
		return uint16(c.segRead(o.seg, o.off, 2, true))
	}
	return c.read16(o.seg, o.off)
}

func (c *CPU) set16(o operand, v uint16) {
	if o.isReg {
		c.R[o.reg] = v
		return
	}
	if o.w32 {
		c.segWrite(o.seg, o.off, 2, true, uint32(v))
		return
	}
	c.write16(o.seg, o.off, v)
}

// regOperand 把 ModRM 的 reg 欄位包成 operand，讓 r/m 與 r 兩邊走同一組存取函式。
func regOperand(i int) operand { return operand{isReg: true, reg: i} }
