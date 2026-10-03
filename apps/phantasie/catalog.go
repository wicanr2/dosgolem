package phantasie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// 譯文 catalog 的載入與查詢（docs/spec/003 §3、§4、§9）。純函式，不依賴機器狀態。

// TSVError 是 TSV 讀取錯誤，帶行號（第 1 列是欄名列）。ParseCatalogTSV、ParseRegions 與
// LoadCatalog 都以它回報，呼叫端可用 errors.As 取得行號。
type TSVError struct {
	Line int
	Msg  string
}

func (e *TSVError) Error() string { return fmt.Sprintf("第 %d 列：%s", e.Line, e.Msg) }

const (
	catHeader          = "key\ttranslation\tsource"
	catProtectedHeader = "key"
	catBOM             = "\xEF\xBB\xBF"
)

// Catalog 實作 Lookup。ui 家族以規範化英文為鍵，prose 以 "h:" 加 12 位十六進位摘要為鍵。
type Catalog struct {
	ui        map[string]string
	prose     map[string]string
	protected map[string]struct{}
}

var _ Lookup = (*Catalog)(nil)

// NormalizeText 是事件文字的規範化（003 §4）：去掉尾端的空白（只有 0x20），開頭的空白保留。
// 不截斷長度：取 Text[:40] 由呼叫端負責。
func NormalizeText(s string) string {
	return strings.TrimRight(s, " ")
}

// Digest 回 prose 的摘要鍵：「h:」加 hex(sha256(規範化文字))[:12]。參數必須已經 NormalizeText。
// 文字以位元組計算（Go 字串即其 UTF-8 位元組），與 tools/prose_build.py 的
// sha256(text.encode("utf-8")) 相同。
func Digest(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return "h:" + hex.EncodeToString(sum[:])[:12]
}

// NewCatalog 由已解析的資料組成 Catalog（測試與組合用）。三個參數都會複製；protected 的鍵先 NormalizeText。
func NewCatalog(ui, prose map[string]string, protected []string) *Catalog {
	c := &Catalog{
		ui:        make(map[string]string, len(ui)),
		prose:     make(map[string]string, len(prose)),
		protected: make(map[string]struct{}, len(protected)),
	}
	for k, v := range ui {
		c.ui[k] = v
	}
	for k, v := range prose {
		c.prose[k] = v
	}
	for _, k := range protected {
		c.protected[NormalizeText(k)] = struct{}{}
	}
	return c
}

// LoadCatalog 讀入 ui、prose 兩份譯文檔與保護清單。uiPath 必填；prosePath、protectedPath 可為空字串（略過）。
// 任何一份有格式錯誤就整體失敗（回傳的 error 含檔名與行號，可用 errors.As 取 *TSVError）。
func LoadCatalog(uiPath, prosePath, protectedPath string) (*Catalog, error) {
	if uiPath == "" {
		return nil, errors.New("uiPath 不可為空")
	}
	ui, err := catLoadFamily(uiPath)
	if err != nil {
		return nil, err
	}
	var prose map[string]string
	if prosePath != "" {
		if prose, err = catLoadFamily(prosePath); err != nil {
			return nil, err
		}
	}
	var protected []string
	if protectedPath != "" {
		data, err := os.ReadFile(protectedPath)
		if err != nil {
			return nil, err
		}
		if protected, err = catParseProtected(data); err != nil {
			return nil, fmt.Errorf("%s：%w", protectedPath, err)
		}
	}
	return NewCatalog(ui, prose, protected), nil
}

func catLoadFamily(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseCatalogTSV(data)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	return m, nil
}

// UI 查 ui 家族（鍵是規範化英文）。ok=false 表示沒有這個鍵；譯文為空字串表示尚未翻譯。
func (c *Catalog) UI(key string) (string, bool) {
	v, ok := c.ui[key]
	return v, ok
}

// Prose 查 prose 家族（鍵是 Digest 的結果）。
func (c *Catalog) Prose(digestKey string) (string, bool) {
	v, ok := c.prose[digestKey]
	return v, ok
}

// Protected 回該規範化文字是否在保護清單內（003 §9）。輸入會再做一次 NormalizeText（冪等）：
// 保護清單寧可多擋，漏擋的代價較高。
func (c *Catalog) Protected(normalized string) bool {
	_, ok := c.protected[NormalizeText(normalized)]
	return ok
}

// Stats 回 ui、prose、保護清單的筆數。
func (c *Catalog) Stats() (ui, prose, protected int) {
	return len(c.ui), len(c.prose), len(c.protected)
}

// ParseCatalogTSV 解析一份譯文 TSV（003 §3）：UTF-8、LF、第一列欄名 key translation source、每列三欄。
//
// 跳脫只有 \t、\\ 與譯文開頭的 \c（載入後變成 CenterMark 前綴）。以下一律回 *TSVError 並指出行號：
// BOM、CR、不合法的 UTF-8、欄名列不符、欄數不是 3、空鍵、其他反斜線序列、鍵重複（rstrip 之後比較）。
// 鍵與譯文的尾端空白 rstrip（執行期載入器的容錯，lint 才視為錯誤）；開頭空白保留。
// 空譯文保留為空字串；source 欄不解析。
func ParseCatalogTSV(data []byte) (map[string]string, error) {
	if bytes.HasPrefix(data, []byte(catBOM)) {
		return nil, &TSVError{1, "檔案含 BOM"}
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		return nil, &TSVError{1 + bytes.Count(data[:i], []byte{'\n'}), "含 CR（要 LF）"}
	}
	lines := strings.Split(string(data), "\n")
	if n := len(lines); lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if len(lines) == 0 {
		return nil, &TSVError{1, "缺少欄名列"}
	}
	out := make(map[string]string, len(lines))
	firstSeen := make(map[string]int, len(lines))
	for i, line := range lines {
		n := i + 1
		if !utf8.ValidString(line) {
			return nil, &TSVError{n, "不是合法的 UTF-8"}
		}
		if n == 1 {
			if line != catHeader {
				return nil, &TSVError{1, "第一列欄名要是 key、translation、source"}
			}
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			return nil, &TSVError{n, fmt.Sprintf("欄數是 %d，要 3", len(cols))}
		}
		key, err := catUnescape(cols[0])
		if err != nil {
			return nil, &TSVError{n, "鍵：" + err.Error()}
		}
		key = NormalizeText(key)
		if key == "" {
			return nil, &TSVError{n, "鍵為空"}
		}
		tr, marker := cols[1], ""
		if strings.HasPrefix(tr, `\c`) {
			marker, tr = string(CenterMark), tr[2:]
		}
		val, err := catUnescape(tr)
		if err != nil {
			return nil, &TSVError{n, "譯文：" + err.Error()}
		}
		if first, dup := firstSeen[key]; dup {
			return nil, &TSVError{n, fmt.Sprintf("鍵 %q 重複（第 %d 列已有）", key, first)}
		}
		firstSeen[key] = n
		out[key] = marker + NormalizeText(val)
	}
	return out, nil
}

// catUnescape 還原 \t 與 \\；其他反斜線序列與尾端的反斜線是錯誤（與 tools/catalog_lib.py 的 unescape 相同）。
func catUnescape(s string) (string, error) {
	if !strings.Contains(s, `\`) {
		return s, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(s) {
			return "", errors.New("尾端的反斜線")
		}
		switch s[i+1] {
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		default:
			r, _ := utf8.DecodeRuneInString(s[i+1:])
			return "", fmt.Errorf("不認得的跳脫 \\%c", r)
		}
		i++
	}
	return b.String(), nil
}

// catParseProtected 讀保護清單（欄：key、note）。語意與 tools/lint_catalog.py、tools/ui_prune.py 相同：
// 取第一欄、不做跳脫還原、略過空白列；另外鍵先 NormalizeText。空鍵與 CR 一律當錯誤，
// 因為保護清單讀歪會讓該鍵永遠比不到（漏擋）。
func catParseProtected(data []byte) ([]string, error) {
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		return nil, &TSVError{1 + bytes.Count(data[:i], []byte{'\n'}), "含 CR（要 LF）"}
	}
	var keys []string
	for i, line := range strings.Split(string(data), "\n") {
		n := i + 1
		if !utf8.ValidString(line) {
			return nil, &TSVError{n, "不是合法的 UTF-8"}
		}
		first := line
		if j := strings.IndexByte(line, '\t'); j >= 0 {
			first = line[:j]
		}
		if n == 1 {
			if first != catProtectedHeader {
				return nil, &TSVError{1, "第一列欄名要是 key、note"}
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		k := NormalizeText(first)
		if k == "" {
			return nil, &TSVError{n, "鍵為空"}
		}
		keys = append(keys, k)
	}
	return keys, nil
}
