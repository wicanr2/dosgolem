package machine

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/434-legacy-core-segmented-mode §9 第 7 項：§6 的指令與 §5.3 的中斷、IRQ、回呼。
// 環境見 segmode_test.go 的 segProg（SS 快取基底 1000h、SP=FFFEh）。

// ---- §6 -------------------------------------------------------------------

// 9A／FF /3：同特權遠呼叫推 CS、IP 各 2 bytes；CA iw：遠返回後 SP 再加 imm。
func TestSegInsnFarCallPushesAndRetfImm(t *testing.T) {
	m := segProg(t, []byte{0x9A, 0x00, 0x00, 0x18, 0x00, 0x90}) // call 18h:0000
	copy(m.Mem[segCode:], []byte{0xCA, 0x04, 0x00})             // retf 4
	putDesc(m, segGDT, 0x10, 0x1000, 0xFFFF, 0x9A, 0)
	m.CPU.Seg[cpu.CS] = 0x10 // 回程的選擇子（快取已是基底 1000h 的實模式推得值）
	sp, ip := m.CPU.R[cpu.SP], m.CPU.IP
	mustStep(t, m, "call far")
	if m.CPU.R[cpu.SP] != sp-4 {
		t.Errorf("遠呼叫推 4 bytes：SP %04X→%04X", sp, m.CPU.R[cpu.SP])
	}
	if got := le16(m, 0x1000+uint32(sp)-2); got != 0x10 {
		t.Errorf("推入的 CS=%04X，應為 0010", got)
	}
	if got := le16(m, 0x1000+uint32(sp)-4); got != ip+5 {
		t.Errorf("推入的 IP=%04X，應為 %04X", got, ip+5)
	}
	mustStep(t, m, "retf 4")
	if m.CPU.Seg[cpu.CS] != 0x10 || m.CPU.IP != ip+5 || m.CPU.R[cpu.SP] != sp+4 {
		t.Errorf("retf 4：CS:IP=%04X:%04X SP=%04X", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP])
	}
	// FF /3：call far [0200]。
	m = segProg(t, []byte{0xFF, 0x1E, 0x00, 0x02})
	copy(m.Mem[0x1200:], []byte{0x34, 0x12, 0x18, 0x00})
	sp = m.CPU.R[cpu.SP]
	mustStep(t, m, "call far [m]")
	if m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.IP != 0x1234 || m.CPU.R[cpu.SP] != sp-4 {
		t.Errorf("FF /3：CS:IP=%04X:%04X SP=%04X", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP])
	}
	// FF /5：jmp far [0200]。
	m = segProg(t, []byte{0xFF, 0x2E, 0x00, 0x02})
	copy(m.Mem[0x1200:], []byte{0x78, 0x56, 0x18, 0x00})
	mustStep(t, m, "jmp far [m]")
	if m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.IP != 0x5678 {
		t.Errorf("FF /5：CS:IP=%04X:%04X", m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
}

// 66 形式的遠轉移與 66 CB 失敗即關閉。
func TestSegInsn66FarTransfersFail(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"66 jmp far", []byte{0x66, 0xEA, 0, 0, 0, 0, 0x18, 0}},
		{"66 call far", []byte{0x66, 0x9A, 0, 0, 0, 0, 0x18, 0}},
		{"66 retf", []byte{0x66, 0xCB}},
		{"66 retf imm", []byte{0x66, 0xCA, 0, 0}},
		{"66 call far [m]", []byte{0x66, 0xFF, 0x1E, 0x00, 0x02}},
		{"66 jmp far [m]", []byte{0x66, 0xFF, 0x2E, 0x00, 0x02}},
	} {
		m := segProg(t, c.code)
		cs := m.CPU.Seg[cpu.CS]
		if err := m.Step(); err == nil {
			t.Errorf("%s：應失敗即關閉", c.name)
		}
		if m.CPU.Seg[cpu.CS] != cs {
			t.Errorf("%s：CS 不得改變", c.name)
		}
	}
}

// lldt：空選擇子、GDT 的 LDT 描述元、TI=1、非 LDT、不存在、超出界限；之後以 TI=1 選擇子載入成功。
func TestSegInsnLLDT(t *testing.T) {
	const ldt = 0x91000
	lldtAX := []byte{0x0F, 0x00, 0xD0} // lldt ax
	for _, c := range []struct {
		name   string
		sel    uint16
		access uint8
		ok     bool
		why    string
	}{
		{"GDT 的 LDT 描述元", 0x48, 0x82, true, ""},
		{"TI=1", 0x4C, 0x82, false, "TI=1"},
		{"不是 LDT（資料段）", 0x48, 0x92, false, "不是 LDT"},
		{"不是 LDT（TSS）", 0x48, 0x89, false, "不是 LDT"},
		{"不存在", 0x48, 0x02, false, "不存在"},
		{"超出 GDT 界限", 0x80, 0x82, false, "界限"},
	} {
		m := segProg(t, lldtAX)
		putDesc(m, segGDT, 0x48, ldt, 0x000F, c.access, 0)
		m.CPU.R[cpu.AX] = c.sel
		err := m.Step()
		sel, lc := m.LDTR()
		if c.ok {
			if err != nil {
				t.Errorf("%s：%v", c.name, err)
			}
			if sel != c.sel || lc != (SegCache{Base: ldt, Limit: 0xF, Attr: 0x82}) {
				t.Errorf("%s：LDTR=%04X %+v", c.name, sel, lc)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s：應失敗（%s），實際 %v", c.name, c.why, err)
		}
		if sel != 0 || lc != (SegCache{}) {
			t.Errorf("%s：失敗時 LDTR 不得改變", c.name)
		}
	}
	// lldt 之後以 TI=1 選擇子載入 DS；再 lldt 0 使 LDTR 不可用。
	m := segProg(t, []byte{
		0x0F, 0x00, 0xD0, // lldt ax
		0xB8, 0x0C, 0x00, 0x8E, 0xD8, // mov ax,0Ch；mov ds,ax（LDT 項 1）
		0x31, 0xC0, 0x0F, 0x00, 0xD0, // xor ax,ax；lldt ax
		0x0F, 0x00, 0xC3, // sldt bx
	})
	putDesc(m, segGDT, 0x48, ldt, 0x000F, 0x82, 0)
	putDesc(m, ldt, 0x08, 0x00ABC000, 0xFFFF, 0x92, 0)
	m.CPU.R[cpu.AX] = 0x48
	for i := 0; i < 3; i++ {
		mustStep(t, m, "lldt；mov ds,0Ch")
	}
	if m.CPU.Seg[cpu.DS] != 0x0C || m.CPU.SegCache(cpu.DS).Base != 0x00ABC000 {
		t.Errorf("TI=1 載入：DS=%04X %+v", m.CPU.Seg[cpu.DS], m.CPU.SegCache(cpu.DS))
	}
	for i := 0; i < 2; i++ {
		mustStep(t, m, "lldt 0")
	}
	if sel, lc := m.LDTR(); sel != 0 || lc.Attr&0x80 != 0 {
		t.Errorf("lldt 0：LDTR=%04X %+v，應不可用", sel, lc)
	}
	m.CPU.R[cpu.BX] = 0xFFFF
	mustStep(t, m, "sldt bx")
	if m.CPU.R[cpu.BX] != 0 {
		t.Errorf("sldt：BX=%04X", m.CPU.R[cpu.BX])
	}
}

// sldt、str 讀 LDTR、TR 選擇子（16 位元，暫存器高半不動）。
func TestSegInsnSLDTSTR(t *testing.T) {
	m := segProg(t, []byte{0x0F, 0x00, 0xC3, 0x0F, 0x00, 0x0E, 0x00, 0x02}) // sldt bx；str [0200]
	m.SetLDTR(0x48, SegCache{Base: 0x91000, Limit: 7, Attr: 0x82})
	m.sys.tr.sel = 0x00F0
	m.CPU.Hi[cpu.BX] = 0xABCD
	mustStep(t, m, "sldt bx")
	mustStep(t, m, "str [0200]")
	if m.CPU.R[cpu.BX] != 0x48 || m.CPU.Hi[cpu.BX] != 0xABCD || le16(m, 0x1200) != 0xF0 {
		t.Errorf("sldt=%04X（高半 %04X）str=%04X", m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX], le16(m, 0x1200))
	}
}

// ltr 是本規格的驗收停點：失敗即關閉，診斷含「S4」與 選擇子:偏移。
func TestSegInsnLTRStops(t *testing.T) {
	m := segProg(t, []byte{0xEA, 0x86, 0x45, 0x18, 0x00})  // jmp 18h:4586
	copy(m.Mem[segCode+0x4586:], []byte{0x0F, 0x00, 0xD9}) // ltr cx
	m.CPU.R[cpu.CX] = 0xF0
	mustStep(t, m, "jmp 18h:4586")
	if got := m.PCLinear(); got != segCode+0x4586 {
		t.Errorf("PCLinear=%08X，應為 %08X", got, segCode+0x4586)
	}
	err := wantStepErr(t, m, "ltr", "S4")
	if err != nil && !strings.Contains(err.Error(), "0018:4586") {
		t.Errorf("診斷應含 0018:4586：%v", err)
	}
	if got := m.InsnLinear(); got != segCode+0x4586 {
		t.Errorf("InsnLinear=%08X，應為 %08X", got, segCode+0x4586)
	}
	if s, o := m.Insn(); s != 0x18 || o != 0x4586 {
		t.Errorf("Insn()=%04X:%04X，應為選擇子:偏移 0018:4586", s, o)
	}
}

// 0F 00 /4、/5、0F 02、0F 03 失敗即關閉；實模式的 0F 00 仍是 433 的訊息。
func TestSegInsnOther0F00Fail(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
		want string
	}{
		{"verr", []byte{0x0F, 0x00, 0xE0}, "分段模式失敗"},
		{"verw", []byte{0x0F, 0x00, 0xE8}, "分段模式失敗"},
		{"lar", []byte{0x0F, 0x02, 0xC0}, "LAR"},
		{"lsl", []byte{0x0F, 0x03, 0xC0}, "LSL"},
	} {
		wantStepErr(t, segProg(t, c.code), c.name, c.want)
	}
	wantStepErr(t, newSysProg(t, []byte{0x0F, 0x00, 0xD0}), "實模式 lldt", "實模式的 0F 00 群組")
}

// 0F 01 群組在分段模式沿用 433 的處理函式：sgdt；mov cr0 設 PG 不回哨兵；clts。
func TestSegInsnSystemGroup(t *testing.T) {
	m := segProg(t, []byte{
		0x0F, 0x01, 0x06, 0x00, 0x02, // sgdt [0200]
		0x0F, 0x06, // clts
		0x0F, 0x22, 0xC0, // mov cr0,eax（設 PG）
	})
	mustStep(t, m, "sgdt")
	if le16(m, 0x1200) != segGDTLim || le16(m, 0x1202) != uint16(segGDT&0xFFFF) || m.Mem[0x1204] != byte(segGDT>>16) {
		t.Errorf("sgdt：% X", m.Mem[0x1200:0x1206])
	}
	mustStep(t, m, "clts")
	// 恆等分頁：頁目錄 80000h、頁表 81000h（前 4 MB）。
	m.Mem[0x80000], m.Mem[0x80001], m.Mem[0x80002] = 0x03, 0x10, 0x08
	for i := uint32(0); i < 1024; i++ {
		e := i<<12 | 3
		m.Mem[0x81000+4*i], m.Mem[0x81000+4*i+1], m.Mem[0x81000+4*i+2] = byte(e), byte(e>>8), byte(e>>16)
	}
	mustNil(t, "SetCR3", m.SetCR3(0x80000))
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x0011, 0x8000
	if err := m.Step(); err != nil {
		t.Fatalf("mov cr0 設 PG：不應回哨兵或錯誤，實際 %v", err)
	}
	if !m.paging || m.CR0() != 0x80000011 || !m.CPU.Segmented() {
		t.Errorf("PG 應由 SetCR0 重算：paging=%v CR0=%08X", m.paging, m.CR0())
	}
}

// hlt 失敗即關閉；in/out 在 CPL ≤ IOPL 時照既有路徑；pushf／popf 不變。
func TestSegInsnHltIOFlags(t *testing.T) {
	wantStepErr(t, segProg(t, []byte{0xF4}), "hlt", "HLT")
	m := segProg(t, []byte{0xE4, 0x61, 0x9C, 0x9D}) // in al,61h；pushf；popf
	mustStep(t, m, "in al,61h（CPL 0 ≤ IOPL 0）")
	fl := m.CPU.Flags
	mustStep(t, m, "pushf")
	mustStep(t, m, "popf")
	if m.CPU.Flags != fl {
		t.Errorf("pushf／popf：旗標 %04X→%04X", fl, m.CPU.Flags)
	}
}

// x87：FNINIT、FNSTCW、FLDCW、FWAIT 以外失敗即關閉。
func TestSegInsnX87(t *testing.T) {
	m := segProg(t, []byte{
		0xDB, 0xE3, // fninit
		0xD9, 0x3E, 0x00, 0x02, // fnstcw [0200]
		0xD9, 0x2E, 0x00, 0x02, // fldcw [0200]
		0x9B,       // fwait
		0xD8, 0xC1, // fadd st,st(1)
	})
	for i := 0; i < 4; i++ {
		mustStep(t, m, "fninit／fnstcw／fldcw／fwait")
	}
	if le16(m, 0x1200) != 0x037F {
		t.Errorf("fnstcw：%04X", le16(m, 0x1200))
	}
	wantStepErr(t, m, "fadd", "x87")
}

// ---- §5.3 -----------------------------------------------------------------

// 分段模式下 INT、IntHook 攔截的 INT 21h、16／32 位元 div 0、BOUND 越界失敗即關閉。
func TestSegInterruptsFail(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"int 10h", []byte{0xCD, 0x10}},
		{"int 21h（IntHook）", []byte{0xCD, 0x21}},
		{"16 位元 div 0", []byte{0xF7, 0xF3}},
		{"32 位元 div 0", []byte{0x66, 0xF7, 0xF3}},
		{"BOUND 越界", []byte{0x62, 0x06, 0x00, 0x02}},
	} {
		m := segProg(t, c.code)
		hooked := false
		m.CPU.IntHook = func(*cpu.CPU, uint8) bool { hooked = true; return true }
		m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0, 0
		m.CPU.R[cpu.AX] = 5
		copy(m.Mem[0x1200:], []byte{0x10, 0x00, 0x20, 0x00}) // BOUND [0200]：10h..20h
		cs, sp := m.CPU.Seg[cpu.CS], m.CPU.R[cpu.SP]
		wantStepErr(t, m, c.name, "435")
		if hooked {
			t.Errorf("%s：檢查在 IntHook 之前，不得呼叫 IntHook", c.name)
		}
		if m.CPU.R[cpu.SP] != sp || m.CPU.Seg[cpu.CS] != cs {
			t.Errorf("%s：不得推入、不得改 CS", c.name)
		}
	}
}

// IRQ0、按鍵、startCallback、periodic 在分段模式延後且不消耗；IRQ0 時鐘照常累積、幀照常計數。
func TestSegIRQDeferred(t *testing.T) {
	m := segProg(t, []byte{0xFA, 0xEB, 0xFE}) // cli；jmp $
	m.IRQ0Every = 5
	m.nextIRQ0 = m.Steps + 5
	m.VGAFrameEvery = 7
	m.nextFrame = m.Steps + 7
	m.PushKey(0x1E)
	m.HoldKey(0x30, m.Steps+3, 2, false)
	m.QueueCallback(QueuedCall{Seg: 0x1234, Off: 0x10})
	m.SetPeriodicFarCall(0x2345, 0x20, 4)
	ticks, frames := m.Ticks, m.Frames
	keys, timed := m.KeyQueueLen(), m.TimedKeysPending()
	cs := m.CPU.Seg[cpu.CS]
	for i := 0; i < 40; i++ {
		mustStep(t, m, "延後中")
	}
	if !m.irq0Pending || m.Ticks != ticks {
		t.Errorf("IRQ0：pending=%v Ticks %d→%d（時鐘照常、遞送延後）", m.irq0Pending, ticks, m.Ticks)
	}
	if m.Frames == frames {
		t.Error("幀計數照常（不經 CPU）")
	}
	if m.KeyQueueLen() != keys || m.TimedKeysPending() != timed || m.KeyIRQs != 0 {
		t.Errorf("按鍵不得消耗：佇列 %d→%d、定時 %d→%d、IRQ1 %d", keys, m.KeyQueueLen(), timed, m.TimedKeysPending(), m.KeyIRQs)
	}
	if m.CallbackPending() != 1 || m.CallbacksMade() != 0 || m.PeriodicCalls() != 0 {
		t.Errorf("回呼不得執行：排隊 %d、已做 %d、週期 %d", m.CallbackPending(), m.CallbacksMade(), m.PeriodicCalls())
	}
	if m.CPU.Seg[cpu.CS] != cs {
		t.Error("不得遞送到任何向量或回呼")
	}
}

// 有待處理 IRQ 且 IF=1 一百萬步後失敗；IF=0 時不失敗。
func TestSegPendingIRQWithIFFails(t *testing.T) {
	run := func(code []byte, n int) (int, error) {
		m := segProg(t, code)
		m.IRQ0Every = 100
		m.nextIRQ0 = m.Steps + 100
		for i := 0; i < n; i++ {
			if err := m.Step(); err != nil {
				return i, err
			}
		}
		return n, nil
	}
	// sti；jmp $：IRQ0 在第 100 步左右掛起，之後每步計數。
	done, err := run([]byte{0xFB, 0xEB, 0xFE}, 1_200_000)
	if err == nil || !strings.Contains(err.Error(), "連續") {
		t.Fatalf("IF=1 且待處理：應在一百萬步後失敗，實際 %v（%d 步）", err, done)
	}
	if done < segWaitLimit || done > segWaitLimit+200 {
		t.Errorf("失敗的步數 %d，應在一百萬步附近", done)
	}
	if _, err := run([]byte{0xFA, 0xEB, 0xFE}, 1_100_000); err != nil {
		t.Errorf("IF=0：不應失敗，實際 %v", err)
	}
}

// Coverage 與斷點：分段模式用 PCLinear()；否則維持 cpu.Addr。
func TestSegCoverageAndBreak(t *testing.T) {
	m := segProg(t, []byte{0xEA, 0x10, 0x00, 0x18, 0x00}) // jmp 18h:0010
	copy(m.Mem[segCode+0x10:], []byte{0x90, 0x90})
	m.Coverage = make([]bool, 0x80000)
	mustStep(t, m, "jmp")
	id := m.AddLinearBreak(segCode + 0x11)
	mustStep(t, m, "nop")
	if !m.Coverage[segCode+0x10] {
		t.Error("Coverage 應記 CS 快取基底＋IP")
	}
	if !m.atBreak() {
		t.Error("分段模式的斷點應比對 PCLinear()")
	}
	m.ClearBreak(id)
	// 實模式：BreakAt 與 Coverage 照 cpu.Addr。
	r := newSysProg(t, []byte{0x90, 0x90})
	r.Coverage = make([]bool, 0x100000)
	r.BreakAt(r.CPU.Seg[cpu.CS], r.CPU.IP+1)
	mustStep(t, r, "nop")
	if !r.Coverage[cpu.Addr(r.CPU.Seg[cpu.CS], r.CPU.IP-1)] || !r.atBreak() {
		t.Error("實模式的 Coverage 與斷點照 cpu.Addr")
	}
	if r.PCLinear() != cpu.Linear(r.CPU.Seg[cpu.CS], r.CPU.IP) || r.InsnLinear() != cpu.Linear(r.CPU.Seg[cpu.CS], r.CPU.IP-1) {
		t.Error("實模式的 PCLinear／InsnLinear 為 cpu.Linear")
	}
}
