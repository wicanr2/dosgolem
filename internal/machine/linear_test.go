package machine

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/432-linear-memory-and-paging 的 Machine 層測試（C1：兩層存取、setter、
// SvcAddr、Peek、PM 編碼；分頁在 C2）。
//
// 需要執行指令的測試把 IRQ0Every 設成 0：沒有掛 internal/dos 時，int 1Ch 向量指到的
// 預設 stub 是 `CD 1C`，沒有人替它 IRET，第一次計時器中斷就會無限遞迴
// （BenchmarkStepRealMode 同理）。

// newPM 造一台 profile 開啟的機器（4 MB 延伸記憶體，程式尚未載入）。
func newPM(t *testing.T) *Machine {
	t.Helper()
	m := newExt(t, 4096)
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	return m
}

func mustNil(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s：%v", what, err)
	}
}

func mustErr(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s：應回錯誤", what)
	}
}

// ---- setter（§2、§10 第 2–4 項）-------------------------------------------

func TestSettersRequireProfile(t *testing.T) {
	m := newExt(t, 4096) // profile 關閉
	mustErr(t, "SetCR0", m.SetCR0(1))
	mustErr(t, "SetCR3", m.SetCR3(0x1000))
	mustErr(t, "SetEFlagsHi", m.SetEFlagsHi(0))
	mustErr(t, "SetVM(true)", m.SetVM(true))
	mustErr(t, "SetVM(false)", m.SetVM(false))
	mustErr(t, "SetPMCPL", m.SetPMCPL(0))
	if m.CR0() != pmCR0Init || m.CR3() != 0 || m.PMCPL() != 0 || m.EFlagsHi() != 0 || m.Translating() {
		t.Errorf("失敗的 setter 不得改狀態：CR0=%08X CR3=%X", m.CR0(), m.CR3())
	}
}

func TestSetProtectedModeProfileConditions(t *testing.T) {
	// 沒有延伸記憶體。
	m := New()
	mustErr(t, "無 Ext 開啟", m.SetProtectedModeProfile(true))
	if m.ProtectedModeProfile() {
		t.Error("失敗後 profile 不得開啟")
	}
	// 記憶體已不是 MemSize（LE 路徑會增長 Mem）。
	m = newExt(t, 4096)
	m.Mem = make([]uint8, 2*MemSize)
	mustErr(t, "Mem 長度不對", m.SetProtectedModeProfile(true))
	// 型號不是 80386。
	m = newExt(t, 4096)
	m.CPU.Model = cpu.Model8086
	mustErr(t, "非 386", m.SetProtectedModeProfile(true))
	// 程式已載入。
	m = newExt(t, 4096)
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	mustErr(t, "已載入", m.SetProtectedModeProfile(true))
	mustErr(t, "已載入（關閉也一樣）", m.SetProtectedModeProfile(false))
	// 同值是空呼叫。
	m = newExt(t, 4096)
	mustNil(t, "關閉→關閉", m.SetProtectedModeProfile(false))
	mustNil(t, "開啟", m.SetProtectedModeProfile(true))
	mustNil(t, "開啟→開啟", m.SetProtectedModeProfile(true))
	// 只能在 CR0 為初值時切換。
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustErr(t, "CR0 非初值時關閉", m.SetProtectedModeProfile(false))
	mustNil(t, "SetCR0 還原", m.SetCR0(0))
	mustNil(t, "SetCR3", m.SetCR3(0x5000))
	mustNil(t, "SetPMCPL", m.SetPMCPL(3))
	mustNil(t, "關閉", m.SetProtectedModeProfile(false))
	if m.CR3() != 0 || m.PMCPL() != 0 || m.EFlagsHi() != 0 || m.Translating() || m.cplUser {
		t.Errorf("關閉後其餘欄位應回初值：CR3=%X CPL=%d", m.CR3(), m.PMCPL())
	}
}

func TestSetCR0Normalization(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0(all)", m.SetCR0(0xFFFFFFFF))
	if got := m.CR0(); got != 0x8000001F {
		t.Errorf("CR0 = %08X，應只保留 bit 0–4 與 bit 31（WP 與其餘清為 0）", got)
	}
	mustNil(t, "SetCR0(WP)", m.SetCR0(1<<16))
	if got := m.CR0(); got != pmCR0Init {
		t.Errorf("只寫 WP 後 CR0 = %08X，應為 %08X", got, pmCR0Init)
	}
	mustNil(t, "SetCR0(0)", m.SetCR0(0))
	if got := m.CR0(); got != pmCR0Init {
		t.Errorf("CR0 = %08X，ET 要補上", got)
	}
}

func TestSetEFlagsHiAndVM(t *testing.T) {
	m := newPM(t)
	mustErr(t, "PE=0 設 VM", m.SetVM(true))
	mustErr(t, "PE=0 設 EFlagsHi.VM", m.SetEFlagsHi(eflagsHiVM))
	mustNil(t, "PE=0 SetVM(false)", m.SetVM(false))
	mustNil(t, "SetCR0(PE)", m.SetCR0(cr0PE))
	mustErr(t, "AC 位元", m.SetEFlagsHi(1<<2))
	mustErr(t, "ID 位元", m.SetEFlagsHi(1<<5))
	mustNil(t, "RF", m.SetEFlagsHi(eflagsHiRF))
	mustNil(t, "SetVM(true)", m.SetVM(true))
	if m.EFlagsHi() != eflagsHiRF|eflagsHiVM {
		t.Errorf("SetVM 應保留 RF：EFlagsHi = %X", m.EFlagsHi())
	}
	if !m.cplUser {
		t.Error("V86 的 CPL 是 3")
	}
	// VM=1 時不能把 PE 清成 0，且失敗不改狀態。
	mustErr(t, "VM=1 時 PE→0", m.SetCR0(0))
	if m.CR0()&cr0PE == 0 || !m.Translating() {
		t.Error("失敗的 SetCR0 不得改狀態")
	}
	mustNil(t, "SetVM(false)", m.SetVM(false))
	if m.EFlagsHi() != eflagsHiRF || m.cplUser {
		t.Errorf("SetVM(false)：EFlagsHi = %X cplUser = %v", m.EFlagsHi(), m.cplUser)
	}
	mustNil(t, "PE→0", m.SetCR0(0))
}

func TestSetPMCPL(t *testing.T) {
	m := newPM(t)
	mustErr(t, "CPL 4", m.SetPMCPL(4))
	mustNil(t, "PE=0 設 CPL 3", m.SetPMCPL(3))
	if m.cplUser {
		t.Error("PE=0 時 cplUser 恆為 false")
	}
	mustNil(t, "SetCR0(PE)", m.SetCR0(cr0PE))
	if !m.cplUser || m.PMCPL() != 3 {
		t.Errorf("PE=1、CPL=3：cplUser = %v", m.cplUser)
	}
	for _, cpl := range []uint8{0, 1, 2} {
		mustNil(t, "SetPMCPL", m.SetPMCPL(cpl))
		if m.cplUser {
			t.Errorf("CPL=%d 不是使用者", cpl)
		}
	}
}

func TestDerivedFields(t *testing.T) {
	type c struct {
		cr0       uint32
		vm        bool
		cpl       uint8
		xlat, usr bool
	}
	for _, tc := range []c{
		{0, false, 0, false, false},
		{cr0PE, false, 0, true, false},
		{cr0PE, false, 3, true, true},
		{cr0PE, true, 0, true, true},
		{cr0PG, false, 3, false, false},              // PE=0、PG=1：不轉換
		{cr0PE | cr0PG, false, 0, true, false},       // 分頁在 C2 驗
		{cr0PE | cr0PG, true, 0, true, true},         //
		{cr0PE | 1<<16 | 0x8, false, 2, true, false}, // TS 無關
	} {
		m := newPM(t)
		mustNil(t, "SetCR0", m.SetCR0(tc.cr0))
		mustNil(t, "SetPMCPL", m.SetPMCPL(tc.cpl))
		if tc.vm {
			mustNil(t, "SetVM", m.SetVM(true))
		}
		if m.xlat != tc.xlat || m.cplUser != tc.usr {
			t.Errorf("%+v：xlat=%v cplUser=%v", tc, m.xlat, m.cplUser)
		}
		if tc.cr0&cr0PE == 0 && m.paging {
			t.Errorf("%+v：PE=0 而 PG=1 時不得分頁", tc)
		}
	}
}

// ---- TLB 世代（§5.1 的作廢條件；TLB 本體在 C2）-----------------------------

func TestTLBGenerationBumps(t *testing.T) {
	m := newPM(t)
	bumped := func(what string, want bool, f func()) {
		t.Helper()
		before := m.tlbGen
		f()
		if got := m.tlbGen != before; got != want {
			t.Errorf("%s：世代變動 = %v，應為 %v", what, got, want)
		}
	}
	bumped("SetCR3", true, func() { mustNil(t, "", m.SetCR3(0x1000)) })
	bumped("SetCR3 同值也作廢", true, func() { mustNil(t, "", m.SetCR3(0x1000)) })
	bumped("PE 改變", true, func() { mustNil(t, "", m.SetCR0(cr0PE)) })
	bumped("CR0 同值", false, func() { mustNil(t, "", m.SetCR0(cr0PE)) })
	bumped("TS 改變", false, func() { mustNil(t, "", m.SetCR0(cr0PE|0x8)) })
	bumped("PG 改變", true, func() { mustNil(t, "", m.SetCR0(cr0PE|cr0PG)) })
	mustNil(t, "", m.SetCR0(0))
	bumped("PE=0 時 PG 改變", true, func() { mustNil(t, "", m.SetCR0(cr0PG)) })
	mustNil(t, "", m.SetCR0(cr0PE))
	bumped("SetA20", true, func() { m.SetA20(true) })
	bumped("SetVM", false, func() { mustNil(t, "", m.SetVM(true)) })
	bumped("SetPMCPL", false, func() { mustNil(t, "", m.SetPMCPL(2)) })
}

func TestTLBGenerationWrapClears(t *testing.T) {
	m := newPM(t)
	m.tlbGen = ^uint32(0)
	m.invalidateTLB()
	if m.tlbGen == 0 {
		t.Error("世代 0 代表『從未填過』，溢位時要整份清除並回到有效值")
	}
}

func TestSetCR3DoesNotAllocate(t *testing.T) {
	m := newPM(t)
	allocs := testing.AllocsPerRun(1, func() {
		for i := 0; i < 100_000; i++ {
			if err := m.SetCR3(uint32(i) << 12); err != nil {
				t.Fatal(err)
			}
		}
	})
	if allocs != 0 {
		t.Errorf("連續 10 萬次 SetCR3 配置 %v 次，應為 0", allocs)
	}
}

// ---- 兩層存取與 A20（§3、§9 第 1、2、3 項）---------------------------------

// 轉換開啟之前，profile 開、PE=0、A20 開時 Read8 與 profile 關閉相同。
func TestProfileOnPEZeroSameAsProfileOff(t *testing.T) {
	for _, a20 := range []bool{false, true} {
		on := newPM(t)
		off := newExt(t, 4096)
		for _, m := range []*Machine{on, off} {
			m.SetA20(a20)
			for i := 0; i < 0x20000; i++ {
				m.Ext[i] = uint8(i*7 + 3)
			}
			for i := 0; i < 0x200; i++ {
				m.Mem[i] = uint8(i*5 + 1)
			}
		}
		for _, a := range []uint32{0, 0x10, 0xFFFFF, 0x100000, 0x100010, 0x10FFEF, 0x10FFF0, 0x10FFFF, 0x110000, 0x200000} {
			if g, w := on.Read8(a), off.Read8(a); g != w {
				t.Errorf("a20=%v Read8(%X)：profile 開 %02X、關 %02X", a20, a, g, w)
			}
			on.Write8(a, 0x5A)
			off.Write8(a, 0x5A)
		}
		if !bytes.Equal(on.Mem, off.Mem) || !bytes.Equal(on.Ext, off.Ext) {
			t.Errorf("a20=%v：寫入後記憶體不同", a20)
		}
	}
}

func TestLinearA20WithoutPaging(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	// 實體 0x0xxxxx 與 0x1xxxxx、0x2xxxxx、0x3xxxxx 的內容各不相同。
	m.Mem[0x1234] = 0x11
	m.Ext[0x1234] = 0x22   // 實體 0x101234
	m.Ext[0x101234] = 0x33 // 實體 0x201234
	m.Ext[0x201234] = 0x44 // 實體 0x301234

	// A20 關：bit 20 被遮掉。
	m.SetA20(false)
	for _, c := range []struct {
		a    uint32
		want uint8
	}{{0x1234, 0x11}, {0x101234, 0x11}, {0x201234, 0x33}, {0x301234, 0x33}} {
		if got := m.Read8(c.a); got != c.want {
			t.Errorf("A20 關 Read8(%X) = %02X，應為 %02X", c.a, got, c.want)
		}
	}
	// 寫入也一樣：0x300000 落在 0x200000。
	m.Write8(0x300008, 0x99)
	if m.Ext[0x100008] != 0x99 {
		t.Errorf("A20 關：0x300008 應落在實體 0x200008，Ext[0x100008] = %02X", m.Ext[0x100008])
	}
	// A20 開：不遮罩。
	m.SetA20(true)
	for _, c := range []struct {
		a    uint32
		want uint8
	}{{0x1234, 0x11}, {0x101234, 0x22}, {0x201234, 0x33}, {0x301234, 0x44}} {
		if got := m.Read8(c.a); got != c.want {
			t.Errorf("A20 開 Read8(%X) = %02X，應為 %02X", c.a, got, c.want)
		}
	}
}

// A20 開時 0x10FFF0 以上不折回（轉換關閉會折回，兩者不同）。
func TestLinearDoesNotFoldAbove10FFF0(t *testing.T) {
	on := newPM(t)
	off := newExt(t, 4096)
	for _, m := range []*Machine{on, off} {
		m.SetA20(true)
		m.Mem[0xFFF0] = 0xAA
		m.Ext[0xFFF0] = 0xBB // 實體 0x10FFF0
	}
	if got := off.Read8(0x10FFF0); got != 0xAA {
		t.Errorf("轉換關閉：0x10FFF0 折回 0xFFF0，讀到 %02X，應為 AA", got)
	}
	mustNil(t, "SetCR0", on.SetCR0(cr0PE))
	if got := on.Read8(0x10FFF0); got != 0xBB {
		t.Errorf("轉換開啟：0x10FFF0 不折回，讀到 %02X，應為 BB", got)
	}
	if got := on.Read8(0x10FFEF); got != on.Ext[0xFFEF] {
		t.Errorf("0x10FFEF 是 HMA 最後一格：%02X", got)
	}
}

func TestLinearOutsideExt(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.SetA20(true)
	end := uint32(ExtBase + len(m.Ext))
	if got := m.Read8(end); got != 0xFF {
		t.Errorf("Ext 之外讀 %02X，應為 FF", got)
	}
	if got := m.Read8(0xFFFFFFFF); got != 0xFF {
		t.Errorf("4 GB 邊界讀 %02X，應為 FF", got)
	}
	before := m.UnmappedPhysWrites
	m.Write8(end, 1)
	m.Write8(0xFFFFFFFF, 1)
	if m.UnmappedPhysWrites != before+2 {
		t.Errorf("UnmappedPhysWrites 增加 %d，應為 2", m.UnmappedPhysWrites-before)
	}
	// A20 關：end = 0x500000 遮成 0x400000，落進 Ext（索引 0x300000）。
	m.Ext[0x300000] = 0x6C
	m.SetA20(false)
	if got := m.Read8(end); got != 0x6C {
		t.Errorf("A20 關：%X 遮成 %X 後應讀到 Ext 的內容 6C，實際 %02X", end, end&^0x100000, got)
	}
	// 寫在 Mem 的 1 MB 邊界上：Write16 跨過 0xFFFFF／0x100000 時兩個 byte 分別歸類。
	m.SetA20(true)
	// 0xFFFFF 在 ROM 區（寫入被丟棄），0x100000 起是 Ext：兩個 byte 各自歸類。
	rom := m.Mem[MemSize-1]
	m.Write16(MemSize-1, 0xBEEF)
	if m.Mem[MemSize-1] != rom || m.Ext[0] != 0xBE {
		t.Errorf("跨 1 MB 的 Write16：Mem[FFFFF]=%02X（ROM 應維持 %02X） Ext[0]=%02X", m.Mem[MemSize-1], rom, m.Ext[0])
	}
	if got := m.Read16(MemSize - 1); got != uint16(rom)|0xBE00 {
		t.Errorf("跨 1 MB 的 Read16 = %04X", got)
	}
}

// 實體層不受 A20 與轉換影響：ReadPhys8 讀的就是實體位址。
func TestPhysLayerIgnoresA20AndTranslation(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.SetA20(false)
	m.Mem[0x10] = 0x11
	m.Ext[0x10] = 0x22 // 實體 0x100010
	if got := m.Read8(0x100010); got != 0x11 {
		t.Errorf("A20 關：線性 0x100010 折回 0x10，讀到 %02X", got)
	}
	if got := m.ReadPhys8(0x100010); got != 0x22 {
		t.Errorf("ReadPhys8(0x100010) = %02X，應為 22（實體位址不遮 A20）", got)
	}
	m.WritePhys8(0x100010, 0x77)
	if m.Ext[0x10] != 0x77 || m.Mem[0x10] != 0x11 {
		t.Error("WritePhys8 應寫進 Ext")
	}
	if m.ReadPhys32(0x100010) != 0x77|uint32(m.Ext[0x11])<<8|uint32(m.Ext[0x12])<<16|uint32(m.Ext[0x13])<<24 {
		t.Error("ReadPhys32 小端")
	}
}

// ROM 保護與平面視窗在轉換開啟時照樣有效（同一份實體層本體）。
func TestLinearUsesPhysLayerSideEffects(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.SetA20(true)
	before, old := m.ROMWrites, m.Mem[ROMBase+4]
	m.Write8(ROMBase+4, old^0xFF)
	if m.ROMWrites != before+1 || m.Mem[ROMBase+4] != old {
		t.Errorf("轉換開啟時 ROM 寫入應被丟棄並計數：ROMWrites %d→%d", before, m.ROMWrites)
	}
	var reads int
	m.WatchReads(0x2000, 0x2000, func(uint32, uint8) { reads++ })
	m.Read8(0x2000)
	if reads != 1 {
		t.Errorf("讀取監看比對實體位址，轉換開啟時也要觸發：%d", reads)
	}
	var writes int
	m.WatchWrites(0x3000, 0x3000, func(uint32, uint8, uint8) { writes++ })
	m.Write8(0x3000, 9)
	if writes != 1 {
		t.Errorf("寫入監看：%d", writes)
	}
	// 平面模式：A0000 視窗進 VGA，不進 Mem。
	m.SetVideoMode(0x0D)
	m.Write8(vgaLo+3, 0x55)
	if m.Mem[vgaLo+3] == 0x55 {
		t.Error("平面模式的 A0000 寫入不該進 Mem")
	}
}

// ---- 監督者存取（§3.3）-----------------------------------------------------

func TestSupervisorAccessRoundTrip(t *testing.T) {
	for _, xlat := range []bool{false, true} {
		m := newPM(t)
		if xlat {
			mustNil(t, "SetCR0", m.SetCR0(cr0PE))
			mustNil(t, "SetPMCPL", m.SetPMCPL(3))
		}
		m.SetA20(true)
		m.WriteSup32(0x2000, 0x44332211)
		m.WriteSup16(0x2004, 0x6655)
		m.WriteSup8(0x2006, 0x77)
		for i, want := range []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77} {
			if got := m.Read8(0x2000 + uint32(i)); got != want {
				t.Errorf("xlat=%v Read8(+%d) = %02X，應為 %02X", xlat, i, got, want)
			}
			if got := m.ReadSup8(0x2000 + uint32(i)); got != want {
				t.Errorf("xlat=%v ReadSup8(+%d) = %02X", xlat, i, got)
			}
		}
		if m.ReadSup16(0x2001) != 0x3322 || m.ReadSup32(0x2000) != 0x44332211 {
			t.Errorf("xlat=%v：ReadSup16／32 小端", xlat)
		}
		// 跨 1 MB 邊界：低兩個 byte 在 ROM 區（寫入被丟棄），高兩個 byte 進 Ext。
		rom := m.ReadSup16(MemSize - 2)
		m.WriteSup32(MemSize-2, 0x04030201)
		if m.Ext[0] != 3 || m.Ext[1] != 4 {
			t.Errorf("xlat=%v：跨界寫入的高半應進 Ext：%02X %02X", xlat, m.Ext[0], m.Ext[1])
		}
		if got := m.ReadSup32(MemSize - 2); got != 0x04030000|uint32(rom) {
			t.Errorf("xlat=%v：跨界 ReadSup32 = %08X，ROM 半邊應維持 %04X", xlat, got, rom)
		}
	}
}

// ---- SvcAddr（§4）-----------------------------------------------------------

func TestSvcAddr(t *testing.T) {
	off := newExt(t, 4096)
	if got, want := off.SvcAddr(0xFFFF, 0x10), cpu.Addr(0xFFFF, 0x10); got != want || got != 0 {
		t.Errorf("轉換關閉：SvcAddr = %X，應等於 cpu.Addr = %X", got, want)
	}
	on := newPM(t)
	if on.SvcAddr(0xFFFF, 0x10) != 0 {
		t.Error("profile 開但 PE=0：維持 cpu.Addr（20 位元遮罩）")
	}
	mustNil(t, "SetCR0", on.SetCR0(cr0PE))
	on.SetA20(true)
	if got := on.SvcAddr(0xFFFF, 0x10); got != 0x100000 || got == cpu.Addr(0xFFFF, 0x10) {
		t.Errorf("轉換開啟：SvcAddr(FFFF:0010) = %X，應為 0x100000 且不同於 cpu.Addr", got)
	}
	if got := on.SvcAddr(0x1234, 0x0056); got != 0x12396 {
		t.Errorf("一般位址：%X", got)
	}
	if got := on.SvcAddr(0xFFFF, 0xFFFF); got != cpu.Linear(0xFFFF, 0xFFFF) {
		t.Errorf("不遮罩：%X", got)
	}
}

// ---- 觀測（§6、§10 第 8 項）-------------------------------------------------

func TestPeekHasNoSideEffects(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.SetA20(true)
	m.Ext[0x20] = 0x5A
	var reads int
	m.WatchReads(0, 0x1FFFFF, func(uint32, uint8) { reads++ })
	if v, ok := m.PeekOK(0x100020); v != 0x5A || !ok {
		t.Errorf("PeekOK = (%02X, %v)", v, ok)
	}
	if v, ok := m.PeekOK16(0x100020); v != 0x5A|uint16(m.Ext[0x21])<<8 || !ok {
		t.Errorf("PeekOK16 = (%04X, %v)", v, ok)
	}
	if m.Peek8(0x100020) != 0x5A || m.Peek16(0x100020) != 0x5A|uint16(m.Ext[0x21])<<8 {
		t.Error("Peek8／Peek16")
	}
	if reads != 0 {
		t.Errorf("Peek 不得觸發讀取監看（%d 次）", reads)
	}
	// 轉換開啟時 Peek 也套 A20。
	m.SetA20(false)
	m.Mem[0x20] = 0x33
	if got := m.Peek8(0x100020); got != 0x33 {
		t.Errorf("A20 關：Peek8(0x100020) = %02X，應折回 0x20", got)
	}
	// Ext 之外讀 FF，但在沒有分頁時 ok 為 true（沒有『頁不存在』）。
	m.SetA20(true)
	if v, ok := m.PeekOK(uint32(ExtBase+len(m.Ext)) + 1); v != 0xFF || !ok {
		t.Errorf("Ext 之外：PeekOK = (%02X, %v)", v, ok)
	}
	// 平面視窗：Peek 回 0 且不載入 latch。
	m.SetVideoMode(0x0D)
	m.VGA.latch = [4]uint8{1, 2, 3, 4}
	if m.Peek8(vgaLo+5) != 0 || m.VGA.latch != [4]uint8{1, 2, 3, 4} {
		t.Error("Peek 不得載入 VGA latch")
	}
}

func TestPeekOKAlwaysTrueWhenNotTranslating(t *testing.T) {
	m := newExt(t, 4096)
	if v, ok := m.PeekOK(0x5000000); !ok || v != m.Peek8(0x5000000) {
		t.Errorf("轉換關閉時 PeekOK 恆為 true：(%02X, %v)", v, ok)
	}
}

// 字組監看：轉換關閉維持 Read16（會觸發讀取監看），轉換開啟改 Peek16（不會）。
func TestPollWordsReadVersusPeek(t *testing.T) {
	m := newPM(t)
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0x90, 0xEB, 0xFD})) // nop; jmp short -3
	m.IRQ0Every = 0
	var reads int
	m.WatchReads(0x5000, 0x5001, func(uint32, uint8) { reads++ })
	m.WatchWord(0x5000, func(*Machine, uint32, uint16, uint16) {})
	base := reads // 登記時取樣（轉換關閉：Read16）
	if base != 2 {
		t.Fatalf("轉換關閉時 WatchWord 登記用 Read16，應觸發 2 次讀取監看，實際 %d", base)
	}
	for i := 0; i < 3; i++ {
		mustNil(t, "Step", m.Step())
	}
	if reads != base+6 {
		t.Errorf("轉換關閉：每步 pollWords 用 Read16（2 byte），讀取監看 %d→%d", base, reads)
	}
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustNil(t, "Step（xlat 改變的那一步）", m.Step())
	at := reads
	for i := 0; i < 3; i++ {
		mustNil(t, "Step", m.Step())
	}
	if reads != at {
		// 取指令也走 Read8，但取指令位址不在 0x5000，所以監看不會觸發。
		t.Errorf("轉換開啟：pollWords 改用 Peek16，不得觸發讀取監看（多了 %d 次）", reads-at)
	}
	// 登記新的 WatchWord 同樣用 Peek16。
	m.WatchWord(0x5000, func(*Machine, uint32, uint16, uint16) {})
	if reads != at {
		t.Errorf("轉換開啟：登記 WatchWord 不得觸發讀取監看")
	}
}

// §10.8：m.xlat 改變的那一步重新取樣 last 值，不發出變化通知。
func TestWordWatchNoNotifyWhenXlatChanges(t *testing.T) {
	m := newPM(t)
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0x90, 0xEB, 0xFD}))
	m.IRQ0Every = 0
	m.SetA20(false)
	m.Mem[0x30], m.Mem[0x31] = 0x11, 0x11 // A20 關：線性 0x100030 折回 0x30
	m.Ext[0x30], m.Ext[0x31] = 0x22, 0x22 // 實體 0x100030
	type chg struct{ old, new uint16 }
	var got []chg
	m.WatchWord(0x100030, func(_ *Machine, _ uint32, o, n uint16) { got = append(got, chg{o, n}) })

	mustNil(t, "Step", m.Step())
	// 『mov cr0』與開 A20 發生在這一步：視角由 0x1111（折回）變成 0x2222（Ext）。
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.SetA20(true)
	mustNil(t, "Step（xlat 改變）", m.Step())
	if len(got) != 0 {
		t.Fatalf("xlat 改變那一步不得發變化通知：%v", got)
	}
	mustNil(t, "Step", m.Step())
	if len(got) != 0 {
		t.Fatalf("重新取樣之後值沒變，不得通知：%v", got)
	}
	// 之後客體真的改了值才通知。
	m.Ext[0x30] = 0x33
	mustNil(t, "Step", m.Step())
	if len(got) != 1 || got[0] != (chg{0x2222, 0x2233}) {
		t.Errorf("xlat 穩定之後的真實變化要通知：%v", got)
	}
}

// ---- 取指令快路徑（§5.4）----------------------------------------------------

func TestSyncCodeFastPathCriteria(t *testing.T) {
	m := newPM(t)
	if m.CPU.Code == nil || &m.CPU.Code[0] != &m.Mem[0] {
		t.Fatal("全部關閉時 CPU.Code 應指向 Mem")
	}
	// 轉換開啟（A20 關）。
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	if m.CPU.Code != nil {
		t.Error("轉換開啟時 CPU.Code 一律停用")
	}
	mustNil(t, "SetCR0", m.SetCR0(0))
	if m.CPU.Code == nil {
		t.Error("轉換回到關閉後 CPU.Code 應恢復")
	}
	// A20。
	m.SetA20(true)
	if m.CPU.Code != nil {
		t.Error("A20 開時 CPU.Code 停用")
	}
	m.SetA20(false)
	// 讀取監看。
	m.WatchReads(0x10, 0x20, func(uint32, uint8) {})
	if m.CPU.Code != nil {
		t.Error("讀取監看開啟時 CPU.Code 停用")
	}
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	m.WatchReads(0, 0, nil)
	if m.CPU.Code != nil {
		t.Error("讀取監看關閉但轉換開啟：仍停用")
	}
	mustNil(t, "SetCR0", m.SetCR0(0))
	if m.CPU.Code == nil {
		t.Error("三個條件都關閉：恢復")
	}
}

// ---- 狀態檔與快照（§7、§10 第 6 項）-----------------------------------------

// TestPMV1Layout 以手造的版本 1 位元組驗解碼相容（433 §2.1：版本 1 只剩讀取；缺的欄位取 SysRegs 初值；
// 434 §2.2：encodePM 輸出版本 3）。版本 2、3 的位移在 sysregs_test.go。
// 434 §2.1：經 LoadState 的版本 1 不得帶 PE=1、VM=0，所以這裡用 VM=1（EFlagsHi＝RF|VM）。
func TestPMV1Layout(t *testing.T) {
	// 逐 byte（小端序）：版本 1、PMProfile 1、pmCPL 2、CR0 80000019h、CR3 DEADB000h、EFlagsHi RF|VM、保留 0。
	v1 := []byte{1, 0, 1, 2, 0x19, 0, 0, 0x80, 0x00, 0xB0, 0xAD, 0xDE, 3, 0, 0, 0}
	f, err := decodePM(v1)
	mustNil(t, "decodePM(版本 1)", err)
	le := binary.LittleEndian
	for _, c := range []struct {
		name      string
		got, want uint64
	}{
		{"版本號（位移 0，2 bytes）", uint64(le.Uint16(v1[0:])), 1},
		{"pmCPL（位移 3）", uint64(f.cpl), 2},
		{"CR0（位移 4，4 bytes）", uint64(f.cr0), 0x80000019},
		{"CR3（位移 8，4 bytes）", uint64(f.cr3), 0xDEADB000},
		{"EFlagsHi（位移 12，2 bytes）", uint64(f.eflagsHi), eflagsHiRF | eflagsHiVM},
	} {
		if c.got != c.want {
			t.Errorf("%s = %X，應為 %X", c.name, c.got, c.want)
		}
	}
	if f.sysRegs != sysRegsInit() {
		t.Errorf("版本 1 缺的 SysRegs 應取初值：%+v", f.sysRegs)
	}
	if f.segmented || f.cache != [6]SegCache{} {
		t.Errorf("版本 1 沒有分段模式狀態：segmented=%v cache=%+v", f.segmented, f.cache)
	}
	// 版本 1 長度不足仍回錯誤。
	if _, err := decodePM(v1[:15]); err == nil {
		t.Error("版本 1 只有 15 bytes：應回錯誤")
	}
	// 經 LoadState 讀版本 1 的狀態檔，再存檔變成版本 4（屬預期；459 §5.3 的升級）。
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	st := decodeStateForTest(t, saveBytes(t, src))
	st.PM = v1
	preV4State(&st) // 459：版本 1 的狀態檔記憶體是舊版的 stub 區
	m := newPM(t)
	m.SetDR(7, 0xFF) // 先弄髒，證明讀檔後回初值
	mustNil(t, "LoadState(版本 1)", m.LoadState(bytes.NewReader(encodeStateForTest(t, st))))
	if m.CR0() != 0x80000019 || m.CR3() != 0xDEADB000 || m.PMCPL() != 2 || m.EFlagsHi() != eflagsHiRF|eflagsHiVM {
		t.Errorf("版本 1 讀入：CR0=%08X CR3=%X CPL=%d hi=%X", m.CR0(), m.CR3(), m.PMCPL(), m.EFlagsHi())
	}
	if m.sys != sysRegsInit() {
		t.Errorf("版本 1 讀入後 SysRegs 應為初值：%+v", m.sys)
	}
	// 459 §5.1、§5.3：再存為版本 4（佇列空，148 bytes），升級血統 bit 5 為 1。
	if b := m.encodePM(); len(b) != pmV4Size || le.Uint16(b) != 4 || b[142]&pmfUpgraded == 0 {
		t.Errorf("版本 1 讀入後再存應為版本 4（148 bytes、升級血統）：%d bytes 版本 %d", len(b), le.Uint16(b))
	}
	// profile 關閉時為 nil。
	off := newExt(t, 4096)
	if off.encodePM() != nil {
		t.Error("profile 關閉時 PM 應為 nil")
	}
}

func TestPMRoundTripThroughSaveState(t *testing.T) {
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	mustNil(t, "SetCR0", src.SetCR0(0x80000000|cr0PE))
	mustNil(t, "SetCR3", src.SetCR3(0x12345000))
	mustNil(t, "SetPMCPL", src.SetPMCPL(3))
	mustNil(t, "SetEFlagsHi", src.SetEFlagsHi(eflagsHiRF|eflagsHiVM))
	data := saveBytes(t, src)
	if s := decodeStateForTest(t, data); len(s.PM) != 148 { // 459 §5.1：版本 4，佇列空時 148
		t.Fatalf("存檔後 PM 應有 148 bytes（版本 4）：%d", len(s.PM))
	}

	dst := newPM(t)
	gen := dst.tlbGen
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	if dst.CR0() != src.CR0() || dst.CR3() != 0x12345000 || dst.PMCPL() != 3 || dst.EFlagsHi() != eflagsHiRF|eflagsHiVM {
		t.Errorf("往返後欄位不同：CR0=%08X CR3=%X CPL=%d hi=%X", dst.CR0(), dst.CR3(), dst.PMCPL(), dst.EFlagsHi())
	}
	if !dst.Translating() || !dst.cplUser || dst.CPU.Code != nil {
		t.Errorf("衍生欄位未重算：xlat=%v cplUser=%v", dst.xlat, dst.cplUser)
	}
	if dst.tlbGen == gen {
		t.Error("LoadState 之後 TLB 世代要加一")
	}
	if !bytes.Equal(saveBytes(t, dst), data) {
		t.Error("存檔→讀檔→存檔不一致")
	}
}

func TestLoadStatePMProfileMismatch(t *testing.T) {
	on := newPM(t)
	mustNil(t, "LoadCOM", on.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	onData := saveBytes(t, on)
	off := newExt(t, 4096)
	mustNil(t, "LoadCOM", off.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	offData := saveBytes(t, off)

	// 狀態檔有 PM、機器 profile 關閉。
	m := newExt(t, 4096)
	before := saveBytes(t, m)
	err := m.LoadState(bytes.NewReader(onData))
	if err == nil || !strings.Contains(err.Error(), "profile") {
		t.Errorf("狀態檔 profile 開、機器關：應回錯誤，實際 %v", err)
	}
	if !bytes.Equal(before, saveBytes(t, m)) {
		t.Error("失敗的 LoadState 不得改變機器")
	}
	// 狀態檔沒有 PM、機器 profile 開啟。
	m = newPM(t)
	before = saveBytes(t, m)
	err = m.LoadState(bytes.NewReader(offData))
	if err == nil || !strings.Contains(err.Error(), "profile") {
		t.Errorf("狀態檔 profile 關、機器開：應回錯誤，實際 %v", err)
	}
	if !bytes.Equal(before, saveBytes(t, m)) {
		t.Error("失敗的 LoadState 不得改變機器")
	}
}

func TestLoadStatePMValidation(t *testing.T) {
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	mustNil(t, "SetCR0", src.SetCR0(cr0PE))
	good := saveBytes(t, src)
	base := decodeStateForTest(t, good)

	mut := func(f func(pm []byte) []byte) []byte {
		s := base
		s.PM = f(append([]byte(nil), base.PM...))
		return encodeStateForTest(t, s)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"未知版本號", mut(func(p []byte) []byte { p[0] = 5; return p })}, // 459：4 已是合法版本
		{"版本號 0", mut(func(p []byte) []byte { p[0] = 0; return p })},
		{"長度小於版本 3 的大小", mut(func(p []byte) []byte { return p[:141] })},
		{"版本 3 只有版本 2 的長度", mut(func(p []byte) []byte { return p[:80] })},
		{"版本 3 只有版本 1 的長度", mut(func(p []byte) []byte { return p[:16] })},
		{"版本 2 長度不足", mut(func(p []byte) []byte { p[0] = 2; return p[:79] })},
		{"版本 1 長度不足", mut(func(p []byte) []byte { p[0] = 1; return p[:15] })},
		{"長度連版本號都不夠", mut(func(p []byte) []byte { return p[:1] })},
		{"PMProfile 不為 1", mut(func(p []byte) []byte { p[2] = 0; return p })},
		{"PMProfile 為 2", mut(func(p []byte) []byte { p[2] = 2; return p })},
		{"pmCPL > 3", mut(func(p []byte) []byte { p[3] = 4; return p })},
		{"CR0 含保留位元 5", mut(func(p []byte) []byte { p[4] |= 0x20; return p })},
		{"CR0 含 WP（bit 16）", mut(func(p []byte) []byte { p[6] |= 0x01; return p })},
		{"CR0 含 bit 30", mut(func(p []byte) []byte { p[7] |= 0x40; return p })},
		{"EFlagsHi 含 AC", mut(func(p []byte) []byte { p[12] |= 0x04; return p })},
		{"EFlagsHi 含 ID", mut(func(p []byte) []byte { p[12] |= 0x20; return p })},
		{"EFlagsHi 高位元", mut(func(p []byte) []byte { p[13] = 1; return p })},
		{"VM=1 而 PE=0", mut(func(p []byte) []byte { p[4] &^= 1; p[12] |= 2; return p })},
	}
	for _, c := range cases {
		m := newPM(t)
		before := saveBytes(t, m)
		if err := m.LoadState(bytes.NewReader(c.data)); err == nil {
			t.Errorf("%s：應回錯誤", c.name)
		}
		if !bytes.Equal(before, saveBytes(t, m)) {
			t.Errorf("%s：失敗的 LoadState 不得改變機器", c.name)
		}
	}
	// 對照組：沒改動的狀態檔載得進去；保留欄位與較長的 PM 不影響。
	m := newPM(t)
	mustNil(t, "原狀態檔", m.LoadState(bytes.NewReader(good)))
	m = newPM(t)
	s := base
	s.PM = append(append([]byte(nil), base.PM...), 0xAA, 0xBB)
	mustNil(t, "尾端多餘 bytes", m.LoadState(bytes.NewReader(encodeStateForTest(t, s))))
	if !m.Translating() {
		t.Error("載入 PE=1 的狀態後 xlat 應為真")
	}
}

func TestSnapshotRestorePM(t *testing.T) {
	m := newPM(t)
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	realSnap := m.Snapshot() // PE=0
	mustNil(t, "SetCR0", m.SetCR0(0x80000000|cr0PE))
	mustNil(t, "SetCR3", m.SetCR3(0x7000))
	mustNil(t, "SetPMCPL", m.SetPMCPL(3))
	mustNil(t, "SetEFlagsHi", m.SetEFlagsHi(eflagsHiVM))
	protSnap := m.Snapshot() // PE=1

	// PE=1 的機器 Restore PE=0 的快照。
	gen := m.tlbGen
	m.Restore(realSnap)
	if m.CR0() != pmCR0Init || m.CR3() != 0 || m.PMCPL() != 0 || m.EFlagsHi() != 0 {
		t.Errorf("Restore(PE=0)：CR0=%08X CR3=%X CPL=%d hi=%X", m.CR0(), m.CR3(), m.PMCPL(), m.EFlagsHi())
	}
	if m.Translating() || m.cplUser || m.paging {
		t.Error("Restore(PE=0)：衍生欄位應全為假")
	}
	if m.tlbGen == gen {
		t.Error("Restore 之後 TLB 世代要加一")
	}
	if m.CPU.Code == nil || &m.CPU.Code[0] != &m.Mem[0] {
		t.Error("Restore(PE=0)：CPU.Code 應恢復指向 Mem")
	}
	// 反向：PE=0 的機器 Restore PE=1 的快照。
	gen = m.tlbGen
	m.Restore(protSnap)
	if m.CR0() != 0x80000000|cr0PE|pmCR0Init || m.CR3() != 0x7000 || m.PMCPL() != 3 || m.EFlagsHi() != eflagsHiVM {
		t.Errorf("Restore(PE=1)：CR0=%08X CR3=%X CPL=%d hi=%X", m.CR0(), m.CR3(), m.PMCPL(), m.EFlagsHi())
	}
	if !m.Translating() || !m.cplUser {
		t.Error("Restore(PE=1)：xlat、cplUser 應為真")
	}
	if m.tlbGen == gen {
		t.Error("Restore 之後 TLB 世代要加一")
	}
	if m.CPU.Code != nil {
		t.Error("Restore(PE=1)：CPU.Code 應停用")
	}
}

func TestRestoreProfileMismatchPanics(t *testing.T) {
	on := newPM(t)
	snapOn := on.Snapshot()
	off := newExt(t, 4096)
	snapOff := off.Snapshot()
	for _, c := range []struct {
		name string
		m    *Machine
		s    *Snapshot
	}{
		{"機器 profile 關、快照開", off, snapOn},
		{"機器 profile 開、快照關", on, snapOff},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s：應 panic", c.name)
					return
				}
				msg, _ := r.(string)
				if !strings.Contains(msg, "profile") || !strings.Contains(msg, "true") || !strings.Contains(msg, "false") {
					t.Errorf("%s：panic 訊息要寫明兩邊設定：%q", c.name, msg)
				}
			}()
			c.m.Restore(c.s)
		}()
	}
}

// profile 關閉時語意不變：存檔的 PM 為 nil，Snapshot／Restore 往返不動 CR0 初值。
func TestProfileOffStateUnchanged(t *testing.T) {
	m := New()
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	if s := decodeStateForTest(t, saveBytes(t, m)); s.PM != nil {
		t.Error("profile 關閉時 PM 應為 nil")
	}
	snap := m.Snapshot()
	m.Restore(snap)
	if m.CR0() != pmCR0Init || m.Translating() {
		t.Errorf("Restore 之後 CR0 = %08X", m.CR0())
	}
}
