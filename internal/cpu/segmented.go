package cpu

import "fmt"

// 分段模式的位址計算與段載入（docs/spec/434-legacy-core-segmented-mode §4、§5.1）。
//
// 只在 segmented=true 時由 cpu.go、modrm.go、ops386.go 的記憶體原語呼叫；實模式路徑不經這裡。

// 段描述元快取屬性（433 §3.2 的 Attr 編碼）。
const (
	attrAccessed = 1 << 0
	attrRW       = 1 << 1 // 資料段：可寫；碼段：可讀
	attrEC       = 1 << 2 // 資料段：向下展開；碼段：一致
	attrExec     = 1 << 3
	attrS        = 1 << 4 // 1 ＝ 碼段或資料段；0 ＝ 系統描述元
	attrP        = 1 << 7
	attrAVL      = 1 << 12
	attrDB       = 1 << 14
	attrG        = 1 << 15
)

// accessKind 是 segCheck 的存取種類。
type accessKind uint8

const (
	accRead accessKind = iota
	accWrite
	accFetch
)

var segNames = [6]string{"ES", "CS", "SS", "DS", "FS", "GS"}

// segCheck 檢查段 i 在偏移 off 的一個 byte 能否以 kind 存取（434 §4.1，刻意從嚴）：
//
//   - 經不可用快取（P=0）存取失敗；
//   - 取指令只檢查界限（CS 由 loadSeg 保證是碼段）；
//   - 寫入：只能是可寫資料段；讀取：資料段，或可讀碼段；系統描述元一律失敗；
//   - 向上展開：偏移 ≤ Limit；向下展開：偏移 > Limit，且 ≤ FFFFh（B=0）或 FFFFFFFFh（B=1）。
//
// 失敗時以 failf 記下段暫存器、選擇子、偏移、界限，回 false。#GP／#SS 的遞送屬 S6。
func (c *CPU) segCheck(i int, off uint32, kind accessKind) bool {
	sc := &c.cache[i]
	a := sc.Attr
	var why string
	switch {
	case a&attrP == 0:
		why = "經不可用的段快取存取"
	case kind == accFetch:
		if off > sc.Limit {
			why = "取指令超出界限"
		}
	case a&attrS == 0:
		why = "段快取是系統描述元"
	case a&attrExec != 0:
		switch {
		case kind == accWrite:
			why = "寫入碼段"
		case a&attrRW == 0:
			why = "讀取不可讀的碼段"
		case off > sc.Limit:
			why = "超出界限"
		}
	case kind == accWrite && a&attrRW == 0:
		why = "寫入唯讀資料段"
	case a&attrEC != 0:
		upper := uint32(0xFFFF)
		if a&attrDB != 0 {
			upper = 0xFFFFFFFF
		}
		if off <= sc.Limit || off > upper {
			why = "超出向下展開段的界限"
		}
	case off > sc.Limit:
		why = "超出界限"
	}
	if why == "" {
		return true
	}
	c.failf("分段模式%s：%s=%04X 偏移 %08X 界限 %08X 屬性 %04X（#GP／#SS 遞送屬 S6，docs/spec/434 §4.1）",
		why, segNames[i], c.segVal(i), off, sc.Limit, a)
	return false
}

// segOffsets 回 n 個 byte 各自的偏移：第一個 byte 是呼叫端給的偏移（16 位元定址時已 ≤ FFFFh，
// 67 定址的單 byte 存取不截斷）；其後 w32 時 32 位元相加，否則各自 16 位元環繞（434 §4.1）。
func segOffsets(off uint32, n int, w32 bool) (o [4]uint32) {
	o[0] = off
	for k := 1; k < n; k++ {
		if w32 {
			o[k] = off + uint32(k)
		} else {
			o[k] = uint32(uint16(off + uint32(k)))
		}
	}
	return o
}

// segRead 讀 n（1、2、4）個 byte，小端。所有 byte 都先檢查，任一失敗就不讀、回 0。
// 線性位址＝快取 Base＋偏移（32 位元環繞），交給 Bus（432 的線性層）。
func (c *CPU) segRead(i int, off uint32, n int, w32 bool) uint32 {
	o := segOffsets(off, n, w32)
	for k := 0; k < n; k++ {
		if !c.segCheck(i, o[k], accRead) {
			return 0
		}
	}
	base := c.cache[i].Base
	var v uint32
	for k := 0; k < n; k++ {
		v |= uint32(c.Bus.Read8(base+o[k])) << (8 * k)
	}
	return v
}

// segWrite 寫 n 個 byte，小端。所有 byte 都先檢查，任一失敗就一個都不寫。
func (c *CPU) segWrite(i int, off uint32, n int, w32 bool, v uint32) {
	o := segOffsets(off, n, w32)
	for k := 0; k < n; k++ {
		if !c.segCheck(i, o[k], accWrite) {
			return
		}
	}
	base := c.cache[i].Base
	for k := 0; k < n; k++ {
		c.Bus.Write8(base+o[k], uint8(v>>(8*k)))
	}
}

// segFetch8 取 CS 快取 Base＋IP 的一個 byte（IP 16 位元，434 §4.1），檢查界限。
func (c *CPU) segFetch8() uint8 {
	if !c.segCheck(CS, uint32(c.IP), accFetch) {
		return 0
	}
	return c.Bus.Read8(c.cache[CS].Base + uint32(c.IP))
}

// stackCheck：分段模式的堆疊操作只支援 B=0 的 SS（434 §4.1；B=1 屬後續規格）。
func (c *CPU) stackCheck() {
	if c.cache[SS].Attr&attrDB != 0 {
		c.failf("分段模式的堆疊操作：SS=%04X 的 B=1（32 位元堆疊）不支援，失敗即關閉（docs/spec/434 §4.1）", c.Seg[SS])
	}
}

// loadSeg 是「載入新段值」的唯一入口（434 §5.1）。segmented=false 時只寫段值，行為不變；
// segmented=true 時讀描述元、檢查、通過後才寫段值與快取。失敗時以 failf 記下原因、
// 段值與快取都不寫，回 false。
func (c *CPU) loadSeg(i int, sel uint16) bool {
	if !c.segmented {
		c.setSegVal(i, sel)
		return true
	}
	return c.loadSegPM(i, sel)
}

// retRPLOK：遠返回時彈出的 CS 的 RPL 必須等於 CPL（434 §5.1）。
func (c *CPU) retRPLOK(sel uint16) bool {
	if cpl := c.Sys.CPL(); uint8(sel&3) != cpl {
		c.failf("遠返回：彈出的 CS=%04X 的 RPL %d ≠ CPL %d（特權變換不支援，docs/spec/434 §5.1）", sel, sel&3, cpl)
		return false
	}
	return true
}

// descriptor 由 8 bytes 描述元組出快取：Limit 依 G 展開；Attr 照 433 §3.2 的編碼
// （access byte、AVL、D/B、G；bit 13 恆 0，所以 386 保留的 bit 21 不帶進來）。
func descriptor(lo, hi uint32) SegCache {
	limit := lo&0xFFFF | hi&0xF0000
	base := lo>>16 | (hi&0xFF)<<16 | hi&0xFF000000
	attr := uint16(hi>>8) & 0xFF
	if hi&(1<<20) != 0 {
		attr |= attrAVL
	}
	if hi&(1<<22) != 0 {
		attr |= attrDB
	}
	if hi&(1<<23) != 0 {
		attr |= attrG
		limit = limit<<12 | 0xFFF
	}
	return SegCache{Base: base, Limit: limit, Attr: attr}
}

// loadSegPM 是分段模式的段載入（434 §5.1）：
//
//  1. 空選擇子（(sel & FFFCh) == 0，RPL 忽略）：CS、SS 失敗；其餘允許，快取設為不可用。
//  2. TI=0 查 GDT、TI=1 查 LDT（LDTR 快取不可用時失敗）；索引×8＋7 > 表界限失敗。
//     描述元以 ReadSup32 經 432 的線性層讀取。
//  3. 檢查照 DOSBox-X CPU_SetSegGeneral（cpu.cpp:2976-3078）與 CPU_JMP（cpu.cpp:1785-1835），
//     違反時失敗即關閉，不送例外。
//  4. 通過後才寫段值與快取。5. 不寫回 accessed 位元。
//
// 只支援 CPL 0：SysBus.CPL() 不為 0 時任何段載入失敗。
func (c *CPU) loadSegPM(i int, sel uint16) bool {
	fail := func(format string, a ...any) bool {
		c.failf("分段模式載入 %s=%04X：%s（docs/spec/434 §5.1）", segNames[i], sel, fmt.Sprintf(format, a...))
		return false
	}
	cpl := c.Sys.CPL()
	if cpl != 0 {
		return fail("CPL %d ≠ 0，本規格只支援 CPL 0", cpl)
	}
	if sel&0xFFFC == 0 {
		if i == CS || i == SS {
			return fail("空選擇子不能載入 CS、SS")
		}
		c.setSegVal(i, sel)
		c.cache[i] = SegCache{}
		return true
	}
	var tbase, tlimit uint32
	if sel&4 != 0 {
		_, ldt := c.Sys.LDTR()
		if ldt.Attr&attrP == 0 {
			return fail("TI=1 而 LDTR 不可用")
		}
		tbase, tlimit = ldt.Base, ldt.Limit
	} else {
		b, l := c.Sys.GDTR()
		tbase, tlimit = b, uint32(l)
	}
	idx := uint32(sel &^ 7)
	if idx+7 > tlimit {
		return fail("索引 %X×8＋7 超出表界限 %X", idx>>3, tlimit)
	}
	d := descriptor(c.Sys.ReadSup32(tbase+idx), c.Sys.ReadSup32(tbase+idx+4))
	a := d.Attr
	rpl := uint8(sel & 3)
	dpl := uint8(a>>5) & 3
	code := a&attrExec != 0
	switch i {
	case SS:
		if a&attrS == 0 || code || a&attrRW == 0 {
			return fail("SS 必須是可寫資料段（access %02X）", uint8(a))
		}
		if rpl != cpl || dpl != cpl {
			return fail("SS 要求 RPL＝CPL＝DPL（RPL %d、CPL %d、DPL %d）", rpl, cpl, dpl)
		}
	case CS:
		if a&attrS == 0 || !code {
			return fail("CS 必須是碼段（TSS、閘門不支援；access %02X）", uint8(a))
		}
		if a&attrEC != 0 {
			return fail("一致碼段的轉移不支援")
		}
		if rpl > cpl || dpl != cpl {
			return fail("CS 要求 RPL ≤ CPL 且 DPL＝CPL（RPL %d、CPL %d、DPL %d）", rpl, cpl, dpl)
		}
		if a&attrDB != 0 {
			return fail("D=1 的 32 位元碼段屬後續規格")
		}
	default: // DS、ES、FS、GS
		if a&attrS == 0 {
			return fail("系統描述元不能載入資料段暫存器（access %02X）", uint8(a))
		}
		if code && a&attrRW == 0 {
			return fail("不可讀的碼段不能載入資料段暫存器")
		}
		if !(code && a&attrEC != 0) && (rpl > dpl || cpl > dpl) {
			return fail("要求 RPL ≤ DPL 且 CPL ≤ DPL（RPL %d、CPL %d、DPL %d）", rpl, cpl, dpl)
		}
	}
	if a&attrP == 0 {
		return fail("段不存在（P=0）")
	}
	if i == CS {
		sel = sel&^3 | uint16(cpl)
	}
	c.setSegVal(i, sel)
	c.cache[i] = d
	return true
}
