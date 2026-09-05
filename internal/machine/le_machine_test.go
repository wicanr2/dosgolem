package machine

import (
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
	services := &FD2StartupDOS{}
	m.CPU.IntHook = services.Handle
	for steps := 0; m.CPU.EIP != 0x3ca7a && steps < 44; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if services.Calls() != 2 || m.CPU.EIP != 0x3ca7a {
		t.Fatalf("entry did not enter DOS/4GW success branch: calls=%d EIP=%X", services.Calls(), m.CPU.EIP)
	}
	if m.CPU.R[cpu386.EAX] != 0x4734ffff || m.CPU.Seg[cpu386.SegGS] != 0x20 || m.CPU.EFlags&cpu386.ZF != 0 {
		t.Fatalf("DOS/4GW return mismatch: EAX=%X GS=%X flags=%X", m.CPU.R[cpu386.EAX], m.CPU.Seg[cpu386.SegGS], m.CPU.EFlags)
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
	word, err := m.Read16(0x52810)
	if err != nil {
		t.Fatal(err)
	}
	if word != 0x24 || m.CPU.R[cpu386.EBX] != 0x50484152 {
		t.Fatalf("entry globals/register mismatch: word=%X EBX=%X", word, m.CPU.R[cpu386.EBX])
	}
	if m.Mem[0x5283a] != 6 || m.Mem[0x5283b] != 22 {
		t.Fatalf("DOS version globals=%d.%d", m.Mem[0x5283a], m.Mem[0x5283b])
	}
}
