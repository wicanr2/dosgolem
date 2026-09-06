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
	var provider *DirectoryReadOnlyFiles
	root := os.Getenv("DOSGOLEM_FD2_ROOT")
	if root != "" {
		provider, err = OpenDirectoryReadOnlyFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { provider.Close() })
	}
	services := NewFD2StartupDOS(provider)
	t.Cleanup(func() { services.Close() })
	m.CPU.IntHook = services.Handle
	if _, err := InstallFD2WatcomRuntime(m); err != nil {
		t.Fatal(err)
	}
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

func TestFD2SecondCallbackStoresClassResultWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x46112 && steps < 448; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x46112 {
		t.Fatalf("second callback class stores not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.Mem[0x52830] != 0 || byte(m.CPU.R[cpu386.EBX]) != 3 || m.Mem[0x527f4] != 3 || m.Mem[0x527f5] != 3 {
		t.Fatalf("second callback class control=%X BL=%X stores=%X,%X", m.Mem[0x52830], byte(m.CPU.R[cpu386.EBX]), m.Mem[0x527f4], m.Mem[0x527f5])
	}
	if m.Mem[0x539c8] != 0 {
		t.Fatalf("second callback record unexpectedly marked: %X", m.Mem[0x539c8])
	}
}

func TestFD2SecondCallbackRecordMarkedWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	returns := 0
	for steps := 0; returns < 2 && steps < 452; steps++ {
		if m.CPU.EIP == 0x45dd6 {
			returns++
			if returns == 2 {
				break
			}
		}
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || returns != 2 || m.CPU.EIP != 0x45dd6 {
		t.Fatalf("second callback record mark not reached: calls=%d returns=%d EIP=%X", services.Calls(), returns, m.CPU.EIP)
	}
	if got := m.Mem[0x539c2:0x539ce]; !bytes.Equal(got, []byte{2, 1, 0xcc, 0xcb, 3, 0, 2, 2, 0xd5, 0x60, 4, 0}) {
		t.Fatalf("callback records=% X", got)
	}
	if m.CPU.R[cpu386.EBX] != 0x539c8 || m.CPU.R[cpu386.ESP] != 0x5569c {
		t.Fatalf("second callback return EBX=%X ESP=%X", m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2ThirdCallbackSelectedWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cbfd && steps < 520; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cbfd {
		t.Fatalf("third callback not selected: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x4cbfd || m.CPU.R[cpu386.EBX] != 0x539da || m.CPU.R[cpu386.ESP] != 0x55698 || m.Mem[0x539da] != 0 {
		t.Fatalf("third callback state EAX=%X EBX=%X ESP=%X status=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ESP], m.Mem[0x539da])
	}
}

func TestFD2ThirdCallbackPushesFSWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cc03 && steps < 470; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc03 || m.CPU.R[cpu386.ESP] != 0x55684 {
		t.Fatalf("third callback PUSH FS state calls=%d EIP=%X ESP=%X", services.Calls(), m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
	value, err := m.Read32(0x55684)
	if err != nil || uint16(value) != m.CPU.Seg[cpu386.SegFS] {
		t.Fatalf("third callback saved FS=%X current=%X err=%v", value, m.CPU.Seg[cpu386.SegFS], err)
	}
}

func TestFD2ThirdCallbackGlobalGateWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cc10 && steps < 475; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc10 {
		t.Fatalf("third callback global gate not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	value, err := m.Read32(0x537fc)
	if err != nil || value != 0 || m.CPU.R[cpu386.EBP] != 0x55680 || m.CPU.R[cpu386.ESP] != 0x5567c || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("third callback global gate value=%X EBP=%X ESP=%X flags=%X err=%v", value, m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], m.CPU.EFlags, err)
	}
}

func TestFD2ThirdCallbackLoadsFSWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cc1d && steps < 477; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc1d {
		t.Fatalf("third callback LFS not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0 || m.CPU.Seg[cpu386.SegFS] != 0x30 || m.CPU.R[cpu386.ESP] != 0x5567c {
		t.Fatalf("third callback LFS EAX=%X FS=%X ESP=%X", m.CPU.R[cpu386.EAX], m.CPU.Seg[cpu386.SegFS], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2ThirdCallbackScanSetupWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cc24 && steps < 480; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc24 {
		t.Fatalf("third callback scan setup not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	local, err := m.Read32(0x5567c)
	if err != nil || local != 0 || m.CPU.R[cpu386.EAX] != 0x30 || m.CPU.R[cpu386.ESI] != 0 || m.CPU.R[cpu386.EBP] != 0x55680 || m.CPU.R[cpu386.ESP] != 0x5567c || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("third callback scan setup local=%X EAX=%X ESI=%X EBP=%X ESP=%X flags=%X err=%v", local, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], m.CPU.EFlags, err)
	}
}

func TestFD2ThirdCallbackEnvironmentFirstByteWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x4cc41 && steps < 485; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc41 {
		t.Fatalf("third callback environment first-byte gate not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.Seg[cpu386.SegES] != 0x30 || m.CPU.R[cpu386.EAX] != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("third callback environment first-byte ES=%X EAX=%X flags=%X", m.CPU.Seg[cpu386.SegES], m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
}

func TestFD2ThirdCallbackFirstAllocationCallWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x36d26 && steps < 490; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x36d26 {
		t.Fatalf("third callback first allocation call not reached: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	argument, errArg := m.Read32(0x55678)
	ret, errRet := m.Read32(0x55674)
	if errArg != nil || errRet != nil || argument != 1 || ret != 0x4cc51 || m.CPU.R[cpu386.ESP] != 0x55674 {
		t.Fatalf("third callback first allocation argument=%X return=%X ESP=%X errors=%v,%v", argument, ret, m.CPU.R[cpu386.ESP], errArg, errRet)
	}
}

func TestFD2ThirdCallbackFirstAllocationReturnsWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	initialMemory := len(m.Mem)
	wantBase := uint32((initialMemory + 3) &^ 3)
	for steps := 0; m.CPU.EIP != 0x4cc51 && steps < 491; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc51 {
		t.Fatalf("first _nmalloc did not return: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != wantBase || m.CPU.R[cpu386.ESP] != 0x55678 || len(m.Mem) != int(wantBase)+4 {
		t.Fatalf("first _nmalloc result=%X ESP=%X memory=%X want-base=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP], len(m.Mem), wantBase)
	}
}

func TestFD2ThirdCallbackSecondAllocationReturnsWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	wantFirst := uint32((len(m.Mem) + 3) &^ 3)
	for steps := 0; m.CPU.EIP != 0x4cc70 && steps < 520; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("second _nmalloc path: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x4cc70 {
		t.Fatalf("second _nmalloc did not return: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != wantFirst+4 || m.CPU.R[cpu386.ESP] != 0x55678 || len(m.Mem) != int(wantFirst)+8 {
		t.Fatalf("second _nmalloc result=%X ESP=%X memory=%X", m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP], len(m.Mem))
	}
}

func TestFD2ThirdCallbackEnvironmentCopyCompletesWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 620 && !(steps > 500 && m.CPU.EIP == 0x45dd3); steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("environment copy path: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x45dd3 || steps <= 500 {
		t.Fatalf("third callback did not return: calls=%d steps=%d EIP=%X", services.Calls(), steps, m.CPU.EIP)
	}
	environmentTable, errTable := m.Read32(0x537fc)
	environmentTail, errTail := m.Read32(0x53800)
	terminator, errTerminator := m.Read32(0x634d8)
	if errTable != nil || errTail != nil || errTerminator != nil || environmentTable != 0x634d8 || environmentTail != 0x634dc || terminator != 0 {
		t.Fatalf("environment table=%X tail=%X terminator=%X errors=%v,%v,%v", environmentTable, environmentTail, terminator, errTable, errTail, errTerminator)
	}
}

func TestFD2PostEnvironmentRuntimeListWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 800 && m.CPU.EIP != 0x46114; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("post-environment runtime list: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	listHead, errHead := m.Read32(0x541ac)
	listGate, errGate := m.Read32(0x541a0)
	if services.Calls() != 2 || m.CPU.EIP != 0x46114 || m.Mem[0x52881]&7 != 4 || listHead == 0 || listGate != 0 || errHead != nil || errGate != nil {
		t.Fatalf("runtime list steps=%d EIP=%X flags=%X head=%X gate=%X calls=%d errors=%v,%v", steps, m.CPU.EIP, m.Mem[0x52881], listHead, listGate, services.Calls(), errHead, errGate)
	}
}

func TestFD2ArgvInitializationWhenProvided(t *testing.T) {
	m, _ := fixedFD2Machine(t)
	steps := 0
	argc := uint32(0)
	for ; steps < 800 && argc == 0; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
		argc, _ = m.Read32(0x5462c)
	}
	argv, errArgv := m.Read32(0x54628)
	internalArgc, errInternalArgc := m.Read32(0x527f8)
	internalArgv, errInternalArgv := m.Read32(0x527fc)
	first, errFirst := m.Read32(argv)
	terminator, errTerminator := m.Read32(argv + 4)
	if argc != 1 || internalArgc != 1 || argv == 0 || internalArgv != argv || first != 0x546b1 || terminator != 0 || m.CPU.EIP != 0x45dd3 || errArgv != nil || errInternalArgc != nil || errInternalArgv != nil || errFirst != nil || errTerminator != nil {
		t.Fatalf("argv init steps=%d EIP=%X argc=%d/%d argv=%X/%X first=%X terminator=%X errors=%v,%v,%v,%v,%v", steps, m.CPU.EIP, argc, internalArgc, argv, internalArgv, first, terminator, errArgv, errInternalArgc, errInternalArgv, errFirst, errTerminator)
	}
}

func TestFD2DeterministicDelayInitializationWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 1200 && m.CPU.EIP != 0x463be; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("delay init: step=%d EIP=%X EAX=%X EBX=%X ECX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], err)
		}
	}
	calibration, err := m.Read32(0x541b0)
	if m.CPU.EIP != 0x463be || services.Calls() != 5 || calibration != 1 || err != nil {
		t.Fatalf("delay init steps=%d EIP=%X calls=%d calibration=%d err=%v", steps, m.CPU.EIP, services.Calls(), calibration, err)
	}
}

func TestFD2ReachesMainWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 2500 && m.CPU.EIP != 0x25bf4; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("main entry: step=%d EIP=%X EAX=%X EBX=%X ECX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], err)
		}
	}
	returnAddress, errReturn := m.Read32(m.CPU.R[cpu386.ESP])
	argc, errArgc := m.Read32(m.CPU.R[cpu386.ESP] + 4)
	argv, errArgv := m.Read32(m.CPU.R[cpu386.ESP] + 8)
	publicArgv, errPublicArgv := m.Read32(0x54628)
	firstArg, errFirstArg := m.Read32(argv)
	if m.CPU.EIP != 0x25bf4 || services.Calls() != 5 || returnAddress != 0x45d91 || argc != 1 || argv == 0 || argv != publicArgv || firstArg != 0x546b1 || errReturn != nil || errArgc != nil || errArgv != nil || errPublicArgv != nil || errFirstArg != nil {
		t.Fatalf("main entry steps=%d EIP=%X calls=%d ESP=%X return=%X argc=%d argv=%X/%X first=%X errors=%v,%v,%v,%v,%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ESP], returnAddress, argc, argv, publicArgv, firstArg, errReturn, errArgc, errArgv, errPublicArgv, errFirstArg)
	}
}

func TestFD2CompletesWatcomStackProbeWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 1500 && m.CPU.EIP != 0x25bfe; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("stack probe: step=%d EIP=%X EAX=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP], err)
		}
	}
	if m.CPU.EIP != 0x25bfe || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != 0x556a4 || m.CPU.R[cpu386.ESP] != 0x55698 {
		t.Fatalf("stack probe steps=%d EIP=%X calls=%d EAX=%X ESP=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2CompletesFirstDPMILockWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 1500 && m.CPU.EIP != 0x37859; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("first DPMI lock: step=%d EIP=%X EAX=%X EBX=%X ECX=%X EDX=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.R[cpu386.ESP], err)
		}
	}
	if m.CPU.EIP != 0x37859 || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != 1 {
		t.Fatalf("first DPMI lock steps=%d EIP=%X calls=%d EAX=%X ESP=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP])
	}
}

func TestFD2CompletesAILDPMILocksWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 2000 && m.CPU.EIP != 0x378de; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL DPMI locks: step=%d EIP=%X EAX=%X EBX=%X ECX=%X EDX=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.R[cpu386.ESP], err)
		}
	}
	initialized, err := m.Read32(0x527e4)
	if m.CPU.EIP != 0x378de || services.Calls() != 5 || initialized != 1 || err != nil {
		t.Fatalf("AIL DPMI locks steps=%d EIP=%X calls=%d initialized=%X err=%v", steps, m.CPU.EIP, services.Calls(), initialized, err)
	}
}

func TestFD2PassesGetenvArgumentWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 2500 && m.CPU.EIP != 0x3f151; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("getenv argument: step=%d EIP=%X EBP=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], err)
		}
	}
	stackDescriptor := m.CPU.Descriptors[m.CPU.Seg[cpu386.SegSS]]
	argument, errArgument := m.Read32(stackDescriptor.Base + m.CPU.R[cpu386.EBP] + 0x14)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("getenv PUSH: step=%d EIP=%X EBP=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], err)
	}
	pushed, errPushed := m.Read32(stackDescriptor.Base + m.CPU.R[cpu386.ESP])
	if m.CPU.EIP != 0x3f154 || services.Calls() != 5 || argument == 0 || pushed != argument || errArgument != nil || errPushed != nil {
		t.Fatalf("getenv argument steps=%d EIP=%X calls=%d SS=%X base=%X EBP=%X ESP=%X argument=%X pushed=%X errors=%v,%v", steps, m.CPU.EIP, services.Calls(), m.CPU.Seg[cpu386.SegSS], stackDescriptor.Base, m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], argument, pushed, errArgument, errPushed)
	}
}

func TestFD2ScansFirstEnvironmentNameWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 2500 && m.CPU.EIP != 0x37818; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("strlen scan: step=%d EIP=%X EAX=%X ECX=%X EDI=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDI], err)
		}
	}
	if m.CPU.EIP != 0x37818 || services.Calls() != 5 || m.CPU.R[cpu386.ECX] == 0xffffffff || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("strlen scan steps=%d EIP=%X calls=%d ECX=%X EDI=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDI], m.CPU.EFlags)
	}
}

func TestFD2ComplementsStrlenCountWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 2500 && m.CPU.EIP != 0x3781a; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("strlen complement: step=%d EIP=%X ECX=%X EDI=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDI], err)
		}
	}
	if m.CPU.EIP != 0x3781a || services.Calls() != 5 || m.CPU.R[cpu386.ECX] != 0x0a || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("strlen complement steps=%d EIP=%X calls=%d ECX=%X EDI=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDI], m.CPU.EFlags)
	}
}

func TestFD2ReadsAILPreferenceWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 3000 && m.CPU.EIP != 0x3f5eb; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL preference read: step=%d EIP=%X EAX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EDX], err)
		}
	}
	index := m.CPU.R[cpu386.EAX]
	expected, errExpected := m.Read32(0x5430c + index*4)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL preference indexed MOV: step=%d EIP=%X index=%X: %v", steps, m.CPU.EIP, index, err)
	}
	if m.CPU.EIP != 0x3f5f2 || services.Calls() != 5 || m.CPU.R[cpu386.EDX] != expected || errExpected != nil {
		t.Fatalf("AIL preference read steps=%d EIP=%X calls=%d index=%X EDX=%X expected=%X err=%v", steps, m.CPU.EIP, services.Calls(), index, m.CPU.R[cpu386.EDX], expected, errExpected)
	}
}

func TestFD2WritesAILPreferenceWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 3000 && m.CPU.EIP != 0x3f5f6; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL preference write: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	index, replacement := m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL preference indexed store: step=%d EIP=%X index=%X: %v", steps, m.CPU.EIP, index, err)
	}
	stored, errStored := m.Read32(0x5430c + index*4)
	if m.CPU.EIP != 0x3f5fd || services.Calls() != 5 || stored != replacement || errStored != nil {
		t.Fatalf("AIL preference write steps=%d EIP=%X calls=%d index=%X stored=%X replacement=%X err=%v", steps, m.CPU.EIP, services.Calls(), index, stored, replacement, errStored)
	}
}

func TestFD2LeavesAILWrapperWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 3000 && m.CPU.EIP != 0x38e1a; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL wrapper exit: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	before, errBefore := m.Read32(0x54178)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL wrapper DEC: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	after, errAfter := m.Read32(0x54178)
	if m.CPU.EIP != 0x38e20 || services.Calls() != 5 || before == 0 || after != before-1 || errBefore != nil || errAfter != nil {
		t.Fatalf("AIL wrapper exit steps=%d EIP=%X calls=%d before=%X after=%X errors=%v,%v", steps, m.CPU.EIP, services.Calls(), before, after, errBefore, errAfter)
	}
}

func TestFD2ComparesAILTableIndexWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3fa4d; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL table index compare: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	before := m.CPU.R[cpu386.EAX]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL table CMP: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, before, err)
	}
	if m.CPU.EIP != 0x3fa50 || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != before || before >= 0x10 || m.CPU.EFlags&cpu386.SF == 0 || m.CPU.EFlags&cpu386.ZF != 0 {
		t.Fatalf("AIL table CMP steps=%d EIP=%X calls=%d EAX=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
}

func TestFD2BranchesThroughAILTableLoopWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3fa50; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL table JL: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL table JL step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x3fa43 || services.Calls() != 5 || m.CPU.R[cpu386.EAX] >= 0x10 {
		t.Fatalf("AIL table JL steps=%d EIP=%X calls=%d EAX=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
}

func TestFD2SavesFlagsBeforeAILInterruptSetupWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3e930; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL PUSHFD: step=%d EIP=%X flags=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.EFlags, m.CPU.R[cpu386.ESP], err)
		}
	}
	flags, beforeESP := m.CPU.EFlags, m.CPU.R[cpu386.ESP]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL PUSHFD step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	stackDescriptor := m.CPU.Descriptors[m.CPU.Seg[cpu386.SegSS]]
	stored, errStored := m.Read32(stackDescriptor.Base + m.CPU.R[cpu386.ESP])
	if m.CPU.EIP != 0x3e931 || services.Calls() != 5 || m.CPU.R[cpu386.ESP] != beforeESP-4 || stored != flags || errStored != nil {
		t.Fatalf("AIL PUSHFD steps=%d EIP=%X calls=%d ESP=%X/%X flags=%X stored=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ESP], beforeESP, flags, stored, errStored)
	}
}

func TestFD2DisablesInterruptsForAILSetupWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3e931; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL CLI: step=%d EIP=%X flags=%X: %v", steps, m.CPU.EIP, m.CPU.EFlags, err)
		}
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL CLI step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x3e932 || services.Calls() != 5 || m.CPU.EFlags&cpu386.IF != 0 {
		t.Fatalf("AIL CLI steps=%d EIP=%X calls=%d flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.EFlags)
	}
}

func TestFD2StoresAILDataSelectorWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3e935; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL DS store: step=%d EIP=%X DS=%X: %v", steps, m.CPU.EIP, m.CPU.Seg[cpu386.SegDS], err)
		}
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL DS store step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	stored, errStored := m.Read16(0x52bee)
	if m.CPU.EIP != 0x3e93c || services.Calls() != 5 || stored != m.CPU.Seg[cpu386.SegDS] || errStored != nil {
		t.Fatalf("AIL DS store steps=%d EIP=%X calls=%d DS=%X stored=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.Seg[cpu386.SegDS], stored, errStored)
	}
}

func TestFD2LoadsAILDataSelectorWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 4000 && m.CPU.EIP != 0x3e93c; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL ES load: step=%d EIP=%X ES=%X: %v", steps, m.CPU.EIP, m.CPU.Seg[cpu386.SegES], err)
		}
	}
	stored, errStored := m.Read16(0x52bee)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL ES load step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x3e943 || services.Calls() != 5 || m.CPU.Seg[cpu386.SegES] != stored || errStored != nil {
		t.Fatalf("AIL ES load steps=%d EIP=%X calls=%d ES=%X stored=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.Seg[cpu386.SegES], stored, errStored)
	}
}

func TestFD2GetsTimerRealModeVectorWhenProvided(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e9b4; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DPMI vector: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	if m.CPU.EIP != 0x3e9b4 || services.Calls() != 5 || uint16(m.CPU.R[cpu386.ECX]) != 0 || uint16(m.CPU.R[cpu386.EDX]) != 0 || m.CPU.EFlags&cpu386.CF != 0 {
		t.Fatalf("DPMI vector steps=%d EIP=%X calls=%d ECX=%X EDX=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.EFlags)
	}
}

func TestFD2PacksTimerRealModeVector(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e9ba; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DPMI vector pack: step=%d EIP=%X ECX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], err)
		}
	}
	if m.CPU.EIP != 0x3e9ba || services.Calls() != 5 || m.CPU.R[cpu386.ECX] != 0 {
		t.Fatalf("DPMI vector pack steps=%d EIP=%X calls=%d ECX=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ECX])
	}
}

func TestFD2ReplacesTimerDOSVector(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e9ef; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DOS vector replace: step=%d EIP=%X EAX=%X EBX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX], err)
		}
	}
	oldOffset, errOffset := m.Read32(0x52bd4)
	oldSelector, errSelector := m.Read16(0x52bd8)
	wantVector := uint64(m.CPU.Seg[cpu386.SegCS])<<32 | 0x3e73e
	if m.CPU.EIP != 0x3e9ef || services.Calls() != 5 || oldOffset != 0 || oldSelector != 0 || services.dosVectors[8] != wantVector || errOffset != nil || errSelector != nil {
		t.Fatalf("DOS vector replace steps=%d EIP=%X calls=%d old=%X:%X new=%X want=%X errors=%v,%v", steps, m.CPU.EIP, services.Calls(), oldSelector, oldOffset, services.dosVectors[8], wantVector, errOffset, errSelector)
	}
}

func TestFD2EntersAILHandlerGate(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e72d; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL handler gate: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	depth, errDepth := m.Read32(0x52bea)
	if m.CPU.EIP != 0x3e72d || services.Calls() != 5 || depth != 1 || errDepth != nil {
		t.Fatalf("AIL handler gate steps=%d EIP=%X calls=%d depth=%d err=%v", steps, m.CPU.EIP, services.Calls(), depth, errDepth)
	}
}

func TestFD2StoresAILIndexedTableEntry(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3f05f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL indexed table: step=%d EIP=%X EBX=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EAX], err)
		}
	}
	value, errValue := m.Read32(0x52b50)
	if m.CPU.EIP != 0x3f05f || services.Calls() != 5 || value != 0xd68d || errValue != nil {
		t.Fatalf("AIL indexed table steps=%d EIP=%X calls=%d value=%X err=%v", steps, m.CPU.EIP, services.Calls(), value, errValue)
	}
}

func TestFD2ClearsAILIndexedTableEntry(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3f05f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL indexed clear setup: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	for offset, value := range []byte{0xef, 0xbe, 0xed, 0xfe} {
		if err := m.Write8(0x52b10+uint32(offset), value); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("AIL indexed clear: EIP=%X: %v", m.CPU.EIP, err)
	}
	value, errValue := m.Read32(0x52b10)
	if m.CPU.EIP != 0x3f069 || services.Calls() != 5 || value != 0 || errValue != nil {
		t.Fatalf("AIL indexed clear steps=%d EIP=%X calls=%d value=%X err=%v", steps, m.CPU.EIP, services.Calls(), value, errValue)
	}
}

func TestFD2ScansAILActiveTableEntry(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e8e4; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL active scan: step=%d EIP=%X EDI=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EDI], err)
		}
	}
	value, errValue := m.Read32(0x52a94)
	if m.CPU.EIP != 0x3e8e4 || services.Calls() != 5 || value != 0 || m.CPU.EFlags&cpu386.ZF == 0 || errValue != nil {
		t.Fatalf("AIL active scan steps=%d EIP=%X calls=%d value=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), value, m.CPU.EFlags, errValue)
	}
}

func TestFD2ReadsAILIndexedTableEntry(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e8ec; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL indexed read: step=%d EIP=%X EDI=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EAX], err)
		}
	}
	if m.CPU.EIP != 0x3e8ec || services.Calls() != 5 || m.CPU.R[cpu386.EDI] != 0x3c || m.CPU.R[cpu386.EAX] != 0xd68d {
		t.Fatalf("AIL indexed read steps=%d EIP=%X calls=%d EDI=%X EAX=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EAX])
	}
}

func TestFD2CompletesAILActiveTableScan(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e8fa; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL active scan loop: step=%d EIP=%X EDI=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EDI], err)
		}
	}
	if m.CPU.EIP != 0x3e8fa || services.Calls() != 5 || m.CPU.R[cpu386.EDI] != 0x40 || m.CPU.R[cpu386.ECX] != 0xd68d || m.CPU.EFlags&cpu386.CF != 0 {
		t.Fatalf("AIL active scan loop steps=%d EIP=%X calls=%d EDI=%X ECX=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.ECX], m.CPU.EFlags)
	}
}

func TestFD2ComparesAILRateThreshold(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e8a6; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL rate threshold: step=%d EIP=%X EBP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBP], err)
		}
	}
	argument, errArgument := m.Read32(m.CPU.R[cpu386.EBP] + 8)
	if m.CPU.EIP != 0x3e8a6 || services.Calls() != 5 || argument != 0xd68d || m.CPU.EFlags&cpu386.ZF == 0 || errArgument != nil {
		t.Fatalf("AIL rate threshold steps=%d EIP=%X calls=%d argument=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), argument, m.CPU.EFlags, errArgument)
	}
}

func TestFD2ProgramsPITControl(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e870; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL PIT control: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	if m.CPU.EIP != 0x3e870 || services.Calls() != 5 || len(m.PortLog) != 1 || m.PortLog[0] != (LEPortWrite{Port: 0x43, Value: 0x36, Sequence: 0}) || m.Ports[0x43] != 0x36 {
		t.Fatalf("AIL PIT control steps=%d EIP=%X calls=%d log=%+v ports=%v", steps, m.CPU.EIP, services.Calls(), m.PortLog, m.Ports)
	}
}

func TestFD2ProgramsPITDivisor(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e882; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL PIT divisor: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	divisor, errDivisor := m.Read32(0x52bde)
	want := []LEPortWrite{{Port: 0x43, Value: 0x36, Sequence: 0}, {Port: 0x40, Value: 0, Sequence: 1}, {Port: 0x40, Value: 0, Sequence: 2}}
	if m.CPU.EIP != 0x3e882 || services.Calls() != 5 || divisor != 0 || errDivisor != nil || len(m.PortLog) != len(want) {
		t.Fatalf("AIL PIT divisor steps=%d EIP=%X calls=%d divisor=%X log=%+v err=%v", steps, m.CPU.EIP, services.Calls(), divisor, m.PortLog, errDivisor)
	}
	for i := range want {
		if m.PortLog[i] != want[i] {
			t.Fatalf("AIL PIT divisor log[%d]=%+v want=%+v", i, m.PortLog[i], want[i])
		}
	}
}

func TestFD2ChecksSavedInterruptFlag(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e889; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL saved IF: step=%d EIP=%X EBP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBP], err)
		}
	}
	saved, errSaved := m.Read8(m.CPU.R[cpu386.EBP] + 5)
	if m.CPU.EIP != 0x3e889 || services.Calls() != 5 || saved&2 != 0 || m.CPU.EFlags&cpu386.ZF == 0 || errSaved != nil {
		t.Fatalf("AIL saved IF steps=%d EIP=%X calls=%d saved=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), saved, m.CPU.EFlags, errSaved)
	}
}

func TestFD2RestoresPITCallerFlags(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3e86a; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL flags setup: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	wantFlags := m.CPU.EFlags
	for ; steps < 5000 && m.CPU.EIP != 0x3e88f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("AIL flags restore: step=%d EIP=%X: %v", steps, m.CPU.EIP, err)
		}
	}
	if m.CPU.EIP != 0x3e88f || services.Calls() != 5 || m.CPU.EFlags != wantFlags {
		t.Fatalf("AIL flags restore steps=%d EIP=%X calls=%d flags=%X want=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.EFlags, wantFlags)
	}
}

func TestFD2AllocatesMDIINIStackFrame(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x43ef2; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("MDI.INI frame setup: step=%d EIP=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ESP], err)
		}
	}
	before := m.CPU.R[cpu386.ESP]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("MDI.INI frame allocation: EIP=%X ESP=%X: %v", m.CPU.EIP, m.CPU.R[cpu386.ESP], err)
	}
	if m.CPU.EIP != 0x43ef8 || services.Calls() != 5 || m.CPU.R[cpu386.ESP] != before-0x118 {
		t.Fatalf("MDI.INI frame steps=%d EIP=%X calls=%d ESP=%X before=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.ESP], before)
	}
}

func TestFD2AddressesMDIINISettingsBuffer(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3f327; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("MDI.INI settings buffer setup: step=%d EIP=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ESP], err)
		}
	}
	before := m.CPU.R[cpu386.ESP]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("MDI.INI settings buffer LEA: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x3f32e || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != before+0x108 || m.CPU.R[cpu386.ESP] != before {
		t.Fatalf("MDI.INI settings buffer steps=%d EIP=%X calls=%d EAX=%X ESP=%X before=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.ESP], before)
	}
}

func TestFD2LoadsMDIINIPathArgument(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3f33c; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("MDI.INI path setup: step=%d EIP=%X ESP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ESP], err)
		}
	}
	beforeESP, beforeFlags := m.CPU.R[cpu386.ESP], m.CPU.EFlags
	want, errRead := m.Read32(beforeESP + 0x184)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("MDI.INI path load: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x3f343 || services.Calls() != 5 || errRead != nil || m.CPU.R[cpu386.EDX] != want || m.CPU.R[cpu386.ESP] != beforeESP || m.CPU.EFlags != beforeFlags {
		t.Fatalf("MDI.INI path steps=%d EIP=%X calls=%d EDX=%X want=%X ESP=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EDX], want, m.CPU.R[cpu386.ESP], m.CPU.EFlags, errRead)
	}
}

func TestFD2ComparesAllocFPTableBound(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3d84f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("allocfp bound setup: step=%d EIP=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBX], err)
		}
	}
	before := m.CPU.R[cpu386.EBX]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("allocfp bound compare: EIP=%X: %v", m.CPU.EIP, err)
	}
	wantCF, wantZF := before < 0x52a48, before == 0x52a48
	if m.CPU.EIP != 0x3d855 || services.Calls() != 5 || m.CPU.R[cpu386.EBX] != before || (m.CPU.EFlags&cpu386.CF != 0) != wantCF || (m.CPU.EFlags&cpu386.ZF != 0) != wantZF {
		t.Fatalf("allocfp bound steps=%d EIP=%X calls=%d EBX=%X flags=%X wantCF=%t wantZF=%t", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EBX], m.CPU.EFlags, wantCF, wantZF)
	}
}

func TestFD2ClearsOpenFileModeBits(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36ec9; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("open mode setup: step=%d EIP=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBX], err)
		}
	}
	beforeEBX := m.CPU.R[cpu386.EBX]
	before, errRead := m.Read8(beforeEBX + 0x0c)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("open mode AND: EIP=%X: %v", m.CPU.EIP, err)
	}
	after, errAfter := m.Read8(beforeEBX + 0x0c)
	if m.CPU.EIP != 0x36ecd || services.Calls() != 5 || errRead != nil || errAfter != nil || after != before&0xfc || m.CPU.R[cpu386.EBX] != beforeEBX {
		t.Fatalf("open mode steps=%d EIP=%X calls=%d before=%X after=%X EBX=%X errors=%v/%v", steps, m.CPU.EIP, services.Calls(), before, after, m.CPU.R[cpu386.EBX], errRead, errAfter)
	}
}

func TestFD2LoadsOpenModeByte(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36e15; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("open mode byte setup: step=%d EIP=%X ESI=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.ESI], err)
		}
	}
	beforeESI, beforeFlags := m.CPU.R[cpu386.ESI], m.CPU.EFlags
	want, errRead := m.Read8(beforeESI)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("open mode byte MOVZX: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x36e18 || services.Calls() != 5 || errRead != nil || m.CPU.R[cpu386.EAX] != uint32(want) || m.CPU.R[cpu386.ESI] != beforeESI || m.CPU.EFlags != beforeFlags {
		t.Fatalf("open mode byte steps=%d EIP=%X calls=%d EAX=%X want=%X ESI=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], want, m.CPU.R[cpu386.ESI], m.CPU.EFlags, errRead)
	}
}

func TestFD2BranchesPastTolowerConversion(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3d7ef; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("tolower upper bound setup: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	beforeEAX, beforeFlags := m.CPU.R[cpu386.EAX], m.CPU.EFlags
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("tolower JG: EIP=%X: %v", m.CPU.EIP, err)
	}
	wantEIP := uint32(0x3d7f1)
	if beforeFlags&cpu386.ZF == 0 && (beforeFlags&cpu386.SF != 0) == (beforeFlags&cpu386.OF != 0) {
		wantEIP = 0x3d7f4
	}
	if m.CPU.EIP != wantEIP || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != beforeEAX || m.CPU.EFlags != beforeFlags {
		t.Fatalf("tolower JG steps=%d EIP=%X want=%X calls=%d EAX=%X flags=%X", steps, m.CPU.EIP, wantEIP, services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
}

func TestFD2BuildsBaseOpenFlags(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36e45; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("base open flags setup: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	before := m.CPU.R[cpu386.EAX]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("base open flags OR: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x36e47 || services.Calls() != 5 || m.CPU.R[cpu386.EAX] != before|3 {
		t.Fatalf("base open flags steps=%d EIP=%X calls=%d EAX=%X before=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], before, m.CPU.EFlags)
	}
}

func TestFD2BuildsBinaryOpenFlags(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36e6f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("binary open flags setup: step=%d EIP=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EDX], err)
		}
	}
	before := m.CPU.R[cpu386.EDX]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("binary open flags OR: EIP=%X: %v", m.CPU.EIP, err)
	}
	want := before&0xffffff00 | uint32(uint8(before)|0x40)
	if m.CPU.EIP != 0x36e72 || services.Calls() != 5 || m.CPU.R[cpu386.EDX] != want {
		t.Fatalf("binary open flags steps=%d EIP=%X calls=%d EDX=%X want=%X flags=%X", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EDX], want, m.CPU.EFlags)
	}
}

func TestFD2AppliesOpenModeFlags(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36ed2; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("apply open flags setup: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	addr, source := m.CPU.R[cpu386.EBX]+0x0c, m.CPU.R[cpu386.EAX]
	before, errRead := m.Read32(addr)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("apply open flags OR: EIP=%X: %v", m.CPU.EIP, err)
	}
	after, errAfter := m.Read32(addr)
	if m.CPU.EIP != 0x36ed5 || services.Calls() != 5 || errRead != nil || errAfter != nil || after != before|source {
		t.Fatalf("apply open flags steps=%d EIP=%X calls=%d before=%X source=%X after=%X errors=%v/%v", steps, m.CPU.EIP, services.Calls(), before, source, after, errRead, errAfter)
	}
}

func TestFD2ReloadsOpenModeByte(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36edb; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("reload mode setup: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	addr, flags := m.CPU.R[cpu386.EAX], m.CPU.EFlags
	want, errRead := m.Read8(addr)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("reload mode MOVZX: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x36ede || services.Calls() != 5 || errRead != nil || m.CPU.R[cpu386.EAX] != uint32(want) || m.CPU.EFlags != flags {
		t.Fatalf("reload mode steps=%d EIP=%X calls=%d EAX=%X want=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], want, m.CPU.EFlags, errRead)
	}
}

func TestFD2SavesNormalizedOpenMode(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x36ee7; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("save mode setup: step=%d EIP=%X EAX=%X EBP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBP], err)
		}
	}
	want, flags := uint8(m.CPU.R[cpu386.EAX]), m.CPU.EFlags
	addr := m.CPU.R[cpu386.EBP] - 4
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("save mode MOV: EIP=%X: %v", m.CPU.EIP, err)
	}
	got, errRead := m.Read8(addr)
	if m.CPU.EIP != 0x36eea || services.Calls() != 5 || errRead != nil || got != want || m.CPU.EFlags != flags {
		t.Fatalf("save mode steps=%d EIP=%X calls=%d got=%X want=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), got, want, m.CPU.EFlags, errRead)
	}
}

func TestFD2BuildsDOSOpenMode(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3cd67; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DOS open mode setup: step=%d EIP=%X EAX=%X EBP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBP], err)
		}
	}
	before, base := m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBP]
	source, errRead := m.Read8(base + 0x1c)
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("DOS open mode OR: EIP=%X: %v", m.CPU.EIP, err)
	}
	want := before&0xffffff00 | uint32(uint8(before)|source)
	if m.CPU.EIP != 0x3cd6a || services.Calls() != 5 || errRead != nil || m.CPU.R[cpu386.EAX] != want || m.CPU.R[cpu386.EBP] != base {
		t.Fatalf("DOS open mode steps=%d EIP=%X calls=%d EAX=%X want=%X source=%X EBP=%X err=%v", steps, m.CPU.EIP, services.Calls(), m.CPU.R[cpu386.EAX], want, source, m.CPU.R[cpu386.EBP], errRead)
	}
}

func TestFD2InitializesDOSOpenHandle(t *testing.T) {
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3cd6a; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("open handle setup: step=%d EIP=%X EBP=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EBP], err)
		}
	}
	base, flags := m.CPU.R[cpu386.EBP], m.CPU.EFlags
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("open handle init: EIP=%X: %v", m.CPU.EIP, err)
	}
	got, errRead := m.Read32(base - 8)
	if m.CPU.EIP != 0x3cd71 || services.Calls() != 5 || errRead != nil || got != 0xffffffff || m.CPU.R[cpu386.EBP] != base || m.CPU.EFlags != flags {
		t.Fatalf("open handle steps=%d EIP=%X calls=%d got=%X EBP=%X flags=%X err=%v", steps, m.CPU.EIP, services.Calls(), got, m.CPU.R[cpu386.EBP], m.CPU.EFlags, errRead)
	}
}

func TestFD2OpensMDIINI(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3cd73; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("MDI.INI open setup: step=%d EIP=%X EAX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EDX], err)
		}
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("MDI.INI open INT 21h: EIP=%X: %v", m.CPU.EIP, err)
	}
	handle := uint16(m.CPU.R[cpu386.EAX])
	if m.CPU.EIP != 0x3cd75 || m.CPU.EFlags&cpu386.CF != 0 || handle < 5 || !services.HasHandle(handle) {
		t.Fatalf("MDI.INI open steps=%d EIP=%X EAX=%X flags=%X handle=%t", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags, services.HasHandle(handle))
	}
}

func TestFD2ConsumesDOSOpenCarry(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3cd79; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DOS open carry consumer: step=%d EIP=%X EAX=%X flags=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags, err)
		}
	}
	handle := uint16(m.CPU.R[cpu386.EAX])
	if m.CPU.EIP != 0x3cd79 || m.CPU.R[cpu386.EAX]&0x80000000 != 0 || m.CPU.EFlags&cpu386.CF != 0 || !services.HasHandle(handle) {
		t.Fatalf("DOS open carry steps=%d EIP=%X EAX=%X flags=%X handle=%t", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags, services.HasHandle(handle))
	}
}

func TestFD2StoresOpenedHandle(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3cd85; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("DOS handle store: step=%d EIP=%X EAX=%X flags=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags, err)
		}
	}
	addr := m.CPU.R[cpu386.EBP] - 8
	if uint64(addr)+4 > uint64(len(m.Mem)) {
		t.Fatalf("DOS handle store address=%X 超界", addr)
	}
	value := binary.LittleEndian.Uint32(m.Mem[addr : addr+4])
	handle := uint16(value)
	if m.CPU.EIP != 0x3cd85 || value != uint32(handle) || !services.HasHandle(handle) {
		t.Fatalf("DOS handle store steps=%d EIP=%X value=%X handle=%t", steps, m.CPU.EIP, value, services.HasHandle(handle))
	}
}

func TestFD2TestsIOModeFlag(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x46375; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("I/O mode flag setup: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	addr := m.CPU.R[cpu386.EBX] + m.CPU.R[cpu386.EAX] + 1
	if uint64(addr) >= uint64(len(m.Mem)) {
		t.Fatalf("I/O mode flag address=%X 超界", addr)
	}
	before := m.Mem[addr]
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("I/O mode flag TEST: EIP=%X: %v", m.CPU.EIP, err)
	}
	wantZero := before&0x40 == 0
	gotZero := m.CPU.EFlags&cpu386.ZF != 0
	if m.CPU.EIP != 0x4637a || m.Mem[addr] != before || gotZero != wantZero {
		t.Fatalf("I/O mode flag steps=%d EIP=%X addr=%X before=%X after=%X ZF=%t", steps, m.CPU.EIP, addr, before, m.Mem[addr], gotZero)
	}
}

func TestFD2MarksIOModeFlag(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x4637d; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("I/O mode flag OR setup: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	addr := m.CPU.R[cpu386.EBX] + m.CPU.R[cpu386.EAX] + 1
	if uint64(addr) >= uint64(len(m.Mem)) {
		t.Fatalf("I/O mode flag address=%X 超界", addr)
	}
	before := m.Mem[addr]
	if before&0x40 != 0 {
		t.Fatalf("I/O mode flag 在 OR 前已設定：addr=%X value=%X", addr, before)
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatalf("I/O mode flag OR: EIP=%X: %v", m.CPU.EIP, err)
	}
	if m.CPU.EIP != 0x46382 || m.Mem[addr] != before|0x40 {
		t.Fatalf("I/O mode flag steps=%d EIP=%X addr=%X before=%X after=%X", steps, m.CPU.EIP, addr, before, m.Mem[addr])
	}
}

func TestFD2MovesHandleIntoBXForIOCTL(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3fb19; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("isatty handle move: step=%d EIP=%X EAX=%X EBX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], err)
		}
	}
	handle := uint16(m.CPU.R[cpu386.EBX])
	if m.CPU.EIP != 0x3fb19 || uint16(m.CPU.R[cpu386.EAX]) != handle || !services.HasHandle(handle) {
		t.Fatalf("isatty handle move steps=%d EIP=%X EAX=%X EBX=%X handle=%t", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], services.HasHandle(handle))
	}
}

func TestFD2QueriesOpenedFileDeviceInformation(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3fb1f; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("isatty IOCTL: step=%d EIP=%X EAX=%X EBX=%X EDX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX], err)
		}
	}
	handle := uint16(m.CPU.R[cpu386.EBX])
	if m.CPU.EIP != 0x3fb1f || m.CPU.EFlags&cpu386.CF != 0 || uint16(m.CPU.R[cpu386.EDX])&0x80 != 0 || !services.HasHandle(handle) {
		t.Fatalf("isatty IOCTL steps=%d EIP=%X EAX=%X EBX=%X EDX=%X flags=%X handle=%t", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.EDX], m.CPU.EFlags, services.HasHandle(handle))
	}
}

func TestFD2TestsOpenedFileDeviceBit(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3fb26; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("isatty device bit: step=%d EIP=%X EDX=%X flags=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EDX], m.CPU.EFlags, err)
		}
	}
	if m.CPU.EIP != 0x3fb26 || uint16(m.CPU.R[cpu386.EDX])&0x80 != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("isatty device bit steps=%d EIP=%X EDX=%X flags=%X", steps, m.CPU.EIP, m.CPU.R[cpu386.EDX], m.CPU.EFlags)
	}
}

func TestFD2NormalizesOpenedFileDeviceBit(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3fb29; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("isatty SETNZ: step=%d EIP=%X EAX=%X flags=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags, err)
		}
	}
	if m.CPU.EIP != 0x3fb29 || uint8(m.CPU.R[cpu386.EAX]) != 0 || m.CPU.EFlags&cpu386.ZF == 0 {
		t.Fatalf("isatty SETNZ steps=%d EIP=%X EAX=%X flags=%X", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
}

func TestFD2ReturnsOpenedFileIsNotTTY(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	steps := 0
	for ; steps < 5000 && m.CPU.EIP != 0x3fb2c; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("isatty return normalize: step=%d EIP=%X EAX=%X: %v", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX], err)
		}
	}
	if m.CPU.EIP != 0x3fb2c || m.CPU.R[cpu386.EAX] != 0 {
		t.Fatalf("isatty return normalize steps=%d EIP=%X EAX=%X", steps, m.CPU.EIP, m.CPU.R[cpu386.EAX])
	}
}
