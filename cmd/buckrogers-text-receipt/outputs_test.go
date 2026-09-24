package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptOutputsCommitAllFilesAndIdenticalJSON(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.rgba")
	overlay := filepath.Join(dir, "overlay.rgba")
	receipt := filepath.Join(dir, "receipt.json")
	outputs := &receiptOutputs{}
	outputs.add(baseline, []byte("baseline-new"))
	outputs.add(overlay, []byte("overlay-new"))
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, receipt, struct {
		Count int `json:"count"`
	}{14}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{baseline: "baseline-new", overlay: "overlay-new", receipt: "{\"count\":14}\n"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	if got, _ := os.ReadFile(receipt); !bytes.Equal(stdout.Bytes(), got) {
		t.Fatalf("stdout and JSON file differ: %q vs %q", stdout.Bytes(), got)
	}
	assertNoReceiptTemps(t, dir)
}

func TestReceiptOutputsStagingFailureLeavesNoNewReceipt(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.rgba")
	overlay := filepath.Join(dir, "overlay.rgba")
	receipt := filepath.Join(dir, "receipt.json")
	outputs := &receiptOutputs{}
	outputs.add(baseline, []byte("baseline-new"))
	outputs.addGenerated(overlay, func(string) error { return errors.New("injected encode failure") })
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, receipt, struct{}{}); err == nil {
		t.Fatal("staging failure must stop commit")
	}
	for _, path := range []string{baseline, overlay, receipt} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed commit left new receipt %s: %v", path, err)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("failure emitted success JSON: %q", stdout.Bytes())
	}
	assertNoReceiptTemps(t, dir)
}

func TestReceiptOutputsPublishFailureRestoresExistingPairAndJSON(t *testing.T) {
	for _, failedAt := range []int{1, 2} {
		t.Run([]string{"overlay", "JSON"}[failedAt-1], func(t *testing.T) {
			dir := t.TempDir()
			paths := []string{filepath.Join(dir, "baseline.rgba"), filepath.Join(dir, "overlay.rgba"), filepath.Join(dir, "receipt.json")}
			for i, path := range paths {
				if err := os.WriteFile(path, []byte{byte('a' + i)}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			outputs := &receiptOutputs{}
			outputs.add(paths[0], []byte("baseline-new"))
			outputs.add(paths[1], []byte("overlay-new"))
			outputs.rename = func(old, new string) error {
				if new == paths[failedAt] {
					return errors.New("injected publish failure")
				}
				return os.Rename(old, new)
			}
			var stdout bytes.Buffer
			commitErr := outputs.commitReceipt(&stdout, paths[2], struct{}{})
			if commitErr == nil {
				t.Fatal("publish failure must stop commit")
			}
			for i, path := range paths {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, []byte{byte('a' + i)}) {
					t.Fatalf("old receipt %s not restored: %q, %v", path, got, err)
				}
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed publish emitted success JSON: %q", stdout.Bytes())
			}
			for _, entry := range mustReadDir(t, dir) {
				if strings.HasPrefix(entry.Name(), ".buckrogers-receipt-") {
					t.Fatalf("temporary receipt left behind after %v: %s", commitErr, entry.Name())
				}
			}
		})
	}
}

type failingReceiptWriter struct{}

func (failingReceiptWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected stdout failure")
}

func TestReceiptOutputsStdoutFailureRestoresExistingFiles(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.rgba")
	if err := os.WriteFile(baseline, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputs := &receiptOutputs{}
	outputs.add(baseline, []byte("new"))
	receipt := filepath.Join(dir, "receipt.json")
	if err := outputs.commitReceipt(failingReceiptWriter{}, receipt, struct{}{}); err == nil {
		t.Fatal("stdout failure must stop commit")
	}
	if got, _ := os.ReadFile(baseline); string(got) != "old" {
		t.Fatalf("old baseline not restored: %q", got)
	}
	if _, err := os.Stat(receipt); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new JSON receipt survived stdout failure: %v", err)
	}
	assertNoReceiptTemps(t, dir)
}

func TestReceiptOutputsFailureAcrossOutputDirectories(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 3)
	for i, name := range []string{"baseline", "overlay", "json"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		paths[i] = filepath.Join(dir, "receipt")
		if err := os.WriteFile(paths[i], []byte{byte('a' + i)}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outputs := &receiptOutputs{}
	outputs.add(paths[0], []byte("new baseline"))
	outputs.add(paths[1], []byte("new overlay"))
	outputs.rename = func(old, new string) error {
		if new == paths[1] {
			return errors.New("injected overlay publish failure")
		}
		return os.Rename(old, new)
	}
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, paths[2], struct{}{}); err == nil {
		t.Fatal("cross-directory publish failure must stop commit")
	}
	for i, path := range paths {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, []byte{byte('a' + i)}) {
			t.Fatalf("old receipt %s not restored: %q, %v", path, got, err)
		}
		assertNoReceiptTemps(t, filepath.Dir(path))
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout preceded successful file commit: %q", stdout.Bytes())
	}
}

func TestReceiptOutputsDirectoryTargetPreservesEarlierFile(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.rgba")
	if err := os.WriteFile(baseline, []byte("old baseline"), 0o600); err != nil {
		t.Fatal(err)
	}
	overlayDirectory := filepath.Join(dir, "overlay.rgba")
	if err := os.Mkdir(overlayDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	outputs := &receiptOutputs{}
	outputs.add(baseline, []byte("new baseline"))
	outputs.add(overlayDirectory, []byte("new overlay"))
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, filepath.Join(dir, "receipt.json"), struct{}{}); err == nil {
		t.Fatal("directory output target must fail")
	}
	if got, _ := os.ReadFile(baseline); string(got) != "old baseline" {
		t.Fatalf("baseline changed before all targets validated: %q", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("invalid target emitted stdout JSON: %q", stdout.Bytes())
	}
	assertNoReceiptTemps(t, dir)
}

func TestReceiptOutputsRejectsPathAliasWithoutChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.json")
	outputs := &receiptOutputs{}
	outputs.add(path, []byte("baseline"))
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, filepath.Join(dir, ".", "receipt.json"), struct{}{}); err == nil {
		t.Fatal("duplicate output path must fail closed")
	}
	if stdout.Len() != 0 {
		t.Fatalf("duplicate path emitted JSON: %q", stdout.Bytes())
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("duplicate path created output: %v", err)
	}
	assertNoReceiptTemps(t, dir)
}

func TestReceiptOutputsRejectsSymlinkedParentAlias(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(realDir, "same.rgba")
	if err := os.WriteFile(target, []byte("old bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputs := &receiptOutputs{}
	outputs.add(target, []byte("new baseline"))
	outputs.add(filepath.Join(aliasDir, "same.rgba"), []byte("new overlay"))
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, filepath.Join(realDir, "receipt.json"), struct{}{}); err == nil {
		t.Fatal("different symlinked parent names for one target must fail")
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "old bytes" {
		t.Fatalf("aliased output changed existing bytes: %q, %v", got, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("aliased output emitted stdout JSON: %q", stdout.Bytes())
	}
	if _, err := os.Stat(filepath.Join(realDir, "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aliased output created JSON: %v", err)
	}
	assertNoReceiptTemps(t, realDir)
}

func TestReceiptOutputsRejectsSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.rgba")
	if err := os.WriteFile(realFile, []byte("old bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "output.rgba")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatal(err)
	}
	outputs := &receiptOutputs{}
	outputs.add(link, []byte("new bytes"))
	var stdout bytes.Buffer
	if err := outputs.commitReceipt(&stdout, filepath.Join(dir, "receipt.json"), struct{}{}); err == nil {
		t.Fatal("symlink target must remain rejected")
	}
	if got, err := os.ReadFile(realFile); err != nil || string(got) != "old bytes" {
		t.Fatalf("symlink target changed existing bytes: %q, %v", got, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target symlink was replaced: %v, %v", info, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("symlink target emitted stdout JSON: %q", stdout.Bytes())
	}
	assertNoReceiptTemps(t, dir)
}

func assertNoReceiptTemps(t *testing.T, dir string) {
	t.Helper()
	entries := mustReadDir(t, dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".buckrogers-receipt-") {
			t.Fatalf("temporary receipt left behind: %s", entry.Name())
		}
	}
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
