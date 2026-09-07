package machine

import (
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// DPMI 主機（`int 31h`）的通用子集。
//
// 契約來源是 **DPMI 1.0 規格**，不是 DOSBox-X——DOSBox-X 沒有這一層
// （它跑真的 extender，見 `docs/knowledge-base/030-protected-mode-and-dos4gw.md`）。
// 這裡實作的是 extender 對**客戶端程式**的公開介面，所以它與哪一支程式無關：
// 換一支 DOS/4GW 程式，這一份照用。
//
// 沒有做的：分頁、例外、`AX=0300h`（模擬實模式中斷）。前兩者只有跑真的
// extender 才需要；`0300h` 要一份 51 byte 的暫存器結構與一顆實模式 CPU，
// 等有程式踩到再開規格——**踩到的時候會留下痕跡**（`Unimplemented`），
// 不會安靜地走錯路。

// 描述子配置的起點與間隔。
//
// ⚠ **我們不模型 RPL 與 TI 位元**：`cpu386` 的描述子表直接用 selector 的
// 數值當鍵。真硬體上 selector 的低三位是 RPL 與 table indicator，程式**會**
// 拿 `selector | 3` 當同一個描述子用——所以配出去的號碼一律讓低三位是 0，
// 程式自己加上 RPL 之後就對不上了。這是已知的簡化，標**假說待驗**：
// 目前沒有目標程式這樣做，真的遇到再把低三位遮掉。
const (
	dpmiFirstSelector = 0x0100
	dpmiSelectorStep  = 8
)

// dpmiHeapAlign 是線性記憶體配置（`AX=0501h`）的對齊，
// 讓配出去的位址在傾印裡一眼看得出是誰的。
const dpmiHeapAlign = 0x10000

// 位址空間切成兩半，界線在 1 MB。
//
// **這是為了讓「實模式段位址」這個東西存在。** `AX=0100h` 配出來的記憶體
// 要同時給得出一個實模式段（16 位元的段號 × 16），所以它必須落在 1 MB 之內；
// `AX=0501h` 的線性記憶體沒有這個限制，就擺在 1 MB 之上。
//
// 兩邊各有各的游標，**不會互相踩到**。合成一個游標的話，程式先配一大塊
// 線性記憶體再要一塊 DOS 記憶體，就會拿到一個大於 1 MB 的位址——
// 而段號只有 16 位元，位址被截斷之後指到映像頭上，寫下去就把自己的碼改了。
const dosMemTop = 0x100000

// DPMIBlock 是一次 `AX=0501h` 配出去的線性區塊。
type DPMIBlock struct {
	Handle uint32
	Base   uint32
	Size   uint32
}

// DPMILock 是一次 `AX=0600h` 鎖定的線性區間。
//
// 鎖定對我們沒有實際作用（沒有分頁），但**要記下來**：程式鎖了才做 DMA
// 或才把位址交給中斷處理常式，這份紀錄是「它打算對哪一段做這種事」的答案。
type DPMILock struct {
	Base, Size uint32
}

// DOSBlock 是一次 `AX=0100h` 配出去的實模式（1 MB 以下）記憶體。
//
// 程式拿它當「把資料交給實模式那一邊」的中轉區：檔名、磁區緩衝、
// VESA 的資訊結構都在這種區塊裡。
type DOSBlock struct {
	Selector uint16 // 保護模式那一邊用的 selector
	Segment  uint16 // 實模式那一邊用的段號
	Base     uint32 // 線性位址（＝ Segment × 16）
	Paras    uint16 // 大小，單位是段（16 bytes）
}

// DPMIHost 是一個 LE 機器上的 DPMI 服務。用 NewDPMIHost 造。
type DPMIHost struct {
	m *LEMachine

	nextSel uint16
	blocks  map[uint32]*DPMIBlock
	brk     uint32

	// DOS 記憶體（`AX=0100h`）的游標與帳本，鍵是 selector。
	dosBrk    uint32
	dosBlocks map[uint16]*DOSBlock
	dosLast   uint16 // 最後配出去的那一塊，`AX=0102h` 只有它能原地長大

	Locks []DPMILock

	realVec [256]uint32 // 實模式向量：段<<16 | 位移
	protVec [256]uint64 // 保護模式向量：selector<<32 | 位移

	// Calls 是每一個功能被叫過幾次，Unimplemented 是沒實作的那些。
	//
	// **收工前看一眼**：`Unimplemented` 非空表示程式要的東西我們沒給，
	// 而它多半不會因此報錯，只是走進另一條路。
	Calls         map[uint16]int
	Unimplemented map[uint16]int
}

// NewDPMIHost 綁一個 LE 機器。
//
// m 可以是 nil：**向量那一組（`0200h`／`0201h`／`0204h`／`0205h`）與版本查詢
// 不需要機器**，而啟動服務在拿到機器之前就會被問這些。之後用 Attach 補上，
// 描述子與線性記憶體那兩組才開始能用——在那之前它們照 DPMI 的慣例回失敗，
// 不會安靜地回一個沒有backing的位址。
func NewDPMIHost(m *LEMachine) *DPMIHost {
	var base uint32
	if m != nil {
		base = uint32(len(m.Mem))
		base = (base + dpmiHeapAlign - 1) &^ (dpmiHeapAlign - 1)
	}
	host := &DPMIHost{
		m:             m,
		nextSel:       dpmiFirstSelector,
		blocks:        map[uint32]*DPMIBlock{},
		dosBlocks:     map[uint16]*DOSBlock{},
		brk:           base,
		Calls:         map[uint16]int{},
		Unimplemented: map[uint16]int{},
	}
	if m != nil {
		host.setLimits(uint32(len(m.Mem)))
	}
	return host
}

// Attach 補上機器（見 NewDPMIHost 的 nil 情形）。
func (h *DPMIHost) Attach(m *LEMachine) {
	h.m = m
	if h.dosBlocks == nil {
		h.dosBlocks = map[uint16]*DOSBlock{}
	}
	h.setLimits(uint32(len(m.Mem)))
}

// setLimits 把兩個游標擺到映像之後：DOS 記憶體接著映像長，
// 線性記憶體從 1 MB（或映像的尾端，取大的）開始。
func (h *DPMIHost) setLimits(imageEnd uint32) {
	h.dosBrk = (imageEnd + 15) &^ 15
	base := (imageEnd + dpmiHeapAlign - 1) &^ (dpmiHeapAlign - 1)
	if base < dosMemTop {
		base = dosMemTop
	}
	h.brk = base
}

// DOSMemory 回目前配出去的 DOS 記憶體區塊，依位址排序（診斷用，順序要固定）。
func (h *DPMIHost) DOSMemory() []DOSBlock {
	out := make([]DOSBlock, 0, len(h.dosBlocks))
	for _, b := range h.dosBlocks {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Base < out[j].Base })
	return out
}

// dosFreeParas 是 DOS 記憶體還剩幾段。配置失敗時 BX 要回這個數字——
// 程式照它決定要不要降級（少載素材、換小一點的緩衝區）。
func (h *DPMIHost) dosFreeParas() uint16 {
	if h.dosBrk >= dosMemTop {
		return 0
	}
	return uint16((dosMemTop - h.dosBrk) / 16)
}

// allocDOS 從 1 MB 以下切一塊出來，回線性位址。
func (h *DPMIHost) allocDOS(paras uint16) (uint32, bool) {
	size := uint32(paras) * 16
	base := h.dosBrk
	end := uint64(base) + uint64(size)
	if end > uint64(dosMemTop) {
		return 0, false
	}
	if end > uint64(len(h.m.Mem)) {
		grown := make([]byte, int(end))
		copy(grown, h.m.Mem)
		h.m.Mem = grown
	}
	h.dosBrk = uint32(end)
	return base, true
}

// SetRealModeVector 讓載入器把實模式向量表的內容交給 DPMI 主機。
//
// 程式用 `AX=0200h` 問「原本的 timer 處理常式在哪」，然後把自己的接上去
// 再 chain 回原本那一支。回 0:0 的話它 chain 到 IVT 本身——那裡的位元組
// 是位址，被讀成指令之後會跑進一片亂碼。
func (h *DPMIHost) SetRealModeVector(n uint8, seg, off uint16) {
	h.realVec[n] = uint32(seg)<<16 | uint32(off)
}

// RealModeVector 回一個實模式向量（段, 位移）。
func (h *DPMIHost) RealModeVector(n uint8) (seg, off uint16) {
	v := h.realVec[n]
	return uint16(v >> 16), uint16(v)
}

// Blocks 回目前配出去的線性區塊，依 base 排序（診斷用，順序要固定）。
func (h *DPMIHost) Blocks() []DPMIBlock {
	out := make([]DPMIBlock, 0, len(h.blocks))
	for _, b := range h.blocks {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Base < out[j].Base })
	return out
}

// AllocSelector 配一個描述子並回 selector。載入器用它擺平坦段。
func (h *DPMIHost) AllocSelector(d cpu386.Descriptor) uint16 {
	sel := h.nextSel
	h.nextSel += dpmiSelectorStep
	h.m.CPU.SetDescriptor(sel, d)
	return sel
}

// Handle 是 `int 31h` 的分派。回 false 表示這一支沒實作
// （呼叫端要照 DPMI 的慣例設 CF 並記一筆）。
func (h *DPMIHost) Handle(c *cpu386.CPU) bool {
	fn := uint16(c.R[cpu386.EAX])
	h.Calls[fn]++
	if h.m == nil && needsMachine(fn) {
		return h.fail(c, 0x8013) // 還沒有記憶體可以配
	}
	switch fn {
	case 0x0000: // 配置 LDT 描述子：CX ＝ 個數 → AX ＝ 第一個 selector
		n := uint16(c.R[cpu386.ECX])
		if n == 0 {
			return h.fail(c, 0x8021) // 無效的值
		}
		first := h.nextSel
		for i := uint16(0); i < n; i++ {
			// 新配的描述子是**空的**（base 0、limit 0、不可寫）。
			// 給 4 GB 的話，程式忘記設 base 就直接用會讀到別人的資料，
			// 而那不會報錯。
			h.m.CPU.SetDescriptor(h.nextSel, cpu386.Descriptor{})
			h.nextSel += dpmiSelectorStep
		}
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(first)
		return h.ok(c)

	case 0x0001: // 釋放 LDT 描述子：BX
		delete(h.m.CPU.Descriptors, uint16(c.R[cpu386.EBX]))
		return h.ok(c)

	case 0x0003: // 取 selector 間隔 → AX
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | dpmiSelectorStep
		return h.ok(c)

	case 0x0006: // 取段基底：BX → CX:DX
		d, ok := h.m.CPU.Descriptors[uint16(c.R[cpu386.EBX])]
		if !ok {
			return h.fail(c, 0x8022) // 無效的 selector
		}
		c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | d.Base>>16
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | d.Base&0xffff
		return h.ok(c)

	case 0x0007: // 設段基底：BX、CX:DX
		sel := uint16(c.R[cpu386.EBX])
		d, ok := h.m.CPU.Descriptors[sel]
		if !ok {
			return h.fail(c, 0x8022)
		}
		d.Base = uint32(uint16(c.R[cpu386.ECX]))<<16 | uint32(uint16(c.R[cpu386.EDX]))
		h.m.CPU.SetDescriptor(sel, d)
		return h.ok(c)

	case 0x0008: // 設段界限：BX、CX:DX
		sel := uint16(c.R[cpu386.EBX])
		d, ok := h.m.CPU.Descriptors[sel]
		if !ok {
			return h.fail(c, 0x8022)
		}
		d.Limit = uint32(uint16(c.R[cpu386.ECX]))<<16 | uint32(uint16(c.R[cpu386.EDX]))
		h.m.CPU.SetDescriptor(sel, d)
		return h.ok(c)

	case 0x0009: // 設存取權限：BX、CL/CH
		// 我們的描述子只有「可不可寫」。權限位元 bit1 是 writable。
		sel := uint16(c.R[cpu386.EBX])
		d, ok := h.m.CPU.Descriptors[sel]
		if !ok {
			return h.fail(c, 0x8022)
		}
		d.Writable = uint8(c.R[cpu386.ECX])&0x02 != 0
		h.m.CPU.SetDescriptor(sel, d)
		return h.ok(c)

	case 0x000A: // 建立別名描述子：BX → AX（同一段記憶體，可寫）
		d, ok := h.m.CPU.Descriptors[uint16(c.R[cpu386.EBX])]
		if !ok {
			return h.fail(c, 0x8022)
		}
		d.Writable = true
		sel := h.AllocSelector(d)
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(sel)
		return h.ok(c)

	case 0x0100: // 配置 DOS 記憶體：BX ＝ 段數 → AX ＝ 實模式段、DX ＝ selector
		paras := uint16(c.R[cpu386.EBX])
		if paras == 0 {
			return h.fail(c, 0x8021)
		}
		base, ok := h.allocDOS(paras)
		if !ok {
			// **失敗要順便回報還剩多少**（BX），這是 DPMI 定的。
			// 不回報的話程式無從決定要降級到多小，多半就直接放棄。
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | uint32(h.dosFreeParas())
			return h.fail(c, 0x8013)
		}
		sel := h.AllocSelector(cpu386.Descriptor{
			Base: base, Limit: uint32(paras)*16 - 1, Writable: true})
		b := &DOSBlock{Selector: sel, Segment: uint16(base >> 4), Base: base, Paras: paras}
		h.dosBlocks[sel] = b
		h.dosLast = sel
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(b.Segment)
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(sel)
		return h.ok(c)

	case 0x0101: // 釋放 DOS 記憶體：DX ＝ selector
		sel := uint16(c.R[cpu386.EDX])
		if _, ok := h.dosBlocks[sel]; !ok {
			return h.fail(c, 0x8022) // 無效的 selector
		}
		delete(h.dosBlocks, sel)
		delete(h.m.CPU.Descriptors, sel)
		// 位址不回收，理由同 `AX=0502h`：釋放之後再配到同一段，
		// 「誰還留著舊指標」就查不出來了。
		return h.ok(c)

	case 0x0102: // 改變 DOS 記憶體大小：BX ＝ 新的段數、DX ＝ selector
		sel := uint16(c.R[cpu386.EDX])
		b, ok := h.dosBlocks[sel]
		if !ok {
			return h.fail(c, 0x8022)
		}
		paras := uint16(c.R[cpu386.EBX])
		if paras == 0 {
			return h.fail(c, 0x8021)
		}
		switch {
		case paras <= b.Paras: // 縮小一定可以
		case sel == h.dosLast: // 只有最後一塊能原地長大
			extra := uint32(paras-b.Paras) * 16
			if uint64(h.dosBrk)+uint64(extra) > uint64(dosMemTop) {
				c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | uint32(h.dosFreeParas()+b.Paras)
				return h.fail(c, 0x8013)
			}
			if _, ok := h.allocDOS(paras - b.Paras); !ok {
				return h.fail(c, 0x8013)
			}
		default:
			// 中間那一塊要長大就得搬家，而**搬家會讓程式手上的實模式段
			// 位址失效**——它多半已經把那個段號存進自己的結構了。
			// 照 DPMI 的慣例回失敗並說還剩多少，讓它自己決定。
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | uint32(h.dosFreeParas())
			return h.fail(c, 0x8013)
		}
		b.Paras = paras
		h.m.CPU.SetDescriptor(sel, cpu386.Descriptor{
			Base: b.Base, Limit: uint32(paras)*16 - 1, Writable: true})
		return h.ok(c)

	case 0x0200: // 取實模式中斷向量：BL → CX:DX
		v := h.realVec[uint8(c.R[cpu386.EBX])]
		c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | v>>16
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | v&0xffff
		return h.ok(c)

	case 0x0201: // 設實模式中斷向量：BL、CX:DX
		h.realVec[uint8(c.R[cpu386.EBX])] =
			uint32(uint16(c.R[cpu386.ECX]))<<16 | uint32(uint16(c.R[cpu386.EDX]))
		return h.ok(c)

	case 0x0204: // 取保護模式中斷向量：BL → CX:EDX
		v := h.protVec[uint8(c.R[cpu386.EBX])]
		c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | uint32(v>>32)
		c.R[cpu386.EDX] = uint32(v)
		return h.ok(c)

	case 0x0205: // 設保護模式中斷向量：BL、CX:EDX
		h.protVec[uint8(c.R[cpu386.EBX])] =
			uint64(uint16(c.R[cpu386.ECX]))<<32 | uint64(c.R[cpu386.EDX])
		return h.ok(c)

	case 0x0400: // 取 DPMI 版本
		// AX ＝ 版本（0.90）、BX ＝ 旗標（bit0 ＝ 32 位元）、
		// CL ＝ 處理器（4 ＝ 486）、DH/DL ＝ 主／從 PIC 基底。
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0x005A
		c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | 0x0001
		c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffffff00 | 0x04
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | 0x0870
		return h.ok(c)

	case 0x0501: // 配置線性記憶體：BX:CX ＝ 大小 → BX:CX ＝ 線性位址、SI:DI ＝ handle
		size := uint32(uint16(c.R[cpu386.EBX]))<<16 | uint32(uint16(c.R[cpu386.ECX]))
		if size == 0 {
			return h.fail(c, 0x8021)
		}
		base, ok := h.alloc(size)
		if !ok {
			return h.fail(c, 0x8013) // 實體記憶體不足
		}
		b := &DPMIBlock{Handle: base, Base: base, Size: size}
		h.blocks[base] = b
		c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | base>>16
		c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | base&0xffff
		c.R[cpu386.ESI] = c.R[cpu386.ESI]&0xffff0000 | b.Handle>>16
		c.R[cpu386.EDI] = c.R[cpu386.EDI]&0xffff0000 | b.Handle&0xffff
		return h.ok(c)

	case 0x0502: // 釋放線性記憶體：SI:DI ＝ handle
		handle := uint32(uint16(c.R[cpu386.ESI]))<<16 | uint32(uint16(c.R[cpu386.EDI]))
		if _, ok := h.blocks[handle]; !ok {
			return h.fail(c, 0x8023) // 無效的 handle
		}
		// **不回收位址空間**：釋放之後再配到同一段，會讓「誰還留著舊指標」
		// 這個問題查不出來。位址單調遞增是刻意的取捨（診斷 > 空間）。
		delete(h.blocks, handle)
		return h.ok(c)

	case 0x0600, 0x0601: // 鎖定／解鎖線性區域：BX:CX ＝ 位址、SI:DI ＝ 長度
		base := uint32(uint16(c.R[cpu386.EBX]))<<16 | uint32(uint16(c.R[cpu386.ECX]))
		size := uint32(uint16(c.R[cpu386.ESI]))<<16 | uint32(uint16(c.R[cpu386.EDI]))
		if fn == 0x0600 {
			h.Locks = append(h.Locks, DPMILock{Base: base, Size: size})
		}
		return h.ok(c)

	case 0x0900: // 取並關中斷
		prev := uint32(0)
		if c.EFlags&cpu386.IF != 0 {
			prev = 1
		}
		c.EFlags &^= cpu386.IF
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffffff00 | prev
		return h.ok(c)

	case 0x0901: // 取並開中斷
		prev := uint32(0)
		if c.EFlags&cpu386.IF != 0 {
			prev = 1
		}
		c.EFlags |= cpu386.IF
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffffff00 | prev
		return h.ok(c)

	case 0x0902: // 取中斷狀態
		state := uint32(0)
		if c.EFlags&cpu386.IF != 0 {
			state = 1
		}
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffffff00 | state
		return h.ok(c)
	}

	h.Unimplemented[fn]++
	return false
}

// needsMachine 標出哪些功能要有 backing 的機器才做得到。
func needsMachine(fn uint16) bool {
	switch fn {
	case 0x0000, 0x0001, 0x0006, 0x0007, 0x0008, 0x0009, 0x000A, 0x0501, 0x0502,
		0x0100, 0x0101, 0x0102:
		return true
	}
	return false
}

// alloc 從線性配置游標切一塊出來，必要時把機器的記憶體長大。
func (h *DPMIHost) alloc(size uint32) (uint32, bool) {
	base := h.brk
	end := uint64(base) + uint64(size)
	if end > uint64(dpmiAddressLimit) {
		return 0, false
	}
	if end > uint64(len(h.m.Mem)) {
		grown := make([]byte, int(end))
		copy(grown, h.m.Mem)
		h.m.Mem = grown
	}
	// 下一塊對齊到 16 bytes：程式常常假設配出來的東西至少對齊到 paragraph。
	h.brk = uint32((end + 15) &^ 15)
	return base, true
}

// dpmiAddressLimit 是我們願意長到多大（64 MB）。
//
// **要有上限**：程式配置失敗時會走「記憶體不足」那條路（少載素材、降音質），
// 那是我們要能觀測的行為；沒有上限的話一個算錯大小的配置會直接把主機吃光。
const dpmiAddressLimit = 64 << 20

func (h *DPMIHost) ok(c *cpu386.CPU) bool {
	c.EFlags &^= cpu386.CF
	return true
}

// fail 照 DPMI 的慣例：CF=1，AX ＝ 錯誤碼。
func (h *DPMIHost) fail(c *cpu386.CPU, code uint16) bool {
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
	c.EFlags |= cpu386.CF
	return true
}
