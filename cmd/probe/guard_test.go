package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/438 §3 第 2 項的 probe 層：pcGuard。

// pagedMachine 造一台分頁開啟的機器：低 1 MB 恆等映射，線性頁 B0h 依 pteB0 設定。
func pagedMachine(t *testing.T, pteB0 uint32) *machine.Machine {
	t.Helper()
	m := machine.New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	m.SetA20(true)
	m.IRQ0Every = 0
	const dir, pt = 0x40000, 0x41000
	for i := uint32(0); i < 1024; i++ {
		m.WritePhys32(dir+i*4, 0)
		m.WritePhys32(pt+i*4, 0)
	}
	m.WritePhys32(dir, pt|7)
	for i := uint32(0); i < 256; i++ {
		m.WritePhys32(pt+i*4, i<<12|7)
	}
	m.WritePhys32(pt+0xB0*4, pteB0)
	if err := m.SetCR3(dir); err != nil {
		t.Fatal(err)
	}
	if err := m.SetCR0(1 | 1<<31); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.CS] = 0xB000
	m.CPU.IP = 0x0010
	return m
}

func TestPCGuardOffUnchanged(t *testing.T) {
	m := machine.New()
	m.CPU.Seg[cpu.CS] = 0xB000
	m.CPU.IP = 0x0010
	if err := pcGuard(m); err == nil || !strings.Contains(err.Error(), "跑出可用記憶體") {
		t.Fatalf("轉換關閉時應照舊攔下 B000:0010，得到 %v", err)
	}
	m.CPU.Seg[cpu.CS] = 0x1000
	if err := pcGuard(m); err != nil {
		t.Fatalf("1000:0010 不該攔：%v", err)
	}
}

func TestPCGuardPagedRAM(t *testing.T) {
	m := pagedMachine(t, 0x200000|7)
	if err := pcGuard(m); err != nil {
		t.Fatalf("線性 B0010 對到 RAM 200010，不該攔：%v", err)
	}
}

func TestPCGuardPagedVideo(t *testing.T) {
	m := pagedMachine(t, 0xB0000|7)
	err := pcGuard(m)
	if err == nil || !strings.Contains(err.Error(), "跑出可用記憶體") {
		t.Fatalf("線性 B0010 對到實體 B0010，應攔下，得到 %v", err)
	}
	m2 := pagedMachine(t, 0xC0000|7)
	m2.CPU.Seg[cpu.CS] = 0x1000 // 線性 10010，恆等映射到 RAM
	if err := pcGuard(m2); err != nil {
		t.Fatalf("1000:0010 不該攔：%v", err)
	}
}

func TestPCGuardPageAbsent(t *testing.T) {
	m := pagedMachine(t, 0)
	if err := pcGuard(m); err != nil {
		t.Fatalf("頁不存在時護欄不攔，得到 %v", err)
	}
	err := m.Step()
	var pf *machine.PageFaultError
	if !errors.As(err, &pf) {
		t.Fatalf("頁不存在時 Step 應回 *PageFaultError，得到 %v", err)
	}
}
