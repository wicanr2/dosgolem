package main

import (
	"os"
	"path/filepath"
	"testing"
)

func putState(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func stateHash(t *testing.T, dir string) string {
	t.Helper()
	s, err := snapshotState(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	return s.Hash
}
func TestStateDigest(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	const empty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if stateHash(t, a) != empty {
		t.Fatal("空目錄摘要錯誤")
	}
	if got, err := snapshotState("", "test"); err != nil || got.Hash != "-" {
		t.Fatalf("未啟用：%v %v", got, err)
	}
	putState(t, a, "a.bin", []byte{0, 1, 2})
	putState(t, b, "a.bin", []byte{0, 1, 2})
	first := stateHash(t, a)
	if first != stateHash(t, b) {
		t.Fatal("同內容的不同路徑應相同")
	}
	putState(t, b, "a.bin", []byte{0, 1, 3})
	if first == stateHash(t, b) {
		t.Fatal("修改一位元組未改摘要")
	}
	if err := os.Rename(filepath.Join(a, "a.bin"), filepath.Join(a, "b.bin")); err != nil {
		t.Fatal(err)
	}
	if first == stateHash(t, a) {
		t.Fatal("不同檔名未改摘要")
	}
}
func TestStateRejectsAliasesAndInvalidNames(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory", "control", "backslash", "invalid-utf8"} {
		t.Run(kind, func(t *testing.T) {
			d := t.TempDir()
			outside := filepath.Join(t.TempDir(), "source.bin")
			if err := os.WriteFile(outside, []byte("pristine"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(outside, filepath.Join(d, "alias.bin"))
			case "hardlink":
				err = os.Link(outside, filepath.Join(d, "alias.bin"))
			case "directory":
				err = os.Mkdir(filepath.Join(d, "child"), 0700)
			case "control":
				err = os.WriteFile(filepath.Join(d, "a\tb"), nil, 0600)
			case "backslash":
				err = os.WriteFile(filepath.Join(d, "a\\b"), nil, 0600)
			case "invalid-utf8":
				err = os.WriteFile(filepath.Join(d, string([]byte{'a', 0xff})), nil, 0600)
			}
			if err != nil {
				t.Skipf("平台不支援測試輸入：%v", err)
			}
			if _, err := snapshotState(d, "bad"); err == nil {
				t.Fatal("未拒絕不安全的輸入")
			}
			b, err := os.ReadFile(outside)
			if err != nil || string(b) != "pristine" {
				t.Fatal("改寫外部檔案")
			}
		})
	}
}
func TestPrepareStatesIsolation(t *testing.T) {
	d := t.TempDir()
	root := filepath.Join(d, "original")
	out := filepath.Join(d, "receipts")
	base := filepath.Join(d, "state")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, r, o, b string
		langs         []string
	}{
		{"same", root, out, root, []string{"zh-TW"}},
		{"inside", root, out, filepath.Join(root, "new"), []string{"zh-TW"}},
		{"ancestor", root, out, d, []string{"zh-TW"}},
		{"out-original", root, filepath.Join(root, "new"), base, []string{"zh-TW"}},
		{"out-scratch", root, filepath.Join(base, "zh-TW", "new"), base, []string{"zh-TW"}},
		{"duplicate", root, out, base, []string{"zh-TW", "zh-TW"}},
		{"traversal", root, out, base, []string{"../escape"}},
		{"no-state-duplicate", root, out, "", []string{"zh-TW", "zh-TW"}},
		{"no-state-invalid", root, out, "", []string{"../escape"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := prepareStates(c.r, c.o, c.b, c.langs); err == nil {
				t.Fatal("預檢未拒絕")
			}
		})
	}
	alias := filepath.Join(d, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareStates(root, out, filepath.Join(alias, "new"), []string{"zh-TW"}); err == nil {
		t.Fatal("未解析祖先的符號連結")
	}
	states, err := prepareStates(root, out, filepath.Join(out, "state"), []string{"zh-TW", "ja"})
	if err != nil {
		t.Fatal(err)
	}
	if states["zh-TW"] == states["ja"] {
		t.Fatal("語言未隔離")
	}
	putState(t, states["ja"], "bad\tname", nil)
	if _, err := prepareStates(root, out, filepath.Join(out, "state"), []string{"zh-TW", "ja"}); err == nil {
		t.Fatal("未預檢第二個語言")
	}
}
