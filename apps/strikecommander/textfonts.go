package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/xlate"
)

// 反鋸齒圖集（strike_commander 規格 011 §3.2、§3.9）：五個字級的 GOLEMAAF 圖集、字集與 manifest，
// 由 strike_commander tools/font/build_noto.sh 烘製；這裡只讀檔、核對與解析，不連結 FreeType。

// TextSizes 是圖集的字級集合（011 §3.1、§3.2.3、§7 待定 9），遞增。
var TextSizes = [...]int{15, 16, 22, 32, 33}

// 檔名與表格（011 §3.2.3）。
const (
	TextManifestName = "zh-TW.cjkb.manifest.tsv"
	TextCharsName    = "zh-TW.cjkb.chars.txt"
	textTableHeader  = "row\tname\tpx\tcount\tbytes\tvalue"
	// TextAnchorMatch、TextAnchorTest 是 TextFonts.Anchor 的兩個值（011 §3.8 fonts.tsv 的 anchor）。
	TextAnchorMatch = "match"
	TextAnchorTest  = "test-atlas"
)

// textMetaKeys 是 manifest 的 7 個 meta 列，順序固定。
var textMetaKeys = [...]string{"kind", "pillow", "freetype", "image_id", "zip_sha256", "otf_sha256", "chars_sha256"}

// TextAtlasName 回字級 px 的圖集檔名。
func TextAtlasName(px int) string { return fmt.Sprintf("zh-TW.cjkb%d.aaf", px) }

// TextFontName 回字級 px 的字型名（011 §3.2.3：SC-CJKB<px>）。
func TextFontName(px int) string { return fmt.Sprintf("SC-CJKB%d", px) }

// FontFile 對應 manifest 與 fonts.tsv 的一個 file 列（011 §3.8）。
type FontFile struct {
	Name             string
	Px, Count, Bytes int
	SHA256           string
}

// TextFonts 是載入並核對過的五個字級圖集（011 §3.9）。
type TextFonts struct {
	BySize map[int]*xlate.AlphaFont
	Chars  map[rune]bool
	Files  []FontFile
	Anchor string
}

// TextAscent 回字級 px 的 em 框上緣在基線上方的像素：⌊(px × 880 + 500) / 1000⌋（011 §3.2.4，OS/2 sTypoAscender 880）。
func TextAscent(px int) int { return (px*880 + 500) / 1000 }

// textRow 是 manifest／預期表的一列。
type textRow struct {
	row, name, px, count, bytes, value string
}

func (r textRow) col(i int) string {
	return [...]string{r.row, r.name, r.px, r.count, r.bytes, r.value}[i]
}

var textColNames = [...]string{"row", "name", "px", "count", "bytes", "value"}

// parseTextTable 嚴格讀 manifest／預期表：UTF-8、LF、最後一列後一個 LF、每列 6 欄、欄值不得空或有多餘空白。
func parseTextTable(b []byte, what string) ([]textRow, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("strikecommander: %s 不是 UTF-8", what)
	}
	s := string(b)
	if !strings.HasSuffix(s, "\n") || strings.HasSuffix(s, "\n\n") || strings.Contains(s, "\r") {
		return nil, fmt.Errorf("strikecommander: %s 要以單一 LF 結尾、不含 CR", what)
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if lines[0] != textTableHeader {
		return nil, fmt.Errorf("strikecommander: %s 第 1 列欄名不符", what)
	}
	var rows []textRow
	for i, ln := range lines[1:] {
		c := strings.Split(ln, "\t")
		if len(c) != 6 {
			return nil, fmt.Errorf("strikecommander: %s 第 %d 列欄數 %d，要 6", what, i+2, len(c))
		}
		for _, v := range c {
			if v == "" || strings.TrimSpace(v) != v {
				return nil, fmt.Errorf("strikecommander: %s 第 %d 列有空欄或多餘空白", what, i+2)
			}
		}
		rows = append(rows, textRow{c[0], c[1], c[2], c[3], c[4], c[5]})
	}
	return rows, nil
}

// splitTextManifest 分出 meta（固定 7 列在前）與 file 列（5 個圖集 px 遞增，最後是字集，px 0）。
func splitTextManifest(rows []textRow) (map[string]string, []textRow, error) {
	if len(rows) < len(textMetaKeys) {
		return nil, nil, fmt.Errorf("strikecommander: manifest 只有 %d 列", len(rows))
	}
	meta := map[string]string{}
	for i, k := range textMetaKeys {
		r := rows[i]
		if r.row != "meta" || r.name != k || r.px != "-" || r.count != "-" || r.bytes != "-" {
			return nil, nil, fmt.Errorf("strikecommander: manifest 第 %d 列應為 meta %s", i+2, k)
		}
		meta[k] = r.value
	}
	files := rows[len(textMetaKeys):]
	if len(files) != len(TextSizes)+1 {
		return nil, nil, fmt.Errorf("strikecommander: manifest 有 %d 個 file 列，字級集合要恰為 %v 加字集", len(files), TextSizes)
	}
	for i, r := range files {
		name, px := TextCharsName, "0"
		if i < len(TextSizes) {
			name, px = TextAtlasName(TextSizes[i]), strconv.Itoa(TextSizes[i])
		}
		if r.row != "file" || r.name != name || r.px != px {
			return nil, nil, fmt.Errorf("strikecommander: manifest 第 %d 個 file 列應為 %s px %s，得 %s %s %s", i+1, name, px, r.row, r.name, r.px)
		}
	}
	return meta, files, nil
}

// compareTextExpected 是 011 §3.2.3 比對規則 1 到 3（第 4 條由 LoadTextFonts 重算檔案）。
func compareTextExpected(meta map[string]string, files, expected []textRow) error {
	for i, r := range expected {
		if r.row != "file" {
			return fmt.Errorf("strikecommander: 預期表第 %d 列不是 file 列", i+2)
		}
	}
	if len(files) != len(expected) {
		return fmt.Errorf("strikecommander: manifest file 列 %d 列，預期表 %d 列", len(files), len(expected))
	}
	for i := range files {
		for c := 0; c < 6; c++ {
			if files[i].col(c) != expected[i].col(c) {
				return fmt.Errorf("strikecommander: 第 %d 個 file 列（%s）的 %s 不同：manifest %s、預期表 %s",
					i+1, files[i].name, textColNames[c], files[i].col(c), expected[i].col(c))
			}
		}
	}
	var charsSHA string
	for _, r := range expected {
		if r.name == TextCharsName {
			charsSHA = r.value
		}
	}
	if meta["chars_sha256"] != charsSHA {
		return fmt.Errorf("strikecommander: manifest chars_sha256 與預期表字集列不同")
	}
	if meta["kind"] != "formal" {
		return fmt.Errorf("strikecommander: manifest kind 是 %q，給了預期表只接受 formal", meta["kind"])
	}
	return nil
}

// LoadTextFonts 讀 dir 的 manifest、五個圖集與字集，逐檔重算 SHA-256 與位元組數（011 §3.9）。
// expected 非空時照 011 §3.2.3 比對規則與預期表比對，Anchor 為 "match"；expected 為空時只接受 kind = test，
// Anchor 為 "test-atlas"。五個圖集的碼點集合兩兩相同且等於字集檔；任一不成立回錯。
func LoadTextFonts(dir, expected string) (*TextFonts, error) {
	mb, err := os.ReadFile(filepath.Join(dir, TextManifestName))
	if err != nil {
		return nil, err
	}
	rows, err := parseTextTable(mb, "manifest")
	if err != nil {
		return nil, err
	}
	meta, files, err := splitTextManifest(rows)
	if err != nil {
		return nil, err
	}
	tf := &TextFonts{BySize: map[int]*xlate.AlphaFont{}}
	for i, r := range files {
		b, err := os.ReadFile(filepath.Join(dir, r.name))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		if r.bytes != strconv.Itoa(len(b)) || r.value != hex.EncodeToString(sum[:]) {
			return nil, fmt.Errorf("strikecommander: %s 的位元組數或 SHA-256 與 manifest 不符", r.name)
		}
		px, _ := strconv.Atoi(r.px)
		count, err := strconv.Atoi(r.count)
		if err != nil {
			return nil, fmt.Errorf("strikecommander: manifest %s 的 count %q 不是整數", r.name, r.count)
		}
		tf.Files = append(tf.Files, FontFile{Name: r.name, Px: px, Count: count, Bytes: len(b), SHA256: r.value})
		if i == len(TextSizes) {
			s := strings.TrimSuffix(string(b), "\n")
			if !utf8.ValidString(s) {
				return nil, fmt.Errorf("strikecommander: %s 不是 UTF-8", r.name)
			}
			tf.Chars = map[rune]bool{}
			for _, c := range s {
				tf.Chars[c] = true
			}
			if utf8.RuneCountInString(s) != count || len(tf.Chars) != count {
				return nil, fmt.Errorf("strikecommander: %s 有 %d 字（不重複 %d），manifest 記 %d", r.name, utf8.RuneCountInString(s), len(tf.Chars), count)
			}
			continue
		}
		f, err := xlate.ParseAlphaFont(b)
		if err != nil {
			return nil, fmt.Errorf("strikecommander: %s：%w", r.name, err)
		}
		if f.Px != px {
			return nil, fmt.Errorf("strikecommander: %s 的圖集 px %d，manifest 記 %d", r.name, f.Px, px)
		}
		if len(f.Glyphs) != count {
			return nil, fmt.Errorf("strikecommander: %s 有 %d 字，manifest 記 %d", r.name, len(f.Glyphs), count)
		}
		f.Name = TextFontName(px)
		tf.BySize[px] = f
	}
	for _, px := range TextSizes {
		f := tf.BySize[px]
		if len(f.Glyphs) != len(tf.Chars) {
			return nil, fmt.Errorf("strikecommander: %s 的碼點集合與字集檔不同", TextAtlasName(px))
		}
		for c := range f.Glyphs {
			if !tf.Chars[c] {
				return nil, fmt.Errorf("strikecommander: %s 的碼點集合與字集檔不同（U+%04X）", TextAtlasName(px), c)
			}
		}
	}
	if meta["chars_sha256"] != files[len(files)-1].value {
		return nil, fmt.Errorf("strikecommander: manifest chars_sha256 與字集列不同")
	}
	if expected == "" {
		if meta["kind"] != "test" {
			return nil, fmt.Errorf("strikecommander: manifest kind 是 %q，沒給預期表只接受 test", meta["kind"])
		}
		tf.Anchor = TextAnchorTest
		return tf, nil
	}
	eb, err := os.ReadFile(expected)
	if err != nil {
		return nil, err
	}
	exp, err := parseTextTable(eb, "預期表")
	if err != nil {
		return nil, err
	}
	if err := compareTextExpected(meta, files, exp); err != nil {
		return nil, err
	}
	tf.Anchor = TextAnchorMatch
	return tf, nil
}

// Font 回字級 px 的圖集；沒有時回 nil。
func (t *TextFonts) Font(px int) *xlate.AlphaFont {
	if t == nil {
		return nil
	}
	return t.BySize[px]
}
