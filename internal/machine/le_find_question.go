package machine

import (
	"io/fs"
	"sort"
	"strings"
)

// 規格323：固定8／3欄位的問號匹配；不採主機glob或LFN規則。
func questionDOSMatch(base, ext, candidateBase, candidateExt string) bool {
	var pattern, candidate [11]byte
	copy(pattern[:8], base)
	copy(pattern[8:], ext)
	copy(candidate[:8], candidateBase)
	copy(candidate[8:], candidateExt)
	for i, ch := range pattern {
		if ch != '?' && ch != candidate[i] {
			return false
		}
	}
	return true
}

func firstQuestionDOSName(files ReadOnlyFileProvider, base, ext string) (string, error) {
	if files == nil {
		return "", fs.ErrNotExist
	}
	lister, ok := files.(interface{ ListReadOnlyNames() ([]string, error) })
	if !ok {
		return "", fs.ErrInvalid
	}
	names, err := lister.ListReadOnlyNames()
	if err != nil {
		return "", err
	}
	// 使用自己的副本；穩定排序是明示平台近似，不改provider的資料。
	names = append([]string(nil), names...)
	sort.SliceStable(names, func(i, j int) bool {
		return strings.ToUpper(names[i]) < strings.ToUpper(names[j])
	})
	for _, name := range names {
		candidateBase, candidateExt, valid := exactDOSName(name)
		if valid && questionDOSMatch(base, ext, candidateBase, candidateExt) {
			return name, nil
		}
	}
	return "", fs.ErrNotExist
}
