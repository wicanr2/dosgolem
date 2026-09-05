package machine

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func TestLoadLESynthetic(t *testing.T) {
	b := leFixture()
	header := b[0x80:]
	binary.LittleEndian.PutUint32(header[0x20:], 1)
	binary.LittleEndian.PutUint32(header[0x24:], 0x1800)
	m, err := LoadLE(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.CPU.EIP != 0x11234 || m.CPU.R[cpu386.ESP] != 0x11800 || len(m.Mem) != 0x12000 {
		t.Fatalf("unexpected LE machine: EIP=%X ESP=%X len=%X", m.CPU.EIP, m.CPU.R[cpu386.ESP], len(m.Mem))
	}
}

func TestFD2EntryPrefixWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	if m.CPU.EIP != 0x3c964 || m.CPU.R[cpu386.ESP] != 0x556b0 {
		t.Fatalf("unexpected entry state: EIP=%X ESP=%X", m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
	wantScanBytes := []byte{0x80, 0x3e, 0x00, 0xac, 0x75, 0xfa, 0x80, 0x3e, 0x00, 0x75, 0xe0, 0xac, 0x46, 0x46, 0x80, 0x3e, 0x00, 0xa4, 0x75, 0xfa, 0x1f}
	if got := m.Mem[0x3cb27 : 0x3cb27+uint32(len(wantScanBytes))]; !bytes.Equal(got, wantScanBytes) {
		t.Fatalf("environment scan bytes=% X", got)
	}
	for steps := 0; m.CPU.EIP != 0x45dd6 && steps < 271; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x45dd6 {
		t.Fatalf("entry did not branch past environment prefix test: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	assertFD2FirstCallbackState(t, m)
}

func fixedFD2Machine(t *testing.T) (*LEMachine, *FD2StartupDOS) {
	t.Helper()
	path := os.Getenv("DOSGOLEM_FD2_EXE")
	if path == "" {
		t.Skip("DOSGOLEM_FD2_EXE 未設定")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 357074 || md5.Sum(b) != [16]byte{0xb9, 0x7c, 0xaf, 0x22, 0x39, 0xa2, 0x7a, 0x89, 0x60, 0x69, 0xd0, 0x35, 0x49, 0xd9, 0x6e, 0x1e} || sha256.Sum256(b) != [32]byte{0x22, 0x2b, 0x7d, 0x06, 0x7a, 0xd4, 0x45, 0x0e, 0xb9, 0xc5, 0xf6, 0xe6, 0xbc, 0xe1, 0x79, 0x7d, 0x54, 0xbb, 0x05, 0x04, 0x17, 0xba, 0x39, 0xce, 0xd6, 0x06, 0x7f, 0x80, 0x39, 0xf2, 0x8c, 0x4f} {
		t.Fatal("FD2.EXE 雜湊或大小不符")
	}
	m, err := LoadLE(b)
	if err != nil {
		t.Fatal(err)
	}
	services := &FD2StartupDOS{}
	m.CPU.IntHook = services.Handle
	return m, services
}

func assertFD2FirstCallbackState(t *testing.T, m *LEMachine) {
	t.Helper()
	if m.CPU.R[cpu386.EAX] != 0x0003037f || m.CPU.R[cpu386.EBX] != 0x539c2 || m.CPU.Seg[cpu386.SegGS] != 0x20 {
		t.Fatalf("first callee prologue mismatch: EAX=%X EBX=%X GS=%X flags=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.Seg[cpu386.SegGS], m.CPU.EFlags)
	}
	selectorGS, err := m.Read16(0x527f0)
	if err != nil {
		t.Fatal(err)
	}
	selectorES, err := m.Read16(0x52810)
	if err != nil {
		t.Fatal(err)
	}
	if selectorGS != 0x20 || selectorES != 0x28 {
		t.Fatalf("selector globals GS=%X ES=%X", selectorGS, selectorES)
	}
	if m.CPU.R[cpu386.ECX] != 0 {
		t.Fatalf("cleared command-tail count ECX=%X", m.CPU.R[cpu386.ECX])
	}
	flatDS, err := m.Read16(0x3c9d8)
	if err != nil || flatDS != 0x160 {
		t.Fatalf("flat ES write=%X err=%v", flatDS, err)
	}
	environmentWord, err := m.Read16(0x52838)
	if err != nil || environmentWord != 0x30 {
		t.Fatalf("environment word=%X err=%v", environmentWord, err)
	}
	if m.CPU.R[cpu386.ESP] != 0x5569c {
		t.Fatalf("protected ESP=%X", m.CPU.R[cpu386.ESP])
	}
	if m.CPU.Seg[cpu386.SegDS] != 0x160 || m.CPU.Seg[cpu386.SegES] != 0x160 {
		t.Fatalf("environment selectors DS=%X ES=%X", m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegES])
	}
	if m.CPU.R[cpu386.EDX] != 0x1c4 || m.CPU.R[cpu386.ECX] != 0 {
		t.Fatalf("command-tail prelude EDX=%X ECX=%X", m.CPU.R[cpu386.EDX], m.CPU.R[cpu386.ECX])
	}
	if m.CPU.R[cpu386.EAX] != 0x0003037f || m.CPU.EFlags&cpu386.DF != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("first callee accumulator EAX=%X flags=%X", m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
	if m.CPU.R[cpu386.ESI] != 0x539e0 || m.CPU.R[cpu386.EDI] != 0x539e0 || m.CPU.R[cpu386.EBX] != 0x539c2 {
		t.Fatalf("callee table pointers ESI=%X EDI=%X EBX=%X", m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EBX])
	}
	if m.CPU.EFlags&cpu386.CF != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("x87 callback gate flags=%X", m.CPU.EFlags)
	}
	stackValue, err := m.Read32(0x556ac)
	if err != nil || stackValue != 0x3cb8a {
		t.Fatalf("CALL return address=%X err=%v", stackValue, err)
	}
	savedESI, err := m.Read32(0x556a8)
	if err != nil || savedESI != 0x546b1 || m.Mem[0x546b0] != 0 || !bytes.Equal(m.Mem[0x546b1:0x546b9], []byte("FD2.EXE\x00")) {
		t.Fatalf("callee saved ESI=%X buffer=% X err=%v", savedESI, m.Mem[0x546b0:0x546b9], err)
	}
	calleeES, err := m.Read32(0x5569c)
	if err != nil || calleeES != 0x160 {
		t.Fatalf("callee PUSH ES stack=%X err=%v", calleeES, err)
	}
	callbackReturn, err := m.Read32(0x55698)
	if err != nil || callbackReturn != 0x45dd3 || m.CPU.Seg[cpu386.SegES] != 0x160 {
		t.Fatalf("callback return=%X ES=%X err=%v", callbackReturn, m.CPU.Seg[cpu386.SegES], err)
	}
	if got := m.Mem[0x539c2:0x539c8]; !bytes.Equal(got, []byte{0x02, 0x01, 0xcc, 0xcb, 0x03, 0x00}) {
		t.Fatalf("executed callback record=% X", got)
	}
	fpuSavedControl, err := m.Read32(0x55684)
	if err != nil || fpuSavedControl != 0x0003037f || m.CPU.FPUControl != 0x127f || m.CPU.FPUDepth != 4 {
		t.Fatalf("FNSTCW stack=%X control=%X depth=%d err=%v", fpuSavedControl, m.CPU.FPUControl, m.CPU.FPUDepth, err)
	}
	stack := uint32(0x556b0)
	for _, addr := range []uint32{0x52818, 0x52804} {
		got, err := m.Read32(addr)
		if err != nil {
			t.Fatal(err)
		}
		if got != stack {
			t.Fatalf("[%X]=%X want %X", addr, got, stack)
		}
	}
	if m.Mem[0x5283a] != 6 || m.Mem[0x5283b] != 22 {
		t.Fatalf("DOS version globals=%d.%d", m.Mem[0x5283a], m.Mem[0x5283b])
	}
}

func TestFD2SecondCallbackAbsoluteGateWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460df && steps < 400; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460df {
		t.Fatalf("second callback gate not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x460d5 || m.CPU.R[cpu386.EBX] != 0x539c8 || m.CPU.R[cpu386.ESP] != 0x55694 {
		t.Fatalf("second callback state EAX=%X EBX=%X ESP=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ESP])
	}
	if m.Mem[0x527f4] != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("absolute gate byte=%X flags=%X", m.Mem[0x527f4], m.CPU.EFlags)
	}
	if got := m.Mem[0x539c2:0x539ce]; !bytes.Equal(got, []byte{2, 1, 0xcc, 0xcb, 3, 0, 0, 2, 0xd5, 0x60, 4, 0}) {
		t.Fatalf("callback records=% X", got)
	}
}

func TestFD2SecondCallbackClearsBLWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460e1 && steps < 401; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460e1 {
		t.Fatalf("second callback BL clear not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EBX] != 0x53900 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("second callback BL clear EBX=%X flags=%X", m.CPU.R[cpu386.EBX], m.CPU.EFlags)
	}
}

func TestFD2SecondCallbackStoresX87ControlWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460ef && steps < 406; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460ef {
		t.Fatalf("second callback x87 control store not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.Mem[0x527f5] != 0 || m.CPU.R[cpu386.EAX] != 0 || m.CPU.R[cpu386.ESP] != 0x55690 {
		t.Fatalf("second callback pre-probe byte=%X EAX=%X ESP=%X", m.Mem[0x527f5], m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
	control, err := m.Read16(0x55690)
	if err != nil || control != 0x037f || m.CPU.FPUControl != 0x037f {
		t.Fatalf("second callback x87 control stack=%04X cpu=%04X err=%v", control, m.CPU.FPUControl, err)
	}
}

func TestFD2SecondCallbackX87ClassGateWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460f6 && steps < 410; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460f6 {
		t.Fatalf("second callback x87 class gate not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x0303 || m.CPU.R[cpu386.ESP] != 0x55694 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("second callback x87 class EAX=%X ESP=%X flags=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP], m.CPU.EFlags)
	}
	if m.Mem[0x539c8] != 0 {
		t.Fatalf("second callback record unexpectedly marked: %X", m.Mem[0x539c8])
	}
}

func TestFD2SecondCallbackLoadsControlBaselineWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460c6 && steps < 414; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460c6 {
		t.Fatalf("second callback control baseline not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x127f || m.CPU.R[cpu386.ESP] != 0x55690 {
		t.Fatalf("second callback control baseline EAX=%X ESP=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2ControlBaselineDispatchWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cbd0 && steps < 415; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cbd0 {
		t.Fatalf("control baseline dispatch not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x127f || m.CPU.R[cpu386.ESP] != 0x55690 {
		t.Fatalf("control baseline dispatch EAX=%X ESP=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2X87SelfTestReturnsWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x460fb && steps < 440; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x460fb {
		t.Fatalf("x87 self-test did not return: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x0103 || m.CPU.R[cpu386.ESP] != 0x55694 {
		t.Fatalf("x87 self-test return EAX=%X ESP=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
	if m.CPU.FPUControl != 0x127f || m.CPU.FPUStatus != 0 || m.CPU.FPUDepth != 0 {
		t.Fatalf("x87 self-test FPU control=%X status=%X depth=%d", m.CPU.FPUControl, m.CPU.FPUStatus, m.CPU.FPUDepth)
	}
}
