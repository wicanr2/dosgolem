package buckrogers

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 057 §5.5 and §5.7: the digests that pin the behaviour of the paths a
// spec must not change.  The baseline values were computed on the clean
// export of dosgolem bba48d9 with the same functions (the adapters in
// shrink_adapter_test.go are the only per-commit part).

// shrinkFontDigest is sha256("shrinkfont1" || W || H || n || sorted (rune,
// glyph)): W, H, n and the rune as uint32 big endian, the glyph bytes as they
// are.  The font name is not part of it (the derived fonts carry the size and
// the threshold in their names from spec 057 on).
func shrinkFontDigest(f *xlate.Font) string {
	h := sha256.New()
	h.Write([]byte("shrinkfont1"))
	u32 := func(v int) {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(v))
		h.Write(b[:])
	}
	u32(f.W)
	u32(f.H)
	u32(len(f.Glyphs))
	rs := make([]rune, 0, len(f.Glyphs))
	for r := range f.Glyphs {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
	for _, r := range rs {
		u32(int(r))
		h.Write(f.Glyphs[r])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// shrinkProbeDigest is the same over a fixed list of runes (the probe sets of
// spec 057 §5.3 and spec 058 §5.3): the glyphs of runes in the order of the
// code points, with the font size in front.
func shrinkProbeDigest(f *xlate.Font, probe []rune) string {
	rs := append([]rune(nil), probe...)
	sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("shrinkprobe1 %dx%d", f.W, f.H)))
	var last rune = -1
	for _, r := range rs {
		if r == last {
			continue
		}
		last = r
		h.Write([]byte(fmt.Sprintf(" %U:", r)))
		h.Write(f.Glyphs[r])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// shrinkBaselineFont is a font a baseline row is about.
type shrinkBaselineFont struct {
	label, lang, path string
	font              *xlate.Font
}

// shrinkBaselineFonts loads the formal fonts (the release fonts and, when
// BUCKROGERS_ZHTW_ETEN_FONT is set, the local Eten font).  It skips the test
// when the environment is not set.
func shrinkBaselineFonts(t *testing.T) []shrinkBaselineFont {
	t.Helper()
	var out []shrinkBaselineFont
	add := func(label, lang, env string, required bool) {
		p := os.Getenv(env)
		if p == "" {
			if required {
				t.Skip(env + " 未設定")
			}
			return
		}
		f, err := xlate.LoadFont(p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, shrinkBaselineFont{label, lang, p, f})
	}
	add("zh-TW-unifont", LangZhTW, "BUCKROGERS_ZHTW_FONT", true)
	add("zh-TW-eten", LangZhTW, "BUCKROGERS_ZHTW_ETEN_FONT", false)
	add("zh-CN", LangZhCN, "BUCKROGERS_ZHCN_FONT", true)
	add("ja", LangJa, "BUCKROGERS_JA_FONT", true)
	add("ko", LangKo, "BUCKROGERS_KO_FONT", true)
	return out
}

func shrinkFileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// shrinkFontDigestRows computes the baseline rows of the fonts: for every
// font, level of the table of its language and scale, the digest of the full
// and of the half derived font.  Columns: font file SHA-256, label, level,
// scale, class, digest.
func shrinkFontDigestRows(t *testing.T) []string {
	t.Helper()
	var rows []string
	for _, bf := range shrinkBaselineFonts(t) {
		sha := shrinkFileSHA(t, bf.path)
		for _, sp := range adapterLevels(bf.lang) {
			for _, scale := range []int{2, 3} {
				fs := shrinkFontsOf(bf.font, sp, scale)
				rows = append(rows,
					strings.Join([]string{sha, bf.label, fmt.Sprint(sp.Level), fmt.Sprint(scale), "full", shrinkFontDigest(fs.Full)}, "\t"),
					strings.Join([]string{sha, bf.label, fmt.Sprint(sp.Level), fmt.Sprint(scale), "half", shrinkFontDigest(fs.Half)}, "\t"))
			}
		}
	}
	return rows
}

func shrinkBaselinePath() string { return filepath.Join("testdata", "shrink_fonts_baseline.tsv") }
