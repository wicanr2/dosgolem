package bootroot

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/session"
)

// sessionNew builds a sealed Booting owner for the handoff test.
func sessionNew(t *testing.T) (*session.Owner, error) {
	t.Helper()
	return session.New(session.Config{InitialScale: host.OutputScale2})
}

func bootrootFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func bootrootRequired(t *testing.T, dir string, names ...string) []RequiredFile {
	t.Helper()
	var out []RequiredFile
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, RequiredFile{Name: name, SHA256: sha256.Sum256(data)})
	}
	return out
}

func TestPrepareSuccessCopiesTree(t *testing.T) {
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{
		"START.EXE":      "fake-exe-bytes",
		"DATA.DAX":       "fake-data",
		"SUB/NESTED.DAX": "nested",
	})
	required := bootrootRequired(t, original, "START.EXE", "DATA.DAX")
	save := filepath.Join(t.TempDir(), "save")
	out, err := Prepare(BootRootInput{OriginalRoot: original, SaveRoot: save, Required: required})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(out.SaveRoot) {
		t.Fatalf("output not absolute: %q", out.SaveRoot)
	}
	if len(out.Verified) != 2 {
		t.Fatalf("verified = %v", out.Verified)
	}
	for name, sum := range out.Verified {
		data, err := os.ReadFile(filepath.Join(out.SaveRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(data) != sum {
			t.Fatalf("%s changed in save", name)
		}
	}
	for _, tc := range []struct {
		path string
		mode os.FileMode
		dir  bool
	}{
		{out.SaveRoot, 0o700, true},
		{filepath.Join(out.SaveRoot, "SUB"), 0o700, true},
		{filepath.Join(out.SaveRoot, "START.EXE"), 0o600, false},
	} {
		info, err := os.Lstat(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() != tc.dir || info.Mode().Perm() != tc.mode {
			t.Fatalf("%s mode = %v dir = %v", tc.path, info.Mode(), info.IsDir())
		}
	}
}

func TestPrepareRejectsWithoutSideEffects(t *testing.T) {
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{"START.EXE": "fake-exe-bytes"})
	good := bootrootRequired(t, original, "START.EXE")
	badSum := good
	badSum[0].SHA256 = sha256.Sum256([]byte("other"))
	big := make([]byte, maxRequiredFileSize+1)
	file := filepath.Join(original, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(original, "link")
	if err := os.Symlink(original, link); err != nil {
		t.Fatal(err)
	}
	bigName := "BIG.DAX"
	if err := os.WriteFile(filepath.Join(original, bigName), big, 0o644); err != nil {
		t.Fatal(err)
	}
	bigReq := RequiredFile{Name: bigName, SHA256: sha256.Sum256(big)}
	nonEmpty := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonEmpty, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	revParent := parentSave(t)
	for _, tc := range []struct {
		name  string
		input BootRootInput
	}{
		{"missing original", BootRootInput{OriginalRoot: filepath.Join(original, "missing"), SaveRoot: filepath.Join(t.TempDir(), "s"), Required: good}},
		{"file original", BootRootInput{OriginalRoot: file, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: good}},
		{"symlink original", BootRootInput{OriginalRoot: link, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: good}},
		{"empty required", bootInputShim(original, t)},
		{"dup required", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{good[0], good[0]}}},
		{"dot name", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: ".", SHA256: good[0].SHA256}}}},
		{"dotdot name", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: "..", SHA256: good[0].SHA256}}}},
		{"backslash name", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: `A\B`, SHA256: good[0].SHA256}}}},
		{"slash name", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: "A/B", SHA256: good[0].SHA256}}}},
		{"colon name", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: "A:B", SHA256: good[0].SHA256}}}},
		{"missing file", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{{Name: "NOPE.EXE", SHA256: good[0].SHA256}}}},
		{"wrong hash", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: badSum}},
		{"oversize", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s"), Required: []RequiredFile{bigReq}}},
		{"file save", BootRootInput{OriginalRoot: original, SaveRoot: file, Required: good}},
		{"symlink save", BootRootInput{OriginalRoot: original, SaveRoot: link, Required: good}},
		{"same save", BootRootInput{OriginalRoot: original, SaveRoot: original, Required: good}},
		{"nested save", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(original, "sub"), Required: good}},
		{"reverse nested", BootRootInput{OriginalRoot: filepath.Join(revParent, "orig"), SaveRoot: revParent, Required: good}},
		{"missing parent", BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "nope", "s"), Required: good}},
		{"nonempty save", BootRootInput{OriginalRoot: original, SaveRoot: nonEmpty, Required: good}},
		{"dotfile nonempty save", BootRootInput{OriginalRoot: original, SaveRoot: dotSave(t), Required: good}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Prepare(tc.input)
			if err == nil {
				t.Fatal("want rejection")
			}
			if len(out.Verified) != 0 {
				t.Fatal("rejection must return empty Verified")
			}
			absSave, absErr := filepath.Abs(tc.input.SaveRoot)
			if absErr != nil {
				t.Fatal(absErr)
			}
			switch tc.name {
			case "file save":
				if data, err := os.ReadFile(absSave); err != nil || string(data) != "x" {
					t.Fatal("pre-existing save file touched")
				}
			case "symlink save":
				if target, err := os.Readlink(absSave); err != nil || target == "" {
					t.Fatal("pre-existing save symlink touched")
				}
			case "nonempty save":
				assertDirNames(t, absSave, "old")
			case "dotfile nonempty save":
				assertDirNames(t, absSave, ".hidden")
			case "reverse nested":
				assertDirNames(t, filepath.Join(absSave, "orig"), "START.EXE")
			case "same save":
				assertDirNames(t, absSave, "BIG.DAX", "START.EXE", "file", "link")
			default:
				if _, err := os.Lstat(absSave); !os.IsNotExist(err) {
					t.Fatalf("rejection left save behind: %v", err)
				}
			}
		})
	}
}

// parentSave builds P/orig with START.EXE identical to the main fixture.
func parentSave(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	bootrootFixture(t, filepath.Join(parent, "orig"), map[string]string{"START.EXE": "fake-exe-bytes"})
	return parent
}

// dotSave builds a save dir containing only a dotfile (still non-empty).
func dotSave(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// assertDirNames checks a directory holds exactly the named entries.
func assertDirNames(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Fatalf("%s holds %d entries, want %d", dir, len(entries), len(names))
	}
	for i, name := range names {
		if entries[i].Name() != name {
			t.Fatalf("%s entry %d = %q, want %q", dir, i, entries[i].Name(), name)
		}
	}
}
func bootInputShim(original string, t *testing.T) BootRootInput {
	t.Helper()
	return BootRootInput{OriginalRoot: original, SaveRoot: filepath.Join(t.TempDir(), "s")}
}

// TestPrepareRejectsLeaveNoTrace asserts every table rejection returns an
// empty Verified map and creates no save tree, except for pre-existing
// file/symlink/non-empty saves which must be left untouched.

func TestPrepareTreeSymlinkFailsClosed(t *testing.T) {
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{"START.EXE": "fake-exe-bytes"})
	if err := os.Symlink("START.EXE", filepath.Join(original, "EVIL.LNK")); err != nil {
		t.Fatal(err)
	}
	required := bootrootRequired(t, original, "START.EXE")
	save := filepath.Join(t.TempDir(), "save")
	if _, err := Prepare(BootRootInput{OriginalRoot: original, SaveRoot: save, Required: required}); err == nil {
		t.Fatal("want rejection for in-tree symlink")
	}
	if _, err := os.Lstat(save); !os.IsNotExist(err) {
		t.Fatal("save root must not remain after failed prepare")
	}
}

func TestPrepareUnreadableSaveRejected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{"START.EXE": "fake-exe-bytes"})
	required := bootrootRequired(t, original, "START.EXE")
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := Prepare(BootRootInput{OriginalRoot: original, SaveRoot: locked, Required: required}); err == nil {
		t.Fatal("want rejection for unwritable save")
	}
}

func TestPrepareConcurrentDistinctRoots(t *testing.T) {
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{"START.EXE": "fake-exe-bytes"})
	required := bootrootRequired(t, original, "START.EXE")
	parent := t.TempDir()
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			_, err := Prepare(BootRootInput{
				OriginalRoot: original,
				SaveRoot:     filepath.Join(parent, string(rune('a'+i))),
				Required:     required,
			})
			done <- err
		}(i)
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrepareHandsSaveToSessionBoot(t *testing.T) {
	original := t.TempDir()
	mz := make([]byte, 512)
	mz[0], mz[1] = 'M', 'Z'
	mz[4] = 1
	mz[8] = 2
	if err := os.WriteFile(filepath.Join(original, "START.EXE"), mz, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mz)
	out, err := Prepare(BootRootInput{
		OriginalRoot: original,
		SaveRoot:     filepath.Join(t.TempDir(), "save"),
		Required:     []RequiredFile{{Name: "START.EXE", SHA256: sum}},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := sessionNew(t)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.ReadFile(filepath.Join(out.SaveRoot, "START.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := owner.BootOriginal(session.BootInput{EXE: exe, ExpectedEXESHA256: sum, SaveRoot: out.SaveRoot})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Phase != session.PhaseRunning {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestPrepareSameRootConcurrentOneWins(t *testing.T) {
	original := t.TempDir()
	bootrootFixture(t, original, map[string]string{"START.EXE": "fake-exe-bytes"})
	required := bootrootRequired(t, original, "START.EXE")
	save := filepath.Join(t.TempDir(), "save")
	const racers = 8
	results := make(chan error, racers)
	for i := 0; i < racers; i++ {
		go func() {
			_, err := Prepare(BootRootInput{OriginalRoot: original, SaveRoot: save, Required: required})
			results <- err
		}()
	}
	wins := 0
	for i := 0; i < racers; i++ {
		if err := <-results; err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("same-root race: %d winners, want 1", wins)
	}
	data, err := os.ReadFile(filepath.Join(save, "START.EXE"))
	if err != nil || string(data) != "fake-exe-bytes" {
		t.Fatal("winning tree is corrupt or incomplete")
	}
}

func TestRemoveCreatedDepthFirst(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(sub, "leaf")
	if err := os.WriteFile(leaf, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeCreated(root, []string{sub, leaf}, false)
	if _, err := os.Lstat(leaf); !os.IsNotExist(err) {
		t.Fatal("leaf not removed")
	}
	if _, err := os.Lstat(sub); !os.IsNotExist(err) {
		t.Fatal("subdir not removed")
	}
	if _, err := os.Lstat(root); err != nil {
		t.Fatal("pre-existing root must survive")
	}
	scratch := filepath.Join(t.TempDir(), "scratch")
	if err := os.Mkdir(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	removeCreated(scratch, nil, true)
	if _, err := os.Lstat(scratch); !os.IsNotExist(err) {
		t.Fatal("created root not removed")
	}
}

// TestColdBootOriginalGameThroughSealedPath is the original acceptance for
// specs 235+236: from the real read-only tree, through Prepare and
// session.BootOriginal, to a Running owner with zero steps.
// No checkpoints, no teleports.  Live stepping needs a Deliver-accepted
// turn and belongs to later slices.  Requires BUCK_COLD_BOOT_ORIGINAL
// to point at the local original tree.
func TestColdBootOriginalGameThroughSealedPath(t *testing.T) {
	original := os.Getenv("BUCK_COLD_BOOT_ORIGINAL")
	if original == "" {
		t.Skip("real original tree not provided")
	}
	const startEXE = "58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1"
	raw, err := hex.DecodeString(startEXE)
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	copy(want[:], raw)
	out, err := Prepare(BootRootInput{
		OriginalRoot: original,
		SaveRoot:     filepath.Join(t.TempDir(), "save"),
		Required:     []RequiredFile{{Name: "START.EXE", SHA256: want}},
	})
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.ReadFile(filepath.Join(out.SaveRoot, "START.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := sessionNew(t)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := owner.BootOriginal(session.BootInput{EXE: exe, ExpectedEXESHA256: want, SaveRoot: out.SaveRoot})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Phase != session.PhaseRunning || receipt.EXESHA256 != want {
		t.Fatalf("receipt = %+v", receipt)
	}
	if got := owner.Status(); got.Phase != session.PhaseRunning {
		t.Fatalf("status = %+v", got)
	}
	// Boot itself must not step.  Live stepping needs a Deliver-accepted
	// turn (layout + generation), which belongs to turn-contract slices,
	// not to boot acceptance.
}
