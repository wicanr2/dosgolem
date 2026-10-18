package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/spec/450 §2.1：LoadFrom 與 ErrNotGzip。

func TestLoadFromMatchesLoad(t *testing.T) {
	m, d := boot(t)
	for i := 0; i < 3000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "s.gz")
	if err := Save(path, m, d); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m2, d2 := boot(t)
	if err := LoadFrom(bytes.NewReader(raw), m2, d2); err != nil {
		t.Fatal(err)
	}
	if m2.Steps != m.Steps || !bytes.Equal(m2.Mem, m.Mem) {
		t.Fatalf("LoadFrom 後步數 %d（要 %d）或記憶體不同", m2.Steps, m.Steps)
	}
}

func TestNotGzipSentinel(t *testing.T) {
	m, d := boot(t)
	err := LoadFrom(strings.NewReader("not gzip"), m, d)
	if !errors.Is(err, ErrNotGzip) {
		t.Fatalf("LoadFrom 錯誤 %v，應包著 ErrNotGzip", err)
	}
	path := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(path, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = Load(path, m, d)
	if !errors.Is(err, ErrNotGzip) || !strings.Contains(err.Error(), path) {
		t.Fatalf("Load 錯誤 %v，應包著 ErrNotGzip 並帶路徑", err)
	}
}
