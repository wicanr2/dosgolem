package ebiten

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/xlate"
)

// HostFont3LocalFiles identifies the two local-only native subsets needed by
// the 3× host chrome. Paths are supplied by the launcher; no font path or
// bitmap is embedded in the program. Both hashes are required so a missing,
// substituted, or stale subset fails before Game.New.
type HostFont3LocalFiles struct {
	WidePath    string
	WideSHA256  [sha256.Size]byte
	ASCIIPath   string
	ASCIISHA256 [sha256.Size]byte
}

// LoadHostFont3 loads only the user-selected native 24-point subsets. It does
// not inspect or accept a 22-point fallback: Wide must be 24×24 and ASCII
// must be 16×24. labels are validated here as well as by Game.New, so a
// launcher fails before creating a frontend when a required glyph is absent.
func LoadHostFont3(files HostFont3LocalFiles, labels HostLabels) (*HostFont3, error) {
	wide, err := loadLockedHostFont3File("Wide", files.WidePath, files.WideSHA256)
	if err != nil {
		return nil, err
	}
	ascii, err := loadLockedHostFont3File("ASCII", files.ASCIIPath, files.ASCIISHA256)
	if err != nil {
		return nil, err
	}
	font := &HostFont3{Wide: wide, ASCII: ascii}
	if err := validateHostFont3(font, labels); err != nil {
		return nil, err
	}
	return font, nil
}

func loadLockedHostFont3File(role, path string, want [sha256.Size]byte) (*xlate.Font, error) {
	if path == "" {
		return nil, fmt.Errorf("frontend/ebiten: 3× host %s 字型路徑不得為空", role)
	}
	if want == ([sha256.Size]byte{}) {
		return nil, fmt.Errorf("frontend/ebiten: 3× host %s 字型必須指定 SHA-256", role)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("frontend/ebiten: 無法讀取 3× host %s 字型：%w", role, err)
	}
	if sha256.Sum256(before) != want {
		return nil, fmt.Errorf("frontend/ebiten: 3× host %s 字型 SHA-256 不符", role)
	}
	font, err := xlate.LoadFont(path)
	if err != nil {
		return nil, fmt.Errorf("frontend/ebiten: 無法載入 3× host %s 字型：%w", role, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("frontend/ebiten: 載入後無法讀取 3× host %s 字型：%w", role, err)
	}
	if !bytes.Equal(before, after) || sha256.Sum256(after) != want {
		return nil, fmt.Errorf("frontend/ebiten: 載入期間 3× host %s 字型已變更", role)
	}
	return font, nil
}
