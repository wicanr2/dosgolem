package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// V86 事件經 IDT 遞送到 ring 0，與 ring 0 iretd 回 V86（docs/spec/435-v86-hooks-and-idt-delivery §2.3–§2.5）。
//
// 遞送分兩段：prepareV86Delivery 只讀不寫（第 1–4 步，失敗時狀態不變並失敗即關閉；讀取時 walk 設 A 位元、
// 填 TLB 是讀取本身的效果），commitV86Delivery 只寫且不會失敗（第 5 步）。照 DOSBox-X CPU_Interrupt 的
// 保護模式分支（cpu/cpu.cpp:1300-1475）。

// 段描述元快取 Attr 的位元（433 §3.2 的編碼，同 internal/cpu 的 attr 常數）。
const (
	attrRW    = 1 << 1
	attrEC    = 1 << 2 // 資料段：向下展開；碼段：一致
	attrExec  = 1 << 3
	attrS     = 1 << 4
	attrP     = 1 << 7
	attrDB    = 1 << 14
	attrDPLSh = 5

	gateInt386  = 0x0E // 386 中斷閘（S=0）
	gateTrap386 = 0x0F // 386 陷阱閘
)

// v86Plan 是 prepare 算好、commit 照寫的遞送內容。
type v86Plan struct {
	vector   uint8
	hasErr   bool
	errCode  uint32
	frameEIP uint32
	gateSel  uint16
	gateIP   uint16
	trap     bool // 陷阱閘：不清 IF
	csCache  SegCache
	ss0      uint16
	esp0     uint32
	ss0Cache SegCache
}

// deliverFail 組遞送失敗的錯誤：含向量、位置與步號。位置在軟體中斷與例外是本道指令的起點（OpAddr），
// 在外部中斷是遞送當下的 CS:IP。
type deliverFail func(format string, a ...any) error

func (m *Machine) deliverErrFn(ev cpu.V86Event, vec *uint8) deliverFail {
	cs, ip := m.CPU.OpAddr()
	if ev.Kind == cpu.V86External {
		cs, ip = m.CPU.Seg[cpu.CS], m.CPU.IP
	}
	return func(format string, a ...any) error {
		return fmt.Errorf("machine: %04X:%04X V86 事件（向量 %02Xh）經 IDT 遞送失敗即關閉：%s（step=%d，docs/spec/435 §2.3）",
			cs, ip, *vec, fmt.Sprintf(format, a...), m.Steps)
	}
}

// gdtDesc 讀 GDT 中 sel 的描述元：TI=0、索引×8＋7 ≤ GDTR 界限才讀。
func (m *Machine) gdtDesc(fail deliverFail, what string, sel uint16) (SegCache, error) {
	if sel&4 != 0 {
		return SegCache{}, fail("%s %04X 的 TI=1（須在 GDT）", what, sel)
	}
	base, limit := m.GDTR()
	idx := uint32(sel &^ 7)
	if idx+7 > uint32(limit) {
		return SegCache{}, fail("%s %04X 超出 GDT 界限 %X", what, sel, limit)
	}
	return cpu.DecodeDescriptor(m.ReadSup32(base+idx), m.ReadSup32(base+idx+4)), nil
}

// segOffOK 是 434 §4.1 的界限判定（單一 byte 偏移）：向上展開 off ≤ Limit；向下展開 off > Limit 且
// ≤ FFFFh（B=0）或 FFFFFFFFh（B=1）。
func segOffOK(sc SegCache, off uint32) bool {
	if sc.Attr&attrEC != 0 {
		upper := uint32(0xFFFF)
		if sc.Attr&attrDB != 0 {
			upper = 0xFFFFFFFF
		}
		return off > sc.Limit && off <= upper
	}
	return off <= sc.Limit
}

// prepareV86Delivery 是 435 §2.3 第 1–4 步（只讀）。讀取留下分頁紀錄時回錯誤，呼叫端以 stepFault 包裝。
func (m *Machine) prepareV86Delivery(ev cpu.V86Event) (v86Plan, error) {
	c := m.CPU
	p := v86Plan{vector: ev.Vector, hasErr: ev.HasError, errCode: ev.ErrorCode, frameEIP: ev.FrameEIP}
	soft := ev.Kind == cpu.V86SoftInt || ev.Kind == cpu.V86Into || ev.Kind == cpu.V86Int3
	fail := m.deliverErrFn(ev, &p.vector)

	// 1. 軟體中斷（CD nn、INTO）的 IOPL 檢查；INT 3 不查（DOSBox-X NoIOPLCheck）。
	if (ev.Kind == cpu.V86SoftInt || ev.Kind == cpu.V86Into) && c.Flags&0x3000 != 0x3000 {
		p = v86Plan{vector: 13, hasErr: true, frameEIP: uint32(c.IP)}
		soft = false
	}

	// 2. IDT：界限、讀閘；軟體中斷先查閘 DPL（不通過改送 #GP(vector*8+2)，回本步重讀向量 13 的閘）。
	idtBase, idtLimit := m.IDTR()
	var lo, hi uint32
	for {
		if uint32(p.vector)*8+7 > uint32(idtLimit) {
			return p, fail("向量 %02Xh 超出 IDT 界限 %X（真機 #GP，屬 S6）", p.vector, idtLimit)
		}
		a := idtBase + uint32(p.vector)*8
		lo, hi = m.ReadSup32(a), m.ReadSup32(a+4)
		if m.pfOK {
			return p, fail("讀 IDT 項（線性 %08X）遇到分頁錯誤", a)
		}
		if soft && hi>>13&3 < 3 {
			p = v86Plan{vector: 13, hasErr: true, errCode: uint32(p.vector)*8 + 2, frameEIP: uint32(c.IP)}
			soft = false
			continue
		}
		break
	}
	switch typ := hi >> 8 & 0x1F; typ {
	case gateInt386:
	case gateTrap386:
		p.trap = true
	default:
		return p, fail("閘型別 %02Xh 不是 386 中斷閘或陷阱閘（286 閘、任務閘屬後續）", typ)
	}
	if hi&0x8000 == 0 {
		return p, fail("閘不存在（P=0）")
	}

	// 3. 目標碼段：TI=0、在 GDT 界限內；非一致碼段、present、DPL 0、D=0；閘偏移高 16 位元為 0。
	p.gateSel = uint16(lo >> 16)
	if p.gateSel&0xFFFC == 0 {
		return p, fail("閘選擇子 %04X 是空選擇子", p.gateSel)
	}
	cs, err := m.gdtDesc(fail, "閘選擇子", p.gateSel)
	if err != nil {
		return p, err
	}
	if m.pfOK {
		return p, fail("讀閘選擇子 %04X 的描述元遇到分頁錯誤", p.gateSel)
	}
	switch {
	case cs.Attr&attrS == 0 || cs.Attr&attrExec == 0:
		return p, fail("閘選擇子 %04X 不是碼段（屬性 %04X）", p.gateSel, cs.Attr)
	case cs.Attr&attrEC != 0:
		return p, fail("閘選擇子 %04X 是一致碼段（屬性 %04X）", p.gateSel, cs.Attr)
	case cs.Attr&attrP == 0:
		return p, fail("閘選擇子 %04X 的碼段不存在", p.gateSel)
	case cs.Attr>>attrDPLSh&3 != 0:
		return p, fail("閘選擇子 %04X 的碼段 DPL %d ≠ 0", p.gateSel, cs.Attr>>attrDPLSh&3)
	case cs.Attr&attrDB != 0:
		return p, fail("閘選擇子 %04X 是 32 位元碼段（D=1，屬後續規格）", p.gateSel)
	}
	if hi>>16 != 0 {
		return p, fail("閘偏移 %08X 高 16 位元不為 0", hi&0xFFFF0000|lo&0xFFFF)
	}
	p.gateIP, p.csCache = uint16(lo), cs

	// 4. 新堆疊：TR 有效；讀 ESP0、SS0；SS0 的檢查（cpu.cpp:1372-1394）；推入範圍的界限與頁面。
	trSel, tr := m.TR()
	if trSel&0xFFFC == 0 || tr.Attr&attrP == 0 {
		return p, fail("TR %04X 無效", trSel)
	}
	p.esp0 = m.ReadSup32(tr.Base + 4)
	p.ss0 = m.ReadSup16(tr.Base + 8)
	if m.pfOK {
		return p, fail("讀 TSS（基底 %08X）的 SS0:ESP0 遇到分頁錯誤", tr.Base)
	}
	if p.ss0&0xFFFC == 0 {
		return p, fail("SS0 %04X 是空選擇子", p.ss0)
	}
	ss, err := m.gdtDesc(fail, "SS0", p.ss0)
	if err != nil {
		return p, err
	}
	if m.pfOK {
		return p, fail("讀 SS0 %04X 的描述元遇到分頁錯誤", p.ss0)
	}
	switch {
	case p.ss0&3 != 0 || ss.Attr>>attrDPLSh&3 != 0:
		return p, fail("SS0 %04X 的 RPL 或 DPL 不為 0（屬性 %04X）", p.ss0, ss.Attr)
	case ss.Attr&attrS == 0 || ss.Attr&attrExec != 0 || ss.Attr&attrRW == 0:
		return p, fail("SS0 %04X 不是可寫資料段（屬性 %04X）", p.ss0, ss.Attr)
	case ss.Attr&attrP == 0:
		return p, fail("SS0 %04X 不存在", p.ss0)
	case ss.Attr&attrDB != 0:
		return p, fail("SS0 %04X 的 B=1（32 位元堆疊，屬後續規格）", p.ss0)
	}
	p.ss0Cache = ss
	for k := 0; k < p.frameDwords(); k++ {
		off := p.pushOff(k)
		for i := uint32(0); i < 4; i++ {
			if !segOffOK(ss, off+i) {
				return p, fail("推入第 %d 個 dword：SS0 %04X 偏移 %X 超出界限 %X", k, p.ss0, off+i, ss.Limit)
			}
		}
		if m.paging {
			for _, a := range [...]uint32{ss.Base + off, ss.Base + off + 3} {
				if _, ok := m.peekXlatPage(a); !ok {
					return p, fail("推入目標線性 %08X 所在頁不存在", a)
				}
			}
		}
	}
	return p, nil
}

// frameDwords 是推入的 dword 數：無錯誤碼 9、有錯誤碼 10。
func (p *v86Plan) frameDwords() int {
	if p.hasErr {
		return 10
	}
	return 9
}

// pushOff 是第 k 個 dword 的位移：uint16(ESP0 − 4(k+1))（B=0，位移 16 位元環繞，dword 本身不環繞）。
func (p *v86Plan) pushOff(k int) uint32 { return uint32(uint16(p.esp0 - 4*uint32(k+1))) }

// commitV86Delivery 是 435 §2.3 第 5 步（只寫、不會失敗）。
func (m *Machine) commitV86Delivery(p v86Plan) {
	c := m.CPU
	frame := [10]uint32{
		uint32(c.GSeg), uint32(c.FSeg), uint32(c.Seg[cpu.DS]), uint32(c.Seg[cpu.ES]), uint32(c.Seg[cpu.SS]),
		uint32(c.Hi[cpu.SP])<<16 | uint32(c.R[cpu.SP]),
		uint32(c.Flags) | uint32(m.eflagsHi)<<16,
		uint32(c.Seg[cpu.CS]), p.frameEIP, p.errCode,
	}
	c.Seg[cpu.SS] = p.ss0
	c.SetSegCache(cpu.SS, p.ss0Cache)
	c.R[cpu.SP], c.Hi[cpu.SP] = uint16(p.esp0), uint16(p.esp0>>16)
	for k := 0; k < p.frameDwords(); k++ {
		a := p.ss0Cache.Base + p.pushOff(k)
		for i := uint32(0); i < 4; i++ {
			m.WriteSup8(a+i, uint8(frame[k]>>(8*i)))
		}
		c.R[cpu.SP] -= 4
	}
	c.Seg[cpu.DS], c.Seg[cpu.ES], c.FSeg, c.GSeg = 0, 0, 0, 0
	for _, i := range [...]int{cpu.DS, cpu.ES, cpu.FS, cpu.GS} {
		c.SetSegCache(i, SegCache{})
	}
	c.Seg[cpu.CS] = p.gateSel &^ 3
	c.SetSegCache(cpu.CS, p.csCache)
	c.IP = p.gateIP
	c.SetSegmented(true)
	// 清 VM、RF 不動（cpu.cpp:1465-1472）。profile 開啟、PE=1、清 VM 的寫入不會失敗。
	_ = m.SetEFlagsHi(m.eflagsHi &^ eflagsHiVM)
	fl := c.Flags &^ (cpu.TF | cpu.NT)
	if !p.trap {
		fl &^= cpu.IF
	}
	c.SetFlags(fl)
	_ = m.SetPMCPL(0)
}

// raiseIRQ 是 tick、keyTick、timedKeyTick 的遞送點（435 §2.4）：ivtGuardIRQ 在 V86 準備好的遞送就提交，
// 否則照實模式走 IVT。
func (m *Machine) raiseIRQ(vec uint8) {
	if m.irqPlanOK {
		m.irqPlanOK = false
		m.commitV86Delivery(m.irqPlan)
		return
	}
	m.CPU.Interrupt(vec)
}

// v86Return 是 435 §2.5 第 4 步：以核心讀出的彈出值回 V86。CPL 由 VM 推得為 3。
func (m *Machine) v86Return() error {
	c := m.CPU
	r := c.V86Return()
	c.SetSegmented(false)
	for i := cpu.ES; i <= cpu.GS; i++ {
		c.SetSegCache(i, SegCache{})
	}
	hi := m.eflagsHi &^ eflagsHiVM
	if r.EFLAGS&(eflagsHiVM<<16) != 0 {
		hi |= eflagsHiVM
	}
	if err := m.SetEFlagsHi(hi); err != nil {
		return fmt.Errorf("machine: ring 0 iretd 回 V86：設 VM：%w", err)
	}
	c.SetFlags(uint16(r.EFLAGS))
	c.IP = uint16(r.EIP)
	c.Seg[cpu.CS], c.Seg[cpu.SS] = uint16(r.CS), uint16(r.SS)
	c.Seg[cpu.ES], c.Seg[cpu.DS] = uint16(r.ES), uint16(r.DS)
	c.FSeg, c.GSeg = uint16(r.FS), uint16(r.GS)
	c.R[cpu.SP], c.Hi[cpu.SP] = uint16(r.ESP), uint16(r.ESP>>16)
	m.resetSegWait()
	return nil
}
