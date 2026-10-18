package strikecommander

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// 譯文 catalog：text/opttest_labels.<lang>.tsv，UTF-8，三欄 key、translation、source，
// 第一列是欄名（001 §2.7）。key 與記憶體讀到的字串位元組完全相同（ASCII），不做任何跳脫還原。
//
// 注意：tools/font 的 catalog_lib.read_tsv 會還原 \t、\\ 並處理開頭的 \c；本 catalog 不做。
// 兩者只在 key 與譯文不含反斜線時一致。001 的 89 個 LABL key 中，普查 TSV 只有
// `Prideaux\\'s niche` 帶反斜線，那是普查工具的跳脫；原始位元組是 `Prideaux's niche`（001 §2.7）。

const catalogHeader = "key\ttranslation\tsource"

// catalogRow 是一列解析結果，lint 與載入共用。
type catalogRow struct {
	line            int
	key, tr, source string
	cols            int
	raw             string
}

func splitCatalog(b []byte) (header string, rows []catalogRow, err error) {
	if !utf8.Valid(b) {
		return "", nil, fmt.Errorf("strikecommander: catalog 不是合法 UTF-8")
	}
	s := string(b)
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return "", nil, fmt.Errorf("strikecommander: catalog 是空的")
	}
	header = lines[0]
	for i, ln := range lines[1:] {
		cols := strings.Split(ln, "\t")
		r := catalogRow{line: i + 2, cols: len(cols), raw: ln}
		if len(cols) == 3 {
			r.key, r.tr, r.source = cols[0], cols[1], cols[2]
		}
		rows = append(rows, r)
	}
	return header, rows, nil
}

// ParseCatalog 解析 catalog 成 key → 譯文。欄名、欄數、key 重複、空譯文都回錯。
// 執行期載入前應先過 Lint；這裡只擋格式錯誤。
func ParseCatalog(b []byte) (map[string]string, error) {
	header, rows, err := splitCatalog(b)
	if err != nil {
		return nil, err
	}
	if header != catalogHeader {
		return nil, fmt.Errorf("strikecommander: catalog 第 1 列欄名要是 key、translation、source")
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.cols != 3 {
			return nil, fmt.Errorf("strikecommander: catalog 第 %d 列欄數 %d，要 3", r.line, r.cols)
		}
		if _, dup := out[r.key]; dup {
			return nil, fmt.Errorf("strikecommander: catalog 第 %d 列 key %q 重複", r.line, r.key)
		}
		if r.tr == "" {
			return nil, fmt.Errorf("strikecommander: catalog 第 %d 列 key %q 譯文是空的", r.line, r.key)
		}
		out[r.key] = r.tr
	}
	return out, nil
}

// LoadCatalog 讀檔後呼叫 ParseCatalog。
func LoadCatalog(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCatalog(b)
}
