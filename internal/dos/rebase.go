package dos

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RebasePath 把狀態檔裡記的開檔路徑改寫到 to 底下（docs/spec/450 §2.2）。
//
// path 與每個前綴先 filepath.Clean；Clean 後是 "."（含空字串）的前綴略過。
// path 嚴格位於前綴底下才算符合：等於前綴不算（避免把目錄開成檔案代號），
// `..` 開頭的相對結果不算（`..x` 這類檔名照常接受）。多個前綴符合時取最長的一個，
// 回 filepath.Join(to, rel)；Join 的結果一定在 to 底下。沒有前綴符合就回錯。
func RebasePath(path string, prefixes []string, to string) (string, error) {
	p := filepath.Clean(path)
	best, bestRel := "", ""
	found := false
	for _, pre := range prefixes {
		pre = filepath.Clean(pre)
		if pre == "." {
			continue
		}
		rel, err := filepath.Rel(pre, p)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if !found || len(pre) > len(best) {
			best, bestRel, found = pre, rel, true
		}
	}
	if !found {
		return "", fmt.Errorf("dos: %s 不在任何改寫前綴 %q 底下（docs/spec/450 §2.2）", path, prefixes)
	}
	return filepath.Join(to, bestRel), nil
}
