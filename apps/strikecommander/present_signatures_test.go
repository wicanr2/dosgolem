package strikecommander

import (
	"encoding/hex"
	"testing"
)

// Frozen independent oracle transcribed from probe5/out/p17-sigs.txt COPIES
// (lines 80-103), not generated from presentDRoutines. Segment offsets refer to
// the EXE load image; runtime CS = load base + seg. The P17 source hashes are:
// OPTTEST e8c47cfea7cd280b24d9bdb5c5be66e92447da5a3f7082cf8e6dbb4524eaa1f1
// MENU    85147ef71c14a41a38222065859bfa44a4c19c66cf51a89c94da0a9a5a942a29
// STRIKE  883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5
// Return IPs are from READY 014 v14 §3.2, independently written here.
var dFrozenP17 = []struct {
	name            string
	seg, entry, ret uint16
	hex             string
}{
	{"blit", 0x39fb, 0, 0x8b, "558bec83ec0656571e06fc8b5e068b76"},
	{"O.shape", 0x3a97, 0, 0xf0, "c852010056571efc16071e0fa98b7606"},
	{"O.puts", 0x3942, 0x34b, 0x3d3, "558bec83ec0656578b76068b7e08ff74"},
	{"O.restore", 0x3b13, 6, 0xea, "c812000056571e8cd88ec08b7e0c8b5e"},
	{"O.fill", 0x39f6, 4, 0x4f, "558bec83ec025657fc8b46088ae08b5e"},
	{"O.local", 0x39ed, 2, 0x92, "558bec83ec0856578b5e068e078b7f02"},
	{"O.erase", 0x3aff, 0xe, 0x144, "c812000056571ec47e108b5e1443d1e3"},
	{"M.restore", 0x1bdb, 0xe, 0xf2, "c812000056571e8cd88ec08b7e0c8b5e"},
	{"M.fill", 0x1b35, 8, 0x53, "558bec83ec025657fc8b46088ae08b5e"},
	{"M.local", 0x1b2c, 6, 0x96, "558bec83ec0856578b5e068e078b7f02"},
	{"M.blit", 0x1b3a, 4, 0x8f, "558bec83ec0656571e06fc8b5e068b76"},
	{"M.puts", 0x1a82, 0x33f, 0x3c7, "558bec83ec0656578b76068b7e08ff74"},
	{"M.shape", 0x1ba2, 2, 0xf2, "c852010056571efc16071e0fa98b7606"},
	{"M.erase", 0x1bc8, 6, 0x13c, "c812000056571ec47e108b5e1443d1e3"},
	{"M.zoom", 0x3bf, 6, 0xf0, "558bec1efcc5760ac47e068a1446b6ff"},
	{"M.rle", 0x100a, 0xc, 0x3e9, "c81c00005756061e9cfcc5760616078d"},
	{"M.memcpy", 0, 0x34bf, 0x34e2, "558bec56578cdac47e06c5760a8b4e0e"},
	{"S.restore", 0x616e, 0xa, 0xee, "c812000056571e8cd88ec08b7e0c8b5e"},
	{"S.fill", 0x602a, 0xa, 0x55, "558bec83ec025657fc8b46088ae08b5e"},
	{"S.local", 0x6021, 8, 0x98, "558bec83ec0856578b5e068e078b7f02"},
	{"S.erase", 0x615b, 2, 0x138, "c812000056571ec47e108b5e1443d1e3"},
	{"S.blit", 0x602f, 6, 0x91, "558bec83ec0656571e06fc8b5e068b76"},
	{"S.puts", 0x5f7a, 0x345, 0x3cd, "558bec83ec0656578b76068b7e08ff74"},
	{"S.shape", 0x60f2, 4, 0xf4, "c852010056571efc16071e0fa98b7606"},
}

func dFrozenEntry(t *testing.T, name string) (cs, entry, ret uint16, sig [16]byte) {
	t.Helper()
	for _, row := range dFrozenP17 {
		if row.name != name {
			continue
		}
		b, err := hex.DecodeString(row.hex)
		if err != nil || len(b) != 16 {
			t.Fatal("bad frozen P17 literal", name, err)
		}
		copy(sig[:], b)
		return tpBase + row.seg, row.entry, row.ret, sig
	}
	t.Fatal("missing frozen P17 entry", name)
	return
}

func TestDP17FrozenSignatures(t *testing.T) {
	if len(presRoutines) != len(dFrozenP17) {
		t.Fatal("P17 entry count")
	}
	for _, want := range dFrozenP17 {
		got := dRow(t, &Presenter{routines: presRoutines}, want.name)
		_, en, ret, sig := dFrozenEntry(t, want.name)
		if got.seg != want.seg || got.entryIP != en || got.retIP != ret || got.sig != sig {
			t.Errorf("%s differs from independent P17/READY literal: %+v", want.name, got)
		}
	}
}
