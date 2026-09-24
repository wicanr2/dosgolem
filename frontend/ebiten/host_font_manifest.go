package ebiten

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// Eten15SourceIdentity is the independently reviewed identity of the local
// 15-point inputs used to rebuild the 2× subset.  It deliberately contains no
// filesystem path or font bytes.
type Eten15SourceIdentity struct {
	ASCII [sha256.Size]byte
	SPC   [sha256.Size]byte
	STD   [sha256.Size]byte
}

// Eten24SourceIdentity is the independently reviewed identity of the local
// 24-point inputs used to rebuild the 3× native subsets.
type Eten24SourceIdentity struct {
	ASCII    [sha256.Size]byte
	SPC      [sha256.Size]byte
	STD      [sha256.Size]byte
	ETUNPACK [sha256.Size]byte
}

// HostFontManifestReview is launcher-owned review data.  A manifest is only
// metadata produced beside local artifacts; it is not trusted as its own
// provenance assertion.  Both identities must be supplied before any font is
// read, Game is built, or window is created.
type HostFontManifestReview struct {
	Font2 Eten15SourceIdentity
	Font3 Eten24SourceIdentity
}

// HostFontManifestPreflight identifies local-only, ignored artifacts.  Paths
// are intentionally launcher supplied so this package never hardcodes a
// private location.
type HostFontManifestPreflight struct {
	CatalogDir        string
	Font2ManifestPath string
	Font2Path         string
	Font3ManifestPath string
	Font3WidePath     string
	Font3ASCIIPath    string
	Labels            HostLabels
	Review            HostFontManifestReview
}

// HostFonts holds the validated host faces ready for Config.  Call this at the
// Linux launch boundary, before creating a DOS machine or Ebitengine window.
type HostFonts struct {
	Font2 *xlate.Font
	Font3 *HostFont3
}

type hostFont2Manifest struct {
	Catalogs            []manifestCatalog `json:"catalogs"`
	CharacterListSHA256 string            `json:"character_list_sha256"`
	DistributionStatus  string            `json:"distribution_status"`
	Format              struct {
		Glyphs int    `json:"glyphs"`
		Magic  string `json:"magic"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"format"`
	OutputSHA256 string `json:"output_sha256"`
	Sources      struct {
		ASCII manifestDigest `json:"asc"`
		SPC   manifestDigest `json:"spc"`
		STD   manifestDigest `json:"std"`
	} `json:"sources"`
	TopPad struct {
		ASCIIX     []int `json:"ascii_x"`
		OutputRows []int `json:"output_rows"`
		SourceRows []int `json:"source_rows"`
	} `json:"top_pad"`
}

type hostFont3Manifest struct {
	Catalog      manifestCatalog `json:"catalog"`
	Codepoints   []string        `json:"codepoints"`
	Status       string          `json:"status"`
	SourceSHA256 struct {
		ASCII    string `json:"ascii"`
		SPC      string `json:"spc"`
		STD      string `json:"std"`
		ETUNPACK string `json:"etunpack"`
	} `json:"source_sha256"`
	Outputs map[string]hostFont3Output `json:"outputs"`
}

type manifestDigest struct {
	Bytes    int    `json:"bytes"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

type manifestCatalog struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

type hostFont3Output struct {
	GlyphCount int    `json:"glyph_count"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SHA256     string `json:"sha256"`
}

// LoadHostFontsFromReviewedManifests verifies local rebuild metadata against
// a separate review, then gives the manifest-pinned output bytes to the
// existing 2×/3× loaders.  It is fail-closed: an absent, malformed, stale, or
// unreviewed manifest never establishes a font source identity.
func LoadHostFontsFromReviewedManifests(in HostFontManifestPreflight) (*HostFonts, error) {
	font2Manifest, err := readHostFont2Manifest(in.Font2ManifestPath)
	if err != nil {
		return nil, err
	}
	font3Manifest, err := readHostFont3Manifest(in.Font3ManifestPath)
	if err != nil {
		return nil, err
	}
	font2Hash, err := verifyHostFont2Manifest(font2Manifest, in.Review.Font2)
	if err != nil {
		return nil, err
	}
	wideHash, asciiHash, err := verifyHostFont3Manifest(font3Manifest, in.Review.Font3)
	if err != nil {
		return nil, err
	}
	if err := verifyCurrentCatalogs(in.CatalogDir, font2Manifest.Catalogs, font3Manifest.Catalog); err != nil {
		return nil, err
	}
	font2, err := LoadHostFont2(HostFont2LocalFile{Path: in.Font2Path, SHA256: font2Hash}, in.Labels)
	if err != nil {
		return nil, err
	}
	font3, err := LoadHostFont3(HostFont3LocalFiles{
		WidePath: in.Font3WidePath, WideSHA256: wideHash,
		ASCIIPath: in.Font3ASCIIPath, ASCIISHA256: asciiHash,
	}, in.Labels)
	if err != nil {
		return nil, err
	}
	return &HostFonts{Font2: font2, Font3: font3}, nil
}

// verifyCurrentCatalogs binds both locally built fonts to the launcher's
// current translation files. A manifest's source/output digests alone do not
// prove that its glyph subset was rebuilt after the catalog changed.
func verifyCurrentCatalogs(dir string, font2 []manifestCatalog, font3 manifestCatalog) error {
	if dir == "" {
		return fmt.Errorf("frontend/ebiten: 缺少目前譯文目錄")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("frontend/ebiten: 無法讀取目前譯文目錄：%w", err)
	}
	current := make(map[string][sha256.Size]byte)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".zh-TW.tsv") {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("frontend/ebiten: 譯文 %q 必須是一般檔案", name)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("frontend/ebiten: 無法讀取譯文 %q：%w", name, err)
		}
		current[name] = sha256.Sum256(data)
	}
	if len(font2) == 0 || len(current) != len(font2) {
		return fmt.Errorf("frontend/ebiten: 2× 字型 manifest 的譯文清單與目前譯文不符")
	}
	seen := make(map[string]bool)
	for _, catalog := range font2 {
		if filepath.Base(catalog.Filename) != catalog.Filename || !strings.HasSuffix(catalog.Filename, ".zh-TW.tsv") || seen[catalog.Filename] {
			return fmt.Errorf("frontend/ebiten: 2× 字型 manifest 譯文名稱無效或重複：%q", catalog.Filename)
		}
		seen[catalog.Filename] = true
		if err := matchCatalogDigest(catalog, current); err != nil {
			return err
		}
	}
	if font3.Filename != "host-ui.zh-TW.tsv" {
		return fmt.Errorf("frontend/ebiten: 3× 字型 manifest 必須指向 host-ui.zh-TW.tsv")
	}
	return matchCatalogDigest(font3, current)
}

func matchCatalogDigest(catalog manifestCatalog, current map[string][sha256.Size]byte) error {
	want, ok := current[catalog.Filename]
	if !ok {
		return fmt.Errorf("frontend/ebiten: 字型 manifest 的譯文 %q 已不存在", catalog.Filename)
	}
	got, err := parseManifestHash("譯文 "+catalog.Filename, catalog.SHA256)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("frontend/ebiten: 字型 manifest 的譯文 %q SHA-256 已過期", catalog.Filename)
	}
	return nil
}

func readHostFont2Manifest(path string) (hostFont2Manifest, error) {
	var manifest hostFont2Manifest
	if err := readJSON(path, &manifest); err != nil {
		return hostFont2Manifest{}, fmt.Errorf("frontend/ebiten: 無法讀取 2× host 字型 manifest：%w", err)
	}
	return manifest, nil
}

func readHostFont3Manifest(path string) (hostFont3Manifest, error) {
	var manifest hostFont3Manifest
	if err := readJSON(path, &manifest); err != nil {
		return hostFont3Manifest{}, fmt.Errorf("frontend/ebiten: 無法讀取 3× host 字型 manifest：%w", err)
	}
	return manifest, nil
}

func readJSON(path string, dst any) error {
	if path == "" {
		return fmt.Errorf("manifest 路徑不得為空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("manifest 含有多個 JSON 值")
		}
		return err
	}
	return nil
}

func verifyHostFont2Manifest(manifest hostFont2Manifest, review Eten15SourceIdentity) ([sha256.Size]byte, error) {
	if manifest.DistributionStatus != "local-only-not-for-distribution" {
		return [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: 2× host 字型 manifest 未標示為僅限本機")
	}
	if manifest.Format.Magic != "GOLEMFNT" || manifest.Format.Width != 16 || manifest.Format.Height != 16 {
		return [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: 2× host 字型 manifest 格式必須是 GOLEMFNT 16x16")
	}
	if err := matchManifestDigest("2× ASCII 來源", manifest.Sources.ASCII.SHA256, review.ASCII); err != nil {
		return [sha256.Size]byte{}, err
	}
	if err := matchManifestDigest("2× SPC 來源", manifest.Sources.SPC.SHA256, review.SPC); err != nil {
		return [sha256.Size]byte{}, err
	}
	if err := matchManifestDigest("2× STD 來源", manifest.Sources.STD.SHA256, review.STD); err != nil {
		return [sha256.Size]byte{}, err
	}
	return parseManifestHash("2× 輸出", manifest.OutputSHA256)
}

func verifyHostFont3Manifest(manifest hostFont3Manifest, review Eten24SourceIdentity) ([sha256.Size]byte, [sha256.Size]byte, error) {
	if manifest.Status != "local-only-not-for-distribution" {
		return [sha256.Size]byte{}, [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: 3× host 字型 manifest 未標示為僅限本機")
	}
	for _, item := range []struct {
		name string
		got  string
		want [sha256.Size]byte
	}{
		{"3× ASCII 來源", manifest.SourceSHA256.ASCII, review.ASCII},
		{"3× SPC 來源", manifest.SourceSHA256.SPC, review.SPC},
		{"3× STD 來源", manifest.SourceSHA256.STD, review.STD},
		{"3× ETUNPACK 來源", manifest.SourceSHA256.ETUNPACK, review.ETUNPACK},
	} {
		if err := matchManifestDigest(item.name, item.got, item.want); err != nil {
			return [sha256.Size]byte{}, [sha256.Size]byte{}, err
		}
	}
	wide, ok := manifest.Outputs["host-wide24.golemfnt"]
	if !ok || wide.Width != 24 || wide.Height != 24 {
		return [sha256.Size]byte{}, [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: 3× Wide 輸出必須是 host-wide24.golemfnt 24x24")
	}
	ascii, ok := manifest.Outputs["host-ascii16x24.golemfnt"]
	if !ok || ascii.Width != 16 || ascii.Height != 24 {
		return [sha256.Size]byte{}, [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: 3× ASCII 輸出必須是 host-ascii16x24.golemfnt 16x24")
	}
	wideHash, err := parseManifestHash("3× Wide 輸出", wide.SHA256)
	if err != nil {
		return [sha256.Size]byte{}, [sha256.Size]byte{}, err
	}
	asciiHash, err := parseManifestHash("3× ASCII 輸出", ascii.SHA256)
	if err != nil {
		return [sha256.Size]byte{}, [sha256.Size]byte{}, err
	}
	return wideHash, asciiHash, nil
}

func matchManifestDigest(name, value string, reviewed [sha256.Size]byte) error {
	if reviewed == ([sha256.Size]byte{}) {
		return fmt.Errorf("frontend/ebiten: %s 缺少獨立審核 SHA-256", name)
	}
	got, err := parseManifestHash(name, value)
	if err != nil {
		return err
	}
	if got != reviewed {
		return fmt.Errorf("frontend/ebiten: %s 與獨立審核 SHA-256 不符", name)
	}
	return nil
}

func parseManifestHash(name, value string) ([sha256.Size]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return [sha256.Size]byte{}, fmt.Errorf("frontend/ebiten: %s SHA-256 無效", name)
	}
	var sum [sha256.Size]byte
	copy(sum[:], decoded)
	return sum, nil
}
