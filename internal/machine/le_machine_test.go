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
	if m.CPU.EIP != 0x3c964 || m.CPU.R[cpu386.ESP] != 0x556b0 {
		t.Fatalf("unexpected entry state: EIP=%X ESP=%X", m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
	wantScanBytes := []byte{0x80, 0x3e, 0x00, 0xac, 0x75, 0xfa, 0x80, 0x3e, 0x00, 0x75, 0xe0, 0xac, 0x46, 0x46, 0x80, 0x3e, 0x00, 0xa4, 0x75, 0xfa, 0x1f}
	if got := m.Mem[0x3cb27 : 0x3cb27+uint32(len(wantScanBytes))]; !bytes.Equal(got, wantScanBytes) {
		t.Fatalf("environment scan bytes=% X", got)
	}
	services := &FD2StartupDOS{}
	m.CPU.IntHook = services.Handle
	for steps := 0; m.CPU.EIP != 0x45dcf && steps < 235; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x45dcf {
		t.Fatalf("entry did not branch past environment prefix test: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x3cbcc || m.CPU.R[cpu386.EBX] != 0x539c2 || m.CPU.Seg[cpu386.SegGS] != 0x20 {
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
	if m.CPU.R[cpu386.EAX] != 0x3cbcc || m.CPU.EFlags&cpu386.DF != 0 || m.CPU.EFlags&cpu386.ZF != 0 {
		t.Fatalf("first callee accumulator EAX=%X flags=%X", m.CPU.R[cpu386.EAX], m.CPU.EFlags)
	}
	if m.CPU.R[cpu386.ESI] != 0x539e0 || m.CPU.R[cpu386.EDI] != 0x539e0 || m.CPU.R[cpu386.EBX] != 0x539c2 {
		t.Fatalf("callee table pointers ESI=%X EDI=%X EBX=%X", m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EBX])
	}
	if m.CPU.EFlags&cpu386.CF != 0 || m.CPU.EFlags&cpu386.ZF != 0 {
		t.Fatalf("callee callback nonzero flags=%X", m.CPU.EFlags)
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
