package session

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// minimalBootMZ builds a 512-byte MZ image: 28-byte header, HeaderPar=2,
// one 512-byte page, no relocations.  Load-only fixture, never stepped.
func minimalBootMZ(t *testing.T) []byte {
	t.Helper()
	out := make([]byte, 512)
	out[0], out[1] = 'M', 'Z'
	binary.LittleEndian.PutUint16(out[4:], 1)
	binary.LittleEndian.PutUint16(out[8:], 2)
	return out
}

func bootTestOwner(t *testing.T) *Owner {
	t.Helper()
	o, err := New(Config{InitialScale: host.OutputScale2})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestBootOriginalNilOwnerRejected(t *testing.T) {
	var o *Owner
	if _, err := o.BootOriginal(BootInput{}); err == nil {
		t.Fatal("want rejection for nil owner")
	}
}

func TestBootOriginalRejectsWithoutFailing(t *testing.T) {
	save := t.TempDir()
	file := filepath.Join(save, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(save, "link")
	if err := os.Symlink(save, link); err != nil {
		t.Fatal(err)
	}
	good := minimalBootMZ(t)
	goodSum := sha256.Sum256(good)
	var zero [32]byte
	for _, tc := range []struct {
		name  string
		input BootInput
	}{
		{"empty EXE", BootInput{SaveRoot: save, ExpectedEXESHA256: goodSum}},
		{"oversize EXE", BootInput{EXE: make([]byte, maxBootEXESize+1), SaveRoot: save, ExpectedEXESHA256: goodSum}},
		{"zero hash", BootInput{EXE: good, SaveRoot: save, ExpectedEXESHA256: zero}},
		{"wrong hash", BootInput{EXE: good, SaveRoot: save, ExpectedEXESHA256: sha256.Sum256([]byte("nope"))}},
		{"empty root", BootInput{EXE: good, ExpectedEXESHA256: goodSum}},
		{"missing root", BootInput{EXE: good, SaveRoot: filepath.Join(save, "missing"), ExpectedEXESHA256: goodSum}},
		{"file root", BootInput{EXE: good, SaveRoot: file, ExpectedEXESHA256: goodSum}},
		{"symlink root", BootInput{EXE: good, SaveRoot: link, ExpectedEXESHA256: goodSum}},
		{"nul root", BootInput{EXE: good, SaveRoot: save + "\x00", ExpectedEXESHA256: goodSum}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := bootTestOwner(t)
			if _, err := o.BootOriginal(tc.input); err == nil {
				t.Fatal("want rejection")
			}
			if got := o.Status(); got.Phase != PhaseBooting || got.FirstFault != nil {
				t.Fatalf("rejection changed phase: %+v", got)
			}
		})
	}
}

func TestBootOriginalUnreadableRootRejected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	save := t.TempDir()
	locked := filepath.Join(save, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	good := minimalBootMZ(t)
	o := bootTestOwner(t)
	input := BootInput{EXE: good, ExpectedEXESHA256: sha256.Sum256(good), SaveRoot: locked}
	if _, err := o.BootOriginal(input); err == nil {
		t.Fatal("want rejection for unwritable root")
	}
	if got := o.Status(); got.Phase != PhaseBooting || got.FirstFault != nil {
		t.Fatalf("rejection changed phase: %+v", got)
	}
}

func TestBootOriginalBadImageFailsClosed(t *testing.T) {
	o := bootTestOwner(t)
	bad := []byte("not an executable at all")
	input := BootInput{EXE: bad, ExpectedEXESHA256: sha256.Sum256(bad), SaveRoot: t.TempDir()}
	if _, err := o.BootOriginal(input); err == nil {
		t.Fatal("want load failure")
	}
	if got := o.Status(); got.Phase != PhaseFailed || got.FirstFault == nil {
		t.Fatalf("bad image did not fail closed: %+v", got)
	}
	before := o.machine.Steps
	if _, err := o.Advance(InstructionBudget(100)); err == nil {
		t.Fatal("Advance after failed boot must not succeed")
	}
	if got := o.machine.Steps; got != before {
		t.Fatalf("failed boot advanced machine: %d -> %d", before, got)
	}
	if _, err := o.BootOriginal(input); err == nil {
		t.Fatal("second boot must be rejected")
	}
}

func TestBootOriginalSuccessRunsWithoutSteps(t *testing.T) {
	o := bootTestOwner(t)
	before := o.machine.Steps
	exe := minimalBootMZ(t)
	sum := sha256.Sum256(exe)
	receipt, err := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sum, SaveRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Phase != PhaseRunning || receipt.EXESHA256 != sum {
		t.Fatalf("receipt = %+v", receipt)
	}
	if got := o.Status(); got.Phase != PhaseRunning {
		t.Fatalf("status = %+v", got)
	}
	if got := o.machine.Steps; got != before {
		t.Fatalf("boot stepped machine: %d -> %d", before, got)
	}
	// Caller mutating its slice after return must not affect the image:
	// MZ magic at the load address must still read back intact.
	for i := range exe {
		exe[i] = 0xFF
	}
	loadAddr := uint32((machine.PSPSeg + 0x10) * 16)
	if got := o.machine.Read8(loadAddr); got != 0x00 {
		t.Fatalf("loaded image changed after caller rewrite: %#02x", got)
	}
	if _, err := o.BootOriginal(BootInput{EXE: minimalBootMZ(t), ExpectedEXESHA256: sum, SaveRoot: t.TempDir()}); err == nil {
		t.Fatal("second boot must be rejected")
	}
}

// TestBootOriginalHashMatchesLoadedImage hammers hash/load consistency:
// every successful boot's receipt hash must equal the machine image bytes.
// In-call mutation by the caller is a contract violation (single-goroutine
// rule); the copy-first implementation is closed by code review.
func TestBootOriginalHashMatchesLoadedImage(t *testing.T) {
	rng := 0x12345
	next := func(n int) int {
		rng = (rng*1103515245 + 12345) & 0x7fffffff
		return rng % n
	}
	for i := 0; i < 50; i++ {
		exe := minimalBootMZ(t)
		for k := 0; k < 4; k++ {
			exe[32+next(len(exe)-32)] = byte(next(256))
		}
		sum := sha256.Sum256(exe)
		o := bootTestOwner(t)
		receipt, err := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sum, SaveRoot: t.TempDir()})
		if err != nil {
			if got := o.Status(); got.Phase != PhaseFailed {
				t.Fatalf("iter %d: dirty failure left phase %+v", i, got)
			}
			continue
		}
		if receipt.EXESHA256 != sum {
			t.Fatalf("iter %d: receipt hash differs from verified bytes", i)
		}
		base := uint32(o.machine.ImageBase)
		image := make([]byte, 0, o.machine.ImageLen)
		for n := 0; n < o.machine.ImageLen; n++ {
			image = append(image, o.machine.Read8(base+uint32(n)))
		}
		if string(image) != string(exe[32:]) {
			t.Fatalf("iter %d: loaded image differs from verified bytes", i)
		}
	}
}
