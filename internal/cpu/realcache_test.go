package cpu

import (
	"errors"
	"strings"
	"testing"
)

// docs/spec/442-segmented-return-to-real-mode §2.10 的 cpu 層測試：替身的 CR(0) 回 PE=0（fakeSys 的 10h）、
// SetSegmented(true)，即殘留快取實模式。

// rcSys 記下描述元表讀取次數（子狀態的段載入不得讀描述元表）。
type rcSys struct {
	fakeSys
	reads int
}

func (s *rcSys) ReadSup32(uint32) uint32 { s.reads++; return 0 }

// newRCCPU：CS=1234h（快取基底 10000h，不是 1234h×16，所以載入其他段不會離開），SS:SP=2000:0100，
// DS、ES、FS、GS 為實模式推得值。程式放在線性 10000h。
func newRCCPU(t *testing.T, code []byte) (*CPU, *testBus, *rcSys) {
	t.Helper()
	b := newTestBus()
	c := New(b)
	c.Model = Model80386
	c.SetFlags(0)
	s := &rcSys{}
	c.Sys = s
	c.SetSegmented(true)
	c.Seg[CS], c.IP = 0x1234, 0
	c.SetSegCache(CS, SegCache{Base: 0x10000, Limit: 0xFFFF, Attr: 0x9B})
	c.Seg[SS], c.R[SP] = 0x2000, 0x0100
	c.SetSegCache(SS, SegCache{Base: 0x20000, Limit: 0xFFFF, Attr: 0x93})
	for _, i := range []int{ES, DS, FS, GS} {
		c.setSegVal(i, 0x3000)
		c.SetSegCache(i, SegCache{Base: 0x30000, Limit: 0xFFFF, Attr: 0x93})
	}
	copy(b.mem[0x10000:], code)
	if !c.realCache() {
		t.Fatal("前提：fakeSys 的 CR(0) 是 PE=0，應為殘留快取實模式")
	}
	return c, b, s
}

// §2.4：基底＝值×16、界限與屬性保留、SS 清 D/B、不讀描述元表；CS 未是推得值，所以不回哨兵。
func TestRealCacheLoadSegCPU(t *testing.T) {
	c, _, s := newRCCPU(t, []byte{0x8E, 0xD8, 0x8E, 0xD0}) // mov ds,ax；mov ss,ax
	c.R[AX] = 0x0456
	c.SetSegCache(DS, SegCache{Base: 0x30000, Limit: 0x7FFF, Attr: 0x5093})
	if err := c.Step(); err != nil {
		t.Fatalf("mov ds,ax：%v", err)
	}
	if got, want := c.SegCache(DS), (SegCache{Base: 0x4560, Limit: 0x7FFF, Attr: 0x5093}); got != want || c.Seg[DS] != 0x456 {
		t.Errorf("DS=%04X 快取 %+v，應為 %+v（資料段的 D/B 保留）", c.Seg[DS], got, want)
	}
	c.SetSegCache(SS, SegCache{Base: 0x20000, Limit: 0x7FFF, Attr: 0x5093})
	if err := c.Step(); err != nil {
		t.Fatalf("mov ss,ax：%v", err)
	}
	if got, want := c.SegCache(SS), (SegCache{Base: 0x4560, Limit: 0x7FFF, Attr: 0x1093}); got != want {
		t.Errorf("SS 快取 %+v，應為 %+v（清 D/B）", got, want)
	}
	if s.reads != 0 || c.leavePending {
		t.Errorf("不得讀描述元表（%d 次）；旗標應已消費（%v）", s.reads, c.leavePending)
	}
}

// 442 §2.4 第 5 條：子狀態的 CS 載入清 D/B。CS 的 D=1 在子狀態不可達（443 §2.9），所以直接呼叫 loadSegReal，
// 不經 Step 的 32 位元預設（443 §2.1）。
func TestRealCacheLoadSegClearsCSD(t *testing.T) {
	c, _, s := newRCCPU(t, []byte{0x90})
	c.SetSegCache(CS, SegCache{Base: 0x30000, Limit: 0x7FFF, Attr: 0x509B})
	if !c.loadSegReal(CS, 0x0456) {
		t.Fatal("loadSegReal(CS) 應成功")
	}
	if got, want := c.SegCache(CS), (SegCache{Base: 0x4560, Limit: 0x7FFF, Attr: 0x109B}); got != want || c.Seg[CS] != 0x456 {
		t.Errorf("CS=%04X 快取 %+v，應為 %+v（清 D/B，界限保留）", c.Seg[CS], got, want)
	}
	if s.reads != 0 {
		t.Errorf("不得讀描述元表（%d 次）", s.reads)
	}
}

// §2.5：子狀態的 iret 不看 NT，走實模式 iret；六個快取都成立時同一步回 ErrLeaveSegmented。
func TestRealCacheIretIgnoresNT(t *testing.T) {
	c, b, _ := newRCCPU(t, []byte{0xCF})
	c.SetFlags(NT)
	copy(b.mem[0x20100:], []byte{0x40, 0x00, 0x23, 0x01, 0x47, 0x02}) // IP 0040、CS 0123、FLAGS 0247
	c.Seg[SS] = 0x2000
	err := c.Step()
	if !errors.Is(err, ErrLeaveSegmented) {
		t.Fatalf("NT=1 的子狀態 iret 應走實模式 iret 並回 ErrLeaveSegmented，實際 %v", err)
	}
	if c.Seg[CS] != 0x0123 || c.IP != 0x0040 || c.Flags != 0x0247 || c.R[SP] != 0x0106 {
		t.Errorf("CS:IP=%04X:%04X FLAGS=%04X SP=%04X", c.Seg[CS], c.IP, c.Flags, c.R[SP])
	}
	if got := c.SegCache(CS); got.Base != 0x1230 || got.Limit != 0xFFFF || got.Attr != 0x9B {
		t.Errorf("CS 快取 %+v", got)
	}
}

// §2.5：66 CF 失敗即關閉，不走 taskReturn、v86ReturnRead（NT=0、NT=1 各一）。
func TestRealCacheIretdFails(t *testing.T) {
	for _, fl := range []uint16{0, NT} {
		c, _, _ := newRCCPU(t, []byte{0x66, 0xCF})
		c.SetFlags(fl)
		err := c.Step()
		if err == nil || errors.Is(err, ErrTaskReturn) || errors.Is(err, ErrV86Return) || !strings.Contains(err.Error(), "442") {
			t.Errorf("FLAGS=%04X：66 CF 應失敗即關閉並註明 442，實際 %v", fl, err)
		}
	}
}

// §2.5：子狀態的 retf 不做 RPL 檢查（CPL 0、彈出 RPL 3 的 CS 照常）。
func TestRealCacheRetfNoRPL(t *testing.T) {
	c, b, _ := newRCCPU(t, []byte{0xCB})
	copy(b.mem[0x20100:], []byte{0x10, 0x00, 0x23, 0x01}) // IP 0010、CS 0123（RPL 3）
	if err := c.Step(); err != nil && !errors.Is(err, ErrLeaveSegmented) {
		t.Fatalf("子狀態 retf 到 RPL 3：%v", err)
	}
	if c.Seg[CS] != 0x0123 || c.IP != 0x0010 || c.R[SP] != 0x0104 {
		t.Errorf("CS:IP=%04X:%04X SP=%04X", c.Seg[CS], c.IP, c.R[SP])
	}
}

// §2.7：段載入成功但同一步稍後失敗時清除旗標、不回哨兵；之後快取即使符合條件也不離開，直到下一次成功的段載入。
// SS 快取是唯讀資料段（RW 不在離開條件內，§2.3 的已知不一致），所以 call far 載入 CS 後的推入失敗。
func TestRealCacheFlagClearedOnFault(t *testing.T) {
	c, b, _ := newRCCPU(t, []byte{0x9A, 0x00, 0x00, 0x00, 0x05}) // call 0500:0000
	c.SetSegCache(SS, SegCache{Base: 0x20000, Limit: 0xFFFF, Attr: 0x91})
	b.mem[0x5000] = 0x90 // nop
	err := c.Step()
	if err == nil || errors.Is(err, ErrLeaveSegmented) {
		t.Fatalf("推入唯讀 SS 應失敗即關閉（不回哨兵），實際 %v", err)
	}
	if c.leavePending {
		t.Error("指令失敗時旗標應清除")
	}
	if c.Seg[CS] != 0x0500 || !c.leaveReady() {
		t.Fatalf("前提：CS 已載入（%04X），六個快取符合離開條件（%v）", c.Seg[CS], c.leaveReady())
	}
	c.R[SP] = 0x0100
	if err := c.Step(); err != nil {
		t.Errorf("之後的 nop 不得離開（沒有新的段載入）：%v", err)
	}
	// 下一次成功的段載入才離開。
	copy(b.mem[c.SegCache(CS).Base+uint32(c.IP):], []byte{0x8E, 0xC3}) // mov es,bx
	c.R[BX] = 0x3000
	if err := c.Step(); !errors.Is(err, ErrLeaveSegmented) {
		t.Errorf("成功的段載入之後應離開，實際 %v", err)
	}
}

// §2.4 第 1 步：pop ds 讀取失敗時 loadSeg 不寫段值與快取、不標記離開檢查。
func TestRealCacheLoadSkipsOnFault(t *testing.T) {
	c, _, _ := newRCCPU(t, []byte{0x1F}) // pop ds
	c.SetSegCache(SS, SegCache{Base: 0x20000, Limit: 0x0FFF, Attr: 0x93})
	c.R[SP] = 0x1000
	ds, cache := c.Seg[DS], c.SegCache(DS)
	if err := c.Step(); err == nil {
		t.Fatal("pop ds 超出 SS 界限應失敗即關閉")
	}
	if c.Seg[DS] != ds || c.SegCache(DS) != cache || c.leavePending {
		t.Errorf("讀取失敗時 DS（%04X→%04X）與快取不得寫入，旗標不設", ds, c.Seg[DS])
	}
}

// §2.5：pop cs（Model8086 的 0F）在子狀態失敗即關閉。
func TestRealCachePopCSFails(t *testing.T) {
	c, b, _ := newRCCPU(t, []byte{0x0F})
	c.Model = Model8086
	copy(b.mem[0x20100:], []byte{0x00, 0x05})
	if err := c.Step(); err == nil || !strings.Contains(err.Error(), "POP CS") {
		t.Fatalf("子狀態的 POP CS 應失敗即關閉：%v", err)
	}
	if c.Seg[CS] != 0x1234 || c.R[SP] != 0x0100 {
		t.Errorf("CS、SP 不得改變：CS=%04X SP=%04X", c.Seg[CS], c.R[SP])
	}
}

// SetSegmented、SetSegCache 清除旗標（§2.7）。
func TestRealCacheSettersClearFlag(t *testing.T) {
	c, _, _ := newRCCPU(t, nil)
	c.leavePending = true
	c.SetSegCache(ES, c.SegCache(ES))
	if c.leavePending {
		t.Error("SetSegCache 應清除旗標")
	}
	c.leavePending = true
	c.SetSegmented(true)
	if c.leavePending {
		t.Error("SetSegmented 應清除旗標")
	}
}

// §2.7 的離開條件逐項：六個段暫存器各自破壞一項時不成立；不比較的欄位（RW、可執行、DPL、accessed、AVL、G、
// 資料段的 B）改變時仍成立。
func TestLeaveReadyConditions(t *testing.T) {
	ready := func() *CPU {
		c, _, _ := newRCCPU(t, nil)
		c.Seg[CS] = 0x1000
		c.SetSegCache(CS, SegCache{Base: 0x10000, Limit: 0xFFFF, Attr: 0x9B})
		return c
	}
	if !ready().leaveReady() {
		t.Fatal("前提：六個快取都是實模式推得值時成立")
	}
	type mod struct {
		name string
		f    func(sc *SegCache)
		ok   bool
	}
	mods := []mod{
		{"基底不是值×16", func(sc *SegCache) { sc.Base += 0x10 }, false},
		{"界限 FFFEh", func(sc *SegCache) { sc.Limit = 0xFFFE }, false},
		{"界限 FFFFFh", func(sc *SegCache) { sc.Limit = 0xFFFFF }, false},
		{"P=0", func(sc *SegCache) { sc.Attr &^= attrP }, false},
		{"S=0", func(sc *SegCache) { sc.Attr &^= attrS }, false},
		{"AVL、G、accessed、DPL 3", func(sc *SegCache) { sc.Attr ^= attrAVL | attrG | attrAccessed | 0x60 }, true},
	}
	for i := ES; i <= GS; i++ {
		for _, md := range mods {
			c := ready()
			sc := c.SegCache(i)
			md.f(&sc)
			c.SetSegCache(i, sc)
			if got := c.leaveReady(); got != md.ok {
				t.Errorf("%s %s：leaveReady=%v，應為 %v", segNames[i], md.name, got, md.ok)
			}
		}
		c := ready()
		sc := c.SegCache(i)
		sc.Attr |= attrDB
		c.SetSegCache(i, sc)
		if got, want := c.leaveReady(), i != CS && i != SS; got != want {
			t.Errorf("%s D/B=1：leaveReady=%v，應為 %v（只檢查 CS、SS）", segNames[i], got, want)
		}
		if i == CS {
			continue
		}
		c = ready()
		c.SetSegCache(i, SegCache{Base: c.SegCache(i).Base, Limit: 0xFFFF, Attr: 0x97}) // 向下展開資料段
		if c.leaveReady() {
			t.Errorf("%s 向下展開的資料段：不應成立", segNames[i])
		}
		c = ready()
		c.SetSegCache(i, SegCache{Base: c.SegCache(i).Base, Limit: 0xFFFF, Attr: 0x9F}) // 一致可讀碼段：EC 不比
		if !c.leaveReady() {
			t.Errorf("%s 一致碼段（EC 是 conforming）：應成立", segNames[i])
		}
		c = ready()
		c.SetSegCache(i, SegCache{Base: c.SegCache(i).Base, Limit: 0xFFFF, Attr: 0x91}) // 唯讀：RW 不比
		if !c.leaveReady() {
			t.Errorf("%s 唯讀資料段：應成立（RW 不比較）", segNames[i])
		}
	}
}
