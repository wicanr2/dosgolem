package dos

import (
	"bytes"
	"encoding/gob"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func split463(t *testing.T) (*DOS, *machine.Machine) {
	t.Helper()
	m := machine.New()
	d := New(m, t.TempDir())
	d.SetOriginalSaveRoots(d.Root, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	t.Cleanup(d.Close)
	return d, m
}
func rename463(m *machine.Machine, d *DOS, a, b string) {
	putName(m, m.CPU, a)
	m.CPU.Seg[cpu.ES] = 0x1000
	m.CPU.R[cpu.DI] = 0x300
	m.WriteBytes(cpu.Addr(0x1000, 0x300), append([]byte(b), 0))
	int21(m, d, 0x5600)
}
func expectDenied463(t *testing.T, m *machine.Machine) {
	t.Helper()
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 5 {
		t.Fatalf("want access denied, AX=%d CF=%v", m.CPU.R[cpu.AX], m.CPU.Flags&cpu.CF)
	}
}
func Test463COWAndUnion(t *testing.T) {
	d, m := split463(t)
	writeFile(t, d.Root, "DATA", "root")
	writeFile(t, d.Root, "ONLY", "only")
	writeFile(t, d.Scratch, "data", "save")
	if got := readHandle(t, m, d, openFile(t, m, d, "DATA", 0), 4); got != "save" {
		t.Fatal(got)
	}
	h := openFile(t, m, d, "ONLY", 2)
	m.WriteBytes(cpu.Addr(0x1000, 0x200), []byte("new!"))
	m.CPU.Seg[cpu.DS] = 0x1000
	m.CPU.R[cpu.DX] = 0x200
	m.CPU.R[cpu.BX] = h
	m.CPU.R[cpu.CX] = 4
	int21(m, d, 0x4000)
	b, _ := os.ReadFile(filepath.Join(d.Root, "ONLY"))
	if string(b) != "only" {
		t.Fatal("original written")
	}
	b, _ = os.ReadFile(filepath.Join(d.Scratch, "ONLY"))
	if string(b) != "new!" {
		t.Fatal(string(b))
	}
	st := d.searchFor("*.*", 0)
	if strings.Join(st.names, ",") != "DATA,ONLY" {
		t.Fatal(st.names)
	}
	putName(m, m.CPU, "DATA")
	int21(m, d, 0x4100)
	if got := readHandle(t, m, d, openFile(t, m, d, "DATA", 0), 4); got != "root" {
		t.Fatal("no-whiteout changed")
	}
	putName(m, m.CPU, "NEW")
	int21(m, d, 0x3C00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("create failed")
	}
	rename463(m, d, "NEW", "RENAMED")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("save rename failed")
	}
	putName(m, m.CPU, "SUB")
	int21(m, d, 0x3900)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("mkdir failed")
	}
	int21(m, d, 0x3A00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("rmdir failed")
	}
}
func Test463SiblingPrefixRename(t *testing.T) {
	parent := t.TempDir()
	orig, save := filepath.Join(parent, "save-original"), filepath.Join(parent, "save")
	os.Mkdir(orig, 0700)
	os.Mkdir(save, 0700)
	m := machine.New()
	d := New(m, orig)
	d.SetOriginalSaveRoots(orig, save)
	d.Install()
	defer d.Close()
	writeFile(t, orig, "A", "root")
	rename463(m, d, "A", "B")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("rename failed")
	}
	b, e := os.ReadFile(filepath.Join(orig, "A"))
	if e != nil || string(b) != "root" {
		t.Fatal("original moved", e)
	}
	b, e = os.ReadFile(filepath.Join(save, "B"))
	if e != nil || string(b) != "root" {
		t.Fatal("copy missing", e)
	}
}
func Test463UnsafeMutation(t *testing.T) {
	for _, kind := range []string{"leaf-symlink", "parent-symlink", "hardlink"} {
		for _, op := range []string{"open", "create", "unlink", "rename-source", "rename-dest", "mkdir", "rmdir"} {
			if kind == "hardlink" && (op == "mkdir" || op == "rmdir") {
				continue
			}
			t.Run(kind+"/"+op, func(t *testing.T) {
				d, m := split463(t)
				writeFile(t, d.Root, "A", "original")
				writeFile(t, d.Scratch, "GOOD", "good")
				name := "BAD"
				switch kind {
				case "leaf-symlink":
					os.Symlink(filepath.Join(d.Root, "A"), filepath.Join(d.Scratch, name))
				case "hardlink":
					os.Link(filepath.Join(d.Root, "A"), filepath.Join(d.Scratch, name))
				case "parent-symlink":
					os.Symlink(d.Root, filepath.Join(d.Scratch, "SUB"))
					name = `SUB\A`
				}
				putName(m, m.CPU, name)
				switch op {
				case "open":
					int21(m, d, 0x3D02)
				case "create":
					int21(m, d, 0x3C00)
				case "unlink":
					int21(m, d, 0x4100)
				case "rename-source":
					rename463(m, d, name, "OUT")
				case "rename-dest":
					rename463(m, d, "GOOD", name)
				case "mkdir":
					int21(m, d, 0x3900)
				case "rmdir":
					int21(m, d, 0x3A00)
				}
				expectDenied463(t, m)
				b, e := os.ReadFile(filepath.Join(d.Root, "A"))
				if e != nil || string(b) != "original" {
					t.Fatal("original polluted", e)
				}
			})
		}
	}
}
func Test463ReadRejectsSymlinks(t *testing.T) {
	d, m := split463(t)
	outside := t.TempDir()
	writeFile(t, outside, "A", "outside")
	os.Symlink(filepath.Join(outside, "A"), filepath.Join(d.Root, "BAD"))
	putName(m, m.CPU, "BAD")
	int21(m, d, 0x3D00)
	expectDenied463(t, m)
}
func editState463(t *testing.T, raw []byte, fn func(*dosState)) []byte {
	t.Helper()
	var s dosState
	if e := gob.NewDecoder(bytes.NewReader(raw)).Decode(&s); e != nil {
		t.Fatal(e)
	}
	fn(&s)
	var b bytes.Buffer
	if e := gob.NewEncoder(&b).Encode(s); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func Test463StateSplit(t *testing.T) {
	f := newRebaseFixture(t)
	d, m := split463(t)
	writeFile(t, d.Root, "X", rebaseX)
	writeFile(t, d.Root, "Z", rebaseZ)
	writeFile(t, d.Scratch, "x", rebaseCX)
	d.RebaseTo, d.RebaseOriginal, d.RebaseFrom = d.Scratch, d.Root, []string{f.b}
	if e := d.LoadState(bytes.NewReader(f.state)); e != nil {
		t.Fatal(e)
	}
	if d.RebaseTo != "" || d.RebaseOriginal != "" || d.RebaseFrom != nil {
		t.Fatal("transients retained")
	}
	if got := readHandle(t, m, d, f.hx, 32); got != rebaseCX[8:] {
		t.Fatal(got)
	}
	if got := readHandle(t, m, d, f.hz, 32); got != rebaseZ[6:] {
		t.Fatal(got)
	}
	if d.handles[f.hx].writable || d.handles[f.hz].writable {
		t.Fatal("restored writable")
	}
	m.CPU.R[cpu.BX] = f.hx
	m.CPU.R[cpu.CX] = 1
	m.CPU.Seg[cpu.DS] = 0x1000
	m.CPU.R[cpu.DX] = 0x200
	m.WriteBytes(cpu.Addr(0x1000, 0x200), []byte("!"))
	int21(m, d, 0x4000)
	b, _ := os.ReadFile(filepath.Join(d.Scratch, "x"))
	if string(b) != rebaseCX {
		t.Fatal("readonly restore changed")
	}
}
func Test463StateRejectKeepsDOS(t *testing.T) {
	f := newRebaseFixture(t)
	for _, kind := range []string{"missing-prefix", "size", "leaf-symlink", "exact-unsafe-fold-safe", "ambiguous-file", "middle-file", "middle-symlink", "middle-ambiguous", "open-fails", "seek-fails", "directory-leaf", "missing"} {
		t.Run(kind, func(t *testing.T) {
			d, m := split463(t)
			writeFile(t, d.Root, "X", rebaseX)
			writeFile(t, d.Root, "Z", rebaseZ)
			writeFile(t, d.Root, "Q", "quiet-old")
			hq := openFile(t, m, d, "Q", 0)
			readHandle(t, m, d, hq, 2)
			from := []string{f.b}
			raw := f.state
			switch kind {
			case "missing-prefix":
				from = nil
			case "size":
				writeFile(t, d.Scratch, "X", "badsize")
			case "leaf-symlink":
				os.Symlink(filepath.Join(d.Root, "X"), filepath.Join(d.Scratch, "X"))
			case "exact-unsafe-fold-safe":
				os.Symlink(filepath.Join(d.Root, "X"), filepath.Join(d.Scratch, "X"))
				writeFile(t, d.Scratch, "x", rebaseX)
			case "ambiguous-file":
				writeFile(t, d.Scratch, "x", rebaseX)
				writeFile(t, d.Scratch, "X", rebaseX)
				raw = editState463(t, raw, func(s *dosState) { s.Handles[0].Path = filepath.Join(f.a, "xX") })
				os.WriteFile(filepath.Join(d.Scratch, "xx"), []byte(rebaseX), 0600)
				os.WriteFile(filepath.Join(d.Scratch, "XX"), []byte(rebaseX), 0600)
			case "middle-file", "middle-symlink", "middle-ambiguous":
				raw = editState463(t, raw, func(s *dosState) { s.Handles[0].Path = filepath.Join(f.a, "dIr", "X") })
				os.Mkdir(filepath.Join(d.Root, "dIr"), 0700)
				writeFile(t, filepath.Join(d.Root, "dIr"), "X", rebaseX)
				if kind == "middle-file" {
					writeFile(t, d.Scratch, "dIr", "file")
				} else if kind == "middle-symlink" {
					os.Symlink(filepath.Join(d.Root, "dIr"), filepath.Join(d.Scratch, "dIr"))
				} else {
					os.Mkdir(filepath.Join(d.Scratch, "Dir"), 0700)
					os.Mkdir(filepath.Join(d.Scratch, "DIR"), 0700)
				}
			case "open-fails":
				writeFile(t, d.Scratch, "Z", rebaseZ)
				os.Chmod(filepath.Join(d.Scratch, "Z"), 0000)
				defer os.Chmod(filepath.Join(d.Scratch, "Z"), 0600)
			case "seek-fails":
				raw = editState463(t, raw, func(s *dosState) { s.Handles[len(s.Handles)-1].Off = -1 })
			case "directory-leaf":
				os.Mkdir(filepath.Join(d.Scratch, "Z"), 0700)
				raw = editState463(t, raw, func(s *dosState) { s.Handles[len(s.Handles)-1].Size = 0 })
			case "missing":
				os.Remove(filepath.Join(d.Root, "Z"))
			}
			d.Dir = "KEPT-OLD"
			before := dosDigest(t, d)
			d.RebaseTo, d.RebaseOriginal, d.RebaseFrom = d.Scratch, d.Root, from
			if e := d.LoadState(bytes.NewReader(raw)); e == nil {
				t.Fatal("unsafe state accepted")
			}
			if after := dosDigest(t, d); after != before {
				t.Fatal("failed preflight changed DOS")
			}
			if d.RebaseTo != "" || d.RebaseOriginal != "" || d.RebaseFrom != nil {
				t.Fatal("failed transients retained")
			}
			if got := readHandle(t, m, d, hq, 32); got != "iet-old" {
				t.Fatal("old handle lost", got)
			}
		})
	}
}

func Test463StampRejectsOriginal(t *testing.T) {
	parent := t.TempDir()
	a, b := filepath.Join(parent, "save-original"), filepath.Join(parent, "save")
	os.Mkdir(a, 0700)
	os.Mkdir(b, 0700)
	writeFile(t, a, "A", "orig")
	m := machine.New()
	d := New(m, a)
	d.SetOriginalSaveRoots(a, b)
	p := filepath.Join(a, "A")
	before, _ := os.Stat(p)
	d.stampScratch(p)
	after, _ := os.Stat(p)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("original mtime changed")
	}
}
func Test463StateLongestPrefix(t *testing.T) {
	f := newRebaseFixture(t)
	raw := editState463(t, f.state, func(s *dosState) { s.Root = "/old"; s.Handles[0].Path = "/old/nested/X"; s.Handles[1].Path = "/old/Z" })
	d, m := split463(t)
	writeFile(t, d.Root, "X", rebaseX)
	writeFile(t, d.Root, "Z", rebaseZ)
	d.RebaseTo, d.RebaseOriginal, d.RebaseFrom = d.Scratch, d.Root, []string{"/old/nested"}
	if e := d.LoadState(bytes.NewReader(raw)); e != nil {
		t.Fatal(e)
	}
	if got := readHandle(t, m, d, f.hx, 32); got != rebaseX[8:] {
		t.Fatal(got)
	}
}
