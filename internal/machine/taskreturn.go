package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 任務返回（docs/spec/436-tss-task-return-v86-entry §3.2）。
//
// 既有核心在分段模式執行 iret／iretd 且 EFLAGS.NT=1 時回 cpu.ErrTaskReturn（IP 已在本道指令起點，
// 下一道的位址由 TaskReturnNextIP 取得），finishStep 呼叫 taskReturn。步驟照 DOSBox-X CPU_SwitchTask 的
// TSwitch_IRET 路徑（cpu/cpu.cpp:905-1091）。第 1–3 步只讀不寫，任何失敗都在第 4 步之前，失敗時狀態與
// CPU.Step 返回時相同（讀取時 walk 設 A 位元、填 TLB 是讀取本身的效果）。

// 386 TSS 的欄位偏移（436 §1 的表）。
const (
	tssBackLink = 0x00
	tssCR3      = 0x1C
	tssEIP      = 0x20
	tssEFLAGS   = 0x24
	tssRegs     = 0x28 // EAX、ECX、EDX、EBX、ESP、EBP、ESI、EDI，各 4 bytes（索引同 cpu.AX…cpu.DI）
	tssSegs     = 0x48 // ES、CS、SS、DS、FS、GS，各 4 bytes
	tssLDT      = 0x60
	tssSaveEnd  = 0x5F // 舊 TSS 的寫入範圍 20h–5Fh 的末端
	tssMinLimit = 0x67 // 386 TSS 的最小界限

	// TSS 描述元 access byte 的低 5 位元（S=0）：386 TSS 忙碌 Bh；忙碌位元 02h；present 80h。
	descTypeMask    = 0x1F
	descTSS386Busy  = 0x0B
	descBusyBit     = 0x02
	descPresent     = 0x80
	descAccessByte  = 5
	eflagsHiAllowed = eflagsHiRF | eflagsHiVM
)

// taskErr 組任務返回失敗的錯誤：含本道指令的選擇子:偏移與步號。
func (m *Machine) taskErr(format string, a ...any) error {
	cs, ip := m.CPU.OpAddr()
	return fmt.Errorf("machine: %04X:%04X 任務返回（iret、NT=1）失敗即關閉：%s（step=%d，docs/spec/436 §3.2）",
		cs, ip, fmt.Sprintf(format, a...), m.Steps)
}

// tssDesc 讀 GDT 中 sel 的描述元（TI=0、在 GDT 界限內才讀），回 access byte 所在的線性位址與快取。
func (m *Machine) tssDesc(what string, sel uint16) (accessAddr uint32, d SegCache, err error) {
	if sel&4 != 0 {
		return 0, d, m.taskErr("%s %04X 的 TI=1（須在 GDT）", what, sel)
	}
	base, limit := m.GDTR()
	idx := uint32(sel &^ 7)
	if idx+7 > uint32(limit) {
		return 0, d, m.taskErr("%s %04X 的索引 %X×8＋7 超出 GDT 界限 %X", what, sel, idx>>3, limit)
	}
	lo, hi := m.ReadSup32(base+idx), m.ReadSup32(base+idx+4)
	return base + idx + descAccessByte, cpu.DecodeDescriptor(lo, hi), nil
}

// newTSS 是第 3 步讀到的新 TSS 內容。
type newTSS struct {
	sel                  uint16
	cache                SegCache
	cr3, eip, eflags     uint32
	regs                 [8]uint32
	es, cs, ss, ds, f, g uint16
}

// taskReturn 做 436 §3.2 的第 1–10 步。回 nil 表示切換完成；錯誤時第 4 步之後的動作都沒做。
func (m *Machine) taskReturn() error {
	c := m.CPU
	// 1. 目前 TR：有效、TI=0、在 GDT 界限內、快取界限 ≥ 67h。不忙碌時 DOSBox-X 只記 log，這裡不失敗。
	curSel, cur := m.TR()
	if curSel&0xFFFC == 0 || cur.Attr&descPresent == 0 {
		return m.taskErr("目前 TR %04X 無效（真機 #TS）", curSel)
	}
	if curSel&4 != 0 {
		return m.taskErr("目前 TR %04X 的 TI=1（真機 #TS）", curSel)
	}
	gdtBase, gdtLimit := m.GDTR()
	curIdx := uint32(curSel &^ 7)
	if curIdx+7 > uint32(gdtLimit) {
		return m.taskErr("目前 TR %04X 的索引 %X×8＋7 超出 GDT 界限 %X（真機 #TS）", curSel, curIdx>>3, gdtLimit)
	}
	if cur.Limit < tssMinLimit {
		return m.taskErr("目前 TR %04X 的快取界限 %X < 67h（真機 #TS）", curSel, cur.Limit)
	}

	// 2. back link。
	back := m.ReadSup16(cur.Base + tssBackLink)
	if m.pfOK {
		return m.taskErr("讀目前 TSS（基底 %08X）的 back link 遇到分頁錯誤", cur.Base)
	}
	if back&0xFFFC == 0 {
		return m.taskErr("目前 TSS 的 back link %04X 是空選擇子", back)
	}

	// 3. 新 TSS：描述元檢查，再讀全部欄位，再檢查內容；最後確認寫入目標存在。
	n, err := m.readNewTSS(back)
	if err != nil {
		return err
	}
	if n.sel == curSel {
		return m.taskErr("back link %04X 與目前 TR 相同（DOSBox-X 有同選擇子特例；刻意從嚴）", n.sel)
	}
	if n.eflags&(eflagsHiVM<<16) == 0 {
		return m.taskErr("新 TSS %04X 的 EFLAGS %08X 的 VM=0（保護模式任務不在範圍）", n.sel, n.eflags)
	}
	if n.eflags>>16&^uint32(eflagsHiAllowed) != 0 {
		return m.taskErr("新 TSS %04X 的 EFLAGS %08X 的 bit 18–31 不為 0（386 沒有 AC、VIF、VIP、ID）", n.sel, n.eflags)
	}
	if n.eip>>16 != 0 {
		return m.taskErr("新 TSS %04X 的 EIP %08X 高 16 位元不為 0（刻意從嚴）", n.sel, n.eip)
	}
	curAccess := gdtBase + curIdx + descAccessByte
	if m.paging {
		for _, a := range [...]uint32{curAccess, cur.Base + tssEIP, cur.Base + tssSaveEnd} {
			if _, ok := m.peekXlatPage(a); !ok {
				return m.taskErr("寫入目標線性 %08X 所在頁不存在（舊 TSS 描述元的 access byte 或舊 TSS 的 20h–5Fh）", a)
			}
		}
	}

	// 4. 清目前 TSS 描述元的忙碌位元（讀改寫）。
	m.WriteSup8(curAccess, m.ReadSup8(curAccess)&^descBusyBit)

	// 5. 目前狀態寫入目前 TSS。不寫 CR3、LDT、back link。
	m.WriteSup32(cur.Base+tssEFLAGS, (uint32(c.Flags)|uint32(m.eflagsHi)<<16)&^uint32(cpu.NT))
	m.WriteSup32(cur.Base+tssEIP, uint32(c.TaskReturnNextIP()))
	for i := 0; i < 8; i++ {
		m.WriteSup32(cur.Base+tssRegs+uint32(4*i), uint32(c.R[i])|uint32(c.Hi[i])<<16)
	}
	segs := [6]uint16{c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS], c.FSeg, c.GSeg}
	for i, v := range segs {
		m.WriteSup32(cur.Base+tssSegs+uint32(4*i), uint32(v))
	}

	// 6. CR3 不同才寫（432 清 TLB）。
	if n.cr3 != m.cr3 {
		if err := m.SetCR(3, n.cr3); err != nil {
			return m.taskErr("寫 CR3：%v", err)
		}
	}

	// 7. 載入新內容。先離開分段模式，SetEFlagsHi 的守門才允許設 VM。RF 清 0。
	c.SetSegmented(false)
	for i := cpu.ES; i <= cpu.GS; i++ {
		c.SetSegCache(i, SegCache{})
	}
	if err := m.SetEFlagsHi(uint16(n.eflags>>16) & eflagsHiVM); err != nil {
		return m.taskErr("設 VM：%v", err)
	}
	c.SetFlags(uint16(n.eflags))
	c.IP = uint16(n.eip)
	for i := 0; i < 8; i++ {
		c.R[i], c.Hi[i] = uint16(n.regs[i]), uint16(n.regs[i]>>16)
	}
	c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS] = n.es, n.cs, n.ss, n.ds
	c.FSeg, c.GSeg = n.f, n.g

	// 8. LDTR 不載入（照 DOSBox-X cpu.cpp:1043-1050）。
	// 9. TR ← 新選擇子（描述元仍為忙碌）；434 的一百萬步計數歸零。
	m.SetTR(n.sel, n.cache)
	m.resetSegWait()
	return nil
}

// readNewTSS 是 §3.2 第 3 步的描述元檢查與欄位讀取（只讀）。讀取留下分頁紀錄時失敗。
func (m *Machine) readNewTSS(sel uint16) (n newTSS, err error) {
	n.sel = sel
	_, d, err := m.tssDesc("back link", sel)
	if err != nil {
		return n, err
	}
	if m.pfOK {
		return n, m.taskErr("讀新 TSS %04X 的描述元遇到分頁錯誤", sel)
	}
	if uint8(d.Attr)&descTypeMask != descTSS386Busy {
		return n, m.taskErr("back link %04X 不是忙碌的 386 TSS（access %02X）", sel, uint8(d.Attr))
	}
	if d.Attr&descPresent == 0 {
		return n, m.taskErr("新 TSS %04X 不存在（P=0）", sel)
	}
	if d.Limit < tssMinLimit {
		return n, m.taskErr("新 TSS %04X 的界限 %X < 67h", sel, d.Limit)
	}
	n.cache = d
	b := d.Base
	n.cr3 = m.ReadSup32(b + tssCR3)
	n.eip = m.ReadSup32(b + tssEIP)
	n.eflags = m.ReadSup32(b + tssEFLAGS)
	for i := range n.regs {
		n.regs[i] = m.ReadSup32(b + tssRegs + uint32(4*i))
	}
	segs := [6]*uint16{&n.es, &n.cs, &n.ss, &n.ds, &n.f, &n.g}
	for i, p := range segs {
		*p = m.ReadSup16(b + tssSegs + uint32(4*i))
	}
	m.ReadSup16(b + tssLDT) // 照 DOSBox-X 讀取（mem_readw），不載入（第 8 步）
	if m.pfOK {
		return n, m.taskErr("讀新 TSS %04X（基底 %08X）的欄位遇到分頁錯誤", sel, b)
	}
	return n, nil
}
