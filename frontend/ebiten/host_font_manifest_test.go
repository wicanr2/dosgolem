package ebiten

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manifestTestHash(name string) [sha256.Size]byte { return sha256.Sum256([]byte(name)) }

func manifestTestHex(sum [sha256.Size]byte) string { return hex.EncodeToString(sum[:]) }

func manifestTestReview() HostFontManifestReview {
	return HostFontManifestReview{
		Font2: Eten15SourceIdentity{ASCII: manifestTestHash("15-ascii"), SPC: manifestTestHash("15-spc"), STD: manifestTestHash("15-std")},
		Font3: Eten24SourceIdentity{ASCII: manifestTestHash("24-ascii"), SPC: manifestTestHash("24-spc"), STD: manifestTestHash("24-std"), ETUNPACK: manifestTestHash("etunpack")},
	}
}

func writeManifestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func hostManifestFixture(t *testing.T) HostFontManifestPreflight {
	t.Helper()
	dir := t.TempDir()
	font2Path := filepath.Join(dir, "host16.golemfnt")
	font2Hash := writeHostFont3Fixture(t, font2Path, 16, 16, []rune{'設', '定', '套', '用', '取', '消', '2', '3', '×'})
	font3 := hostFont3FixtureFiles(t)
	review := manifestTestReview()
	catalogDir := filepath.Join(dir, "text")
	if err := os.Mkdir(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalogName := "host-ui.zh-TW.tsv"
	catalogData := []byte("key\tzh-TW\nhost.settings\t設定\n")
	if err := os.WriteFile(filepath.Join(catalogDir, catalogName), catalogData, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := map[string]string{"filename": catalogName, "sha256": manifestTestHex(sha256.Sum256(catalogData))}
	font2Manifest := map[string]any{
		"catalogs":            []any{catalog},
		"distribution_status": "local-only-not-for-distribution",
		"format":              map[string]any{"magic": "GOLEMFNT", "width": 16, "height": 16},
		"output_sha256":       manifestTestHex(font2Hash),
		"sources": map[string]any{
			"asc": map[string]string{"sha256": manifestTestHex(review.Font2.ASCII)},
			"spc": map[string]string{"sha256": manifestTestHex(review.Font2.SPC)},
			"std": map[string]string{"sha256": manifestTestHex(review.Font2.STD)},
		},
	}
	font3Manifest := map[string]any{
		"catalog": catalog,
		"status":  "local-only-not-for-distribution",
		"source_sha256": map[string]string{
			"ascii": manifestTestHex(review.Font3.ASCII), "spc": manifestTestHex(review.Font3.SPC),
			"std": manifestTestHex(review.Font3.STD), "etunpack": manifestTestHex(review.Font3.ETUNPACK),
		},
		"outputs": map[string]any{
			"host-wide24.golemfnt":     map[string]any{"width": 24, "height": 24, "sha256": manifestTestHex(font3.WideSHA256)},
			"host-ascii16x24.golemfnt": map[string]any{"width": 16, "height": 24, "sha256": manifestTestHex(font3.ASCIISHA256)},
		},
	}
	font2ManifestPath := filepath.Join(dir, "font2-manifest.json")
	font3ManifestPath := filepath.Join(dir, "font3-manifest.json")
	writeManifestJSON(t, font2ManifestPath, font2Manifest)
	writeManifestJSON(t, font3ManifestPath, font3Manifest)
	return HostFontManifestPreflight{
		CatalogDir:        catalogDir,
		Font2ManifestPath: font2ManifestPath, Font2Path: font2Path,
		Font3ManifestPath: font3ManifestPath, Font3WidePath: font3.WidePath, Font3ASCIIPath: font3.ASCIIPath,
		Labels: draftLabels(), Review: review,
	}
}

func TestLoadHostFontsFromReviewedManifests(t *testing.T) {
	fonts, err := LoadHostFontsFromReviewedManifests(hostManifestFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if fonts.Font2.W != 16 || fonts.Font2.H != 16 || fonts.Font3.Wide.W != 24 || fonts.Font3.ASCII.H != 24 {
		t.Fatalf("loaded dimensions: 2×=%dx%d wide=%dx%d ascii=%dx%d", fonts.Font2.W, fonts.Font2.H, fonts.Font3.Wide.W, fonts.Font3.Wide.H, fonts.Font3.ASCII.W, fonts.Font3.ASCII.H)
	}
}

func TestLoadHostFontsFromReviewedManifestsFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, in *HostFontManifestPreflight)
		want   string
	}{
		{"missing-review", func(_ *testing.T, in *HostFontManifestPreflight) { in.Review.Font3.STD = [sha256.Size]byte{} }, "缺少獨立審核"},
		{"unreviewed-source-claim", func(t *testing.T, in *HostFontManifestPreflight) {
			data, err := os.ReadFile(in.Font2ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.Replace(string(data), manifestTestHex(in.Review.Font2.ASCII), manifestTestHex(manifestTestHash("substituted-15-ascii")), 1))
			if err := os.WriteFile(in.Font2ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			in.Font2Path = filepath.Join(t.TempDir(), "never-read.golemfnt")
		}, "與獨立審核 SHA-256 不符"},
		{"not-local-only", func(t *testing.T, in *HostFontManifestPreflight) {
			data, err := os.ReadFile(in.Font3ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.Replace(string(data), "local-only-not-for-distribution", "public", 1))
			if err := os.WriteFile(in.Font3ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "未標示為僅限本機"},
		{"wrong-3x-output-shape", func(t *testing.T, in *HostFontManifestPreflight) {
			data, err := os.ReadFile(in.Font3ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.Replace(string(data), "\"width\":24", "\"width\":22", 1))
			if err := os.WriteFile(in.Font3ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "Wide 輸出必須是"},
		{"stale-output", func(t *testing.T, in *HostFontManifestPreflight) {
			writeHostFont3Fixture(t, in.Font3WidePath, 24, 24, []rune{'設', '定', '套', '用', '取', '消', '×', '新'})
		}, "SHA-256 不符"},
		{"stale-catalog", func(t *testing.T, in *HostFontManifestPreflight) {
			if err := os.WriteFile(filepath.Join(in.CatalogDir, "host-ui.zh-TW.tsv"), []byte("new translation"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "SHA-256 已過期"},
		{"added-catalog", func(t *testing.T, in *HostFontManifestPreflight) {
			if err := os.WriteFile(filepath.Join(in.CatalogDir, "new.zh-TW.tsv"), []byte("new catalog"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "譯文清單與目前譯文不符"},
		{"missing-catalog-dir", func(_ *testing.T, in *HostFontManifestPreflight) {
			in.CatalogDir = ""
		}, "缺少目前譯文目錄"},
		{"unknown-json-field", func(t *testing.T, in *HostFontManifestPreflight) {
			data, err := os.ReadFile(in.Font2ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data[:len(data)-1], []byte(",\"unreviewed\":true}")...)
			if err := os.WriteFile(in.Font2ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "無法讀取 2× host 字型 manifest"},
		{"trailing-json-value", func(t *testing.T, in *HostFontManifestPreflight) {
			data, err := os.ReadFile(in.Font3ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, []byte(" {}")...)
			if err := os.WriteFile(in.Font3ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "無法讀取 3× host 字型 manifest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := hostManifestFixture(t)
			tc.mutate(t, &in)
			if _, err := LoadHostFontsFromReviewedManifests(in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

// This local-only receipt exercises the actual ignored rebuild manifests.  It
// never supplies a private path, digest, or bitmap through source control.
func TestLoadHostFontsFromReviewedManifestsLocal(t *testing.T) {
	get := func(name string) string {
		t.Helper()
		value := os.Getenv(name)
		if value == "" {
			t.Skipf("%s not supplied for local manifest receipt", name)
		}
		return value
	}
	decode := func(name string) [sha256.Size]byte {
		t.Helper()
		value := get(name)
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size {
			t.Fatalf("%s is not a SHA-256 hex value: %q", name, value)
		}
		var sum [sha256.Size]byte
		copy(sum[:], decoded)
		return sum
	}
	in := HostFontManifestPreflight{
		CatalogDir:        get("HOST_FONT_CATALOG_DIR"),
		Font2ManifestPath: get("HOST_FONT2_MANIFEST"),
		Font2Path:         get("HOST_FONT2_PATH"),
		Font3ManifestPath: get("HOST_FONT3_MANIFEST"),
		Font3WidePath:     get("HOST_FONT3_WIDE"),
		Font3ASCIIPath:    get("HOST_FONT3_ASCII"),
		Labels:            draftLabels(),
		Review: HostFontManifestReview{
			Font2: Eten15SourceIdentity{ASCII: decode("HOST_FONT2_ASCII_SOURCE_SHA256"), SPC: decode("HOST_FONT2_SPC_SOURCE_SHA256"), STD: decode("HOST_FONT2_STD_SOURCE_SHA256")},
			Font3: Eten24SourceIdentity{ASCII: decode("HOST_FONT3_ASCII_SOURCE_SHA256"), SPC: decode("HOST_FONT3_SPC_SOURCE_SHA256"), STD: decode("HOST_FONT3_STD_SOURCE_SHA256"), ETUNPACK: decode("HOST_FONT3_ETUNPACK_SOURCE_SHA256")},
		},
	}
	fonts, err := LoadHostFontsFromReviewedManifests(in)
	if err != nil {
		t.Fatal(err)
	}
	if fonts.Font2.W != 16 || fonts.Font2.H != 16 || fonts.Font3.Wide.W != 24 || fonts.Font3.ASCII.W != 16 {
		t.Fatalf("local dimensions: 2×=%dx%d wide=%dx%d ascii=%dx%d", fonts.Font2.W, fonts.Font2.H, fonts.Font3.Wide.W, fonts.Font3.Wide.H, fonts.Font3.ASCII.W, fonts.Font3.ASCII.H)
	}
}
