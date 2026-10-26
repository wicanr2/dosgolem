package fsaccess

import (
	"os"
	"path/filepath"
	"testing"
)

func Test463RootPair(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	file := filepath.Join(b, "file")
	os.WriteFile(file, []byte("x"), 0600)
	link := filepath.Join(b, "link")
	os.Symlink(a, link)
	nested := filepath.Join(a, "nested")
	os.Mkdir(nested, 0700)
	parentAlias := filepath.Join(b, "parentAlias")
	os.Symlink(filepath.Dir(a), parentAlias)
	for _, tc := range []struct {
		name, a, b string
		ok         bool
	}{
		{"valid", a, b, true}, {"sibling-prefix", a, a + "-save", true}, {"same", a, a, false}, {"nested", a, nested, false}, {"reverse", nested, a, false}, {"file-original", file, a, false}, {"file-save", a, file, false}, {"symlink-original", link, b, false}, {"symlink-save", a, link, false},
		{"symlink-trailing-slash", link + string(filepath.Separator), b, false},
		{"ancestor-alias-nested", a, filepath.Join(parentAlias, filepath.Base(a), "nested"), false}, {"alias", a, filepath.Join(parentAlias, filepath.Base(a)), false}, {"nul", a + "\x00", b, false}, {"empty-save", a, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "sibling-prefix" {
				os.Mkdir(tc.b, 0700)
				defer os.Remove(tc.b)
			}
			before, _ := os.Stat(a)
			x, y, e := RootPair(tc.a, tc.b)
			after, _ := os.Stat(a)
			if (e == nil) != tc.ok {
				t.Fatalf("roots=%q,%q err=%v", x, y, e)
			}
			if !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("validation wrote original")
			}
			if tc.ok && (!filepath.IsAbs(x) || !filepath.IsAbs(y)) {
				t.Fatal("not physical absolute")
			}
		})
	}
	os.Chmod(a, 0555)
	defer os.Chmod(a, 0755)
	if _, _, e := RootPair(a, b); e != nil {
		t.Fatal("original writable not required", e)
	}
}
func Test463ResolveAndMutation(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "Dir"), 0700)
	os.WriteFile(filepath.Join(root, "Dir", "Data"), []byte("x"), 0600)
	p, e := Resolve(root, filepath.Join("dir", "data"), false)
	if e != nil || p != filepath.Join(root, "Dir", "Data") {
		t.Fatal(p, e)
	}
	os.WriteFile(filepath.Join(root, "Dir", "DATA"), []byte("y"), 0600)
	if _, e = Resolve(root, filepath.Join("Dir", "dAtA"), false); e == nil {
		t.Fatal("ambiguous accepted")
	}
	if p, e = Resolve(root, filepath.Join("Dir", "Data"), false); e != nil {
		t.Fatal("exact spelling rejected", e)
	}
	for _, name := range []string{"link", "hard", "directory"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(root, name)
			switch name {
			case "link":
				os.Symlink(filepath.Join(root, "Dir", "Data"), p)
			case "hard":
				if e := os.Link(filepath.Join(root, "Dir", "Data"), p); e != nil {
					t.Fatal(e)
				}
			case "directory":
				os.Mkdir(p, 0700)
			}
			if e := Mutation(root, p, false); e == nil {
				t.Fatal("unsafe mutation accepted")
			}
		})
	}
	if _, e = Resolve(root, "../outside", true); e == nil {
		t.Fatal("escaped root")
	}
	if e = Mutation(root, root, true); e == nil {
		t.Fatal("root mutation accepted")
	}
}
