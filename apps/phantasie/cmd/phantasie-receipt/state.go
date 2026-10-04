package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 存檔收據只觀察獨立的 Scratch 葉目錄（專案規格 005 §10）。
type stateFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type statePoint struct {
	Check string      `json:"check"`
	Hash  string      `json:"hash"`
	Files []stateFile `json:"files"`
}
type stateManifest struct {
	Initial statePoint   `json:"initial"`
	Checks  []statePoint `json:"checks"`
}

func snapshotState(dir, check string) (statePoint, error) {
	p := statePoint{Check: check, Hash: "-", Files: []stateFile{}}
	if dir == "" {
		return p, nil
	}
	di, err := os.Lstat(dir)
	if err != nil {
		return p, err
	}
	if !di.IsDir() || di.Mode()&os.ModeSymlink != 0 {
		return p, fmt.Errorf("存檔層必須是一般目錄：%s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return p, err
	}
	h := sha256.New()
	for _, e := range entries {
		name := e.Name()
		if !utf8.ValidString(name) || strings.ContainsRune(name, '\\') || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return p, fmt.Errorf("存檔檔名無法寫入清冊：%q", name)
		}
		info, err := e.Info()
		if err != nil {
			return p, err
		}
		if !info.Mode().IsRegular() {
			return p, fmt.Errorf("存檔層含非一般檔案：%q", name)
		}
		path := filepath.Join(dir, name)
		links, err := stateLinks(path, info)
		if err != nil {
			return p, err
		}
		if links != 1 {
			return p, fmt.Errorf("存檔層含硬連結檔案：%q", name)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return p, err
		}
		f := stateFile{Name: name, Size: int64(len(b)), SHA256: fmt.Sprintf("%x", sha256.Sum256(b))}
		p.Files = append(p.Files, f)
		fmt.Fprintf(h, "%s\t%d\t%s\n", f.Name, f.Size, f.SHA256)
	}
	p.Hash = fmt.Sprintf("%x", h.Sum(nil))
	return p, nil
}

// 尚不存在的路徑也先解析現有祖先，避免 symlink 把輸出導回原版。
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(abs)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		tail = append(tail, filepath.Base(abs))
		abs = parent
	}
}

func containsDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
func overlapDirs(a, b string) bool { return containsDir(a, b) || containsDir(b, a) }

func prepareStates(root, out, base string, langs []string) (map[string]string, error) {
	states := map[string]string{}
	seen := map[string]bool{}
	for _, lang := range langs {
		switch lang {
		case "zh-TW", "zh-CN", "ja", "ko":
		default:
			return nil, fmt.Errorf("不支援的初始語言：%q", lang)
		}
		if seen[lang] {
			return nil, fmt.Errorf("初始語言重複：%s", lang)
		}
		seen[lang] = true
	}
	if base == "" {
		return states, nil
	}
	r, err := canonicalPath(root)
	if err != nil {
		return nil, err
	}
	b, err := canonicalPath(base)
	if err != nil {
		return nil, err
	}
	dest, err := canonicalPath(out)
	if err != nil {
		return nil, err
	}
	if overlapDirs(r, b) || overlapDirs(r, dest) {
		return nil, fmt.Errorf("原版目錄不得與存檔或收據目錄重疊")
	}
	for _, lang := range langs {
		leaf := filepath.Join(b, lang)
		if containsDir(leaf, dest) {
			return nil, fmt.Errorf("收據目錄不得位於存檔層內：%s", lang)
		}
		if info, err := os.Lstat(leaf); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("存檔層必須是一般目錄：%s", leaf)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		states[lang] = leaf
	}
	// 所有語言先預檢；任何錯誤都不啟動第一組原版。
	for _, lang := range langs {
		leaf := states[lang]
		if err := os.MkdirAll(leaf, 0755); err != nil {
			return nil, err
		}
		if _, err := snapshotState(leaf, "initial"); err != nil {
			return nil, err
		}
	}
	return states, nil
}

func writeStateManifest(path string, m stateManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
