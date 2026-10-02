package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Buck repo spec 055: E1 on the zh-CN, ja and ko lanes.  The formal catalogs
// and the release fonts come from BUCKROGERS_CHT_ROOT and
// BUCKROGERS_{ZHTW,ZHCN,JA,KO}_FONT (the convention of ja_test.go); a test
// skips with its reason when they are absent.

type e1FormalLang struct {
	lang, fontEnv string
}

var e1FormalLangs = []e1FormalLang{
	{LangZhTW, "BUCKROGERS_ZHTW_FONT"}, {LangZhCN, "BUCKROGERS_ZHCN_FONT"},
	{LangJa, "BUCKROGERS_JA_FONT"}, {LangKo, "BUCKROGERS_KO_FONT"},
}

// e1FormalInputs returns the text directory, the layout, the lane font and its
// path for one language.
func e1FormalInputs(t *testing.T, lang, fontEnv string) (text string, layout *ManualOverlayLayout, font *xlate.Font, fontPath string) {
	t.Helper()
	root, fontPath := os.Getenv("BUCKROGERS_CHT_ROOT"), os.Getenv(fontEnv)
	if root == "" || fontPath == "" {
		t.Skipf("BUCKROGERS_CHT_ROOT／%s 未設定", fontEnv)
	}
	text = filepath.Join(root, "text")
	b, err := os.ReadFile(filepath.Join(text, "manual-overlay-layout.tsv"))
	if err != nil {
		t.Skip("text/ 沒有 manual-overlay-layout.tsv")
	}
	if layout, err = LoadManualOverlayLayout("manual-overlay-layout.tsv", b); err != nil {
		t.Fatal(err)
	}
	if font, err = xlate.LoadFont(fontPath); err != nil {
		t.Fatal(err)
	}
	return text, layout, font, fontPath
}

func e1FormalCatalog(t *testing.T, text, lang string) (*Catalog, string) {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(text, name))
		if err != nil {
			t.Skipf("text/ 沒有 %s", name)
		}
		return b
	}
	tr := read(LangFile("manual", lang))
	c, err := LoadCatalogLang(read("manual-events.tsv"), read("manual-ordinals.tsv"), tr, lang)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(tr)
	return c, hex.EncodeToString(sum[:])
}

// e1LayoutDigest hashes the layout of every paragraph (line, token kind, runes,
// x, advance), in event-key order, without the font seals: the seals change
// whenever a font is rebuilt, the layout does not.
func e1LayoutDigest(t *testing.T, layout *ManualOverlayLayout, cat *Catalog, font *xlate.Font) (string, int) {
	t.Helper()
	o, err := NewRuntimeManualOverlayLang(layout, cat, font, 3, LangZhTW)
	if err != nil {
		t.Fatal(err)
	}
	o.font.Name = manualDerivedFontIdentity
	base := cloneManualBaseFont(font)
	var keys []string
	entries := map[string]catalogEntryView{}
	for _, e := range cat.byIdentity {
		if e.translation == "" {
			continue
		}
		keys = append(keys, e.eventKey)
		entries[e.eventKey] = catalogEntryView{e.eventKey, e.textKey, e.translation}
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		e := entries[k]
		plan, err := BuildManualE1Plan(layout, cat, DisplayRequest{Generation: 1, EventKey: e.eventKey, TextKey: e.textKey, Translation: e.translation}, base, o.font, o.half)
		if err != nil {
			t.Fatalf("%s：%v", k, err)
		}
		fmt.Fprintf(h, "%s\n", k)
		for _, line := range plan.lines {
			for _, tok := range line.tokens {
				fmt.Fprintf(h, "%d|%s|%q|%d|%d\n", line.row, tok.kind, string(tok.runes), tok.x, tok.advance)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)), len(keys)
}

type catalogEntryView struct{ eventKey, textKey, translation string }

// Digests taken from the plan code of dosgolem 039727e (before spec 055) with
// the release fonts.  They are bound to the SHA-256 of the catalog file and of
// the font file: when either differs the test skips and says why, so a
// proofread catalog or a rebuilt font does not turn it red for a reason that
// has nothing to do with the plan rules.
var e1FrozenLayouts = map[string]struct{ catalog, font, digest string }{
	LangZhTW: {"186b0844b7040a461c2e06f2a3e670e86dd28f3773cc46c3a6516e340c773f0b", "7675cf4e97986985cbeb4b5ed447a2a56d68f4c5bdaf29f6f650164179042697", "08f5386521d5b3dc97289d73dad28597c0fa1ebf4664bc956b3f1c50c5e83c4b"},
	LangZhCN: {"ba35068c5e5b7a2cead41fbb39009fc901994df0b8d2d350239549bedc641707", "2ac47096396778646b9309a59c71d7bba80cf64584d93a782d3b57162cbadd14", "17e0b8bd642c32942a92aa925fa9ad1adb16f2596d2d86e9cfc67118311218e6"},
	LangJa:   {"b9a81a854b10ecc84c92973fbf66f10dfe6348af3e330d2c02dac844396a3e6a", "87ea61ba7898e9416f9b77bea00aaa03f758154ee700701f9205af4f0209e3d6", "7c8cf96d59da60a9db432be254adb4cb30af3a84f72eb31b91df4da266cbb57a"},
}

func TestManualE1PlanLayoutFrozenBeforeSpec055(t *testing.T) {
	for _, c := range e1FormalLangs {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, layout, font, fontPath := e1FormalInputs(t, c.lang, c.fontEnv)
			cat, catSum := e1FormalCatalog(t, text, c.lang)
			fb, err := os.ReadFile(fontPath)
			if err != nil {
				t.Fatal(err)
			}
			fs := sha256.Sum256(fb)
			fontSum := hex.EncodeToString(fs[:])
			want, frozen := e1FrozenLayouts[c.lang]
			if os.Getenv("P55_PRINT") == "" {
				if !frozen {
					t.Skipf("%s 沒有凍結值（規格 055 之前 E1 不支援）", c.lang)
				}
				if want.catalog != catSum || want.font != fontSum {
					t.Skipf("%s 的 catalog 或字型與凍結時不同（catalog %.12s、字型 %.12s），版面摘要不比對", c.lang, catSum, fontSum)
				}
			}
			digest, n := e1LayoutDigest(t, layout, cat, font)
			if os.Getenv("P55_PRINT") != "" {
				t.Logf("FROZEN %s %s %s %s (%d 段)", c.lang, catSum, fontSum, digest, n)
				return
			}
			if digest != want.digest || n != 39 {
				t.Errorf("%s：%d 段版面摘要 %s，凍結值 %s", c.lang, n, digest, want.digest)
			}
		})
	}
}
