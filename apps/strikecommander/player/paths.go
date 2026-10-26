// Package player implements the value-only 016 frontend core, without Ebiten.
package player

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type Paths struct{ Data, Cache, Save, Original, Screenshots, Settings, ErrorLog string }

func PathsFor(goos string, env map[string]string, home string) (Paths, error) {
	join := func(a string, b ...string) string {
		if goos == "windows" {
			return strings.TrimRight(a, `\/`) + `\` + strings.Join(b, `\`)
		}
		return path.Join(append([]string{a}, b...)...)
	}
	abs := func(s string) bool {
		if goos == "windows" {
			return len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') || strings.HasPrefix(s, `\\`)
		}
		return path.IsAbs(s)
	}
	if home == "" || !abs(home) {
		return Paths{}, errors.New("找不到使用者家目錄")
	}
	var data, cache, large string
	switch goos {
	case "linux":
		data = env["XDG_DATA_HOME"]
		if !abs(data) {
			data = join(home, ".local", "share")
		}
		data = join(data, "strikecommander-cht")
		cache = env["XDG_CACHE_HOME"]
		if !abs(cache) {
			cache = join(home, ".cache")
		}
		cache = join(cache, "strikecommander-cht")
		large = data
	case "windows":
		data = env["APPDATA"]
		if !abs(data) {
			data = join(home, "AppData", "Roaming")
		}
		data = join(data, "StrikeCommanderCHT")
		large = env["LOCALAPPDATA"]
		if !abs(large) {
			large = join(home, "AppData", "Local")
		}
		large = join(large, "StrikeCommanderCHT")
		cache = join(large, "cache")
	case "darwin":
		data = join(home, "Library", "Application Support", "StrikeCommanderCHT")
		cache = join(data, "cache")
		large = data
	default:
		return Paths{}, fmt.Errorf("不支援的平台 %s", goos)
	}
	return Paths{data, cache, join(data, "save"), join(large, "original"), join(large, "screenshots"), join(data, "settings.json"), join(data, "strikecommander-error.log")}, nil
}

// canonical resolves existing ancestors too: a missing output below a symlink
// must not bypass the nesting rejection before MkdirAll.
func canonical(p string) (string, error) {
	p, e := filepath.Abs(p)
	if e != nil {
		return "", e
	}
	p = filepath.Clean(p)
	var tail []string
	for {
		q, e := filepath.EvalSymlinks(p)
		if e == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				q = filepath.Join(q, tail[i])
			}
			return q, nil
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", e
		}
		tail = append(tail, filepath.Base(p))
		p = parent
	}
}
func nested(a, b string) bool {
	if runtime.GOOS == "windows" {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	r, e := filepath.Rel(a, b)
	return e == nil && (r == "." || r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)))
}

type pathAncestor struct {
	info os.FileInfo
	tail string
}

// Existing ancestors retain the filesystem's real identity even when spelling
// differs on a case-insensitive volume. Missing suffixes are never created.
func pathAncestors(p string, stat func(string) (os.FileInfo, error)) ([]pathAncestor, error) {
	var ancestors []pathAncestor
	for current := p; ; current = filepath.Dir(current) {
		info, e := stat(current)
		if e == nil {
			tail, e := filepath.Rel(current, p)
			if e != nil {
				return nil, e
			}
			ancestors = append(ancestors, pathAncestor{info: info, tail: tail})
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		if filepath.Dir(current) == current {
			return ancestors, nil
		}
	}
}

func pathsOverlap(a, b string, stat func(string) (os.FileInfo, error)) (bool, error) {
	if nested(a, b) || nested(b, a) {
		return true, nil
	}
	aa, e := pathAncestors(a, stat)
	if e != nil {
		return false, e
	}
	bb, e := pathAncestors(b, stat)
	if e != nil {
		return false, e
	}
	for _, x := range aa {
		for _, y := range bb {
			if os.SameFile(x.info, y.info) && (nested(x.tail, y.tail) || nested(y.tail, x.tail)) {
				return true, nil
			}
		}
	}
	return false, nil
}

func CheckIndependentPaths(cache, save string, sources []string) error {
	return checkIndependentPathsWithStat(cache, save, sources, os.Stat)
}

func checkIndependentPathsWithStat(cache, save string, sources []string, stat func(string) (os.FileInfo, error)) error {
	c, e := canonical(cache)
	if e != nil {
		return e
	}
	s, e := canonical(save)
	if e != nil {
		return e
	}
	overlap, e := pathsOverlap(c, s, stat)
	if e != nil {
		return e
	}
	if overlap {
		return errors.New("快取與存檔層不可互相巢狀")
	}
	for _, p := range sources {
		q, e := canonical(p)
		if e != nil {
			return e
		}
		for _, x := range []string{c, s} {
			overlap, e := pathsOverlap(x, q, stat)
			if e != nil {
				return e
			}
			if overlap {
				return fmt.Errorf("來源與輸出不可互相巢狀：%s", p)
			}
		}
	}
	return nil
}
func OriginalCandidates(explicit, saved []string, exeDir, packageDir, userOriginal string) []string {
	if len(explicit) > 0 {
		return append([]string(nil), explicit...)
	}
	out := append([]string(nil), saved...)
	out = append(out, filepath.Join(exeDir, "original"), filepath.Join(exeDir, "..", "Resources", "original"), filepath.Join(packageDir, "original"), userOriginal)
	seen := map[string]bool{}
	result := []string{}
	for _, s := range out {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
