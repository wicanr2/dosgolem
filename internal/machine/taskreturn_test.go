package machine

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/436-tss-task-return-v86-entry §6 第 3 項（任務返回）與第 5 項（PM 往返）。
//
// 合成環境仿 §1 的表：GDT 在 segGDT（界限擴成 FFh），目前 TR=F0h（386 TSS、忙碌），back link 指向
// 10h（386 TSS、忙碌、VM=1 的 V86 任務）。舊 TSS 先填滿 EEh，只放 back link，用來確認 CR3、LDT、
// back link 未被寫、段值以 32 位元寫入。新 TSS 的六個段值各不相同，用來確認順序。

const (
	trOldSel  = 0xF0
	trNewSel  = 0x10
	trOldBase = 0x81D30
	trNewBase = 0x82000
	trCode    = 0x24596 // 2000:4596
)

// trEnv 是一次任務返回測試的環境。
type trEnv struct {
	m                *Machine
	oldBase, newBase uint32
	opIP             uint16
}

func (e *trEnv) put32(a, v uint32) {
	for i := uint32(0); i < 4; i++ {
		e.m.Mem[a+i] = byte(v >> (8 * i))
	}
}

func (e *trEnv) get32(a uint32) uint32 {
	return uint32(e.m.Mem[a]) | uint32(e.m.Mem[a+1])<<8 | uint32(e.m.Mem[a+2])<<16 | uint32(e.m.Mem[a+3])<<24
}

// installTaskEnv 在已進分段模式、程式在 IP 處的機器上裝好 GDT、兩個 TSS、TR、LDTR 與核心狀態。
func installTaskEnv(t *testing.T, m *Machine, oldBase, newBase uint32) *trEnv {
	t.Helper()
	e := &trEnv{m: m, oldBase: oldBase, newBase: newBase, opIP: m.CPU.IP}
	putDesc(m, segGDT, 0x08, segData, 0xFFFFF, 0x92, 0)
	putDesc(m, segGDT, trOldSel, oldBase, 0x67, 0x8B, 0)
	putDesc(m, segGDT, trNewSel, newBase, 0x2068, 0x8B, 0)
	m.SetGDTR(segGDT, 0xFF)
	m.SetTR(trOldSel, SegCache{Base: oldBase, Limit: 0x67, Attr: 0x8B})
	m.SetLDTR(0x08, SegCache{Base: 0x91400, Limit: 7, Attr: 0x82})

	for i := uint32(0); i < 0x68; i++ {
		m.Mem[oldBase+i] = 0xEE
	}
	m.Mem[oldBase], m.Mem[oldBase+1] = trNewSel, 0
	for i := uint32(0); i < 0x68; i++ {
		m.Mem[newBase+i] = 0
	}
	e.put32(newBase+tssCR3, m.CR3())
	e.put32(newBase+tssEIP, 0x4596)
	e.put32(newBase+tssEFLAGS, 0x00023202)
	for i := uint32(0); i < 8; i++ {
		e.put32(newBase+tssRegs+4*i, 0x01010000*(i+1)+0x0101*(i+1))
	}
	e.put32(newBase+tssRegs+4*cpu.SP, 0x200)
	for i, v := range []uint32{0x2001, 0x2000, 0x2002, 0x2003, 0x2004, 0x2005} {
		e.put32(newBase+tssSegs+4*uint32(i), 0xFFFF0000|v) // 高半是垃圾：只取 16 位元
	}
	e.put32(newBase+tssLDT, 0xBB)
	copy(m.Mem[trCode:], []byte{0x33, 0xC0}) // xor ax,ax

	c := m.CPU
	c.SetFlags(c.Flags | cpu.NT | 0x3000 | cpu.CF)
	for i := 0; i < 8; i++ {
		c.R[i], c.Hi[i] = uint16(0xA000+i), uint16(0xB000+i)
	}
	c.R[cpu.SP] = 0xFFF0
	c.Seg[cpu.ES], c.Seg[cpu.SS], c.Seg[cpu.DS] = 0x10, 0x10, 0x08
	c.FSeg, c.GSeg = 0x08, 0
	return e
}

func taskProg(t *testing.T, code []byte) *trEnv {
	t.Helper()
	return installTaskEnv(t, segProg(t, code), trOldBase, trNewBase)
}

// pagedTaskProg：低 1 MB 恆等映射（頁目錄 70000h）並開分頁，再進分段模式。
func pagedTaskProg(t *testing.T, code []byte, oldBase, newBase uint32) (*trEnv, *tables) {
	t.Helper()
	m, tb := newPagedProg(t, code, fPWU)
	mustNil(t, "enterSegmented", m.enterSegmented())
	return installTaskEnv(t, m, oldBase, newBase), tb
}

// cpuView 是失敗時要保持不變的核心狀態。
type cpuView struct {
	r, hi    [8]uint16
	seg      [4]uint16
	fs, gs   uint16
	ip, fl   uint16
	seg2     bool
	efHi     uint16
	trSel    uint16
	tr       SegCache
	ldtSel   uint16
	cr3      uint32
	gdt, tss string
}

func (e *trEnv) view() cpuView {
	m, c := e.m, e.m.CPU
	v := cpuView{r: c.R, hi: c.Hi, seg: c.Seg, fs: c.FSeg, gs: c.GSeg, ip: c.IP, fl: c.Flags,
		seg2: c.Segmented(), efHi: m.EFlagsHi(), cr3: m.CR3()}
	v.trSel, v.tr = m.TR()
	v.ldtSel, _ = m.LDTR()
	v.gdt = string(m.Mem[segGDT : segGDT+0x100])
	v.tss = string(m.Mem[e.oldBase:e.oldBase+0x68]) + string(m.Mem[e.newBase:e.newBase+0x68])
	return v
}

func TestTaskReturnEntersV86(t *testing.T) {
	e := taskProg(t, []byte{0x66, 0xCF}) // iretd
	m, c := e.m, e.m.CPU
	oldFlags, oldR, oldHi := c.Flags, c.R, c.Hi
	oldSegs := [6]uint16{c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS], c.FSeg, c.GSeg}
	gen := m.tlbGen
	mustStep(t, m, "iretd（NT=1）")

	if !m.VM() || m.EFlagsHi() != eflagsHiVM || c.Segmented() {
		t.Errorf("VM=%v EFlagsHi=%04X segmented=%v，應為 VM=1、RF=0、segmented=false", m.VM(), m.EFlagsHi(), c.Segmented())
	}
	if c.Seg[cpu.CS] != 0x2000 || c.IP != 0x4596 || c.Seg[cpu.SS] != 0x2002 || c.R[cpu.SP] != 0x200 || c.Hi[cpu.SP] != 0 {
		t.Errorf("CS:IP=%04X:%04X SS:ESP=%04X:%04X%04X", c.Seg[cpu.CS], c.IP, c.Seg[cpu.SS], c.Hi[cpu.SP], c.R[cpu.SP])
	}
	if c.Flags != 0x3202 {
		t.Errorf("FLAGS=%04X，應為 3202", c.Flags)
	}
	for i := 0; i < 8; i++ {
		want := e.get32(e.newBase + tssRegs + 4*uint32(i))
		if got := uint32(c.R[i]) | uint32(c.Hi[i])<<16; got != want {
			t.Errorf("通用暫存器 %d = %08X，應為 %08X", i, got, want)
		}
	}
	if c.Seg[cpu.ES] != 0x2001 || c.Seg[cpu.DS] != 0x2003 || c.FSeg != 0x2004 || c.GSeg != 0x2005 {
		t.Errorf("段值 ES=%04X DS=%04X FS=%04X GS=%04X", c.Seg[cpu.ES], c.Seg[cpu.DS], c.FSeg, c.GSeg)
	}
	for i := cpu.ES; i <= cpu.GS; i++ {
		if c.SegCache(i) != (SegCache{}) {
			t.Errorf("段快取 %d 應清 0：%+v", i, c.SegCache(i))
		}
	}
	if sel, tc := m.TR(); sel != trNewSel || tc != (SegCache{Base: e.newBase, Limit: 0x2068, Attr: 0x8B}) {
		t.Errorf("TR=%04X %+v", sel, tc)
	}
	if sel, _ := m.LDTR(); sel != 0x08 {
		t.Errorf("LDTR=%04X，不得載入新 TSS 的 LDT", sel)
	}
	if got := m.Mem[segGDT+trOldSel+5]; got != 0x89 {
		t.Errorf("舊 TSS 描述元 access=%02X，應為 89h（不忙碌）", got)
	}
	if got := m.Mem[segGDT+trNewSel+5]; got != 0x8B {
		t.Errorf("新 TSS 描述元 access=%02X，應仍為 8Bh", got)
	}
	ob := e.oldBase
	if got, want := e.get32(ob+tssEFLAGS), uint32(oldFlags&^cpu.NT); got != want {
		t.Errorf("舊 TSS 的 EFLAGS=%08X，應為 %08X（清 NT）", got, want)
	}
	if got := e.get32(ob + tssEIP); got != uint32(e.opIP)+2 {
		t.Errorf("舊 TSS 的 EIP=%08X，應為下一道 %04X", got, e.opIP+2)
	}
	for i := 0; i < 8; i++ {
		if got, want := e.get32(ob+tssRegs+4*uint32(i)), uint32(oldR[i])|uint32(oldHi[i])<<16; got != want {
			t.Errorf("舊 TSS 暫存器 %d = %08X，應為 %08X", i, got, want)
		}
	}
	for i, v := range oldSegs {
		if got := e.get32(ob + tssSegs + 4*uint32(i)); got != uint32(v) {
			t.Errorf("舊 TSS 段 %d = %08X，應為 %08X（高半 0）", i, got, v)
		}
	}
	if e.get32(ob+tssCR3) != 0xEEEEEEEE || e.get32(ob+tssLDT) != 0xEEEEEEEE || e.get32(ob) != 0xEEEE0000|trNewSel {
		t.Errorf("CR3、LDT、back link 不得寫：%08X %08X %08X", e.get32(ob+tssCR3), e.get32(ob+tssLDT), e.get32(ob))
	}
	if m.tlbGen != gen {
		t.Error("CR3 相同時不得清 TLB")
	}
	// V86 繼續執行（432 的語意）。
	mustStep(t, m, "xor ax,ax（V86）")
	if c.R[cpu.AX] != 0 || c.IP != 0x4598 {
		t.Errorf("V86 繼續：AX=%04X IP=%04X", c.R[cpu.AX], c.IP)
	}
}

func TestTaskReturnCR3AndNotBusyAnd16Bit(t *testing.T) {
	// CR3 不同：寫入並清 TLB。
	e := taskProg(t, []byte{0x66, 0xCF})
	e.put32(e.newBase+tssCR3, 0x5000)
	gen := e.m.tlbGen
	mustStep(t, e.m, "iretd（CR3 不同）")
	if e.m.CR3() != 0x5000 || e.m.tlbGen == gen {
		t.Errorf("CR3=%08X tlbGen %d→%d，應寫入並清 TLB", e.m.CR3(), gen, e.m.tlbGen)
	}
	// 目前 TSS 不忙碌：照常切換。
	e = taskProg(t, []byte{0x66, 0xCF})
	putDesc(e.m, segGDT, trOldSel, e.oldBase, 0x67, 0x89, 0)
	e.m.SetTR(trOldSel, SegCache{Base: e.oldBase, Limit: 0x67, Attr: 0x89})
	mustStep(t, e.m, "iretd（目前 TSS 不忙碌）")
	if !e.m.VM() || e.m.Mem[segGDT+trOldSel+5] != 0x89 {
		t.Errorf("VM=%v 舊描述元 %02X", e.m.VM(), e.m.Mem[segGDT+trOldSel+5])
	}
	// 16 位元 CF 帶 NT=1：同樣任務返回，存入的 EIP 是下一道（長度 1）。
	e = taskProg(t, []byte{0xCF})
	mustStep(t, e.m, "iret（NT=1）")
	if !e.m.VM() || e.m.CPU.IP != 0x4596 || e.get32(e.oldBase+tssEIP) != uint32(e.opIP)+1 {
		t.Errorf("16 位元 iret：VM=%v IP=%04X 舊 EIP=%08X", e.m.VM(), e.m.CPU.IP, e.get32(e.oldBase+tssEIP))
	}
	// NT=0 的 iret、66 iret 仍失敗（訊息含 IRET）。
	for _, code := range [][]byte{{0xCF}, {0x66, 0xCF}} {
		e = taskProg(t, code)
		e.m.CPU.SetFlags(e.m.CPU.Flags &^ cpu.NT)
		wantStepErr(t, e.m, "NT=0 的 iret", "IRET")
	}
}

// §3.2 第 1–3 步的失敗條件：失敗時狀態與 CPU.Step 返回時相同（IP 在本道指令起點）。
func TestTaskReturnFailures(t *testing.T) {
	type tc struct {
		name string
		mut  func(e *trEnv)
		why  string
	}
	setBack := func(sel uint16) func(e *trEnv) {
		return func(e *trEnv) { e.m.Mem[e.oldBase], e.m.Mem[e.oldBase+1] = byte(sel), byte(sel>>8) }
	}
	cases := []tc{
		{"TR 無效", func(e *trEnv) { e.m.SetTR(0, SegCache{}) }, "無效"},
		{"TR 的 TI=1", func(e *trEnv) { e.m.SetTR(trOldSel|4, SegCache{Base: e.oldBase, Limit: 0x67, Attr: 0x8B}) }, "TI=1"},
		{"TR 界限 < 67h", func(e *trEnv) { e.m.SetTR(trOldSel, SegCache{Base: e.oldBase, Limit: 0x66, Attr: 0x8B}) }, "67h"},
		{"back link 為空", setBack(0), "空選擇子"},
		{"指向非忙碌 TSS", func(e *trEnv) { putDesc(e.m, segGDT, trNewSel, e.newBase, 0x2068, 0x89, 0) }, "忙碌"},
		{"back link 的 TI=1", setBack(trNewSel | 4), "TI=1"},
		{"back link 超出 GDT 界限", setBack(0x100), "界限"},
		{"新 TSS 不存在", func(e *trEnv) { putDesc(e.m, segGDT, trNewSel, e.newBase, 0x2068, 0x0B, 0) }, "不存在"},
		{"新 TSS 界限 < 67h", func(e *trEnv) { putDesc(e.m, segGDT, trNewSel, e.newBase, 0x66, 0x8B, 0) }, "67h"},
		{"同選擇子", setBack(trOldSel), "相同"},
		{"VM=0", func(e *trEnv) { e.put32(e.newBase+tssEFLAGS, 0x00003202) }, "VM=0"},
		{"EFLAGS 高位元", func(e *trEnv) { e.put32(e.newBase+tssEFLAGS, 0x00063202) }, "bit 18"},
		{"新 EIP 高半非 0", func(e *trEnv) { e.put32(e.newBase+tssEIP, 0x00014596) }, "EIP"},
	}
	for _, c := range cases {
		e := taskProg(t, []byte{0x66, 0xCF})
		c.mut(e)
		before := e.view()
		err := wantStepErr(t, e.m, c.name, c.why)
		if errors.Is(err, cpu.ErrTaskReturn) {
			t.Errorf("%s：失敗不得回哨兵：%v", c.name, err)
		}
		var pfe *PageFaultError
		if errors.As(err, &pfe) {
			t.Errorf("%s：沒有分頁紀錄時應回一般 error：%v", c.name, err)
		}
		if after := e.view(); after != before {
			t.Errorf("%s：失敗時狀態不得改變\n前 %+v\n後 %+v", c.name, before, after)
		}
		if e.m.CPU.IP != e.opIP {
			t.Errorf("%s：IP=%04X，應在本道指令起點 %04X", c.name, e.m.CPU.IP, e.opIP)
		}
	}
}

// 分頁：讀取遇到不存在頁回 *PageFaultError；寫入目標頁不存在回一般 error。兩者狀態都不變。
func TestTaskReturnPagingFailures(t *testing.T) {
	// 對照：分頁開啟、頁都存在時照常切換。
	e, _ := pagedTaskProg(t, []byte{0x66, 0xCF}, trOldBase, trNewBase)
	mustStep(t, e.m, "iretd（分頁）")
	if !e.m.VM() {
		t.Fatal("分頁開啟時應照常切換")
	}

	// 新 TSS 所在頁不存在。
	e, tb := pagedTaskProg(t, []byte{0x66, 0xCF}, trOldBase, trNewBase)
	tb.setPTE(trNewBase, 0)
	mustNil(t, "SetCR3", e.m.SetCR3(tb.dir))
	before := e.view()
	err := e.m.Step()
	var pfe *PageFaultError
	if !errors.As(err, &pfe) || pfe.Record.Linear>>12 != trNewBase>>12 {
		t.Fatalf("讀取遇到不存在頁：應回 *PageFaultError（線性在新 TSS 頁），實際 %v", err)
	}
	if errors.Is(err, cpu.ErrTaskReturn) || !strings.Contains(err.Error(), "分頁錯誤") {
		t.Errorf("包住的應是任務返回的失敗：%v", err)
	}
	if after := e.view(); after != before {
		t.Errorf("讀取失敗時狀態不得改變\n前 %+v\n後 %+v", before, after)
	}

	// 舊 TSS 跨頁：back link 在存在的頁，20h–5Fh 在不存在的頁。
	const straddle = 0x80FF0
	e, tb = pagedTaskProg(t, []byte{0x66, 0xCF}, straddle, trNewBase)
	tb.setPTE(0x81000, 0)
	mustNil(t, "SetCR3", e.m.SetCR3(tb.dir))
	before = e.view()
	err = wantStepErr(t, e.m, "寫入目標頁不存在", "不存在")
	if errors.As(err, &pfe) {
		t.Errorf("寫入目標的探測無副作用，應回一般 error：%v", err)
	}
	if _, ok := e.m.PageFault(); ok {
		t.Error("探測不得留下分頁紀錄")
	}
	if after := e.view(); after != before {
		t.Errorf("寫入目標不存在時狀態不得改變\n前 %+v\n後 %+v", before, after)
	}
}

// §6 第 5 項：任務返回後 SaveState／LoadState 與 Snapshot／Restore 還原 VM=1、TR、segmented=false，Step 照 V86 繼續。
func TestTaskReturnStateRoundTrip(t *testing.T) {
	e := taskProg(t, []byte{0x66, 0xCF})
	mustStep(t, e.m, "iretd")
	snap := e.m.Snapshot()
	data := saveBytes(t, e.m)
	wantSel, wantTR := e.m.TR()

	check := func(what string, d *Machine) {
		t.Helper()
		if !d.VM() || d.CPU.Segmented() || !d.Translating() {
			t.Errorf("%s：VM=%v segmented=%v xlat=%v", what, d.VM(), d.CPU.Segmented(), d.Translating())
		}
		if sel, tc := d.TR(); sel != wantSel || tc != wantTR {
			t.Errorf("%s：TR=%04X %+v，應為 %04X %+v", what, sel, tc, wantSel, wantTR)
		}
		d.IRQ0Every = 0
		mustStep(t, d, what+"：xor ax,ax（V86）")
		if d.CPU.R[cpu.AX] != 0 || d.CPU.Seg[cpu.CS] != 0x2000 || d.CPU.IP != 0x4598 {
			t.Errorf("%s：V86 繼續 AX=%04X CS:IP=%04X:%04X", what, d.CPU.R[cpu.AX], d.CPU.Seg[cpu.CS], d.CPU.IP)
		}
	}
	other := handoffProg(t)
	other.Restore(snap)
	check("Restore", other)

	dst := newPM(t)
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	if !bytes.Equal(saveBytes(t, dst), data) {
		t.Error("存檔→讀檔→存檔不一致")
	}
	check("LoadState", dst)
}
