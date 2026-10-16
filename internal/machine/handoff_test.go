package machine

import (
	"errors"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/434-legacy-core-segmented-mode §9 第 4 項：進入分段模式（取代 433 §8 第 5 項的交棒測試）。

// handoffProg：mov cr0,eax（EAX=1，PE 0→1）；之後是 nop。
func handoffProg(t *testing.T) *Machine {
	t.Helper()
	m := newSysProg(t, []byte{0x0F, 0x22, 0xC0, 0x90, 0x90})
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x0001, 0
	return m
}

// realModeCaches 是 434 §3.1 第 1 項的實模式推得值。
func realModeCaches(c *cpu.CPU) [6]SegCache {
	vals := [6]uint16{}
	vals[cpu.ES], vals[cpu.CS], vals[cpu.SS], vals[cpu.DS] = c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS]
	vals[cpu.FS], vals[cpu.GS] = c.FSeg, c.GSeg
	var want [6]SegCache
	for i, v := range vals {
		attr := uint16(0x0093)
		if i == cpu.CS {
			attr = 0x009B
		}
		want[i] = SegCache{Base: uint32(v) << 4, Limit: 0xFFFF, Attr: attr}
	}
	return want
}

func TestEnterSegmentedOnPEEdge(t *testing.T) {
	m := handoffProg(t)
	c := m.CPU
	// 六個段值各不相同（FS、GS 走 FSeg、GSeg）；mov cr0 不碰記憶體運算元。
	c.Seg[cpu.ES], c.Seg[cpu.SS], c.Seg[cpu.DS] = 0x1111, 0x3333, 0x4444
	c.FSeg, c.GSeg = 0x5555, 0x6666
	if c.Segmented() {
		t.Fatal("實模式下 segmented 應為 false")
	}
	ip, steps := c.IP, m.Steps
	mustNil(t, "mov cr0 設 PE 的那一步", m.Step())
	if !c.Segmented() {
		t.Fatal("PE 由 0 變 1 後應進分段模式")
	}
	if m.Steps != steps+1 || c.IP != ip+3 || m.CR0()&cr0PE == 0 {
		t.Errorf("mov cr0 照常完成：Steps +%d、IP +%d、CR0=%08X", m.Steps-steps, c.IP-ip, m.CR0())
	}
	want := realModeCaches(c)
	for i := range want {
		if got := c.SegCache(i); got != want[i] {
			t.Errorf("快取 %d = %+v，應為實模式推得值 %+v", i, got, want[i])
		}
	}
	if m.PMCPL() != 0 {
		t.Errorf("進分段模式後 CPL = %d，應為 0", m.PMCPL())
	}
	if c.Code != nil || !m.Translating() {
		t.Error("不變式：segmented ⇒ xlat ⇒ CPU.Code == nil")
	}
	// 負對照：FS、GS 的快取來自 FSeg／GSeg（與 ES、DS 對調會不同）。
	if want[cpu.FS] == want[cpu.ES] || want[cpu.GS] == want[cpu.DS] {
		t.Error("負對照：六個段值應各不相同")
	}
}

// 進入前 CPL 為 3（合成）：enterSegmented 把它寫回 0。
func TestEnterSegmentedResetsCPL(t *testing.T) {
	m := handoffProg(t)
	m.pmCPL = 3 // 實模式下 CPL 沒有意義；直接寫欄位造出非 0 的值
	mustNil(t, "mov cr0 設 PE", m.Step())
	if !m.CPU.Segmented() || m.PMCPL() != 0 || m.cplUser {
		t.Errorf("enterSegmented 應 SetPMCPL(0)：segmented=%v CPL=%d cplUser=%v", m.CPU.Segmented(), m.PMCPL(), m.cplUser)
	}
}

// lmsw 設 PE 同樣進分段模式。
func TestEnterSegmentedViaLMSW(t *testing.T) {
	m := newSysProg(t, []byte{0x0F, 0x01, 0xF0, 0x90})
	m.CPU.R[cpu.AX] = 0x0001
	mustNil(t, "lmsw 1", m.Step())
	if !m.CPU.Segmented() {
		t.Error("lmsw 設 PE 後應進分段模式")
	}
}

// pfSys 包住 SysBus：SetCR 之後注入一筆分頁紀錄（模擬同一步的分頁錯誤）。
type pfSys struct{ *Machine }

func (p pfSys) SetCR(n int, v uint32) error {
	err := p.Machine.SetCR(n, v)
	p.Machine.notePageFault(0x12345678, 0, 0, true, false)
	return err
}

// 同一步有分頁紀錄時回 *PageFaultError，且已進分段模式（434 §3.1 第 2 項）。
func TestEnterSegmentedWithPageFault(t *testing.T) {
	m := handoffProg(t)
	m.CPU.Sys = pfSys{m}
	err := m.Step()
	var pfe *PageFaultError
	if !errors.As(err, &pfe) || !errors.Is(err, cpu.ErrModeSwitch) {
		t.Fatalf("同一步有分頁紀錄時回 *PageFaultError（包住哨兵），實際 %v", err)
	}
	if !m.CPU.Segmented() {
		t.Error("回 *PageFaultError 時應已進分段模式")
	}
	if got, want := m.CPU.SegCache(cpu.CS), realModeCaches(m.CPU)[cpu.CS]; got != want {
		t.Errorf("CS 快取 = %+v，應為 %+v", got, want)
	}
}

// SetCR0 直接設 PE 不進分段模式（434 §2.1 的狀態照 432 語意執行）。
func TestSetCR0DoesNotEnterSegmented(t *testing.T) {
	m := newSysProg(t, []byte{0x90, 0x90})
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	if m.CPU.Segmented() {
		t.Fatal("SetCR0 直接設 PE 不得進分段模式")
	}
	mustStep(t, m, "nop（PE=1、VM=0、segmented=false）")
	if m.CPU.Segmented() {
		t.Error("§2.1 的狀態執行之後仍不得進分段模式")
	}
	for i := 0; i < 6; i++ {
		if c := m.CPU.SegCache(i); c != (SegCache{}) {
			t.Errorf("未分段時快取 %d 應為 0：%+v", i, c)
		}
	}
}

// 分段模式下 SetVM(true)、SetEFlagsHi 設 VM、SetPMCPL(≠0) 都被拒，狀態不改（434 §3.2）。
// 清 PE 的部分由 442 修訂：清 PE 且清 PG 進入殘留快取實模式（另見 TestSegmentedClearPE），清 PE 而保留 PG 被拒。
func TestSegmentedGuards(t *testing.T) {
	m := handoffProg(t)
	mustNil(t, "mov cr0 設 PE", m.Step())
	if !m.CPU.Segmented() {
		t.Fatal("前提：應在分段模式")
	}
	cr0, hi, cpl := m.CR0(), m.EFlagsHi(), m.PMCPL()
	mustErr(t, "分段模式下 SetVM(true)", m.SetVM(true))
	mustErr(t, "分段模式下 SetEFlagsHi 設 VM", m.SetEFlagsHi(eflagsHiVM))
	mustErr(t, "分段模式下 SetEFlagsHi 設 RF|VM", m.SetEFlagsHi(eflagsHiRF|eflagsHiVM))
	for _, c := range []uint8{1, 2, 3} {
		mustErr(t, "分段模式下 SetPMCPL(≠0)", m.SetPMCPL(c))
	}
	if m.CR0() != cr0 || m.EFlagsHi() != hi || m.PMCPL() != cpl || !m.CPU.Segmented() {
		t.Errorf("被拒的寫入不得改狀態：CR0 %08X→%08X hi %X→%X CPL %d→%d", cr0, m.CR0(), hi, m.EFlagsHi(), cpl, m.PMCPL())
	}
	// 不涉及離開分段模式的寫入照常：SetPMCPL(0)、只設 RF、PE 保持 1 而設 PG。
	mustNil(t, "SetPMCPL(0)", m.SetPMCPL(0))
	mustNil(t, "SetEFlagsHi(RF)", m.SetEFlagsHi(eflagsHiRF))
	mustNil(t, "SetCR0(PE|PG)", m.SetCR0(cr0PE|cr0PG))
	if !m.CPU.Segmented() || !m.paging {
		t.Error("PE 保持 1 的寫入不得離開分段模式；PG 由 432 的 SetCR0 重算")
	}
	// 442 §2.2：清 PE 而保留 PG 被拒，CR0 不改。
	cr0 = m.CR0()
	mustErr(t, "分段模式下 SetCR0 清 PE 而保留 PG", m.SetCR0(cr0PG))
	mustErr(t, "分段模式下 SetCR(0) 清 PE 而保留 PG", m.SetCR(0, pmCR0Init|cr0PG))
	if m.CR0() != cr0 || !m.CPU.Segmented() {
		t.Errorf("清 PE 而保留 PG 被拒後 CR0（%08X→%08X）與 segmented 不變", cr0, m.CR0())
	}
}

// 442 §2.2（取代 434 的「分段模式下清 PE 被拒」）：SetCR0(0)、SetCR(0, pmCR0Init) 清 PE 且清 PG 被接受，
// 進入殘留快取實模式（Machine 的 setter 不設離開旗標，所以停在子狀態）。mov cr0 清 PE 的那一步若六個快取恰好是
// 實模式推得值（enterSegmented 設的值）就同一步離開（§3 第 4 項）。
func TestSegmentedClearPE(t *testing.T) {
	for _, set := range []struct {
		name string
		f    func(m *Machine) error
	}{
		{"SetCR0(0)", func(m *Machine) error { return m.SetCR0(0) }},
		{"SetCR(0, pmCR0Init)", func(m *Machine) error { return m.SetCR(0, pmCR0Init) }},
	} {
		m := handoffProg(t)
		mustNil(t, "mov cr0 設 PE", m.Step())
		mustNil(t, set.name, set.f(m))
		if !m.CPU.Segmented() || m.CR0() != pmCR0Init || !m.Translating() || m.CPU.Code != nil || m.paging {
			t.Errorf("%s：應進入殘留快取實模式（segmented=%v CR0=%08X xlat=%v Code==nil=%v paging=%v）",
				set.name, m.CPU.Segmented(), m.CR0(), m.Translating(), m.CPU.Code == nil, m.paging)
		}
	}
	// mov cr0 清 PE：enterSegmented 的快取是實模式推得值，清 PE 的那一步就離開。
	m2 := newSysProg(t, []byte{0x0F, 0x22, 0xC0, 0x0F, 0x22, 0xC3, 0x90}) // mov cr0,eax；mov cr0,ebx
	m2.CPU.R[cpu.AX] = 0x0001
	mustNil(t, "mov cr0 設 PE", m2.Step())
	m2.CPU.R[cpu.BX], m2.CPU.Hi[cpu.BX] = 0x0010, 0
	ip := m2.CPU.IP
	mustNil(t, "分段模式下 mov cr0 清 PE（清 PG）", m2.Step())
	if m2.CR0() != pmCR0Init || m2.CPU.Segmented() || m2.Translating() || m2.CPU.IP != ip+3 {
		t.Errorf("mov cr0 清 PE：CR0=%08X segmented=%v xlat=%v IP+%d", m2.CR0(), m2.CPU.Segmented(), m2.Translating(), m2.CPU.IP-ip)
	}
	for i := 0; i < 6; i++ {
		if c := m2.CPU.SegCache(i); c != (SegCache{}) {
			t.Errorf("離開後快取 %d 應為 0：%+v", i, c)
		}
	}
}

// 未分段（PE=0，或 §2.1 的 PE=1、segmented=false）時 SetPMCPL、SetVM 照 432 的規則。
func TestGuardsInactiveWhenNotSegmented(t *testing.T) {
	m := newSysProg(t, []byte{0x90})
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustNil(t, "未分段時 SetPMCPL(3)", m.SetPMCPL(3))
	mustNil(t, "未分段時 SetVM(true)", m.SetVM(true))
	mustNil(t, "SetVM(false)", m.SetVM(false))
	mustNil(t, "SetPMCPL(0)", m.SetPMCPL(0))
	mustNil(t, "未分段時 SetCR0 清 PE", m.SetCR0(0))
}

// SetProtectedModeProfile 切換時離開分段模式、快取清 0（434 §3.2）。CR0 必須是初值才能切換，
// 所以 segmented=true 時切換本身會被擋；這裡以直接寫 CPU 的分段狀態（PE=0）驗清除。
func TestProfileSwitchClearsSegmented(t *testing.T) {
	m := newPM(t)
	m.CPU.SetSegmented(true)
	m.CPU.SetSegCache(cpu.GS, SegCache{Base: 1, Limit: 2, Attr: 0x93})
	mustNil(t, "SetProtectedModeProfile(false)", m.SetProtectedModeProfile(false))
	if m.CPU.Segmented() || m.CPU.SegCache(cpu.GS) != (SegCache{}) {
		t.Error("關閉 profile 後 segmented 應為 false、快取清 0")
	}
	m.CPU.SetSegmented(true)
	m.CPU.SetSegCache(cpu.ES, SegCache{Base: 3})
	mustNil(t, "SetProtectedModeProfile(true)", m.SetProtectedModeProfile(true))
	if m.CPU.Segmented() || m.CPU.SegCache(cpu.ES) != (SegCache{}) {
		t.Error("開啟 profile 後 segmented 應為 false、快取清 0")
	}
}
