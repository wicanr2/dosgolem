package r5asset

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Only the ten fixed asset files are copied into disposable test output. This
// isolates omission/order tests without changing any original bytes or hashes.
func assetCopyWithout(t *testing.T, sourceRoot string, model, slot int) string {
	t.Helper()
	root := t.TempDir()
	for m, p := range profiles {
		dir := filepath.Join(root, p.Name, "model")
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		for s, f := range p.Files {
			if m == model && s == slot {
				continue
			}
			in, e := os.Open(filepath.Join(sourceRoot, p.Name, "model", f.Name))
			if e != nil {
				t.Fatal(e)
			}
			out, e := os.Create(filepath.Join(dir, f.Name))
			if e != nil {
				in.Close()
				t.Fatal(e)
			}
			_, e = io.Copy(out, in)
			ce := out.Close()
			in.Close()
			if e != nil || ce != nil {
				t.Fatal(e, ce)
			}
		}
	}
	return root
}
func TestTransactionRollbackSecondModelAndImagesTwoThroughSix(t *testing.T) {
	sourceRoot, _ := fixtureRoots(t, true, false)
	for _, c := range [][2]int{{1, 0}, {0, 3}, {0, 4}, {1, 2}, {1, 3}, {1, 4}} {
		t.Run(profiles[c[0]].Name+"/"+profiles[c[0]].Files[c[1]].Name, func(t *testing.T) {
			root := assetCopyWithout(t, sourceRoot, c[0], c[1])
			l, o, base := loaderRig(t, 0, root, Limits{})
			e := l.Load()
			if e == nil {
				t.Fatal("missing late file accepted")
			}
			v, ok := e.(Error)
			if !ok || v.Code != Identity || v.Model != uint8(c[0]+1) || v.Slot != uint8(c[1]) {
				t.Fatal("wrong bounded stage/model/slot", e)
			}
			if o.OwnedBytes() != base+l.limits.ControlBytes || l.Stats().Published || l.Stats().Loading {
				t.Fatal("whole batch rollback", l.Stats())
			}
		})
	}
}
func TestReadPinnedPathSHAAndLength(t *testing.T) {
	root := t.TempDir()
	raw := []byte("synthetic bounded IO fixture\n")
	s := fileIdentity{Name: "model.gltf", Bytes: uint64(len(raw)), SHA: sha256.Sum256(raw)}
	name := filepath.Join(root, s.Name)
	var e error
	if e = os.WriteFile(name, raw, 0600); e != nil {
		t.Fatal(e)
	}
	dst := make([]byte, len(raw))
	if e = readPinnedInto(name, s, dst); e != nil {
		t.Fatal(e)
	}
	raw[len(raw)-1] ^= 1
	if e = os.WriteFile(name, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e = readPinnedInto(name, s, dst); e == nil {
		t.Fatal("SHA accepted")
	}
	if e = os.WriteFile(name, raw[:len(raw)-1], 0600); e != nil {
		t.Fatal(e)
	}
	if e = readPinnedInto(name, s, dst); e == nil {
		t.Fatal("length accepted")
	}
	link := filepath.Join(root, "link")
	if e = os.Symlink(name, link); e != nil {
		t.Fatal(e)
	}
	if e = readPinnedInto(link, s, dst); e == nil {
		t.Fatal("symlink accepted")
	}
	parent := filepath.Join(root, "dirlink")
	if e = os.Symlink(root, parent); e != nil {
		t.Fatal(e)
	}
	if e = readPinnedInto(filepath.Join(parent, s.Name), s, dst); e == nil {
		t.Fatal("parent symlink accepted")
	}
}
