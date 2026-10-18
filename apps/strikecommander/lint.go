package strikecommander

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// LintReport 是 §2.7 lint 的結果。Errors 非空即失敗；Untranslated 只列出、不判失敗。
type LintReport struct {
	Errors       []string
	Untranslated []string // key 集合中 catalog 沒有的 key，排序
}

// OK 回沒有失敗項。
func (r LintReport) OK() bool { return len(r.Errors) == 0 }

// Lint 依 001 §2.7 與 002 §2.3 檢查一份熱點標籤 catalog：
//   - 欄名、每列三欄、key 唯一、譯文非空；
//   - 譯文每字都在全形表裡（半形字、缺字分別報）；範本譯文恰好一個 %s、一個 %d，
//     拿掉這兩個佔位符後其餘字元照同一規則檢查；
//   - 2 倍時 n × 8 ≤ 320（範本以拿掉佔位符後的字數計）；
//   - 每個 key 都在 key 集合（labl ∪ 兩個範本 ∪ 九個短代號）裡；labl 是 OPTIONS.IFF 與
//     OPTIONS2.IFF 的 LABL 去重；
//   - 漏譯（key 集合有而 catalog 沒有）列在 Untranslated；
//   - 防呆（只看 labl）：沒有 key 符合 removeRe 或 countRe、短代號不在 labl、WeaponLongNames 都在 labl。
//
// 字型的固定附加字另由 CheckFontExtra 檢查。
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
	keys := make(map[string]bool, len(labl)+2+len(WeaponShortNames))
	for k, ok := range labl {
		keys[k] = keys[k] || ok
	}
	keys[TemplateCount], keys[TemplateRemove] = true, true
	for _, k := range WeaponShortNames {
		keys[k] = true
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
		if !keys[r.key] {
			bad("第 %d 列 key %q 不在 LABL、範本、短代號集合（孤兒 key）", r.line, r.key)
		}
		if r.tr == "" {
			bad("第 %d 列 key %q 譯文是空的", r.line, r.key)
			continue
		}
		text := r.tr
		if isTemplateKey(r.key) {
			for _, ph := range []string{"%s", "%d"} {
				if n := strings.Count(text, ph); n != 1 {
					bad("第 %d 列範本 %q 的譯文有 %d 個 %s，要恰好 1 個", r.line, r.key, n, ph)
				}
			}
			// 佔位符各拿掉一個；多出的會以半形字報出。
			text = strings.Replace(strings.Replace(text, "%s", "", 1), "%d", "", 1)
		}
		for _, c := range text {
			switch {
			case !wide.Has(c):
				bad("第 %d 列 key %q 缺字 %q（U+%04X）", r.line, r.key, c, c)
			case !wide.Wide(c):
				bad("第 %d 列 key %q 含半形字 %q（U+%04X）", r.line, r.key, c, c)
			}
		}
		if n := utf8.RuneCountInString(text); n*8 > 320 {
			bad("第 %d 列 key %q 譯文 %d 字，2 倍寬 %d 超過 320", r.line, r.key, n, n*8)
		}
	}
	for k, in := range keys {
		if _, ok := seen[k]; in && !ok {
			rep.Untranslated = append(rep.Untranslated, k)
		}
	}
	sort.Strings(rep.Untranslated)

	// 防呆（002 §2.3）：解析的三個前提對這份 LABL 清單成立。
	var lk []string
	for k, ok := range labl {
		if ok {
			lk = append(lk, k)
		}
	}
	sort.Strings(lk)
	for _, k := range lk {
		if removeRe.MatchString(k) || countRe.MatchString(k) {
			bad("LABL key %q 符合組合標籤的式子", k)
		}
	}
	for _, k := range WeaponShortNames {
		if labl[k] {
			bad("短代號 %q 也在 LABL 集合", k)
		}
	}
	for _, k := range WeaponLongNames {
		if !labl[k] {
			bad("武器長名稱 %q 不在 LABL 集合", k)
		}
	}
	return rep
}
