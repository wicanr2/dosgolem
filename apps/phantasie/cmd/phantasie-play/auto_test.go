package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureDoesNotExposeRejectedRouteInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.route")
	marker := "PRIVATE_TEST_MARKER"
	if err := os.WriteFile(path, []byte(marker+" Return\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := recordGameplay(nil, "original", nil, captureOptions{Route: path})
	if err == nil || strings.Contains(err.Error(), marker) || !strings.Contains(err.Error(), "路線解析") {
		t.Fatalf("unsafe or missing diagnostic: %v", err)
	}
}

func TestCaptureRejectsExistingLinkedOrOverlappingOutputs(t *testing.T) {
	base := t.TempDir()
	input := filepath.Join(base, "input")
	if err := os.Mkdir(input, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(input, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{input, filepath.Join(input, "child"), base, filepath.Join(base, "missing", "child"), filepath.Join(base, "link", "child")} {
		if err := captureNewDir(output, input); err == nil {
			t.Fatalf("accepted %s", output)
		}
	}
	output := filepath.Join(base, "new")
	if err := captureNewDir(output, input); err != nil {
		t.Fatal(err)
	}
	if err := captureNewDir(output, input); err == nil {
		t.Fatal("accepted existing output")
	}
}

func TestCaptureRequiresActualBundleFilesAndPaths(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"text", "font", "original"} {
		if err := os.Mkdir(filepath.Join(base, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	names := []string{"backend", "text/protected.tsv"}
	for _, lang := range []string{"zh-TW", "zh-CN", "ja", "ko"} {
		names = append(names, "font/"+lang+".golemfnt")
		for _, family := range []string{"ui", "prose", "manual"} {
			names = append(names, "text/"+family+"."+lang+".tsv")
		}
	}
	var assets []map[string]any
	for _, name := range names {
		data := []byte(name)
		if err := os.WriteFile(filepath.Join(base, name), data, 0644); err != nil {
			t.Fatal(err)
		}
		assets = append(assets, map[string]any{"name": name, "bytes": len(data), "sha256": captureHash(data)})
	}
	b := map[string]any{"schema": 1, "version": "v.1.0.0-20261005", "engine_commit": strings.Repeat("a", 40), "backend": "backend", "text": "text", "font": "font", "local_original": "original", "assets": assets}
	path := filepath.Join(base, "bundle.json")
	write := func() {
		data, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	o := captureOptions{Root: filepath.Join(base, "original"), Text: filepath.Join(base, "text"), Font: filepath.Join(base, "font")}
	inputs := map[string]string{"backend": captureHash([]byte("backend"))}
	write()
	if _, err := captureBundle(path, o, inputs); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		key string
		bad any
	}{{"version", "v.1.0.0-20260230"}, {"engine_commit", strings.Repeat("z", 40)}, {"backend", ".."}, {"text", "../text"}, {"local_original", ".."}} {
		old := b[test.key]
		b[test.key] = test.bad
		write()
		if _, err := captureBundle(path, o, inputs); err == nil {
			t.Fatalf("accepted invalid %s", test.key)
		}
		b[test.key] = old
	}
	write()
	wrong := o
	wrong.Root = base
	if _, err := captureBundle(path, wrong, inputs); err == nil {
		t.Fatal("accepted original data outside bundle")
	}
	if _, err := captureBundle(path, o, map[string]string{"backend": strings.Repeat("0", 64)}); err == nil {
		t.Fatal("accepted a different executing backend")
	}
	if err := os.WriteFile(filepath.Join(base, "text/protected.tsv"), []byte("corrupted payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureBundle(path, o, inputs); err == nil {
		t.Fatal("accepted modified bundle")
	}
}
