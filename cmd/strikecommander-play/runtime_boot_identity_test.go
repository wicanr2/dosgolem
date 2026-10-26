//go:build frontendlinked

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/session"
)

type frontendBootMemory struct {
	psp   uint16
	image []byte
	calls int
	err   error
}

func (m *frontendBootMemory) ReadBootPSP(r []session.BootPSPRange) (session.BootPSPReceipt, error) {
	m.calls++
	if m.err != nil {
		return session.BootPSPReceipt{}, m.err
	}
	for _, v := range r {
		if v.Offset < 256 || uint64(v.Offset-256)+uint64(len(v.Dst)) > uint64(len(m.image)) {
			return session.BootPSPReceipt{}, errors.New("fixture range")
		}
		copy(v.Dst, m.image[v.Offset-256:])
	}
	return session.BootPSPReceipt{PSP: m.psp}, nil
}

func TestFrontendBootUniqueBatch(t *testing.T) {
	// Independent expected bytes: 1234h relocated at segment 0220h gives 1454h.
	raw := make([]byte, 32+512)
	copy(raw, "MZ")
	for at, value := range map[int]uint16{2: 32, 4: 2, 6: 1, 8: 2, 20: 320, 24: 28, 28: 6} {
		binary.LittleEndian.PutUint16(raw[at:], value)
	}
	for i := 32; i < len(raw); i++ {
		raw[i] = byte(i)
	}
	raw[38], raw[39] = 0x34, 0x12
	p, e := parseFrontendProgram("fixture", raw)
	if e != nil {
		t.Fatal(e)
	}
	want := append([]byte(nil), raw[32:]...)
	want[6], want[7] = 0x54, 0x14
	m := &frontendBootMemory{psp: 0x210, image: want}
	name, e := identifyFrontendBootProgram(m, []frontendProgram{p})
	if e != nil || name != "fixture" || m.calls != 1 {
		t.Fatalf("one batch: %q %v %d", name, e, m.calls)
	}
	// Prefix and entry must both match; a partial match is never accepted.
	for _, at := range []int{0, 63, 255, 320} {
		m.image[at] ^= 1
		if _, e = identifyFrontendBootProgram(m, []frontendProgram{p}); e == nil {
			t.Fatalf("accepted changed byte %d", at)
		}
		m.image[at] ^= 1
	}
	if _, e = identifyFrontendBootProgram(m, []frontendProgram{p, p}); e == nil {
		t.Fatal("accepted ambiguous image")
	}
	m.err = errors.New("physical read failed")
	if _, e = identifyFrontendBootProgram(m, []frontendProgram{p}); !errors.Is(e, m.err) {
		t.Fatal("read failure was hidden", e)
	}
	for _, psp := range []uint16{0, 0xfff0} {
		if _, e = relocatedFrontendProgram(p, psp); e == nil {
			t.Fatal("accepted unrepresentable PSP", psp)
		}
	}
	for _, size := range []int{0, 27, 32, 287} {
		if _, e = parseFrontendProgram("short", raw[:size]); e == nil {
			t.Fatal("accepted truncated MZ", size)
		}
	}
}

func TestFrontendProgramRawEvidence(t *testing.T) {
	root, evidence := os.Getenv("SC_FRONTEND_EXE_ROOT"), os.Getenv("SC_FRONTEND_RAW_EVIDENCE")
	if root == "" || evidence == "" {
		t.Skip("explicit read-only EXE root and frozen raw evidence required; no VM")
	}
	library, e := loadFrontendPrograms(root)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Rows []struct {
			CurPSP     uint16
			Candidates []struct {
				Name, ExpectedPrefixSHA256, ExpectedEntrySHA256 string
				EntryOffset                                     uint32
			}
		}
	}
	raw, e := os.ReadFile(evidence)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	checks := 0
	for _, row := range result.Rows {
		if len(row.Candidates) != len(library) {
			t.Fatal("incomplete frozen candidate row")
		}
		for i, p := range library {
			want := row.Candidates[i]
			image, e := relocatedFrontendProgram(p, row.CurPSP)
			if e != nil {
				t.Fatal(e)
			}
			if p.name != want.Name || p.entry != want.EntryOffset || fmt.Sprintf("%x", sha256.Sum256(image[:256])) != want.ExpectedPrefixSHA256 || fmt.Sprintf("%x", sha256.Sum256(image[p.entry:p.entry+64])) != want.ExpectedEntrySHA256 {
				t.Fatalf("MZ relocation differs from frozen raw evidence: %s PSP=%04x", p.name, row.CurPSP)
			}
			checks += 2
		}
	}
	if len(result.Rows) != 6 || checks != 72 {
		t.Fatalf("want six cases/72 prefix-entry checks: %d/%d", len(result.Rows), checks)
	}
	t.Logf("six-case %d prefix/entry hashes; offline only, no session or guest", checks)
}
