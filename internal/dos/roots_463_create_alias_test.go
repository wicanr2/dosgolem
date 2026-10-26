package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// All fixtures are disposable, host-writable synthetic files. A read-only
// mount must not hide an application-level failure to protect OriginalRoot.
func Test463CreateAliasProtectsOriginal(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			d, m := split463(t)
			original := filepath.Join(d.Root, "DATA")
			writeFile(t, d.Root, "DATA", "synthetic original unchanged")
			alias := filepath.Join(d.Scratch, "ABCDEFGH.SAV")
			var err error
			if kind == "symlink" {
				err = os.Symlink(original, alias)
			} else {
				err = os.Link(original, alias)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(original)
			if err != nil {
				t.Fatal(err)
			}
			putName(m, m.CPU, "ABCDEFGHI.SAV")
			int21(m, d, 0x3C00)
			denied := m.CPU.Flags&cpu.CF != 0 && m.CPU.R[cpu.AX] == 5
			data, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(original)
			if err != nil {
				t.Fatal(err)
			}
			if !denied || string(data) != "synthetic original unchanged" || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("alias %s: denied=%v original=%q mtimeEqual=%v", kind, denied, data, before.ModTime().Equal(after.ModTime()))
			}
			if _, err := os.Lstat(filepath.Join(d.Scratch, "ABCDEFGHI.SAV")); !os.IsNotExist(err) {
				t.Fatalf("denied request created long name: %v", err)
			}
		})
	}
}

func Test463CreateSafeAliasAndFreshName(t *testing.T) {
	for _, kind := range []string{"short", "casefold", "fresh"} {
		t.Run(kind, func(t *testing.T) {
			d, m := split463(t)
			wanted, existing := "ABCDEFGHI.SAV", "ABCDEFGH.SAV"
			if kind == "casefold" {
				wanted, existing = "SAVE.SAV", "save.sav"
			}
			if kind == "fresh" {
				wanted, existing = "FRESH.SAV", "FRESH.SAV"
			}
			if kind != "fresh" {
				writeFile(t, d.Scratch, existing, "old save")
			}
			putName(m, m.CPU, wanted)
			int21(m, d, 0x3C00)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("safe create denied: AX=%d", m.CPU.R[cpu.AX])
			}
			data, err := os.ReadFile(filepath.Join(d.Scratch, existing))
			if err != nil || len(data) != 0 {
				t.Fatalf("safe target was not truncated/created: %q %v", data, err)
			}
			if kind != "fresh" {
				entries, err := os.ReadDir(d.Scratch)
				if err != nil || len(entries) != 1 {
					t.Fatalf("duplicate alias: %v %v", entries, err)
				}
			}
		})
	}
}
