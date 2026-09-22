package dosgolem_test

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem"
)

func TestStateFileRoundTripIsAvailableOutsideInternalPackages(t *testing.T) {
	m := dosgolem.New()
	d := dosgolem.NewDOS(m, ".")
	d.Install()
	m.Write8(0x2345, 0x6a)
	path := filepath.Join(t.TempDir(), "state.gz")
	if err := dosgolem.SaveStateFile(path, m, d); err != nil {
		t.Fatal(err)
	}

	restored := dosgolem.New()
	restoredDOS := dosgolem.NewDOS(restored, ".")
	restoredDOS.Install()
	if err := dosgolem.LoadStateFile(path, restored, restoredDOS); err != nil {
		t.Fatal(err)
	}
	if got := restored.Read8(0x2345); got != 0x6a {
		t.Fatalf("restored memory=%#x, want 0x6a", got)
	}
}
