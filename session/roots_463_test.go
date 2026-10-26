package session

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
	"os"
	"path/filepath"
	"testing"
)

func Test463BootOriginal(t *testing.T) {
	o := bootTestOwner(t)
	defer o.Close()
	exe := minimalBootMZ(t)
	a, b := t.TempDir(), t.TempDir()
	os.Chmod(a, 0555)
	defer os.Chmod(a, 0755)
	r, e := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), OriginalRoot: a, SaveRoot: b})
	if e != nil {
		t.Fatal(e)
	}
	if r.Steps != 0 || o.machine.Steps != 0 || o.dos.Root != physical463(t, a) || o.dos.Scratch != physical463(t, b) {
		t.Fatal("split boot mismatch")
	}
}
func Test463BootPairRejects(t *testing.T) {
	exe := minimalBootMZ(t)
	a, b := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(a, "sub"), 0700)
	os.Symlink(a, filepath.Join(b, "link"))
	os.WriteFile(filepath.Join(b, "file"), []byte("x"), 0600)
	for _, pair := range [][2]string{{a, a}, {a, filepath.Join(a, "sub")}, {filepath.Join(a, "sub"), a}, {filepath.Join(b, "link"), b}, {a, filepath.Join(b, "link")}, {filepath.Join(b, "file"), a}, {a, filepath.Join(b, "file")}} {
		t.Run(pair[0]+pair[1], func(t *testing.T) {
			for _, isState := range []bool{false, true} {
				o := bootTestOwner(t)
				before, _ := os.Stat(a)
				var err error
				if isState {
					raw := []byte("invalid state after root gate")
					_, err = o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), OriginalRoot: pair[0], SaveRoot: pair[1]})
				} else {
					_, err = o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), OriginalRoot: pair[0], SaveRoot: pair[1]})
				}
				after, _ := os.Stat(a)
				if err == nil || o.phase != PhaseBooting || o.machine.Steps != 0 {
					t.Fatal("bad pair changed owner", err)
				}
				if !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("pair validation wrote original")
				}
				o.Close()
			}
		})
	}
}
func Test463BootStateZeroInstruction(t *testing.T) {
	old := t.TempDir()
	m := machine.New()
	if e := m.LoadCOM([]byte{0xeb, 0xfe}); e != nil {
		t.Fatal(e)
	}
	d := dos.New(m, old)
	d.Install()
	p := filepath.Join(t.TempDir(), "snapshot")
	if e := state.Save(p, m, d); e != nil {
		t.Fatal(e)
	}
	d.Close()
	o := newStateOwner(t, stateConfig())
	a, s := t.TempDir(), t.TempDir()
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	r, e := o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), OriginalRoot: a, SaveRoot: s})
	if e != nil {
		t.Fatal(e)
	}
	if r.Steps != 0 || o.machine.Steps != 0 || o.dos.Root != physical463(t, a) || o.dos.Scratch != physical463(t, s) || o.dos.RebaseOriginal != "" {
		t.Fatal("state split boot mismatch")
	}
}
func Test463MachineFailureClearsRebase(t *testing.T) {
	o := newStateOwner(t, stateConfig())
	raw := []byte("not gzip")
	_, e := o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), OriginalRoot: t.TempDir(), SaveRoot: t.TempDir()})
	if e == nil || o.dos.RebaseOriginal != "" || o.dos.RebaseTo != "" || o.dos.RebaseFrom != nil {
		t.Fatal("machine failure retained rebase")
	}
}

func Test463SaveMustBeWritable(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.Chmod(b, 0555)
	defer os.Chmod(b, 0755)
	exe := minimalBootMZ(t)
	for _, isState := range []bool{false, true} {
		o := bootTestOwner(t)
		var e error
		if isState {
			raw := []byte("root gate first")
			_, e = o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), OriginalRoot: a, SaveRoot: b})
		} else {
			_, e = o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), OriginalRoot: a, SaveRoot: b})
		}
		if e == nil || o.phase != PhaseBooting {
			t.Fatal("unwritable save accepted/past preflight", e)
		}
		o.Close()
	}
}
func Test463ZeroJSON(t *testing.T) {
	type oldBoot struct {
		EXE               []byte
		ExpectedEXESHA256 [32]byte
		SaveRoot          string
	}
	type oldState struct {
		State               []byte
		ExpectedStateSHA256 [32]byte
		SaveRoot            string
		RebaseFrom          []string
	}
	a, _ := json.Marshal(BootInput{EXE: []byte("x"), SaveRoot: "unchanged"})
	b, _ := json.Marshal(oldBoot{EXE: []byte("x"), SaveRoot: "unchanged"})
	if string(a) != string(b) {
		t.Fatal("zero BootInput JSON changed", string(a))
	}
	a, _ = json.Marshal(StateInput{State: []byte("x"), SaveRoot: "unchanged"})
	b, _ = json.Marshal(oldState{State: []byte("x"), SaveRoot: "unchanged"})
	if string(a) != string(b) {
		t.Fatal("zero StateInput JSON changed", string(a))
	}
}

func physical463(t *testing.T, p string) string {
	t.Helper()
	q, e := filepath.EvalSymlinks(p)
	if e != nil {
		t.Fatal(e)
	}
	q, e = filepath.Abs(q)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
