package buckrogers

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// Local-only original receipt comparison. The control, 2x and 3x savestates
// are never committed. gob map iteration makes raw state file SHA unstable;
// compare loaded machine values and the DOS section instead.
func TestStoryPage9PrivateSameState(t *testing.T) {
	paths := []string{os.Getenv("STORY_PAGE9_CONTROL_STATE"), os.Getenv("STORY_PAGE9_2X_STATE"), os.Getenv("STORY_PAGE9_3X_STATE")}
	for _, path := range paths {
		if path == "" {
			t.Skip("private same-state receipts absent")
		}
	}
	var baseline *machine.Snapshot
	var dosBaseline []byte
	for i, path := range paths {
		m := machine.New()
		d := dos.New(m, ".")
		d.Install()
		if err := state.Load(path, m, d); err != nil {
			t.Fatal(err)
		}
		if m.Steps != 352100000 {
			t.Fatalf("receipt %d stopped at %d", i, m.Steps)
		}
		var savedDOS bytes.Buffer
		if err := d.SaveState(&savedDOS); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			baseline, dosBaseline = m.Snapshot(), savedDOS.Bytes()
			continue
		}
		if !reflect.DeepEqual(baseline, m.Snapshot()) {
			t.Fatalf("receipt %d machine state differs", i)
		}
		if !bytes.Equal(dosBaseline, savedDOS.Bytes()) {
			t.Fatalf("receipt %d DOS state differs", i)
		}
	}
}
