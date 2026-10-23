package machine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/442-segmented-return-to-real-mode §3 第 3–7 項：分段模式清 PE 進入殘留快取實模式（子狀態），
// 六個段暫存器都重載成實模式推得值的那一步離開分段模式。

// ---- 共用 ----------------------------------------------------------------

// pokePhys／peekPhys 直接讀寫實體位址（Mem 或 Ext），不經線性層。
func pokePhys(m *Machine, a uint32, b ...byte) {
	for i, v := range b {
		p := a + uint32(i)
		if p < MemSize {
			m.Mem[p] = v
		} else {
			m.Ext[p-ExtBase] = v
		}
	}
}

func peekPhys(m *Machine, a uint32) uint8 {
	if a < MemSize {
		return m.Mem[a]
	}
	return m.Ext[a-ExtBase]
}

func peek16(m *Machine, a uint32) uint16 { return uint16(peekPhys(m, a)) | uint16(peekPhys(m, a+1))<<8 }

// putDescPhys 同 putDesc，但描述元表可在 1 MB 以上。
func putDescPhys(m *Machine, table uint32, sel uint16, base, limit uint32, access, flags uint8) {
	pokePhys(m, table+uint32(sel&^7), byte(limit), byte(limit>>8), byte(base), byte(base>>8), byte(base>>16),
		access, flags<<4|byte(limit>>16)&0xF, byte(base>>24))
}

// subProg：segProg 之後以 Machine 的 SetCR0 清 PE 進入子狀態（setter 不設離開旗標，所以停在子狀態）。
// GS 快取改成非推得值（GS=08h、基底 60000h），段載入不會因此離開；要離開的測試另行重載 GS。
func subProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := segProg(t, code)
	mustNil(t, "SetCR0 清 PE", m.SetCR0(pmCR0Init))
	m.CPU.GSeg = 0x08
	m.CPU.SetSegCache(cpu.GS, dataCache(segData, 0xFFFF, 0x93))
	if !m.CPU.Segmented() || !m.Translating() || m.CPU.Code != nil {
		t.Fatal("前提：應在殘留快取實模式")
	}
	return m
}

// wantSub 檢查是否仍在子狀態。
func wantSub(t *testing.T, m *Machine, what string, sub bool) {
	t.Helper()
	if got := m.CPU.Segmented(); got != sub {
		t.Fatalf("%s：segmented=%v，應為 %v", what, got, sub)
	}
	if sub && (m.CR0()&cr0PE != 0 || !m.Translating() || m.CPU.Code != nil) {
		t.Fatalf("%s：子狀態應 PE=0、xlat=true、Code==nil（CR0=%08X）", what, m.CR0())
	}
}

func wantCachesZero(t *testing.T, m *Machine, what string) {
	t.Helper()
	for i := 0; i < 6; i++ {
		if c := m.CPU.SegCache(i); c != (SegCache{}) {
			t.Errorf("%s：離開後快取 %d 應為 0：%+v", what, i, c)
		}
	}
}

// cpuState 是失敗即關閉時要保持不變的狀態。
type cpuState struct {
	seg        [4]uint16
	fs, gs, sp uint16
	cache      [6]SegCache
	cr0        uint32
	segmented  bool
}

func takeCPUState(m *Machine) cpuState {
	s := cpuState{seg: m.CPU.Seg, fs: m.CPU.FSeg, gs: m.CPU.GSeg, sp: m.CPU.R[cpu.SP], cr0: m.CR0(), segmented: m.CPU.Segmented()}
	for i := range s.cache {
		s.cache[i] = m.CPU.SegCache(i)
	}
	return s
}

// ---- §3 第 3 項：JEMM 5180–51ED 的合成重現 -------------------------------------------

const (
	jemmX      = 0x110000 // XMS 區塊線性位址（基底在 10FFF0h 以上，20 位元遮罩的錯誤才會現形）
	jemmSP     = 0x13BC
	jemmRetLin = 0x1230 + 0x40 // 框架 CS:IP＝0123:0040
	jemmIRQ8   = 0x80000       // IVT 第 8 項指向 8000:0000
)

// jemmCode 逐位元組照 strike_commander workplace/re/jemm-api/img.dis 第 2600–2638 行（img 5180–51ED）。
var jemmCode = []byte{
	0xEA, 0x87, 0x51, 0x70, 0x00, 0x90, 0x90, 0xB8, 0x78, 0x00, 0x8E, 0xD8, 0x8E, 0xC0, 0x8E, 0xE0,
	0x8E, 0xE8, 0x8E, 0xD0, 0xFA, 0x0F, 0x20, 0xC0, 0x66, 0x25, 0xFE, 0xFF, 0xFF, 0x7F, 0x0F, 0x22,
	0xC0, 0xEB, 0x00, 0x0F, 0x01, 0x1E, 0x18, 0x3E, 0x8B, 0xDC, 0x8B, 0x57, 0x34, 0x8E, 0xC2, 0x8B,
	0x57, 0x3C, 0x8E, 0xE2, 0x8B, 0x57, 0x40, 0x8E, 0xEA, 0x8B, 0x57, 0x30, 0x8E, 0xD2, 0x8B, 0x67,
	0x2C, 0xFF, 0x77, 0x28, 0xFF, 0x77, 0x24, 0xFF, 0x77, 0x20, 0x66, 0xFF, 0x77, 0x10, 0xFF, 0x77,
	0x38, 0x66, 0xB8, 0x00, 0x00, 0x00, 0x00, 0x66, 0x8B, 0x4F, 0x18, 0x66, 0x8B, 0x57, 0x14, 0x66,
	0x8B, 0x6F, 0x08, 0x66, 0x8B, 0x77, 0x04, 0x66, 0x8B, 0x3F, 0x1F, 0x66, 0x5B, 0xCF,
}

// jemmFrame：[bx+0]–[bx+1Fh] 是 pushad 區（EDI、ESI、EBP、ESP、EBX、EDX、ECX、EAX），[bx+20h] 起是 V86 框架
// （EIP、CS、EFLAGS、ESP、SS、ES、DS、FS、GS）。各欄位互不相同；CS 的 RPL 非 0；FLAGS 的 IF＝1。
var jemmFrame = [17]uint32{
	0x11110001, 0x22220002, 0x33330003, 0x44440004, 0x55550005, 0x66660006, 0x77770007, 0x88880008,
	0x0040, 0x0123, 0x00020247, 0x0400, 0x2000, 0x3000, 0x4000, 0x5000, 0x6000,
}

// jemmMachine：profile 開、A20 開；GDT 在 X+1400h（70h 碼段、78h 資料段，基底 X、界限 FFFFh、16 位元）；
// 以分段模式（CS=70h、其餘段的快取基底 X）停在 X+5180h。CR0 帶 MP、TS（清 PE 之後要保留）。
func jemmMachine(t *testing.T, pg bool) *Machine {
	t.Helper()
	m := newSysProg(t, []byte{0x90})
	m.SetA20(true)
	cr0 := uint32(cr0PE | 0x2 | 0x8)
	if pg {
		tb := newTables(m, 0x40000)
		tb.identity(0, 0x200000, fPWU)
		mustNil(t, "SetCR3", m.SetCR3(0x40000))
		cr0 |= cr0PG
	}
	mustNil(t, "SetCR0", m.SetCR0(cr0))
	mustNil(t, "enterSegmented", m.enterSegmented())
	const X = jemmX
	putDescPhys(m, X+0x1400, 0x70, X, 0xFFFF, 0x9A, 0)
	putDescPhys(m, X+0x1400, 0x78, X, 0xFFFF, 0x92, 0)
	m.SetGDTR(X+0x1400, 0x87)
	m.SetIDTR(X+0x1530, 0x7FF)
	c := m.CPU
	c.Seg[cpu.CS], c.IP = 0x70, 0x5180
	c.SetSegCache(cpu.CS, SegCache{Base: X, Limit: 0xFFFF, Attr: 0x9A})
	c.Seg[cpu.ES], c.Seg[cpu.SS], c.Seg[cpu.DS], c.FSeg, c.GSeg = 0x80, 0x88, 0x80, 0x80, 0x80
	for _, i := range []int{cpu.ES, cpu.SS, cpu.DS, cpu.FS, cpu.GS} {
		c.SetSegCache(i, SegCache{Base: X, Limit: 0xFFFF, Attr: 0x92})
	}
	c.R[cpu.SP], c.Hi[cpu.SP] = jemmSP, 0
	c.SetFlags(0x0002)
	pokePhys(m, X+0x5180, jemmCode...)
	pokePhys(m, X+0x3E18, 0xFF, 0x03, 0, 0, 0, 0) // 41ED 存的 IDTR（3FFh, 0）
	pokePhys(m, 0x3E18, 0x34, 0x12, 0xEF, 0xCD, 0xAB, 0x00)
	for a := uint32(0x15180); a < 0x15200; a++ {
		m.Mem[a] = 0xF4 // 線性位址被遮成 20 位元時會取到 hlt
	}
	for k, v := range jemmFrame {
		var w [4]byte
		binary.LittleEndian.PutUint32(w[:], v)
		pokePhys(m, X+jemmSP+uint32(4*k), w[:]...)
	}
	pokePhys(m, jemmRetLin, 0xEB, 0xFE)       // jmp $
	pokePhys(m, 0x20, 0x00, 0x00, 0x00, 0x80) // IVT 第 8 項 → 8000:0000
	pokePhys(m, jemmIRQ8, 0x90)
	m.IRQ0Every = 0
	return m
}

// stepJEMMTo 單步到 IP＝ip（子狀態期間 CS 不是實模式值，以 IP 判斷位置）。
func stepJEMMTo(t *testing.T, m *Machine, ip uint16) {
	t.Helper()
	for n := 0; m.CPU.IP != ip; n++ {
		if n > 64 {
			t.Fatalf("走不到 IP=%04X（停在 %04X:%04X）", ip, m.CPU.Seg[cpu.CS], m.CPU.IP)
		}
		if !m.CPU.Segmented() {
			t.Fatalf("到 IP=%04X 之前就離開分段模式（%04X:%04X）", ip, m.CPU.Seg[cpu.CS], m.CPU.IP)
		}
		mustStep(t, m, "JEMM 序列")
	}
}

// runJEMM 跑完 5180–51ED，逐點檢查 §3 第 3 項的期望。
func runJEMM(t *testing.T, m *Machine, pg bool) {
	t.Helper()
	const X = jemmX
	c := m.CPU
	stepJEMMTo(t, m, 0x519E)
	if m.paging != pg || m.CR0()&cr0PE == 0 {
		t.Fatalf("519E 之前：PE=1、paging=%v", pg)
	}
	mustStep(t, m, "519E mov cr0,eax")
	wantSub(t, m, "519E 之後", true)
	if m.CR0() != 0x1A || m.paging || m.PCLinear() != X+0x51A1 || c.IP != 0x51A1 {
		t.Errorf("519E 之後：CR0=%08X（應 1Ah：ET、MP、TS 保留）paging=%v PCLinear=%X", m.CR0(), m.paging, m.PCLinear())
	}
	mustStep(t, m, "51A1 jmp short")
	if c.IP != 0x51A3 {
		t.Fatalf("51A1 的 EB 00 應取自 X+51A1h：IP=%04X", c.IP)
	}
	mustStep(t, m, "51A3 lidt")
	if b, l := m.IDTR(); b != 0 || l != 0x3FF {
		t.Errorf("51A3 之後 IDTR=(%08X, %04X)，應為（0, 3FFh）：lidt 經 DS 快取讀 X+3E18h", b, l)
	}
	stepJEMMTo(t, m, 0x51BE)
	if ss := c.SegCache(cpu.SS); c.Seg[cpu.SS] != 0x2000 || ss.Base != 0x20000 || ss.Limit != 0xFFFF || ss.Attr&0x4000 != 0 {
		t.Errorf("51BC 之後：SS=%04X 快取 %+v", c.Seg[cpu.SS], ss)
	}
	wantSub(t, m, "51BC 之後", true)
	stepJEMMTo(t, m, 0x51D1)
	push := []uint16{0x4000, 0x0005, 0x5555, 0x0040, 0x0123, 0x0247} // DS、EBX 低、EBX 高、IP、CS、FLAGS
	for k, want := range push {
		if got := peek16(m, 0x20000+0x3F4+uint32(2*k)); got != want {
			t.Errorf("51C1–51CE 的推入：新 SS×16＋%03X = %04X，應為 %04X", 0x3F4+2*k, got, want)
		}
	}
	stepJEMMTo(t, m, 0x51EB)
	if c.Seg[cpu.DS] != 0x4000 || c.SegCache(cpu.DS).Base != 0x40000 {
		t.Errorf("51EA 之後 DS=%04X 基底 %X", c.Seg[cpu.DS], c.SegCache(cpu.DS).Base)
	}
	wantSub(t, m, "51EA 之後", true)
	stepJEMMTo(t, m, 0x51ED)
	mustStep(t, m, "51ED iret")
	wantSub(t, m, "51ED 之後（同一步離開）", false)
	wantCachesZero(t, m, "51ED")
	if m.Translating() {
		t.Error("離開後 Translating() 應為 false")
	}
	want := map[string][2]uint32{
		"CS": {uint32(c.Seg[cpu.CS]), 0x0123}, "IP": {uint32(c.IP), 0x0040}, "FLAGS": {uint32(c.Flags), 0x0247},
		"SS": {uint32(c.Seg[cpu.SS]), 0x2000}, "SP": {uint32(c.R[cpu.SP]), 0x0400}, "ES": {uint32(c.Seg[cpu.ES]), 0x3000},
		"DS": {uint32(c.Seg[cpu.DS]), 0x4000}, "FS": {uint32(c.FSeg), 0x5000}, "GS": {uint32(c.GSeg), 0x6000},
		"EAX": {reg32(c, cpu.AX), 0}, "EBX": {reg32(c, cpu.BX), 0x55550005}, "ECX": {reg32(c, cpu.CX), 0x77770007},
		"EDX": {reg32(c, cpu.DX), 0x66660006}, "EBP": {reg32(c, cpu.BP), 0x33330003}, "ESI": {reg32(c, cpu.SI), 0x22220002},
		"EDI": {reg32(c, cpu.DI), 0x11110001},
	}
	for name, v := range want {
		if v[0] != v[1] {
			t.Errorf("51ED 之後 %s=%X，應為 %X", name, v[0], v[1])
		}
	}
}

func reg32(c *cpu.CPU, r int) uint32 { return uint32(c.Hi[r])<<16 | uint32(c.R[r]) }

func TestRC442JEMMSequence(t *testing.T) {
	for _, pg := range []bool{false, true} {
		m := jemmMachine(t, pg)
		runJEMM(t, m, pg)
	}
}

// 離開後下一步：預先排入的 IRQ0（子狀態期間 irq0Pending 保持、Ticks 不增）經 IVT 遞送。
func TestRC442JEMMIRQ0AfterLeave(t *testing.T) {
	m := jemmMachine(t, false)
	m.irq0Pending, m.picMask = true, 0
	stepJEMMTo(t, m, 0x51ED)
	if !m.irq0Pending || m.Ticks != 0 {
		t.Fatalf("子狀態期間 IRQ0 應保持待處理：pending=%v Ticks=%d", m.irq0Pending, m.Ticks)
	}
	mustStep(t, m, "51ED iret")
	wantSub(t, m, "51ED 之後", false)
	mustStep(t, m, "離開後下一步")
	c := m.CPU
	if m.irq0Pending || m.Ticks != 1 || c.Seg[cpu.CS] != 0x8000 || c.IP != 1 {
		t.Errorf("IRQ0 應經 IVT 遞送：pending=%v Ticks=%d CS:IP=%04X:%04X", m.irq0Pending, m.Ticks, c.Seg[cpu.CS], c.IP)
	}
	for k, want := range []uint16{0x0040, 0x0123, 0x0247} {
		if got := peek16(m, 0x20000+0x3FA+uint32(2*k)); got != want {
			t.Errorf("IRQ0 推入的框架 +%d = %04X，應為 %04X", 2*k, got, want)
		}
	}
}

// 離開後以 IntHook 攔截的 INT 21h 照常執行。
func TestRC442JEMMInt21AfterLeave(t *testing.T) {
	m := jemmMachine(t, false)
	pokePhys(m, jemmRetLin, 0xCD, 0x21)
	hooked := 0
	m.CPU.IntHook = func(_ *cpu.CPU, n uint8) bool {
		if n == 0x21 {
			hooked++
			return true
		}
		return false
	}
	stepJEMMTo(t, m, 0x51ED)
	mustStep(t, m, "51ED iret")
	wantSub(t, m, "51ED 之後", false)
	mustStep(t, m, "INT 21h")
	if hooked != 1 || m.CPU.IP != 0x0042 {
		t.Errorf("INT 21h：IntHook %d 次、IP=%04X", hooked, m.CPU.IP)
	}
}

// 51ED 前 FLAGS 的 NT＝1：仍走實模式 iret（不回 ErrTaskReturn）。
func TestRC442JEMMIretWithNT(t *testing.T) {
	m := jemmMachine(t, false)
	stepJEMMTo(t, m, 0x51ED)
	m.CPU.SetFlags(m.CPU.Flags | cpu.NT)
	mustStep(t, m, "NT=1 的 iret")
	wantSub(t, m, "51ED 之後", false)
	if m.CPU.Seg[cpu.CS] != 0x0123 || m.CPU.IP != 0x0040 || m.CPU.Flags != 0x0247 {
		t.Errorf("CS:IP=%04X:%04X FLAGS=%04X", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Flags)
	}
}

// ---- §3 第 4 項：進入與離開時機 -------------------------------------------------

// 分段模式的快取恰好全是實模式推得值（自建 GDT，描述元基底＝選擇子×16、界限 FFFFh）時，清 PE 的那一步就離開。
func TestRC442LeaveOnEntryStep(t *testing.T) {
	code := []byte{0xB8, 0x28, 0x00, 0x8E, 0xD8, 0x8E, 0xC0, 0x8E, 0xD0, 0x8E, 0xE0, 0x8E, 0xE8, // mov ax,28h；ds、es、ss、fs、gs
		0xEA, 0x12, 0x0E, 0x30, 0x00, // jmp 30h:0E12（＝線性 1112h）
		0x0F, 0x22, 0xC3, 0x90} // mov cr0,ebx；nop
	m := segProg(t, code)
	putDesc(m, segGDT, 0x28, 0x280, 0xFFFF, 0x92, 0)
	putDesc(m, segGDT, 0x30, 0x300, 0xFFFF, 0x9A, 0)
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	for i := 0; i < 7; i++ {
		mustStep(t, m, "分段模式載入")
	}
	if m.CPU.Seg[cpu.CS] != 0x30 || m.CPU.SegCache(cpu.CS).Base != 0x300 || !m.CPU.Segmented() {
		t.Fatalf("前提：CS=30h、基底 300h（CS=%04X）", m.CPU.Seg[cpu.CS])
	}
	mustStep(t, m, "mov cr0 清 PE")
	wantSub(t, m, "清 PE 的那一步", false)
	wantCachesZero(t, m, "清 PE 的那一步")
	if m.CR0() != pmCR0Init || m.CPU.IP != 0x0E15 {
		t.Errorf("CR0=%08X IP=%04X", m.CR0(), m.CPU.IP)
	}
}

// FS、GS 各一例：只有它沒有重載成實模式推得值時不離開，重載後同一步離開。
func TestRC442FSGSDelayLeave(t *testing.T) {
	for _, r := range []struct {
		name     string
		ld, ldCX byte
	}{{"FS", 0xE0, 0xE1}, {"GS", 0xE8, 0xE9}} {
		code := []byte{0xB8, 0x10, 0x00, 0x8E, r.ld, // mov ax,10h；mov fs／gs,ax（基底 1000h ≠ 10h×16）
			0x0F, 0x22, 0xC3, // mov cr0,ebx
			0x8E, 0xDA, // mov ds,dx（DS 重載成同值）
			0x8E, r.ldCX} // mov fs／gs,cx（CX＝100h，基底 1000h）
		m := segProg(t, code)
		m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
		m.CPU.R[cpu.DX], m.CPU.R[cpu.CX] = m.CPU.Seg[cpu.DS], 0x0100
		mustStep(t, m, "mov ax")
		mustStep(t, m, "載入 "+r.name)
		mustStep(t, m, "mov cr0")
		wantSub(t, m, r.name+" 未重載", true)
		mustStep(t, m, "mov ds,dx")
		wantSub(t, m, r.name+" 未重載（DS 重載）", true)
		mustStep(t, m, "重載 "+r.name)
		wantSub(t, m, r.name+" 重載後", false)
	}
}

// SS 在分段模式載入 B＝1 的描述元，清 PE 後不重載 SS：不離開（條件 5），堆疊操作照 434 失敗即關閉；重載 SS 後可以離開。
func TestRC442SSBigNoLeave(t *testing.T) {
	pre := []byte{0xB8, 0x38, 0x00, 0x8E, 0xD0, 0x0F, 0x22, 0xC3} // mov ax,38h；mov ss,ax；mov cr0,ebx
	for _, reload := range []bool{false, true} {
		next := []byte{0x50} // push ax
		if reload {
			next = []byte{0x8E, 0xD0} // mov ss,ax
		}
		m := segProg(t, append(append([]byte{}, pre...), next...))
		putDesc(m, segGDT, 0x38, 0x380, 0xFFFF, 0x92, 0x4) // B=1
		m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
		for i := 0; i < 3; i++ {
			mustStep(t, m, "前置")
		}
		wantSub(t, m, "SS 的 B=1", true)
		if !reload {
			wantStepErr(t, m, "B=1 的堆疊操作", "B=1")
			continue
		}
		mustStep(t, m, "重載 SS")
		wantSub(t, m, "重載 SS 後", false)
	}
}

// 段載入成功但同一步稍後失敗時不離開、不回哨兵。
func TestRC442LateFaultNoLeave(t *testing.T) {
	// call far 的推入超出 SS 界限：SS 在分段模式載入界限 0FFFh 的描述元、子狀態重載、SP=0002h。
	m := segProg(t, []byte{0xB8, 0x40, 0x00, 0x8E, 0xD0, 0x0F, 0x22, 0xC3, // mov ax,40h；mov ss,ax；mov cr0,ebx
		0x8E, 0xD2, 0xBC, 0x02, 0x00, // mov ss,dx；mov sp,2
		0x9A, 0x00, 0x00, 0x00, 0x05}) // call 0500:0000
	putDesc(m, segGDT, 0x40, 0x400, 0x0FFF, 0x92, 0)
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	m.CPU.R[cpu.DX] = 0x2000
	for i := 0; i < 5; i++ {
		mustStep(t, m, "前置")
	}
	if ss := m.CPU.SegCache(cpu.SS); ss.Base != 0x20000 || ss.Limit != 0x0FFF {
		t.Fatalf("前提：子狀態重載 SS 保留界限 0FFFh：%+v", ss)
	}
	err := m.Step()
	if err == nil || errors.Is(err, cpu.ErrLeaveSegmented) || !strings.Contains(err.Error(), "界限") {
		t.Errorf("call far 的推入超出 SS 界限應失敗即關閉（不回哨兵），實際 %v", err)
	}
	wantSub(t, m, "call far 失敗後", true)

	// les 讀遠指標的第二個 word 超出 DS 界限：loadSeg 不寫。
	m = segProg(t, []byte{0xB8, 0x40, 0x00, 0x8E, 0xD8, 0x0F, 0x22, 0xC3, // mov ax,40h；mov ds,ax；mov cr0,ebx
		0xC4, 0x1E, 0xFE, 0x0F}) // les bx,[0FFEh]
	putDesc(m, segGDT, 0x40, 0x400, 0x0FFF, 0x92, 0)
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	for i := 0; i < 3; i++ {
		mustStep(t, m, "前置")
	}
	es, esc := m.CPU.Seg[cpu.ES], m.CPU.SegCache(cpu.ES)
	if err := m.Step(); err == nil || errors.Is(err, cpu.ErrLeaveSegmented) {
		t.Errorf("les 超出 DS 界限應失敗即關閉，實際 %v", err)
	}
	if m.CPU.Seg[cpu.ES] != es || m.CPU.SegCache(cpu.ES) != esc {
		t.Error("les 失敗時 ES 與快取不得寫入")
	}
	wantSub(t, m, "les 失敗後", true)
}

// pop ds、mov ds,[m] 讀取失敗時 loadSeg 不寫段值與快取（§2.4 第 1 步）。
func TestRC442LoadReadFault(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"pop ds", []byte{0xB8, 0x40, 0x00, 0x8E, 0xD0, 0x0F, 0x22, 0xC3, 0xBC, 0x00, 0x10, 0x1F}},     // ss=40h；sp=1000h；pop ds
		{"mov ds,[m]", []byte{0xB8, 0x40, 0x00, 0x8E, 0xD8, 0x0F, 0x22, 0xC3, 0x8E, 0x1E, 0x00, 0x10}}, // ds=40h；mov ds,[1000h]
	} {
		m := segProg(t, c.code)
		putDesc(m, segGDT, 0x40, 0x400, 0x0FFF, 0x92, 0)
		m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
		steps := 3
		if c.name == "pop ds" {
			steps = 4
		}
		for i := 0; i < steps; i++ {
			mustStep(t, m, c.name+" 前置")
		}
		ds, dsc := m.CPU.Seg[cpu.DS], m.CPU.SegCache(cpu.DS)
		if err := m.Step(); err == nil {
			t.Errorf("%s：讀取超出界限應失敗即關閉", c.name)
		}
		if m.CPU.Seg[cpu.DS] != ds || m.CPU.SegCache(cpu.DS) != dsc {
			t.Errorf("%s：DS（%04X→%04X）與快取不得寫入", c.name, ds, m.CPU.Seg[cpu.DS])
		}
	}
}

// unreal：分段模式下 DS 載入基底 0、界限 FFFFFh（G=1）的描述元，清 PE 後 mov ds 載入 0，67 定址讀 [ebx] 取得
// 線性 123456h 的值；不離開子狀態；接著 INT 21h 失敗即關閉，診斷含「442」。
func TestRC442Unreal(t *testing.T) {
	m := segProg(t, []byte{0xB8, 0x48, 0x00, 0x8E, 0xD8, 0x0F, 0x22, 0xC3, // mov ax,48h；mov ds,ax；mov cr0,ebx
		0x8E, 0xD9, // mov ds,cx（CX=0）
		0x66, 0xBB, 0x56, 0x34, 0x12, 0x00, // mov ebx,00123456h
		0x67, 0x8A, 0x03, // mov al,[ebx]
		0xCD, 0x21})
	putDesc(m, segGDT, 0x48, 0, 0xFFFFF, 0x92, 0x8)
	m.SetA20(true)
	pokePhys(m, 0x123456, 0x5A)
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	m.CPU.R[cpu.CX] = 0
	hooked := false
	m.CPU.IntHook = func(*cpu.CPU, uint8) bool { hooked = true; return true }
	for i := 0; i < 4; i++ {
		mustStep(t, m, "前置")
	}
	if ds := m.CPU.SegCache(cpu.DS); ds.Base != 0 || ds.Limit != 0xFFFFFFFF {
		t.Fatalf("mov ds,cx 之後 DS 快取 %+v（基底 0、界限保留）", ds)
	}
	mustStep(t, m, "mov ebx")
	mustStep(t, m, "mov al,[ebx]")
	if uint8(m.CPU.R[cpu.AX]) != 0x5A {
		t.Errorf("67 定址應讀到線性 123456h 的 5Ah，實際 %02X", uint8(m.CPU.R[cpu.AX]))
	}
	wantSub(t, m, "unreal", true)
	wantStepErr(t, m, "unreal 下的 INT 21h", "442")
	if hooked {
		t.Error("IntHook 不得被呼叫")
	}
}

// §2.5 表中每個段載入位置在子狀態各觸發一次：基底＝值×16、界限與屬性保留、CS／SS 的 D/B 清除、不讀描述元表
// （GDTR 界限設 0，經描述元表的載入必定失敗）。
func TestRC442LoadSites(t *testing.T) {
	const val = 0x0200
	push := []byte{0x68, 0x00, 0x02}
	push32 := []byte{0x66, 0x68, 0x00, 0x02, 0x00, 0x00}
	far := func(op ...byte) []byte { return append(op, 0x00, 0x02) } // [0200] 存 0200:0000
	sites := []struct {
		name  string
		reg   int
		pre   []byte
		steps int
		code  []byte
		db    bool // 預設快取帶 D/B（會經堆疊彈出 SS 的位置不帶，否則 434 的 B=1 堆疊失敗即關閉）
		// CS 的列不帶 D：子狀態下 CS 的 D=1 不可達（443 §2.9，SetCR0 擋下），注入的話 Step 依 443 §2.1 改以 32 位元解碼；
		// 「實模式載入清 CS 的 D」改在 internal/cpu 的 TestRealCacheLoadSegClearsCSD 直接驗。
	}{
		{"pop es", cpu.ES, push, 1, []byte{0x07}, true},
		{"pop ss", cpu.SS, push, 1, []byte{0x17}, false},
		{"pop ds", cpu.DS, push, 1, []byte{0x1F}, true},
		{"mov es,ax", cpu.ES, nil, 0, []byte{0x8E, 0xC0}, true},
		{"mov ss,ax", cpu.SS, nil, 0, []byte{0x8E, 0xD0}, true},
		{"mov ds,ax", cpu.DS, nil, 0, []byte{0x8E, 0xD8}, true},
		{"mov fs,ax", cpu.FS, nil, 0, []byte{0x8E, 0xE0}, true},
		{"mov gs,ax", cpu.GS, nil, 0, []byte{0x8E, 0xE8}, true},
		{"call far ptr（9A）", cpu.CS, nil, 0, []byte{0x9A, 0x00, 0x00, 0x00, 0x02}, false},
		{"jmp far ptr（EA）", cpu.CS, nil, 0, []byte{0xEA, 0x00, 0x00, 0x00, 0x02}, false},
		{"call far [m]（FF /3）", cpu.CS, nil, 0, far(0xFF, 0x1E), false},
		{"jmp far [m]（FF /5）", cpu.CS, nil, 0, far(0xFF, 0x2E), false},
		{"les", cpu.ES, nil, 0, far(0xC4, 0x1E), true},
		{"lds", cpu.DS, nil, 0, far(0xC5, 0x1E), true},
		{"retf imm16", cpu.CS, append(append([]byte{}, push...), 0x6A, 0x00), 2, []byte{0xCA, 0x02, 0x00}, false},
		{"retf", cpu.CS, append(append([]byte{}, push...), 0x6A, 0x00), 2, []byte{0xCB}, false},
		{"pop fs", cpu.FS, push, 1, []byte{0x0F, 0xA1}, true},
		{"pop gs", cpu.GS, push, 1, []byte{0x0F, 0xA9}, true},
		{"lss", cpu.SS, nil, 0, far(0x0F, 0xB2, 0x1E), true},
		{"lfs", cpu.FS, nil, 0, far(0x0F, 0xB4, 0x1E), true},
		{"lgs", cpu.GS, nil, 0, far(0x0F, 0xB5, 0x1E), true},
		{"66 pop es", cpu.ES, push32, 1, []byte{0x66, 0x07}, true},
		{"66 pop ss", cpu.SS, push32, 1, []byte{0x66, 0x17}, false},
		{"66 pop ds", cpu.DS, push32, 1, []byte{0x66, 0x1F}, true},
	}
	for _, s := range sites {
		m := subProg(t, append(append([]byte{}, s.pre...), s.code...))
		m.SetGDTR(segGDT, 0)
		m.CPU.R[cpu.AX] = val
		m.CPU.R[cpu.SP] = 0x0400
		copy(m.Mem[0x1200:], []byte{0x00, 0x00, 0x00, 0x02}) // DS:0200 的遠指標 0200:0000
		for i := 0; i < s.steps; i++ {
			mustStep(t, m, s.name+" 的前置")
		}
		attr := uint16(0x1093) // AVL、present、可寫資料段
		if s.reg == cpu.CS {
			attr = 0x109B
		}
		if s.db {
			attr |= 0x4000
		}
		cur := m.CPU.SegCache(s.reg)
		m.CPU.SetSegCache(s.reg, SegCache{Base: cur.Base, Limit: 0x7FFF, Attr: attr})
		if err := m.Step(); err != nil {
			t.Errorf("%s：子狀態的段載入應成功，實際 %v", s.name, err)
			continue
		}
		wantAttr := attr
		if s.reg == cpu.CS || s.reg == cpu.SS {
			wantAttr &^= 0x4000
		}
		want := SegCache{Base: val << 4, Limit: 0x7FFF, Attr: wantAttr}
		if got := m.CPU.SegCache(s.reg); segValOf(m.CPU, s.reg) != val || got != want {
			t.Errorf("%s：%s=%04X 快取 %+v，應為 %04X、%+v", s.name, segNameT[s.reg], segValOf(m.CPU, s.reg), got, val, want)
		}
		wantSub(t, m, s.name+"（界限 7FFFh，不離開）", true)
	}
}

// 目標快取 P＝0（分段模式下 FS 載入空選擇子）時，子狀態 mov fs 設成實模式預設快取，經 FS 讀寫成功，可參與離開。
func TestRC442NullCacheDefault(t *testing.T) {
	m := segProg(t, []byte{0x31, 0xC0, 0x8E, 0xE0, // xor ax,ax；mov fs,ax（空選擇子）
		0xB8, 0x10, 0x00, 0x8E, 0xE8, // mov ax,10h；mov gs,ax（基底 1000h ≠ 10h×16，不是推得值）
		0x0F, 0x22, 0xC3, // mov cr0,ebx
		0x8E, 0xE1, // mov fs,cx
		0x64, 0x88, 0x07, // mov fs:[bx],al
		0x64, 0x8A, 0x27, // mov ah,fs:[bx]
		0x8E, 0xEA}) // mov gs,dx（DX=0）
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0x0300, 0
	for i := 0; i < 5; i++ {
		mustStep(t, m, "前置")
	}
	if m.CPU.SegCache(cpu.FS).Attr&0x80 != 0 {
		t.Fatal("前提：FS 快取不可用")
	}
	wantSub(t, m, "清 PE 之後", true)
	mustStep(t, m, "mov fs,cx")
	if got, want := m.CPU.SegCache(cpu.FS), (SegCache{Base: 0x3000, Limit: 0xFFFF, Attr: 0x0093}); got != want {
		t.Errorf("FS 快取 %+v，應為 %+v", got, want)
	}
	mustStep(t, m, "mov fs:[bx],al")
	mustStep(t, m, "mov ah,fs:[bx]")
	if m.Mem[0x3010] != 0x10 || m.CPU.R[cpu.AX]>>8 != 0x10 {
		t.Errorf("經 FS 讀寫：[3010]=%02X AH=%02X", m.Mem[0x3010], m.CPU.R[cpu.AX]>>8)
	}
	wantSub(t, m, "GS 未重載", true)
	mustStep(t, m, "mov gs,dx")
	wantSub(t, m, "GS 重載後（FS 參與離開）", false)
}

// retf 彈出 RPL 非 0 的 CS 在子狀態成功。
func TestRC442RetfRPL(t *testing.T) {
	m := subProg(t, []byte{0x68, 0x03, 0x01, 0x6A, 0x00, 0xCB}) // push 0103h；push 0；retf
	mustStep(t, m, "push 0103h")
	mustStep(t, m, "push 0")
	mustStep(t, m, "retf 到 RPL 3")
	if m.CPU.Seg[cpu.CS] != 0x0103 || m.CPU.IP != 0 {
		t.Errorf("CS:IP=%04X:%04X", m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
}

// 子狀態內 clts、只改 MP 或 TS 的 mov cr0 成功、不離開子狀態。
func TestRC442CR0WritesInSubstate(t *testing.T) {
	m := subProg(t, []byte{0x0F, 0x06, 0x0F, 0x22, 0xC3, 0x0F, 0x22, 0xC1}) // clts；mov cr0,ebx；mov cr0,ecx
	mustNil(t, "子狀態 SetCR0 設 TS", m.SetCR0(pmCR0Init|0x8))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x12, 0 // MP
	m.CPU.R[cpu.CX], m.CPU.Hi[cpu.CX] = 0x18, 0 // TS
	for _, want := range []uint32{0x10, 0x12, 0x18} {
		mustStep(t, m, "CR0 寫入")
		if m.CR0() != want {
			t.Errorf("CR0=%08X，應為 %08X", m.CR0(), want)
		}
		wantSub(t, m, "CR0 寫入後", true)
	}
}

// ---- §3 第 5 項：A20 關閉 ------------------------------------------------------

func TestRC442A20Off(t *testing.T) {
	m := subProg(t, []byte{0x8A, 0x07, 0x8E, 0xDA, 0x8E, 0xEA}) // mov al,[bx]；mov ds,dx；mov gs,cx
	m.CPU.Seg[cpu.DS] = 0x1234
	m.CPU.SetSegCache(cpu.DS, dataCache(0x110000, 0xFFFF, 0x93))
	m.CPU.R[cpu.BX], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX] = 0, 0x0100, 0
	pokePhys(m, 0x110000, 0xAA)
	pokePhys(m, 0x010000, 0x55)
	m.SetA20(false)
	if m.CPU.Code != nil || !m.Translating() {
		t.Fatal("A20 關且 xlat 開：Code 應為 nil")
	}
	mustStep(t, m, "mov al,[bx]")
	if uint8(m.CPU.R[cpu.AX]) != 0x55 {
		t.Errorf("A20 關：基底 110000h 的存取應落在 010000h（讀到 %02X）", uint8(m.CPU.R[cpu.AX]))
	}
	mustStep(t, m, "mov ds,dx")
	wantSub(t, m, "GS 未重載", true)
	mustStep(t, m, "mov gs,cx")
	wantSub(t, m, "離開", false)
	if m.Translating() || m.A20Enabled() || len(m.CPU.Code) == 0 || &m.CPU.Code[0] != &m.Mem[0] {
		t.Error("離開後 xlat=false、A20 關：Code 應恢復為 m.Mem")
	}
}

// ---- §3 第 6 項：失敗即關閉 -----------------------------------------------------

func TestRC442FailClosed(t *testing.T) {
	far32 := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x02}
	cases := []struct {
		name, want string
		code       []byte
		setup      func(m *Machine)
	}{
		{"int 80h", "442", []byte{0xCD, 0x80}, nil},
		{"int 21h（IntHook）", "442", []byte{0xCD, 0x21}, nil},
		{"int3", "442", []byte{0xCC}, nil},
		{"into（OF=1）", "442", []byte{0xCE}, func(m *Machine) { m.CPU.SetFlags(m.CPU.Flags | cpu.OF) }},
		{"16 位元 div 0", "442", []byte{0xF7, 0xF3}, func(m *Machine) { m.CPU.R[cpu.BX] = 0 }},
		{"32 位元 div 0", "442", []byte{0x66, 0xF7, 0xF3}, func(m *Machine) { m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0, 0 }},
		{"BOUND 越界", "442", []byte{0x62, 0x06, 0x00, 0x02}, func(m *Machine) {
			m.CPU.R[cpu.AX] = 5
			copy(m.Mem[0x1200:], []byte{0x10, 0x00, 0x20, 0x00})
		}},
		{"hlt", "HLT", []byte{0xF4}, nil},
		{"66 CF", "442", []byte{0x66, 0xCF}, nil},
		{"66 EA", "442", append([]byte{0x66, 0xEA}, far32...), nil},
		{"66 9A", "442", append([]byte{0x66, 0x9A}, far32...), nil},
		{"66 CA", "442", []byte{0x66, 0xCA, 0x00, 0x00}, nil},
		{"66 CB", "442", []byte{0x66, 0xCB}, nil},
		{"66 FF /3", "442", []byte{0x66, 0xFF, 0x1E, 0x00, 0x02}, nil},
		{"66 FF /5", "442", []byte{0x66, 0xFF, 0x2E, 0x00, 0x02}, nil},
		{"0F 00 /0 SLDT", "實模式", []byte{0x0F, 0x00, 0xC0}, nil},
		{"0F 00 /1 STR", "實模式", []byte{0x0F, 0x00, 0xC8}, nil},
		{"0F 00 /2 LLDT", "實模式", []byte{0x0F, 0x00, 0xD0}, nil},
		{"0F 00 /3 LTR", "實模式", []byte{0x0F, 0x00, 0xD8}, nil},
		{"LAR", "實模式", []byte{0x0F, 0x02, 0xC0}, nil},
		{"LSL", "實模式", []byte{0x0F, 0x03, 0xC0}, nil},
		{"mov cs", "MOV CS", []byte{0x8E, 0xC8}, nil},
		{"子狀態 mov cr0 設 PE", "PE", []byte{0x0F, 0x22, 0xC3}, func(m *Machine) { m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x11, 0 }},
		{"子狀態 lmsw 設 PE", "PE", []byte{0x0F, 0x01, 0xF0}, func(m *Machine) { m.CPU.R[cpu.AX] = 1 }},
		{"子狀態 mov cr0 設 PG", "PG", []byte{0x0F, 0x22, 0xC3}, func(m *Machine) { m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0x8000 }},
	}
	for _, c := range cases {
		m := subProg(t, c.code)
		hooked := false
		m.CPU.IntHook = func(*cpu.CPU, uint8) bool { hooked = true; return true }
		if c.setup != nil {
			c.setup(m)
		}
		before := takeCPUState(m)
		gb, gl := m.GDTR()
		ib, il := m.IDTR()
		wantStepErr(t, m, c.name, c.want)
		if after := takeCPUState(m); after != before {
			t.Errorf("%s：失敗後狀態不得改變\n before %+v\n after  %+v", c.name, before, after)
		}
		if b, l := m.GDTR(); b != gb || l != gl {
			t.Errorf("%s：GDTR 不得改變", c.name)
		}
		if b, l := m.IDTR(); b != ib || l != il {
			t.Errorf("%s：IDTR 不得改變", c.name)
		}
		if hooked {
			t.Errorf("%s：IntHook 不得被呼叫", c.name)
		}
	}
}

// 分段模式（PE=1）清 PE 而 PG＝1：mov cr0 失敗即關閉，CR0 與 segmented 不變。
func TestRC442ClearPEKeepPGFails(t *testing.T) {
	m := segProg(t, []byte{0x0F, 0x22, 0xC3})
	tb := newTables(m, 0x40000)
	tb.identity(0, 0x100000, fPWU)
	mustNil(t, "SetCR3", m.SetCR3(0x40000))
	mustNil(t, "SetCR0(PE|PG)", m.SetCR0(cr0PE|cr0PG))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0x8000
	before := takeCPUState(m)
	wantStepErr(t, m, "清 PE 而保留 PG", "PG")
	if after := takeCPUState(m); after != before || m.CR0() != cr0PE|cr0PG|pmCR0Init {
		t.Errorf("失敗後 CR0（%08X）與狀態不得改變", m.CR0())
	}
}

// 實模式 iret 的框架超出 SS 界限（SS 界限 0FFFh、SP=0FFEh）：SP、CS、IP、快取不變。
func TestRC442IretFrameLimit(t *testing.T) {
	m := subProg(t, []byte{0xCF})
	m.CPU.SetSegCache(cpu.SS, dataCache(uint32(m.CPU.Seg[cpu.SS])<<4, 0x0FFF, 0x93))
	m.CPU.R[cpu.SP] = 0x0FFE
	before := takeCPUState(m)
	cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
	err := wantStepErr(t, m, "iret 框架超出 SS 界限", "界限")
	var ce *cpu.Error
	if !errors.As(err, &ce) || ce.CS != cs || ce.IP != ip {
		t.Errorf("失敗回報應為本道起點 %04X:%04X：%v", cs, ip, err)
	}
	if after := takeCPUState(m); after != before || m.CPU.IP != ip+1 {
		t.Errorf("失敗後 SP、CS、快取不變、IP 不設成框架值（IP=%04X）", m.CPU.IP)
	}
}

// leaveProg：segProg 之後第一道 mov cr0,ebx 清 PE；segProg 的快取是實模式推得值，所以同一步離開。
func leaveProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := segProg(t, append([]byte{0x0F, 0x22, 0xC3}, code...))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	mustStep(t, m, "mov cr0 清 PE")
	wantSub(t, m, "清 PE 的那一步", false)
	return m
}

// 離開後 IDTR 基底 0、界限 1Fh 時 INT 08h 與 IRQ0 失敗即關閉、INT 07h 照常；界限改回 3FFh 後照常。
func TestRC442IDTRLimitAfterLeave(t *testing.T) {
	m := leaveProg(t, []byte{0xCD, 0x07})
	m.SetIDTR(0, 0x1F)
	mustStep(t, m, "INT 07h（界限 1Fh）")
	if m.CPU.Seg[cpu.CS] != m.Read16(7*4+2) || m.CPU.IP != m.Read16(7*4) {
		t.Errorf("INT 07h 應經 IVT：CS:IP=%04X:%04X", m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	m = leaveProg(t, []byte{0xCD, 0x08})
	m.SetIDTR(0, 0x1F)
	wantStepErr(t, m, "INT 08h（界限 1Fh）", "IDTR")
	m = leaveProg(t, []byte{0xCD, 0x08})
	m.SetIDTR(0, 0x3FF)
	mustStep(t, m, "INT 08h（界限 3FFh）")

	m = leaveProg(t, []byte{0x90, 0x90})
	m.SetIDTR(0, 0x1F)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.picMask, m.irq0Pending = 0, true
	steps := m.Steps
	if err := m.Step(); err == nil || !strings.Contains(err.Error(), "IDTR") || !strings.Contains(err.Error(), "界限 001F") {
		t.Errorf("IRQ0（界限 1Fh）應失敗即關閉並寫出界限，實際 %v", err)
	}
	if !m.irq0Pending || m.Ticks != 0 || m.Steps != steps {
		t.Error("IRQ0 不得被消耗")
	}
	m.SetIDTR(0, 0x3FF)
	mustStep(t, m, "IRQ0（界限 3FFh）")
	if m.irq0Pending || m.Ticks != 1 {
		t.Error("界限改回 3FFh 後 IRQ0 照常遞送")
	}
}

// 離開不受 IDTR 影響：IDTR 仍為保護模式 IDT 時也離開，之後 lidt 還原再 sti，IRQ0 照常遞送。
func TestRC442LeaveIgnoresIDTR(t *testing.T) {
	m := segProg(t, []byte{0x0F, 0x22, 0xC3, // mov cr0,ebx
		0x0F, 0x01, 0x1E, 0x00, 0x02, // lidt [0200]
		0xFB, 0x90, 0x90, 0x90}) // sti；nop
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x10, 0
	copy(m.Mem[0x1200:], []byte{0xFF, 0x03, 0, 0, 0, 0})
	m.SetIDTR(0x12000, 0x7FF)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	m.picMask, m.irq0Pending = 0, true
	mustStep(t, m, "mov cr0 清 PE")
	wantSub(t, m, "IDTR 仍為保護模式 IDT", false)
	if b, _ := m.IDTR(); b != 0x12000 {
		t.Fatalf("離開時 IDTR 不動：基底 %X", b)
	}
	mustStep(t, m, "lidt")
	mustStep(t, m, "sti")
	for i := 0; i < 2 && m.Ticks == 0; i++ {
		mustStep(t, m, "sti 之後")
	}
	if m.Ticks != 1 || m.irq0Pending {
		t.Errorf("lidt 還原、sti 之後 IRQ0 應照常遞送：Ticks=%d pending=%v", m.Ticks, m.irq0Pending)
	}
}

// IRQ0、按鍵、startCallback、periodic 在子狀態延後且不消耗。
func TestRC442IRQDeferred(t *testing.T) {
	m := subProg(t, []byte{0xFA, 0xEB, 0xFE}) // cli；jmp $
	m.IRQ0Every = 5
	m.nextIRQ0 = m.Steps + 5
	m.PushKey(0x1E)
	m.HoldKey(0x30, m.Steps+3, 2, false)
	m.QueueCallback(QueuedCall{Seg: 0x1234, Off: 0x10})
	m.SetPeriodicFarCall(0x2345, 0x20, 4)
	keys, timed := m.KeyQueueLen(), m.TimedKeysPending()
	for i := 0; i < 40; i++ {
		mustStep(t, m, "子狀態延後中")
	}
	wantSub(t, m, "延後中", true)
	if !m.irq0Pending || m.Ticks != 0 {
		t.Errorf("IRQ0：pending=%v Ticks=%d", m.irq0Pending, m.Ticks)
	}
	if m.KeyQueueLen() != keys || m.TimedKeysPending() != timed || m.KeyIRQs != 0 {
		t.Error("按鍵不得消耗")
	}
	if m.CallbackPending() != 1 || m.CallbacksMade() != 0 || m.PeriodicCalls() != 0 {
		t.Error("回呼不得執行")
	}
}

// 有待處理 IRQ 且 IF=1 一百萬步後失敗；IF=0 時不失敗。
func TestRC442PendingIRQWithIFFails(t *testing.T) {
	run := func(code []byte, n int) (int, error) {
		m := subProg(t, code)
		m.IRQ0Every = 100
		m.nextIRQ0 = m.Steps + 100
		for i := 0; i < n; i++ {
			if err := m.Step(); err != nil {
				return i, err
			}
		}
		return n, nil
	}
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

// 離開時 resetSegWait（§2.7 第 2 步）。
func TestRC442LeaveResetsSegWait(t *testing.T) {
	m := subProg(t, []byte{0xFB, 0x90, 0x90, 0x90, 0x8E, 0xEA}) // sti；nop×3；mov gs,dx
	m.CPU.R[cpu.DX] = 0
	m.irq0Pending, m.picMask = true, 0
	for i := 0; i < 4; i++ {
		mustStep(t, m, "子狀態")
	}
	if m.segWaitN == 0 {
		t.Fatal("前提：segWait 已計數")
	}
	mustStep(t, m, "mov gs,dx")
	wantSub(t, m, "離開", false)
	if m.segWaitN != 0 || m.segWaitAt != 0 {
		t.Errorf("離開時 segWait 應歸零：N=%d At=%d", m.segWaitN, m.segWaitAt)
	}
}

// 子狀態下 SetVM(true)、SetEFlagsHi 設 VM、SetPMCPL(3) 被拒。
func TestRC442SetterGuards(t *testing.T) {
	m := subProg(t, []byte{0x90})
	mustErr(t, "子狀態 SetVM(true)", m.SetVM(true))
	mustErr(t, "子狀態 SetEFlagsHi(VM)", m.SetEFlagsHi(eflagsHiVM))
	mustErr(t, "子狀態 SetPMCPL(3)", m.SetPMCPL(3))
	if m.VM() || m.PMCPL() != 0 {
		t.Error("被拒的寫入不得改狀態")
	}
}

// ---- §3 第 7 項：狀態檔 ---------------------------------------------------------

// runToLeavePlus 單步到離開分段模式，再多跑 n 步。
func runToLeavePlus(t *testing.T, m *Machine, n int) {
	t.Helper()
	for i := 0; m.CPU.Segmented(); i++ {
		if i > 64 {
			t.Fatal("走不到離開")
		}
		mustStep(t, m, "到離開")
	}
	for i := 0; i < n; i++ {
		mustStep(t, m, "離開後")
	}
}

// 子狀態（停在 51A3 之後）SaveState／LoadState 與 Snapshot／Restore 往返，接著跑完到離開後再多 100 步，
// 最終 SaveState 位元組與不中斷的執行相同。
func TestRC442StateRoundTrip(t *testing.T) {
	ref := jemmMachine(t, false)
	stepJEMMTo(t, ref, 0x51A8)
	wantSub(t, ref, "51A3 之後", true)
	pm := ref.encodePM()
	// 459 §5.1：版本 4（佇列空時 148 bytes），前 142 bytes 照版本 3。
	if len(pm) != 148 || binary.LittleEndian.Uint16(pm) != 4 || pm[80] != 1 || binary.LittleEndian.Uint32(pm[4:])&cr0PE != 0 {
		t.Errorf("子狀態的 PM：長度 %d、版本 %d、segmented %d、CR0 %08X", len(pm), binary.LittleEndian.Uint16(pm), pm[80],
			binary.LittleEndian.Uint32(pm[4:]))
	}
	data := saveBytes(t, ref)
	snap := ref.Snapshot()
	runToLeavePlus(t, ref, 100)
	want := saveBytes(t, ref)

	d := newPM(t)
	mustNil(t, "LoadState", d.LoadState(bytes.NewReader(data)))
	d.IRQ0Every = 0
	wantSub(t, d, "LoadState 之後", true)
	runToLeavePlus(t, d, 100)
	if !bytes.Equal(saveBytes(t, d), want) {
		t.Error("SaveState／LoadState 往返後跑完：存檔位元組與不中斷的執行不同")
	}

	r := newSysProg(t, []byte{0x90})
	r.Restore(snap)
	wantSub(t, r, "Restore 之後", true)
	runToLeavePlus(t, r, 100)
	if !bytes.Equal(saveBytes(t, r), want) {
		t.Error("Snapshot／Restore 往返後跑完：存檔位元組與不中斷的執行不同")
	}
}

// 手造版本 3：segmented=1、PE=0、PG=0 可讀；PG=1、VM=1、pmCPL≠0 各被拒。
func TestRC442PMValidationSubstate(t *testing.T) {
	src := subProg(t, []byte{0x90})
	base := decodeStateForTest(t, saveBytes(t, src))
	mut := func(f func(pm []byte)) []byte {
		s := base
		s.PM = append([]byte(nil), base.PM...)
		f(s.PM)
		return encodeStateForTest(t, s)
	}
	if base.PM[80] != 1 || base.PM[4]&1 != 0 {
		t.Fatal("前提：segmented=1、PE=0")
	}
	mustNil(t, "子狀態存檔", newPM(t).LoadState(bytes.NewReader(mut(func([]byte) {}))))
	for _, c := range []struct {
		name string
		f    func([]byte)
	}{
		{"PG=1", func(p []byte) { p[7] |= 0x80 }},
		{"VM=1", func(p []byte) { p[12] |= 2 }},
		{"pmCPL=3", func(p []byte) { p[3] = 3 }},
	} {
		if err := newPM(t).LoadState(bytes.NewReader(mut(c.f))); err == nil {
			t.Errorf("子狀態存檔 %s：應被拒", c.name)
		}
	}
}

// 手造「已滿足離開條件」的子狀態存檔讀回後仍在子狀態，下一次段載入才離開。
func TestRC442LoadedReadyStaysUntilLoad(t *testing.T) {
	m := segProg(t, []byte{0x90, 0x8E, 0xDA}) // nop；mov ds,dx
	mustNil(t, "SetCR0 清 PE", m.SetCR0(pmCR0Init))
	m.CPU.R[cpu.DX] = m.CPU.Seg[cpu.DS]
	d := newPM(t)
	mustNil(t, "LoadState", d.LoadState(bytes.NewReader(saveBytes(t, m))))
	d.IRQ0Every = 0
	wantSub(t, d, "讀回", true)
	mustStep(t, d, "nop")
	wantSub(t, d, "nop 之後", true)
	mustStep(t, d, "mov ds,dx")
	wantSub(t, d, "段載入之後", false)
}

// 離開後的存檔 segmented=0 且快取全 0，可讀回。
func TestRC442SaveAfterLeave(t *testing.T) {
	m := leaveProg(t, []byte{0x90})
	data := saveBytes(t, m)
	s := decodeStateForTest(t, data)
	if s.PM[80] != 0 {
		t.Errorf("segmented=%d", s.PM[80])
	}
	for i := 82; i < 142; i++ {
		if s.PM[i] != 0 {
			t.Fatalf("快取應全 0（位移 %d）", i)
		}
	}
	d := newPM(t)
	mustNil(t, "LoadState", d.LoadState(bytes.NewReader(data)))
	if d.CPU.Segmented() {
		t.Error("讀回後 segmented 應為 false")
	}
}
