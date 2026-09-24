package ebiten

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hostFont2FixtureFile(t *testing.T) HostFont2LocalFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host16.golemfnt")
	return HostFont2LocalFile{Path: path, SHA256: writeHostFont3Fixture(t, path, 16, 16,
		[]rune{'設', '定', '套', '用', '取', '消', '2', '3', '×'})}
}

func TestLoadHostFont2AcceptsLockedLocalFace(t *testing.T) {
	file := hostFont2FixtureFile(t)
	font, err := LoadHostFont2(file, draftLabels())
	if err != nil {
		t.Fatal(err)
	}
	if font.W != 16 || font.H != 16 {
		t.Fatalf("2× host dimensions=%dx%d", font.W, font.H)
	}
	first := append([]byte(nil), font.Glyphs['設']...)
	file.SHA256 = writeHostFont3Fixture(t, file.Path, 16, 16,
		[]rune{'設', '定', '套', '用', '取', '消', '2', '3', '×', '新'})
	if string(font.Glyphs['設']) != string(first) {
		t.Fatal("已載入字模被路徑後續替換改動")
	}
}

func TestLoadHostFont2RejectsMissingStaleOrWrongFace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, file *HostFont2LocalFile)
		want   string
	}{
		{"empty-path", func(_ *testing.T, file *HostFont2LocalFile) { file.Path = "" }, "路徑不得為空"},
		{"missing-hash", func(_ *testing.T, file *HostFont2LocalFile) { file.SHA256 = [sha256.Size]byte{} }, "必須指定 SHA-256"},
		{"stale-hash", func(t *testing.T, file *HostFont2LocalFile) {
			writeHostFont3Fixture(t, file.Path, 16, 16, []rune{'設', '定', '套', '用', '取', '消', '2', '3', '×', '新'})
		}, "SHA-256 不符"},
		{"wrong-size", func(t *testing.T, file *HostFont2LocalFile) {
			file.SHA256 = writeHostFont3Fixture(t, file.Path, 22, 22, []rune{'設', '定', '套', '用', '取', '消', '2', '3', '×'})
		}, "16x16"},
		{"missing-label", func(t *testing.T, file *HostFont2LocalFile) {
			file.SHA256 = writeHostFont3Fixture(t, file.Path, 16, 16, []rune{'設', '定', '套', '用', '取', '消', '2', '3'})
		}, "缺少"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := hostFont2FixtureFile(t)
			tc.mutate(t, &file)
			if _, err := LoadHostFont2(file, draftLabels()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

// This local-only check is skipped in public trees that lack the user's font.
func TestLoadHostFont2LocalLockedFace(t *testing.T) {
	path, sumHex := os.Getenv("HOST_FONT2_PATH"), os.Getenv("HOST_FONT2_SHA256")
	if path == "" || sumHex == "" {
		t.Skip("local 2× host font path and SHA-256 not supplied")
	}
	sumBytes, err := hex.DecodeString(sumHex)
	if err != nil || len(sumBytes) != sha256.Size {
		t.Fatalf("invalid HOST_FONT2_SHA256: %v", err)
	}
	var sum [sha256.Size]byte
	copy(sum[:], sumBytes)
	font, err := LoadHostFont2(HostFont2LocalFile{Path: path, SHA256: sum}, draftLabels())
	if err != nil {
		t.Fatal(err)
	}
	if font.W != 16 || font.H != 16 {
		t.Fatalf("local 2× host dimensions=%dx%d", font.W, font.H)
	}
}
