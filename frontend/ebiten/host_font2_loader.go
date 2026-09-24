package ebiten

import (
	"crypto/sha256"

	"github.com/wicanr2/dosgolem/xlate"
)

// HostFont2LocalFile identifies the local-only 16×16 face used by the
// default 2× host chrome. The launcher supplies both the path and its
// expected digest; the launcher must independently establish its provenance.
// No private font bytes are bundled here.
type HostFont2LocalFile struct {
	Path   string
	SHA256 [sha256.Size]byte
}

// LoadHostFont2 binds the digest to the exact bytes parsed by xlate, then
// checks every host label before any Game or window is created. It does not
// change the existing 2× cell width, glyphs, or drawing path.
func LoadHostFont2(file HostFont2LocalFile, labels HostLabels) (*xlate.Font, error) {
	font, err := loadLockedHostFontFile("2×", "host", file.Path, file.SHA256)
	if err != nil {
		return nil, err
	}
	if err := validateHostFont2(font, labels); err != nil {
		return nil, err
	}
	return font, nil
}
