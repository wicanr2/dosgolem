package machine

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryReadOnlyFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "SUB"), 0o700); err != nil {
		t.Fatal(err)
	}
	escapeRoot := t.TempDir()
	escapePath := filepath.Join(escapeRoot, "SECRET")
	if err := os.WriteFile(escapePath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(escapePath, filepath.Join(root, "ESCAPE")); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	file, err := provider.OpenRead("mdi.ini")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil || string(data) != "driver\n" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	for _, name := range []string{"", ".", "..", "../MDI.INI", "/MDI.INI", `C:\\MDI.INI`, "SUB/MDI.INI", "SUB", "ESCAPE"} {
		if file, err := provider.OpenRead(name); err == nil {
			file.Close()
			t.Fatalf("unsafe path %q was accepted", name)
		}
	}
}
