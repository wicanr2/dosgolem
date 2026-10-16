package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/438 §3 第 2 項：PCPhys。

func setPC(m *Machine, cs, ip uint16) {
	m.CPU.Seg[cpu.CS] = cs
	m.CPU.IP = ip
}

func TestPCPhysTranslationOff(t *testing.T) {
	m := New()
	setPC(m, 0xB000, 0x0010)
	p, ok := m.PCPhys()
	if !ok || p != m.PCLinear() {
		t.Fatalf("轉換關閉：PCPhys=(%X,%v)，應等於 PCLinear=%X", p, ok, m.PCLinear())
	}
}

func TestPCPhysNoPaging(t *testing.T) {
	m, tb := newPaged(t)
	// CR3 指向全空的頁目錄：若錯走頁表會回 false。
	if err := m.SetCR3(tb.dir); err != nil {
		t.Fatal(err)
	}
	if err := m.SetCR0(cr0PE); err != nil {
		t.Fatal(err)
	}
	setPC(m, 0xB000, 0x0010)
	if p, ok := m.PCPhys(); !ok || p != 0xB0010 {
		t.Fatalf("PG=0：PCPhys=(%X,%v)，應為 (B0010,true)", p, ok)
	}
	m.SetA20(false)
	setPC(m, 0xFFFF, 0x0020) // 線性 100010
	if p, ok := m.PCPhys(); !ok || p != 0x000010 {
		t.Fatalf("A20 關閉：PCPhys=(%X,%v)，應為 (10,true)", p, ok)
	}
}

func TestPCPhysPaging(t *testing.T) {
	m, tb := newPaged(t)
	tb.identity(0, 0x100000, fPWU)
	tb.mapPage(0xB0000, 0x200000)
	tb.enable(t)
	before := tb.pte(0xB0000)
	tlbBefore, genBefore := m.tlb, m.tlbGen
	setPC(m, 0xB000, 0x0010)
	if p, ok := m.PCPhys(); !ok || p != 0x200010 {
		t.Fatalf("分頁：PCPhys=(%X,%v)，應為 (200010,true)", p, ok)
	}
	if after := tb.pte(0xB0000); after != before {
		t.Errorf("PTE 被改：%08X → %08X", before, after)
	}
	if m.tlbGen != genBefore || (tlbBefore == nil) != (m.tlb == nil) {
		t.Errorf("TLB 被改")
	}
	noFault(t, m, "PCPhys")

	tb.setPTE(0xB0000, 0)
	if p, ok := m.PCPhys(); ok {
		t.Fatalf("頁不存在：PCPhys=(%X,%v)，應回 false", p, ok)
	}
	noFault(t, m, "PCPhys 頁不存在")
}
