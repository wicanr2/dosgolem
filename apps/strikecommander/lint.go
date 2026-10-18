package strikecommander

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// LintReport 是 §2.7 lint 的結果。Errors 非空即失敗；Untranslated 只列出、不判失敗。
type LintReport struct {
	Errors       []string
	Untranslated []string // labl 集合中 catalog 沒有的 key，排序
}

// OK 回沒有失敗項。
func (r LintReport) OK() bool { return len(r.Errors) == 0 }

// Lint 依 001 §2.7 檢查一份熱點標籤 catalog：
//   - 欄名、每列三欄、key 唯一、譯文非空；
//   - 譯文每字都在全形表裡（半形字、缺字分別報）；
//   - 2 倍時 n × 8 ≤ 320；
//   - 每個 key 都在 labl 集合（OPTIONS.IFF 與 OPTIONS2.IFF 的 LABL 去重）裡；
//   - 漏譯（labl 有而 catalog 沒有）列在 Untranslated。
func Lint(b []byte, wide *WideTable, labl map[string]bool) LintReport {
	var rep LintReport
	bad := func(format string, a ...any) { rep.Errors = append(rep.Errors, fmt.Sprintf(format, a...)) }
	header, rows, err := splitCatalog(b)
	if err != nil {
		bad("%v", err)
		return rep
	}
	if header != catalogHeader {
		bad("第 1 列欄名要是 key、translation、source")
	}
	seen := make(map[string]int, len(rows))
	for _, r := range rows {
		if r.cols != 3 {
			bad("第 %d 列欄數 %d，要 3", r.line, r.cols)
			continue
		}
		if first, dup := seen[r.key]; dup {
			bad("第 %d 列 key %q 與第 %d 列重複", r.line, r.key, first)
		} else {
			seen[r.key] = r.line
		}
		if !labl[r.key] {
			bad("第 %d 列 key %q 不在 LABL 集合（孤兒 key）", r.line, r.key)
		}
		if r.tr == "" {
			bad("第 %d 列 key %q 譯文是空的", r.line, r.key)
			continue
		}
		for _, c := range r.tr {
			switch {
			case !wide.Has(c):
				bad("第 %d 列 key %q 缺字 %q（U+%04X）", r.line, r.key, c, c)
			case !wide.Wide(c):
				bad("第 %d 列 key %q 含半形字 %q（U+%04X）", r.line, r.key, c, c)
			}
		}
		if n := utf8.RuneCountInString(r.tr); n*8 > 320 {
			bad("第 %d 列 key %q 譯文 %d 字，2 倍寬 %d 超過 320", r.line, r.key, n, n*8)
		}
	}
	for k := range labl {
		if _, ok := seen[k]; !ok {
			rep.Untranslated = append(rep.Untranslated, k)
		}
	}
	sort.Strings(rep.Untranslated)
	return rep
}
