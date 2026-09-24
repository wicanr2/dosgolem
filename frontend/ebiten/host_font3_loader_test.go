package ebiten

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHostFont3Fixture(t *testing.T, path string, width, height int, glyphs []rune) [sha256.Size]byte {
	t.Helper()
	rowBytes := (width + 7) / 8
	data := make([]byte, 16+len(glyphs)*(5+height*rowBytes))
	copy(data, "GOLEMFNT")
	binary.LittleEndian.PutUint16(data[8:10], uint16(width))
	binary.LittleEndian.PutUint16(data[10:12], uint16(height))
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(glyphs)))
	for index, r := range glyphs {
		offset := 16 + index*(5+height*rowBytes)
		binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(r))
		data[offset+5] = 0x80 // at least one visible ink bit
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

func hostFont3FixtureFiles(t *testing.T) HostFont3LocalFiles {
	t.Helper()
	directory := t.TempDir()
	widePath := filepath.Join(directory, "wide24.golemfnt")
	asciiPath := filepath.Join(directory, "ascii16x24.golemfnt")
	return HostFont3LocalFiles{
		WidePath: widePath, WideSHA256: writeHostFont3Fixture(t, widePath, 24, 24, []rune{'設', '定', '套', '用', '取', '消', '×'}),
		ASCIIPath: asciiPath, ASCIISHA256: writeHostFont3Fixture(t, asciiPath, 16, 24, []rune{'2', '3'}),
	}
}

func TestLoadHostFont3AcceptsLockedNativeSubsets(t *testing.T) {
	files := hostFont3FixtureFiles(t)
	font, err := LoadHostFont3(files, draftLabels())
	if err != nil {
		t.Fatal(err)
	}
	if font.Wide.W != 24 || font.Wide.H != 24 || font.ASCII.W != 16 || font.ASCII.H != 24 {
		t.Fatalf("loaded dimensions: wide=%dx%d ascii=%dx%d", font.Wide.W, font.Wide.H, font.ASCII.W, font.ASCII.H)
	}
}

func TestLoadHostFont3RejectsUntrustedOrNonNativeInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, files *HostFont3LocalFiles)
		want   string
	}{
		{"empty-wide-path", func(_ *testing.T, files *HostFont3LocalFiles) { files.WidePath = "" }, "路徑不得為空"},
		{"missing-hash", func(_ *testing.T, files *HostFont3LocalFiles) { files.ASCIISHA256 = [sha256.Size]byte{} }, "必須指定 SHA-256"},
		{"wrong-hash", func(_ *testing.T, files *HostFont3LocalFiles) { files.WideSHA256[0] ^= 0xff }, "SHA-256 不符"},
		{"substituted-file", func(t *testing.T, files *HostFont3LocalFiles) {
			writeHostFont3Fixture(t, files.WidePath, 24, 24, []rune{'設', '定', '套', '用', '取', '消', '×', '新'})
		}, "SHA-256 不符"},
		{"old-wide-22", func(t *testing.T, files *HostFont3LocalFiles) {
			files.WideSHA256 = writeHostFont3Fixture(t, files.WidePath, 22, 22, []rune{'設', '定', '套', '用', '取', '消', '×'})
		}, "24x24"},
		{"missing-required-glyph", func(t *testing.T, files *HostFont3LocalFiles) {
			files.WideSHA256 = writeHostFont3Fixture(t, files.WidePath, 24, 24, []rune{'設', '定', '套', '用', '取', '消'})
		}, "缺少"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := hostFont3FixtureFiles(t)
			tc.mutate(t, &files)
			if _, err := LoadHostFont3(files, draftLabels()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}
