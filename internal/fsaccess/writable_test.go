package fsaccess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritable(t *testing.T) {
	dir := t.TempDir()
	if err := Writable(dir); err != nil {
		t.Fatal(err)
	}
	if err := Writable(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("不存在的目錄不應可寫")
	}
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() != 0 {
		if err := Writable(ro); err == nil {
			t.Fatal("唯讀目錄不應可寫")
		}
	}
}
