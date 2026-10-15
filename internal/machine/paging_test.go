package machine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/432-linear-memory-and-paging §5、§9 第 2–8 項、§10 的分頁測試（C2）。
//
// 頁表全是測試裡合成的（只存在於測試）。需要執行指令的測試把 IRQ0Every 設成 0：
// 沒有掛 internal/dos 時，int 1Ch 向量指到的預設 stub 是 `CD 1C`，沒有人替它 IRET，
// 第一次計時器中斷就會無限遞迴（BenchmarkStepRealMode 同理）。

const (
	fP = pteP
	fW = pteW
	fU = pteU
	fA = pteA
	fD = pteD

	fPWU = fP | fW | fU
)

// put32／get32 直接讀寫實體位址（Mem 或 Ext），不經任何計數、監看與保護，
// 用來建頁表與檢查頁表項。
func put32(m *Machine, phys, v uint32) {
	if phys < MemSize {
		binary.LittleEndian.PutUint32(m.Mem[phys:], v)
		return
	}
	binary.LittleEndian.PutUint32(m.Ext[phys-ExtBase:], v)
}

func get32(m *Machine, phys uint32) uint32 {
	if phys < MemSize {
		return binary.LittleEndian.Uint32(m.Mem[phys:])
	}
	return binary.LittleEndian.Uint32(m.Ext[phys-ExtBase:])
}

// tables 是測試用的頁表建構器：頁目錄在 dir，頁表依需要從 dir+0x1000 往上配。
type tables struct {
	m      *Machine
	dir    uint32
	nextPT uint32
}

func newTables(m *Machine, dir uint32) *tables {
	for i := uint32(0); i < 1024; i++ {
		put32(m, dir+i*4, 0)
	}
	return &tables{m: m, dir: dir, nextPT: dir + 0x1000}
}

func (t *tables) pdeAddr(lin uint32) uint32 { return t.dir + (lin>>22)*4 }

// pt 回 lin 所在頁表的實體基底；沒有就配一張（PDE 旗標 pdeFlags）。
func (t *tables) pt(lin uint32, pdeFlags uint32) uint32 {
	pde := get32(t.m, t.pdeAddr(lin))
	if pde&fP != 0 {
		return pde & 0xFFFFF000
	}
	base := t.nextPT
	t.nextPT += 0x1000
	for i := uint32(0); i < 1024; i++ {
		put32(t.m, base+i*4, 0)
	}
	put32(t.m, t.pdeAddr(lin), base|pdeFlags)
	return base
}

func (t *tables) pteAddr(lin uint32) uint32 {
	return t.pt(lin, fPWU) + (lin>>12&0x3FF)*4
}

// mapFlags 把 lin 所在頁映到 phys，PDE 旗標只在建立頁表時使用。
func (t *tables) mapFlags(lin, phys, pdeFlags, pteFlags uint32) {
	base := t.pt(lin, pdeFlags)
	put32(t.m, base+(lin>>12&0x3FF)*4, phys&0xFFFFF000|pteFlags)
}

func (t *tables) mapPage(lin, phys uint32) { t.mapFlags(lin, phys, fPWU, fPWU) }

func (t *tables) identity(lo, hi uint32, flags uint32) {
	for a := lo; a < hi; a += 0x1000 {
		t.mapFlags(a, a, fPWU, flags)
	}
}

func (t *tables) setPTE(lin, v uint32)  { put32(t.m, t.pteAddr(lin), v) }
func (t *tables) pte(lin uint32) uint32 { return get32(t.m, t.pteAddr(lin)) }
func (t *tables) pde(lin uint32) uint32 { return get32(t.m, t.pdeAddr(lin)) }

// enable 設 CR3 並開分頁（PE|PG）。
func (t *tables) enable(tb testing.TB) {
	tb.Helper()
	if err := t.m.SetCR3(t.dir); err != nil {
		tb.Fatal(err)
	}
	if err := t.m.SetCR0(cr0PE | cr0PG); err != nil {
		tb.Fatal(err)
	}
}

// newPaged 造一台 profile 開、A20 開的機器（4 MB 延伸記憶體），頁目錄在 0x40000，尚未開分頁。
func newPaged(tb testing.TB) (*Machine, *tables) {
	tb.Helper()
	m := New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		tb.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		tb.Fatal(err)
	}
	m.SetA20(true)
	m.IRQ0Every = 0
	return m, newTables(m, 0x40000)
}

// newPagedProg 造一台載入了 COM 程式、低 1 MB 恆等映射（旗標 flags）的分頁機器。
func newPagedProg(tb testing.TB, code []byte, flags uint32) (*Machine, *tables) {
	tb.Helper()
	m := New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		tb.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		tb.Fatal(err)
	}
	if err := m.LoadCOM(code); err != nil {
		tb.Fatal(err)
	}
	m.SetA20(true)
	m.IRQ0Every = 0
	t := newTables(m, 0x70000)
	t.identity(0, 0x100000, flags)
	t.enable(tb)
	return m, t
}

func wantFault(t *testing.T, m *Machine, what string, lin uint32, write, user bool) PageFaultRecord {
	t.Helper()
	rec, ok := m.PageFault()
	if !ok {
		t.Fatalf("%s：應有分頁錯誤紀錄", what)
	}
	if rec.Linear != lin || rec.Write != write || rec.User != user {
		t.Errorf("%s：紀錄 %+v，應為 Linear=%X Write=%v User=%v", what, rec, lin, write, user)
	}
	return rec
}

func noFault(t *testing.T, m *Machine, what string) {
	t.Helper()
	if rec, ok := m.PageFault(); ok {
		t.Errorf("%s：不該有分頁錯誤紀錄：%+v", what, rec)
	}
}

// ---- §9 第 4 項：JEMM 映射 ----------------------------------------------------

func TestPagingJEMMMappingState1(t *testing.T) {
	m, tb := newPaged(t)
	// 狀態 1（`strike_commander` docs/re/003-jemm-api.md §3）：
	// B0000–B7FFF、C0000–FFFFF 全指實體 B8000；HMA 100000–10FFFF 指實體 A0000–AFFFF。
	tb.identity(0, 0xB0000, fPWU)
	for a := uint32(0xB0000); a < 0xB8000; a += 0x1000 {
		tb.mapPage(a, 0xB8000)
	}
	tb.mapPage(0xB8000, 0xB8000)
	tb.mapPage(0xB9000, 0xB9000)
	for a := uint32(0xC0000); a < 0x100000; a += 0x1000 {
		tb.mapPage(a, 0xB8000)
	}
	for i := uint32(0); i < 16; i++ {
		tb.mapPage(0x100000+i*0x1000, 0xA0000+i*0x1000)
	}
	tb.enable(t)

	m.Write8(0xF0010, 0x5A) // 線性 F0000 映到 B8000：寫入成功，不是 ROM 丟棄
	if m.Mem[0xB8010] != 0x5A || m.ROMWrites != 0 {
		t.Errorf("線性 F0010 → 實體 B8010：Mem=%02X ROMWrites=%d", m.Mem[0xB8010], m.ROMWrites)
	}
	if got := m.Read8(0xC0010); got != 0x5A {
		t.Errorf("線性 C0010 也指 B8010：%02X", got)
	}
	m.Write8(0xB0020, 0x66)
	if m.Mem[0xB8020] != 0x66 {
		t.Errorf("線性 B0020 → 實體 B8020：%02X", m.Mem[0xB8020])
	}
	m.Write8(0x100123, 0x77) // HMA → A0000
	if m.Mem[0xA0123] != 0x77 {
		t.Errorf("線性 100123 → 實體 A0123：%02X", m.Mem[0xA0123])
	}
	m.Mem[0xAFFFF] = 0x88
	if got := m.Read8(0x10FFFF); got != 0x88 {
		t.Errorf("線性 10FFFF → 實體 AFFFF：%02X", got)
	}
	noFault(t, m, "JEMM 映射")
	if m.Ext[0x123] != 0 {
		t.Error("HMA 的寫入不該落在 Ext")
	}

	// 線性頁映到 ROMBase 以上的實體頁：寫入被丟棄並計 ROMWrites。
	tb.mapPage(0x5000, 0xF0000)
	before, old := m.ROMWrites, m.Mem[0xF0004]
	m.Write8(0x5004, old^0xFF)
	if m.ROMWrites != before+1 || m.Mem[0xF0004] != old {
		t.Errorf("映到 ROM 頁：ROMWrites %d→%d Mem=%02X（應維持 %02X）", before, m.ROMWrites, m.Mem[0xF0004], old)
	}

	// 跨頁且兩頁實體不連續。
	tb.mapPage(0x6000, 0x20000)
	tb.mapPage(0x7000, 0x30000)
	m.Write16(0x6FFF, 0xBEEF)
	if m.Mem[0x20FFF] != 0xEF || m.Mem[0x30000] != 0xBE {
		t.Errorf("跨頁 Write16：Mem[20FFF]=%02X Mem[30000]=%02X", m.Mem[0x20FFF], m.Mem[0x30000])
	}
	if got := m.Read16(0x6FFF); got != 0xBEEF {
		t.Errorf("跨頁 Read16 = %04X", got)
	}
	// 第二頁錯誤：第一個 byte 的寫入已發生（逐 byte 轉換）。
	tb.setPTE(0x7000, 0)
	m.SetCR3(tb.dir) // 作廢舊轉換
	m.Write16(0x6FFE, 0xCAFE)
	if m.Mem[0x20FFE] != 0xFE || m.Mem[0x20FFF] != 0xCA {
		t.Errorf("同頁的兩個 byte 都該寫入：%02X %02X", m.Mem[0x20FFE], m.Mem[0x20FFF])
	}
	m.Write16(0x6FFF, 0x1234)
	if m.Mem[0x20FFF] != 0x34 {
		t.Errorf("第二頁錯誤時第一個 byte 的寫入已發生：%02X", m.Mem[0x20FFF])
	}
	wantFault(t, m, "跨頁第二頁", 0x7000, true, false)
}

// ---- §9 第 2 項：頁表走訪讀實體位址（不遞迴）---------------------------------------

func TestPagingWalkReadsPhysicalAddresses(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	// 頁目錄（40000）與頁表（41000）所在的線性頁映到別處、內容完全不同。
	tb.mapPage(0x40000, 0x90000)
	tb.mapPage(0x41000, 0x91000)
	// 線性 0 頁映到實體 80000。
	tb.mapPage(0x0000, 0x80000)
	m.Mem[0x0000] = 0x11
	m.Mem[0x80000] = 0x22
	m.Mem[0x90000], m.Mem[0x91000] = 0x33, 0x44
	tb.enable(t)

	if got := m.Read8(0); got != 0x22 {
		t.Errorf("線性 0 → 實體 80000：%02X", got)
	}
	if got := m.ReadPhys8(0); got != 0x11 {
		t.Errorf("ReadPhys8(0) = %02X，應讀實體 0（%02X）", got, 0x11)
	}
	if m.Read8(0x40000) != 0x33 || m.Read8(0x41000) != 0x44 {
		t.Error("頁表所在的線性頁映到別處之後，存取該線性頁走新的映射")
	}
	noFault(t, m, "走訪")
	// 走訪若走線性層會遞迴；這裡其他頁照常轉換就是證據。
	if m.Read8(0x2000) != m.Mem[0x2000] {
		t.Error("其他頁的恆等映射")
	}

	// 頁表放在 Ext（實體 200000 起）。
	m2, _ := newPaged(t)
	tb2 := newTables(m2, 0x200000)
	tb2.identity(0, 0x100000, fPWU)
	tb2.mapPage(0x10000, 0x20000)
	m2.Mem[0x20000] = 0x5C
	tb2.enable(t)
	if got := m2.Read8(0x10000); got != 0x5C {
		t.Errorf("頁表在 Ext：Read8 = %02X", got)
	}
	// 走訪位址超出 Ext：讀到 FFFFFFFF，即 P=1 的垃圾項；不崩潰。
	m2.SetCR3(0x7000000)
	m2.Read8(0x1000)
	rec, ok := m2.PageFault()
	if ok {
		t.Errorf("垃圾項 FFFFFFFF 的 P=1：走訪繼續到 PTE（也是 FFFFFFFF），轉換成垃圾實體頁，不是錯誤：%+v", rec)
	}
}

// ---- §9 第 3 項：A20 與分頁 -------------------------------------------------------

func TestPagingA20(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x101000)
	tb.mapPage(0x11000, 0x301000)
	tb.enable(t)
	m.Mem[0x1000] = 0x11
	m.Ext[0x1000] = 0x22   // 實體 101000
	m.Ext[0x101000] = 0x33 // 實體 201000
	m.Ext[0x201000] = 0x44 // 實體 301000

	m.SetA20(false)
	if got := m.Read8(0x10000); got != 0x11 {
		t.Errorf("A20 關：轉換後的實體 101000 被遮成 001000：%02X", got)
	}
	if got := m.Read8(0x11000); got != 0x33 {
		t.Errorf("A20 關：0x301000 落在 0x201000：%02X", got)
	}
	m.Write8(0x11008, 0x99)
	if m.Ext[0x101008] != 0x99 {
		t.Errorf("A20 關的寫入：%02X", m.Ext[0x101008])
	}
	m.SetA20(true)
	if got := m.Read8(0x10000); got != 0x22 {
		t.Errorf("A20 開：不折回：%02X", got)
	}
	if got := m.Read8(0x11000); got != 0x44 {
		t.Errorf("A20 開：%02X", got)
	}
	noFault(t, m, "A20")

	// 走訪本身也受 A20 影響：CR3 在 0x180000（bit 20 設起），A20 關時頁目錄實際在 0x080000。
	m3, _ := newPaged(t)
	tb3 := newTables(m3, 0x80000)
	tb3.identity(0, 0x100000, fPWU)
	tb3.mapPage(0x10000, 0x30000)
	m3.Mem[0x30000] = 0x6D
	m3.SetA20(false)
	m3.SetCR3(0x180000) // A20 關：走訪位址遮成 0x80000
	m3.SetCR0(cr0PE | cr0PG)
	if got := m3.Read8(0x10000); got != 0x6D {
		t.Errorf("A20 關：頁目錄位址 0x180000 應遮成 0x80000，Read8 = %02X", got)
	}

	// PE=0 而 PG=1：不分頁。
	m4, tb4 := newPaged(t)
	tb4.mapPage(0x2000, 0x9000)
	m4.SetCR3(tb4.dir)
	if err := m4.SetCR0(cr0PG); err != nil {
		t.Fatal(err)
	}
	m4.Mem[0x2000] = 0x77
	if m4.paging || m4.Read8(0x2000) != 0x77 {
		t.Error("PE=0 而 PG=1 時不分頁")
	}
}

// ---- §9 第 5 項：舊轉換（惰性、不逐出）-------------------------------------------

func TestPagingStaleTranslations(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x20000)
	tb.mapPage(0x11000, 0x21000)
	m.Mem[0x20000], m.Mem[0x21000], m.Mem[0x22000] = 0xA1, 0xB1, 0xC1
	tb.enable(t)

	if m.Read8(0x10000) != 0xA1 {
		t.Fatal("初始映射")
	}
	// 改 PTE 而未重載 CR3：沒存取過的頁讀到新值，存取過的頁讀到舊值。
	tb.setPTE(0x10000, 0x22000|fPWU)
	tb.setPTE(0x11000, 0x22000|fPWU)
	if got := m.Read8(0x10000); got != 0xA1 {
		t.Errorf("已存取過的頁讀到舊轉換：%02X，應為 A1", got)
	}
	if got := m.Read8(0x11000); got != 0xC1 {
		t.Errorf("沒存取過的頁讀到新值：%02X，應為 C1", got)
	}
	// 寫 CR3 之後讀到新值。
	m.SetCR3(tb.dir)
	if got := m.Read8(0x10000); got != 0xC1 {
		t.Errorf("寫 CR3 後：%02X，應為 C1", got)
	}
	// A20 切換後舊條目作廢。
	tb.setPTE(0x10000, 0x21000|fPWU)
	if got := m.Read8(0x10000); got != 0xC1 {
		t.Errorf("尚未作廢：%02X", got)
	}
	m.SetA20(false)
	m.SetA20(true)
	if got := m.Read8(0x10000); got != 0xB1 {
		t.Errorf("A20 切換後：%02X，應為 B1", got)
	}
	// CR0.PG 改變（關再開）也作廢。
	tb.setPTE(0x10000, 0x20000|fPWU)
	m.SetCR0(cr0PE)
	m.SetCR0(cr0PE | cr0PG)
	if got := m.Read8(0x10000); got != 0xA1 {
		t.Errorf("PG 改變後：%02X，應為 A1", got)
	}
}

// 不同頁號在第一層、第二層的條目互不影響；不逐出。
func TestPagingTLBIsSparseAndNeverEvicts(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	// 三個頁：第一層索引不同、第二層索引相同或不同。
	lins := []uint32{0x00001000, 0x00401000, 0x00800000 - 0x1000 + 0x0, 0x00C01000}
	for i, lin := range lins {
		tb.mapPage(lin, 0x20000+uint32(i)*0x1000)
		m.Mem[0x20000+i*0x1000] = uint8(0xA0 + i)
	}
	tb.enable(t)
	for i, lin := range lins {
		if got := m.Read8(lin); got != uint8(0xA0+i) {
			t.Fatalf("頁 %X：%02X", lin, got)
		}
	}
	// 全部改指向實體 60000：已存取過的頁都維持舊值（沒有碰撞逐出）。
	m.Mem[0x60000] = 0xEE
	for _, lin := range lins {
		tb.setPTE(lin, 0x60000|fPWU)
	}
	for i, lin := range lins {
		if got := m.Read8(lin); got != uint8(0xA0+i) {
			t.Errorf("頁 %X 應仍讀到舊轉換 %02X：%02X", lin, 0xA0+i, got)
		}
	}
	// 沒存取過的鄰居頁不受影響：讀到新值。
	tb.mapPage(0x00002000, 0x60000)
	if got := m.Read8(0x00002000); got != 0xEE {
		t.Errorf("鄰頁：%02X", got)
	}

	// 大量頁：300 頁填滿之後全部改 PTE，仍全部是舊轉換（不論 TLB 容量與雜湊）。
	for i := uint32(0); i < 300; i++ {
		tb.mapPage(0x200000+i*0x1000, 0x20000+(i%16)*0x1000)
	}
	for i := uint32(0); i < 300; i++ {
		m.Read8(0x200000 + i*0x1000)
	}
	for i := uint32(0); i < 300; i++ {
		tb.setPTE(0x200000+i*0x1000, 0x60000|fPWU)
	}
	for i := uint32(0); i < 300; i++ {
		if got := m.Read8(0x200000 + i*0x1000); got == 0xEE {
			t.Fatalf("第 %d 頁被逐出：讀到新轉換", i)
		}
	}
}

// 世代溢位時整份清除：溢位前填入的條目不得在溢位後復活。
func TestPagingGenerationWrapClearsEntries(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x20000)
	m.Mem[0x20000], m.Mem[0x21000] = 0xA1, 0xB1
	tb.enable(t)
	m.tlbGen = ^uint32(0) - 1
	m.SetCR3(tb.dir) // 世代 = 最大值
	if m.Read8(0x10000) != 0xA1 {
		t.Fatal("初始")
	}
	tb.setPTE(0x10000, 0x21000|fPWU)
	m.SetCR3(tb.dir) // 溢位 → 清除
	if got := m.Read8(0x10000); got != 0xB1 {
		t.Errorf("溢位清除後應讀到新映射：%02X", got)
	}
}

// 不採負向快取：P=0 後修好 PTE 而不重載 CR3，再存取成功。
func TestPagingNoNegativeCache(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x20000)
	m.Mem[0x20000] = 0x4D
	tb.setPTE(0x10000, 0x20000|fW|fU) // P=0
	tb.enable(t)
	if got := m.Read8(0x10000); got != 0xFF {
		t.Errorf("頁不存在讀回 %02X，應為 FF", got)
	}
	wantFault(t, m, "PTE P=0", 0x10000, false, false)
	tb.setPTE(0x10000, 0x20000|fPWU)
	if got := m.Read8(0x10000); got != 0x4D {
		t.Errorf("修好 PTE 之後（沒重載 CR3）讀到 %02X，應為 4D", got)
	}
	// PDE 的 P=0 也一樣。
	m.pfOK = false
	tb.mapPage(0x800000, 0x20000)
	pde := tb.pde(0x800000)
	put32(m, tb.pdeAddr(0x800000), pde&^fP)
	if got := m.Read8(0x800000); got != 0xFF {
		t.Errorf("PDE P=0 讀回 %02X，應為 FF", got)
	}
	wantFault(t, m, "PDE P=0", 0x800000, false, false)
	m.pfOK = false
	put32(m, tb.pdeAddr(0x800000), pde)
	if got := m.Read8(0x800000); got != 0x4D {
		t.Errorf("修好 PDE 之後讀到 %02X，應為 4D", got)
	}
}

// ---- §9 第 6 項：accessed 與 dirty ------------------------------------------------

func TestPagingAccessedDirtyOrder(t *testing.T) {
	// 讀不設 dirty；讀後寫設一次 dirty；第二次寫入 PageTableWrites 不增加。
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x20000)
	tb.enable(t)
	if tb.pde(0x10000)&fA != 0 || tb.pte(0x10000)&(fA|fD) != 0 {
		t.Fatal("起始不該有 A／D")
	}
	m.PageTableWrites = 0
	m.Read8(0x10000)
	if tb.pde(0x10000)&fA == 0 || tb.pte(0x10000)&fA == 0 || tb.pte(0x10000)&fD != 0 {
		t.Errorf("讀：PDE=%08X PTE=%08X，應 A 已設、D 未設", tb.pde(0x10000), tb.pte(0x10000))
	}
	if m.PageTableWrites != 2 {
		t.Errorf("讀：PageTableWrites = %d，應為 2（PDE accessed、PTE accessed）", m.PageTableWrites)
	}
	m.Read8(0x10001)
	if m.PageTableWrites != 2 {
		t.Errorf("TLB 命中不改頁表：%d", m.PageTableWrites)
	}
	m.Write8(0x10000, 1)
	if tb.pte(0x10000)&fD == 0 || m.PageTableWrites != 3 {
		t.Errorf("讀後寫：PTE=%08X PageTableWrites=%d，應設 D 且 +1", tb.pte(0x10000), m.PageTableWrites)
	}
	m.Write8(0x10001, 2)
	m.Write8(0x10002, 3)
	if m.PageTableWrites != 3 {
		t.Errorf("第二次寫入不增加 PageTableWrites：%d", m.PageTableWrites)
	}

	// 第一次存取就是寫入：A 與 D 一次寫回。
	tb.mapPage(0x1000000, 0x31000)
	m.PageTableWrites = 0
	m.Write8(0x1000000, 9)
	if tb.pte(0x1000000)&(fA|fD) != fA|fD {
		t.Errorf("首次寫入：PTE=%08X", tb.pte(0x1000000))
	}
	if m.PageTableWrites != 2 { // PDE（本張頁表第一次被用到）＋PTE
		t.Errorf("首次寫入：PageTableWrites = %d，應為 2", m.PageTableWrites)
	}
	// PDE 已有 accessed：同頁表的另一頁只寫 PTE。
	tb.mapPage(0x1001000, 0x32000)
	m.PageTableWrites = 0
	m.Read8(0x1001000)
	if m.PageTableWrites != 1 {
		t.Errorf("PDE 已有 accessed：PageTableWrites = %d，應為 1", m.PageTableWrites)
	}
}

// 錯誤條件各一例：accessed／dirty 的寫回順序。
func TestPagingAccessedDirtyOnFaults(t *testing.T) {
	cases := []struct {
		name               string
		pdeFlags, pteFlags uint32
		write, user        bool
		wantPDEA           bool // PDE 的 accessed 是否已寫回
		wantPTEA, wantPTED bool
	}{
		{"權限錯誤：使用者寫唯讀頁", fPWU, fP | fU, true, true, true, false, false},
		{"權限錯誤：使用者讀 U/S=0 的 PTE", fPWU, fP | fW, false, true, true, false, false},
		{"權限錯誤：PDE 的 U/S=0", fP | fW, fPWU, false, true, true, false, false},
		{"PTE P=0", fPWU, fW | fU, false, false, true, false, false},
		{"PDE P=0", fW | fU, fPWU, false, false, false, false, false},
		{"監督者不受 U/S 與 R/W 限制", fP, fP, true, false, true, true, true},
	}
	for _, c := range cases {
		m, tb := newPaged(t)
		tb.mapFlags(0x10000, 0x20000, c.pdeFlags, c.pteFlags)
		tb.enable(t)
		if c.user {
			if err := m.SetPMCPL(3); err != nil {
				t.Fatal(err)
			}
		}
		m.PageTableWrites = 0
		if c.write {
			m.Write8(0x10000, 1)
		} else {
			m.Read8(0x10000)
		}
		pde, pte := tb.pde(0x10000), tb.pte(0x10000)
		if got := pde&fA != 0; got != c.wantPDEA && c.pdeFlags&fP != 0 {
			t.Errorf("%s：PDE accessed = %v，應為 %v", c.name, got, c.wantPDEA)
		}
		if c.pdeFlags&fP == 0 && m.PageTableWrites != 0 {
			t.Errorf("%s：PDE P=0 時不該寫任何頁表項（%d 次）", c.name, m.PageTableWrites)
		}
		if got := pte&fA != 0; got != c.wantPTEA {
			t.Errorf("%s：PTE accessed = %v，應為 %v", c.name, got, c.wantPTEA)
		}
		if got := pte&fD != 0; got != c.wantPTED {
			t.Errorf("%s：PTE dirty = %v，應為 %v", c.name, got, c.wantPTED)
		}
	}
}

// 讀入的條目之後被寫入：用條目的舊實體頁，PTE 讀改寫 dirty 位元。
func TestPagingDirtyRewriteKeepsOldPage(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x10000, 0x20000)
	tb.enable(t)
	m.Read8(0x10000) // 填入（dirty = false）
	tb.setPTE(0x10000, 0x30000|fPWU|fA)
	m.Write8(0x10000, 0x7B)
	if m.Mem[0x20000] != 0x7B || m.Mem[0x30000] == 0x7B {
		t.Errorf("應寫到條目的舊實體頁 20000：Mem[20000]=%02X Mem[30000]=%02X", m.Mem[0x20000], m.Mem[0x30000])
	}
	if got := tb.pte(0x10000); got != 0x30000|fPWU|fA|fD {
		t.Errorf("PTE = %08X，應為新頁基底加 D（讀改寫 dirty 位元）", got)
	}
	noFault(t, m, "舊轉換寫入")
}

// §10 第 9 項：頁目錄自我映射，accessed 位元的結果與 DOSBox-X 相同。
func TestPagingSelfMappedDirectory(t *testing.T) {
	// PDE[1] 指向頁目錄本身：線性 (1<<22)|(1<<12) 的 PTE 位址就是 PDE[1] 本身。
	const lin = 1<<22 | 1<<12
	for _, write := range []bool{false, true} {
		m, tb := newPaged(t)
		tb.identity(0, 0x100000, fPWU)
		put32(m, tb.dir+4, tb.dir|fPWU)
		tb.enable(t)
		m.PageTableWrites = 0
		if write {
			m.Write8(lin, 0x5A)
		} else {
			m.Read8(lin)
		}
		got := get32(m, tb.dir+4)
		want := tb.dir | fPWU | fA
		if write {
			want |= fD
		}
		if got != want {
			t.Errorf("write=%v：自我映射的項目 = %08X，應為 %08X", write, got, want)
		}
		// PTE 是 PDE 寫回 accessed 之前讀的：讀取時兩次寫回（PDE 的 A、PTE 的 A，後者與前者
		// 同值但相對於『讀到的值』有變）。與 DOSBox-X 的順序一致。
		if m.PageTableWrites != 2 {
			t.Errorf("write=%v：PageTableWrites = %d，應為 2", write, m.PageTableWrites)
		}
		// 資料頁就是頁目錄本身那一頁（實體 dir）。
		if write && m.Mem[tb.dir] != 0x5A {
			t.Errorf("寫入應落在實體 %X：%02X", tb.dir, m.Mem[tb.dir])
		}
	}
}

// ---- §9 第 7 項：權限 --------------------------------------------------------------

// 直接以 Read8／Write8 驗權限表：user 為 V86（SetVM）。
func TestPagingPermissionMatrix(t *testing.T) {
	type c struct {
		name               string
		pdeFlags, pteFlags uint32
		readFault          bool
		writeFault         bool
	}
	for _, tc := range []c{
		{"全開", fPWU, fPWU, false, false},
		{"PTE 唯讀", fPWU, fP | fU, false, true},
		{"PDE 唯讀", fP | fU, fPWU, false, true},
		{"PTE U/S=0", fPWU, fP | fW, true, true},
		{"PDE U/S=0", fP | fW, fPWU, true, true},
		{"兩者 U/S=0", fP | fW, fP | fW, true, true},
		{"U 與 W 在不同層", fP | fU, fP | fW, true, true},
	} {
		for _, mode := range []string{"V86", "CPL3", "CPL0"} {
			m, tb := newPaged(t)
			tb.mapFlags(0x10000, 0x20000, tc.pdeFlags, tc.pteFlags)
			tb.enable(t)
			user := mode != "CPL0"
			switch mode {
			case "V86":
				mustNil(t, "SetVM", m.SetVM(true))
			case "CPL3":
				mustNil(t, "SetPMCPL", m.SetPMCPL(3))
			}
			m.Mem[0x20000] = 0x3C
			got := m.Read8(0x10000)
			_, rf := m.PageFault()
			wantRF := user && tc.readFault
			if rf != wantRF {
				t.Errorf("%s/%s 讀：錯誤 = %v，應為 %v", tc.name, mode, rf, wantRF)
			}
			if !rf && got != 0x3C {
				t.Errorf("%s/%s 讀到 %02X", tc.name, mode, got)
			}
			if rf && got != 0xFF {
				t.Errorf("%s/%s 錯誤時讀回 %02X，應為 FF", tc.name, mode, got)
			}
			m.pfOK = false
			m.Write8(0x10001, 0x4E)
			_, wf := m.PageFault()
			wantWF := user && tc.writeFault
			if wf != wantWF {
				t.Errorf("%s/%s 寫：錯誤 = %v，應為 %v", tc.name, mode, wf, wantWF)
			}
			if wf && m.Mem[0x20001] == 0x4E {
				t.Errorf("%s/%s 錯誤的寫入應丟棄", tc.name, mode)
			}
			if !wf && m.Mem[0x20001] != 0x4E {
				t.Errorf("%s/%s 寫入沒落到實體頁", tc.name, mode)
			}
		}
	}
}

// 經由既有核心執行 mov [mem],al（V86 注入）：寫 R/W=0 頁使 Step 失敗。
func TestPagingStepWriteFaultInV86(t *testing.T) {
	m, tb := newPagedProg(t, []byte{0x88, 0x07, 0x90}, fPWU) // mov [bx],al; nop
	tb.mapFlags(0x50000, 0x50000, fPWU, fP|fU)               // R/W=0
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.BX] = 0x5000, 0
	m.CPU.R[cpu.AX] = 0x7E
	mustNil(t, "SetVM(true)", m.SetVM(true))

	err := m.Step()
	var pfe *PageFaultError
	if !errors.As(err, &pfe) {
		t.Fatalf("Step 應回 *PageFaultError，實際 %v", err)
	}
	rec := pfe.Record
	if rec.Linear != 0x50000 || !rec.Write || !rec.User || rec.PTE != tb.pte(0x50000) || rec.PDE != tb.pde(0x50000) {
		t.Errorf("紀錄 %+v（PTE 應為 %08X）", rec, tb.pte(0x50000))
	}
	if rec.Steps != m.Steps {
		t.Errorf("紀錄的 Steps = %d，應為產生時的 m.Steps = %d", rec.Steps, m.Steps)
	}
	if m.Mem[0x50000] == 0x7E {
		t.Error("寫入應丟棄")
	}
	if pfe.Unwrap() != nil {
		t.Errorf("CPU 沒有錯誤時不該包住別的錯誤：%v", pfe.Unwrap())
	}

	// SetVM(false)、SetPMCPL(0)：同一道指令寫 R/W=0 頁成功（依 431 此時應由 cpu386 執行，
	// 這裡只驗 Machine 的權限判斷）。
	mustNil(t, "SetVM(false)", m.SetVM(false))
	mustNil(t, "SetPMCPL(0)", m.SetPMCPL(0))
	m.CPU.IP = 0x100
	mustNil(t, "Step（CPL0）", m.Step())
	if m.Mem[0x50000] != 0x7E {
		t.Errorf("CPL=0 的寫入應成功：%02X", m.Mem[0x50000])
	}
}

// 監督者存取：CPL=3 時 ReadSup8 讀 U/S=0 頁、WriteSup8 寫 R/W=0 頁成功。
func TestPagingSupervisorAccess(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapFlags(0x10000, 0x20000, fPWU, fP|fW) // U/S=0
	tb.mapFlags(0x11000, 0x21000, fPWU, fP|fU) // R/W=0
	tb.enable(t)
	mustNil(t, "SetPMCPL", m.SetPMCPL(3))
	m.Mem[0x20000] = 0x61
	if got := m.ReadSup8(0x10000); got != 0x61 {
		t.Errorf("ReadSup8 讀 U/S=0 頁：%02X", got)
	}
	m.WriteSup8(0x11000, 0x62)
	if m.Mem[0x21000] != 0x62 {
		t.Errorf("WriteSup8 寫 R/W=0 頁：%02X", m.Mem[0x21000])
	}
	noFault(t, m, "監督者存取")
	// 同一頁以使用者身分仍然錯誤，即使條目已被監督者存取填入（TLB 命中而權限錯誤）。
	if got := m.Read8(0x10000); got != 0xFF {
		t.Errorf("使用者讀 U/S=0 頁（TLB 命中）= %02X，應為 FF", got)
	}
	rec := wantFault(t, m, "TLB 命中的權限錯誤", 0x10000, false, true)
	if rec.PDE != tb.pde(0x10000) || rec.PTE != tb.pte(0x10000) {
		t.Errorf("TLB 命中的權限錯誤：紀錄 PDE=%08X PTE=%08X，應重讀為 %08X %08X", rec.PDE, rec.PTE, tb.pde(0x10000), tb.pte(0x10000))
	}
	if rec.PDE&fA == 0 || rec.PTE&fP == 0 {
		t.Errorf("紀錄內容不合理：%+v", rec)
	}
}

// ---- §9 第 8 項、§10 第 5 項：錯誤紀錄的生命週期 -----------------------------------------

func TestPageFaultRecordLifecycle(t *testing.T) {
	// mov [bx],ax; nop; nop
	m, tb := newPagedProg(t, []byte{0x89, 0x07, 0x90, 0x90}, fPWU)
	tb.setPTE(0x50000, 0)                              // 線性 50000 頁不存在
	tb.setPTE(0x51000, 0)                              // 線性 51000 頁不存在
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.BX] = 0x5000, 0xFFF // Write16 跨 50FFF／51000
	m.CPU.R[cpu.AX] = 0xBEEF
	mustNil(t, "SetVM", m.SetVM(true))

	// 同一步兩次錯誤（兩個 byte 各一次）只保留第一筆。
	err := m.Step()
	var pfe *PageFaultError
	if !errors.As(err, &pfe) {
		t.Fatalf("Step 應回 *PageFaultError：%v", err)
	}
	if pfe.Record.Linear != 0x50FFF || !pfe.Record.Write {
		t.Errorf("應只留第一筆（0x50FFF，寫）：%+v", pfe.Record)
	}
	if rec, ok := m.PageFault(); !ok || rec != pfe.Record {
		t.Errorf("PageFault() = %+v, %v", rec, ok)
	}
	if _, ok := m.LastPageFault(); ok {
		t.Error("LastPageFault 在第一次錯誤之前應為空")
	}

	// 下一個沒有錯誤的 Step 回 nil，LastPageFault 是上一筆，PageFault 清空。
	if err := m.Step(); err != nil {
		t.Fatalf("沒有錯誤的 Step 應回 nil：%v", err)
	}
	if last, ok := m.LastPageFault(); !ok || last != pfe.Record {
		t.Errorf("LastPageFault = %+v, %v，應為上一筆", last, ok)
	}
	noFault(t, m, "下一步之後")
	// 沒有殘留紀錄時 Step 不動 LastPageFault。
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if last, ok := m.LastPageFault(); !ok || last != pfe.Record {
		t.Error("沒有殘留紀錄時 LastPageFault 不該被動")
	}

	// Step 以外的 Read8 錯誤：不讓下一個 Step 失敗，可由 PageFault() 查到。
	m.CPU.IP = 0x103 // nop
	m.Steps += 100
	m.SetVM(false)
	m.SetPMCPL(3)
	if got := m.Read8(0x50000); got != 0xFF {
		t.Errorf("Read8 = %02X", got)
	}
	outside := wantFault(t, m, "Step 外的讀", 0x50000, false, true)
	if outside.Steps != m.Steps {
		t.Errorf("Steps = %d，應為 %d", outside.Steps, m.Steps)
	}
	// 連續多筆：PageFault 非空就不覆蓋。
	m.Read8(0x51000)
	if rec, _ := m.PageFault(); rec.Linear != 0x50000 {
		t.Errorf("PageFault 非空時不覆蓋：%+v", rec)
	}
	if err := m.Step(); err != nil {
		t.Fatalf("Step 外的錯誤不該讓下一個 Step 失敗：%v", err)
	}
	if last, _ := m.LastPageFault(); last != outside {
		t.Errorf("殘留紀錄在 Step 第一動作移到 LastPageFault：%+v", last)
	}
	noFault(t, m, "Step 之後")

	// Peek 不產生紀錄。
	if v, ok := m.PeekOK(0x50000); v != 0xFF || ok {
		t.Errorf("PeekOK(頁不存在) = (%02X, %v)，應為 (FF, false)", v, ok)
	}
	m.Peek8(0x51000)
	m.Peek16(0x50FFF)
	noFault(t, m, "Peek")
}

// Restore、LoadState 清空兩個紀錄。
func TestPageFaultRecordsClearedByRestoreAndLoadState(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.setPTE(0x10000, 0)
	tb.enable(t)
	snap := m.Snapshot()
	m.Read8(0x10000)
	m.pfLast, m.pfLastOK = m.pf, true
	if _, ok := m.PageFault(); !ok {
		t.Fatal("前置：應有紀錄")
	}
	m.Restore(snap)
	if _, ok := m.PageFault(); ok {
		t.Error("Restore 應清空 PageFault")
	}
	if _, ok := m.LastPageFault(); ok {
		t.Error("Restore 應清空 LastPageFault")
	}
	// LoadState。
	src, _ := newPaged(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0x90}))
	data := saveBytes(t, src)
	dst, tbd := newPaged(t)
	tbd.identity(0, 0x100000, fPWU)
	tbd.setPTE(0x10000, 0)
	tbd.enable(t)
	dst.Read8(0x10000)
	dst.pfLast, dst.pfLastOK = dst.pf, true
	if _, ok := dst.PageFault(); !ok {
		t.Fatal("前置：應有紀錄")
	}
	// 狀態檔的 PM 是 PE=0：兩邊 profile 一致即可載入。
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	if _, ok := dst.PageFault(); ok {
		t.Error("LoadState 應清空 PageFault")
	}
	if _, ok := dst.LastPageFault(); ok {
		t.Error("LoadState 應清空 LastPageFault")
	}
}

// §10 補充驗收第 1 項：取指令落在不存在頁、讀回 0xFF 被解成其他東西時，Step 回
// *PageFaultError 且 errors.As 取得紀錄；CPU 自己也出錯時以 %w 包住。
func TestPagingFetchFromAbsentPage(t *testing.T) {
	m, tb := newPagedProg(t, []byte{0x90, 0x90}, fPWU)
	tb.setPTE(0x1000, 0) // COM 程式所在的線性頁（PSP 0100:0000 ＝ 1000）整頁不存在
	mustNil(t, "SetVM", m.SetVM(true))
	m.CPU.IP = 0x100
	err := m.Step()
	var pfe *PageFaultError
	if !errors.As(err, &pfe) {
		t.Fatalf("Step 應回 *PageFaultError：%v", err)
	}
	if pfe.Record.Linear != 0x1100 || pfe.Record.Write || !pfe.Record.User {
		t.Errorf("取指令的紀錄：%+v", pfe.Record)
	}
	// 0xFF 0xFF…：若 CPU 因此出錯，錯誤以 %w 包住，errors.Unwrap 取得；若 CPU 把它當成
	// 合法指令執行，Err 為 nil。兩者都要保持分頁錯誤優先。
	if pfe.Err != nil && !errors.Is(err, pfe.Err) {
		t.Errorf("errors.Is 應可穿過 PageFaultError：%v", err)
	}
	t.Logf("取指令錯誤：%v", err)
}

// 以自訂的 CPU 錯誤驗 stepFault 的包裝與優先序（哨兵同此，433、435 的哨兵比照）。
func TestStepFaultWrapsCPUError(t *testing.T) {
	m := New()
	sentinel := errors.New("sentinel")
	m.pf = PageFaultRecord{Linear: 0x1234, Write: true}
	m.pfOK = true
	err := m.stepFault(sentinel)
	var pfe *PageFaultError
	if !errors.As(err, &pfe) || !errors.Is(err, sentinel) {
		t.Fatalf("errors.As 與 errors.Is 都應可用：%v", err)
	}
	if pfe.Record.Linear != 0x1234 {
		t.Errorf("紀錄：%+v", pfe.Record)
	}
	if msg := err.Error(); len(msg) == 0 || msg == sentinel.Error() {
		t.Errorf("錯誤訊息要附上分頁紀錄：%q", msg)
	}
	if plain := m.stepFault(nil); errors.Unwrap(plain) != nil {
		t.Error("沒有 CPU 錯誤時不包任何東西")
	}
}

// ---- §9 第 10 項：Peek -------------------------------------------------------------

func TestPagingPeekHasNoSideEffects(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapFlags(0x10000, 0x20000, fPWU, fP|fW) // U/S=0
	tb.mapPage(0x11000, 0x21000)
	m.Mem[0x20000], m.Mem[0x21000] = 0x71, 0x72
	tb.enable(t)
	mustNil(t, "SetPMCPL", m.SetPMCPL(3))
	m.PageTableWrites = 0

	if got := m.Peek8(0x10000); got != 0x71 {
		t.Errorf("Peek8 對 U/S=0 頁仍讀得到：%02X", got)
	}
	if v, ok := m.PeekOK(0x11000); v != 0x72 || !ok {
		t.Errorf("PeekOK = (%02X, %v)", v, ok)
	}
	if tb.pde(0x10000)&fA != 0 || tb.pte(0x10000)&fA != 0 || m.PageTableWrites != 0 {
		t.Error("Peek 不得改 accessed")
	}
	noFault(t, m, "Peek")
	// 不填 TLB：之後改 PTE，Read8 看到新映射。
	tb.setPTE(0x11000, 0x22000|fPWU)
	m.Mem[0x22000] = 0x73
	if got := m.Read8(0x11000); got != 0x73 {
		t.Errorf("Peek 沒填 TLB，Read8 應走新映射：%02X", got)
	}
	// TLB 條目世代符合就用（看到 CPU 此刻會看到的，含舊轉換）。
	tb.setPTE(0x11000, 0x21000|fPWU)
	if got := m.Peek8(0x11000); got != 0x73 {
		t.Errorf("Peek 應看到 TLB 內的舊轉換：%02X", got)
	}
	// 頁不存在。
	tb.setPTE(0x12000, 0)
	if v, ok := m.PeekOK(0x12000); v != 0xFF || ok {
		t.Errorf("頁不存在：(%02X, %v)", v, ok)
	}
	if v, ok := m.PeekOK16(0x11FFF); ok || v>>8 != 0xFF {
		t.Errorf("PeekOK16 跨進不存在的頁：(%04X, %v)", v, ok)
	}
	// PDE P=0。
	if _, ok := m.PeekOK(0x3000000); ok {
		t.Error("PDE P=0 應回 false")
	}
	noFault(t, m, "Peek 之後")
}

// ---- §3.4：BIOS 與硬體側走實體層 ---------------------------------------------------

// 第 0 頁被重映射（JEMM 的別名）時，BDA 與鍵盤緩衝仍讀寫實體 0。
func TestBIOSSideIgnoresPageRemapping(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x0000, 0x80000) // 線性第 0 頁 → 實體 80000
	tb.enable(t)
	for i := range m.Mem[0x80000:0x81000] {
		m.Mem[0x80000+i] = 0xEE
	}
	snap := append([]byte(nil), m.Mem[0x80000:0x81000]...)

	// 鍵盤緩衝（BDA 0040:001E…）。
	headPhys := m.ReadPhys16(bdaSeg*16 + kbHead)
	if !m.PushBIOSKey(0x1E, 'a') {
		t.Fatal("PushBIOSKey")
	}
	if m.BIOSKeyCount() != 1 {
		t.Errorf("BIOSKeyCount = %d", m.BIOSKeyCount())
	}
	if key, ok := m.PeekKey(); !ok || key != 0x1E<<8|'a' {
		t.Errorf("PeekKey = %04X, %v", key, ok)
	}
	if got := m.ReadPhys16(bdaSeg*16 + uint32(headPhys)); got != 0x1E<<8|'a' {
		t.Errorf("緩衝內容應在實體 0040:%04X：%04X", headPhys, got)
	}
	if key, ok := m.PopKey(); !ok || key != 0x1E<<8|'a' {
		t.Errorf("PopKey = %04X, %v", key, ok)
	}
	m.FlushKeys()
	// BIOS tick 計數。
	before := uint32(m.ReadPhys16(0x46C)) | uint32(m.ReadPhys16(0x46E))<<16
	m.bumpBDATicks()
	after := uint32(m.ReadPhys16(0x46C)) | uint32(m.ReadPhys16(0x46E))<<16
	if after != before+1 {
		t.Errorf("bumpBDATicks：%d → %d", before, after)
	}
	// 文字畫面讀出：B8000 不受第 0 頁影響；讀出視訊欄數用實體 0x44A。
	m.Mem[0x44A], m.Mem[0x44B] = 80, 0
	m.Mem[0xB8000] = 'Z'
	if rows := m.TextScreen(0); rows[0] == "" || rows[0][0] != 'Z' {
		t.Errorf("TextScreen 讀實體 B8000：%q", rows[0])
	}
	// 客體線性視角的讀寫則走映射：線性 0x400 是實體 80400。
	if got := m.Read8(0x400); got != 0xEE {
		t.Errorf("線性 0x400 應走映射到 80400：%02X", got)
	}
	// 實體 80000 那一頁完全沒被 BIOS 側動過。
	for i, v := range m.Mem[0x80000:0x81000] {
		if v != snap[i] {
			t.Fatalf("實體 80000 頁的 +%X 被 BIOS 側改動：%02X → %02X", i, snap[i], v)
		}
	}
	noFault(t, m, "BIOS 側")
}

// IVT 讀取（keyTick 檢查 int 09h 向量是否仍指 StubSeg）走實體層。
func TestKeyboardIVTReadIgnoresPageRemapping(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0x0000, 0x80000)
	tb.enable(t)
	// 線性視角的 IVT（實體 80000）全是 0xEE；實體 IVT 的 int 09h 向量指 StubSeg（預設）。
	for i := range m.Mem[0x80000:0x81000] {
		m.Mem[0x80000+i] = 0xEE
	}
	if m.ReadPhys16(0x09*4+2) != StubSeg {
		t.Fatal("前置：實體 IVT 的 int 09h 應指 StubSeg")
	}
	m.QueueScanCodes(0x1E)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	// keyTick 讀實體 IVT 判斷「int 09h 還指著 stub」，於是留著不送（keyStalls++）；
	// 若誤走線性層會讀到 EEEE，把鍵送進一個不存在的處理常式（KeyIRQs++）。
	m.keyTick()
	if m.KeyIRQs != 0 || m.KeyStalls() != 1 || m.KeyQueueLen() != 1 {
		t.Errorf("keyTick 應依實體 IVT 判斷：KeyIRQs=%d KeyStalls=%d 佇列=%d", m.KeyIRQs, m.KeyStalls(), m.KeyQueueLen())
	}
	noFault(t, m, "keyTick")
}
